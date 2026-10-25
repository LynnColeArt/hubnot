package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ShallowDependencyKind string

const (
	shallowActorPredecessor   ShallowDependencyKind = "actor predecessor"
	shallowCandidateEvent     ShallowDependencyKind = "candidate event"
	shallowProposalCodeRef    ShallowDependencyKind = "proposal code ref"
	shallowBaseCommit         ShallowDependencyKind = "base commit"
	shallowPolicyBlob         ShallowDependencyKind = "policy blob"
	shallowPipelineDefinition ShallowDependencyKind = "pipeline definition"
	shallowRunRequest         ShallowDependencyKind = "run request"
	shallowRunResult          ShallowDependencyKind = "run result"
	shallowDecision           ShallowDependencyKind = "decision"
	shallowSelectedFact       ShallowDependencyKind = "selected event fact"
	shallowMergeAncestor      ShallowDependencyKind = "merge ancestor"
	shallowMemoryPredecessor  ShallowDependencyKind = "memory stream predecessor"
	shallowMemoryLifecycle    ShallowDependencyKind = "memory lifecycle target"
	shallowMemoryAnchor       ShallowDependencyKind = "memory anchor"
	shallowMemoryEvidence     ShallowDependencyKind = "memory evidence"
	shallowMemoryStream       ShallowDependencyKind = "supplying memory stream"
)

// ShallowDependencyGap is an immutable dependency that cannot be proven from
// the accepted object/fact set because the repository has a shallow boundary.
// Cause is retained for errors.Is/errors.As without exposing Git paths or
// credentials in the stable operator diagnostic.
type ShallowDependencyGap struct {
	Operation         string                `json:"operation"`
	Kind              ShallowDependencyKind `json:"kind"`
	MissingID         string                `json:"missingID"`
	Objectish         string                `json:"objectish,omitempty"`
	ObjectType        string                `json:"objectType,omitempty"`
	OwnerKind         string                `json:"ownerKind,omitempty"`
	OwnerID           string                `json:"ownerID,omitempty"`
	OwnerMemoryID     string                `json:"ownerMemoryID,omitempty"`
	OwnerStream       string                `json:"ownerStream,omitempty"`
	Remote            string                `json:"remote,omitempty"`
	RequiredRef       string                `json:"requiredRef,omitempty"`
	RequiredSelectors []string              `json:"requiredSelectors,omitempty"`
	Recovery          string                `json:"recovery"`
	Cause             error                 `json:"-"`
}

func (gap *ShallowDependencyGap) Error() string {
	parts := []string{
		"shallow dependency gap",
		"operation=" + safeDiagnostic(gap.Operation),
		"kind=" + string(gap.Kind),
		"missing=" + safeDiagnostic(gap.MissingID),
	}
	if gap.OwnerKind != "" && gap.OwnerID != "" {
		parts = append(parts, "owner="+safeDiagnostic(gap.OwnerKind)+":"+safeDiagnostic(gap.OwnerID))
	}
	if gap.OwnerMemoryID != "" {
		parts = append(parts, "memory="+safeDiagnostic(gap.OwnerMemoryID))
	}
	if gap.OwnerStream != "" {
		parts = append(parts, "stream="+safeDiagnostic(gap.OwnerStream))
	}
	if gap.Remote != "" {
		parts = append(parts, "remote="+safeDiagnostic(gap.Remote))
	}
	if gap.RequiredRef != "" {
		parts = append(parts, "ref="+safeDiagnostic(gap.RequiredRef))
	}
	if gap.Recovery != "" {
		parts = append(parts, "recovery="+safeDiagnostic(gap.Recovery))
	}
	return strings.Join(parts, "; ")
}

func (gap *ShallowDependencyGap) Unwrap() error { return gap.Cause }

type exactDependency struct {
	Operation     string
	Kind          ShallowDependencyKind
	MissingID     string
	Objectish     string
	ObjectType    string
	OwnerKind     string
	OwnerID       string
	OwnerMemoryID string
	OwnerStream   string
	Remote        string
	RequiredRef   string
}

type gitObjectProbe struct {
	Exists bool
	Type   string
}

type ReplicationAcceptancePendingError struct {
	Transaction string
	Remote      string
	ObjectID    string
	Cause       error
}

func (pending *ReplicationAcceptancePendingError) Error() string {
	parts := []string{"replication acceptance pending", "transaction=" + safeDiagnostic(pending.Transaction)}
	if pending.Remote != "" {
		parts = append(parts, "remote="+safeDiagnostic(pending.Remote))
	}
	if pending.ObjectID != "" {
		parts = append(parts, "object="+safeDiagnostic(pending.ObjectID))
	}
	if pending.Cause != nil {
		parts = append(parts, "state=invalid")
	}
	if pending.Remote != "" {
		parts = append(parts, "recovery=retry hn sync "+safeDiagnostic(pending.Remote)+" --recover-shallow")
	} else {
		parts = append(parts, "recovery=repair the local replication transaction record before retrying sync")
	}
	return strings.Join(parts, "; ")
}

func (pending *ReplicationAcceptancePendingError) Unwrap() error { return pending.Cause }

var shallowObjectProbe = probeExactGitObject

func probeExactGitObject(objectish string) (gitObjectProbe, error) {
	if validFullGitObjectID(objectish) {
		gitDir, err := requireGitRepository()
		if err != nil {
			return gitObjectProbe{}, err
		}
		unaccepted, err := replicationObjectIsUnaccepted(gitDir, objectish)
		if err != nil {
			return gitObjectProbe{}, err
		}
		if unaccepted {
			return gitObjectProbe{}, nil
		}
	}
	return probeExactGitObjectAt("", objectish)
}

func probeExactGitObjectAt(gitDir, objectish string) (gitObjectProbe, error) {
	output, err := gitInputWithDirectory(gitDir, []byte(objectish+"\n"), nil, "cat-file", "--batch-check=%(objectname) %(objecttype)")
	if err != nil {
		return gitObjectProbe{}, err
	}
	fields := strings.Fields(strings.TrimSpace(string(output)))
	if len(fields) == 2 && fields[1] == "missing" {
		return gitObjectProbe{}, nil
	}
	if len(fields) != 2 {
		return gitObjectProbe{}, fmt.Errorf("Git returned invalid object probe for %s", safeDiagnostic(objectish))
	}
	return gitObjectProbe{Exists: true, Type: fields[1]}, nil
}

const replicationUnacceptedObjectsVersion = 1

type replicationUnacceptedObjects struct {
	Version int      `json:"version"`
	IDs     []string `json:"ids"`
}

func replicationUnacceptedObjectsPath(gitDir string) string {
	return filepath.Join(gitDir, "hn", "replication", "unaccepted-objects.json")
}

func loadReplicationUnacceptedObjects(gitDir string) (map[string]bool, error) {
	path := replicationUnacceptedObjectsPath(gitDir)
	encoded, err := readPrivateFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]bool), nil
	}
	if err != nil {
		return nil, err
	}
	var record replicationUnacceptedObjects
	if err := decodePrivateJSON(encoded, &record, "unaccepted replication objects"); err != nil {
		return nil, err
	}
	if record.Version != replicationUnacceptedObjectsVersion {
		return nil, fmt.Errorf("unsupported unaccepted replication object record")
	}
	objects := make(map[string]bool, len(record.IDs))
	for _, id := range record.IDs {
		if !validFullGitObjectID(id) {
			return nil, fmt.Errorf("invalid unaccepted replication object ID")
		}
		objects[id] = true
	}
	return objects, nil
}

