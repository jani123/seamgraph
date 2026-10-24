#!/usr/bin/env python3
"""Create disposable local review repositories without remote dependencies."""
import argparse
import difflib
import json
import os
from pathlib import Path
import shutil
import subprocess

REPO = Path(__file__).resolve().parents[2]
TEMPLATE = REPO / 'examples/order-fulfillment/app'
CHECKOUT = 'storefront/checkout/checkout.ts'


def git(root, *args):
    # Ignore caller Git routing/identity settings; never run hooks or sign commits.
    env = {key: value for key, value in os.environ.items() if not key.startswith('GIT_')}
    env.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull,
               GIT_AUTHOR_NAME='Example Author', GIT_AUTHOR_EMAIL='author@example.invalid',
               GIT_COMMITTER_NAME='Example Author', GIT_COMMITTER_EMAIL='author@example.invalid',
               GIT_AUTHOR_DATE='2026-01-01T12:00:00+0000',
               GIT_COMMITTER_DATE='2026-01-01T12:00:00+0000', LC_ALL='C')
    return subprocess.run(['git', '-C', str(root), '-c', 'core.hooksPath=' + os.devnull,
                           '-c', 'commit.gpgsign=false', '-c', 'core.autocrlf=false',
                           *args], env=env, check=True, capture_output=True).stdout.decode().strip()


def write(root, name, content):
    path = root / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content)


def edit(root, name, old, new):
    path = root / name
    source = path.read_text()
    if old not in source:
        raise ValueError(f'{name}: missing edit anchor {old!r}')
    path.write_text(source.replace(old, new))


def tree(root):
    return {p.relative_to(root).as_posix(): p.read_bytes() for p in root.rglob('*')
            if p.is_file() and not any(part in {'.git', '.seamgraph', '__pycache__'}
                                      for part in p.relative_to(root).parts)}


def line_delta(before, after):
    """Authored-tree oracle, independent of Git/SeamGraph rename and diff logic.

    Disable rename attribution, use disjoint deletes/inserts from SequenceMatcher.
    Scenario text deliberately has simple, unambiguous hunks. Binary is unknown.
    """
    if b'\0' in before or b'\0' in after:
        return None
    added = deleted = 0
    matcher = difflib.SequenceMatcher(None, before.splitlines(), after.splitlines(), autojunk=False)
    for tag, i, j, k, l in matcher.get_opcodes():
        if tag in {'replace', 'delete'}:
            deleted += j - i
        if tag in {'replace', 'insert'}:
            added += l - k
    return {'added': added, 'deleted': deleted, 'churn': added + deleted}


class History:
    def __init__(self, root):
        self.root = root
        self.commits = []
        self.baseline = {}
        self.previous = {}

    def commit(self, message, group, scope=None, breaking=False, merge=False):
        current = tree(self.root)
        deltas = {name: line_delta(self.previous.get(name, b''), current.get(name, b''))
                  for name in sorted(self.previous.keys() | current.keys())
                  if self.previous.get(name) != current.get(name)}
        git(self.root, 'add', '-A')
        git(self.root, 'commit', '-qm', message)
        revision = git(self.root, 'rev-parse', 'HEAD')
        self.commits.append({'revision': revision, 'message': message, 'group': group,
                             'scope': scope, 'breaking': breaking, 'merge': merge, 'files': deltas})
        self.previous = current
        return revision

    def branch(self, name, start='main'):
        git(self.root, 'checkout', '-q', '-b', name, start)
        self.previous = tree(self.root)

    def history_summary(self, base, head):
        # The review/history branch is linear until an explicit target merge.
        # Only authored review commits count; exclude target-only and merge commits.
        selected = [c for c in self.commits if c['revision'] in self.activity]
        totals = {}
        groups = {}
        scopes = {}
        for commit in selected:
            groups[commit['group']] = groups.get(commit['group'], 0) + 1
            scopes[commit['scope'] or '(none)'] = scopes.get(commit['scope'] or '(none)', 0) + 1
            for name, delta in commit['files'].items():
                total = totals.setdefault(name, {'touches': 0, 'added': 0, 'deleted': 0, 'binary': False})
                total['touches'] += 1
                if delta is None:
                    total['binary'] = True
                else:
                    total['added'] += delta['added']
                    total['deleted'] += delta['deleted']
        for total in totals.values():
            total['churn'] = None if total['binary'] else total['added'] + total['deleted']
            if total['binary']:
                total['added'] = total['deleted'] = None
        final = tree(self.root)
        net = {name: line_delta(self.baseline.get(name, b''), final.get(name, b''))
               for name in sorted(self.baseline.keys() | final.keys())
               if self.baseline.get(name) != final.get(name)}
        return {'base': base, 'head': head, 'groups': groups, 'scopes': scopes,
                'breakingCommits': [c['revision'] for c in selected if c['breaking']],
                'activityRevisions': self.activity, 'files': totals, 'net': net,
                'policy': 'target..review; exclude merges; no rename following; paths counted separately'}


