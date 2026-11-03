package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type efficiencyFixture struct {
	t                                         *testing.T
	root, remote, marker, base, head, keyPath string
	author, worker, trustedA, trustedB        *Identity
	proposal                                  *StoredEvent
}

func newEfficiencyFixture(t *testing.T, governed bool) *efficiencyFixture {
	t.Helper()
	f := &efficiencyFixture{t: t, root: inReplicationTestRepository(t)}
	var err error
	f.worker, f.keyPath, err = createIdentity("Automatic worker")
	if err != nil {
		t.Fatal(err)
	}
	f.author, f.trustedA, f.trustedB = testIdentity(t, "Requester"), testIdentity(t, "Qualified A"), testIdentity(t, "Qualified B")
	pipelines := map[string]PipelinePolicy{}
	if governed {
		pipelines["test"] = PipelinePolicy{RequiredResults: 2, TrustedRunners: []string{f.trustedA.Actor, f.trustedB.Actor}}
	}
	writeTestPolicy(t, f.root, PolicyDocument{Version: policyVersion, Maintainers: []string{f.author.Actor}, Proposals: ProposalPolicy{RequiredAccepts: 1, AllowAuthorApproval: true}, Pipelines: pipelines})
	f.marker = filepath.Join(t.TempDir(), "backend-marker")
	if err := os.WriteFile(f.marker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	pipeline := PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "Observe backend", Command: "sh", Args: []string{"-c", `printf 'executed\n' >> "$1"`, "hn-test", f.marker}}}}
	raw, err := json.Marshal(pipeline)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.root, ".hn", "pipelines", "test.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", ".hn")
	mustGit(t, "commit", "-qm", "Base with native pipeline")
	f.base = mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-qm", "Candidate")
	f.head = mustGitText(t, "rev-parse", "HEAD")
	f.remote = filepath.Join(t.TempDir(), "remote.git")
	mustGit(t, "clone", "-q", "--bare", f.root, f.remote)
	mustGit(t, "--git-dir="+f.remote, "update-ref", "refs/heads/main", f.base)
	mustGit(t, "remote", "add", "origin", f.remote)
	f.proposal = appendPolicyTestProposal(t, f.author, f.base, f.head, "Original candidate")
	return f
}
func (f *efficiencyFixture) request(actor *Identity, proposal *StoredEvent, pipeline, definition string) *StoredEvent {
	f.t.Helper()
	if definition == "" {
		_, _, digest, err := loadPipeline(proposal.Event.Head, "test")
		if err != nil {
			f.t.Fatal(err)
		}
		definition = digest
	}
	e, err := nextEvent(actor, "run.request")
	if err != nil {
		f.t.Fatal(err)
	}
	e.Subject, e.Pipeline, e.Definition, e.Commit = proposal.ID, pipeline, definition, proposal.Event.Head
	s, err := appendEvent(e, actor)
	if err != nil {
		f.t.Fatal(err)
	}
	return s
}
func (f *efficiencyFixture) revision() *StoredEvent {
	f.t.Helper()
	e, err := nextEvent(f.author, "proposal.revise")
	if err != nil {
		f.t.Fatal(err)
	}
	e.Subject, e.Base, e.Head = f.proposal.ID, f.base, f.head
	s, err := appendEvent(e, f.author)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := createProposalRef(s.ID, f.head); err != nil {
		f.t.Fatal(err)
	}
	return s
}
func (f *efficiencyFixture) merge(p *StoredEvent) {
	f.t.Helper()
	_, _, digest, err := loadPolicy(f.base)
	if err != nil {
		f.t.Fatal(err)
	}
	d := appendRevisionDecision(f.t, f.author, p.ID, digest, nil)
	appendRevisionMerge(f.t, f.author, p, digest, d)
}
func (f *efficiencyFixture) options() runnerOptions {
	return runnerOptions{Remote: "origin", Pipeline: "test", AcceptedActor: f.author.Actor, Backend: hostBackend{}}
}
func (f *efficiencyFixture) snapshot() []string {
	f.t.Helper()
	key, err := os.ReadFile(f.keyPath)
	if err != nil {
		f.t.Fatal(err)
	}
	return []string{mustGitText(f.t, "for-each-ref", "--format=%(refname) %(objectname)"), mustGitText(f.t, "cat-file", "--batch-all-objects", "--batch-check=%(objectname)"), string(key), mustGitText(f.t, "--git-dir="+f.remote, "for-each-ref", "--format=%(refname) %(objectname)")}
}
func (f *efficiencyFixture) skip(options runnerOptions, reason string) {
	f.t.Helper()
	for i := 0; i < 2; i++ {
		if err := runnerSync("origin"); err != nil {
			f.t.Fatal(err)
		}
	}
	before := f.snapshot()
	var ran bool
	output, err := captureTestOutput(f.t, func() error { var e error; ran, e = runnerOnceContext(context.Background(), options); return e })
	if err != nil || ran {
		f.t.Fatalf("expected automatic skip, ran=%t err=%v output=%s", ran, err, output)
	}
	if !strings.Contains(output, reason) {
		f.t.Fatalf("skip omitted reason %q: %s", reason, output)
	}
	if got := f.snapshot(); !reflect.DeepEqual(before, got) {
		f.t.Fatalf("automatic skip changed refs, objects or signer state: before=%q after=%q", before, got)
	}
	raw, err := os.ReadFile(f.marker)
	if err != nil || len(raw) != 0 {
		f.t.Fatalf("backend ran during skip: %q %v", raw, err)
	}
}
func (f *efficiencyFixture) execute(options runnerOptions) {
	f.t.Helper()
	ran, err := runnerOnce(options)
	if err != nil || !ran {
		f.t.Fatalf("eligible request did not run: %t %v", ran, err)
	}
	raw, err := os.ReadFile(f.marker)
	if err != nil || !strings.Contains(string(raw), "executed") {
		f.t.Fatalf("positive backend marker absent: %q %v", raw, err)
	}
}
func TestRunnerEfficiencyLineage(t *testing.T) {
	for _, kind := range []string{"superseded", "merged", "closed", "conflicted"} {
		t.Run(kind, func(t *testing.T) {
			f := newEfficiencyFixture(t, false)
			f.request(f.author, f.proposal, "test", "")
			switch kind {
			case "superseded":
				f.revision()
			case "merged":
				f.merge(f.proposal)
			case "closed":
				p := f.revision()
				f.merge(p)
			case "conflicted":
				a, b := f.revision(), f.revision()
				f.merge(a)
				f.merge(b)
			}
			reason := map[string]string{"closed": "lineage closed", "conflicted": "lineage merge conflict"}[kind]
			if reason == "" {
				reason = kind
			}
			f.skip(f.options(), reason)
			// The control is new eligible intent after all no-effect assertions.
			p := appendPolicyTestProposal(t, f.author, f.base, f.head, "Independent eligible candidate")
			f.request(f.author, p, "test", "")
			f.execute(f.options())
		})
	}
}

