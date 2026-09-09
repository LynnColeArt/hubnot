package main

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// IssueReadError classifies ingestion failures without erasing their underlying
// diagnostics (including replication acceptance recovery information).
type IssueReadError struct {
	Code string
	Err  error
}

func (e *IssueReadError) Error() string { return e.Code + ": " + e.Err.Error() }
func (e *IssueReadError) Unwrap() error { return e.Err }
func issueReadFailure(code string, err error) error {
	var typed *IssueReadError
	if errors.As(err, &typed) {
		return err
	}
	return &IssueReadError{Code: code, Err: err}
}

type issueReadLimits struct {
	events, refs  int
	object, total int64
}

func defaultIssueReadLimits() issueReadLimits {
	return issueReadLimits{events: 100000, refs: 100000, object: 8 << 20, total: 256 << 20}
}

// collectIssueEvents rebuilds from accepted signed objects only. Its token binds
// the selected ref names and OIDs observed before and after loading, not a lock
// or a promise that another actor cannot subsequently append.
func collectIssueEvents() ([]StoredEvent, string, error) {
	return collectIssueEventsWithLimits(defaultIssueReadLimits())
}

func collectIssueEventsWithLimits(limits issueReadLimits) ([]StoredEvent, string, error) {
	gitDir, err := requireGitRepository()
	if err != nil {
		return nil, "", issueReadFailure("repository_error", err)
	}
	roots, snapshot, err := issueReadRefs(limits)
	if err != nil {
		return nil, "", err
	}
	denied, pending, err := loadReplicationAcceptanceState(gitDir)
	if err != nil {
		return nil, "", issueReadFailure("invalid_history", &ReplicationAcceptancePendingError{Cause: err})
	}
	commits := make([]string, 0)
	if len(roots) > 0 {
		seen := make(map[string]bool)
		err = issueGitLines(strings.NewReader(strings.Join(roots, "\n")+"\n"), func(line string) error {
			if !validGitOID(line) {
				return issueReadFailure("invalid_history", fmt.Errorf("invalid history commit identifier"))
			}
			if seen[line] {
				return nil
			}
			if len(commits) >= limits.events {
				return issueReadFailure("resource_limit", fmt.Errorf("issue history exceeds %d events", limits.events))
			}
			seen[line] = true
			commits = append(commits, line)
			return nil
		}, "rev-list", "--reverse", "--stdin")
		if err != nil {
			return nil, "", err
		}
	}
	for _, commit := range commits {
		if err := issueReadAdmission(commit, denied, pending); err != nil {
			return nil, "", err
		}
	}
	var events []StoredEvent
	if len(commits) > 0 {
		events, err = issueReadStoredEvents(commits, limits)
		if err != nil {
			return nil, "", err
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Event.Timestamp != events[j].Event.Timestamp {
			return events[i].Event.Timestamp < events[j].Event.Timestamp
		}
		return events[i].ID < events[j].ID
	})
	if err = validateActorChains(events); err != nil {
		return nil, "", issueReadFailure("invalid_history", err)
	}
	if err = validateEventRelationships(events); err != nil {
		return nil, "", issueReadFailure("invalid_history", err)
	}
	denied, pending, err = loadReplicationAcceptanceState(gitDir)
	if err != nil {
		return nil, "", issueReadFailure("invalid_history", &ReplicationAcceptancePendingError{Cause: err})
	}
	for _, commit := range commits {
		if err := issueReadAdmission(commit, denied, pending); err != nil {
			return nil, "", err
		}
	}
	_, after, err := issueReadRefs(limits)
	if err != nil {
		return nil, "", err
	}
	if after != snapshot {
		return nil, "", issueReadFailure("concurrent_update", fmt.Errorf("accepted issue histories changed while reading; retry"))
	}
	return events, snapshot, nil
}

