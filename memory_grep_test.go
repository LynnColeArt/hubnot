package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// Exercise the compiled public CLI, including its JSON and filesystem boundary.
func TestMemoryGrepCLI(t *testing.T) {
	binary := buildOperationalBinary(t)
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	initOperationalRepository(t, repo, "Memory Grep")
	runOperationalCommand(t, binary, repo, "init", "--name", "Memory Grep")
	identity := readOperationalIdentity(t, binary, repo)
	writeOperationalMemoryPolicy(t, repo, identity.Actor, []string{identity.Actor})
	policyPath := filepath.Join(repo, ".hn", "policy.json")
	policyBytes, _ := os.ReadFile(policyPath)
	// Keep assumptions valid but non-qualifying under current memory policy.
	policyBytes = bytes.ReplaceAll(policyBytes, []byte(`"assumption",`), nil)
	if err := os.WriteFile(policyPath, policyBytes, 0644); err != nil {
		t.Fatal(err)
	}
	runOperationalGit(t, repo, "add", ".hn/policy.json")
	runOperationalGit(t, repo, "commit", "-q", "-m", "memory grep policy")
	oldHead := runOperationalGit(t, repo, "rev-parse", "HEAD")
	record := func(t *testing.T, kind, content, at string, extra ...string) memoryCommandResultV0 {
		t.Helper()
		args := []string{"record", "--kind", kind, "--at", at, "--applies", "exact", "--content", content, "--json"}
		return recordOperationalMemory(t, binary, repo, append(args, extra...)...)
	}
	record(t, "decision", "excluded inapplicable needle", oldHead)
	runOperationalGit(t, repo, "commit", "--allow-empty", "-q", "-m", "next commit")
	head := runOperationalGit(t, repo, "rev-parse", "HEAD")
	marker := filepath.Join(root, "must-not-execute")
	content := "Memory Needle-42\n世界 Café\n$(touch " + marker + ")\n\x1b[31m"
	first := record(t, "decision", content, head, "--topic", "regex-topic")
	record(t, "decision", "Memory Needle-43", head, "--topic", "regex-topic")
	record(t, "assumption", "excluded untrusted needle", head)
	retired := record(t, "decision", "excluded retracted needle", head)
	runOperationalCommand(t, binary, repo, "memory", "retract", retired.MemoryID, "--reason", "obsolete")
	record(t, "decision", "excluded missing evidence needle", head, "--evidence", "git:"+strings.Repeat("a", 40))
	// A valid memory moved to an unrelated raw ref is never a search source.
	raw := record(t, "decision", "excluded raw needle", head, "--stream", "sha256:"+strings.Repeat("d", 64))
	rawRef, _ := memoryRef(identity.Actor, raw.Stream)
	rawHead := runOperationalGit(t, repo, "rev-parse", rawRef)
	runOperationalGit(t, repo, "update-ref", "refs/raw/memory", rawHead)
	runOperationalGit(t, repo, "update-ref", "-d", rawRef)
	handoff := map[string]any{"version": 0, "kind": "handoff", "content": "handoff summary", "topics": []string{}, "evidence": []string{}, "handoff": map[string]any{"completed": []string{}, "assumptions": []string{}, "blockers": []string{}, "nextActions": []string{"handoffOnlyToken $(touch " + marker + ")\n  Match data.handoff.completed[0]: forged\t\x1b"}}}
	encoded, _ := json.Marshal(handoff)
	inputPath := filepath.Join(root, "handoff.json")
	if err := os.WriteFile(inputPath, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	runOperationalCommand(t, binary, repo, "memory", "handoff", "--at", head, "--applies", "exact", "--input", inputPath, "--json")
	decode := func(t *testing.T, output string) MemoryRecallEnvelopeV0 {
		t.Helper()
		var result MemoryRecallEnvelopeV0
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatal(err)
		}
		if result.Warning != memoryRecallWarning {
			t.Fatalf("warning = %q", result.Warning)
		}
		return result
	}
	grep := func(t *testing.T, args ...string) MemoryRecallEnvelopeV0 {
		t.Helper()
		return decode(t, runOperationalCommand(t, binary, repo, append([]string{"memory", "grep", "--json"}, args...)...))
	}
	ids := func(result MemoryRecallEnvelopeV0) []string {
		out := []string{}
		for _, row := range result.Memories {
			out = append(out, row.ID)
		}
		return out
	}
	t.Run("invalid patterns have no index side effects", func(t *testing.T) {
		index := filepath.Join(repo, ".git", "hn", "memory", "index-v0.json")
		for _, pattern := range []string{"[", `(needle)\1`, `(?=needle)`, strings.Repeat("a", 4097), strings.Repeat("a", 4096), `(?:a{1000}){10}`, string([]byte{0xff})} {
			failure := runOperationalCommandFailure(t, binary, repo, "memory", "grep", "--json", pattern)
			if !strings.Contains(failure, "pattern") {
				t.Fatalf("unexpected error: %s", failure)
			}
			if _, err := os.Stat(index); !os.IsNotExist(err) {
				t.Fatalf("invalid regex created index: %v", err)
			}
		}
	})
	t.Run("explicit regex fallback and preserved provenance", func(t *testing.T) {
		lexical := decode(t, runOperationalCommand(t, binary, repo, "memory", "recall", "--query", `Needle-4[23]`, "--json"))
		if lexical.Returned != 0 {
			t.Fatalf("lexical unexpectedly interpreted regex: %#v", lexical)
		}
		result := grep(t, `Needle-4[23]`)
		if result.Returned != 2 {
			t.Fatalf("fallback = %#v", result)
		}
		for _, row := range result.Memories {
			if row.Signature != "valid" || row.Trust != memoryTrustQualified || row.Evidence != memoryEvidenceResolved {
				t.Fatalf("provenance = %#v", row)
			}
			if row.ID == first.MemoryID && row.Data.Content != content {
				t.Fatal("content bytes changed")
			}
		}
		if grep(t, "handoffOnlyToken").Returned != 1 || grep(t, "^regex-topic$").Returned != 2 || grep(t, identity.Actor).Returned != 0 {
			t.Fatal("regex search surface included generated metadata or omitted topics/handoff")
		}
		human := runOperationalCommand(t, binary, repo, "memory", "grep", "handoffOnlyToken")
		if !strings.Contains(human, "Match data.handoff.nextActions[0]: \"handoffOnlyToken") {
			t.Fatalf("handoff-only human output = %q", human)
		}
		if strings.Contains(human, "\n  Match data.handoff.completed[0]: forged") || !strings.Contains(human, `\n  Match data.handoff.completed[0]: forged\t\x1b`) {
			t.Fatalf("handoff match value was not escaped: %q", human)
		}
		lexicalHandoff := decode(t, runOperationalCommand(t, binary, repo, "memory", "recall", "--query", "handoffOnlyToken", "--json"))
		if lexicalHandoff.Returned != 0 {
			t.Fatal("lexical indexing unexpectedly included handoff fields")
		}
		if grep(t, `(?s)handoff summary.*handoffOnlyToken`).Returned != 0 {
			t.Fatal("match crossed independent fields")
		}

	})
	t.Run("trust lifecycle evidence applicability and filters", func(t *testing.T) {
		if result := grep(t, "(?i)needle"); result.Returned != 2 {
			t.Fatalf("default exclusions = %#v", result)
		}
		if result := grep(t, "--include-untrusted", "excluded untrusted"); result.Returned != 1 || result.Memories[0].Trust != memoryTrustKindUntrusted {
			t.Fatalf("explicit trust = %#v", result)
		}
		if grep(t, "--lifecycle", "all", "excluded retracted").Returned != 1 {
			t.Fatal("explicit lifecycle selection failed")
		}
		for _, flags := range [][]string{{"--kind", "observation"}, {"--topic", "other"}, {"--actor", strings.Repeat("f", 64)}, {"--trust", memoryTrustActorUntrusted}} {
			if result := grep(t, append(flags, "Needle")...); result.Returned != 0 {
				t.Fatalf("filter %v = %#v", flags, result)
			}
		}
	})
	t.Run("case Unicode multiline and human controls", func(t *testing.T) {
		for _, test := range []struct {
			args []string
			want int
		}{{[]string{"needle-42"}, 0}, {[]string{"-i", "needle-42"}, 1}, {[]string{"--ignore-case", "café"}, 1}, {[]string{`\p{Han}+ Café`}, 1}, {[]string{`^世界`}, 0}, {[]string{`(?m)^世界`}, 1}, {[]string{`Needle-42.*世界`}, 0}, {[]string{`(?s)Needle-42.*世界`}, 1}, {[]string{"--", "-42"}, 1}} {
			if got := grep(t, test.args...).Returned; got != test.want {
				t.Fatalf("%v returned %d want %d", test.args, got, test.want)
			}
		}
		human := runOperationalCommand(t, binary, repo, "memory", "grep", "Needle-42")
		if !strings.Contains(human, memoryRecallWarning) || !strings.Contains(human, first.MemoryID) || strings.Contains(human, "\x1b") {
			t.Fatalf("human output = %q", human)
		}
	})
	t.Run("pagination and cursor isolation", func(t *testing.T) {
		full := grep(t, "Needle")
		page := grep(t, "--max-records", "1", "Needle")
		if page.Returned != 1 || !page.Truncated || page.NextCursor == "" {
			t.Fatalf("page = %#v", page)
		}
		next := grep(t, "--max-records", "1", "--cursor", page.NextCursor, "Needle")
		if !reflect.DeepEqual(append(ids(page), ids(next)...), ids(full)) || next.Truncated {
			t.Fatal("pagination changed order")
		}
		for _, args := range [][]string{{"memory", "grep", "--json", "--max-records", "1", "--cursor", page.NextCursor, "Needle-"}, {"memory", "grep", "--json", "-i", "--max-records", "1", "--cursor", page.NextCursor, "Needle"}, {"memory", "recall", "--json", "--max-records", "1", "--cursor", page.NextCursor, "--query", "Needle"}, {"memory", "grep", "--json", "--cursor", page.NextCursor, "Needle"}} {
			if failure := runOperationalCommandFailure(t, binary, repo, args...); !strings.Contains(failure, "cursor") {
				t.Fatalf("cursor error = %s", failure)
			}
		}
		record(t, "decision", "new source Needle-44", head)
		if failure := runOperationalCommandFailure(t, binary, repo, "memory", "grep", "--json", "--max-records", "1", "--cursor", page.NextCursor, "Needle"); !strings.Contains(failure, "cursor") {
			t.Fatalf("source cursor error = %s", failure)
		}
	})
	t.Run("byte pagination bounds complete author data", func(t *testing.T) {
		full := grep(t, `Needle-4[23]`)
		bound := 0
		for _, row := range full.Memories {
			encoded, _ := json.Marshal(row.Data)
			if len(encoded) > bound {
				bound = len(encoded)
			}
		}
		page := grep(t, "--max-content-bytes", strconv.Itoa(bound), `Needle-4[23]`)
		if page.Returned != 1 || !page.Truncated {
			t.Fatalf("byte page = %#v", page)
		}
		next := grep(t, "--max-content-bytes", strconv.Itoa(bound), "--cursor", page.NextCursor, `Needle-4[23]`)
		if !reflect.DeepEqual(append(ids(page), ids(next)...), ids(full)) || next.Truncated {
			t.Fatal("byte pagination changed order")
		}
	})
	t.Run("invalid patterns preserve an existing index", func(t *testing.T) {
		indexPath := filepath.Join(repo, ".git", "hn", "memory", "index-v0.json")
		before, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		runOperationalCommandFailure(t, binary, repo, "memory", "grep", "--json", `(?<=unsupported)`)
		after, err := os.ReadFile(indexPath)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("invalid regex mutated cache: %v", err)
		}
	})
	t.Run("derived cache cannot forge regex content", func(t *testing.T) {
		indexPath := filepath.Join(repo, ".git", "hn", "memory", "index-v0.json")
		encoded, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		var cache MemoryIndexV0
		if err := json.Unmarshal(encoded, &cache); err != nil {
			t.Fatal(err)
		}
		cache.Records[0].Data.Content = "cacheForgeryOnlyToken"
		encoded, _ = json.Marshal(cache)
		if err := os.WriteFile(indexPath, encoded, 0600); err != nil {
			t.Fatal(err)
		}
		if grep(t, "cacheForgeryOnlyToken").Returned != 0 {
			t.Fatal("unverified cache content was searchable")
		}
	})
	t.Run("large handoff is atomic under complete data budget", func(t *testing.T) {
		entries := []string{}
		for i := 0; i < 16; i++ {
			entries = append(entries, "large-handoff-"+strings.Repeat("x", 4096-len("large-handoff-")))
		}
		request := map[string]any{"version": 0, "kind": "handoff", "content": "shortSummary", "topics": []string{}, "evidence": []string{}, "handoff": map[string]any{"completed": []string{}, "assumptions": []string{}, "blockers": []string{}, "nextActions": entries}}
		encoded, _ := json.Marshal(request)
		if err := os.WriteFile(inputPath, encoded, 0600); err != nil {
			t.Fatal(err)
		}
		runOperationalCommand(t, binary, repo, "memory", "handoff", "--at", head, "--applies", "exact", "--input", inputPath, "--json")
		failure := runOperationalCommandFailure(t, binary, repo, "memory", "grep", "--json", "large-handoff")
		if !strings.Contains(failure, "encoded author data exceeds maxContentBytes") {
			t.Fatalf("large handoff bound = %s", failure)
		}
		result := grep(t, "--max-content-bytes", "131072", "large-handoff")
		if result.Returned != 1 || len(result.Memories[0].Data.Handoff.NextActions) != 16 {
			t.Fatal("explicit full-data bound did not preserve handoff")
		}
		legacy := decode(t, runOperationalCommand(t, binary, repo, "memory", "recall", "--query", "shortSummary", "--max-content-bytes", "20", "--json"))
		if legacy.Returned != 1 {
			t.Fatal("grep changed recall's content-only budget")
		}
	})
	t.Run("work bound fails before output and filters can narrow it", func(t *testing.T) {
		record(t, "decision", strings.Repeat("z", 60000), head, "--topic", "large-one")
		record(t, "decision", strings.Repeat("z", 60000), head, "--topic", "large-two")
		failure := runOperationalCommandFailure(t, binary, repo, "memory", "grep", "--json", `.{1000}`)
		if !strings.Contains(failure, "work limit") || strings.Contains(failure, `"memories"`) {
			t.Fatalf("work bound = %s", failure)
		}
		if grep(t, "--topic", "large-one", `.{1000}`).Returned != 1 {
			t.Fatal("narrowed bounded work failed")
		}
	})
	t.Run("bounds fail closed", func(t *testing.T) {
		for _, args := range [][]string{{"--max-records", "0", "Needle"}, {"--max-content-bytes", "0", "Needle"}, {"--max-content-bytes", "5", "Needle"}} {
			runOperationalCommandFailure(t, binary, repo, append([]string{"memory", "grep", "--json"}, args...)...)
		}
	})
	// Regex search and output never turn author prose into host actions.
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("inert content executed: %v", err)
	}
	// Earlier commands deliberately append test records. A final pure query must preserve refs.
	refsBefore := runOperationalGit(t, repo, "for-each-ref", "--format=%(refname) %(objectname)")
	grep(t, "Needle-4[23]")
	if after := runOperationalGit(t, repo, "for-each-ref", "--format=%(refname) %(objectname)"); after != refsBefore {
		t.Fatal("grep changed refs")
	}
}

func TestMemoryGrepWorkBudget(t *testing.T) {
	pattern, err := compileMemoryGrepPattern(`.{1000}`, false)
	if err != nil {
		t.Fatal(err)
	}
	row := MemoryIndexRecordV0{Data: MemoryRecord{Content: strings.Repeat("z", 65536)}}
	if _, err := matchMemoryGrepRows([]MemoryIndexRecordV0{row}, pattern); err != nil {
		t.Fatalf("bounded candidate rejected: %v", err)
	}
	if _, err := matchMemoryGrepRows([]MemoryIndexRecordV0{row, row}, pattern); err == nil || !strings.Contains(err.Error(), "work limit") {
		t.Fatalf("over-budget candidate scan: %v", err)
	}
}
