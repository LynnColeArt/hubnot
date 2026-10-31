//go:build ignore

// Explicit maintainer operation; ordinary tests only read the frozen corpus.
// This program uses only Go's standard library, Git, and the selected release.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func sum(b []byte) string         { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func write(path string, b []byte) { must(os.WriteFile(path, b, 0644)) }
func encoded(v any) []byte        { b, e := json.MarshalIndent(v, "", "  "); must(e); return append(b, '\n') }

func main() {
	archive := flag.String("archive", "", "exact downloaded release archive")
	version := flag.String("version", "", "released version")
	source := flag.String("source", "", "released source commit")
	want := flag.String("sha256", "", "verified release archive SHA256")
	checksums := flag.String("checksums", "", "release SHA256SUMS listing")
	receipt := flag.String("receipt", "", "optional public release receipt JSON")
	out := flag.String("out", "", "new corpus directory (must not exist)")
	flag.Parse()
	if *archive == "" || *version == "" || *source == "" || *out == "" || *want == "" || *checksums == "" {
		panic("archive, version, source, sha256, checksums and out required")
	}
	if _, e := os.Stat(*out); !os.IsNotExist(e) {
		panic("output must be new")
	}
	raw, e := os.ReadFile(*archive)
	must(e)
	if sum(raw) != *want {
		panic("release archive checksum mismatch")
	}
	listing, e := os.ReadFile(*checksums)
	must(e)
	listed := false
	for _, line := range strings.Split(string(listing), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[0] == *want && strings.TrimPrefix(parts[1], "*") == filepath.Base(*archive) {
			listed = true
		}
	}
	if !listed {
		panic("archive not in release checksum listing")
	}
	temp, e := os.MkdirTemp("", "hn-released-writer-")
	must(e)
	defer os.RemoveAll(temp)
	gz, e := gzip.NewReader(bytes.NewReader(raw))
	must(e)
	tr := tar.NewReader(gz)
	var binary, build []byte
	var buildName string
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		must(e)
		switch filepath.Base(h.Name) {
		case "hn":
			binary, e = io.ReadAll(tr)
			must(e)
		case "BUILD.json", "BUILDINFO.json":
			build, e = io.ReadAll(tr)
			must(e)
			buildName = filepath.Base(h.Name)
		}
	}
	must(gz.Close())
	if len(binary) == 0 || len(build) == 0 {
		panic("missing executable/build record")
	}
	var metadata map[string]any
	must(json.Unmarshal(build, &metadata))
	recordedSource, recordedDigest := metadata["source_commit"], metadata["executable_sha256"]
	if buildName == "BUILDINFO.json" {
		recordedSource, recordedDigest = metadata["commit"], metadata["binary_sha256"]
	}
	if recordedSource != *source || recordedDigest != sum(binary) || metadata["version"] != *version {
		panic("release provenance mismatch")
	}
	bin := filepath.Join(temp, "hn")
	must(os.WriteFile(bin, binary, 0700))
	git, e := exec.LookPath("git")
	must(e)
	only := filepath.Join(temp, "path")
	must(os.Mkdir(only, 0700))
	must(os.Symlink(git, filepath.Join(only, "git")))
	env := []string{"PATH=" + only, "HOME=" + temp, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "GIT_AUTHOR_NAME=Compatibility corpus", "GIT_AUTHOR_EMAIL=corpus@example.invalid", "GIT_COMMITTER_NAME=Compatibility corpus", "GIT_COMMITTER_EMAIL=corpus@example.invalid"}
	repo := filepath.Join(temp, "repository")
	must(os.Mkdir(repo, 0700))
	run := func(input []byte, command string, args ...string) []byte {
		c := exec.Command(command, args...)
		c.Dir = repo
		c.Env = env
		c.Stdin = bytes.NewReader(input)
		b, e := c.CombinedOutput()
		if e != nil {
			panic(fmt.Sprintf("%s %q: %v\n%s", command, args, e, b))
		}
		return bytes.TrimSpace(b)
	}
	g := func(args ...string) string { return string(run(nil, git, args...)) }
	hn := func(args ...string) []byte { return run(nil, bin, args...) }
	g("init", "-q", "--initial-branch=main")
	write(filepath.Join(repo, "README.txt"), []byte("Public compatibility corpus from Hubnot "+*version+".\n"))
	g("add", "README.txt")
	g("commit", "-qm", "Public code baseline")
	hn("init", "--name", "Released writer "+*version)
	observations := map[string]json.RawMessage{}
	id := func(b []byte, key string) string {
		var v struct{ Data map[string]any }
		must(json.Unmarshal(b, &v))
		s, ok := v.Data[key].(string)
		if !ok {
			panic(string(b))
		}
		return s
	}
	issue := id(hn("issue", "open", "--body", "Historical signed issue body", "--operation", "historical-issue", "--json", "Historical issue "+*version), "issue_id")
	hn("issue", "comment", issue, "Historical signed comment")
	hn("issue", "close", issue, "--expect", issue, "--operation", "historical-close", "--json")
	observations["issue"] = hn("issue", "show", issue, "--json")
	states := map[string]string{
		"project":    `{"schema":"hn.project/1","title":"Historical roadmap","body":"Released project state","archived":false,"fields":[{"id":"phase","name":"Phase","type":"single_select","options":[{"id":"todo","name":"Todo"}]}],"views":[],"metadata":{"corpus/release":"` + *version + `"}}`,
		"discussion": `{"schema":"hn.discussion/1","title":"Historical discussion","body":"Preserve this thread","category":"general","status":"open"}`,
		"workflow":   `{"schema":"hn.workflow/1","name":"historical","title":"Inert historical workflow","description":"Not execution authority","enabled":false}`,
	}
	ids := map[string]string{"issue": issue}
	for _, family := range []string{"project", "discussion", "workflow", "project-item", "discussion-reply"} {
		if family == "project-item" {
			states[family] = `{"schema":"hn.project-item/1","project_id":"` + ids["project"] + `","project_revision":"` + ids["project"] + `","target":{"kind":"issue","id":"` + issue + `"},"values":{"phase":{"type":"single_select","option_id":"todo"}},"archived":false,"position":"0"}`
		}
		if family == "discussion-reply" {
			states[family] = `{"schema":"hn.discussion-reply/1","discussion_id":"` + ids["discussion"] + `","body":"Historical signed reply"}`
		}
		p := filepath.Join(temp, family+".json")
		write(p, []byte(states[family]))
		b := hn(family, "open", "--input", p, "--json")
		ids[family] = id(b, "resource_id")
		observations[family] = hn(family, "show", ids[family], "--json")
	}
	observations["discussion"] = hn("discussion", "show", ids["discussion"], "--json")
	// Include a lifecycle edge, separate from the collaboration actor chain.
	first := hn("memory", "record", "--kind", "decision", "--at", "HEAD", "--applies", "descendants", "--topic", "compatibility", "--content", "Historical memory decision", "--json")
	var firstMem map[string]any
	must(json.Unmarshal(first, &firstMem))
	memory := findID(firstMem)
	if memory == "" {
		panic(string(first))
	}
	hn("memory", "supersede", memory, "--kind", "decision", "--at", "HEAD", "--applies", "descendants", "--content", "Corrected historical memory", "--json")
	ids["memory"] = memory
	observations["memory"] = hn("memory", "show", memory, "--json")
	observations["recall"] = hn("memory", "recall", "--at", "HEAD", "--include-untrusted", "--lifecycle", "all", "--json")
	must(os.MkdirAll(*out, 0755))
	write(filepath.Join(*out, buildName), build)
	write(filepath.Join(*out, "RELEASE_SHA256SUMS"), listing)
	if *receipt != "" {
		raw, e := os.ReadFile(*receipt)
		must(e)
		write(filepath.Join(*out, "GITHUB_RELEASE.json"), raw)
	}
	write(filepath.Join(*out, "observations.json"), encoded(observations))
	refs := map[string]string{}
	for _, line := range strings.Split(g("for-each-ref", "--format=%(refname) %(objectname)"), "\n") {
		f := strings.Fields(line)
		refs[f[0]] = f[1]
	}
	abs, e := filepath.Abs(*out)
	must(e)
	g("bundle", "create", filepath.Join(abs, "public.bundle"), "--all")
	files := map[string]string{}
	if *version != "0.0.1-dev" {
		archiveOut := filepath.Join(abs, "project.hn")
		args := []string{"migrate", "plan", "--source-kind", "local", "--source-repository", repo, "--destination-kind", "archive", "--destination-repository", archiveOut, "--session", filepath.Join(temp, "export")}
		for ref := range refs {
			args = append(args, "--ref", ref)
		}
		hn(args...)
		hn("migrate", "apply", "--session", filepath.Join(temp, "export"), "--confirm-apply")
		observations["archive"] = hn("migrate", "inspect", "--archive", archiveOut)
		write(filepath.Join(abs, "observations.json"), encoded(observations))
	}
	// Separate synthetic future-format actors: valid Ed25519 envelopes and Git
	// objects, built without importing any Hubnot encoder or production package.
	refusals := map[string]string{}
	for _, kind := range []string{"protocol", "resource"} {
		pub, key, e := ed25519.GenerateKey(rand.Reader)
		must(e)
		actor := sum(pub)
		event := map[string]any{"protocol": "hn/0", "kind": "issue.open", "actor": actor, "actorName": "Future format corpus", "publicKey": base64.RawStdEncoding.EncodeToString(pub), "sequence": 1, "timestamp": "2026-10-28T00:00:00Z", "title": "Future protocol issue"}
		if kind == "protocol" {
			event["protocol"] = "hn/999"
		} else {
			event["kind"] = "project.open"
			delete(event, "title")
			event["body"] = `{"schema":"hn.project/999","title":"Future project","archived":false}`
		}
		payload, e := json.Marshal(event)
		must(e)
		sig := base64.RawStdEncoding.EncodeToString(ed25519.Sign(key, payload))
		blob := string(run(payload, git, "hash-object", "-w", "--stdin"))
		signature := string(run([]byte(sig), git, "hash-object", "-w", "--stdin"))
		tree := string(run([]byte(fmt.Sprintf("100644 blob %s\tevent.json\n100644 blob %s\tsignature\n", blob, signature)), git, "mktree"))
		commit := g("commit-tree", tree, "-m", "hn event sha256:"+sum(payload))
		ref := "refs/hn/actors/" + actor
		g("update-ref", ref, commit)
		g("bundle", "create", filepath.Join(abs, "future-"+kind+".bundle"), ref)
		g("update-ref", "-d", ref)
		refusals[kind] = actor
	}
	entries, e := os.ReadDir(abs)
	must(e)
	for _, entry := range entries {
		b, e := os.ReadFile(filepath.Join(abs, entry.Name()))
		must(e)
		files[entry.Name()] = sum(b)
	}
	provenance := map[string]any{"version": *version, "source_commit": *source, "release_url": "https://github.com/LynnColeArt/hubnot/releases/tag/v" + *version, "release_archive": filepath.Base(*archive), "release_archive_sha256": *want, "executable_sha256": sum(binary), "qualification": "exact release archive executable; Linux AMD64 standard writer", "files_sha256": files, "refs": refs, "ids": ids, "future_actors": refusals}
	write(filepath.Join(abs, "provenance.json"), encoded(provenance))
	fmt.Printf("Generated %s from release executable %s; disposable signer directory removed on exit\n", *out, sum(binary))
}
func findID(v any) string {
	switch x := v.(type) {
	case map[string]any:
		for _, key := range []string{"id", "memory_id", "memoryId"} {
			if s, ok := x[key].(string); ok && strings.HasPrefix(s, "sha256:") {
				return s
			}
		}
		for _, y := range x {
			if s := findID(y); s != "" {
				return s
			}
		}
	}
	return ""
}