def cancellation(root):
    with (root / 'orders/order.go').open('a') as source:
        source.write('''
// Cancel releases stock and refunds only pending orders; repeat cancellation is safe.
func (s *Service) Cancel(id string) error {
 order, ok := s.orders[id]
 if !ok { return errors.New("missing order") }
 if order.Status == Cancelled { return nil }
 if order.Status != Pending { return errors.New("cannot cancel fulfilled order") }
 s.stock.Release(id)
 s.payment.Refund(id)
 order.Status = Cancelled
 s.orders[id] = order
 return nil
}
''')
    write(root, 'orders/cancel_test.go', '''package orders
import (
 "testing"
 "example.test/fulfillment/inventory"
 "example.test/fulfillment/payments"
)
func TestCancel(t *testing.T) {
 stock, payment := inventory.New(), payments.New()
 service := New(stock, payment)
 if _, err := service.Place("cancel-1", "book", 2, 1200); err != nil { t.Fatal(err) }
 for range 2 { if err := service.Cancel("cancel-1"); err != nil { t.Fatal(err) } }
 order, _ := service.Get("cancel-1")
 if order.Status != Cancelled || stock.Available("book") != 4 || payment.Charges != 1 || payment.Refunds != 1 { t.Fatal("cancellation leaked or duplicated effects") }
 if _, err := service.Place("cancel-1", "book", 2, 1200); err != nil { t.Fatal(err) }
 if payment.Charges != 1 { t.Fatal("cancelled retry charged again") }
 if err := service.Fulfill("cancel-1"); err == nil { t.Fatal("fulfilled cancelled order") }
 if err := service.Cancel("missing"); err == nil { t.Fatal("cancelled missing order") }
 if _, err := service.Place("fulfilled-1", "book", 1, 600); err != nil { t.Fatal(err) }
 if err := service.Fulfill("fulfilled-1"); err != nil { t.Fatal(err) }
 if err := service.Cancel("fulfilled-1"); err == nil { t.Fatal("cancelled fulfilled order") }
}
''')
    edit(root, 'api/http.go', '\t\torder, err := service.Place', '''        if r.URL.Path == "/orders/cancel" {
            if err := service.Cancel(request.ID); err != nil { http.Error(w, err.Error(), http.StatusConflict); return }
            _ = json.NewEncoder(w).Encode(Response{ID: request.ID, Status: orders.Cancelled})
            return
        }
\t\torder, err := service.Place''')
    write(root, 'api/cancel_test.go', '''package api
import (
 "encoding/json"
 "net/http/httptest"
 "strings"
 "testing"
 "example.test/fulfillment/inventory"
 "example.test/fulfillment/orders"
 "example.test/fulfillment/payments"
)
func TestCancelHTTP(t *testing.T) {
 stock, payment := inventory.New(), payments.New()
 service := orders.New(stock, payment)
 if _, err := service.Place("cancel-http", "book", 2, 1200); err != nil { t.Fatal(err) }
 handler := Handler(service)
 for range 2 {
  response := httptest.NewRecorder()
  handler.ServeHTTP(response, httptest.NewRequest("POST", "/orders/cancel", strings.NewReader(`{"id":"cancel-http"}`)))
  var result Response
  if response.Code != 200 { t.Fatalf("cancel HTTP: %d %s", response.Code, response.Body.String()) }
  if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Status != orders.Cancelled { t.Fatalf("cancel response: %v %v", result, err) }
 }
 if stock.Available("book") != 4 || payment.Refunds != 1 { t.Fatal("HTTP retries duplicated cancellation") }
}
''')
    edit(root, 'storefront/client/index.ts', 'get(id: string): Promise<Order> }',
         'get(id: string): Promise<Order>; cancel(id: string): Promise<Order> }')
    edit(root, 'storefront/client/index.ts', ' status(id: string)',
         ' cancel(id: string): Promise<Order> { return this.transport.cancel(id); }\n status(id: string)')
    edit(root, 'storefront/tests/checkout.test.ts', 'get: async id =>',
         'cancel: async id => ({id, status: "cancelled"}), get: async id =>')
    edit(root, 'storefront/tests/checkout.test.ts', ' let rejected = false;',
         ' if ((await client.cancel("o-1")).status !== "cancelled") throw new Error("cancel status");\n let rejected = false;')
    with (root / CHECKOUT).open('a') as source:
        source.write('\nexport async function cancel(client: Client, id: string): Promise<Order> { return client.cancel(id); }\n')
    edit(root, 'python/fulfillment/worker.py', '        self.shipments = {}',
         '        self.shipments = {}\n        self.cancelled = set()')
    edit(root, 'python/fulfillment/worker.py', '        if message["type"] != "order.ready":',
         '''        if message["type"] == "order.cancelled":
            self.cancelled.add(message["order_id"])
            self.shipments.pop(message["order_id"], None)
            return None
        if message["order_id"] in self.cancelled:
            raise ValueError("order cancelled")
        if message["type"] != "order.ready":''')
    with (root / 'python/test_flow.py').open('a') as source:
        source.write('''
    def test_cancel(self):
        worker = Worker()
        worker.process({"type": "order.ready", "order_id": "o-1"})
        for _ in range(2):
            self.assertIsNone(worker.process({"type": "order.cancelled", "order_id": "o-1"}))
        self.assertEqual(worker.count, 0)
        with self.assertRaises(ValueError):
            worker.process({"type": "order.ready", "order_id": "o-1"})
''')
    edit(root, 'seamgraph-contracts.yaml', 'links:\n', '''  - id: behavior:order.cancel
    seamId: seam:orders.lifecycle
    description: Repeated cancellation releases stock and refunds once
    kind: behavior
links:
  - testId: example.test/fulfillment/orders/TestCancel
    requirementIds: [behavior:order.cancel]
    memberIds: []
    reviewed: true
''')
    subprocess.run(['gofmt', '-w', 'orders', 'api'], cwd=root, check=True)


