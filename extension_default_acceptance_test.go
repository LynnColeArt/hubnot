package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

var defaultBinaryDir = flag.String("hn-default-binaries", "", "explicit directory containing standard and native-only non-race hn executables")
var defaultEvidence = flag.String("hn-default-evidence", "", "external directory for raw default qualification transcripts")
var defaultDonor = flag.String("hn-default-donor", "", "external output filename for the public-only large artifact bundle")
var defaultDetectorMutant = flag.String("hn-default-detector-mutant", "", "qualification negative control: disable one detector cause")

var defaultConfigurations = []string{"absent", "disabled", "missing", "incompatible", "crashed", "malformed", "many-custom", "oversized", "EOF", "blocked", "cancelled", "malformed-custom", "opaque-custom"}

func defaultBinary(t *testing.T, variant string) string {
	t.Helper()
	if *defaultBinaryDir != "" {
		p := filepath.Join(*defaultBinaryDir, variant)
		info, err := os.Stat(p)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
			t.Fatalf("explicit default executable %s: %v", variant, err)
		}
		return p
	}
	tags := ""
	if variant == "native-only" {
		tags = "nogithub,noforge"
	}
	return buildMigrationCLIBinary(t, tags)
}

type defaultFixture struct {
	t                                    *testing.T
	f                                    *betaContributionCLI
	repo, config, common, module, marker string
	sequence                             int
}

