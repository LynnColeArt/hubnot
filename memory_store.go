package main

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	memoryRefPrefix         = "refs/hn/memory/"
	acceptedMemoryRefPrefix = "refs/hn/remotes/"
	maxMemoryPayloadBytes   = 512 * 1024
)

// StoredMemory retains both protocol identity and Git transport identity.
// Callers must use ID, never Commit, for signed memory relationships.
type StoredMemory struct {
	ID        string
	Commit    string
	Envelope  MemoryEnvelope
	Payload   []byte
	Signature []byte
}

type memoryStreamSource struct {
	Ref      string
	Remote   string
	Actor    string
	Stream   string
	Head     string
	Accepted bool
}

func memoryRef(actor, stream string) (string, error) {
	if !validActorFingerprint(actor) {
		return "", fmt.Errorf("invalid memory actor")
	}
	if !validMemoryStreamID(stream) {
		return "", fmt.Errorf("invalid memory stream")
	}
	return memoryRefPrefix + actor + "/" + strings.TrimPrefix(stream, "sha256:"), nil
}

func acceptedMemoryRef(remote, actor, stream string) (string, error) {
	if !validMemoryRemote(remote) {
		return "", fmt.Errorf("invalid memory remote")
	}
	local, err := memoryRef(actor, stream)
	if err != nil {
		return "", err
	}
	return acceptedMemoryRefPrefix + remote + "/memory/" + strings.TrimPrefix(local, memoryRefPrefix), nil
}

func parseMemoryRef(ref string) (actor, stream string, ok bool) {
	if !strings.HasPrefix(ref, memoryRefPrefix) {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(ref, memoryRefPrefix), "/")
	if len(parts) != 2 || !validActorFingerprint(parts[0]) {
		return "", "", false
	}
	stream = "sha256:" + parts[1]
	if !validMemoryStreamID(stream) {
		return "", "", false
	}
	return parts[0], stream, true
}

