#!/usr/bin/env python3
"""Pin verified GitHub release archives for the Devbox flake."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import tempfile
from urllib.request import urlopen

ASSETS = {
    'x86_64-linux': 'seamgraph_linux_amd64.tar.gz',
    'aarch64-linux': 'seamgraph_linux_arm64.tar.gz',
    'x86_64-darwin': 'seamgraph_darwin_amd64.tar.gz',
    'aarch64-darwin': 'seamgraph_darwin_arm64.tar.gz',
}
ROOT = Path(__file__).resolve().parents[1]


def read_asset(base, directory, name):
    if directory is not None:
        return (directory / name).read_bytes()
    with urlopen(f'{base}/{name}', timeout=60) as response:
        return response.read()


def release_metadata(version, repository, directory=None):
    if not re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+(?:[-.][A-Za-z0-9.-]+)?', version):
        raise ValueError('version must be vX.Y.Z, optionally with a prerelease suffix')
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repository):
        raise ValueError('repository must be owner/repo')
    base = f'https://github.com/{repository}/releases/download/{version}'
    checksums = {}
    for line in read_asset(base, directory, 'SHA256SUMS').decode().splitlines():
        parts = line.split()
        if len(parts) != 2 or parts[1] not in ASSETS.values():
            continue
        checksum, name = parts
        if name in checksums or not re.fullmatch(r'[a-fA-F0-9]{64}', checksum):
            raise ValueError(f'duplicate or invalid checksum for {name}')
        checksums[name] = checksum.lower()
    assets = {}
    for system, name in ASSETS.items():
        expected = checksums.get(name)
        if expected is None:
            raise ValueError(f'missing checksum for {name}')
        actual = hashlib.sha256(read_asset(base, directory, name)).hexdigest()
        if actual != expected:
            raise ValueError(f'checksum mismatch for {name}; nothing updated')
        assets[system] = {'name': name, 'sha256': actual}
    return {'repository': repository, 'version': version, 'assets': assets}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('version', help='published tag, for example v0.1.0')
    parser.add_argument('--repository', default='jani123/seamgraph')
    parser.add_argument('--assets-dir', type=Path,
                        help='verify local release artifacts instead of downloading (CI/offline)')
    parser.add_argument('--output', type=Path, default=ROOT / 'nix/release.json')
    args = parser.parse_args()
    try:
        metadata = release_metadata(args.version, args.repository, args.assets_dir)
        args.output.parent.mkdir(parents=True, exist_ok=True)
        temporary = None
        try:
            with tempfile.NamedTemporaryFile(mode='w', dir=args.output.parent,
                                             delete=False) as output:
                temporary = Path(output.name)
                output.write(json.dumps(metadata, indent=2) + '\n')
            os.replace(temporary, args.output)
        finally:
            if temporary is not None:
                temporary.unlink(missing_ok=True)
    except (OSError, ValueError) as error:
        parser.exit(1, f'Devbox release update failed: {error}\n')
    print(f'Pinned {args.repository} {args.version} in {args.output}')


if __name__ == '__main__':
    main()
