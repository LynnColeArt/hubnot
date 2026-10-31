# Optional charter data example

`charter-model` is a standalone Go executable using only the public
`hn.extension/1` framed protocol. Its independent namespace is
`example.charter`, its model version is `example.charter-selection/1`, and
its initial declarations have schema version 1. This demonstrates data-model
feasibility. It does not claim production Spec Kitty schema/API compatibility.
It contains no resolver, service, composition, activation or execution engine.

Four example types are published through generic host methods:

| Type | Shape | Structured fields |
|---|---|---|
| `example.charter/directive` | document | `code`, `severity` (`must` or `should`) |
| `example.charter/tactic` | document | `code`, `goal` |
| `example.charter/pack` | list | `label` |
| `example.charter/charter` | ticket | `project`, `status` (`selected`) |

All fields are required; unknown fields refuse. Free text fields have a 64-byte
UTF-8 bound. Enum strings have their longest permitted spelling as their byte
bound (6 for severity, 8 for status). Directive schema version 2 additionally
requires `rationale`, also at most 64 bytes. Titles/content use the host's public
artifact limits; stored content is never interpolated into a shell or executed.
The pack orders member `z-directive` before `a-tactic`. Members and typed
relationships name exact signed revision IDs with `target_mode:"revision"` and
`required:true`. The ticket selects directive, tactic and pack independently.

