# Public issue consumer example

This small Go program uses the `hn.issue/1` CLI interface. It intentionally
creates and revises demonstration work in the repository you explicitly name.
Use a disposable repository for the example. Go 1.26 and Git are sufficient;
there is no SDK import, service, jq, or additional Go module.

From the Hubnot source checkout:

```sh
mkdir -p /tmp/hn-issue-example-bin
go build -o /tmp/hn-issue-example-bin/hn .
go build -o /tmp/hn-issue-example-bin/issue-consumer ./examples/issue-consumer
```

Create a new empty demonstration directory, then initialize it:

```sh
mkdir /tmp/hn-issue-example-repository
git -C /tmp/hn-issue-example-repository init -b main
git -C /tmp/hn-issue-example-repository config user.name "Example"
git -C /tmp/hn-issue-example-repository config user.email "example@hn.invalid"
cd /tmp/hn-issue-example-repository
/tmp/hn-issue-example-bin/hn init --name "Example device"
/tmp/hn-issue-example-bin/issue-consumer \
  --hn /tmp/hn-issue-example-bin/hn \
  --repo /tmp/hn-issue-example-repository
```

Choose different new temporary paths if those names already exist. The program
requires both paths and uses the supplied repository as each subprocess's
working directory. It never initializes or selects an ambient repository.

The result is one JSON summary with `issue_id`, `original_revision_id`,
`revision_id`, `replay_id`, and `stale_code`. The two final revision IDs must
match, and the stale code is `stale_revision`. Each invocation uses fresh
operation keys and creates one new issue; only its retry repeats an operation.

The program:

1. Creates rich work with criteria, a label, descriptive assignment and metadata.
2. Reads the issue and a bounded list page through the public API.
3. Replaces its full state with the expected head and an operation key.
4. Replays exactly that request and verifies the original revision is returned.
5. Attempts a different operation against the obsolete head and recognizes the
   typed stale error.

Commands use argument arrays with a ten-second deadline, bounded stdout/stderr
capture, schema/exit checks and full identifiers. Issue text is never executed.
The example's capture ceiling is 8 MiB stdout and 32 KiB stderr; it is a small
consumer demonstration, not a general-purpose large-history exporter.

Status, assignment and criteria are descriptive. Success does not authorize
execution or prove review/merge readiness. For full replacement, cursor,
conflict and retry semantics, see [issues-v1](../../docs/issues-v1.md).
