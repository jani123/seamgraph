"""Acceptance checks against authored facts, real runners and CLI workflows."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from urllib.parse import unquote

from generate import CHECKOUT, REPO, generate, git

CLI = REPO / '.build/seamgraph'
TSC = REPO / 'tools/ts-analyzer/node_modules/typescript/bin/tsc'


def command(root, args, expected=0, input=None):
    result = subprocess.run([str(arg) for arg in args], cwd=root, input=input,
                            text=True, capture_output=True, timeout=120)
    if result.returncode != expected:
        raise AssertionError(f'{args}: expected {expected}, got {result.returncode}\n{result.stdout}\n{result.stderr}')
    return result.stdout


class Scenarios(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temporary = tempfile.TemporaryDirectory(prefix='seamgraph-order-review-')
        cls.addClassCleanup(cls.temporary.cleanup)
        cls.root = Path(cls.temporary.name) / 'review'
        cls.expected = generate(cls.root)

    def setUp(self):
        git(self.root, 'checkout', '-q', 'main')
        evidence = self.root / '.seamgraph/evidence.json'
        evidence.unlink(missing_ok=True)

    def invoke(self, *args, expected=0):
        output = command(self.root, [CLI, *args, '--root', self.root], expected)
        return json.loads(output) if '--json' in args else output

    def checkout(self, branch):
        git(self.root, 'checkout', '-q', branch)

    def test_application_flows(self):
        for branch in ['main', 'review/cancellation']:
            with self.subTest(branch=branch):
                self.checkout(branch)
                command(self.root, ['go', 'test', '-race', '-count=1', './...'])
                command(self.root / 'python', [sys.executable, '-B', '-m', 'unittest', '-v', 'test_flow'])
                command(self.root, ['node', TSC, '-p', 'storefront/tsconfig.json', '--noEmit', 'false',
                                    '--module', 'commonjs', '--moduleResolution', 'node', '--outDir', '.seamgraph/ts'])
                command(self.root, ['node', '.seamgraph/ts/tests/checkout.test.js'])
                event = command(self.root, ['go', 'run', './cmd/demo'])
                summary = command(self.root, [sys.executable, '-B', 'python/demo.py'], input=event)
                self.assertEqual(json.loads(summary), {'fulfilled': 1, 'order_ids': ['o-1']})
                self.assertEqual(self.invoke('check', '--json')['violations'], [])

    def test_graph_facts_and_exports(self):
        snapshot = self.invoke('scan', '--json')
        self.assertEqual(len(snapshot['modules']), 18)
        symbols = {unquote(s['id']): s for s in snapshot['symbols']}
        for suffix in ['orders.Service.Place', 'orders.Order.ID', 'orders.Status',
                       'Cart.total', 'Client.place', 'Shipment', 'Worker.process', 'Worker.count']:
            self.assertTrue(any(s.endswith(suffix) for s in symbols), suffix)
        self.assertTrue(any(s['kind'] == 'interface' for s in symbols.values()))
        self.assertTrue(any(s['visibility'] == 'private' for s in symbols.values()))
        imports = {(r['fromId'], r['toId']) for r in snapshot['relations'] if r['kind'] == 'imports'}
        for provider, consumer in [('pricing', 'checkout'), ('discounts', 'pricing'), ('tax', 'discounts'),
                                   ('routing', 'tax'), ('dispatch', 'routing'), ('audit', 'dispatch'),
                                   ('pricing', 'tax')]:
            self.assertIn(('module:' + consumer, 'module:' + provider), imports)
        self.assertFalse(any((source.startswith('module:fulfillment') and target.startswith('module:orders'))
                             for source, target in imports))
        self.assertEqual({d['code'] for d in snapshot['diagnostics']}, {'TS_DYNAMIC_REFERENCE'})
        details = self.invoke('inspect', 'seam:orders.lifecycle', '--json')
        self.assertEqual({r['fromId'] for r in details['relations'] if r['kind'] == 'uses'}, {'module:demo', 'module:http', 'module:orders'})
        self.assertTrue(all(r['evidence'] for r in details['relations']))
        export = self.root / '.seamgraph/snapshot.json'
        self.invoke('snapshot', '--output', str(export))
        self.assertEqual(json.loads(export.read_text()), snapshot)

    def test_boundary_and_revision_comparisons(self):
        self.checkout('review/boundary')
        violations = self.invoke('check', '--json', expected=1)['violations']
        self.assertEqual(len(violations), 1)
        self.assertIn('DebugAvailable', str(violations))
        self.checkout('review/boundary-fixed')
        self.assertEqual(self.invoke('check', '--json')['violations'], [])
        before = git(self.root, 'status', '--porcelain')
        delta = self.invoke('diff', '--base', 'main', '--head', 'review/boundary', '--json')
        self.assertEqual(delta['base'], self.expected['refs']['main'])
        self.assertEqual(delta['head'], self.expected['refs']['review/boundary'])
        self.assertIn('DebugAvailable', str(delta['diff']['changes']))
        self.assertEqual(git(self.root, 'status', '--porcelain'), before)
        resolved = self.invoke('diff', '--base', 'review/boundary', '--head', 'review/boundary-fixed', '--json')
        self.assertTrue(any(c['kind'] == 'violation' and c['status'] == 'removed' for c in resolved['diff']['changes']))
        cancellation = self.invoke('diff', '--base', 'main', '--head', 'review/cancellation', '--json')
        self.assertIn('Service.Cancel', str(cancellation['diff']['changes']))
        self.assertIn('Client.cancel', str(cancellation['diff']['changes']))

    def test_payment_break_is_explicit_incomplete_analysis(self):
        self.checkout('review/payment-breaking')
        result = subprocess.run([CLI, 'check', '--root', self.root, '--json'], capture_output=True, text=True)
        self.assertEqual(result.returncode, 2)
        self.assertIn('int64', result.stderr)
        self.assertEqual(result.stdout, '')
        result = subprocess.run(['go', 'test', './...'], cwd=self.root, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('int64', result.stderr)
        self.checkout('review/payment-fixed')
        self.assertEqual(self.invoke('check', '--json')['violations'], [])
        delta = self.invoke('diff', '--base', 'main', '--head', 'review/payment-fixed', '--json')
        changes = delta['diff']['changes']
        self.assertTrue(any(c['kind'] == 'symbol' and c['status'] == 'modified' and 'Authorize' in c['entityId'] for c in changes))
        self.assertIn('orders/order.go', str(delta))

    def test_evidence_transitions_and_revision_identity(self):
        self.invoke('test', '--runner', 'go')
        def status(expected, code):
            report = self.invoke('check', '--contracts', '--json', expected=code)
            result = next(r for r in report['contracts']['results'] if r['seamId'] == 'seam:orders.lifecycle')
            self.assertEqual(result['status'], expected)
            self.assertFalse(result['memberCoverage']['known'])
            return result
        status('verified', 0)
        self.checkout('review/stale')
        status('stale', 2)
        self.assertIn('Any positive amount', (self.root / 'docs/checkout.md').read_text())
        self.assertFalse((self.root / 'docs/missing-retry.md').exists())
        self.checkout('review/evidence-fixed')
        self.invoke('test', '--runner', 'go')
        status('verified', 0)
        self.assertFalse((self.root / 'docs/checkout-copy.md').exists())
        self.checkout('review/evidence-failed')
        self.invoke('test', '--runner', 'go', expected=2)
        status('failed', 1)
        self.checkout('review/evidence-missing')
        self.invoke('test', '--runner', 'go')
        missing = status('partial', 1)
        self.assertIn('behavior:order.unmapped', str(missing))

    def test_history_oracle_against_git(self):
        summary = self.expected['history']
        self.assertEqual(summary['groups'], {'feat': 1, 'fix': 1, 'revert': 1, 'chore': 1,
                                            'unmatched': 1, 'refactor': 2, 'assets': 1})
        self.assertEqual(summary['scopes'], {'checkout': 2, 'fixtures': 1, 'files': 1, 'status': 1, 'brand': 1, '(none)': 2})
        self.assertEqual(len(summary['breakingCommits']), 2)
        self.assertEqual(summary['files'][CHECKOUT], {'touches': 3, 'added': 3, 'deleted': 3, 'churn': 6, 'binary': False})
        self.assertNotIn(CHECKOUT, summary['net'])
        self.assertNotIn('unassigned/temporary.txt', summary['net'])
        self.assertIsNone(summary['net']['unassigned/logo.bin'])
        self.assertEqual(len(git(self.root, 'rev-list', '--parents', '-n', '1', summary['head']).split()), 3)
        self.assertEqual(git(self.root, 'merge-base', 'main', 'target'), summary['base'])
        self.assertEqual(git(self.root, 'merge-base', 'target', 'review/history'), summary['target'])
        activity = set(git(self.root, 'rev-list', '--no-merges', 'target..review/history').splitlines())
        self.assertEqual(activity, set(summary['activityRevisions']))
        def numstat(*args):
            result = {}
            for line in git(self.root, *args).splitlines():
                added, deleted, name = line.split('\t')
                result[name] = None if added == '-' else {'added': int(added), 'deleted': int(deleted), 'churn': int(added) + int(deleted)}
            return result
        for commit in self.expected['commits']:
            if commit['revision'] in activity:
                self.assertEqual(numstat('diff-tree', '--no-commit-id', '--numstat', '-r', '--no-renames', commit['revision']), commit['files'])
        self.assertEqual(numstat('diff', '--numstat', '--no-renames', summary['base'], summary['head']), summary['net'])
        moves = git(self.root, 'diff', '--name-status', '-M', summary['base'], summary['head'])
        self.assertIn('R100\tunassigned/status.txt\tunassigned/overview.txt', moves)
        self.assertRegex(moves, r'R0[0-9]{2}\tstorefront/status/status.ts\tstorefront/status/order-status.ts')
        # Tree comparison against the diverging target excludes its already-merged note.
        self.assertNotIn('docs/target.md', numstat('diff', '--numstat', '--no-renames', summary['target'], summary['head']))

    def test_reproducibility_and_working_changes(self):
        other = Path(self.temporary.name) / 'working'
        metadata = generate(other, working=True)
        self.assertEqual(metadata['refs'], self.expected['refs'])
        self.assertEqual(git(other, 'status', '--porcelain').splitlines(),
                         ['MM ' + CHECKOUT, '?? unassigned/new-review.txt'])
        staged = git(other, 'diff', '--cached')
        unstaged = git(other, 'diff')
        self.assertIn('Stage order', staged)
        self.assertIn('Unstaged order', unstaged)
        before = git(other, 'status', '--porcelain')
        diff = json.loads(command(other, [CLI, 'diff', '--root', other, '--base', 'main', '--head', 'WORKTREE', '--json']))
        self.assertIn(CHECKOUT, str(diff))
        self.assertIn('unassigned/new-review.txt', str(diff))
        committed = command(other, [CLI, 'diff', '--root', other, '--base', 'main', '--head', 'review/history', '--json'])
        self.assertNotIn('unassigned/new-review.txt', committed)
        self.assertNotIn('Unstaged order', committed)
        self.assertNotIn(CHECKOUT, [f['path'] for f in json.loads(committed)['files']])
        self.assertEqual(git(other, 'status', '--porcelain'), before)
        with self.assertRaises(FileExistsError):
            generate(other)
        with self.assertRaises(ValueError):
            generate(Path(self.temporary.name) / 'too-large', warehouses=201)

    def test_bounded_larger_graph(self):
        root = Path(self.temporary.name) / 'large'
        metadata = generate(root, warehouses=120)
        started = time.monotonic()
        graph = json.loads(command(root, [CLI, 'scan', '--root', root, '--json']))
        elapsed = time.monotonic() - started
        self.assertEqual(len(graph['modules']), 138)
        self.assertEqual(metadata['size']['warehouseFiles'], 120)
        self.assertGreater(len(graph['symbols']), 600)
        self.assertLess(elapsed, 30, f'138-module scan took {elapsed:.1f}s (budget 30s)')
        self.assertLess(len(json.dumps(graph)), 3_000_000)
        print(f'\nLarger scenario: {len(graph["modules"])} modules, {len(graph["symbols"])} symbols, {elapsed:.2f}s', flush=True)


if __name__ == '__main__':
    unittest.main()
