# Optional GitHub issue interoperability

**Mission branch**: `feat/optional-github-interop`  
**Created**: 2026-10-24 (America/Chicago)  
**Status**: Ready for implementation  
**Intent confirmed in chat**: Maintainers and Andrew can use a parallel gh-style issue interface within Hubnot and explicitly import/export GitHub tickets. Hubnot remains standalone. The user selected GitHub integration and CLI compatibility first, excluding protected-check rules and automerge, and requested a parallel implementation within Hubnot rather than an unmodified gh client and API service.

## User Scenarios & Testing

### Story 1: Familiar local issue commands (P1)
A contributor creates, lists, views, edits, comments on, closes and reopens native Hubnot issues using familiar gh-style commands. The same issues remain available through the existing native interface. No GitHub account, gh executable or network is involved.

Acceptance: create through the compatibility interface, view through the native interface, edit through either, and observe matching state. JSON field selection and filtering support scripts. Unsupported operations/flags fail explicitly. Conflicted issues are visible and mutations refuse to invent a winner.

### Story 2: Explicit GitHub import (P1)
A maintainer explicitly imports one issue or a repository backlog, including descriptions, labels, assignee source information, lifecycle and comments. Original authors, times, URLs and identifiers remain source provenance; the importing native actor signs the new facts.

Acceptance: import a paginated backlog, repeat without duplicates, update unchanged local copies, preserve local-only edits when remote data is unchanged, and refuse competing edits. Imported tickets remain usable after credentials/integration removal. PRs returned by issue endpoints are excluded.

### Story 3: Explicit GitHub export (P1)
A maintainer explicitly exports a selected native issue and its comments to a chosen repository, then safely repeats or resumes the operation. Creation/update uses the authenticated GitHub identity; source attribution remains visible.

Acceptance: selected issue creates one external ticket, state and comments survive, a repeated export makes no duplicate, and ambiguous network outcomes require reconciliation instead of blind POST retries. Conflicting edits refuse an overwrite.

### Edge Cases
Missing credentials; permission/rate-limit failures; token-shaped error responses; cross-origin redirects; overlarge payloads and pages; Unicode; duplicate mappings/markers; native revision conflicts; interrupted publication; simultaneous native edits; GitHub fields with no native equivalent; comment edits/deletions; a fresh clone without optional local journal state.

## Requirements

### Functional Requirements
| ID | Title | Requirement | Priority | Status |
|---|---|---|---|---|
| FR-001 | Parallel local CLI | Provide native gh-style issue create/list/view/edit/comment/close/reopen commands operating on the same signed issues as hn issue, without invoking gh or GitHub. | High | Open |
| FR-002 | Script compatibility | Support documented familiar flags, filtering, explicit JSON field selection and nonzero failures; reject unsupported semantics explicitly. | High | Open |
| FR-003 | Optional imports | Explicitly import one selected GitHub issue or all bounded repository issue pages, excluding pull requests. | High | Open |
| FR-004 | Portable provenance | Preserve original issue/comment IDs, URLs, author and timestamps as source claims. Imports use the native actor and preserve native-only information. | High | Open |
| FR-005 | Safe repeat/update | Discover stable source mappings, reject duplicates, preserve local edits and refuse competing remote/local changes; use guarded native mutations and retry-safe operations. | High | Open |
| FR-006 | Explicit exports | Export one explicitly selected native issue with supported fields, lifecycle and comments to the chosen GitHub repository; document unmappable fields. | High | Open |
| FR-007 | Publication recovery | Repeat/resume exports without blind creation retries; use durable checkpoints and complete bounded marker reconciliation, refusing ambiguous matches and outcomes. | High | Open |
| FR-008 | Explicit access | Contact GitHub only from explicit transport commands; credentials are optional for public reads, required for writes, and never stored in signed issue data or diagnostics. | High | Open |
| FR-009 | Independent core | Build/test/use native and compatibility issue commands with GitHub transport completely excluded. Removing transport must not affect imported issues. | High | Open |
| FR-010 | Honest boundary/docs | Publish exact command examples, supported compatibility subset, conflict/recovery behavior, bounds, attribution and offline-build instructions. | High | Open |

| FR-011 | Provider-neutral transfer | Keep transfer orchestration, source identities, checkpoints and issue/comment data independent of provider-specific DTOs. Expose explicit provider/project selection; GitHub is the first optional adapter. | High | Open |
| FR-012 | Honest migration capabilities | Represent issues, projects and workflows as distinct migration resources and report implemented, mapping-required or unsupported resources before any write. Document extension contracts and the first delivery boundary; never claim arbitrary workflow equivalence. | High | Open |

### Non-Functional Requirements
| ID | Title | Requirement | Category | Priority | Status |
|---|---|---|---|---|---|
| NFR-001 | Bounded hostile input | Bound HTTP response size, page count and request duration; enforce existing native payload limits before publishing imported content. Fail rather than truncate or partially accept a malformed issue. | Security | High | Open |
| NFR-002 | Quality | Format all changed Go sources; pass core and integration race tests, vet, builds with and without transport, and diff checks. | Correctness | High | Open |
| NFR-003 | Offline acceptance | Automated local compatibility and core-only acceptance perform zero GitHub requests and need no gh executable or GitHub credentials. Transport acceptance uses a local fake HTTP provider and real native issue storage. | Independence | High | Open |

### Constraints
| ID | Title | Constraint | Category | Priority | Status |
|---|---|---|---|---|---|
| C-001 | Standalone | No mandatory GitHub/gh dependency, API service, database, external Go module, workflow or Docker requirement. Transport is optional CLI functionality within Hubnot. | Product | High | Open |
| C-002 | Existing authority | Do not alter signed wire schemas, identity/policy authority or existing issue behavior; descriptive external metadata grants no authority. | Protocol | High | Open |
| C-003 | Bounded scope | No continuous sync, complete GitHub API emulation, PR/run compatibility, new protected-check policy or automerge. CLI compatibility is a documented issue-command subset. GitHub issues are the first working adapter; provider-neutral project/workflow capability reports distinguish future mappings from implemented migration. | Scope | High | Open |

## Key Entities
Native issue and signed revision; imported external issue and comment; source provenance; field baseline; export checkpoint; external publication marker. External attribution never substitutes for a native cryptographic signer.

## Success Criteria
1. The full local issue lifecycle succeeds offline through both interfaces against identical native issues.
2. Repeated paginated import/export fixtures create exactly one mapped issue and one copy of each unchanged comment.
3. Competing local/remote changes and duplicate mappings produce explicit errors without overwrites.
4. Transport-excluded builds retain full native operation and imported ticket content.
5. All scoped and repository quality gates pass, including an independent work-package review.

## Assumptions
Compatibility preserves useful command/flag/JSON conventions, while native identifiers and conflict rules stay explicit. External read snapshots cannot supply distributed locks. Exports require a single local publisher and do not promise globally exactly-once concurrent publication. Unsupported GitHub-only fields are documented rather than silently claimed as preserved.

## Additional user steering: provider portability

The user requested alternate providers and eventual import/export/migration of projects and workflows at will. The immediate design therefore uses a provider-neutral transfer system, with GitHub as its first adapter. No specific alternate provider or workflow format was named. This delivery establishes explicit resource/capability reports and provider extension contracts, and implements issue transfer; concrete project/workflow conversion requires source/destination mappings and must report manual or unsupported semantics before writes. A scope question is pending; independent provider-neutral issue work continues meanwhile.
