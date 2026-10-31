package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	c "hubnot/internal/contribution"
)

var betaScaleSize = flag.Int("hn-scale-size", 0, "explicit workload units (2..64); zero leaves the larger scenario disabled")
var betaScaleNative = flag.Bool("hn-scale-native", false, "select the combined native-only executable for scale scenarios")

// The normal suite executes a real small journey. Larger measurements require
// an explicit invocation; no timing depends on the speed of a CI machine.
func TestBetaScaleSmoke(t *testing.T) {
	betaScaleMixed(t, betaContributionBinary(t, false), 1)
}

func TestBetaScaleScenarios(t *testing.T) {
	if *betaScaleSize == 0 {
		t.Skip("larger workload: explicitly pass -hn-scale-size=8 (or 24)")
	}
	if *betaScaleSize < 2 || *betaScaleSize > 64 {
		t.Fatal("hn-scale-size must be 2..64")
	}
	binary := betaContributionBinary(t, *betaScaleNative)
	t.Run("mixed-default-off", func(t *testing.T) { betaScaleMixed(t, binary, *betaScaleSize) })
	t.Run("enabled-default-budgets", func(t *testing.T) { betaScaleIntake(t, binary, c.DefaultLimits(), 3) })
	t.Run("enabled-explicit-example-budgets", func(t *testing.T) {
		// These are example choices, not defaults or a capacity recommendation.
		l := betaContributionLimits()
		l.Window.Requests, l.Retained.Requests = 6, 6
		betaScaleIntake(t, binary, l, 3)
	})
	t.Run("durable-input-recovery", func(t *testing.T) { betaScaleRecovery(t, binary) })
}

// Reports process wall time and captured output bytes only. This deliberately
// makes no total CPU/RSS claim about Git grandchildren or the whole machine.
func betaScaleMeasure(f *betaContributionCLI, repo, label string, ok bool, args ...string) string {
	f.t.Helper()
	start := time.Now()
	out := string(f.process(repo, f.binary, ok, args...))
	f.t.Logf("SCALE_COMMAND label=%s wall_ms=%.3f output_bytes=%d expected_success=%t argv=%q", label, float64(time.Since(start).Microseconds())/1000, len(out), ok, args)
	return out
}

func betaScaleStorage(f *betaContributionCLI, repo, label string) {
	f.t.Helper()
	var bytes, files int64
	err := filepath.WalkDir(filepath.Join(repo, ".git"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			i, err := d.Info()
			if err != nil {
				return err
			}
			bytes += i.Size()
			files++
		}
		return nil
	})
	if err != nil {
		f.t.Fatal(err)
	}
	objects := f.git(repo, "rev-list", "--objects", "--all")
	// Object paths may contain spaces, so count lines rather than tokens.
	objectCount := 0
	if objects != "" {
		objectCount = len(strings.Split(objects, "\n"))
	}
	f.t.Logf("SCALE_STORAGE label=%s git_regular_file_bytes=%d git_regular_files=%d reachable_objects=%d refs=%d count_objects=%q", label, bytes, files, objectCount, len(strings.Fields(f.git(repo, "for-each-ref", "--format=%(refname)"))), f.git(repo, "count-objects", "-v"))
}

func betaScalePage(t *testing.T, raw string, count int) {
	t.Helper()
	v := betaContributionJSON[struct {
		Data struct {
			Items []json.RawMessage `json:"items"`
			Next  string            `json:"next_cursor"`
		} `json:"data"`
	}](t, raw)
	if len(v.Data.Items) != count || v.Data.Next != "" {
		t.Fatalf("unexpected page, want %d complete items: %s", count, raw)
	}
}

