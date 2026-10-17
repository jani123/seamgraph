# ADR 0005: Literal project root and exact Git inputs

Status: implemented, 2026-10-16.

The confirmed literal/disjoint-root policy includes `paths: [.]` for projects with
source packages at their project root. It owns the entire project and therefore
cannot coexist with any separately declared module root. Evidence file paths
still cannot be `.`. Dependency/environment/generated directories are pruned by
adapters. This extends the initial P1 schema without introducing glob or nested
ownership precedence. All three adapters have a project-root integration test.

Git comparisons and revision inspection read exact `ls-tree`/`cat-file` blobs
into isolated temporary directories. Ordinary git archive obeys export-ignore and
export-subst attributes, which can change the revision's analysis inputs; direct
blob extraction avoids that. Symlinks/submodules are rejected explicitly. No
checkout, Git hook, working-tree modification or persistent snapshot DB is needed.
Server-session caches include commit, configuration digest and analysis context;
verification evidence is assessed independently against the requested revision.
