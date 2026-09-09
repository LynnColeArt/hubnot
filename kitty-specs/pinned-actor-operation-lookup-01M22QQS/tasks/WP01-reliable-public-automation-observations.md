---
work_package_id: WP01
title: Reliable Public Automation Observations
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
planning_base_branch: feat/pinned-actor-operation-lookup
merge_target_branch: feat/pinned-actor-operation-lookup
branch_strategy: Planning artifacts for this mission were generated on feat/pinned-actor-operation-lookup. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/pinned-actor-operation-lookup unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T003
- T004
history: []
agent_profile: implementer-ivan
authoritative_surface: issue_commands.go
create_intent:
- automation_public_test.go
execution_mode: code_change
lane: planned
owned_files:
- identity_continuity.go
- issue_commands.go
- main.go
- automation_public_test.go
- docs/issues-v1.md
- docs/identity-v0.md
- docs/threat-model.md
role: implementer
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Load implementer-ivan using the canonical spk-doctrine-profile-load skill and
`spec-kitty agent profile show implementer-ivan --json`. Apply its identity,
boundaries and action governance before implementation. Architect/Planner
profiles must not implement; switch explicitly to Ivan. User authorized the
whole prerequisite mission without discovery interruptions. Independent review
is still required: do not approve your own work.

## Objective

Make public automation writes attributable to the expected signer and make
original results recoverable without an adapter database. Add complete catalog
pages to avoid one verified-history subprocess per issue. Preserve hn/0 signed
history, default human/JSON commands, resource budgets and conflict semantics.

## Context and prerequisites

Read spec.md, plan.md, contracts/public-cli.md and the existing command seams.
The rich-issue baseline is source 5a0e9cda. Its public schema is hn.issue/1;
identity discovery adds independent hn.identity/1. Do not merge or alter the
existing PR2 branch. Work only in the runtime-generated lane workspace.

The existing reader verifies histories and has typed read errors. Reuse it;
never parse private Git objects to bypass admission, and never use collectEvents
instead of the bounded issue reader. Existing issueMachineRequested handles
boolean spellings, flag arity, delimiter and free-text distinctions.

## T001 — Public identity and pinned mutation

Write subprocess tests first for `identity show --json` and pinned issue writes.
Assert exact schema/ok/data shape and the absence of private_key, privateKey,
identity path and other private fields. Identity output contains actor, name,
public_key only. Preserve human identity show output when --json is absent or
false. Reject unknown flags/positional arguments with one machine error when
machine mode is selected. Reuse existing boolean flag grammar behavior.

Add optional --actor to open, revise, resolve, close, reopen and comment.
When present it must be the full canonical 64 lowercase hexadecimal actor
fingerprint; empty and malformed values are invalid_input. Omitted pin uses
active identity exactly as before. Lookup's actor is separately required.

Compare pin to the identity object actually passed to nextEvent and appendEvent.
Do not satisfy the pin with an earlier discovery call and then reload identity.
Check before append and before returning a replay for another active signer.
A mismatch returns stable actor_mismatch and no ref mutation. The captured
identity remains the signer even if the active identity file changes afterward.

Tests cover every mutation name, success with matching actor, active identity
change before subprocess launch, and the captured-identity seam during apply.
For failures compare all refs/hn names and OIDs before/after. Test malformed pin
validation before repository access. No private identity contents in errors.

## T002 — Verified original operation lookup

Implement `issue operation --actor ACTOR --operation KEY [--json]` as read-only.
Both flags are required; no issue ID, pagination, input or free text. Reject
unsupported flags. Key keeps existing 1..128 printable non-whitespace ASCII
rules. Actor fingerprint is exact and explicit; never infer active actor.

Use one collectIssueEvents call and filter exact signed Actor and Operation.
No match: not_found. More than one distinct event: operation_conflict.
Do not choose latest timestamp or arbitrary ordering. Other actors' equal keys
are independent; invalid/admission/resource failures cannot become absence.

