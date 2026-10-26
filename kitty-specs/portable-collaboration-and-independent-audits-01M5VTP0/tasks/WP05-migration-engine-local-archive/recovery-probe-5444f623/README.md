# WP05 portable recovery reproduction evidence

Both failures were observed against product commit `5444f6236ded9c1732d0cb9e1744d75f28d86a1f`. This directory packages existing debugging evidence; it is not a formal WP review or verdict. No tests were rerun while packaging.

## Contents

- `missing-progress-probe.go.txt`: exact probe function used for missing progress; after the initial issue-confirmed/comment-inflight result (counts 1/1), deleting only progress.json and applying again produces counts **2/2**.
- `destination-change-probe.go.txt`: exact probe function used for a destination change; planning against one repository and applying the adapter configured for another creates **1 issue** before receipt validation rejects it.
- `baseline-migration_engine_test.go.txt`: frozen original fixture/imports/tests from the source commit, used by both probes. All Go evidence is stored as `.go.txt` so copying this directory under kitty-specs introduces no Go package.
- `*.failure.log`: captured original failing outputs, including counts and final errors. These are intentionally failing pre-fix assertions, not later validation logs.
- `source-binding.json`: full product commit and SHA-256 of original Git blob bytes for the relevant boundary and fixture.
- `prepare_overlay.py`: builds a new absolute-path Go overlay for any explicitly chosen local checkout; it performs no test execution and makes no checkout changes.

## Recreate without old temporary paths

Use a checkout at the full source commit above to reproduce the captured failures. A different checkout tests that chosen product state; results may change after fixes. The frozen test fixture targets the original API and may require intentional adaptation if a later contract changes.

Set `evidence` to this directory's preserved location and `checkout` to the desired checkout; the output directory should be temporary and outside the repository. For example:

```sh
evidence=/absolute/path/to/preserved/wp05-recovery-evidence
checkout=/absolute/path/to/nichthub-checkout
output=$(mktemp -d /tmp/wp05-replay-XXXXXX)
python3 "$evidence/prepare_overlay.py" "$checkout" "$output"
cd "$checkout"
go test -overlay "$output/overlay.json" -run '^TestProbeMissingProgressDoesNotRetryUncertainCreate$' -count=1 .
go test -overlay "$output/overlay.json" -run '^TestProbeSavedDestinationChangeRefusesBeforeWrite$' -count=1 .
```

Each original command returned exit code 1. No original ephemeral directory is required. The only `.go` output is generated into the operator-selected temporary output directory; the chosen checkout remains unchanged.

## Relevant boundary

- Missing state: `ApplyMigration`, migration_engine.go lines 236-250, initializes fresh progress when progress.json is absent. `PrepareMigration` had not saved an initial checkpoint.
- Destination mismatch: migration_engine.go lines 307-317 calls Apply before ValidateReceipt. The original ApplyContext and BoundWrite (internal/migration/plan.go lines 52-57 and 83-92) provide no frozen destination to the adapter. Local/archive/Git concrete baselines are outside this fixture's claim.

Original focused existing controls also passed: root binding/blob/journal tests (`ok hubnot 0.052s`) and internal binding/progress/reconciliation tests (`ok hubnot/internal/migration 0.005s`). No full suite was duplicated by the probe.
