#!/usr/bin/env python3
"""Plan UTC calendar releases and commit verified Devbox pins without losing changes."""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
import re
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
PIN = 'nix/release.json'
BOT = ['-c', 'user.name=github-actions[bot]', '-c',
       'user.email=41898282+github-actions[bot]@users.noreply.github.com']


def git(root, *args):
    return subprocess.check_output(['git', *args], cwd=root, text=True).strip()


def version_tuple(tag):
    match = re.fullmatch(r'v([1-9][0-9]{3})\.([1-9]|1[0-2])\.([1-9][0-9]*)', tag)
    return tuple(map(int, match.groups())) if match else None


def ancestor(root, older, newer):
    result = subprocess.run(['git', 'merge-base', '--is-ancestor', older, newer],
                            cwd=root, check=False)
    if result.returncode not in (0, 1):
        raise RuntimeError('cannot determine release ancestry')
    return result.returncode == 0


def plan(root, today):
    tags = [tag for tag in git(root, 'tag', '--list').splitlines() if version_tuple(tag)]
    latest = max(tags, key=version_tuple) if tags else None
    if latest:
        released = git(root, 'rev-parse', latest + '^{commit}')
        head = git(root, 'rev-parse', 'HEAD')
        if head == released:
            # Retry a partial release (including one whose pin update failed).
            return {'release': 'true', 'version': latest}
        if ancestor(root, head, released):
            return {'release': 'false', 'version': ''}
        if not ancestor(root, released, head):
            raise ValueError('latest calendar release is not an ancestor of this source')
        changed = git(root, 'diff', '--name-only', latest, 'HEAD').splitlines()
        if not any(name != PIN for name in changed):
            return {'release': 'false', 'version': ''}
        if version_tuple(latest)[:2] > (today.year, today.month):
            raise ValueError('latest release is ahead of the current UTC month')
    month = (today.year, today.month)
    sequence = max((version_tuple(tag)[2] for tag in tags
                    if version_tuple(tag)[:2] == month), default=0) + 1
    return {'release': 'true', 'version': f'v{today.year}.{today.month}.{sequence}'}


def pin_needed(current, proposed):
    target = version_tuple(proposed['version'])
    if target is None:
        raise ValueError('automatic pins require a calendar version vYYYY.M.N')
    if current is None:
        return True
    existing = version_tuple(current['version'])
    if existing is not None and existing > target:
        return False
    if current['version'] == proposed['version']:
        if current != proposed:
            raise ValueError('the same release version has different metadata; refusing to replace it')
        return False
    return True


def push_pin(root):
    return subprocess.run(['git', 'push', 'origin', 'HEAD:refs/heads/main'],
                          cwd=root, text=True, capture_output=True, check=False)


def update_pin(root, metadata):
    # Each attempt starts at the current remote main. Only the pin is changed;
    # concurrent source commits are preserved and a newer pin is never replaced.
    for _ in range(3):
        git(root, 'fetch', 'origin', 'refs/heads/main')
        base = git(root, 'rev-parse', 'FETCH_HEAD')
        with tempfile.TemporaryDirectory(prefix='seamgraph-release-pin-') as temporary:
            checkout = Path(temporary) / 'checkout'
            git(root, 'worktree', 'add', '--detach', str(checkout), base)
            try:
                destination = checkout / PIN
                current = json.loads(destination.read_text()) if destination.exists() else None
                if not pin_needed(current, metadata):
                    return False
                destination.parent.mkdir(parents=True, exist_ok=True)
                destination.write_text(json.dumps(metadata, indent=2) + '\n')
                git(checkout, 'add', PIN)
                git(checkout, *BOT, 'commit', '-m',
                    f"build: pin Devbox package to {metadata['version']}")
                pushed = push_pin(checkout)
                if pushed.returncode == 0:
                    return True
            finally:
                git(root, 'worktree', 'remove', '--force', str(checkout))
    raise RuntimeError('cannot push Devbox pin after three attempts: ' + pushed.stderr.strip())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    planning = commands.add_parser('plan')
    planning.add_argument('--output', type=Path, help='GitHub Actions output file')
    pinning = commands.add_parser('pin')
    pinning.add_argument('metadata', type=Path, help='metadata verified by update-devbox-release.py')
    args = parser.parse_args()
    try:
        if args.command == 'plan':
            result = plan(ROOT, datetime.now(timezone.utc).date())
            output = ''.join(f'{name}={value}\n' for name, value in result.items())
            print(output, end='')
            if args.output:
                with args.output.open('a') as stream:
                    stream.write(output)
        else:
            changed = update_pin(ROOT, json.loads(args.metadata.read_text()))
            print('Published Devbox pin' if changed else 'Devbox pin is already current or newer')
    except (OSError, ValueError, RuntimeError, subprocess.CalledProcessError) as error:
        parser.exit(1, f'Calendar release failed: {error}\n')


if __name__ == '__main__':
    main()