func (f *efficiencyFixture) result(actor *Identity, request *StoredEvent, outcome string) {
	f.t.Helper()
	e, err := nextEvent(actor, "run.result")
	if err != nil {
		f.t.Fatal(err)
	}
	log := []byte(outcome + "\n")
	e.Subject, e.Pipeline, e.Definition, e.Commit = request.ID, request.Event.Pipeline, request.Event.Definition, request.Event.Commit
	e.Outcome, e.Log, e.Backend, e.Platform, e.Runner = outcome, eventID(log), "host", "test/test", "hn/test"
	if outcome == "failed" {
		e.ExitCode = 1
	}
	if _, err := appendEventWithAttachments(e, actor, map[string][]byte{"log.txt": log}); err != nil {
		f.t.Fatal(err)
	}
}
func TestRunnerEfficiencyTrustedThreshold(t *testing.T) {
	f := newEfficiencyFixture(t, true)
	first := f.request(f.author, f.proposal, "test", "")
	second := f.request(f.author, f.proposal, "test", "")
	f.result(f.trustedA, first, "passed")
	f.result(f.trustedA, second, "passed")
	f.result(f.trustedB, second, "passed")
	// Checked-out policy is incidental; only the signed base policy counts.
	writeTestPolicy(t, f.root, PolicyDocument{Version: policyVersion, Maintainers: []string{f.author.Actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}})
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-qm", "Checkout policy differs")
	f.skip(f.options(), "pipeline satisfied by 2 qualifying trusted results")
	// Latest failure replaces this runner's pass even on an older request. A
	// different automatic worker can execute after observing the replaced pass.
	f.result(f.trustedA, first, "failed")
	f.execute(f.options())
}
func TestRunnerEfficiencyInsufficientEvidence(t *testing.T) {
	for _, kind := range []string{"duplicate-runner", "untrusted", "unauthorized", "wrong-definition", "wrong-pipeline", "other-candidate"} {
		t.Run(kind, func(t *testing.T) {
			f := newEfficiencyFixture(t, true)
			r := f.request(f.author, f.proposal, "test", "")
			f.result(f.trustedA, r, "passed")
			other := f.request(f.author, f.proposal, "test", "")
			actor := f.trustedB
			switch kind {
			case "duplicate-runner":
				actor = f.trustedA
			case "untrusted":
				actor = testIdentity(t, "Untrusted")
			case "unauthorized":
				other = f.request(testIdentity(t, "Unauthorized requester"), f.proposal, "test", "")
			case "wrong-definition":
				other = f.request(f.author, f.proposal, "test", eventID([]byte("wrong")))
			case "wrong-pipeline":
				other = f.request(f.author, f.proposal, "other", "")
			case "other-candidate":
				p := appendPolicyTestProposal(t, f.author, f.base, f.head, "Other candidate")
				other = f.request(f.author, p, "test", "")
			}
			f.result(actor, other, "passed")
			// Eligible genuine backend execution proves the evidence did not suppress.
			f.execute(f.options())
		})
	}
}
func TestRunnerEfficiencyUngovernedAndManual(t *testing.T) {
	t.Run("ungoverned", func(t *testing.T) {
		f := newEfficiencyFixture(t, false)
		r := f.request(f.author, f.proposal, "test", "")
		f.result(f.trustedA, r, "passed")
		f.execute(f.options())
	})
	t.Run("manual-when-satisfied", func(t *testing.T) {
		f := newEfficiencyFixture(t, true)
		r := f.request(f.author, f.proposal, "test", "")
		f.result(f.trustedA, r, "passed")
		f.result(f.trustedB, r, "passed")
		f.skip(f.options(), "pipeline satisfied")
		if err := cmdRunExecute([]string{r.ID, "--backend", "host", "--allow-unsafe-host-execution"}); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(f.marker)
		if len(raw) == 0 {
			t.Fatal("explicit execution was suppressed")
		}
	})
}
func TestRunnerEfficiencyStaleBase(t *testing.T) {
	f := newEfficiencyFixture(t, false)
	f.request(f.author, f.proposal, "test", "")
	options := f.options()
	options.SkipStaleBase = true
	options.Target = "refs/heads/main"
	mustGit(t, "--git-dir="+f.remote, "update-ref", "refs/heads/main", f.head)
	f.skip(options, "stale base")
	// Fresh observation, not a cached target or local HEAD. Local HEAD is the
	// candidate throughout; restoring the advertised base makes this eligible.
	mustGit(t, "--git-dir="+f.remote, "update-ref", "refs/heads/main", f.base)
	f.execute(options)
}
func TestRunnerEfficiencyMovedBaseWithoutOption(t *testing.T) {
	f := newEfficiencyFixture(t, false)
	f.request(f.author, f.proposal, "test", "")
	mustGit(t, "--git-dir="+f.remote, "update-ref", "refs/heads/main", f.head)
	f.execute(f.options())
}
func TestRunnerEfficiencyMissingTarget(t *testing.T) {
	f := newEfficiencyFixture(t, false)
	f.request(f.author, f.proposal, "test", "")
	for i := 0; i < 2; i++ {
		if err := runnerSync("origin"); err != nil {
			t.Fatal(err)
		}
	}
	before := f.snapshot()
	options := f.options()
	options.SkipStaleBase = true
	options.Target = "refs/heads/missing"
	ran, err := runnerOnce(options)
	if err == nil || ran {
		t.Fatalf("missing target executed: %t %v", ran, err)
	}
	if got := f.snapshot(); !reflect.DeepEqual(before, got) {
		t.Fatal("target refusal changed refs, objects or private signer")
	}
	raw, _ := os.ReadFile(f.marker)
	if len(raw) != 0 {
		t.Fatal("backend ran on uncertain target")
	}
	options.Target = "refs/heads/main"
	f.execute(options)
}
func TestRunnerTargetOptions(t *testing.T) {
	for _, args := range [][]string{{"--skip-stale-base"}, {"--target", "refs/heads/main"}, {"--skip-stale-base", "--target", "main"}, {"--skip-stale-base", "--target", "refs/tags/main"}, {"--skip-stale-base", "--target", "refs/heads/../main"}} {
		args = append([]string{"--accept-pipeline", "test", "--accept-actor", strings.Repeat("a", 64)}, args...)
		if _, err := parseRunnerOptions("test", args, false); err == nil || !strings.Contains(err.Error(), "target") {
			t.Fatalf("invalid target options returned %v for %q", err, args)
		}
	}
}

