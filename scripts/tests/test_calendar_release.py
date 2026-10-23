"""Exercise calendar versions and pin publication with real local Git repositories."""
from datetime import date
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[1] / 'calendar-release.py'
SPEC = importlib.util.spec_from_file_location('calendar_release', SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def git(root, *args):
    return MODULE.git(root, *args)


def commit(root, name, content):
    destination = root / name
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(content)
    git(root, 'add', name)
    git(root, '-c', 'user.name=Test', '-c', 'user.email=test@example.invalid',
        'commit', '-m', 'test: update fixture')
    return git(root, 'rev-parse', 'HEAD')


class CalendarVersionTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix='seamgraph-calendar-')
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        git(self.root, 'init', '-b', 'main')
        self.first = commit(self.root, 'README.md', 'initial\n')
        self.today = date(2026, 10, 23)

    def tag_then_change(self, tag):
        git(self.root, 'tag', tag)
        return commit(self.root, 'README.md', tag + '\n')

    def test_first_release_and_monthly_sequence(self):
        self.assertEqual(MODULE.plan(self.root, self.today),
                         {'release': 'true', 'version': 'v2026.10.1'})
        self.tag_then_change('v2026.10.1')
        self.assertEqual(MODULE.plan(self.root, self.today)['version'], 'v2026.10.2')
        git(self.root, 'tag', 'v2026.10.9', self.first)
        self.assertEqual(MODULE.plan(self.root, self.today)['version'], 'v2026.10.10')

    def test_month_and_year_rollover_reset_sequence(self):
        self.tag_then_change('v2026.10.9')
        self.assertEqual(MODULE.plan(self.root, date(2026, 11, 1))['version'], 'v2026.11.1')
        self.assertEqual(MODULE.plan(self.root, date(2027, 1, 1))['version'], 'v2027.1.1')

    def test_semver_and_prerelease_tags_do_not_affect_calendar_sequence(self):
        for tag in ['v0.1.0', 'v2026.10.1-rc.1', 'not-a-release']:
            git(self.root, 'tag', tag)
        self.assertEqual(MODULE.plan(self.root, self.today)['version'], 'v2026.10.1')

    def test_retry_reuses_version_even_in_a_new_month(self):
        git(self.root, 'tag', 'v2026.10.1')
        self.assertEqual(MODULE.plan(self.root, date(2026, 11, 1)),
                         {'release': 'true', 'version': 'v2026.10.1'})

    def test_pin_only_commit_does_not_create_another_release(self):
        git(self.root, 'tag', 'v2026.10.1')
        commit(self.root, MODULE.PIN, '{}\n')
        self.assertEqual(MODULE.plan(self.root, self.today)['release'], 'false')

    def test_superseded_source_is_not_released_again(self):
        self.tag_then_change('v2026.10.1')
        git(self.root, 'tag', 'v2026.10.2')
        git(self.root, 'checkout', '--detach', self.first)
        self.assertEqual(MODULE.plan(self.root, self.today)['release'], 'false')

    def test_divergent_source_and_future_month_fail(self):
        released = self.tag_then_change('v2026.11.1')
        with self.assertRaisesRegex(ValueError, 'ahead'):
            MODULE.plan(self.root, self.today)
        git(self.root, 'tag', '-d', 'v2026.11.1')
        git(self.root, 'tag', 'v2026.10.1', released)
        git(self.root, 'checkout', '--detach', self.first)
        commit(self.root, 'other.txt', 'divergent\n')
        with self.assertRaisesRegex(ValueError, 'ancestor'):
            MODULE.plan(self.root, self.today)


class CalendarPinTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix='seamgraph-calendar-pin-')
        self.addCleanup(temporary.cleanup)
        self.directory = Path(temporary.name)
        self.remote = self.directory / 'remote.git'
        self.root = self.directory / 'source'
        self.writer = self.directory / 'writer'
        subprocess.run(['git', 'init', '--bare', '-b', 'main', str(self.remote)],
                       check=True, capture_output=True)
        subprocess.run(['git', 'clone', str(self.remote), str(self.root)],
                       check=True, capture_output=True)
        commit(self.root, MODULE.PIN, 'null\n')
        git(self.root, 'push', 'origin', 'main')
        subprocess.run(['git', 'clone', str(self.remote), str(self.writer)],
                       check=True, capture_output=True)
        self.metadata = {'version': 'v2026.10.1', 'repository': 'example/seamgraph', 'assets': {}}

    def remote_pin(self):
        return json.loads(git(self.root, '--git-dir=' + str(self.remote), 'show', 'main:' + MODULE.PIN))

    def test_preserves_concurrent_source_commits_and_uses_conventional_subject(self):
        commit(self.writer, 'new-source.txt', 'new source\n')
        git(self.writer, 'push', 'origin', 'main')
        original = git(self.root, 'rev-parse', 'HEAD')
        self.assertTrue(MODULE.update_pin(self.root, self.metadata))
        self.assertEqual(self.remote_pin(), self.metadata)
        self.assertEqual(git(self.remote, 'show', 'main:new-source.txt'), 'new source')
        self.assertEqual(git(self.remote, 'log', '-1', '--format=%s', 'main'),
                         'build: pin Devbox package to v2026.10.1')
        self.assertEqual(git(self.root, 'rev-parse', 'HEAD'), original)
        self.assertEqual(git(self.root, 'status', '--porcelain'), '')
        self.assertEqual(len(git(self.root, 'worktree', 'list').splitlines()), 1)

    def test_retry_and_older_version_do_not_change_remote(self):
        self.assertTrue(MODULE.update_pin(self.root, self.metadata))
        head = git(self.remote, 'rev-parse', 'main')
        self.assertFalse(MODULE.update_pin(self.root, self.metadata))
        older = {**self.metadata, 'version': 'v2026.9.9'}
        self.assertFalse(MODULE.update_pin(self.root, older))
        self.assertEqual(git(self.remote, 'rev-parse', 'main'), head)

    def test_conflicting_metadata_for_same_version_is_rejected(self):
        MODULE.update_pin(self.root, self.metadata)
        with self.assertRaisesRegex(ValueError, 'different metadata'):
            MODULE.update_pin(self.root, {**self.metadata, 'assets': {'unexpected': 'changed'}})
        self.assertEqual(self.remote_pin(), self.metadata)

    def test_push_race_retries_from_updated_main(self):
        original = MODULE.push_pin
        attempts = []

        def push(checkout):
            if not attempts:
                commit(self.writer, 'concurrent.txt', 'keep this\n')
                git(self.writer, 'push', 'origin', 'main')
            attempts.append(checkout)
            return original(checkout)

        with patch.object(MODULE, 'push_pin', side_effect=push):
            self.assertTrue(MODULE.update_pin(self.root, self.metadata))
        self.assertEqual(len(attempts), 2)
        self.assertEqual(self.remote_pin(), self.metadata)
        self.assertEqual(git(self.remote, 'show', 'main:concurrent.txt'), 'keep this')
