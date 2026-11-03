package main

import (
	"strings"
	"testing"
	"time"
)

func TestRunnerSyncProgress(t *testing.T) {
	inReplicationTestRepository(t)
	// Exercise a real failed sync, then a real successful bare-remote sync.
	// The first line must precede cmdSync, which otherwise stays silent while
	// fetching/verifying, and both terminal outcomes must identify duration.
	for _, success := range []bool{false, true} {
		remote := "missing"
		if success {
			remote = "origin"
			bare := t.TempDir()
			mustGit(t, "init", "--bare", "-q", bare)
			mustGit(t, "remote", "add", remote, bare)
		}
		out, err := captureTestOutput(t, func() error { return runnerSync(remote) })
		if (err == nil) != success {
			t.Fatalf("sync success=%t: %v", success, err)
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) < 2 {
			t.Fatal("missing sync lifecycle lines", out)
		}
		first, last := lines[0], lines[len(lines)-1]
		end := "failed"
		if success {
			end = "completed"
		}
		if !strings.Contains(first, "Runner sync started for ") || !strings.Contains(last, "Runner sync "+end+" for ") {
			t.Fatal("missing start/terminal sync status", out)
		}
		for _, line := range []string{first, last} {
			stamp := strings.SplitN(line, " ", 2)[0]
			if _, err := time.Parse(time.RFC3339, stamp); err != nil || !strings.HasSuffix(stamp, "Z") {
				t.Fatal("runner status lacks UTC RFC3339 timestamp", line)
			}
		}
		parts := strings.Split(last, " after ")
		if len(parts) != 2 {
			t.Fatal("terminal sync status lacks duration", last)
		}
		if elapsed, err := time.ParseDuration(parts[1]); err != nil || elapsed < 0 {
			t.Fatal("invalid sync duration", last)
		}
	}
}
