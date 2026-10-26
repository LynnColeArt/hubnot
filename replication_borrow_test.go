package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// borrowFixture publishes a proposal whose base already holds many code
// objects, then clones a receiver that has that code but none of the Hubnot
// refs. It returns the receiver path and the selected actor and proposal.
type borrowClone int

const (
	fullClone borrowClone = iota
	shallowClone
	partialClone
)

func borrowFixture(t *testing.T, clone borrowClone) (receiver string, actor *Identity, proposal *StoredEvent, head string) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	root := t.TempDir()
	publisher := filepath.Join(root, "publisher")
	remote := filepath.Join(root, "project.git")
	receiver = filepath.Join(root, "receiver")
	if err := os.Mkdir(publisher, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "-C", publisher, "init", "-q", "-b", "main")
	mustGit(t, "-C", publisher, "config", "user.name", "Publisher")
	mustGit(t, "-C", publisher, "config", "user.email", "publisher@hn.invalid")
	for i := 0; i < 300; i++ {
		name := filepath.Join(publisher, "vendor", fmt.Sprintf("crate%03d.rs", i))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(fmt.Sprintf("pub fn f%d() {}\n", i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustGit(t, "-C", publisher, "add", "-A")
	mustGit(t, "-C", publisher, "commit", "-q", "-m", "base with vendored code")
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "checkout", "-q", "-b", "candidate")
	if err := os.WriteFile(filepath.Join(publisher, "change.rs"), []byte("pub fn change() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "change.rs")
	mustGit(t, "commit", "-q", "-m", "candidate")
	head = mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "checkout", "-q", "main")
	actor = testIdentity(t, "Borrow Actor")
	opened := newEvent(actor, "proposal.open", 1, "")
	opened.Title = "borrow fixture"
	opened.Base = base
	opened.Head = head
	proposal, err = appendEvent(opened, actor)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, head); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "clone", "-q", "--bare", publisher, remote)
	mustGit(t, "-C", remote, "config", "uploadpack.allowFilter", "true")
	mustGit(t, "remote", "add", "origin", remote)
	for _, ref := range []string{actorRef(actor.Actor), proposalRef(proposal.ID)} {
		mustGit(t, "push", "-q", "origin", ref+":"+ref)
	}
	cloneArgs := []string{"clone", "--no-local", "-q", "--branch", "main", "--single-branch"}
	switch clone {
	case shallowClone:
		cloneArgs = append(cloneArgs, "--depth", "1")
	case partialClone:
		cloneArgs = append(cloneArgs, "--filter=blob:none")
	}
	mustGit(t, append(cloneArgs, "file://"+remote, receiver)...)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	if clone != partialClone {
		mustGit(t, "remote", "set-url", "origin", remote)
	}
	// No lazy fetch: in a partial clone the probe itself would fetch it.
	if _, err := gitInputWithDirectory("", nil, []string{"GIT_NO_LAZY_FETCH=1"}, "cat-file", "-e", head+"^{commit}"); err == nil {
		t.Fatal("test precondition failed: receiver already has the candidate commit")
	}
	if err := cmdReplication([]string{"select", "origin", "--actor", actor.Actor, "--proposal", proposal.ID}); err != nil {
		t.Fatal(err)
	}
	return receiver, actor, proposal, head
}

// quarantineState reports the quarantine's alternates file and how many
// objects it stores itself, while the transaction is between fetch and copy.
func quarantineState(t *testing.T) (alternates string, localObjects int) {
	t.Helper()
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	found, err := filepath.Glob(filepath.Join(gitDir, "hn", "replication", "quarantine", "txn-*"))
	if err != nil || len(found) != 1 {
		t.Fatalf("quarantine directories = %v, err=%v", found, err)
	}
	if data, err := os.ReadFile(filepath.Join(found[0], "objects", "info", "alternates")); err == nil {
		alternates = strings.TrimSpace(string(data))
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, line := range strings.Split(mustGitTextAtDir(t, found[0], "count-objects", "-v"), "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if ok && (key == "count" || key == "in-pack") {
			n, err := strconv.Atoi(value)
			if err != nil {
				t.Fatal(err)
			}
			localObjects += n
		}
	}
	return alternates, localObjects
}

