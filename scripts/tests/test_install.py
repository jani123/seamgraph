"""Exercise the actual installer without network or real system modifications."""
import hashlib
import io
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'install.sh'

class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='seamgraph-installer-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.tools = self.root / 'tools'
        self.tools.mkdir()
        self.install = self.root / 'with spaces' / 'bin'
        self.archive = self.root / 'seamgraph_linux_amd64.tar.gz'
        content = b'#!/bin/sh\nif [ "$1" = doctor ]; then exit "${TEST_DOCTOR_EXIT:-0}"; fi\necho "seamgraph v1.2.3"\n'
        with tarfile.open(self.archive, 'w:gz') as t:
            info = tarfile.TarInfo('seamgraph')
            info.mode = 0o755
            info.size = len(content)
            t.addfile(info, io.BytesIO(content))
        self.sums = self.root / 'SHA256SUMS'
        self.sums.write_text(hashlib.sha256(self.archive.read_bytes()).hexdigest() + '  ' + self.archive.name + '\n')
        self.executable('uname', '#!/bin/sh\ncase "$1" in -s) echo Linux;; -m) echo x86_64;; esac\n')
        self.executable('curl', '''#!/bin/sh
for arg do
 case "$arg" in https://*) url=$arg;; esac
 last=$arg
done
if [ -n "${TEST_FETCH_LOG:-}" ]; then printf '%s\n' "$url" >> "$TEST_FETCH_LOG"; fi
case "$url" in */SHA256SUMS) cp "$TEST_SUMS" "$last";; *) cp "$TEST_ARCHIVE" "$last";; esac
''')
        self.env = dict(os.environ, PATH=str(self.tools) + os.pathsep + os.environ['PATH'], TEST_SUMS=str(self.sums), TEST_ARCHIVE=str(self.archive))

    def executable(self, name, content):
        p = self.tools / name
        p.write_text(content)
        p.chmod(0o755)

    def run_install(self, *args):
        return subprocess.run(['sh', str(SCRIPT), '--repository', 'example/seamgraph', '--version', 'v1.2.3', '--bin-dir', str(self.install), *args], env=self.env, text=True, capture_output=True)

    def test_install_and_check_selected_languages(self):
        result = self.run_install('--languages', 'python')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(os.access(self.install / 'seamgraph', os.X_OK))
        self.assertIn('seamgraph v1.2.3', result.stdout)
        self.assertEqual(list(self.install.glob('.seamgraph-install.*')), [])

    def test_checksum_failure_preserves_existing_install(self):
        self.install.mkdir(parents=True)
        target = self.install / 'seamgraph'
        target.write_text('existing installation')
        self.sums.write_text('0' * 64 + '  ' + self.archive.name + '\n')
        result = self.run_install()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('checksum mismatch', result.stderr)
        self.assertEqual(target.read_text(), 'existing installation')

    def test_missing_runtime_keeps_installed_binary_and_returns_two(self):
        self.env['TEST_DOCTOR_EXIT'] = '2'
        result = self.run_install('--languages', 'python')
        self.assertEqual(result.returncode, 2)
        self.assertTrue((self.install / 'seamgraph').exists())
        self.assertIn('SeamGraph is installed', result.stderr)

    def test_intel_macos_is_rejected_before_download(self):
        self.executable('uname', '#!/bin/sh\ncase "$1" in -s) echo Darwin;; -m) echo x86_64;; esac\n')
        result = self.run_install()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Intel macOS is not supported', result.stderr)
        self.assertFalse(self.install.exists())

    def test_unsupported_platform_does_not_install(self):
        self.executable('uname', '#!/bin/sh\necho Unsupported\n')
        result = self.run_install()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.install.exists())

    def test_stamped_installer_pins_its_release(self):
        stamped = self.root / 'stamped.sh'
        stamped.write_text(SCRIPT.read_text().replace("DEFAULT_REPOSITORY='jani123/seamgraph'", "DEFAULT_REPOSITORY='example/seamgraph'").replace('__SEAMGRAPH_VERSION__', 'v1.2.3'))
        log = self.root / 'fetch-log'
        self.env['TEST_FETCH_LOG'] = str(log)
        result = subprocess.run(['sh', str(stamped), '--bin-dir', str(self.install)], env=self.env, text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('/releases/download/v1.2.3/', log.read_text())
        self.assertNotIn('/latest/', log.read_text())

    def test_unknown_flag_is_rejected(self):
        result = self.run_install('--sudo')
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.install.exists())

if __name__ == '__main__':
    unittest.main()
