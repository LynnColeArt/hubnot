package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// replicationReceipts reads every transaction receipt in the current clone.
func replicationReceipts(t *testing.T) map[string]replicationTransactionRecord {
	t.Helper()
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	paths, err := filepath.Glob(filepath.Join(gitDir, "hn", "replication", "transactions", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	receipts := map[string]replicationTransactionRecord{}
	for _, path := range paths {
		encoded, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var record replicationTransactionRecord
		if err := json.Unmarshal(encoded, &record); err != nil {
			t.Fatal(err)
		}
		receipts[record.ID] = record
	}
	return receipts
}

func newReceipt(t *testing.T, before map[string]replicationTransactionRecord) replicationTransactionRecord {
	t.Helper()
	var found []replicationTransactionRecord
	for id, record := range replicationReceipts(t) {
		if _, old := before[id]; !old {
			found = append(found, record)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected one new receipt, found %d", len(found))
	}
	return found[0]
}

func reachableFrom(t *testing.T, commit string) []string {
	t.Helper()
	objects, err := reachableObjectIDsAt(mustGitText(t, "rev-parse", "--absolute-git-dir"), []string{commit})
	if err != nil {
		t.Fatal(err)
	}
	return objects
}

// A transaction that promotes objects the clone already accepted records only
// what it newly accepts, while the clone's acceptance stays the same.
func TestReplicationReceiptRecordsOnlyNewlyAcceptedObjects(t *testing.T) {
	publisher, _, receiver, identity, first := setupMemoryReplicationFixture(t)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	firstSelection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{first.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
	before := replicationReceipts(t)
	if _, err := runReplicationTransaction(firstSelection); err != nil {
		t.Fatal(err)
	}
	firstReceipt := newReceipt(t, before)
	firstObjects := reachableFrom(t, first.Commit)
	if !slices.Equal(*firstReceipt.AcceptedObjects, sortedUniqueStrings(firstObjects...)) {
		t.Fatalf("first receipt accepted %d objects, want the %d new ones", len(*firstReceipt.AcceptedObjects), len(firstObjects))
	}

	if err := os.Chdir(publisher); err != nil {
		t.Fatal(err)
	}
	second := appendReplicableMemory(t, identity, fullMemoryID("b"), nil, "independent second stream")
	secondRef := mustMemoryRef(t, identity.Actor, second.Envelope.Stream)
	mustGit(t, "push", "-q", "origin", secondRef+":"+secondRef)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	both := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{first.Envelope.Stream, second.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
	before = replicationReceipts(t)
	if _, err := runReplicationTransaction(both); err != nil {
		t.Fatal(err)
	}
	receipt := newReceipt(t, before)
	accepted := map[string]bool{}
	for _, id := range *receipt.AcceptedObjects {
		accepted[id] = true
	}
	for _, id := range *receipt.PendingObjects {
		if !accepted[id] {
			t.Fatalf("receipt does not accept its own pending object %s", id)
		}
	}
	held := map[string]bool{}
	for _, id := range firstObjects {
		held[id] = true
	}
	for _, id := range *receipt.AcceptedObjects {
		if held[id] {
			t.Fatalf("receipt accepts %s again, which the first transaction already accepted", id)
		}
	}
	if len(*receipt.AcceptedObjects) == 0 {
		t.Fatal("receipt accepts none of the second stream's objects")
	}
	denied, pending, err := loadReplicationAcceptanceState(mustGitText(t, "rev-parse", "--absolute-git-dir"))
	if err != nil || len(denied) != 0 || len(pending) != 0 {
		t.Fatalf("acceptance after both transactions: denied=%d pending=%d err=%v", len(denied), len(pending), err)
	}
	firstAccepted, _ := acceptedMemoryRef("origin", identity.Actor, first.Envelope.Stream)
	secondAccepted, _ := acceptedMemoryRef("origin", identity.Actor, second.Envelope.Stream)
	assertRefValue(t, firstAccepted, first.Commit)
	assertRefValue(t, secondAccepted, second.Commit)
}

// Objects that an interrupted transaction left denied are accepted by a later
// transaction that requires them, which also completes the interrupted one.
func TestReplicationReceiptAcceptsObjectsAnotherTransactionLeftDenied(t *testing.T) {
	_, _, receiver, identity, first := setupMemoryReplicationFixture(t)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	selection := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{first.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
	replicationAfterCopyHook = func() error { return os.ErrClosed }
	_, err := runReplicationTransaction(selection)
	replicationAfterCopyHook = nil
	if err == nil {
		t.Fatal("first transaction did not stop after copy")
	}
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	denied, _, err := loadReplicationAcceptanceState(gitDir)
	if err != nil || len(denied) == 0 {
		t.Fatalf("interrupted transaction left nothing denied: %d, %v", len(denied), err)
	}
	before := replicationReceipts(t)
	if _, err := runReplicationTransaction(selection); err != nil {
		t.Fatal(err)
	}
	receipt := newReceipt(t, before)
	accepted := map[string]bool{}
	for _, id := range *receipt.AcceptedObjects {
		accepted[id] = true
	}
	for id := range denied {
		if !accepted[id] {
			t.Fatalf("receipt leaves %s denied", id)
		}
	}
	after, pending, err := loadReplicationAcceptanceState(gitDir)
	if err != nil || len(after) != 0 || len(pending) != 0 {
		t.Fatalf("acceptance after recovery: denied=%d pending=%d err=%v", len(after), len(pending), err)
	}
	anchors, err := filepath.Glob(filepath.Join(gitDir, "hn", "replication", "anchors", "*.json"))
	if err != nil || len(anchors) != 0 {
		t.Fatalf("interrupted transaction's anchor remains: %v, %v", anchors, err)
	}
	firstAccepted, _ := acceptedMemoryRef("origin", identity.Actor, first.Envelope.Stream)
	assertRefValue(t, firstAccepted, first.Commit)
}

func deniedAmong(t *testing.T, objects []string) []string {
	t.Helper()
	denied, _, err := loadReplicationAcceptanceState(mustGitText(t, "rev-parse", "--absolute-git-dir"))
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, id := range objects {
		if denied[id] {
			found = append(found, id)
		}
	}
	return found
}

// A complete receipt whose anchor survived a crash can still be rewritten
// as validated by recovery, so a later transaction through another remote
// must not leave out the objects that receipt accepts.
func TestReplicationReceiptDoesNotRelyOnAnAnchoredReceipt(t *testing.T) {
	_, remote, receiver, _, first := setupMemoryReplicationFixture(t)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "remote", "add", "mirror", remote)
	stream := []string{first.Envelope.Stream}
	origin := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: stream, Budgets: defaultReplicationBudgets()}
	mirror := ReplicationSelection{Version: replicationSelectionVersion, Remote: "mirror", Memories: stream, Budgets: defaultReplicationBudgets()}

	// The complete receipt is written, then the process stops before it
	// removes the pending anchor.
	replicationRecordTransaction = func(gitDir string, result replicationTransactionResult, state string) error {
		if err := recordReplicationTransaction(gitDir, result, state); err != nil {
			return err
		}
		if state == "complete" {
			return os.ErrClosed
		}
		return nil
	}
	_, err := runReplicationTransaction(origin)
	replicationRecordTransaction = recordReplicationTransaction
	if err == nil {
		t.Fatal("origin transaction did not stop after its complete receipt")
	}
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	if anchors, _ := filepath.Glob(filepath.Join(gitDir, "hn", "replication", "anchors", "*.json")); len(anchors) != 1 {
		t.Fatalf("expected the origin anchor to survive, found %d", len(anchors))
	}
	if _, err := runReplicationTransaction(mirror); err != nil {
		t.Fatal(err)
	}
	objects := reachableFrom(t, first.Commit)
	if denied := deniedAmong(t, objects); len(denied) != 0 {
		t.Fatalf("mirror import left %d objects denied", len(denied))
	}
	// Recovery rewrites the anchored origin receipt as validated, and the
	// recovering transaction stops before it completes.
	replicationAfterCopyHook = func() error { return os.ErrClosed }
	_, err = runReplicationTransaction(origin)
	replicationAfterCopyHook = nil
	if err == nil {
		t.Fatal("recovering origin transaction did not stop")
	}
	if receipts := replicationReceipts(t); len(receipts) == 0 {
		t.Fatal("no receipts")
	}
	if denied := deniedAmong(t, objects); len(denied) != 0 {
		t.Fatalf("the stream imported through mirror became denied: %d objects", len(denied))
	}
}

// A transaction that validated another transaction's whole pending set
// completes it, even when its own receipt records only part of that set.
func TestReplicationReconciliationUsesTheWholeValidatedClosure(t *testing.T) {
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
	firstOnly := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{first.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
	both := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: []string{first.Envelope.Stream, second.Envelope.Stream}, Budgets: defaultReplicationBudgets()}
	replicationAfterCopyHook = func() error { return os.ErrClosed }
	_, err := runReplicationTransaction(both)
	replicationAfterCopyHook = nil
	if err == nil {
		t.Fatal("first transaction did not stop after copy")
	}
	if _, err := runReplicationTransaction(firstOnly); err != nil {
		t.Fatal(err)
	}
	if _, err := runReplicationTransaction(both); err != nil {
		t.Fatal(err)
	}
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	anchors, err := filepath.Glob(filepath.Join(gitDir, "hn", "replication", "anchors", "*.json"))
	if err != nil || len(anchors) != 0 {
		t.Fatalf("interrupted transaction was not reconciled: anchors %v, %v", anchors, err)
	}
	denied, pending, err := loadReplicationAcceptanceState(gitDir)
	if err != nil || len(denied) != 0 || len(pending) != 0 {
		t.Fatalf("after reconciliation: denied=%d pending=%d err=%v", len(denied), len(pending), err)
	}
}

// A new transaction never takes the ID of an existing receipt: replacing a
// settled receipt would drop acceptance that later receipts leave out.
func TestReplicationTransactionNeverReusesAReceiptID(t *testing.T) {
	_, remote, receiver, _, first := setupMemoryReplicationFixture(t)
	if err := os.Chdir(receiver); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "remote", "add", "mirror", remote)
	stream := []string{first.Envelope.Stream}
	origin := ReplicationSelection{Version: replicationSelectionVersion, Remote: "origin", Memories: stream, Budgets: defaultReplicationBudgets()}
	mirror := ReplicationSelection{Version: replicationSelectionVersion, Remote: "mirror", Memories: stream, Budgets: defaultReplicationBudgets()}
	// Leave the stream's objects denied by an interrupted origin import, then
	// accept them through mirror in a settled receipt.
	replicationAfterCopyHook = func() error { return os.ErrClosed }
	_, err := runReplicationTransaction(origin)
	replicationAfterCopyHook = nil
	if err == nil {
		t.Fatal("origin transaction did not stop after copy")
	}
	before := replicationReceipts(t)
	if _, err := runReplicationTransaction(mirror); err != nil {
		t.Fatal(err)
	}
	settled := newReceipt(t, before)
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	receiptPath := filepath.Join(gitDir, "hn", "replication", "transactions", settled.ID+".json")
	original, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	// The next allocation first offers the settled receipt's ID.
	offered := 0
	replicationMkdirTemp = func(dir, pattern string) (string, error) {
		offered++
		if offered == 1 {
			path := filepath.Join(dir, settled.ID)
			return path, os.Mkdir(path, 0o700)
		}
		return os.MkdirTemp(dir, pattern)
	}
	defer func() { replicationMkdirTemp = os.MkdirTemp }()
	result, err := runReplicationTransaction(mirror)
	if err != nil {
		t.Fatal(err)
	}
	if offered < 2 || result.ID == settled.ID {
		t.Fatalf("transaction took the settled receipt's ID %s (offers %d)", settled.ID, offered)
	}
	if after, err := os.ReadFile(receiptPath); err != nil || !bytes.Equal(after, original) {
		t.Fatalf("settled receipt changed: %v", err)
	}
	if denied := deniedAmong(t, reachableFrom(t, first.Commit)); len(denied) != 0 {
		t.Fatalf("%d objects denied after the second mirror import", len(denied))
	}
}
