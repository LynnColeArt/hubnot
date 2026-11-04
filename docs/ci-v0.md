# CI and runner v0

Hubnot CI separates coordination from computation:

Hubnot's own [project CI tiers and commands](project-ci.md) use this protocol.
Fast feedback is separate from explicit stress and real-clock soak requests.

The fast portability selection keeps 21 representative roots, including the
runner target-option, exact trusted-result counting and moderated legacy-identity
refusal controls. The full requester/two-runner, lineage, withdrawal and archive
journey remains in the explicit native integration selection. Its original eight
roots are retained, alongside eleven bounded native consumer-contract roots
(19 total), including the full public journey in actual default/native-only
executables and deterministic resolver ambiguity coverage. The action verifies
the explicit consumer inventory and rejects overlap with the unchanged 21 fast
roots. Its existing eight-minute race-package budget and 600-second pipeline step
are unchanged, as are existing assertions, fixture sizes and per-operation deadlines.
The [consumer contract](consumer-cli-v1.md) documents full identities, attributed
verdicts, exact receipts and the separate readiness/lineage observations.

```text
signed proposal
      ↓
signed run request ── exact commit + exact pipeline digest
      ↓
explicit runner execution
      ↓
signed result ─────── outcome + exit code + duration + log digest
```

Any identity can produce a result. The protocol preserves who claimed what;
the exact signed proposal base policy decides which results count for readiness
and automatic threshold suppression.

Results also sign the claimed execution backend, operating-system/architecture
pair, and runner implementation version. These fields make environment claims
inspectable; they do not independently prove those claims.

## Pipeline format

Pipelines live at `.hn/pipelines/<name>.json` in the proposed commit:

```json
{
  "version": "hn.pipeline/0",
  "steps": [
    {
      "name": "Unit tests",
      "command": "go",
      "args": ["test", "./..."],
      "workingDirectory": ".",
      "timeoutSeconds": 300
    }
  ]
}
```

`command` is executed directly, without a shell. A repository-defined custom
action is an executable tracked in the commit, for example
`./.hn/actions/test`. Shell behavior is only present if that executable itself
is a shell script.

Unknown JSON fields are rejected. Pipelines are limited to 64 steps, each step
is time-limited, and relative command and working-directory paths may not
escape the extracted checkout.

## Request verification

Before publishing a request, the client verifies that:

1. the subject is a signed proposal;
2. the proposal's code ref exists and matches its signed head;
3. the named pipeline exists in that exact commit;
4. the pipeline parses and passes structural validation.

The request signs the proposal ID, commit, pipeline name, and SHA-256 digest of
the exact pipeline bytes.

Before execution, a runner repeats every check. A pipeline changed in another
commit cannot satisfy the request.

One actor produces at most one current result for an exact request by default.
`hn run execute <full-request-id> --rerun` deliberately executes again and
publishes a replacement signed result; it does not rewrite the prior event.

## Reference sandbox executor

The default Linux backend uses Bubblewrap. It:

- resolves the `bwrap` executable only from the same fixed canonical system
  directories exposed through the sandbox `PATH`, ignoring ambient `PATH`;
- extracts the requested commit into a generated temporary directory;
- rejects unsafe archive paths and unsupported archive entry types;
- creates separate user, PID, network, IPC, UTS, and cgroup namespaces;
- drops capabilities and does not expose the runner's normal `HOME`;
- mounts system tool directories read-only;
- exposes the generated checkout as the only writable project workspace;
- exposes `PATH`, an isolated temporary `HOME` and `TMPDIR`, `CI=true`, and the
  exact `HN_COMMIT`;
- provides no external network interface;
- stops after the first failed or timed-out step;
- caps the signed combined stdout/stderr log at 1 MiB;
- removes the temporary checkout after execution.

The integration test runs the same custom action through both backends and
verifies that a host-only marker visible to the host backend is absent inside
Bubblewrap.

The sandbox still relies on the host kernel and currently lacks seccomp rules,
strong CPU/memory/disk quotas, controlled network capabilities, and portable
non-Linux implementations. It should not yet be treated as a hardened hostile
multi-tenant boundary.

An unsandboxed fallback exists for development. It retains the host user's
normal filesystem and network access and requires both:

```text
--backend host --allow-unsafe-host-execution
```

## Runner discovery

`hn runner once` and `hn runner watch` synchronize HN refs and discover signed
requests. They require a local acceptance policy with two exact matches:

```text
accepted pipeline name
accepted request-signer fingerprint
```

The signer match uses the full 32-byte public-key fingerprint. There is no
"accept anyone" mode. This means a trusted actor explicitly requests execution
of a particular proposal while the runner independently chooses whether to
honor requests from that actor.

`runner once` handles at most one request and publishes its result. `runner
watch` repeats this process, waits when no work is available, survives transient
sync errors, and cancels active execution on SIGINT or SIGTERM.

Runner-owned status/error messages include UTC RFC3339 timestamps. Each sync
prints a start message before its potentially slow work, then completion or
failure with elapsed time. These are human diagnostics, not a stable JSON API.

