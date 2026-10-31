# WP20 independent review — changes requested

Reviewed delivery: `c8a38b2ce35ef0957a54522e9d721d8e03f1d1ef`; all303 Go/module inputs exactly map SHA256 `988e6e88bc5bb9432c3251cafac01b972f9d0a3d32eb96dfa6aded0cc26e27fd`. Actual supplied consumer SHA256 `da07d4d873210d2cd890b3a13ce1b4e0eb909f6033bb8f5001a3f655c01af3fa`. Reviewer actual tool `codex`, resolved profile `reviewer-renata`.

## R1 — P1: Keep consumed uncertain native outcomes pending until original-operation recovery

`examples/extensions/fake-tracker/main.go:341–347` saves every `ok:false` response as `prior.Refusal`, including the valid correlated host `uncertain_operation` / `inspect_operation` response naming the exact dispatched native actor/key. On restart, `intent()` at lines311–312 returns this saved uncertainty before querying `resource.operation`. For updates, `importItem()` at line429 excludes every record with a nonnull `Refusal` from pending recovery, then reads fresh external source and may choose a new version/operation key.

This contradicts FR-007/FR-021, T081–T083, host-v1.md §§3/6–7 (post-dispatch uncertainty retains its original operation and requires inspection), and the plan's immutable recovery rule. A consumed uncertainty response is not a definitive refusal or proof that publication did not happen.

Reproducer (outside repository, no source edits):

```sh
python /home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-WP20-review/uncertain-response-probe.py
```

The probe runs the exact standalone executable using public framed hello/identity/mutation responses and an actual local HTTP source. Its successful initial import control emits identity, issue and comment requests and consumes all success responses. Then:

1. Initial issue mutation receives a valid correlated host `uncertain_operation` with the exact actor/operation, `recovery=inspect_operation`, null data, and a complete closed error envelope. The connector saves it. On a second import invocation, valid found/replay response frames are available, but the consumer emits only `identity.public` and exits1; it never emits `resource.operation` for the original request.
2. A successful initial import is followed by update-2 receiving that valid uncertainty response. After the HTTP source advances to version3, a second update invocation performs one fresh GET and emits a distinct update-3 mutation key, without any original-operation lookup. The probe's valid scripted acknowledgement permits exit0. This proves the consumer's wrong request emission and durable journal transition; it does not claim any scripted host response published real native history.

Exact original/update keys, input/output frames, complete journal/report per session and HTTP read counts are in `uncertain-response-results.json` and adjacent `uncertain-import-*.json` / `uncertain-update-*.json` in the evidence directory below. The original key is `tracker-c0bcd7ff587533cef8c3c26af3a74362ec12959fa2dc9708c540b0de4cc141a3`; the newly emitted key is `tracker-eac2cadc3b630b6644c1b6dbdba77d194496ab88af3bdfd05842414dc393c953`.

Required correction: distinguish definitive refusals from unsettled post-dispatch outcomes. Preserve a consumed `uncertain_operation` as the same immutable pending intent; on the next explicit invocation, query/reconcile its original actor/key and replay only that exact saved request under the contract's found/verified-absent/uncertain rules. Pending updates must be selected before any external GET/new key even when a response recorded uncertainty. Handle BOTH newly received nondefinitive errors and journals ALREADY containing the persisted uncertain `Refusal` shown by these artifacts; merely preventing future writes to that field does not recover existing pending operations. Preserve true definitive refusals and the original response evidence. Classify the published recovery guidance deliberately: `inspect_operation` preserves the unsettled original operation, while `retry_identical` never permits changed actor/key/parents/content or blindly selecting a new intent; `do_not_retry` definitive refusals remain explicit stops. Do not add automatic retry behavior beyond the frozen contract. Add meaningful actual-executable public-boundary controls for consumed uncertainty on initial import and update, including a newer external version/native head, and show that unresolved lookup cannot trigger a fresh source read or new key.

WP23 depends on WP20 and must not treat this consumer as approved until corrected/re-reviewed; downstream integration must use the corrected source.

## Verification and scope

All299 inherited inputs equal the approved consumer union; only its six declared approved dependency updates differ from approved19. All remaining293 inputs, production+module equivalence since6f, approved contracts, binary/evidence hashes and five owned source paths were checked. There are no reviewer edits to application/test code.

Independent runtime checks: native default path plus real registry-read detector passed; all7 actual standard-host/standalone-consumer process cases passed (12 test events); all15 consumer unit tests passed (50 events); all14 reviewer-authored framing controls passed with exact negative cause/no following request/API read/journal; the intentionally premature-checkpoint executable failed exactly `external response loss advanced checkpoint or did not really write`. No skipped tests. These positive cases do not cover or waive R1. Supplied implementation standard/combined/race evidence remains bound to exact source; no unrelated global suite was repeated.

Anti-pattern checklist: 1 Dead code **PASS** (main→consume/serve, HTTP handler, durable I/O and validation all have production callers); 2 Synthetic-fixture test **PASS** (FR acceptance invokes the actual external executable; synthetic frames are explicit fault inputs to real client code); 3 Silent empty return **PASS** (unmapped resources/unchanged revisions are documented no-ops; errors fail closed); 4 FR coverage **FAIL** (consumed uncertain mutation recovery in FR-007/FR-021 is missing and broken); 5 Frozen surface **PASS** (no authored frozen/contract/module/baseline changes; approved inherited integration exact); 6 Locked decision **FAIL** (new intent is selected before original uncertainty settles); 7 Shared-file ownership **PASS** (five owned deliverables only; six dependency paths are approved coordinated imports); 8 Production fragility **PASS** (no panic/bare throw or undocumented transient crash; fail-loud errors are explicit, with R1's recovery semantics separately rejected).

Evidence: `/home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-WP20-review/`. All303 inputs remain unchanged after checks. Every owned subprocess was waited and both independent HTTP threads joined; process census found no running supplied host/consumer/mutant. No Mac, whole-mission, beta/main or release verdict is claimed.
