# Try an optional plugin

Most Hubnot users need only `hn` and Git. Issues, projects, discussions, workflows,
memory and migration work without plugins. Hubnot calls plugins **extensions**:
trusted local executables you explicitly register, grant permissions and run.
Cloning a repository or reading its data never installs or starts one.

This walkthrough creates a custom maintenance ticket with the inventory example,
then removes the extension and reads the ticket without it. It uses a disposable
repository and isolated user configuration, leaving your ordinary projects and
extension registrations alone. Run the shell blocks in order in one terminal.
Stop at a failed command and inspect the error before continuing.

## 1. Choose the executable

Install [Hubnot 0.1.0-beta.2](../README.md#download-and-install) for your platform
and put `hn` on PATH. You need Git and an ordinary Linux/macOS shell. The example
publisher is separate from `hn`; use a trusted copy built for your native CPU
from the matching Hubnot source. Building it yourself additionally needs Go
1.26 or newer. Go is not an application or plugin runtime requirement.

Start in the root of the matching `v0.1.0-beta.2` source checkout:

```sh
umask 077
DEMO=$(mktemp -d "$HOME/hubnot-plugin-demo.XXXXXX")
HN=$(command -v hn)
"$HN" version
CGO_ENABLED=0 go build -buildvcs=false -trimpath \
  -o "$DEMO/inventory-model" ./examples/extensions/inventory-model
```

The version should be `0.1.0-beta.2`. If using an already supplied matching
publisher, copy it to `$DEMO/inventory-model` instead of running the build and
ensure it is executable. The [qualification kit](releases/0.1.0-beta.2.md#assets-and-historical-handoff-records)
contains reference copies for demonstrations/tests, not production connectors.
Only run an executable you trust: extensions have the same OS-user powers as you.

## 2. Create the isolated demo

```sh
mkdir -p "$DEMO/repository" "$DEMO/private" "$DEMO/config" \
  "$DEMO/home/Library/Application Support"
git init --quiet "$DEMO/repository"
cd "$DEMO/repository"
demo_hn() {
  env HOME="$DEMO/home" XDG_CONFIG_HOME="$DEMO/config" "$HN" "$@"
}
demo_hn init --name 'Plugin demo'
```

Use `demo_hn` throughout this walkthrough. Its temporary HOME isolates the Mac
registry too; macOS does not use XDG_CONFIG_HOME for `os.UserConfigDir`.
Normal `hn` calls keep your normal configuration. The demo repository contains a
new private signing identity; keep the whole demo private.

## 3. Register, inspect and grant

The manifest lists the methods the publisher understands. It does not grant them.

```sh
cat > "$DEMO/private/manifest.json" <<'JSON'
{"protocol":"hn.extension-manifest/1","module_id":"example.inventory/publisher","module_version":"1","host_protocol":"hn.extension/1","capabilities":["identity.public","artifact.type.publish","artifact.mutate"]}
JSON
demo_hn extension register --manifest "$DEMO/private/manifest.json" \
  --executable "$DEMO/inventory-model" \
  --arg --input --arg "$DEMO/private/command.json" \
  --arg --output --arg "$DEMO/private/result.json" --json
demo_hn extension inspect example.inventory/publisher --json
demo_hn extension enable example.inventory/publisher \
  --grant native:identity=read \
  --grant 'artifact:example.inventory/*=create,type.publish' --json
```

Registration records the executable's path, bytes and arguments. Enablement
grants public identity read and type publication/creation in the example's
artifact namespace for this repository. It grants no edits, conflict resolution,
policy changes or runner access. Neither command starts the publisher.

## 4. Publish a type, then a ticket

First declare the sample's maintenance-ticket type:

```sh
cat > "$DEMO/private/command.json" <<'JSON'
{"action":"type","model":"maintenance","schema_version":1,"parents":[],"operation":"quickstart-maintenance-type-v1"}
JSON
demo_hn extension run example.inventory/publisher --json && \
  cat "$DEMO/private/result.json"
```

**Stop if the command fails or the private result has `ok:false`.** The host's
management summary can succeed while the separate business request is refused.
Read `result.json` after every run; no JSON utility is needed. After business
success, copy its complete `data.event_id`, including `sha256:`, at the prompt:

```sh
printf 'Paste the type data.event_id: '
read -r TYPE_ID
cat > "$DEMO/private/command.json" <<JSON
{"action":"open","model":"maintenance","operation":"quickstart-maintenance-open","record":null,"parents":[],"type_revision":"$TYPE_ID","title":"Inspect pump 42","content":"Check vibration before the next shift.","fields":{"priority":"routine","completed":false},"members":[],"relationships":[],"conversion_sources":[]}
JSON
cp "$DEMO/private/command.json" "$DEMO/private/original-open.json"
demo_hn extension run example.inventory/publisher --json && \
  cat "$DEMO/private/result.json"
```

Again, continue only on business `ok:true`. Copy the complete returned
`data.resource_id` for generic inspection:

```sh
printf 'Paste the ticket data.resource_id: '
read -r TICKET_ID
demo_hn artifact show "$TICKET_ID" --json
demo_hn artifact list --type example.inventory/maintenance --shape ticket --limit 20 --json
demo_hn artifact search --type example.inventory/maintenance --literal 'pump 42' --limit 20 --json
```

This is a **custom artifact ticket**, not a new built-in `hn issue` command.
The same sample illustrates all three generic shapes:

| Sample model | Shape | Example fields |
| --- | --- | --- |
| `item` | Document | SKU, quantity, location |
| `stock` | Ordered list | Warehouse, counted flag, ordered members |
| `maintenance` | Ticket | Priority, completed flag |

See the [inventory example](../examples/extensions/inventory-model/README.md)
for the other two models and the [consumer guide](extension-consumers.md) for
longer publication, conversion and recovery recipes. Custom field meanings stay
with the consumer; Hubnot validates generic structure, schema and signatures.

## 5. Disable and remove; keep the data

```sh
demo_hn extension disable example.inventory/publisher --json
demo_hn extension remove example.inventory/publisher --json
demo_hn artifact show "$TICKET_ID" --json
demo_hn artifact history "$TICKET_ID" --json
demo_hn issue list --json
```

The custom ticket and its schema remain readable without the publisher. Ordinary
native commands still work. Removal unregisters the executable; it does not
delete signed public records or your private application files. Keep `$DEMO`
for inspection. Share only deliberately selected public artifact exports, not
the demo HOME, Git private state, command/results or an entire workspace.

## Compatibility, upgrades and troubleshooting

Use a publisher implementing the documented `hn.extension/1` host protocol.
Product version, module version and a custom type's schema version are separate.
Publishing schema v2 does not silently convert old records: use the
[explicit conversion recipe](extension-consumers.md#publish-version-2-and-convert-an-item).
Custom data never activates Spec Kitty doctrine or any other domain engine.

| Symptom | Next step |
| --- | --- |
| `hn` or `artifact`/`extension` unavailable | Check the executable path/version and use a matching supported package. Plugins are not bundled into older alpha binaries. |
| Executable changed or registration differs | Inspect it. To adopt a trusted replacement, explicitly remove/register again and re-enable the complete intended grant set in each repository. No automatic update or grant inheritance occurs. |
| Permission/capability refusal | Compare the manifest, current grants and requested action. This demo grants create/type.publish, not revise/resolve. A grant change is an explicit decision. |
| Management succeeds, business result refuses | Stop, retain the original command and follow the business error/recovery instruction. Do not infer success from exit 0 alone. |
| Lost result or changed-intent retry | Keep the original actor, operation, full state and parents. Use [artifact operation lookup](artifacts-v1.md#cli-and-exact-recovery); an uncertain lookup is not permission to create another record. |

Grants mediate calls through Hubnot; they do not sandbox a module's direct file
or network access. Signed text, schema validity, search matches and a manifest
grant no policy, prompt or execution authority. Read the [host reference](extensions.md)
for protocol/grant details and the [artifact reference](artifacts-v1.md) for data
formats, bounds and public exchange.
