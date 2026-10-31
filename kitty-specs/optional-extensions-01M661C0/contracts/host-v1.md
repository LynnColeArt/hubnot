# Optional host contract — hn.extension/1

WP02 freezes one local, explicitly invoked stdio host for implementation. This is
a public wire/design contract, not a claim that the implementation exists.
The host mediates calls; a same-OS-user process still has ordinary filesystem powers.
No provider/model/MCP/service/language runtime is required by native Hubnot.

## 1. Local registration, repository binding and grants

Only explicit extension commands access local executable registration or session
state. Ordinary native commands and generic artifact commands do not parse it.
A repository declaration at .hn/extensions.json is inert JSON
{"protocol":"hn.extension-declarations/1","modules":[{"module_id":ID,"module_version":V}]}.
It never contains an automatically selected executable, grants or executable code.

Manifest has exactly protocol="hn.extension-manifest/1", module_id, module_version,
host_protocol="hn.extension/1", capabilities. module_id uses the artifact type-ID
grammar and reserved namespaces. module_version is 1..64 printable non-whitespace
ASCII bytes, compared exactly, with no implicit version range. capabilities is a
distinct array drawn from the 14 first methods below; it declares interest, not
authority. Manifest inspection reads bounded JSON and starts/probes nothing.

User-local registry: os.UserConfigDir()/hubnot/extensions/registry.json.
Its object is {"protocol":"hn.extension-registry/1","registrations":[REG]}.
REG has exactly registration_id, registration_instance, manifest, manifest_digest,
executable, executable_digest, argv. registration_instance is exactly 32 lower-case
hex digits encoding 16 bytes from the operating system cryptographic random source.
It is private local lifecycle state, never selected by a manifest or caller.
executable is the operator-selected physically resolved absolute regular executable
path, not a shell; argv is a UTF-8 string array with no NUL bytes. A manifest does
not choose either. Digests are D(C(manifest)) and D(executable bytes).
registration_id = D(C(object containing argv, executable, executable_digest,
manifest_digest, registration_instance)). An exact duplicate of an existing
registration preserves its registration_instance and registration_id. A new
registration, including re-registration after removal with identical manifest,
executable and argv, draws a fresh registration_instance; never reconstruct it
from the module descriptor or reuse a removed instance. Random-source failure
returns host/repository_error with recovery=do_not_retry before registry or
enablement replacement. A malformed/missing instance or a registration_id that
does not match this preimage makes the private registry invalid; do not synthesize
or repair it. Re-resolve/re-hash before launch; symlink/replacement/content or
argv/manifest changes invalidate the registration binding and require re-enablement.
Do not execute the binary to inspect its version.

Repository identity is bound by the host to the verified physical Git common
directory, using the existing safe worktree/reciprocal-pointer validation.
repository_id = D(UTF8("hn-extension-repository-v1") || 00 ||
UTF8(physically resolved absolute common-directory path)).
This is a local physical binding, not a globally portable project identity.
Linked worktrees share that binding. The request has no repository/module override.
Do not trust a request-provided path or a path copied from a cloned declaration.

