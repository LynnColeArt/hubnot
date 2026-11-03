# Independent native-user AI-agent observation — WP26 T105

Date: 2026-11-03. **Bounded trial completed, with actual initial failures and friction retained.** This is an independently executed fresh AI-agent observation, not human-user research, CI replay or implementer validation. The participant used only its isolated kit's TRIAL/inputs, unchanged public user guides, supplied executable and ordinary host utilities. The WP26 evidence writer did not contact/coach the participant or read partial outputs. This reconciliation began only after root delivered the completed report.

## Exact inputs and environment

| Input | Identity |
| --- | --- |
| Actual package / source / tree | `0.0.0-optional-extensions-qualification.1`; `d5aee5d0226c2e86615c1a0910d3cc8b9592cce7`; `52ae0a18359665b0697a24ba917d33f480f52775` |
| Target / variant | Linux AMD64, standard; no Mac or native-only observation in this participant |
| Supplied archive identity | `hubnot_0.0.0-optional-extensions-qualification.1_linux_amd64.tar.gz`, SHA256 `eb20950180fa2bc9cd1f453f813b2436dd1f8827796fd269a3442f8b9e18e973` |
| Independently observed hn digest | `0af01c366e4a85639f0c90970d14003ae2693db574448c0ba51281ba75fee99a`, same in author and reader PATH |
| Public-guide source | `39d72080478ae875924aee216cb4c5d9194ccc45`; all 36 supplied public-file hashes matched final participant audit |
| Host | Linux x86_64, `Linux-7.0.0-42-generic-x86_64-with-glibc2.39`; Git2.43.0 |
| Report | `/home/lynn/Documents/Codex/2026-10-24/wa/work/hubnot-independent-acceptance-20261103/cold-trial-native-user/report.md`; SHA256 `f94ca39de1376a942a9b75a1bd439a4a6f200a701c4bfda41ee44314265f25e1` |
| Actual transcript | `/home/lynn/Documents/Codex/2026-10-24/wa/work/hubnot-independent-acceptance-20261103/cold-trial-native-user/commands.jsonl`; SHA256 `eb2f07aadf09b41104bbbe0330274a37bd7a76a630251d1838097d36bc8dbc94` |

Archive/source/tree provenance was supplied by kit inputs; the participant did not have/reconstruct the archive or source. Actual executable/public-guide hashes were independently checked. Root/WP25 separately bind those bytes to the exact current qualified archive. Guides actually read: README SHA256 `dffc28ab1599bd733f02746768e1d8ceceed1b4dfa62fd5ff3eb6ff15b23de69`, issues `3ad858bad8c004af5e2b17c5b31b47bcaa0b1d7546dc19702d83b26ec676b2e9`, projects `b7fbf1c9b508b0ed2bc7c69fe24439982250b73254581f55647abb0995d3d2d9`, discussions `0debf1dda2380d9ee800bbead423cae71fb03c46ff28327db19e3b8eb9ea50ac`, replication `798ddfe4591b94f4f17679a77b0ed2bb803e4df1645b86e40fb383cdbaf488c4`, resources `dbd9c93c821e66e055d21111ffe971fb8766731e71f9d51c70ae80df5af4cc88`.

Both wrappers used `env -i`, separate HOME/PATH, no system/global Git config or prompting, disposable identities and local repositories. Each application PATH contained only hn and Git. Host Python/shell recording/editing aids were outside that PATH and were not runtime dependencies. No source build, provider/account, extension setup, global setting change or lifecycle command occurred. The first two shell reads were manually transcribed before automatic recording and are labelled as excerpts; the hn/Git transcript contains actual argv/workdir/exit/UTC/duration/stdout/stderr.

## Observed commands and outcomes

Readback of the actual transcript confirms **52 hn/Git invocations:49 exit 0 and3 exit 1**. Host preparation/audits are separately labelled and not extra application successes.

| Stage | Actual outcome |
| --- | --- |
| First README start | `hn init --name 'Offline garden author'` in an empty directory failed exit1: `hn: not inside a Git repository`. `hn identity public` also failed exit1. `hn issue open --body ... --json 'Label the watering jug'` failed exit1 with `repository_error` / not inside Git. These failures remain failures. |
| Independent recovery | Participant inferred `git init -b main` from ordinary Git knowledge; then unchanged native init succeeded. No procedural coaching or source inspection supplied the recovery. |
| Issue / project | Opened and full-state revised a native issue with exact expected head; added a comment, created a project and linked item. Public JSON provided full IDs; no ID-format failure occurred. |
| Discussion | Created thread/reply, pinned exact reply revision as answer, edited reply; reader displayed pinned original answer while reply listing displayed later text. Added reader nested reply. |
| Outbound exchange | Author `hn sync origin` to disposable local bare remote verified9 events. Ordinary branch push was separate. Git clone initially had empty native issue list before hn sync. Reader initialized its own actor, selected exact author and explicit positive budgets, then reconstructed9 verified events. |
| Return exchange | Reader independently signed issue comment/close and nested reply; sync verified12 events. Author selected both actors and synchronized; reads showed closed issue,3 revisions,2 attributed comments and2 discussion replies. Both repositories agreed on ordinary Git HEAD and reader recovered README. |
| Default optionality | No extension registry/configuration/grant/launch/provider/runner/account step was used or requested. Reader actor/key filename inventory contained only its own actor; HOME directories remained empty. This is observed native independence, not internal syscall tracing. |

Author actor `391469dbe6b6838c575f66abd5c7646b34436b0110caf34f7929ba64f293e66c`; reader actor `681edab97c90a907c6e83178582d3fc26aee139be365aba45255ff6b0f317e10`. Issue opening `sha256:8ad49cb09c0635a5a47de366eaeb059e80b92dea257ae3d1c291bf9d762a9296`; author revision `sha256:aa73cfba454335f31940b5584ccd29ce65456fe7134ee363ea8cef0f53259565`; reader closure `sha256:214c283dd0be4d399708287022323f2c56d8bd7b9f2d9775c531c5e2e23eaaf6`. Full remaining public IDs, exact commands and outputs are in the completed report/transcript. No private key contents were read or printed by recording/verification aids.

## Friction and disposition

**NU-DOC-01 — explicit Git prerequisite needed.** The README Start-a-repository block began with hn init without saying the directory must already be a Git repository. This produced three real failures. Owner: root/documentation maintainer, narrow README correction and independent review (implemented/dispositioned below). The participant ultimately completed using unchanged native instructions after ordinary Git initialization; a later documentation correction does not retroactively erase the trial failures or establish a cold trial of the corrected guide.

Full IDs and revision-versus-opening IDs required careful movement between outputs and commands. Existing guides enabled success; host JSON editing helped. No human copying/usability metric is inferred. README advertises older0.0.3-dev while TRIAL directs the unreleased supplied candidate; participant followed the explicit candidate. This is not evidence that downloading the README-linked release gives these features.

## Limits and conclusion

T105's native init/ticket/discussion/exchange task was actually completed without extension configuration. The initial missing prerequisite remains a documented finding corrected in the later source-guide follow-through below. It is not an observed default extension regression. No hostile-input/no-effect private snapshot control, concurrent conflict, large-history/ceiling, keyless third clone, archive download, other platform/variant or extension behavior was claimed. There are no human observations and no Mac browser-install certification. Trial keys remain disposable private state in the isolated kit, not publication material. Formal WP26 approval and mission acceptance are separate.

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
