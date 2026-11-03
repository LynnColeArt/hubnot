# Independent host and trust-boundary review — WP26 T104

Review date: 2026-11-03. Reviewer: fresh delegated Codex agent, resolved `reviewer-renata` with `spec-kitty agent profile show reviewer-renata` and implement-scoped charter context. This evidence author did not implement application code. This is the WP26 boundary assessment; a separate formal reviewer must review the four WP26 deliverables before approval.

**Result: no actionable blocking trust/default/history defect found in the reviewed scope.** This is a source review plus bounded Linux public-caller execution, not a new all-platform qualification or a claim that absent evidence proves safety. T105/T106 finished independent AI-agent reports are now reconciled in their own records; their documentation findings now have current-source corrections/separate verification below. Independent formal WP26 review remains required.

## Exact inputs and authority

Integrated lane HEAD `e5aa9aad6217b300238e936448e17f8e8329f229`, tree `e5d15973913591ec38954faee0e04eb0a02c041f`. Qualified package source `d5aee5d0226c2e86615c1a0910d3cc8b9592cce7`, tree `52ae0a18359665b0697a24ba917d33f480f52775`, compiler `go1.26.4`. No Go/module, action, pipeline, workflow or fast-selection input differs between qualified `d5` and this reviewed HEAD (`git diff --name-only d5aee5d0 HEAD -- '*.go' go.mod go.sum .hn/actions/ci .hn/pipelines .github/workflows ci/fast-tests.txt` returned no paths). Review-only documents do not change those inputs.

The Linux AMD64 standard archive is `hubnot_0.0.0-optional-extensions-qualification.1_linux_amd64.tar.gz`, SHA256 `eb20950180fa2bc9cd1f453f813b2436dd1f8827796fd269a3442f8b9e18e973`; extracted `hn` SHA256 `0af01c366e4a85639f0c90970d14003ae2693db574448c0ba51281ba75fee99a`. It was extracted from the retained current acceptance bundle, not rebuilt. Host: `Linux-7.0.0-42-generic-x86_64-with-glibc2.39`, `git version 2.43.0`; test harness compiler `go version go1.26.4 linux/amd64`. The raw child is the separately executed compiled Go test harness; it handwrites frames and is not the supplied reference application. Fixtures are source-assisted; host admission is through the supplied package executable.

Frozen inputs read from the mission: `spec.md`, `plan.md`, `data-model.md`, `research.md`, `coverage.md`, `contracts/artifact-v1.md`, `contracts/host-v1.md`, `contracts/default-workflow.md`, `contracts/conformance-fixtures.md`, current `ci-integration-decision.md` and `wp25-review-continuation.md`. Canonical lifecycle authority is the coordination mission `status.events.jsonl` and installed `event_sourced_review_result`; primary planning snapshots are not verdict authority. WP25 approved event `01M6G7PDFWH5EGKQE6T59C7YRH` binds formal independent review of `da6bd2b95a71cafb4cdbc27613a266cafae64183`. Its report is `/home/lynn/Documents/Codex/2026-10-24/wa/work/hubnot-independent-acceptance-20261103/hubnot-WP25-independent-review.md`. See [readiness](readiness.md) for all 25 exact approval payloads and source/evidence bindings.

## Contract-to-live-caller assessment

