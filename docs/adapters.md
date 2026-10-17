# Adapter guarantees and limits

All adapters return the same validated version-1 IR. Configured source roots must
exist, be disjoint and contain no source symlinks; external-root reads are rejected.
Unowned external libraries are not architectural modules. Analysis never executes
application code. Only the explicit `test` command executes supported test runners.

## Go

Uses `golang.org/x/tools/go/packages`, `go/types` and AST positions. The active Go
build context is honored: GOOS, GOARCH, GOFLAGS (including `-tags`) and go.work.
Go dependencies must already be installed. Runtime module/toolchain downloads are
disabled and module files are loaded in readonly mode (or vendor mode when
a vendor manifest exists). Build failures, cycles, missing dependencies and invalid internal-package
imports fail analysis rather than yielding a clean graph. Production test sources
are excluded from architecture; tests are collected separately.

Package-scope declarations, named methods and struct fields have stable IDs based
on their project-relative package directory and qualified declaration name. Select
`Reader`, `Reader.Read`, or a root-relative qualified selector such as
`internal/world.Reader` when multiple packages contain the same name. Selecting a
public type includes its public named methods and fields. Explicit interfaces
include their method contracts. Compiler-resolved references support import aliases.
Structural satisfaction is a separate `implements` fact; it is not data-flow proof
and does not create a consumer edge on its own.

Reflective invocation and plugin loading produce explicit dynamic diagnostics.
No whole-program runtime-call or reflective data-flow analysis is claimed. A
constructor returning an otherwise undeclared type does not automatically grant
access to all that type's members. Select the intended public types/members as
part of a seam. Multi-module Go projects require a valid Go workspace.

## TypeScript

The bundled TypeScript 5.9.3 Compiler API resolves tsconfig paths, aliased symbols,
type-only imports, barrel exports and re-exports. Entrypoint seams contain their
resolved exports and selected public class/interface members. Readonly/optional modifiers, accessor types and constructor accessibility are
retained in public contract signatures; their implementation bodies do not affect
architecture signatures. Deep imports retain explicit bypass evidence, including
when a consumer also imports a permitted public entrypoint.

Configuration is discovered above configured roots within the project. A module
may explicitly set `tsconfig: tsconfig.json` (relative to the project root).
Currently all configured TypeScript roots must use one shared configuration;
multiple independent tsconfig projects/references fail explicitly. JS/JSX-only
projects and framework-generated declarations are outside the TS-only adapter's
current guarantee. Standard declaration libraries are bundled; project-specific
types and npm dependencies must be installed beforehand.

Dynamic imports, require/eval and computed cross-module access that cannot be
resolved produce uncertainty diagnostics. Exact data flow through returned object
shapes and runtime mutation is not proven; public named types should be selected
explicitly when their members are part of the intended seam.

## Python

Uses Python's standard `ast`, never imports project modules, and resolves configured
import roots, absolute/relative imports, aliases, static re-exports, literal
`__all__`, namespace packages and `src/` layouts. Declare `importRoots: [src]`
when imports begin below src. Python entrypoints must be `.py` files. Public class
methods are included with selected classes; property accessor/decorator headers
are retained together so changing a getter cannot disappear behind its setter. Static project import paths determine
seam bypasses; this is an architecture policy, not Python-enforced encapsulation.

Unknown third-party imports are reported explicitly; installed third-party packages
are not imported to guess runtime behavior. Dynamic imports (including importlib
aliases), eval/exec, getattr/setattr, imported attribute mutation and `__getattr__`
exports produce dynamic/unresolved diagnostics. Unresolvable exports cannot cleanly
pass strict checks. Custom import hooks, runtime monkey patching, nonliteral
`__all__`, generated modules and arbitrary runtime attribute dispatch are not proven.

Test files (`test_*.py`, `*_test.py`) are excluded from production architecture.
The current resolver models module-level imports and exports; imports nested in
functions/conditions emit explicit uncertainty diagnostics. Namespace rebinding
and dynamic exports also produce diagnostics; full flow-sensitive resolution
remains an area for further hardening. Python has no compiler-enforced public seam guarantee.

## Mixed-language projects

`testdata/projects/mixed` is a working project with Go, TypeScript and Python
modules. Each language's resolved facts are merged into the common IR. Policy and
Git/contract comparison then run once over the merged graph. A configured
Python-to-Go grant expresses integration intent; it does not invent a static
runtime/FFI/RPC relation. Those relationships need an adapter that can supply
verifiable evidence for the particular integration mechanism. General RPC/FFI or
service-protocol discovery is not implemented.