func saveReplicationUnacceptedObjects(gitDir string, objects map[string]bool) error {
	directory := filepath.Dir(replicationUnacceptedObjectsPath(gitDir))
	if err := ensurePrivateDirectory(directory); err != nil {
		return err
	}
	ids := make([]string, 0, len(objects))
	for id := range objects {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	encoded, err := json.MarshalIndent(replicationUnacceptedObjects{Version: replicationUnacceptedObjectsVersion, IDs: ids}, "", "  ")
	if err != nil {
		return err
	}
	return writePrivateFileAtomic(replicationUnacceptedObjectsPath(gitDir), append(encoded, '\n'))
}

func markReplicationObjectsUnaccepted(gitDir string, objectIDs []string) error {
	objects, err := loadReplicationUnacceptedObjects(gitDir)
	if err != nil {
		return err
	}
	changed := false
	for _, id := range objectIDs {
		if objects[id] {
			continue
		}
		probe, err := probeExactGitObjectAt(gitDir, id)
		if err != nil {
			return err
		}
		if !probe.Exists {
			objects[id] = true
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return saveReplicationUnacceptedObjects(gitDir, objects)
}

func acceptReplicationObjects(gitDir string, objectIDs []string) error {
	objects, err := loadReplicationUnacceptedObjects(gitDir)
	if err != nil {
		return err
	}
	changed := false
	for _, id := range objectIDs {
		if objects[id] {
			delete(objects, id)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return saveReplicationUnacceptedObjects(gitDir, objects)
}

func replicationObjectIsUnaccepted(gitDir, objectID string) (bool, error) {
	objects, _, err := loadReplicationAcceptanceState(gitDir)
	if err != nil {
		return false, &ReplicationAcceptancePendingError{ObjectID: objectID, Cause: err}
	}
	return objects[objectID], nil
}

func replicationTransactionsPath(gitDir string) string {
	return filepath.Join(gitDir, "hn", "replication", "transactions")
}

func replicationAnchorsPath(gitDir string) string {
	return filepath.Join(gitDir, "hn", "replication", "anchors")
}

func ensureReplicationStateDirectory(gitDir, directory string) error {
	for _, path := range []string{
		filepath.Join(gitDir, "hn"),
		filepath.Join(gitDir, "hn", "replication"),
		directory,
	} {
		if err := ensurePrivateDirectory(path); err != nil {
			return err
		}
	}
	return nil
}

func readReplicationStateDirectory(gitDir, directory string) ([]os.DirEntry, bool, error) {
	for _, path := range []string{filepath.Join(gitDir, "hn"), filepath.Join(gitDir, "hn", "replication")} {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		if err := validatePrivateDirectory(path, info); err != nil {
			return nil, false, err
		}
	}
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := validatePrivateDirectory(directory, info); err != nil {
		return nil, false, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, false, err
	}
	return entries, true, nil
}

func validReplicationTransactionID(id string) bool {
	if !strings.HasPrefix(id, "txn-") || len(id) <= len("txn-") || len(id) > 128 {
		return false
	}
	for _, character := range id[len("txn-"):] {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validateReplicationObjectIDs(transaction string, ids []string) error {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !validFullGitObjectID(id) || seen[id] {
			return fmt.Errorf("replication transaction %s contains invalid or duplicate object ID", transaction)
		}
		seen[id] = true
	}
	return nil
}

func loadReplicationTransactionRecords(gitDir string) ([]replicationTransactionRecord, error) {
	directory := replicationTransactionsPath(gitDir)
	entries, exists, err := readReplicationStateDirectory(gitDir, directory)
	if !exists && err == nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read replication transaction directory: %w", err)
	}
	records := make([]replicationTransactionRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		encoded, err := readPrivateFileBounded(path, maxReplicationStateBytes)
		if err != nil {
			return nil, fmt.Errorf("read replication transaction %s: %w", entry.Name(), err)
		}
		var record replicationTransactionRecord
		if err := decodePrivateJSON(encoded, &record, "replication transaction "+entry.Name()); err != nil {
			return nil, err
		}
		if !validReplicationTransactionID(record.ID) || entry.Name() != record.ID+".json" || !validReplicationRemote(record.Remote) {
			return nil, fmt.Errorf("invalid replication transaction identity in %s", entry.Name())
		}
		if record.State != "validated" && record.State != "complete" {
			return nil, fmt.Errorf("invalid replication transaction state in %s", entry.Name())
		}
		if record.Version != 1 && record.Version != replicationTransactionRecordVersion {
			return nil, fmt.Errorf("unsupported replication transaction record in %s", entry.Name())
		}
		if record.Version == replicationTransactionRecordVersion {
			if record.PendingObjects == nil || record.AcceptedObjects == nil {
				return nil, fmt.Errorf("replication transaction %s is missing durable acceptance state", record.ID)
			}
			if err := validateReplicationObjectIDs(record.ID, *record.PendingObjects); err != nil {
				return nil, err
			}
			if err := validateReplicationObjectIDs(record.ID, *record.AcceptedObjects); err != nil {
				return nil, err
			}
			for _, promotion := range record.Promotions {
				_, _, actorOK := parseAcceptedActorRef(promotion.Ref)
				_, _, proposalOK := parseAcceptedProposalRef(promotion.Ref)
				_, _, _, memoryOK := parseAcceptedMemoryRef(promotion.Ref)
				if (!actorOK && !proposalOK && !memoryOK) || !validGitOID(promotion.NewOID) || (promotion.OldOID != "" && !validGitOID(promotion.OldOID)) {
					return nil, fmt.Errorf("replication transaction %s contains invalid promotion", record.ID)
				}
			}
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records, nil
}

func loadReplicationPendingAnchors(gitDir string) ([]replicationPendingAnchor, error) {
	directory := replicationAnchorsPath(gitDir)
	entries, exists, err := readReplicationStateDirectory(gitDir, directory)
	if !exists && err == nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read replication pending-anchor directory: %w", err)
	}
	anchors := make([]replicationPendingAnchor, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			return nil, fmt.Errorf("invalid replication pending-anchor entry %s", entry.Name())
		}
		path := filepath.Join(directory, entry.Name())
		encoded, err := readPrivateFileBounded(path, maxReplicationStateBytes)
		if err != nil {
			return nil, fmt.Errorf("read replication pending anchor %s: %w", entry.Name(), err)
		}
		var anchor replicationPendingAnchor
		if err := decodePrivateJSON(encoded, &anchor, "replication pending anchor "+entry.Name()); err != nil {
			return nil, err
		}
		if anchor.Version != replicationPendingAnchorVersion || !validReplicationTransactionID(anchor.ID) ||
			entry.Name() != anchor.ID+".json" || !validReplicationRemote(anchor.Remote) || anchor.PendingObjects == nil {
			return nil, fmt.Errorf("invalid replication pending anchor %s", entry.Name())
		}
		if err := validateReplicationObjectIDs(anchor.ID, *anchor.PendingObjects); err != nil {
			return nil, err
		}
		anchors = append(anchors, anchor)
	}
	sort.Slice(anchors, func(i, j int) bool { return anchors[i].ID < anchors[j].ID })
	return anchors, nil
}

func loadReplicationAcceptanceState(gitDir string) (map[string]bool, map[string]replicationTransactionRecord, error) {
	records, err := loadReplicationTransactionRecords(gitDir)
	if err != nil {
		return nil, nil, err
	}
	anchors, err := loadReplicationPendingAnchors(gitDir)
	if err != nil {
		return nil, nil, err
	}
	recordsByID := make(map[string]replicationTransactionRecord, len(records))
	for _, record := range records {
		recordsByID[record.ID] = record
	}
	anchorsByID := make(map[string]replicationPendingAnchor, len(anchors))
	for _, anchor := range anchors {
		anchorsByID[anchor.ID] = anchor
		record, exists := recordsByID[anchor.ID]
		if !exists {
			return nil, nil, fmt.Errorf("replication pending anchor %s has no durable transaction receipt", anchor.ID)
		}
		if record.Version != replicationTransactionRecordVersion || record.Remote != anchor.Remote {
			return nil, nil, fmt.Errorf("replication pending anchor %s does not match its transaction receipt", anchor.ID)
		}
		if record.State == "complete" {
			accepted := make(map[string]bool, len(*record.AcceptedObjects))
			for _, id := range *record.AcceptedObjects {
				accepted[id] = true
			}
			for _, id := range *anchor.PendingObjects {
				if !accepted[id] {
					return nil, nil, fmt.Errorf("completed replication transaction %s does not accept every anchored object", anchor.ID)
				}
			}
		}
	}
	denied := make(map[string]bool)
	accepted := make(map[string]bool)
	pending := make(map[string]replicationTransactionRecord)
	legacyNeeded := false
	for _, record := range records {
		if record.Version == 1 {
			if record.State == "validated" {
				legacyNeeded = true
				pending[record.ID] = record
			}
			continue
		}
		if record.State == "validated" {
			anchor, exists := anchorsByID[record.ID]
			if !exists {
				return nil, nil, fmt.Errorf("validated replication transaction %s is missing its durable pending anchor", record.ID)
			}
			if !sameReplicationObjectSet(*record.PendingObjects, *anchor.PendingObjects) {
				return nil, nil, fmt.Errorf("validated replication transaction %s differs from its durable pending anchor", record.ID)
			}
			pending[record.ID] = record
			for _, id := range *anchor.PendingObjects {
				denied[id] = true
			}
			continue
		}
		for _, id := range *record.AcceptedObjects {
			accepted[id] = true
		}
	}
	if legacyNeeded {
		legacy, err := loadReplicationUnacceptedObjects(gitDir)
		if err != nil {
			return nil, nil, err
		}
		if len(legacy) == 0 {
			return nil, nil, fmt.Errorf("validated legacy replication transaction is missing durable unaccepted-object state")
		}
		for id := range legacy {
			denied[id] = true
		}
	}
	for id := range accepted {
		delete(denied, id)
	}
	return denied, pending, nil
}

func sameReplicationObjectSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	values := make(map[string]bool, len(left))
	for _, id := range left {
		values[id] = true
	}
	for _, id := range right {
		if !values[id] {
			return false
		}
	}
	return true
}

func createReplicationPendingAnchor(gitDir string, result replicationTransactionResult) error {
	if !validReplicationTransactionID(result.ID) || !validReplicationRemote(result.Remote) {
		return fmt.Errorf("invalid replication pending-anchor identity")
	}
	pending := sortedUniqueStrings(result.pendingObjects...)
	if err := validateReplicationObjectIDs(result.ID, pending); err != nil {
		return err
	}
	directory := replicationAnchorsPath(gitDir)
	if err := ensureReplicationStateDirectory(gitDir, directory); err != nil {
		return err
	}
	anchor := replicationPendingAnchor{Version: replicationPendingAnchorVersion, ID: result.ID, Remote: result.Remote, PendingObjects: &pending}
	encoded, err := json.MarshalIndent(anchor, "", "  ")
	if err != nil {
		return err
	}
	if len(encoded)+1 > maxReplicationStateBytes {
		return fmt.Errorf("replication pending anchor %s is too large", result.ID)
	}
	return writePrivateFileAtomic(filepath.Join(directory, result.ID+".json"), append(encoded, '\n'))
}

func removeReplicationPendingAnchor(gitDir, transaction string) error {
	if !validReplicationTransactionID(transaction) {
		return fmt.Errorf("invalid replication pending-anchor identity")
	}
	directory := replicationAnchorsPath(gitDir)
	if _, exists, err := readReplicationStateDirectory(gitDir, directory); err != nil {
		return err
	} else if !exists {
		return nil
	}
	path := filepath.Join(directory, transaction+".json")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncPrivateDirectory(directory)
}

// A reader is scoped to one immutable-ref read, never cached across commands
// or transactions. Re-reading the complete journal for every event turns normal
// history traversal into journal-size times event-count work.
type replicationAcceptanceReader struct {
	denied  map[string]bool
	pending map[string]replicationTransactionRecord
}

func newReplicationAcceptanceReader(gitDir string) (replicationAcceptanceReader, error) {
	denied, pending, err := loadReplicationAcceptanceState(gitDir)
	if err != nil {
		return replicationAcceptanceReader{}, &ReplicationAcceptancePendingError{Cause: err}
	}
	return replicationAcceptanceReader{denied: denied, pending: pending}, nil
}

func replicationPendingError(gitDir, objectID string) error {
	reader, err := newReplicationAcceptanceReader(gitDir)
	if err != nil {
		return err
	}
	return reader.check(objectID)
}

func (reader replicationAcceptanceReader) check(objectID string) error {
	if !reader.denied[objectID] {
		return nil
	}
	for _, record := range reader.pending {
		if record.PendingObjects == nil {
			continue
		}
		for _, id := range *record.PendingObjects {
			if id == objectID {
				return &ReplicationAcceptancePendingError{Transaction: record.ID, Remote: record.Remote, ObjectID: objectID}
			}
		}
	}
	return &ReplicationAcceptancePendingError{ObjectID: objectID}
}

func reconcileCompletedReplicationTransactions(gitDir string, completed replicationTransactionResult) error {
	anchors, err := loadReplicationPendingAnchors(gitDir)
	if err != nil {
		return err
	}
	accepted := make(map[string]bool, len(completed.acceptedObjects))
	for _, id := range completed.acceptedObjects {
		accepted[id] = true
	}
	for _, anchor := range anchors {
		if anchor.Remote != completed.Remote || anchor.ID == completed.ID {
			continue
		}
		covered := true
		for _, id := range *anchor.PendingObjects {
			if !accepted[id] {
				covered = false
				break
			}
		}
		if !covered {
			continue
		}
		result := replicationTransactionResult{ID: anchor.ID, Remote: anchor.Remote,
			pendingObjects: append([]string(nil), (*anchor.PendingObjects)...), acceptedObjects: append([]string(nil), (*anchor.PendingObjects)...)}
		if err := recordReplicationTransaction(gitDir, result, "complete"); err != nil {
			return err
		}
		if err := removeReplicationPendingAnchor(gitDir, anchor.ID); err != nil {
			return err
		}
	}
	return nil
}

func repairAnchoredReplicationReceiptsForRecovery(gitDir, remote string) error {
	anchors, err := loadReplicationPendingAnchors(gitDir)
	if err != nil {
		return err
	}
	for _, anchor := range anchors {
		if anchor.Remote != remote {
			continue
		}
		result := replicationTransactionResult{
			ID: anchor.ID, Remote: anchor.Remote,
			pendingObjects:  append([]string(nil), (*anchor.PendingObjects)...),
			acceptedObjects: append([]string(nil), (*anchor.PendingObjects)...),
		}
		if err := recordReplicationTransaction(gitDir, result, "validated"); err != nil {
			return fmt.Errorf("repair replication transaction %s from its durable pending anchor: %w", anchor.ID, err)
		}
	}
	return nil
}

func repositoryIsShallow() (bool, error) {
	value, err := gitText("rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, err
	}
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("Git returned invalid shallow-repository state %q", value)
	}
}

func requireExactDependency(dependency exactDependency) error {
	objectish := dependency.Objectish
	if objectish == "" {
		objectish = dependency.MissingID
	}
	objectType := dependency.ObjectType
	if objectType == "" {
		objectType = "object"
	}
	probe, err := shallowObjectProbe(objectish)
	if err != nil {
		return fmt.Errorf("inspect exact %s %s: %w", dependency.Kind, dependency.MissingID, err)
	}
	if probe.Exists {
		if objectType != "object" && probe.Type != objectType {
			return fmt.Errorf("required exact %s %s has Git object type %s, want %s", dependency.Kind, dependency.MissingID, probe.Type, objectType)
		}
		return nil
	}
	cause := fmt.Errorf("required exact %s %s is unavailable", dependency.Kind, dependency.MissingID)
	return classifyShallowDependency(dependency, cause)
}

func classifyShallowDependency(dependency exactDependency, cause error) error {
	shallow, err := repositoryIsShallow()
	if err != nil {
		return fmt.Errorf("inspect shallow boundary: %w", err)
	}
	if !shallow {
		return cause
	}
	remote, ref, recovery, err := shallowRecoveryGuidance(dependency)
	if err != nil {
		return fmt.Errorf("inspect saved replication selection: %w", err)
	}
	gap := &ShallowDependencyGap{
		Operation:     dependency.Operation,
		Kind:          dependency.Kind,
		MissingID:     dependency.MissingID,
		Objectish:     dependency.Objectish,
		ObjectType:    dependency.ObjectType,
		OwnerKind:     dependency.OwnerKind,
		OwnerID:       dependency.OwnerID,
		OwnerMemoryID: dependency.OwnerMemoryID,
		OwnerStream:   dependency.OwnerStream,
		Remote:        remote,
		RequiredRef:   ref,
		Recovery:      recovery,
		Cause:         cause,
	}
	if err := recordShallowDependencyGap(gap); err != nil {
		return fmt.Errorf("record shallow dependency gap: %w", err)
	}
	return gap
}

func memoryShallowDependencyKind(kind string) ShallowDependencyKind {
	switch kind {
	case "lifecycle-target":
		return shallowMemoryLifecycle
	case "anchor-commit", "anchor-path", "query-commit", "anchor-ancestry":
		return shallowMemoryAnchor
	case "stream-predecessor":
		return shallowMemoryPredecessor
	case "supplying-stream":
		return shallowMemoryStream
	default:
		return shallowMemoryEvidence
	}
}

const shallowGapRecordVersion = 2

type shallowVerificationScope struct {
	Operation    string `json:"operation"`
	Subject      string `json:"subject,omitempty"`
	Base         string `json:"base,omitempty"`
	Head         string `json:"head,omitempty"`
	Current      string `json:"current,omitempty"`
	Pipeline     string `json:"pipeline,omitempty"`
	ProposedFile string `json:"proposedFile,omitempty"`
}

var activeShallowVerificationScope *shallowVerificationScope

type shallowGapRecord struct {
	Version int                       `json:"version"`
	Gap     *ShallowDependencyGap     `json:"gap"`
	Scope   *shallowVerificationScope `json:"scope,omitempty"`
}

func shallowGapPath() (string, error) {
	gitDir, err := requireGitRepository()
	if err != nil {
		return "", err
	}
	root := filepath.Join(gitDir, "hn")
	if err := ensurePrivateDirectory(root); err != nil {
		return "", err
	}
	return filepath.Join(root, "shallow-gap.json"), nil
}

func recordShallowDependencyGap(gap *ShallowDependencyGap) error {
	path, err := shallowGapPath()
	if err != nil {
		return err
	}
	record := shallowGapRecord{Version: shallowGapRecordVersion, Gap: gap}
	if activeShallowVerificationScope != nil {
		scope := *activeShallowVerificationScope
		record.Scope = &scope
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return writePrivateFileAtomic(path, append(encoded, '\n'))
}

func loadShallowDependencyGap() (*ShallowDependencyGap, error) {
	record, err := loadShallowGapRecord()
	if err != nil {
		return nil, err
	}
	return record.Gap, nil
}

func loadShallowGapRecord() (*shallowGapRecord, error) {
	path, err := shallowGapPath()
	if err != nil {
		return nil, err
	}
	encoded, err := readPrivateFile(path)
	if err != nil {
		return nil, err
	}
	var record shallowGapRecord
	if err := decodePrivateJSON(encoded, &record, "shallow dependency gap"); err != nil {
		return nil, err
	}
	if record.Version != shallowGapRecordVersion || record.Gap == nil {
		return nil, fmt.Errorf("unsupported shallow dependency gap record")
	}
	return &record, nil
}

func shallowRecoveryGuidance(dependency exactDependency) (string, string, string, error) {
	remote := dependency.Remote
	ref := dependency.RequiredRef
	if ref == "" {
		switch dependency.OwnerKind {
		case replicationActor:
			ref = actorRef(dependency.OwnerID)
		case replicationProposal:
			ref = proposalRef(dependency.OwnerID)
		case replicationMemory:
			// Memory refs bind both owner and stream. Callers that know the
			// owner must supply RequiredRef; never guess an owner here.
		}
	}
	selected := false
	if dependency.OwnerID != "" {
		var err error
		remote, selected, err = selectedRemoteFor(dependency.OwnerKind, dependency.OwnerID, remote)
		if err != nil {
			return "", "", "", err
		}
	}
	if remote == "" {
		remote = "origin"
	}
	if selected {
		return remote, ref, "hn sync " + remote + " --recover-shallow", nil
	}
	switch dependency.OwnerKind {
	case replicationActor:
		return remote, ref, "hn replication select " + remote + " --actor " + dependency.OwnerID + " (preserve existing selectors and budgets), then hn sync " + remote + " --recover-shallow", nil
	case replicationProposal:
		return remote, ref, "hn replication select " + remote + " --proposal " + dependency.OwnerID + " (preserve existing selectors and budgets), then hn sync " + remote + " --recover-shallow", nil
	case replicationMemory:
		return remote, ref, "hn replication select " + remote + " --memory " + dependency.OwnerID + " (preserve existing selectors and budgets), then hn sync " + remote + " --recover-shallow", nil
	default:
		switch dependency.Kind {
		case shallowCandidateEvent, shallowRunRequest, shallowRunResult, shallowDecision, shallowSelectedFact, shallowActorPredecessor:
			return remote, ref, "select the actor history supplying full event ID " + dependency.MissingID + ", then retry", nil
		default:
			return remote, ref, "select the candidate ref supplying full Git object ID " + dependency.MissingID + ", then retry", nil
		}
	}
}

func selectedRemoteFor(kind, id, preferred string) (string, bool, error) {
	remoteText, err := gitText("remote")
	if err != nil {
		return "", false, err
	}
	remotes := strings.Fields(remoteText)
	sort.Strings(remotes)
	if preferred != "" {
		remotes = append([]string{preferred}, remotes...)
		remotes = sortedUniqueStrings(remotes...)
	}
	for _, remote := range remotes {
		selection, explicit, err := loadReplicationSelection(remote)
		if err != nil {
			return "", false, err
		}
		if !explicit || selection.All {
			continue
		}
		values := selection.Actors
		if kind == replicationProposal {
			values = selection.Proposals
		} else if kind == replicationMemory {
			values = selection.Memories
		}
		for _, value := range values {
			if value == id {
				return remote, true, nil
			}
		}
	}
	return preferred, false, nil
}

type shallowEventFact struct {
	Stored StoredEvent
	Remote string
	Actor  string
	Ref    string
}

// guardShallowEventClosure validates only the facts already reachable from
// accepted refs. Complete repositories keep their existing validation path.
func guardShallowEventClosure(operation string) error {
	shallow, err := repositoryIsShallow()
	if err != nil || !shallow {
		return err
	}
	facts, err := shallowAcceptedFacts(operation)
	if err != nil {
		return err
	}
	byID := make(map[string]shallowEventFact, len(facts))
	for _, fact := range facts {
		byID[fact.Stored.ID] = fact
	}
	for _, fact := range facts {
		event := fact.Stored.Event
		missing := func(id string, kind ShallowDependencyKind, ownerKind, ownerID string) error {
			if id == "" {
				return nil
			}
			if _, exists := byID[id]; exists {
				return nil
			}
			return classifyShallowDependency(exactDependency{
				Operation: operation, Kind: kind, MissingID: id,
				OwnerKind: ownerKind, OwnerID: ownerID,
			}, fmt.Errorf("signed event %s (%s) requires unavailable fact %s", fact.Stored.ID, event.Kind, id))
		}
		for _, reference := range resourceReferences(event) {
			if err := missing(reference, shallowSelectedFact, "", ""); err != nil {
				return err
			}
		}
		switch event.Kind {
		case "proposal.revise", "review.submit":
			if err := missing(event.Subject, shallowCandidateEvent, replicationProposal, event.Subject); err != nil {
				return err
			}
		case "run.request":
			if err := missing(event.Subject, shallowCandidateEvent, replicationProposal, event.Subject); err != nil {
				return err
			}
		case "run.result":
			if err := missing(event.Subject, shallowRunRequest, "", ""); err != nil {
				return err
			}
		case "proposal.decision":
			if err := missing(event.Subject, shallowCandidateEvent, replicationProposal, event.Subject); err != nil {
				return err
			}
			for _, evidence := range event.Evidence {
				if err := missing(evidence, shallowSelectedFact, "", ""); err != nil {
					return err
				}
			}
		case "proposal.merged":
			if err := missing(event.Subject, shallowCandidateEvent, replicationProposal, event.Subject); err != nil {
				return err
			}
			for _, evidence := range event.Evidence {
				if err := missing(evidence, shallowDecision, "", ""); err != nil {
					return err
				}
			}
		case "identity.accept":
			if err := missing(event.Subject, shallowActorPredecessor, replicationActor, event.Actor); err != nil {
				return err
			}
		}
	}
	return nil
}

func shallowAcceptedFacts(operation string) ([]shallowEventFact, error) {
	refText, err := gitText("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors", "refs/hn/remotes")
	if err != nil {
		return nil, err
	}
	facts := make([]shallowEventFact, 0)
	fields := strings.Fields(refText)
	for index := 0; index+1 < len(fields); index += 2 {
		ref, head := fields[index], fields[index+1]
		actor, remote, accepted := shallowActorRefOwner(ref)
		if actor == "" {
			continue
		}
		chainText, chainErr := gitText("rev-list", "--reverse", head)
		if chainErr != nil {
			probe, probeErr := shallowObjectProbe(head)
			if probeErr != nil {
				return nil, probeErr
			}
			if probe.Exists {
				return nil, chainErr
			}
			return nil, classifyShallowDependency(exactDependency{
				Operation: operation, Kind: shallowActorPredecessor, MissingID: head,
				OwnerKind: replicationActor, OwnerID: actor, Remote: remote, RequiredRef: actorRef(actor),
			}, fmt.Errorf("accepted actor head is unavailable"))
		}
		commits := strings.Fields(chainText)
		for _, commit := range commits {
			stored, loadErr := loadStoredEventAt("", commit)
			if loadErr != nil {
				// rev-list proved the commit exists. Decode/signature/tree failures
				// are present-invalid data, never recoverable shallow omission.
				return nil, loadErr
			}
			facts = append(facts, shallowEventFact{Stored: *stored, Remote: remote, Actor: actor, Ref: ref})
		}
		if len(commits) == 0 {
			continue
		}
		first := facts[len(facts)-len(commits)].Stored
		if first.Event.Sequence > 1 && first.Event.Previous != "" {
			dependency := exactDependency{
				Operation: operation, Kind: shallowActorPredecessor, MissingID: first.Event.Previous,
				OwnerKind: replicationActor, OwnerID: actor, Remote: remote, RequiredRef: actorRef(actor),
			}
			if !accepted {
				dependency.Remote = ""
			}
			return nil, classifyShallowDependency(dependency, fmt.Errorf("signed actor chain begins after its required predecessor"))
		}
		gitDir, err := requireGitRepository()
		if err != nil {
			return nil, err
		}
		for _, commit := range commits {
			if err := replicationPendingError(gitDir, commit); err != nil {
				return nil, err
			}
		}
	}
	return facts, nil
}

// collectAcceptedEventsForReplication permits WP04 to replace a depth-limited
// accepted actor chain only when that exact actor is already selected. The
// incomplete chain is never treated as evidence; it is omitted and the full
// quarantined actor chain must replace it before promotion.
func collectAcceptedEventsForReplication(selection ReplicationSelection) ([]StoredEvent, error) {
	shallow, err := repositoryIsShallow()
	if err != nil {
		return nil, err
	}
	selectedActors := make(map[string]bool, len(selection.Actors))
	for _, actor := range selection.Actors {
		selectedActors[actor] = true
	}
	refText, err := gitText("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors", "refs/hn/remotes")
	if err != nil {
		return nil, err
	}
	gitDir, err := requireGitRepository()
	if err != nil {
		return nil, err
	}
	acceptance, err := newReplicationAcceptanceReader(gitDir)
	if err != nil {
		return nil, err
	}
	verified := make(map[string]*StoredEvent)
	byID := make(map[string]StoredEvent)
	fields := strings.Fields(refText)
	for index := 0; index+1 < len(fields); index += 2 {
		ref, head := fields[index], fields[index+1]
		actor, remote, _ := shallowActorRefOwner(ref)
		if actor == "" {
			continue
		}
		if acceptance.denied[head] && selectedActors[actor] {
			continue
		}
		chainText, err := gitText("rev-list", "--reverse", head)
		if err != nil {
			return nil, classifyShallowDependency(exactDependency{
				Operation: "selected shallow recovery", Kind: shallowActorPredecessor, MissingID: head,
				OwnerKind: replicationActor, OwnerID: actor, Remote: remote, RequiredRef: actorRef(actor),
			}, fmt.Errorf("accepted actor head is unavailable"))
		}
		commits := strings.Fields(chainText)
		selectedPending := false
		if selectedActors[actor] {
			for _, commit := range commits {
				if acceptance.denied[commit] {
					selectedPending = true
					break
				}
			}
		}
		if selectedPending {
			continue
		}
		chain := make([]StoredEvent, 0, len(commits))
		for _, commit := range commits {
			if err := acceptance.check(commit); err != nil {
				return nil, err
			}
			stored := verified[commit]
			if stored == nil {
				stored, err = loadStoredEventAt("", commit)
				if err != nil {
					return nil, err
				}
				verified[commit] = stored
			}
			chain = append(chain, *stored)
		}
		if shallow && len(chain) > 0 && chain[0].Event.Sequence > 1 {
			if selectedActors[actor] {
				continue
			}
			return nil, classifyShallowDependency(exactDependency{
				Operation: "selected shallow recovery", Kind: shallowActorPredecessor, MissingID: chain[0].Event.Previous,
				OwnerKind: replicationActor, OwnerID: actor, Remote: remote, RequiredRef: actorRef(actor),
			}, fmt.Errorf("accepted actor chain remains depth-limited and is not selected for recovery"))
		}
		for _, stored := range chain {
			byID[stored.ID] = stored
		}
	}
	events := mapStoredEvents(byID)
	if err := validateActorChains(events); err != nil {
		return nil, err
	}
	return events, nil
}

func releaseRecoveredShallowBoundaries(gitDir string, promotions []replicationPromotion) error {
	selectedBoundaries := make(map[string]bool)
	selectedActors := make(map[string]bool)
	for _, promotion := range promotions {
		_, actor, actorRef := parseAcceptedActorRef(promotion.Ref)
		if actorRef {
			selectedActors[actor] = true
			if promotion.OldOID != "" {
				selectedBoundaries[promotion.OldOID] = true
			}
			continue
		}
		if _, _, proposalRef := parseAcceptedProposalRef(promotion.Ref); proposalRef {
			if promotion.OldOID != "" {
				selectedBoundaries[promotion.OldOID] = true
			}
			selectedBoundaries[promotion.NewOID] = true
			continue
		}
		if _, _, _, memoryRef := parseAcceptedMemoryRef(promotion.Ref); memoryRef {
			if promotion.OldOID != "" {
				selectedBoundaries[promotion.OldOID] = true
			}
			selectedBoundaries[promotion.NewOID] = true
		}
	}
	if len(selectedActors) == 0 && len(selectedBoundaries) == 0 {
		return nil
	}
	pathText, err := gitTextAt(gitDir, "rev-parse", "--git-path", "shallow")
	if err != nil {
		return err
	}
	path := pathText
	if !filepath.IsAbs(path) {
		path = filepath.Join(gitDir, path)
	}
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("Git shallow boundary file is not a regular file")
	}
	kept := make([]string, 0)
	changed := false
	for _, boundary := range strings.Fields(string(contents)) {
		selected := selectedBoundaries[boundary]
		if !selected {
			if stored, err := loadStoredEventAt(gitDir, boundary); err == nil {
				selected = selectedActors[stored.Event.Actor]
			}
		}
		if !selected {
			kept = append(kept, boundary)
			continue
		}
		commit, err := gitTextAt(gitDir, "cat-file", "-p", boundary)
		if err != nil {
			return fmt.Errorf("verify recovered shallow boundary %s", boundary)
		}
		parentAvailable := false
		for _, line := range strings.Split(commit, "\n") {
			if !strings.HasPrefix(line, "parent ") {
				continue
			}
			parent := strings.TrimPrefix(line, "parent ")
			if _, err := gitOutputAt(gitDir, "cat-file", "-e", parent+"^{commit}"); err == nil {
				parentAvailable = true
				break
			}
		}
		if !parentAvailable {
			kept = append(kept, boundary)
			continue
		}
		primary, err := gitTextAt(gitDir, "for-each-ref", "--format=%(objectname)", "refs/heads")
		if err != nil {
			return err
		}
		primaryContainsBoundary := false
		for _, head := range strings.Fields(primary) {
			if _, err := gitOutputAt(gitDir, "merge-base", "--is-ancestor", boundary, head); err == nil {
				primaryContainsBoundary = true
				break
			}
		}
		if primaryContainsBoundary {
			kept = append(kept, boundary)
			continue
		}
		changed = true
	}
	if !changed {
		return nil
	}
	updated := []byte(strings.Join(kept, "\n"))
	if len(updated) > 0 {
		updated = append(updated, '\n')
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".hn-shallow-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(updated); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

func validateExactEventReferenceClosure(events []StoredEvent) error {
	byID := make(map[string]bool, len(events))
	for _, stored := range events {
		byID[stored.ID] = true
	}
	for _, stored := range events {
		for _, reference := range replicationEventReferences(stored.Event) {
			if !byID[reference] {
				return fmt.Errorf("event %s (%s) requires unavailable exact fact %s", stored.ID, stored.Event.Kind, reference)
			}
		}
	}
	return nil
}

func shallowActorRefOwner(ref string) (actor, remote string, accepted bool) {
	if strings.HasPrefix(ref, "refs/hn/actors/") {
		actor = strings.TrimPrefix(ref, "refs/hn/actors/")
		if validActorFingerprint(actor) {
			return actor, "", false
		}
		return "", "", false
	}
	remote, actor, accepted = parseAcceptedActorRef(ref)
	return actor, remote, accepted
}

func guardProposalDependencies(operation string, proposal *StoredEvent, events []StoredEvent) error {
	baseKind := shallowBaseCommit
	if operation == "proposal merge" {
		baseKind = shallowMergeAncestor
	}
	if err := requireExactDependency(exactDependency{
		Operation: operation, Kind: baseKind, MissingID: proposal.Event.Base,
		ObjectType: "commit", OwnerKind: replicationProposal, OwnerID: proposal.ID,
	}); err != nil {
		return err
	}
	if err := requireExactDependency(exactDependency{
		Operation: operation, Kind: shallowProposalCodeRef, MissingID: proposal.Event.Head,
		ObjectType: "commit", OwnerKind: replicationProposal, OwnerID: proposal.ID,
	}); err != nil {
		return err
	}
	head, exists, err := proposalHead(proposal.ID)
	if err != nil {
		return err
	}
	if !exists {
		return classifyShallowDependency(exactDependency{
			Operation: operation, Kind: shallowProposalCodeRef, MissingID: proposal.Event.Head,
			OwnerKind: replicationProposal, OwnerID: proposal.ID, RequiredRef: proposalRef(proposal.ID),
		}, fmt.Errorf("proposal code ref is unavailable"))
	}
	if head != proposal.Event.Head {
		return fmt.Errorf("proposal code ref does not match the signed proposal")
	}
	_ = events // events are intentionally supplied by the caller's fresh accepted projection.
	return nil
}

func guardProposalEvaluationDependencies(operation string, proposal *StoredEvent, events []StoredEvent) error {
	if err := guardProposalDependencies(operation, proposal, events); err != nil {
		return err
	}
	if err := guardBasePolicy(operation, proposal.Event.Base, replicationProposal, proposal.ID); err != nil {
		return err
	}
	policy, _, _, err := loadPolicy(proposal.Event.Base)
	if err != nil {
		return err
	}
	for name := range policy.Pipelines {
		if err := guardPipelineDefinition(operation, proposal.Event.Head, name, proposal.ID, ""); err != nil {
			return err
		}
	}
	return nil
}

func resolveCommitDependency(operation string, kind ShallowDependencyKind, revision, ownerKind, ownerID string) (string, error) {
	if validFullGitObjectID(revision) {
		if err := requireExactDependency(exactDependency{
			Operation: operation, Kind: kind, MissingID: revision,
			Objectish: revision, ObjectType: "commit", OwnerKind: ownerKind, OwnerID: ownerID,
		}); err != nil {
			return "", err
		}
		return revision, nil
	}
	commit, err := resolveCommit(revision)
	if err == nil {
		return commit, nil
	}
	return "", err
}

func validFullGitObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func guardBasePolicy(operation, base, ownerKind, ownerID string) error {
	if err := requireExactDependency(exactDependency{
		Operation: operation, Kind: shallowBaseCommit, MissingID: base,
		ObjectType: "commit", OwnerKind: ownerKind, OwnerID: ownerID,
	}); err != nil {
		return err
	}
	policyOID, exists, err := exactTreeEntry(base, ".hn/policy.json")
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("no .hn/policy.json at commit %s", base)
	}
	if err := requireExactDependency(exactDependency{
		Operation: operation, Kind: shallowPolicyBlob, MissingID: policyOID,
		Objectish: policyOID, ObjectType: "blob", OwnerKind: ownerKind, OwnerID: ownerID,
	}); err != nil {
		return err
	}
	return nil
}

func exactTreeEntry(commit, path string) (string, bool, error) {
	output, err := gitText("ls-tree", commit, "--", path)
	if err != nil {
		return "", false, err
	}
	if output == "" {
		return "", false, nil
	}
	fields := strings.Fields(output)
	if len(fields) < 3 || fields[1] != "blob" || !validFullGitObjectID(fields[2]) {
		return "", false, fmt.Errorf("invalid Git tree entry for %s at %s", safeDiagnostic(path), commit)
	}
	return fields[2], true, nil
}

func guardPipelineDefinition(operation, commit, name, ownerID, expectedDefinition string) error {
	if err := requireExactDependency(exactDependency{
		Operation: operation, Kind: shallowProposalCodeRef, MissingID: commit,
		ObjectType: "commit", OwnerKind: replicationProposal, OwnerID: ownerID,
	}); err != nil {
		return err
	}
	pipelineOID, exists, err := exactTreeEntry(commit, ".hn/pipelines/"+name+".json")
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("pipeline %q does not exist at commit %s", name, commit)
	}
	missingID := pipelineOID
	if validEventID(expectedDefinition) {
		missingID = expectedDefinition
	}
	if err := requireExactDependency(exactDependency{
		Operation: operation, Kind: shallowPipelineDefinition, MissingID: missingID,
		Objectish: pipelineOID, ObjectType: "blob", OwnerKind: replicationProposal, OwnerID: ownerID,
	}); err != nil {
		return err
	}
	return nil
}

func guardCandidateCreationDependencies(operation, base, head string) error {
	if err := guardBasePolicy(operation, base, "", ""); err != nil {
		return err
	}
	policy, _, _, err := loadPolicy(base)
	if err != nil {
		return err
	}
	for name := range policy.Pipelines {
		if err := guardPipelineDefinition(operation, head, name, "", ""); err != nil {
			return err
		}
	}
	return nil
}

func guardProposalQuery(operation, query string) error {
	if err := guardShallowEventClosure(operation); err != nil {
		return err
	}
	events, err := collectEvents()
	if err != nil {
		return err
	}
	proposal, err := resolveEventDependency(operation, query, shallowCandidateEvent, events)
	if err != nil {
		return err
	}
	return guardProposalEvaluationDependencies(operation, proposal, events)
}

func resolveEventDependency(operation, query string, kind ShallowDependencyKind, events []StoredEvent) (*StoredEvent, error) {
	stored, err := resolveEvent(events, query)
	if err == nil {
		return stored, nil
	}
	if !validEventID(query) {
		return nil, err
	}
	return nil, classifyShallowDependency(exactDependency{
		Operation: operation, Kind: kind, MissingID: query,
	}, err)
}

func guardMergeAncestry(operation string, proposal *StoredEvent, current string) error {
	for _, commit := range []string{proposal.Event.Base, current} {
		if err := requireExactDependency(exactDependency{
			Operation: operation, Kind: shallowMergeAncestor, MissingID: commit,
			ObjectType: "commit", OwnerKind: replicationProposal, OwnerID: proposal.ID,
		}); err != nil {
			return err
		}
	}
	ancestor, missing, err := exactCommitAncestor(proposal.Event.Base, current)
	if err != nil {
		return err
	}
	if missing != "" {
		return classifyShallowDependency(exactDependency{
			Operation: operation, Kind: shallowMergeAncestor, MissingID: missing,
			ObjectType: "commit", OwnerKind: replicationProposal, OwnerID: proposal.ID,
		}, fmt.Errorf("exact merge ancestry requires unavailable parent %s", missing))
	}
	if !ancestor {
		return fmt.Errorf("current commit %s does not descend from proposal base %s", current, proposal.Event.Base)
	}
	return nil
}

// exactCommitAncestor walks immutable commit-parent identities directly. Git's
// revision walkers intentionally stop at .git/shallow; integrity verification
// must not when every exact parent object is already present.
func exactCommitAncestor(ancestor, descendant string) (bool, string, error) {
	return exactCommitAncestorUntil(ancestor, descendant)
}

func exactCommitAncestorUntil(ancestor, descendant string, stop ...string) (bool, string, error) {
	pending := []string{descendant}
	seen := make(map[string]bool)
	stops := make(map[string]bool, len(stop))
	for _, commit := range stop {
		stops[commit] = true
	}
	firstMissing := ""
	for len(pending) > 0 {
		commit := pending[0]
		pending = pending[1:]
		if seen[commit] {
			continue
		}
		seen[commit] = true
		if commit == ancestor {
			return true, "", nil
		}
		if stops[commit] {
			continue
		}
		probe, err := shallowObjectProbe(commit)
		if err != nil {
			return false, "", fmt.Errorf("inspect exact ancestry commit %s: %w", commit, err)
		}
		if !probe.Exists {
			if firstMissing == "" {
				firstMissing = commit
			}
			continue
		}
		if probe.Type != "commit" {
			return false, "", fmt.Errorf("exact ancestry object %s has Git object type %s, want commit", commit, probe.Type)
		}
		parents, err := exactCommitParents(commit)
		if err != nil {
			return false, "", err
		}
		pending = append(pending, parents...)
	}
	return false, firstMissing, nil
}

func exactCommitParents(commit string) ([]string, error) {
	contents, err := gitText("cat-file", "commit", commit)
	if err != nil {
		return nil, fmt.Errorf("read exact ancestry commit %s: %w", commit, err)
	}
	parents := make([]string, 0, 2)
	treeSeen := false
	for _, line := range strings.Split(contents, "\n") {
		if line == "" {
			break
		}
		switch {
		case strings.HasPrefix(line, "tree "):
			tree := strings.TrimPrefix(line, "tree ")
			if treeSeen || !validFullGitObjectID(tree) {
				return nil, fmt.Errorf("malformed commit tree in %s", commit)
			}
			treeSeen = true
		case strings.HasPrefix(line, "parent "):
			parent := strings.TrimPrefix(line, "parent ")
			if !validFullGitObjectID(parent) {
				return nil, fmt.Errorf("malformed commit parent in %s", commit)
			}
			parents = append(parents, parent)
		}
	}
	if !treeSeen {
		return nil, fmt.Errorf("malformed commit tree in %s", commit)
	}
	return parents, nil
}

func prepareShallowVerification(scope shallowVerificationScope) error {
	shallow, err := repositoryIsShallow()
	if err != nil || !shallow {
		return err
	}
	return withShallowVerificationScope(scope, func() error {
		return verifyShallowOperation(scope)
	})
}

func withShallowVerificationScope(scope shallowVerificationScope, verify func() error) error {
	previous := activeShallowVerificationScope
	activeShallowVerificationScope = &scope
	defer func() { activeShallowVerificationScope = previous }()
	return verify()
}

func verifyShallowOperation(scope shallowVerificationScope) error {
	switch scope.Operation {
	case "proposal list", "run list", "log":
		return guardShallowEventClosure(scope.Operation)
	case "proposal show":
		_, proposal, err := resolveShallowOperationEvent(scope, shallowCandidateEvent)
		if err != nil {
			return err
		}
		if !isProposalKind(proposal.Event.Kind) {
			return fmt.Errorf("%s is not a proposal", shortID(proposal.ID))
		}
		return nil
	case "proposal status", "proposal decision":
		events, proposal, err := resolveShallowOperationEvent(scope, shallowCandidateEvent)
		if err != nil {
			return err
		}
		if err := guardProposalEvaluationDependencies(scope.Operation, proposal, events); err != nil {
			return err
		}
		_, err = evaluateProposal(proposal, events)
		return err
	case "proposal review":
		events, proposal, err := resolveShallowOperationEvent(scope, shallowCandidateEvent)
		if err != nil {
			return err
		}
		if !isProposalKind(proposal.Event.Kind) {
			return fmt.Errorf("%s is not a proposal", shortID(proposal.ID))
		}
		return guardProposalDependencies(scope.Operation, proposal, events)
	case "proposal merge":
		events, proposal, err := resolveShallowOperationEvent(scope, shallowCandidateEvent)
		if err != nil {
			return err
		}
		if err := guardProposalEvaluationDependencies(scope.Operation, proposal, events); err != nil {
			return err
		}
		if _, err := evaluateProposal(proposal, events); err != nil {
			return err
		}
		current := scope.Current
		if current == "" {
			current, err = resolveCommit("HEAD")
			if err != nil {
				return err
			}
			if activeShallowVerificationScope != nil {
				activeShallowVerificationScope.Current = current
			}
		}
		if err := guardMergeAncestry(scope.Operation, proposal, current); err != nil {
			return err
		}
		_, missing, err := exactCommitAncestorUntil(proposal.Event.Head, current, proposal.Event.Base)
		if err != nil {
			return err
		}
		if missing != "" {
			return classifyShallowDependency(exactDependency{
				Operation: scope.Operation, Kind: shallowMergeAncestor, MissingID: missing,
				ObjectType: "commit", OwnerKind: replicationProposal, OwnerID: proposal.ID,
			}, fmt.Errorf("exact proposal containment requires unavailable parent %s", missing))
		}
		return nil
	case "proposal open", "proposal revision":
		if err := guardShallowEventClosure(scope.Operation); err != nil {
			return err
		}
		base, err := resolveCommitDependency(scope.Operation, shallowBaseCommit, scope.Base, "", "")
		if err != nil {
			return err
		}
		head, err := resolveCommitDependency(scope.Operation, shallowProposalCodeRef, scope.Head, "", "")
		if err != nil {
			return err
		}
		if base == head {
			return fmt.Errorf("proposal base and head resolve to the same commit")
		}
		if activeShallowVerificationScope != nil {
			activeShallowVerificationScope.Base = base
			activeShallowVerificationScope.Head = head
		}
		if err := guardCandidateCreationDependencies(scope.Operation, base, head); err != nil {
			return err
		}
		if _, err := policyAmendmentDiagnostic(base, head); err != nil {
			return err
		}
		if scope.Operation == "proposal revision" {
			_, predecessor, err := resolveShallowOperationEvent(scope, shallowCandidateEvent)
			if err != nil {
				return err
			}
			if !isProposalKind(predecessor.Event.Kind) {
				return fmt.Errorf("%s is not a proposal", shortID(predecessor.ID))
			}
		}
		return nil
	case "run request":
		events, proposal, err := resolveShallowOperationEvent(scope, shallowCandidateEvent)
		if err != nil {
			return err
		}
		if !isProposalKind(proposal.Event.Kind) {
			return fmt.Errorf("%s is not a proposal", shortID(proposal.ID))
		}
		if err := guardProposalDependencies(scope.Operation, proposal, events); err != nil {
			return err
		}
		if err := guardBasePolicy(scope.Operation, proposal.Event.Base, replicationProposal, proposal.ID); err != nil {
			return err
		}
		if err := requireProposalCode(proposal); err != nil {
			return err
		}
		if err := guardPipelineDefinition(scope.Operation, proposal.Event.Head, scope.Pipeline, proposal.ID, ""); err != nil {
			return err
		}
		_, _, _, err = loadPipeline(proposal.Event.Head, scope.Pipeline)
		return err
	case "run show":
		_, request, err := resolveShallowOperationEvent(scope, shallowRunRequest)
		if err != nil {
			return err
		}
		if request.Event.Kind != "run.request" {
			return fmt.Errorf("%s is not a run request", shortID(request.ID))
		}
		return nil
	case "run logs":
		_, result, err := resolveShallowOperationEvent(scope, shallowRunResult)
		if err != nil {
			return err
		}
		if result.Event.Kind != "run.result" {
			return fmt.Errorf("%s is not a run result", shortID(result.ID))
		}
		if _, exists := result.Attachments["log.txt"]; !exists {
			return fmt.Errorf("result has no verified log")
		}
		return nil
	case "run execute":
		events, request, err := resolveShallowOperationEvent(scope, shallowRunRequest)
		if err != nil {
			return err
		}
		if request.Event.Kind != "run.request" {
			return fmt.Errorf("%s is not a run request", shortID(request.ID))
		}
		proposal, err := resolveEventDependency(scope.Operation, request.Event.Subject, shallowCandidateEvent, events)
		if err != nil {
			return err
		}
		if !isProposalKind(proposal.Event.Kind) || proposal.Event.Head != request.Event.Commit {
			return fmt.Errorf("run request does not match its signed proposal")
		}
		if err := guardProposalDependencies(scope.Operation, proposal, events); err != nil {
			return err
		}
		if err := guardPipelineDefinition(scope.Operation, request.Event.Commit, request.Event.Pipeline, proposal.ID, request.Event.Definition); err != nil {
			return err
		}
		if err := requireProposalCode(proposal); err != nil {
			return err
		}
		_, _, definition, err := loadPipeline(request.Event.Commit, request.Event.Pipeline)
		if err != nil {
			return err
		}
		if definition != request.Event.Definition {
			return fmt.Errorf("pipeline definition does not match the signed run request")
		}
		return nil
	case "policy show":
		commit, err := resolveCommitDependency("policy", shallowBaseCommit, scope.Subject, "", "")
		if err != nil {
			return err
		}
		if activeShallowVerificationScope != nil {
			activeShallowVerificationScope.Subject = commit
		}
		_, err = loadPolicyRevision("policy", commit)
		return err
	case "policy check":
		base, err := resolveCommitDependency("base policy", shallowBaseCommit, scope.Base, "", "")
		if err != nil {
			return err
		}
		if activeShallowVerificationScope != nil {
			activeShallowVerificationScope.Base = base
		}
		if _, err := loadPolicyRevision("base", base); err != nil {
			return err
		}
		if scope.Head != "" {
			head, err := resolveCommitDependency("proposed policy", shallowProposalCodeRef, scope.Head, "", "")
			if err != nil {
				return err
			}
			if activeShallowVerificationScope != nil {
				activeShallowVerificationScope.Head = head
			}
			_, err = loadPolicyRevision("proposed", head)
			return err
		}
		_, err = loadPolicyFile("proposed", scope.ProposedFile)
		return err
	default:
		return fmt.Errorf("unsupported shallow verification scope %q", scope.Operation)
	}
}

func resolveShallowOperationEvent(scope shallowVerificationScope, kind ShallowDependencyKind) ([]StoredEvent, *StoredEvent, error) {
	if err := guardShallowEventClosure(scope.Operation); err != nil {
		return nil, nil, err
	}
	events, err := collectEvents()
	if err != nil {
		return nil, nil, err
	}
	stored, err := resolveEventDependency(scope.Operation, scope.Subject, kind, events)
	if err != nil {
		return nil, nil, err
	}
	if activeShallowVerificationScope != nil {
		activeShallowVerificationScope.Subject = stored.ID
	}
	return events, stored, nil
}

// recoverSelectedShallow deliberately contains no Git fetch implementation.
// It requires an explicit exact selection and delegates to WP04's single
// validate-before-promotion quarantine transaction.
func recoverSelectedShallow(remote string) error {
	if !validReplicationRemote(remote) {
		return fmt.Errorf("invalid remote name %q", remote)
	}
	shallow, err := repositoryIsShallow()
	if err != nil {
		return err
	}
	if !shallow {
		return fmt.Errorf("selected shallow recovery requires a shallow repository")
	}
	record, err := loadShallowGapRecord()
	if errors.Is(err, os.ErrNotExist) {
		discoveryErr := guardShallowEventClosure("shallow recovery discovery")
		var discovered *ShallowDependencyGap
		if !errors.As(discoveryErr, &discovered) {
			if discoveryErr != nil {
				return discoveryErr
			}
			// Already recovered is an idempotent no-op followed by fresh
			// accepted-projection verification.
			return guardShallowEventClosure("shallow recovery verification")
		}
		record, err = loadShallowGapRecord()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	gap := record.Gap
	if gap.Remote != "" && gap.Remote != remote {
		return fmt.Errorf("recorded shallow gap belongs to remote %s, not %s", gap.Remote, remote)
	}
	verify := func() error {
		if record.Scope != nil {
			return withShallowVerificationScope(*record.Scope, func() error {
				return verifyShallowOperation(*record.Scope)
			})
		}
		return verifyRecordedShallowGap(gap)
	}
	verificationErr := verify()
	if verificationErr == nil {
		return clearShallowGapRecord()
	}
	var currentGap *ShallowDependencyGap
	var pendingAcceptance *ReplicationAcceptancePendingError
	if errors.As(verificationErr, &currentGap) {
		gap = currentGap
		if refreshed, refreshErr := loadShallowGapRecord(); refreshErr == nil {
			record = refreshed
		}
	} else if !errors.As(verificationErr, &pendingAcceptance) {
		return fmt.Errorf("recorded shallow operation still fails fresh verification: %w", verificationErr)
	}
	if gap.Remote != "" && gap.Remote != remote {
		return fmt.Errorf("fresh shallow gap belongs to remote %s, not %s", gap.Remote, remote)
	}
	selection, explicit, err := loadReplicationSelection(remote)
	if err != nil {
		return err
	}
	if !explicit || selection.All {
		return fmt.Errorf("%w: save an exact actor/proposal/memory selection and positive budgets for remote %s", errShallowRecoverySelectionRequired, remote)
	}
	selectionPath, err := replicationSelectionPath(remote)
	if err != nil {
		return err
	}
	selectionBytes, err := readPrivateFile(selectionPath)
	if err != nil {
		return err
	}
	subset, err := recoverySelectionSubset(selection, gap)
	if err != nil {
		return err
	}
	result, transactionErr := runReplicationTransaction(subset)
	for _, outcome := range result.Outcomes {
		if outcome.Status == replicationPromoted {
			fmt.Printf("Shallow recovery %s %s: promoted\n", outcome.Kind, outcome.ID)
		} else {
			fmt.Printf("Shallow recovery %s %s: failed (%s): %s\n", outcome.Kind, outcome.ID, outcome.Status, oneLine(outcome.Diagnostic))
		}
	}
	if transactionErr != nil {
		return transactionErr
	}
	if result.hasFailures() {
		return fmt.Errorf("selected shallow recovery failed for one or more exact selections; accepted state remains authoritative")
	}
	afterBytes, err := readPrivateFile(selectionPath)
	if err != nil {
		return err
	}
	if !bytes.Equal(selectionBytes, afterBytes) {
		return fmt.Errorf("selected shallow recovery changed its saved selection bytes")
	}
	if err := verify(); err != nil {
		return fmt.Errorf("shallow recovery imported the exact selected supplier, but the complete original operation still fails fresh verification: %w", err)
	}
	return clearShallowGapRecord()
}

func clearShallowGapRecord() error {
	path, err := shallowGapPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("full shallow verification succeeded, but durable recovery state could not be cleared: %w", err)
	}
	return nil
}

func recoverySelectionSubset(selection ReplicationSelection, gap *ShallowDependencyGap) (ReplicationSelection, error) {
	subset := ReplicationSelection{
		Version: selection.Version,
		Remote:  selection.Remote,
		Budgets: selection.Budgets,
	}
	switch gap.OwnerKind {
	case replicationActor:
		for _, actor := range selection.Actors {
			if actor == gap.OwnerID {
				subset.Actors = []string{actor}
				return subset, nil
			}
		}
		return ReplicationSelection{}, fmt.Errorf("actor supplier %s is not in the saved exact selection; %s", gap.OwnerID, gap.Recovery)
	case replicationProposal:
		for _, proposal := range selection.Proposals {
			if proposal == gap.OwnerID {
				subset.Proposals = []string{proposal}
				return subset, nil
			}
		}
		return ReplicationSelection{}, fmt.Errorf("candidate supplier %s is not in the saved exact selection; %s", gap.OwnerID, gap.Recovery)
	case replicationMemory:
		for _, stream := range selection.Memories {
			if stream == gap.OwnerID {
				subset.Memories = []string{stream}
				for _, selector := range gap.RequiredSelectors {
					kind, id, ok := strings.Cut(selector, ":")
					if !ok {
						return ReplicationSelection{}, fmt.Errorf("invalid recorded memory recovery selector")
					}
					switch kind {
					case replicationActor:
						if !stringInSlice(id, selection.Actors) {
							return ReplicationSelection{}, fmt.Errorf("required actor supplier %s is no longer selected", id)
						}
						subset.Actors = append(subset.Actors, id)
					case replicationProposal:
						if !stringInSlice(id, selection.Proposals) {
							return ReplicationSelection{}, fmt.Errorf("required proposal supplier %s is no longer selected", id)
						}
						subset.Proposals = append(subset.Proposals, id)
					case replicationMemory:
						if !stringInSlice(id, selection.Memories) {
							return ReplicationSelection{}, fmt.Errorf("required memory supplier %s is no longer selected", id)
						}
						subset.Memories = append(subset.Memories, id)
					default:
						return ReplicationSelection{}, fmt.Errorf("invalid recorded memory recovery selector kind")
					}
				}
				subset.Actors = sortedUniqueStrings(subset.Actors...)
				subset.Proposals = sortedUniqueStrings(subset.Proposals...)
				subset.Memories = sortedUniqueStrings(subset.Memories...)
				return subset, nil
			}
		}
		return ReplicationSelection{}, fmt.Errorf("memory supplier %s is not in the saved exact selection; %s", gap.OwnerID, gap.Recovery)
	default:
		return ReplicationSelection{}, fmt.Errorf("supplier for exact missing ID %s is not derivable from signed facts; %s", gap.MissingID, gap.Recovery)
	}
}

func stringInSlice(value string, values []string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func verifyRecordedShallowGap(gap *ShallowDependencyGap) error {
	switch gap.Kind {
	case shallowActorPredecessor:
		return guardShallowEventClosure(gap.Operation)
	case shallowCandidateEvent, shallowRunRequest, shallowRunResult, shallowDecision, shallowSelectedFact:
		if err := guardShallowEventClosure(gap.Operation); err != nil {
			return err
		}
		events, err := collectEvents()
		if err != nil {
			return err
		}
		_, err = resolveEventDependency(gap.Operation, gap.MissingID, gap.Kind, events)
		return err
	case shallowMemoryPredecessor, shallowMemoryLifecycle, shallowMemoryAnchor, shallowMemoryEvidence, shallowMemoryStream:
		return verifyRecordedMemoryGap(gap)
	default:
		return requireExactDependency(exactDependency{
			Operation: gap.Operation, Kind: gap.Kind, MissingID: gap.MissingID,
			Objectish: gap.Objectish, ObjectType: gap.ObjectType,
			OwnerKind: gap.OwnerKind, OwnerID: gap.OwnerID,
			OwnerMemoryID: gap.OwnerMemoryID, OwnerStream: gap.OwnerStream,
			Remote: gap.Remote, RequiredRef: gap.RequiredRef,
		})
	}
}

func verifyRecordedMemoryGap(gap *ShallowDependencyGap) error {
	memories, err := collectMemories()
	if err != nil {
		return err
	}
	events, err := collectEvents()
	if err != nil {
		return err
	}
	projection := ProjectMemories(memories, MemoryProjectionContext{Events: events})
	for _, dependency := range projection.MissingDependencies {
		if dependency.OwnerID == gap.OwnerMemoryID && dependency.Stream == gap.OwnerStream && dependency.MissingID == gap.MissingID {
			return gap
		}
	}
	for _, diagnostic := range projection.Diagnostics {
		if diagnostic.MemoryID == gap.OwnerMemoryID {
			return fmt.Errorf("recorded memory operation is invalid: %s", diagnostic.Code)
		}
	}
	for _, row := range projection.Rows {
		if row.ID != gap.OwnerMemoryID {
			continue
		}
		if row.Applicability == memoryApplicabilityAnchorMissing || row.Evidence == memoryEvidenceMissing || row.Lifecycle == memoryLifecycleDependencyMissing {
			return gap
		}
		if row.Applicability == memoryApplicabilityAnchorInvalid || row.Evidence == memoryEvidenceInvalid {
			return fmt.Errorf("recorded memory operation has invalid anchor or evidence")
		}
		return nil
	}
	return gap
}

var _ error = (*ShallowDependencyGap)(nil)
