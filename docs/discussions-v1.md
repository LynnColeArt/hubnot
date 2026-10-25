# Native discussions and optional GitHub transfer

Hubnot discussion threads and replies are signed Git resources. They work offline
without GitHub, `gh`, a service or a runner, including in a `nogithub` build.
Threads contain descriptive categories and open/closed state; replies have their
own revisions. An accepted answer names an exact reply revision, so later edits
cannot silently change the accepted text. Imported categories, locks and source
authors grant no moderation or policy authority.

See the [native resource contract](resources-v1.md) for signed event admission,
common envelopes/limits and Git transport, and the
[transfer guide](github-interoperability.md) for optional provider configuration.
Older alpha clients reject the new resource kinds; upgrade clients before
selecting actor histories that contain them.

## Thread and reply lifecycle

Replace the uppercase ID placeholders below with full `sha256:<64-hex>` IDs from
JSON command output. Opening output contains `resource_id` and `event_id` in
`data`; replies return their own independent resource IDs.

```sh
hn discussion open --title "How should imports work?" --body "Design notes" --category design --operation import-design --json
hn discussion revise THREAD_ID --expect THREAD_REV --title "Import design" --body "Updated notes" --json
hn discussion category THREAD_ID --expect THREAD_REV --category decisions --json
hn discussion reply add THREAD_ID --body "Keep explicit source provenance" --operation first-reply --json
hn discussion reply add THREAD_ID --reply-to REPLY_ID --body "Agreed" --json
hn discussion reply edit REPLY_ID --expect REPLY_REV --body "Revised proposal" --json
hn discussion answer THREAD_ID --expect THREAD_REV --reply EXACT_REPLY_REV --json
hn discussion answer THREAD_ID --expect THREAD_REV --clear --json
hn discussion close THREAD_ID --expect THREAD_REV --json
hn discussion reopen THREAD_ID --expect THREAD_REV --json
```

`--reply-to` names a reply **opening**, while `answer --reply` names an exact reply
opening/revision. Both must belong to the thread. Native reply nesting is bounded
to eight levels and cannot form cycles. A reply's thread and parent are immutable
across revisions. Changing answer/category/lifecycle appends a signed thread
revision; it does not rewrite earlier content.

All writes accept `--actor ACTOR`, `--operation KEY` and `--json`. An explicit
actor must match the selected native signer. Operation keys are actor-global,
1..128 printable non-whitespace ASCII bytes; a byte-identical retry returns the
original result even after later revisions. Reusing a key for different input or
a different family fails.

## Full-state input and conflicts

Use full state for metadata edits or conflict resolution. A thread state file:

```json
{
  "schema":"hn.discussion/1",
  "title":"Import design",
  "body":"Keep native facts portable",
  "category":"design",
  "status":"open",
  "metadata":{"team/context":"architecture"}
}
```

`accepted_reply_revision` is optional and, when present, is a full reply event ID.
A reply state file has `schema:"hn.discussion-reply/1"`, `discussion_id`, optional
`reply_to`, nonempty `body`, and optional namespaced `metadata`.

```sh
hn discussion open --input thread.json --json
hn discussion revise THREAD_ID --expect THREAD_REV --input thread.json --json
hn discussion resolve THREAD_ID --expect HEAD_A --expect HEAD_B --input thread.json --json
hn discussion-reply open --input reply.json --json
hn discussion-reply revise REPLY_ID --expect REPLY_REV --input reply.json --json
hn discussion-reply resolve REPLY_ID --expect HEAD_A --expect HEAD_B --input reply.json --json
```

Ordinary revise requires the sole current head. Resolve requires all current
heads; no actor or timestamp implicitly wins. Conflicted views expose every head
with `state:null`, and mutation refuses until explicitly resolved. Strict JSON
rejects unknown fields, duplicate keys, malformed Unicode and invalid references.
Title/body limits are 1024/65536 UTF-8 bytes, category is a nonempty identifier up
to 256 bytes, and each full authenticated event must fit 256 KiB. Shared metadata
bounds are 64 namespaced string keys, each value at most 4096 bytes.

## Read signed discussion state

```sh
hn discussion list --limit 50 --json
hn discussion show THREAD_ID --json
hn discussion replies THREAD_ID --limit 50 --json
hn discussion heads THREAD_ID --json
hn discussion history THREAD_ID --limit 50 --json
hn discussion-reply show REPLY_ID --json
hn discussion-reply history REPLY_ID --limit 50 --json
hn discussion operation --actor ACTOR --operation import-design --json
hn discussion-reply operation --actor ACTOR --operation first-reply --json
```