| Boundary | Actual source and caller | Observed control / assessment |
| --- | --- | --- |
| Raw grants and closed methods | `grants.go` `Authorize`/`methodAllows`; `session.go` `AuthorizeRequest`, request loop; `protocol.go` closed parsing; `extensionHostDispatch` | Each declared method also requires actual selector/action. Observation requires both read and observe. Native six-family read-only writes returned host `denied_access`/`do_not_retry` with unchanged protected state. Privileged, runner, policy, identity-lifecycle, memory-mutation and sync-apply requests returned `unsupported_capability`; a declaration/grant cannot create those adapters. |
| Canonical repository and registration | `extensionRun` recheck; `OpenEnablements`/`Load`; registration availability/rehash; existing safe physical Git resolver | Binding derives from verified physical common directory, registration instance and current random epoch; it is not a portable project authority. Each raw request rechecks current registration/enablement, while idle monitoring revokes sessions. Forged `repository_id` was rejected as `invalid_input` before missing-signer semantics. Existing fake-tracker control additionally tries a separately initialized, unenabled repository, proves no consumer launch/report change and zero external writes. |
| Signer pins and mutation ordering | `extensionNativeMutate` compares selected public identity with request actor; native issue/resource writers retain actor/operation/expected-parent validation; artifact writers retain actual signer validation | Missing signer refuses without creating identity. Artifact wrong actor returns `actor_mismatch`; changed-source namespace cannot be laundered by a granted conversion destination. Grant/policy refusals compare objects, signed refs and protected private state. Reading public identity remains a four-field public result with no initialization. |
| Per-request native authority | `extensionNativeDispatch` authorizes, checks repository, takes existing contribution-command lock, then uses governed native projections/writers; artifact mutation calls `artifactMutationPolicy` before signer/intent | No host-lifetime native write lock and no alternate privileged writer. Existing governed six-family/pending/policy/refusal controls exercise admission/visibility; fresh artifact selected-policy calls returned `unsupported_capability` before prohibited changes even with write grants. This review did not rerun every governed-family scenario. |
| Scoped output and credential boundary | Native fixed public-error mapping; artifact scoped selection after ID resolution; private stderr drain, finite framing, published inventory | Fresh artifact tests keep an ungranted same-stream/dependency body out of read/search/inspect results, while retaining signed provenance IDs. `PRIVATE-WP18`, registration instance/executable digest and semantic-intent markers do not leak. Native error projection avoids raw Git paths/stderr; identity is public only. A same-user extension can independently read files: host mediation is not an OS sandbox. |
| Unknown data and closure | `selectArtifactClosure` visited/bounded queue; exact required type/parent/previous/conversion/link closure; `artifactExchange` verified accepted promotion versus explicit opaque mode; finite pure schema | Missing required dependencies refuse; valid missing optional link returns `missing_optional`; unrecognized schema version returns `unsupported_schema`. Opaque preservation never sets accepted refs/validity. Signed custom data stays separate from `hn/0`/`hn-memory/0`; no native kind alias or automatic semantic activation. Old-client generic support is not promised. |
| Default optionality and execution | `optional_bootstrap.go` only installs compiled handlers behind completion barrier; registry access is explicit lifecycle; external launch is executable+argv, not shell | Native commands do not parse registry or start modules. Existing detector/default historical evidence includes deliberately violating controls; seven frozen source checks and six-root default evidence retain exact source attribution. No production dependency was added. This review does not relabel the older measurements as a current-HEAD capture. |
| Bounded lifecycle / uncertain operations | `session.go`, `process.go`, finite framed reader and cancellation; native/artifact operation lookup | Finite request/output/stderr/session/cleanup gates remain unchanged. A lost acknowledgement is uncertainty with immutable actor/operation recovery, never proof of absence. WP25 already reviewed fixture-stage/cleanup and exact-source race evidence; this bounded review did not repeat large corpora, timeout/soak or all tail races. |

## Fresh bounded execution

The supplied-binary environment was `HN_WP17_BINARY=/tmp/hubnot-wp26-boundary-controls/hn`, `HN_WP18_BINARY` at the same path, and `HN_WP17_EVIDENCE=/tmp/hubnot-wp26-boundary-controls`. Exact command:

```sh
go test -count=1 -race -json -run '^(TestExtensionNativeIdentityMissingCLI|TestExtensionNativeGrantDenialsCLI|TestExtensionNativeUnboundAndUnsupportedFramesCLI|TestExtensionNativeMissingSignerDoesNotInitializeCLI|TestExtensionArtifactTypeReadRawCLI|TestExtensionArtifactGrantRefusalsRawCLI|TestExtensionArtifactPolicyAndValidationRawCLI)$' .
```