func TestRunnerTargetConfiguredAuthenticationAndSanitization(t *testing.T) {
	f := newEfficiencyFixture(t, false)
	// A real Git-produced advertisement behind an authenticated HTTP endpoint.
	advertisement, err := exec.Command("git", "upload-pack", "--stateless-rpc", "--advertise-refs", f.remote).Output()
	if err != nil {
		t.Fatal(err)
	}
	var authenticated atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "operator" || password != "local-test-token" {
			w.Header().Set("WWW-Authenticate", `Basic realm="native fixture"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		authenticated.Add(1)
		w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
		_, _ = w.Write(append([]byte("001e# service=git-upload-pack\n0000"), advertisement...))
	}))
	defer server.Close()
	home := t.TempDir()
	helperLog := filepath.Join(t.TempDir(), "credential-helper-env")
	helper := filepath.Join(t.TempDir(), "credentials")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s|%%s|%%s|%%s\\n' \"$GIT_TERMINAL_PROMPT\" \"$GIT_ASKPASS\" \"$SSH_ASKPASS\" \"$SSH_ASKPASS_REQUIRE\" >> %q\ncat >/dev/null\nif [ \"$1\" = get ]; then printf 'username=operator\\npassword=local-test-token\\n'; fi\n", helperLog)
	if err := os.WriteFile(helper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(home, ".gitconfig")
	// Reset any machine-level helper only in this fixture's temporary config;
	// then prove the intentionally configured helper survives sanitization.
	mustGit(t, "config", "--file", config, "--add", "credential.helper", "")
	mustGit(t, "config", "--file", config, "--add", "credential.helper", helper)
	mustGit(t, "config", "--file", config, "url."+server.URL+"/.insteadOf", "https://native-fixture.invalid/")
	mustGit(t, "remote", "set-url", "origin", "https://native-fixture.invalid/remote.git")
	// Redirects and injected config must not override this configured repository.
	rogue := filepath.Join(t.TempDir(), "rogue.git")
	mustGit(t, "init", "-q", "--bare", rogue)
	before := f.snapshot()
	t.Run("poisoned-ambient", func(t *testing.T) {
		for key, value := range map[string]string{"HOME": home, "XDG_CONFIG_HOME": filepath.Join(home, "xdg"), "GIT_DIR": rogue, "GIT_COMMON_DIR": rogue, "GIT_WORK_TREE": t.TempDir(), "GIT_OBJECT_DIRECTORY": t.TempDir(), "GIT_ALTERNATE_OBJECT_DIRECTORIES": "/nonexistent/objects", "GIT_NAMESPACE": "wrong", "GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "remote.origin.url", "GIT_CONFIG_VALUE_0": "/nonexistent/remote", "GIT_CONFIG_PARAMETERS": "'remote.origin.url=/nonexistent/other'", "GIT_CONFIG_GLOBAL": "/nonexistent/config", "GIT_CONFIG_SYSTEM": "/nonexistent/system", "GIT_CONFIG_NOSYSTEM": "1", "GIT_REPLACE_REF_BASE": "refs/wrong/"} {
			t.Setenv(key, value)
		}
		oid, err := observeRunnerTarget(context.Background(), "origin", "refs/heads/main")
		if err != nil || oid != f.base {
			t.Fatalf("configured authenticated target lost after sanitization: %q %v", oid, err)
		}
	})
	if authenticated.Load() == 0 {
		t.Fatal("no authenticated Git request occurred")
	}
	raw, err := os.ReadFile(helperLog)
	if err != nil || !strings.Contains(string(raw), "0|||never") {
		t.Fatalf("configured credentials not used with builtin prompts disabled: %q %v", raw, err)
	}
	if got := f.snapshot(); !reflect.DeepEqual(before, got) {
		t.Fatal("read-only authenticated probe changed repository or signer")
	}
}
func TestRunnerTargetBuiltinPromptsDisabled(t *testing.T) {
	newEfficiencyFixture(t, false)
	marker := filepath.Join(t.TempDir(), "askpass-marker")
	if err := os.WriteFile(marker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	askpass := filepath.Join(t.TempDir(), "askpass")
	if err := os.WriteFile(askpass, []byte(fmt.Sprintf("#!/bin/sh\nprintf called >> %q\n", marker)), 0700); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	mustGit(t, "config", "--file", filepath.Join(home, ".gitconfig"), "core.askPass", askpass)
	mustGit(t, "config", "--file", filepath.Join(home, ".gitconfig"), "credential.helper", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="no credentials"`)
		w.WriteHeader(401)
	}))
	defer server.Close()
	mustGit(t, "remote", "set-url", "origin", server.URL+"/remote.git")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("GIT_ASKPASS", askpass)
	t.Setenv("SSH_ASKPASS", askpass)
	t.Setenv("SSH_ASKPASS_REQUIRE", "force")
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	start := time.Now()
	if _, err := observeRunnerTarget(context.Background(), "origin", "refs/heads/main"); err == nil {
		t.Fatal("unauthenticated observation succeeded")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("builtin credential prompt did not fail promptly")
	}
	raw, err := os.ReadFile(marker)
	if err != nil || len(raw) != 0 {
		t.Fatalf("askpass executed: %q %v", raw, err)
	}
}
func TestRunnerTargetSSHConfiguration(t *testing.T) {
	f := newEfficiencyFixture(t, false)
	ledger := filepath.Join(t.TempDir(), "ssh-arguments")
	ssh := filepath.Join(t.TempDir(), "ssh")
	script := fmt.Sprintf("#!/bin/sh\nprintf 'ASKPASS=%%s|%%s|%%s AGENT=%%s\\n' \"$GIT_ASKPASS\" \"$SSH_ASKPASS\" \"$SSH_ASKPASS_REQUIRE\" \"$SSH_AUTH_SOCK\" >> %q\nfor arg do printf 'ARG=%%s\\n' \"$arg\" >> %q; last=$arg; done\nexec sh -c \"$last\"\n", ledger, ledger)
	if err := os.WriteFile(ssh, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	identity := filepath.Join(t.TempDir(), `identity with spaces\literal`)
	command := fmt.Sprintf("'%s' -i \"%s\" -o 'ProxyCommand=ssh -W %%h:%%p jump' -o BatchMode=no", ssh, identity)
	mustGit(t, "config", "core.sshCommand", command)
	mustGit(t, "remote", "set-url", "origin", "fixture-host:"+f.remote)
	t.Setenv("SSH_AUTH_SOCK", "/native-fixture/agent")
	t.Setenv("GIT_SSH_COMMAND", "")
	t.Setenv("GIT_SSH", "")
	t.Setenv("GIT_SSH_VARIANT", "")
	before := f.snapshot()
	oid, err := observeRunnerTarget(context.Background(), "origin", "refs/heads/main")
	if err != nil || oid != f.base {
		t.Fatalf("standard configured SSH failed: %q %v", oid, err)
	}
	raw, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, expected := range []string{"ASKPASS=||never AGENT=/native-fixture/agent", "ARG=-oBatchMode=yes", "ARG=" + identity, "ARG=ProxyCommand=ssh -W %h:%p jump", "ARG=BatchMode=no"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("SSH lost configured auth argument %q: %s", expected, text)
		}
	}
	if strings.Index(text, "ARG=-oBatchMode=yes") > strings.Index(text, "ARG=BatchMode=no") {
		t.Fatal("configured option overrode first-value BatchMode=yes")
	}
	if got := f.snapshot(); !reflect.DeepEqual(before, got) {
		t.Fatal("SSH observation changed repository state")
	}
	// A configured shell wrapper is not silently replaced with bare ssh.
	wrapperMarker := filepath.Join(t.TempDir(), "wrapper-marker")
	if err := os.WriteFile(wrapperMarker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "config", "core.sshCommand", fmt.Sprintf("printf called >> %q; ssh", wrapperMarker))
	if _, err := observeRunnerTarget(context.Background(), "origin", "refs/heads/main"); err == nil || !strings.Contains(err.Error(), "standard OpenSSH") {
		t.Fatalf("unsupported wrapper did not refuse actionably: %v", err)
	}
	raw, _ = os.ReadFile(wrapperMarker)
	if len(raw) != 0 {
		t.Fatal("unsupported SSH wrapper executed")
	}
}

