---
affected_files: []
cycle_number: 7
mission_slug: optional-extensions-01M661C0
reproduction_command:
reviewed_at: '2026-10-31T13:01:58Z'
reviewer_agent: codex
wp_id: WP15
---

# Config-fixture portability audit

Source-only investigation of lane-o (`9477ea432472fb9fca7d8f338c150cca6d7a2a1a`); no Go/build/host/process probes, product edits, test edits, contract edits, or lifecycle mutations. No Darwin runtime result is asserted. Exact inspected source SHA-256 values are in `source-hashes.json` and `SHA256SUMS` beside this report. The three preflight-listed product/root-test hashes still match the supplied preflight, although its recorded source commit differs.

## Actual directory selection

`/usr/local/go/src/os/file.go:557–594` is authoritative for this installed source: Darwin/iOS use `HOME/Library/Application Support` (567–572), ignoring `XDG_CONFIG_HOME`; Unix defaults use absolute `XDG_CONFIG_HOME`, falling back to `HOME/.config` (581–590). Production `internal/extension/registration.go:179–192` correctly invokes `os.UserConfigDir` at 181 and appends `hubnot/extensions/registry.json` at 188. This is a fixture mismatch, not grounds for changing the native default or production directory policy.

`operational_acceptance_test.go:1149–1179` removes inherited HOME and explicitly overridden keys, sets fixture HOME at 1171, and appends caller overrides at 1176–1177. It has no platform branch and does not rewrite the caller's `config` variable. Its HOME isolation nevertheless lets the child CLI select its real Darwin configuration root. Merely setting XDG in a CLI-only fixture is not itself a wrong-path assertion.

## Exact affected assertions/traps and ownership

All paths in this table are relative to the inspected lane-o worktree.

| Owner | Source location | Source-level finding on Darwin |
| --- | --- | --- |
| WP13 | `extension_registration_test.go:63–64,177–187` | Config variable is `HOME/config`; registered CLI state instead belongs below `HOME/Library/Application Support`. Direct read at 178 ignores its error; JSON decode at 182–183 then fails before instance-binding checks. |
| WP13 | Same file: `220–227,240–249,255–270` | Instance corruption, incompatible-manifest replacement, restoration and byte-preservation checks all reuse the wrong `registry` variable. These remain relevant defects even though the earlier decode prevents reaching them. |
| WP13 | Same file: `361–365` | The malformed-registry arm of the native inertness trap uses that same wrong path. The accompanying `.git/hn/extensions/enabled.json` and `.hn/extensions.json` paths are repository-relative and are not affected by OS config resolution. |
| WP13 | Same file: `421–429,453,455–488` | Read-detector setup plants malformed bytes at the wrong registry (422/426). The explicit `extension inspect` control requires `invalid_input` at 455–457, and absolute/relative `cat` controls at 476–484 reuse the incorrect planted path. The detector checks basenames at 468–469; those checks need no weakening. This test first requires `strace` at 414–416 and may skip when unavailable; this audit does not claim it executes on Darwin. |
| WP13 | Same file: `501–502,552–560,589–595` | Raw-array CLI test registers via native selection, then reads the wrong file at 553 (explicitly fails its error check at 554–555); all later mutation/no-repair assertions reuse it. |
| WP16 | `extension_host_lifecycle_test.go:187,191,567–570` | Fixture config variable is `HOME/config`, while register/enable/run commands use fixture HOME and select the real native root. The one manual registry overwrite at 567 targets the wrong path; with the fixture-created directory layout it encounters an absent parent instead of corrupting the live registry. It cannot establish the intended malformed-state/native-command control. |

Ownership is explicit in `kitty-specs/optional-extensions-01M661C0/tasks/WP13-extension-registration.md:37–43` and `WP16-extension-host-lifecycle.md:42–49`. WP13 owns both registration test files and production registration; WP16 owns the root host lifecycle test. The response-completion implementation owner retains the sole heavy execution slot; this audit used none.

## Correct fixtures / exclusions

- `internal/extension/registration_test.go:14–29` returns an explicit temporary registry pathname. `Register(path,...)`, `LoadRegistry(path)`, and reads/mutations all receive that same path (e.g. 41–51,119–151). It does not call `RegistryPath` or assume the OS-selected config directory. No correction is justified there for this finding.
- `newHostFixture:186–208` passes HOME/XDG to child CLI commands; setup does not read `f.config` directly. This lets application-side `os.UserConfigDir` account for the OS. The source-level failure is the later direct use at 567, not every host setup/run test.
- Manifest/result/PID marker paths under fixture HOME and enablement/declaration paths under fixture repository are explicit locations, not OS config-root assumptions. No direct capture-storage assertion occurs in the three requested test files.

## Minimal proposed test-only correction (not applied)

Add a WP13-owned helper in `extension_registration_test.go` that obtains the actual OS root under the fixture environment:

```go
func extensionTestConfigDir(t *testing.T, home string) string {
    t.Helper()
    t.Setenv("HOME", home)
    t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
    config, err := os.UserConfigDir()
    if err != nil {
        t.Fatal(err)
    }
    return config
}
```

Replace just the three root registration-test initializations at 63, 421 and 501 with `config := extensionTestConfigDir(t, home)`, and WP16's initialization at 187 with `f.config = extensionTestConfigDir(t, f.home)`. Continue passing the returned `config` through the existing child environment and deriving direct registry reads/writes from it. Darwin ignores XDG but returns the matching fixture HOME native root; Unix keeps its existing fixture config location. These targeted tests contain no `t.Parallel`; the helper's process environment changes are restored by testing cleanup. Do not call `os.UserConfigDir` against the parent/user's ambient environment before installing fixture HOME/XDG.

Check the first registry read's error at 178 explicitly as a useful test diagnostic. Preserve invalid-input/verification-failure expectations, exact malformed-byte retention, native-effect checks, absolute/relative read controls, process detector and the explicit malformed-registry `extension inspect` negative control. Do not silently skip a case or switch the detector to merely accepting a nonexistent malformed path. Do not modify `RegistryPath`, native startup behavior, frozen contracts, or lifecycle state for this repair.

Adjacent static observation, outside this WP13/WP16 correction scope: `extension_protocol_commands_test.go:63–77` also plants a malformed registry at `HOME/config` before frame inspection; final qualification should route that fixture to its actual owner. This audit makes no further claims about that test or other config users.