The result distinguishes `generic_validation` (the host's signature/schema
checks) from `semantic_interpretation` (this example's three-link selection).
A generically valid ticket can fail the demonstration selection check. Neither
result grants policy, prompt or execution authority.

## Build and publish

Use a supplied `hn` executable and optional `charter-model` executable, or build
the latter from the repository root with `go build -o /absolute/bin/charter-model
./examples/extensions/charter-model`. No runtime SDK or other module is imported.
The commands below run in a new, ordinary Git repository. Replace executable
paths with actual absolute paths. Keep result/input/state files outside public
Git history. `HOME` and XDG configuration must be isolated per process when
running multiple actors in tests; never export `GIT_DIR` or `GIT_WORK_TREE` into
these application commands.

```sh
mkdir -p "$HOME/charter-example-private"
CHARTER_PRIVATE="$HOME/charter-example-private"
CHARTER_EXE=/absolute/bin/charter-model
git init --quiet
hn init --name 'Example author'
"$CHARTER_EXE" --manifest > "$CHARTER_PRIVATE/manifest.json"
printf '%s\n' '{"action":"publish"}' > "$CHARTER_PRIVATE/input.json"
hn extension register --manifest "$CHARTER_PRIVATE/manifest.json" \
  --executable "$CHARTER_EXE" \
  --arg --input --arg "$CHARTER_PRIVATE/input.json" \
  --arg --output --arg "$CHARTER_PRIVATE/result.json" \
  --arg --state --arg "$CHARTER_PRIVATE/selection.json" --json
hn extension enable example.charter/model \
  --grant native:identity=read \
  --grant artifact:example.charter/*=read,create,revise,type.publish --json
hn extension run example.charter/model --json
cat "$CHARTER_PRIVATE/result.json"
```

`hn extension run` prints the management outcome. The executable writes its
application result to the explicit `--output` file, using atomic replacement
and file sync. `--state` stores the eight exact public IDs and four original
payloads after a complete publication. The example uses fixed, descriptive
operation keys for one demonstration corpus per actor. To retry the same
publication, retain the same actor and exact input text; changed content under
the same keys refuses. This is not a connector recovery journal or a guarantee
about external systems. A failed run can already have published a prefix;
inspect `artifact.operation` before choosing a new intent. The example never
automatically retries a refused/uncertain request.

The input is a closed object. Actions are:

* `{"action":"publish","directive_text":"optional exact text"}`.
* `{"action":"inspect-selection"}` reads the exact public ticket, then its
  three exact targets with signed `inspect`, reconstructing stored text/order.
* `{"action":"evolve","parent":"sha256:FULL_CURRENT_DIRECTIVE_HEAD"}` publishes
  directive schema 2 and explicitly converts the supplied sole head. It requires
  a full exact parent and preserves the original state file and ticket selection.
* `{"action":"call","method":"artifact.read","params":{...}}` sends one raw
  public request and retains its complete success/refusal in `response`. This is
  used for explicit revision, resolution, operation lookup and generic queries.
  Supported methods are exactly the seven listed in `--manifest`; this does not
  enable native governance, transport, arbitrary commands or memory mutations.

Every command may additionally supply `expected_repository_id` and
`expected_registration_id`, copied from `hn extension inspect
example.charter/model --json`. The executable checks those pins against the
hello before any business request. It always validates module/version, complete
hello shape, session/digest grammar and all five advertised limits. There is one
outstanding request, with a fresh correlation ID per request. Input/state files
are capped at 1 MiB; frames at 8 MiB; JSON values at 64 nested levels. Duplicate
keys, unsafe/noninteger numbers, malformed Unicode and unknown command fields
refuse, with no silent truncation. These local consumer limits do not change
host limits. stdout is reserved for wire frames during a host session; diagnostics
go to private stderr and never include request/file contents.

The client consumes the full preceding response prefix and payload, checks its
exact request ID and closed envelope, then validates that method's complete typed
success data or closed error before another call. A malformed/truncated reply
stops publication; an explicit business refusal is preserved and handled as a
refusal. These transport checks do not verify signatures or evaluate schemas,
lineage, policy or example selection meaning. Those remain separate checks.
The host's bounded final-byte observation allowance does not permit this client
to pipeline requests, skip response validation or automatically retry.

## Explicit conflict and evolution

Create an independent actor B with its own initialized Git repository and its
own home/config. Add one bare remote to both repositories. Publish/fetch the
ticket first, then independently revise the same original directive on A and B
before exchanging the later streams. Use the `call` action and this full request
shape, replacing the exact IDs/actor/payload from `selection.json`:

```json
{"action":"call","method":"artifact.mutate","params":{
  "kind":"record.revise","record":"sha256:OPENING",
  "parents":["sha256:ORIGINAL_HEAD"],
  "payload":{"shape":"document","type_id":"example.charter/directive",
    "type_revision":"sha256:EXACT_DECLARATION","title":"Evidence integrity directive",
    "content":"An explicit full replacement", "fields":{"code":"EVIDENCE-01","severity":"must"},
    "members":[],"relationships":[],"conversion_sources":[]},
  "actor":"ACTUAL_64_HEX_ACTOR","operation":"explicit-edit-A"}}
```

Place the command in the same input file and run the registered module again.
Actor B uses its own actual actor and operation. Fetch both streams through the
remote. Select both `--id FULL_CHARTER_ID --id FULL_NEW_REVISION_ID` for B's push
and A's fetch: retaining the charter preserves the remote's established actor
prefix, and selecting the new revision explicitly carries that branch.
`hn artifact heads OPENING --json` must show two heads and `state:null`.
Do not pick a timestamp winner or retry a stale revise against a newer head.
Resolution is a new `record.resolve` request with a new operation key, a complete
chosen payload, and **every** current head in sorted `parents`. Add `resolve`
explicitly to the artifact grant by running `hn extension enable` again with
the complete grant set. A partial resolution is `artifact/stale_revision`;
revise on conflict is `artifact/resource_conflict`; absent resolve grant is
`host/denied_access`. All refuse before signing/publication. The acceptance test
also verifies a stale parent after successful resolution.

After resolution, put the exact resolved head in the `evolve` command above.
The result reports `new_type` and `converted`. `hn artifact show OPENING --json`
now follows the converted head; `inspect-selection` still returns the old
selected directive text/schema and explicit pack order. There is no implicit
charter update. An intentional later ticket revision must be a separate full
`artifact.mutate` request with a new operation key and its exact expected head.

## Public reconstruction without the consumer

Use full IDs from the application result; abbreviated placeholders below are
instructions to substitute actual full IDs. Run in the appropriate repository.

```sh
git init --bare --quiet /absolute/public.git
git remote add public /absolute/public.git
hn artifact push --remote public --id FULL_CHARTER_ID --json
hn extension disable example.charter/model --json
hn extension remove example.charter/model --json
# The optional executable and its private input/output/state can now be removed.
```

In an independent initialized repository/home, without installing this module:

```sh
git init --quiet
hn init --name 'Independent reader'
git remote add public /absolute/public.git
hn artifact fetch --remote public --id FULL_CHARTER_ID --json
hn artifact inspect --id FULL_CHARTER_ID --json
hn artifact inspect --id FULL_OLD_DIRECTIVE_ID --json
hn artifact type show FULL_OLD_DECLARATION_ID --json
hn artifact history FULL_DIRECTIVE_OPENING_ID --json
hn artifact list --type example.charter/pack --shape list --limit 200 --json
hn artifact search --type example.charter/directive --literal historical --limit 200 --json
hn artifact search --type example.charter/directive --regex 'Converted.*guidance' --limit 200 --json
hn artifact export --id FULL_CHARTER_ID --output /absolute/public-bundle.json --json
hn issue open 'Ordinary native collaboration still works'
```

Selected artifact push/fetch carries custom refs; ordinary `git clone` alone
does not promise them. `inspect` returns a complete signed closure: locate the
item whose `id` is the requested exact revision. `show`, `heads`, `list` and
`search` report current logical heads. A search match is data, not authority.
No private identity/registration/grant/journal file is required at the destination.
After evolution, add `--id FULL_CONVERTED_DIRECTIVE_ID` to **both** push/fetch and
to export if the recipient should also reconstruct current converted data. The
charter's exact old selection alone requires only its historical closure; it
does not silently select later revisions. The acceptance bundle explicitly
selects both the original charter and converted directive.

The owned root acceptance test runs this actual executable against real host
processes. It verifies every example field bound, exact signature identities,
two-actor conflict/resolution, conversion and old-selection persistence, bare
remote reconstruction, generic access after executable removal, native issue
and memory use, inert executable-looking text and decoded-object leak controls.
Its missing-type control removes the declaration actor's ref/objects from a
valid independent dependent stream and requires `incomplete_dependency` with a
schema cause, with no accepted/private-state change. This is separate from
arbitrary Git corruption. Final platform/performance qualification remains a
mission-level responsibility.
