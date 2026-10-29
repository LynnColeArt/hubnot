package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Source-assisted adapter proof: public CLI fixtures create authentic signed
// material, then a test-only Git wrapper stalls the selected post-intake phase.
// This is not package process-kill or synchronous kernel-I/O cancellation proof.
func TestContributionPostIntakeDeadlines(t *testing.T) {
	binary := betaContributionBinary(t, false)
	for _, phase := range []string{"coverage", "copy", "receipt"} {
		t.Run(phase, func(t *testing.T) {
			f := newBetaContributionCLI(t, binary)
			limits := betaContributionLimits()
			limits.Milliseconds = 2000
			owner, _, policy := f.enabled("late-deadline-"+phase, limits, false)
			source := f.repository()
			f.hn(source, true, "init", "--name", "Disposable contributor")
			revision := contributionEventID(t, f.hn(source, true, "issue", "open", "--json", "Retained exact input"))
			input := f.prepare(source, policy.Contributions.Project, revision)
			t.Chdir(owner)
			a, err := loadContributionAuthority()
			if err != nil {
				t.Fatal(err)
			}
			x, err := contributionExchange(a)
			if err != nil {
				t.Fatal(err)
			}
			const intent = "completed-before-late-stall"
			attempt, err := x.Receive(context.Background(), input.File, input.Digest, intent, input.Refs)
			if err != nil || attempt.Status != "pending" || attempt.Reserved.Milliseconds != limits.Milliseconds {
				t.Fatalf("initial Receive must complete before fault installation: %+v %v", attempt, err)
			}
			t.Logf("initial Receive COMPLETED: phase=%s digest=%s refs=%v reservedMillis=%d", phase, attempt.Artifact, attempt.Refs, attempt.Reserved.Milliseconds)
			if phase == "receipt" {
				if err = contributionRetainIntake(a, x, intent); err != nil {
					t.Fatal(err)
				}
			}
			before, _, err := x.Accounting()
			if err != nil {
				t.Fatal(err)
			}
			refsBefore := f.git(owner, "show-ref")
			// A changed current policy must not extend the original reservation.
			x.policy.Limits.Milliseconds = 60000
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			gitdir, err := contributionGitDirectory()
			if err != nil {
				t.Fatal(err)
			}
			wrapperDir := t.TempDir()
			marker := filepath.Join(wrapperDir, "phase-reached")
			pattern := "*' init --bare '*"
			if phase == "coverage" {
				pattern = fmt.Sprintf("'--git-dir=%s hash-object -t '*", gitdir)
			} else if phase == "copy" {
				pattern = fmt.Sprintf("'--git-dir=%s hash-object -w '*", gitdir)
			}
			// exec gives CommandContext ownership of the actual stalled process;
			// the eight-second fallback also bounds the original-runtime red run.
			script := fmt.Sprintf("#!/bin/sh\ncase \"$*\" in\n%s) printf '%%s\\n' '%s' > '%s'; exec /bin/sleep 8 ;;\nesac\nexec '%s' \"$@\"\n", pattern, phase, marker, git)
			if err = os.WriteFile(filepath.Join(wrapperDir, "git"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			path := os.Getenv("PATH")
			t.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+path)
			start := time.Now()
			if phase == "receipt" {
				err = contributionReceiptIntake(a, x, intent)
			} else {
				err = contributionRetainIntake(a, x, intent)
			}
			elapsed := time.Since(start)
			t.Setenv("PATH", path)
			marked, markerErr := os.ReadFile(marker)
			if markerErr != nil || strings.TrimSpace(string(marked)) != phase {
				t.Fatalf("late-phase positive marker missing; early intake cancellation cannot satisfy test: %q %v", marked, markerErr)
			}
			t.Logf("positive late-phase marker=%s; actual context-aware subprocess returned after %v: %v", marked, elapsed, err)
			if err == nil || elapsed > 5*time.Second {
				t.Errorf("post-intake %s failed to cancel within original 2s reservation (5s harness allowance): elapsed=%v err=%v", phase, elapsed, err)
			}
			if after := f.git(owner, "show-ref"); after != refsBefore {
				t.Fatal("canceled late phase published raw/manifest/receipt refs")
			}
			after, attempts, err := x.Accounting()
			if err != nil || !reflect.DeepEqual(before, after) || len(attempts) != 1 {
				t.Fatalf("late cancellation changed original charged ledger: %+v %v", after, err)
			}
			if attempts[0].Status != "pending" || attempts[0].Artifact != input.Digest || !reflect.DeepEqual(attempts[0].Refs, input.Refs) || attempts[0].Reserved != attempt.Reserved {
				t.Fatalf("retained exact selection/reservation lost: %+v", attempts[0])
			}
			inspected, err := x.Inspect(context.Background(), intent)
			if err != nil {
				t.Fatal(err)
			}
			if len(inspected.Records) != 1 || inspected.Records[0].ID() != input.Submission || inspected.Intake.Artifact != input.Digest {
				t.Fatal("retained immutable signed input changed")
			}
			_ = inspected.Close()
			if err = os.Remove(input.File); err != nil {
				t.Fatal(err)
			}
			replayed, err := x.Receive(context.Background(), input.File, input.Digest, intent, input.Refs)
			if err != nil || !replayed.Replayed || replayed.Reserved != attempt.Reserved {
				t.Fatalf("exact retry reopened foreign input or changed reservation: %+v %v", replayed, err)
			}
			if err = contributionRetainIntake(a, x, intent); err != nil {
				t.Fatal("raw exact retry:", err)
			}
			retried, _, err := x.Accounting()
			if err != nil || !reflect.DeepEqual(before, retried) {
				t.Fatal("raw retry charged/refunded original request", err)
			}
			if err = contributionReceiptIntake(a, x, intent); err != nil {
				t.Fatal("receipt exact retry:", err)
			}
			if got := f.hn(owner, true, "issue", "list", "--json"); !strings.Contains(got, "Retained exact input") {
				t.Fatal("retry did not admit original exact content:", got)
			}
			t.Log("exact retained retry completed with separate charged receipt control; partial immutable objects may remain charged")
		})
	}
}
