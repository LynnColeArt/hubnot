package main

import (
	"errors"
	"testing"
)

func TestBatchEventReadsRefreshAcceptanceBetweenOperations(t *testing.T) {
	gitDir := setupOperationalLedgerRepository(t)
	events, err := collectEvents()
	if err != nil || len(events) != 1 {
		t.Fatalf("initial read: %d, %v", len(events), err)
	}
	if _, err := collectAcceptedEventsForReplication(ReplicationSelection{}); err != nil {
		t.Fatal(err)
	}
	// A later transaction must not inherit an earlier batch's acceptance view.
	recordOperationalTransactionFixture(t, gitDir, "txn-later-pending", "validated", []string{events[0].Commit}, nil)
	for name, read := range map[string]func() ([]StoredEvent, error){
		"ordinary facts":    collectEvents,
		"replication facts": func() ([]StoredEvent, error) { return collectAcceptedEventsForReplication(ReplicationSelection{}) },
	} {
		if _, err := read(); err == nil {
			t.Fatalf("%s reused stale acceptance", name)
		} else {
			var pending *ReplicationAcceptancePendingError
			if !errors.As(err, &pending) || pending.ObjectID != events[0].Commit {
				t.Fatalf("%s: wrong pending diagnostic: %v", name, err)
			}
		}
	}
}

func TestReplicationBatchDeduplicatesSharedVerifiedActorHistory(t *testing.T) {
	setupOperationalLedgerRepository(t)
	before, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	actor, head := before[0].Event.Actor, before[0].Commit
	for _, remote := range []string{"one", "two", "three"} {
		mustGit(t, "update-ref", acceptedActorRef(remote, actor), head)
	}
	after, err := collectAcceptedEventsForReplication(ReplicationSelection{})
	if err != nil || len(after) != 1 || after[0].ID != before[0].ID {
		t.Fatalf("shared actor history: %+v, %v", after, err)
	}
}
