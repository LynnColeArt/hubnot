# Final-head evidence binding

Every check in this directory targets c5c131a210ee906779baca1ed1be00c1ab58ced1 (owned source SHA256 values in source-sha256.txt).

Focused root/client race and vet/build/format/diff logs were captured from the exact staged working-tree bytes subsequently committed as c5c131a2; there were no intervening source edits. Tagged binary builds and all c5c131a2-prefixed CLI logs ran after that commit. The CLI binaries were built from c5 source, with distinct private repository/HOME fixtures per variant and a Git-only PATH. These fixtures/binaries are outside this directory and must not be archived with it.

The authorized final complete command is go test -race -timeout 20m ./..., with a finite invocation-only runtime allowance and all assertions/tests enabled. It started at 2026-10-27T03:18:54Z. It completed with exit0 (hubnot634.177s). The complete final raw log is go-test-race-20m.log, copied only after completion from /tmp/hubnot-wp07-validation-TDAfuz/go-test-race-20m-c5c131a2.log together with its successful exit status. No duplicate full suite runs.

The earlier b2d36c28 default600s timeout, original source hash/head, initial fixture failures and rejected-unexecuted cleanup batch remain in /tmp/hubnot-wp07-validation-TDAfuz. Those outcomes are superseded history, never current-head pass evidence. Resource contention has not been proved as the timeout cause. The deletion-containing batch was not executed; a later explicitly authorized non-deleting batch left disposable fixtures outside evidence.

Final complete result: PASS, exit0. The final raw log is now go-test-race-20m.log; root package completed in634.177s. All package race tests passed. This positive result supersedes the earlier running status above; it does not erase or reinterpret the failed b2d600s invocation. There were no source edits during the final gate.

Normal for_review transition completed at source c5c131a2, event01M5XPSFHVD7FGEKHV2ZGBEDXY, after root resolved the unrelated WP08 ownership precheck. Runtime architectural authority was unavailable (no_coverage), not blocked and not force-bypassed; it is unverified. The first blocked transition log is retained separately.
