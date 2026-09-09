package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestIssueReadEmptyAndSelectedRefs(t *testing.T) {
	withMemoryRepository(t, func() {
		events, snapshot, err := collectIssueEvents()
		if err != nil || len(events) != 0 || snapshot == "" {
			t.Fatalf("empty: %v %q %v", events, snapshot, err)
		}
		identity := testIdentity(t, "reader")
		event := appendIdentityTestEvent(t, identity, "issue.open", func(e *Event) { e.Title = "one" })
		want, err := collectEvents()
		if err != nil {
			t.Fatal(err)
		}
		got, token, err := collectIssueEvents()
		if err != nil || !reflect.DeepEqual(got, want) || token == snapshot {
			t.Fatalf("read mismatch: %v", err)
		}
		mustGit(t, "update-ref", "refs/hn/remotes/origin/actors/"+identity.Actor, event.Commit)
		got, duplicateToken, err := collectIssueEvents()
		if err != nil || !reflect.DeepEqual(got, want) || duplicateToken == token {
			t.Fatalf("duplicate root: %v", err)
		}
		mustGit(t, "update-ref", "refs/hn/proposals/ignored", event.Commit)
		mustGit(t, "update-ref", "refs/hn/remotes/origin/irrelevant", event.Commit)
		_, after, err := collectIssueEvents()
		if err != nil || after != duplicateToken {
			t.Fatalf("unrelated ref changed snapshot: %v", err)
		}
		mustGit(t, "update-ref", "refs/hn/actors/bad", event.Commit)
		assertIssueReadError(t, "invalid_history")
	})
}

func assertIssueReadError(t *testing.T, code string) error {
	t.Helper()
	events, token, err := collectIssueEvents()
	var typed *IssueReadError
	if len(events) != 0 || token != "" || !errors.As(err, &typed) || typed.Code != code {
		t.Fatalf("want %s without partial result; got %d %q %v", code, len(events), token, err)
	}
	return err
}

func TestIssueReadObjectFramingAndBudgets(t *testing.T) {
	oid := strings.Repeat("a", 40)
	for _, tt := range []struct {
		name, wire    string
		object, total int64
		want          string
	}{
		{"valid", oid + " blob 3\nabc\n", 3, 3, ""},
		{"object limit", oid + " blob 4\n", 3, 10, "resource_limit"},
		{"total limit", oid + " blob 3\n", 10, 2, "resource_limit"},
		{"negative", oid + " blob -1\n", 10, 10, "invalid_history"},
		{"overflow", oid + " blob 18446744073709551616\n", 10, 10, "invalid_history"},
		{"wrong type", oid + " tree 0\n\n", 10, 10, "invalid_history"},
		{"missing", "abc missing\n", 10, 10, "invalid_history"},
		{"short header", oid + " blob", 10, 10, "invalid_history"},
		{"short body", oid + " blob 3\na", 10, 10, "invalid_history"},
		{"bad terminator", oid + " blob 3\nabcX", 10, 10, "invalid_history"},
		{"long header", strings.Repeat("x", 4096) + "\n", 10, 10, "invalid_history"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			used := int64(0)
			got, err := readIssueBatchObject(bufio.NewReaderSize(strings.NewReader(tt.wire), 1024), issueReadLimits{object: tt.object, total: tt.total}, &used)
			if tt.want == "" {
				if err != nil || string(got) != "abc" || used != 3 {
					t.Fatalf("%q %d %v", got, used, err)
				}
				return
			}
			var typed *IssueReadError
			if !errors.As(err, &typed) || typed.Code != tt.want {
				t.Fatalf("want %s, got %v", tt.want, err)
			}
		})
	}
}