Exit 0, 53.198s package time, seven roots / 13 named test/subtest passes, zero failures/skips. The positive artifact type-read resolves the exact published type ID. Artifact grant controls first perform an authorized create and read, then fail denied revise/resolve/convert/type publication, ungranted read/search, namespace laundering, wrong signer and missing schema for their asserted codes. Policy/validation controls exercise successful public operation lookup, optional dependency diagnostics and inert relationships before selected-policy refusal. Native identity reads succeed without identity; six-family denied writes, unsupported/forged/bounded frames and missing-signer mutation refuse for exact asserted causes. Every refused operation covered by these fixtures compares the protected before/after state; they do not pass on arbitrary nonzero exit.

Native frames are retained as ten `raw-stdio-*.json` records. Invalid-session checks and artifact requests are assertion-backed public-caller evidence; not every successful artifact response stdout is retained independently. Raw Go JSON and stderr are retained, with no source fallback. Evidence directory: `/tmp/hubnot-wp26-boundary-controls`; root must retain this packet with final mission evidence. File hashes:

| File | Bytes | SHA256 |
| --- | ---: | --- |
| `live-controls.jsonl` | 10352 | `73c80878433db70bf230a1c09e730ddbc60e9712c3a94242778936b2c67f53dc` |
| `live-controls.stderr` | 0 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `raw-stdio-1253202409.json` | 709 | `a630641868d977f15610b2ea1f31d03cc4d1a999bf5149236e8fafc26872f561` |
| `raw-stdio-1299383299.json` | 380 | `1978af5161a5e51ed8ac3c64ce87cb38051a43866bfab2bd517189bdfdc30be7` |
| `raw-stdio-1747028311.json` | 713 | `3ce7b5830d179cf069901e3e4d45fb6f2f6a46747c2207363d73e6c18c1eb03f` |
| `raw-stdio-2090829439.json` | 920 | `337d88eff327f6c04617715a2981e67ffe487285b18df4fe5b761463a738f9e5` |
| `raw-stdio-2450958371.json` | 684 | `e2ffe09bde11ccfb252a954e06cbcd84d42b3911c822f95b1cd4791b93657dd1` |
| `raw-stdio-2688775123.json` | 773 | `10a30576f81de497f95d8209fbc419a0b520ccc048460cab35f6609f24e786e1` |
| `raw-stdio-286554957.json` | 515 | `08a2952cb34976bda62e17f0f5221b58b03ad3154f49885c64793712a7594b40` |
| `raw-stdio-2891590903.json` | 736 | `c54584803451b0a0bd08a8cedd81e7bd07a6eb1b51b874894ff97b2900831cbe` |
| `raw-stdio-3511624838.json` | 791 | `a906283dd0ec2e3efbf227dcc84dcdb5c21e14db2573b2683c13cc4827f9f008` |
| `raw-stdio-382618183.json` | 395 | `9fe29a828be255d198278ee94b4ed50e50af85e76cb69b9109d5d730f1ca3d1b` |

## Findings and limitations

No new actionable security/default/history finding was identified. There is no blanket inference from green tests: the review is bounded to the inputs/callers above. No source fix or acceptance-contract change was made.

| ID | Disposition | Owner / follow-through |
| --- | --- | --- |
| BND-L01 | Host mediation is intentionally not isolation against malicious same-user processes or escaped process groups. | Extension operator/author must trust the selected executable; public guide already states this boundary. No unsupported sandbox claim is made. |
| EXP-OBS-01 | WP25 formal review accepted the documented host-final-validity boundary. Peer response receipt before expiry remains unproven; no response deadline is invented. | Closed as a nonblocking evidence limitation for this contract by independent WP25 review. A future receipt-timing contract needs separate correlated evidence. |
| BND-L02 | No new exhaustive Intel pass, every tail-race fault injection, inaccessible recovery-directory loader or literal 116384/116385 pair is proved here. | Root retains the manifest's explicit limit/evidence dispositions; current smoke and historical exhaustive evidence stay distinct. |
| DIST-MAC | Actual quarantined browser download Gatekeeper rejection remains failed/uncertified. | Maintainer accepts unsigned Mac beta as a disclosed limitation. Future signing/notarization requires new assets/provenance/native launch retest; it is not this functional review's blocker. |
| OBS-DOC | Completed separate fresh AI-agent trials are incorporated; native prerequisite and author recipe/publication friction remain explicit. | Root adopted the narrow documentation follow-through recorded below; formal WP26 review remains. No human/Mac/own-wire-client or retroactive corrected-guide trial is invented. |

