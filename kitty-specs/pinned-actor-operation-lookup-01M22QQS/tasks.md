# Tasks: Pinned Actor and Operation Lookup

## WP01 — Reliable Public Automation Observations

Priority P1. One cohesive command-boundary package; no dependencies. Independent public tests exercise signer discovery/pinning, original operation recovery, and bounded detailed catalog reading.

T001 Add public identity JSON and optional pinned issue signer.
T002 Add verified exact actor/key operation lookup.
T003 Add complete detailed catalog pages and immutable opening metadata with mode-bound cursors.
T004 Qualify public errors, races and compatibility; publish docs and evidence.

Prompt: [WP01-reliable-public-automation-observations.md](tasks/WP01-reliable-public-automation-observations.md)

Requirements: FR-001, FR-002, FR-003, FR-004, FR-005, FR-006.
Implementation: write public regressions first, extend existing CLI seams, preserve immutable authority and no-write failures, then run focused/full gates. No parallel source lanes because all features touch the shared issue parser. No new service, store or dependencies. Independent reviewer required before accept/merge.
