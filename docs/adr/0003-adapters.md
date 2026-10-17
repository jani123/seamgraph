# ADR 0003: Adapter execution and evidence

Status: implementation, 2026-10-16.

Go uses go/packages with syntax, dependencies and type information in the current
Go build environment (GOOS/GOARCH/GOFLAGS, including build tags). No YAML command
execution. Tests are excluded from production architecture. Canonical paths use
project-relative package directories plus declaration/receiver names, not lines.
Interface satisfaction is recorded as implements, never data-flow proof.

TypeScript uses a bundled Node helper and pinned TypeScript Compiler API. It
resolves tsconfig, symbols, barrels and aliases. Python uses a bundled Python 3.10+
AST helper, configured import roots and explicit re-export resolution. Both helpers
have a versioned request/response protocol, emit normalized facts and diagnostics,
and never import or execute project application code. Helpers and runtime assets
are embedded in the Go binary; dependencies are installed at development build
 time, never downloaded at runtime.

Each adapter returns the same IR. A registry owns language dispatch. Cross-language
allow declarations do not create observed edges. Mixed projects merge adapters'
facts; unverifiable runtime/FFI/service calls remain explicitly uncertain.
