# Implementation Plan: Pinned Actor and Operation Lookup

**Branch**: feat/pinned-actor-operation-lookup | **Date**: 2026-09-09 | **Spec**: spec.md

## Summary

Add public identity JSON, optional mutation signer pin, verified actor/key operation lookup, and optional detailed list pages. Use existing identity capture, verified-reader and CLI envelope seams. The parent confirmed all contract choices, including original operation/timestamp and explicit actor for lookup. User authorized continuing through tasks and implementation without intermediate approval questions; independent code review remains mandatory.

## Technical Context

**Language/Version**: Go 1.26, root package main.
**Primary Dependencies**: Standard library and existing Git subprocess runner; no dependency changes.
**Storage**: Existing signed hn/0 events and refs only; no indexes or databases.
**Testing**: Test-first public CLI subprocess fixtures, focused same-package captured-identity test, existing verification fixtures; race/vet/build/gofmt/diff gates.
**Target Platform**: Existing Linux/macOS CLI platforms.
**Project Type**: Single Go CLI.
**Performance Goals**: One existing verified reader pass per lookup or detailed page, never N per-issue subprocess scans.
**Constraints**: Existing 100000 events/ref entries, 8 MiB object, 256 MiB aggregate reader bounds; 256 KiB rich payload limit. One full unique lookup result, no truncation. Detailed page default50/max200 states bounded by verified aggregate reader bytes.
**Scale/Scope**: One cohesive WP across identity_continuity.go, issue_commands.go, public tests, help/docs.

## Charter Check

HN-001/002: no new durable authority or event schema; original signed semantics retained.
HN-003: validate full actor and printable bounded operation before reading; verified reader fails closed; public-only identity object.
HN-004: no implicit conflict resolution. HN-005: docs and public tests ship together.
All gates pass at design; no bulk rename or security-impacting dependency decision. Independent adversarial code review will challenge actor pin/replay/privacy before approval.

## Architecture and Data Flow

identity show parses only --json, uses existing active identity loader, emits public-only identity/1 or unchanged human output.
Issue flags add --actor to mutations and actor/key-only operation lookup; list adds --details. All reuse registered flag grammar and exactly-one JSON error selection. Reject unsupported flags and positional arguments.
The mutation loads identity once and passes that same immutable object to nextEvent and appendEvent. Compare expected actor to that actual object before any append/replay. Check the identity object again at applyIssueMutation's entry so the captured-identity seam cannot bypass the pin. Do not reload mutable global identity during append. A changed identity file after capture cannot substitute another signer.
Operation lookup calls collectIssueEvents, matches exact Actor+Operation, returns typed not_found or operation_conflict, otherwise returns original signed fields. Use issueRootID for opening versus revision/comment identities. Do not use current selected issue state. Existing verifyEvent checks request digest; include exact recorded Request. Read errors retain IssueReadError codes through errors.As.
Details list uses existing catalog once, with rows {id,creator,state,conflict,head_count}; state is complete canonical state or null. No head IDs nested in rows: existing heads command provides them. Same paging counters/ordering, details bound in cursor fingerprint. Default summaries byte-compatible in shape.

## Contract

See contracts/public-cli.md. Actor is the full current public-key fingerprint in repository's canonical format (reuse validActorFingerprint). Lookup requires explicit actor and operation; mutations omit pin by default. Public identity schema is hn.identity/1; issue schema stays hn.issue/1. No distributed CAS claim.

## Project Structure

Owned implementation paths: identity_continuity.go, issue_commands.go, main.go, automation_public_test.go (new), docs/issues-v1.md, docs/identity-v0.md, docs/threat-model.md.
New tests are one cohesive public consumer suite. If existing identity doc path differs, resolve before finalization and use the actual existing path.

## Implementation Concern Map

### IC-01 — Public signer and pinned writes
- Requirements: FR-001, FR-002, FR-004, FR-005.
- Surfaces: identity_continuity.go, issue_commands.go, main.go, public tests/docs.
- Risk: private key leakage, error-mode drift, check/use identity race.

### IC-02 — Original operations and complete catalog pages
- Requirements: FR-003, FR-004, FR-005, FR-006.
- Surfaces: issue_commands.go, public tests/docs.
- Risk: current-state substitution, duplicate winner, unverified absence, cursor mode confusion or hidden truncation.

## Verification and delivery

Write public regressions before code. Cover identity public-only output, mismatch zero writes for all mutation commands, active identity changed before subprocess launch, captured identity preserved through append, original lookup after edits/comments, original comment timestamp/body, explicit actor isolation, divergent keys and duplicate signed records, strict flags, invalid/resource-limited history, details metadata and conflict/null plus cross-mode stale cursors. Run go test -race -count=1 -timeout=10m ./..., go vet ./..., external go build, gofmt and git diff --check. Targeted source commit only, runtime mark and normal readiness, then root dispatches independent reviewer. No push/PR changes or self-approval.
