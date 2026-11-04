# Native consumer CLI v1

The native consumer contract is optional. Discover actual supported capabilities
before selecting machine output:

```sh
hn version --json
hn --version --json
hn capabilities --json
```

These equivalent static commands work outside a repository without identity,
Git, authority selection, contribution locks, extension registration or optional
module bootstrap. `hn version` and `hn --version` retain their original text
bytes. JSON discovery accepts only `--json` (or `--json=true`) and no positional
arguments. Unknown flags return `native.invalid_input`, exit 1.

Success has exactly the envelope `{ok:true,data:...,error:null}`; failure has
`{ok:false,data:null,error:...}` and exit 1. Successful discovery data contains
`schema: "hn.consumer/1"`, the binary's `version`, and a `capabilities` object.
A capability is supported only when present; consumers must not infer support
from the binary version or from this document's future additions.

Current capability entries:

| ID | Version | Meaning |
|---|---|---|
| `native.discovery` | 1 | The three static commands above. |
| `contribution.existing-json` | 1 | The pre-existing contribution output described below; it does not use the consumer envelope. |

All errors use the existing closed native code inventory in
[extensions.md](extensions.md#closed-errors-and-immutable-recovery).
The error object has exactly `origin`, `code`, `message`, `recovery`, `operation`.
Messages are sanitized UTF-8 and at most 4096 bytes. This contract's unkeyed
commands return `operation:null` and `recovery:"do_not_retry"`: inspect the
outcome before making a new explicit user intent. JSON receipts add no operation
key, lost-acknowledgement recovery or automatic mutation retry promise. A failed
command also emits a generic CLI diagnostic on stderr; parse stdout for the
machine envelope.

## Existing contribution output audit

`contribution_commands.go` has nine `--json` flag registration sites. They are
compatibility flags, since their successful data output already uses
`contributionPrint` regardless of the flag. They do **not** select `hn.consumer/1`
or the `{ok,data,error}` envelope. Errors keep their existing CLI behavior.

| Registration site | Commands/aliases | Existing output |
|---|---|---|
| authority set | `contribution authority set` | Selected authority object. |
| unselected restore | `contribution restore` without selected authority | Native intake object. |
| receive/restore | `contribution receive`, `contribution restore` | Native intake object. |
| retained | `contribution retained show`, `contribution retained register` | Snapshot/registration object. |
| intake | `contribution intake` | Receiver-local attempt, ledger and descriptor. |
| inspect | `contribution inspect` | Existing view or selected record/raw payload. |
| archive | `contribution archive` | Existing archive descriptor. |
| prepare | `contribution prepare` | Existing submission/artifact descriptor. |
| raw inspect | `contribution inspect --raw` | Exact immutable native/contribution payload and signature. |

Several registration sites serve aliases or subcommands; nine sites does not
mean nine distinct CLI command names. Other contribution commands may also emit
JSON through their existing handlers. This is a consistency/discovery audit,
not an envelope transition: no contribution default output bytes, signatures,
accepted-history selection or authority change here.