No full suite, capture, 100000-record corpus, real ten-minute soak or hosted job was run for this review. No frozen Mac worktree, package/alias/ref, source file, lifecycle state, primary user file or stash was modified. The excluded private incident directory was not traversed.

## Reviewed source identities

Each SHA256 below is the actual file read at initial boundary-review HEAD `e5aa9aad6217b300238e936448e17f8e8329f229`; these initial input identities are preserved. Later documentation hashes are separately recorded below; production/test inputs are unchanged. Exact public contracts are bound separately in readiness.

| Path | SHA256 |
| --- | --- |
| `internal/extension/grants.go` | `3d62ac552d6332a3c5164952553a39793ee9b71886c2f0269a40945ab05e7261` |
| `internal/extension/session.go` | `bf7f6069ee25603c8b1f0a436c7c14ade9a25f9c08feb160604a88c7e5a5df9d` |
| `internal/extension/process.go` | `7d3270f1b3f42759a1e1e800c4e0167e50617135cedc938b2883e619438495df` |
| `internal/extension/protocol.go` | `26eb4f2448a6c5e723aebdc07c73ff03004103358f17b954eee1b842866d68e5` |
| `internal/extension/registration.go` | `e4f7c36cf0324d6a7a7906e050d2dd2f7a6cf081496b795966b6c9556712b1b0` |
| `extension_host_command.go` | `d8416a0a33696ff8539f385956c01aadead14cb2849f8c187507e9801fdcdb1c` |
| `extension_enablement.go` | `70d8a6d297c3ef1c28609a3bece499e28dfc51159f7c69fbd200bca991117c4d` |
| `extension_native_adapter.go` | `e63fd7d98dd73777e5389522d9f68d81ead945d8191ce5276d4877b7e3e466d7` |
| `extension_artifact_adapter.go` | `a38b34c12b2df6d1295f4d79351dc8c62e2ae2d310158adc9d8fce6c3a7453b4` |
| `extension_changes.go` | `a9f057252c69840216d98e982290d1e5e4caa05b49a3fcf53edcdd83495bcf0e` |
| `optional_bootstrap.go` | `7a92764036e6d8c865467e94cd8f36fc7dbf324495486bf17a720f64084a5b2b` |
| `artifact_closure.go` | `07635fdc43da88388a40a68adb8c1a58afbe02ac689baa0ef4e4d89345b1c418` |
| `artifact_read.go` | `b0196cb7187e4b08e3ff044e1a83037dea1e250ab4e2fb234de570c149aa5f2f` |
| `artifact_exchange.go` | `e2f396a7ea7171a0da1898ea904e754f586f256fb5ba15bd8997b3d6e27930f3` |
| `artifact_store.go` | `7400bdf6f02ad2b979cd5a33dd3b9b00f28aaa68cbb40def6beb9c483e95542c` |
| `internal/artifact/schema.go` | `57a5a7bb51113c44ee65d2e4c9d0f98ada7b07738d573624b1f376ccc026e8ad` |
| `extension_native_acceptance_test.go` | `d83fafaf9189c9ca4e3df75caa97f5f433742d54e60f908e51be8262e9429699` |
| `extension_artifact_acceptance_test.go` | `9fe651ee207a7721009dd007e4de6911166a6f4c3d9bb465952a8097ceec2591` |
| `extension_fake_tracker_acceptance_test.go` | `57a51130057fa7fc6fa2b2cc8f2bcc7d44566637036416391d8d4be405ad65a2` |
| `extension_host_lifecycle_test.go` | `df3556f288785c5c0982c5e5cc968e4af1a588acce052ba7e3fcea6e96942308` |
| `docs/extensions.md` | `7c6f44b9a2911d4a1944eb98f4bb4eca93243ac64eae1a0a58ed44f3ea73518b` |
| `docs/extension-consumers.md` | `0b282f3bc69c6508ac2ef2e824d9d02a5afb390aae16a355ed34d15a715a1b9a` |

