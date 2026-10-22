package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMemoryReplicationSelectionCompatibilityAndValidation(t *testing.T) {
	inReplicationTestRepository(t)
	actor := deterministicMemoryIdentity().Actor
	streamA := fullMemoryID("1")
	streamB := fullMemoryID("2")
	proposal := "sha256:" + strings.Repeat("3", 64)

	if err := cmdReplication([]string{
		"select", "origin", "--actor", actor, "--proposal", proposal,
		"--memory", streamB, "--memory", streamA,
	}); err != nil {
		t.Fatal(err)
	}
	selection, explicit, err := loadReplicationSelection("origin")
	if err != nil {
		t.Fatal(err)
	}
	if !explicit || !reflect.DeepEqual(selection.Memories, []string{streamA, streamB}) {
		t.Fatalf("memory selection = %#v, explicit=%t", selection.Memories, explicit)
	}
	shown, err := captureTestOutput(t, func() error { return cmdReplication([]string{"show", "origin"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shown, "memory: "+streamA) || strings.Index(shown, streamA) > strings.Index(shown, streamB) {
		t.Fatalf("memory selection display is not deterministic:\n%s", shown)
	}

	cases := [][]string{
		{"select", "--memory", streamA[:20]},
		{"select", "--memory", strings.ToUpper(streamA)},
		{"select", "--memory", streamA + " "},
		{"select", "--memory", streamA, "--memory", streamA},
		{"select", "--all", "--memory", streamA},
	}
	for _, args := range cases {
		if err := cmdReplication(args); err == nil {
			t.Fatalf("cmdReplication(%q) succeeded", args)
		}
	}

	legacy := ReplicationSelection{
		Version: replicationSelectionVersion, Remote: "legacy", Actors: []string{actor},
		Budgets: defaultReplicationBudgets(),
	}
	path, err := replicationSelectionPath("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, explicit, err := loadReplicationSelection("legacy")
	if err != nil || !explicit || len(loaded.Memories) != 0 || !reflect.DeepEqual(loaded.Actors, legacy.Actors) {
		t.Fatalf("legacy selection = %#v, explicit=%t, err=%v", loaded, explicit, err)
	}
}

func TestMemoryReplicationDiscoveryRequestAndPromotion(t *testing.T) {
	root := t.TempDir()
	publisher := filepath.Join(root, "publisher")
	remote := filepath.Join(root, "project.git")
	receiver := filepath.Join(root, "receiver")
	mustGit(t, "init", "-q", "-b", "main", publisher)
	mustGit(t, "-C", publisher, "config", "user.name", "Publisher")
	mustGit(t, "-C", publisher, "config", "user.email", "publisher@hn.invalid")
	mustGit(t, "-C", publisher, "commit", "--allow-empty", "-q", "-m", "base")
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	identity := deterministicMemoryIdentity()
	envelope := validMemoryEnvelopeFixture(memoryOperationRecord)
	envelope.Record.Anchor = MemoryAnchor{Commit: mustGitText(t, "rev-parse", "HEAD")}
	envelope.Record.Applicability = Applicability{Mode: memoryApplicabilityExact}
	envelope.Record.Evidence = []string{}
	stored, err := appendMemory(envelope, identity)
	if err != nil {
		t.Fatal(err)
	}
	localRef, err := memoryRef(identity.Actor, stored.Envelope.Stream)
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, "clone", "-q", "--bare", publisher, remote)
	mustGit(t, "remote", "add", "origin", remote)
	mustGit(t, "push", "-q", "origin", localRef+":"+localRef)
	mustGit(t, "clone", "-q", remote, receiver)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}

	selection := ReplicationSelection{
		Version: replicationSelectionVersion, Remote: "origin", Memories: []string{stored.Envelope.Stream},
		Budgets: defaultReplicationBudgets(),
	}
	result, err := runReplicationTransaction(selection)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Outcomes) != 1 || result.Outcomes[0].Kind != replicationMemory || result.Outcomes[0].ID != stored.Envelope.Stream || result.Outcomes[0].Status != replicationPromoted {
		t.Fatalf("memory outcomes = %#v", result.Outcomes)
	}
	accepted, err := acceptedMemoryRef("origin", identity.Actor, stored.Envelope.Stream)
	if err != nil {
		t.Fatal(err)
	}
	assertRefValue(t, accepted, stored.Commit)
	memories, err := collectMemories()
	if err != nil || len(memories) != 1 || memories[0].ID != stored.ID {
		t.Fatalf("accepted memories = %#v, err=%v", memories, err)
	}
}

func TestMemoryReplicationExplicitCollaborationSelectionRequestsNoMemory(t *testing.T) {
	inReplicationTestRepository(t)
	actor := deterministicMemoryIdentity().Actor
	stream := fullMemoryID("4")
	ref, err := memoryRef(actor, stream)
	if err != nil {
		t.Fatal(err)
	}
	requests, _, err := resolveReplicationRequests(ReplicationSelection{
		Version: replicationSelectionVersion, Remote: "origin", Actors: []string{actor},
		Budgets: defaultReplicationBudgets(),
	}, testAdvertisedRemote(t, map[string]string{ref: mustGitText(t, "rev-parse", "HEAD")}))
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range requests {
		if request.Kind == replicationMemory {
			t.Fatalf("actor-only selection requested memory: %#v", request)
		}
	}
}

func TestMemoryReplicationDiscoveryRejectsAmbiguousOwnersAndIgnoresMalformedRefs(t *testing.T) {
	inReplicationTestRepository(t)
	stream := fullMemoryID("5")
	alice := deterministicMemoryIdentity()
	bob := testIdentity(t, "Bob Memory")
	validAlice, _ := memoryRef(alice.Actor, stream)
	validBob, _ := memoryRef(bob.Actor, stream)
	malformed := memoryRefPrefix + alice.Actor + "/" + strings.Repeat("A", 64)
	head := mustGitText(t, "rev-parse", "HEAD")
	remote := testAdvertisedRemote(t, map[string]string{validAlice: head, validBob: head, malformed: head})
	requests, outcomes, err := resolveReplicationRequests(ReplicationSelection{
		Version: replicationSelectionVersion, Remote: "origin", All: true,
		Budgets: defaultReplicationBudgets(),
	}, remote)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range requests {
		if request.Kind == replicationMemory {
			t.Fatalf("ambiguous or malformed memory was requested: %#v", request)
		}
	}
	if !replicationOutcomeHasStatus(outcomes, replicationMemory, stream, replicationStructuralInvalid) {
		t.Fatalf("ambiguous memory owners were not a scoped failure: %#v", outcomes)
	}
}

func TestMemoryReplicationPublishUsesExactLocalRefs(t *testing.T) {
	inReplicationTestRepository(t)
	remote := filepath.Join(t.TempDir(), "published.git")
	initBareMainRemote(t, remote)
	mustGit(t, "remote", "add", "origin", remote)
	identity := deterministicMemoryIdentity()
	stored := appendReplicableMemory(t, identity, defaultMemoryStream(identity.Actor), nil, "publish")
	ref, _ := memoryRef(identity.Actor, stored.Envelope.Stream)
	if err := publishLocalFacts("origin", nil); err != nil {
		t.Fatal(err)
	}
	advertised := mustGitText(t, "ls-remote", "--refs", remote, ref)
	if !strings.Contains(advertised, stored.Commit+"\t"+ref) {
		t.Fatalf("published refs = %q, want exact %s", advertised, ref)
	}
}

func TestMemoryReplicationValidationIsolatedFromValidStream(t *testing.T) {
	root := t.TempDir()
	publisher := filepath.Join(root, "publisher")
	remote := filepath.Join(root, "project.git")
	receiver := filepath.Join(root, "receiver")
	mustGit(t, "init", "-q", "-b", "main", publisher)
	mustGit(t, "-C", publisher, "config", "user.name", "Publisher")
	mustGit(t, "-C", publisher, "config", "user.email", "publisher@hn.invalid")
	mustGit(t, "-C", publisher, "commit", "--allow-empty", "-q", "-m", "base")
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	alice := deterministicMemoryIdentity()
	valid := appendReplicableMemory(t, alice, defaultMemoryStream(alice.Actor), nil, "valid")
	bob := testIdentity(t, "Bob Memory")
	invalid := appendReplicableMemory(t, bob, defaultMemoryStream(bob.Actor), nil, "invalid signature")
	badSignature := append([]byte(nil), invalid.Signature...)
	badSignature[0] ^= 0xff
	badCommit := writeRawMemoryCommit(t, invalid.Payload, badSignature, nil)
	badRef, _ := memoryRef(bob.Actor, invalid.Envelope.Stream)
	mustGit(t, "update-ref", badRef, badCommit)

	mustGit(t, "clone", "-q", "--bare", publisher, remote)
	mustGit(t, "remote", "add", "origin", remote)
	validRef, _ := memoryRef(alice.Actor, valid.Envelope.Stream)
	for _, ref := range []string{validRef, badRef} {
		mustGit(t, "push", "-q", "origin", ref+":"+ref)
	}
	mustGit(t, "clone", "-q", remote, receiver)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	result, err := runReplicationTransaction(ReplicationSelection{
		Version: replicationSelectionVersion, Remote: "origin",
		Memories: []string{valid.Envelope.Stream, invalid.Envelope.Stream}, Budgets: defaultReplicationBudgets(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !replicationOutcomeHasStatus(result.Outcomes, replicationMemory, valid.Envelope.Stream, replicationPromoted) ||
		!replicationOutcomeHasStatus(result.Outcomes, replicationMemory, invalid.Envelope.Stream, replicationStructuralInvalid) {
		t.Fatalf("isolated validation outcomes = %#v", result.Outcomes)
	}
	validAccepted, _ := acceptedMemoryRef("origin", alice.Actor, valid.Envelope.Stream)
	badAccepted, _ := acceptedMemoryRef("origin", bob.Actor, invalid.Envelope.Stream)
	assertRefValue(t, validAccepted, valid.Commit)
	assertRefAbsent(t, badAccepted)
}

func TestMemoryReplicationMixedHostileTransactionPreservesCollaborationBytes(t *testing.T) {
	root := t.TempDir()
	publisher := filepath.Join(root, "publisher")
	remote := filepath.Join(root, "project.git")
	receiver := filepath.Join(root, "receiver")
	mustGit(t, "init", "-q", "-b", "main", publisher)
	mustGit(t, "-C", publisher, "config", "user.name", "Publisher")
	mustGit(t, "-C", publisher, "config", "user.email", "publisher@hn.invalid")
	mustGit(t, "-C", publisher, "commit", "--allow-empty", "-q", "-m", "base")
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "candidate")
	head := mustGitText(t, "rev-parse", "HEAD")
	alice := deterministicMemoryIdentity()
	proposalEvent := newEvent(alice, "proposal.open", 1, "")
	proposalEvent.Title, proposalEvent.Base, proposalEvent.Head = "mixed candidate", base, head
	proposal, err := appendEvent(proposalEvent, alice)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, head); err != nil {
		t.Fatal(err)
	}
	bob := testIdentity(t, "Independent Collaborator")
	issue := newEvent(bob, "issue.open", 1, "")
	issue.Title = "collaboration survives hostile memory"
	if _, err := appendEvent(issue, bob); err != nil {
		t.Fatal(err)
	}
	wantEvents, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	valid := appendReplicableMemory(t, alice, fullMemoryID("1"), nil, "valid memory")
	bad := appendReplicableMemory(t, bob, fullMemoryID("2"), nil, "invalid signature")
	badSignature := append([]byte(nil), bad.Signature...)
	badSignature[0] ^= 0xff
	badCommit := writeRawMemoryCommit(t, bad.Payload, badSignature, nil)
	badRef, _ := memoryRef(bob.Actor, bad.Envelope.Stream)
	mustGit(t, "update-ref", badRef, badCommit)
	missingEnvelope := validMemoryEnvelopeFixture(memoryOperationRecord)
	missingEnvelope.Stream = fullMemoryID("3")
	missingEnvelope.Record.Anchor = MemoryAnchor{Commit: strings.Repeat("9", 40)}
	missingEnvelope.Record.Applicability = Applicability{Mode: memoryApplicabilityExact}
	missingEnvelope.Record.Evidence = []string{}
	missing, err := appendMemory(missingEnvelope, alice)
	if err != nil {
		t.Fatal(err)
	}
	over := appendReplicableMemory(t, alice, fullMemoryID("4"), nil, "over one")
	over = appendReplicableMemory(t, alice, over.Envelope.Stream, over, "over two")
	over = appendReplicableMemory(t, alice, over.Envelope.Stream, over, "over three")

	initBareMainRemote(t, remote)
	mustGit(t, "remote", "add", "origin", remote)
	refs := []string{actorRef(alice.Actor), actorRef(bob.Actor), proposalRef(proposal.ID)}
	for _, stored := range []*StoredMemory{valid, missing, over} {
		ref, _ := memoryRef(stored.Envelope.Actor, stored.Envelope.Stream)
		refs = append(refs, ref)
	}
	refs = append(refs, badRef)
	for _, ref := range append([]string{"main"}, refs...) {
		mustGit(t, "push", "-q", "origin", ref+":"+ref)
	}
	mustGit(t, "clone", "-q", remote, receiver)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	budgets := defaultReplicationBudgets()
	budgets.MaxEvents = 2
	result, err := runReplicationTransaction(ReplicationSelection{
		Version: replicationSelectionVersion, Remote: "origin",
		Actors: []string{alice.Actor, bob.Actor}, Proposals: []string{proposal.ID},
		Memories: []string{valid.Envelope.Stream, bad.Envelope.Stream, missing.Envelope.Stream, over.Envelope.Stream}, Budgets: budgets,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ kind, id, status string }{
		{replicationActor, alice.Actor, replicationPromoted}, {replicationActor, bob.Actor, replicationPromoted},
		{replicationProposal, proposal.ID, replicationPromoted}, {replicationMemory, valid.Envelope.Stream, replicationPromoted},
		{replicationMemory, bad.Envelope.Stream, replicationStructuralInvalid},
		{replicationMemory, missing.Envelope.Stream, replicationDependencyMissing},
		{replicationMemory, over.Envelope.Stream, replicationOverBudget},
	} {
		if !replicationOutcomeHasStatus(result.Outcomes, want.kind, want.id, want.status) {
			t.Fatalf("missing outcome %s %s %s: %#v", want.kind, want.id, want.status, result.Outcomes)
		}
	}
	gotEvents, err := collectEvents()
	if err != nil || len(gotEvents) != len(wantEvents) {
		t.Fatalf("collaboration projection = %#v, err=%v", gotEvents, err)
	}
	for index := range wantEvents {
		if gotEvents[index].ID != wantEvents[index].ID || !reflect.DeepEqual(gotEvents[index].Payload, wantEvents[index].Payload) {
			t.Fatalf("collaboration bytes changed at %d", index)
		}
	}
	for _, stored := range []*StoredMemory{bad, missing, over} {
		ref, _ := acceptedMemoryRef("origin", stored.Envelope.Actor, stored.Envelope.Stream)
		assertRefAbsent(t, ref)
	}
}

func TestMemoryReplicationBudgetCountsCommits(t *testing.T) {
	publisher, remote, receiver, identity, first := setupMemoryReplicationFixture(t)
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	second := appendReplicableMemory(t, identity, first.Envelope.Stream, first, "second")
	empty := filepath.Join(t.TempDir(), "empty.git")
	mustGit(t, "init", "--bare", "-q", empty)
	fullMeasurements, err := measureQuarantinedSelection(filepath.Join(publisher, ".git"), empty, replicationMemory, second.Commit)
	if err != nil {
		t.Fatal(err)
	}
	if fullMeasurements.LargestAttachmentBytes != 0 {
		t.Fatalf("valid v0 memory attachment measurement = %d", fullMeasurements.LargestAttachmentBytes)
	}
	if err := copyGitObjects(filepath.Join(publisher, ".git"), empty, []string{first.Commit}); err != nil {
		t.Fatal(err)
	}
	incremental, err := measureQuarantinedSelectionUnderBudgets(filepath.Join(publisher, ".git"), empty, replicationMemory, second.Commit, first.Commit, defaultReplicationBudgets(), "incremental memory")
	if err != nil {
		t.Fatal(err)
	}
	if incremental.Objects >= fullMeasurements.Objects || incremental.TotalBytes >= fullMeasurements.TotalBytes {
		t.Fatalf("same-stream accepted root was not discounted: full=%#v incremental=%#v", fullMeasurements, incremental)
	}
	chargedDespiteResidue, err := measureQuarantinedSelectionUnderBudgets(filepath.Join(publisher, ".git"), empty, replicationMemory, second.Commit, "", defaultReplicationBudgets(), "unreferenced memory")
	if err != nil || !reflect.DeepEqual(chargedDespiteResidue, fullMeasurements) {
		t.Fatalf("unreferenced/shared objects weakened budget: got=%#v want=%#v err=%v", chargedDespiteResidue, fullMeasurements, err)
	}
	for _, dimension := range []struct {
		name  string
		value int64
		set   func(*ReplicationBudgets, int64)
	}{
		{"events", fullMeasurements.Events, func(b *ReplicationBudgets, n int64) { b.MaxEvents = n }},
		{"objects", fullMeasurements.Objects, func(b *ReplicationBudgets, n int64) { b.MaxObjects = n }},
		{"object-bytes", fullMeasurements.LargestObjectBytes, func(b *ReplicationBudgets, n int64) { b.MaxObjectBytes = n }},
		{"total-bytes", fullMeasurements.TotalBytes, func(b *ReplicationBudgets, n int64) { b.MaxTotalBytes = n }},
	} {
		for _, delta := range []int64{-1, 0, 1} {
			budgets := defaultReplicationBudgets()
			dimension.set(&budgets, dimension.value+delta)
			_, err := measureQuarantinedSelectionUnderBudgets(filepath.Join(publisher, ".git"), empty, replicationMemory, second.Commit, "", budgets, "memory "+first.Envelope.Stream)
			if (delta < 0) != (err != nil) {
				t.Fatalf("%s boundary delta %d: %v", dimension.name, delta, err)
			}
		}
	}
	ref, _ := memoryRef(identity.Actor, first.Envelope.Stream)
	mustGit(t, "push", "-q", "--force", "origin", ref+":"+ref)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	budgets := defaultReplicationBudgets()
	budgets.MaxEvents = 1
	result, err := runReplicationTransaction(ReplicationSelection{
		Version: replicationSelectionVersion, Remote: "origin", Memories: []string{first.Envelope.Stream}, Budgets: budgets,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !replicationOutcomeHasStatus(result.Outcomes, replicationMemory, first.Envelope.Stream, replicationOverBudget) {
		t.Fatalf("two-commit stream was not over max-events=1: %#v (head %s, remote %s)", result.Outcomes, second.Commit, remote)
	}
}

func TestMemoryReplicationValidationRejectsAttemptedAttachment(t *testing.T) {
	inReplicationTestRepository(t)
	identity := deterministicMemoryIdentity()
	stored := appendReplicableMemory(t, identity, defaultMemoryStream(identity.Actor), nil, "no attachments")
	attachment := mustGitTextFromInput(t, []byte("forbidden"), "hash-object", "-w", "--stdin")
	entries := exactTreeEntriesForTest(t, stored.Commit)
	treeInput := "100644 blob " + attachment + "\tattachment.bin\n" +
		"100644 blob " + entries["memory.json"] + "\tmemory.json\n" +
		"100644 blob " + entries["signature"] + "\tsignature\n"
	tree := mustGitTextFromInput(t, []byte(treeInput), "mktree")
	commit := mustGitTextFromInput(t, nil, "commit-tree", tree, "-m", "hostile memory attachment")
	_, err := loadMemoryStreamAt("", memoryStreamSource{
		Ref: mustMemoryRef(t, identity.Actor, stored.Envelope.Stream), Actor: identity.Actor, Stream: stored.Envelope.Stream, Head: commit,
	})
	if err == nil || !strings.Contains(err.Error(), "unexpected entry") {
		t.Fatalf("attempted attachment validation = %v", err)
	}
}

func exactTreeEntriesForTest(t *testing.T, commit string) map[string]string {
	t.Helper()
	entries, err := exactMemoryTreeAt("", commit)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func mustMemoryRef(t *testing.T, actor, stream string) string {
	t.Helper()
	ref, err := memoryRef(actor, stream)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func TestMemoryReplicationTransactionInterruptionPreservesAcceptedRef(t *testing.T) {
	for _, phase := range []string{"after-fetch", "after-measure", "after-copy", "before-promote"} {
		t.Run(phase, func(t *testing.T) {
			_, _, receiver, identity, stored := setupMemoryReplicationFixture(t)
			if err := os.Chdir(receiver); err != nil {
				t.Fatal(err)
			}
			selection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{stored.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
			hook := func() error { return os.ErrClosed }
			switch phase {
			case "after-fetch":
				replicationAfterFetchHook = hook
			case "after-measure":
				replicationAfterMeasureHook = hook
			case "after-copy":
				replicationAfterCopyHook = hook
			case "before-promote":
				replicationBeforePromoteHook = hook
			}
			_, err := runReplicationTransaction(selection)
			replicationAfterFetchHook, replicationAfterMeasureHook, replicationAfterCopyHook, replicationBeforePromoteHook = nil, nil, nil, nil
			if err == nil {
				t.Fatalf("%s interruption succeeded", phase)
			}
			accepted, _ := acceptedMemoryRef("origin", identity.Actor, stored.Envelope.Stream)
			assertRefAbsent(t, accepted)
			if memories, collectErr := collectMemories(); collectErr != nil || len(memories) != 0 {
				t.Fatalf("%s exposed residue: %#v, %v", phase, memories, collectErr)
			}
			result, err := runReplicationTransaction(selection)
			if err != nil || !replicationOutcomeHasStatus(result.Outcomes, replicationMemory, stored.Envelope.Stream, replicationPromoted) {
				t.Fatalf("%s retry outcomes = %#v, err=%v", phase, result.Outcomes, err)
			}
			assertRefValue(t, accepted, stored.Commit)
		})
	}
}

type mixedMemoryTransactionFixture struct {
	selection                       ReplicationSelection
	actorRef, actorOld, actorNew    string
	proposalRef, proposalNew        string
	memoryRef, memoryOld, memoryNew string
	oldEvents, newEvents            []StoredEvent
}

func setupMixedMemoryTransactionFixture(t *testing.T) mixedMemoryTransactionFixture {
	t.Helper()
	root := t.TempDir()
	publisher, remote, receiver := filepath.Join(root, "publisher"), filepath.Join(root, "project.git"), filepath.Join(root, "receiver")
	mustGit(t, "init", "-q", "-b", "main", publisher)
	mustGit(t, "-C", publisher, "config", "user.name", "Publisher")
	mustGit(t, "-C", publisher, "config", "user.email", "publisher@hn.invalid")
	mustGit(t, "-C", publisher, "commit", "--allow-empty", "-q", "-m", "base")
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	base := mustGitText(t, "rev-parse", "HEAD")
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "candidate")
	candidate := mustGitText(t, "rev-parse", "HEAD")
	identity := deterministicMemoryIdentity()
	issue := newEvent(identity, "issue.open", 1, "")
	issue.Title = "old accepted collaboration"
	firstEvent, err := appendEvent(issue, identity)
	if err != nil {
		t.Fatal(err)
	}
	firstMemory := appendReplicableMemory(t, identity, defaultMemoryStream(identity.Actor), nil, "old accepted memory")
	initBareMainRemote(t, remote)
	mustGit(t, "remote", "add", "origin", remote)
	mustGit(t, "push", "-q", "origin", "main:main")
	mustGit(t, "clone", "-q", remote, receiver)
	actorSource, memorySource := actorRef(identity.Actor), mustMemoryRef(t, identity.Actor, firstMemory.Envelope.Stream)
	mustGit(t, "push", "-q", "origin", actorSource+":"+actorSource, memorySource+":"+memorySource)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	oldSelection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Actors: []string{identity.Actor}, Memories: []string{firstMemory.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
	if _, err := runReplicationTransaction(oldSelection); err != nil {
		t.Fatal(err)
	}
	oldEvents, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	proposalEnvelope := newEvent(identity, "proposal.open", 2, firstEvent.ID)
	proposalEnvelope.Title, proposalEnvelope.Base, proposalEnvelope.Head = "new mixed proposal", base, candidate
	proposal, err := appendEvent(proposalEnvelope, identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, candidate); err != nil {
		t.Fatal(err)
	}
	secondMemory := appendReplicableMemory(t, identity, firstMemory.Envelope.Stream, firstMemory, "new mixed memory")
	newEvents, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, "push", "-q", "--force", "origin", actorSource+":"+actorSource, memorySource+":"+memorySource)
	mustGit(t, "push", "-q", "origin", proposalRef(proposal.ID)+":"+proposalRef(proposal.ID))
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	acceptedMemory, _ := acceptedMemoryRef("origin", identity.Actor, firstMemory.Envelope.Stream)
	return mixedMemoryTransactionFixture{
		selection: ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Actors: []string{identity.Actor}, Proposals: []string{proposal.ID}, Memories: []string{firstMemory.Envelope.Stream}, Budgets: defaultReplicationBudgets()},
		actorRef:  acceptedActorRef("origin", identity.Actor), actorOld: firstEvent.Commit, actorNew: proposal.Commit,
		proposalRef: acceptedProposalRef("origin", proposal.ID), proposalNew: candidate,
		memoryRef: acceptedMemory, memoryOld: firstMemory.Commit, memoryNew: secondMemory.Commit,
		oldEvents: oldEvents, newEvents: newEvents,
	}
}

func assertMixedMemoryTransactionState(t *testing.T, fixture mixedMemoryTransactionFixture, advanced bool) {
	t.Helper()
	if advanced {
		assertRefValue(t, fixture.actorRef, fixture.actorNew)
		assertRefValue(t, fixture.proposalRef, fixture.proposalNew)
		assertRefValue(t, fixture.memoryRef, fixture.memoryNew)
	} else {
		assertRefValue(t, fixture.actorRef, fixture.actorOld)
		assertRefAbsent(t, fixture.proposalRef)
		assertRefValue(t, fixture.memoryRef, fixture.memoryOld)
	}
	head, exists, err := refValue(fixture.actorRef)
	if err != nil || !exists {
		t.Fatalf("inspect mixed accepted actor ref: exists=%t err=%v", exists, err)
	}
	got, err := loadActorEventsAt(mustGitText(t, "rev-parse", "--absolute-git-dir"), head)
	want := fixture.oldEvents
	if advanced {
		want = fixture.newEvents
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("mixed collaboration bytes advanced=%t: got=%#v want=%#v err=%v", advanced, got, want, err)
	}
}

func TestMemoryReplicationMixedTransactionAtomicityAtEveryBoundary(t *testing.T) {
	for _, phase := range []string{"after-fetch", "after-measure", "after-copy", "before-promote"} {
		t.Run(phase, func(t *testing.T) {
			fixture := setupMixedMemoryTransactionFixture(t)
			hook := func() error { return os.ErrClosed }
			switch phase {
			case "after-fetch":
				replicationAfterFetchHook = hook
			case "after-measure":
				replicationAfterMeasureHook = hook
			case "after-copy":
				replicationAfterCopyHook = hook
			case "before-promote":
				replicationBeforePromoteHook = hook
			}
			_, err := runReplicationTransaction(fixture.selection)
			replicationAfterFetchHook, replicationAfterMeasureHook, replicationAfterCopyHook, replicationBeforePromoteHook = nil, nil, nil, nil
			if err == nil {
				t.Fatalf("%s mixed interruption succeeded", phase)
			}
			assertMixedMemoryTransactionState(t, fixture, false)
			if _, err := runReplicationTransaction(fixture.selection); err != nil {
				t.Fatal(err)
			}
			assertMixedMemoryTransactionState(t, fixture, true)
		})
	}
	t.Run("validated-receipt", func(t *testing.T) {
		fixture := setupMixedMemoryTransactionFixture(t)
		original := replicationRecordTransaction
		replicationRecordTransaction = func(_ string, _ replicationTransactionResult, state string) error {
			if state == "validated" {
				return os.ErrClosed
			}
			return nil
		}
		_, err := runReplicationTransaction(fixture.selection)
		replicationRecordTransaction = original
		if err == nil {
			t.Fatal("mixed validated receipt failure succeeded")
		}
		assertMixedMemoryTransactionState(t, fixture, false)
	})
	t.Run("ref-transaction", func(t *testing.T) {
		fixture := setupMixedMemoryTransactionFixture(t)
		replicationBeforePromoteHook = func() error {
			mustGit(t, "update-ref", fixture.proposalRef, fixture.actorOld)
			return nil
		}
		_, err := runReplicationTransaction(fixture.selection)
		replicationBeforePromoteHook = nil
		if err == nil || !strings.Contains(err.Error(), "accepted-ref transaction failed") {
			t.Fatalf("mixed ref transaction failure = %v", err)
		}
		assertRefValue(t, fixture.actorRef, fixture.actorOld)
		assertRefValue(t, fixture.memoryRef, fixture.memoryOld)
		assertRefValue(t, fixture.proposalRef, fixture.actorOld)
		head, exists, refErr := refValue(fixture.actorRef)
		got, collectErr := loadActorEventsAt(mustGitText(t, "rev-parse", "--absolute-git-dir"), head)
		if refErr != nil || !exists || collectErr != nil || !reflect.DeepEqual(got, fixture.oldEvents) {
			t.Fatalf("mixed ref failure changed collaboration bytes: %#v, %v", got, collectErr)
		}
	})
	t.Run("completion-receipt", func(t *testing.T) {
		fixture := setupMixedMemoryTransactionFixture(t)
		original := replicationRecordTransaction
		replicationRecordTransaction = func(gitDir string, result replicationTransactionResult, state string) error {
			if state == "complete" {
				return os.ErrClosed
			}
			return original(gitDir, result, state)
		}
		_, err := runReplicationTransaction(fixture.selection)
		replicationRecordTransaction = original
		if err == nil || !strings.Contains(err.Error(), "promotion succeeded") {
			t.Fatalf("mixed completion receipt failure = %v", err)
		}
		assertMixedMemoryTransactionState(t, fixture, true)
	})
}

func TestMemoryReplicationTransactionReceiptFailuresAreTruthful(t *testing.T) {
	t.Run("validated receipt", func(t *testing.T) {
		_, _, receiver, identity, stored := setupMemoryReplicationFixture(t)
		if err := os.Chdir(receiver); err != nil {
			t.Fatal(err)
		}
		selection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{stored.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
		original := replicationRecordTransaction
		replicationRecordTransaction = func(_ string, _ replicationTransactionResult, state string) error {
			if state == "validated" {
				return os.ErrClosed
			}
			return nil
		}
		_, err := runReplicationTransaction(selection)
		replicationRecordTransaction = original
		if err == nil || !strings.Contains(err.Error(), "validated transaction recording") {
			t.Fatalf("validated receipt failure = %v", err)
		}
		accepted, _ := acceptedMemoryRef("origin", identity.Actor, stored.Envelope.Stream)
		assertRefAbsent(t, accepted)
	})
	t.Run("completion receipt", func(t *testing.T) {
		_, _, receiver, identity, stored := setupMemoryReplicationFixture(t)
		if err := os.Chdir(receiver); err != nil {
			t.Fatal(err)
		}
		selection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{stored.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
		original := replicationRecordTransaction
		replicationRecordTransaction = func(gitDir string, result replicationTransactionResult, state string) error {
			if state == "complete" {
				return os.ErrClosed
			}
			return original(gitDir, result, state)
		}
		_, err := runReplicationTransaction(selection)
		replicationRecordTransaction = original
		if err == nil || !strings.Contains(err.Error(), "promotion succeeded") {
			t.Fatalf("completion receipt failure = %v", err)
		}
		accepted, _ := acceptedMemoryRef("origin", identity.Actor, stored.Envelope.Stream)
		assertRefValue(t, accepted, stored.Commit)
		if _, err := runReplicationTransaction(selection); err != nil {
			t.Fatalf("completion retry: %v", err)
		}
		assertRefValue(t, accepted, stored.Commit)
	})
	t.Run("pending anchor process boundary", func(t *testing.T) {
		_, _, receiver, identity, stored := setupMemoryReplicationFixture(t)
		if err := os.Chdir(receiver); err != nil {
			t.Fatal(err)
		}
		selection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{stored.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
		t.Setenv("HN_INTERNAL_TESTING", "1")
		t.Setenv("HN_TEST_REPLICATION_INTERRUPT_AFTER", "after-pending-anchor")
		_, err := runReplicationTransaction(selection)
		if err == nil || !strings.Contains(err.Error(), "pending anchor") {
			t.Fatalf("pending anchor interruption = %v", err)
		}
		accepted, _ := acceptedMemoryRef("origin", identity.Actor, stored.Envelope.Stream)
		assertRefAbsent(t, accepted)
		t.Setenv("HN_TEST_REPLICATION_INTERRUPT_AFTER", "")
		if _, err := runReplicationTransaction(selection); err != nil {
			t.Fatalf("pending-anchor retry: %v", err)
		}
		assertRefValue(t, accepted, stored.Commit)
	})
	t.Run("completion process boundary", func(t *testing.T) {
		_, _, receiver, identity, stored := setupMemoryReplicationFixture(t)
		if err := os.Chdir(receiver); err != nil {
			t.Fatal(err)
		}
		selection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{stored.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
		t.Setenv("HN_INTERNAL_TESTING", "1")
		t.Setenv("HN_TEST_REPLICATION_INTERRUPT_AFTER", "before-completion-receipt")
		_, err := runReplicationTransaction(selection)
		if err == nil || !strings.Contains(err.Error(), "promotion succeeded") {
			t.Fatalf("completion boundary = %v", err)
		}
		accepted, _ := acceptedMemoryRef("origin", identity.Actor, stored.Envelope.Stream)
		assertRefValue(t, accepted, stored.Commit)
		t.Setenv("HN_TEST_REPLICATION_INTERRUPT_AFTER", "")
		if _, err := runReplicationTransaction(selection); err != nil {
			t.Fatalf("completion-boundary retry: %v", err)
		}
	})
	t.Run("atomic ref transaction", func(t *testing.T) {
		_, _, receiver, _, stored := setupMemoryReplicationFixture(t)
		if err := os.Chdir(receiver); err != nil {
			t.Fatal(err)
		}
		selection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{stored.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
		accepted, _ := acceptedMemoryRef("origin", stored.Envelope.Actor, stored.Envelope.Stream)
		competitor := mustGitText(t, "rev-parse", "HEAD")
		replicationBeforePromoteHook = func() error {
			mustGit(t, "update-ref", accepted, competitor)
			return nil
		}
		_, err := runReplicationTransaction(selection)
		replicationBeforePromoteHook = nil
		if err == nil || !strings.Contains(err.Error(), "accepted-ref transaction failed") {
			t.Fatalf("ref transaction failure = %v", err)
		}
		assertRefValue(t, accepted, competitor)
	})
}

func TestMemoryReplicationTransactionDoesNotClearAnotherPendingStream(t *testing.T) {
	publisher, _, receiver, identity, first := setupMemoryReplicationFixture(t)
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	second := appendReplicableMemory(t, identity, fullMemoryID("b"), nil, "independent second stream")
	secondRef := mustMemoryRef(t, identity.Actor, second.Envelope.Stream)
	mustGit(t, "push", "-q", "origin", secondRef+":"+secondRef)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	firstSelection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{first.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
	secondSelection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{second.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
	replicationAfterCopyHook = func() error { return os.ErrClosed }
	_, err := runReplicationTransaction(firstSelection)
	replicationAfterCopyHook = nil
	if err == nil {
		t.Fatal("first transaction did not stop after copy")
	}
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	anchorsBefore, err := filepath.Glob(filepath.Join(gitDir, "hn", "replication", "anchors", "*.json"))
	if err != nil || len(anchorsBefore) != 1 {
		t.Fatalf("pending anchors before independent transaction = %v, %v", anchorsBefore, err)
	}
	if _, err := runReplicationTransaction(secondSelection); err != nil {
		t.Fatal(err)
	}
	anchorsAfter, err := filepath.Glob(filepath.Join(gitDir, "hn", "replication", "anchors", "*.json"))
	if err != nil || len(anchorsAfter) != 1 || anchorsAfter[0] != anchorsBefore[0] {
		t.Fatalf("independent transaction changed pending anchor: before=%v after=%v err=%v", anchorsBefore, anchorsAfter, err)
	}
	firstAccepted, _ := acceptedMemoryRef("origin", identity.Actor, first.Envelope.Stream)
	secondAccepted, _ := acceptedMemoryRef("origin", identity.Actor, second.Envelope.Stream)
	assertRefAbsent(t, firstAccepted)
	assertRefValue(t, secondAccepted, second.Commit)
	memories, err := collectMemories()
	if err != nil || len(memories) != 1 || memories[0].ID != second.ID {
		t.Fatalf("pending residue crossed into accepted projection: %#v, %v", memories, err)
	}
	if _, err := runReplicationTransaction(firstSelection); err != nil {
		t.Fatal(err)
	}
	assertRefValue(t, firstAccepted, first.Commit)
}

func TestMemoryReplicationShallowGapAndRecoveryUseProductionPath(t *testing.T) {
	root := t.TempDir()
	publisher := filepath.Join(root, "publisher")
	remote := filepath.Join(root, "project.git")
	receiver := filepath.Join(root, "receiver")
	mustGit(t, "init", "-q", "-b", "main", publisher)
	mustGit(t, "-C", publisher, "config", "user.name", "Publisher")
	mustGit(t, "-C", publisher, "config", "user.email", "publisher@hn.invalid")
	mustGit(t, "-C", publisher, "commit", "--allow-empty", "-q", "-m", "base")
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	base := mustGitText(t, "rev-parse", "HEAD")
	tree := mustGitText(t, "rev-parse", "HEAD^{tree}")
	anchor := mustGitTextFromInput(t, nil, "commit-tree", tree, "-p", base, "-m", "unadvertised anchor")
	identity := deterministicMemoryIdentity()
	proposalEvent := newEvent(identity, "proposal.open", 1, "")
	proposalEvent.Title = "memory anchor supplier"
	proposalEvent.Base = base
	proposalEvent.Head = anchor
	proposal, err := appendEvent(proposalEvent, identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, anchor); err != nil {
		t.Fatal(err)
	}
	envelope := validMemoryEnvelopeFixture(memoryOperationRecord)
	envelope.Record.Anchor = MemoryAnchor{Commit: anchor}
	envelope.Record.Applicability = Applicability{Mode: memoryApplicabilityExact}
	envelope.Record.Evidence = []string{}
	memory, err := appendMemory(envelope, identity)
	if err != nil {
		t.Fatal(err)
	}
	memoryLocal, _ := memoryRef(identity.Actor, memory.Envelope.Stream)
	initBareMainRemote(t, remote)
	mustGit(t, "remote", "add", "origin", remote)
	mustGit(t, "push", "-q", "origin", "main:main")
	mustGit(t, "push", "-q", "origin", memoryLocal+":"+memoryLocal)
	mustGit(t, "clone", "-q", remote, receiver)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	if err := copyGitObjects(filepath.Join(publisher, ".git"), filepath.Join(receiver, ".git"), []string{proposal.Commit}); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "update-ref", acceptedActorRef("origin", identity.Actor), proposal.Commit)
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	if err := os.WriteFile(filepath.Join(gitDir, "shallow"), []byte(mustGitText(t, "rev-parse", "HEAD")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	selection := ReplicationSelection{
		Version: replicationSelectionVersion, Remote: "origin",
		Proposals: []string{proposal.ID}, Memories: []string{memory.Envelope.Stream}, Budgets: defaultReplicationBudgets(),
	}
	if err := saveReplicationSelection(selection); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(mustSelectionPath(t, "origin"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := runReplicationTransaction(selection)
	if err != nil {
		t.Fatal(err)
	}
	if !replicationOutcomeHasStatus(result.Outcomes, replicationMemory, memory.Envelope.Stream, replicationDependencyMissing) {
		t.Fatalf("initial shallow outcomes = %#v", result.Outcomes)
	}
	gap, err := loadShallowDependencyGap()
	if err != nil {
		t.Fatal(err)
	}
	if gap.OwnerMemoryID != memory.ID || gap.OwnerStream != memory.Envelope.Stream || gap.MissingID != anchor ||
		gap.Kind != shallowMemoryAnchor || gap.Remote != "origin" || !strings.Contains(gap.Recovery, "hn sync origin --recover-shallow") ||
		gap.RequiredRef != proposalRef(proposal.ID) ||
		!reflect.DeepEqual(gap.RequiredSelectors, []string{replicationProposal + ":" + proposal.ID}) {
		t.Fatalf("durable memory gap = %#v", gap)
	}
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "push", "-q", "origin", proposalRef(proposal.ID)+":"+proposalRef(proposal.ID))
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	if err := recoverSelectedShallow("origin"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(mustSelectionPath(t, "origin"))
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("recovery changed selection bytes: err=%v", err)
	}
	accepted, _ := acceptedMemoryRef("origin", identity.Actor, memory.Envelope.Stream)
	assertRefValue(t, accepted, memory.Commit)
	assertRefValue(t, acceptedProposalRef("origin", proposal.ID), anchor)
	if _, err := loadShallowDependencyGap(); !os.IsNotExist(err) {
		t.Fatalf("successful recovery retained gap: %v", err)
	}
}

func TestMemoryReplicationTypedShallowSupplierRecovery(t *testing.T) {
	for _, dependency := range []string{"lifecycle-target", "evidence-event", "evidence-memory"} {
		t.Run(dependency, func(t *testing.T) {
			root := t.TempDir()
			publisher, remote, receiver := filepath.Join(root, "publisher"), filepath.Join(root, "project.git"), filepath.Join(root, "receiver")
			mustGit(t, "init", "-q", "-b", "main", publisher)
			mustGit(t, "-C", publisher, "config", "user.name", "Publisher")
			mustGit(t, "-C", publisher, "config", "user.email", "publisher@hn.invalid")
			mustGit(t, "-C", publisher, "commit", "--allow-empty", "-q", "-m", "base")
			original, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chdir(original) })
			if err := os.Chdir(publisher); err != nil {
				t.Fatal(err)
			}
			ownerIdentity := deterministicMemoryIdentity()
			supplierIdentity := ownerIdentity
			ownerStream, supplierStream := fullMemoryID("c"), fullMemoryID("d")
			var supplierRef, supplierSelector, supplierCommit, missingID string
			ownerEnvelope := validMemoryEnvelopeFixture(memoryOperationRecord)
			ownerEnvelope.Stream = ownerStream
			ownerEnvelope.Record.Anchor = MemoryAnchor{Commit: mustGitText(t, "rev-parse", "HEAD")}
			ownerEnvelope.Record.Applicability = Applicability{Mode: memoryApplicabilityExact}
			ownerEnvelope.Record.Evidence = []string{}
			if dependency == "evidence-event" {
				supplierIdentity = testIdentity(t, "Typed Evidence Supplier")
				event := newEvent(supplierIdentity, "issue.open", 1, "")
				event.Title = "typed evidence supplier"
				stored, err := appendEvent(event, supplierIdentity)
				if err != nil {
					t.Fatal(err)
				}
				missingID, supplierCommit = stored.ID, stored.Commit
				supplierRef, supplierSelector = actorRef(supplierIdentity.Actor), replicationActor+":"+supplierIdentity.Actor
				ownerEnvelope.Record.Evidence = []string{"event:" + missingID}
			} else {
				supplier := appendReplicableMemory(t, supplierIdentity, supplierStream, nil, "typed supplier")
				missingID, supplierCommit = supplier.ID, supplier.Commit
				supplierRef, supplierSelector = mustMemoryRef(t, supplierIdentity.Actor, supplierStream), replicationMemory+":"+supplierStream
				if dependency == "lifecycle-target" {
					ownerEnvelope.Operation = memoryOperationSupersede
					ownerEnvelope.Target = missingID
				} else {
					ownerEnvelope.Record.Evidence = []string{"memory:" + missingID}
				}
			}
			owner, err := appendMemory(ownerEnvelope, ownerIdentity)
			if err != nil {
				t.Fatal(err)
			}
			ownerRef := mustMemoryRef(t, ownerIdentity.Actor, ownerStream)
			initBareMainRemote(t, remote)
			mustGit(t, "remote", "add", "origin", remote)
			mustGit(t, "push", "-q", "origin", "main:main")
			mustGit(t, "clone", "-q", remote, receiver)
			mustGit(t, "push", "-q", "origin", ownerRef+":"+ownerRef, supplierRef+":"+supplierRef)
			if err := os.Chdir(receiver); err != nil {
				t.Fatal(err)
			}
			if err := copyGitObjects(filepath.Join(publisher, ".git"), filepath.Join(receiver, ".git"), []string{supplierCommit}); err != nil {
				t.Fatal(err)
			}
			gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
			if err := os.WriteFile(filepath.Join(gitDir, "shallow"), []byte(mustGitText(t, "rev-parse", "HEAD")+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			selection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{ownerStream}, Budgets: defaultReplicationBudgets()}
			if err := saveReplicationSelection(selection); err != nil {
				t.Fatal(err)
			}
			result, err := runReplicationTransaction(selection)
			if err != nil || !replicationOutcomeHasStatus(result.Outcomes, replicationMemory, ownerStream, replicationDependencyMissing) {
				t.Fatalf("typed gap outcomes = %#v, err=%v", result.Outcomes, err)
			}
			gap, err := loadShallowDependencyGap()
			if err != nil {
				t.Fatal(err)
			}
			if gap.MissingID != missingID || gap.RequiredRef != supplierRef || !reflect.DeepEqual(gap.RequiredSelectors, []string{supplierSelector}) || !strings.Contains(gap.Recovery, strings.Replace(supplierSelector, ":", " ", 1)) {
				t.Fatalf("typed supplier gap = %#v", gap)
			}
			if dependency == "evidence-event" {
				selection.Actors = []string{supplierIdentity.Actor}
			} else {
				selection.Memories = append(selection.Memories, supplierStream)
			}
			if err := saveReplicationSelection(selection); err != nil {
				t.Fatal(err)
			}
			if err := recoverSelectedShallow("origin"); err != nil {
				t.Fatal(err)
			}
			acceptedOwner, _ := acceptedMemoryRef("origin", ownerIdentity.Actor, ownerStream)
			assertRefValue(t, acceptedOwner, owner.Commit)
			if _, err := loadShallowDependencyGap(); !os.IsNotExist(err) {
				t.Fatalf("successful typed recovery retained gap: %v", err)
			}
		})
	}
}

func TestMemoryReplicationGitEvidenceShallowSupplierRecovery(t *testing.T) {
	root := t.TempDir()
	publisher, remote, receiver := filepath.Join(root, "publisher"), filepath.Join(root, "project.git"), filepath.Join(root, "receiver")
	mustGit(t, "init", "-q", "-b", "main", publisher)
	mustGit(t, "-C", publisher, "config", "user.name", "Publisher")
	mustGit(t, "-C", publisher, "config", "user.email", "publisher@hn.invalid")
	mustGit(t, "-C", publisher, "commit", "--allow-empty", "-q", "-m", "base")
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	base, tree := mustGitText(t, "rev-parse", "HEAD"), mustGitText(t, "rev-parse", "HEAD^{tree}")
	evidenceCommit := mustGitTextFromInput(t, nil, "commit-tree", tree, "-p", base, "-m", "unadvertised evidence")
	identity := deterministicMemoryIdentity()
	proposalEnvelope := newEvent(identity, "proposal.open", 1, "")
	proposalEnvelope.Title, proposalEnvelope.Base, proposalEnvelope.Head = "git evidence supplier", base, evidenceCommit
	proposal, err := appendEvent(proposalEnvelope, identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := createProposalRef(proposal.ID, evidenceCommit); err != nil {
		t.Fatal(err)
	}
	envelope := validMemoryEnvelopeFixture(memoryOperationRecord)
	envelope.Record.Anchor = MemoryAnchor{Commit: base}
	envelope.Record.Applicability = Applicability{Mode: memoryApplicabilityExact}
	envelope.Record.Evidence = []string{"git:" + evidenceCommit}
	memory, err := appendMemory(envelope, identity)
	if err != nil {
		t.Fatal(err)
	}
	memorySource := mustMemoryRef(t, identity.Actor, memory.Envelope.Stream)
	initBareMainRemote(t, remote)
	mustGit(t, "remote", "add", "origin", remote)
	mustGit(t, "push", "-q", "origin", "main:main", memorySource+":"+memorySource)
	mustGit(t, "clone", "-q", remote, receiver)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	if err := copyGitObjects(filepath.Join(publisher, ".git"), filepath.Join(receiver, ".git"), []string{proposal.Commit}); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "update-ref", acceptedActorRef("origin", identity.Actor), proposal.Commit)
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	if err := os.WriteFile(filepath.Join(gitDir, "shallow"), []byte(base+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	selection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Proposals: []string{proposal.ID}, Memories: []string{memory.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
	if err := saveReplicationSelection(selection); err != nil {
		t.Fatal(err)
	}
	result, err := runReplicationTransaction(selection)
	if err != nil || !replicationOutcomeHasStatus(result.Outcomes, replicationMemory, memory.Envelope.Stream, replicationDependencyMissing) {
		t.Fatalf("git evidence outcomes = %#v, err=%v", result.Outcomes, err)
	}
	gap, err := loadShallowDependencyGap()
	if err != nil {
		t.Fatal(err)
	}
	if gap.Kind != shallowMemoryEvidence || gap.MissingID != evidenceCommit || gap.RequiredRef != proposalRef(proposal.ID) || !reflect.DeepEqual(gap.RequiredSelectors, []string{replicationProposal + ":" + proposal.ID}) {
		t.Fatalf("git evidence gap = %#v", gap)
	}
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "push", "-q", "origin", proposalRef(proposal.ID)+":"+proposalRef(proposal.ID))
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	if err := recoverSelectedShallow("origin"); err != nil {
		t.Fatal(err)
	}
	accepted, _ := acceptedMemoryRef("origin", identity.Actor, memory.Envelope.Stream)
	assertRefValue(t, accepted, memory.Commit)
}

func TestMemoryReplicationShallowWrongTypeIsNotRecoverableGap(t *testing.T) {
	publisher, _, receiver, identity, _ := setupMemoryReplicationFixture(t)
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	blobPayload := []byte("wrong type anchor")
	blob := mustGitTextFromInput(t, blobPayload, "hash-object", "-w", "--stdin")
	envelope := validMemoryEnvelopeFixture(memoryOperationRecord)
	envelope.Stream = fullMemoryID("a")
	envelope.Record.Anchor = MemoryAnchor{Commit: blob}
	envelope.Record.Applicability = Applicability{Mode: memoryApplicabilityExact}
	envelope.Record.Evidence = []string{}
	stored, err := appendMemory(envelope, identity)
	if err != nil {
		t.Fatal(err)
	}
	ref := mustMemoryRef(t, identity.Actor, stored.Envelope.Stream)
	mustGit(t, "push", "-q", "origin", ref+":"+ref)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	if got := mustGitTextFromInput(t, blobPayload, "hash-object", "-w", "--stdin"); got != blob {
		t.Fatalf("blob identity changed: %s", got)
	}
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	if err := os.WriteFile(filepath.Join(gitDir, "shallow"), []byte(mustGitText(t, "rev-parse", "HEAD")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := runReplicationTransaction(ReplicationSelection{
		Version: replicationSelectionVersion, Remote: "origin", Memories: []string{stored.Envelope.Stream}, Budgets: defaultReplicationBudgets(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !replicationOutcomeHasStatus(result.Outcomes, replicationMemory, stored.Envelope.Stream, replicationRelationshipBad) {
		t.Fatalf("wrong-type anchor outcomes = %#v", result.Outcomes)
	}
	if _, err := loadShallowDependencyGap(); !os.IsNotExist(err) {
		t.Fatalf("wrong-type anchor became a shallow gap: %v", err)
	}
}

func mustSelectionPath(t *testing.T, remote string) string {
	t.Helper()
	path, err := replicationSelectionPath(remote)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMemoryReplicationFreshCloneConvergesWithoutPrivateState(t *testing.T) {
	publisher, remote, receiver, identity, stored := setupMemoryReplicationFixture(t)
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	supersede := nextMemoryEnvelope(t, identity, stored.Envelope.Stream, stored, "superseded content")
	supersede.Operation = memoryOperationSupersede
	supersede.Target = stored.ID
	supersede.Record.Anchor = MemoryAnchor{Commit: mustGitText(t, "rev-parse", "HEAD")}
	supersede.Record.Applicability = Applicability{Mode: memoryApplicabilityExact}
	supersede.Record.Evidence = []string{}
	successor, err := appendMemory(supersede, identity)
	if err != nil {
		t.Fatal(err)
	}
	verificationEnvelope := validMemoryEnvelopeFixture(memoryOperationRecord)
	verificationEnvelope.Stream = fullMemoryID("8")
	verificationEnvelope.Record.Kind = memoryKindVerification
	verificationEnvelope.Record.Anchor = MemoryAnchor{Commit: mustGitText(t, "rev-parse", "HEAD")}
	verificationEnvelope.Record.Applicability = Applicability{Mode: memoryApplicabilityExact}
	verificationEnvelope.Record.Evidence = []string{"memory:" + stored.ID, "git:" + mustGitText(t, "rev-parse", "HEAD")}
	verification, err := appendMemory(verificationEnvelope, identity)
	if err != nil {
		t.Fatal(err)
	}
	collaborator := testIdentity(t, "Clone Collaborator")
	issue := newEvent(collaborator, "issue.open", 1, "")
	issue.Title = "collaboration-only compatibility"
	if _, err := appendEvent(issue, collaborator); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{mustMemoryRef(t, identity.Actor, stored.Envelope.Stream), mustMemoryRef(t, identity.Actor, verification.Envelope.Stream), actorRef(collaborator.Actor)} {
		mustGit(t, "push", "-q", "--force", "origin", ref+":"+ref)
	}
	publisherMemories, err := collectMemories()
	if err != nil {
		t.Fatal(err)
	}
	atCommit := mustGitText(t, "rev-parse", "HEAD")
	publisherProjection := ProjectMemories(publisherMemories, MemoryProjectionContext{AtCommit: atCommit})
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "no-global-config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if err := cmdReplication([]string{"select", "origin", "--memory", stored.Envelope.Stream, "--memory", verification.Envelope.Stream}); err != nil {
		t.Fatal(err)
	}
	if output, err := captureTestOutput(t, func() error { return cmdSync(nil) }); err != nil {
		t.Fatalf("fresh-clone sync failed: %v\n%s", err, output)
	}
	accepted, _ := acceptedMemoryRef("origin", identity.Actor, stored.Envelope.Stream)
	assertRefValue(t, accepted, successor.Commit)
	memories, err := collectMemories()
	if err != nil || len(memories) != 3 {
		t.Fatalf("fresh clone memories = %#v, err=%v", memories, err)
	}
	cloneProjection := ProjectMemories(memories, MemoryProjectionContext{AtCommit: atCommit})
	if !reflect.DeepEqual(cloneProjection, publisherProjection) {
		t.Fatalf("fresh clone projection diverged:\npublisher=%#v\nclone=%#v", publisherProjection, cloneProjection)
	}
	if len(cloneProjection.Relationships) != 1 || cloneProjection.Relationships[0].Operation != memoryOperationSupersede ||
		len(cloneProjection.Rows) != 3 || cloneProjection.Rows[0].Applicability == "" {
		t.Fatalf("fresh clone omitted lifecycle/applicability/evidence: %#v", cloneProjection)
	}
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	for _, private := range []string{
		filepath.Join(gitDir, "hn", "identity.json"),
		filepath.Join(gitDir, "hn", "identities"),
		filepath.Join(gitDir, "hn", "memory", "index-v0.json"),
	} {
		if _, err := os.Stat(private); !os.IsNotExist(err) {
			t.Fatalf("fresh-clone sync created private state %s: %v", filepath.Base(private), err)
		}
	}
	collaborationOnly := filepath.Join(t.TempDir(), "collaboration-only")
	mustGit(t, "clone", "-q", remote, collaborationOnly)
	if err := os.Chdir(collaborationOnly); err != nil {
		t.Fatal(err)
	}
	result, err := runReplicationTransaction(ReplicationSelection{
		Version: replicationSelectionVersion, Remote: "origin", Actors: []string{collaborator.Actor}, Budgets: defaultReplicationBudgets(),
	})
	if err != nil || result.hasFailures() {
		t.Fatalf("collaboration-only sync = %#v, err=%v", result.Outcomes, err)
	}
	if refs := mustGitText(t, "for-each-ref", "--format=%(refname)", "refs/hn/remotes/origin/memory"); refs != "" {
		t.Fatalf("collaboration-only selection imported memory refs: %s", refs)
	}
}

func appendReplicableMemory(t *testing.T, identity *Identity, stream string, previous *StoredMemory, content string) *StoredMemory {
	t.Helper()
	envelope := validMemoryEnvelopeFixture(memoryOperationRecord)
	envelope.Actor = identity.Actor
	envelope.ActorName = identity.Name
	envelope.PublicKey = identity.PublicKey
	envelope.Stream = stream
	envelope.Record.Anchor = MemoryAnchor{Commit: mustGitText(t, "rev-parse", "HEAD")}
	envelope.Record.Applicability = Applicability{Mode: memoryApplicabilityExact}
	envelope.Record.Evidence = []string{}
	envelope.Record.Content = content
	if previous != nil {
		envelope.Sequence = previous.Envelope.Sequence + 1
		envelope.Previous = previous.ID
		envelope.Timestamp = "2026-08-30T12:35:00Z"
	}
	stored, err := appendMemory(envelope, identity)
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func setupMemoryReplicationFixture(t *testing.T) (publisher, remote, receiver string, identity *Identity, stored *StoredMemory) {
	t.Helper()
	root := t.TempDir()
	publisher = filepath.Join(root, "publisher")
	remote = filepath.Join(root, "project.git")
	receiver = filepath.Join(root, "receiver")
	mustGit(t, "init", "-q", "-b", "main", publisher)
	mustGit(t, "-C", publisher, "config", "user.name", "Publisher")
	mustGit(t, "-C", publisher, "config", "user.email", "publisher@hn.invalid")
	mustGit(t, "-C", publisher, "commit", "--allow-empty", "-q", "-m", "base")
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	identity = deterministicMemoryIdentity()
	stored = appendReplicableMemory(t, identity, defaultMemoryStream(identity.Actor), nil, "replicable")
	initBareMainRemote(t, remote)
	mustGit(t, "remote", "add", "origin", remote)
	mustGit(t, "push", "-q", "origin", "main:main")
	mustGit(t, "clone", "-q", remote, receiver)
	ref, _ := memoryRef(identity.Actor, stored.Envelope.Stream)
	mustGit(t, "push", "-q", "origin", ref+":"+ref)
	return publisher, remote, receiver, identity, stored
}

func testAdvertisedRemote(t *testing.T, refs map[string]string) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "advertised.git")
	initBareMainRemote(t, remote)
	for ref, oid := range refs {
		mustGit(t, "push", "-q", remote, oid+":"+ref)
	}
	return remote
}

func initBareMainRemote(t *testing.T, remote string) {
	t.Helper()
	mustGit(t, "init", "--bare", "-q", remote)
	mustGit(t, "--git-dir", remote, "symbolic-ref", "HEAD", "refs/heads/main")
}
