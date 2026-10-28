package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"hubnot/internal/forge"
	"hubnot/internal/migration"
)

func TestBetaRotationJournalRefusalPreservesRecovery(t *testing.T) {
	for _, variant := range []struct{ name, tags string }{{"standard", ""}, {"native-only", "noforge"}} {
		t.Run(variant.name, func(t *testing.T) {
			binary := buildMigrationCLIBinary(t, variant.tags)
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			for _, scenario := range []struct {
				name, refusal                        string
				missingTarget                        bool
				noCommand, noRotation, futureCommand bool
			}{
				{name: "reconciled missing authorization", refusal: "not found"},
				{name: "rotation-only missing authorization", refusal: "not found", noCommand: true},
				{name: "command-only missing authorization", refusal: "not found", noRotation: true},
				{name: "future command", refusal: "unsupported identity rotation command version", futureCommand: true},
				{name: "missing target and future state", refusal: "unsupported identity rotation version", missingTarget: true},
				{name: "missing target and unsafe state", refusal: "private", missingTarget: true},
			} {
				t.Run(scenario.name, func(t *testing.T) {
					c := betaRecoveryCLI{t: t, binary: binary, git: git, root: t.TempDir(), env: []string{"PATH=" + filepath.Dir(git), "HOME=" + t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0"}}
					repo := filepath.Join(c.root, "repository")
					c.gitAt(c.root, "init", "-q", "-b", "main", repo)
					c.hn(repo, true, "init", "--name", "Disposable predecessor")
					predecessor := c.identity(repo)
					target := testIdentity(t, "Disposable target")
					root := filepath.Join(repo, ".git", "hn")
					state := identityRotationState{Version: identityRotationVersion, PredecessorActor: predecessor.Actor, TargetActor: target.Actor, Relationship: identityRelationshipSuccessor}
					command := identityRotationCommandState{Version: identityRotationCommandVersion, State: state, Target: *target}
					if scenario.futureCommand {
						command.Version++
					}
					if scenario.noRotation {
						command.State.AuthorizationEvent = "sha256:" + strings.Repeat("a", 64)
					}
					if !scenario.noCommand {
						betaJournalJSON(t, filepath.Join(root, "rotation-command.json"), command)
					}
					if !scenario.missingTarget {
						betaJournalJSON(t, filepath.Join(root, "identities", target.Actor+".json"), identityRecord{Version: identityRecordVersion, Actor: target.Actor, Name: target.Name, PublicKey: target.PublicKey, PrivateKey: target.PrivateKey, Lifecycle: identityLifecycleAvailable})
						// Only rotation.json has progress. Identically bad journals would
						// miss the command journal's write during reconciliation.
						state.AuthorizationEvent = "sha256:" + strings.Repeat("a", 64)
					}
					if scenario.missingTarget && strings.Contains(scenario.name, "future") {
						state.Version++
					}
					path := filepath.Join(root, "rotation.json")
					if !scenario.noRotation {
						betaJournalJSON(t, path, state)
					}
					if strings.Contains(scenario.name, "unsafe") {
						if err := os.Chmod(path, 0644); err != nil {
							t.Fatal(err)
						}
					}
					if output := c.noWrite(repo, "identity", "rotate"); !strings.Contains(output, scenario.refusal) {
						t.Fatalf("wrong refusal: %s", output)
					}
				})
			}
		})
	}
}

func betaJournalJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestBetaMigrationFutureSessionRefusesWithoutReset(t *testing.T) {
	binary := buildMigrationCLIBinary(t, "")
	for _, checkpoint := range []struct{ file, schema, refusal string }{
		{"plan.json", "hn.migration-plan/1", "noncanonical saved plan"},
		{"progress.json", "hn.migration-progress/1", "progress binding mismatch"},
		{"cli.json", "hn.migration-cli/1", "CLI descriptor does not bind saved plan/destination"},
	} {
		t.Run(checkpoint.file, func(t *testing.T) {
			root := t.TempDir()
			output := filepath.Join(root, "destination.tar")
			destination := ArchiveMigrationDestination{output}
			snapshot, blobs := migrationForeignSnapshot()
			session, err := PrepareMigration(context.Background(), filepath.Join(root, "session"), snapshot, blobs, migration.Selection{Destination: migration.Locator{Kind: "archive", Repository: output}}, destination, migration.DefaultLimits(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := saveMigrationCLI(session, migration.ProviderConfig{Locator: session.Plan.Destination}); err != nil {
				t.Fatal(err)
			}
			// Reseal the future schema with this disposable session's real HMAC
			// key. A broken MAC would prove authentication, not version refusal.
			raw, err := migrationUnseal(session.Directory, checkpoint.file, 64<<20)
			if err != nil {
				t.Fatal(err)
			}
			future := bytes.Replace(raw, []byte(checkpoint.schema), []byte(strings.TrimSuffix(checkpoint.schema, "1")+"999"), 1)
			if bytes.Equal(raw, future) {
				t.Fatal("fixture did not change the schema")
			}
			if err := migrationSeal(session.Directory, checkpoint.file, future); err != nil {
				t.Fatal(err)
			}
			if verified, err := migrationUnseal(session.Directory, checkpoint.file, 64<<20); err != nil || !bytes.Equal(verified, future) {
				t.Fatalf("future fixture is not authenticated: %v", err)
			}
			before := policyPublicSnapshot(t, root)
			out := migrationCLIProcess(t, binary, root, "", nil, false, "resume", "--session", session.Directory, "--confirm-apply")
			if !strings.Contains(out, checkpoint.refusal) || strings.Contains(out, "authentication failed") {
				t.Fatalf("wrong future-version refusal: %s", out)
			}
			if !reflect.DeepEqual(before, policyPublicSnapshot(t, root)) {
				t.Fatal("future session refusal reset progress or changed destination/session files")
			}
		})
	}
}

func TestBetaTransferFutureJournalRefusesWithoutReset(t *testing.T) {
	for _, family := range []string{"native issue import", "resource import", "signed issue export", "embedded remote intent"} {
		t.Run(family, func(t *testing.T) {
			repo := issueCommandTestRepo(t)
			var retry func() error
			var writes func() int
			var refusal string
			if family == "native issue import" {
				f := newForgeExportFake(t)
				o := transferOptions{command: "import", provider: "other", project: "team/project", key: "1", resource: forge.ResourceIssue}
				identity, err := loadIdentity()
				if err != nil {
					t.Fatal(err)
				}
				intent := forgeNativeIntent{Schema: "hn.forge-native-intent/1", Scope: forgeImportScope(f, o), Actor: identity.Actor, Command: "open", State: &IssueState{Title: "Retained source", Status: "open"}, Operation: forgeOperation("future-test")}
				if err := forgeValidateIntent(intent, identity); err != nil {
					t.Fatal(err)
				}
				intent.Schema = "hn.forge-native-intent/999"
				if err := forgeStoreJSON("native-intent.json", intent); err != nil {
					t.Fatal(err)
				}
				retry = func() error { return runForgeTransfer(o) }
				writes = func() int { return f.creates + f.updates + f.commentCreates }
				refusal = "invalid native transfer intent"
			} else if family == "resource import" {
				p := &bridgeProvider{graph: bridgeProjectGraph()}
				bridgeRegister(t, p)
				o := bridgeOptions(forge.ResourceProject)
				j, err := forgeBuildResourcePlan(p, o, p.graph, nil)
				if err != nil {
					t.Fatal(err)
				}
				identity, err := loadIdentity()
				if err != nil {
					t.Fatal(err)
				}
				j.Actor = identity.Actor
				j.Schema = "hn.forge-resource-import/999"
				if err := forgeStoreJSON(forgeResourceJournalName(j.Scope), j); err != nil {
					t.Fatal(err)
				}
				retry = func() error { return runForgeTransfer(o) }
				writes = func() int { return p.reads }
				refusal = "resource recovery checkpoint does not match selection"
			} else {
				f := newForgeExportFake(t)
				id := forgeExportNativeOpen(t, IssueState{Title: "Disposable export"})
				f.absentCreate = true
				if _, err := forgeExportTestRun(t, id); err == nil {
					t.Fatal("fixture did not retain an uncertain export")
				}
				o := transferOptions{project: "team/project", id: id}
				name := forgeExportCheckpointName(forgeExportScope(f, o))
				var j forgeIssueExportCheckpoint
				if found, err := forgeLoadJSON(name, &j); err != nil || !found || j.Pending == nil {
					t.Fatalf("missing uncertain journal: %v", err)
				}
				if family == "signed issue export" {
					j.Schema = "hn.forge-issue-export/999"
					refusal = "export checkpoint identity is invalid"
				} else {
					j.Pending.Schema = "hn.forge-remote-intent/999"
					refusal = "invalid exact external write intent"
				}
				identity, err := loadIdentity()
				if err != nil {
					t.Fatal(err)
				}
				if err := forgeExportSignCheckpoint(&j, identity); err != nil {
					t.Fatal(err)
				}
				if err := forgeStoreJSON(name, j); err != nil {
					t.Fatal(err)
				}
				if err := forgeExportVerifyCheckpoint(name, j, identity); err != nil {
					t.Fatalf("future fixture is not signed: %v", err)
				}
				retry = func() error { _, err := forgeExportTestRun(t, id); return err }
				writes = func() int { return f.creates + f.updates + f.commentCreates }
			}
			before, count := policyPublicSnapshot(t, repo), writes()
			if err := retry(); err == nil || !strings.Contains(err.Error(), refusal) {
				t.Fatalf("wrong future-version refusal: %v", err)
			}
			if writes() != count || !reflect.DeepEqual(before, policyPublicSnapshot(t, repo)) {
				t.Fatal("future journal refusal changed private state, native history or provider calls")
			}
		})
	}
}