def large(root, count):
    modules = []
    grants = []
    for i in range(count):
        name = f'warehouse-{i:03}'
        path = f'storefront/warehouses/site-{i:03}/index.ts'
        write(root, path, f'''import {{ Client, Order }} from "../../client/index";
export interface Warehouse {{ readonly code: string; capacity: number }}
export const warehouse: Warehouse = {{code: "{name}", capacity: {i + 10}}};
export async function status(client: Client, id: string): Promise<Order> {{ return client.status(id); }}
''')
        modules.append(f'  {name}:\n    language: typescript\n    paths: [storefront/warehouses/site-{i:03}]\n    seams:\n      status:\n        entrypoints: [{path}]\n')
        grants.append(f'    - {{from: {name}, to: client.orders}}\n')
    config = (root / 'seamgraph.yaml').read_text().replace('policies:\n', ''.join(modules) + 'policies:\n')
    write(root, 'seamgraph.yaml', config + ''.join(grants))


def generate(destination, warehouses=0, working=False):
    root = Path(destination).absolute()
    if root.exists():
        raise FileExistsError(f'Refusing existing destination: {root}')
    if not 0 <= warehouses <= 200:
        raise ValueError('warehouses must be between 0 and 200')
    shutil.copytree(TEMPLATE, root, ignore=shutil.ignore_patterns('__pycache__', '*.pyc', '.seamgraph', '.git', 'node_modules', '.venv'))
    if warehouses:
        large(root, warehouses)
    # Empty template disables machine-installed Git templates and hooks.
    git(root, 'init', '-q', '--initial-branch=main', '--template=')
    history = History(root)
    base = history.commit('feat: seed local fulfillment application', 'feat')
    history.baseline = tree(root)
    history.branch('target')
    write(root, 'docs/target.md', 'Target-only deployment note.\n')
    target = history.commit('docs(operations): clarify local deployment', 'docs', 'operations')
    history.branch('review/history')
    history.activity = []
    def activity(message, group, scope=None, breaking=False):
        revision = history.commit(message, group, scope, breaking)
        history.activity.append(revision)
        return revision
    edit(root, CHECKOUT, 'Place order', 'Review order')
    activity('feat(checkout): review before placing', 'feat', 'checkout')
    edit(root, CHECKOUT, 'Review order', 'Confirm order')
    activity('fix(checkout): clarify confirmation', 'fix', 'checkout')
    git(root, 'revert', '--no-commit', 'HEAD')
    git(root, 'revert', '--no-commit', 'HEAD~1')
    activity('revert: restore checkout label\n\nReverts the two checkout label edits.', 'revert')
    write(root, 'unassigned/temporary.txt', 'temporary review note\n')
    activity('chore(fixtures): add temporary note', 'chore', 'fixtures')
    (root / 'unassigned/temporary.txt').unlink()
    activity('Remove temporary note', 'unmatched')
    git(root, 'mv', 'unassigned/status.txt', 'unassigned/overview.txt')
    activity('refactor(files): move overview exactly', 'refactor', 'files')
    git(root, 'mv', 'storefront/status/status.ts', 'storefront/status/order-status.ts')
    edit(root, 'storefront/status/order-status.ts', 'function status(', 'function orderStatus(')
    activity('refactor(status)!: rename order status helper', 'refactor', 'status', True)
    (root / 'unassigned/logo.bin').write_bytes(b'\x00SEAMGRAPH\x01\x02')
    activity('assets(brand): add local binary marker\n\nBREAKING CHANGE: consumers must accept binary artifacts.', 'assets', 'brand', True)
    review_tip = git(root, 'rev-parse', 'HEAD')
    # Exclude target-only activity and merge diffs from activity metrics.
    git(root, 'merge', '--no-ff', '--no-commit', 'target')
    merged = history.commit('Merge diverging target', 'unmatched', merge=True)
    summary = history.history_summary(base, merged)
    summary.update(target=target, beforeMerge=review_tip)
    history.branch('review/cancellation')
    cancellation(root)
    history.commit('feat(orders): cancel across local adapters', 'feat', 'orders')
    history.branch('review/payment-breaking')
    edit(root, 'payments/payment.go', 'Authorize(id string, amount int)', 'Authorize(id string, amount int64)')
    edit(root, 'payments/payment.go', 'old.Amount != amount', 'old.Amount != int(amount)')
    edit(root, 'payments/payment.go', 'Amount: amount}', 'Amount: int(amount)}')
    history.commit('feat(payments)!: widen authorization amount', 'feat', 'payments', True)
    history.branch('review/payment-fixed', 'review/payment-breaking')
    edit(root, 'orders/order.go', 's.payment.Authorize(id, amount)', 's.payment.Authorize(id, int64(amount))')
    history.commit('fix(orders): adapt authorization consumer', 'fix', 'orders')
    history.branch('review/boundary')
    with (root / 'orders/order.go').open('a') as source:
        source.write('\nfunc (s *Service) DebugStock(sku string) int { return inventory.DebugAvailable(s.stock, sku) }\n')
    history.commit('fix(inventory): inspect availability through a bypass', 'fix', 'inventory')
    history.branch('review/boundary-fixed', 'review/boundary')
    edit(root, 'orders/order.go', 'inventory.DebugAvailable(s.stock, sku)', 's.stock.Available(sku)')
    history.commit('fix(inventory): use reservation seam', 'fix', 'inventory')
    history.branch('review/stale')
    # A real behavioral change invalidates old evidence, while existing assertions pass.
    edit(root, 'orders/order.go', 'if old, ok := s.orders[id]; ok {',
         'if amount > 100000 { return Order{}, errors.New("order limit exceeded") }\n\tif old, ok := s.orders[id]; ok {')
    write(root, 'docs/checkout.md', '# Checkout\n\nAny positive amount is accepted. See [retry rules](missing-retry.md).\n')
    write(root, 'docs/checkout-copy.md', '# Ordering\n\nAny positive amount is accepted.\n')
    history.commit('fix(orders): bound authorization size', 'fix', 'orders')
    history.branch('review/evidence-fixed', 'review/stale')
    write(root, 'docs/checkout.md', '# Checkout\n\nAmounts above 100000 cents are rejected. See [requirements](requirements.md).\n')
    (root / 'docs/checkout-copy.md').unlink()
    with (root / 'orders/order_test.go').open('a') as source:
        source.write('''
func TestOrderLimit(t *testing.T) {
 service := New(inventory.New(), payments.New())
 if _, err := service.Place("large", "book", 1, 100001); err == nil { t.Fatal("accepted over-limit order") }
 if _, err := service.Place("limit", "book", 1, 100000); err != nil { t.Fatal(err) }
}
''')
    edit(root, 'seamgraph-contracts.yaml', 'links:\n', '''  - id: behavior:order.limit
    seamId: seam:orders.lifecycle
    description: Amounts above 100000 cents are rejected and the limit is accepted
    kind: behavior
links:
  - testId: example.test/fulfillment/orders/TestOrderLimit
    requirementIds: [behavior:order.limit]
    memberIds: []
    reviewed: true
''')
    history.commit('test(orders): verify limit and refresh guidance', 'test', 'orders')
    history.branch('review/evidence-failed')
    edit(root, 'orders/order.go', 's.stock.Release(id)', '// Deliberate defect: reservation is not released')
    history.commit('fix(orders): simulate leaked reservation', 'fix', 'orders')
    history.branch('review/evidence-missing')
    edit(root, 'seamgraph-contracts.yaml', 'links:\n', '''  - id: behavior:order.unmapped
    seamId: seam:orders.lifecycle
    description: Deliberately unmapped cancellation obligation
    kind: behavior
links:
''')
    history.commit('test(orders): declare unmapped requirement', 'test', 'orders')
    refs = {name: git(root, 'rev-parse', name) for name in git(root, 'for-each-ref', '--format=%(refname:short)', 'refs/heads').splitlines()}
    git(root, 'checkout', '-q', 'main')
    if working:
        edit(root, CHECKOUT, 'Place order', 'Stage order')
        git(root, 'add', CHECKOUT)
        edit(root, CHECKOUT, 'Stage order', 'Unstaged order')
        write(root, 'unassigned/new-review.txt', 'untracked review note\n')
    metadata = {'version': 1, 'refs': refs, 'history': summary, 'commits': history.commits,
                'working': working, 'warehouses': warehouses,
                'size': {'modules': 18 + warehouses, 'warehouseFiles': warehouses}}
    write(root, '.seamgraph/scenarios.json', json.dumps(metadata, indent=2) + '\n')
    return metadata


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('destination', type=Path)
    parser.add_argument('--warehouses', type=int, default=0, help='bounded larger graph: 0..200 modules')
    parser.add_argument('--working', action='store_true', help='leave staged, unstaged and untracked edits on main')
    args = parser.parse_args()
    metadata = generate(args.destination, args.warehouses, args.working)
    print(f'Created {args.destination}: {len(metadata["refs"])} refs, {metadata["size"]["modules"]} modules')


if __name__ == '__main__':
    main()