func defaultNew(t *testing.T, binary, repo string) *defaultFixture {
	t.Helper()
	f := newBetaContributionCLI(t, binary)
	home := strings.TrimPrefix(f.env[1], "HOME=")
	config := filepath.Join(home, "config")
	if runtime.GOOS == "darwin" {
		config = filepath.Join(home, "Library", "Application Support")
	}
	f.env = append(f.env, "XDG_CONFIG_HOME="+config, "HN_WP16_CHILD=1")
	if repo == "" {
		repo = f.repository()
		f.git(repo, "config", "user.name", "Default Fixture")
		f.git(repo, "config", "user.email", "default@example.invalid")
	}
	common := f.git(repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	return &defaultFixture{t: t, f: f, repo: repo, config: config, common: common, module: filepath.Join(t.TempDir(), "forbidden-module"), marker: filepath.Join(t.TempDir(), "data-activated")}
}

func (d *defaultFixture) registry() string {
	return filepath.Join(d.config, "hubnot", "extensions", "registry.json")
}
func (d *defaultFixture) activated() bool {
	if *defaultDetectorMutant == "data-activation" {
		return false
	}
	_, err := os.Stat(d.marker)
	return !os.IsNotExist(err)
}
func (d *defaultFixture) roots() []string {
	return []string{filepath.Dir(d.registry()), filepath.Join(d.common, "hn", "extensions"), filepath.Join(d.common, "hn", "artifacts", "intents"), filepath.Join(d.repo, ".hn", "extensions.json")}
}
func (d *defaultFixture) snapshot() map[string]string {
	values := map[string]string{}
	for i, root := range d.roots() {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		for k, v := range policyPublicSnapshot(d.t, root) {
			values[fmt.Sprint(i)+":"+k] = v
		}
	}
	return values
}

// %file observes probes and transient writes too. -yy resolves dirfd paths;
// matching both cwd-relative and absolute paths also covers AT_FDCWD reads.
func (d *defaultFixture) violations(trace string) map[string]bool {
	found := map[string]bool{}
	for _, line := range strings.Split(trace, "\n") {
		for _, root := range append(d.roots(), d.module) {
			relative, _ := filepath.Rel(d.repo, root)
			if !strings.Contains(line, root) && !strings.Contains(line, `"`+relative) {
				continue
			}
			// A tracked inert declaration participates in ordinary Git status and
			// merge metadata checks. Observe content opens, not harmless lstat.
			if root == filepath.Join(d.repo, ".hn", "extensions.json") && !strings.Contains(line, "open") && !strings.Contains(line, "unlink") && !strings.Contains(line, "rename") {
				continue
			}
			cause := "registry-read"
			if root == d.module {
				cause = "module-probe"
				if strings.Contains(line, "execve(") {
					cause = "module-start"
				}
			}
			if strings.Contains(line, "O_WRONLY") || strings.Contains(line, "O_RDWR") || strings.Contains(line, "O_CREAT") || strings.Contains(line, "mkdir") || strings.Contains(line, "unlink") || strings.Contains(line, "rename") {
				cause = "extension-state-write"
			}
			found[cause] = true
		}
		if strings.Contains(line, "connect(") && (strings.Contains(line, "AF_INET") || strings.Contains(line, "AF_UNIX")) {
			found["provider-connect"] = true
		}
	}
	delete(found, *defaultDetectorMutant)
	return found
}

func (d *defaultFixture) process(program, input string, ok bool, args ...string) []byte {
	d.t.Helper()
	d.sequence++
	before := d.snapshot()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	trace := filepath.Join(d.t.TempDir(), "effects.trace")
	command, argv := program, args
	if *extensionBaselineTrace != "" {
		command = *extensionBaselineTrace
		argv = append([]string{"-f", "-qq", "-yy", "-s", "4096", "-e", "trace=%file,process,network", "-o", trace, "--", program}, args...)
	}
	cmd := exec.CommandContext(ctx, command, argv...)
	cmd.Dir = d.repo
	cmd.Env = d.f.env
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if *defaultEvidence != "" {
		if e := os.MkdirAll(*defaultEvidence, 0700); e != nil {
			d.t.Fatal(e)
		}
		file, e := os.CreateTemp(*defaultEvidence, "default-*.json")
		if e != nil {
			d.t.Fatal(e)
		}
		_ = json.NewEncoder(file).Encode(map[string]any{"test": d.t.Name(), "program": program, "argv": args, "env": d.f.env, "exit": fmt.Sprint(err), "output": string(out), "trace": trace})
		file.Close()
	}
	if ctx.Err() != nil || (err == nil) != ok {
		d.t.Fatalf("public %q exit=%v deadline=%v\n%s", args, err, ctx.Err(), out)
	}
	if !reflect.DeepEqual(before, d.snapshot()) {
		d.t.Fatalf("public %q changed extension-private state", args)
	}
	if *extensionBaselineTrace != "" {
		raw, e := os.ReadFile(trace)
		if e != nil {
			d.t.Fatal(e)
		}
		if *defaultEvidence != "" {
			file, e := os.CreateTemp(*defaultEvidence, "effects-*.trace")
			if e != nil {
				d.t.Fatal(e)
			}
			_, _ = file.Write(raw)
			file.Close()
		}
		if causes := d.violations(string(raw)); len(causes) != 0 {
			d.t.Fatalf("public %q forbidden effects %v", args, causes)
		}
	}
	if d.activated() {
		d.t.Fatal("stored data activated")
	}
	return out
}
func (d *defaultFixture) call(ok bool, args ...string) string {
	d.t.Helper()
	return string(d.process(d.f.binary, "", ok, args...))
}
func (d *defaultFixture) data(args ...string) map[string]any {
	d.t.Helper()
	var v struct {
		OK   bool
		Data map[string]any
	}
	raw := d.call(true, args...)
	if json.Unmarshal([]byte(raw), &v) != nil || !v.OK {
		d.t.Fatal(raw)
	}
	return v.Data
}

func (d *defaultFixture) configure(name, donor string) {
	d.t.Helper()
	if name == "many-custom" || name == "malformed-custom" || name == "opaque-custom" {
		d.f.git(d.repo, "fetch", "--no-write-fetch-head", donor, "+refs/hn/artifacts/*:refs/hn/artifacts/*")
		if name != "many-custom" {
			value := "not valid custom protocol"
			if name == "opaque-custom" {
				value = `{"protocol":"future-opaque/999","content":"touch ` + d.marker + `"}`
			}
			cmd := exec.Command("git", "hash-object", "-w", "--stdin")
			cmd.Dir = d.repo
			cmd.Env = d.f.env
			cmd.Stdin = strings.NewReader(value)
			raw, e := cmd.Output()
			if e != nil {
				d.t.Fatal(e)
			}
			d.f.git(d.repo, "update-ref", "refs/hn/artifacts/isolated-"+name, strings.TrimSpace(string(raw)))
		}
		return
	}
	if name == "absent" {
		return
	}
	child, e := os.Executable()
	if e != nil {
		d.t.Fatal(e)
	}
	raw, e := os.ReadFile(child)
	if e != nil {
		d.t.Fatal(e)
	}
	betaContributionWrite(d.t, d.module, raw, 0700)
	mode := "idle"
	switch name {
	case "incompatible":
		mode = "incompatible"
	case "crashed":
		mode = "crash"
	case "oversized":
		mode = "oversize"
	case "EOF":
		mode = "eof"
	case "blocked", "cancelled":
		mode = "startup-hang"
	}
	manifest := filepath.Join(d.t.TempDir(), "manifest.json")
	betaContributionWrite(d.t, manifest, []byte(`{"protocol":"hn.extension-manifest/1","module_id":"example.default/tool","module_version":"1","host_protocol":"hn.extension/1","capabilities":["capabilities.list","resource.read"]}`), 0600)
	d.f.hn(d.repo, true, "extension", "register", "--manifest", manifest, "--executable", d.module, "--arg", "-test.run=^TestExtensionDefaultModule$", "--arg", "--", "--arg", mode, "--arg", d.marker, "--json")
	if name != "disabled" {
		d.f.hn(d.repo, true, "extension", "enable", "example.default/tool", "--grant", "native:issue=read", "--json")
	}
	if name == "missing" {
		if e := os.Remove(d.module); e != nil {
			d.t.Fatal(e)
		}
	}
	if name == "malformed" {
		betaContributionWrite(d.t, d.registry(), []byte("{ malformed actual user registry"), 0600)
	}
	// The declaration is inert even when it is not parseable.
	betaContributionWrite(d.t, filepath.Join(d.repo, ".hn", "extensions.json"), []byte("{ inert malformed declaration"), 0600)
}

// This exact registered external executable emits the named protocol fault.
func TestExtensionDefaultModule(t *testing.T) {
	at := 0
	for i, a := range os.Args {
		if a == "--" {
			at = i + 1
			break
		}
	}
	if at == 0 {
		return
	}
	mode, marker := os.Args[at], os.Args[at+1]
	if e := os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0600); e != nil {
		os.Exit(80)
	}
	_ = hostRead(os.Stdin)
	switch mode {
	case "eof":
		os.Exit(0)
	case "incompatible":
		hostWrite(map[string]any{"protocol": "hn.extension/999", "ready": true})
	case "startup-hang":
		for {
			time.Sleep(time.Hour)
		}
	default:
		hostWrite(map[string]any{"protocol": "hn.extension/1", "ready": true})
		if mode == "crash" {
			os.Exit(19)
		}
		if mode == "oversize" {
			_, _ = os.Stdout.Write([]byte{0, 128, 0, 1})
		}
	}
	for {
		time.Sleep(time.Hour)
	}
}