func parseAcceptedMemoryRef(ref string) (remote, actor, stream string, ok bool) {
	if !strings.HasPrefix(ref, acceptedMemoryRefPrefix) {
		return "", "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(ref, acceptedMemoryRefPrefix), "/")
	if len(parts) != 4 || parts[1] != "memory" || !validMemoryRemote(parts[0]) || !validActorFingerprint(parts[2]) {
		return "", "", "", false
	}
	stream = "sha256:" + parts[3]
	if !validMemoryStreamID(stream) {
		return "", "", "", false
	}
	return parts[0], parts[2], stream, true
}

func validMemoryRemote(remote string) bool {
	return validReplicationRemote(remote) && !strings.HasSuffix(remote, ".lock")
}

func appendMemory(envelope MemoryEnvelope, identity *Identity) (*StoredMemory, error) {
	ref, err := memoryRef(envelope.Actor, envelope.Stream)
	if err != nil {
		return nil, err
	}
	expected, exists, err := refValue(ref)
	if err != nil {
		return nil, err
	}
	if !exists {
		expected = ""
	}
	return appendMemoryAtHead(envelope, identity, expected)
}

// appendMemoryAtHead appends against one caller-observed head. It deliberately
// does not reload after a CAS failure: changing sequence or previous requires a
// newly signed envelope and therefore belongs to the caller's retry.
func appendMemoryAtHead(envelope MemoryEnvelope, identity *Identity, expectedHead string) (*StoredMemory, error) {
	ref, err := memoryRef(envelope.Actor, envelope.Stream)
	if err != nil {
		return nil, err
	}
	if err := ensureMemorySigningRoot(envelope.Actor, envelope.Stream, expectedHead); err != nil {
		return nil, err
	}
	payload, signature, err := encodeAndSignMemory(envelope, identity)
	if err != nil {
		return nil, err
	}
	id := memoryID(payload)

	if expectedHead == "" {
		if envelope.Sequence != 1 || envelope.Previous != "" {
			return nil, fmt.Errorf("first memory in stream must have sequence 1 and no previous memory")
		}
	} else {
		previous, err := loadMemoryStreamAt("", memoryStreamSource{
			Ref: ref, Actor: envelope.Actor, Stream: envelope.Stream, Head: expectedHead,
		})
		if err != nil {
			return nil, fmt.Errorf("read memory stream head %s: %w", safeDiagnostic(expectedHead), err)
		}
		head := previous[len(previous)-1]
		if envelope.Sequence != head.Envelope.Sequence+1 || envelope.Previous != head.ID {
			return nil, fmt.Errorf("memory stream changed while creating memory; reload and retry with a newly signed envelope")
		}
	}

	memoryBlob, err := gitInput(payload, nil, "hash-object", "-w", "--stdin")
	if err != nil {
		return nil, fmt.Errorf("write memory payload: %w", err)
	}
	signatureText := []byte(base64.RawStdEncoding.EncodeToString(signature))
	signatureBlob, err := gitInput(signatureText, nil, "hash-object", "-w", "--stdin")
	if err != nil {
		return nil, fmt.Errorf("write memory signature: %w", err)
	}
	treeInput := fmt.Sprintf(
		"100644 blob %s\tmemory.json\n100644 blob %s\tsignature\n",
		strings.TrimSpace(string(memoryBlob)), strings.TrimSpace(string(signatureBlob)),
	)
	tree, err := gitInput([]byte(treeInput), nil, "mktree")
	if err != nil {
		return nil, fmt.Errorf("write memory tree: %w", err)
	}

	commitArgs := []string{"commit-tree", strings.TrimSpace(string(tree)), "-m", "hn memory " + id}
	if expectedHead != "" {
		commitArgs = append(commitArgs, "-p", expectedHead)
	}
	email := shortID(identity.Actor) + "@hn.invalid"
	env := []string{
		"GIT_AUTHOR_NAME=" + identity.Name,
		"GIT_AUTHOR_EMAIL=" + email,
		"GIT_AUTHOR_DATE=" + envelope.Timestamp,
		"GIT_COMMITTER_NAME=" + identity.Name,
		"GIT_COMMITTER_EMAIL=" + email,
		"GIT_COMMITTER_DATE=" + envelope.Timestamp,
	}
	commit, err := gitInput(nil, env, commitArgs...)
	if err != nil {
		return nil, fmt.Errorf("write memory commit: %w", err)
	}
	commitID := strings.TrimSpace(string(commit))
	if _, err := gitOutput("update-ref", "-m", "hn: append memory", ref, commitID, expectedHead); err != nil {
		return nil, fmt.Errorf("append memory stream %s: compare-and-swap failed; reload and retry with a newly signed envelope", safeDiagnostic(ref))
	}
	return &StoredMemory{ID: id, Commit: commitID, Envelope: envelope, Payload: payload, Signature: signature}, nil
}

// ensureMemorySigningRoot leaves unrelated actors and streams independent.
// loadMemoryStreamAt verifies owner, sequence, signed/Git predecessors and
// pending admission without importing objects or changing private state.
func ensureMemorySigningRoot(actor, stream, head string) error {
	refs, err := gitText("for-each-ref", "--format=%(refname) %(objectname)", acceptedMemoryRefPrefix)
	if err != nil {
		return err
	}
	var sources []memoryStreamSource
	fields := strings.Fields(refs)
	for i := 0; i+1 < len(fields); i += 2 {
		remote, owner, id, ok := parseAcceptedMemoryRef(fields[i])
		if ok && owner == actor && id == stream {
			sources = append(sources, memoryStreamSource{Ref: fields[i], Remote: remote, Actor: actor, Stream: stream, Head: fields[i+1], Accepted: true})
		}
	}
	if len(sources) == 0 {
		return nil
	}
	ids := func(source memoryStreamSource) ([]string, error) {
		memories, err := loadMemoryStreamAt("", source)
		if err != nil {
			return nil, err
		}
		result := make([]string, len(memories))
		for i, memory := range memories {
			result[i] = memory.ID
		}
		return result, nil
	}
	var local []string
	if head != "" {
		ref, err := memoryRef(actor, stream)
		if err != nil {
			return err
		}
		local, err = ids(memoryStreamSource{Ref: ref, Actor: actor, Stream: stream, Head: head})
		if err != nil {
			return fmt.Errorf("verify memory signing root: %w", err)
		}
	}
	for _, source := range sources {
		accepted, err := ids(source)
		if err != nil {
			return fmt.Errorf("verify accepted memory history: %w", err)
		}
		if err := requireSigningPrefix(local, accepted, "memory"); err != nil {
			return err
		}
	}
	return nil
}

func loadStoredMemory(commit string) (*StoredMemory, error) {
	gitDir, err := requireGitRepository()
	if err != nil {
		return nil, err
	}
	if err := replicationPendingError(gitDir, commit); err != nil {
		return nil, err
	}
	return loadStoredMemoryAt("", commit)
}

func loadStoredMemoryAt(gitDir, commit string) (*StoredMemory, error) {
	entries, err := exactMemoryTreeAt(gitDir, commit)
	if err != nil {
		return nil, err
	}
	payload, err := boundedMemoryBlobAt(gitDir, entries["memory.json"], maxMemoryPayloadBytes, "memory.json", commit)
	if err != nil {
		return nil, err
	}
	signatureEncoded, err := boundedMemoryBlobAt(gitDir, entries["signature"], 128, "signature", commit)
	if err != nil {
		return nil, err
	}
	signature, err := base64.RawStdEncoding.DecodeString(string(signatureEncoded))
	if err != nil || base64.RawStdEncoding.EncodeToString(signature) != string(signatureEncoded) {
		return nil, fmt.Errorf("memory commit %s has invalid signature encoding", safeDiagnostic(commit))
	}
	envelope, id, err := verifyMemory(payload, signature)
	if err != nil {
		return nil, fmt.Errorf("verify memory commit %s: %w", safeDiagnostic(commit), err)
	}
	return &StoredMemory{ID: id, Commit: commit, Envelope: envelope, Payload: payload, Signature: signature}, nil
}

func exactMemoryTreeAt(gitDir, commit string) (map[string]string, error) {
	objectType, err := gitTextAt(gitDir, "cat-file", "-t", commit)
	if err != nil || objectType != "commit" {
		return nil, fmt.Errorf("memory commit %s is unavailable or not a commit", safeDiagnostic(commit))
	}
	output, err := gitOutputAt(gitDir, "ls-tree", "-z", commit)
	if err != nil {
		return nil, fmt.Errorf("inspect memory tree at %s: %w", safeDiagnostic(commit), err)
	}
	entries := make(map[string]string, 2)
	for _, raw := range strings.Split(string(output), "\x00") {
		if raw == "" {
			continue
		}
		header, name, found := strings.Cut(raw, "\t")
		fields := strings.Fields(header)
		if !found || len(fields) != 3 || fields[0] != "100644" || fields[1] != "blob" || name == "" {
			return nil, fmt.Errorf("memory commit %s has invalid tree entry", safeDiagnostic(commit))
		}
		if name != "memory.json" && name != "signature" {
			return nil, fmt.Errorf("memory commit %s tree contains unexpected entry %s", safeDiagnostic(commit), safeDiagnostic(name))
		}
		if _, duplicate := entries[name]; duplicate {
			return nil, fmt.Errorf("memory commit %s tree contains duplicate entry %s", safeDiagnostic(commit), name)
		}
		entries[name] = fields[2]
	}
	if len(entries) != 2 || entries["memory.json"] == "" || entries["signature"] == "" {
		return nil, fmt.Errorf("memory commit %s tree must contain exactly memory.json and signature", safeDiagnostic(commit))
	}
	return entries, nil
}

func boundedMemoryBlobAt(gitDir, object string, limit int64, name, commit string) ([]byte, error) {
	sizeText, err := gitTextAt(gitDir, "cat-file", "-s", object)
	if err != nil {
		return nil, fmt.Errorf("read %s size at memory commit %s", name, safeDiagnostic(commit))
	}
	size, err := strconv.ParseInt(sizeText, 10, 64)
	if err != nil || size < 0 || size > limit {
		return nil, fmt.Errorf("%s at memory commit %s exceeds its bound", name, safeDiagnostic(commit))
	}
	payload, err := gitOutputAt(gitDir, "cat-file", "blob", object)
	if err != nil || int64(len(payload)) != size {
		return nil, fmt.Errorf("read %s at memory commit %s", name, safeDiagnostic(commit))
	}
	return payload, nil
}

func memoryCommitParentsAt(gitDir, commit string) ([]string, error) {
	text, err := gitTextAt(gitDir, "cat-file", "-p", commit)
	if err != nil {
		return nil, fmt.Errorf("memory commit %s is unavailable while reading parents: %w", safeDiagnostic(commit), err)
	}
	var parents []string
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "parent ") {
			parent := strings.TrimPrefix(line, "parent ")
			if parent == "" || strings.Contains(parent, " ") {
				return nil, fmt.Errorf("memory commit %s has invalid parent header", safeDiagnostic(commit))
			}
			parents = append(parents, parent)
		}
	}
	return parents, nil
}