func TestReplicationQuarantineBorrowsMainObjectsAndFetchesOnlyNewOnes(t *testing.T) {
	receiver, actor, proposal, head := borrowFixture(t, fullClone)
	var alternates string
	var local int
	replicationAfterFetchHook = func() error {
		alternates, local = quarantineState(t)
		return nil
	}
	t.Cleanup(func() { replicationAfterFetchHook = nil })
	if output, err := captureTestOutput(t, func() error { return cmdSync(nil) }); err != nil {
		t.Fatalf("sync failed: %v\n%s", err, output)
	}
	wantObjects, err := filepath.EvalSymlinks(filepath.Join(receiver, ".git", "objects"))
	if err != nil {
		t.Fatal(err)
	}
	gotObjects, err := filepath.EvalSymlinks(alternates)
	if err != nil || gotObjects != wantObjects {
		t.Fatalf("quarantine alternates = %q, want the receiver object database %q (err=%v)", alternates, wantObjects, err)
	}
	// The receiver already has the 300 vendored blobs; only the event commit,
	// its tree and blobs, and the candidate commit, tree and new blob are new.
	if local == 0 || local > 20 {
		t.Fatalf("quarantine stored %d objects itself; want only the new ones", local)
	}
	assertRefValue(t, acceptedActorRef("origin", actor.Actor), proposal.Commit)
	assertRefValue(t, acceptedProposalRef("origin", proposal.ID), head)
	if _, err := gitOutput("cat-file", "-e", head+"^{commit}"); err != nil {
		t.Fatal("candidate commit was not copied into the main object database")
	}
	if strings.TrimSpace(mustGitText(t, "fsck", "--connectivity-only", "--no-dangling")) != "" {
		t.Fatal("main object database is not connected after import")
	}
}

func TestReplicationBorrowedObjectsStillCountAgainstBudgets(t *testing.T) {
	_, _, proposal, _ := borrowFixture(t, fullClone)
	// The candidate reaches more than 300 objects, all but a few borrowed from
	// the receiver. Borrowing must not discount them.
	if err := cmdReplication([]string{"select", "origin", "--proposal", proposal.ID, "--max-objects", "100"}); err != nil {
		t.Fatal(err)
	}
	output, err := captureTestOutput(t, func() error { return cmdSync(nil) })
	if err == nil {
		t.Fatalf("sync accepted a selection over its object budget:\n%s", output)
	}
	if !strings.Contains(output+err.Error(), "max-objects") {
		t.Fatalf("sync failure did not name the object budget:\n%s\n%v", output, err)
	}
	assertRefAbsent(t, acceptedProposalRef("origin", proposal.ID))
}

func assertIsolatedQuarantineImports(t *testing.T, clone borrowClone) {
	t.Helper()
	_, actor, proposal, head := borrowFixture(t, clone)
	sawQuarantine := false
	replicationAfterFetchHook = func() error {
		sawQuarantine = true
		if alternates, _ := quarantineState(t); alternates != "" {
			t.Fatalf("quarantine borrowed %q from an incomplete receiver", alternates)
		}
		return nil
	}
	t.Cleanup(func() { replicationAfterFetchHook = nil })
	output, err := captureTestOutput(t, func() error { return cmdSync(nil) })
	if !sawQuarantine {
		t.Fatalf("sync never reached the quarantine: %v\n%s", err, output)
	}
	if err != nil {
		t.Fatalf("sync failed with an isolated quarantine: %v\n%s", err, output)
	}
	assertRefValue(t, acceptedActorRef("origin", actor.Actor), proposal.Commit)
	assertRefValue(t, acceptedProposalRef("origin", proposal.ID), head)
}

func TestReplicationShallowReceiverKeepsIsolatedQuarantine(t *testing.T) {
	assertIsolatedQuarantineImports(t, shallowClone)
}

func TestReplicationPartialCloneReceiverKeepsIsolatedQuarantine(t *testing.T) {
	assertIsolatedQuarantineImports(t, partialClone)
}

func TestGitObjectsPresentIgnoresReplacementRefs(t *testing.T) {
	inReplicationTestRepository(t)
	present := mustGitText(t, "rev-parse", "HEAD")
	absent := strings.Repeat("ab", 20)
	mustGit(t, "update-ref", "refs/replace/"+absent, present)
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	got, err := gitObjectsPresentAt(gitDir, []string{absent, present})
	if err != nil {
		t.Fatal(err)
	}
	if got[absent] || !got[present] {
		t.Fatalf("presence = %v; a replacement must not make an absent object present", got)
	}
	if _, err := gitObjectsPresentAt(gitDir, []string{strings.ToUpper(present)}); err == nil {
		t.Fatal("non-canonical object ID was accepted")
	}
	if got, err := gitObjectsPresentAt(gitDir, nil); err != nil || len(got) != 0 {
		t.Fatalf("empty input = %v, %v", got, err)
	}
}