func betaScaleMemory(t *testing.T, raw string, n int) {
	t.Helper()
	v := betaContributionJSON[MemoryRecallEnvelopeV0](t, raw)
	if v.Matched != n || v.Returned != n || v.Truncated || len(v.MissingDependencies) != 0 || len(v.Memories) != n {
		t.Fatalf("empty, truncated or incomplete search, want %d: %s", n, raw)
	}
	for _, row := range v.Memories {
		if row.Trust != memoryTrustQualified || row.Lifecycle != memoryLifecycleActive || row.Applicability != memoryApplicabilityApplicable || row.Evidence != memoryEvidenceResolved || !strings.Contains(row.Data.Content, "scale needle") || row.Signature != "valid" || len(row.Dependencies) != 0 {
			t.Fatalf("unqualified memory search row: %+v", row)
		}
	}
}

func betaScaleMixed(t *testing.T, binary string, n int) {
	f := newBetaContributionCLI(t, binary)
	repo := f.repository()
	f.hn(repo, true, "init", "--name", "Scale author")
	actor := f.actor(repo)
	// Memory trust is explicit, independent of contributions. No contribution
	// section/authority is installed by initialization or this memory policy.
	policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}, Memory: &MemoryPolicy{TrustedActors: []string{actor}, TrustedKinds: []string{"decision"}}}
	raw, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	betaContributionWrite(t, filepath.Join(repo, ".hn/policy.json"), raw, 0600)
	f.commit(repo, "Explicit memory trust only")
	policyBefore := f.git(repo, "rev-parse", "HEAD")
	f.hn(repo, false, "contribution", "authority", "show", "--json")
	project := betaContributionJSON[struct {
		Data ResourceMutationResult `json:"data"`
	}](t, f.hn(repo, true, "project", "open", "--title", "Scale project", "--json")).Data.ResourceID
	var issue, discussion, memory string
	for i := 0; i < n; i++ {
		title := fmt.Sprintf("Scale issue %03d", i)
		issue = betaRecoveryID(t, f.hn(repo, true, "issue", "open", "--body", strings.Repeat("reproduction details ", 16), "--json", title))
		f.hn(repo, true, "issue", "comment", issue, "--body", "Scale comment", "--json")
		discussion = betaContributionJSON[struct {
			Data ResourceMutationResult `json:"data"`
		}](t, f.hn(repo, true, "discussion", "open", "--title", fmt.Sprintf("Scale thread %03d", i), "--body", "Design context", "--category", "general", "--json")).Data.ResourceID
		f.hn(repo, true, "discussion", "reply", "add", discussion, "--body", "Scale answer", "--json")
		f.hn(repo, true, "project-item", "add", "--project", project, "--target", "issue:"+issue, "--json")
		memory = betaRecoveryID(t, f.hn(repo, true, "memory", "record", "--kind", "decision", "--at", "HEAD", "--applies", "descendants", "--content", fmt.Sprintf("scale needle %03d: %s", i, strings.Repeat("architecture context ", 16)), "--evidence", "git:"+policyBefore, "--json"))
	}
	if project == "" || discussion == "" {
		t.Fatal("missing real native resource IDs")
	}
	pageArgs := [][]string{{"issue", "list", "--limit", "100", "--json"}, {"discussion", "list", "--limit", "100", "--json"}, {"project-item", "list", "--project", project, "--limit", "100", "--json"}}
	for i, args := range pageArgs {
		// Validate the fixture before recording its repeated read cost.
		betaScalePage(t, f.hn(repo, true, args...), n)
		for _, pass := range []string{"first", "repeat"} {
			betaScalePage(t, betaScaleMeasure(f, repo, fmt.Sprintf("list-%d-%s", i, pass), true, args...), n)
		}
	}
	for _, command := range []string{"recall", "grep"} {
		args := []string{"memory", command, "--at", "HEAD", "--max-records", strconv.Itoa(n), "--max-content-bytes", "65536", "--json"}
		if command == "recall" {
			args = append(args, "--query", "scale needle")
		} else {
			args = append(args, "scale[ ]needle")
		}
		betaScaleMemory(t, f.hn(repo, true, args...), n)
		if err := os.Remove(filepath.Join(repo, ".git/hn/memory/index-v0.json")); err != nil {
			t.Fatal(err)
		}
		for _, pass := range []string{"index-absent", "index-present"} {
			betaScaleMemory(t, betaScaleMeasure(f, repo, command+"-"+pass, true, args...), n)
		}
	}
	betaContributionContains(t, betaScaleMeasure(f, repo, "issue-lookup", true, "issue", "show", issue, "--json"), "Scale issue")
	refs := strings.Fields(f.git(repo, "for-each-ref", "--format=%(refname)", "refs/heads/", "refs/hn/actors/", "refs/hn/memory/"))
	heads := f.git(repo, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/", "refs/hn/actors/", "refs/hn/memory/")
	root := t.TempDir()
	bundle := filepath.Join(root, "public.bundle")
	start := time.Now()
	f.git(repo, append([]string{"bundle", "create", bundle}, refs...)...)
	t.Logf("SCALE_COMMAND label=public-bundle-create wall_ms=%.3f selected_refs=%d", float64(time.Since(start).Microseconds())/1000, len(refs))
	observer := filepath.Join(root, "keyless")
	start = time.Now()
	// A selected-ref bundle has no symbolic HEAD; explicitly select the source
	// branch rather than relying on the receiving Git's default branch name.
	f.git(root, "clone", "--quiet", "--branch", f.git(repo, "symbolic-ref", "--short", "HEAD"), bundle, observer)
	f.git(observer, "fetch", "--quiet", "--no-write-fetch-head", bundle, "refs/hn/*:refs/hn/*")
	t.Logf("SCALE_COMMAND label=keyless-clone-fetch wall_ms=%.3f", float64(time.Since(start).Microseconds())/1000)
	if heads != f.git(observer, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/", "refs/hn/actors/", "refs/hn/memory/") {
		t.Fatal("restore changed exact public heads")
	}
	f.hn(observer, false, "identity", "show", "--json")
	betaScalePage(t, betaScaleMeasure(f, observer, "restored-issue-list", true, pageArgs[0]...), n)
	betaContributionContains(t, betaScaleMeasure(f, observer, "restored-memory-show", true, "memory", "show", memory, "--json"), "scale needle")
	betaScaleMigrate(f, repo, root, refs)
	if f.git(repo, "rev-parse", "HEAD") != policyBefore {
		t.Fatal("read/migration altered selected code/policy")
	}
	f.hn(repo, false, "contribution", "authority", "show", "--json")
	nativeEvents := f.git(repo, "rev-list", "--count", "refs/hn/actors/"+actor)
	memoryRefs := strings.Fields(f.git(repo, "for-each-ref", "--format=%(refname)", "refs/hn/memory/"))
	if len(memoryRefs) != 1 || f.git(repo, "rev-list", "--count", memoryRefs[0]) != strconv.Itoa(n) {
		t.Fatal("fixture memory stream/count differs from selected public results")
	}
	t.Logf("SCALE_FIXTURE units=%d authors=1 native_events=%s memory_records=%d issues=%d comments=%d discussions=%d replies=%d projects=1 project_items=%d memory_content_bytes_each~=%d enabled_contributions=false", n, nativeEvents, n, n, n, n, n, n, 340)
	betaScaleStorage(f, repo, "mixed-source")
	betaScaleStorage(f, observer, "mixed-restored")
}

