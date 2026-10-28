# Historical data compatibility

The [beta support contract](beta-support.md) is prospective. The executable
qualification in `TestBetaReleasedWriterCompatibility` covers public native data
written by the Linux AMD64 **standard executables from the actual release
archives** below, read by current standard and native-only (`noforge`) builds.
This does not qualify every release command, operating system, provider adapter,
or private alpha recovery journal, and does not itself declare a beta release.

| Released writer | Released source commit | Native facts | Project archive |
| --- | --- | --- | --- |
| 0.0.1-dev | `f82b57420f0c825a9ea6f3b9f93ede680c8cf06f` | Tested | Not introduced yet |
| 0.0.2-dev | `e80880abc792e2c386182e06027fc251b134c0d2` | Tested | `hn.project-archive/1`, `hn.project-snapshot/1` |
| 0.0.3-dev | `01a152cfbb822a7be9ccbc51dac34e3caa0cf925` | Tested | `hn.project-archive/1`, `hn.project-snapshot/1` |

The frozen corpus under `testdata/compatibility/` records the release URL,
archive and executable SHA-256 digests, original embedded build record, every
fixture file digest, exact public ref/OID pairs, historical IDs and semantic
JSON observations from each writer. The release numbers are independent of the
wire versions: these releases write collaboration `hn/0`, memory `hn-memory/0`
and version 1 resource schemas. These are not source rebuilds or outputs from
the current encoder. The corpus contains public Git objects and observations;
it contains no private signer, credentials or release executable.

The tests exercise a closed process environment with only Git on PATH, an
isolated HOME, no global Git configuration, and no provider service. The
current reader builds once per variant. Ordinary test runs never invoke the
fixture generator or a historical writer.

The representative histories include an issue opening, signed comment and
closed-state revision; projects, project items with exact schema/issue
references, discussions, replies and inert workflows; and a memory record with
a signed supersession edge. Tests compare historical JSON field meanings,
identities, types, lifecycle, stream ownership and trust classification while
allowing additive JSON fields. Opaque query snapshots/cursors and human warning
wording are not compatibility fingerprints. Git object bytes, signed payloads,
signatures and historical ref heads stay exact after reading, fresh-actor work
and historical archive import. A fresh actor can create an issue, record memory
and comment on the historical issue without changing the historical signer.

Public reconstruction requires no private signer or adopted audit authority.
Recall may create its disposable local lexical index. It does not install
policy or enable protected checks, automerge, workflows or audits. Possessing
historical data, importing a snapshot or initializing a new actor grants no
policy role. Project archives preserve explicitly selected public refs and
objects; they are not a backup of private keys or local recovery state.

## Unsupported formats and rollback

Public issue reading refuses a signed future `hn/999` protocol and a signed
`hn.project/999` resource schema, with a nonzero exit and a JSON error. The
synthetic future-format actors have independently verified Ed25519 signatures;
a broken signature is not accepted as version-refusal evidence. These probes
use a separate fixture ref and compare the entire repository before and after
the read, including private signer files and accepted refs.

Archive inspection and import planning refuse future archive and snapshot
schemas. Each negative archive is derived from a valid historical archive by
changing exactly one manifest schema string; all member bytes and framing are
otherwise retained. The tests require the intended schema diagnosis, unchanged
source/destination repository state, and no saved import session. They do not
pretend a corrupted tar or truncated record proves unsupported-version handling.

Upgrade readers before selecting histories containing formats or event kinds
they do not understand. Keep a verified public backup, any separately protected
private backup, and the compatible originating binary before upgrading. An
older reader may reject newer facts. Product version rollback never authorizes
rewriting signed data or reinterpreting an unsupported private journal as a new
session. Finish or explicitly recover such a session with its compatible
originating binary; see [migration recovery](migration-cli.md) and
[identity continuity](identity-v0.md). The retired short namespace remains
outside this baseline.

## Maintaining the corpus

Run the focused qualification with:

```sh
go test -count=1 -run '^TestBetaReleasedWriterCompatibility$' -timeout 8m .
```

The explicit Go/Git-only generation procedure and pinned archive digests are in
[testdata/compatibility/README.md](../testdata/compatibility/README.md). Preserve
existing fixture bytes and expectations when extending coverage; add a new
version or scenario directory. Never refresh expected output from the reader
under test to make a regression disappear. Generation uses disposable signers
outside the tracked output, removes them at exit, verifies the embedded
executable/source provenance, and refuses an existing output directory. New
random signers and timestamps mean reproduction is semantic, not a promise of
byte-identical newly generated bundles. Review and hash new evidence explicitly.
