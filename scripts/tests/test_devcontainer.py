"""CLI-oriented port discovery and dev launcher checks without a Docker daemon."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import shutil
import socket
import urllib.error
import urllib.request
import tempfile
import time
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('browser_url', ROOT / '.devcontainer/browser-url.py')
url_module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(url_module)


class BrowserURLTests(unittest.TestCase):
    def test_discovers_ephemeral_port_with_actual_compose_project(self):
        calls = []
        def docker(*args):
            calls.append(args)
            if args[0] == 'ps':
                return 'relay-id\n' if any('service=dev-ui' in a for a in args) else 'workspace-id\n'
            if args[1] == 'workspace-id':
                return json.dumps([{'Config': {'Labels': {'com.docker.compose.project': 'arbitrary-project-name'}}}])
            return json.dumps([{'NetworkSettings': {'Ports': {'8080/tcp': [{'HostIp': '127.0.0.1', 'HostPort': '49173'}]}}}])
        with patch.object(url_module, 'docker', side_effect=docker):
            self.assertEqual(url_module.browser_url('/host/project with spaces'), 'http://127.0.0.1:49173')
        self.assertTrue(any('label=devcontainer.local_folder=/host/project with spaces' in c for c in calls))
        self.assertTrue(any('label=com.docker.compose.project=arbitrary-project-name' in c for c in calls))

    def test_does_not_guess_when_workspace_is_ambiguous(self):
        with patch.object(url_module, 'docker', return_value='first\nsecond\n'):
            with self.assertRaisesRegex(RuntimeError, 'Expected one'):
                url_module.browser_url('/host/project')

    def test_rejects_missing_or_nonlocal_publication(self):
        for ports in [{}, {'8080/tcp': None}, {'8080/tcp': [{'HostIp': '0.0.0.0', 'HostPort': '49173'}]}]:
            with self.subTest(ports=ports):
                responses = ['workspace', json.dumps([{'Config': {'Labels': {'com.docker.compose.project': 'project'}}}]), 'relay', json.dumps([{'NetworkSettings': {'Ports': ports}}])]
                with patch.object(url_module, 'docker', side_effect=responses):
                    with self.assertRaisesRegex(RuntimeError, 'localhost port'):
                        url_module.browser_url('/host/project')


class DevLauncherTests(unittest.TestCase):
    def test_background_supervisor_is_singleton_and_can_stop(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / '.devcontainer').mkdir()
            (root / '.build').mkdir()
            script = root / '.devcontainer/start-dev.py'
            script.write_text((ROOT / '.devcontainer/start-dev.py').read_text())
            stub = root / 'devbox'
            stub.write_text('#!/bin/sh\necho started >> starts\nexec sleep 60\n')
            stub.chmod(0o755)
            env = dict(os.environ, PATH=f'{root}:{os.environ["PATH"]}')
            first = subprocess.Popen(['python3', str(script), '--run'], env=env,
                                     start_new_session=True)
            try:
                deadline = time.monotonic() + 5
                while not (root / 'starts').exists() and time.monotonic() < deadline:
                    time.sleep(0.02)
                self.assertTrue((root / 'starts').exists())
                subprocess.run(['python3', str(script), '--run'], env=env,
                               timeout=5, check=True)
                self.assertEqual((root / 'starts').read_text().splitlines(), ['started'])
                subprocess.run(['python3', str(script), '--stop'], env=env,
                               timeout=5, check=True)
                first.wait(timeout=5)
                self.assertFalse((root / '.build/dev-server.pid').exists())
            finally:
                if first.poll() is None:
                    os.killpg(first.pid, 9)
                    first.wait()

    def arguments(self, container):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            binary = root / '.build/air/seamgraph'
            binary.parent.mkdir(parents=True)
            binary.write_text('#!/bin/sh\nprintf "%s\\n" "$@"\n')
            binary.chmod(0o755)
            state = root / 'state'
            state.write_text('127.0.0.1:45678\n')
            env = dict(os.environ, SEAMGRAPH_DEV_STATE=str(state))
            env.pop('SEAMGRAPH_DEV_ADDR', None)
            env.pop('SEAMGRAPH_DEVCONTAINER', None)
            if container:
                env['SEAMGRAPH_DEVCONTAINER'] = '1'
                python = root / 'python3'
                python.write_text('#!/bin/sh\nprintf "%s\\n" "$@"\n')
                python.chmod(0o755)
                env['PATH'] = f'{root}:{os.environ["PATH"]}'
            result = subprocess.run(['sh', str(ROOT / 'scripts/dev-server.sh')], cwd=root, env=env, text=True, capture_output=True, check=True)
            return result.stdout.splitlines()

    def test_container_uses_published_port_and_explicit_network_mode(self):
        args = self.arguments(True)
        self.assertEqual(args[:2], ['.devcontainer/dev-servers.py', '.build/air/seamgraph'])
        self.assertIn('--allow-network', args)
        self.assertEqual(args[args.index('--addr') + 1], '0.0.0.0:8765')

    def test_native_launcher_retains_session_loopback_port(self):
        args = self.arguments(False)
        self.assertNotIn('--allow-network', args)
        self.assertEqual(args[args.index('--addr') + 1], '127.0.0.1:45678')



class ExamplePortTests(unittest.TestCase):
    def test_example_url_selects_its_own_localhost_binding(self):
        responses = ['workspace', json.dumps([{'Config': {'Labels': {'com.docker.compose.project': 'project'}}}]),
                     'relay', json.dumps([{'NetworkSettings': {'Ports': {
                         '8080/tcp': [{'HostIp': '127.0.0.1', 'HostPort': '49000'}],
                         '8082/tcp': [{'HostIp': '127.0.0.1', 'HostPort': '49002'}]}}}])]
        with patch.object(url_module, 'docker', side_effect=responses):
            self.assertEqual(url_module.browser_url('/host/project', 8082), 'http://127.0.0.1:49002')

    def test_forwarding_and_fixed_relay_targets_cover_every_example(self):
        examples = json.loads((ROOT / '.devcontainer/examples.json').read_text())
        config = json.loads((ROOT / '.devcontainer/devcontainer.json').read_text())
        relay = (ROOT / '.devcontainer/dev-ui/default.conf').read_text()
        compose = (ROOT / '.devcontainer/compose.yaml').read_text()
        self.assertEqual(len({e['port'] for e in examples}), len(examples))
        for example in examples:
            self.assertIn(example['port'], config['forwardPorts'])
            self.assertEqual(config['portsAttributes'][str(example['port'])]['label'], example['label'])
            self.assertIn(f"listen {example['relayPort']};", relay)
            self.assertIn(f"{example['relayPort']} {example['port']};", relay)
            self.assertIn(f"- target: {example['relayPort']}\n        host_ip: 127.0.0.1", compose)

    def test_live_servers_restart_without_resetting_reviews_and_stop_together(self):
        with tempfile.TemporaryDirectory(prefix='seamgraph-dev-ports-') as directory:
            root = Path(directory)
            (root / '.devcontainer').mkdir()
            (root / 'scripts/examples').mkdir(parents=True)
            shutil.copy(ROOT / 'scripts/examples/generate.py', root / 'scripts/examples/generate.py')
            shutil.copytree(ROOT / 'examples/order-fulfillment/app', root / 'examples/order-fulfillment/app',
                            ignore=shutil.ignore_patterns('__pycache__', '.seamgraph'))
            script = root / '.devcontainer/dev-servers.py'
            shutil.copy(ROOT / '.devcontainer/dev-servers.py', script)
            examples = json.loads((ROOT / '.devcontainer/examples.json').read_text())
            reserved = [socket.socket() for _ in range(4)]
            try:
                for sock in reserved:
                    sock.bind(('127.0.0.1', 0))
                ports = [sock.getsockname()[1] for sock in reserved]
            finally:
                for sock in reserved:
                    sock.close()
            for example, port in zip(examples, ports[1:]):
                example['port'] = port
            (root / '.devcontainer/examples.json').write_text(json.dumps(examples))
            client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
            def start(log):
                process = subprocess.Popen(['python3', str(script), str(ROOT / '.build/seamgraph'),
                                            'serve', '--root', str(ROOT / 'testdata/projects/go'),
                                            '--addr', f'127.0.0.1:{ports[0]}'], stdout=log, stderr=log)
                def cleanup():
                    if process.poll() is None:
                        process.terminate()
                        try:
                            process.wait(timeout=5)
                        except subprocess.TimeoutExpired:
                            process.kill()
                            process.wait()
                self.addCleanup(cleanup)
                pending = set(ports)
                deadline = time.monotonic() + 25
                while pending and time.monotonic() < deadline:
                    self.assertIsNone(process.poll(), (root / 'servers.log').read_text())
                    for port in list(pending):
                        try:
                            with client.open(f'http://127.0.0.1:{port}/', timeout=0.2) as response:
                                if response.status == 200:
                                    pending.remove(port)
                        except OSError:
                            pass
                    time.sleep(0.05)
                self.assertFalse(pending, (root / 'servers.log').read_text())
                return process
            def assert_stopped():
                for port in ports:
                    with self.assertRaises(urllib.error.URLError):
                        client.open(f'http://127.0.0.1:{port}/', timeout=0.2)
            with (root / 'servers.log').open('w') as log:
                process = start(log)
                try:
                    for port, count in zip(ports, [2, 18, 18, 138]):
                        with client.open(f'http://127.0.0.1:{port}/api/model', timeout=20) as response:
                            model = json.load(response)
                        self.assertEqual(len(model['snapshot']['modules']), count)
                    working = root / '.build/dev-examples/working'
                    state = subprocess.check_output(['git', '-C', str(working), 'status', '--porcelain'])
                    self.assertIn(b'MM storefront/checkout/checkout.ts', state)
                    self.assertIn(b'?? unassigned/new-review.txt', state)
                    note = working / '.seamgraph/local-review.txt'
                    note.write_text('Keep this review across rebuilds.\n')
                    process.terminate()
                    self.assertEqual(process.wait(timeout=5), 0)
                    assert_stopped()
                    process = start(log)
                    self.assertEqual(note.read_text(), 'Keep this review across rebuilds.\n')
                    self.assertEqual(subprocess.check_output(['git', '-C', str(working), 'status', '--porcelain']), state)
                    # An unavailable sibling must stop the group rather than leave
                    # a partially healthy set of example ports after an Air build.
                    children = Path(f'/proc/{process.pid}/task/{process.pid}/children').read_text().split()
                    self.assertEqual(len(children), 4)
                    os.kill(int(children[-1]), 15)
                    self.assertEqual(process.wait(timeout=5), 1)
                    assert_stopped()
                finally:
                    if process.poll() is None:
                        process.terminate()
                        process.wait(timeout=5)


if __name__ == '__main__':
    unittest.main()