func betaScaleMigrate(f *betaContributionCLI, repo, root string, refs []string) {
	f.t.Helper()
	archive, session := filepath.Join(root, "project.hn"), filepath.Join(root, "export")
	args := []string{"migrate", "plan", "--source-kind", "local", "--source-repository", repo, "--destination-kind", "archive", "--destination-repository", archive, "--session", session}
	for _, ref := range refs {
		args = append(args, "--ref", ref)
	}
	betaScaleMeasure(f, root, "migration-export-plan", true, args...)
	betaScaleMeasure(f, root, "migration-export-apply", true, "migrate", "apply", "--session", session, "--confirm-apply")
	inspection := betaScaleMeasure(f, root, "migration-inspect", true, "migrate", "inspect", "--archive", archive)
	for _, ref := range refs {
		betaContributionContains(f.t, inspection, f.git(repo, "rev-parse", ref))
	}
	destination := f.repository()
	importSession := filepath.Join(root, "import")
	args = []string{"migrate", "plan", "--source-kind", "archive", "--source-repository", archive, "--destination-kind", "local", "--destination-repository", destination, "--session", importSession}
	// The local/governance finding deliberately preserves imported historical
	// policy without adopting it as authority. No foreign provider is involved.
	args = append(args, "--decision", "local/governance=preserve")
	betaScaleMeasure(f, root, "migration-import-plan", true, args...)
	betaScaleMeasure(f, root, "migration-import-apply", true, "migrate", "apply", "--session", importSession, "--confirm-apply")
	imports := betaContributionJSON[[]string](f.t, f.hn(root, true, "migrate", "imports", "--repository", destination))
	if len(imports) != 1 {
		f.t.Fatal("missing exact imported snapshot")
	}
	betaContributionContains(f.t, f.hn(root, true, "migrate", "inspect", "--repository", destination, "--snapshot", imports[0]), "hn.project-snapshot/1")
	imported := f.git(destination, "for-each-ref", "--format=%(objectname)", "refs/hn/")
	for _, ref := range refs {
		if strings.HasPrefix(ref, "refs/hn/") {
			betaContributionContains(f.t, imported, f.git(repo, "rev-parse", ref))
		}
	}
	f.hn(destination, false, "identity", "show", "--json")
	i, err := os.Stat(archive)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Logf("SCALE_ARCHIVE bytes=%d selected_refs=%d", i.Size(), len(refs))
}

