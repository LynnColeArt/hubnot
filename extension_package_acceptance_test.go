package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha1"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hubnot/internal/migration"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Package qualification is an explicit mode. A source-suite return is never a
// package observation. Each actual row requires the complete immutable bundle,
// separately supplied target-bound samples, and an already compiled harness.
func TestExtensionPackageQualification(t *testing.T) {
	if *portablePackageDirectory == "" && *betaPackageBundle == "" && os.Getenv("HN_WP25_REFERENCE_DIR") == "" {
		t.Log("source-suite only: no exact-package row requested")
		return
	}
	if *portablePackageDirectory == "" || *betaPackageBundle == "" || os.Getenv("HN_WP25_REFERENCE_DIR") == "" {
		t.Fatal("exact-package mode requires package directory, bundle and reference assets")
	}
	variant := os.Getenv("HN_WP25_VARIANT")
	tags := ""
	if variant == "native-only" {
		tags = "nogithub,noforge"
	} else if variant != "standard" {
		t.Fatal("exact-package variant must be standard or native-only")
	}
	// Bundle verification itself creates temporary directories, so establish
	// the physical OS temp root before it or any nested/forked fixture runs.
	wp25NormalizeQualificationTemp(t)
	wp25QualificationTempControls(t)
	// Verify every archive, BUILD, checksum, target, source and executable before
	// executing either candidate. The extracted selection independently checks
	// the executable's Go build settings as well as its transported BUILD file.
	TestBetaPlatformCandidateBundle(t)
	binary := portableJourneyBinary(t, tags)
	references := wp25References(t, os.Getenv("HN_WP25_REFERENCE_DIR"))
	if evidence := os.Getenv("HN_WP25_EVIDENCE"); evidence != "" {
		if err := os.MkdirAll(evidence, 0700); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"HN_PORTABLE_TEST_TRANSCRIPTS", "HN_CONTRIBUTION_TEST_TRANSCRIPTS", "HN_WP20_EVIDENCE", "HN_WP21_EVIDENCE", "HN_WP22_EVIDENCE"} {
			path := filepath.Join(evidence, key)
			if err := os.MkdirAll(path, 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv(key, path)
		}
	}
	incomingXDG, incomingXDGPresent := os.LookupEnv("XDG_CONFIG_HOME")
	wp25Record(t, map[string]any{"control": "qualification-fixture-config-isolation", "incoming_xdg_present": incomingXDGPresent, "incoming_xdg_value": incomingXDG, "selected_xdg_value": "", "candidate_sha256": wp25Digest(t, binary), "harness_sha256": wp25Digest(t, wp25Self(t)), "reason": "legacy fixtures isolate HOME; an empty XDG preserves each Linux fixture HOME fallback; deeper explicit OS configuration pins remain intact", "causal_evidence": "same frozen harness/candidate shared-XDG registration failure versus blank-XDG complete pass; original failed CI ambient path was not retained"})
	t.Setenv("XDG_CONFIG_HOME", "")
	if os.Getenv("HN_WP25_MISSING_REFERENCE_CONTROL") == "1" {
		t.Fatal("missing-reference control unexpectedly reached execution")
	}
	root := t.TempDir()
	marker := filepath.Join(root, "compiler-fallback")
	trap := filepath.Join(root, "go")
	if err := os.WriteFile(trap, []byte("#!/bin/sh\nprintf fallback > '"+marker+"'\nexit 97\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	// Exercise the trap once, then remove only its intentional control marker.
	if err := exec.Command("go", "version").Run(); err == nil {
		t.Fatal("compiler trap did not refuse")
	}
	if raw, err := os.ReadFile(marker); err != nil || string(raw) != "fallback" {
		t.Fatal("compiler trap positive control failed", err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"HN_WP04_BINARY", "HN_WP05_BINARY", "HN_WP13_BINARY", "HN_WP15_BINARY", "HN_WP16_BINARY", "HN_WP17_BINARY", "HN_WP18_BINARY", "HN_WP21_BINARY", "HN_WP22_BINARY"} {
		t.Setenv(key, binary)
	}
	for key, name := range map[string]string{"HN_WP20_CONSUMER": "fake-tracker", "HN_WP21_CONSUMER": "charter", "HN_WP22_CONSUMER": "inventory"} {
		t.Setenv(key, references[name])
	}
	wp25MissingReferenceControl(t)
	t.Run("native-public-journey", func(t *testing.T) { wp25NativePublicJourney(t, binary) })
	t.Run("remaining-typed-limits", func(t *testing.T) { wp25RemainingTypedLimits(t, binary, references) })
	t.Run("direct-envelope", func(t *testing.T) { wp25EnvelopeBounds(t, binary) })
	t.Run("management-limits", func(t *testing.T) { wp25ManagementLimits(t, binary) })
	t.Run("standalone-local-input-limits", func(t *testing.T) { wp25ConsumerInputLimits(t, references) })
	t.Run("selected-ref-live-bound", wp25SelectedRefLimit)
	t.Run("native-limits", wp25NativeLimits)
	t.Run("inherited-public-count-limits", func(t *testing.T) { wp25InheritedPublicCountLimits(t, binary) })
	t.Run("lifecycle-limits", func(t *testing.T) { wp25LifecycleLimits(t, binary) })
	t.Run("artifact-limits", wp25ArtifactLimits)
	t.Run("read-object-and-aggregate-limits", wp25ReadBudgets)
	t.Run("exchange-object-limit", wp25ExchangeObjectLimit)
	t.Run("artifact-parent-limit", wp25ArtifactParentLimit)
	t.Run("artifact-record-count-and-enclosing-deadline", wp25ArtifactRecordCountLimit)
	// These are real supplied-host public calls. Signed fixture construction,
	// hostile peers in this test executable and source-assisted context checks
	// retain those narrower labels; none is a standalone reference substitute.
	for _, journey := range wp25PublicJourneys {
		if journey.name == "TestExtensionFakeTrackerNativeDefaultInert" {
			t.Run(journey.name, func(t *testing.T) { wp25TopLevelFixtureJourney(t, journey.name) })
		} else {
			t.Run(journey.name, journey.run)
		}
	}
	if raw, err := os.ReadFile(marker); !os.IsNotExist(err) {
		t.Fatalf("forbidden source fallback: %v %s", err, raw)
	}
	evidence := os.Getenv("HN_WP25_EVIDENCE")
	if evidence != "" && !t.Failed() {
		if err := os.MkdirAll(evidence, 0700); err != nil {
			t.Fatal(err)
		}
		report := map[string]any{"schema": "hn.extension-package-row/1", "source_commit": strings.TrimSpace(runOperationalGit(t, ".", "rev-parse", "HEAD")), "source_tree": strings.TrimSpace(runOperationalGit(t, ".", "rev-parse", "HEAD^{tree}")), "target": runtime.GOOS + "/" + runtime.GOARCH, "harness_compiler": runtime.Version(), "qualified_git_version": strings.TrimSpace(runOperationalGit(t, ".", "--version")), "harness_sha256": wp25Digest(t, wp25Self(t)), "variant": variant, "executable": binary, "executable_sha256": wp25Digest(t, binary), "references": references, "source_fallback": false, "result": "PASS"}
		raw, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(filepath.Join(evidence, variant+"-row.json"), append(raw, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// Only the explicit package harness accepts Darwin's two OS-owned root
// aliases. Constructed aliases are refused rather than made safe by resolving
// them. No production safepath rule or inherited negative fixture is changed.
func wp25NormalizeQualificationTemp(t *testing.T) {
	t.Helper()
	incoming, present := os.LookupEnv("TMPDIR")
	lexical := os.TempDir()
	physical, observed, err := wp25PhysicalQualificationTemp(lexical)
	observed["control"] = "qualification-physical-temp-root"
	observed["before_any_tempdir"] = true
	observed["incoming_tmpdir_present"] = present
	observed["incoming_tmpdir_raw"] = incoming
	observed["goos"] = runtime.GOOS
	observed["process_id"] = os.Getpid()
	observed["error"] = fmt.Sprint(err)
	wp25Record(t, observed)
	if err != nil {
		t.Fatal("qualification temporary root is not physical or a verified OS alias", err)
	}
	t.Setenv("TMPDIR", physical)
	if os.TempDir() != physical {
		t.Fatal("qualification did not select its verified physical temporary root")
	}
}

// These controls exercise only qualification-fixture path handling. Actual
// native Darwin package journeys remain mandatory; this helper cannot replace
// them or broaden the product's unsafe-alias acceptance policy.
func wp25QualificationTempControls(t *testing.T) {
	t.Helper()
	selected := os.TempDir()
	physical, rootObservation, err := wp25PhysicalQualificationTemp(selected)
	if err != nil || physical != selected || rootObservation["same_directory"] != true {
		t.Fatal("selected qualification temporary root is not physical", selected, physical, err)
	}
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || root != resolved {
		t.Fatal("qualification TempDir retained a lexical alias", root, resolved, err)
	}
	relative, err := filepath.Rel(selected, root)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		t.Fatal("qualification TempDir escaped the selected physical root", selected, root, err)
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "constructed-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	followed, err := os.Stat(alias)
	if err != nil || !os.SameFile(rootInfo, followed) {
		t.Fatal("constructed alias control does not select the same directory", err)
	}
	got, aliasObservation, aliasErr := wp25PhysicalQualificationTemp(alias)
	wp25Record(t, map[string]any{"control": "qualification-constructed-temp-alias-refusal", "selected_tmpdir": selected, "created_tempdir": root, "created_tempdir_physical": resolved, "created_directory_mode_raw": uint32(rootInfo.Mode()), "created_directory_sys_raw": fmt.Sprintf("%#v", rootInfo.Sys()), "constructed_alias": alias, "constructed_target_same_directory": os.SameFile(rootInfo, followed), "unexpected_selected_root": got, "helper_observation": aliasObservation, "error": fmt.Sprint(aliasErr), "classification": "source-assisted harness path control; not product alias acceptance or an actual Darwin package substitute"})
	if aliasErr == nil || got != "" || !strings.HasPrefix(aliasErr.Error(), "unverified temporary root alias:") {
		t.Fatal("constructed alias was normalized or refused for the wrong cause", got, aliasErr)
	}
	after, err := os.Stat(alias)
	if err != nil || !os.SameFile(rootInfo, after) {
		t.Fatal("refused alias inspection changed its target identity", err)
	}
	link, err := os.Readlink(alias)
	if err != nil || link != root {
		t.Fatal("refused alias inspection changed its link", link, err)
	}
	got, physicalObservation, err := wp25PhysicalQualificationTemp(root)
	wp25Record(t, map[string]any{"control": "qualification-physical-temp-positive", "selected_tmpdir": selected, "fixture_root": root, "physical_result": got, "helper_observation": physicalObservation, "error": fmt.Sprint(err), "classification": "source-assisted harness path control; subsequent exact candidate/reference paths remain independently verified"})
	if err != nil || got != root || physicalObservation["same_directory"] != true {
		t.Fatal("physical fixture root refused after constructed-alias negative", got, err)
	}
	if runtime.GOOS == "darwin" {
		for _, lexical := range []string{"/var", "/tmp"} {
			info, err := os.Lstat(lexical)
			if os.IsNotExist(err) {
				wp25Record(t, map[string]any{"control": "qualification-darwin-system-temp-alias", "lexical_path": lexical, "present": false, "alias_observed": false, "scope": "actual OS path absent; no alias-positive or product proof inferred"})
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			aliasPresent := info.Mode()&os.ModeSymlink != 0
			got, observed, err := wp25PhysicalQualificationTemp(lexical)
			wp25Record(t, map[string]any{"control": "qualification-darwin-system-temp-alias", "lexical_path": lexical, "present": true, "alias_observed": aliasPresent, "physical_result": got, "helper_observation": observed, "error": fmt.Sprint(err), "goos": runtime.GOOS, "scope": "actual Darwin OS-root helper check only; actual native package/physical-path marker journeys still required"})
			if err != nil || observed["same_directory"] != true {
				t.Fatal("actual Darwin system temporary path failed identity/owner/mode verification", lexical, err)
			}
			if aliasPresent && got != map[string]string{"/var": "/private/var", "/tmp": "/private/tmp"}[lexical] {
				t.Fatal("actual Darwin system temporary alias selected another target", lexical, got)
			}
		}
	} else {
		wp25Record(t, map[string]any{"control": "qualification-darwin-system-temp-alias-scope", "goos": runtime.GOOS, "actual_darwin_alias_observation": false, "scope": "constructed-alias and physical-root helper controls only; no Darwin package claim"})
	}
}

func TestExtensionPackagePhysicalTempFixtureControls(t *testing.T) {
	wp25NormalizeQualificationTemp(t)
	wp25QualificationTempControls(t)
	wp25Record(t, map[string]any{"control": "qualification-temp-fixture-helper-test", "source_assisted": true, "exact_package_or_platform_pass": false, "scope": "same maintained helper; actual native package journeys remain independently required"})
}

func wp25PhysicalQualificationTemp(lexical string) (string, map[string]any, error) {
	observed := map[string]any{"lexical_temp_raw": lexical, "inspection_creates_no_tempdir": true}
	fail := func(err error) (string, map[string]any, error) { return "", observed, err }
	clean := filepath.Clean(lexical)
	trimmed := strings.TrimRight(lexical, string(os.PathSeparator))
	if trimmed == "" {
		trimmed = string(os.PathSeparator)
	}
	if !filepath.IsAbs(lexical) || clean != trimmed {
		return fail(fmt.Errorf("noncanonical temporary root %q", lexical))
	}
	observed["lexical_temp_clean"] = clean
	stat := func(info os.FileInfo) map[string]any {
		value := map[string]any{"name": info.Name(), "mode_raw": uint32(info.Mode()), "mode_string": info.Mode().String(), "size": info.Size(), "sys_raw": fmt.Sprintf("%#v", info.Sys())}
		if native, ok := info.Sys().(*syscall.Stat_t); ok {
			value["uid"], value["gid"] = native.Uid, native.Gid
			value["device"], value["inode"] = native.Dev, native.Ino
		}
		return value
	}
	lexicalInfo, err := os.Stat(clean)
	if err != nil || !lexicalInfo.IsDir() {
		return fail(fmt.Errorf("temporary root directory: %v", err))
	}
	observed["lexical_directory_stat"] = stat(lexicalInfo)
	current := string(os.PathSeparator)
	aliases := []map[string]any{}
	for _, component := range strings.Split(strings.TrimPrefix(clean, current), string(os.PathSeparator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return fail(err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			if !info.IsDir() {
				return fail(fmt.Errorf("temporary root component is not a directory: %s", current))
			}
			continue
		}
		link, err := os.Readlink(current)
		if err != nil {
			return fail(err)
		}
		target := link
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(current), target)
		}
		target = filepath.Clean(target)
		alias := map[string]any{"lexical_path": current, "link_target_raw": link, "target_path": target, "link_stat": stat(info)}
		aliases = append(aliases, alias)
		observed["aliases"] = aliases
		expected := map[string]string{"/var": "/private/var", "/tmp": "/private/tmp"}[current]
		linkStat, ok := info.Sys().(*syscall.Stat_t)
		if runtime.GOOS != "darwin" || expected == "" || target != expected || !ok || linkStat.Uid != 0 {
			return fail(fmt.Errorf("unverified temporary root alias: %s", current))
		}
		// EvalSymlinks on the expected OS target must not conceal another alias.
		resolvedTarget, err := filepath.EvalSymlinks(target)
		targetInfo, targetErr := os.Lstat(target)
		if err != nil || targetErr != nil {
			return fail(fmt.Errorf("OS temporary alias target: %v %v", err, targetErr))
		}
		alias["target_stat"] = stat(targetInfo)
		targetStat, ok := targetInfo.Sys().(*syscall.Stat_t)
		expectedMode := os.FileMode(0755)
		if current == "/tmp" {
			// The ordinary Darwin system temp directory is root-owned sticky1777.
			// Its exact OS alias is allowed; an arbitrary writable alias is not.
			expectedMode = 0777 | os.ModeSticky
		}
		mode := targetInfo.Mode() & (os.ModePerm | os.ModeSticky | os.ModeSetuid | os.ModeSetgid)
		alias["expected_target_mode_raw"] = uint32(expectedMode)
		if resolvedTarget != target || !targetInfo.IsDir() || targetInfo.Mode()&os.ModeSymlink != 0 || mode != expectedMode || !ok || targetStat.Uid != 0 {
			return fail(fmt.Errorf("unsafe OS temporary alias target: %s", target))
		}
		followed, err := os.Stat(current)
		alias["same_target_directory"] = err == nil && os.SameFile(followed, targetInfo)
		if err != nil || !os.SameFile(followed, targetInfo) {
			return fail(fmt.Errorf("OS temporary alias target identity changed: %s", current))
		}
		current = target
	}
	physical, err := filepath.EvalSymlinks(clean)
	if err != nil || physical != current {
		return fail(fmt.Errorf("temporary root changed during verification: %v", err))
	}
	physicalInfo, err := os.Stat(physical)
	if err != nil {
		return fail(err)
	}
	observed["physical_temp"] = physical
	observed["physical_directory_stat"] = stat(physicalInfo)
	observed["same_directory"] = os.SameFile(lexicalInfo, physicalInfo)
	observed["aliases"] = aliases
	if !physicalInfo.IsDir() || !os.SameFile(lexicalInfo, physicalInfo) {
		return fail(fmt.Errorf("temporary root alias changes directory identity"))
	}
	return physical, observed, nil
}

type wp25ReferenceManifest struct {
	Schema       string `json:"schema"`
	SourceCommit string `json:"source_commit"`
	SourceTree   string `json:"source_tree"`
	Compiler     string `json:"compiler"`
	Assets       []struct {
		Name, Target, Sample, Package, SHA256 string
		BuildInfoSHA256                       string `json:"build_info_sha256"`
	} `json:"assets"`
}

func wp25Digest(t *testing.T, path string) string {
	t.Helper()
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		t.Fatal(e)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
func wp25Self(t *testing.T) string {
	t.Helper()
	p, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func wp25References(t *testing.T, root string) map[string]string {
	t.Helper()
	var m wp25ReferenceManifest
	raw, e := os.ReadFile(filepath.Join(root, "manifest.json"))
	if e != nil {
		t.Fatalf("missing exact-source reference manifest: %v", e)
	}
	if e = json.Unmarshal(raw, &m); e != nil {
		t.Fatal(e)
	}
	head := strings.TrimSpace(runOperationalGit(t, ".", "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runOperationalGit(t, ".", "rev-parse", "HEAD^{tree}"))
	if m.Schema != "hn.extension-references/1" || m.SourceCommit != head || m.SourceTree != tree || m.Compiler != "go1.26.4" || len(m.Assets) != 12 {
		t.Fatal("reference manifest source/tree/compiler/full matrix mismatch")
	}
	seen := map[string]bool{}
	selected := map[string]string{}
	for _, a := range m.Assets {
		samplePath := map[string]string{"fake-tracker": "./examples/extensions/fake-tracker", "charter": "./examples/extensions/charter-model", "inventory": "./examples/extensions/inventory-model"}[a.Sample]
		validTarget := a.Target == "linux/amd64" || a.Target == "linux/arm64" || a.Target == "darwin/amd64" || a.Target == "darwin/arm64"
		if !validTarget || samplePath == "" || a.Package != samplePath || a.Name != strings.ReplaceAll(a.Target, "/", "_")+"/"+a.Sample || seen[a.Name] {
			t.Fatal("reference asset inventory mismatch", a.Name)
		}
		seen[a.Name] = true
		name := filepath.Join(root, filepath.FromSlash(a.Name))
		st, e := os.Lstat(name)
		if e != nil {
			t.Fatalf("missing exact-source reference asset %s: %v", a.Name, e)
		}
		if !st.Mode().IsRegular() || st.Mode()&0111 == 0 || wp25Digest(t, name) != a.SHA256 || wp25Digest(t, name+".buildinfo") != a.BuildInfoSHA256 {
			t.Fatal("reference file/hash mismatch", a.Name)
		}
		info, e := buildinfo.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		settings := map[string]string{}
		for _, v := range info.Settings {
			settings[v.Key] = v.Value
		}
		if info.GoVersion != m.Compiler || settings["GOOS"]+"/"+settings["GOARCH"] != a.Target || settings["CGO_ENABLED"] != "0" || info.Path != "hubnot/"+strings.TrimPrefix(samplePath, "./") || settings["-trimpath"] != "true" {
			t.Fatal("reference executable compiler/target/package/settings mismatch", a.Name, info)
		}
		if a.Target == runtime.GOOS+"/"+runtime.GOARCH {
			selected[a.Sample] = name
		}
		t.Logf("reference asset=%s sha256=%s source=%s tree=%s compiler=%s", a.Name, a.SHA256, m.SourceCommit, m.SourceTree, m.Compiler)
	}
	if len(selected) != 3 {
		t.Fatal("missing native reference assets")
	}
	return selected
}
func wp25MissingReferenceControl(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	raw, e := os.ReadFile(filepath.Join(os.Getenv("HN_WP25_REFERENCE_DIR"), "manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "manifest.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(wp25Self(t), "-test.run=^TestExtensionPackageMissingReferenceProbe$", "-test.v")
	cmd.Env = append(os.Environ(), "HN_WP25_MISSING_REFERENCE_CONTROL=1", "HN_WP25_REFERENCE_DIR="+root)
	out, e := cmd.CombinedOutput()
	t.Logf("missing-asset negative control: %v\n%s", e, out)
	if e == nil || !bytes.Contains(out, []byte("missing exact-source reference asset")) {
		t.Fatal("missing reference did not refuse for the asserted cause")
	}
}
func TestExtensionPackageMissingReferenceProbe(t *testing.T) {
	if os.Getenv("HN_WP25_MISSING_REFERENCE_CONTROL") == "1" {
		wp25References(t, os.Getenv("HN_WP25_REFERENCE_DIR"))
	}
}

type wp25Journey struct {
	name string
	run  func(*testing.T)
}

var wp25PublicJourneys = []wp25Journey{
	{"TestArtifactStoreHistoricalNativeBytesCLI", TestArtifactStoreHistoricalNativeBytesCLI},
	{"TestArtifactStoreIntentLockAndUncertainCLI", TestArtifactStoreIntentLockAndUncertainCLI},
	{"TestArtifactStoreCASRaceCLI", TestArtifactStoreCASRaceCLI},
	{"TestArtifactStoreIntentStageContractCLI", TestArtifactStoreIntentStageContractCLI},
	{"TestArtifactStorePrivateIntentByteBudgetCLI", TestArtifactStorePrivateIntentByteBudgetCLI},
	{"TestArtifactStoreStrictIntentCLI", TestArtifactStoreStrictIntentCLI},
	{"TestArtifactStoreExactRetryRecoveryCLI", TestArtifactStoreExactRetryRecoveryCLI},
	{"TestArtifactStoreStrictNestedIntentCLI", TestArtifactStoreStrictNestedIntentCLI},
	{"TestArtifactStoreIntentPayloadShapeCLI", TestArtifactStoreIntentPayloadShapeCLI},
	{"TestArtifactCatalogCollectionAndCycleCLI", TestArtifactCatalogCollectionAndCycleCLI},
	{"TestArtifactClosureBeyondExchangeAncestryCLI", TestArtifactClosureBeyondExchangeAncestryCLI},
	{"TestArtifactSchemaCommands", TestArtifactSchemaCommands},
	{"TestArtifactStorePublishCLI", TestArtifactStorePublishCLI},
	{"TestArtifactStoreOpenAndRefusalsCLI", TestArtifactStoreOpenAndRefusalsCLI},
	{"TestArtifactStoreSelectedPolicyCLI", TestArtifactStoreSelectedPolicyCLI},
	{"TestArtifactStoreTypeEvolutionCLI", TestArtifactStoreTypeEvolutionCLI},
	{"TestArtifactStoreSHA256GitCLI", TestArtifactStoreSHA256GitCLI},
	{"TestArtifactStoreAcceptedReadCLI", TestArtifactStoreAcceptedReadCLI},
	{"TestArtifactStoreMissingObjectNoFetchCLI", TestArtifactStoreMissingObjectNoFetchCLI},
	{"TestArtifactStoreLargeVerifiedParentContextCLI", TestArtifactStoreLargeVerifiedParentContextCLI},
	{"TestArtifactStoreConflictingActorGitHistoriesCLI", TestArtifactStoreConflictingActorGitHistoriesCLI},
	{"TestArtifactStoreCorruptGitObjectCLI", TestArtifactStoreCorruptGitObjectCLI},
	{"TestArtifactStoreKeylessReadCLI", TestArtifactStoreKeylessReadCLI},
	{"TestArtifactStoreOpeningReplayObservesImportedHeadsCLI", TestArtifactStoreOpeningReplayObservesImportedHeadsCLI},
	{"TestArtifactStoreExplicitPublicSources", TestArtifactStoreExplicitPublicSources},
	{"TestArtifactStoreSourceChangesDuringRead", TestArtifactStoreSourceChangesDuringRead},
	{"TestArtifactStoreCountPreflightGraphCLI", TestArtifactStoreCountPreflightGraphCLI},
	{"TestArtifactStoreFullReaderProtocolCLI", TestArtifactStoreFullReaderProtocolCLI},
	{"TestArtifactFullReaderOwnerFinalizationCLI", TestArtifactFullReaderOwnerFinalizationCLI},
	{"TestArtifactFullReaderCancellationCLI", TestArtifactFullReaderCancellationCLI},
	{"TestArtifactFullReaderOneShotLifecycle", TestArtifactFullReaderOneShotLifecycle},
	{"TestArtifactIOPipeOwnership", TestArtifactIOPipeOwnership},
	{"TestArtifactFullReaderFirstErrorCLI", TestArtifactFullReaderFirstErrorCLI},
	{"TestArtifactBlobWriterProtocolCLI", TestArtifactBlobWriterProtocolCLI},
	{"TestArtifactBlobWriterUnreachablePublicCLI", TestArtifactBlobWriterUnreachablePublicCLI},
	{"TestArtifactStoreCountPreflightBatchProtocolCLI", TestArtifactStoreCountPreflightBatchProtocolCLI},
	{"TestArtifactStoreCountPreflightBeforeBodyCLI", TestArtifactStoreCountPreflightBeforeBodyCLI},
	{"TestArtifactStoreCountPreflightUniqueCommitBytesCLI", TestArtifactStoreCountPreflightUniqueCommitBytesCLI},
	{"TestArtifactStoreCountPreflightRawParentCLI", TestArtifactStoreCountPreflightRawParentCLI},
	{"TestArtifactStoreCountPreflightCancellationCLI", TestArtifactStoreCountPreflightCancellationCLI},
	{"TestArtifactClosureCausalRefusalsCLI", TestArtifactClosureCausalRefusalsCLI},
	{"TestArtifactClosureMissingGitObjectCLI", TestArtifactClosureMissingGitObjectCLI},
	{"TestArtifactClosurePublicOnlyReconstructionCLI", TestArtifactClosurePublicOnlyReconstructionCLI},
	{"TestArtifactClosureSelectorRefusalsCLI", TestArtifactClosureSelectorRefusalsCLI},
	{"TestArtifactClosureSelectedPrefixCLI", TestArtifactClosureSelectedPrefixCLI},
	{"TestArtifactClosureLogicalConflictAndCycleCLI", TestArtifactClosureLogicalConflictAndCycleCLI},
	{"TestArtifactClosureConversionDependenciesCLI", TestArtifactClosureConversionDependenciesCLI},
	{"TestArtifactGitIndependentClonesCLI", TestArtifactGitIndependentClonesCLI},
	{"TestArtifactGitCausalRefusalsCLI", TestArtifactGitCausalRefusalsCLI},
	{"TestArtifactGitSelectorRefusalsCLI", TestArtifactGitSelectorRefusalsCLI},
	{"TestArtifactGitLogicalHeadsAndOrderCLI", TestArtifactGitLogicalHeadsAndOrderCLI},
	{"TestArtifactGitAncestryLimitCLI", TestArtifactGitAncestryLimitCLI},
	{"TestArtifactGitPolicyBeforeEffectsCLI", TestArtifactGitPolicyBeforeEffectsCLI},
	{"TestArtifactGitSHA256CLI", TestArtifactGitSHA256CLI},
	{"TestArtifactGitWireBudgetBeforeCopyCLI", TestArtifactGitWireBudgetBeforeCopyCLI},
	{"TestArtifactGitDispatchedPushFailureIsUncertainCLI", TestArtifactGitDispatchedPushFailureIsUncertainCLI},
	{"TestArtifactGitIndependentNativeSelectionBudgetCLI", TestArtifactGitIndependentNativeSelectionBudgetCLI},
	{"TestArtifactGitFrozenRawBodiesCLI", TestArtifactGitFrozenRawBodiesCLI},
	{"TestArtifactGitFetchNamedAcceptedNamespaceCLI", TestArtifactGitFetchNamedAcceptedNamespaceCLI},
	{"TestArtifactGitFetchNamedNamespaceRace", TestArtifactGitFetchNamedNamespaceRace},
	{"TestArtifactGitNamedTransactionStaleOldIsAtomic", TestArtifactGitNamedTransactionStaleOldIsAtomic},
	{"TestArtifactGitFetchAcknowledgementCLI", TestArtifactGitFetchAcknowledgementCLI},
	{"TestArtifactExchangeIndependentCLI", TestArtifactExchangeIndependentCLI},
	{"TestArtifactExchangeRefusalsCLI", TestArtifactExchangeRefusalsCLI},
	{"TestArtifactExchangeOpaqueCLI", TestArtifactExchangeOpaqueCLI},
	{"TestArtifactExchangeSelectorRefusalsCLI", TestArtifactExchangeSelectorRefusalsCLI},
	{"TestArtifactExchangeWireBoundCLI", TestArtifactExchangeWireBoundCLI},
	{"TestArtifactExchangeMissingSchemaCLI", TestArtifactExchangeMissingSchemaCLI},
	{"TestArtifactExchangeShapesAndHeadsCLI", TestArtifactExchangeShapesAndHeadsCLI},
	{"TestArtifactExchangeCASWrongOldCLI", TestArtifactExchangeCASWrongOldCLI},
	{"TestArtifactExchangeOpaqueAggregateAndLockCLI", TestArtifactExchangeOpaqueAggregateAndLockCLI},
	{"TestArtifactExchangeDepthLiveCLI", TestArtifactExchangeDepthLiveCLI},
	{"TestArtifactExchangeSelectedPolicyCLI", TestArtifactExchangeSelectedPolicyCLI},
	{"TestArtifactExchangeMalformedSignaturePreservationCLI", TestArtifactExchangeMalformedSignaturePreservationCLI},
	{"TestArtifactExchangeLostAcknowledgementCLI", TestArtifactExchangeLostAcknowledgementCLI},
	{"TestArtifactExchangeFrozenTransportBodiesCLI", TestArtifactExchangeFrozenTransportBodiesCLI},
	{"TestArtifactExchangeRefAliasesBeyondObjectCountCLI", TestArtifactExchangeRefAliasesBeyondObjectCountCLI},
	{"TestArtifactExchangeUnreachableObjectsCLI", TestArtifactExchangeUnreachableObjectsCLI},
	{"TestArtifactExchangeOpaqueRawCommitsAreNotSignedRecordsCLI", TestArtifactExchangeOpaqueRawCommitsAreNotSignedRecordsCLI},
	{"TestArtifactExchangeNamedAcceptedRefIsolationCLI", TestArtifactExchangeNamedAcceptedRefIsolationCLI},
	{"TestArtifactRevisionLiveCLI", TestArtifactRevisionLiveCLI},
	{"TestArtifactRevisionIndependentActorsCLI", TestArtifactRevisionIndependentActorsCLI},
	{"TestArtifactRevisionCausalRefusalsCLI", TestArtifactRevisionCausalRefusalsCLI},
	{"TestArtifactRevisionEnvelopeBytesCLI", TestArtifactRevisionEnvelopeBytesCLI},
	{"TestArtifactRevisionResumeOriginalCandidateCLI", TestArtifactRevisionResumeOriginalCandidateCLI},
	{"TestArtifactOperationReadOnlyCLI", TestArtifactOperationReadOnlyCLI},
	{"TestArtifactOperationPublishedLookupAndRetryCLI", TestArtifactOperationPublishedLookupAndRetryCLI},
	{"TestArtifactOperationFoundAndSourceCutCLI", TestArtifactOperationFoundAndSourceCutCLI},
	{"TestArtifactOperationWriterEvidenceCLI", TestArtifactOperationWriterEvidenceCLI},
	{"TestArtifactOperationNewWriterCLI", TestArtifactOperationNewWriterCLI},
	{"TestArtifactOperationRefusalsAndBoundsCLI", TestArtifactOperationRefusalsAndBoundsCLI},
	{"TestArtifactOperationInterruptedRestartCLI", TestArtifactOperationInterruptedRestartCLI},
	{"TestArtifactOperationInvalidPublicHistoryCLI", TestArtifactOperationInvalidPublicHistoryCLI},
	{"TestArtifactOperationJournalFailureCLI", TestArtifactOperationJournalFailureCLI},
	{"TestArtifactOperationCurrentConflictAndPublicReconstructionCLI", TestArtifactOperationCurrentConflictAndPublicReconstructionCLI},
	{"TestArtifactListLiveCLI", TestArtifactListLiveCLI},
	{"TestArtifactLocalHelpAndHumanListCLI", TestArtifactLocalHelpAndHumanListCLI},
	{"TestArtifactListExactPageBoundCLI", TestArtifactListExactPageBoundCLI},
	{"TestArtifactSearchHelpAndPublicOnlyReaderCLI", TestArtifactSearchHelpAndPublicOnlyReaderCLI},
	{"TestArtifactSearchPublicCLI", TestArtifactSearchPublicCLI},
	{"TestArtifactSearchPagingAndRefusalsCLI", TestArtifactSearchPagingAndRefusalsCLI},
	{"TestArtifactSearchEntireSelectionBudgetCLI", TestArtifactSearchEntireSelectionBudgetCLI},
	{"TestArtifactSearchOversizedPageCLI", TestArtifactSearchOversizedPageCLI},
	{"TestArtifactSearchConflictAndUnselectedCLI", TestArtifactSearchConflictAndUnselectedCLI},
	{"TestArtifactSearchInertConsumerControlCLI", TestArtifactSearchInertConsumerControlCLI},
	{"TestArtifactFullPublicJourneyCLI", TestArtifactFullPublicJourneyCLI},
	{"TestArtifactCombinedOpaqueAndNativeCLI", TestArtifactCombinedOpaqueAndNativeCLI},
	{"TestArtifactCombinedFetchLostAcknowledgementCLI", TestArtifactCombinedFetchLostAcknowledgementCLI},
	{"TestExtensionRegistrationCLI", TestExtensionRegistrationCLI},
	{"TestExtensionRegistryRawArrayTypesCLI", TestExtensionRegistryRawArrayTypesCLI},
	{"TestExtensionArtifactReadGrantCLI", TestExtensionArtifactReadGrantCLI},
	{"TestExtensionEnablementCLI", TestExtensionEnablementCLI},
	{"TestExtensionFrameInspectCLI", TestExtensionFrameInspectCLI},
	{"TestExtensionFrameInspectPartialDeadlineCLI", TestExtensionFrameInspectPartialDeadlineCLI},
	{"TestExtensionFrameInputRestoresStdin", TestExtensionFrameInputRestoresStdin},
	{"TestExtensionRunCLI", TestExtensionRunCLI},
	{"TestExtensionRunRefusalsCLI", TestExtensionRunRefusalsCLI},
	{"TestExtensionRunDisableAndSlotsCLI", TestExtensionRunDisableAndSlotsCLI},
	{"TestExtensionRunNoImplicitLaunchCLI", TestExtensionRunNoImplicitLaunchCLI},
	{"TestExtensionRunStartupDeadlineCLI", TestExtensionRunStartupDeadlineCLI},
	{"TestExtensionRunRemoveAcrossRepositoriesCLI", TestExtensionRunRemoveAcrossRepositoriesCLI},
	{"TestExtensionRunIdleDoesNotHoldNativeLockCLI", TestExtensionRunIdleDoesNotHoldNativeLockCLI},
	{"TestExtensionRunIdleDeadlineCLI", TestExtensionRunIdleDeadlineCLI},
	{"TestExtensionRunCrashCleansPipeOwningDescendantCLI", TestExtensionRunCrashCleansPipeOwningDescendantCLI},
	{"TestExtensionNativeIdentityMissingCLI", TestExtensionNativeIdentityMissingCLI},
	{"TestExtensionNativeSixFamiliesCLI", TestExtensionNativeSixFamiliesCLI},
	{"TestExtensionNativeGrantDenialsCLI", TestExtensionNativeGrantDenialsCLI},
	{"TestExtensionNativeGovernedSixFamiliesCLI", TestExtensionNativeGovernedSixFamiliesCLI},
	{"TestExtensionNativePendingAndPolicyRefusalCLI", TestExtensionNativePendingAndPolicyRefusalCLI},
	{"TestExtensionNativeSelectedCutAndPagingCLI", TestExtensionNativeSelectedCutAndPagingCLI},
	{"TestExtensionNativeInvalidHistoryCLI", TestExtensionNativeInvalidHistoryCLI},
	{"TestExtensionNativeCancellationSettlementCLI", TestExtensionNativeCancellationSettlementCLI},
	{"TestExtensionNativeConflictAndResolveGrantCLI", TestExtensionNativeConflictAndResolveGrantCLI},
	{"TestExtensionNativeUnboundAndUnsupportedFramesCLI", TestExtensionNativeUnboundAndUnsupportedFramesCLI},
	{"TestExtensionNativeMissingSignerDoesNotInitializeCLI", TestExtensionNativeMissingSignerDoesNotInitializeCLI},
	{"TestExtensionNativeBusyLockBeforeWriterCLI", TestExtensionNativeBusyLockBeforeWriterCLI},
	{"TestExtensionNativePrivateReservationLookupCLI", TestExtensionNativePrivateReservationLookupCLI},
	{"TestExtensionArtifactTypeReadRawCLI", TestExtensionArtifactTypeReadRawCLI},
	{"TestExtensionArtifactModelsRawCLI", TestExtensionArtifactModelsRawCLI},
	{"TestExtensionArtifactGrantRefusalsRawCLI", TestExtensionArtifactGrantRefusalsRawCLI},
	{"TestExtensionArtifactSearchBoundsAndCursorRawCLI", TestExtensionArtifactSearchBoundsAndCursorRawCLI},
	{"TestExtensionArtifactFullResponseAtAndOverRawCLI", TestExtensionArtifactFullResponseAtAndOverRawCLI},
	{"TestExtensionArtifactOpaqueRawCLI", TestExtensionArtifactOpaqueRawCLI},
	{"TestExtensionArtifactInterruptedRecoveryRawCLI", TestExtensionArtifactInterruptedRecoveryRawCLI},
	{"TestExtensionArtifactPolicyAndValidationRawCLI", TestExtensionArtifactPolicyAndValidationRawCLI},
	{"TestExtensionArtifactConflictRawCLI", TestExtensionArtifactConflictRawCLI},
	{"TestExtensionArtifactNoReadGrantRawCLI", TestExtensionArtifactNoReadGrantRawCLI},
	{"TestExtensionArtifactSelectedTypeCutRawCLI", TestExtensionArtifactSelectedTypeCutRawCLI},
	{"TestExtensionArtifactEmptyNamespaceCutRawCLI", TestExtensionArtifactEmptyNamespaceCutRawCLI},
	{"TestExtensionArtifactPostProjectionSourceRecheckRawCLI", TestExtensionArtifactPostProjectionSourceRecheckRawCLI},
	{"TestExtensionArtifactReplayIntentFieldsRawCLI", TestExtensionArtifactReplayIntentFieldsRawCLI},
	{"TestExtensionChangesRawEmptyCapture", TestExtensionChangesRawEmptyCapture},
	{"TestExtensionChangesPagingAndDurableNativeAppend", TestExtensionChangesPagingAndDurableNativeAppend},
	{"TestExtensionChangesStalePageAndSafeRescan", TestExtensionChangesStalePageAndSafeRescan},
	{"TestExtensionChangesCustomAndMixedAppend", TestExtensionChangesCustomAndMixedAppend},
	{"TestExtensionChangesAuthorizationAndVerifierLoss", TestExtensionChangesAuthorizationAndVerifierLoss},
	{"TestExtensionChangesUnsupportedCustomPolicyBeforeState", TestExtensionChangesUnsupportedCustomPolicyBeforeState},
	{"TestExtensionChangesInvalidHistoryAndQuarantine", TestExtensionChangesInvalidHistoryAndQuarantine},
	{"TestExtensionChangesGovernedTwoActorVisibility", TestExtensionChangesGovernedTwoActorVisibility},
	{"TestExtensionChangesRawEntryCapacity", TestExtensionChangesRawEntryCapacity},
	{"TestExtensionChangesRawCanonicalCapacity", TestExtensionChangesRawCanonicalCapacity},
	{"TestExtensionChangesRealTenMinuteExpiry", TestExtensionChangesRealTenMinuteExpiry},
	{"TestExtensionChangesRawWireBoundary", TestExtensionChangesRawWireBoundary},
	{"TestExtensionChangesRawTypedBoundaries", TestExtensionChangesRawTypedBoundaries},
	{"TestExtensionChangesNativeDefaultRemainsInert", TestExtensionChangesNativeDefaultRemainsInert},
	{"TestExtensionChangesVerifiedHistoryRewindInvalidates", TestExtensionChangesVerifiedHistoryRewindInvalidates},
	{"TestExtensionChangesAllVisibleCommentRevisions", TestExtensionChangesAllVisibleCommentRevisions},
	{"TestExtensionChangesSourceRaceDuringPage", TestExtensionChangesSourceRaceDuringPage},
	{"TestExtensionFakeTrackerStandaloneRecovery", TestExtensionFakeTrackerStandaloneRecovery},
	{"TestExtensionFakeTrackerReplacementDisconnectAndInvalidation", TestExtensionFakeTrackerReplacementDisconnectAndInvalidation},
	{"TestExtensionFakeTrackerConcurrentEditsRefuseWithoutResolution", TestExtensionFakeTrackerConcurrentEditsRefuseWithoutResolution},
	{"TestExtensionFakeTrackerRawGrantRepositoryAndPrivacyControls", TestExtensionFakeTrackerRawGrantRepositoryAndPrivacyControls},
	{"TestExtensionFakeTrackerNativeDefaultInert", TestExtensionFakeTrackerNativeDefaultInert},
	{"TestExtensionFakeTrackerRecoveryKeepsOriginalContentAndParents", TestExtensionFakeTrackerRecoveryKeepsOriginalContentAndParents},
	{"TestExtensionFakeTrackerRejectsHostResponseFaults", TestExtensionFakeTrackerRejectsHostResponseFaults},
	{"TestExtensionFakeTrackerConsumedUncertaintyRecovery", TestExtensionFakeTrackerConsumedUncertaintyRecovery},
	{"TestExtensionCharterStandaloneJourney", TestExtensionCharterStandaloneJourney},
	{"TestExtensionCharterMissingRequiredType", TestExtensionCharterMissingRequiredType},
	{"TestInventoryStandalonePublicJourney", TestInventoryStandalonePublicJourney},
	{"TestInventoryMissingRequiredSchemaPublicExchange", TestInventoryMissingRequiredSchemaPublicExchange},
	{"TestInventoryStructuredFieldBounds", TestInventoryStructuredFieldBounds},
	{"TestInventorySuppliedConsumerResponseFaultsStopBeforeEffects", TestInventorySuppliedConsumerResponseFaultsStopBeforeEffects},
}

func wp25EnvelopeBounds(t *testing.T, binary string) {
	type boundary struct {
		name   string
		at     int
		code   string
		change func(map[string]any, int)
	}
	record := func(m map[string]any) {
		m["kind"] = "record.open"
		m["payload"] = map[string]any{"shape": "document", "type_id": "example/note", "type_revision": artifactVectorID, "title": "title", "content": "", "fields": map[string]any{}, "members": []any{}, "relationships": []any{}, "conversion_sources": []any{}}
	}
	cases := []boundary{
		{"operation", 128, "invalid_input", func(m map[string]any, n int) { m["operation"] = strings.Repeat("x", n) }},
		{"title", 1024, "invalid_input", func(m map[string]any, n int) {
			record(m)
			m["payload"].(map[string]any)["title"] = strings.Repeat("x", n)
		}},
		{"namespace", 64, "invalid_input", func(m map[string]any, n int) {
			m["payload"].(map[string]any)["type_id"] = strings.Repeat("x", n) + "/name"
		}},
		{"type-name", 64, "invalid_input", func(m map[string]any, n int) {
			m["payload"].(map[string]any)["type_id"] = "example/" + strings.Repeat("x", n)
		}},
		{"field-name", 64, "invalid_input", func(m map[string]any, n int) {
			record(m)
			m["payload"].(map[string]any)["fields"] = map[string]any{strings.Repeat("x", n): true}
		}},
		{"properties", 64, "resource_limit", func(m map[string]any, n int) {
			record(m)
			fields := map[string]any{}
			for i := 0; i < n; i++ {
				fields[fmt.Sprintf("p%d", i)] = i
			}
			m["payload"].(map[string]any)["fields"] = fields
		}},
		{"depth", 8, "resource_limit", func(m map[string]any, n int) {
			record(m)
			fields := map[string]any{}
			current := fields
			for i := 1; i < n; i++ {
				next := map[string]any{}
				current["nest"] = next
				current = next
			}
			m["payload"].(map[string]any)["fields"] = fields
		}},
		{"array", 1024, "resource_limit", func(m map[string]any, n int) {
			record(m)
			m["payload"].(map[string]any)["fields"] = map[string]any{"array": make([]any, n)}
		}},
		{"parents", 1024, "resource_limit", func(m map[string]any, n int) {
			record(m)
			m["kind"] = "record.resolve"
			m["record"] = artifactVectorID
			parents := []any{}
			for i := 0; i < n; i++ {
				parents = append(parents, fmt.Sprintf("sha256:%064x", i))
			}
			m["parents"] = parents
		}},
		{"members", 1024, "resource_limit", func(m map[string]any, n int) {
			record(m)
			p := m["payload"].(map[string]any)
			p["shape"] = "list"
			members := []any{}
			for i := 0; i < n; i++ {
				members = append(members, map[string]any{"member_id": fmt.Sprintf("member%d", i), "target": artifactVectorID, "target_mode": "revision", "required": false})
			}
			p["members"] = members
		}},
		{"member-id", 128, "invalid_input", func(m map[string]any, n int) {
			record(m)
			p := m["payload"].(map[string]any)
			p["shape"] = "list"
			p["members"] = []any{map[string]any{"member_id": strings.Repeat("x", n), "target": artifactVectorID, "target_mode": "logical", "required": true}}
		}},
		{"relationships", 256, "resource_limit", func(m map[string]any, n int) {
			record(m)
			links := []any{}
			for i := 0; i < n; i++ {
				links = append(links, map[string]any{"relation": fmt.Sprintf("example/r%d", i), "target": artifactVectorID, "target_mode": "revision", "required": false})
			}
			m["payload"].(map[string]any)["relationships"] = links
		}},
		{"schema-bytes", 65536, "resource_limit", func(m map[string]any, n int) {
			node := map[string]any{"type": "string", "maxBytes": 65536, "enum": []any{""}}
			schema := map[string]any{"language": "hn.schema/1", "root": map[string]any{"type": "object", "properties": map[string]any{"text": node}, "required": []any{}, "additionalProperties": false}}
			base, _ := json.Marshal(schema)
			node["enum"] = []any{strings.Repeat("x", n-len(base))}
			m["payload"].(map[string]any)["schema"] = schema
		}},
	}
	directory := t.TempDir()
	check := func(raw, sig, code string) {
		t.Helper()
		cmd := exec.Command(binary, "artifact", "envelope", "validate", "--input", "-", "--signature", sig, "--json")
		cmd.Dir = directory
		cmd.Env = operationalTestEnvironment(directory)
		cmd.Stdin = strings.NewReader(raw)
		out, e := cmd.Output()
		wp25Record(t, map[string]any{"program": binary, "argv": cmd.Args[1:], "stdin": raw, "stdout": string(out), "exit": fmt.Sprint(e), "expected_code": code})
		var result struct {
			OK    bool `json:"ok"`
			Error *struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if je := json.Unmarshal(out, &result); je != nil {
			t.Fatalf("invalid JSON %v %s", je, out)
		}
		if code == "" {
			if e != nil || !result.OK {
				t.Fatalf("at bound: %v %s", e, out)
			}
		} else if e == nil || result.OK || result.Error == nil || result.Error.Code != code {
			t.Fatalf("over bound wanted %s: %v %s", code, e, out)
		}
	}
	for _, b := range cases {
		t.Run(b.name, func(t *testing.T) {
			for _, n := range []int{b.at, b.at + 1} {
				raw, sig := artifactSignedCLI(t, func(m map[string]any) { b.change(m, n) })
				code := ""
				if n > b.at {
					code = b.code
				}
				check(raw, sig, code)
			}
		})
	}
	for _, b := range []struct {
		name   string
		max    int64
		change func(map[string]any, int64)
	}{
		{"schema-version", 2147483647, func(m map[string]any, n int64) { m["payload"].(map[string]any)["schema_version"] = n }},
		{"sequence", 9007199254740991, func(m map[string]any, n int64) { m["sequence"] = n; m["previous"] = artifactVectorID }},
	} {
		t.Run(b.name, func(t *testing.T) {
			for _, n := range []int64{b.max, b.max + 1} {
				raw, sig := artifactSignedCLI(t, func(m map[string]any) { b.change(m, n) })
				code := ""
				if n > b.max {
					code = "invalid_input"
				}
				check(raw, sig, code)
			}
		})
	}
	// With the 64 KiB schema ceiling, a valid closed type cannot reach 128 KiB.
	// At equality this deliberately invalid identifier reaches the identifier
	// refusal; the extra byte instead triggers the earlier aggregate defense.
	for _, n := range []int{131072, 131073} {
		raw, sig := artifactSignedCLI(t, func(m map[string]any) {
			p := m["payload"].(map[string]any)
			p["type_id"] = ""
			base, _ := json.Marshal(p)
			p["type_id"] = strings.Repeat("x", n-len(base))
		})
		code := "invalid_input"
		if n > 131072 {
			code = "resource_limit"
		}
		check(raw, sig, code)
	}
}

// These extra caller pairs close numeric gaps identified in the static audit.
// They also run in the fixed-source suite; that mode is explicitly source-
// assisted and is never counted as one of the eight package rows.
func TestExtensionPackagePublicLimits(t *testing.T) {
	binary := extensionTestBinary(t)
	wp25ManagementLimits(t, binary)
	t.Setenv("HN_WP05_BINARY", binary)
	t.Run("selected-ref-live-bound", wp25SelectedRefLimit)
}

type wp25ManagementFixture struct {
	t                                                    *testing.T
	binary, repo, home, config, manifest, module, marker string
	env                                                  []string
}

func wp25ManagementNew(t *testing.T, binary string) *wp25ManagementFixture {
	t.Helper()
	m := &wp25ManagementFixture{t: t, binary: binary, repo: t.TempDir(), home: t.TempDir()}
	m.config = wp25FixtureConfig(t, m.home)
	m.env = operationalTestEnvironmentWith(m.home, map[string]string{"XDG_CONFIG_HOME": m.config})
	cmd := exec.Command("git", "init", "-q", m.repo)
	cmd.Env = m.env
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("git init: %v %s", e, out)
	}
	m.manifest = filepath.Join(t.TempDir(), "manifest.json")
	m.module = filepath.Join(t.TempDir(), "module")
	m.marker = filepath.Join(t.TempDir(), "module-started")
	if e := os.WriteFile(m.module, []byte("#!/bin/sh\nprintf forbidden > '"+m.marker+"'\n"), 0700); e != nil {
		t.Fatal(e)
	}
	return m
}
func (m *wp25ManagementFixture) manifestBytes(id, version string) []byte {
	raw, _ := json.Marshal(map[string]any{"protocol": "hn.extension-manifest/1", "module_id": id, "module_version": version, "host_protocol": "hn.extension/1", "capabilities": []string{"resource.read", "resource.mutate", "artifact.read", "artifact.mutate", "artifact.type.publish", "changes.capture"}})
	return raw
}
func (m *wp25ManagementFixture) writeManifest(id, version string) {
	m.t.Helper()
	if e := os.WriteFile(m.manifest, m.manifestBytes(id, version), 0600); e != nil {
		m.t.Fatal(e)
	}
}
func (m *wp25ManagementFixture) call(code string, args ...string) extensionCLIResult {
	m.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.binary, append([]string{"extension"}, args...)...)
	cmd.Dir = m.repo
	cmd.Env = m.env
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	e := cmd.Run()
	processExit := fmt.Sprint(e)
	var v extensionCLIResult
	if ctx.Err() != nil || json.Unmarshal(out.Bytes(), &v) != nil {
		m.t.Fatalf("management %v: %v %s %s", args, e, &out, &stderr)
	}
	if code == "" {
		if e != nil || !v.OK {
			m.t.Fatalf("management positive %v: %v %s %s", args, e, &out, &stderr)
		}
	} else if e == nil || v.OK || v.Data != nil || v.Error.Code != code || v.Error.Origin != "host" || v.Error.Recovery != "do_not_retry" {
		m.t.Fatalf("management want %s: %v %s %s", code, e, &out, &stderr)
	}
	if _, e := os.Stat(m.marker); !os.IsNotExist(e) {
		m.t.Fatal("management launched executable", e)
	}
	if root := os.Getenv("HN_WP25_EVIDENCE"); root != "" {
		if e := os.MkdirAll(root, 0700); e != nil {
			m.t.Fatal(e)
		}
		f, e := os.OpenFile(filepath.Join(root, "management-transcript.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			m.t.Fatal(e)
		}
		raw, _ := json.Marshal(map[string]any{"test": m.t.Name(), "argv": args, "expected": code, "exit": processExit, "stdout": out.String(), "stderr": stderr.String()})
		_, e = f.Write(append(raw, '\n'))
		f.Close()
		if e != nil {
			m.t.Fatal(e)
		}
	}
	return v
}
func (m *wp25ManagementFixture) register(code string, argv []string) {
	m.t.Helper()
	args := []string{"register", "--manifest", m.manifest, "--executable", m.module}
	for _, a := range argv {
		args = append(args, "--arg", a)
	}
	args = append(args, "--json")
	m.call(code, args...)
}
func (m *wp25ManagementFixture) snapshot() map[string]string {
	v := protocolTree(m.t, m.repo)
	for p, h := range protocolTree(m.t, m.home) {
		v["home/"+p] = h
	}
	wp25Record(m.t, map[string]any{"control": "management-protected-snapshot", "snapshot": v})
	return v
}
func (m *wp25ManagementFixture) refuse(code string, args ...string) {
	m.t.Helper()
	before := m.snapshot()
	m.call(code, args...)
	after := m.snapshot()
	wp25Record(m.t, map[string]any{"before": before, "after": after, "expected": "no management effects"})
	if !reflect.DeepEqual(before, after) {
		m.t.Fatal("refusal changed registry, keys, enablement, objects or refs")
	}
}
func wp25ManagementLimits(t *testing.T, binary string) {
	for _, pair := range []struct {
		name     string
		at, over []string
	}{
		{"argv-count", make([]string, 64), make([]string, 65)},
		{"argv-value", []string{strings.Repeat("x", 4096)}, []string{strings.Repeat("x", 4097)}},
		{"argv-sum", wp25RepeatedStrings(16, 4096), append(wp25RepeatedStrings(16, 4096), "x")},
	} {
		t.Run(pair.name, func(t *testing.T) {
			m := wp25ManagementNew(t, binary)
			m.writeManifest("example.qualification/tool", "1")
			m.register("", pair.at)
			before := m.snapshot()
			m.register("resource_limit", pair.over)
			if !reflect.DeepEqual(before, m.snapshot()) {
				t.Fatal("argv refusal changed state")
			}
		})
	}
	t.Run("manifest-and-id-version", func(t *testing.T) {
		m := wp25ManagementNew(t, binary)
		for _, n := range []int{64, 65} {
			m.writeManifest(strings.Repeat("x", n)+"/tool", "1")
			before := m.snapshot()
			code := ""
			if n == 65 {
				code = "invalid_input"
			}
			m.register(code, nil)
			if code != "" && !reflect.DeepEqual(before, m.snapshot()) {
				t.Fatal("module ID refusal changed state")
			}
			m.writeManifest("example.qualification/tool", strings.Repeat("v", n))
			before = m.snapshot()
			m.register(code, nil)
			if code != "" && !reflect.DeepEqual(before, m.snapshot()) {
				t.Fatal("version refusal changed state")
			}
		}
		m.call("", "remove", "example.qualification/tool", "--json")
		raw := m.manifestBytes("example.qualification/tool", "1")
		for _, n := range []int{131072, 131073} {
			b := append(append([]byte{}, raw...), bytes.Repeat([]byte(" "), n-len(raw))...)
			if e := os.WriteFile(m.manifest, b, 0600); e != nil {
				t.Fatal(e)
			}
			before := m.snapshot()
			code := ""
			if n == 131073 {
				code = "resource_limit"
			}
			m.register(code, nil)
			if code != "" && !reflect.DeepEqual(before, m.snapshot()) {
				t.Fatal("manifest refusal changed state")
			}
		}
	})
	t.Run("executable-hash", func(t *testing.T) {
		m := wp25ManagementNew(t, binary)
		m.writeManifest("example.qualification/tool", "1")
		file, e := os.OpenFile(m.module, os.O_WRONLY, 0700)
		if e != nil {
			t.Fatal(e)
		}
		if e = file.Truncate(134217728); e != nil {
			t.Fatal(e)
		}
		file.Close()
		m.register("", nil)
		file, e = os.OpenFile(m.module, os.O_WRONLY, 0700)
		if e != nil {
			t.Fatal(e)
		}
		if e = file.Truncate(134217729); e != nil {
			t.Fatal(e)
		}
		file.Close()
		before := m.snapshot()
		m.register("resource_limit", nil)
		if !reflect.DeepEqual(before, m.snapshot()) {
			t.Fatal("executable hash refusal changed state")
		}
	})
	t.Run("registry-enablement-counts-and-bytes", func(t *testing.T) {
		m := wp25ManagementNew(t, binary)
		for i := 0; i < 128; i++ {
			id := fmt.Sprintf("example.qualification/m%d", i)
			m.writeManifest(id, "1")
			m.register("", nil)
			m.call("", "enable", id, "--grant", "native:issue=read", "--json")
		}
		inspect := m.call("", "list", "--json")
		var modules []any
		if json.Unmarshal(inspect.Data["modules"], &modules) != nil || len(modules) != 128 {
			t.Fatal("128 actual registrations were not admitted")
		}
		registry := filepath.Join(m.config, "hubnot", "extensions", "registry.json")
		enabled := filepath.Join(m.repo, ".git", "hn", "extensions", "enabled.json")
		original, e := os.ReadFile(registry)
		if e != nil {
			t.Fatal(e)
		}
		m.writeManifest("example.qualification/overflow", "1")
		before := m.snapshot()
		m.register("resource_limit", nil)
		if !reflect.DeepEqual(before, m.snapshot()) {
			t.Fatal("129th registration changed state")
		}
		for _, path := range []string{registry, enabled} {
			raw, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			for _, n := range []int{1048576, 1048577} {
				b := append(append([]byte{}, raw...), bytes.Repeat([]byte(" "), n-len(raw))...)
				if e := os.WriteFile(path, b, 0600); e != nil {
					t.Fatal(e)
				}
				code := ""
				if n == 1048577 {
					code = "resource_limit"
				}
				m.refuse(code, "list", "--json")
			}
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
		}
		// Loader count defense: source-assisted local corruption, followed by the
		// actual supplied CLI. The 128-entry positive above used real management.
		for _, p := range []struct{ path, field string }{{registry, "registrations"}, {enabled, "entries"}} {
			raw, e := os.ReadFile(p.path)
			if e != nil {
				t.Fatal(e)
			}
			var doc map[string]any
			if e = json.Unmarshal(raw, &doc); e != nil {
				t.Fatal(e)
			}
			entries := doc[p.field].([]any)
			doc[p.field] = append(entries, entries[0])
			bad, _ := json.Marshal(doc)
			if e = os.WriteFile(p.path, bad, 0600); e != nil {
				t.Fatal(e)
			}
			m.refuse("resource_limit", "list", "--json")
			if e = os.WriteFile(p.path, raw, 0600); e != nil {
				t.Fatal(e)
			}
		}
		if !bytes.Equal(original, wp25Read(t, registry)) {
			t.Fatal("registry bytes changed during raw loader controls")
		}
	})
	t.Run("grants-and-independent-actions", func(t *testing.T) {
		m := wp25ManagementNew(t, binary)
		m.writeManifest("example.qualification/tool", "1")
		m.register("", nil)
		args := []string{"enable", "example.qualification/tool"}
		for i := 0; i < 256; i++ {
			actions := "read"
			if i == 0 {
				actions = "read,observe,create,revise,resolve,type.publish"
			}
			selector := fmt.Sprintf("artifact:example.qualification/t%d", i)
			if i == 1 {
				selector, actions = "artifact:opaque", "raw.inspect"
			}
			args = append(args, "--grant", selector+"="+actions)
		}
		args = append(args, "--json")
		result := m.call("", args...)
		var grants []struct {
			Selector string
			Actions  []string
		}
		if json.Unmarshal(result.Data["grants"], &grants) != nil || len(grants) != 256 {
			t.Fatal("256 grant positive not observed")
		}
		want := []string{"read", "observe", "create", "revise", "resolve", "type.publish", "raw.inspect"}
		var got []string
		for _, g := range grants {
			if g.Selector == "artifact:example.qualification/t0" {
				got = append(got, g.Actions...)
			}
			if g.Selector == "artifact:opaque" {
				got = append(got, g.Actions...)
			}
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatal("independent seven action inventory changed", got)
		}
		over := append(append([]string{}, args[:len(args)-1]...), "--grant", "artifact:example.qualification/overflow=read", "--json")
		m.refuse("resource_limit", over...)
		for _, bad := range []string{"read,observe,create,revise,resolve,type.publish,raw.inspect,execute", "read,read", "execute"} {
			m.refuse("invalid_input", "enable", "example.qualification/tool", "--grant", "artifact:example.qualification/t0="+bad, "--json")
		}
	})
}
func wp25Read(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func wp25RepeatedStrings(count, size int) []string {
	v := make([]string, count)
	for i := range v {
		v[i] = strings.Repeat("x", size)
	}
	return v
}
func wp25SelectedRefLimit(t *testing.T) {
	f := wp25ArtifactCLI(t)
	ref := artifactFrozenRef(t, f)
	commit := artifactRawCommit(t, f, artifactVector, artifactVectorSignature, "", "hn artifact "+artifactVectorID, false)
	suffix := strings.TrimPrefix(ref, "refs/hn/")
	var packed bytes.Buffer
	packed.WriteString("# pack-refs with: sorted\n")
	for i := 0; i < 100000; i++ {
		fmt.Fprintf(&packed, "%s refs/hn/remotes/r%06d/%s\n", commit, i, suffix)
	}
	path := filepath.Join(f.repo, ".git", "packed-refs")
	if e := os.WriteFile(path, packed.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	before := f.snapshot()
	wp25ArtifactCall(f, "", "", "artifact", "type", "show", artifactVectorID, "--json")
	wp25Unchanged(f, before)
	fmt.Fprintf(&packed, "%s refs/hn/remotes/r100000/%s\n", commit, suffix)
	if e := os.WriteFile(path, packed.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	before = f.snapshot()
	v := wp25ArtifactCall(f, "", "resource_limit", "artifact", "type", "show", artifactVectorID, "--json")
	artifactErrorShape(t, v, "artifact", "resource_limit", "do_not_retry", nil)
	wp25Unchanged(f, before)
}

func TestExtensionPackageArtifactLimits(t *testing.T) {
	t.Setenv("HN_WP05_BINARY", artifactStoreBinary(t))
	wp25ArtifactLimits(t)
	t.Run("read-object-and-aggregate-limits", wp25ReadBudgets)
	t.Run("exchange-object-limit", wp25ExchangeObjectLimit)
	t.Run("artifact-parent-limit", wp25ArtifactParentLimit)
	t.Run("artifact-record-count-and-enclosing-deadline", wp25ArtifactRecordCountLimit)
}
func wp25ArtifactLimits(t *testing.T) {
	t.Run("finite-schema-numeric-pairs", func(t *testing.T) {
		f := wp25ArtifactCLI(t)
		object := func(props map[string]any) map[string]any {
			return map[string]any{"type": "object", "properties": props, "required": []any{}, "additionalProperties": false}
		}
		schema := func(node map[string]any) map[string]any {
			return map[string]any{"language": "hn.schema/1", "root": node}
		}
		check := func(v map[string]any, code string) {
			t.Helper()
			raw, _ := json.Marshal(v)
			before := f.snapshot()
			out := wp25ArtifactCall(f, string(raw), code, "artifact", "schema", "validate", "--input", "-", "--json")
			if code != "" {
				artifactErrorShape(t, out, "artifact", code, "do_not_retry", nil)
			}
			wp25Unchanged(f, before)
		}
		for _, n := range []int{8, 9} {
			node := object(map[string]any{})
			for i := 1; i < n; i++ {
				node = object(map[string]any{"nested": node})
			}
			code := ""
			if n == 9 {
				code = "resource_limit"
			}
			check(schema(node), code)
		}
		for _, n := range []int{64, 65} {
			props := map[string]any{}
			for i := 0; i < n; i++ {
				props[fmt.Sprintf("p%d", i)] = map[string]any{"type": "boolean"}
			}
			code := ""
			if n == 65 {
				code = "resource_limit"
			}
			check(schema(object(props)), code)
		}
		for _, n := range []int{0, 1024, 1025} {
			code := ""
			if n == 1025 {
				code = "invalid_input"
			}
			check(schema(object(map[string]any{"array": map[string]any{"type": "array", "items": map[string]any{"type": "null"}, "maxItems": n}})), code)
		}
		for _, n := range []int{0, 1048576, 1048577} {
			code := ""
			if n == 1048577 {
				code = "invalid_input"
			}
			check(schema(object(map[string]any{"text": map[string]any{"type": "string", "maxBytes": n}})), code)
		}
		for _, n := range []int{1024, 1025} {
			values := make([]any, n)
			for i := range values {
				values[i] = fmt.Sprintf("v%d", i)
			}
			code := ""
			if n == 1025 {
				code = "resource_limit"
			}
			check(schema(object(map[string]any{"text": map[string]any{"type": "string", "maxBytes": 64, "enum": values}})), code)
		}
		for _, n := range []int{65536, 65537} {
			raw, _ := json.Marshal(schema(object(map[string]any{})))
			raw = append(raw, bytes.Repeat([]byte(" "), n-len(raw))...)
			code := ""
			if n == 65537 {
				code = "resource_limit"
			}
			before := f.snapshot()
			out := wp25ArtifactCall(f, string(raw), code, "artifact", "schema", "validate", "--input", "-", "--json")
			if code != "" {
				artifactErrorShape(t, out, "artifact", code, "do_not_retry", nil)
			}
			wp25Unchanged(f, before)
		}
	})
	t.Run("published-list-and-relationship-closure", func(t *testing.T) {
		f := wp25ArtifactCLI(t)
		typ := revisionEvent(wp25Publish(f, artifactStoreType, "target-type", ""))
		target := revisionEvent(wp25RevisionCall(f, "open", revisionInput(t, nil, []string{}, revisionPayload(typ, "target")), "target-open", ""))
		listPayload := strings.ReplaceAll(strings.ReplaceAll(artifactStoreType, "example.test/note", "example.qualification/list"), `"shape":"document"`, `"shape":"list"`)
		listType := revisionEvent(wp25Publish(f, listPayload, "list-type", ""))
		for _, n := range []int{1024, 1025} {
			payload := revisionPayload(listType, "list-bound")
			payload["shape"] = "list"
			payload["type_id"] = "example.qualification/list"
			members := make([]any, n)
			for i := range members {
				members[i] = map[string]any{"member_id": fmt.Sprintf("member%d", i), "target": target, "target_mode": "revision", "required": true}
			}
			payload["members"] = members
			code := ""
			if n == 1025 {
				code = "resource_limit"
			}
			before := f.snapshot()
			result := wp25RevisionCall(f, "open", revisionInput(t, nil, []string{}, payload), fmt.Sprintf("members-%d", n), code)
			if code != "" {
				artifactErrorShape(t, result, "artifact", code, "do_not_retry", nil)
				wp25Unchanged(f, before)
			}
		}
		for _, n := range []int{256, 257} {
			payload := revisionPayload(typ, "links-bound")
			links := make([]any, n)
			for i := range links {
				links[i] = map[string]any{"relation": fmt.Sprintf("example.qualification/r%d", i), "target": target, "target_mode": "logical", "required": true}
			}
			payload["relationships"] = links
			code := ""
			if n == 257 {
				code = "resource_limit"
			}
			before := f.snapshot()
			result := wp25RevisionCall(f, "open", revisionInput(t, nil, []string{}, payload), fmt.Sprintf("links-%d", n), code)
			if code != "" {
				artifactErrorShape(t, result, "artifact", code, "do_not_retry", nil)
				wp25Unchanged(f, before)
			}
		}
		// An otherwise-valid full closure is available. A required nonexistent target
		// fails before signer/journal/publication; integrity never implies admission.
		payload := revisionPayload(typ, "missing-target")
		payload["relationships"] = []any{map[string]any{"relation": "example.qualification/needs", "target": "sha256:" + strings.Repeat("f", 64), "target_mode": "revision", "required": true}}
		before := f.snapshot()
		v := wp25RevisionCall(f, "open", revisionInput(t, nil, []string{}, payload), "missing-target", "incomplete_dependency")
		artifactErrorShape(t, v, "artifact", "incomplete_dependency", "do_not_retry", nil)
		wp25Unchanged(f, before)
	})
}

// Independently signed public fixtures: the candidate is the verifying reader.
// No application reader counter is seeded and no helper result is credited as
// executable admission. Git metadata padding is unsigned transport metadata.
func wp25Signed(t *testing.T, seedText string) (raw, sig, id, actor string) {
	t.Helper()
	var m map[string]any
	if e := json.Unmarshal([]byte(artifactVector), &m); e != nil {
		t.Fatal(e)
	}
	seed := sha256.Sum256([]byte(seedText))
	key := ed25519.NewKeyFromSeed(seed[:])
	pub := key.Public().(ed25519.PublicKey)
	a := sha256.Sum256(pub)
	actor = hex.EncodeToString(a[:])
	m["actor"] = actor
	m["public_key"] = base64.RawStdEncoding.EncodeToString(pub)
	m["stream"] = eventID([]byte("hn-artifact-stream-v1\x00" + actor))
	semantic := map[string]any{}
	for _, k := range []string{"actor", "kind", "operation", "parents", "payload", "protocol", "record"} {
		semantic[k] = m[k]
	}
	request, _ := json.Marshal(semantic)
	m["request"] = eventID(append([]byte("hn-artifact-request-v1\x00"), request...))
	body, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	signature := ed25519.Sign(key, append([]byte("hn-artifact/1\x00"), body...))
	return string(body), base64.RawStdEncoding.EncodeToString(signature), eventID(body), actor
}
func wp25PaddedCommit(t *testing.T, f *artifactCLIFixture, tree, id string, size int) string {
	t.Helper()
	prefix := "tree " + tree + "\nauthor Fixture <fixture@example.invalid> 1793404800 +0000\ncommitter Fixture <fixture@example.invalid> 1793404800 +0000\nx-padding "
	suffix := "\n\nhn artifact " + id + "\n"
	padding := size - len(prefix) - len(suffix)
	if padding < 0 {
		t.Fatal("negative metadata padding")
	}
	return f.gitInput(prefix+strings.Repeat("x", padding)+suffix, "hash-object", "-t", "commit", "-w", "--stdin")
}
func wp25ReadBudgets(t *testing.T) {
	t.Run("object-eight-MiB", func(t *testing.T) {
		f := wp25ArtifactCLI(t)
		raw, sig, id, actor := wp25Signed(t, "WP25 public object cap fixture")
		original := artifactRawCommit(t, f, raw, sig, "", "hn artifact "+id, false)
		tree := f.git("rev-parse", original+"^{tree}")
		for _, n := range []int{8388608, 8388609} {
			commit := wp25PaddedCommit(t, f, tree, id, n)
			f.git("update-ref", artifactRef(actor), commit)
			before := f.snapshot()
			code := ""
			if n == 8388609 {
				code = "resource_limit"
			}
			out := wp25ArtifactCall(f, "", code, "artifact", "type", "show", id, "--json")
			if code != "" {
				artifactErrorShape(t, out, "artifact", code, "do_not_retry", nil)
			}
			wp25Unchanged(f, before)
		}
	})
	t.Run("full-256-MiB-unique-objects", func(t *testing.T) {
		f := wp25ArtifactCLI(t)
		type part struct{ tree, id, actor string }
		parts := make([]part, 32)
		small := 0
		seen := map[string]bool{}
		for i := range parts {
			raw, sig, id, actor := wp25Signed(t, fmt.Sprintf("WP25 public aggregate cap actor %d", i))
			commit := artifactRawCommit(t, f, raw, sig, "", "hn artifact "+id, false)
			tree := f.git("rev-parse", commit+"^{tree}")
			parts[i] = part{tree, id, actor}
			for _, oid := range []string{tree, f.git("rev-parse", commit+":artifact.json"), f.git("rev-parse", commit+":signature")} {
				if !seen[oid] {
					seen[oid] = true
					var size int
					if _, e := fmt.Sscan(f.git("cat-file", "-s", oid), &size); e != nil {
						t.Fatal(e)
					}
					small += size
				}
			}
		}
		// Exactly 32 commit objects plus their unique two-blob trees sum to256MiB.
		// One object's padding compensates for every other charged object byte.
		total := small
		for i, p := range parts {
			size := 8388608
			if i == len(parts)-1 {
				size -= small
			}
			commit := wp25PaddedCommit(t, f, p.tree, p.id, size)
			f.git("update-ref", artifactRef(p.actor), commit)
			total += size
		}
		if total != 268435456 {
			t.Fatal("independent aggregate fixture sum", total)
		}
		before := f.snapshot()
		wp25ArtifactCall(f, "", "", "artifact", "type", "show", parts[0].id, "--json")
		wp25Unchanged(f, before)
		last := parts[len(parts)-1]
		commit := wp25PaddedCommit(t, f, last.tree, last.id, 8388608-small+1)
		f.git("update-ref", artifactRef(last.actor), commit)
		before = f.snapshot()
		out := wp25ArtifactCall(f, "", "resource_limit", "artifact", "type", "show", parts[0].id, "--json")
		artifactErrorShape(t, out, "artifact", "resource_limit", "do_not_retry", nil)
		if !strings.Contains(out["error"].(map[string]any)["message"].(string), "aggregate") {
			t.Fatal("wrong aggregate refusal cause", out)
		}
		wp25Unchanged(f, before)
	})
}

// Local sample input ceilings are application-owned. These supplied-process
// controls stop at a scripted host response, not at native publication. The
// complete real host/reference/clone journeys remain separate required cases.
func wp25ConsumerInputLimits(t *testing.T, references map[string]string) {
	t.Run("tracker-service-limits", func(t *testing.T) { wp25TrackerServiceLimits(t, references["fake-tracker"]) })
	t.Run("tracker-client-limits", func(t *testing.T) { wp25TrackerClientLimits(t, references["fake-tracker"]) })
	id := "sha256:" + strings.Repeat("a", 64)
	hello := func(module string) map[string]any {
		return map[string]any{"protocol": "hn.extension/1", "hello": map[string]any{"session_id": strings.Repeat("1", 32), "repository_id": id, "module_id": module, "module_version": "1", "registration_id": id}, "limits": map[string]any{"frame_bytes": 8388608, "read_timeout_ms": 30000, "mutation_timeout_ms": 240000, "page_max": 200, "one_outstanding": true}}
	}
	frame := func(v any) []byte { raw, _ := json.Marshal(v); return protocolRawFrame(raw) }
	for _, sample := range []struct {
		name, module string
		limit        int
		command      map[string]any
		overMessage  string
	}{
		{"charter", "example.charter/model", 1048576, map[string]any{"action": "call", "method": "identity.public", "params": map[string]any{}}, "input exceeds 1048576 bytes"},
		{"inventory", "example.inventory/publisher", 8388608, map[string]any{"action": "type", "model": "item", "schema_version": 1, "parents": []string{}, "operation": "local-byte-control"}, "command exceeds host frame limit"},
	} {
		t.Run(sample.name+"-local-command-bytes", func(t *testing.T) {
			directory := t.TempDir()
			input, output, state := filepath.Join(directory, "input"), filepath.Join(directory, "output"), filepath.Join(directory, "state")
			raw, _ := json.Marshal(sample.command)
			response := map[string]any{"protocol": "hn.extension/1", "request_id": "charter-1", "ok": true, "data": map[string]any{"initialized": false, "actor": nil, "name": nil, "public_key": nil}, "error": nil}
			if sample.name == "inventory" {
				response = map[string]any{"protocol": "hn.extension/1", "request_id": nil, "ok": false, "data": nil, "error": map[string]any{"origin": "host", "code": "denied_access", "message": "scripted terminal input-ceiling control", "recovery": "do_not_retry", "operation": nil}}
			}
			incoming := append(frame(hello(sample.module)), frame(response)...)
			for _, n := range []int{sample.limit, sample.limit + 1} {
				padded := append(append([]byte{}, raw...), bytes.Repeat([]byte(" "), n-len(raw))...)
				if e := os.WriteFile(input, padded, 0600); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(output, []byte("PRIVATE-UNCHANGED-OUTPUT"), 0600); e != nil {
					t.Fatal(e)
				}
				args := []string{"--input", input, "--output", output}
				if sample.name == "charter" {
					args = append(args, "--state", state)
				}
				cmd := exec.Command(references[sample.name], args...)
				cmd.Dir = directory
				cmd.Env = operationalTestEnvironment(directory)
				cmd.Stdin = bytes.NewReader(incoming)
				var wire, diagnostic bytes.Buffer
				cmd.Stdout = &wire
				cmd.Stderr = &diagnostic
				e := cmd.Run()
				t.Logf("standalone %s input=%d exit=%v wire=%s diagnostic=%s", sample.name, n, e, wire.String(), diagnostic.String())
				if n == sample.limit {
					if e != nil || wire.Len() == 0 || bytes.Equal(wp25Read(t, output), []byte("PRIVATE-UNCHANGED-OUTPUT")) {
						t.Fatal("local command equality did not reach and consume the scripted typed host response", e)
					}
				} else {
					if e == nil || !strings.Contains(diagnostic.String(), sample.overMessage) || wire.Len() != 0 || !bytes.Equal(wp25Read(t, output), []byte("PRIVATE-UNCHANGED-OUTPUT")) {
						t.Fatal("first-over local command did not stop before host negotiation/output replacement")
					}
				}
			}
		})
	}
	t.Run("tracker-config-plan-journal-input-bytes", func(t *testing.T) {
		directory := t.TempDir()
		config, plan, journal, report := filepath.Join(directory, "config"), filepath.Join(directory, "plan"), filepath.Join(directory, "journal"), filepath.Join(directory, "report")
		cfg := map[string]any{"api": "http://127.0.0.1:1", "token": "PRIVATE-NO-CONTACT-CREDENTIAL", "connection": "byte-control", "source": "tracker:item:42", "journal": journal, "report": report}
		p := map[string]any{"action": "observe", "limit": 20}
		j := map[string]any{"repository": id, "module": "example.tracker/tool", "registration": id, "connection": "byte-control", "source": "tracker:item:42", "api": "http://127.0.0.1:1", "credential": "PRIVATE-NO-CONTACT-CREDENTIAL", "native": map[string]any{}, "external": map[string]any{}, "baseline": map[string]any{}, "entries": map[string]any{}}
		data := map[string]map[string]any{config: cfg, plan: p, journal: j}
		write := func(path string, n int) {
			raw, _ := json.Marshal(data[path])
			if n > 0 {
				raw = append(raw, bytes.Repeat([]byte(" "), n-len(raw))...)
			}
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
		}
		for path := range data {
			write(path, 0)
		}
		refusal := map[string]any{"protocol": "hn.extension/1", "request_id": "tracker-1", "ok": false, "data": nil, "error": map[string]any{"origin": "host", "code": "denied_access", "message": "scripted input-ceiling refusal", "recovery": "do_not_retry", "operation": nil}}
		incoming := append(frame(hello("example.tracker/tool")), frame(refusal)...)
		for _, bound := range []struct {
			path  string
			limit int
		}{{config, 131072}, {plan, 131072}, {journal, 16777216}} {
			for _, n := range []int{bound.limit, bound.limit + 1} {
				write(bound.path, n)
				before := wp25Read(t, journal)
				cmd := exec.Command(references["fake-tracker"], "--config", config, "--plan", plan)
				cmd.Dir = directory
				cmd.Env = operationalTestEnvironment(directory)
				cmd.Stdin = bytes.NewReader(incoming)
				var wire, diagnostic bytes.Buffer
				cmd.Stdout = &wire
				cmd.Stderr = &diagnostic
				e := cmd.Run()
				t.Logf("supplied tracker %s bytes=%d exit=%v wire=%s diagnostic=%s", filepath.Base(bound.path), n, e, wire.String(), diagnostic.String())
				if e == nil {
					t.Fatal("scripted denied request reported success")
				}
				if n == bound.limit {
					if !strings.Contains(diagnostic.String(), "denied_access") || wire.Len() == 0 {
						t.Fatal("input equality did not reach typed host refusal")
					}
				} else if !strings.Contains(diagnostic.String(), "resource_limit") || ((bound.path == config || bound.path == plan) && wire.Len() != 0) {
					t.Fatal("local byte overflow wrong cause/order")
				}
				if !bytes.Equal(before, wp25Read(t, journal)) {
					t.Fatal("input/refusal changed immutable journal")
				}
			}
			write(bound.path, 0)
		}
	})
}
func TestExtensionPackageConsumerInputLimits(t *testing.T) {
	wp25ConsumerInputLimits(t, map[string]string{"charter": charterConsumer(t), "inventory": inventoryBinary(t), "fake-tracker": fakeTrackerBinary(t)})
}

// The existing plain Go/Git public journey, bound directly to this row's verified
// executable. It intentionally uses no compiler or provider executable.
func wp25NativePublicJourney(t *testing.T, binary string) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir()
	if err = os.Symlink(git, filepath.Join(path, "git")); err != nil {
		t.Fatal(err)
	}
	// A deliberately closed process environment: no ambient tokens, Go, Python,
	// provider CLIs, user Git configuration, credential helpers or services.
	env := []string{"PATH=" + path, "HOME=" + t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS="}
	call := func(repo string, ok bool, args ...string) string {
		return portablePublic(t, binary, repo, env, ok, args...)
	}
	audit := func(repo string, ok bool, args ...string) json.RawMessage {
		out := call(repo, ok, append([]string{"audit"}, append(args, "--json")...)...)
		var v struct {
			Schema string
			OK     bool
			Data   json.RawMessage
		}
		if err := json.NewDecoder(strings.NewReader(out)).Decode(&v); err != nil || v.Schema != "hn.audit/1" || v.OK != ok {
			t.Fatalf("audit envelope: %s %v", out, err)
		}
		return v.Data
	}
	source := migrationCLIRepository(t)
	configureOperationalGit(t, source, "Portable owner")
	call(source, true, "init", "--name", "Portable owner")
	actor := operationalLabel(t, call(source, true, "identity", "public"), "Actor:")
	actorRef := "refs/hn/actors/" + actor
	before := portableRefs(t, source)
	audit(source, false, "request", "--at", "HEAD", "--rule", "observe")
	if before != portableRefs(t, source) {
		t.Fatal("disabled audit changed accepted facts")
	}
	if _, err := os.Stat(filepath.Join(source, ".hn/policy.json")); !os.IsNotExist(err) {
		t.Fatal("init installed policy")
	}
	call(source, true, "issue", "open", "--body", "Native work survives", "Original signed work")
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{"check": {RequiredResults: 1, TrustedRunners: []string{actor}}}, Audits: &AuditPolicy{Scope: "portable", RuleOwners: []string{actor}, Requesters: []string{actor}, Executors: []string{actor}}}
	raw, _ := json.Marshal(policy)
	policyPublicFile(t, source, ".hn/policy.json", raw)
	policyPublicFile(t, source, ".hn/pipelines/check.json", []byte(`{"version":"hn.pipeline/0","steps":[{"name":"check","command":"git","args":["--version"]}]}`))
	// Git itself supplies an explicit test command; its alias uses Git's own
	// shell support and shell builtins, with no other executable on PATH.
	definition := PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "observe", Command: "git", Args: []string{"-c", `alias.observe=!printf '%s\n' 'HN_AUDIT_FINDING {"key":"Missing-Test","title":"Add coverage","body":"Observed on old code"}'; exit 1`, "observe"}}}}
	raw, _ = json.Marshal(definition)
	policyPublicFile(t, source, ".hn/pipelines/observe.json", raw)
	runOperationalGit(t, source, "add", ".hn")
	runOperationalGit(t, source, "commit", "-qm", "Explicit native and independent audit policy")
	base := runOperationalGit(t, source, "rev-parse", "HEAD")
	runOperationalGit(t, source, "switch", "-qc", "candidate")
	policyPublicFile(t, source, "work.txt", []byte("candidate code\n"))
	runOperationalGit(t, source, "add", "work.txt")
	runOperationalGit(t, source, "commit", "-qm", "Candidate")
	head := runOperationalGit(t, source, "rev-parse", "HEAD")
	call(source, true, "proposal", "open", "--base", base, "--head", head, "Native proposal")
	proposal := latestOperationalEvent(t, source, actorRef).ID
	runOperationalGit(t, source, "switch", "-q", "main")
	status := call(source, true, "proposal", "status", proposal)
	audit(source, true, "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
	audit(source, true, "rule", "authorize", "observe", "--source", "pipeline", "--at", base)
	request := auditPublicID(t, audit(source, true, "request", "--at", base, "--rule", "observe", "--operation", "old-code"))
	before = portableRefs(t, source)
	audit(source, false, "execute", request, "--backend", "host")
	if before != portableRefs(t, source) {
		t.Fatal("unsafe-host refusal published a result")
	}
	result := auditPublicID(t, audit(source, true, "execute", request, "--backend", "host", "--allow-unsafe-host-execution"))
	shown := string(audit(source, true, "show", result))
	if !strings.Contains(shown, `"outcome":"failed"`) || !strings.Contains(shown, base) || !strings.Contains(shown, `"key":"missing-test"`) {
		t.Fatal(shown)
	}
	if status != call(source, true, "proposal", "status", proposal) {
		t.Fatal("audit altered required-check readiness")
	}
	before = portableRefs(t, source)
	call(source, false, "decide", proposal, "--accept")
	call(source, false, "merge", proposal)
	call(source, false, "run", "show", result)
	if before != portableRefs(t, source) || runOperationalGit(t, source, "rev-parse", "HEAD") != base {
		t.Fatal("audit supplied required-check/merge authority")
	}
	// Real native CI remains required and sufficient despite the failed audit.
	call(source, true, "run", "request", proposal, "check")
	run := latestOperationalEvent(t, source, actorRef).ID
	call(source, true, "run", "execute", run, "--backend", "host", "--allow-unsafe-host-execution")
	call(source, true, "decide", proposal, "--accept")
	call(source, true, "merge", proposal)
	if runOperationalGit(t, source, "merge-base", "--is-ancestor", head, "HEAD") != "" {
		t.Fatal("native merge failed")
	}
	// Auditing already merged/old code needs no proposal, and does not open work.
	oldIssues := portableData(t, call(source, true, "issue", "list", "--json"))
	next := auditPublicID(t, audit(source, true, "execute", request, "--backend", "host", "--allow-unsafe-host-execution"))
	if !bytes.Equal(oldIssues, portableData(t, call(source, true, "issue", "list", "--json"))) {
		t.Fatal("execute implicitly opened follow-up work")
	}
	first := audit(source, true, "follow-up", next, "--operation", "triage")
	again := audit(source, true, "follow-up", next, "--operation", "triage")

	var links struct {
		Associations []struct {
			IssueID string `json:"issue_id"`
		}
	}
	if err := json.Unmarshal(first, &links); err != nil || len(links.Associations) != 1 {
		t.Fatalf("follow-up %s %v", first, err)
	}
	followup := links.Associations[0].IssueID
	if !strings.Contains(string(again), followup) {
		t.Fatal("follow-up retry changed association")
	}
	call(source, true, "gh", "issue", "edit", followup, "--body", "Operator triage is preserved")
	before = portableRefs(t, source)
	audit(source, true, "follow-up", result)
	if before != portableRefs(t, source) || !strings.Contains(call(source, true, "issue", "show", followup, "--json"), "Operator triage is preserved") {
		t.Fatal("stable finding dedup rewrote work")
	}

	directory := t.TempDir()
	foreign, blobs := migrationForeignSnapshot()
	foreignFile := filepath.Join(directory, "foreign.hn")
	file, err := os.Create(foreignFile)
	if err != nil {
		t.Fatal(err)
	}
	err = migration.WriteArchive(context.Background(), file, foreign, blobs, migration.DefaultLimits())
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	foreignPlan := filepath.Join(directory, "foreign-import")
	call(directory, true, "migrate", "plan", "--source-kind", "archive", "--source-repository", foreignFile, "--destination-kind", "local", "--destination-repository", source, "--session", foreignPlan, "--decision", "local/governance=preserve")
	call(directory, true, "migrate", "apply", "--session", foreignPlan, "--confirm-apply")
	var foreignImports []string
	if err := json.Unmarshal([]byte(call(directory, true, "migrate", "imports", "--repository", source)), &foreignImports); err != nil || len(foreignImports) != 1 {
		t.Fatal(foreignImports, err)
	}
	archive := filepath.Join(directory, "combined.hn")
	session := filepath.Join(directory, "export")
	call(directory, true, "migrate", "plan", "--source-kind", "imported", "--source-repository", source, "--snapshot", foreignImports[0], "--source-git", source, "--ref", "refs/heads/main", "--ref", actorRef, "--ref", "refs/hn/proposals/"+strings.TrimPrefix(proposal, "sha256:"), "--destination-kind", "archive", "--destination-repository", archive, "--session", session)
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatal("planning published archive")
	}
	sourceOffline := source + "-offline"
	if err := os.Rename(source, sourceOffline); err != nil {
		t.Fatal(err)
	}
	call(directory, false, "migrate", "apply", "--session", session)
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatal("unconfirmed apply published archive")
	}
	call(directory, true, "migrate", "apply", "--session", session, "--confirm-apply")
	for digest, expected := range blobs {
		if got := call(directory, true, "migrate", "inspect", "--archive", archive, "--blob", digest); got != string(expected) {
			t.Fatal("foreign byte preservation mismatch")
		}
	}
	destination := migrationCLIRepository(t)
	policyPublicFile(t, destination, ".hn/policy.json", []byte("destination policy sentinel\n"))
	sentinel, _ := os.ReadFile(filepath.Join(destination, ".hn/policy.json"))
	before = portableRefs(t, destination)
	plan := filepath.Join(directory, "import")
	args := []string{"migrate", "plan", "--source-kind", "archive", "--source-repository", archive, "--destination-kind", "local", "--destination-repository", destination, "--session", plan}
	call(directory, false, args...)
	if before != portableRefs(t, destination) {
		t.Fatal("missing governance choice changed accepted refs")
	}
	call(directory, true, append(args, "--decision", "local/governance=preserve")...)
	call(directory, true, "migrate", "apply", "--session", plan, "--confirm-apply")
	gotPolicy, _ := os.ReadFile(filepath.Join(destination, ".hn/policy.json"))
	if !bytes.Equal(sentinel, gotPolicy) {
		t.Fatal("import replaced destination policy")
	}
	// Exact event payload, signature and attachment objects survive admission.
	commits := strings.Fields(runOperationalGit(t, sourceOffline, "rev-list", actorRef))
	for _, commit := range commits {
		for _, name := range strings.Fields(runOperationalGit(t, sourceOffline, "ls-tree", "--name-only", commit)) {
			a := portableGitBytes(t, sourceOffline, "show", commit+":"+name)
			b := portableGitBytes(t, destination, "show", commit+":"+name)
			if !bytes.Equal(a, b) {
				t.Fatalf("signed bytes changed: %s:%s", commit, name)
			}
		}
	}
	audit(destination, true, "show", next)
	audit(destination, true, "logs", next)
	before = portableRefs(t, destination)
	audit(destination, false, "authority", "show")
	audit(destination, false, "execute", request, "--backend", "host", "--allow-unsafe-host-execution")
	call(destination, false, "identity", "public")
	if before != portableRefs(t, destination) {
		t.Fatal("keyless imported history gained authority")
	}
	imports := call(directory, true, "migrate", "imports", "--repository", destination)
	var digests []string
	if err := json.Unmarshal([]byte(imports), &digests); err != nil || len(digests) != 1 {
		t.Fatal(imports, err)
	}
	inspection := call(directory, true, "migrate", "inspect", "--repository", destination, "--snapshot", digests[0])
	if !strings.Contains(inspection, "source commenter") || !strings.Contains(inspection, "Source-only conversation") {
		t.Fatal("attributed foreign records lost")
	}
	for digest, expected := range blobs {
		if got := call(directory, true, "migrate", "inspect", "--repository", destination, "--snapshot", digests[0], "--blob", digest); got != string(expected) {
			t.Fatal("offline foreign bytes changed")
		}
	}
	// Hostile public import refuses before any accepted visibility or file effect.
	hostile := filepath.Join(directory, "hostile.hn")
	var bad bytes.Buffer
	tw := tar.NewWriter(&bad)
	if err := tw.WriteHeader(&tar.Header{Name: "../escaped", Mode: 0600, Size: 1}); err != nil {
		t.Fatal(err)
	}
	_, _ = tw.Write([]byte("x"))
	_ = tw.Close()
	policyPublicFile(t, directory, "hostile.hn", bad.Bytes())
	snapshot := policyPublicSnapshot(t, destination)
	call(directory, false, "migrate", "plan", "--source-kind", "archive", "--source-repository", hostile, "--destination-kind", "local", "--destination-repository", destination, "--session", filepath.Join(directory, "rejected"), "--decision", "local/governance=preserve")
	if !reflect.DeepEqual(snapshot, policyPublicSnapshot(t, destination)) {
		t.Fatal("hostile archive changed destination")
	}
	if _, err := os.Stat(filepath.Join(directory, "escaped")); !os.IsNotExist(err) {
		t.Fatal("archive traversal escaped")
	}
	// Continue using a deliberately new local signer; historical actors stay observations.
	call(destination, true, "init", "--name", "Local continuation")
	audit(destination, false, "authority", "show")
	own := operationalLabel(t, call(destination, true, "identity", "public"), "Actor:")
	if own == actor {
		t.Fatal("import installed source signer")
	}
	call(destination, true, "issue", "open", "Continue independently")
	if runOperationalGit(t, destination, "for-each-ref", "--format=%(refname)", "refs/hn/actors/") != "refs/hn/actors/"+own {
		t.Fatal("source history became signing-owned")
	}
	// Installing policy and adopting an anchor are separate deliberate local actions.
	configureOperationalGit(t, destination, "Local continuation")
	policy.Maintainers = []string{own}
	policy.Audits = &AuditPolicy{Scope: "local-only", RuleOwners: []string{own}, Requesters: []string{own}, Executors: []string{own}}
	policy.Pipelines = map[string]PipelinePolicy{}
	localPolicy, _ := json.Marshal(policy)
	policyPublicFile(t, destination, ".hn/policy.json", localPolicy)
	policyPublicFile(t, destination, ".hn/pipelines/observe.json", raw)
	runOperationalGit(t, destination, "add", ".hn")
	runOperationalGit(t, destination, "commit", "-qm", "Explicit local audit policy")
	audit(destination, true, "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
	audit(destination, true, "rule", "authorize", "observe", "--source", "pipeline", "--at", "HEAD")
	localRequest := auditPublicID(t, audit(destination, true, "request", "--at", "HEAD", "--rule", "observe"))
	audit(destination, true, "execute", localRequest, "--backend", "host", "--allow-unsafe-host-execution")
	// Ordinary Git replication carries audit relationships without private anchor/key.
	remote := filepath.Join(directory, "remote.git")
	runOperationalGit(t, sourceOffline, "init", "--bare", "-q", remote)
	runOperationalGit(t, sourceOffline, "remote", "add", "origin", remote)
	call(sourceOffline, true, "sync", "origin")
	reader := migrationCLIRepository(t)
	runOperationalGit(t, reader, "remote", "add", "origin", remote)
	call(reader, true, "replication", "select", "origin", "--actor", actor, "--proposal", proposal)
	call(reader, true, "sync", "origin")
	if !strings.Contains(string(audit(reader, true, "show", next)), next) || !strings.Contains(call(reader, true, "issue", "show", followup, "--json"), request) {
		t.Fatal("ordinary Git lost audit/follow-up relationships")
	}
	audit(reader, true, "logs", next)
	before = portableRefs(t, reader)
	audit(reader, false, "authority", "show")
	audit(reader, false, "execute", request, "--backend", "host", "--allow-unsafe-host-execution")
	if before != portableRefs(t, reader) {
		t.Fatal("replication installed authority")
	}
}

func wp25Record(t *testing.T, event map[string]any) {
	t.Helper()
	root := os.Getenv("HN_WP25_EVIDENCE")
	if root == "" {
		return
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		t.Fatal(e)
	}
	event["test"] = t.Name()
	f, e := os.OpenFile(filepath.Join(root, "new-controls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	e = json.NewEncoder(f).Encode(event)
	ce := f.Close()
	if e != nil || ce != nil {
		t.Fatal(e, ce)
	}
}
func wp25ArtifactCall(f *artifactCLIFixture, input, code string, args ...string) map[string]any {
	f.t.Helper()
	cmd := exec.Command(f.binary, args...)
	cmd.Dir = f.repo
	cmd.Env = operationalTestEnvironmentWith(f.home, f.overrides)
	cmd.Stdin = strings.NewReader(input)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	e := cmd.Run()
	wp25Record(f.t, map[string]any{"program": f.binary, "cwd": f.repo, "argv": args, "input": input, "stdout": out.String(), "stderr": stderr.String(), "exit": fmt.Sprint(e), "expected_code": code})
	var v map[string]any
	if je := json.Unmarshal(out.Bytes(), &v); je != nil {
		f.t.Fatalf("new caller JSON %v %s %s", je, &out, &stderr)
	}
	if code == "" {
		if e != nil || v["ok"] != true {
			f.t.Fatalf("new caller positive %v %s %s", e, &out, &stderr)
		}
	} else {
		errorValue, ok := v["error"].(map[string]any)
		if e == nil || v["ok"] != false || v["data"] != nil || !ok || errorValue["code"] != code {
			f.t.Fatalf("new caller want %s: %v %s %s", code, e, &out, &stderr)
		}
	}
	return v
}
func wp25RevisionCall(f *artifactCLIFixture, kind, input, key, code string) map[string]any {
	return wp25ArtifactCall(f, input, code, "artifact", kind, "--input", "-", "--actor", f.actor, "--operation", key, "--json")
}
func wp25Publish(f *artifactCLIFixture, input, key, code string) map[string]any {
	return wp25ArtifactCall(f, input, code, "artifact", "type", "publish", "--input", "-", "--actor", f.actor, "--operation", key, "--json")
}
func wp25Unchanged(f *artifactCLIFixture, before map[string]string) {
	f.t.Helper()
	after := f.snapshot()
	wp25Record(f.t, map[string]any{"before": before, "after": after, "expected": "byte-identical objects/refs/keys/intent state"})
	if !reflect.DeepEqual(before, after) {
		f.t.Fatal("new bound/refusal changed protected bytes or paths")
	}
}

// Public-fixture construction below is SOURCE-ASSISTED;
// admission, mutation, reads and refusals always execute HN_WP17_BINARY.
func wp25NativeLimits(t *testing.T) {
	if os.Getenv("HN_WP17_BINARY") == "" {
		t.Fatal("native limits require an independently supplied host")
	}
	grants := []string{}
	for _, family := range []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow"} {
		grants = append(grants, "native:"+family+"=read,create,revise,resolve")
	}
	f := newNativeHostFixture(t, grants...)
	actor := f.actor()
	serial := 0
	copyState := func(s map[string]any) map[string]any {
		raw, _ := json.Marshal(s)
		var out map[string]any
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	project := func() map[string]any {
		return map[string]any{"schema": "hn.project/1", "title": "Project", "body": "", "archived": false}
	}
	workflow := func() map[string]any {
		return map[string]any{"schema": "hn.workflow/1", "name": "stored", "title": "Stored", "description": "", "enabled": false}
	}
	pipeline := func() map[string]any {
		return map[string]any{"version": "hn.pipeline/0", "steps": []any{map[string]any{"name": "Inert", "command": "touch", "args": []any{"WP25-MUST-NOT-EXECUTE"}}}}
	}
	open := func(family string, state map[string]any) string {
		serial++
		key := fmt.Sprintf("wp25-native-%05d", serial)
		result := nativeOK(f.t, wp25NativeRequests(f.t, f, nativeMutation(family, actor, key, state))[0])
		id, ok := result["event_id"].(string)
		if !ok || !strings.HasPrefix(id, "sha256:") || result["resource_id"] != id || result["head_count"] != float64(1) || result["replayed"] != false || result["family"] != family {
			f.t.Fatalf("native ceiling publication identity: %v", result)
		}
		recovered := nativeOK(f.t, wp25NativeRequests(f.t, f, nativeLookup(family, actor, key))[0])
		if recovered["status"] != "found" || recovered["event_id"] != id || recovered["resource_id"] != id {
			f.t.Fatalf("ceiling operation identity: %v", recovered)
		}
		return id
	}
	// Host typed violations terminate the session; semantic native violations
	// return a correlated error. Neither path may alter any native state.
	reject := func(family string, state map[string]any, origin, code string) {
		serial++
		request := nativeMutation(family, actor, fmt.Sprintf("wp25-native-%05d", serial), state)
		before := hostNativeSnapshot(f.t, f.repo)
		if origin == "host" {
			wp25NativeTerminal(f.t, f, request, code)
		} else {
			response := wp25NativeRequests(f.t, f, request)[0]
			nativeDenial(f.t, response, origin, code, "do_not_retry")
			if err, ok := response["error"].(map[string]any); !ok || len(err) != 5 || err["operation"] != nil {
				f.t.Fatalf("closed native refusal: %v", response)
			}
		}
		nativeSnapshotEqual(f.t, f, before)
	}
	pair := func(name, family string, good, bad map[string]any, origin, code string) {
		t.Run(name, func(t *testing.T) {
			wp25Record(t, map[string]any{"control": "native-boundary-expectation", "name": name, "family": family, "positive_state": good, "negative_state": bad, "negative_origin": origin, "negative_code": code, "negative_recovery": "do_not_retry"})
			local := *f
			local.t = t
			prior := f
			f = &local
			defer func() { f = prior }()
			open(family, good)
			reject(family, bad, origin, code)
		})
	}
	// N02 native identifiers, independent field/option/view names and identifiers.
	for _, which := range []string{"field-id", "option-id", "view-id", "field-name", "option-name", "view-name"} {
		good := project()
		field := map[string]any{"id": "field", "name": "Field", "type": "single_select", "options": []any{map[string]any{"id": "option", "name": "Option"}}}
		view := map[string]any{"id": "view", "name": "View", "layout": "table"}
		good["fields"], good["views"] = []any{field}, []any{view}
		cap := 64
		if strings.HasSuffix(which, "name") {
			cap = 256
		}
		set := func(state map[string]any, value string) {
			field := state["fields"].([]any)[0].(map[string]any)
			view := state["views"].([]any)[0].(map[string]any)
			switch which {
			case "field-id":
				field["id"] = value
			case "option-id":
				field["options"].([]any)[0].(map[string]any)["id"] = value
			case "view-id":
				view["id"] = value
			case "field-name":
				field["name"] = value
			case "option-name":
				field["options"].([]any)[0].(map[string]any)["name"] = value
			case "view-name":
				view["name"] = value
			}
		}
		set(good, strings.Repeat("a", cap))
		bad := copyState(good)
		set(bad, strings.Repeat("a", cap+1))
		code := "invalid_input"
		if cap == 256 {
			code = "resource_limit"
		}
		pair("N02-"+which, "project", good, bad, "host", code)
	}
	for _, value := range []string{"", ".", ".."} {
		s := project()
		s["fields"] = []any{map[string]any{"id": value, "name": "Bad", "type": "text"}}
		reject("project", s, "host", "invalid_input")
	}
	// N03 independent collection ceilings: use short elements so the aggregate
	// state/event cap does not dominate the count being tested.
	for _, which := range []string{"fields", "options", "views"} {
		cap := 64
		if which == "views" {
			cap = 16
		}
		makeState := func(n int) map[string]any {
			s := project()
			items := []any{}
			for i := 0; i < n; i++ {
				id := fmt.Sprintf("i%02d", i)
				switch which {
				case "fields":
					items = append(items, map[string]any{"id": id, "name": "F", "type": "text"})
				case "options":
					items = append(items, map[string]any{"id": id, "name": "O"})
				case "views":
					items = append(items, map[string]any{"id": id, "name": "V", "layout": "table"})
				}
			}
			if which == "options" {
				s["fields"] = []any{map[string]any{"id": "select", "name": "Select", "type": "single_select", "options": items}}
			} else {
				s[which] = items
			}
			return s
		}
		pair("N03-"+which, "project", makeState(cap), makeState(cap+1), "host", "resource_limit")
	}
	// One real pinned project carries all item scalar/count fixtures.
	schema := project()
	fields := []any{map[string]any{"id": "text", "name": "Text", "type": "text"}, map[string]any{"id": "number", "name": "Number", "type": "number"}, map[string]any{"id": "date", "name": "Date", "type": "date"}, map[string]any{"id": "select", "name": "Select", "type": "single_select", "options": []any{map[string]any{"id": strings.Repeat("o", 64), "name": "Option"}}}}
	for i := 4; i < 64; i++ {
		fields = append(fields, map[string]any{"id": fmt.Sprintf("f%02d", i), "name": "F", "type": "text"})
	}
	schema["fields"] = fields
	projectID := open("project", schema)
	item := func() map[string]any {
		return map[string]any{"schema": "hn.project-item/1", "project_id": projectID, "project_revision": projectID, "target": map[string]any{"kind": "draft", "title": "Draft", "body": ""}, "archived": false, "position": "0"}
	}
	values := map[string]any{"text": map[string]any{"type": "text", "text": ""}, "number": map[string]any{"type": "number", "number": "0"}, "date": map[string]any{"type": "date", "date": "2024-02-29"}, "select": map[string]any{"type": "single_select", "option_id": strings.Repeat("o", 64)}}
	for i := 4; i < 64; i++ {
		values[fmt.Sprintf("f%02d", i)] = map[string]any{"type": "text", "text": ""}
	}
	good := item()
	good["values"] = values
	bad := copyState(good)
	bad["values"].(map[string]any)["extra"] = map[string]any{"type": "text", "text": ""}
	pair("N03-item-values", "project-item", good, bad, "host", "resource_limit")
	for _, which := range []string{"text", "number", "date", "select", "position", "url"} {
		good := item()
		bad := item()
		code := "invalid_input"
		switch which {
		case "text":
			good["values"] = map[string]any{"text": map[string]any{"type": "text", "text": strings.Repeat("x", 4096)}}
			bad["values"] = map[string]any{"text": map[string]any{"type": "text", "text": strings.Repeat("x", 4097)}}
			code = "resource_limit"
		case "number":
			good["values"] = map[string]any{"number": map[string]any{"type": "number", "number": strings.Repeat("9", 64)}}
			bad["values"] = map[string]any{"number": map[string]any{"type": "number", "number": strings.Repeat("9", 65)}}
		case "date":
			good["values"] = map[string]any{"date": map[string]any{"type": "date", "date": "2024-02-29"}}
			bad["values"] = map[string]any{"date": map[string]any{"type": "date", "date": "2024-02-290"}}
		case "select":
			good["values"] = map[string]any{"select": map[string]any{"type": "single_select", "option_id": strings.Repeat("o", 64)}}
			bad["values"] = map[string]any{"select": map[string]any{"type": "single_select", "option_id": strings.Repeat("o", 65)}}
		case "position":
			good["position"], bad["position"] = strings.Repeat("9", 64), strings.Repeat("9", 65)
		case "url":
			prefix := "https://example.invalid/"
			good["target"] = map[string]any{"kind": "external", "url": prefix + strings.Repeat("x", 4096-len(prefix))}
			bad["target"] = map[string]any{"kind": "external", "url": prefix + strings.Repeat("x", 4097-len(prefix))}
		}
		pair("N04-"+which, "project-item", good, bad, "host", code)
	}
	for _, decimal := range []string{"1e2", "-0", "1.0"} {
		s := item()
		s["position"] = decimal
		reject("project-item", s, "host", "invalid_input")
	}
	s := item()
	s["position"] = ""
	open("project-item", s)
	// N05 metadata count, whole namespaced key and value are independent pairs.
	for _, which := range []string{"entries", "key", "value"} {
		good := nativeIssueState("Metadata")
		bad := copyState(good)
		a, b := map[string]any{}, map[string]any{}
		switch which {
		case "entries":
			for i := 0; i < 64; i++ {
				key := fmt.Sprintf("wp25/k%02d", i)
				a[key], b[key] = "x", "x"
			}
			b["wp25/extra"] = "x"
		case "key":
			a["x/"+strings.Repeat("k", 126)] = "x"
			b["x/"+strings.Repeat("k", 127)] = "x"
		case "value":
			a["wp25/key"] = strings.Repeat("x", 4096)
			b["wp25/key"] = strings.Repeat("x", 4097)
		}
		good["metadata"], bad["metadata"] = a, b
		pair("N05-"+which, "issue", good, bad, "host", "resource_limit")
	}
	// N06 inherited title/body ceilings apply independently to every title/body
	// owning family. Empty body is valid; titles stay nonblank.
	for _, family := range []string{"issue", "project", "discussion", "workflow", "project-item"} {
		for _, which := range []string{"title", "body"} {
			var good map[string]any
			switch family {
			case "issue":
				good = nativeIssueState("Text")
			case "project":
				good = project()
			case "discussion":
				good = map[string]any{"schema": "hn.discussion/1", "title": "Thread", "body": "", "category": "general", "status": "open"}
			case "workflow":
				good = workflow()
			case "project-item":
				good = item()
			}
			bad := copyState(good)
			cap := 1024
			if which == "body" {
				cap = 65536
			}
			key := which
			if family == "workflow" && which == "body" {
				key = "description"
			}
			if family == "project-item" {
				good["target"].(map[string]any)[key] = strings.Repeat("x", cap)
				bad["target"].(map[string]any)[key] = strings.Repeat("x", cap+1)
			} else {
				good[key] = strings.Repeat("x", cap)
				bad[key] = strings.Repeat("x", cap+1)
			}
			pair("N06-"+family+"-"+which, family, good, bad, "host", "resource_limit")
		}
	}
	// N07 issue collections and nested element maxima. Keep count and byte
	// maxima separate: 64 simultaneous 4096-byte criteria exceed the enclosing
	// 256KiB state/event bound and are deliberately NOT claimed as an admission.
	for _, which := range []string{"criteria", "labels", "assignees"} {
		makeState := func(n int) map[string]any {
			s := nativeIssueState("Collection")
			a := []any{}
			for i := 0; i < n; i++ {
				id := fmt.Sprintf("i%02d", i)
				if which == "criteria" {
					a = append(a, map[string]any{"id": id, "text": "Criterion"})
				} else {
					a = append(a, id)
				}
			}
			s[which] = a
			return s
		}
		pair("N07-"+which, "issue", makeState(64), makeState(65), "host", "resource_limit")
	}
	for _, which := range []string{"criterion-id", "criterion-text", "label", "assignee"} {
		good := nativeIssueState("Nested")
		bad := copyState(good)
		cap := 256
		if which == "criterion-id" {
			cap = 64
		}
		if which == "criterion-text" {
			cap = 4096
		}
		set := func(s map[string]any, n int) {
			switch which {
			case "criterion-id":
				s["criteria"] = []any{map[string]any{"id": strings.Repeat("x", n), "text": "Criterion"}}
			case "criterion-text":
				s["criteria"] = []any{map[string]any{"id": "criterion", "text": strings.Repeat("x", n)}}
			case "label":
				s["labels"] = []any{strings.Repeat("x", n)}
			case "assignee":
				s["assignees"] = []any{strings.Repeat("x", n)}
			}
		}
		set(good, cap)
		set(bad, cap+1)
		pair("N07-"+which, "issue", good, bad, "host", "resource_limit")
	}
	// Source-assisted target production; the actual candidate must resolve every
	// relation target and accept exactly128 relations, then reject129 pre-effect.
	relationIDs, relationEntries := changesBulkProducer(t, f, make([]string, 128))
	wp25Record(t, map[string]any{"control": "source-assisted-native-relation-targets", "source_assisted": true, "ids": relationIDs, "public_entries": relationEntries})
	good = nativeIssueState("Relations")
	rels := []any{}
	for _, id := range relationIDs {
		rels = append(rels, map[string]any{"kind": "related", "target": id})
	}
	good["relations"] = rels
	bad = copyState(good)
	bad["relations"] = append(bad["relations"].([]any), map[string]any{"kind": "blocks", "target": relationIDs[0]})
	pair("N07-relations", "issue", good, bad, "host", "resource_limit")
	// N09 discussion category and reply body have their own minima/maxima.
	thread := map[string]any{"schema": "hn.discussion/1", "title": "Thread", "body": "", "category": strings.Repeat("c", 256), "status": "open"}
	bad = copyState(thread)
	bad["category"] = strings.Repeat("c", 257)
	pair("N09-category", "discussion", thread, bad, "host", "resource_limit")
	threadID := open("discussion", thread)
	reply := map[string]any{"schema": "hn.discussion-reply/1", "discussion_id": threadID, "body": strings.Repeat("x", 65536)}
	bad = copyState(reply)
	bad["body"] = strings.Repeat("x", 65537)
	pair("N09-reply-body", "discussion-reply", reply, bad, "host", "resource_limit")
	bad = copyState(reply)
	bad["body"] = ""
	reject("discussion-reply", bad, "host", "invalid_input")
	bad = copyState(thread)
	bad["category"] = ""
	reject("discussion", bad, "host", "invalid_input")
	// N10 sources: independent count and per-source bytes, correct distinct
	// digests. Eight simultaneous1MiB sources are dominated by2MiB aggregate.
	for _, which := range []string{"sources", "source-bytes", "source-format"} {
		makeState := func(n int) map[string]any {
			s := workflow()
			docs := []any{}
			if which == "sources" {
				for i := 0; i < n; i++ {
					content := fmt.Sprintf("source-%02d", i)
					docs = append(docs, map[string]any{"format": "yaml", "content": content, "digest": wp25NativeDigest([]byte(content))})
				}
			} else {
				content := "source"
				format := "yaml"
				if which == "source-bytes" {
					content = strings.Repeat("x", n)
				} else {
					format = strings.Repeat("f", n)
				}
				docs = append(docs, map[string]any{"format": format, "content": content, "digest": wp25NativeDigest([]byte(content))})
			}
			s["source_documents"] = docs
			return s
		}
		cap := 8
		code := "resource_limit"
		if which == "source-bytes" {
			cap = 1048576
		}
		if which == "source-format" {
			cap = 64
			code = "invalid_input"
		}
		pair("N10-"+which, "workflow", makeState(cap), makeState(cap+1), map[string]string{"source-format": "native", "sources": "host", "source-bytes": "host"}[which], code)
	}
	// N11 stored pipeline limits. Nothing here invokes a pipeline runner.
	for _, which := range []string{"steps", "args", "argument-bytes", "timeout", "step-name"} {
		makeState := func(n int) map[string]any {
			s := workflow()
			p := pipeline()
			step := p["steps"].([]any)[0].(map[string]any)
			switch which {
			case "steps":
				steps := []any{}
				for i := 0; i < n; i++ {
					steps = append(steps, map[string]any{"name": fmt.Sprintf("step%02d", i), "command": "touch", "args": []any{"WP25-MUST-NOT-EXECUTE"}})
				}
				p["steps"] = steps
			case "args":
				args := []any{}
				for i := 0; i < n; i++ {
					args = append(args, "x")
				}
				step["args"] = args
			case "argument-bytes":
				step["args"] = []any{strings.Repeat("x", n)}
			case "timeout":
				step["timeoutSeconds"] = n
			case "step-name":
				step["name"] = strings.Repeat("x", n)
			}
			s["pipeline"] = p
			s["enabled"] = true
			return s
		}
		cap := 64
		code := "resource_limit"
		switch which {
		case "args", "step-name":
			cap = 256
		case "argument-bytes":
			cap = 16384
		case "timeout":
			cap = 3600
			code = "invalid_input"
		}
		pair("N11-"+which, "workflow", makeState(cap), makeState(cap+1), "host", code)
		if which == "steps" {
			reject("workflow", makeState(0), "host", "invalid_input")
		}
		if which == "timeout" {
			open("workflow", makeState(0))
			reject("workflow", makeState(-1), "host", "invalid_input")
		}
	}
	// Pipeline canonical bytes are charged independently inside the2MiB state.
	makePipeline := func(n int) map[string]any {
		s := workflow()
		p := pipeline()
		p["steps"].([]any)[0].(map[string]any)["command"] = "x"
		raw, _ := json.Marshal(p)
		padding := n - len(raw) + 1
		if padding < 1 {
			t.Fatal("pipeline fixture underflow")
		}
		p["steps"].([]any)[0].(map[string]any)["command"] = strings.Repeat("x", padding)
		check, _ := json.Marshal(p)
		if len(check) != n {
			t.Fatalf("pipeline fixture bytes %d != %d", len(check), n)
		}
		s["pipeline"] = p
		s["enabled"] = true
		return s
	}
	pair("N10-pipeline-canonical-bytes", "workflow", makePipeline(1048576), makePipeline(1048577), "host", "resource_limit")
	for _, which := range []string{"nul-command", "nul-argument", "escaping-command", "escaping-directory", "enabled-no-pipeline", "bad-source-digest"} {
		s := workflow()
		s["pipeline"] = pipeline()
		origin := "native"
		switch which {
		case "nul-command":
			s["pipeline"].(map[string]any)["steps"].([]any)[0].(map[string]any)["command"] = "x\x00y"
			origin = "host"
		case "nul-argument":
			s["pipeline"].(map[string]any)["steps"].([]any)[0].(map[string]any)["args"] = []any{"x\x00y"}
			origin = "host"
		case "escaping-command":
			s["pipeline"].(map[string]any)["steps"].([]any)[0].(map[string]any)["command"] = "../escape"
		case "escaping-directory":
			s["pipeline"].(map[string]any)["steps"].([]any)[0].(map[string]any)["workingDirectory"] = "../escape"
		case "enabled-no-pipeline":
			delete(s, "pipeline")
			s["enabled"] = true
		case "bad-source-digest":
			s["source_documents"] = []any{map[string]any{"format": "yaml", "content": "source", "digest": wp25NativeDigest([]byte("different"))}}
		}
		reject("workflow", s, origin, "invalid_input")
	}
	s = workflow()
	s["name"] = strings.Repeat("w", 64)
	bad = copyState(s)
	bad["name"] = strings.Repeat("w", 65)
	pair("N02-workflow-name", "workflow", s, bad, "host", "invalid_input")
	if _, err := os.Stat(filepath.Join(f.repo, "WP25-MUST-NOT-EXECUTE")); !os.IsNotExist(err) {
		t.Fatal("stored pipeline executed", err)
	}
	t.Log("dominated combinations:64 metadata values at4096B,64 criteria at4096B,8 workflow sources at1MiB each cannot all be admitted simultaneously under their enclosing authenticated aggregate caps; independent dimensions above use small other fields")
	t.Run("N08-expected-parents", wp25NativeParentLimit)
	t.Run("N01-authenticated-payload", wp25NativeAuthenticatedLimit)
}

func wp25NativeDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", sum)
}

func wp25NativeTerminal(t *testing.T, f *nativeHostFixture, request map[string]any, code string) {
	t.Helper()
	before := hostNativeSnapshot(t, f.repo)
	raw, err := json.Marshal([]any{request})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(f.binary, "extension", "run", "example.native/tool", "--json")
	cmd.Dir, cmd.Env = f.repo, f.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	after := hostNativeSnapshot(t, f.repo)
	wp25Record(t, map[string]any{"control": "native-terminal-refusal", "binary": f.binary, "request_raw": string(raw), "request_sha256": wp25NativeDigest(raw), "stdout_raw": stdout.String(), "stderr_raw": stderr.String(), "process_error": fmt.Sprint(err), "expected_origin": "host", "expected_code": code, "expected_recovery": "do_not_retry", "protected_before_sha256": wp25NativeSnapshotDigest(before), "protected_after_sha256": wp25NativeSnapshotDigest(after)})
	var result map[string]any
	if err == nil || json.Unmarshal(stdout.Bytes(), &result) != nil {
		t.Fatalf("terminal host refusal %s: %v %s %s", code, err, &stdout, &stderr)
	}
	nativeDenial(t, result, "host", code, "do_not_retry")
	problem, ok := result["error"].(map[string]any)
	if !ok || len(problem) != 5 || problem["operation"] != nil {
		t.Fatalf("closed terminal host refusal: %v", result)
	}
}

// SOURCE-ASSISTED deterministic signed public fixture construction. Only public
// event/signature Git trees are imported; no private signer/keyring is copied.
// Each event below is a first event for its own independent actor. Resetting the
// ref also keeps an intentionally over-limit replacement a true first event,
// so its refusal cannot accidentally be caused by broken actor ancestry.
func wp25NativePublicFixture(t *testing.T, f *nativeHostFixture, events []Event, keys []ed25519.PrivateKey) []string {
	t.Helper()
	if len(events) != len(keys) {
		t.Fatal("public fixture key count")
	}
	var stream bytes.Buffer
	ids := []string{}
	zeros := strings.Repeat("0", 40)
	if f.git("rev-parse", "--show-object-format") == "sha256" {
		zeros = strings.Repeat("0", 64)
	}
	for i, event := range events {
		raw, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		signature := base64.RawStdEncoding.EncodeToString(ed25519.Sign(keys[i], raw))
		id := wp25NativeDigest(raw)
		ids = append(ids, id)
		ref := "refs/hn/actors/" + event.Actor
		wp25Record(t, map[string]any{"control": "source-assisted-public-native-fixture", "actor": event.Actor, "event_id": id, "event_payload_raw": string(raw), "signature_base64": signature, "source_assisted": true, "candidate_observation": false})
		message := "hn event " + id
		fmt.Fprintf(&stream, "reset %s\nfrom %s\n\ncommit %s\ncommitter WP25 public fixture <fixture@hn.invalid> 1793433600 +0000\ndata %d\n%s\ndeleteall\nM 100644 inline event.json\ndata %d\n", ref, zeros, ref, len(message), message, len(raw))
		stream.Write(raw)
		fmt.Fprintf(&stream, "\nM 100644 inline signature\ndata %d\n%s\n\n", len(signature), signature)
	}
	cmd := exec.Command("git", "fast-import", "--quiet")
	cmd.Dir, cmd.Env, cmd.Stdin = f.repo, f.env, &stream
	if raw, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("source-assisted public fixture import: %v %s", err, raw)
	}
	t.Logf("SOURCE-ASSISTED public fixture construction: %d independently signed first-event actor streams; subsequent caller observations use the supplied candidate", len(events))
	return ids
}
func wp25NativePublicEvent(label, kind string) (Event, ed25519.PrivateKey) {
	seed := sha256.Sum256([]byte("WP25 public fixture " + label))
	key := ed25519.NewKeyFromSeed(seed[:])
	public := key.Public().(ed25519.PublicKey)
	sum := sha256.Sum256(public)
	return Event{Protocol: "hn/0", Kind: kind, Actor: hex.EncodeToString(sum[:]), ActorName: "WP25 public fixture", PublicKey: base64.RawStdEncoding.EncodeToString(public), Sequence: 1, Timestamp: "2026-10-31T08:00:00Z"}, key
}

func wp25NativeParentLimit(t *testing.T) {
	f := newNativeHostFixture(t, "native:issue=read,create,revise,resolve")
	actor := f.actor()
	state := nativeIssueState("Parent ceiling")
	root := nativeOK(t, wp25NativeRequests(t, f, nativeMutation("issue", actor, "wp25-parent-root", state))[0])["event_id"].(string)
	// Construct200 genuine, independently signed concurrent revisions of the
	// same visible root. Their validation is independently performed by candidate.
	events := []Event{}
	keys := []ed25519.PrivateKey{}
	for i := 0; i < 200; i++ {
		event, key := wp25NativePublicEvent(fmt.Sprintf("parent-%03d", i), "issue.revise")
		event.Subject = root
		event.Parents = []string{root}
		event.Intent = "revise"
		event.Issue = &IssueState{Title: fmt.Sprintf("Concurrent %03d", i), Status: "open", Criteria: []IssueCriterion{}, Labels: []string{}, Assignees: []string{}, Relations: []IssueRelation{}, Metadata: map[string]string{}}
		events = append(events, event)
		keys = append(keys, key)
	}
	heads := wp25NativePublicFixture(t, f, events, keys)
	sort.Strings(heads)
	before := hostNativeSnapshot(t, f.repo)
	page := nativeOK(t, wp25NativeRequests(t, f, nativeReadRequest("issue", root))[0])
	items := page["items"].([]any)
	if len(items) != 1 {
		t.Fatal("200-parent source root absent", page)
	}
	view := items[0].(map[string]any)
	if view["conflict"] != true || view["state"] != nil || len(view["heads"].([]any)) != 200 {
		t.Fatal("candidate did not verify200 genuine concurrent heads", view)
	}
	actual := []string{}
	for _, h := range view["heads"].([]any) {
		actual = append(actual, h.(map[string]any)["id"].(string))
	}
	sort.Strings(actual)
	if !reflect.DeepEqual(actual, heads) {
		t.Fatal("candidate changed independent200-head expected set")
	}
	nativeSnapshotEqual(t, f, before)
	expected := []any{}
	for _, id := range heads {
		expected = append(expected, id)
	}
	request := nativeMutation("issue", actor, "wp25-parent-over", nativeIssueState("Resolved"))
	params := request["params"].(map[string]any)
	params["intent"], params["id"] = "resolve", root
	over := append([]string{}, heads...)
	over = append(over, wp25NativeDigest([]byte("independent201st parent")))
	sort.Strings(over)
	overAny := []any{}
	for _, id := range over {
		overAny = append(overAny, id)
	}
	params["expected"] = overAny
	before = hostNativeSnapshot(t, f.repo)
	wp25NativeTerminal(t, f, request, "resource_limit")
	nativeSnapshotEqual(t, f, before)
	params["expected"], params["operation"] = expected, "wp25-parent-good"
	result := nativeOK(t, wp25NativeRequests(t, f, request)[0])
	if result["resource_id"] != root || result["head_count"] != float64(1) || result["conflict"] != false || result["replayed"] != false {
		t.Fatal("actual200-parent resolution failed", result)
	}
	recovered := nativeOK(t, wp25NativeRequests(t, f, nativeLookup("issue", actor, "wp25-parent-good"))[0])
	if recovered["status"] != "found" || recovered["event_id"] != result["event_id"] || recovered["resource_id"] != root {
		t.Fatal("200-parent resolution operation identity", recovered)
	}
}

func wp25NativeAuthenticatedLimit(t *testing.T) {
	for _, family := range []string{"project", "workflow"} {
		t.Run(family, func(t *testing.T) {
			f := newNativeHostFixture(t, "native:"+family+"=read")
			event, key := wp25NativePublicEvent("authenticated-"+family, family+".open")
			event.Intent = "open"
			cap := 262144
			state := map[string]any{"schema": "hn.project/1", "title": "Aggregate", "body": strings.Repeat("x", 65536), "archived": false, "fields": []any{}, "views": []any{}, "metadata": map[string]any{}}
			if family == "workflow" {
				cap = 2097152
				state = map[string]any{"schema": "hn.workflow/1", "name": "aggregate", "title": "Aggregate", "description": "", "enabled": false, "source_documents": []any{map[string]any{"format": "yaml", "content": "a", "digest": wp25NativeDigest([]byte("a"))}, map[string]any{"format": "yaml", "content": "bb", "digest": wp25NativeDigest([]byte("bb"))}}, "metadata": map[string]any{}}
			}
			// Measure the independently encoded complete signed payload, not state
			// bytes. Plain ASCII padding changes its wire size exactly one byte/byte.
			if family == "project" {
				metadata := state["metadata"].(map[string]any)
				for i := 0; i < 64; i++ {
					metadata[fmt.Sprintf("wp25/k%02d", i)] = ""
				}
			}
			rawState, _ := json.Marshal(state)
			event.Body = string(rawState)
			base, _ := json.Marshal(event)
			padding := cap - len(base)
			if padding < 1 {
				t.Fatal("authenticated fixture has no padding space")
			}
			if family == "project" {
				metadata := state["metadata"].(map[string]any)
				for i := 0; i < 64 && padding > 0; i++ {
					n := 4096
					if n > padding {
						n = padding
					}
					metadata[fmt.Sprintf("wp25/k%02d", i)] = strings.Repeat("x", n)
					padding -= n
				}
				if padding != 0 {
					t.Fatal("ordinary aggregate cannot fit legal per-field padding")
				}
			} else {
				docs := state["source_documents"].([]any)
				first := 1048576
				second := padding + 3 - first
				if second < 1 || second > 1048576 {
					t.Fatal("workflow aggregate fixture violates independent source cap")
				}
				for i, n := range []int{first, second} {
					content := strings.Repeat(string(rune('a'+i)), n)
					docs[i].(map[string]any)["content"] = content
					docs[i].(map[string]any)["digest"] = wp25NativeDigest([]byte(content))
				}
			}
			rawState, _ = json.Marshal(state)
			event.Body = string(rawState)
			exact, _ := json.Marshal(event)
			if len(exact) != cap {
				t.Fatalf("independent authenticated payload has%d bytes, want%d", len(exact), cap)
			}
			id := wp25NativePublicFixture(t, f, []Event{event}, []ed25519.PrivateKey{key})[0]
			before := hostNativeSnapshot(t, f.repo)
			page := nativeOK(t, wp25NativeRequests(t, f, nativeReadRequest(family, id))[0])
			items := page["items"].([]any)
			if len(items) != 1 || items[0].(map[string]any)["id"] != id {
				t.Fatal("exact authenticated payload was not accepted", page)
			}
			nativeSnapshotEqual(t, f, before)
			// Add exactly one legal semantic byte while keeping every constituent
			// field under its own ceiling. Re-sign the independently measured payload.
			if family == "project" {
				metadata := state["metadata"].(map[string]any)
				added := false
				for i := 0; i < 64; i++ {
					key := fmt.Sprintf("wp25/k%02d", i)
					s := metadata[key].(string)
					if len(s) < 4096 {
						metadata[key] = s + "x"
						added = true
						break
					}
				}
				if !added {
					t.Fatal("no legal ordinary first-over padding location")
				}
			} else {
				doc := state["source_documents"].([]any)[1].(map[string]any)
				content := doc["content"].(string) + "b"
				if len(content) > 1048576 {
					t.Fatal("first-over workflow accidentally exceeds per-source bound")
				}
				doc["content"], doc["digest"] = content, wp25NativeDigest([]byte(content))
			}
			rawState, _ = json.Marshal(state)
			event.Body = string(rawState)
			over, _ := json.Marshal(event)
			if len(over) != cap+1 {
				t.Fatalf("first-over authenticated payload has%d bytes,want%d", len(over), cap+1)
			}
			// Replace only this independent fixture actor's selected root; the
			// second controlled history deliberately tests an oversized signed input.
			f.git("update-ref", "-d", "refs/hn/actors/"+event.Actor)
			overID := wp25NativePublicFixture(t, f, []Event{event}, []ed25519.PrivateKey{key})[0]
			before = hostNativeSnapshot(t, f.repo)
			response := wp25NativeRequests(t, f, nativeReadRequest(family, overID))[0]
			nativeDenial(t, response, "native", "invalid_history", "do_not_retry")
			problem := response["error"].(map[string]any)
			if len(problem) != 5 || problem["operation"] != nil {
				t.Fatal("authenticated first-over refusal shape", response)
			}
			nativeSnapshotEqual(t, f, before)
			t.Logf("N01 actual supplied host authenticated READ boundary:%d accepted/%d rejected; signed public fixture production source-assisted, no native write-at-cap claim", cap, cap+1)
		})
	}
}

// New-control recording preserves exact child frames and both protected-state
// digests. Source construction labels above cannot become package execution.
func wp25NativeSnapshotDigest(snapshot map[string]string) string {
	raw, _ := json.Marshal(snapshot)
	return wp25NativeDigest(raw)
}
func wp25NativeRequests(t *testing.T, f *nativeHostFixture, requests ...map[string]any) []map[string]any {
	t.Helper()
	before := hostNativeSnapshot(t, f.repo)
	defer func() {
		input, inputErr := os.ReadFile(f.input)
		output, outputErr := os.ReadFile(f.output)
		after := hostNativeSnapshot(t, f.repo)
		wp25Record(t, map[string]any{"control": "native-live-boundary", "binary": f.binary, "request_raw": string(input), "request_sha256": wp25NativeDigest(input), "response_raw": string(output), "response_sha256": wp25NativeDigest(output), "request_read_error": fmt.Sprint(inputErr), "response_read_error": fmt.Sprint(outputErr), "protected_before_sha256": wp25NativeSnapshotDigest(before), "protected_after_sha256": wp25NativeSnapshotDigest(after)})
	}()
	return f.requests(requests...)
}

func TestExtensionPackageNativeLimits(t *testing.T) {
	binary := extensionTestBinary(t)
	t.Setenv("HN_WP17_BINARY", binary)
	wp25NativeLimits(t)
}

func wp25ExchangeObjectLimit(t *testing.T) {
	src := wp25ArtifactCLI(t)
	typ := revisionEvent(wp25Publish(src, artifactStoreType, "object-table-type", ""))
	path := filepath.Join(t.TempDir(), "bundle.json")
	wp25ArtifactCall(src, "", "", "artifact", "export", "--id", typ, "--output", path, "--json")
	var bundle map[string]any
	if e := json.Unmarshal(wp25Read(t, path), &bundle); e != nil {
		t.Fatal(e)
	}
	objects := bundle["objects"].([]any)
	for i := len(objects); i < 4097; i++ {
		body := []byte(fmt.Sprintf("WP25 unselected public object %04d", i))
		header := []byte(fmt.Sprintf("blob %d\x00", len(body)))
		hash := sha1.Sum(append(header, body...))
		objects = append(objects, map[string]any{"oid": fmt.Sprintf("%x", hash), "type": "blob", "data": base64.RawStdEncoding.EncodeToString(body)})
	}
	for _, n := range []int{4096, 4097} {
		bundle["objects"] = objects[:n]
		raw, _ := json.Marshal(bundle)
		if e := os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
		dst := wp25ArtifactCLI(t)
		before := dst.snapshot()
		code := ""
		if n == 4097 {
			code = "resource_limit"
		}
		out := wp25ArtifactCall(dst, "", code, "artifact", "import", "--input", path, "--json")
		wp25Record(t, map[string]any{"control": "exchange-object-table", "objects": n, "wire_bytes": len(raw), "wire_sha256": eventID(raw), "expected_selected_ids": []string{typ}, "source_fixture": "source-assisted public object table; supplied executable performs hash/closure/count admission"})
		if code != "" {
			artifactErrorShape(t, out, "artifact", code, "do_not_retry", nil)
			wp25Unchanged(dst, before)
		} else {
			data := out["data"].(map[string]any)
			if !reflect.DeepEqual(data["imported_ids"], []any{typ}) || len(data["opaque_ids"].([]any)) != 0 {
				t.Fatal("object table equality altered selected closure", data)
			}
		}
	}
}

func wp25ArtifactParentLimit(t *testing.T) {
	f := wp25ArtifactCLI(t)
	seed := sha256.Sum256([]byte("WP25 independently signed public artifact branches"))
	key := ed25519.NewKeyFromSeed(seed[:])
	pub := key.Public().(ed25519.PublicKey)
	actorSum := sha256.Sum256(pub)
	actor := hex.EncodeToString(actorSum[:])
	streamID := eventID([]byte("hn-artifact-stream-v1\x00" + actor))
	ref := "refs/hn/remotes/parents/artifacts/" + actor + "/" + strings.TrimPrefix(streamID, "sha256:")
	var stream bytes.Buffer
	sequence := 0
	previous := ""
	ids := []string{}
	appendPublic := func(kind, record, operation string, parents []string, payload map[string]any) string {
		sequence++
		semantic := map[string]any{"protocol": "hn-artifact/1", "actor": actor, "kind": kind, "record": record, "operation": operation, "parents": parents, "payload": payload}
		request, _ := json.Marshal(semantic)
		envelope := map[string]any{}
		for k, v := range semantic {
			envelope[k] = v
		}
		envelope["request"] = eventID(append([]byte("hn-artifact-request-v1\x00"), request...))
		envelope["public_key"] = base64.RawStdEncoding.EncodeToString(pub)
		envelope["stream"] = streamID
		envelope["sequence"] = sequence
		envelope["previous"] = previous
		envelope["timestamp"] = "2026-10-31T00:00:00Z"
		raw, _ := json.Marshal(envelope)
		id := eventID(raw)
		sig := base64.RawStdEncoding.EncodeToString(ed25519.Sign(key, append([]byte("hn-artifact/1\x00"), raw...)))
		message := "hn artifact " + id + "\n"
		fmt.Fprintf(&stream, "commit %s\nmark :%d\nauthor Fixture <fixture@example.invalid> 1793404800 +0000\ncommitter Fixture <fixture@example.invalid> 1793404800 +0000\ndata %d\n%s", ref, sequence, len(message), message)
		if sequence > 1 {
			fmt.Fprintf(&stream, "from :%d\n", sequence-1)
		}
		stream.WriteString("deleteall\n")
		for _, file := range []struct{ name, body string }{{"artifact.json", string(raw)}, {"signature", sig}} {
			fmt.Fprintf(&stream, "M 100644 inline %s\ndata %d\n%s\n", file.name, len(file.body), file.body)
		}
		stream.WriteByte('\n')
		previous = id
		ids = append(ids, id)
		return id
	}
	var declaration map[string]any
	if e := json.Unmarshal([]byte(artifactStoreType), &declaration); e != nil {
		t.Fatal(e)
	}
	typ := appendPublic("type.publish", "", "parent-type", []string{}, declaration["payload"].(map[string]any))
	evolvedPayload := declaration["payload"].(map[string]any)
	evolvedPayload["schema_version"] = 2
	evolved := appendPublic("type.publish", "", "parent-type-evolved", []string{typ}, evolvedPayload)
	opening := appendPublic("record.open", "", "parent-open", []string{}, revisionPayload(typ, "Opening"))
	heads := []string{}
	for i := 0; i < 1025; i++ {
		heads = append(heads, appendPublic("record.revise", opening, fmt.Sprintf("parent-branch-%04d", i), []string{opening}, revisionPayload(typ, fmt.Sprintf("Branch %04d", i))))
	}
	stream.WriteString("done\n")
	wp25SavePublicFixture(t, "artifact-parent-graph.fast-import", stream.Bytes())
	f.gitInput(stream.String(), "fast-import", "--quiet", "--done")
	sort.Strings(heads)
	before := f.snapshot()
	v := wp25RevisionCall(f, "resolve", revisionInput(t, opening, heads, revisionPayload(typ, "Explicit 1025-head refusal")), "parents-over", "resource_limit")
	artifactErrorShape(t, v, "artifact", "resource_limit", "do_not_retry", nil)
	wp25Unchanged(f, before)
	// Select the independently constructed prefix with1024 genuine current heads.
	// This source-assisted selection change precedes the measured positive call.
	tip := f.git("rev-parse", ref+"^")
	f.git("update-ref", ref, tip)
	removed := ids[len(ids)-1]
	selected := []string{}
	for _, id := range heads {
		if id != removed {
			selected = append(selected, id)
		}
	}
	if len(selected) != 1024 {
		t.Fatal("parent fixture does not contain1024 distinct heads")
	}
	converted := revisionPayload(evolved, "Explicit1024-headconversion-resolution")
	converted["conversion_sources"] = selected
	overConversions := append(append([]string{}, selected...), opening)
	sort.Strings(overConversions)
	overPayload := revisionPayload(evolved, "Conversion-sourcefirstover")
	overPayload["conversion_sources"] = overConversions
	before = f.snapshot()
	overResult := wp25RevisionCall(f, "resolve", revisionInput(t, opening, selected, overPayload), "conversion-sources-over", "resource_limit")
	artifactErrorShape(t, overResult, "artifact", "resource_limit", "do_not_retry", nil)
	wp25Unchanged(f, before)
	// The explicit host mutation route has the contracted240s mutation
	// budget. The ordinary30s CLI may publish then truthfully return uncertainty
	// while verifying this large graph; that failed development run is retained.
	af := &extensionArtifactFixture{artifactCLIFixture: f, module: "example.qualification/parents"}
	af.input = filepath.Join(f.home, "parent-requests.json")
	af.output = filepath.Join(f.home, "parent-responses.json")
	f.overrides["HN_WP18_CHILD"] = "1"
	manifest := filepath.Join(f.home, "parent-manifest.json")
	if e := os.MkdirAll(f.home, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(manifest, []byte(`{"protocol":"hn.extension-manifest/1","module_id":"example.qualification/parents","module_version":"1","host_protocol":"hn.extension/1","capabilities":["artifact.mutate"]}`), 0600); e != nil {
		t.Fatal(e)
	}
	wp25ArtifactCall(f, "", "", "extension", "register", "--manifest", manifest, "--executable", wp25Self(t), "--arg", "-test.run=^TestExtensionArtifactChild$", "--arg", "--", "--arg", af.input, "--arg", af.output, "--json")
	wp25ArtifactCall(f, "", "", "extension", "enable", af.module, "--grant", "artifact:example.test/note=resolve", "--json")
	response := af.request("artifact.mutate", extensionArtifactMutationParams(af, "record.resolve", opening, selected, converted, "parents-at"), "")
	wp25Record(t, map[string]any{"control": "1024-parent-host-mutation", "request_raw": string(wp25Read(t, af.input)), "response_raw": string(wp25Read(t, af.output)), "response": response})
	data := response["data"].(map[string]any)
	if data["head_count"] != float64(1) || data["conflict"] != false || data["replayed"] != false {
		t.Fatal("1024-parent actual resolution did not produce one exact head", data)
	}
	wp25Record(t, map[string]any{"control": "artifact-parent-cardinality", "fixture_class": "independent signed public graph via Git fast-import; source-assisted construction only", "opening": opening, "type": typ, "selected_parents": selected, "over_parents": heads, "resolved_event": data["event_id"], "converted_type": evolved, "conversion_sources": selected, "over_conversion_sources": overConversions, "fixture_wire_sha256": eventID(stream.Bytes())})
}

// Existing fixture helpers construct source-assisted inputs; the lifecycle
// assertions send frames and commands to the independently supplied executable.

func wp25LifecycleLimits(t *testing.T, binary string) {
	t.Helper()
	t.Run("H08-H10-typed-and-inspect-deadlines", func(t *testing.T) { wp25LifecycleTypedLimits(t, binary) })
	t.Run("H08-real-host-typed-token-boundary", func(t *testing.T) { wp25LifecycleTypedHostTokens(t, binary) })
	t.Run("H09-real-read-and-mutation-deadlines", func(t *testing.T) { wp25LifecycleRequestDeadlines(t, binary) })
	t.Run("H09-exchange-real-stage-deadlines", func(t *testing.T) { wp25LifecycleStageDeadlines(t, binary) })
	t.Run("H14-real-revocation-and-owned-child-cleanup", func(t *testing.T) { wp25LifecycleCleanup(t, binary) })
}

func wp25LifecycleRecord(t *testing.T, name string, value any) {
	t.Helper()
	wp25Record(t, map[string]any{"control": "lifecycle/" + name, "evidence": value})
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("WP25 lifecycle %s: %s", name, raw)
	if root := os.Getenv("HN_WP25_EVIDENCE"); root != "" {
		dir := filepath.Join(root, "lifecycle")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		file, err := os.CreateTemp(dir, name+"-*.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(raw); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func wp25LifecycleEnvelope(t *testing.T, output []byte, runErr error, origin, code, recovery string, operation any) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil || len(result) != 3 {
		t.Fatalf("closed public envelope: %v; run=%v; output=%s", err, runErr, output)
	}
	if code == "" {
		if runErr != nil || result["ok"] != true || result["error"] != nil || result["data"] == nil {
			t.Fatalf("success: %v %s", runErr, output)
		}
		return result
	}
	failure, ok := result["error"].(map[string]any)
	if runErr == nil || result["ok"] != false || result["data"] != nil || !ok || len(failure) != 5 || failure["origin"] != origin || failure["code"] != code || failure["recovery"] != recovery || !reflect.DeepEqual(failure["operation"], operation) {
		t.Fatalf("want %s/%s %s operation=%v; run=%v output=%s", origin, code, recovery, operation, runErr, output)
	}
	return result
}

func wp25LifecycleCLI(t *testing.T, binary, repo string, env []string, input []byte, origin, code, recovery string, args ...string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir, cmd.Env, cmd.Stdin = repo, env, bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	started := time.Now()
	err := cmd.Run()
	wp25LifecycleRecord(t, "cli", map[string]any{"args": args, "stdout": stdout.String(), "stderr": stderr.String(), "elapsed_ns": time.Since(started).Nanoseconds(), "exit_error": fmt.Sprint(err), "watchdog": fmt.Sprint(ctx.Err())})
	if ctx.Err() != nil {
		t.Fatalf("CLI watchdog, %v: %s", args, &stderr)
	}
	return wp25LifecycleEnvelope(t, stdout.Bytes(), err, origin, code, recovery, nil)
}

func wp25LifecycleTypedLimits(t *testing.T, binary string) {
	home, repo := t.TempDir(), t.TempDir()
	config := extensionTestConfigDir(t, home)
	env := operationalTestEnvironmentWith(home, map[string]string{"XDG_CONFIG_HOME": config})
	cmd := exec.Command("git", "init", "-q", repo)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Git fixture: %v %s", err, out)
	}
	before := hostNativeSnapshot(t, repo)
	inspect := func(value any, kind, code string, extra ...string) map[string]any {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		args := append([]string{"extension", "frame", "inspect", "--input", "-", "--kind", kind, "--json"}, extra...)
		origin, recovery := "", ""
		if code != "" {
			origin, recovery = "host", "do_not_retry"
		}
		return wp25LifecycleCLI(t, binary, repo, env, protocolRawFrame(raw), origin, code, recovery, args...)
	}
	for _, count := range []int{128, 129} {
		code := ""
		if count == 129 {
			code = "invalid_input"
		}
		request := map[string]any{"protocol": "hn.extension/1", "request_id": strings.Repeat("r", count), "method": "capabilities.list", "params": map[string]any{}}
		result := inspect(request, "request", code)
		if code == "" && result["data"].(map[string]any)["request_id"] != strings.Repeat("r", count) {
			t.Fatal("request ID silently changed", result)
		}
	}
	for _, count := range []int{4096, 4097} {
		code := ""
		if count == 4097 {
			code = "resource_limit"
		}
		request := map[string]any{"protocol": "hn.extension/1", "request_id": "token", "method": "resource.read", "params": map[string]any{"family": "issue", "id": nil, "limit": 1, "cursor": strings.Repeat("c", count)}}
		result := inspect(request, "request", code)
		if code == "" && result["data"].(map[string]any)["dispatch"] != "not_performed" {
			t.Fatal("typed token check unexpectedly dispatched", result)
		}
		// This accepts 4096 typed bytes, not an issued or authentic cursor.
	}
	for _, timeout := range []int{1, 240000, 240001} {
		code := ""
		if timeout == 240001 {
			code = "invalid_input"
		}
		inspect(map[string]any{"protocol": "hn.extension/1", "request_id": "timeout", "method": "capabilities.list", "params": map[string]any{}, "timeout_ms": timeout}, "request", code)
	}
	ready := map[string]any{"protocol": "hn.extension/1", "ready": true}
	for _, timeout := range []int{1, 30000, 0, 30001} {
		code := ""
		if timeout == 0 || timeout == 30001 {
			code = "invalid_input"
		}
		inspect(ready, "ready", code, "--timeout-ms", strconv.Itoa(timeout))
	}
	// Hold stdin open without a byte for the full 30000ms upper bound. The
	// transport's real wall-clock cap, rather than a short injected timer, fires.
	ctx, cancel := context.WithTimeout(context.Background(), 37*time.Second)
	defer cancel()
	blocked := exec.CommandContext(ctx, binary, "extension", "frame", "inspect", "--input", "-", "--kind", "ready", "--timeout-ms", "30000", "--json")
	blocked.Dir, blocked.Env = repo, env
	pipe, err := blocked.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	blocked.Stdout, blocked.Stderr = &stdout, &stderr
	started := time.Now()
	if err := blocked.Start(); err != nil {
		t.Fatal(err)
	}
	err = blocked.Wait()
	_ = pipe.Close()
	elapsed := time.Since(started)
	wp25LifecycleRecord(t, "inspect-full-30000ms", map[string]any{"elapsed_ns": elapsed.Nanoseconds(), "stdout": stdout.String(), "stderr": stderr.String(), "run_error": fmt.Sprint(err), "watchdog": fmt.Sprint(ctx.Err())})
	if ctx.Err() != nil || elapsed < 30*time.Second || elapsed > 35*time.Second {
		t.Fatalf("full inspect budget: %s watchdog=%v output=%s", elapsed, ctx.Err(), &stdout)
	}
	wp25LifecycleEnvelope(t, stdout.Bytes(), err, "host", "cancelled", "do_not_retry", nil)
	afterInspect := hostNativeSnapshot(t, repo)
	wp25LifecycleRecord(t, "inspect-state", map[string]any{"before_snapshot": before, "after_snapshot": afterInspect, "before_sha256": wp25LifecycleSnapshotDigest(before), "after_sha256": wp25LifecycleSnapshotDigest(afterInspect)})
	if !reflect.DeepEqual(before, afterInspect) {
		t.Fatal("typed/refused/expired inspection changed Git objects, refs or private native state")
	}
	// Actual host callers supplement the independent frame-inspect validator.
	for _, tc := range []struct {
		name                   string
		request                map[string]any
		code, origin, recovery string
	}{
		{"id128", map[string]any{"protocol": "hn.extension/1", "request_id": strings.Repeat("r", 128), "method": "capabilities.list", "params": map[string]any{}}, "", "", ""},
		{"id129", map[string]any{"protocol": "hn.extension/1", "request_id": strings.Repeat("r", 129), "method": "capabilities.list", "params": map[string]any{}}, "invalid_input", "host", "do_not_retry"},
		{"timeout240000", map[string]any{"protocol": "hn.extension/1", "request_id": "timeout-max", "method": "capabilities.list", "params": map[string]any{}, "timeout_ms": 240000}, "", "", ""},
		{"timeout240001", map[string]any{"protocol": "hn.extension/1", "request_id": "timeout-over", "method": "capabilities.list", "params": map[string]any{}, "timeout_ms": 240001}, "invalid_input", "host", "do_not_retry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := wp25LifecycleFixture(t, binary, "request", tc.request)
			before := hostNativeSnapshot(t, f.repo)
			out, runErr, elapsed := wp25LifecycleRun(t, f, 15*time.Second)
			wp25LifecycleEnvelope(t, out, runErr, tc.origin, tc.code, tc.recovery, nil)
			if tc.code == "" {
				response := wp25LifecycleReadJSON(t, f.result)
				if response["ok"] != true || response["request_id"] != tc.request["request_id"] {
					t.Fatal("actual response/request identity", response)
				}
			}
			after := hostNativeSnapshot(t, f.repo)
			wp25LifecycleRecord(t, "typed-state", map[string]any{"request": tc.request, "before_snapshot": before, "after_snapshot": after, "before_sha256": wp25LifecycleSnapshotDigest(before), "after_sha256": wp25LifecycleSnapshotDigest(after)})
			if !reflect.DeepEqual(before, after) {
				t.Fatal("host typed admission/refusal changed native state")
			}
			wp25LifecycleRecord(t, tc.name, map[string]any{"elapsed_ns": elapsed.Nanoseconds(), "stdout": string(out), "run_error": fmt.Sprint(runErr)})
		})
	}
}

// This independently executed handwritten peer records signal and frame-send
// timestamps. It imports no production host/client transport or response type.
func TestExtensionPackageLifecycleChild(t *testing.T) {
	if os.Getenv("HN_WP25_LIFECYCLE_CHILD") != "1" {
		return
	}
	at := 0
	for i, arg := range os.Args {
		if arg == "--" {
			at = i + 1
			break
		}
	}
	if at == 0 || len(os.Args) != at+3 {
		os.Exit(91)
	}
	mode, marker, result := os.Args[at], os.Args[at+1], os.Args[at+2]
	stamp := func(path string) {
		raw, _ := json.Marshal(map[string]any{"pid": os.Getpid(), "unix_ns": time.Now().UnixNano()})
		if os.WriteFile(path, raw, 0600) != nil {
			os.Exit(92)
		}
	}
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGTERM)
	go func() {
		for range signals {
			stamp(marker + ".term.json")
			if mode == "term-exit" {
				os.Exit(0)
			}
		}
	}()
	if os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0600) != nil {
		os.Exit(93)
	}
	read := func() (map[string]any, error) {
		var prefix [4]byte
		if _, err := io.ReadFull(os.Stdin, prefix[:]); err != nil {
			return nil, err
		}
		n := binary.BigEndian.Uint32(prefix[:])
		if n == 0 || n > 8388608 {
			return nil, fmt.Errorf("invalid frame")
		}
		raw := make([]byte, n)
		if _, err := io.ReadFull(os.Stdin, raw); err != nil {
			return nil, err
		}
		var value map[string]any
		err := json.Unmarshal(raw, &value)
		return value, err
	}
	write := func(value any) {
		raw, _ := json.Marshal(value)
		var prefix [4]byte
		binary.BigEndian.PutUint32(prefix[:], uint32(len(raw)))
		os.Stdout.Write(prefix[:])
		os.Stdout.Write(raw)
	}
	if _, err := read(); err != nil {
		os.Exit(94)
	}
	write(map[string]any{"protocol": "hn.extension/1", "ready": true})
	stamp(marker + ".ready.json")
	if mode == "request" {
		raw, err := os.ReadFile(result + ".request.json")
		if err != nil {
			os.Exit(95)
		}
		var request map[string]any
		if json.Unmarshal(raw, &request) != nil {
			os.Exit(96)
		}
		stamp(marker + ".sent.json")
		write(request)
		response, err := read()
		if err == nil {
			raw, _ := json.Marshal(response)
			if os.WriteFile(result, raw, 0600) != nil {
				os.Exit(97)
			}
			os.Exit(0)
		}
	}
	for {
		time.Sleep(time.Hour)
	}
}

func wp25LifecycleFixture(t *testing.T, binary, mode string, request map[string]any) *hostFixture {
	t.Helper()
	f := &hostFixture{t: t, binary: binary, home: t.TempDir(), repo: t.TempDir(), module: "example.wp25/lifecycle"}
	f.config = extensionTestConfigDir(t, f.home)
	f.marker, f.result, f.manifest = filepath.Join(f.home, "started"), filepath.Join(f.home, "response.json"), filepath.Join(f.home, "manifest.json")
	f.env = operationalTestEnvironmentWith(f.home, map[string]string{"XDG_CONFIG_HOME": f.config, "HN_WP25_LIFECYCLE_CHILD": "1"})
	git := exec.Command("git", "init", "-q", f.repo)
	git.Env = f.env
	if raw, err := git.CombinedOutput(); err != nil {
		t.Fatalf("Git fixture: %v %s", err, raw)
	}
	manifest := `{"protocol":"hn.extension-manifest/1","module_id":"example.wp25/lifecycle","module_version":"1","host_protocol":"hn.extension/1","capabilities":["capabilities.list","resource.read","resource.mutate","resource.operation"]}`
	if err := os.WriteFile(f.manifest, []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f.invoke("", "register", "--manifest", f.manifest, "--executable", self, "--arg", "-test.run=^TestExtensionPackageLifecycleChild$", "--arg", "--", "--arg", mode, "--arg", f.marker, "--arg", f.result, "--json")
	f.invoke("", "enable", f.module, "--grant", "native:issue=read,create", "--json")
	if request != nil {
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(f.result+".request.json", raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func wp25LifecycleReadJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func wp25LifecycleStamp(t *testing.T, path string) time.Time {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		UnixNS int64 `json:"unix_ns"`
	}
	if err = json.Unmarshal(raw, &value); err != nil || value.UnixNS == 0 {
		t.Fatalf("timestamp: %v %s", err, raw)
	}
	return time.Unix(0, value.UnixNS)
}
func wp25LifecycleRun(t *testing.T, f *hostFixture, budget time.Duration) ([]byte, error, time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.binary, "extension", "run", f.module, "--json")
	cmd.Dir, cmd.Env = f.repo, f.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	started := time.Now()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := waitHostMarker(t, f.marker)
	defer syscall.Kill(-pid, syscall.SIGKILL)
	err := cmd.Wait()
	elapsed := time.Since(started)
	wp25LifecycleRecord(t, "host-run", map[string]any{"pid": pid, "stdout": stdout.String(), "stderr": stderr.String(), "elapsed_ns": elapsed.Nanoseconds(), "run_error": fmt.Sprint(err), "watchdog": fmt.Sprint(ctx.Err())})
	if ctx.Err() != nil {
		t.Fatalf("host exceeded independent watchdog %s: %s", budget, &stdout)
	}
	assertHostExited(t, pid)
	return stdout.Bytes(), err, elapsed
}

func wp25LifecycleRequestDeadlines(t *testing.T, binary string) {
	// The control wrapper delays one real Git read, then delegates to actual Git.
	// It cannot manufacture a successful native read/publication or shorten any
	// production clock. All fixture/signal files live outside protected state.
	for _, tc := range []struct {
		kind          string
		budget, stall time.Duration
	}{{"read", 30 * time.Second, 31 * time.Second}, {"mutation", 240 * time.Second, 245 * time.Second}} {
		t.Run(tc.kind, func(t *testing.T) {
			f := wp25LifecycleFixture(t, binary, "request", nil)
			init := exec.Command(binary, "init", "--name", "WP25 lifecycle author")
			init.Dir, init.Env = f.repo, f.env
			if out, err := init.CombinedOutput(); err != nil {
				t.Fatalf("native init: %v %s", err, out)
			}
			identity := wp25LifecycleCLI(t, binary, f.repo, f.env, nil, "", "", "", "identity", "show", "--json")
			if identity["schema"] != "hn.identity/1" {
				t.Fatal("native identity schema", identity)
			}
			actor := identity["data"].(map[string]any)["actor"].(string)
			request := map[string]any{"protocol": "hn.extension/1", "request_id": "full-real-" + tc.kind, "method": "resource.read", "params": map[string]any{"family": "issue", "id": nil, "limit": 1, "cursor": nil}}
			key := "wp25-full-real-mutation-budget"
			if tc.kind == "mutation" {
				request = map[string]any{"protocol": "hn.extension/1", "request_id": "full-real-mutation", "method": "resource.mutate", "params": map[string]any{"family": "issue", "intent": "open", "id": nil, "expected": []any{}, "actor": actor, "operation": key, "state": nativeIssueState("WP25 full 240s deadline must not publish"), "comment": ""}, "timeout_ms": 240000}
			}
			raw, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(f.result+".request.json", raw, 0600); err != nil {
				t.Fatal(err)
			}
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			wrap, control := t.TempDir(), t.TempDir()
			reached := filepath.Join(control, "actual-git-stage-reached")
			script := "#!/bin/sh\nfor arg do\n if [ \"$arg\" = for-each-ref ] && [ -f \"$HN_WP25_REQUEST_SENT\" ]; then\n  if mkdir \"$HN_WP25_STALL_ONCE\" 2>/dev/null; then\n   : > \"$HN_WP25_STALL_REACHED\"\n   sleep \"$HN_WP25_STALL_SECONDS\"\n  fi\n fi\ndone\nexec '" + strings.ReplaceAll(git, "'", "'\\''") + "' \"$@\"\n"
			if err = os.WriteFile(filepath.Join(wrap, "git"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			f.env = append(f.env, "PATH="+wrap+string(os.PathListSeparator)+os.Getenv("PATH"), "HN_WP25_REQUEST_SENT="+f.marker+".sent.json", "HN_WP25_STALL_ONCE="+filepath.Join(control, "once"), "HN_WP25_STALL_REACHED="+reached, "HN_WP25_STALL_SECONDS="+strconv.Itoa(int(tc.stall/time.Second)))
			before := hostNativeSnapshot(t, f.repo)
			out, runErr, elapsed := wp25LifecycleRun(t, f, tc.stall+15*time.Second)
			if _, err := os.Stat(reached); err != nil {
				t.Fatal("asserted real native Git stage was not reached", err)
			}
			sent, term := wp25LifecycleStamp(t, f.marker+".sent.json"), wp25LifecycleStamp(t, f.marker+".term.json")
			observed := term.Sub(sent)
			wp25LifecycleRecord(t, tc.kind+"-full-budget", map[string]any{"request": request, "budget_ns": tc.budget.Nanoseconds(), "source_assisted_git_delay_ns": tc.stall.Nanoseconds(), "peer_first_write_ns": sent.UnixNano(), "child_term_ns": term.UnixNano(), "observed_notice_ns": observed.Nanoseconds(), "host_settled_ns": elapsed.Nanoseconds(), "stdout": string(out), "error": fmt.Sprint(runErr), "git_wrapper": script, "boundary": "request deadline/child TERM observed independently from synchronous real Git settlement"})
			if observed < tc.budget || observed > tc.budget+2*time.Second {
				t.Fatalf("real %s request budget not observed at its full duration: %s", tc.kind, observed)
			}
			if tc.kind == "read" {
				wp25LifecycleEnvelope(t, out, runErr, "host", "cancelled", "do_not_retry", nil)
			} else {
				wp25LifecycleEnvelope(t, out, runErr, "host", "uncertain_operation", "inspect_operation", map[string]any{"protocol": "hn/0", "actor": actor, "operation": key})
			}
			after := hostNativeSnapshot(t, f.repo)
			wp25LifecycleRecord(t, "request-deadline-state", map[string]any{"request": request, "before_snapshot": before, "after_snapshot": after, "before_sha256": wp25LifecycleSnapshotDigest(before), "after_sha256": wp25LifecycleSnapshotDigest(after)})
			if !reflect.DeepEqual(before, after) {
				t.Fatal("expired pre-publication request changed signing state, intents, objects or accepted refs")
			}
			// An ordinary positive operation after the deliberate stalled command
			// demonstrates that a failed host did not poison the native repository.
			f.env = operationalTestEnvironmentWith(f.home, map[string]string{"XDG_CONFIG_HOME": f.config, "HN_WP25_LIFECYCLE_CHILD": "1"})
			if tc.kind == "mutation" {
				lookup := map[string]any{"protocol": "hn.extension/1", "request_id": "deadline-operation-inspect", "method": "resource.operation", "params": map[string]any{"family": "issue", "actor": actor, "operation": key}}
				raw, _ := json.Marshal(lookup)
				if err := os.WriteFile(f.result+".request.json", raw, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(f.marker); err != nil {
					t.Fatal(err)
				}
				lookupOut, lookupErr, _ := wp25LifecycleRun(t, f, 15*time.Second)
				wp25LifecycleEnvelope(t, lookupOut, lookupErr, "", "", "", nil)
				response := wp25LifecycleReadJSON(t, f.result)
				if response["ok"] != true {
					t.Fatal("actual operation lookup refused", response)
				}
				data := response["data"].(map[string]any)
				if data["status"] != "verified_absent" || data["actor"] != actor || data["operation"] != key || data["event_id"] != nil || data["resource_id"] != nil || data["result"] != nil {
					t.Fatal("actual pre-publication deadline lookup", data)
				}
				wp25LifecycleRecord(t, "full-mutation-operation-inspect", map[string]any{"request": lookup, "response": response, "stdout": string(lookupOut), "before_sha256": wp25LifecycleSnapshotDigest(after), "after_sha256": wp25LifecycleSnapshotDigest(hostNativeSnapshot(t, f.repo))})
				if !reflect.DeepEqual(after, hostNativeSnapshot(t, f.repo)) {
					t.Fatal("operation inspection changed native/private state")
				}
			}
			positive := exec.Command(binary, "issue", "open", "--body", "positive control after deadline", "WP25 positive after deadline")
			positive.Dir, positive.Env = f.repo, f.env
			if raw, err := positive.CombinedOutput(); err != nil {
				t.Fatalf("actual post-deadline native positive: %v %s", err, raw)
			}
			if reflect.DeepEqual(before, hostNativeSnapshot(t, f.repo)) {
				t.Fatal("actual accepted mutation did not trip native-state detector")
			}
		})
	}
}

func wp25LifecycleStageDeadlines(t *testing.T, binary string) {
	// Public file exchange has30s input/validation/copy stage ceilings.
	// The export-read case below is a read within export validation, not a
	// separate additional read-stage clock; blocked import stdin is distinct.
	// Exact clock equality is a scheduling frontier, not guaranteed successful
	// admission. Valid ordinary calls are positive controls; stalled commands
	// remain open beyond30s and must be cancelled by the real frozen stage.
	t.Setenv("HN_WP05_BINARY", binary)
	source := artifactCLI(t)
	typ := revisionEvent(source.publish(artifactStoreType, "wp25-stage-type", ""))
	path, _ := exchangeExport(source, typ)
	t.Run("input-full30s", func(t *testing.T) { wp25LifecycleExchangeInputDeadline(t, binary, source, path) })
	for _, stage := range []string{"read", "validation", "copy"} {
		t.Run(stage, func(t *testing.T) {
			dst := exchangeDestination(t, source, "sha1")
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			wrap, control := t.TempDir(), t.TempDir()
			reached := filepath.Join(control, "real-stage-reached")
			// Stage read is the source loader in public export. Validation's
			// hash-object is confined to its disposable quarantine. Copy's
			// hash-object selects the destination's exact Git directory.
			script := "#!/bin/sh\nmatch=0\ncommand=\nwhere=\nfor arg do\n case \"$arg\" in --git-dir=*) where=${arg#--git-dir=} ;; for-each-ref|hash-object) command=$arg ;; esac\ndone\ncase \"$HN_WP25_STAGE\" in\n read) [ \"$command\" = for-each-ref ] && match=1 ;;\n validation) case \"$where\" in *hn-artifact-quarantine-*) [ \"$command\" = hash-object ] && match=1 ;; esac ;;\n copy) [ \"$where\" = \"$HN_WP25_DEST_GIT\" ] && [ \"$command\" = hash-object ] && match=1 ;;\nesac\nif [ \"$match\" = 1 ]; then\n : > \"$HN_WP25_STAGE_REACHED\"\n exec sleep 35\nfi\nexec '" + strings.ReplaceAll(git, "'", "'\\''") + "' \"$@\"\n"
			if err = os.WriteFile(filepath.Join(wrap, "git"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			fixture := dst
			args := []string{"artifact", "import", "--input", path, "--json"}
			if stage == "read" {
				fixture = source
				args = []string{"artifact", "export", "--id", typ, "--output", filepath.Join(control, "public-output.json"), "--json"}
			}
			before := hostNativeSnapshot(t, fixture.repo)
			env := operationalTestEnvironmentWith(fixture.home, map[string]string{"PATH": wrap + string(os.PathListSeparator) + os.Getenv("PATH"), "HN_WP25_STAGE": stage, "HN_WP25_STAGE_REACHED": reached, "HN_WP25_DEST_GIT": dst.git("rev-parse", "--absolute-git-dir")})
			ctx, cancel := context.WithTimeout(context.Background(), 38*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, args...)
			cmd.Dir, cmd.Env = fixture.repo, env
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			started := time.Now()
			runErr := cmd.Run()
			elapsed := time.Since(started)
			wp25LifecycleRecord(t, "exchange-"+stage, map[string]any{"args": args, "elapsed_ns": elapsed.Nanoseconds(), "stdout": stdout.String(), "stderr": stderr.String(), "run_error": fmt.Sprint(runErr), "watchdog": fmt.Sprint(ctx.Err()), "wrapper": script})
			if ctx.Err() != nil || elapsed < 30*time.Second || elapsed > 35*time.Second {
				t.Fatalf("actual30s %s stage: %s watchdog=%v %s", stage, elapsed, ctx.Err(), &stdout)
			}
			if _, err := os.Stat(reached); err != nil {
				t.Fatal("asserted real stage never reached", stage, err)
			}
			wp25LifecycleEnvelope(t, stdout.Bytes(), runErr, "artifact", "cancelled", "do_not_retry", nil)
			after := hostNativeSnapshot(t, fixture.repo)
			wp25LifecycleRecord(t, "stage-deadline-state", map[string]any{"stage": stage, "before_snapshot": before, "after_snapshot": after, "before_sha256": wp25LifecycleSnapshotDigest(before), "after_sha256": wp25LifecycleSnapshotDigest(after)})
			if !reflect.DeepEqual(before, after) {
				t.Fatal("expired pre-copy stage changed destination/source signer, objects, journals or refs")
			}
			if stage == "read" {
				if _, err := os.Stat(filepath.Join(control, "public-output.json")); !os.IsNotExist(err) {
					t.Fatal("cancelled export installed output", err)
				}
			} else {
				// The same complete signed transport succeeds without the stalled
				// Git control, proving both its actual stage reachability and the
				// source of the prior cancellation.
				dst.call("", "", "artifact", "import", "--input", path, "--json")
				shown := dst.call("", "", "artifact", "type", "show", typ, "--json")
				if len(shown["data"].(map[string]any)["items"].([]any)) != 1 {
					t.Fatal("positive complete exchange did not reconstruct signed type", shown)
				}
				if reflect.DeepEqual(before, hostNativeSnapshot(t, dst.repo)) {
					t.Fatal("successful import did not trip the exact destination effect detector")
				}
			}
		})
	}
}

func wp25LifecycleCleanup(t *testing.T, binary string) {
	for _, mode := range []string{"term-exit", "ignore-term"} {
		t.Run(mode, func(t *testing.T) {
			f := wp25LifecycleFixture(t, binary, mode, nil)
			before := hostNativeSnapshot(t, f.repo)
			delete(before, "/.git/hn/extensions/enabled.json")
			registry := filepath.Join(f.config, "hubnot", "extensions", "registry.json")
			registryBefore, err := os.ReadFile(registry)
			if err != nil || len(registryBefore) == 0 {
				t.Fatalf("real registered config: %v", err)
			}
			enabled := filepath.Join(f.repo, ".git", "hn", "extensions", "enabled.json")
			enabledBefore, err := os.ReadFile(enabled)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			host := exec.CommandContext(ctx, binary, "extension", "run", f.module, "--json")
			host.Dir, host.Env = f.repo, f.env
			var hostOut, hostErr bytes.Buffer
			host.Stdout, host.Stderr = &hostOut, &hostErr
			if err := host.Start(); err != nil {
				t.Fatal(err)
			}
			pid := waitHostMarker(t, f.marker)
			defer syscall.Kill(-pid, syscall.SIGKILL)
			done := make(chan error, 1)
			go func() { done <- host.Wait() }()
			deadline := time.Now().Add(5 * time.Second)
			for {
				if _, err := os.Stat(f.marker + ".ready.json"); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("registered signal witness did not handshake")
				}
				time.Sleep(time.Millisecond)
			}
			// Observe the bytes actually replaced by the public disable caller.
			// This is a measured bound on notice after visible epoch replacement;
			// timer scheduling and measurement errors remain raw evidence on failure.
			revoked := make(chan time.Time, 1)
			watch, watchCancel := context.WithCancel(context.Background())
			defer watchCancel()
			go func() {
				for {
					raw, err := os.ReadFile(enabled)
					if err == nil && !bytes.Equal(raw, enabledBefore) {
						revoked <- time.Now()
						return
					}
					select {
					case <-watch.Done():
						return
					case <-time.After(time.Millisecond):
					}
				}
			}()
			type observed struct {
				term, dead   time.Time
				survivedTerm bool
			}
			observations := make(chan observed, 1)
			go func() {
				var result observed
				for time.Now().Before(deadline.Add(8 * time.Second)) {
					if result.term.IsZero() {
						raw, err := os.ReadFile(f.marker + ".term.json")
						if err == nil {
							var value struct {
								UnixNS int64 `json:"unix_ns"`
							}
							if json.Unmarshal(raw, &value) == nil && value.UnixNS > 0 {
								result.term = time.Unix(0, value.UnixNS)
							}
						}
					}
					if !result.term.IsZero() && time.Since(result.term) >= 250*time.Millisecond && syscall.Kill(pid, 0) == nil {
						result.survivedTerm = true
					}
					if syscall.Kill(pid, 0) == syscall.ESRCH {
						result.dead = time.Now()
						observations <- result
						return
					}
					time.Sleep(2 * time.Millisecond)
				}
				observations <- result
			}()
			disable := f.command("disable", f.module, "--json")
			var disableOut, disableErr bytes.Buffer
			disable.Stdout, disable.Stderr = &disableOut, &disableErr
			started := time.Now()
			runErr := disable.Run()
			finished := time.Now()
			result := wp25LifecycleEnvelope(t, disableOut.Bytes(), runErr, "", "", "", nil)["data"].(map[string]any)
			if result["cleanup"] != "verified" || len(result["stopped_sessions"].([]any)) != 1 {
				t.Fatal("disable failed to join the real owned session", result)
			}
			var epoch time.Time
			select {
			case epoch = <-revoked:
			case <-time.After(time.Second):
				t.Fatal("real disable did not replace observed enablement bytes")
			}
			measured := <-observations
			select {
			case err = <-done:
			case <-time.After(time.Second):
				t.Fatal("revoked host was not joined after cleanup")
			}
			if ctx.Err() != nil {
				t.Fatal("cleanup exceeded independent watchdog", ctx.Err())
			}
			wp25LifecycleEnvelope(t, hostOut.Bytes(), err, "host", "denied_access", "do_not_retry", nil)
			wp25LifecycleRecord(t, "cleanup-"+mode, map[string]any{"child_pid": pid, "disable_started_ns": started.UnixNano(), "epoch_replacement_observed_ns": epoch.UnixNano(), "term_ns": measured.term.UnixNano(), "owned_child_exit_observed_ns": measured.dead.UnixNano(), "disable_finished_ns": finished.UnixNano(), "notice_ns": measured.term.Sub(epoch).Nanoseconds(), "term_to_exit_ns": measured.dead.Sub(measured.term).Nanoseconds(), "total_cleanup_ns": finished.Sub(started).Nanoseconds(), "survived_term_250ms": measured.survivedTerm, "disable_stdout": disableOut.String(), "disable_stderr": disableErr.String(), "host_stdout": hostOut.String(), "host_stderr": hostErr.String(), "measurement_poll_ns": int64(2 * time.Millisecond), "lower_grace_measurement_tolerance_ns": int64(20 * time.Millisecond)})
			if measured.term.IsZero() || measured.dead.IsZero() {
				t.Fatal("actual process signal/exit detector missed its positive control")
			}
			notice := measured.term.Sub(epoch)
			if notice < -2*time.Millisecond || notice > 250*time.Millisecond {
				t.Fatalf("real revocation notice beyond250ms: %s (negative<=2ms is independent observer order)", notice)
			}
			if finished.Sub(started) > 5*time.Second {
				t.Fatalf("cleanup did not verify owned exit inside5s: duration=%s exit=%s finish=%s", finished.Sub(started), measured.dead, finished)
			}
			if mode == "ignore-term" {
				if !measured.survivedTerm {
					t.Fatal("TERM-ignore positive control did not stay alive after the actual signal")
				}
				// TERM delivery and disappearance are external observations. Keep
				// their20ms measurement tolerance explicit; production still runs
				// its full unmodified two-second timer before KILL.
				if measured.dead.Sub(measured.term) < 2*time.Second-20*time.Millisecond {
					t.Fatalf("owned group killed before real2s grace: %s", measured.dead.Sub(measured.term))
				}
			} else if measured.survivedTerm {
				t.Fatal("TERM-exit positive control did not exit promptly")
			}
			assertHostExited(t, pid)
			after := hostNativeSnapshot(t, f.repo)
			delete(after, "/.git/hn/extensions/enabled.json")
			wp25LifecycleRecord(t, "cleanup-state", map[string]any{"mode": mode, "before_snapshot": before, "after_snapshot": after, "before_sha256": wp25LifecycleSnapshotDigest(before), "after_sha256": wp25LifecycleSnapshotDigest(after)})
			if !reflect.DeepEqual(before, after) {
				t.Fatal("revocation cleanup changed signer, native objects, journals or accepted refs")
			}
			registryAfter, err := os.ReadFile(registry)
			if err != nil || !bytes.Equal(registryBefore, registryAfter) {
				t.Fatal("disable changed registered executable identity", err)
			}
			if err = os.Remove(f.marker); err != nil {
				t.Fatal(err)
			}
			f.invoke("denied_access", "run", f.module, "--json")
			if _, err := os.Stat(f.marker); !os.IsNotExist(err) {
				t.Fatal("revoked host relaunched child", err)
			}
		})
	}
}

func wp25LifecycleSnapshotDigest(value map[string]string) string {
	raw, _ := json.Marshal(value)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func wp25LifecycleTypedHostTokens(t *testing.T, binary string) {
	for _, size := range []int{4096, 4097} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			request := map[string]any{"protocol": "hn.extension/1", "request_id": "typed-token", "method": "resource.read", "params": map[string]any{"family": "issue", "id": nil, "limit": 1, "cursor": strings.Repeat("c", size)}}
			f := wp25LifecycleFixture(t, binary, "request", request)
			before := hostNativeSnapshot(t, f.repo)
			out, runErr, elapsed := wp25LifecycleRun(t, f, 15*time.Second)
			var response any
			if size == 4097 {
				wp25LifecycleEnvelope(t, out, runErr, "host", "resource_limit", "do_not_retry", nil)
			} else {
				wp25LifecycleEnvelope(t, out, runErr, "", "", "", nil)
				observed := wp25LifecycleReadJSON(t, f.result)
				failure, ok := observed["error"].(map[string]any)
				if len(observed) != 5 || observed["protocol"] != "hn.extension/1" || observed["request_id"] != "typed-token" || observed["ok"] != false || observed["data"] != nil || !ok || len(failure) != 5 || failure["origin"] != "native" || failure["code"] != "invalid_cursor" || failure["recovery"] != "rescan_required" || failure["operation"] != nil {
					t.Fatal("4096-byte typed token did not reach authentic-cursor validation", observed)
				}
				response = observed
			}
			after := hostNativeSnapshot(t, f.repo)
			wp25LifecycleRecord(t, "typed-token-host", map[string]any{"request": request, "response": response, "stdout": string(out), "exit_error": fmt.Sprint(runErr), "elapsed_ns": elapsed.Nanoseconds(), "before_snapshot": before, "after_snapshot": after, "before_sha256": wp25LifecycleSnapshotDigest(before), "after_sha256": wp25LifecycleSnapshotDigest(after), "scope": "typed byte admission; 4096 bytes are deliberately unissued and receive native invalid_cursor, never authentic-token success"})
			if !reflect.DeepEqual(before, after) {
				t.Fatal("typed cursor refusal changed native objects, refs or private state")
			}
		})
	}
}

func wp25LifecycleExchangeInputDeadline(t *testing.T, binary string, source *artifactCLIFixture, publicPath string) {
	t.Helper()
	dst := exchangeDestination(t, source, "sha1")
	before := hostNativeSnapshot(t, dst.repo)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "artifact", "import", "--input", "-", "--json")
	cmd.Dir, cmd.Env = dst.repo, operationalTestEnvironment(dst.home)
	pipe, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	started := time.Now()
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// A blocked synchronous stdin syscall may outlast cancellation. Release
	// only its input at31s, then require the expired context to refuse before
	// validation/copy/promotion. No hard kernel-interruption SLA is asserted.
	released := make(chan time.Time, 1)
	settle := time.AfterFunc(31*time.Second, func() { _ = pipe.Close(); released <- time.Now() })
	runErr := cmd.Wait()
	releaseTime := time.Time{}
	if settle.Stop() {
		_ = pipe.Close()
	} else {
		releaseTime = <-released
	}
	elapsed := time.Since(started)
	_ = pipe.Close()
	after := hostNativeSnapshot(t, dst.repo)
	wp25LifecycleRecord(t, "exchange-input-full30s", map[string]any{"args": cmd.Args, "elapsed_ns": elapsed.Nanoseconds(), "stdout": stdout.String(), "stderr": stderr.String(), "exit_error": fmt.Sprint(runErr), "watchdog": fmt.Sprint(ctx.Err()), "before_snapshot": before, "after_snapshot": after, "before_sha256": wp25LifecycleSnapshotDigest(before), "after_sha256": wp25LifecycleSnapshotDigest(after), "input_control": "empty stdin pipe held through30s context budget, independent EOF release at31s if synchronous read remains blocked", "independent_eof_release": releaseTime, "boundary": "expired context refusal after synchronous read settles; no hard kernel-I/O interruption claim"})
	if ctx.Err() != nil || elapsed < 30*time.Second || elapsed >= 90*time.Second {
		t.Fatalf("full30s public exchange input stage: %s watchdog=%v %s", elapsed, ctx.Err(), &stdout)
	}
	wp25LifecycleEnvelope(t, stdout.Bytes(), runErr, "artifact", "cancelled", "do_not_retry", nil)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("timed-out exchange input created signer, object, intent, journal or ref state")
	}
	public, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	wp25ArtifactCall(dst, string(public), "", "artifact", "import", "--input", "-", "--json")
	if reflect.DeepEqual(before, hostNativeSnapshot(t, dst.repo)) {
		t.Fatal("valid supplied stdin exchange did not trip actual destination effect detector")
	}
}

func TestExtensionPackageLifecycleLimits(t *testing.T) {
	binary := extensionTestBinary(t)
	for _, key := range []string{"HN_WP13_BINARY", "HN_WP15_BINARY", "HN_WP16_BINARY", "HN_WP17_BINARY", "HN_WP05_BINARY"} {
		t.Setenv(key, binary)
	}
	wp25LifecycleLimits(t, binary)
}

// All observations below are against the independently supplied standalone
// fake-tracker process. No sample package is imported into this harness.
func wp25TrackerServiceLimits(t *testing.T, consumer string) {
	if consumer == "" {
		t.Fatal("tracker service limits require supplied consumer")
	}
	marshal := func(value any) []byte {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	initial := func() map[string]any {
		return map[string]any{"item": map[string]any{"id": "42", "version": 1, "title": "Original", "body": "Original body", "author": "claimed author", "time": "2026-10-31T00:00:00Z", "comments": []any{}}, "receipts": map[string]any{}, "writes": 0}
	}
	credential := marshal(map[string]any{"token": "wp25-service-token"})
	pad := func(raw []byte, n int) []byte {
		if len(raw) > n {
			t.Fatal("tracker JSON padding underflow")
		}
		out := append([]byte{}, raw...)
		return append(out, bytes.Repeat([]byte(" "), n-len(raw))...)
	}
	for _, which := range []string{"credential", "service-state"} {
		t.Run("E04-loader-"+which, func(t *testing.T) {
			cap := 131072
			if which == "service-state" {
				cap = 16777216
			}
			for _, over := range []bool{false, true} {
				n := cap
				if over {
					n++
				}
				c, s := credential, marshal(initial())
				if which == "credential" {
					c = pad(c, n)
				} else {
					s = pad(s, n)
				}
				process := wp25TrackerStartService(t, consumer, c, s, over)
				if !over {
					status, raw := process.http(t, "GET", "/item", nil)
					var item map[string]any
					if status != 200 || json.Unmarshal(raw, &item) != nil || item["id"] != "42" || item["version"] != float64(1) {
						t.Fatalf("at-bound standalone service GET:%d %s", status, raw)
					}
					process.unchangedState(t, s)
					process.stop(t)
				}
			}
		})
	}
	t.Run("E04-HTTP-request-body", func(t *testing.T) {
		state := marshal(initial())
		process := wp25TrackerStartService(t, consumer, credential, state, false)
		defer process.stop(t)
		request := map[string]any{"key": "http-at", "expected_version": 1, "source": "source", "resource": "resource", "revisions": []any{}, "title": "HTTP at", "body": "updated", "lose_ack": false}
		at := pad(marshal(request), 8388608)
		status, raw := process.http(t, "POST", "/writes", at)
		var receipt map[string]any
		if status != 200 || json.Unmarshal(raw, &receipt) != nil || receipt["version"] != float64(2) {
			t.Fatalf("8MiB request equality:%d %s", status, raw)
		}
		saved, err := os.ReadFile(process.state)
		if err != nil {
			t.Fatal(err)
		}
		var persisted map[string]any
		if json.Unmarshal(saved, &persisted) != nil || persisted["writes"] != float64(1) {
			t.Fatal("8MiB request did not persist exact one write")
		}
		request["key"], request["expected_version"] = "http-over", 2
		over := pad(marshal(request), 8388609)
		status, raw = process.http(t, "POST", "/writes", over)
		if status != 413 || string(raw) != "resource_limit\n" {
			t.Fatalf("HTTP first-over refusal:%d %s", status, raw)
		}
		process.unchangedState(t, saved)
		status, raw = process.http(t, "GET", "/writes/http-over", nil)
		if status != 404 || string(raw) != "unconfirmed\n" {
			t.Fatalf("HTTP over-limit created receipt:%d %s", status, raw)
		}
		process.unchangedState(t, saved)
	})
	t.Run("E04-service-state-writer", func(t *testing.T) {
		// A small retained receipt makes the16MiB writer ceiling reachable with a
		// request strictly below8MiB. The new body appears in both item and receipt.
		seedRequest := map[string]any{"key": "seed", "expected_version": 0, "source": strings.Repeat("s", 4096), "resource": "seed", "revisions": []any{}, "title": "seed", "body": "seed", "lose_ack": false}
		start := initial()
		start["receipts"] = map[string]any{"seed": map[string]any{"request": seedRequest, "version": 1}}
		startRaw := marshal(start)
		request := map[string]any{"key": "writer", "expected_version": 1, "source": "", "resource": "resource", "revisions": []any{}, "title": "Writer", "body": "", "lose_ack": false}
		expected := func(r map[string]any) []byte {
			var next map[string]any
			if err := json.Unmarshal(startRaw, &next); err != nil {
				t.Fatal(err)
			}
			item := next["item"].(map[string]any)
			item["title"], item["body"], item["version"] = r["title"], r["body"], 2
			next["writes"] = 1
			next["receipts"].(map[string]any)["writer"] = map[string]any{"request": r, "version": 2}
			return marshal(next)
		}
		base := len(expected(request))
		if (16777216-base)%2 != 0 {
			request["source"] = "s"
			base = len(expected(request))
		}
		bodyBytes := (16777216 - base) / 2
		if bodyBytes < 1 {
			t.Fatal("writer fixture underflow")
		}
		request["body"] = strings.Repeat("x", bodyBytes)
		for _, over := range []bool{false, true} {
			r := map[string]any{}
			for k, v := range request {
				r[k] = v
			}
			wanted := 16777216
			if over {
				r["source"] = r["source"].(string) + "s"
				wanted++
			}
			expectedRaw := expected(r)
			requestRaw := marshal(r)
			if len(expectedRaw) != wanted || len(requestRaw) >= 8388608 {
				t.Fatalf("writer independent size proof: prospective state%d want%d; request%d", len(expectedRaw), wanted, len(requestRaw))
			}
			wp25Record(t, map[string]any{"control": "tracker-state-writer-independent-size", "prospective_state_bytes": len(expectedRaw), "prospective_state_sha256": wp25TrackerServiceHash(expectedRaw), "request_bytes": len(requestRaw), "request_sha256": wp25TrackerServiceHash(requestRaw), "retained_seed_bytes": len(startRaw), "body_bytes": bodyBytes, "first_over": over})
			process := wp25TrackerStartService(t, consumer, credential, startRaw, false)
			status, raw := process.http(t, "POST", "/writes", requestRaw)
			if over {
				if status != 503 || string(raw) != "unconfirmed\n" {
					t.Fatalf("writer first-over public refusal:%d %s", status, raw)
				}
				process.unchangedState(t, startRaw)
				status, raw = process.http(t, "GET", "/writes/writer", nil)
				if status != 404 || string(raw) != "unconfirmed\n" {
					t.Fatalf("writer overflow retained receipt:%d %s", status, raw)
				}
				process.unchangedState(t, startRaw)
			} else {
				var receipt map[string]any
				if status != 200 || json.Unmarshal(raw, &receipt) != nil || receipt["version"] != float64(2) {
					t.Fatalf("writer equality response:%d %s", status, raw)
				}
				saved, err := os.ReadFile(process.state)
				if err != nil {
					t.Fatal(err)
				}
				var actual, want any
				if json.Unmarshal(saved, &actual) != nil || json.Unmarshal(expectedRaw, &want) != nil || len(saved) != 16777216 || !reflect.DeepEqual(actual, want) {
					t.Fatal("actual service writer did not persist independent exact16MiB state")
				}
				wp25Record(t, map[string]any{"control": "tracker-state-writer-at-cap", "state_bytes": len(saved), "state_sha256": wp25TrackerServiceHash(saved), "decoded_equals_independent_expected": true})
			}
			process.stop(t)
		}
		t.Log("service writer16MiB/+1 is reachable below8MiB HTTP request cap with retained receipt; first-over is publicly503 unconfirmed, with unchanged item/receipt/file, not a public resource_limit response")
	})
}

type wp25TrackerServiceProcess struct {
	cmd                                 *exec.Cmd
	stdout, stderr                      bytes.Buffer
	done                                chan error
	state, credential, ready, root, url string
	stopped                             bool
}

func wp25TrackerServiceHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", sum)
}
func wp25TrackerStartService(t *testing.T, consumer string, credential, state []byte, wantFailure bool) *wp25TrackerServiceProcess {
	t.Helper()
	root := t.TempDir()
	p := &wp25TrackerServiceProcess{root: root, state: filepath.Join(root, "state.json"), credential: filepath.Join(root, "credential.json"), ready: filepath.Join(root, "ready"), done: make(chan error, 1)}
	for path, raw := range map[string][]byte{p.state: state, p.credential: credential} {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	p.cmd = exec.Command(consumer, "serve", "--state", p.state, "--listen", "127.0.0.1:0", "--ready", p.ready, "--credential", p.credential)
	p.cmd.Env = operationalTestEnvironmentWith(root, map[string]string{"XDG_CONFIG_HOME": filepath.Join(root, "config")})
	p.cmd.Stdout, p.cmd.Stderr = &p.stdout, &p.stderr
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { p.done <- p.cmd.Wait() }()
	t.Cleanup(func() { p.stop(t) })
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-p.done:
			p.stopped = true
			ready, readyErr := os.ReadFile(p.ready)
			after, readErr := os.ReadFile(p.state)
			afterCredential, credentialErr := os.ReadFile(p.credential)
			wp25Record(t, map[string]any{"control": "tracker-service-loader-exit", "binary": consumer, "stdout_raw": p.stdout.String(), "stderr_raw": p.stderr.String(), "process_error": fmt.Sprint(err), "credential_bytes": len(credential), "credential_sha256": wp25TrackerServiceHash(credential), "credential_after_sha256": wp25TrackerServiceHash(afterCredential), "state_bytes": len(state), "state_before_sha256": wp25TrackerServiceHash(state), "state_after_sha256": wp25TrackerServiceHash(after), "ready_raw": string(ready), "ready_error": fmt.Sprint(readyErr)})
			if !wantFailure || err == nil || p.stdout.Len() != 0 || p.stderr.String() != "fake tracker stopped: resource_limit\n" || !os.IsNotExist(readyErr) || readErr != nil || credentialErr != nil || !bytes.Equal(after, state) || !bytes.Equal(afterCredential, credential) {
				t.Fatalf("service loader refusal: wantFailure=%v err=%v stdout=%s stderr=%s", wantFailure, err, &p.stdout, &p.stderr)
			}
			return p
		case <-deadline.C:
			p.stop(t)
			t.Fatal("supplied tracker service did not settle startup within15s")
		case <-tick.C:
			ready, err := os.ReadFile(p.ready)
			if err == nil {
				if wantFailure {
					p.stop(t)
					t.Fatal("over-limit loader reached ready/listen")
				}
				if !strings.HasPrefix(string(ready), "http://127.0.0.1:") {
					p.stop(t)
					t.Fatalf("unexpected ready address:%s", ready)
				}
				p.url = string(ready)
				wp25Record(t, map[string]any{"control": "tracker-service-ready", "binary": consumer, "credential_bytes": len(credential), "credential_sha256": wp25TrackerServiceHash(credential), "state_bytes": len(state), "state_sha256": wp25TrackerServiceHash(state), "ready_raw": string(ready)})
				return p
			}
		}
	}
}
func (p *wp25TrackerServiceProcess) stop(t *testing.T) {
	t.Helper()
	if p.stopped {
		return
	}
	p.stopped = true
	_ = p.cmd.Process.Kill()
	err := <-p.done
	wp25Record(t, map[string]any{"control": "tracker-service-process-reaped", "stdout_raw": p.stdout.String(), "stderr_raw": p.stderr.String(), "process_error": fmt.Sprint(err), "intentional_test_cleanup": true})
}
func (p *wp25TrackerServiceProcess) unchangedState(t *testing.T, want []byte) {
	t.Helper()
	actual, err := os.ReadFile(p.state)
	wp25Record(t, map[string]any{"control": "tracker-service-state-no-effect", "before_sha256": wp25TrackerServiceHash(want), "after_sha256": wp25TrackerServiceHash(actual), "read_error": fmt.Sprint(err)})
	if err != nil || !bytes.Equal(actual, want) {
		t.Fatal("refused/read tracker operation changed protected state")
	}
}
func (p *wp25TrackerServiceProcess) http(t *testing.T, method, path string, body []byte) (int, []byte) {
	t.Helper()
	before, beforeErr := os.ReadFile(p.state)
	if beforeErr != nil {
		t.Fatal(beforeErr)
	}
	request, err := http.NewRequest(method, p.url+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer wp25-service-token")
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		wp25Record(t, map[string]any{"control": "tracker-service-http-transport-failure", "method": method, "path": path, "request_bytes": len(body), "request_sha256": wp25TrackerServiceHash(body), "transport_error": err.Error()})
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, 16777217))
	after, afterErr := os.ReadFile(p.state)
	wp25Record(t, map[string]any{"control": "tracker-service-http", "method": method, "path": path, "request_bytes": len(body), "request_sha256": wp25TrackerServiceHash(body), "http_status": response.StatusCode, "response_headers": response.Header, "response_raw": string(raw), "response_sha256": wp25TrackerServiceHash(raw), "response_read_error": fmt.Sprint(readErr), "state_before_sha256": wp25TrackerServiceHash(before), "state_after_sha256": wp25TrackerServiceHash(after), "state_after_read_error": fmt.Sprint(afterErr)})
	if readErr != nil || afterErr != nil || len(raw) > 16777216 {
		t.Fatal("tracker service response/state observation failed", readErr, afterErr)
	}
	return response.StatusCode, raw
}

// SOURCE-ASSISTED public fixture construction only. The signed literal artifact
// chain is independently encoded, signed and imported; production record helpers
// do not construct or validate it. The supplied executable performs each read.
func wp25ArtifactRecordCountLimit(t *testing.T) {
	binary := os.Getenv("HN_WP05_BINARY")
	if binary == "" {
		t.Fatal("R02 requires supplied artifact binary")
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	repo := filepath.Join(root, "repo")
	config := wp25FixtureConfig(t, home)
	f := &artifactCLIFixture{t: t, binary: binary, repo: repo, home: home, overrides: map[string]string{"XDG_CONFIG_HOME": config}}
	f.git("init", "-q")
	evidence := os.Getenv("HN_WP25_EVIDENCE")
	if evidence == "" {
		evidence = t.TempDir()
	}
	evidence = filepath.Join(evidence, "source-assistance", "R02")
	if err := os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	ledgerPath := filepath.Join(evidence, "signed-input-ledger.jsonl")
	ledger, err := os.Create(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	ledgerHash := sha256.New()
	ledgerWriter := io.MultiWriter(ledger, ledgerHash)
	ledgerBytes := int64(0)
	ledgerAtBytes := int64(0)
	ledgerAtHash := ""
	wirePath := filepath.Join(evidence, "public-chain.fast-import")
	wire, err := os.Create(wirePath)
	if err != nil {
		t.Fatal(err)
	}
	wireHash := sha256.New()
	wireWriter := bufio.NewWriterSize(io.MultiWriter(wire, wireHash), 1<<20)
	seed := sha256.Sum256([]byte("WP25 R02 deterministic public count fixture"))
	key := ed25519.NewKeyFromSeed(seed[:])
	public := key.Public().(ed25519.PublicKey)
	actorSum := sha256.Sum256(public)
	actor := hex.EncodeToString(actorSum[:])
	streamSum := sha256.Sum256([]byte("hn-artifact-stream-v1\x00" + actor))
	streamID := fmt.Sprintf("sha256:%x", streamSum)
	ref := "refs/hn/artifacts/" + actor + "/" + hex.EncodeToString(streamSum[:])
	fmt.Fprintf(wireWriter, "reset %s\nfrom %s\n\n", ref, strings.Repeat("0", 40))
	previous, typeID, atID, overID := "", "", "", ""
	maxEnvelope := 0
	payloadBytes := int64(0)
	for sequence := 1; sequence <= 100001; sequence++ {
		kind := "record.open"
		payload := map[string]any{"shape": "document", "type_id": "example.count/note", "type_revision": typeID, "title": "r", "content": "", "fields": map[string]any{}, "members": []any{}, "relationships": []any{}, "conversion_sources": []any{}}
		if sequence == 1 {
			kind = "type.publish"
			payload = map[string]any{"type_id": "example.count/note", "schema_version": 1, "shape": "document", "schema": map[string]any{"language": "hn.schema/1", "root": map[string]any{"type": "object", "properties": map[string]any{}, "required": []any{}, "additionalProperties": false}}}
		}
		operation := fmt.Sprintf("r02-%06d", sequence)
		semantic := map[string]any{"actor": actor, "kind": kind, "operation": operation, "parents": []any{}, "payload": payload, "protocol": "hn-artifact/1", "record": ""}
		semanticRaw, err := json.Marshal(semantic)
		if err != nil {
			t.Fatal(err)
		}
		requestSum := sha256.Sum256(append([]byte("hn-artifact-request-v1\x00"), semanticRaw...))
		request := fmt.Sprintf("sha256:%x", requestSum)
		envelope := map[string]any{"protocol": "hn-artifact/1", "kind": kind, "actor": actor, "public_key": base64.RawStdEncoding.EncodeToString(public), "stream": streamID, "sequence": sequence, "previous": previous, "timestamp": "2026-10-31T08:00:00Z", "operation": operation, "request": request, "record": "", "parents": []any{}, "payload": payload}
		raw, err := json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		signedBytes := append([]byte("hn-artifact/1\x00"), raw...)
		signatureBytes := ed25519.Sign(key, signedBytes)
		if !ed25519.Verify(public, signedBytes, signatureBytes) {
			t.Fatal("R02 independent public signature check")
		}
		signature := base64.RawStdEncoding.EncodeToString(signatureBytes)
		idSum := sha256.Sum256(raw)
		id := fmt.Sprintf("sha256:%x", idSum)
		if len(raw) > maxEnvelope {
			maxEnvelope = len(raw)
		}
		payloadBytes += int64(len(raw))
		if sequence == 1 {
			typeID = id
		}
		if sequence == 100000 {
			atID = id
		}
		if sequence == 100001 {
			overID = id
		}
		message := "hn artifact " + id + "\n"
		fmt.Fprintf(wireWriter, "commit %s\nmark :%d\ncommitter WP25 public count fixture <fixture@hn.invalid> 1793433600 +0000\ndata %d\n%s\n", ref, sequence, len(message), message)
		if sequence > 1 {
			fmt.Fprintf(wireWriter, "from :%d\n", sequence-1)
		}
		fmt.Fprintf(wireWriter, "deleteall\nM 100644 inline artifact.json\ndata %d\n", len(raw))
		if _, err := wireWriter.Write(raw); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(wireWriter, "\nM 100644 inline signature\ndata %d\n%s\n\n", len(signature), signature)
		line, err := json.Marshal(map[string]any{"sequence": sequence, "kind": kind, "operation": operation, "id": id, "previous": previous, "request": request, "payload_bytes": len(raw), "payload_sha256": id, "signature_bytes": len(signature), "signature_sha256": wp25RecordCountHash([]byte(signature))})
		if err != nil {
			t.Fatal(err)
		}
		line = append(line, '\n')
		n, err := ledgerWriter.Write(line)
		if err != nil {
			t.Fatal(err)
		}
		ledgerBytes += int64(n)
		if sequence == 100000 {
			ledgerAtBytes = ledgerBytes
			ledgerAtHash = fmt.Sprintf("sha256:%x", ledgerHash.Sum(nil))
		}
		previous = id
	}
	if err := wireWriter.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := wire.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	if maxEnvelope >= 1048576 || payloadBytes >= 268435456 {
		t.Fatal("R02 source fixture violates enclosing envelope/aggregate dimension")
	}
	wireInput, err := os.Open(wirePath)
	if err != nil {
		t.Fatal(err)
	}
	importer := exec.Command("git", "fast-import", "--quiet")
	importer.Dir, importer.Env, importer.Stdin = repo, operationalTestEnvironmentWith(home, f.overrides), wireInput
	var importOut, importErr bytes.Buffer
	importer.Stdout, importer.Stderr = &importOut, &importErr
	constructionStart := time.Now()
	importError := importer.Run()
	wireInput.Close()
	wp25Record(t, map[string]any{"control": "R02-source-assisted-fast-import", "source_assisted": true, "stdout_raw": importOut.String(), "stderr_raw": importErr.String(), "process_error": fmt.Sprint(importError), "elapsed_ns": time.Since(constructionStart).Nanoseconds(), "wire_sha256": fmt.Sprintf("sha256:%x", wireHash.Sum(nil)), "signed_inputs": 100001, "type_publications": 1, "record_opens": 100000, "max_envelope_bytes": maxEnvelope, "envelope_bytes_sum": payloadBytes})
	if importError != nil {
		t.Fatal("R02 public import", importError, &importErr)
	}
	fullTip := f.git("rev-parse", ref)
	atTip := f.git("rev-parse", ref+"~1")
	common := map[string]any{"source_assisted": true, "not_exact_package_admission": true, "public_actor": actor, "public_stream": streamID, "ref": ref, "type_id": typeID, "fixed_timestamp": "2026-10-31T08:00:00Z", "producer": "standard JSON+ed25519, literal closed artifact envelope/schema, Git fast-import; no production record helper", "signed_input_ledger": ledgerPath, "signed_input_ledger_bytes": ledgerBytes, "signed_input_ledger_sha256": fmt.Sprintf("sha256:%x", ledgerHash.Sum(nil)), "full_fast_import_path": wirePath, "full_fast_import_sha256": fmt.Sprintf("sha256:%x", wireHash.Sum(nil)), "cuts": []any{}}
	for _, row := range []struct {
		count        int
		tip, last    string
		ledgerPrefix int64
		ledgerHash   string
	}{{100000, atTip, atID, ledgerAtBytes, ledgerAtHash}, {100001, fullTip, overID, ledgerBytes, fmt.Sprintf("sha256:%x", ledgerHash.Sum(nil))}} {
		f.git("update-ref", ref, row.tip)
		// A distinct accepted remote selects the overlapping prefix. The count
		// is of unique immutable commits, not the sum of per-ref chain lengths.
		prefixRef := strings.Replace(ref, "refs/hn/", "refs/hn/remotes/count-prefix/", 1)
		prefixTip := f.git("rev-parse", row.tip+"~1")
		f.git("update-ref", prefixRef, prefixTip)
		inventory := wp25RecordCountInventory(t, f, row.tip, row.count, filepath.Join(evidence, fmt.Sprintf("objects-%d.txt", row.count)))
		inventory["records"] = row.count
		inventory["overlapping_selected_ref"] = prefixRef
		inventory["overlapping_selected_tip"] = prefixTip
		inventory["type_publications"] = 1
		inventory["record_opens"] = row.count - 1
		inventory["tip"] = row.tip
		inventory["last_artifact_id"] = row.last
		inventory["input_ledger_prefix_bytes"] = row.ledgerPrefix
		inventory["input_ledger_prefix_sha256"] = row.ledgerHash
		before := f.snapshot()
		beforeRaw, _ := json.Marshal(before)
		watchdog, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		cmd := exec.CommandContext(watchdog, binary, "artifact", "type", "show", typeID, "--json")
		cmd.Dir, cmd.Env = repo, operationalTestEnvironmentWith(home, f.overrides)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		started := time.Now()
		runErr := cmd.Run()
		elapsed := time.Since(started)
		watchdogErr := watchdog.Err()
		cancel()
		after := f.snapshot()
		afterRaw, _ := json.Marshal(after)
		observation := map[string]any{"control": "R02-public-count-boundary", "binary": binary, "args": cmd.Args[1:], "cwd": repo, "source_assisted_construction_only": true, "source_cut_tip": row.tip, "source_records": row.count, "stdout_raw": stdout.String(), "stderr_raw": stderr.String(), "elapsed_ns": elapsed.Nanoseconds(), "process_error": fmt.Sprint(runErr), "watchdog_error": fmt.Sprint(watchdogErr), "protected_before_sha256": wp25RecordCountHash(beforeRaw), "protected_after_sha256": wp25RecordCountHash(afterRaw), "successful_at_cap_admission_claimed": false}
		wp25Record(t, observation)
		if watchdogErr != nil {
			t.Fatal("R02 watchdog killed candidate before its normal30s stage settled")
		}
		var result map[string]any
		if runErr == nil && row.count == 100000 {
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result["ok"] != true || result["error"] != nil {
				t.Fatalf("R02 zero exit lacks complete public success: %s", &stdout)
			}
			data, ok := result["data"].(map[string]any)
			if !ok {
				t.Fatal("R02 success lacks typed data")
			}
			items, ok := data["items"].([]any)
			if !ok || len(items) != 1 || items[0].(map[string]any)["id"] != typeID || data["next_cursor"] != nil {
				t.Fatal("R02 equality success changed exact selected type", data)
			}
			f.unchanged(before)
			inventory["observed_classification"] = "actual complete100000-record read succeeded within existing budget"
			inventory["actual_code"] = ""
			inventory["elapsed_ns"] = elapsed.Nanoseconds()
			common["cuts"] = append(common["cuts"].([]any), inventory)
			raw, _ := json.MarshalIndent(common, "", "  ")
			if err := os.WriteFile(filepath.Join(evidence, "R02-source-assistance.json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			wp25Record(t, map[string]any{"control": "R02-source-assistance-map", "map": inventory, "successful_at_cap_admission_claimed": true})
			continue
		}
		if runErr == nil || json.Unmarshal(stdout.Bytes(), &result) != nil || len(result) != 3 || result["ok"] != false || result["data"] != nil {
			t.Fatalf("R02 must return real bounded refusal:%v %s", runErr, &stdout)
		}
		problem, ok := result["error"].(map[string]any)
		if !ok || len(problem) != 5 || problem["origin"] != "artifact" || problem["recovery"] != "do_not_retry" || problem["operation"] != nil {
			t.Fatalf("R02 refusal identity:%s", &stdout)
		}
		code, _ := problem["code"].(string)
		if row.count == 100001 && (code != "resource_limit" || problem["message"] != "selected artifact record limit exceeded") {
			t.Fatalf("R02 actual first-over must reach the selected record-count guard: %s", &stdout)
		}
		switch code {
		case "cancelled":
			if elapsed < 29*time.Second || elapsed >= 90*time.Second || problem["message"] != "artifact read cancelled" {
				t.Fatalf("R02 cancellation is not actual enclosing30s read stage:%v %s", elapsed, &stdout)
			}
			inventory["observed_classification"] = "count ceiling admitted equality; subsequent normal30s full verification cancelled; causal threshold proof is a separate mutation observation"
		case "resource_limit":
			if row.count != 100001 || problem["message"] != "selected artifact record limit exceeded" {
				t.Fatalf("R02 unexpected earlier limit:%s", &stdout)
			}
			inventory["observed_classification"] = "actual first-over preflight record guard observed; unchanged protected state"
		default:
			t.Fatalf("R02 wrong bounded cause:%s", &stdout)
		}
		if row.count == 100000 && code != "cancelled" {
			t.Fatal("R02 count equality did not observe the real enclosing deadline")
		}
		f.unchanged(before)
		if _, err := os.Stat(filepath.Join(repo, ".git", "hn")); !os.IsNotExist(err) {
			t.Fatal("R02 keyless read initialized private native state", err)
		}
		inventory["actual_code"], inventory["actual_message"], inventory["elapsed_ns"] = code, problem["message"], elapsed.Nanoseconds()
		common["cuts"] = append(common["cuts"].([]any), inventory)
		raw, _ := json.MarshalIndent(common, "", "  ")
		if err := os.WriteFile(filepath.Join(evidence, "R02-source-assistance.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		wp25Record(t, map[string]any{"control": "R02-source-assistance-map", "map": inventory, "source_assistance_json": filepath.Join(evidence, "R02-source-assistance.json")})
	}
	t.Log("R02 count100000/+1 uses complete independent signed corpus/object maps. Equality may pass the count gate then cancel during full verification; complete history success is not inferred. Local threshold99999/removal public controls separately establish causality.")
}
func wp25RecordCountHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", sum)
}
func wp25RecordCountInventory(t *testing.T, f *artifactCLIFixture, tip string, records int, path string) map[string]any {
	t.Helper()
	env := operationalTestEnvironmentWith(f.home, f.overrides)
	walk := exec.Command("git", "rev-list", "--objects", "--no-object-names", tip)
	walk.Dir, walk.Env = f.repo, env
	oids, err := walk.Output()
	if err != nil {
		t.Fatal("R02 independent object enumeration", err)
	}
	cat := exec.Command("git", "cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize)")
	cat.Dir, cat.Env, cat.Stdin = f.repo, env, bytes.NewReader(oids)
	raw, err := cat.Output()
	if err != nil {
		t.Fatal("R02 independent object size inventory", err)
	}
	lines := strings.FieldsFunc(string(raw), func(r rune) bool { return r == '\n' })
	sort.Strings(lines)
	counts := map[string]int{"commit": 0, "tree": 0, "blob": 0}
	total := int64(0)
	maximum := int64(0)
	previous := ""
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) != 3 || len(parts[0]) != 40 || parts[0] == previous {
			t.Fatal("R02 non-closed independent object inventory", line)
		}
		previous = parts[0]
		n, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || n < 0 {
			t.Fatal("R02 invalid independent object length", line)
		}
		if _, ok := counts[parts[1]]; !ok {
			t.Fatal("R02 unexpected object type", line)
		}
		counts[parts[1]]++
		total += n
		if n > maximum {
			maximum = n
		}
	}
	if len(lines) != 4*records || counts["commit"] != records || counts["tree"] != records || counts["blob"] != 2*records || total >= 268435456 || maximum >= 8388608 {
		t.Fatalf("R02 object dimensions must isolate count: records%d counts%v objects%d bytes%d max%d", records, counts, len(lines), total, maximum)
	}
	canonical := []byte(strings.Join(lines, "\n") + "\n")
	if err := os.WriteFile(path, canonical, 0600); err != nil {
		t.Fatal(err)
	}
	return map[string]any{"object_inventory": path, "object_inventory_sha256": wp25RecordCountHash(canonical), "object_inventory_bytes": len(canonical), "unique_objects": len(lines), "object_counts": counts, "raw_object_bytes": total, "max_object_bytes": maximum, "aggregate_bound": 268435456, "individual_object_bound": 8388608, "closure": "complete accepted single actor chain; four distinct public Git objects per signed envelope"}
}

func wp25SavePublicFixture(t *testing.T, name string, raw []byte) {
	t.Helper()
	if root := os.Getenv("HN_WP25_EVIDENCE"); root != "" {
		dir := filepath.Join(root, "source-assistance")
		if e := os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(dir, name), raw, 0600); e != nil {
			t.Fatal(e)
		}
		wp25Record(t, map[string]any{"source_assisted": true, "public_fixture": filepath.Join(dir, name), "bytes": len(raw), "sha256": eventID(raw)})
	}
}

// These tests exercise supplied public validators and consumers.
// Typed utility success is explicitly not
// authenticated publication, emitted head cardinality, authority or history.
func wp25RemainingTypedLimits(t *testing.T, binary string, references map[string]string) {
	t.Run("A14-signed-year-and-safe-integer-domain", func(t *testing.T) {
		for _, tc := range []struct {
			name, code string
			change     func(map[string]any)
		}{
			{"year0001", "", func(m map[string]any) { m["timestamp"] = "0001-01-01T00:00:00Z" }},
			{"year9999", "", func(m map[string]any) { m["timestamp"] = "9999-12-31T23:59:59Z" }},
			{"year0000", "invalid_input", func(m map[string]any) { m["timestamp"] = "0000-01-01T00:00:00Z" }},
			{"year10000", "invalid_input", func(m map[string]any) { m["timestamp"] = "10000-01-01T00:00:00Z" }},
			{"schema-version-one", "", func(m map[string]any) { m["payload"].(map[string]any)["schema_version"] = 1 }},
			{"schema-version-zero", "invalid_input", func(m map[string]any) { m["payload"].(map[string]any)["schema_version"] = 0 }},
			{"sequence-one", "", func(m map[string]any) { m["sequence"] = 1; m["previous"] = "" }},
			{"sequence-zero", "invalid_input", func(m map[string]any) { m["sequence"] = 0; m["previous"] = "" }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				raw, sig := artifactSignedCLI(t, tc.change)
				wp25RemainingEnvelope(t, binary, raw, sig, tc.code, "signed-envelope-domain; no actor-chain or current-head assertion")
			})
		}
		for _, tc := range []struct{ name, value, code string }{
			{"integer-min", "-9007199254740991", ""}, {"integer-zero", "0", ""}, {"integer-max", "9007199254740991", ""},
			{"integer-under", "-9007199254740992", "invalid_input"}, {"integer-over", "9007199254740992", "invalid_input"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				raw, sig := artifactSignedCLI(t, func(m map[string]any) {
					wp25RemainingRecord(m)
					m["payload"].(map[string]any)["fields"] = map[string]any{"number": json.Number(tc.value)}
				})
				wp25RemainingEnvelope(t, binary, raw, sig, tc.code, "canonical safe-integer domain at public signed-envelope validator; schema/type history intentionally not supplied")
			})
		}
	})
	t.Run("A16-explicit-envelope-parent-cardinalities", func(t *testing.T) {
		for _, kind := range []string{"type.publish", "record.open", "record.revise", "record.convert", "record.resolve"} {
			for _, n := range []int{0, 1, 2} {
				t.Run(fmt.Sprintf("%s-%d", kind, n), func(t *testing.T) {
					code := ""
					switch kind {
					case "type.publish":
						if n > 1 {
							code = "invalid_input"
						}
					case "record.open":
						if n != 0 {
							code = "invalid_input"
						}
					case "record.revise", "record.convert":
						if n != 1 {
							code = "invalid_input"
						}
					case "record.resolve":
						if n < 1 {
							code = "invalid_input"
						}
					}
					raw, sig := artifactSignedCLI(t, func(m map[string]any) {
						if kind != "type.publish" {
							wp25RemainingRecord(m)
						}
						m["kind"] = kind
						parents := make([]string, n)
						for i := range parents {
							parents[i] = fmt.Sprintf("sha256:%064x", i+1)
						}
						m["parents"] = parents
						if kind == "type.publish" && n > 0 {
							m["payload"].(map[string]any)["schema_version"] = 2
						}
						if kind != "type.publish" && kind != "record.open" {
							m["record"] = artifactVectorID
						}
						if kind == "record.convert" && n > 0 {
							m["payload"].(map[string]any)["conversion_sources"] = []string{parents[0]}
						}
					})
					wp25RemainingEnvelope(t, binary, raw, sig, code, "envelope cardinality only; selected TestArtifactStoreTypeEvolutionCLI/TestArtifactRevisionLiveCLI/TestArtifactRevisionCausalRefusalsCLI supply actual semantic writer positives")
				})
			}
		}
	})
	t.Run("H08-public-typed-error-message", func(t *testing.T) {
		for _, n := range []int{4096, 4097} {
			code := ""
			if n == 4097 {
				code = "resource_limit"
			}
			value := map[string]any{"protocol": "hn.extension/1", "request_id": "typed-error-message", "ok": false, "data": nil, "error": map[string]any{"origin": "host", "code": "denied_access", "message": strings.Repeat("m", n), "recovery": "do_not_retry", "operation": nil}}
			wp25RemainingResponse(t, binary, value, "typed-error-message", "", code, "typed error-message consumer boundary; does not claim host actually emitted4096-byte diagnostic")
		}
	})
	t.Run("H15-public-typed-head-count-zero-and-maxsafe", func(t *testing.T) {
		digest := "sha256:" + strings.Repeat("a", 64)
		for _, n := range []string{"0", "1", "9007199254740991", "9007199254740992"} {
			code := ""
			if n == "9007199254740992" {
				code = "invalid_input"
			}
			admission := "admitted"
			if n == "0" {
				admission = "pending"
			}
			value := map[string]any{"protocol": "hn.extension/1", "request_id": "typed-head-count", "ok": true, "data": map[string]any{"protocol": "hn/0", "family": "issue", "type_id": nil, "resource_id": digest, "event_id": digest, "replayed": false, "conflict": false, "head_count": json.Number(n), "admission": admission, "source_cut": digest}, "error": nil}
			wp25RemainingResponse(t, binary, value, "typed-head-count", "resource.mutate", code, "independent typed consumer numeric domain; no emitted maxsafe-cardinality/publication claim; zero pending visible heads remains positive")
		}
	})
	t.Run("E03-supplied-charter-parser-depth", func(t *testing.T) { wp25RemainingCharterDepth(t, references["charter"]) })
}

func wp25RemainingRecord(m map[string]any) {
	m["kind"] = "record.open"
	m["record"] = ""
	m["parents"] = []string{}
	m["payload"] = map[string]any{"shape": "document", "type_id": "example/note", "type_revision": artifactVectorID, "title": "Title", "content": "", "fields": map[string]any{}, "members": []any{}, "relationships": []any{}, "conversion_sources": []any{}}
}

func wp25RemainingEnvelope(t *testing.T, binary, raw, sig, code, scope string) {
	t.Helper()
	home, repo := t.TempDir(), t.TempDir()
	beforeRepo, beforeHome := protocolTree(t, repo), protocolTree(t, home)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "artifact", "envelope", "validate", "--input", "-", "--signature", sig, "--json")
	cmd.Dir, cmd.Env, cmd.Stdin = repo, operationalTestEnvironment(home), strings.NewReader(raw)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	started := time.Now()
	runErr := cmd.Run()
	afterRepo, afterHome := protocolTree(t, repo), protocolTree(t, home)
	wp25Record(t, map[string]any{"control": "remaining-signed-envelope", "scope": scope, "program": binary, "argv": cmd.Args[1:], "stdin": raw, "signature": sig, "stdout": stdout.String(), "stderr": stderr.String(), "exit_error": fmt.Sprint(runErr), "watchdog": fmt.Sprint(ctx.Err()), "elapsed_ns": time.Since(started).Nanoseconds(), "expected_code": code, "before_repo": beforeRepo, "after_repo": afterRepo, "before_home": beforeHome, "after_home": afterHome})
	if ctx.Err() != nil {
		t.Fatal("signed envelope watchdog", ctx.Err())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil || len(out) != 3 {
		t.Fatalf("closed artifact envelope: %v %s", err, &stdout)
	}
	if code == "" {
		if runErr != nil || out["ok"] != true || out["error"] != nil || out["data"] == nil {
			t.Fatalf("valid public envelope: %v %s", runErr, &stdout)
		}
	} else {
		wp25LifecycleEnvelope(t, stdout.Bytes(), runErr, "artifact", code, "do_not_retry", nil)
	}
	if !reflect.DeepEqual(beforeRepo, afterRepo) || !reflect.DeepEqual(beforeHome, afterHome) {
		t.Fatal("validator changed native/repository/config state")
	}
}

func wp25RemainingResponse(t *testing.T, binary string, value map[string]any, requestID, method, code, scope string) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	home, repo := t.TempDir(), t.TempDir()
	beforeRepo, beforeHome := protocolTree(t, repo), protocolTree(t, home)
	args := []string{"extension", "frame", "inspect", "--input", "-", "--kind", "response", "--expected-request-id", requestID, "--json"}
	if method != "" {
		args = append(args, "--method", method)
	}
	origin, recovery := "", ""
	if code != "" {
		origin, recovery = "host", "do_not_retry"
	}
	result := wp25LifecycleCLI(t, binary, repo, operationalTestEnvironment(home), protocolRawFrame(raw), origin, code, recovery, args...)
	if code == "" {
		data := result["data"].(map[string]any)
		if data["validation"] != "frame_and_typed_message" || data["dispatch"] != "not_performed" || data["request_id"] != requestID {
			t.Fatal("public utility overstated typed validation", data)
		}
	}
	afterRepo, afterHome := protocolTree(t, repo), protocolTree(t, home)
	wp25Record(t, map[string]any{"control": "remaining-typed-response", "scope": scope, "response_raw": string(raw), "argv": args, "expected_code": code, "public_result": result, "before_repo": beforeRepo, "after_repo": afterRepo, "before_home": beforeHome, "after_home": afterHome})
	if !reflect.DeepEqual(beforeRepo, afterRepo) || !reflect.DeepEqual(beforeHome, afterHome) {
		t.Fatal("typed response utility changed repository/config state")
	}
}

func wp25RemainingCharterDepth(t *testing.T, consumer string) {
	t.Helper()
	if consumer == "" {
		t.Fatal("charter parser depth requires supplied executable")
	}
	dir := t.TempDir()
	input, output, state := filepath.Join(dir, "input.json"), filepath.Join(dir, "output.json"), filepath.Join(dir, "state.json")
	for _, n := range []int{64, 65} {
		// Root is parser depth0. n-1 nested unknown-field objects leave the
		// scalar at depth n.64 reaches the independent closed-command refusal;
		//65 reaches parser recursion refusal before host negotiation.
		raw := `{"action":"call","method":"identity.public","params":{},"unknown":` + strings.Repeat(`{"x":`, n-1) + `0` + strings.Repeat(`}`, n-1) + `}`
		for path, value := range map[string]string{input: raw, output: "PRIVATE-UNCHANGED-OUTPUT", state: "PRIVATE-UNCHANGED-STATE"} {
			if err := os.WriteFile(path, []byte(value), 0600); err != nil {
				t.Fatal(err)
			}
		}
		before := protocolTree(t, dir)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, consumer, "--input", input, "--output", output, "--state", state)
		cmd.Dir, cmd.Env, cmd.Stdin = dir, operationalTestEnvironment(dir), bytes.NewReader(nil)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		started := time.Now()
		runErr := cmd.Run()
		watchdog := ctx.Err()
		cancel()
		after := protocolTree(t, dir)
		cause := "unknown application command field"
		if n == 65 {
			cause = "JSON nesting exceeds 64"
		}
		wp25Record(t, map[string]any{"control": "charter-supplied-parser-depth", "depth": n, "consumer": consumer, "consumer_sha256": wp25Digest(t, consumer), "argv": cmd.Args[1:], "stdin_file_raw": raw, "stdout": stdout.String(), "stderr": stderr.String(), "expected_refusal_cause": cause, "exit_error": fmt.Sprint(runErr), "watchdog": fmt.Sprint(watchdog), "elapsed_ns": time.Since(started).Nanoseconds(), "before_snapshot": before, "after_snapshot": after, "scope": "depth64 parser acceptance reaches semantic unknown-field refusal;65 parser refuses; neither claims supported64-level application command or native host publication"})
		if watchdog != nil || runErr == nil || stdout.Len() != 0 || stderr.String() != "charter-model: "+cause+"\n" {
			t.Fatalf("supplied charter depth%d cause/order: exit=%v watchdog=%v stdout=%s stderr=%s", n, runErr, watchdog, &stdout, &stderr)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatal("local parser/closed-command refusal replaced private selection/output")
		}
	}
}

func TestExtensionPackageRemainingTypedLimits(t *testing.T) {
	binary := extensionTestBinary(t)
	wp25RemainingTypedLimits(t, binary, map[string]string{"charter": charterConsumer(t)})
}

// No sample or host package is imported by these controls.
// These are independently supplied consumer controls with literal scripted host
// frames. Normal actual host/standalone/clone journeys remain separate evidence.
func wp25TrackerClientLimits(t *testing.T, consumer string) {
	if consumer == "" {
		t.Fatal("client limits require supplied fake-tracker executable")
	}
	t.Run("E04-client-observe-plan-limits", func(t *testing.T) { wp25TrackerClientPagePlans(t, consumer) })
	t.Run("E04-client-HTTP-response-eight-MiB", func(t *testing.T) {
		item := map[string]any{"id": "42", "version": 1, "title": "Valid HTTP item", "body": "small", "author": "claim", "time": "2026-10-31T00:00:00Z", "comments": []any{}}
		base := wp25TrackerClientJSON(t, item)
		for _, n := range []int{8388608, 8388609} {
			body := append(append([]byte{}, base...), bytes.Repeat([]byte(" "), n-len(base))...)
			events := make(chan map[string]any, 4)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				written, err := w.Write(body)
				events <- map[string]any{"method": r.Method, "path": r.URL.Path, "credential_matches": r.Header.Get("Authorization") == "Bearer WP25-client-fixture-token", "body_bytes": len(body), "body_sha256": wp25TrackerClientHash(body), "written": written, "write_error": fmt.Sprint(err)}
			}))
			t.Cleanup(server.Close)
			f := wp25TrackerClientNew(t, consumer, server.URL, map[string]any{"action": "import"})
			before := wp25Read(t, f.journal)
			incoming := append(protocolRawFrame(wp25TrackerClientJSON(t, f.hello)), protocolRawFrame(wp25TrackerClientJSON(t, f.identity))...)
			want := "invalid_input"
			if n == 8388609 {
				want = "resource_limit"
			}
			wire := f.run(t, incoming, want)
			var event map[string]any
			select {
			case event = <-events:
			case <-time.After(2 * time.Second):
				t.Fatal("actual standalone client did not consume loopback HTTP response")
			}
			server.Close()
			if event["method"] != "GET" || event["path"] != "/item" || event["credential_matches"] != true {
				t.Fatal("HTTP boundary observed wrong real caller", event)
			}
			if len(wire) != 2 || wire[1]["method"] != "identity.public" {
				t.Fatal("HTTP control unexpectedly dispatched native mutation", wire)
			}
			if !bytes.Equal(before, wp25Read(t, f.journal)) {
				t.Fatal("HTTP input refusal prepared immutable native intent or changed journal")
			}
			wp25Record(t, map[string]any{"control": "tracker-client-HTTP-response", "wire_bytes": n, "HTTP_event": event, "response_asset": wp25TrackerClientAsset(t, fmt.Sprintf("HTTP-%d.json", n), body), "expected_exit_cause": want, "host_requests": wire, "journal_unchanged": true, "scope": "8MiB response equality parses valid item and reaches independent unsupported0comments refusal;8MiB+1 stops in HTTP readBounded before item/intent creation; no native publication claim"})
		}
	})
	t.Run("E04-client-journal-writer-sixteen-MiB", func(t *testing.T) {
		for _, over := range []bool{false, true} {
			f := wp25TrackerClientNew(t, consumer, "http://127.0.0.1:1", map[string]any{"action": "unavailable-fixture-action"})
			// Existing retained journal is valid, fully bound and below its read
			// cap. The tiny identity response adds only the actor/key binding.
			f.j["actor"], f.j["public_key"] = "", ""
			f.j["entries"] = map[string]any{"retained-private-entry": map[string]any{"padding": ""}}
			prospective := wp25TrackerClientCopy(t, f.j)
			data := f.identity["data"].(map[string]any)
			prospective["actor"], prospective["public_key"] = data["actor"], data["public_key"]
			wanted := 16777216
			if over {
				wanted++
			}
			padding := wanted - len(wp25TrackerClientJournal(t, prospective))
			if padding < 1 {
				t.Fatal("journal writer fixture underflow")
			}
			f.j["entries"].(map[string]any)["retained-private-entry"].(map[string]any)["padding"] = strings.Repeat("x", padding)
			prospective = wp25TrackerClientCopy(t, f.j)
			prospective["actor"], prospective["public_key"] = data["actor"], data["public_key"]
			before, after := wp25TrackerClientJournal(t, f.j), wp25TrackerClientJournal(t, prospective)
			if len(before) >= 16777216 || len(after) != wanted {
				t.Fatalf("independent journal sizes before%d after%d wanted%d", len(before), len(after), wanted)
			}
			if err := os.WriteFile(f.journal, before, 0600); err != nil {
				t.Fatal(err)
			}
			incoming := append(protocolRawFrame(wp25TrackerClientJSON(t, f.hello)), protocolRawFrame(wp25TrackerClientJSON(t, f.identity))...)
			want := "invalid_input"
			if over {
				want = "resource_limit"
			}
			wire := f.run(t, incoming, want)
			observed := wp25Read(t, f.journal)
			if len(wire) != 2 || wire[1]["method"] != "identity.public" {
				t.Fatal("journal writer control issued extra host calls", wire)
			}
			if over {
				if !bytes.Equal(before, observed) {
					t.Fatal("first-over journal writer replaced retained journal")
				}
			} else {
				if !bytes.Equal(after, observed) {
					t.Fatal("actual journal writer did not persist independent exact16MiB encoding")
				}
			}
			wp25Record(t, map[string]any{"control": "tracker-client-journal-writer", "first_over": over, "before_bytes": len(before), "prospective_bytes": len(after), "actual_bytes": len(observed), "before_sha256": wp25TrackerClientHash(before), "prospective_sha256": wp25TrackerClientHash(after), "actual_sha256": wp25TrackerClientHash(observed), "before_asset": wp25TrackerClientAsset(t, fmt.Sprintf("journal-before-%t.json", over), before), "prospective_asset": wp25TrackerClientAsset(t, fmt.Sprintf("journal-prospective-%t.json", over), after), "actual_asset": wp25TrackerClientAsset(t, fmt.Sprintf("journal-actual-%t.json", over), observed), "expected_exit_cause": want, "scope": "private writer equality reaches independent unavailableaction refusal after tiny identity binding; first-over fails active save with resource_limit before action; no native mutation/publication"})
		}
	})
	t.Run("E04-client-report-writer-sixteen-MiB", func(t *testing.T) {
		for _, over := range []bool{false, true} {
			f := wp25TrackerClientNew(t, consumer, "http://127.0.0.1:1", map[string]any{"action": "observe", "limit": 20})
			identityData := f.identity["data"].(map[string]any)
			identityData["name"] = ""
			identityData["name"] = strings.Repeat("n", 8388500-len(wp25TrackerClientJSON(t, f.identity)))
			firstResponse := wp25TrackerClientJSON(t, f.identity)
			if len(firstResponse) != 8388500 || len(firstResponse) >= 8388608 {
				t.Fatal("identity frame must fit actual8MiB cap")
			}
			selection := map[string]any{"families": []string{"issue"}, "types": []string{}}
			params := map[string]any{"selection": selection, "limit": 20}
			actor := identityData["actor"].(string)
			id := "sha256:" + strings.Repeat("b", 64)
			comments := make([]any, 128)
			for i := range comments {
				body := ""
				if i < 127 {
					body = strings.Repeat("c", 65536)
				}
				comments[i] = map[string]any{"id": fmt.Sprintf("sha256:%064x", i+100), "actor": actor, "body": body}
			}
			view := map[string]any{"protocol": "hn/0", "family": "issue", "id": id, "creator": actor, "heads": []any{}, "conflict": false, "state": nativeIssueState("Independently framed view"), "comments": comments}
			entry := map[string]any{"protocol": "hn/0", "family": "issue", "type_id": nil, "resource_id": id, "revision_ids": []string{}, "value": view}
			page := map[string]any{"capture": "capture", "snapshot_id": id, "selection": selection, "selection_digest": id, "source_cut": id, "mode": "replace", "items": []any{entry}, "next_cursor": nil, "complete": true, "checkpoint": "checkpoint", "expires_at": "2030-01-01T00:00:00Z"}
			second := map[string]any{"protocol": "hn.extension/1", "request_id": "tracker-2", "ok": true, "data": page, "error": nil}
			calls := []map[string]any{{"method": "identity.public", "params": map[string]any{}, "response": f.identity}, {"method": "changes.capture", "params": params, "response": second}}
			wanted := 16777216
			if over {
				wanted++
			}
			padding := wanted - len(wp25TrackerClientReport(t, f.hello, calls, ""))
			if padding < 0 || padding > 65536 {
				t.Fatalf("report padding must fit independently bounded lastcomment: %d", padding)
			}
			comments[127].(map[string]any)["body"] = strings.Repeat("c", padding)
			secondResponse := wp25TrackerClientJSON(t, second)
			prospective := wp25TrackerClientReport(t, f.hello, calls, "")
			prior := wp25TrackerClientReport(t, f.hello, calls[:1], "")
			if len(prospective) != wanted || len(secondResponse) >= 8388608 {
				t.Fatalf("report target%d want%d, secondframe%d", len(prospective), wanted, len(secondResponse))
			}
			beforeJournal := wp25Read(t, f.journal)
			incoming := append(protocolRawFrame(wp25TrackerClientJSON(t, f.hello)), protocolRawFrame(firstResponse)...)
			incoming = append(incoming, protocolRawFrame(secondResponse)...)
			want := ""
			if over {
				want = "resource_limit"
			}
			wire := f.run(t, incoming, want)
			observed := wp25Read(t, f.report)
			if len(wire) != 3 || wire[1]["method"] != "identity.public" || wire[2]["method"] != "changes.capture" {
				t.Fatal("report threshold did not use exact two actual client calls", wire)
			}
			if over {
				if !bytes.Equal(prior, observed) {
					t.Fatal("active report first-over replaced last complete report")
				}
				if !bytes.Equal(beforeJournal, wp25Read(t, f.journal)) {
					t.Fatal("report first-over advanced private checkpoint before response validation")
				}
			} else {
				if !bytes.Equal(prospective, observed) {
					t.Fatal("active report equality did not persist independent exact16MiB report")
				}
				journal := wp25TrackerClientCopyJSON(t, wp25Read(t, f.journal))
				if journal["checkpoint"] != "checkpoint" || len(journal["entries"].(map[string]any)) != 1 {
					t.Fatal("at-bound valid observed page did not complete private selection", journal["checkpoint"])
				}
			}
			wp25Record(t, map[string]any{"control": "tracker-client-report-writer", "first_over": over, "first_frame_bytes": len(firstResponse), "second_frame_bytes": len(secondResponse), "prospective_report_bytes": len(prospective), "prospective_sha256": wp25TrackerClientHash(prospective), "actual_report_bytes": len(observed), "actual_sha256": wp25TrackerClientHash(observed), "prior_report_sha256": wp25TrackerClientHash(prior), "incoming_frames_asset": wp25TrackerClientAsset(t, fmt.Sprintf("report-incoming-%t.frames", over), incoming), "prospective_report_asset": wp25TrackerClientAsset(t, fmt.Sprintf("report-prospective-%t.json", over), prospective), "actual_report_asset": wp25TrackerClientAsset(t, fmt.Sprintf("report-actual-%t.json", over), observed), "scope": "active call report write before typed response validation; both individual frames below8MiB; at-cap normal valid page completes; final deferred status-save may exceed cap and must preserve active exact-cap report, not truncate; first-over activecall is resource_limit and preserves previous complete file/checkpoint"})
		}
	})
}

type wp25TrackerClientFixture struct {
	consumer, dir, config, plan, journal, report string
	hello, identity, j                           map[string]any
}

func wp25TrackerClientJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func wp25TrackerClientCopy(t *testing.T, v map[string]any) map[string]any {
	return wp25TrackerClientCopyJSON(t, wp25TrackerClientJSON(t, v))
}
func wp25TrackerClientCopyJSON(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func wp25TrackerClientHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", sum)
}
func wp25TrackerClientOrdered(t *testing.T, keys []string, v map[string]any) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(wp25TrackerClientJSON(t, k))
		b.WriteByte(':')
		b.Write(wp25TrackerClientJSON(t, v[k]))
	}
	b.WriteByte('}')
	return b.Bytes()
}
func wp25TrackerClientJournal(t *testing.T, j map[string]any) []byte {
	return wp25TrackerClientOrdered(t, []string{"repository", "module", "registration", "connection", "source", "api", "credential", "actor", "public_key", "native", "external", "external_version", "initial_item", "native_head", "baseline", "entries", "selection", "snapshot", "checkpoint"}, j)
}
func wp25TrackerClientReport(t *testing.T, hello map[string]any, calls []map[string]any, status string) []byte {
	var b bytes.Buffer
	b.WriteString(`{"hello":`)
	b.Write(wp25TrackerClientJSON(t, hello))
	b.WriteString(`,"calls":[`)
	for i, c := range calls {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(wp25TrackerClientOrdered(t, []string{"method", "params", "response"}, c))
	}
	b.WriteString(`],"status":`)
	b.Write(wp25TrackerClientJSON(t, status))
	b.WriteByte('}')
	return b.Bytes()
}
func wp25TrackerClientAsset(t *testing.T, name string, raw []byte) string {
	t.Helper()
	root := os.Getenv("HN_WP25_EVIDENCE")
	if root == "" {
		root = t.TempDir()
	}
	dir := filepath.Join(root, "source-assistance", "tracker-client-limits")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func wp25TrackerClientNew(t *testing.T, consumer, api string, plan map[string]any) *wp25TrackerClientFixture {
	t.Helper()
	dir := t.TempDir()
	f := &wp25TrackerClientFixture{consumer: consumer, dir: dir, config: filepath.Join(dir, "config.json"), plan: filepath.Join(dir, "plan.json"), journal: filepath.Join(dir, "journal.json"), report: filepath.Join(dir, "report.json")}
	id := "sha256:" + strings.Repeat("a", 64)
	pub := make([]byte, 32)
	actorHash := sha256.Sum256(pub)
	actor := hex.EncodeToString(actorHash[:])
	key := base64.RawStdEncoding.EncodeToString(pub)
	f.hello = map[string]any{"protocol": "hn.extension/1", "hello": map[string]any{"session_id": strings.Repeat("1", 32), "repository_id": id, "module_id": "example.tracker/tool", "module_version": "1", "registration_id": id}, "limits": map[string]any{"frame_bytes": 8388608, "read_timeout_ms": 30000, "mutation_timeout_ms": 240000, "page_max": 200, "one_outstanding": true}}
	f.identity = map[string]any{"protocol": "hn.extension/1", "request_id": "tracker-1", "ok": true, "data": map[string]any{"initialized": true, "actor": actor, "name": "WP25 scripted host identity", "public_key": key}, "error": nil}
	f.j = map[string]any{"repository": id, "module": "example.tracker/tool", "registration": id, "connection": "client-boundary", "source": "tracker:item:42", "api": api, "credential": "WP25-client-fixture-token", "actor": actor, "public_key": key, "native": map[string]any{}, "external": map[string]any{}, "external_version": 0, "initial_item": nil, "native_head": "", "baseline": map[string]any{}, "entries": map[string]any{}, "selection": "", "snapshot": "", "checkpoint": ""}
	config := map[string]any{"api": api, "token": "WP25-client-fixture-token", "connection": "client-boundary", "source": "tracker:item:42", "journal": f.journal, "report": f.report, "marker": ""}
	for path, raw := range map[string][]byte{f.config: wp25TrackerClientJSON(t, config), f.plan: wp25TrackerClientJSON(t, plan), f.journal: wp25TrackerClientJournal(t, f.j), f.report: []byte("PRIVATE-UNCHANGED-REPORT")} {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return f
}
func (f *wp25TrackerClientFixture) run(t *testing.T, incoming []byte, want string) []map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.consumer, "--config", f.config, "--plan", f.plan)
	cmd.Dir, cmd.Env, cmd.Stdin = f.dir, operationalTestEnvironment(f.dir), bytes.NewReader(incoming)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	started := time.Now()
	runErr := cmd.Run()
	wire := []map[string]any{}
	reader := bytes.NewReader(stdout.Bytes())
	for reader.Len() > 0 {
		var p [4]byte
		if _, err := io.ReadFull(reader, p[:]); err != nil {
			t.Fatal("partial actual client prefix", err)
		}
		n := binary.BigEndian.Uint32(p[:])
		if n == 0 || n > 8388608 {
			t.Fatal("invalid actual client frame length", n)
		}
		raw := make([]byte, n)
		if _, err := io.ReadFull(reader, raw); err != nil {
			t.Fatal("partial actual client frame", err)
		}
		wire = append(wire, wp25TrackerClientCopyJSON(t, raw))
	}
	wp25Record(t, map[string]any{"control": "supplied-tracker-client-run", "consumer": f.consumer, "consumer_sha256": wp25Digest(t, f.consumer), "argv": cmd.Args[1:], "config_raw": string(wp25Read(t, f.config)), "plan_raw": string(wp25Read(t, f.plan)), "incoming_frames_bytes": len(incoming), "incoming_frames_sha256": wp25TrackerClientHash(incoming), "incoming_asset": wp25TrackerClientAsset(t, fmt.Sprintf("incoming-%d.frames", started.UnixNano()), incoming), "stdout_raw": string(stdout.Bytes()), "stderr_raw": stderr.String(), "process_error": fmt.Sprint(runErr), "watchdog_error": fmt.Sprint(ctx.Err()), "elapsed_ns": time.Since(started).Nanoseconds(), "expected_cause": want, "scope": "scripted independent host frames; supplied standalone consumer boundary, no actual host publication"})
	if ctx.Err() != nil {
		t.Fatal("supplied tracker client watchdog", ctx.Err())
	}
	if want == "" {
		if runErr != nil || stderr.Len() != 0 {
			t.Fatalf("valid actual client control: %v %s", runErr, &stderr)
		}
	} else if runErr == nil || stderr.String() != "fake tracker stopped: "+want+"\n" {
		t.Fatalf("actual client expected exact %s: %v %s", want, runErr, &stderr)
	}
	return wire
}

func wp25TrackerClientPagePlans(t *testing.T, consumer string) {
	for _, limit := range []int{0, 1, 200, 201} {
		f := wp25TrackerClientNew(t, consumer, "http://127.0.0.1:1", map[string]any{"action": "observe", "limit": limit})
		refusal := map[string]any{"protocol": "hn.extension/1", "request_id": nil, "ok": false, "data": nil, "error": map[string]any{"origin": "host", "code": "denied_access", "message": "independent consumer page-plan control", "recovery": "do_not_retry", "operation": nil}}
		incoming := append(protocolRawFrame(wp25TrackerClientJSON(t, f.hello)), protocolRawFrame(wp25TrackerClientJSON(t, f.identity))...)
		incoming = append(incoming, protocolRawFrame(wp25TrackerClientJSON(t, refusal))...)
		before := wp25Read(t, f.journal)
		want := "denied_access"
		if limit == 201 {
			want = "invalid_input"
		}
		wire := f.run(t, incoming, want)
		if limit == 201 {
			if len(wire) != 2 {
				t.Fatal("consumer over-limit plan reached changes dispatch", wire)
			}
		} else {
			expected := limit
			if expected == 0 {
				expected = 20
			}
			if len(wire) != 3 || wire[2]["method"] != "changes.capture" || wire[2]["params"].(map[string]any)["limit"] != float64(expected) {
				t.Fatal("actual consumer page-plan limit/default", limit, wire)
			}
		}
		if !bytes.Equal(before, wp25Read(t, f.journal)) {
			t.Fatal("refused/denied consumer plan advanced private checkpoint")
		}
		wp25Record(t, map[string]any{"control": "tracker-client-observe-plan", "plan_limit": limit, "expected_cause": want, "actual_frames": wire, "journal_unchanged": true, "scope": "0 selects default20;1/200 reach actual changes.capture request then independent known host refusal;201 refuses before changes request. No claim host returned200 entries in this control."})
	}
}

// All construction below is SOURCE-ASSISTED standard Ed25519 + Git fast-import.
// Only wp25PublicCountCall executes the independently supplied candidate.
type wp25PublicCountFixture struct {
	t                            *testing.T
	binary, repo, root, evidence string
	env                          []string
}

func wp25PublicCountNew(t *testing.T, binary, scope string) *wp25PublicCountFixture {
	t.Helper()
	if binary == "" {
		t.Fatal("public count control requires supplied candidate binary")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	home := filepath.Join(root, "home")
	for _, p := range []string{repo, home} {
		if e := os.MkdirAll(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	config := wp25FixtureConfig(t, home)
	git, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "path")
	if e = os.Mkdir(path, 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(git, filepath.Join(path, "git")); e != nil {
		t.Fatal(e)
	}
	// No executable source/build fallback: a Go attempt emits a recognizable failure.
	if e = os.WriteFile(filepath.Join(path, "go"), []byte("#!/bin/sh\necho WP25_SOURCE_FALLBACK_FORBIDDEN >&2\nexit 97\n"), 0700); e != nil {
		t.Fatal(e)
	}
	env := []string{"PATH=" + path, "HOME=" + home, "XDG_CONFIG_HOME=" + config, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "LC_ALL=C"}
	evidence := os.Getenv("HN_WP25_EVIDENCE")
	if evidence == "" {
		evidence = t.TempDir()
	}
	evidence = filepath.Join(evidence, "source-assistance", scope)
	if e = os.MkdirAll(evidence, 0700); e != nil {
		t.Fatal(e)
	}
	f := &wp25PublicCountFixture{t, binary, repo, root, evidence, env}
	f.git(nil, "init", "-q", "--initial-branch=main")
	f.git(nil, "-c", "user.name=WP25 public fixture", "-c", "user.email=fixture@hn.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture", "--allow-empty")
	return f
}
func (f *wp25PublicCountFixture) git(input io.Reader, args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env, cmd.Stdin = f.repo, f.env, input
	out, e := cmd.CombinedOutput()
	if e != nil {
		f.t.Fatalf("SOURCE-ASSISTED git %q: %v %s", args, e, out)
	}
	return strings.TrimSpace(string(out))
}
func (f *wp25PublicCountFixture) snapshot() map[string]string {
	f.t.Helper()
	out := map[string]string{}
	e := filepath.Walk(f.root, func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if i.IsDir() {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		out[strings.TrimPrefix(p, f.root)] = wp25NativeDigest(b)
		return nil
	})
	if e != nil {
		f.t.Fatal(e)
	}
	return out
}
func wp25PublicCountCall(t *testing.T, f *wp25PublicCountFixture, scope string, readOnly, ok bool, budget time.Duration, args ...string) []byte {
	t.Helper()
	before := f.snapshot()
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.binary, args...)
	cmd.Dir, cmd.Env = f.repo, f.env
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	start := time.Now()
	err := cmd.Run()
	after := f.snapshot()
	wp25Record(t, map[string]any{"control": scope, "candidate_observation": true, "source_assisted": false, "binary": f.binary, "binary_sha256": wp25Digest(t, f.binary), "cwd": f.repo, "argv": args, "stdout_raw": out.String(), "stderr_raw": stderr.String(), "process_error": fmt.Sprint(err), "watchdog_error": fmt.Sprint(ctx.Err()), "watchdog_nanoseconds": int64(budget), "elapsed_nanoseconds": int64(time.Since(start)), "protected_before_sha256": wp25NativeSnapshotDigest(before), "protected_after_sha256": wp25NativeSnapshotDigest(after), "protected_before": before, "protected_after": after, "configured_budget_isolation": strings.HasPrefix(scope, "contribution-graph-")})
	if ctx.Err() != nil || (err == nil) != ok {
		t.Fatalf("%s candidate %q: %v watchdog=%v\n%s\n%s", scope, args, err, ctx.Err(), &out, &stderr)
	}
	if readOnly && !reflect.DeepEqual(before, after) {
		t.Fatalf("%s changed protected repository/config/key bytes", scope)
	}
	return bytes.Clone(out.Bytes())
}

// The canonical native event reader accepts legacy hn/0 issue.open records. This
// independent wire mirror includes only their documented fields, not Event or
// production signer/ID helpers. Every record has an independently checked ID,
// public signature, sequence and exact previous-ID link in an archived ledger.
type wp25PublicCountEvent struct {
	Protocol  string `json:"protocol"`
	Kind      string `json:"kind"`
	Actor     string `json:"actor"`
	ActorName string `json:"actorName"`
	PublicKey string `json:"publicKey"`
	Sequence  uint64 `json:"sequence"`
	Timestamp string `json:"timestamp"`
	Previous  string `json:"previous,omitempty"`
	Title     string `json:"title"`
}

func wp25PublicNativeCorpus(t *testing.T, f *wp25PublicCountFixture, n int) (actor, first, at, over, terminal string) {
	t.Helper()
	seed := sha256.Sum256([]byte("WP25 public native 100000 event corpus"))
	key := ed25519.NewKeyFromSeed(seed[:])
	public := key.Public().(ed25519.PublicKey)
	a := sha256.Sum256(public)
	actor = hex.EncodeToString(a[:])
	ref := "refs/hn/actors/" + actor
	wirePath := filepath.Join(f.evidence, "public-events.fast-import")
	wire, e := os.Create(wirePath)
	if e != nil {
		t.Fatal(e)
	}
	ledgerPath := filepath.Join(f.evidence, "public-events.jsonl")
	ledger, e := os.Create(ledgerPath)
	if e != nil {
		t.Fatal(e)
	}
	w := bufio.NewWriterSize(wire, 1<<20)
	l := bufio.NewWriterSize(ledger, 1<<20)
	previous := ""
	seen := map[string]bool{}
	aggregate := int64(0)
	for i := 1; i <= n; i++ {
		raw, e := json.Marshal(wp25PublicCountEvent{"hn/0", "issue.open", actor, "WP25 public count", base64.RawStdEncoding.EncodeToString(public), uint64(i), "2026-10-31T08:00:00Z", previous, fmt.Sprintf("Public count %06d", i)})
		if e != nil {
			t.Fatal(e)
		}
		digest := sha256.Sum256(raw)
		id := fmt.Sprintf("sha256:%x", digest)
		signature := ed25519.Sign(key, raw)
		encoded := base64.RawStdEncoding.EncodeToString(signature)
		if seen[id] || !ed25519.Verify(public, raw, signature) || len(raw) > 8<<20 {
			t.Fatal("independent native corpus invariant")
		}
		seen[id] = true
		aggregate += int64(len(raw) + len(encoded))
		if i == 1 {
			first = id
		}
		if i == min(n, 100000) {
			terminal = id
		}
		row, _ := json.Marshal(map[string]any{"mark": i, "event_id": id, "payload_raw": string(raw), "signature_base64": encoded, "payload_bytes": len(raw), "selected_sequence": i})
		if _, e = l.Write(append(row, '\n')); e != nil {
			t.Fatal(e)
		}
		msg := "hn event " + id
		fmt.Fprintf(w, "commit %s\nmark :%d\ncommitter WP25 public fixture <fixture@hn.invalid> 1793433600 +0000\ndata %d\n%s\n", ref, i, len(msg), msg)
		if i > 1 {
			fmt.Fprintf(w, "from :%d\n", i-1)
		}
		fmt.Fprintf(w, "deleteall\nM 100644 inline event.json\ndata %d\n", len(raw))
		w.Write(raw)
		fmt.Fprintf(w, "\nM 100644 inline signature\ndata %d\n%s\n\n", len(encoded), encoded)
		previous = id
	}
	if aggregate >= 256<<20 {
		t.Fatalf("native count isolated aggregate=%d", aggregate)
	}
	if e = w.Flush(); e != nil {
		t.Fatal(e)
	}
	if e = l.Flush(); e != nil {
		t.Fatal(e)
	}
	if e = wire.Close(); e != nil {
		t.Fatal(e)
	}
	if e = ledger.Close(); e != nil {
		t.Fatal(e)
	}
	input, e := os.Open(wirePath)
	if e != nil {
		t.Fatal(e)
	}
	marksPath := filepath.Join(f.evidence, "git-marks.txt")
	f.git(input, "fast-import", "--quiet", "--export-marks="+marksPath)
	input.Close()
	raw, e := os.ReadFile(marksPath)
	if e != nil {
		t.Fatal(e)
	}
	marks := map[int]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			t.Fatal("invalid native fixture mark", line)
		}
		i, e := strconv.Atoi(strings.TrimPrefix(parts[0], ":"))
		if e != nil || marks[i] != "" {
			t.Fatal("duplicate/invalid native mark", line)
		}
		marks[i] = parts[1]
	}
	if len(marks) != n || len(seen) != n {
		t.Fatal("incomplete independent native fixture map")
	}
	at = marks[min(n, 100000)]
	over = marks[n]
	wp25Record(t, map[string]any{"control": "source-assisted-native-count-corpus", "source_assisted": true, "candidate_observation": false, "records": n, "unique_ids": len(seen), "aggregate_payload_signature_bytes": aggregate, "actor": actor, "first_event_id": first, "terminal_at_event_id": terminal, "at_tip": at, "over_tip": over, "wire_path": wirePath, "wire_sha256": wp25Digest(t, wirePath), "ledger_path": ledgerPath, "ledger_sha256": wp25Digest(t, ledgerPath), "marks_path": marksPath, "marks_sha256": wp25Digest(t, marksPath)})
	return
}
func wp25PublicNativeResult(t *testing.T, raw []byte, id, title, code, message string) {
	t.Helper()
	var v struct {
		Schema string `json:"schema"`
		OK     bool   `json:"ok"`
		Data   struct {
			ID        string   `json:"id"`
			HeadCount int      `json:"head_count"`
			HeadIDs   []string `json:"head_ids"`
			State     *struct {
				Title string `json:"title"`
			} `json:"state"`
		} `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &v) != nil || v.Schema != "hn.issue/1" {
		t.Fatalf("native issue envelope %s", raw)
	}
	if code != "" {
		var shape map[string]json.RawMessage
		if json.Unmarshal(raw, &shape) != nil {
			t.Fatal("native refusal envelope is not an object")
		}
		if data, exists := shape["data"]; exists && string(data) != "null" {
			t.Fatal("native refusal returned success data", string(data))
		}
		if v.OK || v.Error == nil || v.Error.Code != code || v.Error.Message != message {
			t.Fatalf("native exact refusal %s", raw)
		}
		return
	}
	if !v.OK || v.Error != nil || v.Data.ID != id || v.Data.HeadCount != 1 || !reflect.DeepEqual(v.Data.HeadIDs, []string{id}) || v.Data.State == nil || v.Data.State.Title != title {
		t.Fatalf("native independently expected signed issue %s", raw)
	}
}
func wp25PublicNativeRefLimit(t *testing.T, binary string) {
	f := wp25PublicCountNew(t, binary, "native-public-refs")
	actor, first, tip, _, _ := wp25PublicNativeCorpus(t, f, 1)
	selectRefs := func(n int) {
		rows := []string{tip + " refs/hn/actors/" + actor}
		for i := 1; i < n; i++ {
			rows = append(rows, fmt.Sprintf("%s refs/hn/remotes/r%06d/actors/%s", tip, i, actor))
		}
		sort.Strings(rows)
		raw := []byte("# pack-refs with: peeled fully-peeled sorted\n" + strings.Join(rows, "\n") + "\n")
		if e := os.WriteFile(filepath.Join(f.repo, ".git", "packed-refs"), raw, 0600); e != nil {
			t.Fatal(e)
		}
		actual := f.git(nil, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors", "refs/hn/remotes")
		if len(strings.Split(actual, "\n")) != n {
			t.Fatal("actual public ref selection count")
		}
		wp25Record(t, map[string]any{"control": "source-assisted-native-ref-selection", "source_assisted": true, "candidate_observation": false, "refs": n, "complete_refs_raw": actual, "complete_refs_sha256": wp25NativeDigest([]byte(actual)), "packed_refs_sha256": wp25NativeDigest(raw), "single_authenticated_tip": tip})
	}
	for _, n := range []int{100000, 100001, 100000} {
		selectRefs(n)
		raw := wp25PublicCountCall(t, f, fmt.Sprintf("native-public-refs-%d", n), true, n == 100000, 15*time.Minute, "issue", "show", first, "--json")
		if n == 100000 {
			wp25PublicNativeResult(t, raw, first, "Public count 000001", "", "")
		} else {
			wp25PublicNativeResult(t, raw, first, "Public count 000001", "resource_limit", "resource_limit: issue ref enumeration exceeds 100000 entries")
		}
	}
}
func wp25PublicNativeEventLimit(t *testing.T, binary string) {
	f := wp25PublicCountNew(t, binary, "native-public-events")
	actor, first, at, over, terminal := wp25PublicNativeCorpus(t, f, 100001)
	ref := "refs/hn/actors/" + actor
	for index, step := range []struct {
		n   int
		tip string
	}{{100000, at}, {100001, over}, {100000, at}} {
		f.git(nil, "update-ref", ref, step.tip)
		actual := f.git(nil, "rev-list", "--count", step.tip)
		if actual != strconv.Itoa(step.n) {
			t.Fatal("actual selected native event closure", actual, step.n)
		}
		wp25Record(t, map[string]any{"control": "source-assisted-native-event-selection", "source_assisted": true, "candidate_observation": false, "records": step.n, "tip": step.tip, "actual_git_selected_commit_count": actual, "terminal_at_event_id": terminal, "first_event_id": first})
		query, title := first, "Public count 000001"
		if index == 2 {
			query, title = terminal, "Public count 100000"
		}
		raw := wp25PublicCountCall(t, f, fmt.Sprintf("native-public-events-%d-step-%d", step.n, index), true, step.n == 100000, 45*time.Minute, "issue", "show", query, "--json")
		if step.n == 100001 {
			wp25PublicNativeResult(t, raw, query, title, "resource_limit", "resource_limit: issue history exceeds 100000 events")
			continue
		}
		wp25PublicNativeResult(t, raw, query, title, "", "")
		if index == 0 {
			// Query the independent signed terminal ID as well as the opening ID.
			// Its sequence100000 and exact previous-ID chain prevent an early prefix
			// truncation from masquerading as a successful at-cap public read.
			last := wp25PublicCountCall(t, f, "native-public-events-100000-terminal", true, true, 45*time.Minute, "issue", "show", terminal, "--json")
			wp25PublicNativeResult(t, last, terminal, "Public count 100000", "", "")
		}
	}
}

func wp25InheritedPublicCountLimits(t *testing.T, binary string) {
	t.Run("native-refs100000", func(t *testing.T) { wp25PublicNativeRefLimit(t, binary) })
	t.Run("native-events100000", func(t *testing.T) { wp25PublicNativeEventLimit(t, binary) })
	t.Run("contribution-graph16384", func(t *testing.T) { wp25PublicContributionGraphLimit(t, binary) })
}

func TestExtensionPackageInheritedPublicCountLimits(t *testing.T) {
	wp25InheritedPublicCountLimits(t, extensionTestBinary(t))
}

// SOURCE-ASSISTED construction uses literal independent contribution wire
// mirrors and standard Ed25519; no c.Sign/c.Verify/c.Reduce or host helper can
// supply the actual public caller's graph result. Independent moderator keys
// remain in fixture memory; the legitimate maintainer keyring is created by the
// supplied public init command and is protected throughout threshold reads.
// Native-CLI configured-budget isolation, not a default-
// budget/extension-host throughput observation: only milliseconds=3600000 is
// changed in this test's explicit committed policy. All other limits are literal
// defaults and remain unchanged in both the product and this policy.
type wp25PublicGraphAuthority struct {
	Commit string `json:"commit"`
	Digest string `json:"digest"`
}
type wp25PublicGraphTarget struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}
type wp25PublicGraphDecision struct {
	Action    string                   `json:"action"`
	Target    wp25PublicGraphTarget    `json:"target"`
	Authority wp25PublicGraphAuthority `json:"authority"`
	Parents   []string                 `json:"parents"`
}

// Field order is the hn.contribution/1 canonical wire order, independently
// mirrored here; omitted submission/receipt members are absent in decisions.
type wp25PublicGraphRecord struct {
	Version   string                   `json:"version"`
	Kind      string                   `json:"kind"`
	Project   string                   `json:"project"`
	Actor     string                   `json:"actor"`
	PublicKey string                   `json:"publicKey"`
	Intent    string                   `json:"intent"`
	Decision  *wp25PublicGraphDecision `json:"decision,omitempty"`
}
type wp25PublicGraphManifest struct {
	Schema  string   `json:"schema"`
	Project string   `json:"project"`
	Actor   string   `json:"actor"`
	Records []string `json:"records"`
}
type wp25PublicGraphSigner struct {
	actor  string
	public ed25519.PublicKey
	key    ed25519.PrivateKey
}
type wp25PublicGraphHead struct {
	ref, tip string
	expected map[string]string
}

func wp25PublicGraphManifestBuild(t *testing.T, f *wp25PublicCountFixture, project string, authority wp25PublicGraphAuthority, signer wp25PublicGraphSigner, start, n int, label string) wp25PublicGraphHead {
	t.Helper()
	ref := "refs/hn/contributions/" + project[7:] + "/" + signer.actor
	type material struct {
		id       string
		raw, sig []byte
	}
	records := []material{}
	expected := map[string]string{}
	ids := []string{}
	ledgerPath := filepath.Join(f.evidence, label+".jsonl")
	ledger, e := os.Create(ledgerPath)
	if e != nil {
		t.Fatal(e)
	}
	l := bufio.NewWriter(ledger)
	for i := start; i < start+n; i++ {
		targetSum := sha256.Sum256([]byte(fmt.Sprintf("WP25 graph independent target %06d", i)))
		target := hex.EncodeToString(targetSum[:])
		if target == signer.actor {
			t.Fatal("target equals signing moderator")
		}
		record := wp25PublicGraphRecord{"hn.contribution/1", "decision", project, signer.actor, base64.RawStdEncoding.EncodeToString(signer.public), fmt.Sprintf("wp25-graph-block-%06d", i), &wp25PublicGraphDecision{"block", wp25PublicGraphTarget{"actor", target}, authority, []string{}}}
		raw, e := json.Marshal(record)
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(raw)
		id := fmt.Sprintf("sha256:%x", sum)
		sig := ed25519.Sign(signer.key, raw)
		// Independent closed-data and cryptographic construction controls; the real
		// candidate must repeat its own strict canonical/semantic/authentication work.
		var decoded wp25PublicGraphRecord
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if e = decoder.Decode(&decoded); e != nil {
			t.Fatal(e)
		}
		canonical, e := json.Marshal(decoded)
		if e != nil || !bytes.Equal(canonical, raw) || !ed25519.Verify(signer.public, raw, sig) || len(raw) > 65536 || len(sig) != 64 || decoded.Decision.Authority != authority || decoded.Decision.Target.Kind != "actor" || decoded.Decision.Action != "block" || decoded.Decision.Parents == nil || len(decoded.Decision.Parents) != 0 || expected[target] != "" {
			t.Fatal("independent graph payload/signature/semantic invariant")
		}
		expected[target] = id
		ids = append(ids, id)
		records = append(records, material{id, raw, sig})
		row, _ := json.Marshal(map[string]any{"record_id": id, "payload_raw": string(raw), "signature_base64": base64.RawStdEncoding.EncodeToString(sig), "target": target, "actor": signer.actor, "authority": authority})
		if _, e = l.Write(append(row, '\n')); e != nil {
			t.Fatal(e)
		}
	}
	sort.Strings(ids)
	sort.Slice(records, func(i, j int) bool { return records[i].id < records[j].id })
	manifest := wp25PublicGraphManifest{"hn.contribution-manifest/1", project, signer.actor, ids}
	manifestRaw, e := json.Marshal(manifest)
	if e != nil {
		t.Fatal(e)
	}
	wirePath := filepath.Join(f.evidence, label+".fast-import")
	wire, e := os.Create(wirePath)
	if e != nil {
		t.Fatal(e)
	}
	w := bufio.NewWriterSize(wire, 1<<20)
	zeros := strings.Repeat("0", 40)
	if f.git(nil, "rev-parse", "--show-object-format") == "sha256" {
		zeros = strings.Repeat("0", 64)
	}
	fmt.Fprintf(w, "reset %s\nfrom %s\n\ncommit %s\ncommitter WP25 public graph <fixture@hn.invalid> 1793433600 +0000\ndata 23\nWP25 public graph block\ndeleteall\n", ref, zeros, ref)
	file := func(name string, raw []byte) {
		fmt.Fprintf(w, "M 100644 inline %s\ndata %d\n", name, len(raw))
		w.Write(raw)
		w.WriteByte('\n')
	}
	file("manifest.json", manifestRaw)
	for _, r := range records {
		file(r.id[7:]+".payload", r.raw)
		file(r.id[7:]+".signature", r.sig)
	}
	w.WriteByte('\n')
	if e = w.Flush(); e != nil {
		t.Fatal(e)
	}
	if e = wire.Close(); e != nil {
		t.Fatal(e)
	}
	if e = l.Flush(); e != nil {
		t.Fatal(e)
	}
	if e = ledger.Close(); e != nil {
		t.Fatal(e)
	}
	input, e := os.Open(wirePath)
	if e != nil {
		t.Fatal(e)
	}
	f.git(input, "fast-import", "--quiet")
	input.Close()
	tip := f.git(nil, "rev-parse", ref)
	// Full actual Git closure map and exact raw manifest are retained; expected
	// record selection does not come from the candidate's graph or limits.
	closure := f.git(nil, "rev-list", "--objects", "--no-object-names", tip)
	objects := strings.Split(closure, "\n")
	if len(objects) != 2*n+3 || len(objects) > 4096 {
		t.Fatalf("independently expected manifest objects got%d want%d", len(objects), 2*n+3)
	}
	actualManifest := f.git(nil, "show", tip+":manifest.json")
	if actualManifest != string(manifestRaw) {
		t.Fatal("actual raw Git manifest differs")
	}
	wp25Record(t, map[string]any{"control": "source-assisted-contribution-manifest", "source_assisted": true, "candidate_observation": false, "ref": ref, "tip": tip, "records": n, "record_ids": ids, "manifest_raw": string(manifestRaw), "manifest_sha256": wp25NativeDigest(manifestRaw), "complete_object_ids_raw": closure, "complete_object_ids_sha256": wp25NativeDigest([]byte(closure)), "objects": len(objects), "wire_path": wirePath, "wire_sha256": wp25Digest(t, wirePath), "ledger_path": ledgerPath, "ledger_sha256": wp25Digest(t, ledgerPath)})
	return wp25PublicGraphHead{ref, tip, expected}
}

// Retention is real public setup, with an independently encoded exact plan
// and receipt. Empty native roots and two Git roots are deliberate: actor-block
// decisions have no native dependencies and every manifest has no commit parent.
func wp25PublicGraphRetain(t *testing.T, f *wp25PublicCountFixture, authority wp25PublicGraphAuthority, project string, head wp25PublicGraphHead, label string) {
	t.Helper()
	roots := []string{authority.Commit, head.tip}
	sort.Strings(roots)
	plan := struct {
		Version   string                   `json:"version"`
		Project   string                   `json:"project"`
		Authority wp25PublicGraphAuthority `json:"authority"`
		Native    []string                 `json:"native"`
		Heads     []string                 `json:"heads"`
		Roots     []string                 `json:"roots"`
	}{"hn.contribution-retained-plan/1", project, authority, nil, []string{head.tip}, roots}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	expected := wp25NativeDigest(raw)
	receipt := wp25PublicCountCall(t, f, "contribution-graph-setup-retain-"+label, false, true, 62*time.Minute, "contribution", "retained", "register", "--expect", expected)
	var got struct {
		Registered string   `json:"registered"`
		Native     int      `json:"native"`
		Heads      []string `json:"heads"`
	}
	decoder := json.NewDecoder(bytes.NewReader(receipt))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&got); err != nil || got.Registered != expected || got.Native != 0 || !reflect.DeepEqual(got.Heads, []string{head.tip}) {
		t.Fatalf("actual retained receipt differs from independent exact plan: %v %s", err, receipt)
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		t.Fatal("retained receipt has trailing output")
	}
	wp25Record(t, map[string]any{"control": "contribution-graph-genuine-retention-receipt", "setup_only": true, "label": label, "independent_plan_raw": string(raw), "independent_plan_digest": expected, "actual_receipt_raw": string(receipt), "registered_head": head.tip, "registered_record_count": len(head.expected), "native_fact_count": 0})
}

func wp25PublicGraphPending(t *testing.T, f *wp25PublicCountFixture, authority wp25PublicGraphAuthority, project string, expected map[string]string, over bool) {
	t.Helper()
	before := f.snapshot()
	ctx, cancel := context.WithTimeout(context.Background(), 62*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.binary, "contribution", "pending")
	cmd.Dir, cmd.Env = f.repo, f.env
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	start := time.Now()
	err := cmd.Run()
	after := f.snapshot()
	stage := "actual Reduce complete view"
	if over {
		stage = "contributionRecords aggregate loader before Reduce"
	}
	wp25Record(t, map[string]any{"control": fmt.Sprintf("contribution-graph-%d", len(expected)), "candidate_observation": true, "source_assisted": false, "binary": f.binary, "binary_sha256": wp25Digest(t, f.binary), "cwd": f.repo, "argv": []string{"contribution", "pending"}, "stdout_raw": out.String(), "stderr_raw": stderr.String(), "process_error": fmt.Sprint(err), "watchdog_error": fmt.Sprint(ctx.Err()), "elapsed_nanoseconds": int64(time.Since(start)), "protected_before_sha256": wp25NativeSnapshotDigest(before), "protected_after_sha256": wp25NativeSnapshotDigest(after), "protected_before": before, "protected_after": after, "expected_target_record_map": expected, "graph_family": "all-envelope specialization of published combined cap", "envelope_records": len(expected), "native_dependency_facts": 0, "observation_stage": stage, "configured_policy_milliseconds": 3600000, "configured_budget_isolation": true, "default_or_host_throughput_claim": false})
	if !reflect.DeepEqual(before, after) {
		t.Fatal("actual graph threshold caller changed accepted/private/config/key bytes")
	}
	if ctx.Err() != nil {
		t.Fatal("graph watchdog expired", ctx.Err())
	}
	if over {
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 1 || out.Len() != 0 || stderr.String() != "hn: contribution graph bound\n" {
			t.Fatalf("actual graph16385 exact refusal: %v stdout=%s stderr=%s", err, &out, &stderr)
		}
		return
	}
	if err != nil || stderr.Len() != 0 {
		t.Fatalf("actual graph16384 failed: %v stdout=%s stderr=%s", err, &out, &stderr)
	}
	var view struct {
		Project   string                   `json:"project"`
		Authority wp25PublicGraphAuthority `json:"authority"`
		Blocked   map[string]bool          `json:"blockedActors"`
		Invalid   []json.RawMessage        `json:"invalid"`
		Conflicts []json.RawMessage        `json:"conflicts"`
		Pending   []string                 `json:"pending"`
		Revoked   []string                 `json:"revoked"`
		Roots     map[string]string        `json:"roots"`
		Admitted  map[string]bool          `json:"admitted"`
		Visible   map[string]bool          `json:"visible"`
		Heads     map[string][]string      `json:"heads"`
		Decisions []struct {
			Target wp25PublicGraphTarget `json:"target"`
			Heads  []string              `json:"heads"`
		} `json:"decisionHeads"`
	}
	decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&view); e != nil {
		t.Fatal("strict actual graph view", e)
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		t.Fatal("trailing actual graph output")
	}
	if view.Project != project || view.Authority != authority || len(view.Blocked) != len(expected) || len(view.Decisions) != len(expected) || len(view.Invalid) != 0 || len(view.Conflicts) != 0 || len(view.Pending) != 0 || len(view.Revoked) != 0 || len(view.Roots) != 0 || len(view.Admitted) != 0 || len(view.Visible) != 0 || len(view.Heads) != 0 {
		t.Fatal("actual complete graph expected sets differ")
	}
	actual := map[string]string{}
	for _, d := range view.Decisions {
		if d.Target.Kind != "actor" || len(d.Heads) != 1 || actual[d.Target.ID] != "" {
			t.Fatal("actual graph duplicate/malformed decision head")
		}
		actual[d.Target.ID] = d.Heads[0]
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("actual reducer decision identities differ from independent signed corpus")
	}
	for target := range expected {
		if !view.Blocked[target] {
			t.Fatal("actual reducer failed effective current moderator block", target)
		}
	}
}
func wp25PublicContributionGraphLimit(t *testing.T, binary string) {
	f := wp25PublicCountNew(t, binary, "contribution-public-graph")
	wp25PublicCountCall(t, f, "contribution-graph-setup-native-identity", false, true, time.Minute, "init", "--name", "WP25 graph maintainer")
	identity := wp25PublicCountCall(t, f, "contribution-graph-setup-public-identity", true, true, time.Minute, "identity", "show", "--json")
	var actor struct {
		OK   bool `json:"ok"`
		Data struct {
			Actor string `json:"actor"`
		} `json:"data"`
	}
	if json.Unmarshal(identity, &actor) != nil || !actor.OK || len(actor.Data.Actor) != 64 {
		t.Fatalf("real public maintainer identity %s", identity)
	}
	signers := []wp25PublicGraphSigner{}
	moderators := []string{}
	for i := 0; i < 9; i++ {
		seed := sha256.Sum256([]byte(fmt.Sprintf("WP25 graph actual moderator %d", i)))
		key := ed25519.NewKeyFromSeed(seed[:])
		public := key.Public().(ed25519.PublicKey)
		sum := sha256.Sum256(public)
		a := hex.EncodeToString(sum[:])
		signers = append(signers, wp25PublicGraphSigner{a, public, key})
		moderators = append(moderators, a)
	}
	sort.Strings(moderators)
	sort.Slice(signers, func(i, j int) bool { return signers[i].actor < signers[j].actor })
	project := wp25NativeDigest([]byte("WP25 literal public contribution graph16384"))
	limits := map[string]any{"wireBytes": 16777216, "decodedBytes": 33554432, "objects": 4096, "events": 2048, "depth": 256, "milliseconds": 3600000, "window": map[string]any{"bytes": 268435456, "objects": 32768, "events": 16384, "requests": 256}, "retained": map[string]any{"bytes": 1073741824, "objects": 131072, "events": 65536, "requests": 1024}}
	policy := map[string]any{"version": "hn.policy/0", "maintainers": []string{actor.Data.Actor}, "proposals": map[string]any{"requiredAccepts": 1, "trustedReviewers": []string{}}, "pipelines": map[string]any{}, "contributions": map[string]any{"project": project, "moderators": moderators, "rules": []any{}, "baseline": []string{}, "limits": limits}}
	raw, e := json.Marshal(policy)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Join(f.repo, ".hn"), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(f.repo, ".hn", "policy.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	f.git(nil, "add", ".hn/policy.json")
	f.git(nil, "-c", "user.name=WP25 public fixture", "-c", "user.email=fixture@hn.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "explicit capacity-isolation policy")
	authority := wp25PublicGraphAuthority{f.git(nil, "rev-parse", "refs/heads/main"), wp25NativeDigest(raw)}
	wp25Record(t, map[string]any{"control": "source-assisted-explicit-contribution-policy", "source_assisted": true, "candidate_observation": false, "policy_raw": string(raw), "policy_digest": authority.Digest, "authority_commit": authority.Commit, "configured_policy_milliseconds": 3600000, "all_other_limits_literal_defaults": true, "configured_budget_isolation": true})
	wp25PublicCountCall(t, f, "contribution-graph-setup-authority", false, true, 62*time.Minute, "contribution", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent")
	heads := []wp25PublicGraphHead{}
	expected := map[string]string{}
	offset := 0
	for i, signer := range signers {
		n := 2040
		if i == 8 {
			n = 64
		}
		h := wp25PublicGraphManifestBuild(t, f, project, authority, signer, offset, n, fmt.Sprintf("manifest-%d-at", i))
		offset += n
		heads = append(heads, h)
		// Only this head is selected while it is genuinely charged/registered by
		// the supplied public caller. Default4096 object cap applies independently.
		closure := f.git(nil, "rev-list", "--objects", "--no-object-names", authority.Commit, h.tip)
		if len(strings.Split(closure, "\n")) > 4096 {
			t.Fatal("staged genuine retention exceeds default4096 objects")
		}
		wp25PublicGraphRetain(t, f, authority, project, h, fmt.Sprintf("at-%d", i))
		f.git(nil, "update-ref", "-d", h.ref)
		for target, id := range h.expected {
			if expected[target] != "" {
				t.Fatal("duplicate graph target")
			}
			expected[target] = id
		}
		if i == 3 || i == 7 {
			old, next := "initial", "wp25-graph-2"
			if i == 7 {
				old, next = "wp25-graph-2", "wp25-graph-3"
			}
			wp25PublicCountCall(t, f, "contribution-graph-setup-authorized-rollover", false, true, time.Minute, "contribution", "rollover", "--expect", old, "--window", next)
		}
	}
	if offset != 16384 || len(expected) != 16384 {
		t.Fatal("independent whole graph is incomplete")
	}
	for _, h := range heads {
		f.git(nil, "update-ref", h.ref, h.tip)
	}
	wp25PublicGraphPending(t, f, authority, project, expected, false)
	// The replacement has the same64 original valid ninth-moderator records and
	// exactly one additional valid signed decision. Its parentless manifest avoids
	// changing per-manifest history/depth/object selection or authority semantics.
	for _, h := range heads {
		f.git(nil, "update-ref", "-d", h.ref)
	}
	over := wp25PublicGraphManifestBuild(t, f, project, authority, signers[8], 16320, 65, "manifest-8-over")
	// Authenticate and genuinely retain this replacement alone as well: a
	// missing retention guard must not be a latent alternative refusal.
	wp25PublicGraphRetain(t, f, authority, project, over, "over-8")
	for i, h := range heads {
		tip := h.tip
		if i == 8 {
			tip = over.tip
		}
		f.git(nil, "update-ref", h.ref, tip)
	}
	overExpected := map[string]string{}
	for target, id := range expected {
		overExpected[target] = id
	}
	for target, id := range over.expected {
		overExpected[target] = id
	}
	if len(overExpected) != 16385 {
		t.Fatal("independent first-over graph differs by more than one record")
	}
	wp25PublicGraphPending(t, f, authority, project, overExpected, true)
	// Recovery changes only the ninth accepted ref back to the exact already
	// registered64-record head; no budget/accounting/authority metadata is edited.
	f.git(nil, "update-ref", heads[8].ref, heads[8].tip)
	wp25PublicGraphPending(t, f, authority, project, expected, false)
}

// One inherited trace-evidence helper uses its top-level test name directly as
// an os.CreateTemp pattern. Run that exact unchanged test as a compiled-self
// child so its name has no subtest slash. This is a harness naming workaround;
// the candidate and standalone consumer retain their independently pinned paths.
func wp25TopLevelFixtureJourney(t *testing.T, name string) {
	t.Helper()
	if name != "TestExtensionFakeTrackerNativeDefaultInert" {
		t.Fatal("unapproved standalone test-name workaround", name)
	}
	if os.Getenv("GORACE") != "atexit_sleep_ms=0" {
		t.Fatal("qualification race harness requires its recorded exit-delay-only option")
	}
	selectedTemp := os.TempDir()
	physicalTemp, tempErr := filepath.EvalSymlinks(selectedTemp)
	if tempErr != nil || selectedTemp != physicalTemp {
		t.Fatal("compiled-self fixture requires the selected physical temporary root", selectedTemp, physicalTemp, tempErr)
	}
	configuredTrace := *extensionBaselineTrace
	// Preserve the exact explicitly requested instrumentation in the child.
	// An empty selection remains empty; it is never credited as syscall proof.
	args := []string{"-test.run=^" + name + "$", "-test.v", "-test.count=1", "-test.timeout=15m", "-hn-baseline-trace", configuredTrace}
	self := wp25Self(t)
	expectedHarness := wp25Digest(t, self)
	ctx, cancel := context.WithTimeout(context.Background(), 16*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, self, args...)
	cmd.Env = os.Environ()
	forwardedTemp := ""
	for _, entry := range cmd.Env {
		if strings.HasPrefix(entry, "TMPDIR=") {
			forwardedTemp = strings.TrimPrefix(entry, "TMPDIR=")
		}
	}
	if forwardedTemp != os.Getenv("TMPDIR") {
		t.Fatal("compiled-self fixture lost selected TMPDIR")
	}
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	started := time.Now()
	err := cmd.Run()
	runs, passes, failures, skips := 0, 0, 0, 0
	for _, line := range strings.Split(out.String(), "\n") {
		if line == "=== RUN   "+name {
			runs++
		}
		if strings.HasPrefix(line, "--- PASS: "+name+" (") {
			passes++
		}
		if strings.HasPrefix(strings.TrimSpace(line), "--- FAIL:") {
			failures++
		}
		if strings.HasPrefix(strings.TrimSpace(line), "--- SKIP:") {
			skips++
		}
	}
	wp25Record(t, map[string]any{"control": "unchanged-top-level-fixture-journey", "test_name": name, "harness_executable": self, "harness_sha256": wp25Digest(t, self), "argv": args, "stdout_raw": out.String(), "stderr_raw": stderr.String(), "process_error": fmt.Sprint(err), "watchdog_error": fmt.Sprint(ctx.Err()), "elapsed_ns": time.Since(started).Nanoseconds(), "GORACE": os.Getenv("GORACE"), "configured_trace_executable": configuredTrace, "forwarded_trace_executable": args[len(args)-1], "selected_tempdir": selectedTemp, "physical_tempdir": physicalTemp, "forwarded_tmpdir_raw": forwardedTemp, "candidate": os.Getenv("HN_WP17_BINARY"), "candidate_sha256": wp25Digest(t, os.Getenv("HN_WP17_BINARY")), "consumer": os.Getenv("HN_WP20_CONSUMER"), "consumer_sha256": wp25Digest(t, os.Getenv("HN_WP20_CONSUMER")), "named_runs": runs, "named_passes": passes, "fail_events": failures, "skip_events": skips, "classification": "actual supplied candidate/reference native-default assertions; compiled-self source test peer; independent standalone consumer journeys remain separate"})
	if wp25Digest(t, self) != expectedHarness {
		t.Fatal("compiled-self fixture harness changed during execution")
	}
	if err != nil || ctx.Err() != nil || runs != 1 || passes != 1 || failures != 0 || skips != 0 || strings.TrimSpace(stderr.String()) != "" || !strings.HasSuffix(strings.TrimSpace(out.String()), "PASS") {
		t.Fatalf("unchanged top-level fixture did not execute exactly once and pass: %v watchdog=%v\n%s\n%s", err, ctx.Err(), &out, &stderr)
	}
}
