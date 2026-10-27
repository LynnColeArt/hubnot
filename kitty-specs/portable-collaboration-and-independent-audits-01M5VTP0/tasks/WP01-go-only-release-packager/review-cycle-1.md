---
affected_files: []
cycle_number: 1
mission_slug: portable-collaboration-and-independent-audits-01M5VTP0
reproduction_command:
reviewed_at: '2026-10-26T11:34:54Z'
reviewer_agent: user
wp_id: WP01
---

# WP01 review: changes requested

Reviewed source: `86dc00678c42d495c6442addfcc8da8abd983ffa`.

## Blocking finding P1: the cleanliness check executes repository clean filters

`internal/release/package.go:144` calls `git status`. `controlledEnvironment`
(lines 315–324) disables global/system Git configuration, but repository-local
configuration remains active, and `gitCommand` (lines 340–341) only disables
fsmonitor. For tracked files with a filter attribute, Git's status refresh can
invoke `filter.<driver>.clean` or a process filter through a shell. This makes
packaging run undeclared executables before its isolated source build.

This contradicts `contracts/release-v1.md:3–5` (only Git and Go, never a shell)
and the documented controlled build/executable boundary. This is observable with
a clean committed source, not an assertion about hypothetical shell injection:
the reviewer built the packager from the exact source above, committed a minimal
fixture with `README.md filter=review`, configured repository-local
`filter.review.clean = touch /tmp/.../unexpected-execution; cat` **after** the
commit, and changed only the README mtime. Packaging returned exit 0, generated
local release assets, and created the marker. No preceding command ran the filter.

Reproduce and inspect the exact commands/output:

```text
python3 kitty-specs/portable-collaboration-and-independent-audits-01M5VTP0/tasks/WP01-go-only-release-packager/review-evidence-86dc0067/reproduce-clean-filter.py
```

The Python script is a review-only harness; product packaging must remain Go/Git.
The durable `.log` beside it records source and packager SHA-256.

### Required remediation

Prevent cleanliness/source inspection from executing configured Git filters or
other undeclared executable helpers. A safe explicit refusal before any filter
invocation is acceptable if a configured input cannot be inspected safely;
otherwise perform an isolated, nonexecuting cleanliness check. Cover both clean
and process filter configuration rather than only the illustrated driver name.
Do not merely document implicit shell execution or weaken the contract.

Add a public package/CLI regression with a configured marker-producing filter:
the marker must remain absent, and any refusal must leave no output/staging or
source-state mutation. Retain real dirty-index/worktree detection and existing
normal-source reproducibility behavior. Run source-bound required checks for the
fixed commit and preserve the evidence/feedback.

## Passing evidence and remaining limits

Focused race tests, independent full-matrix archive inspections, manifest/build
bindings, byte-identical repeated assets, Linux AMD64 standard/native-only native
CLI smoke, and the source-bound full race suite passed. No second blocker found.
The completed framework pre-review gate is `no_coverage` because its Python
architectural authority is unavailable, with `blocked=false` and
`force_bypassed=false`; this is separate from Go validation.

Windows/macOS packager runtime and non-Linux-AMD64 artifact runtime remain
unqualified, as explicitly documented. Cross-compilation does not satisfy those
runtime claims. Public releases/tags were not mutated. WP12 depends on WP01;
its owner must refresh/rebase against the corrected approved WP01 before final
integration acceptance.
