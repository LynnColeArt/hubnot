---
work_package_id: WP02
title: Bounded optional GitHub REST client
dependencies: []
requirement_refs:
- FR-003
- FR-006
- FR-008
- NFR-001
planning_base_branch: feat/optional-github-interop
merge_target_branch: feat/optional-github-interop
branch_strategy: Planning artifacts for this mission were generated on feat/optional-github-interop. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/optional-github-interop unless the human explicitly redirects the landing branch.
subtasks:
- T005
- T006
- T007
- T008
history: []
agent_profile: implementer-ivan
authoritative_surface: internal/githubapi/
create_intent:
- internal/githubapi/client.go
- internal/githubapi/client_test.go
execution_mode: code_change
lane: planned
owned_files:
- internal/githubapi/client.go
- internal/githubapi/client_test.go
tags: []
tracker_refs: []
---

## ⚡ Do This First: Load Agent Profile

Read AGENTS.md, this mission's spec.md and plan.md. Load implementer-ivan through the canonical profile-load skill and `spec-kitty agent profile show implementer-ivan --json`. Work only in the runtime-provided lane workspace. Use the existing native issue query/mutation and validation functions; never bypass event verification or alter signed wire schemas. Write behavior tests before implementation. This mission is explicitly authorized to completion; do not ask the user for repetitive permission. Independent review is required: do not approve your own work.

## Runtime and delivery

Use `spec-kitty next --agent <your-agent-name> --mission optional-github-interop-01M5RK1Z --json` and the generated action context. Implementation is a separate agent from review. Commit only owned source/tests/docs (and required task evidence), not unrelated generated state. Record subtasks with mark-status. Move to for_review only after relevant gates; if a synchronous gate remains running, poll it to completion rather than kill/retry. Return exact commit, commands/results, limitations and workspace. Do not merge to public main or publish GitHub issues.

## Global invariants

Core Hubnot and all local compatibility commands work offline with no credentials or gh executable. Explicit GitHub transport is standard-library only and excluded by the nogithub build tag. Imported facts use the native signer; external attribution remains source metadata. No implicit sync, timestamps-as-winner, shell execution of ticket text, or unsupported silent semantics. Existing 256 KiB native bounds apply, including 64 KiB body and namespaced metadata value <=4096 bytes. No credentials in logs/signed metadata/checkpoints. All list/history readers consume complete bounded pages, and stale snapshot errors refuse partial answers.

## Objective and implementation contract

Implement standard-library-only package hubnot/internal/githubapi per IC-02 plan contract. Put `//go:build !nogithub` on implementation and tests so core-only build excludes the package entirely. No module dependencies.

Required exported exact API:
`NewClient(baseURL, token string) (*Client, error)`; methods GetIssue(context.Context, string, int) (Issue,error), ListIssues(context.Context,string) ([]Issue,error), ListComments(context.Context,string,int) ([]Comment,error), CreateIssue(context.Context,string,IssueInput) (Issue,error), UpdateIssue(context.Context,string,int,IssueInput) (Issue,error), CreateComment(context.Context,string,int,string) (Comment,error).
DTO fields: Issue has ID int64, NodeID string, Number int, Title/Body/State/HTMLURL/CreatedAt/UpdatedAt string, User User, Labels []Label, Assignees []User, PullRequest json.RawMessage. Comment has ID int64, NodeID/Body/HTMLURL/CreatedAt/UpdatedAt string, User User. User has Login string; Label Name string. IssueInput Title/Body/State string, Labels/Assignees []string with correct snake_case JSON (include empty collections when clearing, create issue may omit state because REST create lacks it; coordinator handles closing).

Use HTTPS by default; permit explicit loopback-only HTTP for fixtures, rejecting ambiguous hostname/userinfo/query/fragment. Never follow redirects (even same origin is simplest), never take pagination to another origin/path/repo, never send token in URL, and never return raw HTTP error bodies/header values that could leak token. Validate repo is exactly OWNER/NAME with safe path segments and positive issue numbers. Timeout <=30 seconds/request, response <=4 MiB, <=100 pages/list and <=64 MiB aggregate (export bounds constants for tests if useful). Read all pages via validated Link or explicit page iteration until exhaustion; never return partial slices on limits/errors. Filter PullRequest in ListIssues; GetIssue can signal PR for coordinator. GET auth optional; write auth mandatory and checked before requests. No automatic retries on POST/PATCH/network failure. Successful empty/malformed JSON fails, trailing JSON fails. Reject unexpected HTTP status. Check valid response identifiers and shape where needed; source content bounds are coordinator responsibility.

API media type and version documented in research; use supported stable version. Error type can expose status code for operation recovery, sanitized message only. Tests use httptest, not live GitHub. Ensure redaction tests include token-shaped error payloads and unsafe base URLs. Do not edit root command routing.

## Subtasks

T005 Define provider contract and HTTP fixture tests for pagination, redirects, credentials and failures first.
T006 Implement strict explicit base URL/repository validation and sanitized bounded HTTP requests without write retries.
T007 Implement issue/comment reads and writes, complete pagination and PR filtering.
T008 Verify client bounds, same-origin handling and omission from nogithub builds.

## Review criteria

Behavior matches every referenced requirement; scoped and full required gates pass; no unrelated files, private data or binaries staged. Unsupported behavior fails explicitly. Reviewer independently probes hostile/failure cases and the standalone boundary.