func betaScaleReceiveArgs(a betaContributionArtifact, operation string) []string {
	args := []string{"contribution", "receive", "--file", a.File, "--digest", a.Digest, "--operation", operation, "--json"}
	for _, ref := range a.Refs {
		args = append(args, "--ref", ref)
	}
	return args
}

func betaScaleIntake(t *testing.T, binary string, limits c.Limits, attempts int) {
	f := newBetaContributionCLI(t, binary)
	receiver, _, policy := f.enabled(t.Name(), limits, false)
	limitsJSON, _ := json.Marshal(limits)
	t.Logf("SCALE_POLICY exact_limits=%s", limitsJSON)
	for i := 0; i < attempts; i++ {
		contributor := f.repository()
		f.hn(contributor, true, "init", "--name", fmt.Sprintf("Scale stranger %d", i))
		revision := contributionEventID(t, f.hn(contributor, true, "issue", "open", "--json", fmt.Sprintf("Capacity issue %d", i)))
		a := f.prepare(contributor, policy.Contributions.Project, revision)
		operation := fmt.Sprintf("capacity-%d", i)
		ok := i < 2 || limits.WireBytes != c.DefaultLimits().WireBytes
		out := betaScaleMeasure(f, receiver, "automatic-intake-"+strconv.Itoa(i), ok, betaScaleReceiveArgs(a, operation)...)
		if !ok {
			betaContributionContains(t, out, "budget exhausted")
		}
		status := betaContributionJSON[betaContributionIntakeSnapshot](t, f.hn(receiver, true, "contribution", "intake", "--operation", operation, "--json"))
		t.Logf("SCALE_INTAKE operation=%s snapshot=%s", operation, mustBetaScaleJSON(t, status))
		if status.Attempt.WireBytes <= 0 || status.Attempt.Events <= 0 || status.Attempt.Objects <= 0 {
			t.Fatal("fixture was not read/verified")
		}
		wantCharges := int64(2 * (i + 1))
		if !ok {
			wantCharges-- // Input fits; the automatic receipt does not.
		}
		if status.Ledger.RetainedUsed.Requests != wantCharges {
			t.Fatalf("unexpected aggregate charges: got %d, want %d", status.Ledger.RetainedUsed.Requests, wantCharges)
		}
		if ok {
			betaContributionContains(t, f.hn(receiver, true, "issue", "show", revision, "--json"), "Capacity issue")
			before := status.Ledger
			betaScaleMeasure(f, receiver, "exact-complete-retry-"+strconv.Itoa(i), true, betaScaleReceiveArgs(a, operation)...)
			after := betaContributionJSON[betaContributionIntakeSnapshot](t, f.hn(receiver, true, "contribution", "intake", "--operation", operation, "--json"))
			if !reflect.DeepEqual(before, after.Ledger) {
				t.Fatal("completed retry changed charged ledger")
			}
		} else {
			list := f.hn(receiver, true, "issue", "list", "--json")
			if strings.Contains(list, "Capacity issue 2") {
				t.Fatal("failed receipt made input shared-visible")
			}
		}
	}
	betaScaleStorage(f, receiver, "enabled-receiver")
}

