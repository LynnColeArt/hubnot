# Selected replication and shallow recovery v0

Hubnot imports collaboration facts through an explicit validate-before-
promotion transaction. Selection controls which remote histories may enter the
local accepted projection; project policy independently controls whose valid
claims qualify as authority.

## Save a per-remote selection

```sh
hn replication select [REMOTE] \
  [--actor <full-64-hex-actor>]... \
  [--proposal <full-sha256-candidate-event-id>]... \
  [--memory <full-sha256-stream-id>]... \
  [--all] \
  [--max-events N] \
  [--max-objects N] \
  [--max-object-bytes N] \
  [--max-attachment-bytes N] \
  [--max-total-bytes N]

hn replication show [REMOTE]
```

The remote defaults to `origin`. Actor, candidate, and memory-stream selectors
must be full IDs. Every budget is a positive integer. `--all` cannot be
combined with exact selectors, and an exact selection must name at least one
actor, candidate, or memory stream. An actor/candidate-only selection imports
zero memory; memory selection is independent transport authorization.

Selections are sorted and saved as owner-only local state below
`.git/hn/replication/selections/`; they are never committed or published.
`show` reports whether the selection was explicitly saved, whether
compatibility-all is active, every full selected ID, and every budget.

When flags are omitted, the saved budget defaults are:

| Budget | Default |
| --- | ---: |
| `max-events` | 10,000 |
| `max-objects` | 100,000 |
| `max-object-bytes` | 67,108,864 (64 MiB) |
| `max-attachment-bytes` | 16,777,216 (16 MiB) |
| `max-total-bytes` | 1,073,741,824 (1 GiB) |

No saved selection means version-0 bounded compatibility-all. The client
enumerates advertised `refs/hn/actors/*`, `refs/hn/proposals/*`, and
`refs/hn/memory/*/*`, accepts only well-formed ref names, and sends every
discovered ref through the same quarantine, budgets, validation, and promotion
path. A memory stream ID must resolve to one unambiguous actor owner. An
explicit saved selection remains authoritative until replaced.

## Exact fetch and quarantine lifecycle

### Actor publication after import

After importing, `hn sync` publishes local actor refs with ordinary non-forced
Git pushes. A local copy can be older than the same actor's history on the
remote, particularly in older clones. If the exact actor history verified and
promoted during this sync already contains that local copy, publication skips
the redundant push. The local ref is retained and the accepted remote ref
records the newer verified history. Other local actors can still publish their
new events normally.

This check uses only successful actor selections from the current import, not
cached accepted refs or unvalidated advertisements. Missing remote actors are
still published; newer local histories still fast-forward. Divergent histories
still fail publication rather than being force-pushed or silently ignored.
Proposal and memory publication behavior is unchanged.

For each selected actor, Hubnot requests exactly:

```text
+refs/hn/actors/<full-actor>:refs/hn/quarantine/actors/<full-actor>
```

For each selected candidate, it requests exactly:

```text
+refs/hn/proposals/<candidate-hash>:refs/hn/quarantine/proposals/<candidate-hash>
```

For each selected memory stream, it requests exactly the one advertised owner:

```text
+refs/hn/memory/<full-actor>/<stream-hash>:refs/hn/quarantine/memory/<full-actor>/<stream-hash>
```

The destination refs live inside a generated separate bare repository, not the
main object database. The transaction then:

1. reads ordinary Git ref advertisement and resolves only selected refs;
2. exact-fetches each advertised selection into quarantine;
3. confirms the fetched root equals the advertised object ID;
4. measures the selected graph against every configured budget;
5. validates root/object types, exact event or two-file memory tree shape,
   signatures, actor fingerprints, sequence and parent chains, signed
   attachments, candidate ref/head bindings, identity relations, and all exact
   event, memory lifecycle, anchor, policy, pipeline, evidence, and merge
   relationships;
6. classifies valid, invalid, over-budget, and missing-dependency selections
   independently, then propagates failure only to selections that depend on a
   failed selection;
7. writes a private pending-acceptance anchor and validated transaction
   receipt before copying objects;
8. copies required promotable objects with ordinary Git pack plumbing and
   verifies their presence in the main object database;
9. removes quarantine, then updates all promotable accepted refs in one atomic
   `git update-ref` transaction;
10. releases only the verified shallow boundaries involved, records a complete
    receipt, and removes the pending anchor.

Accepted roots are:

```text
refs/hn/remotes/<remote>/actors/<full-actor>
refs/hn/remotes/<remote>/proposals/<candidate-hash>
refs/hn/remotes/<remote>/memory/<full-actor>/<stream-hash>
```

`collectEvents` projects only local actor refs and these accepted remote refs.
Objects copied before a failed accepted-ref transaction may remain as
unreferenced residue, but they are not accepted roots and trust-sensitive
operations remain fail-closed. Durable anchors/receipts prevent an interrupted
copy from being mistaken for acceptance; retry reconciles exact transaction
state.

## Budget semantics

