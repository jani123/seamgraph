"""Verify release pins reject incomplete/tampered artifacts without changing a pin."""
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[1] / 'update-devbox-release.py'
SPEC = importlib.util.spec_from_file_location('devbox_release', SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class DevboxReleaseTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix='seamgraph-devbox-release-')
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.output = self.root / 'release.json'
        self.output.write_text('existing pin\n')
        self.checksums = []
        for name in MODULE.ASSETS.values():
            data = name.encode()
            (self.root / name).write_bytes(data)
            self.checksums.append(f'{hashlib.sha256(data).hexdigest()}  {name}')
        self.write_checksums()

    def write_checksums(self):
        (self.root / 'SHA256SUMS').write_text('\n'.join(self.checksums) + '\n')

    def update(self, version='v1.2.3', repository='example/seamgraph'):
        return subprocess.run(
            [sys.executable, str(SCRIPT), version, '--repository', repository,
             '--assets-dir', str(self.root), '--output', str(self.output)],
            text=True, capture_output=True, check=False,
        )

    def assert_rejected(self, result):
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertEqual(self.output.read_text(), 'existing pin\n')

    def test_pins_all_supported_verified_archives(self):
        result = self.update(version='v1.2.3-rc.1')
        self.assertEqual(result.returncode, 0, result.stderr)
        metadata = json.loads(self.output.read_text())
        self.assertEqual(metadata['version'], 'v1.2.3-rc.1')
        self.assertEqual(metadata['repository'], 'example/seamgraph')
        self.assertEqual(set(metadata['assets']), set(MODULE.ASSETS))
        for system, asset in metadata['assets'].items():
            self.assertEqual(asset['name'], MODULE.ASSETS[system])
            self.assertEqual(asset['sha256'], hashlib.sha256(asset['name'].encode()).hexdigest())

    def test_tampered_archive_preserves_existing_pin(self):
        (self.root / 'seamgraph_darwin_arm64.tar.gz').write_bytes(b'tampered')
        result = self.update()
        self.assert_rejected(result)
        self.assertIn('checksum mismatch', result.stderr)

    def test_missing_archive_preserves_existing_pin(self):
        (self.root / 'seamgraph_linux_arm64.tar.gz').unlink()
        self.assert_rejected(self.update())

    def test_missing_duplicate_and_invalid_checksums_are_rejected(self):
        original = self.checksums.copy()
        for lines in [original[1:], original + original[:1],
                      ['invalid  seamgraph_linux_amd64.tar.gz'] + original[1:]]:
            with self.subTest(lines=lines):
                self.checksums = lines
                self.write_checksums()
                self.assert_rejected(self.update())

    def test_invalid_version_and_repository_are_rejected(self):
        for args in [('latest', 'example/seamgraph'), ('v1.2.3', '../bad/repo')]:
            with self.subTest(args=args):
                self.assert_rejected(self.update(*args))

    def test_published_downloads_use_pinned_release_urls(self):
        fetched = []

        def download(url, timeout):
            self.assertEqual(timeout, 60)
            fetched.append(url)
            return (self.root / url.rsplit('/', 1)[1]).open('rb')

        with patch.object(MODULE, 'urlopen', side_effect=download):
            metadata = MODULE.release_metadata('v1.2.3', 'example/seamgraph')
        base = 'https://github.com/example/seamgraph/releases/download/v1.2.3/'
        self.assertEqual(fetched, [base + name for name in ['SHA256SUMS', *MODULE.ASSETS.values()]])
        self.assertEqual(len(metadata['assets']), 3)