func issueReadAdmission(commit string, denied map[string]bool, pending map[string]replicationTransactionRecord) error {
	if !denied[commit] {
		return nil
	}
	failure := &ReplicationAcceptancePendingError{ObjectID: commit}
	for _, record := range pending {
		if record.PendingObjects != nil {
			for _, id := range *record.PendingObjects {
				if id == commit {
					failure.Transaction = record.ID
					failure.Remote = record.Remote
					return issueReadFailure("invalid_history", failure)
				}
			}
		}
	}
	return issueReadFailure("invalid_history", failure)
}

func issueReadRefs(limits issueReadLimits) ([]string, string, error) {
	var pairs []string
	roots := make(map[string]bool)
	count := 0
	err := issueGitLines(nil, func(line string) error {
		count++
		if count > limits.refs {
			return issueReadFailure("resource_limit", fmt.Errorf("issue ref enumeration exceeds %d entries", limits.refs))
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || !validGitOID(fields[1]) {
			return issueReadFailure("invalid_history", fmt.Errorf("invalid accepted ref record"))
		}
		ref, oid := fields[0], fields[1]
		if strings.HasPrefix(ref, "refs/hn/actors/") {
			if !validActorFingerprint(strings.TrimPrefix(ref, "refs/hn/actors/")) {
				return issueReadFailure("invalid_history", fmt.Errorf("invalid local actor ref %q", ref))
			}
		} else if _, _, ok := parseAcceptedActorRef(ref); !ok {
			return nil
		}
		pairs = append(pairs, ref+" "+oid+"\n")
		roots[oid] = true
		return nil
	}, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors", "refs/hn/remotes")
	if err != nil {
		return nil, "", err
	}
	sort.Strings(pairs)
	heads := make([]string, 0, len(roots))
	for head := range roots {
		heads = append(heads, head)
	}
	sort.Strings(heads)
	return heads, eventID([]byte("hn.issue.snapshot/1\n" + strings.Join(pairs, ""))), nil
}

// issueReadStderr drains stderr without allowing a failing Git process to grow
// an unbounded diagnostic buffer. It is accessed only by exec's copy goroutine
// until Wait returns.
type issueReadStderr struct{ data []byte }

func (b *issueReadStderr) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 4096 - len(b.data)
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}

func issueGitLines(input io.Reader, visit func(string) error, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdin = input
	stderr := &issueReadStderr{}
	cmd.Stderr = stderr
	output, err := cmd.StdoutPipe()
	if err != nil {
		return issueReadFailure("repository_error", err)
	}
	if err = cmd.Start(); err != nil {
		return issueReadFailure("repository_error", err)
	}
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), 65536)
	var readErr error
	for scanner.Scan() {
		if readErr = visit(scanner.Text()); readErr != nil {
			break
		}
	}
	if readErr == nil && scanner.Err() != nil {
		readErr = issueReadFailure("resource_limit", fmt.Errorf("bounded Git output: %w", scanner.Err()))
	}
	if readErr != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if readErr != nil {
		return readErr
	}
	if waitErr != nil {
		return issueReadFailure("repository_error", fmt.Errorf("git %s: %s", args[0], commandFailure(waitErr, string(stderr.data))))
	}
	return nil
}

