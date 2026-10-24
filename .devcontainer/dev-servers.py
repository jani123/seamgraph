#!/usr/bin/env python3
"""Serve the main workspace and disposable examples for one Air build lifetime."""
import json
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]


def prepare_examples():
    examples = json.loads((ROOT / '.devcontainer/examples.json').read_text())
    for example in examples:
        directory = ROOT / '.build/dev-examples' / example['name']
        if not directory.exists():
            directory.parent.mkdir(parents=True, exist_ok=True)
            # Publish only complete repositories, so an interrupted Air build
            # cannot leave a partially generated example at its final path.
            with tempfile.TemporaryDirectory(prefix='prepare-', dir=directory.parent) as temporary:
                prepared = Path(temporary) / 'repository'
                command = [sys.executable, str(ROOT / 'scripts/examples/generate.py'), str(prepared),
                           '--warehouses', str(example['warehouses'])]
                if example['working']:
                    command.append('--working')
                subprocess.run(command, cwd=ROOT, check=True)
                prepared.rename(directory)
        if not (directory / '.seamgraph/scenarios.json').is_file():
            raise RuntimeError(f'Existing example has no scenario manifest: {directory}')
        example['root'] = directory
    return examples


def serve(binary, arguments, examples):
    children = []
    stopping = False
    def stop(*_):
        nonlocal stopping
        stopping = True
    previous = {sig: signal.signal(sig, stop) for sig in (signal.SIGINT, signal.SIGTERM)}
    try:
        children.append(subprocess.Popen([binary, *arguments], cwd=ROOT))
        for example in examples:
            if stopping:
                return 0
            children.append(subprocess.Popen([binary, 'serve', '--root', str(example['root']),
                                              '--addr', f'0.0.0.0:{example["port"]}',
                                              '--allow-network'], cwd=ROOT))
            print(f'{example["label"]}: container port {example["port"]} ({example["root"]})', flush=True)
        while not stopping:
            for child in children:
                code = child.poll()
                if code is not None:
                    print(f'Development server exited ({code}); stopping sibling servers.', file=sys.stderr)
                    return code if code > 0 else 1
            time.sleep(0.1)
        return 0
    finally:
        for child in children:
            if child.poll() is None:
                child.terminate()
        deadline = time.monotonic() + 1
        for child in children:
            try:
                child.wait(timeout=max(0, deadline - time.monotonic()))
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait()
        for sig, handler in previous.items():
            signal.signal(sig, handler)


if __name__ == '__main__':
    sys.exit(serve(sys.argv[1], sys.argv[2:], prepare_examples()))