// Explicit run controls establish the registered configurations are real faults,
// rather than labels placed on unused fixtures. Ordinary journeys above never
// run the module. These explicit invocations must reap their children.
func TestExtensionDefaultFailureConfigurations(t *testing.T) {
	for _, variant := range []string{"standard", "native-only"} {
		binary := defaultBinary(t, variant)
		for _, c := range []struct{ name, code string }{{"disabled", "denied_access"}, {"missing", "unavailable_module"}, {"incompatible", "incompatible_version"}, {"crashed", "unavailable_module"}, {"malformed", "invalid_input"}, {"oversized", "resource_limit"}, {"EOF", "unavailable_module"}, {"blocked", "cancelled"}, {"cancelled", "cancelled"}} {
			t.Run(variant+"/"+c.name, func(t *testing.T) {
				d := defaultNew(t, binary, "")
				d.configure(c.name, "")
				before := d.f.git(d.repo, "for-each-ref", "--format=%(refname) %(objectname)")
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, binary, "extension", "run", "example.default/tool", "--json")
				cmd.Dir, cmd.Env = d.repo, d.f.env
				var output bytes.Buffer
				cmd.Stdout, cmd.Stderr = &output, &output
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				if c.name == "cancelled" {
					until := time.Now().Add(5 * time.Second)
					for time.Now().Before(until) {
						if _, e := os.Stat(d.marker); e == nil {
							break
						}
						time.Sleep(10 * time.Millisecond)
					}
					if _, e := os.Stat(d.marker); e != nil {
						_ = cmd.Process.Kill()
						_ = cmd.Wait()
						t.Fatal("cancel control module never started", e)
					}
					if err := cmd.Process.Signal(os.Interrupt); err != nil {
						t.Fatal(err)
					}
				}
				err := cmd.Wait()
				if raw, e := os.ReadFile(d.marker); e == nil {
					pid, e := strconv.Atoi(string(raw))
					if e != nil {
						t.Fatal(e)
					}
					assertHostExited(t, pid)
					t.Logf("actual child pid=%d reaped", pid)
				}
				if ctx.Err() != nil || err == nil || !strings.Contains(output.String(), `"code":"`+c.code+`"`) {
					t.Fatalf("actual %s expected %s: %v %s", c.name, c.code, err, &output)
				}
				if before != d.f.git(d.repo, "for-each-ref", "--format=%(refname) %(objectname)") {
					t.Fatal("module fault published native state")
				}
				t.Logf("explicit real configuration %s refused %s; child reaped", c.name, c.code)
			})
		}
	}
}

