# Inventory model example

This optional standalone Go executable publishes an inventory item document, an
ordered stock list, and a maintenance ticket through `hn.extension/1`. Hubnot
stores generic signed data. The executable owns these example schemas and their
inventory meaning; it has no Hubnot/internal, charter, service, or SDK dependency.
It is a storage and collaboration example, not an inventory management system.

Build from the repository root:

```sh
go build -o /absolute/private/inventory-model ./examples/extensions/inventory-model
```

Initialize the repository/identity with ordinary `hn init`. Create this explicit
local manifest (paths and local command/result files are operator choices):

```json
{"protocol":"hn.extension-manifest/1","module_id":"example.inventory/publisher","module_version":"1","host_protocol":"hn.extension/1","capabilities":["identity.public","artifact.type.publish","artifact.mutate"]}
```

Register the executable and grant only this repository's inventory namespace:

```sh
hn extension register --manifest /absolute/private/manifest.json \
  --executable /absolute/private/inventory-model \
  --arg --input --arg /absolute/private/command.json \
  --arg --output --arg /absolute/private/result.json --json
hn extension enable example.inventory/publisher \
  --grant native:identity=read \
  --grant artifact:example.inventory/*=create,revise,type.publish --json
hn extension run example.inventory/publisher --json
```

Each explicit run reads one command, validates the bound host hello, sends ready,
reads `identity.public`, and sends one inventory request only after fully
validating a successful initialized identity response. A terminal response or
identity error stops that sequence. Host stdout is the management summary. The complete business
response is in the explicit private result file, mode 0600. A host business refusal
is preserved there with its exact origin/code/recovery and completes the session;
transport, input, or malformed/uninitialized identity responses terminate the
child; valid business errors are preserved without another request. No background discovery,
automatic identity initialization, latest-head retry, resolution or migration runs.

First put this in command.json, then run the extension:

```json
{"action":"type","model":"item","schema_version":1,"parents":[],"operation":"item-type-v1"}
```

Copy the returned full `data.event_id` into `type_revision` in the next command:

```json
{"action":"open","model":"item","operation":"pump-open","record":null,"parents":[],"type_revision":"sha256:REPLACE_WITH_EXACT_TYPE_ID","title":"Pump 42","content":"Exact maintenance instructions\n雪","fields":{"sku":"PUMP-0042","quantity":7,"location":"North shelf"},"members":[],"relationships":[],"conversion_sources":[]}
```

The independently defined types are:

| Model | Public type | Shape | Version 1 required fields |
|---|---|---|---|
| `item` | `example.inventory/item` | document | `sku` string, `quantity` nonnegative integer, `location` string |
| `stock` | `example.inventory/stock` | list | `warehouse` string, `counted` boolean |
| `maintenance` | `example.inventory/maintenance` | ticket | `priority` (`routine` or `urgent`), `completed` boolean |

All free string fields have a 64-byte UTF-8 bound; priority has a 7-byte bound
and the two-value enum. Quantity has minimum zero. Acceptance checks each bound
at its limit and immediately beyond it. Every fields object is closed. Version 2 adds the required string field `unit` to
each type. `type` publishes the complete selected version with explicit declaration
parents. It does not change existing records. `convert` must explicitly select the
new exact type revision and supply the full new fields, exact old sole head in
`parents`, and that same head in `conversion_sources`. All older signed records
retain their original exact schema, text, fields, order and relationships.

`open`, `revise`, `resolve` and `convert` use the full command state shown above.
`record` is null for open and the full opening ID otherwise. `revise` requires the
current sole head; `resolve` requires every current head in increasing ID order.
Concurrent edits by separately initialized actors remain visible conflicts.
Resolution additionally requires the operator to explicitly enable the `resolve`
action; the executable never upgrades grants. Reuse an operation key only for the
identical original intent; changed state/parents under the key must refuse. If a
result file is lost after publication, inspect the original public actor/key with
`hn artifact operation --type example.inventory/item --actor ACTUAL_ACTOR --operation ORIGINAL_KEY --json`.
Preserve the original command before retrying; an uncertain lookup does not prove
absence. This one-command example does not promise an application journal or
automatic crash recovery.