func TestIssueReadRejectsCorruptionAndPending(t *testing.T) {
	for _, kind := range []string{"payload", "signature", "encoding", "sequence", "previous", "relationship", "pending", "broken pending"} {
		t.Run(kind, func(t *testing.T) {
			withMemoryRepository(t, func() {
				identity := testIdentity(t, "reader")
				event := newEvent(identity, "issue.open", 1, "")
				event.Title = "valid"
				switch kind {
				case "sequence":
					event.Sequence = 2
				case "previous":
					event.Previous = "sha256:" + strings.Repeat("a", 64)
				case "relationship":
					event.Kind = "issue.comment"
					event.Subject = "sha256:" + strings.Repeat("a", 64)
					event.Title = ""
					event.Body = "missing"
				}
				payload, sig, err := encodeAndSign(event, identity)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "payload" {
					payload = bytes.Replace(payload, []byte("valid"), []byte("other"), 1)
				}
				if kind == "signature" {
					sig[0] ^= 1
				}
				encoded := []byte(base64.RawStdEncoding.EncodeToString(sig))
				if kind == "encoding" {
					encoded = []byte("!!!")
				}
				commit := writeMemoryCommitFromEntries(t, []memoryTreeFixture{{Mode: "100644", Kind: "blob", Name: "event.json", OID: writeMemoryBlob(t, payload)}, {Mode: "100644", Kind: "blob", Name: "signature", OID: writeMemoryBlob(t, encoded)}}, nil)
				mustGit(t, "update-ref", actorRef(identity.Actor), commit)
				if kind == "pending" || kind == "broken pending" {
					gitDir, _ := requireGitRepository()
					recordMemoryPendingFixture(t, gitDir, commit)
					if kind == "broken pending" {
						paths, _ := filepath.Glob(filepath.Join(gitDir, "hn", "replication", "transactions", "*"))
						for _, path := range paths {
							if err := os.WriteFile(path, []byte("bad"), 0600); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
				err = assertIssueReadError(t, "invalid_history")
				if kind == "pending" || kind == "broken pending" {
					var admission *ReplicationAcceptancePendingError
					if !errors.As(err, &admission) {
						t.Fatalf("lost admission recovery information: %v", err)
					}
					if kind == "broken pending" && admission.Cause == nil {
						t.Fatalf("corrupt admission receipt was not diagnosed: %v", err)
					}
					if kind == "pending" && (admission.Transaction == "" || admission.ObjectID != commit) {
						t.Fatalf("missing exact pending transaction/object: %v", err)
					}
				}
			})
		})
	}
}

func TestIssueReadCountAndByteLimits(t *testing.T) {
	withMemoryRepository(t, func() {
		identity := testIdentity(t, "reader")
		stored := appendIdentityTestEvent(t, identity, "issue.open", func(e *Event) { e.Title = "one" })
		byteCount := int64(len(stored.Payload) + len(base64.RawStdEncoding.EncodeToString(stored.Signature)))
		limits := defaultIssueReadLimits()
		limits.events = 1
		limits.total = byteCount
		if _, _, err := collectIssueEventsWithLimits(limits); err != nil {
			t.Fatal(err)
		}
		limits.total--
		if _, _, err := collectIssueEventsWithLimits(limits); err == nil {
			t.Fatal("total excess accepted")
		}
		limits = defaultIssueReadLimits()
		limits.events = 1
		appendIdentityTestEvent(t, identity, "issue.open", func(e *Event) { e.Title = "two" })
		if _, _, err := collectIssueEventsWithLimits(limits); err == nil {
			t.Fatal("count excess accepted")
		}
	})
}

func TestIssueReadThousandIssueEqualityAndScaling(t *testing.T) {
	withMemoryRepository(t, func() {
		identity := testIdentity(t, "reader")
		var previous *StoredEvent
		var firstCommit string
		for i := 0; i < 1000; i++ {
			event := newEvent(identity, "issue.open", uint64(i+1), "")
			if previous != nil {
				event.Previous = previous.ID
			}
			event.Title = fmt.Sprintf("Issue %04d", i)
			event.Body = "A realistic work description with context and a requested outcome."
			var err error
			previous, err = appendEvent(event, identity)
			if i == 0 && err == nil {
				firstCommit = previous.Commit
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		realGit, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		wrapper := t.TempDir()
		logPath := filepath.Join(wrapper, "calls")
		script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$HN_TEST_ISSUE_CALLS\"\nexec \"$HN_TEST_REAL_GIT\" \"$@\"\n"
		if err := os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HN_TEST_REAL_GIT", realGit)
		t.Setenv("HN_TEST_ISSUE_CALLS", logPath)
		t.Setenv("PATH", wrapper+string(os.PathListSeparator)+os.Getenv("PATH"))
		start := time.Now()
		want, err := collectEvents()
		oldDuration := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		oldCalls, _ := os.ReadFile(logPath)
		if err := os.WriteFile(logPath, nil, 0600); err != nil {
			t.Fatal(err)
		}
		start = time.Now()
		got, token, err := collectIssueEvents()
		newDuration := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		calls, _ := os.ReadFile(logPath)
		if len(got) != 1000 || !reflect.DeepEqual(got, want) {
			t.Fatal("1000-issue facts differ")
		}
		if strings.Count(string(calls), "cat-file --batch\n") != 1 || strings.Count(string(calls), "rev-list ") != 1 || strings.Contains(string(calls), "show ") {
			t.Fatalf("nonbatch read:\n%s", calls)
		}
		again, againToken, err := collectIssueEvents()
		if err != nil || token != againToken || !reflect.DeepEqual(got, again) {
			t.Fatalf("unstable rebuild: %v", err)
		}

		mustGit(t, "update-ref", actorRef(identity.Actor), firstCommit)
		if err := os.WriteFile(logPath, nil, 0600); err != nil {
			t.Fatal(err)
		}
		small, _, err := collectIssueEvents()
		if err != nil || len(small) != 1 {
			t.Fatalf("small fixture: %v", err)
		}
		smallCalls, _ := os.ReadFile(logPath)
		if bytes.Count(calls, []byte("\n")) != bytes.Count(smallCalls, []byte("\n")) {
			t.Fatalf("process count grows with issues: small=%s large=%s", smallCalls, calls)
		}
		t.Logf("1000 issues: old=%s/%d git processes; batch=%s/%d git processes; payload/signature batches=1; semantic equality=true", oldDuration, bytes.Count(oldCalls, []byte("\n")), newDuration, bytes.Count(calls, []byte("\n")))
	})
}

func TestIssueReadSnapshotMovesDuringBatch(t *testing.T) {
	withMemoryRepository(t, func() {
		identity := testIdentity(t, "reader")
		first := appendIdentityTestEvent(t, identity, "issue.open", func(e *Event) { e.Title = "one" })
		second := appendIdentityTestEvent(t, identity, "issue.open", func(e *Event) { e.Title = "two" })
		mustGit(t, "update-ref", actorRef(identity.Actor), first.Commit)
		realGit, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		wrapper := t.TempDir()
		script := "#!/bin/sh\nif [ \"$1\" = cat-file ]; then \"$HN_TEST_REAL_GIT\" update-ref \"$HN_TEST_ISSUE_REF\" \"$HN_TEST_ISSUE_HEAD\" || exit; fi\nexec \"$HN_TEST_REAL_GIT\" \"$@\"\n"
		if err := os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HN_TEST_REAL_GIT", realGit)
		t.Setenv("HN_TEST_ISSUE_REF", actorRef(identity.Actor))
		t.Setenv("HN_TEST_ISSUE_HEAD", second.Commit)
		t.Setenv("PATH", wrapper+string(os.PathListSeparator)+os.Getenv("PATH"))
		assertIssueReadError(t, "concurrent_update")
		events, _, err := collectIssueEvents()
		if err != nil || len(events) != 2 {
			t.Fatalf("retry: %d %v", len(events), err)
		}
	})
}

func TestIssueReadAttachments(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			withMemoryRepository(t, func() {
				identity := testIdentity(t, "runner")
				proposal := appendIdentityTestEvent(t, identity, "proposal.open", func(e *Event) {
					e.Title = "proposal"
					e.Base = strings.Repeat("a", 40)
					e.Head = strings.Repeat("b", 40)
				})
				request := appendIdentityTestEvent(t, identity, "run.request", func(e *Event) {
					e.Subject = proposal.ID
					e.Pipeline = "test"
					e.Commit = proposal.Event.Head
					e.Definition = eventID([]byte("pipeline"))
				})
				event := newEvent(identity, "run.result", request.Event.Sequence+1, request.ID)
				event.Subject = request.ID
				event.Pipeline = request.Event.Pipeline
				event.Commit = request.Event.Commit
				event.Definition = request.Event.Definition
				event.Outcome = "passed"
				event.Backend = "host"
				event.Platform = "test"
				event.Runner = "fixture"
				event.Log = eventID([]byte("hello\n"))
				payload, sig, err := encodeAndSign(event, identity)
				if err != nil {
					t.Fatal(err)
				}
				entries := []memoryTreeFixture{{Mode: "100644", Kind: "blob", Name: "event.json", OID: writeMemoryBlob(t, payload)}, {Mode: "100644", Kind: "blob", Name: "signature", OID: writeMemoryBlob(t, []byte(base64.RawStdEncoding.EncodeToString(sig)))}}
				if mode != "missing" {
					log := []byte("hello\n")
					if mode == "corrupt" {
						log = []byte("wrong\n")
					}
					entries = append(entries, memoryTreeFixture{Mode: "100644", Kind: "blob", Name: "log.txt", OID: writeMemoryBlob(t, log)})
				}
				commit := writeMemoryCommitFromEntries(t, entries, []string{request.Commit})
				mustGit(t, "update-ref", actorRef(identity.Actor), commit)
				if mode != "valid" {
					assertIssueReadError(t, "invalid_history")
					return
				}
				want, err := collectEvents()
				if err != nil {
					t.Fatal(err)
				}
				got, _, err := collectIssueEvents()
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("attachment mismatch: %v", err)
				}
			})
		})
	}
}

func TestIssueReadChildFailure(t *testing.T) {
	for _, tt := range []struct {
		name, script, code string
		exitCode           int
	}{
		{"failed child", "printf '%s\\n' 'failed child' >&2; exit 9", "repository_error", 9},
		{"failed truncated body", "read request; printf '%s\\n' 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa blob 3'; printf a; printf '%s\\n' 'failed child' >&2; exit 9", "repository_error", 9},
		{"successful malformed stream", "read request; printf '%s\\n' 'malformed'; exit 0", "invalid_history", 0},
		{"cancel malformed stream", "read request; printf '%s\\n' 'malformed'; read more", "invalid_history", 0},
		{"cancel over budget", "read request; printf '%s\\n' 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa blob 8388609'; read more", "resource_limit", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			withMemoryRepository(t, func() {
				identity := testIdentity(t, "reader")
				appendIdentityTestEvent(t, identity, "issue.open", func(e *Event) { e.Title = "one" })
				realGit, err := exec.LookPath("git")
				if err != nil {
					t.Fatal(err)
				}
				wrapper := t.TempDir()
				script := "#!/bin/sh\nif [ \"$1\" = cat-file ]; then " + tt.script + "; exit; fi\nexec \"$HN_TEST_REAL_GIT\" \"$@\"\n"
				if err := os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("HN_TEST_REAL_GIT", realGit)
				t.Setenv("PATH", wrapper+string(os.PathListSeparator)+os.Getenv("PATH"))
				err = assertIssueReadError(t, tt.code)
				var child *exec.ExitError
				if tt.exitCode != 0 {
					if !strings.Contains(err.Error(), "failed child") || !errors.As(err, &child) || child.ExitCode() != tt.exitCode {
						t.Fatalf("lost child diagnostic/exit cause: %v", err)
					}
				} else if errors.As(err, &child) {
					t.Fatalf("cleanup cancellation replaced reader diagnosis: %v", err)
				}
			})
		})
	}
}

func TestIssueReadSHA256Git(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(original); err != nil {
			t.Error(err)
		}
	}()
	if _, err := gitOutput("init", "-q", "--object-format=sha256"); err != nil {
		t.Skipf("Git SHA256 unavailable: %v", err)
	}
	mustGit(t, "config", "user.name", "Test")
	mustGit(t, "config", "user.email", "test@hn.invalid")
	identity := testIdentity(t, "reader")
	stored := appendIdentityTestEvent(t, identity, "issue.open", func(e *Event) { e.Title = "sha256 repository" })
	got, _, err := collectIssueEvents()
	if err != nil || len(got) != 1 || got[0].ID != stored.ID || len(got[0].Commit) != 64 {
		t.Fatalf("sha256 Git read: %v %v", got, err)
	}
}
