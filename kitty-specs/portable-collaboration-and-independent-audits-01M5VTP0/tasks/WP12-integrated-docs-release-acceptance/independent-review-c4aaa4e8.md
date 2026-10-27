# WP12 independent review — approved

Reviewer: `codex-integrated-independent-review`, resolved profile `reviewer-renata`.
Review date: 2026-10-27. This reviewer did not implement WP12. The orchestrator
claimed the review and will record this verdict through the canonical
`move-task`/`review_result` event seam, referencing this report's commit. This
report does not itself mutate lifecycle state or claim mission acceptance.

**Verdict: APPROVE. No blocking findings.** All 26 requirements have meaningful
behavioral evidence at the reviewed integrated source, and all eight mandatory
review items are accounted for below. Host and framework qualifications remain
limited as stated; approval does not convert cross-compilation into runtime proof.

## Exact review boundary

- Product workspace: `/home/lynn/projects/nichthub/.worktrees/portable-collaboration-and-independent-audits-01M5VTP0-lane-k`.
- Final source: `c4aaa4e840420b07d83617c6373d6f4088e2c077`.
- Final tree: `7e39bfbb51c70dd588f849f1697e5c24d88e8907`.
- Actual combined dependency base: `c9a0092a81c65b3acb39685f36959a9b7ed5e022`.
- WP12 changes exactly six owned files: `portable_collaboration_acceptance_test.go`,
  `README.md`, `CHANGELOG.md`, `docs/protocol-v0.md`, `docs/threat-model.md`,
  `docs/github-interoperability.md`. No production Go, module, defaults, CI
  thresholds, release tag or executable file is changed by these three commits.
- Shared specification, plan, research, data model, quickstart, contracts,
  ownership metadata and evidence were read from the canonical primary checkout.
  The contracts are `audits-v1.md`, `migration-v1.md`, `release-v1.md`.
- `AGENTS.md`, the complete generated review prompt, resolved Renata profile and
  action-scoped review charter were read. The generated prompt retains authored
  implementer metadata; this review uses the independently resolved reviewer
  identity rather than adopting that authoring recommendation.

## Independent checks and evidence integrity

Actual focused race checks at final source passed, without repeating the full
memory/performance suite:

1. Public integrated journey plus Azure Boards Project-only apply/resume,
   GitLab→GitHub guided/resume/offline, foreign-authority poisoning refusal and
   missing-checkpoint no-replay: root package **67.629s**, all five named tests
   executed. The integrated journey itself passed all three freshly compiled
   ordinary/`nogithub`/`noforge` subtests in **59.30s**.
2. GitHub inert/nonvacuous registration, GitLab exact/excess budgets, both
   Forgejo versions and both Gitea versions with real issue/comment recovery,
   distinct registrations, useful Bitbucket snapshots/unavailable-resource
   reporting, and Azure Server PAT work-item/comment read/write: root package
   **2.203s**, all eight named tests executed.
3. Archive hostile-member matrix, verify-before-write, dependency/digest/budget
   refusals, nonvacuous provider coverage, loss/dependency binding and separate
   Project/Repository scope: `internal/migration` **1.049s**, all six named tests
   executed. Traversal, absolute paths, symlink, hardlink, unknown, duplicate,
   digest, missing, oversized, trailing and gzip cases actually ran.
4. Independent `go vet ./...`, `go build ./...`, owned-file gofmt, complete WP12
   `git diff --check` and whole product diff check passed.

Commands, named results and log hashes are in
`independent-review-c4aaa4e8.json`. Live reviewer logs are in `/tmp`; no private
fixture repository, identity, key, HOME or binary is committed with this review.

The author's single unfiltered `go test -race -timeout 20m ./...` completed at
`8c88ede54ce98c176d0cec7211581de23140b923`, root package 744.090s, all eleven
tested packages passed, exit 0. I read its raw log/exit and independently checked
the Git trees: only one paragraph in `docs/threat-model.md` changed afterward;
all 670 other entries and all Go/module contents/modes are identical. This is
valid source-bound reuse, not a claimed second full run. Archived tagged public
matrices pass at 115.600s (`nogithub`) and 144.233s (`noforge`).

