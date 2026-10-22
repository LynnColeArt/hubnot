package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncPublishesNewActorDespiteStaleLocalCopyOfRemoteActor(t *testing.T) {
	selection, owner, original, root := setupSingleActorReplication(t)
	receiver, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runReplicationTransaction(selection); err != nil {
		t.Fatal(err)
	}
	// Older clones can retain an imported actor in the local publication
	// namespace, even though that actor has since advanced elsewhere.
	mustGit(t, "update-ref", actorRef(owner.Actor), original.Commit)
	if err := os.Chdir(filepath.Join(root, "publisher")); err != nil {
		t.Fatal(err)
	}
	advanced := appendPublicationIssue(t, owner, "new owner event")
	mustGit(t, "push", "-q", "origin", actorRef(owner.Actor)+":"+actorRef(owner.Actor))
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	seat := testIdentity(t, "Independent seat")
	local := appendPublicationIssue(t, seat, "new seat receipt")
	for range 2 {
		if err := cmdSync([]string{"origin"}); err != nil {
			t.Fatalf("sync with stale copied actor: %v", err)
		}
		assertRefValue(t, actorRef(owner.Actor), original.Commit)
		assertRefValue(t, acceptedActorRef("origin", owner.Actor), advanced.Commit)
		assertPublishedActor(t, filepath.Join(root, "project.git"), owner.Actor, advanced.Commit)
		assertPublishedActor(t, filepath.Join(root, "project.git"), seat.Actor, local.Commit)
	}
	verifier := filepath.Join(root, "verifier")
	mustGit(t, "clone", "--no-local", "-q", filepath.Join(root, "project.git"), verifier)
	if err := os.Chdir(verifier); err != nil {
		t.Fatal(err)
	}
	if err := cmdSync([]string{"origin"}); err != nil {
		t.Fatal(err)
	}
	events, err := collectEvents()
	if err != nil || len(events) != 3 {
		t.Fatalf("verified remote events=%d, %v", len(events), err)
	}
	assertRefValue(t, acceptedActorRef("origin", seat.Actor), local.Commit)
}

func TestSyncDoesNotHideDivergentActorPublication(t *testing.T) {
	selection, owner, original, root := setupSingleActorReplication(t)
	receiver, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runReplicationTransaction(selection); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "update-ref", actorRef(owner.Actor), original.Commit)
	if err := os.Chdir(filepath.Join(root, "publisher")); err != nil {
		t.Fatal(err)
	}
	remoteHead := appendPublicationIssue(t, owner, "remote fork")
	mustGit(t, "push", "-q", "origin", actorRef(owner.Actor)+":"+actorRef(owner.Actor))
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	localHead := appendPublicationIssue(t, owner, "different local fork")
	if err := cmdSync([]string{"origin"}); err == nil {
		t.Fatal("divergent history unexpectedly synchronized")
	}
	assertRefValue(t, actorRef(owner.Actor), localHead.Commit)
	assertPublishedActor(t, filepath.Join(root, "project.git"), owner.Actor, remoteHead.Commit)
}

func TestSyncStillPublishesNewerLocalActorHistory(t *testing.T) {
	selection, actor, original, root := setupSingleActorReplication(t)
	if _, err := runReplicationTransaction(selection); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "update-ref", actorRef(actor.Actor), original.Commit)
	newer := appendPublicationIssue(t, actor, "unpublished local progress")
	if err := cmdSync([]string{"origin"}); err != nil {
		t.Fatal(err)
	}
	assertPublishedActor(t, filepath.Join(root, "project.git"), actor.Actor, newer.Commit)
}

func TestSyncDoesNotUseCachedAcceptedHeadToSkipMissingRemoteActor(t *testing.T) {
	selection, actor, original, root := setupSingleActorReplication(t)
	if _, err := runReplicationTransaction(selection); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "update-ref", actorRef(actor.Actor), original.Commit)
	remote := filepath.Join(root, "project.git")
	mustGit(t, "--git-dir="+remote, "update-ref", "-d", actorRef(actor.Actor))
	if err := cmdSync([]string{"origin"}); err != nil {
		t.Fatal(err)
	}
	assertPublishedActor(t, remote, actor.Actor, original.Commit)
}

func appendPublicationIssue(t *testing.T, identity *Identity, title string) *StoredEvent {
	t.Helper()
	event, err := nextEvent(identity, "issue.open")
	if err != nil {
		t.Fatal(err)
	}
	event.Title = title
	stored, err := appendEvent(event, identity)
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func assertPublishedActor(t *testing.T, remote, actor, head string) {
	t.Helper()
	got := strings.Fields(mustGitText(t, "ls-remote", "--refs", remote, actorRef(actor)))
	if len(got) != 2 || got[0] != head || got[1] != actorRef(actor) {
		t.Fatalf("published actor=%v, want %s %s", got, head, actorRef(actor))
	}
}
