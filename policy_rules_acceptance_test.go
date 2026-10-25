package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// All claims in these scenarios originate from the actual CLI. Fixture maps and
// public Git event payload inspection deliberately avoid private event writers.
type policyPublicFixture struct {
	cli          map[string]interoperabilityCLI
	actors       map[string]operationalIdentity
	remote, base string
	sequence     int
}

func policyPublicFile(t *testing.T, repo, name string, content []byte) {
	t.Helper()
	target := filepath.Join(repo, name)
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, content, 0600); err != nil {
		t.Fatal(err)
	}
}

func newPolicyPublicFixture(t *testing.T, binary string, protect, automatic, pipeline, nullOptions bool) *policyPublicFixture {
	t.Helper()
	seed := filepath.Join(t.TempDir(), "seed")
	initOperationalRepository(t, seed, "Seed")
	runOperationalGit(t, seed, "commit", "--allow-empty", "-q", "-m", "seed")
	f := &policyPublicFixture{cli: map[string]interoperabilityCLI{}, actors: map[string]operationalIdentity{}, remote: filepath.Join(t.TempDir(), "project.git")}
	runOperationalGit(t, "", "clone", "--bare", "-q", seed, f.remote)
	for _, role := range []string{"owner", "author", "reviewer", "guardian"} {
		repo := filepath.Join(t.TempDir(), role)
		runOperationalGit(t, "", "clone", "--no-local", "-q", f.remote, repo)
		configureOperationalGit(t, repo, role)
		cli := interopFixture(t, binary, repo)
		cli.call(t, false, "init", "--name", role)
		public := cli.call(t, false, "identity", "public")
		f.cli[role] = cli
		f.actors[role] = operationalIdentity{Actor: operationalLabel(t, public, "Actor:"), PublicKey: operationalLabel(t, public, "Public key:")}
		if _, err := os.Stat(filepath.Join(repo, ".hn", "policy.json")); !os.IsNotExist(err) {
			t.Fatalf("init unexpectedly installed policy: %v", err)
		}
	}
	policy := map[string]any{
		"version": "hn.policy/0", "maintainers": []string{f.actors["owner"].Actor},
		"proposals": map[string]any{"requiredApprovals": 1, "requiredAccepts": 1, "trustedReviewers": []string{f.actors["reviewer"].Actor}, "allowAuthorApproval": false},
		"pipelines": map[string]any{},
	}
	if nullOptions {
		policy["protectedChecks"] = nil
		policy["automerge"] = nil
	}
	if protect {
		policy["protectedChecks"] = map[string]any{"paths": []string{"lint.conf", "scripts/lint/"}, "trustedReviewers": []string{f.actors["guardian"].Actor}, "requiredApprovals": 1}
	}
	if automatic {
		policy["automerge"] = map[string]any{"rules": []map[string]any{
			{"name": "docs-only", "targetBranch": "main", "actors": []string{f.actors["owner"].Actor}, "authors": []string{f.actors["author"].Actor}, "paths": []string{"docs/"}},
			{"name": "maintenance", "targetBranch": "main", "actors": []string{f.actors["owner"].Actor}, "authors": []string{f.actors["author"].Actor}, "paths": []string{"lint.conf", ".hn/policy.json", ".hn/pipelines/"}},
		}}
	}
	if pipeline {
		policy["pipelines"] = map[string]any{"lint": map[string]any{"requiredResults": 1, "trustedRunners": []string{f.actors["owner"].Actor}}}
	}
	encoded, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	owner := f.cli["owner"]
	policyPublicFile(t, owner.repo, ".hn/policy.json", append(encoded, '\n'))
	policyPublicFile(t, owner.repo, "lint.conf", []byte("strict=true\n"))
	policyPublicFile(t, owner.repo, "docs/base.md", []byte("Base documentation\n"))
	if pipeline {
		policyPublicFile(t, owner.repo, ".hn/pipelines/lint.json", []byte(`{"version":"hn.pipeline/0","steps":[{"name":"Lint","command":"go","args":["vet","./..."]}]}`))
	}
	runOperationalGit(t, owner.repo, "add", ".hn", "lint.conf", "docs/base.md")
	runOperationalGit(t, owner.repo, "commit", "-q", "-m", "explicit repository policy")
	f.base = runOperationalGit(t, owner.repo, "rev-parse", "HEAD")
	runOperationalGit(t, owner.repo, "push", "-q", "origin", "main")
	for _, role := range []string{"author", "reviewer", "guardian"} {
		runOperationalGit(t, f.cli[role].repo, "fetch", "-q", "origin", "main")
		runOperationalGit(t, f.cli[role].repo, "merge", "--ff-only", "origin/main")
	}
	return f
}

