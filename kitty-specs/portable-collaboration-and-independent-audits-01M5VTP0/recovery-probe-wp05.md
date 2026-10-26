# WP05 focused recovery probe

Read-only debugging probe at product commit `5444f6236ded9c1732d0cb9e1744d75f28d86a1f`, before final validation. This is not a formal work-package review or lifecycle verdict. No product, primary, or status files changed. The imported foreign-record continuation and shared Git HTTPS authentication investigations were excluded.

Read the WP05 prompt, frozen migration contract, migration engine documentation, and plan/binding/journal/progress implementations. Temporary tests replace `migration_engine_test.go` only through Go's overlay mechanism:

`/tmp/wp05-recovery-probe-72ahkkw1/overlay.json`

## 1. Missing authenticated checkpoint repeats uncertain creates

`ApplyMigration` (`migration_engine.go:236-250`) treats a missing `progress.json` as an unused session. `PrepareMigration` saves the plan and snapshot but no initial progress checkpoint. Consequently missing/corrupt recovery state cannot be distinguished from a first apply.

Reproduction: prepare the existing issue/comment fixture, apply once (issue confirmed, comment create returns `io.ErrUnexpectedEOF`), remove only `progress.json`, then apply again. Counts change from **issues=1/comments=1** to **issues=2/comments=2**. The second run calls both mutations rather than reconciling the uncertain comment. Its returned error is `uncertain comment; resume reconciles only: unexpected EOF`.

```sh
go test -overlay /tmp/wp05-recovery-probe-72ahkkw1/overlay.json -run '^TestProbeMissingProgressDoesNotRetryUncertainCreate$' -count=1 .
```

The test intentionally fails its no-repeat assertion. Root confirmed this is relevant recovery corruption and relayed it to the author. Suggested shape: create an authenticated initial checkpoint during prepare and refuse its later absence, while binding the actor safely on first apply. This finding concerns absent state, without forging or using the private HMAC key.

## 2. Changed provider destination is rejected after its first effect

The saved plan has a destination, but `ApplyContext` and `BoundWrite` expose no frozen destination (`internal/migration/plan.go:52-57,83-92`). Closed issue/comment payloads likewise have no destination. `ApplyMigration` calls the provided adapter (`migration_engine.go:311`) and validates the destination only in its returned receipt (`migration_engine.go:316`; `ValidateReceipt`, `internal/migration/plan.go:444-454`).

Reproduction: prepare the existing provider-neutral fixture with repository `planned-destination`, change the adapter's configured repository to `different-destination`, then apply. The first create occurs (**issues=1/comments=0**) before the engine rejects the receipt with `receipt does not prove exact bound write`.

```sh
go test -overlay /tmp/wp05-recovery-probe-72ahkkw1/overlay.json -run '^TestProbeSavedDestinationChangeRefusesBeforeWrite$' -count=1 .
```

The test intentionally fails its pre-effect refusal assertion. This is a provider contract/API gap: a reopened adapter cannot compare its configured target against the saved destination using its engine-provided input. Local/archive/Git implementations have additional payload baselines and are not claimed vulnerable by this fixture. Root and the implementation agent received this reproduction for assessment before provider adapter work.

## Focused positive controls

Existing focused tests passed (no full suite duplicated):

- Root tests: persisted comment binding and negative reconciliation never retrying; verified blob refusal before writes; journal tampering/locking/unsafe paths. `ok hubnot 0.052s`.
- Internal tests: exact persisted bytes and confirmed predecessors; exact positive closed receipts; explicit loss decisions and dependency binding. `ok hubnot/internal/migration 0.005s`.

No additional concrete gap was found in the inspected exact-only inflight reconciliation, confirmed dependency restoration, immutable payload copies, authenticated plan/snapshot loading, or explicit loss-decision validation. These observations are bounded to this focused probe and source commit.
