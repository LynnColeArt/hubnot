package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestReplicationSelectionRoundTripAndRejectsAmbiguity(t *testing.T) {
	root := inReplicationTestRepository(t)
	alice := testIdentity(t, "Alice")
	proposal := "sha256:" + strings.Repeat("a", 64)

	if err := cmdReplication([]string{
		"select", "origin",
		"--actor", alice.Actor,
		"--proposal", proposal,
		"--max-events", "7",
		"--max-objects", "11",
		"--max-object-bytes", "13",
		"--max-attachment-bytes", "17",
		"--max-total-bytes", "19",
	}); err != nil {
		t.Fatal(err)
	}
	selection, explicit, err := loadReplicationSelection("origin")
	if err != nil {
		t.Fatal(err)
	}
	want := ReplicationSelection{
		Version:   replicationSelectionVersion,
		Remote:    "origin",
		Actors:    []string{alice.Actor},
		Proposals: []string{proposal},
		Budgets: ReplicationBudgets{
			MaxEvents:          7,
			MaxObjects:         11,
			MaxObjectBytes:     13,
			MaxAttachmentBytes: 17,
			MaxTotalBytes:      19,
		},
	}
	if !explicit || !reflect.DeepEqual(selection, want) {
		t.Fatalf("selection = %#v, explicit=%t; want %#v", selection, explicit, want)
	}
	path, err := replicationSelectionPath("origin")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("selection mode = %o, want 600", info.Mode().Perm())
	}
	if !strings.HasPrefix(path, filepath.Join(root, ".git", "hn")+string(os.PathSeparator)) {
		t.Fatalf("selection escaped .git/hn: %s", path)
	}
	shown, err := captureTestOutput(t, func() error { return cmdReplication([]string{"show", "origin"}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, exact := range []string{alice.Actor, proposal, "max-events: 7", "compatibility-all: false"} {
		if !strings.Contains(shown, exact) {
			t.Fatalf("show output omitted %q:\n%s", exact, shown)
		}
	}

	cases := []struct {
		name string
		args []string
	}{
		{"short actor", []string{"select", "--actor", alice.Actor[:12]}},
		{"short proposal", []string{"select", "--proposal", proposal[:20]}},
		{"duplicate actor", []string{"select", "--actor", alice.Actor, "--actor", alice.Actor}},
		{"duplicate proposal", []string{"select", "--proposal", proposal, "--proposal", proposal}},
		{"all and actor", []string{"select", "--all", "--actor", alice.Actor}},
		{"empty selection", []string{"select"}},
		{"zero budget", []string{"select", "--all", "--max-events", "0"}},
		{"negative budget", []string{"select", "--all", "--max-objects", "-1"}},
		{"remote traversal", []string{"select", "../origin", "--all"}},
		{"remote alias collision", []string{"select", "a/b", "--all"}},
		{"remote option injection", []string{"select", "--upload-pack=evil", "--all"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := cmdReplication(test.args); err == nil {
				t.Fatalf("cmdReplication(%q) succeeded", test.args)
			}
		})
	}
}

func TestReplicationCompatibilitySelectionIsBounded(t *testing.T) {
	inReplicationTestRepository(t)
	selection, explicit, err := loadReplicationSelection("origin")
	if err != nil {
		t.Fatal(err)
	}
	if explicit || !selection.All {
		t.Fatalf("default selection = %#v, explicit=%t", selection, explicit)
	}
	if err := selection.Budgets.validate(); err != nil {
		t.Fatalf("compatibility budgets are not positive: %v", err)
	}
}

func TestReplicationBudgetBoundaries(t *testing.T) {
	root := inReplicationTestRepository(t)
	identity := testIdentity(t, "Budget Actor")
	writeActiveTestIdentity(t, identity)
	issue, err := nextEvent(identity, "issue.open")
	if err != nil {
		t.Fatal(err)
	}
	issue.Title = "budget fixture"
	if _, err := appendEvent(issue, identity); err != nil {
		t.Fatal(err)
	}
	log := []byte("attachment payload with a measurable boundary")
	result, err := nextEvent(identity, "run.result")
	if err != nil {
		t.Fatal(err)
	}
	result.Subject = "sha256:" + strings.Repeat("1", 64)
	result.Pipeline = "test"
	result.Definition = "sha256:" + strings.Repeat("2", 64)
	result.Commit = mustGitText(t, "rev-parse", "HEAD")
	result.Outcome = "passed"
	result.Log = eventID(log)
	result.Backend = "sandbox"
	result.Platform = "test"
	result.Runner = identity.Actor
	stored, err := appendEventWithAttachments(result, identity, map[string][]byte{"log.txt": log})
	if err != nil {
		t.Fatal(err)
	}
	baseline := filepath.Join(root, "empty.git")
	mustGit(t, "init", "--bare", "-q", baseline)
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	measurements, err := measureQuarantinedSelection(gitDir, baseline, replicationActor, stored.Commit)
	if err != nil {
		t.Fatal(err)
	}

	dimensions := []struct {
		name  string
		value int64
		set   func(*ReplicationBudgets, int64)
	}{
		{"events", measurements.Events, func(b *ReplicationBudgets, n int64) { b.MaxEvents = n }},
		{"objects", measurements.Objects, func(b *ReplicationBudgets, n int64) { b.MaxObjects = n }},
		{"object-bytes", measurements.LargestObjectBytes, func(b *ReplicationBudgets, n int64) { b.MaxObjectBytes = n }},
		{"attachment-bytes", measurements.LargestAttachmentBytes, func(b *ReplicationBudgets, n int64) { b.MaxAttachmentBytes = n }},
		{"total-bytes", measurements.TotalBytes, func(b *ReplicationBudgets, n int64) { b.MaxTotalBytes = n }},
	}
	for _, dimension := range dimensions {
		if dimension.value <= 1 {
			t.Fatalf("%s fixture value = %d, need > 1", dimension.name, dimension.value)
		}
		for _, boundary := range []struct {
			name    string
			limit   int64
			wantErr bool
		}{
			{"measured-one-above-limit", dimension.value - 1, true},
			{"measured-exactly-at-limit", dimension.value, false},
			{"measured-one-below-limit", dimension.value + 1, false},
		} {
			t.Run(dimension.name+"/"+boundary.name, func(t *testing.T) {
				budgets := ReplicationBudgets{
					MaxEvents: 1 << 30, MaxObjects: 1 << 30,
					MaxObjectBytes: 1 << 60, MaxAttachmentBytes: 1 << 60, MaxTotalBytes: 1 << 60,
				}
				dimension.set(&budgets, boundary.limit)
				err := enforceReplicationBudgets("actor "+identity.Actor, budgets, measurements)
				if (err != nil) != boundary.wantErr {
					t.Fatalf("limit %d, measurements %#v: %v", boundary.limit, measurements, err)
				}
				if err != nil && (!strings.Contains(err.Error(), identity.Actor) || !strings.Contains(err.Error(), dimension.name)) {
					t.Fatalf("budget diagnostic is not exact: %v", err)
				}
			})
		}
	}
}

func TestSelectedReplicationQuarantinesAndPromotesIndependently(t *testing.T) {
	root := t.TempDir()
	seed := filepath.Join(root, "seed")
	remote := filepath.Join(root, "project.git")
	receiver := filepath.Join(root, "receiver")
	if err := os.Mkdir(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	mustGit(t, "-C", seed, "init", "-q", "-b", "main")
	mustGit(t, "-C", seed, "config", "user.name", "Seed")
	mustGit(t, "-C", seed, "config", "user.email", "seed@hn.invalid")
	mustGit(t, "-C", seed, "commit", "--allow-empty", "-q", "-m", "base")
	mustGit(t, "clone", "-q", "--bare", seed, remote)
	mustGit(t, "clone", "-q", remote, receiver)

	if err := os.Chdir(seed); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "remote", "add", "origin", remote)
	alice := testIdentity(t, "Alice")
	writeActiveTestIdentity(t, alice)
	aliceIssue, err := nextEvent(alice, "issue.open")
	if err != nil {
		t.Fatal(err)
	}
	aliceIssue.Title = "selected valid history"
	aliceStored, err := appendEvent(aliceIssue, alice)
	if err != nil {
		t.Fatal(err)
	}
	carol := testIdentity(t, "Carol")
	carolIssue := newEvent(carol, "issue.open", 1, "")
	carolIssue.Title = "must remain unselected"
	carolStored, err := appendEvent(carolIssue, carol)
	if err != nil {
		t.Fatal(err)
	}
	bob := testIdentity(t, "Bob")
	bobIssue := newEvent(bob, "issue.open", 1, "")
	bobIssue.Title = "previously accepted history"
	bobStored, err := appendEvent(bobIssue, bob)
	if err != nil {
		t.Fatal(err)
	}
	badCommit := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "update-ref", actorRef(bob.Actor), badCommit)

	base := badCommit
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "candidate")
	head := mustGitText(t, "rev-parse", "HEAD")
	proposalEvent, err := nextEvent(alice, "proposal.open")
	if err != nil {
		t.Fatal(err)
	}
	proposalEvent.Title = "selected candidate"
	proposalEvent.Base = base
	proposalEvent.Head = head
	proposal, err := appendEvent(proposalEvent, alice)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, head); err != nil {
		t.Fatal(err)
	}
	dave := testIdentity(t, "Dave")
	mismatchEvent := newEvent(dave, "proposal.open", 1, "")
	mismatchEvent.Title = "mismatched candidate"
	mismatchEvent.Base = base
	mismatchEvent.Head = head
	mismatch, err := appendEvent(mismatchEvent, dave)
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, "update-ref", proposalRef(mismatch.ID), base)
	erin := testIdentity(t, "Erin")
	missingID := "sha256:" + strings.Repeat("8", 64)
	missingComment := newEvent(erin, "issue.comment", 1, "")
	missingComment.Subject = missingID
	missingComment.Body = "dependency is deliberately absent"
	if _, err := appendEvent(missingComment, erin); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{actorRef(alice.Actor), actorRef(carol.Actor), actorRef(bob.Actor), actorRef(dave.Actor), actorRef(erin.Actor), proposalRef(proposal.ID), proposalRef(mismatch.ID)} {
		mustGit(t, "push", "-q", "origin", ref+":"+ref)
	}

	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	if err := copyGitObjects(filepath.Join(seed, ".git"), filepath.Join(receiver, ".git"), []string{bobStored.Commit}); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "update-ref", acceptedActorRef("origin", bob.Actor), bobStored.Commit)
	selection := []string{
		"select", "origin", "--actor", alice.Actor, "--actor", bob.Actor, "--actor", dave.Actor, "--actor", erin.Actor,
		"--proposal", proposal.ID, "--proposal", mismatch.ID,
	}
	if err := cmdReplication(selection); err != nil {
		t.Fatal(err)
	}
	observedQuarantine := false
	replicationAfterFetchHook = func() error {
		observedQuarantine = true
		refs := strings.Fields(mustGitText(t, "for-each-ref", "--format=%(refname)", "refs/hn/remotes/origin"))
		if !reflect.DeepEqual(refs, []string{acceptedActorRef("origin", bob.Actor)}) {
			t.Fatalf("accepted refs changed before promotion: %v", refs)
		}
		assertRefValue(t, acceptedActorRef("origin", bob.Actor), bobStored.Commit)
		return nil
	}
	t.Cleanup(func() { replicationAfterFetchHook = nil })
	output, syncErr := captureTestOutput(t, func() error { return cmdSync(nil) })
	if syncErr == nil {
		t.Fatalf("mixed valid/invalid sync succeeded:\n%s", output)
	}
	if !observedQuarantine {
		t.Fatal("sync never reached isolated post-fetch hook")
	}
	if strings.Contains(output+syncErr.Error(), root) {
		t.Fatalf("sync diagnostic leaked a host-private path:\n%s\n%v", output, syncErr)
	}
	for _, exact := range []string{bob.Actor, mismatch.ID, "ref/head mismatch", erin.Actor, missingID, "recovery:", "failed"} {
		if !strings.Contains(output+syncErr.Error(), exact) {
			t.Fatalf("sync diagnostic omitted %q:\n%s\n%v", exact, output, syncErr)
		}
	}
	assertRefValue(t, acceptedActorRef("origin", alice.Actor), proposal.Commit)
	assertRefValue(t, acceptedProposalRef("origin", proposal.ID), head)
	assertRefValue(t, acceptedActorRef("origin", bob.Actor), bobStored.Commit)
	assertRefAbsent(t, acceptedActorRef("origin", dave.Actor))
	assertRefAbsent(t, acceptedActorRef("origin", erin.Actor))
	assertRefAbsent(t, acceptedActorRef("origin", carol.Actor))
	assertRefAbsent(t, acceptedProposalRef("origin", mismatch.ID))
	if _, exists, err := refValue(actorRef(carol.Actor)); err != nil || exists {
		t.Fatalf("unselected actor entered local actor namespace, exists=%t err=%v", exists, err)
	}
	if _, err := gitOutput("cat-file", "-e", carolStored.Commit+"^{object}"); err == nil {
		t.Fatal("unselected actor objects were requested")
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolveFullEvent(events, aliceStored.ID); err != nil {
		t.Fatalf("promoted valid actor is not usable: %v", err)
	}
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	transactionFiles, err := filepath.Glob(filepath.Join(gitDir, "hn", "replication", "transactions", "*.json"))
	if err != nil || len(transactionFiles) != 1 {
		t.Fatalf("transaction records = %v, err=%v", transactionFiles, err)
	}
	record, err := os.ReadFile(transactionFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(record), root) || !strings.Contains(string(record), mismatch.ID) || !strings.Contains(string(record), replicationPromoted) {
		t.Fatalf("transaction record leaked paths or omitted exact outcomes:\n%s", record)
	}
	quarantines, err := filepath.Glob(filepath.Join(gitDir, "hn", "replication", "quarantine", "txn-*"))
	if err != nil || len(quarantines) != 0 {
		t.Fatalf("quarantine was not cleaned: %v, err=%v", quarantines, err)
	}
}