func loadMemoryStreamAt(gitDir string, source memoryStreamSource) ([]StoredMemory, error) {
	if err := validateMemoryStreamSource(source); err != nil {
		return nil, err
	}
	if source.Head == "" {
		return nil, fmt.Errorf("memory source %s has no head", safeDiagnostic(source.Ref))
	}
	mainGitDir := gitDir
	if gitDir == "" {
		var err error
		mainGitDir, err = requireGitRepository()
		if err != nil {
			return nil, err
		}
	}

	commits := make([]string, 0)
	for commit := source.Head; ; {
		if gitDir == "" {
			if err := replicationPendingError(mainGitDir, commit); err != nil {
				return nil, err
			}
		}
		parents, err := memoryCommitParentsAt(gitDir, commit)
		if err != nil {
			return nil, fmt.Errorf("memory source %s: %w", safeDiagnostic(source.Ref), err)
		}
		if len(parents) > 1 {
			return nil, fmt.Errorf("memory source %s commit %s has %d parents; exactly one is allowed", safeDiagnostic(source.Ref), safeDiagnostic(commit), len(parents))
		}
		commits = append(commits, commit)
		if len(parents) == 0 {
			break
		}
		commit = parents[0]
	}
	for left, right := 0, len(commits)-1; left < right; left, right = left+1, right-1 {
		commits[left], commits[right] = commits[right], commits[left]
	}

	memories := make([]StoredMemory, 0, len(commits))
	for index, commit := range commits {
		stored, err := loadStoredMemoryAt(gitDir, commit)
		if err != nil {
			return nil, fmt.Errorf("memory source %s: %w", safeDiagnostic(source.Ref), err)
		}
		wantSequence := uint64(index + 1)
		if stored.Envelope.Actor != source.Actor {
			return nil, fmt.Errorf("memory source %s commit %s has owner %s, want %s", safeDiagnostic(source.Ref), safeDiagnostic(commit), safeDiagnostic(stored.Envelope.Actor), safeDiagnostic(source.Actor))
		}
		if stored.Envelope.Stream != source.Stream {
			return nil, fmt.Errorf("memory source %s commit %s has stream %s, want %s", safeDiagnostic(source.Ref), safeDiagnostic(commit), safeDiagnostic(stored.Envelope.Stream), safeDiagnostic(source.Stream))
		}
		if stored.Envelope.Sequence != wantSequence {
			return nil, fmt.Errorf("memory source %s commit %s has sequence %d, want %d", safeDiagnostic(source.Ref), safeDiagnostic(commit), stored.Envelope.Sequence, wantSequence)
		}
		if index == 0 {
			if stored.Envelope.Previous != "" {
				return nil, fmt.Errorf("memory source %s sequence 1 has a previous memory", safeDiagnostic(source.Ref))
			}
		} else if stored.Envelope.Previous != memories[index-1].ID {
			return nil, fmt.Errorf("memory source %s sequence %d previous memory does not match its predecessor", safeDiagnostic(source.Ref), wantSequence)
		}
		memories = append(memories, *stored)
	}
	return memories, nil
}