Budgets are evaluated per selected ref. Objects shared by two selections count
independently for both. Only objects reachable from that same selection's
previous accepted ref are discounted from object and byte measurements;
unrelated objects already present locally cannot weaken a later check.

| Budget | Measurement |
| --- | --- |
| `max-events` | Number of commits/events in the selected actor or memory history |
| `max-objects` | Newly reachable Git objects beyond that selection's previous accepted value |
| `max-object-bytes` | Largest individual newly reachable Git object |
| `max-attachment-bytes` | Largest attachment in the selected actor history (`log.txt` today) |
| `max-total-bytes` | Sum of newly reachable Git object sizes |

All comparisons are inclusive: if a measured value is `N`, a configured limit
of `N-1` rejects, `N` accepts, and `N+1` accepts. A rejected selection reports
the full selected ID, budget name, configured value, and measured value, and
its accepted ref does not advance.

Standard Git may receive the selected pack before Hubnot can measure its
contents. These are hard validation, promotion, accepted-projection, and
retention boundaries. They are not portable hard pre-download network, CPU,
memory, or disk quotas.

## Per-selection failure isolation

`hn sync` reports one outcome per full selected actor/candidate/stream ID. A
memory outcome has `kind=memory` and the full stream ID:

- `promoted` — selected data validated and its accepted ref committed;
- `over-budget` — at least one inclusive limit was exceeded;
- `structurally-invalid` — fetch/root/object/event-chain validation failed;
- `relationship-invalid` — an available referenced fact contradicts the
  signed relationship;
- `dependency-missing` — an exact required actor, event, candidate, code ref,
  or evidence fact was not selected/advertised.

A valid selection promotes independently only when its relationship closure
does not depend on a failed selection. A failed actor selection therefore
blocks a selected candidate or evidence chain that needs it, but not an
unrelated valid history.

Missing dependency is not treated as malformed data. Diagnostics include the
failing selection's full ID, referencing event's full ID and kind, missing full
ID when derivable, and an exact next action such as:

```text
add exact proposal selection --proposal <full-sha256-candidate-id>
select the full actor history supplying <full-sha256-event-id>
```

No similarly titled candidate, related device, predecessor/successor, or short
prefix is substituted.

## Publication is separate from import

After import, `hn sync [REMOTE]` uses ordinary explicit Git pushes to publish
all local `refs/hn/actors/*` histories, locally present
`refs/hn/proposals/*` candidate refs, and local `refs/hn/memory/*/*` streams.
Remote histories accepted under `refs/hn/remotes/*` are not republished as
local facts. The command does not push a primary branch:

```sh
hn sync origin
git push origin main:main
```

These are separate observable and retryable actions. Use exact ref targets for
manual staged publication; do not wildcard-push or force-rewrite immutable
collaboration facts.

## Depth-limited repositories

A shallow repository is allowed when every exact dependency required by an
operation already exists. Trust-sensitive operations explicitly probe their
candidate event, code ref/head, base commit, policy blob, pipeline definition,
run request/result, decision, merge ancestry, selected evidence, and actor
predecessors.

If an exact object/fact is absent beyond the shallow boundary, the command
fails with a durable diagnostic containing:

- original operation;
- dependency kind and full missing ID;
- owning full actor/candidate ID when derivable;
- remote and exact supplying ref;
- the exact selection/recovery action.

Recovery is explicit:

```sh
hn replication select origin \
  --actor <full-supplying-actor> \
  --proposal <full-supplying-candidate> \
  --memory <full-supplying-stream> \
  --max-events 10000 \
  --max-objects 100000 \
  --max-object-bytes 16777216 \
  --max-attachment-bytes 1048576 \
  --max-total-bytes 268435456
hn sync origin --recover-shallow
```

Recovery requires an explicitly saved exact selection; compatibility-all is
refused. It builds the smallest subset containing the recorded supplier, runs
that subset through the same quarantine/budgets/validation/atomic promotion,
checks that the saved selection bytes did not change, and replays the complete
original verification before clearing the gap. It never issues `--unshallow`,
never fetches all branch history, never silently adds a selector, and never
reuses a partial trust decision.

Calling `--recover-shallow` in a complete repository is an error. Retrying
after successful recovery is an idempotent accepted-projection verification.

## Private transaction state and threat boundary

Selections, generated quarantine repositories, transaction receipts, pending
anchors, unaccepted-object records, and shallow-gap records live below
`.git/hn/replication/` or `.git/hn/`. Directories require mode `0700`; state
files require mode `0600`; symlinks, unexpected entries, unknown JSON fields,
oversized records, missing matching anchors/receipts, and unsafe modes fail
closed.

The alpha does not claim that selection implies trust, that quotas stop bytes
before network receipt, that rejected unreachable residue is securely erased,
or that a remote cannot advertise hostile data. Its guarantee is narrower:
selected data is not an accepted projection root until bounded validation and
atomic promotion succeed. Memory recall consumes only local and promoted
accepted roots, never quarantine.
