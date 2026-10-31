# Frozen released-writer corpus

Each version directory contains a complete public Git bundle and semantic JSON
observations produced by its exact release archive executable. Version 0.0.2-dev
and later include that executable's project archive. `BUILDINFO.json` (0.0.1)
or `BUILD.json` (later) is copied verbatim from the release archive.
`provenance.json` pins release source, archive and executable SHA-256, fixture
file SHA-256, historical IDs and exact ref/OID pairs. No private signer files or
executables are retained. Only new synthetic future-format actors are created
with standard-library Ed25519, independently of Hubnot's encoders; their bundles
are explicitly named `future-*` and are not historical compatibility evidence.

The archives used here are:

| Version | Standard Linux AMD64 archive SHA-256 |
| --- | --- |
| 0.0.1-dev | `7606efc4018c6afb93538f7325603854860039fc642e05f8a06529cca246d070` |
| 0.0.2-dev | `cde0c34fb8b43a461ca73d850b255e94a17be0c0ac54a14f2e42536042bd7a21` |
| 0.0.3-dev | `dfb54281846272d2fe5a84ad8c2fcd099aeadf5ad901859a857ca60f4ea8ba77` |

Release URLs are recorded per version and follow
`https://github.com/LynnColeArt/hubnot/releases/tag/vVERSION`; the corresponding
archive is `hubnot_VERSION_linux_amd64.tar.gz` on that release. The 0.0.1-dev
archive receipt was checked against GitHub's release digest and SHA256SUMS. The
0.0.2/0.0.3 archives came from retained release packaging evidence and were
checked against their published SHA256SUMS. The generator extracts the binary
from the verified archive rather than assuming a similarly named local binary
is authentic, then compares its digest/source with the embedded build record.

## Explicit generation

Generation is a maintainer operation, never part of ordinary tests. Obtain and
verify the exact release archive and checksum listing separately. From the repo
root, choose a **new** output directory and pass its released source commit and
verified archive checksum, for example:

```sh
go run testdata/compatibility/generate.go \
  -archive /path/to/hubnot_0.0.3-dev_linux_amd64.tar.gz \
  -checksums /path/to/SHA256SUMS \
  -version 0.0.3-dev \
  -source 01a152cfbb822a7be9ccbc51dac34e3caa0cf925 \
  -sha256 dfb54281846272d2fe5a84ad8c2fcd099aeadf5ad901859a857ca60f4ea8ba77 \
  -out /path/to/new-corpus-directory
```

Use the matching source/digest from each directory's provenance to reproduce
another baseline. This needs Go and Git plus an executable for the local OS/CPU;
the pinned writers here require Linux AMD64. The generator requires no Python,
provider account, network API or user keyring. It creates a private temporary
HOME/repository and Git-only PATH, invokes the released public CLI for every
historical fact and archive, and removes the entire temporary directory,
including fresh private keys and extracted executable, when it exits. Successful
output never contains private recovery sessions or signer state. Future probes
use fresh disposable keys held only in process memory.

The generator refuses an existing output directory. Fresh keys and wall-clock
signatures make repeated generation semantically equivalent, not byte-identical.
Do not replace frozen baseline files during maintenance. Add and review a new
scenario, retain its exact hashes, and explain any intended support change.
Historical file integrity is checked during both reader variants. The test
compares semantic observations as a subset so legitimate additive JSON fields
remain possible, and compares the exact Git object closure to detect rewrites.
