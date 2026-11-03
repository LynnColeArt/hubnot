package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"hubnot/internal/extension"
)

// Cold cases run in fresh processes; the shared Once is never reset.
func optionalBootstrapChild(t *testing.T, test, mode string, corpus bool) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "bin")
	if err = os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(git, filepath.Join(path, "git")); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	if err = os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	fixture, err := filepath.Abs("testdata/compatibility/0.0.3-dev")
	if err != nil {
		t.Fatal(err)
	}
	env := []string{"PATH=" + path, "HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, "config"), "XDG_DATA_HOME=" + filepath.Join(home, "data"), "XDG_CACHE_HOME=" + filepath.Join(home, "cache"), "XDG_STATE_HOME=" + filepath.Join(home, "state"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "LANG=C", "LC_ALL=C", "HN_OPTIONAL_BOOTSTRAP_CHILD=" + mode, "HN_OPTIONAL_BOOTSTRAP_FIXTURE=" + fixture}
	dir := root
	if corpus {
		dir = filepath.Join(root, "reader")
		for _, args := range [][]string{{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "clone", "-q", filepath.Join(fixture, "public.bundle"), dir}, {"-C", dir, "fetch", "--no-write-fetch-head", filepath.Join(fixture, "public.bundle"), "+refs/hn/*:refs/hn/*"}} {
			cmd := exec.Command(git, args...)
			cmd.Dir = root
			cmd.Env = env
			if out, e := cmd.CombinedOutput(); e != nil {
				t.Fatalf("public corpus: %v\n%s", e, out)
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-test.run=^"+test+"$", "-test.count=1")
	cmd.Dir = dir
	cmd.Env = env
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("fresh %s: %v\n%s", mode, e, out)
	}
}
func optionalBootstrapCold(t *testing.T) {
	t.Helper()
	if artifactCommands != nil || artifactTypeCommands != nil || extensionSubcommands != nil || extensionHostMethods != nil {
		t.Fatal("ordinary native startup constructed optional dispatch tables")
	}
	if extensionChanges.captures != nil {
		t.Fatal("ordinary native startup constructed capture cache")
	}
	for _, name := range []string{"artifact", "extension"} {
		if _, present := localCommands[name]; present {
			t.Fatalf("optional lookup grew native map: %s", name)
		}
	}
	if len(localCommands) != 8 {
		t.Fatalf("native map keys changed: %d", len(localCommands))
	}
}
func TestOptionalBootstrapColdNative(t *testing.T) {
	if os.Getenv("HN_OPTIONAL_BOOTSTRAP_CHILD") != "cold-native" {
		optionalBootstrapChild(t, "TestOptionalBootstrapColdNative", "cold-native", true)
		return
	}
	optionalBootstrapCold(t)
	for _, args := range [][]string{{"version"}, {"--help"}, {"extension", "help"}} {
		if err := run(args); err != nil {
			t.Fatal(err)
		}
		optionalBootstrapCold(t)
	}
	out, err := os.CreateTemp(t.TempDir(), "native-*.json")
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = out
	err = run([]string{"issue", "show", "sha256:1415cf2d081520783c3b91acd6d732f7ef3bf4abdb49824c83cc6554622eeb19", "--json"})
	os.Stdout = original
	if closeErr := out.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	var got any
	if json.Unmarshal(raw, &got) != nil {
		t.Fatal("native output is not JSON")
	}
	var expected map[string]any
	betaReadJSON(t, filepath.Join(os.Getenv("HN_OPTIONAL_BOOTSTRAP_FIXTURE"), "observations.json"), &expected)
	betaSemanticSubset(t, "", expected["issue"], got)
	optionalBootstrapCold(t)
}
func TestOptionalBootstrapReaders(t *testing.T) {
	mode := os.Getenv("HN_OPTIONAL_BOOTSTRAP_CHILD")
	if !strings.HasPrefix(mode, "reader-") {
		for _, name := range []string{"artifact", "type", "extension", "host"} {
			t.Run(name, func(t *testing.T) { optionalBootstrapChild(t, "TestOptionalBootstrapReaders", "reader-"+name, false) })
		}
		return
	}
	optionalBootstrapCold(t)
	var err error
	switch mode {
	case "reader-artifact":
		err = cmdArtifact(nil)
	case "reader-type":
		err = cmdArtifactType(nil)
	case "reader-extension":
		err = cmdExtension([]string{"not-a-command"})
	case "reader-host":
		_, err = extensionHostDispatch(context.Background(), extension.Invocation{})
	default:
		t.Fatal(mode)
	}
	if err == nil {
		t.Fatal("invalid explicit request unexpectedly succeeded")
	}
	if err = optionalBootstrapBindingError(); err != nil {
		t.Fatal(err)
	}
}
func TestOptionalBootstrapConcurrentFirstUse(t *testing.T) {
	if os.Getenv("HN_OPTIONAL_BOOTSTRAP_CHILD") != "concurrent" {
		optionalBootstrapChild(t, "TestOptionalBootstrapConcurrentFirstUse", "concurrent", false)
		return
	}
	optionalBootstrapCold(t)
	const users = 64
	start := make(chan struct{})
	results := make(chan error, users)
	var wait sync.WaitGroup
	for range users {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			ensureOptionalBootstrap()
			results <- optionalBootstrapBindingError()
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	// Calls through the actual readers must not replace the completed tables.
	before := reflect.ValueOf(extensionHostMethods).Pointer()
	_, _ = extensionHostDispatch(context.Background(), extension.Invocation{})
	if reflect.ValueOf(extensionHostMethods).Pointer() != before {
		t.Fatal("bootstrap replaced a published table")
	}
}
func TestOptionalBootstrapFinalBindings(t *testing.T) {
	ensureOptionalBootstrap()
	if err := optionalBootstrapBindingError(); err != nil {
		t.Fatal(err)
	}
	// Named-function identity supplements, rather than substitutes for, the live
	// Git/revision CLI tests that verify closure directions/kinds/view semantics.
}

func optionalBootstrapBindingError() error {
	if len(artifactCommands) != 21 {
		return fmt.Errorf("artifactCommands: want21 bindings, got%d", len(artifactCommands))
	}
	for name, want := range map[string]any{
		"--help":    cmdArtifactSearchHelp,
		"convert":   nil,
		"envelope":  cmdArtifactEnvelope,
		"export":    artifactExchangeExportCommand,
		"fetch":     nil,
		"heads":     nil,
		"help":      cmdArtifactSearchHelp,
		"history":   nil,
		"import":    artifactExchangeImportCommand,
		"inspect":   artifactExchangeInspectCommand,
		"list":      cmdArtifactList,
		"open":      cmdArtifactOpen,
		"operation": cmdArtifactOperation,
		"push":      nil,
		"record":    cmdArtifactRecord,
		"resolve":   nil,
		"revise":    nil,
		"schema":    cmdArtifactSchema,
		"search":    cmdArtifactSearch,
		"show":      nil,
		"type":      cmdArtifactType,
	} {
		got := artifactCommands[name]
		if got == nil {
			return fmt.Errorf("artifactCommands: missing%s", name)
		}
		if want != nil && reflect.ValueOf(got).Pointer() != reflect.ValueOf(want).Pointer() {
			return fmt.Errorf("artifactCommands: wrong final handler for%s", name)
		}
	}
	if len(artifactTypeCommands) != 3 {
		return fmt.Errorf("artifactTypeCommands: want3 bindings, got%d", len(artifactTypeCommands))
	}
	for name, want := range map[string]any{
		"publish":  cmdArtifactTypePublish,
		"show":     cmdArtifactTypeShow,
		"validate": cmdArtifactTypeValidate,
	} {
		got := artifactTypeCommands[name]
		if got == nil {
			return fmt.Errorf("artifactTypeCommands: missing%s", name)
		}
		if want != nil && reflect.ValueOf(got).Pointer() != reflect.ValueOf(want).Pointer() {
			return fmt.Errorf("artifactTypeCommands: wrong final handler for%s", name)
		}
	}
	if len(extensionSubcommands) != 8 {
		return fmt.Errorf("extensionSubcommands: want8 bindings, got%d", len(extensionSubcommands))
	}
	for name, want := range map[string]any{
		"disable":  extensionDisableAndCleanup,
		"enable":   extensionEnableAndCleanup,
		"frame":    extensionFrame,
		"inspect":  extensionInspectEnablement,
		"list":     extensionInspectEnablement,
		"register": extensionRegister,
		"remove":   extensionRemoveAndCleanup,
		"run":      extensionRun,
	} {
		got := extensionSubcommands[name]
		if got == nil {
			return fmt.Errorf("extensionSubcommands: missing%s", name)
		}
		if want != nil && reflect.ValueOf(got).Pointer() != reflect.ValueOf(want).Pointer() {
			return fmt.Errorf("extensionSubcommands: wrong final handler for%s", name)
		}
	}
	if len(extensionHostMethods) != 13 {
		return fmt.Errorf("extensionHostMethods: want13 bindings, got%d", len(extensionHostMethods))
	}
	for name, want := range map[string]any{
		"artifact.mutate":       extensionArtifactDispatch,
		"artifact.operation":    extensionArtifactDispatch,
		"artifact.read":         extensionArtifactDispatch,
		"artifact.search":       extensionArtifactDispatch,
		"artifact.type.publish": extensionArtifactDispatch,
		"artifact.type.read":    extensionArtifactDispatch,
		"changes.capture":       extensionChangesDispatch,
		"changes.page":          extensionChangesDispatch,
		"changes.since":         extensionChangesDispatch,
		"identity.public":       extensionNativeDispatch,
		"resource.mutate":       extensionNativeDispatch,
		"resource.operation":    extensionNativeDispatch,
		"resource.read":         extensionNativeDispatch,
	} {
		got := extensionHostMethods[name]
		if got == nil {
			return fmt.Errorf("extensionHostMethods: missing%s", name)
		}
		if want != nil && reflect.ValueOf(got).Pointer() != reflect.ValueOf(want).Pointer() {
			return fmt.Errorf("extensionHostMethods: wrong final handler for%s", name)
		}
	}
	if got, present := lookupLocalCommand("artifact"); !present || got == nil || reflect.ValueOf(got).Pointer() != reflect.ValueOf(cmdArtifact).Pointer() {
		return fmt.Errorf("native router artifact drift")
	}
	if got, present := lookupLocalCommand("extension"); !present || got == nil || reflect.ValueOf(got).Pointer() != reflect.ValueOf(extensionHostCommand).Pointer() {
		return fmt.Errorf("native router extension drift")
	}
	return nil
}

func TestOptionalBootstrapZeroStartup(t *testing.T) {
	if os.Getenv("HN_OPTIONAL_BOOTSTRAP_CHILD") != "zero-startup" {
		optionalBootstrapChild(t, "TestOptionalBootstrapZeroStartup", "zero-startup", false)
		return
	}
	optionalBootstrapCold(t)
	if extensionChanges.captures != nil {
		t.Error("cold optional capture cache was constructed")
	}
	for _, name := range []string{"artifact", "extension"} {
		if _, present := localCommands[name]; present {
			t.Errorf("cold optional router %s grew native registration map", name)
		}
	}
}

func optionalBootstrapOutput(t *testing.T, call func()) string {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = out
	func() { defer func() { os.Stdout = original }(); call() }()
	if err = out.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func TestOptionalBootstrapLookupAndHelp(t *testing.T) {
	if os.Getenv("HN_OPTIONAL_BOOTSTRAP_CHILD") != "lookup" {
		optionalBootstrapChild(t, "TestOptionalBootstrapLookupAndHelp", "lookup", false)
		return
	}
	optionalBootstrapCold(t)
	before := len(localCommands)
	for name, want := range map[string]any{"artifact": cmdArtifact, "extension": extensionHostCommand, "audit": cmdAudit} {
		got, present := lookupLocalCommand(name)
		if !present || got == nil || reflect.ValueOf(got).Pointer() != reflect.ValueOf(want).Pointer() {
			t.Fatal("lookup drift", name)
		}
	}
	if got, present := lookupLocalCommand("unknown-local"); present || got != nil {
		t.Fatal("unknown local command appeared")
	}
	expected := []string{"artifact", "audit", "contribution", "discussion", "discussion-reply", "extension", "migrate", "project", "project-item", "workflow"}
	names := localCommandNames()
	sort.Strings(names)
	if !reflect.DeepEqual(names, expected) {
		t.Fatal(names)
	}
	output := optionalBootstrapOutput(t, printUsage)
	suffix := "\nNative resource commands:\n"
	for _, name := range expected {
		suffix += "  hn " + name + " help\n"
	}
	if !strings.HasSuffix(output, suffix) {
		t.Fatal("sorted public help drift")
	}
	for _, name := range []string{"artifact", "extension", "generic-test"} {
		localCommands[name] = nil
		nilCount := 0
		for _, entry := range localCommandNames() {
			if entry == name {
				nilCount++
			}
		}
		if nilCount != 1 {
			t.Fatal("nil override missing or duplicated in help", name, nilCount)
		}
		got, present := lookupLocalCommand(name)
		if !present || got != nil {
			t.Fatal("nil override fell through", name)
		}
		invoked := false
		override := func(args []string) error { invoked = reflect.DeepEqual(args, []string{"one", "two"}); return nil }
		localCommands[name] = override
		got, present = lookupLocalCommand(name)
		if !present || got == nil || got([]string{"one", "two"}) != nil || !invoked {
			t.Fatal("generic override/arguments drift", name)
		}
		count := 0
		for _, entry := range localCommandNames() {
			if entry == name {
				count++
			}
		}
		if count != 1 {
			t.Fatal("duplicate or missing help name", name, count)
		}
		delete(localCommands, name)
	}
	if len(localCommands) != before {
		t.Fatal("lookup/help grew native map")
	}
	if err := run([]string{"artifact", "help"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"extension", "help"}); err != nil {
		t.Fatal(err)
	}
	if len(localCommands) != before {
		t.Fatal("explicit optional use grew native map")
	}
	for _, name := range []string{"artifact", "extension"} {
		if _, present := localCommands[name]; present {
			t.Fatal("optional key inserted", name)
		}
	}
}
func TestOptionalBootstrapMainVocabularies(t *testing.T) {
	for _, bad := range []string{"Accepted", "accepted ", "accepted\x00", "unknown"} {
		raw, _ := json.Marshal(map[string]any{"protocol": artifactExchangeProtocol, "mode": bad, "refs": []any{}, "objects": []any{}})
		_, err := decodeArtifactExchange(context.Background(), raw, 40)
		revisionError(t, err, "invalid_input")
	}

	check := func(values, want []string, valid func(string) bool) {
		t.Helper()
		if len(values) != len(want) {
			t.Fatal("vocabulary size")
		}
		seen := map[string]bool{}
		expected := map[string]bool{}
		for _, v := range want {
			expected[v] = true
		}
		for _, v := range values {
			if seen[v] || !expected[v] || !valid(v) {
				t.Fatal("duplicate/unknown/rejected value", v)
			}
			seen[v] = true
			for _, bad := range []string{"", strings.ToUpper(v), v + " ", " " + v, v + "\x00", "prefix" + v, v + "suffix"} {
				if !expected[bad] && valid(bad) {
					t.Fatal("near miss admitted", bad)
				}
			}
		}
	}
	modes := artifactExchangeModeVocabulary()
	objects := artifactExchangeObjectTypeVocabulary()
	shapes := artifactListShapeVocabulary()
	check(modes[:], []string{"accepted", "opaque"}, validArtifactExchangeMode)
	check(objects[:], []string{"blob", "tree", "commit"}, validArtifactExchangeObjectType)
	check(shapes[:], []string{"document", "list", "ticket"}, validArtifactListShape)
	modes[0] = "changed"
	objects[0] = "changed"
	shapes[0] = "changed"
	if artifactExchangeModeVocabulary()[0] != "accepted" || artifactExchangeObjectTypeVocabulary()[0] != "blob" || artifactListShapeVocabulary()[0] != "document" {
		t.Fatal("returned arrays alias")
	}
	for _, bad := range []string{"document ", "Document", "document\x00", "unknown"} {
		if err := cmdArtifactList([]string{"--shape", bad, "--json"}); err == nil {
			t.Fatal("invalid shape reached repository", bad)
		}
	}
	repo := t.TempDir()
	object := exchangeObject("blob", []byte("public"))
	object.Type = "tag"
	revisionError(t, artifactExchangeCopy(context.Background(), repo, []artifactExchangeObject{object}), "verification_failure")
	if entries, err := os.ReadDir(repo); err != nil || len(entries) != 0 {
		t.Fatal("unsupported object copied before refusal", entries, err)
	}
}
func TestOptionalBootstrapCaptureFirstUse(t *testing.T) {
	if os.Getenv("HN_OPTIONAL_BOOTSTRAP_CHILD") != "capture-first" {
		optionalBootstrapChild(t, "TestOptionalBootstrapCaptureFirstUse", "capture-first", false)
		return
	}
	optionalBootstrapCold(t)
	extensionChangesForget("missing")
	extensionChanges.Lock()
	missing := extensionChanges.captures["missing"]
	stillNil := extensionChanges.captures == nil
	extensionChanges.Unlock()
	if missing != nil || !stillNil {
		t.Fatal("missing read/delete constructed cache")
	}
	start := make(chan struct{})
	var wait sync.WaitGroup
	for n := range 64 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			ensureOptionalBootstrap()
			if !validArtifactListShape("document") || !validArtifactExchangeMode("accepted") || !validArtifactExchangeObjectType("blob") {
				t.Error("mixed vocabulary drift")
			}
			extensionChangesRetain(&extensionChangeCapture{token: fmt.Sprint(n), bytes: 1, created: time.Now()})
		}()
	}
	close(start)
	wait.Wait()
	extensionChanges.Lock()
	defer extensionChanges.Unlock()
	if extensionChanges.captures == nil || len(extensionChanges.captures) != extensionChangeCacheCount {
		t.Fatal("first retain/eviction incomplete", len(extensionChanges.captures))
	}
	for token, capture := range extensionChanges.captures {
		if capture == nil || capture.token != token || capture.bytes != 1 {
			t.Fatal("partially published capture")
		}
	}
}