func TestReplicationFailurePreservesPreviouslyAcceptedRef(t *testing.T) {
	inReplicationTestRepository(t)
	identity := testIdentity(t, "Alice")
	old := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "new promoted root")
	newOID := mustGitText(t, "rev-parse", "HEAD")
	actorDestination := acceptedActorRef("origin", identity.Actor)
	proposalDestination := acceptedProposalRef("origin", "sha256:"+strings.Repeat("7", 64))
	mustGit(t, "update-ref", actorDestination, old)
	mustGit(t, "update-ref", proposalDestination, old)
	if err := promoteReplicationRefs([]replicationPromotion{
		{Ref: actorDestination, NewOID: newOID, OldOID: old},
		{Ref: proposalDestination, NewOID: newOID, OldOID: strings.Repeat("0", len(old))},
	}); err == nil {
		t.Fatal("promotion with a stale expected-old value succeeded")
	}
	assertRefValue(t, actorDestination, old)
	assertRefValue(t, proposalDestination, old)
}

func inReplicationTestRepository(t *testing.T) string {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	// Compare containment against the same physical root Git resolves.
	root, err = os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, "init", "-q", "-b", "main")
	// Object-immutability assertions require a quiescent fixture: recent Git
	// versions can detach auto-maintenance after commits and race directory
	// snapshots (including creating/removing objects/maintenance.lock).
	mustGit(t, "config", "maintenance.auto", "false")
	mustGit(t, "config", "gc.auto", "0")
	mustGit(t, "config", "user.name", "Test")
	mustGit(t, "config", "user.email", "test@hn.invalid")
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "base")
	return root
}

