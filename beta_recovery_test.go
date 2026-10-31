package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Recovery fixtures use only newly generated disposable identities. No command
// inherits HOME, credentials, Git configuration, or the operator's keyring.
type betaRecoveryCLI struct {
	t                 *testing.T
	binary, git, root string
	env               []string
}

func (c betaRecoveryCLI) gitAt(repo string, args ...string) string {
	c.t.Helper()
	return strings.TrimSpace(portablePublic(c.t, c.git, repo, c.env, true, args...))
}
func (c betaRecoveryCLI) hn(repo string, ok bool, args ...string) string {
	c.t.Helper()
	return portablePublic(c.t, c.binary, repo, c.env, ok, args...)
}
func (c betaRecoveryCLI) identity(repo string) operationalIdentity {
	c.t.Helper()
	out := c.hn(repo, true, "identity", "public")
	return operationalIdentity{Actor: operationalLabel(c.t, out, "Actor:"), PublicKey: operationalLabel(c.t, out, "Public key:")}
}
func (c betaRecoveryCLI) authorize(repo string, target operationalIdentity, ok bool) string {
	c.t.Helper()
	return c.hn(repo, ok, "identity", "authorize", "--relationship", "device", "--actor", target.Actor, "--public-key", target.PublicKey)
}
func (c betaRecoveryCLI) record(repo, text, stream string, ok bool) string {
	c.t.Helper()
	args := []string{"memory", "record", "--kind", "decision", "--at", "HEAD", "--applies", "descendants", "--content", text, "--json"}
	if stream != "" {
		args = append(args, "--stream", stream)
	}
	return c.hn(repo, ok, args...)
}
func betaRecoveryID(t *testing.T, raw string) string {
	t.Helper()
	var v struct {
		MemoryID string `json:"memoryId"`
		Data     struct {
			ID      string
			IssueID string `json:"issue_id"`
		}
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("missing public ID: %s (%v)", raw, err)
	}
	if v.Data.ID == "" {
		v.Data.ID = v.Data.IssueID
		if v.Data.ID == "" {
			v.Data.ID = v.MemoryID
		}
	}
	if v.Data.ID == "" {
		t.Fatalf("missing public ID: %s", raw)
	}
	return v.Data.ID
}
func (c betaRecoveryCLI) noWrite(repo string, args ...string) string {
	c.t.Helper()
	before := policyPublicSnapshot(c.t, repo)
	out := c.hn(repo, false, args...)
	if !reflect.DeepEqual(before, policyPublicSnapshot(c.t, repo)) {
		c.t.Fatalf("refused recovery command %v changed objects, refs, files or private state", args)
	}
	return out
}
func (c betaRecoveryCLI) noAppend(repo string, target operationalIdentity, stream string) {
	c.t.Helper()
	c.t.Run("actor-refusal", func(t *testing.T) {
		c := c
		c.t = t
		out := c.noWrite(repo, "identity", "authorize", "--relationship", "device", "--actor", target.Actor, "--public-key", target.PublicKey)
		if !strings.Contains(out, "signing root") {
			t.Fatalf("wrong actor refusal: %s", out)
		}
	})
	c.t.Run("memory-refusal", func(t *testing.T) {
		c := c
		c.t = t
		args := []string{"memory", "record", "--kind", "decision", "--at", "HEAD", "--applies", "descendants", "--content", "must refuse", "--stream", stream, "--json"}
		if out := c.noWrite(repo, args...); !strings.Contains(out, "signing root") {
			t.Fatalf("wrong memory refusal: %s", out)
		}
	})
}
func betaCopyPrivate(t *testing.T, source, destination string, actor string) {
	t.Helper()
	// Exactly the completed keyring backup recipe: no authority, caches or journals.
	for _, name := range []string{"active", filepath.Join("identities", actor+".json")} {
		raw, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Dir(filepath.Join(destination, name))
		if err = os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(destination, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func (c betaRecoveryCLI) refs(repo string) map[string]string {
	c.t.Helper()
	refs := map[string]string{}
	for _, line := range strings.Split(c.gitAt(repo, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn", "refs/heads/main"), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 {
			refs[f[0]] = f[1]
		}
	}
	return refs
}
func (c betaRecoveryCLI) clone(bundle, name string) string {
	c.t.Helper()
	repo := filepath.Join(c.root, name)
	c.gitAt(c.root, "clone", "-q", "--branch", "main", bundle, repo)
	c.gitAt(repo, "config", "user.name", "Recovery rehearsal")
	c.gitAt(repo, "config", "user.email", "recovery@hn.invalid")
	return repo
}
func (c betaRecoveryCLI) assertKeyless(repo string) {
	c.t.Helper()
	c.noWrite(repo, "identity", "show", "--json")
	for _, name := range []string{"active", "identities", "identity.json", "rotation.json", "rotation-command.json", "audit/authority.json", "automerge"} {
		if _, err := os.Lstat(filepath.Join(repo, ".git", "hn", name)); !os.IsNotExist(err) {
			c.t.Fatalf("public recovery installed %s: %v", name, err)
		}
	}
}

func TestBetaRecoveryJourneys(t *testing.T) {
	for _, variant := range []struct{ name, tags string }{{"standard", ""}, {"native-only", "noforge"}} {
		t.Run(variant.name, func(t *testing.T) {
			binary := buildMigrationCLIBinary(t, variant.tags)
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			gitOnly := t.TempDir()
			if err = os.Symlink(git, filepath.Join(gitOnly, "git")); err != nil {
				t.Fatal(err)
			}
			c := betaRecoveryCLI{t: t, binary: binary, git: git, root: t.TempDir(), env: []string{"PATH=" + gitOnly, "HOME=" + t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS="}}
			source := filepath.Join(c.root, "source")
			c.gitAt(c.root, "init", "-q", "-b", "main", source)
			c.gitAt(source, "config", "user.name", "Original writer")
			c.gitAt(source, "config", "user.email", "original@hn.invalid")
			c.gitAt(source, "commit", "--allow-empty", "-qm", "Public code")
			c.hn(source, true, "init", "--name", "Original writer")
			original := c.identity(source)
			targetRepo := c.clone(source, "target")
			c.hn(targetRepo, true, "init", "--name", "Distinct device")
			target := c.identity(targetRepo)
			issue := betaRecoveryID(t, c.hn(source, true, "issue", "open", "--json", "Retained issue"))
			memory := betaRecoveryID(t, c.record(source, "Retained memory", "", true))
			refs := c.refs(source)
			actorRef := "refs/hn/actors/" + original.Actor
			var memoryRef string
			for ref := range refs {
				if strings.HasPrefix(ref, "refs/hn/memory/") {
					memoryRef = ref
				}
			}
			stream := "sha256:" + filepath.Base(memoryRef)
			historical := betaHistoricalObjects(t, source, c.env, refs)
			// A public bundle explicitly selects code and custom refs. Ordinary clone
			// does not recover those refs; the exact no-write-fetch-head recipe does.
			oldBundle := filepath.Join(c.root, "before-disaster.bundle")
			c.gitAt(source, "bundle", "create", oldBundle, "refs/heads/main", actorRef, memoryRef)
			c.gitAt(source, "bundle", "verify", oldBundle)
			privateBackup := filepath.Join(c.root, "private-backup")
			betaCopyPrivate(t, filepath.Join(source, ".git", "hn"), privateBackup, original.Actor)
			observer := c.clone(oldBundle, "keyless")
			if got := c.gitAt(observer, "for-each-ref", "--format=%(refname)", "refs/hn"); got != "" {
				t.Fatal("ordinary clone recovered custom refs unexpectedly")
			}
			c.gitAt(observer, "fetch", "--no-write-fetch-head", oldBundle, "refs/hn/*:refs/hn/*")
			c.hn(observer, true, "issue", "show", issue, "--json")
			c.hn(observer, true, "memory", "show", memory, "--json")
			c.assertKeyless(observer)
			if _, err := os.Stat(filepath.Join(observer, ".hn")); !os.IsNotExist(err) {
				t.Fatalf("public reconstruction installed optional configuration: %v", err)
			}
			betaAssertHistoricalRefs(t, observer, c.env, refs)
			if !bytes.Equal(historical, betaHistoricalObjects(t, observer, c.env, refs)) {
				t.Fatal("public backup changed historical object bytes")
			}
			// A distinct device can contribute without acquiring the lost signer's roles.
			c.hn(observer, true, "init", "--name", "Replacement actor")
			fresh := c.identity(observer)
			if fresh.Actor == original.Actor {
				t.Fatal("replacement reused lost actor")
			}
			c.hn(observer, true, "issue", "comment", issue, "Fresh actor continuation")
			betaAssertHistoricalRefs(t, observer, c.env, refs)

			// Advance both chains beyond the private backup, publish to a real bare
			// remote, then fence the former writer before making the restored key usable.
			c.authorize(source, target, true)
			c.record(source, "Latest accepted memory", stream, true)
			latest := c.refs(source)
			latestObjects := betaHistoricalObjects(t, source, c.env, latest)
			remote := filepath.Join(c.root, "retained.git")
			c.gitAt(c.root, "init", "--bare", "-q", remote)
			c.gitAt(source, "remote", "add", "origin", remote)
			c.gitAt(source, "push", "-q", "origin", "refs/heads/main", actorRef, memoryRef)
			if err = os.Rename(filepath.Join(source, ".git", "hn"), filepath.Join(c.root, "fenced-original-private")); err != nil {
				t.Fatal(err)
			}
			c.hn(source, false, "identity", "show", "--json")
			for _, state := range []string{"missing", "stale", "divergent"} {
				t.Run(state, func(t *testing.T) {
					c := c
					c.t = t
					restore := c.clone(oldBundle, "restore-"+state)
					c.gitAt(restore, "fetch", "--no-write-fetch-head", oldBundle, "refs/hn/*:refs/hn/*")
					betaCopyPrivate(t, privateBackup, filepath.Join(restore, ".git", "hn"), original.Actor)
					var forkActor, forkMemory string
					if state == "divergent" {
						c.authorize(restore, target, true)
						c.record(restore, "Disconnected conflicting memory", stream, true)
						forkActor = c.gitAt(restore, "rev-parse", actorRef)
						forkMemory = c.gitAt(restore, "rev-parse", memoryRef)
						c.gitAt(restore, "update-ref", "-d", actorRef, forkActor)
						c.gitAt(restore, "update-ref", "-d", memoryRef, forkMemory)
					}
					if state == "missing" {
						c.gitAt(restore, "update-ref", "-d", actorRef, refs[actorRef])
						c.gitAt(restore, "update-ref", "-d", memoryRef, refs[memoryRef])
					}
					c.gitAt(restore, "remote", "set-url", "origin", remote)
					c.hn(restore, true, "replication", "select", "origin", "--actor", original.Actor, "--memory", stream)
					// Memory publication can fail after import. The accepted refs, rather than
					// sync's aggregate exit status, establish what this restored writer knows.
					cmd := exec.Command(binary, "sync", "origin")
					cmd.Dir = restore
					cmd.Env = c.env
					_, _ = cmd.CombinedOutput()
					acceptedActor := "refs/hn/remotes/origin/actors/" + original.Actor
					acceptedMemory := "refs/hn/remotes/origin/memory/" + original.Actor + "/" + filepath.Base(memoryRef)
					if c.gitAt(restore, "rev-parse", acceptedActor) != latest[actorRef] || c.gitAt(restore, "rev-parse", acceptedMemory) != latest[memoryRef] {
						t.Fatal("sync did not import exact accepted heads")
					}
					if state == "divergent" {
						// Reproduce an operator restoring a valid disconnected fork
						// after newer accepted heads have already been recovered.
						c.gitAt(restore, "update-ref", actorRef, forkActor, "")
						c.gitAt(restore, "update-ref", memoryRef, forkMemory, "")
					}
					c.noAppend(restore, target, stream)
					if state == "divergent" {
						unrelated := c.clone(oldBundle, "unrelated-actor")
						// Retain both conflicting observations as public data;
						// neither belongs to this new signer's actor or stream.
						c.gitAt(unrelated, "fetch", "--no-write-fetch-head", restore, acceptedActor+":"+acceptedActor, actorRef+":refs/hn/remotes/fork/actors/"+original.Actor, acceptedMemory+":"+acceptedMemory, memoryRef+":refs/hn/remotes/fork/memory/"+original.Actor+"/"+filepath.Base(memoryRef))
						c.hn(unrelated, true, "init", "--name", "Independent writer")
						c.authorize(unrelated, target, true)
						c.record(unrelated, "Independent actor and stream", "", true)
					}
					if state != "divergent" {
						// Reconcile only after verifying the retained history, using exact old
						// values. Public reads validate signatures without adopting a signing root.
						c.hn(restore, true, "identity", "list")
						c.hn(restore, true, "memory", "show", memory, "--json")
						oldActor, oldMemory := refs[actorRef], refs[memoryRef]
						if state == "missing" {
							oldActor = ""
							oldMemory = ""
						}
						c.gitAt(restore, "update-ref", actorRef, latest[actorRef], oldActor)
						c.gitAt(restore, "update-ref", memoryRef, latest[memoryRef], oldMemory)
						c.authorize(restore, target, true)
						c.record(restore, "Exclusive restored continuation", stream, true)
						// Accepted older prefixes must not obstruct subsequent owned appends.
						c.authorize(restore, target, true)
						c.record(restore, "Continued after accepted prefix", stream, true)
						if !bytes.Equal(latestObjects, betaHistoricalObjects(t, restore, c.env, latest)) {
							t.Fatal("private recovery rewrote signed history")
						}
					}
					// Other streams remain writable even when this stream conflicts.
					other := "sha256:" + strings.Repeat("9", 64)
					c.record(restore, "Independent stream", other, true)
					// Each rehearsal fences its writer before the next restoration.
					if err := os.RemoveAll(filepath.Join(restore, ".git", "hn", "identities")); err != nil {
						t.Fatal(err)
					}
				})
			}
			t.Run("unsafe-private-state", func(t *testing.T) {
				c := c
				c.t = t
				restore := c.clone(oldBundle, "unsafe-private")
				root := filepath.Join(restore, ".git", "hn")
				betaCopyPrivate(t, privateBackup, root, original.Actor)
				key := filepath.Join(root, "identities", original.Actor+".json")
				if err := os.Chmod(key, 0644); err != nil {
					t.Fatal(err)
				}
				c.noWrite(restore, "identity", "show", "--json")
				c.noWrite(restore, "identity", "authorize", "--relationship", "device", "--actor", target.Actor, "--public-key", target.PublicKey)
				if err := os.Chmod(key, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(root, 0755); err != nil {
					t.Fatal(err)
				}
				c.noWrite(restore, "identity", "show", "--json")
				if err := os.Chmod(root, 0700); err != nil {
					t.Fatal(err)
				}
				saved := filepath.Join(c.root, "symlinked-test-key")
				if err := os.Rename(key, saved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(saved, key); err != nil {
					t.Fatal(err)
				}
				// Snapshot helper follows a regular-file symlink's bytes, so
				// retain its link target explicitly as part of the assertion.
				before, _ := os.Readlink(key)
				c.noWrite(restore, "identity", "show", "--json")
				after, _ := os.Readlink(key)
				if before != after {
					t.Fatal("unsafe restore altered symlink")
				}
			})
			t.Run("linked-worktree-boundary", func(t *testing.T) {
				c := c
				c.t = t
				linked := filepath.Join(c.root, "linked")
				c.gitAt(source, "worktree", "add", "-qb", "recovery-linked", linked, "main")
				privateDir := c.gitAt(linked, "rev-parse", "--absolute-git-dir")
				if privateDir == filepath.Join(source, ".git") {
					t.Fatal("linked private directory aliases main")
				}
				c.noWrite(linked, "identity", "show", "--json")
				c.hn(linked, true, "init", "--name", "Distinct linked worktree actor")
				linkedActor := c.identity(linked)
				if linkedActor.Actor == original.Actor {
					t.Fatal("linked worktree reused fenced identity")
				}
				if _, err := os.Stat(filepath.Join(privateDir, "hn", "identities", linkedActor.Actor+".json")); err != nil {
					t.Fatal(err)
				}
				c.authorize(linked, target, true)
				if c.gitAt(source, "rev-parse", "refs/hn/actors/"+linkedActor.Actor) == "" {
					t.Fatal("linked worktree refs were not shared")
				}
				c.hn(source, false, "identity", "show", "--json")
			})
			t.Run("lost-sole-authority", func(t *testing.T) { c := c; c.t = t; c.authorityJourney(false) })
			t.Run("explicit-role-amendment", func(t *testing.T) { c := c; c.t = t; c.authorityJourney(true) })
		})
	}
}

func (c betaRecoveryCLI) proposal(repo, title string) string {
	c.t.Helper()
	before := c.refs(repo)
	c.hn(repo, true, "proposal", "open", "--base", "main", "--head", "HEAD", title)
	for ref := range c.refs(repo) {
		if strings.HasPrefix(ref, "refs/hn/proposals/") && before[ref] == "" {
			return "sha256:" + filepath.Base(ref)
		}
	}
	c.t.Fatal("proposal did not publish exact code ref")
	return ""
}
func betaRecoveryPolicy(t *testing.T, repo string, maintainers, auditActors []string) {
	t.Helper()
	policy := map[string]any{"version": "hn.policy/0", "maintainers": maintainers, "proposals": map[string]any{"requiredApprovals": 0, "requiredAccepts": 1, "trustedReviewers": []string{}, "allowAuthorApproval": false}, "pipelines": map[string]any{}, "audits": map[string]any{"scope": "recovery-rehearsal", "ruleOwners": auditActors, "requesters": auditActors, "executors": auditActors}}
	raw, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	policyPublicFile(t, repo, ".hn/policy.json", raw)
}
func (c betaRecoveryCLI) authorityJourney(withRemaining bool) {
	t := c.t
	suffix := "sole"
	if withRemaining {
		suffix = "remaining"
	}
	owner := c.clone(filepath.Join(c.root, "before-disaster.bundle"), "authority-owner-"+suffix)
	c.hn(owner, true, "init", "--name", "Authority before loss")
	old := c.identity(owner)
	remaining := c.clone(owner, "remaining-"+suffix)
	c.hn(remaining, true, "init", "--name", "Remaining authority")
	other := c.identity(remaining)
	maintainers := []string{old.Actor}
	if withRemaining {
		maintainers = append(maintainers, other.Actor)
	}
	betaRecoveryPolicy(t, owner, maintainers, []string{old.Actor})
	policyPublicFile(t, owner, "rules/check.json", []byte(`{"version":"hn.pipeline/0","steps":[{"name":"Inspect only","command":"git","args":["--version"]}]}`))
	c.gitAt(owner, "add", ".hn/policy.json", "rules/check.json")
	c.gitAt(owner, "commit", "-qm", "Explicit pre-loss roles")
	c.hn(owner, true, "audit", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	c.hn(owner, true, "audit", "rule", "authorize", "check", "--source", "git", "--at", "HEAD", "--path", "rules/check.json", "--json")
	first := c.refs(owner)
	originalObjects := betaHistoricalObjects(t, owner, c.env, first)
	auditActorRef := "refs/hn/actors/" + old.Actor
	backup := filepath.Join(c.root, "authority-"+suffix+".bundle")
	c.gitAt(owner, "bundle", "create", backup, "refs/heads/main", auditActorRef)
	c.gitAt(owner, "bundle", "verify", backup)
	// A same-actor local audit predecessor must not import its embedded pack
	// while discovering a position that the common append guard will refuse.
	private := filepath.Join(c.root, "audit-private-"+suffix)
	betaCopyPrivate(t, filepath.Join(owner, ".git", "hn"), private, old.Actor)
	c.authorize(owner, other, true)
	auditRestore := c.clone(backup, "audit-restore-"+suffix)
	c.gitAt(auditRestore, "fetch", "--no-write-fetch-head", backup, auditActorRef+":"+auditActorRef)
	betaCopyPrivate(t, private, filepath.Join(auditRestore, ".git", "hn"), old.Actor)
	accepted := "refs/hn/remotes/retained/actors/" + old.Actor
	c.gitAt(auditRestore, "fetch", "--no-write-fetch-head", owner, auditActorRef+":"+accepted)
	c.noWrite(auditRestore, "identity", "authorize", "--relationship", "device", "--actor", other.Actor, "--public-key", other.PublicKey)
	if err := os.RemoveAll(filepath.Join(auditRestore, ".git", "hn", "identities")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(owner, ".git", "hn"), filepath.Join(c.root, "lost-authority-"+suffix)); err != nil {
		t.Fatal(err)
	}
	c.hn(owner, false, "identity", "show", "--json")
	fresh := c.clone(backup, "authority-fresh-"+suffix)
	c.gitAt(fresh, "fetch", "--no-write-fetch-head", backup, auditActorRef+":"+auditActorRef)
	c.assertKeyless(fresh)
	c.noWrite(fresh, "audit", "authority", "show", "--json")
	c.hn(fresh, true, "init", "--name", "Fresh device after key loss")
	replacement := c.identity(fresh)
	if replacement.Actor == old.Actor {
		t.Fatal("key loss reused actor")
	}
	c.noWrite(fresh, "audit", "rule", "authorize", "check", "--source", "git", "--at", "HEAD", "--path", "rules/check.json", "--json")
	c.hn(fresh, true, "audit", "authority", "set", "--ref", "refs/heads/main", "--expect", "absent", "--json")
	if out := c.noWrite(fresh, "audit", "rule", "authorize", "check", "--source", "git", "--at", "HEAD", "--path", "rules/check.json", "--json"); !strings.Contains(out, "not a current audit owner") {
		t.Fatal(out)
	}
	c.gitAt(fresh, "checkout", "-qb", "role-amendment")
	betaRecoveryPolicy(t, fresh, []string{replacement.Actor}, []string{replacement.Actor})
	c.gitAt(fresh, "add", ".hn/policy.json")
	c.gitAt(fresh, "commit", "-qm", "Propose explicit replacement roles")
	candidate := c.proposal(fresh, "Replacement requires remaining base authority")
	// Normal imported audit inspection may install its verified snapshot pack.
	// Complete that documented read before measuring an authority refusal.
	c.hn(fresh, true, "identity", "list")
	if out := c.noWrite(fresh, "decide", candidate, "--accept"); !strings.Contains(out, "not a maintainer") {
		t.Fatal(out)
	}
	if withRemaining {
		c.gitAt(remaining, "fetch", "--no-write-fetch-head", backup, "refs/heads/main:refs/remotes/backup/main")
		c.gitAt(remaining, "reset", "--hard", "refs/remotes/backup/main")
		c.gitAt(remaining, "remote", "add", "candidate", fresh)
		c.hn(remaining, true, "replication", "select", "candidate", "--actor", old.Actor, "--actor", replacement.Actor, "--proposal", candidate)
		c.hn(remaining, true, "sync", "candidate")
		c.hn(remaining, true, "decide", candidate, "--accept")
		c.hn(remaining, true, "merge", candidate)
		c.gitAt(fresh, "checkout", "main")
		c.gitAt(fresh, "fetch", "--no-write-fetch-head", remaining, "refs/heads/main:refs/remotes/recovery/main")
		c.gitAt(fresh, "merge", "--ff-only", "refs/remotes/recovery/main")
		c.gitAt(fresh, "remote", "add", "recovery", remaining)
		c.hn(fresh, true, "replication", "select", "recovery", "--actor", other.Actor)
		c.hn(fresh, true, "sync", "recovery")
		c.hn(fresh, true, "audit", "rule", "authorize", "check", "--source", "git", "--at", "HEAD", "--path", "rules/check.json", "--json")
	}
	// The original audit fact remains exact despite lost keys and role changes.
	if c.gitAt(fresh, "rev-parse", auditActorRef) != first[auditActorRef] {
		t.Fatal("authority recovery rewrote old signed head")
	}
	if !bytes.Equal(originalObjects, betaHistoricalObjects(t, fresh, c.env, first)) {
		t.Fatal("authority recovery changed prior object bytes")
	}
}