## Documentation finding closure — separate verification

Root adopted documentation commit `1867c365c45a4b6289de640e6e2fdc9dbad2d316`, normally merged into lane HEAD `a2c21657ed6c6de22fe6f1109c6ae143917d2b3c`, tree `de702a661abf86f9bf9711b5c067010c143335ac`. The original trials remain byte-identical at guide source `39d72080478ae875924aee216cb4c5d9194ccc45` and qualified binary source `d5aee5d0226c2e86615c1a0910d3cc8b9592cce7`. This is later documentation follow-through, not a repeat cold trial or replacement of any existing package. No production Go/module, test, action, pipeline, workflow, packager, tool or policy input changed.

NU-DOC-01 is corrected by README's explicit existing-Git/new-project `git init` prerequisite. EA-DOC-01 is corrected by the full schema-v2/explicit-conversion recipe in packaged `docs/extension-consumers.md#publish-version-2-and-convert-an-item`; inventory README and extension guide link to that single recipe. It preserves full state/exact IDs and explicitly stops unless the private business result has `ok:true`, addressing EA-UX-01's already-documented management/business distinction without changing runtime semantics. EA-PUB-01 is corrected in current source guides by immutable public `d5` contract URLs and explicit matching-source-archive/offline Markdown paths. Binary archives still do not include the contract directory, and existing distributed guide bytes remain unchanged. No repack, added runtime provider/network dependency, canonical-contract duplicate or completed own-wire-client trial is claimed.

The separate short verification packet is `/home/lynn/Documents/Codex/2026-10-24/wa/work/hubnot-independent-acceptance-20261103/docs-example-verification-7017dfk8`. Readback confirms 18 actual Git/hn calls, all exit 0, including a normally completed management session whose private business response refused changed replay. It verified Git/native initialization, publication, v2 declaration without implicit conversion, explicit conversion, old/new history, replay refusal and disable/removal. These are documentation-example verification calls, not participant observations. Later final placement/text adjustments were checked statically; **the 18 calls were not rerun after that placement**. The final readback verifies unique recipe anchors and product-doc paths; the example README itself is a source-tree document rather than a binary-bundle member. The final five document hashes below match actual merged lane files. Independent formal WP26 review remains the final review of this closure.

| Final documentation path | SHA256 |
| --- | --- |
| `README.md` | `14d05db07244e78b3ebca6e5d54b168c0318430294e977b3d893c3f4929784e2` |
| `examples/extensions/inventory-model/README.md` | `a1d557fb15cb4d05e7f347aee9b665a2563580be04513abb18e179ed4b1ce384` |
| `docs/extensions.md` | `833f0d4fef4d17dd1823054c231f516d0c79c49842a7d208a85bb0d7ee84be9f` |
| `docs/artifacts-v1.md` | `1e6a945defed13a7ce96db3fac20dc4e9e21d47ac962233fc332f85603671913` |
| `docs/extension-consumers.md` | `e31955d85484c0cb8d2a6554cdf9863fcda6a59d73de80116f1aa70247cfd5b5` |

| Verification file | SHA256 |
| --- | --- |
| `summary.json` | `c2aeeb65a007133e853247b6542917e65ab9a1cdbb3c26ffcf5cb071e62dde8e` |
| `commands.jsonl` | `66a8ec58ff7d1b756e24e4aece13e99bdd9fcf5899de53af0305667e8cf9320c` |
| `documentation-placement-readback.json` | `a7e1037591da30f19c37db0ec823f83876dd130e5aaffe11350723a427ab1a60` |
| `five-file-final-diff.patch` | `82e0b117f97df7bc3dea0a6fed663b9450a6eccf903fe2ad3a94da01a514174b` |