func issueReadStoredEvents(commits []string, limits issueReadLimits) (result []StoredEvent, resultErr error) {
	cmd := exec.Command("git", "cat-file", "--batch")
	stderr := &issueReadStderr{}
	cmd.Stderr = stderr
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, issueReadFailure("repository_error", err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		_ = input.Close()
		return nil, issueReadFailure("repository_error", err)
	}
	if err = cmd.Start(); err != nil {
		_ = input.Close()
		return nil, issueReadFailure("repository_error", err)
	}
	finished := false
	defer func() {
		if finished {
			return
		}
		// Give a naturally terminating child a bounded opportunity to report
		// its real exit cause. Closing stdin first could itself cause a child
		// failure and misclassify a deliberate parser/limit cancellation.
		exited := make(chan error, 1)
		go func() { exited <- cmd.Wait() }()
		timer := time.NewTimer(100 * time.Millisecond)
		defer timer.Stop()
		select {
		case waitErr := <-exited:
			var readerErr *IssueReadError
			limited := errors.As(resultErr, &readerErr) && readerErr.Code == "resource_limit"
			if waitErr != nil && !limited {
				resultErr = issueReadFailure("repository_error", fmt.Errorf("git cat-file: %s: %w", strings.TrimSpace(string(stderr.data)), waitErr))
			}
		case <-timer.C:
			// This kill is our cancellation, not evidence of repository failure.
			_ = cmd.Process.Kill()
			<-exited
		}
		_ = input.Close()
	}()
	reader := bufio.NewReaderSize(output, 4096)
	used := int64(0)
	read := func(path string) ([]byte, error) {
		if _, err := io.WriteString(input, path+"\n"); err != nil {
			return nil, issueReadFailure("repository_error", err)
		}
		return readIssueBatchObject(reader, limits, &used)
	}
	events := make([]StoredEvent, 0, len(commits))
	for _, commit := range commits {
		payload, err := read(commit + ":event.json")
		if err != nil {
			return nil, err
		}
		encoded, err := read(commit + ":signature")
		if err != nil {
			return nil, err
		}
		signature, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
		if err != nil {
			return nil, issueReadFailure("invalid_history", fmt.Errorf("invalid signature encoding in %s", commit))
		}
		event, id, err := verifyEvent(payload, signature)
		if err != nil {
			return nil, issueReadFailure("invalid_history", fmt.Errorf("verify %s: %w", commit, err))
		}
		stored := StoredEvent{ID: id, Commit: commit, Event: event, Payload: payload, Signature: signature, Attachments: make(map[string][]byte)}
		if event.Kind == "run.result" {
			log, err := read(commit + ":log.txt")
			if err != nil {
				return nil, err
			}
			if eventID(log) != event.Log {
				return nil, issueReadFailure("invalid_history", fmt.Errorf("run result %s log digest does not match", shortID(id)))
			}
			stored.Attachments["log.txt"] = log
		}
		events = append(events, stored)
	}
	if err = input.Close(); err != nil {
		return nil, issueReadFailure("repository_error", err)
	}
	// No request remains outstanding. Require EOF so surplus/malformed responses
	// cannot be silently accepted as a successful batch.
	if _, err = reader.ReadByte(); err != io.EOF {
		return nil, issueReadFailure("invalid_history", fmt.Errorf("unexpected trailing batch output: %v", err))
	}
	err = cmd.Wait()
	finished = true
	if err != nil {
		return nil, issueReadFailure("repository_error", fmt.Errorf("git cat-file: %s: %w", strings.TrimSpace(string(stderr.data)), err))
	}
	return events, nil
}

func readIssueBatchObject(reader *bufio.Reader, limits issueReadLimits, used *int64) ([]byte, error) {
	header, err := reader.ReadSlice('\n')
	if err != nil || len(header) > 1024 {
		return nil, issueReadFailure("invalid_history", fmt.Errorf("malformed or truncated batch header"))
	}
	fields := strings.Fields(string(header))
	if len(fields) != 3 || !validGitOID(fields[0]) || fields[1] != "blob" {
		return nil, issueReadFailure("invalid_history", fmt.Errorf("missing object or invalid batch object header"))
	}
	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || size < 0 {
		return nil, issueReadFailure("invalid_history", fmt.Errorf("invalid batch object size"))
	}
	if size > limits.object || *used > limits.total || size > limits.total-*used {
		return nil, issueReadFailure("resource_limit", fmt.Errorf("issue object read exceeds byte budget"))
	}
	// The production object ceiling fits int on all supported platforms. Also
	// guard custom limits so conversion cannot wrap on 32-bit systems.
	if uint64(size) > uint64(^uint(0)>>1) {
		return nil, issueReadFailure("resource_limit", fmt.Errorf("issue object size exceeds addressable memory"))
	}
	body := make([]byte, int(size))
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, issueReadFailure("invalid_history", fmt.Errorf("truncated batch body: %w", err))
	}
	terminator, err := reader.ReadByte()
	if err != nil || terminator != '\n' {
		return nil, issueReadFailure("invalid_history", fmt.Errorf("invalid batch body terminator"))
	}
	*used += size
	return body, nil
}