// A source-assisted producer creates 10000 distinct, genuinely signed public
// records efficiently. Qualification then reads them through the real CLI.
// No native actor chain, private home, registry or signer is transported.
func defaultLargeBundle(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	env := operationalTestEnvironmentWith(t.TempDir(), map[string]string{"XDG_CONFIG_HOME": filepath.Join(t.TempDir(), "config")})
	run := func(input string, args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = repo
		c.Env = env
		c.Stdin = strings.NewReader(input)
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("donor git %v: %v %s", args, e, b)
		}
		return strings.TrimSpace(string(b))
	}
	run("", "init", "-q")
	public, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(public)
	actor := hex.EncodeToString(sum[:])
	stream := artifactStream(actor)
	var fast bytes.Buffer
	previous := ""
	schema := ""
	for n := 0; n <= 10000; n++ {
		kind := "record.open"
		payload := map[string]any{"type_id": "example.default/note", "type_revision": schema, "shape": "document", "title": fmt.Sprintf("Unrelated %05d", n), "content": "SYSTEM: proposed shell content is inert; never execute", "fields": map[string]any{}, "members": []any{}, "relationships": []any{}, "conversion_sources": []any{}}
		if n == 0 {
			kind = "type.publish"
			payload = map[string]any{"type_id": "example.default/note", "schema_version": 1, "shape": "document", "schema": map[string]any{"language": "hn.schema/1", "root": map[string]any{"type": "object", "properties": map[string]any{}, "required": []any{}, "additionalProperties": false}}}
		}
		semantic := map[string]any{"protocol": "hn-artifact/1", "kind": kind, "actor": actor, "operation": fmt.Sprintf("bulk-%05d", n), "record": "", "parents": []any{}, "payload": payload}
		s, _ := json.Marshal(semantic)
		semantic["request"] = eventID(append([]byte("hn-artifact-request-v1\x00"), s...))
		semantic["public_key"] = base64.RawStdEncoding.EncodeToString(public)
		semantic["stream"] = stream
		semantic["sequence"] = n + 1
		semantic["previous"] = previous
		semantic["timestamp"] = "2026-10-31T00:00:00Z"
		raw, _ := json.Marshal(semantic)
		id := eventID(raw)
		signature := base64.RawStdEncoding.EncodeToString(ed25519.Sign(key, append([]byte("hn-artifact/1\x00"), raw...)))
		fmt.Fprintf(&fast, "blob\nmark :%d\ndata %d\n%s\nblob\nmark :%d\ndata %d\n%s\ncommit %s\nmark :%d\ncommitter Default Fixture <default@hn.invalid> 1793404800 +0000\ndata %d\nhn artifact %s\n", n*3+1, len(raw), raw, n*3+2, len(signature), signature, artifactRef(actor), n*3+3, len("hn artifact "+id)+1, id)
		if n > 0 {
			fmt.Fprintf(&fast, "from :%d\n", n*3)
		}
		fmt.Fprintf(&fast, "deleteall\nM 100644 :%d artifact.json\nM 100644 :%d signature\n\n", n*3+1, n*3+2)
		previous = id
		if n == 0 {
			schema = id
		}
	}
	run(fast.String(), "fast-import", "--quiet")
	bundle := filepath.Join(t.TempDir(), "large.bundle")
	if *defaultDonor != "" {
		bundle = *defaultDonor
	}
	run("", "bundle", "create", bundle, artifactRef(actor))
	t.Logf("public donor records=10000 type=1 stream=%s schema=%s", artifactRef(actor), schema)
	return bundle
}

