# Hubnot foundation constraint audit

Date: 2026-09-09. Reviewer: codex-independent-foundation-review, Reviewer Renata.

This is partial foundation evidence for C-001 through C-003, not approval of the completed mission or its unfinished CLI. No implementation or runtime files were changed by this audit.

## Reviewed boundary

- Baseline: `a4d4bbfb9ba7275275b8d1bb281cf988baadeb9f` (main before this mission).
- Integrated foundation snapshot: `4a798fcd` in `/home/lynn/projects/nichthub/.worktrees/rich-issues-reliable-automation-01M22EG0-lane-c`.
- WP01 approved source: `cb52d48ee80d57868f5311b66d32f8c8457fd2f1`; current owned Go files are byte-identical to that source.
- WP02 approved source: `adcc6e7ad5486db271ec04e270e0aa2534fb5ceb`; both reader files are byte-identical to that source.
- WP03 implementation is active. Its commands, machine output, replay behavior and user-facing authority language are outside this audit.

## Findings

No blocking constraint violation was found in the approved foundations.

| Constraint | Foundation assessment | Evidence and limits |
|---|---|---|
| C-001 — Repository authority | PASS within reviewed boundary | `go.mod` remains exactly `module hubnot` / `go 1.26`; no `go.sum` or dependency change. New imports are Go standard library only. Issue model and projection use in-memory values; reader invokes Git read operations and does not create a persistent cache, database, service or alternative source of truth. Existing signed append implementation is unchanged. |
| C-002 — Consumer independence | PASS within reviewed boundary | State contains descriptive status, criteria, assignees, labels, relations and string metadata. No Go Kitty types, imports, leases, runtime authorization, review acceptance or integration decisions occur in new production files. Status validation only checks open/closed and lifecycle consistency; assignees are bounded strings. Issue graph reports conflict provenance and cycles rather than an execution-ready answer. |
| C-003 — Compatibility and scope | PASS within reviewed boundary, with documented forward-compatibility limit | Existing Event fields retain order and JSON tags; five optional fields are appended with `omitempty`. Legacy issue shape bypasses new rich-state limits. Signature and event-ID functions still operate on original payload bytes. Existing namespaces and historical tracked evidence are unchanged. New event kinds may be rejected by older executables, as explicitly documented in the plan; rolling back an executable does not erase new signed events. |

## Concrete source observations

`event.go` adds the optional issue fields, a conditional raw rich-payload bound and issue-specific validation. It does not alter key derivation, Ed25519 signing, payload hashing, actor identity, protocol version or the existing Event field sequence.

`store.go` adds pure issue relationship validation before existing relationship validation. The actor-ref append/CAS implementation remains byte-identical to baseline. No global issue-CAS or distributed-lock guarantee has been added.

`quarantine.go` extends dependency enumeration and validation for issue roots, parents and relation targets, and propagates supplier failures before evaluating exact closure. It continues to use existing selection, quarantine and promotion machinery; no new transport protocol, implicit supplier admission or external service was introduced.

`issue_read.go` selects accepted actor refs from `refs/hn/actors` and parsed accepted remote actor refs. It pins OIDs, checks pending acceptance, verifies original signed bytes, actor chains, relationships and attachments, and compares the accepted snapshot before returning. Its new snapshot marker is `hn.issue.snapshot/1`; it does not introduce the old namespace. Git subprocesses are read operations (`for-each-ref`, `rev-list`, `cat-file`) and no shell interpolation is used in production.

`issue_model.go` and `issue_projection.go` contain no filesystem persistence or network calls. Their authority boundary is explicit: projection consumes already verified events; status and assignment do not authorize execution. Maximal revision heads derive from ancestry, and multiple heads have no selected current state. This does not replace the command-layer stale/snapshot guards still assigned to WP03.

## Tracked preservation evidence

Compared Git tree entries at the baseline and pinned integrated snapshot, including each existing path's blob OID:

- All **97** baseline `kitty-specs/` files are unchanged, preserving the prior missions' tracked evidence and history documents.
- All **4** baseline `.hn/` and `.nh/` configuration paths are unchanged.
- All **21** baseline identity, memory and policy Go source/test files are unchanged.
- Existing non-mission files changed only in `event.go`, `quarantine.go` and `store.go`.
- Added files comprise the seven foundation Go source/test files plus new mission/runtime operation artifacts; those operation artifacts are workflow records, not application authority or an application dependency.
- `git diff` between each approved source commit and the current corresponding owned Go paths is empty.
- `git diff` of `go.mod` and `go.sum` against baseline is empty.

This is a tracked-tree preservation check. It does not claim to have re-downloaded any remote release archive or independently inspected every local actor ref. None of those resources was mutated by this audit.

## Behavioral evidence carried forward

The immediately preceding independent cycle-2 reviews exercised the exact same approved foundation source:

- WP01: `go test -race -run 'Test(RichIssue|Issue)' -count=1 -timeout=60s -v .` passed in **1.625s**. Includes legacy byte fixture, oversized legacy opening, raw rich payload exact-cap/cap+1 cases, signed tampering, state validation, conflict/partial resolution and replication closure/recovery.
- WP01 original cycle-1 oversized signed-payload reproduction, executed through its preserved test overlay, now passes in **0.006s**.
- WP02: focused reader race suite passed in **2.701s**, covering selected refs, limits/framing, payload/signature/chain/relationship failure, pending acceptance, moving snapshots, attachment integrity, child failure/cancellation and SHA-256 Git.
- Both reviewed candidates passed `git diff --check`.
- Implementer full-gate evidence was reviewed previously; this audit does not relabel those runs as independent executions and did not repeat the full suite without a new concern.

The runtime recorded independent approval events `01M22H9808621AD3KE0JEXWEE6` (WP02) and `01M22HFRBNTENVJFA6YMFDCXA7` (WP01), as reported by the coordinating agent after the documented metadata-guard recovery.

## Remaining integrated checks

WP03 review must verify that every query uses the verified reader, that JSON and human output preserve conflict/authority semantics, that retries and local mutation guards match signed intent, and that paging stays bounded. WP04 must supply public consumer acceptance and current protocol/threat-model documentation. Final integrated race/vet/build/diff checks and mission acceptance remain outstanding; this audit does not substitute for them.
