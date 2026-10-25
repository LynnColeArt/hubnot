# Implementation Plan: Optional GitHub issue interoperability

**Branch / planning base / merge target**: `feat/optional-github-interop` (matches checkout)  
**Date**: 2026-10-24  
**Spec**: [spec.md](spec.md)

## Summary
Add a parallel local issue command parser at `hn gh issue ...`, and explicit `hn github import/export ...` transport. Use native issue query/mutation functions and signed metadata; no wire-schema change. GitHub HTTP code lives in a separate standard-library package and transport-specific root files. A `nogithub` build excludes that package from the executable; compatibility commands remain offline and available. No service, subprocess gh dependency or external Go modules.

## Technical Context
**Language/Version**: Go 1.26 (existing module `hubnot`).
**Primary Dependencies**: Existing Go standard library and Git; net/http only for explicit optional transport.
**Storage**: Existing signed issue events/metadata; private local adapter checkpoint below the Git directory, never protocol authority or credential storage.
**Testing**: Native issue CLI contract tests, httptest provider fixtures, public subprocess lifecycle and transport-disabled build; race/vet/build/diff gates.
**Target Platform**: Existing CLI platforms; local HTTP only for loopback fixture/development endpoints, HTTPS for external hosts.
**Constraints**: Existing 256 KiB event and field limits; finite response/page/time budgets; no automatic writes or background sync.

## Charter Check
HN-001: all durable imported collaboration remains native signed Git data, transport excluded on demand. HN-002/HN-004: import revisions consume exact native heads and preserve signed history. HN-003: bounded hostile HTTP, strict source validation, no redirected credentials. HN-005: update issue/threat docs and examples. No charter exception.

## Architecture and Interfaces
`hn gh issue` -> local parser -> existing issue queries/mutations -> signed Git storage.
`hn github import/export` -> transfer coordinator -> native issue operations + optional githubapi HTTP client.
`go build -tags nogithub` -> native commands + local compatibility + disabled-transport stub; no githubapi dependency in binary.

### IC-01 Local compatibility
Implement cmdGH(args []string) error in gh_compat.go, supporting issue create/list/view/edit/comment/close/reopen, common long and short flags and --json field selection. Native IDs remain authoritative; native conflicts never gain an arbitrary winner. Expose a documented supported subset, not an executable named gh that silently intercepts real GitHub commands. Use existing canonical state validation and actor-pinned mutation operations. Reads do not require identity. Core main routing is added in IC-03 after dependencies land.

### IC-02 Optional provider client
Package internal/githubapi with NewClient(baseURL, token string) (*Client,error). Client methods take context.Context and repo string (owner/name): GetIssue(ctx,repo,number), ListIssues(ctx,repo), ListComments(ctx,repo,number), CreateIssue(ctx,repo,input), UpdateIssue(ctx,repo,number,input), CreateComment(ctx,repo,number,body). Export Issue, Comment, User, Label, IssueInput DTOs matching REST JSON; IDs int64, NodeID string, Number int, Title/Body/State/HTMLURL/CreatedAt/UpdatedAt strings, User User, Labels []Label, Assignees []User, PullRequest json.RawMessage; Comment analogous ID/NodeID/Body/User/times/HTMLURL. IssueInput has title/body/state/labels/assignees. Client follows bounded same-origin pagination, skips PRs, rejects redirects, returns sanitized HTTP errors and never retries POST/PATCH automatically. Headers use documented GitHub media type and stable API version. Validate explicit repo and base URL, forbid URL userinfo/query/fragment, require HTTPS except explicit loopback HTTP, and never expose tokens in errors.

### IC-03 Import/export coordinator
Commands: `hn github import --repo OWNER/REPO [--issue NUMBER] [--api-url URL] [--dry-run]`; `hn github export ISSUE --repo OWNER/REPO [--api-url URL] [--dry-run]`. HN_GITHUB_TOKEN supplies optional read/required write credential. Public reads may be anonymous. Local compatibility never reads it. Transport implementation has !nogithub build constraint; stub has nogithub constraint.