I independently recalculated every outer `validation-c4aaa4e8/SHA256SUMS` entry,
every one of the raw archive's 1,008 indexed payload hashes, all 251 negative
supplement payload hashes and all 55 canonical-receipt payload hashes. Their
archives contain exactly 1,009, 252 and 56 unique, safe regular members including
each inner index. Raw proof SHA-256 is
`a6fc0e35f9cf712be0e9c0dd82cd0d37f71de2eeff92c02c84489921d90ac877`;
negative supplement is
`400a16f391fb131c32a90c147381f9b0ee9a49c0f3c0820a62fa995c4bb86433`.
Tar/gzip parsers were used, not UTF-8 reads of compressed files.

A separate reviewer-written standard-library archive inspection of the actual
final packages in `/tmp/wp12-artifacts-c4aaa4e8-one` and `-two` verified:
all 11 outputs byte-identical; complete checksum membership; asset and executable
digests; source/tree/compiler/version binding; normalized gzip/tar headers,
ordering, modes and timestamps; all eight ELF/Mach-O CPU architectures; embedded
BUILD equality with provenance; exact maintained document bytes; and every byte
and mode of all 671 committed source files against Git objects. The generated
INSTALL guide's target/version/variant and runtime limitations were inspected
separately. An initial reviewer probe incorrectly expected generated INSTALL.md
to be a committed file; correcting that verifier assumption required no product
change. Final provenance SHA-256 is
`cb711df23b68972adc045284643946bb1d2f0ddc376b878e5459f8ae8bc8da58`;
SHA256SUMS file SHA-256 is
`87b33a2aace99ea01de4ac73d69528d8830003fbc95e6a95b6c2324917b8a40f`.

## Requirement assessment

Each row is PASS at this integrated source. `Integrated` below means the actual
compiled three-variant public journey, independently rerun here. Other named
tests are present at the final source and executed by the source-equivalent
unfiltered full suite; reviewer reruns are identified explicitly above. This
assessment uses assertions and runtime wiring, not merely prior WP approvals.