// Only the ls-remote transport response is faulted; repository discovery/config
// commands still invoke real Git. The runner execution/authority path is real.
func installRunnerTargetFaultPeer(t *testing.T) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
last=
for arg do last=$arg; done
for arg do
 if [ "$arg" = ls-remote ] && [ "$last" = refs/heads/main ]; then
  case "$HN_TARGET_FAULT" in
   malformed) printf 'bad advertisement\n'; exit 0;;
   missing) exit 0;;
   duplicate) printf '%%s\trefs/heads/main\n%%s\trefs/heads/main\n' "$HN_TARGET_OID" "$HN_TARGET_OID"; exit 0;;
   other-ref) printf '%%s\trefs/heads/other\n' "$HN_TARGET_OID"; exit 0;;
   stderr-overflow|stdout-overflow|aggregate-boundary|combined-streams)
    if [ "$HN_TARGET_FAULT" = stderr-overflow ]; then exec 1>&2; fi
    limit=1025; if [ "$HN_TARGET_FAULT" = aggregate-boundary ]; then limit=1024; fi
    i=0; while [ "$i" -lt "$limit" ]; do
     if [ "$HN_TARGET_FAULT" = combined-streams ] && [ "$i" -ge 512 ]; then printf '................................................................' >&2; else printf '................................................................'; fi
     i=$((i+1)); done; exit 0;;
   error) printf 'private-secret-not-for-diagnostics\n' >&2; exit 1;;
   stall) sleep 30; exit 1;;
   inherited-pipes)
    (printf started > "$HN_TARGET_CHILD_STARTED"; sleep 1; printf leaked >> "$HN_TARGET_CHILD_MARKER") &
    while [ ! -s "$HN_TARGET_CHILD_STARTED" ]; do :; done
    exit 0;;
  esac
 fi