func assertRefValue(t *testing.T, ref, want string) {
	t.Helper()
	got, exists, err := refValue(ref)
	if err != nil || !exists || got != want {
		t.Fatalf("%s = %q, exists=%t, err=%v; want %s", ref, got, exists, err, want)
	}
}

func assertRefAbsent(t *testing.T, ref string) {
	t.Helper()
	if value, exists, err := refValue(ref); err != nil || exists {
		t.Fatalf("%s = %q, exists=%t, err=%v; want absent", ref, value, exists, err)
	}
}

func TestReplicationRecoveryFlagRejectsCompleteRepository(t *testing.T) {
	inReplicationTestRepository(t)
	err := cmdSync([]string{"--recover-shallow"})
	if err == nil || !strings.Contains(err.Error(), "requires a shallow repository") {
		t.Fatalf("recover-shallow returned %v", err)
	}
}

func TestReplicationRejectsSignedMergeWithWrongProposalHeadIndependently(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	publisher := filepath.Join(root, "publisher")
	remote := filepath.Join(root, "project.git")
	receiver := filepath.Join(root, "receiver")
	if err := os.Mkdir(publisher, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	mustGit(t, "-C", publisher, "init", "-q", "-b", "main")
	mustGit(t, "-C", publisher, "config", "user.name", "Publisher")
	mustGit(t, "-C", publisher, "config", "user.email", "publisher@hn.invalid")
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	alice := testIdentity(t, "Alice")
	writeTestPolicy(t, publisher, PolicyDocument{
		Version: policyVersion, Maintainers: []string{alice.Actor},
		Proposals: ProposalPolicy{RequiredAccepts: 1, AllowAuthorApproval: true},
		Pipelines: map[string]PipelinePolicy{},
	})
	mustGit(t, "add", ".hn/policy.json")
	mustGit(t, "commit", "-q", "-m", "policy")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "candidate")
	head := mustGitText(t, "rev-parse", "HEAD")

	proposalEvent := newEvent(alice, "proposal.open", 1, "")
	proposalEvent.Title = "wrong merge head must not cross quarantine"
	proposalEvent.Base = base
	proposalEvent.Head = head
	proposal, err := appendEvent(proposalEvent, alice)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, head); err != nil {
		t.Fatal(err)
	}
	_, _, policyDigest, err := loadPolicy(base)
	if err != nil {
		t.Fatal(err)
	}
	decision := appendRevisionDecision(t, alice, proposal.ID, policyDigest, nil)
	mergeEvent, err := nextEvent(alice, "proposal.merged")
	if err != nil {
		t.Fatal(err)
	}
	mergeEvent.Subject = proposal.ID
	mergeEvent.Policy = policyDigest
	mergeEvent.Head = base // signed, well-shaped, and deliberately wrong
	mergeEvent.Commit = head
	mergeEvent.Evidence = []string{decision.ID}
	badMerge, err := appendEvent(mergeEvent, alice)
	if err != nil {
		t.Fatal(err)
	}
	bob := testIdentity(t, "Bob")
	bobIssue := newEvent(bob, "issue.open", 1, "")
	bobIssue.Title = "independent valid actor"
	bobStored, err := appendEvent(bobIssue, bob)
	if err != nil {
		t.Fatal(err)
	}

	mustGit(t, "clone", "-q", "--bare", publisher, remote)
	mustGit(t, "remote", "add", "origin", remote)
	for _, ref := range []string{actorRef(alice.Actor), actorRef(bob.Actor), proposalRef(proposal.ID)} {
		mustGit(t, "push", "-q", "origin", ref+":"+ref)
	}
	mustGit(t, "clone", "-q", remote, receiver)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	selection := ReplicationSelection{
		Version: replicationSelectionVersion, Remote: "origin",
		Actors: []string{alice.Actor, bob.Actor}, Proposals: []string{proposal.ID},
		Budgets: defaultReplicationBudgets(),
	}
	result, err := runReplicationTransaction(selection)
	if err != nil {
		t.Fatal(err)
	}
	if !result.hasFailures() {
		t.Fatalf("wrong-head merge transaction reported success: %#v", result.Outcomes)
	}
	assertRefAbsent(t, acceptedActorRef("origin", alice.Actor))
	assertRefValue(t, acceptedActorRef("origin", bob.Actor), bobStored.Commit)
	events, err := collectEvents()
	if err != nil {
		t.Fatalf("bad merge poisoned accepted projection: %v", err)
	}
	if _, err := resolveFullEvent(events, bobStored.ID); err != nil {
		t.Fatalf("independent actor was not usable: %v", err)
	}
	if _, err := resolveFullEvent(events, badMerge.ID); err == nil {
		t.Fatal("wrong-head merge became visible")
	}
}