| Requirement | Verdict | Concrete behavior and proof boundary |
| --- | --- | --- |
| FR-001 | PASS | Integrated refuses audit-as-CI/decide/merge with unchanged refs/readiness, then real native CI allows merge despite failed audit. No policy is installed by init. `TestAuditFactsNeverSatisfyProposalEvidence` checks both outcomes; existing protected-check acceptance remains unchanged. |
| FR-002 | PASS | Integrated exports selected Git/native closure and foreign bytes, applies the saved plan after source removal and compares actual event payload/signature/attachment bytes. Snapshot/native tests exclude identities, anchors, journals, unrelated refs and working junk. Selected committed history remains exact. |
| FR-003 | PASS | Integrated preserves destination policy, admits observed histories, reads keylessly, refuses imported signer/anchor authority, then continues with a distinct local actor and deliberate local policy/anchor adoption. `TestProjectSnapshotKeylessObservedHistoriesRoundTrip` and local continuation assertions cover isolation. |
| FR-004 | PASS | Integrated inspects attributed foreign issue/comment parents and exact blobs offline. Real GL→GH journey retains unsupported retrieved changes/reviews/workflow source; provider snapshot tests check PR/MR/thread topology, attachments and custom data rather than synthesizing native approvals. |
| FR-005 | PASS | Local, archive, imported and provider commands reach the shared saved-plan engine. Integrated source-removal apply and actual GL→GH+Git journey demonstrate immutable observations rather than source refresh. |
| FR-006 | PASS | Actual guided GL→GH test refuses EOF at loss decisions and final approval, previews without writes and uses the same saved apply/resume path; explicit missing confirmation also refuses. |
| FR-007 | PASS | Missing governance/loss decisions and invalid scope refuse before effects; plan tests assert loss findings and receipt dependencies. Provider fixtures distinguish unavailable reads, preservation, mapping and unsupported recreation. |
| FR-008 | PASS | `main` local command dispatch reaches migration/audit handlers; shared provider contracts and registrations are live. Reviewer reran nonvacuous conformance/inert registration; unchanged legacy transfers execute in the full suite. No native provider-name dispatch was introduced by WP12. |
| FR-009 | PASS | Reviewer reran compiled self-hosted GitLab fixture→GitHub issue/comment plus selected Git migration, with explicit mappings and fresh-process lost-response reconciliation. Exact issue receipt supplies the comment parent; no duplicate create. |
| FR-010 | PASS | Reviewer reran useful issue/comment recovery under separate Forgejo 11/16 and Gitea 1.24/1.27 profiles and distinct registrations. Shared transport does not bypass each profile's version, identity and projection guards. |
| FR-011 | PASS | Reviewer reran useful Bitbucket repository/PR/workflow snapshot coverage and 200/401/403/404/410 distinctions. Removed issue selection refuses without a request; unavailable data is not empty/preserved success. Destination recreation stays unsupported. |
| FR-012 | PASS | Reviewer reran compiled positive Project-without-Repository Boards saved apply/resume, exactly one mapped item/comment and zero planning writes. Unexpected repository requests fail the fixture. Separate Server PAT test covers mapped read/write; immutable project/process/type/state remain explicit. |
| FR-013 | PASS | Reviewer reran lost-comment-response fresh-process resume and missing checkpoint refusal. Source review confirms sealed progress, destination equality/preflight, persisted bound payload before inflight/effect, confirmed dependency receipts and exact-only reconciliation; negative/inexact lookup has no path to another Apply. |
| FR-014 | PASS | Imported foreign content never becomes an actor stream, policy, anchor, native review/CI approval or runnable source. Integrated keyless refusals and explicit local continuation are real CLI calls; workflow authorization/inert-source tests cover separate conversion and authorization. |
| FR-015 | PASS | Integrated independently compiled and executed all three variants in Git-only PATH/private HOME/no credentials. Canonical negative proof also positively checks actual dependency/file/symbol presence in ordinary/retained-provider builds and absence of GH only versus all adapters. |
| FR-016 | PASS | Separate signed audit family binds old/already-merged code without requiring a proposal. Integrated proves failed audit does not block or qualify merging; explicit decision-evidence tests reject passed and failed audits as CI evidence. Old encodings omit absent audit fields. |
| FR-017 | PASS | Request/import/init/proposal/merge introduce no executor or service. Integrated unsafe-host refusal preserves accepted refs and only explicit execute succeeds. Source dispatch reaches existing backend selection and `executePipeline`; transport/mid-run withdrawal tests prevent invalid result publication. |
| FR-018 | PASS | Actual audit request/result/log dependencies survive archive admission and ordinary Git replication into keyless readers. Protocol/quarantine tests reject malformed/forged relationships, undeclared objects and incomplete native closure before accepted visibility. |
| FR-019 | PASS | Reviewer reran foreign authority poisoning refusal. Source checks anchor resolution, matching-only policy selector, current role/digest and accepted lineage/head projection. Integrated import-without-anchor fails; deliberate sole-owner local authorization succeeds. Candidate/stale/revoked/conflict and policy-identical code-advance tests cover the remaining current-authority cases. |
| FR-020 | PASS | Integrated execute opens no issue; explicit follow-up retries/later results reuse the linked issue and preserve operator edits/refs, and Git replication retains finding/request/result links. Interrupted-create/collision tests cover recovery/bounds. Concurrent publisher limits are stated honestly. |
| FR-021 | PASS | Actual final two-build outputs and Go release source use only Go/Git for packaging, with no shell/module dependency and isolated exact source. Reviewer verified outputs/architectures/source/provenance independently. Cross-compilation and actual Linux runtime proof remain distinct. No public publication operation is part of WP12. |
| FR-022 | PASS | All five maintained overview documents match parsed public command grammar, capability profiles, native mapping versus raw preservation/recreation, offline use, audit independence, separate ownership/authority, uncertainty recovery, packaging and host limits. Final threat paragraph correctly distinguishes native signed imports from foreign observations. |
| NFR-001 | PASS | Positive defaults remain 10,000 records, 1,024 blobs/refs, 16 MiB blob/manifest/object, 256 MiB attachments, 100,000 Git objects and 512 MiB Git material. Source rejects zero/overflow; reviewer reran dependency/budget and exact/excess GitLab controls; provider/native suites cover propagated bounds. |
| NFR-002 | PASS | Reviewer reran complete hostile-member controls and verify-before-write; integrated traversal refusal preserves the destination snapshot and creates no escaped file. Digests/dependencies are checked before observation promotion; full provider tests cover origin/pagination/credential guards. |
| NFR-003 | PASS | One unfiltered completed source-equivalent full race suite plus fresh focused race/public checks, vet/build/gofmt and clean WP12/product diff. No weakened defaults/CI/test assertions. The framework architectural helper is explicitly UNVERIFIED, not counted as a passing product test. |
| NFR-004 | PASS | Reviewer independently verified all 11 twice-built outputs, exact source and eight target/variant binaries. Windows/macOS packager cross-build evidence is explicit; only exercised Linux AMD64/ARM64 journeys are runtime-qualified. Unavailable hosts are recorded as allowed by this requirement. |

