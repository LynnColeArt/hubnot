# Hosted Git compatibility

Hubnot needs an ordinary Git remote to accept, advertise, and transfer
objects reachable through `refs/hn/*`. This document records direct
observations only; it does not infer support for providers or configurations
that were not tested. Hosting UI/API state is not evidence.

These dated transport observations are distinct from optional provider API
adapters and the [current release's package runtime matrix](releases/0.1.0-beta.2.md).
Earlier `nh` commands/paths below are preserved historical observations; the
active executable and namespace are `hn`, with no legacy compatibility bridge.

| Remote | Result | Observed |
| --- | --- | --- |
| Local bare Git repository | Actor/candidate/memory refs, exact selected fetch, quarantine, budgets, and fresh-clone reconstruction passed | 2026-08-31 |
| GitHub.com private repository over HTTPS | Actor ref push, advertisement, and two-actor reconstruction passed | 2026-08-29 |
| GitHub.com public repository over HTTPS | Actor/candidate/memory publication, exact selected quarantine fetch, and identity-free fresh-clone reconstruction passed | 2026-08-31 |
| GitLab | Not tested | — |
| Other hosted providers | Not tested | — |

## Local bare Git

Automated black-box acceptance creates a fresh local bare remote and uses only
ordinary Git transport. The three-run operational scenario and focused hostile
fixtures directly establish:

- separate actor and candidate refs are advertised and exact-fetched;
- actor and candidate objects cross the remote without a Hubnot service;
- explicit selections exclude unselected refs;
- valid selections can promote while invalid, mismatched, over-budget, and
  dependency-missing selections remain absent from accepted refs;
- an identity-free depth-1 clone reconstructs exact policy, identity,
  candidate, CI, review, decision, merge, and Git commit bindings;
- selected recovery can obtain an exact missing predecessor while the
  repository remains shallow and without importing an unselected actor.

On 2026-08-31, `TestOperationalAgentMemory` separately created two actors,
recorded all six memory kinds, superseded and retracted exact records, added a
cross-actor challenge, and published the two exact memory stream refs through
`nh sync`. A credential-disabled fresh clone initially had no local or accepted
memory refs, private identity, or index. After saving only those two full stream
selectors, it synchronized through quarantine, rebuilt
`.git/nh/memory/index-v0.json`, and recalled the same full IDs, ownership,
anchors, lifecycle edges, evidence, trust classes, content digests, and inert
handoff lists. Deleting and rebuilding the index produced byte-identical index
and recall JSON. Three consecutive offline runs passed.

That local-bare observation establishes ordinary Git transport and protocol
behavior, not public-host policy or retention. Hostile memory containing shell-
and tool-shaped prose, ANSI controls, newlines, and Unicode produced no marker
effect and remained nested under the recall data boundary.

The detailed run evidence and bounds are in
[self-hosting-alpha.md](self-hosting-alpha.md).

## Local bare `hn` pre-public observation

On 2026-09-01, candidate `hn` version `0.0.1-dev`, built from
`84d82ca52b86b520cbfad7a155f789b0eb914f66`, initialized an ephemeral actor,
opened one signed issue, and synchronized it to a fresh local bare Git remote.
Ordinary `git ls-remote` advertised exactly:

```text
544b02c42d391502c18b4171071888c79f772731 refs/hn/actors/fd388bbc342c4f157ee72c4dca963bb5f1c4416af3290216b3e33134cbba9497
```

A fresh clone saved that full actor selector with positive budgets, ran
`hn sync origin`, and promoted the same OID to:

```text
refs/hn/remotes/origin/actors/fd388bbc342c4f157ee72c4dca963bb5f1c4416af3290216b3e33134cbba9497
```

`hn log` reconstructed the signed issue. Searches of both the remote
advertisement and verifier refs returned no `refs/nh/*`. The sequence used
only `git init --bare`, `hn init`, `hn issue open`, `hn sync`,
`hn replication select`, `git ls-remote`, and `git for-each-ref` over a local
file transport. This is pre-public candidate evidence, not a claim that the
final cutover commit or `refs/hn/*` have been published to GitHub.com.

## GitHub.com private observation

The 2026-08-29 test used a temporary private repository under an authenticated
personal account. GitHub accepted actor refs outside the branch and tag
namespaces:

```text
refs/nh/actors/2f2534ef...
refs/nh/actors/e675ea17...
```

Ordinary `git push`, `git ls-remote`, and explicit Git fetch established:

1. a new actor ref was accepted and advertised unchanged;
2. a fresh clone fetched actor history only after Hubnot supplied an
   explicit custom refspec;
3. a second actor published a separate signed comment history;
4. the first clone fetched it and reconstructed both verified events;
5. every event commit and parent needed by those histories transferred.

That observation preceded candidate refs and selected quarantine, so it does
not claim those newer surfaces for the private-repository configuration.

## GitHub.com public operational-alpha observation

Before the operational-alpha public mutation on 2026-08-30, ordinary Git
advertisement for `https://github.com/LynnColeArt/hubnot.git` returned:

```text
refs/heads/main e8b0955a3d10b5677fb0dfaba42580f8f4473080
refs/nh/actors/36944394addccd027292abc8183f332af8b4590925291ee8f1e8d8f09446b7dd 2b0e261037e0760f83536fedfad044e95deff1fe
refs/nh/proposals/4fd8813d5849241173eb48bbebd6858535545fa514a80b88d3fdae0e59377290 327199e686dbe0bcc4cc4bf92f10237472b083a8
```

These values were obtained with `git ls-remote`, not a provider API. Stage 1
published the two actor histories and policy candidate before separately
advancing `main` to
`08291d6f363fa68ca66b84d922d46cc8302fc9a0`. Stage 2 advanced the two actor
histories and added the release candidate before separately advancing `main`.
The final ordinary-Git advertisement was:

```text
refs/heads/main 7e41574792be828ba507e5df7adda71662475483
refs/nh/actors/36944394addccd027292abc8183f332af8b4590925291ee8f1e8d8f09446b7dd 4d9caba6548c50eba49ed89dc8ee45960226c01e
refs/nh/actors/10425b859ef6c86afe46e9586a1c26a9fe007f7a0aa38a46a3c316fcfc69a1db 8bc318bdd0a8a37e3fc55d692651811124516c14
refs/nh/proposals/4fd8813d5849241173eb48bbebd6858535545fa514a80b88d3fdae0e59377290 327199e686dbe0bcc4cc4bf92f10237472b083a8
refs/nh/proposals/52aee3a8a6fd110321b3506e9408397158d02d3dfe63747cb445effa157e0a95 a3ef855ca062c191a6c7e5c14709de67bdfb4f1c
refs/nh/proposals/122a6dc9ba587f787897395e89c38bd48aef1f68dc5d7dc35121562a1073371d 07b64dfdb11dc376644e068f201ca998bb54defe
```

A fresh HTTPS clone at depth 1 saved exact selections for the two actors and
three proposals with budgets 10,000 events, 100,000 objects, 16,777,216 bytes
per object, 1,048,576 bytes per attachment, and 268,435,456 total bytes. It
promoted 24 verified events, reconstructed the release as merged with its
exact amended policy and role-distinct evidence, and remained shallow. It had
no private keyring, legacy identity record, or local public actor ref.

The complete event, policy, object, and commit record is in
[the public proof record](self-hosting-alpha.md).

## GitHub.com public agent-memory observation

On 2026-08-31, the public endpoint
`https://github.com/LynnColeArt/hubnot.git` was exercised using ordinary
`git` and `nh` commands only. No provider API, UI state, Docker operation,
service, model API, copied keyring, or copied index participated.

At `2026-08-31T17:51:34Z`, `git ls-remote` showed `main` at
`7e41574792be828ba507e5df7adda71662475483`, with no
`proof/agent-memory-v0-20260831` branch and no advertised memory refs. The
proof then pushed only the exact new branch and exact actor/memory refs. It did
not force, delete, wildcard-push, tag, or update `main`. At
`2026-08-31T17:55:59Z`, advertisement was:

```text
fc6e3fff308658f3c1e33ab819631d6001ddf9a5 refs/heads/proof/agent-memory-v0-20260831
3f1257bd84ef354390b5f82d25c056663976bc3e refs/nh/actors/9584bdce9dcefc43a05c8dd34c77f4967419d610a28041e2da4b4fad33f726fc
c7700147f216d7edd474b33364a6ea5415c965b5 refs/nh/actors/d91339c192742a9a7676ec2da81d8e6177dbe0b07058729e6c0674a7a20e01f1
a0db69f5dbf77ecaa2f6d3a5ebe0f8e686369d51 refs/nh/memory/9584bdce9dcefc43a05c8dd34c77f4967419d610a28041e2da4b4fad33f726fc/52fa6c89cf8a92cc1be017bd1d49957f0ce0210f56386f7a57af7b9249add648
a50f4a004dcd89858b599d9ed85df7ece72173a3 refs/nh/memory/d91339c192742a9a7676ec2da81d8e6177dbe0b07058729e6c0674a7a20e01f1/aaaaa298cb8ff0bb41a6e59c899fe64e0a6d0989921e2ace7b12359c0c8d77f7
```

The branch policy digest was
`sha256:ff545e1bbe0e07176b6ee7639093f1143b17af4797dd3af165bf4d14c19d7804`.
It qualified both exact actors and all six memory kinds. The author stream
contained this decision and handoff:

```text
sha256:3579329703b5ee53d0c124e42fadcacafd9f74eb062f426c96f0ffedff211333 decision
sha256:e6113e0985758ebaaa39f130df7b26f13913fee6bf732c5b034c0c1c8ae94e1f handoff
```

The successor stream contained challenge
`sha256:f468dcea0749451404c3e21806e2cbddd438d8cf06c0c6408e7f3a94c18f3c7d`
against that exact decision, with evidence bound to proof commit
`fc6e3fff308658f3c1e33ab819631d6001ddf9a5`.

Publication of each brand-new local stream completed before its first import
snapshot knew the ref, so that first `nh sync` reported the exact new stream as
`dependency-missing` while still advertising it. After `git ls-remote`
confirmed the full ref and OID, an unchanged retry promoted every exact
selection. No accepted ref was inferred from push output.

A new `--single-branch` HTTPS clone ran with an empty isolated home,
`GIT_CONFIG_NOSYSTEM=1`, terminal prompts and askpass disabled, token variables
empty, and no SSH agent. Before selection it had no `refs/nh/*`, `.git/nh`
identity, index, embedding, adapter, or Docker state. The verifier saved these
full selectors with positive budgets:

```sh
nh replication select origin \
  --actor 9584bdce9dcefc43a05c8dd34c77f4967419d610a28041e2da4b4fad33f726fc \
  --actor d91339c192742a9a7676ec2da81d8e6177dbe0b07058729e6c0674a7a20e01f1 \
  --memory sha256:52fa6c89cf8a92cc1be017bd1d49957f0ce0210f56386f7a57af7b9249add648 \
  --memory sha256:aaaaa298cb8ff0bb41a6e59c899fe64e0a6d0989921e2ace7b12359c0c8d77f7 \
  --max-events 10000 --max-objects 30000 \
  --max-object-bytes 16777216 --max-attachment-bytes 1048576 \
  --max-total-bytes 134217728
nh sync origin
nh memory index rebuild
nh memory recall --at HEAD --json
```

All four selections reported `promoted`. Clone, synchronization, index build,
and recall took 1.35 s, 2.70 s, 0.05 s, and 0.05 s respectively on the observed
client. The index had mode `0600`, 5,805 bytes, source fingerprint
`sha256:4203a45253c628959b593929bc17bec1f7a4f622c7a22d8a87566ced5cf0df47`,
and file SHA-256
`2b14b5c3d82aaa9f98589e00f6d65fc248c74444501860e5a86723acef6ef04c`.
Deleting and rebuilding it produced the identical digest.

Recall returned exactly two qualified, applicable, active, evidence-resolved,
signature-valid records with query digest
`sha256:3db4aec9870c5ea2b792b23153e2141e7f3e79cda88129f933276b89368f21a9`.
The decision retained the full challenge ID, and the handoff retained all four
inert lists. A record attempt exited 1 with `no identity`; the verifier did not
gain either originating key.

The proof establishes exact transport and reconstruction for these two actors,
streams, branch bytes, client, endpoint, and observation time. It does not
establish distinct humans, semantic truth, prompt or operational authority,
permanent host retention, portable pre-download quotas, provider generality,
moderation, redaction, deletion, or global erasure.

## Clone behavior

An ordinary `git clone` and ordinary branch fetch do not import custom refs,
because Git configures a branch refspec by default. This is expected. After an
explicit selection, `hn sync` requests exact selected actor/candidate refs and
stores validated roots below `refs/hn/remotes/<remote>/*`.

The primary branch and collaboration refs are separate namespaces and separate
publication outcomes. A host accepting one does not imply that the other was
updated; verify both with ordinary Git:

```sh
git ls-remote --symref origin HEAD
git ls-remote origin 'refs/heads/main' 'refs/hn/actors/*' 'refs/hn/proposals/*'
git ls-remote origin 'refs/hn/memory/*/*'
```

## Limits not established by these observations

- GitHub Enterprise Server or organization-specific ref policies;
- very large actor/candidate counts, histories, or packs;
- very large memory-stream counts, histories, or packs;
- provider garbage-collection and long-term custom-ref retention;
- deletion, rollback, force-push, and global redaction behavior;
- partial-clone filters;
- portable hard pre-download quotas;
- providers other than the explicitly observed local Git and GitHub.com
  configurations.

A successful transport observation does not make the host a policy authority,
prove separate humans, or strengthen the local runner sandbox.