done
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
func TestRunnerTargetFaultRefusals(t *testing.T) {
	for _, mode := range []string{"malformed", "missing", "duplicate", "other-ref", "stdout-overflow", "stderr-overflow", "aggregate-boundary", "combined-streams", "error"} {
		t.Run(mode, func(t *testing.T) {
			f := newEfficiencyFixture(t, false)
			f.request(f.author, f.proposal, "test", "")
			for i := 0; i < 2; i++ {
				if err := runnerSync("origin"); err != nil {
					t.Fatal(err)
				}
			}
			before := f.snapshot()
			installRunnerTargetFaultPeer(t)
			t.Setenv("HN_TARGET_FAULT", mode)
			t.Setenv("HN_TARGET_OID", f.base)
			options := f.options()
			options.SkipStaleBase = true
			options.Target = "refs/heads/main"
			ran, err := runnerOnce(options)
			if err == nil || ran {
				t.Fatalf("fault %s executed: %t %v", mode, ran, err)
			}
			if strings.Contains(err.Error(), "import phase") {
				t.Fatalf("fault intercepted normal sync rather than selected target: %v", err)
			}
			if strings.Contains(err.Error(), "private-secret") {
				t.Fatal("transport diagnostic leaked stderr")
			}
			if (strings.Contains(mode, "overflow") || mode == "aggregate-boundary" || mode == "combined-streams") && !strings.Contains(err.Error(), "64KiB") {
				t.Fatalf("overflow refused for wrong reason: %v", err)
			}
			if got := f.snapshot(); !reflect.DeepEqual(before, got) {
				t.Fatal("target fault changed refs, objects or signer")
			}
			raw, e := os.ReadFile(f.marker)
			if e != nil || len(raw) != 0 {
				t.Fatalf("fault executed backend: %q %v", raw, e)
			}
			// Clear only the fault transport after all no-effect assertions. Marker is
			// never reset, and the same configured remote then gives a real success.
			t.Setenv("HN_TARGET_FAULT", "")
			f.execute(options)
		})
	}
}
func TestRunnerTargetCancellationAndDeadline(t *testing.T) {
	newEfficiencyFixture(t, false)
	installRunnerTargetFaultPeer(t)
	t.Setenv("HN_TARGET_FAULT", "stall")
	if runnerTargetTimeout > 15*time.Second || runnerTargetOutputLimit > 64<<10 {
		t.Fatal("probe exceeds contract limits")
	}
	for _, mode := range []string{"cancelled", "caller-deadline", "builtin-deadline"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			var cancel context.CancelFunc
			switch mode {
			case "cancelled":
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "caller-deadline":
				ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
			}
			start := time.Now()
			if _, err := observeRunnerTarget(ctx, "origin", "refs/heads/main"); err == nil {
				t.Fatal("stalled observation succeeded")
			}
			budget := runnerTargetTimeout + time.Second
			if mode != "builtin-deadline" {
				budget = time.Second
			}
			if time.Since(start) > budget {
				t.Fatal("cancellation/deadline did not bound process and pipes")
			}
		})
	}
}
func TestRunnerTargetInheritedPipeCleanup(t *testing.T) {
	newEfficiencyFixture(t, false)
	installRunnerTargetFaultPeer(t)
	t.Setenv("HN_TARGET_FAULT", "inherited-pipes")
	marker, started := filepath.Join(t.TempDir(), "child-effects"), filepath.Join(t.TempDir(), "child-started")
	if err := os.WriteFile(marker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HN_TARGET_CHILD_MARKER", marker)
	t.Setenv("HN_TARGET_CHILD_STARTED", started)
	start := time.Now()
	if _, err := observeRunnerTarget(context.Background(), "origin", "refs/heads/main"); err == nil {
		t.Fatal("descendant-held pipe yielded successful observation")
	}
	if time.Since(start) > time.Second {
		t.Fatal("inherited pipes blocked bounded wait")
	}
	raw, err := os.ReadFile(started)
	if err != nil || string(raw) != "started" {
		t.Fatal("control did not start descendant")
	}
	time.Sleep(1200 * time.Millisecond)
	raw, err = os.ReadFile(marker)
	if err != nil || len(raw) != 0 {
		t.Fatalf("owned descendant survived probe: %q %v", raw, err)
	}
}

func TestRunnerEfficiencyExistingFilters(t *testing.T) {
	for _, kind := range []string{"actor", "pipeline", "own-result"} {
		t.Run(kind, func(t *testing.T) {
			f := newEfficiencyFixture(t, false)
			r := f.request(f.author, f.proposal, "test", "")
			options := f.options()
			switch kind {
			case "actor":
				options.AcceptedActor = f.trustedA.Actor
			case "pipeline":
				options.Pipeline = "other"
			case "own-result":
				f.result(f.worker, r, "passed")
			}
			f.skip(options, "")
			if kind == "own-result" {
				f.request(f.author, f.proposal, "test", "")
			}
			f.execute(f.options())
		})
	}
}

func TestRunnerEfficiencyUnmergedSiblings(t *testing.T) {
	f := newEfficiencyFixture(t, false)
	a := f.revision()
	f.revision()
	f.request(f.author, a, "test", "")
	f.execute(f.options())
}