## Eight mandatory review items

| Item | Verdict | Finding |
| --- | --- | --- |
| 1. Dead code | N/A for new production code | WP12 adds only tests/docs. Integrated feature reachability was additionally checked: `main.go` → `localCommands`, `audit_commands.go`/`migration_commands.go` registration → actual authoring/execution/plan/apply handlers, shared migration engine and existing executor. Real public reruns verify live effects. |
| 2. Synthetic fixture tests | PASS | New test data is input to real compiled CLI production commands; assertions inspect actual writes, signed Git objects, refs, preserved blobs and remote HTTP effects. Removing the implementation prevents these journeys from passing. The Azure fixture implements API responses but never fabricates the asserted write count. |
| 3. Silent empty return | N/A for production additions | There are no new production code paths or silent production catches/returns. New test helpers fail through `t.Fatal/Fatalf` on command, JSON, filesystem and state-contract errors. |
| 4. FR coverage | PASS | All 22 FRs and four NFRs have concrete assertions/inspection in the table, plus canonical evidence references. A blanket reference to Integrated was not treated as sufficient for provider/packager behavior; those actual specialized tests and packages were checked. |
| 5. Frozen surface | PASS | The exact three WP12 commits change only the six allowed files. No commit changes old signed-event encodings, production/module/CI/default surfaces or historical release assets. Exact archive/native object comparisons and raw evidence hashes pass. No private key or generated binary is present in the diff. |
| 6. Locked decisions | PASS | No new source contradicts the no-service/no-implicit-execution/no-authority-import/no-blind-create/immutable-history constraints. Docs state unavailable mappings and host limits. Native release tooling remains Go/Git-only; reviewer-side archive inspection introduces no product dependency. |
| 7. Shared-file ownership | PASS | `lanes.json` and all WP owned-file lists assign these six files only to lane-k/WP12. The root integration-followthrough and planning-whitespace coordination notes cover the existing shared integration constraints; no overlapping stream guide was edited. |
| 8. Production fragility | N/A | No production raises/panics/error paths are added by this WP. Refusals exercised here preserve observable accepted state and require explicit operator recovery. |

## Canonical receipts and remaining boundaries

Canonical primary acceptance matrix has exactly 26 passing criteria. All three
first-class negative invariants are `confirmed_absent`, backed by actual named
test outputs and positive compile controls through the official executed seam:
`NI-INTEGRATED-DEFAULTS-AUDIT-IMPORT-HOSTILE`,
`NI-PLANNING-EOF-NO-BLIND-CREATE`, `NI-ACTUAL-FORGE-COMPILE-EXCLUSION`.
Their source guards and canonical write receipts are preserved. The canonical
issue #13 verdict is fixed from integrated audit/follow-up behavior; the external
GitHub issue remains open pending public PR merge. This is not an external issue
closure or publication claim.

Final Linux AMD64 and ARM64 standard/native-only package execution proofs were
read and hash-bound to the actual packages. ARM proof names the authorized
Golden Goddess Linux aarch64/Git 2.43.0 host and contains only public outputs.
This reviewer did not independently reconnect to that host. No full ARM Go suite,
macOS CLI runtime, macOS/Windows packager-host runtime or Windows CLI distribution
is qualified. Those gaps are documented and permitted by the explicit spec.

`tests.architectural._gate_coverage` remains unavailable/no_coverage and
UNVERIFIED. The ordinary pre-review transition completed without block/force/skip;
none of the actual product checks was waived. Inherited lane planning whitespace
is preserved as a failed broader check; WP12 and whole product diffs are clean.
The canonical-primary repair/coordination note is valid, with an unqualified
whole-feature diff check still required after consolidation. Stale scaffold
matrix notes do not override actual canonical criterion/negative-invariant truth;
root may reconcile prose after recording this independent verdict.

Root follow-through: record approval using this committed report as approval
reference with explicit codex/reviewer-renata/independent reviewer identity;
verify the event, run canonical mission acceptance, consolidate and repeat the
whole-feature diff check. Publication remains the separately authorized reviewable
PR workflow. This review performs no merge, push, tag or release action.