Unique output is hn.issue/1 envelope with snapshot and complete original data:
issue_id, event_id, actor, operation, intent, kind, timestamp, parents, request,
state for opening/revision or body for comment. Include body for empty comments.
Use the original signed state and timestamp, never the issue's current state.
Preserve full IDs, parents and full strings. Do not trim or summarize content.
Return recorded request digest only after existing verified reader validation.
No mutation or new storage, and no local mutable cache required.

Write real subprocess cases: create then revise/comment, recover original
operation unchanged; comment recovery retains exact body/time; same key from
another signer absent or independently found; changed content reuse rejects;
fixture with two valid signed records for one actor/key yields conflict. Include
invalid-history and bounded-reader failure behavior using established fixtures.
Human lookup should display useful identifiers/provenance; JSON remains the
consumer contract. A lookup result is an observed historical fact, not proof of
current global consensus or execution permission.

## T003 — Complete catalog pages

Add optional bool --details only to issue list. Default summaries remain
unchanged. Detail rows contain id, creator, state, conflict, head_count.
State is complete canonical IssueState or explicit null under conflict.
Do not include nested head lists: callers use the existing heads command.
Keep items,total,conflict,cycle_count envelope data and existing page50/max200.

Build catalog once per invocation. Do not spawn show for each issue. Bind the
details flag into cursor fingerprint; summary→details and reverse cursors must
fail invalid_input. Existing stale snapshot behavior remains unchanged.

Test at least three issues with metadata and small pages, follow all pages,
assert no duplicates/omissions and full state equality. Include a conflicted
issue and assert null state and correct head_count. Assert no hidden truncation
for large body/metadata and no default-summary shape change. Test explicit
false details and wrong-query cursors. Human details may render full state
using existing human helpers, while preserving human default list output.

## T004 — Public documentation and qualification

Update main/issue help and docs/issues-v1.md, docs/identity-v0.md,
docs/threat-model.md. Explain independent schemas, public-only fields,
optional-by-default pin, explicit lookup actor, actor_mismatch repair binding,
original operation/time semantics, actor-scoped keys and detailed page bounds.
Document original-state replay and unchanged concurrency limitations.

Do not claim global issue CAS, atomic assignment, safe compensation or Go Kitty
execution authority. No Go Kitty files belong to this package. Do not modify
protocol hn/0 or persist an operation index. New flags are general-purpose.

Once focused tests pass run go test -race -count=1 -timeout=10m ./..., go vet
./..., go build -o /tmp/hn-prerequisite-candidate ./..., gofmt and git diff
--check. Build outside the repository so runtime cannot autocommit a binary.
Save command logs outside the worktree and send parent active log paths.

## Branch strategy and integration

Planning and merge target: feat/pinned-actor-operation-lookup. Runtime computes
one lane because parser changes are cohesive. Do not hand-edit lifecycle state.
Use targeted stage/commit of only the owned files. Preserve unrelated runtime
metadata and historical worktrees. Any lane hygiene rejection is reported to
root; do not force readiness, clean metadata or restore generated binaries.

## Definition of done

All public scenarios and full required gates pass at the exact source commit.
Docs and help match the actual schema. Changed paths are exclusively owned
files, with only automation_public_test.go newly created. Runtime marks
T001–T004 done and normal transition to for_review is attempted. Parent receives
commit hash, logs, contract checkpoint and canonical independent review claim.
No self-approval, push, PR edit, accept or merge before independent approval.

## Reviewer guidance

Challenge identity check/use races, private-field exposure, unknown/false flags,
wrong actor/key, duplicate operation winners, old-state versus latest-state
substitution, empty body omission, verification bypass, detail pagination mode
confusion, hidden truncation, and accidental capability inflation. Verify actual
production CLI calls the new helpers and errors publish zero refs. Review exact
source commit; inherited planning metadata is outside the implementation diff.