func TestReplicationErrorsNeverDiscloseTransportOrSetupSecrets(t *testing.T) {
	t.Run("advertisement", func(t *testing.T) {
		inReplicationTestRepository(t)
		secretURL := "file:///definitely-missing/review-user:REVIEW_SECRET/repo.git"
		mustGit(t, "remote", "add", "review", secretURL)
		_, err := runReplicationTransaction(ReplicationSelection{
			Version: replicationSelectionVersion, Remote: "review", All: true,
			Budgets: defaultReplicationBudgets(),
		})
		if err == nil {
			t.Fatal("missing credential-bearing remote succeeded")
		}
		for _, private := range []string{"REVIEW_SECRET", "review-user", "/definitely-missing", secretURL} {
			if strings.Contains(err.Error(), private) {
				t.Fatalf("advertisement error leaked %q: %v", private, err)
			}
		}
		if !strings.Contains(err.Error(), "advertis") || !strings.Contains(err.Error(), "review") {
			t.Fatalf("advertisement error lost safe phase/remote context: %v", err)
		}
	})

	t.Run("quarantine setup", func(t *testing.T) {
		original, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		root := filepath.Join(t.TempDir(), "review-user-REVIEW_SECRET")
		if err := os.Mkdir(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(root); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chdir(original) })
		mustGit(t, "init", "-q", "-b", "main")
		mustGit(t, "config", "user.name", "Test")
		mustGit(t, "config", "user.email", "test@hn.invalid")
		mustGit(t, "commit", "--allow-empty", "-q", "-m", "base")
		remote := filepath.Join(t.TempDir(), "empty.git")
		mustGit(t, "init", "--bare", "-q", remote)
		mustGit(t, "remote", "add", "review", remote)
		gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
		if err := os.WriteFile(filepath.Join(gitDir, "hn"), []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err = runReplicationTransaction(ReplicationSelection{
			Version: replicationSelectionVersion, Remote: "review", All: true,
			Budgets: defaultReplicationBudgets(),
		})
		if err == nil {
			t.Fatal("hostile quarantine setup succeeded")
		}
		for _, private := range []string{"REVIEW_SECRET", root, gitDir} {
			if strings.Contains(err.Error(), private) {
				t.Fatalf("setup error leaked %q: %v", private, err)
			}
		}
		if !strings.Contains(err.Error(), "quarantine setup") || !strings.Contains(err.Error(), "review") {
			t.Fatalf("setup error lost safe phase/remote context: %v", err)
		}
	})

	t.Run("exact fetch", func(t *testing.T) {
		inReplicationTestRepository(t)
		actor := testIdentity(t, "Dangling Actor").Actor
		secretRoot := filepath.Join(t.TempDir(), "review-user-REVIEW_SECRET")
		remote := filepath.Join(secretRoot, "dangling.git")
		if err := os.Mkdir(secretRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		mustGit(t, "init", "--bare", "-q", remote)
		refPath := filepath.Join(remote, filepath.FromSlash(actorRef(actor)))
		if err := os.MkdirAll(filepath.Dir(refPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(refPath, []byte(strings.Repeat("1", 40)+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		mustGit(t, "remote", "add", "review", remote)
		result, err := runReplicationTransaction(ReplicationSelection{
			Version: replicationSelectionVersion, Remote: "review", Actors: []string{actor},
			Budgets: defaultReplicationBudgets(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if !replicationOutcomeHasStatus(result.Outcomes, replicationActor, actor, replicationStructuralInvalid) {
			t.Fatalf("dangling exact ref was not rejected: %#v", result.Outcomes)
		}
		diagnostic := result.Outcomes[0].Diagnostic
		for _, private := range []string{"REVIEW_SECRET", "review-user", secretRoot, remote} {
			if strings.Contains(diagnostic, private) {
				t.Fatalf("fetch diagnostic leaked %q: %s", private, diagnostic)
			}
		}
		if !strings.Contains(diagnostic, "review") || !strings.Contains(diagnostic, actor) {
			t.Fatalf("fetch diagnostic lost safe remote/selection context: %s", diagnostic)
		}
	})
}

func TestReplicationTransactionPhaseFailuresAreSanitizedAndAtomic(t *testing.T) {
	t.Run("object copy", func(t *testing.T) {
		selection, actor, stored, privateRoot := setupSingleActorReplication(t)
		originalCopy := replicationCopyObjects
		replicationCopyObjects = func(_, _ string, _ []string) error {
			return errors.New("copy REVIEW_SECRET from " + privateRoot)
		}
		t.Cleanup(func() { replicationCopyObjects = originalCopy })

		_, err := runReplicationTransaction(selection)
		assertSanitizedReplicationPhaseError(t, err, "object copy", selection.Remote, privateRoot)
		assertRefAbsent(t, acceptedActorRef(selection.Remote, actor.Actor))
		if _, err := gitOutput("cat-file", "-e", stored.Commit+"^{object}"); err == nil {
			t.Fatal("failed object copy made selected object visible")
		}
	})

	t.Run("quarantine cleanup", func(t *testing.T) {
		selection, actor, _, privateRoot := setupSingleActorReplication(t)
		originalRemove := replicationRemoveQuarantine
		replicationRemoveQuarantine = func(_, _ string) error {
			return errors.New("cleanup REVIEW_SECRET at " + privateRoot)
		}
		t.Cleanup(func() { replicationRemoveQuarantine = originalRemove })

		_, err := runReplicationTransaction(selection)
		assertSanitizedReplicationPhaseError(t, err, "quarantine cleanup", selection.Remote, privateRoot)
		assertRefAbsent(t, acceptedActorRef(selection.Remote, actor.Actor))
	})

	t.Run("completion journal after promotion", func(t *testing.T) {
		selection, actor, stored, privateRoot := setupSingleActorReplication(t)
		originalRecord := replicationRecordTransaction
		replicationRecordTransaction = func(gitDir string, result replicationTransactionResult, state string) error {
			if state == "complete" {
				return errors.New("journal REVIEW_SECRET at " + privateRoot)
			}
			return originalRecord(gitDir, result, state)
		}
		t.Cleanup(func() { replicationRecordTransaction = originalRecord })

		result, err := runReplicationTransaction(selection)
		assertSanitizedReplicationPhaseError(t, err, "promotion succeeded", selection.Remote, privateRoot)
		if !strings.Contains(err.Error(), "completion recording failed") {
			t.Fatalf("journal error did not distinguish post-promotion recording failure: %v", err)
		}
		if result.Promoted != 1 || !replicationOutcomeHasStatus(result.Outcomes, replicationActor, actor.Actor, replicationPromoted) {
			t.Fatalf("result did not truthfully preserve completed promotion: %#v", result)
		}
		assertRefValue(t, acceptedActorRef(selection.Remote, actor.Actor), stored.Commit)
		mustGit(t, "cat-file", "-e", stored.Commit+"^{object}")
		gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
		record, readErr := os.ReadFile(filepath.Join(gitDir, "hn", "replication", "transactions", result.ID+".json"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(record), `"state": "validated"`) || strings.Contains(string(record), `"state": "complete"`) {
			t.Fatalf("journal does not truthfully retain its last durable state:\n%s", record)
		}
	})
}

func TestReplicationRejectsHostileValidationLayersIndependently(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	publisher := filepath.Join(root, "publisher")
	remote := filepath.Join(root, "project.git")
	receiver := filepath.Join(root, "receiver")
	if err := os.Mkdir(publisher, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	mustGit(t, "-C", publisher, "init", "-q", "-b", "main")
	mustGit(t, "-C", publisher, "config", "user.name", "Publisher")
	mustGit(t, "-C", publisher, "config", "user.email", "publisher@hn.invalid")
	mustGit(t, "-C", publisher, "commit", "--allow-empty", "-q", "-m", "base")
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}

	good := testIdentity(t, "Good Actor")
	goodEvent := newEvent(good, "issue.open", 1, "")
	goodEvent.Title = "independently usable"
	goodStored, err := appendEvent(goodEvent, good)
	if err != nil {
		t.Fatal(err)
	}

	signatureActor := testIdentity(t, "Bad Signature")
	signaturePrior := appendIssueFixture(t, signatureActor, "accepted signature predecessor")
	appendCorruptIdentityEventCommit(t, signatureActor, signaturePrior, false)
	signatureHead := mustGitText(t, "rev-parse", actorRef(signatureActor.Actor))

	chainActor := testIdentity(t, "Bad Chain")
	chainPrior := appendIssueFixture(t, chainActor, "accepted chain predecessor")
	chainEvent := newEvent(chainActor, "issue.open", 3, chainPrior.ID)
	chainEvent.Title = "skips sequence two"
	chainBad := appendRawSignedEventCommit(t, chainActor, chainEvent, chainPrior.Commit, nil)

	treeActor := testIdentity(t, "Bad Tree")
	treePrior := appendIssueFixture(t, treeActor, "accepted tree predecessor")
	treeEvent := newEvent(treeActor, "issue.open", 2, treePrior.ID)
	treeEvent.Title = "unsupported unsigned attachment"
	treeBad := appendRawSignedEventCommit(t, treeActor, treeEvent, treePrior.Commit, map[string][]byte{"surprise.bin": []byte("unsigned")})

	authorizer := testIdentity(t, "Identity Authorizer")
	target := testIdentity(t, "Expected Identity Target")
	authorizationEvent := newEvent(authorizer, "identity.authorize", 1, "")
	authorizationEvent.Relationship = identityRelationshipDevice
	authorizationEvent.TargetActor = target.Actor
	authorizationEvent.TargetKey = target.PublicKey
	authorization, err := appendEvent(authorizationEvent, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	wrongTarget := testIdentity(t, "Wrong Identity Target")
	identityPrior := appendIssueFixture(t, wrongTarget, "accepted identity predecessor")
	acceptanceEvent := newEvent(wrongTarget, "identity.accept", 2, identityPrior.ID)
	acceptanceEvent.Subject = authorization.ID
	wrongAcceptance, err := appendEvent(acceptanceEvent, wrongTarget)
	if err != nil {
		t.Fatal(err)
	}

	mustGit(t, "clone", "-q", "--bare", publisher, remote)
	mustGit(t, "remote", "add", "origin", remote)
	actors := []*Identity{good, signatureActor, chainActor, treeActor, authorizer, wrongTarget}
	for _, actor := range actors {
		ref := actorRef(actor.Actor)
		mustGit(t, "push", "-q", "origin", ref+":"+ref)
	}
	mustGit(t, "clone", "--no-local", "-q", remote, receiver)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	priorRoots := []string{signaturePrior.Commit, chainPrior.Commit, treePrior.Commit, identityPrior.Commit}
	if err := copyGitObjects(filepath.Join(publisher, ".git"), filepath.Join(receiver, ".git"), priorRoots); err != nil {
		t.Fatal(err)
	}
	for index, actor := range []*Identity{signatureActor, chainActor, treeActor, wrongTarget} {
		mustGit(t, "update-ref", acceptedActorRef("origin", actor.Actor), priorRoots[index])
	}

	actorIDs := make([]string, 0, len(actors))
	for _, actor := range actors {
		actorIDs = append(actorIDs, actor.Actor)
	}
	result, err := runReplicationTransaction(ReplicationSelection{
		Version: replicationSelectionVersion, Remote: "origin", Actors: actorIDs,
		Budgets: defaultReplicationBudgets(),
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRefValue(t, acceptedActorRef("origin", good.Actor), goodStored.Commit)
	assertRefValue(t, acceptedActorRef("origin", authorizer.Actor), authorization.Commit)
	for index, rejected := range []struct {
		actor *Identity
		head  string
	}{
		{signatureActor, signatureHead},
		{chainActor, chainBad.Commit},
		{treeActor, treeBad.Commit},
		{wrongTarget, wrongAcceptance.Commit},
	} {
		assertRefValue(t, acceptedActorRef("origin", rejected.actor.Actor), priorRoots[index])
		if _, err := gitOutput("cat-file", "-e", rejected.head+"^{object}"); err == nil {
			t.Fatalf("rejected actor %s head object became visible", rejected.actor.Actor)
		}
	}
	for _, actor := range []*Identity{signatureActor, chainActor, treeActor} {
		if !replicationOutcomeHasStatus(result.Outcomes, replicationActor, actor.Actor, replicationStructuralInvalid) {
			t.Fatalf("actor %s did not fail structural validation: %#v", actor.Actor, result.Outcomes)
		}
	}
	if !replicationOutcomeHasStatus(result.Outcomes, replicationActor, wrongTarget.Actor, replicationRelationshipBad) {
		t.Fatalf("wrong identity acceptance did not fail relationship validation: %#v", result.Outcomes)
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolveFullEvent(events, goodStored.ID); err != nil {
		t.Fatalf("independent valid actor is not usable: %v", err)
	}
}

func appendIssueFixture(t *testing.T, identity *Identity, title string) *StoredEvent {
	t.Helper()
	event := newEvent(identity, "issue.open", 1, "")
	event.Title = title
	stored, err := appendEvent(event, identity)
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func appendRawSignedEventCommit(t *testing.T, identity *Identity, event Event, parent string, extra map[string][]byte) *StoredEvent {
	t.Helper()
	payload, signature, err := encodeAndSign(event, identity)
	if err != nil {
		t.Fatal(err)
	}
	blobs := map[string][]byte{
		"event.json": payload,
		"signature":  []byte(base64.RawStdEncoding.EncodeToString(signature)),
	}
	for name, contents := range extra {
		blobs[name] = contents
	}
	names := make([]string, 0, len(blobs))
	for name := range blobs {
		names = append(names, name)
	}
	sort.Strings(names)
	var treeInput strings.Builder
	for _, name := range names {
		blob, err := gitInput(blobs[name], nil, "hash-object", "-w", "--stdin")
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&treeInput, "100644 blob %s\t%s\n", strings.TrimSpace(string(blob)), name)
	}
	tree, err := gitInput([]byte(treeInput.String()), nil, "mktree")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"commit-tree", strings.TrimSpace(string(tree)), "-m", "hostile signed event"}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	commit, err := gitInput(nil, nil, args...)
	if err != nil {
		t.Fatal(err)
	}
	commitID := strings.TrimSpace(string(commit))
	if _, err := gitOutput("update-ref", actorRef(identity.Actor), commitID, parent); err != nil {
		t.Fatal(err)
	}
	return &StoredEvent{ID: eventID(payload), Commit: commitID, Event: event, Payload: payload, Signature: signature, Attachments: extra}
}

func setupSingleActorReplication(t *testing.T) (ReplicationSelection, *Identity, *StoredEvent, string) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	privateRoot := filepath.Join(t.TempDir(), "review-user-REVIEW_SECRET")
	publisher := filepath.Join(privateRoot, "publisher")
	remote := filepath.Join(privateRoot, "project.git")
	receiver := filepath.Join(privateRoot, "receiver")
	if err := os.Mkdir(privateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(publisher, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	mustGit(t, "-C", publisher, "init", "-q", "-b", "main")
	mustGit(t, "-C", publisher, "config", "user.name", "Publisher")
	mustGit(t, "-C", publisher, "config", "user.email", "publisher@hn.invalid")
	mustGit(t, "-C", publisher, "commit", "--allow-empty", "-q", "-m", "base")
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	actor := testIdentity(t, "Transaction Actor")
	event := newEvent(actor, "issue.open", 1, "")
	event.Title = "transaction phase fixture"
	stored, err := appendEvent(event, actor)
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, "clone", "-q", "--bare", publisher, remote)
	mustGit(t, "remote", "add", "origin", remote)
	mustGit(t, "push", "-q", "origin", actorRef(actor.Actor)+":"+actorRef(actor.Actor))
	mustGit(t, "clone", "--no-local", "-q", remote, receiver)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput("cat-file", "-e", stored.Commit+"^{object}"); err == nil {
		t.Fatal("test precondition failed: actor object already exists")
	}
	selection := ReplicationSelection{
		Version: replicationSelectionVersion, Remote: "origin", Actors: []string{actor.Actor},
		Budgets: defaultReplicationBudgets(),
	}
	return selection, actor, stored, privateRoot
}

func assertSanitizedReplicationPhaseError(t *testing.T, err error, phase, remote string, privateValues ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s failure unexpectedly succeeded", phase)
	}
	for _, private := range append([]string{"REVIEW_SECRET", "review-user"}, privateValues...) {
		if strings.Contains(err.Error(), private) {
			t.Fatalf("%s error leaked %q: %v", phase, private, err)
		}
	}
	if !strings.Contains(err.Error(), phase) || !strings.Contains(err.Error(), remote) {
		t.Fatalf("%s error lost safe phase/remote context: %v", phase, err)
	}
}

func TestReplicationTransactionBudgetBoundaries(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	publisher := filepath.Join(root, "publisher")
	remote := filepath.Join(root, "project.git")
	if err := os.Mkdir(publisher, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	mustGit(t, "-C", publisher, "init", "-q", "-b", "main")
	mustGit(t, "-C", publisher, "config", "user.name", "Budget Publisher")
	mustGit(t, "-C", publisher, "config", "user.email", "budget@hn.invalid")
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "base")
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "candidate")
	head := mustGitText(t, "rev-parse", "HEAD")
	actor := testIdentity(t, "Budget Actor")
	proposalEvent := newEvent(actor, "proposal.open", 1, "")
	proposalEvent.Title = "budget transaction fixture"
	proposalEvent.Base = base
	proposalEvent.Head = head
	proposal, err := appendEvent(proposalEvent, actor)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, head); err != nil {
		t.Fatal(err)
	}
	requestEvent, err := nextEvent(actor, "run.request")
	if err != nil {
		t.Fatal(err)
	}
	requestEvent.Subject = proposal.ID
	requestEvent.Pipeline = "test"
	requestEvent.Definition = "sha256:" + strings.Repeat("4", 64)
	requestEvent.Commit = head
	request, err := appendEvent(requestEvent, actor)
	if err != nil {
		t.Fatal(err)
	}
	log := []byte(strings.Repeat("measured-attachment-", 512))
	resultEvent, err := nextEvent(actor, "run.result")
	if err != nil {
		t.Fatal(err)
	}
	resultEvent.Subject = request.ID
	resultEvent.Pipeline = request.Event.Pipeline
	resultEvent.Definition = request.Event.Definition
	resultEvent.Commit = head
	resultEvent.Outcome = "passed"
	resultEvent.Log = eventID(log)
	resultEvent.Backend = "sandbox"
	resultEvent.Platform = "test"
	resultEvent.Runner = actor.Actor
	actorHead, err := appendEventWithAttachments(resultEvent, actor, map[string][]byte{"log.txt": log})
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, "clone", "-q", "--bare", publisher, remote)
	mustGit(t, "remote", "add", "origin", remote)
	for _, ref := range []string{actorRef(actor.Actor), proposalRef(proposal.ID)} {
		mustGit(t, "push", "-q", "origin", ref+":"+ref)
	}
	empty := filepath.Join(root, "empty.git")
	mustGit(t, "init", "--bare", "-q", empty)
	actorMeasurements, err := measureQuarantinedSelection(filepath.Join(publisher, ".git"), empty, replicationActor, actorHead.Commit)
	if err != nil {
		t.Fatal(err)
	}
	proposalMeasurements, err := measureQuarantinedSelection(filepath.Join(publisher, ".git"), empty, replicationProposal, head)
	if err != nil {
		t.Fatal(err)
	}
	if proposalMeasurements.Objects > actorMeasurements.Objects || proposalMeasurements.LargestObjectBytes > actorMeasurements.LargestObjectBytes || proposalMeasurements.TotalBytes > actorMeasurements.TotalBytes {
		t.Fatalf("proposal graph must fit actor-derived isolated limits: actor=%#v proposal=%#v", actorMeasurements, proposalMeasurements)
	}

	dimensions := []struct {
		name  string
		value int64
		set   func(*ReplicationBudgets, int64)
	}{
		{"max-events", actorMeasurements.Events, func(b *ReplicationBudgets, n int64) { b.MaxEvents = n }},
		{"max-objects", actorMeasurements.Objects, func(b *ReplicationBudgets, n int64) { b.MaxObjects = n }},
		{"max-object-bytes", actorMeasurements.LargestObjectBytes, func(b *ReplicationBudgets, n int64) { b.MaxObjectBytes = n }},
		{"max-attachment-bytes", actorMeasurements.LargestAttachmentBytes, func(b *ReplicationBudgets, n int64) { b.MaxAttachmentBytes = n }},
		{"max-total-bytes", actorMeasurements.TotalBytes, func(b *ReplicationBudgets, n int64) { b.MaxTotalBytes = n }},
	}
	for _, dimension := range dimensions {
		for _, boundary := range []struct {
			name   string
			limit  int64
			reject bool
		}{
			{"measured-one-above-limit", dimension.value - 1, true},
			{"measured-exactly-at-limit", dimension.value, false},
			{"measured-one-below-limit", dimension.value + 1, false},
		} {
			t.Run(dimension.name+"/"+boundary.name, func(t *testing.T) {
				receiver := filepath.Join(root, strings.ReplaceAll(dimension.name+"-"+boundary.name, "/", "-"))
				mustGit(t, "clone", "--no-local", "-q", remote, receiver)
				if err := os.Chdir(receiver); err != nil {
					t.Fatal(err)
				}
				if _, err := gitOutput("cat-file", "-e", actorHead.Commit+"^{object}"); err == nil {
					t.Fatalf("test precondition failed: selected actor object already exists before replication")
				}
				budgets := ReplicationBudgets{
					MaxEvents: 1 << 30, MaxObjects: 1 << 30, MaxObjectBytes: 1 << 60,
					MaxAttachmentBytes: 1 << 60, MaxTotalBytes: 1 << 60,
				}
				dimension.set(&budgets, boundary.limit)
				transaction, err := runReplicationTransaction(ReplicationSelection{
					Version: replicationSelectionVersion, Remote: "origin",
					Actors: []string{actor.Actor}, Proposals: []string{proposal.ID}, Budgets: budgets,
				})
				if err != nil {
					t.Fatal(err)
				}
				if boundary.reject {
					if !transaction.hasFailures() || !replicationOutcomeHasStatus(transaction.Outcomes, replicationActor, actor.Actor, replicationOverBudget) {
						t.Fatalf("%s did not reject actor at limit %d: %#v", dimension.name, boundary.limit, transaction.Outcomes)
					}
					assertRefAbsent(t, acceptedActorRef("origin", actor.Actor))
					assertRefAbsent(t, acceptedProposalRef("origin", proposal.ID))
					if _, err := gitOutput("cat-file", "-e", actorHead.Commit+"^{object}"); err == nil {
						t.Fatalf("%s rejected actor object became visible", dimension.name)
					}
				} else {
					if transaction.hasFailures() {
						t.Fatalf("%s rejected exact/below boundary: %#v", dimension.name, transaction.Outcomes)
					}
					assertRefValue(t, acceptedActorRef("origin", actor.Actor), actorHead.Commit)
					assertRefValue(t, acceptedProposalRef("origin", proposal.ID), head)
					mustGit(t, "cat-file", "-e", actorHead.Commit+"^{object}")
				}
				if err := os.Chdir(publisher); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func replicationOutcomeHasStatus(outcomes []ReplicationOutcome, kind, id, status string) bool {
	for _, outcome := range outcomes {
		if outcome.Kind == kind && outcome.ID == id && outcome.Status == status {
			return true
		}
	}
	return false
}