func (f *policyPublicFixture) open(t *testing.T, changes map[string][]byte) operationalEvent {
	t.Helper()
	return f.openFrom(t, f.base, f.base, changes)
}

func (f *policyPublicFixture) openFrom(t *testing.T, start, base string, changes map[string][]byte) operationalEvent {
	t.Helper()
	f.sequence++
	author := f.cli["author"]
	runOperationalGit(t, author.repo, "switch", "-q", "-c", fmt.Sprintf("candidate-%d", f.sequence), start)
	names := make([]string, 0, len(changes))
	for name := range changes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		policyPublicFile(t, author.repo, name, changes[name])
		runOperationalGit(t, author.repo, "add", "--", name)
	}
	runOperationalGit(t, author.repo, "commit", "-q", "-m", "candidate")
	head := runOperationalGit(t, author.repo, "rev-parse", "HEAD")
	author.call(t, false, "proposal", "open", "--base", base, "--head", head, "Public policy acceptance")
	proposal := latestOperationalEvent(t, author.repo, operationalActorRef(f.actors["author"].Actor))
	if proposal.Kind != "proposal.open" || proposal.Base != base || proposal.Head != head {
		t.Fatalf("bad public candidate binding: %+v", proposal)
	}
	runOperationalGit(t, author.repo, "switch", "-q", "main")
	author.call(t, false, "sync", "origin")
	return proposal
}

func (f *policyPublicFixture) review(t *testing.T, role string, proposal operationalEvent) operationalEvent {
	t.Helper()
	cli := f.cli[role]
	cli.call(t, false, "sync", "origin")
	cli.call(t, false, "review", proposal.ID, "--approve", "--body", "Reviewed the exact candidate")
	review := latestOperationalEvent(t, cli.repo, operationalActorRef(f.actors[role].Actor))
	if review.Kind != "review.submit" || review.Subject != proposal.ID {
		t.Fatal("review did not bind exact public candidate")
	}
	cli.call(t, false, "sync", "origin")
	return review
}

func (f *policyPublicFixture) accept(t *testing.T, proposal operationalEvent) operationalEvent {
	t.Helper()
	owner := f.cli["owner"]
	owner.call(t, false, "sync", "origin")
	owner.call(t, false, "decide", proposal.ID, "--accept")
	decision := latestOperationalEvent(t, owner.repo, operationalActorRef(f.actors["owner"].Actor))
	if decision.Kind != "proposal.decision" || decision.Subject != proposal.ID {
		t.Fatal("acceptance did not bind exact public candidate")
	}
	return decision
}

func (f *policyPublicFixture) publish(t *testing.T) {
	t.Helper()
	owner := f.cli["owner"]
	owner.call(t, false, "sync", "origin")
	runOperationalGit(t, owner.repo, "push", "-q", "origin", "main")
	f.base = runOperationalGit(t, owner.repo, "rev-parse", "main")
	for _, role := range []string{"author", "reviewer", "guardian"} {
		runOperationalGit(t, f.cli[role].repo, "fetch", "-q", "origin", "main")
		runOperationalGit(t, f.cli[role].repo, "merge", "--ff-only", "origin/main")
	}
}

