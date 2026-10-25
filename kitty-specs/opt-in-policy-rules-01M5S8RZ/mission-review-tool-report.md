# Generic mission review diagnostic and lossless archive

Command: `spec-kitty review --mission opt-in-policy-rules-01M5S8RZ --mode post-merge`.
Exit code: **1**. Helper result: **unavailable**, `MISSION_REVIEW_TEST_EXTRA_MISSING`.
No scanner gate or product verdict was generated. The separate independent Go audit remains PASS in [mission-review-report.md](mission-review-report.md).

The exact original combined stdout/stderr bytes are preserved losslessly in [mission-review-tool-report.raw.gz](mission-review-tool-report.raw.gz). This Markdown file is a readable display and integrity manifest; its diagnostic display omits Rich's trailing padding and is **not** the byte-exact raw output. No diagnostic content or verdict was changed.

## Integrity manifest

| Item | Value |
|---|---|
| Original source commit | `86fee55312182d5bd8bd5beee511ba5fb4acbab7` |
| Original source path | `kitty-specs/opt-in-policy-rules-01M5S8RZ/mission-review-tool-report.md` |
| Original Git blob | `41011dd8e156b71dcbbc914382ddf259227978fa` |
| Decoded raw bytes | `871` |
| Decoded raw SHA-256 | `f08200d422a95956152098e5ad3da50f220b93236b50c7b8a9d3eabff14b270d` |
| Gzip archive bytes | `385` |
| Gzip archive SHA-256 | `9845d0666e008905b031c33accda37fb0945144dcd33a8076089260b367f7b1b` |

The archive uses gzip with deterministic `mtime=0`. Decompression was compared directly against the original Git blob above and found byte-for-byte identical, including all original padding and newlines.

From this artifact directory, verify the archive and decoded digest with:

```sh
gzip -t mission-review-tool-report.raw.gz
sha256sum mission-review-tool-report.raw.gz
gzip -dc mission-review-tool-report.raw.gz | sha256sum
```

The two SHA-256 outputs must equal the archive and decoded hashes in the manifest. The original source can independently be decoded from Git at the repository root:

```sh
git show 86fee55312182d5bd8bd5beee511ba5fb4acbab7:kitty-specs/opt-in-policy-rules-01M5S8RZ/mission-review-tool-report.md | sha256sum
```

Its output must equal the decoded raw SHA-256. These commands inspect the archived diagnostic; they do not rerun or change the unavailable helper result.

## Readable diagnostic

```text
WARNING  Could not evaluate session from storage
spec-kitty: logged_out_on_connected_teamspace teamspace=LynnColeArt/hubnot command=review action=run-spec-kitty-auth-login
Error: MISSION_REVIEW_TEST_EXTRA_MISSING: pytest is not importable from the
active Python interpreter. Run `UV_TOOL_BIN_DIR=/home/lynn/.local/bin uv tool
install --force --with pytest spec-kitty-cli==3.2.7` to install pytest into that
interpreter, then retry.
{"diagnostic_code": "MISSION_REVIEW_TEST_EXTRA_MISSING", "message": "pytest is not importable from the active Python interpreter. Run `UV_TOOL_BIN_DIR=/home/lynn/.local/bin uv tool install --force --with pytest spec-kitty-cli==3.2.7` to install pytest into that interpreter, then retry.", "remediation": "UV_TOOL_BIN_DIR=/home/lynn/.local/bin uv tool install --force --with pytest spec-kitty-cli==3.2.7"}
```
