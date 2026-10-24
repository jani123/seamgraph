# Order fulfillment review workspace

This bounded application connects a TypeScript cart and checkout, a Go order
service with inventory and payment fakes, and a Python shipping worker and
reporting consumer. Deterministic stock starts with four books and ten pens;
amounts are integer cents. It needs no accounts, broker, database or remote data.

From the SeamGraph repository root, after the ordinary development setup
(`task deps && task build`):

```sh
python3 scripts/examples/generate.py .build/fulfillment
.build/seamgraph serve --root .build/fulfillment
```

Open http://127.0.0.1:8765. Generation creates a **new** local Git repository and
refuses an existing destination; choose a new path to regenerate. It neither
changes this repository's branches nor commits here. Go, Git, Node and Python
must already be available; generation invokes only Git and gofmt, and application
code has no third-party dependencies. The TypeScript test compiler comes from
SeamGraph's installed development dependencies. No runtime download is needed.

In the devcontainer, fulfillment, working-change and larger-graph repositories
also start automatically on container ports 8766, 8767 and 8768. Run
`task devcontainer:urls` from the host for their separately published localhost
URLs; VS Code labels and forwards the ports. See
[example endpoints](../../docs/development.md#example-endpoints) for setup and lifecycle.

Run the application and its assertions:

```sh
cd .build/fulfillment
go run ./cmd/demo | python3 -B python/demo.py
# {"fulfilled": 1, "order_ids": ["o-1"]}
go test -race ./...
(cd python && python3 -B -m unittest -v test_flow)
cd ../.. # return to the SeamGraph repository root
```

```sh
node tools/ts-analyzer/node_modules/typescript/bin/tsc \
  -p .build/fulfillment/storefront/tsconfig.json --noEmit false \
  --module commonjs --moduleResolution node \
  --outDir .build/fulfillment/.seamgraph/ts
node .build/fulfillment/.seamgraph/ts/tests/checkout.test.js
```

The Go demo emits a decoded-message contract consumed by the Python stdin demo.
It represents an in-process fulfillment request, not a reliable delivery system.
The Go HTTP handler is exercised with httptest; the TypeScript transport is an
injected local fake. The components share documented wire shapes without a
running HTTP server or broker. See the application's [wire contracts](app/docs/contracts.md),
[requirements](app/docs/requirements.md) and [ADR](app/docs/adr/0001-local-fakes.md)
for the intended behavior and omitted production concerns.

`task examples:test` exercises real Go/TypeScript/Python behavior, CLI scanning,
inspection, comparison, snapshot exports, contract collection and history facts.
The browser suite also covers the Review workspace from directory navigation and
hotspot investigation through persisted findings and evidence-preserving exports.
It creates fresh temporary repositories and leaves the source template unchanged.
`task check` includes this suite and the generated example's real browser test.
See the [scenario guide](SCENARIOS.md) for branch-by-branch review questions.

The 18 architecture modules include a six-stage quote dependency path and a
pricing → discounts → tax → pricing type-reference cycle. Calls branch through
stock and payments, and both HTTP and demo consumers use orders. Similarly named
status files and objects and the `unassigned/` directory exercise ambiguity and
files outside module roots. The quote stages are deliberately tiny navigation
fixtures, not realistic tax or pricing engines.

Go examples include named types, interfaces, functions, methods, fields, constants
and variables. TypeScript includes classes, interfaces, aliases, functions,
properties/accessors, enum and constant declarations. Python includes classes,
methods, properties, functions, annotated variables, dataclasses and static
re-exports. Adapter kind labels are coarser than these language constructs.
Private code stays visible in raw scans but may be absent from architecture diffs.
Python dataclass fields and instance attributes are not independently guaranteed
symbols. The computed array access in checkout intentionally retains
`TS_DYNAMIC_REFERENCE`; policy uses `unresolved: warn`. It does not establish
complete reference resolution. See [adapter guarantees](../../docs/adapters.md).

For larger navigation/rendering checks:

```sh
python3 scripts/examples/generate.py .build/fulfillment-large --warehouses 120
.build/seamgraph serve --root .build/fulfillment-large
```

This adds 120 nested warehouse consumer files, giving 138 architecture modules
and more than 600 symbols. The default automated scan budget is 30 seconds and
3 MB of JSON; the browser budget is 10 seconds to load/render and search down to
one warehouse, with a separate 15-second budget per revision comparison.
Those are regression budgets on a prepared development machine,
not throughput guarantees across hardware. Generation accepts at most 200 extra
warehouses. Browser checks save `.build/order-review-large.png` for manual review.

At fit-to-screen zoom, the shared-consumer warehouse graph is too dense to read
as an overview. Use All references plus search or focus to inspect a subset.
The rendering budget measures responsiveness, not diagram legibility; this is a
manual usability finding for the later explorer/navigation work.
