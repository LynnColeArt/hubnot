package main

import (
 "encoding/hex"
 "fmt"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestIndependentAuditMalformedTargetCommit(t *testing.T) {
 _, id, at := auditFixture(t)
 auditAuthorizeFixture(t, id, at)
 tree := mustGitText(t, "rev-parse", at+"^{tree}")
 raw := fmt.Sprintf("tree %s\ngarbage header\n\nmessage\n", tree)
 oidRaw, err := gitInput([]byte(raw), nil, "hash-object", "--literally", "-w", "-t", "commit", "--stdin")
 if err != nil { t.Fatal(err) }
 target := strings.TrimSpace(string(oidRaw))
 if _, err := AuditRequest(target, "check", "", id); err == nil { t.Fatal("malformed commit without author/committer accepted by AuditRequest") } else { t.Log(err) }
}

func TestIndependentAuditMalformedSnapshotReplication(t *testing.T) {
 for _, kind := range []string{"missing-identities", "tree-as-blob"} {
 t.Run(kind, func(t *testing.T) {
 publisher, id, at := auditFixture(t)
 auditAuthorizeFixture(t, id, at)
 tree := mustGitText(t, "rev-parse", at+"^{tree}")
 raw := fmt.Sprintf("tree %s\ngarbage header\n\nmessage\n", tree)
 if kind == "tree-as-blob" {
  treeID, err := hex.DecodeString(tree)
  if err != nil { t.Fatal(err) }
  treeRaw := append([]byte("100644 wrong-type\x00"), treeID...)
  invalidTree, err := gitInput(treeRaw, nil, "hash-object", "--literally", "-w", "-t", "tree", "--stdin")
  if err != nil { t.Fatal(err) }
  raw = fmt.Sprintf("tree %s\nauthor Test <test@hn.invalid> 1000000000 +0000\ncommitter Test <test@hn.invalid> 1000000000 +0000\n\nmessage\n", strings.TrimSpace(string(invalidTree)))
 }
 oidRaw, err := gitInput([]byte(raw), nil, "hash-object", "--literally", "-w", "-t", "commit", "--stdin")
 if err != nil { t.Fatal(err) }
 target := strings.TrimSpace(string(oidRaw))
 req, err := AuditRequest(target, "check", "", id)
 if err != nil { t.Logf("local correctly refused %s: %v",kind,err); return }
 t.Logf("local accepted malformed %s target %s request %s",kind,target,req.ID)
 remote := filepath.Join(t.TempDir(), "project.git")
 receiver := filepath.Join(t.TempDir(), "reader")
 mustGit(t, "clone", "-q", "--bare", publisher, remote)
 mustGit(t, "remote", "add", "origin", remote)
 mustGit(t, "push", "-q", "origin", actorRef(id.Actor)+":"+actorRef(id.Actor))
 mustGit(t, "clone", "-q", "--no-local", remote, receiver)
 if err := os.Chdir(receiver); err != nil { t.Fatal(err) }
 selection := ReplicationSelection{Version: replicationSelectionVersion, Remote:"origin", Actors:[]string{id.Actor}, Budgets:defaultReplicationBudgets()}
 got, err := runReplicationTransaction(selection)
 if err != nil { t.Fatal(err) }
 events, err := collectEvents()
 if err != nil { t.Fatal(err) }
 for _, outcome := range got.Outcomes { t.Logf("replication kind=%s status=%s diagnostic=%q accepted event count=%d",outcome.Kind,outcome.Status,outcome.Diagnostic,len(events)) }
 if _, err := gitOutput("fsck", "--strict", "--no-reflogs", target); err != nil { t.Logf("Git rejects admitted snapshot structure: %v",err) }
 if replicationOutcomeHasStatus(got.Outcomes,replicationActor,id.Actor,replicationPromoted) {
  t.Errorf("malformed %s target promoted to accepted visibility",kind)
 }
 })
 }
}