func validateMemoryStreamSource(source memoryStreamSource) error {
	if actor, stream, ok := parseMemoryRef(source.Ref); ok {
		if source.Accepted || source.Remote != "" || source.Actor != actor || source.Stream != stream {
			return fmt.Errorf("memory source %s metadata does not match its local ref", safeDiagnostic(source.Ref))
		}
		return nil
	}
	remote, actor, stream, ok := parseAcceptedMemoryRef(source.Ref)
	if !ok || !source.Accepted || source.Remote != remote || source.Actor != actor || source.Stream != stream {
		return fmt.Errorf("memory source %s is not canonical", safeDiagnostic(source.Ref))
	}
	return nil
}

func collectMemories() ([]StoredMemory, error) {
	text, err := gitText("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/memory", "refs/hn/remotes")
	if err != nil {
		return nil, err
	}
	if text == "" {
		return nil, nil
	}
	sources := make([]memoryStreamSource, 0)
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid memory ref advertisement")
		}
		ref, head := fields[0], fields[1]
		if strings.HasPrefix(ref, memoryRefPrefix) {
			actor, stream, ok := parseMemoryRef(ref)
			if !ok {
				return nil, fmt.Errorf("malformed memory ref %s", safeDiagnostic(ref))
			}
			sources = append(sources, memoryStreamSource{Ref: ref, Actor: actor, Stream: stream, Head: head})
			continue
		}
		if !strings.HasPrefix(ref, acceptedMemoryRefPrefix) {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(ref, acceptedMemoryRefPrefix), "/")
		if len(parts) < 2 || parts[1] != "memory" {
			continue
		}
		remote, actor, stream, ok := parseAcceptedMemoryRef(ref)
		if !ok {
			return nil, fmt.Errorf("malformed accepted memory ref %s", safeDiagnostic(ref))
		}
		sources = append(sources, memoryStreamSource{Ref: ref, Remote: remote, Actor: actor, Stream: stream, Head: head, Accepted: true})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Ref < sources[j].Ref })

	byID := make(map[string]StoredMemory)
	for _, source := range sources {
		stream, err := loadMemoryStreamAt("", source)
		if err != nil {
			return nil, err
		}
		for _, stored := range stream {
			prior, exists := byID[stored.ID]
			if !exists || stored.Commit < prior.Commit {
				byID[stored.ID] = stored
			}
		}
	}
	memories := make([]StoredMemory, 0, len(byID))
	for _, stored := range byID {
		memories = append(memories, stored)
	}
	sort.Slice(memories, func(i, j int) bool {
		left, right := memories[i].Envelope, memories[j].Envelope
		if left.Actor != right.Actor {
			return left.Actor < right.Actor
		}
		if left.Stream != right.Stream {
			return left.Stream < right.Stream
		}
		if left.Sequence != right.Sequence {
			return left.Sequence < right.Sequence
		}
		return memories[i].ID < memories[j].ID
	})
	return memories, nil
}