`hn run request` records locally and prints a publication reminder. It does not
sync implicitly: run `hn sync REMOTE` explicitly before expecting a remote
runner to see the request. Creation and every `hn run show REQUEST` state print
the full `Request: sha256:…` identifier needed by the withdrawal command.

## Useful work and queued withdrawal (unreleased source)

These runner-efficiency features are source-only. Published v0.1.0-beta.3
archives and Debian packages do not contain them; their assets and qualification
remain unchanged.

After normal synchronization, automatic runners explain and skip superseded,
merged, closed or conflicted proposal lineages, observed withdrawn requests,
and requests whose required policy pipeline already has enough qualifying
trusted results. Skips do not execute a backend or append a synthetic result.
The shared readiness evaluator uses the exact candidate/head, pipeline bytes
and policy at the signed base. It counts each trusted runner's latest result
across authorized matching requests once; a latest failure replaces that
runner's earlier pass. Untrusted, unauthorized and mismatched evidence cannot
satisfy the threshold. An absent/empty required pipeline policy preserves
existing explicit execution. An explicit `run execute` is not suppressed merely
because the automatic threshold is satisfied; normal authority/backend gates
and the same-request `--rerun` requirement still apply.

```sh
hn run request FULL_CANDIDATE proof
# Save the full Request: sha256:… line, or recover it with:
hn run show REQUEST_PREFIX
hn sync origin

# Original requester, before future execution:
hn run withdraw FULL_REQUEST --reason "Replaced by a revised candidate" --json
hn sync origin
hn run show FULL_REQUEST
hn run list
```

Withdrawal signs locally; only the explicit sync publishes it. The original
request actor/key alone may withdraw. Reasons are trimmed, nonblank UTF-8 text
of at most 4096 bytes. A retry with the same normalized reason returns the
original withdrawal without signing again, even after later actor events;
a different reason or signer refuses without changing state. List/show bind
withdrawn status to the full request and withdrawal event IDs. Both explicit
execution and automatic runners refuse/skip an observed withdrawn request
before backend work. A fresh request is distinct intent, though automatic
scheduling can still suppress it when enough evidence already exists.

This is queued-only, future-start suppression. It does not interrupt running
processes or undo external effects, nor revoke valid earlier or late results,
decisions or merges. The new explicit `run.withdraw` kind remains under `hn/0`;
older readers may reject that event and subsequent advancement of its actor
history. Upgrade participating readers before explicitly writing it.

`hn run help` and `hn run capabilities --json` are static, keyless discovery,
even outside a repository. They contact no remote, launch no worker and grant
no trust, execution or extension authority.

For [offline project archives](migration-cli.md#local-archive-without-accounts),
explicitly select the requester and required trusted-runner actor histories,
including accepted remote actor refs when applicable, and the exact proposal
code refs. An omitted exact
cross-actor decision/result dependency refuses export; import must preserve
signed payloads/signatures and request/withdrawal dependencies. Import does
not switch destination code, install private signers or adopt historical
execution/merge roles. Explicitly initialize a local signer to author new work.

Installation, import, upgrade and
ordinary default use enable no policy, daemon, account, database, extension or
runner. GitHub is incidental to this native Go/Git workflow.

## Explicit stale-base observation (unreleased source)

```sh
hn runner once --accept-pipeline proof --accept-actor FULL_REQUESTER_ACTOR \
  --skip-stale-base --target refs/heads/main --remote origin
```

`runner watch` accepts the same options. The two stale-base options must be
supplied together, and the target must be a full `refs/heads/NAME` branch ref.
Without them, movement of a target alone adds no new skip. With them, the runner
reads a fresh advertisement of that exact branch from the configured remote;
it never infers a target from checkout HEAD or a stale local tracking ref.
The probe neither fetches code nor writes refs. A moved base skips; missing,
ambiguous, malformed, failed, oversized or timed-out observation prevents a
backend start. The cancellable probe has a 10-second deadline and 64-KiB output
bound. This is a snapshot: later target movement remains possible, with no lock.

Normal configured Git/SSH authentication is preserved. Built-in interactive
Git/SSH prompts are disabled on supported transports; configured credential
helpers and supported SSH configuration remain an operator-trusted boundary,
subject to the same bounded failure. Unsupported SSH wrappers/options refuse
rather than guessing authentication. No credentials are replaced or published.

Two concurrent or partitioned runners can start before either observes the
other's result or a withdrawal. Threshold suppression is not a global lock or
an exactly-once promise. Optional native concurrent-start coordination is
tracked separately in [#37](https://github.com/LynnColeArt/hubnot/issues/37).
Deep native qualification before a later release remains
[#38](https://github.com/LynnColeArt/hubnot/issues/38); bounded source checks do
not certify published binaries or Mac runtime behavior.

## Attestation limits

A valid signature proves attribution and integrity. It does not prove that the
runner used the reference executor, ran the commands faithfully, or reported an
honest result. Trust policy, reproducible multi-runner agreement, and stronger
environment attestations remain separate concerns.
