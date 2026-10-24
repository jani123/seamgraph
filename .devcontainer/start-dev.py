#!/usr/bin/env python3
"""Keep one background Air session per checkout, independent of CLI terminals."""
import fcntl
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
STATE = ROOT / '.build'
LOG = STATE / 'dev-server.log'
PID = STATE / 'dev-server.pid'


def run():
    # The supervisor holds the lock for the entire Air lifetime, including builds.
    with (STATE / 'dev-server.lock').open('w') as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return
        PID.write_text(str(os.getpid()))
        try:
            # Group signals also reach Task/Air. Wait for them before releasing
            # the singleton lock instead of exiting ahead of the child.
            signal.signal(signal.SIGTERM, lambda *_: None)
            signal.signal(signal.SIGINT, lambda *_: None)
            result = subprocess.run(['devbox', 'run', '--', 'task', 'dev', '--', *sys.argv[2:]],
                                    cwd=ROOT)
            if result.returncode > 0:
                raise SystemExit(result.returncode)
        finally:
            PID.unlink(missing_ok=True)


def stop():
    if PID.exists():
        pid = int(PID.read_text())
        try:
            command = Path(f'/proc/{pid}/cmdline').read_bytes().split(b'\0')
            if os.fsencode(Path(__file__).resolve()) in command and b'--run' in command:
                os.killpg(pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        except FileNotFoundError:
            pass


def start():
    with LOG.open('ab') as log:
        subprocess.Popen([sys.executable, str(Path(__file__).resolve()), '--run', *sys.argv[1:]],
                         cwd=ROOT, stdin=subprocess.DEVNULL, stdout=log, stderr=log,
                         start_new_session=True)
    print(f'Waiting for the development server; logs: {LOG}', flush=True)
    # Do not send a local readiness probe through the container's egress proxy.
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    deadline = time.monotonic() + 120
    ports = [8765, *[example['port'] for example in json.loads((ROOT / '.devcontainer/examples.json').read_text())]]
    pending = set(ports)
    while time.monotonic() < deadline:
        for port in list(pending):
            try:
                with client.open(f'http://127.0.0.1:{port}/', timeout=2) as response:
                    if response.status == 200:
                        pending.remove(port)
            except (OSError, urllib.error.URLError):
                pass
        if not pending:
            print('Development servers ready on container ports ' + ', '.join(map(str, ports)) + '.', flush=True)
            return
        time.sleep(1)
    raise SystemExit(f'Development servers did not become ready within 120s (pending ports: {sorted(pending)}). See {LOG}')


if __name__ == '__main__':
    STATE.mkdir(exist_ok=True)
    if sys.argv[1:2] == ['--run']:
        run()
    elif sys.argv[1:2] == ['--stop']:
        stop()
    else:
        start()