func policyPublicSnapshot(t *testing.T, repo string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	if err := filepath.Walk(repo, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			snapshot[relative+"/"] = info.Mode().String()
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		snapshot[relative] = info.Mode().String() + ":" + hex.EncodeToString(sum[:])
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func policyPublicNoWrite(t *testing.T, cli interoperabilityCLI, failure bool, args ...string) string {
	t.Helper()
	before := policyPublicSnapshot(t, cli.repo)
	output := cli.call(t, failure, args...)
	if after := policyPublicSnapshot(t, cli.repo); !reflect.DeepEqual(before, after) {
		t.Fatalf("read-only/refused command %v changed repository files", args)
	}
	return output
}

func policyPublicRemote(t *testing.T, repo string) string {
	t.Helper()
	return runOperationalGit(t, repo, "ls-remote", "--refs", "origin")
}

func policyPublicAssertMerged(t *testing.T, f *policyPublicFixture, proposal, decision operationalEvent) operationalEvent {
	t.Helper()
	owner := f.cli["owner"]
	merged := latestOperationalEvent(t, owner.repo, operationalActorRef(f.actors["owner"].Actor))
	current := runOperationalGit(t, owner.repo, "rev-parse", "HEAD")
	if merged.Kind != "proposal.merged" || merged.Actor != f.actors["owner"].Actor || merged.Subject != proposal.ID || merged.Head != proposal.Head || merged.Commit != current || merged.Policy != decision.Policy || !reflect.DeepEqual(merged.Evidence, []string{decision.ID}) {
		t.Fatalf("incorrect public merge evidence: %+v", merged)
	}
	parents := strings.Fields(runOperationalGit(t, owner.repo, "show", "-s", "--format=%P", "HEAD"))
	if !reflect.DeepEqual(parents, []string{proposal.Base, proposal.Head}) {
		t.Fatalf("incorrect merge parents: %q", parents)
	}
	if status := runOperationalGit(t, owner.repo, "--no-optional-locks", "status", "--porcelain"); status != "" {
		t.Fatalf("dirty merged checkout: %s", status)
	}
	return merged
}

func policyPublicKeylessClone(t *testing.T, f *policyPublicFixture, binary string) interoperabilityCLI {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "keyless")
	runOperationalGit(t, "", "clone", "--no-local", "-q", f.remote, repo)
	configureOperationalGit(t, repo, "Keyless")
	cli := interopFixture(t, binary, repo)
	cli.call(t, false, "sync", "origin")
	for _, name := range []string{"active", "identities", "identity.json"} {
		if _, err := os.Stat(filepath.Join(repo, ".git", "hn", name)); !os.IsNotExist(err) {
			t.Fatalf("keyless clone has identity state %s: %v", name, err)
		}
	}
	return cli
}

func TestPolicyRulesPublicAcceptance(t *testing.T) {
	normal := buildOperationalBinary(t)
	native := filepath.Join(t.TempDir(), "hn-native")
	build := exec.Command("go", "build", "-tags", "nogithub", "-o", native, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	for _, variant := range []struct{ name, binary string }{{"default", normal}, {"nogithub", native}} {
		t.Run(variant.name, func(t *testing.T) {
			for _, nullOptions := range []bool{false, true} {
				name := "absent-defaults"
				if nullOptions {
					name = "null-disabled"
				}
				t.Run(name, func(t *testing.T) {
					f := newPolicyPublicFixture(t, variant.binary, false, false, false, nullOptions)
					owner := f.cli["owner"]
					show := owner.call(t, false, "policy", "show")
					assertOperationalContains(t, show, "Protected checks: disabled", "Automerge: disabled")
					proposal := f.open(t, map[string][]byte{"docs/change.md": []byte("Ordinary manual work\n")})
					f.review(t, "reviewer", proposal)
					decision := f.accept(t, proposal)
					out := policyPublicNoWrite(t, owner, true, "automerge", proposal.ID, "--rule", "docs-only")
					assertOperationalContains(t, out, "disabled")
					owner.call(t, false, "merge", proposal.ID)
					policyPublicAssertMerged(t, f, proposal, decision)
				})
			}
			t.Run("protection-only", func(t *testing.T) {
				f := newPolicyPublicFixture(t, variant.binary, true, false, true, false)
				owner := f.cli["owner"]
				show := owner.call(t, false, "policy", "show")
				assertOperationalContains(t, show, "Protected checks: enabled", "Automerge: disabled", ".hn/pipelines/lint.json")
				proposal := f.open(t, map[string][]byte{".hn/pipelines/lint.json": []byte(`{"version":"hn.pipeline/0","steps":[{"name":"No-op lint","command":"git","args":["--version"]}]}`)})
				owner.call(t, false, "sync", "origin")
				owner.call(t, false, "run", "request", proposal.ID, "lint")
				request := latestOperationalEvent(t, owner.repo, operationalActorRef(f.actors["owner"].Actor))
				owner.call(t, false, "run", "execute", request.ID, "--backend", "host", "--allow-unsafe-host-execution")
				result := latestOperationalEvent(t, owner.repo, operationalActorRef(f.actors["owner"].Actor))
				if result.Kind != "run.result" || result.Subject != request.ID || result.Commit != proposal.Head || result.Definition != request.Definition {
					t.Fatalf("run evidence mismatch: %+v", result)
				}
				owner.call(t, false, "sync", "origin")
				ordinary := f.review(t, "reviewer", proposal)
				owner.call(t, false, "sync", "origin")
				status := owner.call(t, false, "proposal", "status", proposal.ID)
				assertOperationalContains(t, status, "0/1 designated approvals", ".hn/pipelines/lint.json", "1/1 trusted passes")
				policyPublicNoWrite(t, owner, true, "decide", proposal.ID, "--accept")
				guardian := f.review(t, "guardian", proposal)
				decision := f.accept(t, proposal)
				expected := []string{ordinary.ID, guardian.ID, result.ID}
				sort.Strings(expected)
				if !reflect.DeepEqual(decision.Evidence, expected) {
					t.Fatalf("decision does not bind exact public evidence IDs: %q", decision.Evidence)
				}
				owner.call(t, false, "sync", "origin")
				keyless := policyPublicKeylessClone(t, f, variant.binary)
				assertOperationalContains(t, keyless.call(t, false, "proposal", "status", proposal.ID), "1/1 designated approvals", "accepted")
				remoteBefore := policyPublicRemote(t, owner.repo)
				owner.call(t, false, "merge", proposal.ID)
				policyPublicAssertMerged(t, f, proposal, decision)
				if after := policyPublicRemote(t, owner.repo); after != remoteBefore {
					t.Fatal("manual local merge unexpectedly published")
				}
			})
			t.Run("automerge-only", func(t *testing.T) { policyPublicAutomatic(t, variant.binary, false) })
			t.Run("both-opt-ins", func(t *testing.T) { policyPublicAutomatic(t, variant.binary, true) })
		})
	}
}

func policyPublicAutomatic(t *testing.T, binary string, protected bool) {
	t.Helper()
	f := newPolicyPublicFixture(t, binary, protected, true, false, false)
	owner := f.cli["owner"]
	proposal := f.open(t, map[string][]byte{"docs/change.md": []byte("Automatic documentation\n")})
	owner.call(t, false, "sync", "origin")
	assertOperationalContains(t, policyPublicNoWrite(t, owner, true, "automerge", proposal.ID, "--rule", "docs-only"), "acceptances are missing")
	f.review(t, "reviewer", proposal)
	decision := f.accept(t, proposal)
	owner.call(t, false, "sync", "origin")
	keyless := policyPublicKeylessClone(t, f, binary)
	preview := policyPublicNoWrite(t, keyless, false, "automerge", proposal.ID, "--rule", "docs-only")
	assertOperationalContains(t, preview, "Eligible:", proposal.ID, proposal.Base, proposal.Head, decision.Policy, f.actors["owner"].Actor)
	policyPublicNoWrite(t, keyless, true, "automerge", proposal.ID, "--rule", "docs-only", "--execute")
	// Existing Git commands create real in-progress metadata; preview must not
	// finish/abort it or rewrite its index while refusing automation.
	runOperationalGit(t, keyless.repo, "merge", "--no-ff", "--no-commit", proposal.Head)
	assertOperationalContains(t, policyPublicNoWrite(t, keyless, true, "automerge", proposal.ID, "--rule", "docs-only"), "unfinished Git operation")
	runOperationalGit(t, keyless.repo, "merge", "--abort")
	runOperationalGit(t, keyless.repo, "config", "remote.origin.promisor", "true")
	assertOperationalContains(t, policyPublicNoWrite(t, keyless, true, "automerge", proposal.ID, "--rule", "docs-only"), "partial/promisor")
	runOperationalGit(t, keyless.repo, "config", "--unset", "remote.origin.promisor")
	shallow := filepath.Join(t.TempDir(), "shallow")
	runOperationalGit(t, "", "clone", "--depth=1", "--no-local", "-q", f.remote, shallow)
	shallowCLI := interopFixture(t, binary, shallow)
	assertOperationalContains(t, policyPublicNoWrite(t, shallowCLI, true, "automerge", proposal.ID, "--rule", "docs-only"), "full repository")
	unauthorized := f.cli["author"]
	unauthorized.call(t, false, "sync", "origin")
	assertOperationalContains(t, policyPublicNoWrite(t, unauthorized, true, "automerge", proposal.ID, "--rule", "docs-only", "--execute"), "not an executing actor")
	actorBefore := runOperationalGit(t, owner.repo, "rev-parse", operationalActorRef(f.actors["owner"].Actor))
	remoteBefore := policyPublicRemote(t, owner.repo)
	owner.call(t, false, "automerge", proposal.ID, "--rule", "docs-only", "--execute")
	merge := policyPublicAssertMerged(t, f, proposal, decision)
	if merge.Previous != decision.ID {
		t.Fatal("automerge manufactured a review/acceptance instead of appending only the merge fact")
	}
	parent := runOperationalGit(t, owner.repo, "rev-parse", operationalActorRef(f.actors["owner"].Actor)+"^")
	if parent != actorBefore {
		t.Fatal("automerge appended additional unsigned or approval facts")
	}
	if after := policyPublicRemote(t, owner.repo); after != remoteBefore {
		t.Fatal("automerge published without explicit sync/push")
	}
	policyPublicNoWrite(t, owner, true, "automerge", proposal.ID, "--rule", "docs-only", "--execute")
	f.publish(t)
	if protected {
		// Proposed policy cannot remove its own base-policy protected approval gate.
		raw, err := os.ReadFile(filepath.Join(owner.repo, ".hn/policy.json"))
		if err != nil {
			t.Fatal(err)
		}
		var proposed map[string]any
		if err := json.Unmarshal(raw, &proposed); err != nil {
			t.Fatal(err)
		}
		delete(proposed, "protectedChecks")
		withoutProtection, err := json.Marshal(proposed)
		if err != nil {
			t.Fatal(err)
		}
		removal := f.open(t, map[string][]byte{".hn/policy.json": withoutProtection})
		f.review(t, "reviewer", removal)
		owner.call(t, false, "sync", "origin")
		assertOperationalContains(t, owner.call(t, false, "proposal", "status", removal.ID), ".hn/policy.json", "0/1 designated approvals")
		policyPublicNoWrite(t, owner, true, "decide", removal.ID, "--accept")
	}
	// A rule explicitly permits these files, so refusal cannot be explained by
	// a docs-only allowlist mismatch. Policy/protected changes are always manual.
	var changes map[string][]byte
	if protected {
		changes = map[string][]byte{"lint.conf": []byte("strict=false\n")}
	} else {
		raw, err := os.ReadFile(filepath.Join(owner.repo, ".hn/policy.json"))
		if err != nil {
			t.Fatal(err)
		}
		changes = map[string][]byte{".hn/policy.json": append(raw, '\n')}
	}
	manual := f.open(t, changes)
	f.review(t, "reviewer", manual)
	if protected {
		f.review(t, "guardian", manual)
	}
	manualDecision := f.accept(t, manual)
	reason := "policy amendments require manual merge"
	if protected {
		reason = "protected check changes require manual merge"
	}
	assertOperationalContains(t, policyPublicNoWrite(t, owner, true, "automerge", manual.ID, "--rule", "maintenance", "--execute"), reason)
	// Simulate code merge completed before its event using ordinary Git; repair
	// must append only its missing native fact, never a second code merge.
	runOperationalGit(t, owner.repo, "merge", "--no-ff", "-m", "Simulated interrupted native merge", manual.Head)
	mergedHead := runOperationalGit(t, owner.repo, "rev-parse", "HEAD")
	owner.call(t, false, "merge", manual.ID)
	policyPublicAssertMerged(t, f, manual, manualDecision)
	if after := runOperationalGit(t, owner.repo, "rev-parse", "HEAD"); after != mergedHead {
		t.Fatal("repair created another code merge")
	}
	f.publish(t)
	stale := f.open(t, map[string][]byte{"docs/stale.md": []byte("Preview cannot authorize a changed target\n")})
	f.review(t, "reviewer", stale)
	f.accept(t, stale)
	policyPublicNoWrite(t, owner, false, "automerge", stale.ID, "--rule", "docs-only")
	runOperationalGit(t, owner.repo, "commit", "--allow-empty", "-q", "-m", "advance target after preview")
	assertOperationalContains(t, policyPublicNoWrite(t, owner, true, "automerge", stale.ID, "--rule", "docs-only", "--execute"), "target no longer equals")
	if protected {
		// A signed B-to-H diff can hide restoration of a protected file tightened
		// on T. The current-target gate must stop it even with identical policies.
		f.publish(t)
		base := f.base
		original, err := os.ReadFile(filepath.Join(owner.repo, "lint.conf"))
		if err != nil {
			t.Fatal(err)
		}
		tightened := append(append([]byte(nil), original...), []byte("additional=true\n")...)
		policyPublicFile(t, owner.repo, "lint.conf", tightened)
		runOperationalGit(t, owner.repo, "add", "lint.conf")
		runOperationalGit(t, owner.repo, "commit", "-q", "-m", "tighten current protected baseline")
		f.publish(t)
		hidden := f.openFrom(t, f.base, base, map[string][]byte{"lint.conf": original, "docs/hidden.md": []byte("Old signed diff hides lint reversion\n")})
		f.review(t, "reviewer", hidden)
		hiddenDecision := f.accept(t, hidden)
		currentPolicy := owner.call(t, false, "policy", "show")
		if operationalLabel(t, currentPolicy, "Policy digest:") != hiddenDecision.Policy {
			t.Fatal("baseline regression accidentally changed policy bytes")
		}
		assertOperationalContains(t, policyPublicNoWrite(t, owner, true, "merge", hidden.ID), "protected check baseline changed", "revise")
		actual, err := os.ReadFile(filepath.Join(owner.repo, "lint.conf"))
		if err != nil || !reflect.DeepEqual(actual, tightened) {
			t.Fatal("refusal did not preserve target tightening")
		}
		// A different exact target policy also independently requires revision.
		raw, err := os.ReadFile(filepath.Join(owner.repo, ".hn/policy.json"))
		if err != nil {
			t.Fatal(err)
		}
		policyPublicFile(t, owner.repo, ".hn/policy.json", append(raw, '\n'))
		runOperationalGit(t, owner.repo, "add", ".hn/policy.json")
		runOperationalGit(t, owner.repo, "commit", "-q", "-m", "change current target policy bytes")
		assertOperationalContains(t, policyPublicNoWrite(t, owner, true, "merge", hidden.ID), "different policy", "revise")
	}
}