func mustBetaScaleJSON(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func betaScaleRecovery(t *testing.T, binary string) {
	f := newBetaContributionCLI(t, binary)
	receiver, _, policy := f.enabled(t.Name(), betaContributionLimits(), false)
	contributor := f.repository()
	f.hn(contributor, true, "init", "--name", "Recovery author")
	revision := contributionEventID(t, f.hn(contributor, true, "issue", "open", "--json", "Scale recovered issue"))
	a := f.prepare(contributor, policy.Contributions.Project, revision)
	x, err := OpenContributionExchange(filepath.Join(receiver, ".git/hn/contribution/inbox-"+policy.Contributions.Project[7:]), *policy.Contributions, "initial")
	if err != nil {
		t.Fatal(err)
	}
	x.fault = func(at string) error {
		if at == "artifact-durable" {
			return fmt.Errorf("source-assisted durable interruption")
		}
		return nil
	}
	attempt, err := x.Receive(context.Background(), a.File, a.Digest, "recover-scale", a.Refs)
	if err == nil || attempt.Status != "uncertain" || attempt.WireBytes <= 0 {
		t.Fatalf("source interruption setup: %+v %v", attempt, err)
	}
	before := betaContributionJSON[betaContributionIntakeSnapshot](t, f.hn(receiver, true, "contribution", "intake", "--operation", "recover-scale", "--json"))
	if err := os.Remove(a.File); err != nil {
		t.Fatal(err)
	}
	f.hn(receiver, true, "contribution", "rollover", "--expect", "initial", "--window", "scale-next")
	// Only the original selected public executable reconciles the missing source.
	betaScaleMeasure(f, receiver, "durable-missing-source-retry", true, betaScaleReceiveArgs(a, "recover-scale")...)
	after := betaContributionJSON[betaContributionIntakeSnapshot](t, f.hn(receiver, true, "contribution", "intake", "--operation", "recover-scale", "--json"))
	if after.Attempt.Intent != before.Attempt.Intent || after.Attempt.Reserved != before.Attempt.Reserved || after.Ledger.Reservations[0] != before.Ledger.Reservations[0] || after.Ledger.RetainedUsed.Requests != before.Ledger.RetainedUsed.Requests+1 || after.Ledger.WindowUsed.Requests != 1 || after.Ledger.Window != "scale-next" {
		t.Fatalf("input reservation not preserved: %s -> %s", mustBetaScaleJSON(t, before), mustBetaScaleJSON(t, after))
	}
	betaContributionContains(t, f.hn(receiver, true, "issue", "show", revision, "--json"), "Scale recovered issue")
	t.Logf("SCALE_RECOVERY source_assisted_fault=true before=%s after=%s", mustBetaScaleJSON(t, before), mustBetaScaleJSON(t, after))
}