Import collects complete bounded issue/comments data before mutating that issue, validates native content and attribution lengths, and refuses truncation. github/source opening metadata stores API origin, repo, immutable ID/node ID, number, URL, original author/time. Current github/baseline tracks digest of managed title/body/status/labels/assignees. Discover mappings from immutable opening metadata over all list detail pages; duplicate/invalid/conflicted claims refuse. Three-way update: remote same as baseline preserves local edits; local same as baseline permits remote revision; both changed refuses. Preserve criteria/relations/unrelated metadata. Source changes append signed revisions, comment edits append attributed corrections. Actor-pinned deterministic operation keys and persisted exact mutation intent recover uncertain local writes without refreshing expected heads.

Export requires one explicit issue, no conflicts, and preflights supported fields. Stable issue/comment body markers plus a Git-directory checkpoint enable repeat/resume. Reconcile complete bounded provider listings before retrying uncertain creation. Ambiguous matches refuse. A checkpoint stores native snapshot and external baseline, never tokens; external edits are checked before updates and competing changes refuse. No automatic POST retries or promise of distributed exactly-once writes; document one local exporter and explicit manual reconciliation if outcome cannot be established. Initial imports exported back to their source use their stable source mapping only after validating external identity. GitHub author/time cannot be impersonated by normal create APIs; include attribution. No silently dropping native-only criteria/relations/metadata: preserve locally and document export representation/limitations.

### IC-04 Acceptance and documentation
Add public CLI scenarios proving same native issue lifecycle through hn issue and hn gh issue, absent gh/credentials. Build nogithub and verify imported issues remain accessible, compatibility works, and github commands explain exclusion. Run transport end-to-end against a local fake provider with actual native signed storage. Document supported flags/fields, selection, provenance, conflict behavior, checkpoint/recovery limitations and optional build.

## Risks and Decisions
The user specifically requested parallel implementation within Hubnot; therefore no separate mandatory executable/module or HTTP service. Reuse native operations directly. API client is an optional package, not a policy authority. A remote body marker is descriptive and forgeable: multiple claims refuse and existing mappings require identity validation. GitHub writes cannot guarantee compare-and-swap; conservative baseline checks and single-exporter constraint are explicit. Errors and resource bounds fail closed. No live GitHub writes are necessary for verification.

