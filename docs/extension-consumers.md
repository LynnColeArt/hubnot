# Authoring optional extension consumers

Use [the host contract](extensions.md) for lifecycle, grants, fourteen methods,
framing and errors, and [portable artifacts](artifacts-v1.md) for generic storage.
These source examples are separate optional executables, not installed services,
SDKs, production connectors or public release assets. Native Hubnot users need
only `hn` and Git; building an example from a source checkout additionally needs
the checkout's Go toolchain. There are no sample-asset download links implied here.

## Choose the smallest sample

| Sample | Methods it actually declares/validates | Initial grants |
|---|---|---|
| [fake-tracker](../examples/extensions/fake-tracker/README.md) (`example.tracker/tool`) | identity.public, resource.read/mutate/operation, changes.capture/page/since, capabilities.list | native:identity=read; native:issue=read,observe,create,revise |
| [charter-model](../examples/extensions/charter-model/README.md) (`example.charter/model`) | identity.public, artifact.type.publish/type.read, artifact.mutate/read/search/operation | native:identity=read; artifact:example.charter/*=read,create,revise,type.publish |
| [inventory-model](../examples/extensions/inventory-model/README.md) (`example.inventory/publisher`) | identity.public, artifact.type.publish, artifact.mutate | native:identity=read; artifact:example.inventory/*=create,revise,type.publish |

The tracker handles only native issue views; the inventory publisher has no
read/search/operation client. Its generic inspection/recovery uses `hn artifact`.
These are deliberate sample subsets, not the broader host's capability inventory.
No sample initially has resolve authority; adding resolve requires an explicit
complete grant replacement. Granting a method does not teach a consumer to validate
its response. Do not broaden the client without implementing its closed result
validation and stop-on-fault behavior.

## Inventory publication and plugin-free reading

Use supplied executables built from the same source, or optionally build the
sample **from the source checkout root**:

```sh
go build -buildvcs=false -o /absolute/private/inventory-model ./examples/extensions/inventory-model
```

Run the following in a fresh ordinary Git repository with `hn` on PATH. Choose an
actual absolute path for INVENTORY_EXE first. Application files stay private and
outside public Git. This example's shell, Git and supplied executables do not need
Python, jq, a provider, account, service or Go at runtime.

```sh
INVENTORY_EXE=/absolute/private/inventory-model
PRIVATE=$(mktemp -d)
chmod 700 "$PRIVATE"
umask 077
git init --quiet
hn init --name 'Inventory author'
cat > "$PRIVATE/manifest.json" <<'JSON'
{"protocol":"hn.extension-manifest/1","module_id":"example.inventory/publisher","module_version":"1","host_protocol":"hn.extension/1","capabilities":["identity.public","artifact.type.publish","artifact.mutate"]}
JSON
hn extension register --manifest "$PRIVATE/manifest.json" \
  --executable "$INVENTORY_EXE" \
  --arg --input --arg "$PRIVATE/command.json" \
  --arg --output --arg "$PRIVATE/result.json" --json
hn extension inspect example.inventory/publisher --json
hn extension enable example.inventory/publisher \
  --grant native:identity=read \
  --grant artifact:example.inventory/*=create,revise,type.publish --json
cat > "$PRIVATE/command.json" <<'JSON'
{"action":"type","model":"item","schema_version":1,"parents":[],"operation":"guide-item-type-v1"}
JSON
hn extension run example.inventory/publisher --json
cat "$PRIVATE/result.json"
```

The parent prints session management, while result.json holds the business
response. Continue only if the business response has ok=true. Copy its full
`data.event_id` into TYPE_ID at the prompt; do not paste a guessed/abbreviated ID:

```sh
printf 'Paste the actual type data.event_id: '
read -r TYPE_ID
cat > "$PRIVATE/command.json" <<JSON
{"action":"open","model":"item","operation":"guide-pump-open","record":null,"parents":[],"type_revision":"$TYPE_ID","title":"Pump 42","content":"Exact maintenance text\n雪","fields":{"sku":"PUMP-0042","quantity":7,"location":"North shelf"},"members":[],"relationships":[],"conversion_sources":[]}
JSON
cp "$PRIVATE/command.json" "$PRIVATE/original-open.json"
hn extension run example.inventory/publisher --json
cat "$PRIVATE/result.json"
hn identity show --json
```

Continue only after business success. Copy the actual opening `data.resource_id`
and the public identity response's `data.actor` into the next prompts. Keep the
original request and actor/key binding for recovery; never copy private keys:

```sh
printf 'Paste the actual opening data.resource_id: '
read -r RECORD_ID
printf 'Paste the actual public data.actor: '
read -r ACTOR
hn artifact type show "$TYPE_ID" --json
hn artifact show "$RECORD_ID" --json
hn artifact heads "$RECORD_ID" --json
hn artifact history "$RECORD_ID" --json
hn artifact inspect --id "$RECORD_ID" --json
hn artifact list --type example.inventory/item --shape document --limit 200 --json
hn artifact search --type example.inventory/item --literal Pump --limit 200 --json
hn artifact search --type example.inventory/item --regex 'Pump|maintenance' --limit 200 --json
hn artifact operation --type example.inventory/item --actor "$ACTOR" \
  --operation guide-pump-open --json
hn artifact export --id "$RECORD_ID" --output "$PRIVATE/public.json" --json
```

CLI search has no --shape; shape selection belongs to list (and separately to host
artifact.search). Exact historical `inspect` returns closure items, while show and
search display current heads. Do not interpret a search match as authority.

To demonstrate lost application acknowledgement without a new intent, preserve
the original command, discard only the application result, perform the lookup
above, then explicitly replay the identical command:

```sh
rm "$PRIVATE/result.json"
cp "$PRIVATE/original-open.json" "$PRIVATE/command.json"
hn extension run example.inventory/publisher --json
cat "$PRIVATE/result.json"
```

A found original operation permits comparison/exact replay; event/resource IDs
remain original and replayed=true. This controlled deletion is **application
result loss**, not a claim to reproduce a transport crash. A genuinely uncertain
lookup must stop; do not replay on an invented absence or change payload/parents
under the old key. The publisher has no durable recovery journal.

For independent public transfer, choose a new absolute remote directory. Retain
RECORD_ID (public) for the reader, and configure the same remote there:

```sh
PUBLIC_REMOTE="$PRIVATE/public.git"
git init --bare --quiet "$PUBLIC_REMOTE"
git remote add public "$PUBLIC_REMOTE"
hn artifact push --remote public --id "$RECORD_ID" --json
hn extension disable example.inventory/publisher --json
hn extension remove example.inventory/publisher --json
hn artifact show "$RECORD_ID" --json
```

In a **separately initialized repository**, with a different user config/home and
its own identity, set PUBLIC_REMOTE and RECORD_ID to those actual public values.
Do not copy the source identity/registry/grants/journal. No publisher is installed:

```sh
git init --quiet
hn init --name 'Independent reader'
git remote add public "$PUBLIC_REMOTE"
hn artifact fetch --remote public --id "$RECORD_ID" --json
hn artifact inspect --id "$RECORD_ID" --json
hn artifact history "$RECORD_ID" --json
hn artifact search --type example.inventory/item --literal Pump --limit 200 --json
hn issue open 'Native collaboration after optional publisher removal'
```

Selected fetch carries required public closure; plain clone alone does not promise
custom refs. File import is an alternative: `hn artifact import --input
/absolute/public.json --json`. Public data remains usable after module removal.

The [full inventory sample](../examples/extensions/inventory-model/README.md)
also covers document item, ordered stock list and maintenance ticket schemas,
explicit schema-v2 conversion, two-actor conflicts and full-head resolution.
Exact old text/schema/order/links survive evolution. A new declaration or new
current head never silently changes an old pinned selection.

## Charter data without doctrine activation

The [charter sample's copyable recipe](../examples/extensions/charter-model/README.md#build-and-publish)
publishes directive/tactic documents, an ordered pack and a ticket selecting exact
signed revisions. Supply its optional executable, invoke `--manifest` to emit inert
JSON, register fixed --input/--output/--state paths, explicitly enable the seven
methods' namespace grants, then run. Inputs are publish, inspect-selection, evolve
with an exact parent, or call with one of its seven declared methods.

The pack deliberately orders z-directive before a-tactic. Selection reconstruction
reads exact historical IDs, not latest names. Its semantic_interpretation is a
small demonstration check, separate from generic signature/schema validation;
neither is project/prompt/execution authority. It is not Spec Kitty schema/API
conformance, a resolver, composition engine, activation engine or runtime.

For conflict, two independently initialized actors revise the same exact original
parent before exchange. Select both the established charter anchor and new branch
when transferring the branch; inspect all heads. Resolve with a new operation key,
full chosen payload, **every** sorted current parent and explicit resolve grant.
Partial resolution is artifact/stale_revision, revise-on-conflict is
artifact/resource_conflict, absent resolve grant is host/denied_access, all with
do_not_retry. Evolve explicitly publishes schema 2 and converts the selected sole
head; the original ticket remains pinned to old schema/content. Transfer/export
must additionally select the converted revision when the reader should have it.

## Recovery and observation recipes

The [fake tracker](../examples/extensions/fake-tracker/README.md#explicit-actions-and-recovery)
is a one-issue/one-comment experiment with a separately started loopback HTTP fake
service and private credential/config/plan/journal/report. That service is an
optional experiment dependency, never a native dependency. External authors, times,
source metadata and comment markers are untrusted descriptive claims; only the
connector's own successful import receipt establishes its private association.

| Trigger | Explicit recipe and result |
|---|---|
| Initial import | Save full external snapshot, actual actor/public key, original issue/comment request content/parents and durable keys before native effects. Write plan `{"action":"import"}` and run the registered tracker. |
| Real native acknowledgement loss | The sample's `pause:"native-sent:comment"` fault control pauses after sending the real request and before consuming its reply. Its acceptance harness observes publication, terminates that owned consumer and reaps the parent. Restart with the same private binding/journal and `{"action":"import"}`: lookup the original actor/key first; found permits exact replay and no duplicate comment. Do not kill arbitrary PIDs or infer nonpublication from disconnect. |
| Explicit later external version | Plan `{"action":"update"}`. Any pending original update settles before a fresh source read/key/version choice. Preserve its saved parent/content even after newer source/native edits. |
| Consumed uncertain/refused lookup | Correlated inspect_operation/retry_identical stays pending with the original response/intent. Next invocation queries it first: found permits exact replay; verified_absent permits only the identical saved request; uncertain or lookup refusal stops before new source reads/effects. A definitive do_not_retry mutation refusal remains a stop. |
| Native/external conflict | Native stale_revision or external_conflict: retain failed original intent; inspect and choose a new explicit intent. No automatic parent refresh, overwrite, new key or resolution. |
| Lost external write reply | Persist exact external write before POST. The fake service commits receipt before dropping the connection; restart queries its own receipt and compares full original request/result. Missing receipt is external_uncertain, not absence or permission to POST again. Arbitrary providers need their own reconciliation; no global exactly-once promise. |
| Fresh replacement | Plan `{"action":"observe","fresh":true,"limit":1}`. Stage every page and reconcile external receipts before atomically replacing the exact selected set/checkpoint. Never commit partial staging or infer global order from IDs/times. |
| Resume / invalidation | Plan `{"action":"observe"}` uses the completed checkpoint. Captures/cursors cannot cross sessions; checkpoint_invalidated/rescan_required retains old committed set/checkpoint. Operator explicitly chooses fresh:true, discards partial staging, then replaces the whole selection only on completion. Duplicates dedupe by exact revisions. |
| Native admission | A granted native request still passes actual policy. Pending creation can return admission=pending with no visible heads; operation lookup can be uncertain/visibility_restricted. Native permission_denied/admission_refused is do_not_retry and produces no permitted publication. Do not turn pending/refused into success or bypass visibility through raw histories. |

These fault demonstrations are for a controlled disposable repository. Follow the
sample's private marker/continuation instructions, retain original input and wait
for owned processes to exit before restart. The host does not expose private
checkpoint/key/journal internals as business parameters. Raw diagnostic calls still
undergo declaration, binding, epoch, exact scope and grant checks.

## Representative observed refusals

The direct guide replay preserved the original actor/key/content, then changed
only content under the same key. The business response was:

```json
{"protocol":"hn.extension/1","request_id":"inventory","ok":false,"data":null,"error":{"origin":"artifact","code":"operation_conflict","message":"private operation key binds a different immutable request","recovery":"do_not_retry","operation":null}}
```

An open without create authority returned this error object:

```json
{"origin":"host","code":"denied_access","message":"action is not granted","recovery":"do_not_retry","operation":null}
```

Both controls left all public refs/Git objects and protected signing/intent files
unchanged. A separate wrong-schema control returned artifact/invalid_input,
message `integer below minimum`, likewise before effects. The real tracker paging
invalidation replay retained this error, then required an explicit fresh capture:

```json
{"origin":"host","code":"checkpoint_invalidated","message":"capture or checkpoint is unavailable or no longer verifiable; discard incomplete staging and capture the exact selection again","recovery":"rescan_required","operation":null}
```

These observed values belong to the bound source below. Do not assume all refusal
messages are identical across contexts, collapse origins, or equate a successful
management summary with a successful business request.

## Compatibility and evidence scope

This guide was qualified on Linux AMD64 with Go 1.26.4 and Git 2.43.0 against source
`6b2fe2e2d55e30ebc28bfc1cb6a4d9f7824508a5`, tree
`f38de81c3a657a18a159a1a9aaf11770974c7038`. All 311 Go/module inputs match the approved
three-consumer union (tracker `63e9a537`, charter `23983b5b`, inventory `7bee433c`).
The following standard supplied executables used `-buildvcs=false`; input maps,
exact commands/transcripts, guide digests and refused-effect comparisons are
retained in the WP23 qualification evidence. These are qualification assets, not
a published release or download location.

| Executable | SHA-256 |
|---|---|
| hn | c66b6a09446731dd98869c4e785c4c92347ef58cff8c4b058fa5d9fc7a16c7ad |
| charter-model | 5cb8348715302dd4a5128ce5ecfa6c427abd438d6d0d8f3a4c94f5bace240204 |
| inventory-model | 674540df9230ea5f6c084674d651734c3b444ac30a2dcd423ebe6b64f3c8a317 |
| fake-tracker | ccdd4f3248ad03953fdca90f393f36b382fa573855e13dcfff8d378e05cf73fa |

The supplied-binary acceptance replay passed CharterStandaloneJourney,
InventoryStandalonePublicJourney, FakeTrackerStandaloneRecovery,
FakeTrackerReplacementDisconnectAndInvalidation, FakeTrackerConsumedUncertaintyRecovery
and NativePendingAndPolicyRefusalCLI (all with their TestExtension/TestInventory
prefixes in source). This covers actual separate consumers, native admission,
conflict/resolve/conversion, public reconstruction after removal, lost native and
external acknowledgement, atomic paging/restart and intended-cause no-effect
refusals. ConsumedUncertaintyRecovery separately uses controlled handwritten host
responses to check the consumer's stop/restart decisions; it is not evidence that
the integrated host produced every simulated outcome. The direct guide replay
additionally checks generated-ID CLI operations and exact refused-effect controls.

Unchanged complex host mechanisms and bounds retain their independently approved
source-bound WP02–WP22 evidence; documentation does not create a new platform pass.
Final **all eight** Linux/macOS × AMD64/ARM64 × standard/native-only package rows,
paired default performance, cold native-user/extension-author trials and final
independent acceptance remain pending their qualification work packages. No human
adoption, repaired-Mac, beta acceptance or release-publication claim follows here.

## Author checklist

Read the complete prior response and validate closed envelope, correlation and
method-specific success/error shape before another call. A terminal null-ID or
malformed response ends the session; do not write subsequent requests. Keep
signature/schema/history verification distinct from consumer business checks.
Bind actual repository/registration and initialized signer; preserve actor/public
key across recovery and stop when binding changes. Keep credentials, journals,
application reports, signing roots and raw stderr outside public exchange. Trust
the executable as a same-user process: host grants are not a sandbox. No stored
content or namespace becomes policy, prompt or execution authority.
