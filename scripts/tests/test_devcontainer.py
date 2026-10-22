"""CLI-oriented port discovery and dev launcher checks without a Docker daemon."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
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
            result = subprocess.run(['sh', str(ROOT / 'scripts/dev-server.sh')], cwd=root, env=env, text=True, capture_output=True, check=True)
            return result.stdout.splitlines()

    def test_container_uses_published_port_and_explicit_network_mode(self):
        args = self.arguments(True)
        self.assertIn('--allow-network', args)
        self.assertEqual(args[args.index('--addr') + 1], '0.0.0.0:8765')

    def test_native_launcher_retains_session_loopback_port(self):
        args = self.arguments(False)
        self.assertNotIn('--allow-network', args)
        self.assertEqual(args[args.index('--addr') + 1], '127.0.0.1:45678')


if __name__ == '__main__':
    unittest.main()