## Verification
Each WP writes behavior tests first and passes scoped tests/build/vet/format/diff. Final run: `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, `go test -tags nogithub -count=1 ./...`, `go build -tags nogithub ./...`, and verify nogithub dependency graph excludes internal/githubapi. Independent review approves every WP before mission acceptance and consolidation onto feat/optional-github-interop. Deliver a reviewable PR against main; no automatic merge to public main.


## Recovery contract refinement (pre-implementation review)

Export checkpoint phases are prepared/in-flight/confirmed. A complete negative marker scan after an uncertain POST is not proof that the POST never executed: refuse another POST and require manual reconciliation. Positive marker recovery must validate immutable destination identity and exact intended transmitted projection, not marker alone. Export baseline digests cover the normalized transmitted representation including generated provenance/markers; they are distinct from import managed-state digests. The first export back to an imported issue's source may use the signed imported source projection as baseline only if it matches current remote content; missing or mismatched baseline refuses overwrite. Imports validate a complete issue/comment set before the first mutation, but multiple signed events are not atomic: interrupted imports can leave valid partial work and resume through the journal/provenance checks. Do not claim transactionality.


## Provider portability amendment

User steering expands the architecture beyond GitHub. `internal/forge` defines pure standard-library neutral data and a narrow Provider interface; no HTTP, GitHub DTO or SDK dependency. Issue ID is an opaque immutable provider value and Key is a separate opaque lookup selector, avoiding universal numeric-number assumptions. GitHub's optional adapter converts githubapi DTOs and validates its own owner/repo and numeric selectors. Generic source identity is {provider, endpoint, project, id, key}; source/baseline/observation metadata use forge/* and checkpoints/markers include provider identity. These are new unshipped fields, so no public migration/legacy alias is needed.

Canonical explicit commands become `hn transfer import/export --provider github --project OWNER/REPO ...`; `hn github import/export --repo ...` remains an optional convenience alias. Local `hn gh issue` remains purely native. GitHub factory registration is isolated in a !nogithub file; generic orchestration imports no HTTP/provider client and tagged builds refuse unavailable provider without credentials/network.

Neutral contract: Issue{ID,Key,Title,Body,State,Author,URL,CreatedAt,UpdatedAt strings; Labels,Assignees []string; IsChangeRequest bool}; Comment{ID,Body,Author,URL,CreatedAt,UpdatedAt strings}; IssueInput{Title,Body,State strings; Labels,Assignees []string}. Provider exposes Kind(), Endpoint(), and context-bearing issue/comment read/write methods with opaque project/key strings. GitHub adapter uses client.BaseURL() and node:/id: stable identifiers.

Migration resources distinguish issue, project and workflow. Add a pure capability/report model and an explicit capabilities/plan surface so unsupported project/workflow transfer fails before network/write. Reports classify supported/mapped, mapping-required/manual and unsupported semantics rather than pretending workflow runners/secrets/triggers/policy are interchangeable. The first implementation foundation supports issues; the confirmed expansion below adds native discussion/project/workflow adapters through the same isolated contracts. No guessed alternate provider is implemented.

Cross-direction recovery: imported source exported back to its verified exact source keeps ordinary body and suppresses validated source-origin comments, avoiding generated footer echo. A native export marker seen on re-import without proven source mapping refuses remapping rather than creating duplicate native issues or trusting a forgeable marker.


## Confirmed native resource expansion

User now explicitly requires native projects, discussions and workflows plus optional import/export. Extend the existing mission, keeping reviewed issue/HTTP foundations. Add one shared core resource engine and separate resource command and adapter WPs. New events reuse existing signed actor refs and existing Event fields, with registered type-specific open/revise kinds, strict versioned JSON states in Body, Parents/Intent/Operation/Request allowed only for those registered kinds. No IssueState flattening or opaque migration-only records. Preserve old signed bytes and authority. Old alpha binaries will reject unknown new kinds; document this upgrade boundary.

Core admission seams: event.go content/raw payload validation; issue_model.go explicit resource carveout; store.go relationship checks; quarantine.go dependency/reference extraction and final pre-promotion validation; shallow.go exact closure/recovery. Reuse bounded accepted-actor reader (100k events/refs,8MiB objects,256MiB aggregate). Projects/replies/items are separate bounded aggregates; no unbounded whole-board/thread event. Project items pin project schema revision; accepted answers pin exact reply revision. Shared engine provides canonical typed state, exact-head guards, immutable replay recovery, multi-head conflict projection and explicit resolution. Operation keys remain actor-global across kinds.

Native project fields: text, finite number, date, single-select; bounded table/board views. Items reference native issue/proposal/discussion or explicit draft/external descriptive target and exact schema revision; unsupported provider field/view types need mapping reports. Native discussion categories/status are descriptive collaborative state, not imported moderation authority. Replies are independently revisioned, same-thread bounded nesting; accepted answer pins content revision. Native workflows contain an existing validated hn.pipeline/0 definition and optional inert source documents/digests; 2MiB authenticated workflow-event ceiling, other resource events256KiB. Explicit materialization writes native pipeline bytes into .hn/pipelines; run request verifies exact signed workflow revision bytes against the proposal commit before existing policy/runner path. No automatic execution or role changes.

Command registration: WP03 main local extension-command map permits native engine registration without future overlap in main.go; resource-specific WPs add specialized commands over engine. Provider-neutral resource transfer helpers/registry bridge core typed state and optional resource interfaces. GitHub Discussions use GraphQL categories/comments/replies/answer/state mutations; Projects V2 need user/org owner+number/node selectors, field/item identity remapping; workflow definitions need contents-at-pinned-commit read and SHA/branch-guarded writes. GitHub-specific SDK/service is never required; GraphQL stays bounded/sanitized and rejects partial data/errors. External mutations journal each step and require verified positive reconciliation after uncertainty; clientMutationId is not an idempotency guarantee.

Workflow migration: arbitrary Actions YAML is preserved as inert source until an explicit native pipeline is supplied; own generated constrained Actions documents may round-trip if exact rendered definition/manifest validates. Exporting native pipelines to provider execution requires explicit runner/trigger/checkout/environment mapping and report of semantic differences; no silent equivalence. Re-export of untouched preserved source may restore exact bytes at explicit destination branch under blob-SHA guard. Native execution remains the existing governed runner path.

Final acceptance must exercise all native resource families offline and through a bare remote/clone, nogithub dependency exclusion, provider fake HTTP/GraphQL mutations/pagination/recovery, reference remapping and pre-write unsupported-mapping refusal. All native and provider WPs independently reviewed before final mission acceptance/consolidation.