Enablement is private at COMMON_GIT_DIR/hn/extensions/enabled.json:
{"protocol":"hn.extension-enablement/1","entries":[ENTRY]}.
ENTRY has registration_id, repository_id, epoch, enabled, grants.
epoch is a 32-lower-case-hex random value replaced at each enable/disable/grant edit.
enabled is boolean. Grant has exactly selector, actions.
selector is native:FAMILY (the six data families or identity) or
artifact:NAMESPACE/NAME or artifact:NAMESPACE/*; no path expansion/general glob.
actions is a distinct array from read, observe, create, revise, resolve,
type.publish, raw.inspect. The additional exact selector artifact:opaque permits
ONLY raw.inspect of explicitly preserved public quarantine objects. Native identity permits read only. Type publishing is
custom only; raw.inspect is custom only. Grant sets do not authorize governance,
identity mutation, execution, arbitrary CLI arguments, providers or transport apply.
A declared method must also have the appropriate actual selector/action grant.
Changing registration/epoch revokes existing sessions; re-enable is explicit.

Registry/enablement/declaration files are strict JSON, private directories 0700
and files 0600, no unsafe symlink following; use atomic replacement. Missing files
mean no registration/enablement; malformed files fail explicit extension commands.
No automatic initialization, installation, sync, grant or policy selection occurs.
Private executable paths/argv, credentials, signing roots, journals, raw stderr
and local control descriptors are excluded from public results/export.

## 2. Session startup and finite framing

Start exactly the registered executable plus argv, without a shell, in the selected
repository worktree. stdin/stdout are private pipes; stderr is separate bounded
private diagnostics. Establish a distinct owned process group where supported;
cleanup targets the child/group this host created, never a caller-supplied PID.
This is not containment of a malicious process that escapes its group.

A frame is an unsigned 4-byte big-endian byte length followed by one UTF-8 JSON
object. Payload length 1..8388608 bytes. Reject zero/oversize length before payload
allocation; bounded streaming parser rejects duplicate keys, invalid Unicode,
trailing data, unknown envelope fields and nonobjects. JSON value rules are those
in artifact-v1.md; transport objects need not have canonical spacing/key order.

Host sends first:
~~~json
{"protocol":"hn.extension/1","hello":{"session_id":"00000000000000000000000000000001","repository_id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","module_id":"example.inventory/tool","module_version":"1","registration_id":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},"limits":{"frame_bytes":8388608,"read_timeout_ms":30000,"mutation_timeout_ms":240000,"page_max":200,"one_outstanding":true}}
~~~
IDs in this hello example illustrate grammar; literal fixture identities are in
conformance-fixtures.md. session_id is host-generated 128-bit random lower-case hex.
Module must answer exactly {"protocol":"hn.extension/1","ready":true}.
Startup is an absolute 10-second budget from beginning launch validation through
hashing/spawn/hello write/full ready frame, including partial prefix/payload and
blocked I/O. Incompatible manifest refuses before spawn. No business request is
dispatched before ready.

A business request has required protocol, request_id, method, params and optional
timeout_ms. request_id is 1..128 ASCII bytes 0x21..0x7e, unique within the session;
it is correlation, never durable mutation identity.
~~~json
{"protocol":"hn.extension/1","request_id":"r1","method":"capabilities.list","params":{}}
~~~
Only one business request may be outstanding. The host's reader may accept this
separate control while that request is outstanding:
~~~json
{"protocol":"hn.extension/1","cancel":"r1"}
~~~
A cancel object has exactly those two fields, consumes no business-request slot
and receives no separate acknowledgement. It must name the current outstanding
request. Invalid control/second business request is a protocol error: cancel the
active context, close the session, retain any dispatched mutation as uncertain.
Control reading never dispatches an additional writer.

Idle frame-read budget is 30 seconds beginning when waiting for a new request,
even with no prefix bytes. Partial prefixes/payloads must complete within that
budget. While a business request is outstanding, an idle control reader uses its
remaining deadline; after the first control byte, its partial-frame budget is
min(30 seconds, remaining request deadline). Request execution deadline is measured
from its first prefix byte: read default 30 seconds, mutation default 240 seconds;
timeout_ms may choose 1..240000 milliseconds and never extend a stricter native
deadline. The initial partial-frame budget is not extended by an unread timeout.
Response encoding/blocked writes count against the request budget.

Every ordinary response has exactly protocol, request_id, ok, data, error.
~~~json
{"protocol":"hn.extension/1","request_id":"identity1","ok":true,"data":{"initialized":false,"actor":null,"name":null,"public_key":null},"error":null}
~~~
The success example is a complete identity.public response for request identity1.\nCapabilities responses must contain the full inventory in §4.
Success has ok=true and error=null; failure has ok=false,data=null. A frame/startup
error without a readable request ID may emit one terminal response with
request_id=null, then closes; valid requests always echo their exact ID.

Error has exactly origin, code, message, recovery, operation.
origin is host, native or artifact. message is sanitized UTF-8, at most 4096 bytes;
never dump stderr, environment, private journal/input or credentials.
operation is null or {"protocol":P,"actor":A,"operation":K}; P is hn/0 or
hn-artifact/1. recovery is do_not_retry, retry_identical, inspect_operation or
rescan_required. It is guidance, not an automatic retry.
~~~json
{"protocol":"hn.extension/1","request_id":"r2","ok":false,"data":null,"error":{"origin":"host","code":"denied_access","message":"action is not granted","recovery":"do_not_retry","operation":null}}
~~~

## 3. Cancellation, disable and native writer boundary

Before dispatch, validate frame/input, host binding, declared method, compatibility,
enablement epoch and grants. Refusal before dispatch creates no signing material,
intent, journal or public object/ref. Recheck the actual active actor immediately
before each write; caller actor must match, and missing identity is not initialized.

After dispatch, a timeout/EOF/cancel/crash/disabled epoch may have interrupted an
effect. Stop accepting business requests; cancel the same parent context and
terminate the owned child/group, kill after 2 seconds if necessary, verify exit
within 5 seconds. Stderr byte 65537 is resource_limit and cancels the session;
retain at most 64 KiB privately, not a silent successful truncated log.
Hosts observe enablement/epoch changes at most every 250 ms plus before every
request. Disable revokes all local sessions of that registration/repository; hosts
stop only their own child handles, not raw PIDs from files. Detection is included
in the observed 5-second disable-cleanup budget. Separate session-slot locks cap
four hosts per physical repository; no contribution write lock is held by idle hosts.
A failed disable cleanup reports failure, never "disabled successfully" while
advertised owned processes remain. Public records remain intact.

A pre-dispatch cancellation is cancelled/do_not_retry. A dispatched mutation whose
complete outcome is not established is uncertain_operation/inspect_operation with
original actor/key/protocol, even when its child disconnected. Do not return
cancelled/verified_absent merely because a timer fired. A completed native refusal
or success retains its actual result. An oversized/unwritable response after a
write also leaves an uncertain operation rather than a pre-effect size refusal.

No detached writer may publish after a definitive refusal. Join the native writer
before releasing its per-request lock or accepting another business request.
On synchronous/kernel I/O that cannot stop promptly, child cleanup remains
observed but writer settlement/host exit may exceed the watchdog; preserve an
uncertain result. This explicit limitation is not a hard kernel-I/O SLA.

### Frozen WP17 context/lock seam (O1)

Existing native wrappers retain their behavior; add explicit Context variants,
not a copied writer, mutable global request context or subprocess argv bridge:

- resource_engine.go: MutateResourceContext(ctx,input) and
  applyResourceMutationContext(ctx,input,identity,captured,events).
  MutateResource/applyResourceMutation call these with context.Background().
- issue_commands.go: executeIssueMutationContext(ctx,options) and
  applyIssueMutationContext(ctx,options,identity,captured,events,snapshot).
  Existing functions remain wrappers using context.Background().
- contribution_moderation.go: contributionFinishNativeContext(parent,stored,plan)
  and contributionRetryNativeContext(parent,stored); existing wrappers pass
  context.Background(). The existing contributionContext receives parent and
  retains min(parent deadline, native reservation/stage limit). Existing retry
  and finish flow use that same parent; no background-rooted inner replacement.
- main.go (WP16 owner): exclude only explicit "extension run" from the outer
  whole-command contribution lock. Other native commands keep their outer lock.
  Local static dispatch performs no extension I/O on ordinary native paths.
- Each host native request takes existing contributionCommandLock for the actual
  native family/operation and releases after the synchronous writer settles.
  A nonblocking busy lock is repository_error/retry_identical before dispatch.
  Recheck identity, selected authority/policy and governed projection under that
  lock; preserve append CAS, contribution admission/reservations and promotion
  rechecks. Never acquire the same lock recursively through run().
  Reads/capture needing a coherent native view take that same scoped lock.

Root must amend WP17 ownership for resource_engine.go and issue_commands.go before
WP17 starts; contribution_moderation.go is already its declared shared writer.
These are context threading additions, not permission to change native signed
bytes, digests, replay order, visibility, refusals or direct CLI semantics.
Live tests must prove idle/blocked host versus another native command, two modules,
governed host mutation, cancellation on both sides of publication and no late
publication after a definitive refusal.

## 4. Strict method types and grants

Notation below defines JSON types, not executable pseudocode. Every listed field
is required unless explicitly marked optional; reject unknown fields. Empty arrays
are [], never null. ID means full D digest; Actor means the native fingerprint.
Family is issue, project, project-item, discussion, discussion-reply or workflow.
ReadSelection = {"families":[Family...],"types":[TypeID...]}: distinct sorted
arrays, at least one entry, no implicit all; exact custom type IDs only.
Cut = D(C({repository_id,refs,selection,authority_commit,policy_digest,view_digest})).
refs is the distinct selected accepted [{ref:string,oid:GitOID}] array sorted by
ref; authority_commit is the exact selected native policy Git commit or "";
policy_digest is its exact public policy-byte D digest or "".
view_digest = D(C({admitted_ids:[ID],visible_ids:[ID],retained_refs:[{ref,oid}]})),
with increasing distinct IDs from the same verified selected native/custom
projection and ref-sorted retained observations. Custom projection has no selected
contribution policy; native projection includes the actual admission/moderation/
retention result. Do not derive visible_ids from raw signature validity.
Empty absent authority fields are "", not omitted. IDs/refs contributing to these
internal fingerprints need not be disclosed beyond the caller's read grants.
SnapshotToken, Cursor and Checkpoint are opaque bounded strings <=4096 bytes;
a consumer must not construct them or interpret lexical order.

RecordView = {protocol:"hn-artifact/1",id:openingID,type_id:TypeID,shape:Shape,
heads:[{id:ID,actor:Actor,type_revision:ID,payload:RecordPayload}],conflict:boolean,
state:RecordPayload|null,diagnostics:[Diagnostic]}.
state is null on conflict, otherwise the sole current payload. Head arrays sort
by full ID. Diagnostic = {code:CODE,id:ID|null,message:string}; CODE is
incomplete_dependency, unsupported_schema, target_conflict, missing_optional,
invalid_history or opaque_unverified. A diagnostic is not semantic authority.

NativeView = {protocol:"hn/0",family:Family,id:ID,creator:Actor,
heads:[{id:ID,actor:Actor,state:NativeState}],conflict:boolean,
state:NativeState|null,comments:[{id:ID,actor:Actor,body:string}]}.
comments is [] for non-issue families; a conflict has state=null.
These are normalized host views of the same governed native projection, not new
storage/writers or raw chains. Native state fields/types are specified below.

MutationResult = {protocol:P,family:Family|null,type_id:TypeID|null,
resource_id:ID,event_id:ID,replayed:boolean,conflict:boolean,head_count:integer,
admission:null|"admitted"|"pending"|"hidden",source_cut:Cut}.
For native issue, resource_id maps the writer's issue_id; other native families
map resource_id unchanged. event_id is the writer's exact signed ID. admission
maps absent/empty native admission to null and otherwise preserves the actual
native value (see native admission note below). Custom results use family=null,
type_id set,admission=null. Type publication uses conflict=false,head_count=1
for that exact declaration, not a namespace winner. Current conflict/head_count are observations at the
returned source cut, not immutable content of the original acknowledgement.

OperationResult = {status:"found"|"verified_absent"|"uncertain",protocol:P,
actor:Actor,operation:Key,family:Family|null,type_id:TypeID|null,
request:ID|null,resource_id:ID|null,event_id:ID|null,
result:MutationResult|null,source_cut:Cut,reason:null|Reason}.
found has non-null request/resource_id/event_id/result and reason=null.
verified_absent has those four fields null and reason=null, under the complete
absence conditions in artifact-v1.md; the native adapter must prove its equivalent
complete selected history and no pending local publication evidence.
uncertain has no invented result and reason one of incomplete_selection,
unavailable_history,private_intent_pending,visibility_restricted,
publication_unconfirmed. Invalid/contradictory history is an error, not uncertain.
A lookup cannot disclose a hidden native submission by reading raw histories.

Page(T) = {items:[T],source_cut:Cut,next_cursor:Cursor|null}.
A null next_cursor means this selection is complete at that cut. Limit is 1..200,
default 20 only where explicitly optional; total encoded response still <=8 MiB.
No result truncation. If a single complete state cannot fit, resource_limit with
explicit external/local export or narrower-selection guidance, not silent omission.

| Method | Exact params | Data | Required action |
|---|---|---|---|
| capabilities.list | {} | {session:Session,capabilities:[Capability],unsupported:[Unsupported]} | none beyond enabled compatible session |
| identity.public | {} | {initialized:boolean,actor:Actor|null,name:string|null,public_key:string|null} | read on native:identity; no initialization |
| resource.read | {family:Family,id:ID|null,limit:integer,cursor:Cursor|null} | Page(NativeView); id=null lists, otherwise exactly one selected resource or not_found | read on native:family |
| resource.mutate | {family:Family,intent:Intent,id:ID|null,expected:[ID],actor:Actor,operation:Key,state:NativeState|null,comment:string} | MutationResult | open/comment=create; revise=revise; resolve=resolve |
| resource.operation | {family:Family,actor:Actor,operation:Key} | OperationResult | read on native:family; wrong-family key is operation_conflict |
| artifact.read | {id:ID|null,opaque_id:ID|null,type_id:TypeID|null,limit:integer,cursor:Cursor|null,mode:"view"|"heads"|"history"|"inspect",opaque:boolean} | Page(RecordView) for view/heads; Page(SignedArtifact) for history/inspect | read on matching custom type; opaque requires raw.inspect on artifact:opaque |
| artifact.mutate | {kind:"record.open"|"record.revise"|"record.resolve"|"record.convert",record:ID|null,parents:[ID],payload:RecordPayload,actor:Actor,operation:Key} | MutationResult | open=create, revise/convert=revise, resolve=resolve |
| artifact.operation | {type_id:TypeID,actor:Actor,operation:Key} | OperationResult including type.publish; wrong-type key is operation_conflict | read on matching custom type |
| artifact.search | {type_id:TypeID|null,shape:Shape|null,literal:string|null,regex:string|null,limit:integer,cursor:Cursor|null} | Page(RecordView) | read on every selected custom type |
| artifact.type.read | {id:ID|null,type_id:TypeID|null,limit:integer,cursor:Cursor|null} | Page(SignedArtifact) | read on selected custom type |
| artifact.type.publish | {parents:[ID],payload:TypePayload,actor:Actor,operation:Key} | MutationResult with resource_id=event_id=declarationID | type.publish on payload.type_id |
| changes.capture | {selection:ReadSelection,limit:integer} | ChangePage | observe AND read on every selected family/type |
| changes.page | {capture:SnapshotToken,cursor:Cursor,limit:integer} | ChangePage | same grants/epoch/selection as capture |
| changes.since | {checkpoint:Checkpoint,selection:ReadSelection,limit:integer} | ChangePage beginning a new replacement snapshot | same observe/read grants; valid prior checkpoint |

Intent for resource.mutate: open/revise/resolve for every family; issue additionally
comment. No native convert, close/reopen convenience request or arbitrary action
is advertised; issue status can be explicitly revised through the existing writer.
Open has id=null,expected=[]; other native intents have id set. Comment has
state=null,expected=[],comment non-empty; other intents have comment="" and full
state. Expected parents use the native contract (including issue limit 200).
No partial-resolution shortcut is exposed.

SignedArtifact is one of two exact shapes:
- {id:ID,envelope:ArtifactEnvelope,signature:string,
  validation:"verified"|"unsupported"|"incomplete",diagnostics:[Diagnostic]}.
- {id:null,opaque_id:ID,envelope:null,signature:null,validation:"opaque",
  raw:{oid:string,type:string,data:Base64},diagnostics:[Diagnostic]}.
The second shape is permitted only by explicit opaque inspection; it cannot
invent a decoded envelope, artifact identity or verified signature. Normal modes
never have raw or validation=opaque.
Normal artifact.read requires id or type_id, opaque_id=null; history requires id.
Opaque reads require mode=inspect,opaque=true,id=null,type_id=null and one explicit
opaque_id; that identity is D(UTF8(Git object type) || 00 || raw Git object body).
It is NOT a signed artifact ID, logical target or accepted revision. Only the
bounded explicitly preserved PUBLIC quarantine index may resolve it. No arbitrary
Git ODB object, working-tree/source path, private journal or credential can be
retrieved by this selector. Missing opaque IDs return not_found; no fallback to a
native or accepted object. A signed ID can never substitute for opaque_id.
An otherwise legal 8MiB raw object may exceed the 8MiB response after Base64/JSON
overhead: refuse resource_limit and direct explicit generic CLI/file diagnostic
export. A smaller page cannot fix a single oversized full object; never truncate.
Opaque pages retain consuming exchange/read bounds and never infer a namespace,
semantic validity, accepted visibility or promotion permission. A grant-filtered list never reports a complete global absence.
Search requires exactly one of literal/regex, searches title/content and canonical
fields text, and validates the complete selected work budget before any output.
Type/read selection with both id and type_id must agree. Custom grants filter
selection explicitly; no unsupported or denied type is silently promoted.

Session = {session_id,module_id,module_version,registration_id,repository_id,
protocol:"hn.extension/1"} with values selected at startup.
Capability = {method:string,declared:boolean,granted:boolean,enabled:boolean,
compatible:boolean,availability:"observed"|"not_probed"|"unavailable",
recovery:Recovery}. Method inventory contains each of the 14 rows exactly once.
Granted reports whether an applicable selector/action exists; every actual request
still undergoes exact scope checking. Observed availability means a successful
session handshake, not unexercised compatibility inferred from a version.
Unsupported = {surface:string,status:"unsupported",recovery:"do_not_retry"}.
Unsupported surfaces are memory.mutate, proposal, review, decide, merge,
policy.mutate, identity.mutate, run, runner, audit, automerge, sync.apply,
migration.apply. No installation enables them.

### Native states retained from the integrated beta source

These tables pin JSON field spelling. Existing native validation and limits remain
authoritative; the host does not accept unknown fields, weaken canonicalizers or
translate descriptions into execution. Optional native fields may be absent, with
the same native defaults; required fields below are required in host input.
All state text is UTF-8, native resource payload <=256 KiB, workflow <=2 MiB.

| State | Required fields | Optional native fields |
|---|---|---|
| issue | title:string,body:string,status:"open"|"closed",criteria:[{id,text}],labels:[string],assignees:[string],relations:[{kind:"blocks"|"related"|"parent",target:ID}],metadata:map(string,string) | none |
| project | schema:"hn.project/1",title:string,body:string,archived:boolean | fields:[ProjectField],views:[ProjectView],metadata:map(string,string) |
| project-item | schema:"hn.project-item/1",project_id:ID,project_revision:ID,target:ProjectTarget,archived:boolean,position:string | values:map(string,ProjectValue),metadata |
| discussion | schema:"hn.discussion/1",title:string,body:string,category:string,status:string | accepted_reply_revision:ID,metadata |
| discussion-reply | schema:"hn.discussion-reply/1",discussion_id:ID,body:string | reply_to:ID,metadata |
| workflow | schema:"hn.workflow/1",name:string,title:string,description:string,enabled:boolean | pipeline:PipelineDefinition,source_documents:[{format,content,digest}],metadata |

ProjectField={id,name,type,options?:[{id,name}]}; ProjectView={id,name,layout,
group_by_field?:string}. ProjectTarget={kind,id?:ID,title?:string,body?:string,
url?:string}; ProjectValue={type,text?:string,number?:string,date?:string,
option_id?:string}. Field/value enum domains and all exact PipelineDefinition
nested fields remain the published native contracts in docs/projects-v1.md and
docs/workflows-v1.md and the integrated resource_model.go canonicalizers; this
adapter does not create a second native format. Number/date are native strings,
not floating-point JSON. See native limitations/examples in §6.
Mutation input full state is passed to CanonicalIssueState/CanonicalResourceState
and the normal writer, not reconstructed by shell arguments.

The native nested contracts are explicitly:
- ProjectField.type and ProjectValue.type: text, number, date, single_select.
  ProjectView.layout: table or board; group_by_field names a single_select field.
  ProjectTarget.kind: issue, proposal or discussion with only a full target ID;
  draft with only title/body; external with only an HTTP(S) URL without userinfo.
- ProjectValue has exactly one corresponding value field: text <=4096 bytes;
  number is canonical decimal string <=64 bytes (no exponent, -0 or trailing
  fractional zero); date is valid YYYY-MM-DD; option_id is a native identifier.
- PipelineDefinition={version:"hn.pipeline/0",steps:[PipelineStep]}.
  PipelineStep={name:string,command:string,args?:[string],
  workingDirectory?:string,timeoutSeconds?:integer}. There are 1..64 steps;
  nonempty name/command, no command NUL, command path and directory cannot escape
  the checkout; <=256 arguments, each <=16384 bytes/no NUL; timeout 0..3600
  seconds, 0 retaining native default. This stores a definition and executes none.
- Native identifiers are 1..64 ASCII letters/digits/dot/underscore/hyphen,
  excluding "."/".."; field/option/view names <=256 UTF-8 bytes.
  Projects have <=64 fields, <=64 options/field and <=16 views.
  Items have <=64 values and canonical decimal position (empty native input
  defaults to "0"). Native metadata has <=64 namespaced string entries,
  key <=128 and value <=4096 UTF-8 bytes.
- Native issue title/body limits are 1024/65536 bytes; <=64 criteria, labels,
  assignees and metadata; <=128 relations. Criterion ID <=64/text <=4096;
  labels/assignees <=256 bytes, distinct; issue expected parents <=200.
  Native canonicalizers retain their exact set sorting and criterion order.
- Discussion status is open/closed, category is 1..256 bytes; reply body is
  nonempty <=65536 bytes. Workflow name is a native identifier, title/description
  <=1024/65536 bytes; <=8 source documents, each <=1MiB, distinct correct content
  digests. Enabled workflow requires pipeline; pipeline C bytes <=1MiB.
  These narrower native limits apply inside the general 8MiB host frame.

### Descriptive connector associations

No special provider trust is inferred. A connector may use native namespaced
metadata, e.g. key "example.tracker/source" (native max128 bytes) with value
"tracker:item:42" (native max4096 bytes), or plain issue comment text. These values
are bounded untrusted descriptions, not verified remote evidence, actor identity,
policy/grant selection or executable instructions. Private API credentials/journals
remain connector-owned and are never copied into public metadata.

## 5. Accepted observations: atomic replacement, not an event-order promise

Change observation reads the same accepted visible native projection, including
admission, moderation, retention and selected authority, plus verified custom
records when that combination is supported. Never substitute raw signed chains.
Custom data under selected contribution policy is not advertised as a governed
custom feed; requesting that unsupported selection refuses before capture state.
Capture itself causes no transport sync, execution or native write.

ChangeEntry = {protocol:P,family:Family|null,type_id:TypeID|null,
resource_id:ID,revision_ids:[ID],value:NativeView|RecordView}.
One complete current resource per entry; revision_ids are all visible current
heads sorted by full ID (plus issue comment IDs for issue view). Dedupe key is
(repository_id,selection_digest,resource_id), with full revision identities for
content comparison. Entries sort by (protocol,family-or-type_id,resource_id).
No timestamp/full-ID ordering is global causal order.

ChangePage = {capture:SnapshotToken,snapshot_id:ID,selection:ReadSelection,
selection_digest:ID,source_cut:Cut,mode:"replace",items:[ChangeEntry],
next_cursor:Cursor|null,complete:boolean,checkpoint:Checkpoint|null,
expires_at:UTCtimestamp}. selection_digest=D(C(selection)).
snapshot_id=D(C(object containing source_cut,selection_digest,entries)) for the
ENTIRE captured sorted set. It is not recomputed from one page.
At most 10000 entries and 16 MiB C bytes for the complete entry array; captures
expire 10 minutes after creation and are session-bound. Every page rechecks
registration epoch, grants, source cut, authority/policy/moderation/retention view.
Any mismatch, expiry, restart or missing captured state invalidates the cursor.
Do not silently mix old and new pages. Duplicate page requests return the same
captured page while valid; limit changes are permitted but never skip offsets.

Atomic replacement consumer algorithm:
1. Begin an empty staging set for snapshot_id/selection_digest, preserving the
   previously committed visible set and checkpoint.
2. Add/dedupe every page's entries into staging. Pages have one snapshot/cut and
   deterministic offsets encoded by opaque cursors; an interruption never advances
   the durable checkpoint or deletes the prior set.
3. Only the final page has complete=true,next_cursor=null and a non-null checkpoint.
   In one consumer transaction replace the previous set for EXACTLY that selection
   with staging (thereby removing any previously exported now-hidden/absent entry)
   and persist the checkpoint. All earlier pages have complete=false and
   checkpoint=null. Empty snapshots still send a final completion page.
4. On invalidation discard incomplete staging and rescan explicitly from capture.
   Selection changes require a new independent snapshot; do not reuse a checkpoint
   under a different selection or infer removals outside its exact selection.
5. changes.since validates prior repository/selection/grants/view checkpoint. If
   compatible it begins a COMPLETE current replacement snapshot (possibly
   identical), not a sparse diff. Thus hide/restore without a record edit is
   reconciled on the next completed snapshot. If the old view/retention cut
   cannot be verified, return checkpoint_invalidated/rescan_required and require
   a fresh capture; no silently successful gap.

Checkpoint is a bounded authenticated/validated host token binding repository,
selection, source cut, view and completed snapshot. It may persist across sessions
when that cut can still be verified; cursors/captures may not. Local keyed token
material is private; a missing/replaced verifier returns explicit invalidation.
Replay is at-least-once descriptive observation, not an exactly-once write stream.

For >10000 entries or >16 MiB, return resource_limit before paging and create no
apparently complete snapshot. Reducing page size CANNOT fix capture capacity.
Use explicit bounded selections/partitions (e.g. separate listed families/types),
or a local full export outside this host contract; if one type alone is too large,
report that this first capture API cannot represent the selection. A page that
exceeds frame bound refuses resource_limit with smaller-page guidance; a single
oversized complete state cannot be silently truncated.

## 6. Closed errors, recovery and exact native examples

Host-origin codes are exactly denied_access, incompatible_version,
unavailable_module, unsupported_capability, resource_limit, cancelled,
uncertain_operation, checkpoint_invalidated, verification_failure, invalid_input,
not_found, repository_error.

Native-origin codes observed in integrated issue/resource entry points are exactly
actor_mismatch, admission_pending, admission_refused, ambiguous_id, concurrent_update,
invalid_cursor, invalid_history, invalid_input, issue_conflict, not_found,
operation_conflict, permission_denied, repository_error, resource_conflict,
resource_limit, stale_cursor, stale_revision.
No unknown native error becomes success. A plain non-coded native failure is
repository_error with sanitized native context. Future codes require contract review.

Artifact-origin codes are exactly invalid_input, resource_limit,
verification_failure, invalid_history, incomplete_dependency, unsupported_schema,
actor_stream_conflict, operation_conflict, stale_revision, resource_conflict,
concurrent_update, not_found, repository_error, cancelled, uncertain_operation.
Diagnostic target_conflict/missing_optional/opaque_unverified are not success
codes or authority. A source record whose own schema and required closure are
valid may retain a conflicted logical target without choosing a head; its write
returns the normal MutationResult and subsequent artifact.read exposes that
target_conflict diagnostic. Opaque preservation never reports accepted promotion.

| Condition | Origin/code | Recovery |
|---|---|---|
| absent grant | host/denied_access | do_not_retry |
| unsupported method/family/policy combination | host/unsupported_capability | do_not_retry |
| changed executable/protocol | host/incompatible_version or unavailable_module | do_not_retry; inspect/re-enable explicitly |
| malformed/oversized input before dispatch | invalid_input or resource_limit | do_not_retry |
| failed signature/history | verification_failure or native invalid_history | do_not_retry |
| native stale revision/conflicted record | native/stale_revision,issue_conflict,resource_conflict | do_not_retry; inspect and choose a NEW explicit intent/key |
| changed replay | operation_conflict | do_not_retry |
| actor mismatch | native/actor_mismatch | do_not_retry |
| pre-publication CAS or busy operation lock | concurrent_update or repository_error | retry_identical only after inspection/settlement |
| native rejected admission | native/admission_refused or permission_denied | do_not_retry |
| signed result with pending admission | native/admission_pending | inspect_operation |
| post-dispatch lost result / response too large | uncertain_operation | inspect_operation |
| incomplete lookup | OperationResult.status=uncertain | inspect_operation |
| invalid cursor/checkpoint/view | checkpoint_invalidated or native stale_cursor | rescan_required |
| pre-dispatch cancel | host/cancelled | do_not_retry |

Native examples retain actual integrated strings, not renamed approximations:
- actor_mismatch: "active signing actor differs from the expected actor; repair the binding" (issue).
- stale_revision: "expected revision is no longer the current head" (issue).
- stale_revision: "expected revisions must exactly match every current head" (resources).
- issue_conflict: "issue has concurrent heads; inspect heads and resolve explicitly".
- resource_conflict: "resource has concurrent heads; inspect heads and resolve explicitly".
- operation_conflict: "operation key was already used for different content" (issue).
- operation_conflict: "operation key was already used for different kind, intent, content or expectations" (resources).
- concurrent_update: "actor history changed; retry the same operation and expectations" (issue).
- not_found: "resource not found in requested family" (resources).

The adapter preserves these code/message semantics; it may sanitize private details,
but cannot map stale/conflict/permission/admission into success or universal retry.
The issue/resource native key scope spans their shared actor hn/0 history: reusing
a native key in another supported family is operation_conflict. Artifact keys have
their separate explicitly signed protocol scope. Missing local connector journals
do not prove remote API absence; connector reconciliation stays application-owned.
contributionAdmission returns exactly empty, pending, hidden or admitted in
the integrated source. Empty maps to JSON null; the other three values remain
verbatim. In particular, "rejected" and "withdrawn" are not native return values.
Closed output tests must mirror this exact set; it is descriptive, not authority.

## 7. CLI, private bounds and required conformance

Static explicit commands:
- hn extension register --manifest FILE --executable PATH [--arg VALUE] --json
- hn extension inspect [MODULE_ID] --json
- hn extension enable MODULE_ID --grant SELECTOR=ACTION[,ACTION] --json
- hn extension disable MODULE_ID --json
- hn extension remove MODULE_ID --json
- hn extension run MODULE_ID --json
No startup hook, background daemon, package downloader, shell argv bridge or implicit
provider sync. register/inspect/enable/disable/remove never execute a module;
run alone launches. Removing registration revokes enablement and stops owned
sessions through the same disable path; it never deletes signed artifacts.
Enablement entries in other repositories may remain on disk, but name the removed
registration_id and cannot authorize a new registration instance. Re-registering
the same module therefore requires explicit enablement in every repository before
it can run; no search of all repositories or restoration of old grants is implied.
Management and parent run JSON use the artifact CLI success/error envelope;
the registered MODULE's private stdin/stdout are the framed protocol. Parent hn
stdout is one bounded management result, never duplicate module traffic or mixed
human logs. Module diagnostics use its bounded private stderr; reference consumers
own any application result/journal files explicitly.

Management success data is closed:
- register: {module_id,module_version,registration_id,manifest_digest,
  executable_digest,status:"registered",enabled:false}. An exact duplicate
  registration is idempotent; changed existing module ID refuses with
  invalid_input until explicitly removed/re-registered. Nothing auto-updates.
- inspect: {modules:[{module_id,module_version,registration_id,manifest_digest,
  executable_digest,repository_id:ID|null,enabled:boolean,compatible:boolean,
  availability:"not_probed"|"unavailable",grants:[Grant],
  status:"registered"|"missing"|"changed"|"incompatible"}],unsupported:[Unsupported]}.
  inspect never launches; a missing explicit module returns host/not_found with
  recovery=do_not_retry and operation=null. This is a management lookup miss, not
  a native or artifact lookup failure.
- enable/disable: {module_id,registration_id,repository_id,epoch,enabled,
  grants:[Grant],stopped_sessions:[SessionID],cleanup:"not_needed"|"verified"}.
  A new epoch also revokes prior sessions; success requires their verified cleanup.
  Neither command launches a new module.
- remove: {module_id,registration_id,removed:true,stopped_sessions:[SessionID],
  public_data_retained:true}. Removal first uses the same finite disable path.
- run: {session:Session,outcome:"completed",completed_requests:integer,
  exit_code:0,pending_operations:[]}. Completed requests may include deliberately
  refused requests that the module handled; this is not blanket business success.

Parent CLI exit0 and ok=true require successful management completion or clean
module exit0 with no unfinished operation. Startup/handshake failure or nonzero
module exit without a pending dispatched write yields exit1,ok=false,data=null,
a host unavailable_module/incompatible_version error. Pre-dispatch cancellation
uses cancelled. Deadline/EOF/crash/response loss with an unresolved dispatched
mutation uses uncertain_operation/inspect_operation with its actual actor/key;
it NEVER emits the completed summary. Normal child EOF after all responses and
exit0 is completion; EOF during startup/request/response is not. Any known public
result is retained independently of the child/parent exit. An example failure is:
~~~json
{"ok":false,"data":null,"error":{"origin":"host","code":"uncertain_operation","message":"dispatched mutation outcome requires inspection","recovery":"inspect_operation","operation":{"protocol":"hn-artifact/1","actor":"81ebe0bf3a2be9358144a0cabbea22d5b79c008514c61207556a6b5d044d07c0","operation":"asset-open"}}}
~~~
The zero actor illustrates field grammar only; real fixture actors are in
conformance-fixtures.md, and the host always reports the actual bound actor.

| Boundary | At limit | First over / malformed control | Required result |
|---|---:|---:|---|
| frame payload | 8388608 bytes | 8388609 or 0 | resource_limit / invalid_input before allocation |
| manifest/declaration | 131072 bytes | 131073 | resource_limit before spawn |
| registry/enablement file | 1048576 bytes | 1048577 | resource_limit, no implicit repair |
| registrations/enablement entries | 128 | 129 | resource_limit |
| argv count / each bytes / aggregate bytes | 64 / 4096 / 65536 | one greater | resource_limit |
| executable hashing input | 134217728 bytes | 134217729 | resource_limit before spawn |
| grants / actions per grant | 256 / 7 | 257 / duplicate or eighth | resource_limit / invalid_input |
| request ID / module-version bytes | 128 / 64 | 129 / 65 | invalid_input |
| error message / token bytes | 4096 / 4096 | 4097 | resource_limit, no secret-bearing truncation |
| startup | 10 seconds | stalled prefix/payload/write past deadline | cancelled/unavailable, no dispatch |
| default read / mutation, max request | 30 / 240 / 240 seconds | request context expired | cancelled before dispatch; uncertain after write dispatch |
| stderr | 65536 bytes | 65537 | resource_limit, finite child cleanup |
| outstanding requests / repository sessions | 1 / 4 | second / fifth | invalid_input / resource_limit |
| terminate-to-kill / child exit | 2 / 5 seconds | ignored TERM / still alive | observed kill / cleanup failure |
| page limit | 200 | 201 or 0 | resource_limit / invalid_input |
| captured entries / bytes / lifetime | 10000 / 16777216 / 10 minutes | one greater / expired | resource_limit / checkpoint_invalidated |
| enablement observation interval | 250 milliseconds | missed revocation bound | qualification failure, not waived |

Every bound has positive and violating fixtures in conformance-fixtures.md.
Required live checks include raw ungranted write before journal/signer effects,
exact/changed replay, publication interruption, partial/blocked frame cancellation,
disable ownership, unrelated native and second-module progress, governed admission,
visibility hide/restore without edits and atomic interrupted-rescan behavior.

### 7.1 Inert frame-inspection slice

WP15 registers the literal frame subcommand through compiled
extensionSubcommands, with no initialization-time I/O:

- `hn extension frame inspect --input FILE|- --kind hello|ready|request|response|cancel [--expected-request-id ID] [--method METHOD] [--timeout-ms MS] [--json]`

FILE is exactly one explicit local regular file, or `-` for stdin consumed once.
The input is raw bytes: one unsigned four-byte big-endian length followed by one
UTF-8 JSON message, not Base64, a JSON descriptor, a frame list or unframed JSON.
Require EOF after that one frame; a second frame or any trailing byte refuses.
--input and --kind are required and occur once; all other flags occur at most once.
Unknown flags/positionals/kinds refuse invalid_input. MS is integer1..30000;
default30000 uses the existing30-second read budget. Its absolute timer covers
opening/reading the input, partial prefix/payload and the final EOF check. Timeout
or caller cancellation is host/cancelled, with no dispatched operation. A named
nonregular input refuses before consuming its stream. No input bytes are echoed.

Reject zero length as invalid_input and length>8388608 as resource_limit before
payload allocation. Partial prefix/payload, malformed Unicode/JSON, duplicate
keys, nonobjects, trailing data, unknown top-level or nested typed fields, wrong
primitive/null/array types and malformed correlation fields refuse invalid_input.
The same bounded transport JSON rules and first-host message/param/result shapes
from sections2/4 apply, including method membership, value bounds and nullability.
Unknown protocol is incompatible_version; an unknown method is
unsupported_capability. This validates transport and closed typed message shape,
not schema semantics, signature integrity or business preconditions.

--kind selects exactly the existing hello, ready, business-request, ordinary-
response or cancel-control shape, without sending it or creating a session.
Correlation flags have this closed matrix:

- hello/ready: neither --expected-request-id nor --method is permitted.
- request: neither flag is permitted; its own known method selects the exact
  params shape and its own request_id must satisfy the protocol grammar.
- cancel: --expected-request-id is required and must equal cancel; --method is
  forbidden. This proves only supplied ID equality, not an outstanding request.
- response with a nonnull request_id: --expected-request-id is required and must
  match exactly. A successful response additionally requires --method to select
  its exact success data shape. A failed response forbids --method and validates
  the existing closed error shape, including data=null and actual origin/code.
- terminal response with request_id=null: both flags are forbidden; it must be a
  failed response with data=null and a valid closed error. A success with null
  request_id is invalid_input.

Wrong supplied correlation or a flag combination outside this matrix refuses
invalid_input. A request's timeout_ms, if present, retains its existing protocol
bounds; it is data being inspected, not this utility's read timeout. Responses
must preserve the existing ok/data/error consistency. No supplied method or ID
substitutes for a host-selected repository, module, grants or durable actor/key.

Success uses the ordinary management JSON/exit0 wrapper; data is exactly
{frame_bytes:integer,payload_bytes:integer,message_kind:Kind,
request_id:string|null,method:string|null,validation:"frame_and_typed_message",
session_state:"not_checked",authorization:"not_checked",
semantic_validation:"not_checked",signature_integrity:"not_checked",
dispatch:"not_performed"}. frame_bytes=payload_bytes+4. request_id is the actual
request/response ID or cancel target, otherwise null. method is the request's
method or the supplied successful-response method, otherwise null. A known
business refusal inside an otherwise valid response is inspect success: the tool
validates that response's shape, not the success of its business operation.
Failure uses exit1,ok=false,data=null and the existing host-origin closed error
set, with recovery=do_not_retry,operation=null. Sanitized input I/O failure is
host/repository_error. This adds no error origin/code and no host method.

The utility never parses registration/enablement, authorizes or dispatches a
business call, loads identity/accepted state, signs, writes an intent/ref/public
object, launches a module, or applies transport. It does not assert handshake
order, request-ID uniqueness across a session, one-outstanding-request state,
response ordering, cancellation of a live writer, session cleanup or deadlines
of an actual child. Those require WP16+ real-session qualification. This current
slice exposes only bounded frame/message value inspection: do not introduce an
unused public encoder/session-writer API or an artificial encode/decode call
merely to anticipate the later real child writer. The native
outer-lock rules remain unchanged; no default native work or lock bypass is added.