`show` includes `reply_count` and the pinned `accepted_answer` state/actor/revision
when available; it does not accept pagination. Use `replies` for independent
reply pages. Other bounded reads use `--limit 1..200` and opaque `--cursor TOKEN`;
changed accepted refs invalidate the cursor. Read selectors accept unambiguous
prefixes of at least eight hex digits. JSON responses contain `ok` and `data`, or
an `error` code/message on failure; failures also use nonzero exit status.

Reads need no private key or provider credential. Accepted signed actor histories
contain the thread/reply/answer dependencies required to reconstruct a discussion
in another clone. Missing dependencies quarantine incoming history rather than
publishing incomplete accepted state. A local adapter journal is not needed to
read the imported native discussion after Git replication.

## Optional GitHub Discussions

GitHub discussions use repository selectors and numeric lookup keys; native IDs
remain separate. GraphQL requires `HN_GITHUB_TOKEN` even for public reads.

```sh
hn transfer capabilities --provider github --resource discussion
hn transfer import --provider github --resource discussion --project OWNER/REPO --key 7 --map history=snapshot --dry-run
hn transfer import --provider github --resource discussion --project OWNER/REPO --key 7 --map history=snapshot
hn transfer export THREAD_ID --provider github --resource discussion --project OWNER/REPO --map history=snapshot --map category=CATEGORY_NODE_ID --dry-run
hn transfer export THREAD_ID --provider github --resource discussion --project OWNER/REPO --map history=snapshot --map category=CATEGORY_NODE_ID
```

Omit `--key` to collect the bounded repository discussion listing and complete
selected threads. Comments and replies are independently paged before native
application; inaccessible or contradictory references refuse incomplete import.

| Mapping or feature | Boundary |
| --- | --- |
| `history=snapshot` | Required for import and export; acknowledges present-day text snapshots rather than complete edit/deletion history |
| `category=CATEGORY_NODE_ID` | Required for export; existing accessible destination category, answerable if an accepted answer is selected; category creation is unsupported |
| `losses=acknowledge` | Required when observed lock/moderation, poll, votes/reactions, or minimized/deleted reply semantics cannot be represented as native behavior; claims remain descriptive |
| Title/body, source author/time/IDs, category, open/closed state | Supported in the explicit subset; local signer and authenticated GitHub writer remain distinct |
| Thread comments and one level of replies | Supported, topologically published as separate mutations |
| Deeper native nesting | Export refuses before writes; manually reconcile the structure |
| Accepted answer | Supported only when the pinned native answer revision is the current exported reply content; historical-content mismatch refuses before writes |
| Complete source edit/deletion history, polls/votes/reactions, locks/roles/permissions | No equivalence or imported moderation authority; explicit acknowledgment does not recreate them |

Unsupported or unused mapping keys fail. Use the dry-run and capability reports
before a transfer; acknowledgment records a deliberate representational loss and
never grants authority. Original source authors cannot be impersonated by normal
GitHub creation APIs, so exported native text includes visible attribution.

Verified same-source export keeps ordinary source bodies and updates mapped
replies without footer echo or duplicate roots. Source identity uses immutable
discussion and repository IDs, so a repository rename does not manufacture a new
copy. Returning to a genuinely different repository is an explicit copy. Missing
or inaccessible source identity refuses automatic copy selection. Unchanged
source replies deleted remotely are not resurrected; changed/missing associations
require explicit reconciliation. Exact native publications seen on re-import are
refused when verified against native signed history, even without a journal.

Real exports require the selected native signing identity as well as the GitHub
credential. The complete canonical private checkpoint is signed with domain
separation; resume with the original exporter identity. Identity changes, unsigned
or noncanonical journals, changed scope and manufactured progress/bindings refuse
before writes. Dry-run can verify a public journal signature without private keys.
This signature protects recovery state; it does not prove remote success or grant
provider permissions.

Transfer graphs are limited to 256 nodes and 8 MiB of serialized data, including
each selected thread and reply. Oversized graphs fail rather than truncate.
The whole future checkpoint must fit 8 MiB before the first remote mutation.
Every root/reply/content/answer/state write is separately journaled. A retry
recovers the frozen native graph before considering newer edits, and an uncertain
write needs exact positive remote reconciliation. A negative scan never permits
blind creation retry. Multiple publishers and remote concurrent edits cannot be
made atomic by this adapter; conflicts and ambiguous associations refuse rather
than overwrite. Preserve private recovery files for explicit reconciliation.