Lists alone have ordered `members`, for example:

```json
[{"member_id":"z-first","target":"sha256:EXACT_ITEM_REVISION","target_mode":"revision","required":true},{"member_id":"a-second","target":"sha256:EXACT_TICKET_REVISION","target_mode":"revision","required":true}]
```

Member IDs do not determine order. Typed `relationships` are generic links such as
`{"relation":"example.inventory/services","target":"sha256:OPENING_ID","target_mode":"logical","required":true}`.
`revision` pins an exact signed version; `logical` selects the target record's
current heads. Hubnot validates generic structure/closure, not the business meaning
of service work, unit names, stock counts or ticket completion.

The publisher consumes only `identity.public`, `artifact.type.publish`, and
`artifact.mutate`. It fully reads each response, checks exact envelope keys and
request correlation, and validates the complete method-specific identity or
custom mutation result before sending another call or writing application output.
It checks closed error origin/code/recovery values and preserves the original
operation tuple. A valid terminal error with `request_id:null` is recorded and
stops the session; malformed responses stop without becoming application results.
These transport checks do not grant business authority or verify artifact signatures.

Generic inspection/search are the plugin-free CLI's responsibility. Earlier
unvalidated read/search convenience commands were removed from this example before
approval. No generic SignedArtifact or schema client SDK is introduced. The
independent acceptance harness exercises host shape-search with handwritten frames.
Input/framing retain the frozen 8MiB limit, strict JSON/integer/Unicode rules,
exact correlation and one outstanding request.

Public data needs no consumer installation:

```sh
hn artifact push --remote public --id EXACT_RECORD_ID --json
# In a separately initialized repository, with its own HOME/config and identity:
hn artifact fetch --remote public --id EXACT_RECORD_ID --json
hn artifact inspect --id EXACT_REVISION_ID --json
hn artifact type show EXACT_TYPE_REVISION_ID --json
hn artifact history RECORD_OPENING_ID --json
hn artifact search --type example.inventory/item --literal Pump --limit 200 --json
hn artifact list --shape ticket --limit 200 --json
hn artifact search --type example.inventory/maintenance --regex 'Pump|maintenance' --limit 200 --json
hn artifact export --id EXACT_RECORD_ID --output public.json --json
hn extension remove example.inventory/publisher --json
```

Configure the same real bare remote explicitly in both repositories. Transfer
only public artifact histories; never copy source identity, registration, grant,
credential or journal files. Removing registration/binary preserves generic reads,
search, export, and ordinary native ticket and memory use. Executable-looking
stored text and schema values remain inert data and grant no execution or policy
authority. The host is mediation, not a sandbox for a malicious same-user process.

Focused reproducible acceptance uses the actual standalone executable and one
explicitly supplied host (use the same source for standard and combined native
builds):

```sh
HN_WP22_BINARY=/absolute/hn HN_WP22_CONSUMER=/absolute/inventory-model \
  go test -count=1 -v -run '^TestInventory' .
go test -count=1 ./examples/extensions/inventory-model
```

Acceptance covers all three shapes/two actors, exact-cause before-effect refusals,
explicit conversion, required schema omission, selected public exchange to an
independent plugin-free clone, inert/leak detector controls, and removal. Combined
charter/inventory corpus qualification uses an independently approved public
charter exchange embedded byte-for-byte as inert test data (original producer
`4eb5e574`, public bundle SHA256
`24f344ba260b8a75b28e8c7a7cdcc5a083ab3a1f027e53b9872beda8af95d954`).
The companion original IDs, payload/order/link expectations and generic queries
are retained before and after actual publisher removal; no charter code or
private application state is imported. Final package/default/performance qualification belongs to the
mission's qualification work packages.