func TestExtensionDefaultPrepareCorpus(t *testing.T) {
	if *defaultDonor == "" {
		t.Skip("explicit external donor output required")
	}
	defaultLargeBundle(t)
}

func TestExtensionDefaultNativeMatrix(t *testing.T) {
	donor := defaultLargeBundle(t)
	for _, variant := range []string{"standard", "native-only"} {
		binary := defaultBinary(t, variant)
		for _, configuration := range defaultConfigurations {
			t.Run(variant+"/"+configuration, func(t *testing.T) {
				d := defaultNew(t, binary, "")
				d.configure(configuration, donor)
				for _, args := range [][]string{{"version"}, {"--help"}, {"extension", "help"}} {
					d.call(true, args...)
				}
				d.call(true, "init", "--name", "Default native owner")
				actor := d.f.actor(d.repo)
				policy := PolicyDocument{Version: policyVersion, Maintainers: []string{actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{"check": {RequiredResults: 1, TrustedRunners: []string{actor}}}, Memory: &MemoryPolicy{TrustedActors: []string{actor}, TrustedKinds: []string{"decision"}}}
				raw, _ := json.Marshal(policy)
				betaContributionWrite(t, filepath.Join(d.repo, ".hn", "policy.json"), raw, 0600)
				betaContributionWrite(t, filepath.Join(d.repo, ".hn", "pipelines", "check.json"), []byte(`{"version":"hn.pipeline/0","steps":[{"name":"check","command":"git","args":["--version"]}]}`), 0600)
				d.f.commit(d.repo, "Explicit native policy")
				issue := d.data("issue", "open", "--operation", "native-open", "--json", "Default issue")
				id := issue["issue_id"].(string)
				revision := issue["event_id"].(string)
				d.call(true, "issue", "show", id, "--json")
				d.call(true, "issue", "comment", id, "--body", "Preserved native comment", "--json")
				closed := d.data("issue", "close", id, "--expect", revision, "--operation", "native-close", "--json")["event_id"].(string)
				refused := d.f.git(d.repo, "show-ref")
				out := d.call(false, "issue", "close", id, "--expect", revision, "--operation", "stale-close", "--json")
				betaContributionContains(t, out, "stale_revision")
				if refused != d.f.git(d.repo, "show-ref") {
					t.Fatal("stale refusal changed refs")
				}
				reopened := d.data("issue", "reopen", id, "--expect", closed, "--json")["event_id"].(string)
				peerRepo := filepath.Join(t.TempDir(), "peer")
				d.f.git(d.repo, "clone", "-q", d.repo, peerRepo)
				d.f.git(peerRepo, "fetch", "--no-write-fetch-head", d.repo, "+refs/hn/actors/*:refs/hn/actors/*")
				peer := defaultNew(t, binary, peerRepo)
				peer.configure(configuration, donor)
				peer.call(true, "init", "--name", "Independent native peer")
				localState, _ := CanonicalIssueState(IssueState{Title: "Default issue local", Status: "open"})
				peerState, _ := CanonicalIssueState(IssueState{Title: "Default issue peer", Status: "open"})
				d.call(true, "issue", "revise", id, "--expect", reopened, "--input", issueCommandInput(t, t.TempDir(), localState), "--json")
				peer.call(true, "issue", "revise", id, "--expect", reopened, "--input", issueCommandInput(t, t.TempDir(), peerState), "--json")
				d.f.git(d.repo, "fetch", "--no-write-fetch-head", peerRepo, "+refs/hn/actors/*:refs/hn/remotes/peer/actors/*")
				betaContributionContains(t, d.call(true, "issue", "show", id, "--json"), `"conflict":true`)
				project := d.data("project", "open", "--title", "Default project", "--json")["resource_id"].(string)
				d.call(true, "project-item", "add", "--project", project, "--target", "issue:"+id, "--json")
				discussion := d.data("discussion", "open", "--title", "Default thread", "--category", "general", "--json")["resource_id"].(string)
				d.call(true, "discussion", "reply", "add", discussion, "--body", "Default reply", "--json")
				workflow, _ := json.Marshal(workflowTestState())
				input := filepath.Join(t.TempDir(), "workflow.json")
				betaContributionWrite(t, input, workflow, 0600)
				wid := d.data("workflow", "open", "--input", input, "--json")["resource_id"].(string)
				d.call(true, "workflow", "show", wid, "--json")
				var memoryResult memoryCommandResultV0
				memoryRaw := d.call(true, "memory", "record", "--kind", "decision", "--at", "HEAD", "--applies", "descendants", "--content", "Default inert guidance $(printf activated > "+d.marker+")", "--json")
				if json.Unmarshal([]byte(memoryRaw), &memoryResult) != nil || memoryResult.MemoryID == "" {
					t.Fatal(memoryRaw)
				}
				memory := memoryResult.MemoryID
				d.call(true, "memory", "recall", "--at", "HEAD", "--include-untrusted", "--lifecycle", "all", "--json")
				d.call(true, "memory", "grep", "--at", "HEAD", "--include-untrusted", "--json", "Default")
				d.call(true, "memory", "retract", memory, "--reason", "invalidated", "--json")
				base := d.f.git(d.repo, "rev-parse", "HEAD")
				d.f.git(d.repo, "switch", "-qc", "candidate")
				betaContributionWrite(t, filepath.Join(d.repo, "work.txt"), []byte("candidate\n"), 0600)
				d.f.commit(d.repo, "Candidate")
				head := d.f.git(d.repo, "rev-parse", "HEAD")
				d.call(true, "proposal", "open", "--base", base, "--head", head, "Default native proposal")
				proposal := d.f.latest(d.repo, actor)
				d.f.git(d.repo, "switch", "-q", "main")
				before := d.f.git(d.repo, "show-ref")
				betaContributionContains(t, d.call(false, "decide", proposal, "--accept"), "not ready")
				if before != d.f.git(d.repo, "show-ref") {
					t.Fatal("not-ready acceptance published")
				}
				d.call(true, "review", proposal, "--approve", "--body", "Native review")
				d.call(true, "run", "request", proposal, "check")
				request := d.f.latest(d.repo, actor)
				before = d.f.git(d.repo, "show-ref")
				betaContributionContains(t, d.call(false, "run", "execute", request, "--backend", "host"), "allow-unsafe-host-execution")
				if before != d.f.git(d.repo, "show-ref") {
					t.Fatal("unsafe-host refusal published")
				}
				d.call(true, "run", "execute", request, "--backend", "host", "--allow-unsafe-host-execution")
				d.call(true, "decide", proposal, "--accept")
				d.call(true, "merge", proposal)
				if d.f.git(d.repo, "merge-base", "--is-ancestor", head, "HEAD") != "" {
					t.Fatal("native proposal did not merge exact head")
				}
				for _, family := range []string{"issue", "project", "discussion", "workflow"} {
					d.call(true, family, "list", "--json")
				}
				// Export only public Git refs; a new keyless reader receives signed native
				// collaboration and memory without extension/private setup.
				bundle := filepath.Join(t.TempDir(), "native.bundle")
				d.f.git(d.repo, "bundle", "create", bundle, "--all")
				clone := filepath.Join(t.TempDir(), "clone")
				d.f.git(d.repo, "clone", "-q", bundle, clone)
				d.f.git(clone, "fetch", "--no-write-fetch-head", bundle, "+refs/hn/actors/*:refs/hn/actors/*", "+refs/hn/memory/*:refs/hn/memory/*")
				reader := defaultNew(t, binary, clone)
				reader.configure(configuration, donor)
				betaContributionContains(t, reader.call(true, "issue", "show", id, "--json"), "Default issue")
				reader.call(true, "memory", "recall", "--at", "HEAD", "--include-untrusted", "--lifecycle", "all", "--json")
				betaContributionContains(t, reader.call(false, "identity", "show", "--json"), "identity")
				t.Logf("native journey passed configuration=%s variant=%s syscall_observed=%t", configuration, variant, *extensionBaselineTrace != "")
			})
		}
	}
}

func TestExtensionDefaultHistoricalMatrix(t *testing.T) {
	donor := defaultLargeBundle(t)
	for _, variant := range []string{"standard", "native-only"} {
		binary := defaultBinary(t, variant)
		for _, configuration := range defaultConfigurations {
			for _, release := range []string{"0.0.1-dev", "0.0.2-dev", "0.0.3-dev"} {
				t.Run(variant+"/"+configuration+"/"+release, func(t *testing.T) {
					fixture, e := filepath.Abs(filepath.Join("testdata", "compatibility", release))
					if e != nil {
						t.Fatal(e)
					}
					var corpus betaHistoricalCorpus
					var expected map[string]any
					betaReadJSON(t, filepath.Join(fixture, "provenance.json"), &corpus)
					betaReadJSON(t, filepath.Join(fixture, "observations.json"), &expected)
					f := newBetaContributionCLI(t, binary)
					repo := filepath.Join(t.TempDir(), "reader")
					f.git(filepath.Dir(repo), "-c", "maintenance.auto=false", "-c", "gc.auto=0", "clone", "-q", filepath.Join(fixture, "public.bundle"), repo)
					f.git(repo, "fetch", "--no-write-fetch-head", filepath.Join(fixture, "public.bundle"), "+refs/hn/*:refs/hn/*")
					d := defaultNew(t, binary, repo)
					d.configure(configuration, donor)
					before := betaHistoricalObjects(t, repo, d.f.env, corpus.Refs)
					for _, family := range []string{"issue", "project", "project-item", "discussion", "discussion-reply", "workflow", "memory"} {
						var got any
						out := d.call(true, family, "show", corpus.IDs[family], "--json")
						if json.Unmarshal([]byte(out), &got) != nil {
							t.Fatal(out)
						}
						betaSemanticSubset(t, "", expected[family], got)
					}
					for _, args := range [][]string{{"memory", "recall", "--at", "HEAD", "--include-untrusted", "--lifecycle", "all", "--json"}, {"memory", "grep", "--at", "HEAD", "--include-untrusted", "--lifecycle", "all", "--json", "[Hh]istorical"}} {
						d.call(true, args...)
					}
					betaAssertHistoricalRefs(t, repo, d.f.env, corpus.Refs)
					if !bytes.Equal(before, betaHistoricalObjects(t, repo, d.f.env, corpus.Refs)) {
						t.Fatal("native signed bytes/IDs/signatures changed")
					}
					t.Logf("historical exact object bytes release=%s sha256=%x", release, sha256.Sum256(before))
					d.call(false, "identity", "show", "--json")
					for _, name := range []string{"identity.json", "identities", "active", "rotation.json", "keyring"} {
						if _, e := os.Stat(filepath.Join(d.common, "hn", name)); !os.IsNotExist(e) {
							t.Fatal("keyless read created identity", name, e)
						}
					}
				})
			}
		}
	}
}

func TestExtensionDefaultControl(t *testing.T) {
	mode := os.Getenv("HN_DEFAULT_CONTROL")
	if mode == "" {
		return
	}
	var err error
	switch mode {
	case "registry-read":
		_, err = os.ReadFile(os.Getenv("HN_DEFAULT_TARGET"))
	case "module-probe":
		_, err = os.Stat(os.Getenv("HN_DEFAULT_TARGET"))
	case "module-start":
		c := exec.Command(os.Getenv("HN_DEFAULT_TARGET"), "-test.run=^TestExtensionDefaultControl$")
		c.Env = append(os.Environ(), "HN_DEFAULT_CONTROL=noop")
		err = c.Run()
	case "provider-connect":
		var c net.Conn
		c, err = net.DialTimeout("tcp", os.Getenv("HN_DEFAULT_TARGET"), time.Second)
		if err == nil {
			err = c.Close()
		}
	case "extension-state-write":
		p := os.Getenv("HN_DEFAULT_TARGET")
		err = os.WriteFile(p, []byte("transient private effect"), 0600)
		if err == nil {
			err = os.Remove(p)
		}
	case "noop":
	default:
		err = fmt.Errorf("unknown default control")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func TestExtensionDefaultDetectors(t *testing.T) {
	if *extensionBaselineTrace == "" {
		t.Skip("explicit OS trace executable required; no syscall proof without it")
	}
	d := defaultNew(t, defaultBinary(t, "standard"), "")
	t.Run("data-activation", func(t *testing.T) {
		command := exec.Command("/bin/sh", "-c", `printf activated > "$1"`, "activation-control", d.marker)
		command.Dir, command.Env = d.repo, d.f.env
		if out, e := command.CombinedOutput(); e != nil {
			t.Fatal(e, string(out))
		}
		if !d.activated() {
			t.Fatal("detector missed real data activation")
		}
		if e := os.Remove(d.marker); e != nil {
			t.Fatal(e)
		}
	})
	self, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(self)
	if e != nil {
		t.Fatal(e)
	}
	betaContributionWrite(t, d.module, raw, 0700)
	for _, root := range d.roots()[:3] {
		if e := os.MkdirAll(root, 0700); e != nil {
			t.Fatal(e)
		}
	}
	betaContributionWrite(t, d.registry(), []byte("malformed registry"), 0600)
	betaContributionWrite(t, filepath.Join(d.common, "hn", "extensions", "enabled.json"), []byte("malformed enablement"), 0600)
	betaContributionWrite(t, filepath.Join(d.repo, ".hn", "extensions.json"), []byte("declaration"), 0600)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	cases := []struct{ cause, target string }{{"module-probe", d.module}, {"module-start", d.module}, {"provider-connect", listener.Addr().String()}}
	for _, p := range []string{d.registry(), filepath.Join(d.common, "hn", "extensions", "enabled.json"), filepath.Join(d.repo, ".hn", "extensions.json")} {
		relative, _ := filepath.Rel(d.repo, p)
		cases = append(cases, struct{ cause, target string }{"registry-read", p}, struct{ cause, target string }{"registry-read", relative})
	}
	for _, root := range d.roots()[:3] {
		cases = append(cases, struct{ cause, target string }{"extension-state-write", filepath.Join(root, "transient-control")})
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("%s-%d", c.cause, i), func(t *testing.T) {
			trace := filepath.Join(t.TempDir(), "control.trace")
			env := append(append([]string{}, d.f.env...), "HN_DEFAULT_CONTROL="+c.cause, "HN_DEFAULT_TARGET="+c.target)
			cmd := exec.Command(*extensionBaselineTrace, "-f", "-qq", "-yy", "-s", "4096", "-e", "trace=%file,process,network", "-o", trace, "--", self, "-test.run=^TestExtensionDefaultControl$")
			cmd.Dir = d.repo
			cmd.Env = env
			before := d.snapshot()
			out, e := cmd.CombinedOutput()
			if e != nil {
				t.Fatal(e, string(out))
			}
			raw, e := os.ReadFile(trace)
			if e != nil {
				t.Fatal(e)
			}
			if *defaultEvidence != "" {
				if e := os.MkdirAll(*defaultEvidence, 0700); e != nil {
					t.Fatal(e)
				}
				file, e := os.CreateTemp(*defaultEvidence, "control-"+c.cause+"-*.trace")
				if e != nil {
					t.Fatal(e)
				}
				_, _ = file.Write(raw)
				file.Close()
			}
			if !d.violations(string(raw))[c.cause] {
				t.Fatalf("detector missed actual %s %s", c.cause, c.target)
			}
			if c.cause == "extension-state-write" && !reflect.DeepEqual(before, d.snapshot()) {
				t.Fatal("transient control did not restore net state")
			}
			t.Logf("detected exact cause=%s; net state equality alone is insufficient", c.cause)
		})
	}
}
