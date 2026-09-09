package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	replicationFetched           = "fetched"
	replicationOverBudget        = "over-budget"
	replicationStructuralInvalid = "structurally-invalid"
	replicationDependencyMissing = "dependency-missing"
	replicationRelationshipBad   = "relationship-invalid"
	replicationPromotable        = "promotable"
	replicationPromoted          = "promoted"
)

type replicationRequest struct {
	Kind          string
	ID            string
	SourceRef     string
	QuarantineRef string
	OID           string
}

func (request replicationRequest) key() string { return request.Kind + ":" + request.ID }

type ReplicationOutcome struct {
	Kind              string                  `json:"kind"`
	ID                string                  `json:"id"`
	Status            string                  `json:"status"`
	Diagnostic        string                  `json:"diagnostic,omitempty"`
	DependencyKind    string                  `json:"dependencyKind,omitempty"`
	MissingID         string                  `json:"missingId,omitempty"`
	OwnerMemoryID     string                  `json:"ownerMemoryId,omitempty"`
	OwnerStream       string                  `json:"ownerStream,omitempty"`
	RequiredRef       string                  `json:"requiredRef,omitempty"`
	RequiredSelectors []string                `json:"requiredSelectors,omitempty"`
	Recovery          string                  `json:"recovery,omitempty"`
	Measurements      ReplicationMeasurements `json:"measurements"`
	request           replicationRequest
	events            []StoredEvent
	memories          []StoredMemory
	dependencies      []string
	acceptedOld       string
}

type replicationTransactionResult struct {
	ID       string
	Remote   string
	Outcomes []ReplicationOutcome
	Promoted int

	promotions      []replicationPromotion
	pendingObjects  []string
	acceptedObjects []string
}

func (result replicationTransactionResult) hasFailures() bool {
	for _, outcome := range result.Outcomes {
		if outcome.Status != replicationPromoted {
			return true
		}
	}
	return false
}

type replicationPromotion struct {
	Ref    string `json:"ref"`
	NewOID string `json:"newOid"`
	OldOID string `json:"oldOid,omitempty"`
}

const replicationTransactionRecordVersion = 2

const (
	replicationPendingAnchorVersion = 1
	maxReplicationStateBytes        = 16 << 20
)

type replicationTransactionRecord struct {
	Version         int                    `json:"version"`
	ID              string                 `json:"id"`
	Remote          string                 `json:"remote"`
	State           string                 `json:"state"`
	Outcomes        []ReplicationOutcome   `json:"outcomes"`
	Promotions      []replicationPromotion `json:"promotions,omitempty"`
	PendingObjects  *[]string              `json:"pendingObjects,omitempty"`
	AcceptedObjects *[]string              `json:"acceptedObjects,omitempty"`
}

type replicationPendingAnchor struct {
	Version        int       `json:"version"`
	ID             string    `json:"id"`
	Remote         string    `json:"remote"`
	PendingObjects *[]string `json:"pendingObjects"`
}

type replicationDependency struct {
	EventID   string
	EventKind string
	Missing   string
	Recovery  string
	Key       string
	Reason    string
}

type replicationBudgetExceeded struct{ err error }

func (failure replicationBudgetExceeded) Error() string { return failure.err.Error() }
func (failure replicationBudgetExceeded) Unwrap() error { return failure.err }

var (
	replicationBeforeFetchHook   func() error
	replicationAfterFetchHook    func() error
	replicationAfterMeasureHook  func() error
	replicationAfterCopyHook     func() error
	replicationBeforePromoteHook func() error
	replicationReleaseShallow    = releaseRecoveredShallowBoundaries
	replicationCopyObjects       = copyGitObjects
	replicationRemoveQuarantine  = removeGeneratedQuarantine
	replicationRecordTransaction = recordReplicationTransaction
)

func runReplicationProcessHook(step string) error {
	if os.Getenv("HN_INTERNAL_TESTING") == "1" && os.Getenv("HN_TEST_REPLICATION_INTERRUPT_AFTER") == step {
		return fmt.Errorf("injected replication interruption at %s", step)
	}
	return nil
}

func replicationPhaseError(remote, phase string) error {
	return fmt.Errorf("replication %s failed for remote %s", phase, remote)
}

func acceptedActorRef(remote, actor string) string {
	return "refs/hn/remotes/" + remote + "/actors/" + actor
}

func acceptedProposalRef(remote, proposal string) string {
	return "refs/hn/remotes/" + remote + "/proposals/" + strings.TrimPrefix(proposal, "sha256:")
}

func parseAcceptedActorRef(ref string) (string, string, bool) {
	const prefix = "refs/hn/remotes/"
	if !strings.HasPrefix(ref, prefix) {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(ref, prefix), "/")
	if len(parts) != 3 || parts[1] != "actors" || !validReplicationRemote(parts[0]) || !validActorFingerprint(parts[2]) {
		return "", "", false
	}
	return parts[0], parts[2], true
}

func parseAcceptedProposalRef(ref string) (string, string, bool) {
	const prefix = "refs/hn/remotes/"
	if !strings.HasPrefix(ref, prefix) {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(ref, prefix), "/")
	if len(parts) != 3 || parts[1] != "proposals" || !validReplicationRemote(parts[0]) {
		return "", "", false
	}
	id := "sha256:" + parts[2]
	if !validEventID(id) {
		return "", "", false
	}
	return parts[0], id, true
}

func runReplicationTransaction(selection ReplicationSelection) (replicationTransactionResult, error) {
	result := replicationTransactionResult{Remote: selection.Remote}
	if err := validateReplicationSelection(selection); err != nil {
		return result, err
	}
	remoteURL, err := gitText("remote", "get-url", selection.Remote)
	if err != nil {
		return result, fmt.Errorf("remote %q does not exist", selection.Remote)
	}
	mainGitDir, err := requireGitRepository()
	if err != nil {
		return result, replicationPhaseError(selection.Remote, "repository setup")
	}
	requests, initialOutcomes, err := resolveReplicationRequests(selection, remoteURL)
	if err != nil {
		return result, err
	}
	result.Outcomes = append(result.Outcomes, initialOutcomes...)
	quarantineRoot := filepath.Join(mainGitDir, "hn", "replication", "quarantine")
	for _, directory := range []string{filepath.Join(mainGitDir, "hn"), filepath.Join(mainGitDir, "hn", "replication"), quarantineRoot} {
		if err := ensurePrivateDirectory(directory); err != nil {
			return result, replicationPhaseError(selection.Remote, "quarantine setup")
		}
	}
	if err := repairAnchoredReplicationReceiptsForRecovery(mainGitDir, selection.Remote); err != nil {
		return result, replicationPhaseError(selection.Remote, "pending acceptance reconciliation")
	}
	quarantineDir, err := os.MkdirTemp(quarantineRoot, "txn-")
	if err != nil {
		return result, replicationPhaseError(selection.Remote, "quarantine setup")
	}
	result.ID = filepath.Base(quarantineDir)
	cleanupNeeded := true
	defer func() {
		if cleanupNeeded {
			_ = replicationRemoveQuarantine(quarantineRoot, quarantineDir)
		}
	}()
	if _, err := gitOutput("init", "--bare", "--quiet", quarantineDir); err != nil {
		return result, replicationPhaseError(selection.Remote, "quarantine setup")
	}
	if replicationBeforeFetchHook != nil {
		if err := replicationBeforeFetchHook(); err != nil {
			return result, replicationPhaseError(selection.Remote, "quarantine fetch")
		}
	}

	fetched := make([]replicationRequest, 0, len(requests))
	for _, request := range requests {
		if _, err := gitOutputAt(quarantineDir, "fetch", "--no-tags", "--no-write-fetch-head", "--", remoteURL,
			"+"+request.SourceRef+":"+request.QuarantineRef); err != nil {
			result.Outcomes = append(result.Outcomes, failedReplicationOutcome(request, replicationStructuralInvalid,
				fmt.Sprintf("selection %s %s failed exact fetch from remote %s", request.Kind, request.ID, selection.Remote)))
			continue
		}
		oid, exists, err := refValueAt(quarantineDir, request.QuarantineRef)
		if err != nil || !exists || oid != request.OID {
			result.Outcomes = append(result.Outcomes, failedReplicationOutcome(request, replicationStructuralInvalid,
				fmt.Sprintf("selection %s %s fetched unexpected object: advertised=%s fetched=%s", request.Kind, request.ID, request.OID, oid)))
			continue
		}
		request.OID = oid
		fetched = append(fetched, request)
	}
	if replicationAfterFetchHook != nil {
		if err := replicationAfterFetchHook(); err != nil {
			return result, replicationPhaseError(selection.Remote, "quarantine verification")
		}
	}

	outcomes := make(map[string]*ReplicationOutcome, len(fetched))
	for _, request := range fetched {
		outcome := &ReplicationOutcome{Kind: request.Kind, ID: request.ID, Status: replicationFetched, request: request}
		acceptedRoot, _, err := refValue(acceptedRefForRequest(selection.Remote, request))
		if err != nil {
			return result, replicationPhaseError(selection.Remote, "accepted-ref inspection")
		}
		outcome.acceptedOld = acceptedRoot
		measured, err := measureQuarantinedSelectionUnderBudgets(quarantineDir, mainGitDir, request.Kind, request.OID, acceptedRoot, selection.Budgets, request.Kind+" "+request.ID)
		if err != nil {
			var budgetFailure replicationBudgetExceeded
			if errors.As(err, &budgetFailure) {
				outcome.Status = replicationOverBudget
				outcome.Diagnostic = budgetFailure.Error()
				outcome.Measurements = measured
				outcomes[request.key()] = outcome
				continue
			}
			outcome.Status = replicationStructuralInvalid
			outcome.Diagnostic = redactReplicationDiagnostic(
				fmt.Sprintf("selection %s %s is structurally invalid: %v", request.Kind, request.ID, err),
				quarantineDir, mainGitDir, remoteURL,
			)
			outcomes[request.key()] = outcome
			continue
		}
		outcome.Measurements = measured
		if err := enforceReplicationBudgets(request.Kind+" "+request.ID, selection.Budgets, measured); err != nil {
			outcome.Status = replicationOverBudget
			outcome.Diagnostic = err.Error()
			outcomes[request.key()] = outcome
			continue
		}
		if request.Kind == replicationActor {
			events, err := validateQuarantinedActor(quarantineDir, request.ID, request.OID)
			if err != nil {
				outcome.Status = replicationStructuralInvalid
				outcome.Diagnostic = redactReplicationDiagnostic(
					fmt.Sprintf("selection actor %s is structurally invalid: %v", request.ID, err),
					quarantineDir, mainGitDir, remoteURL,
				)
				outcomes[request.key()] = outcome
				continue
			}
			outcome.events = events
		} else if request.Kind == replicationMemory {
			actor, stream, ok := parseMemoryRef(request.SourceRef)
			if !ok || stream != request.ID {
				outcome.Status = replicationStructuralInvalid
				outcome.Diagnostic = fmt.Sprintf("selection memory %s has an invalid owner-bound source ref", request.ID)
				outcomes[request.key()] = outcome
				continue
			}
			memories, err := loadMemoryStreamAt(quarantineDir, memoryStreamSource{
				Ref: request.SourceRef, Actor: actor, Stream: stream, Head: request.OID,
			})
			if err != nil {
				outcome.Status = replicationStructuralInvalid
				outcome.Diagnostic = redactReplicationDiagnostic(
					fmt.Sprintf("selection memory %s is structurally invalid: %v", request.ID, err),
					quarantineDir, mainGitDir, remoteURL,
				)
				outcomes[request.key()] = outcome
				continue
			}
			outcome.memories = memories
		}
		outcome.Status = replicationPromotable
		outcomes[request.key()] = outcome
	}
	if replicationAfterMeasureHook != nil {
		if err := replicationAfterMeasureHook(); err != nil {
			return result, replicationPhaseError(selection.Remote, "quarantine measurement")
		}
	}

	acceptedEvents, err := collectAcceptedEventsForReplication(selection)
	if err != nil {
		return result, replicationPhaseError(selection.Remote, "accepted-fact inspection")
	}
	byID := make(map[string]StoredEvent, len(acceptedEvents))
	eventOwner := make(map[string]string)
	for _, event := range acceptedEvents {
		byID[event.ID] = event
	}
	for key, outcome := range outcomes {
		if outcome.Status != replicationPromotable || outcome.Kind != replicationActor {
			continue
		}
		for _, event := range outcome.events {
			byID[event.ID] = event
			eventOwner[event.ID] = key
		}
	}
	codeHeads, codeOwners, err := acceptedProposalHeads()
	if err != nil {
		return result, replicationPhaseError(selection.Remote, "accepted-proposal inspection")
	}
	for key, outcome := range outcomes {
		if outcome.Status == replicationPromotable && outcome.Kind == replicationProposal {
			codeHeads[outcome.ID] = outcome.request.OID
			codeOwners[outcome.ID] = key
		}
	}

	for key, outcome := range outcomes {
		if outcome.Status != replicationPromotable {
			continue
		}
		if outcome.Kind == replicationProposal {
			proposal, exists := byID[outcome.ID]
			if !exists {
				markReplicationDependency(outcome, replicationDependency{
					Missing: outcome.ID, Recovery: "select the full actor history that contains candidate " + outcome.ID,
					Reason: "candidate event is absent from accepted and selected actor histories",
				})
				continue
			}
			if !isProposalKind(proposal.Event.Kind) || proposal.Event.Head != outcome.request.OID {
				outcome.Status = replicationRelationshipBad
				outcome.Diagnostic = fmt.Sprintf("proposal selection %s ref/head mismatch: selected=%s event-head=%s", outcome.ID, outcome.request.OID, proposal.Event.Head)
				continue
			}
			if owner := eventOwner[outcome.ID]; owner != "" && owner != key {
				outcome.dependencies = append(outcome.dependencies, owner)
			}
			continue
		}
		for _, event := range outcome.events {
			dependency, relationErr := replicationEventDependency(quarantineDir, mainGitDir, event, byID, codeHeads, eventOwner, codeOwners)
			if relationErr != nil {
				outcome.Status = replicationRelationshipBad
				outcome.Diagnostic = fmt.Sprintf("selection actor %s event %s (%s) is invalid: %v", outcome.ID, event.ID, event.Event.Kind, relationErr)
				break
			}
			if dependency != nil {
				markReplicationDependency(outcome, *dependency)
				break
			}
			for _, reference := range replicationEventReferences(event.Event) {
				if owner := eventOwner[reference]; owner != "" && owner != key {
					outcome.dependencies = append(outcome.dependencies, owner)
				}
			}
			if isProposalKind(event.Event.Kind) {
				if owner := codeOwners[event.ID]; owner != "" && owner != key {
					outcome.dependencies = append(outcome.dependencies, owner)
				}
			}
		}
	}
	classifyReplicationMemoryDependencies(selection, quarantineDir, mainGitDir, acceptedEvents, outcomes)
	// Exclude dependents of quarantined suppliers before checking exact closure.
	propagateReplicationFailures(outcomes)
	projectionEvents := selectedProjectionEvents(acceptedEvents, outcomes)
	if err := validateActorChains(projectionEvents); err != nil {
		return result, replicationPhaseError(selection.Remote, "actor-chain projection")
	}
	if err := validateExactEventReferenceClosure(projectionEvents); err != nil {
		return result, replicationPhaseError(selection.Remote, "event-reference projection")
	}
	if _, err := validateIssueRelationships(projectionEvents); err != nil {
		return result, replicationPhaseError(selection.Remote, "issue-revision projection")
	}
	if _, err := ProjectIdentityContinuity(projectionEvents); err != nil {
		for _, outcome := range outcomes {
			if outcome.Status != replicationPromotable || outcome.Kind != replicationActor {
				continue
			}
			candidate := append(append([]StoredEvent(nil), acceptedEvents...), outcome.events...)
			if _, candidateErr := ProjectIdentityContinuity(candidate); candidateErr != nil {
				outcome.Status = replicationRelationshipBad
				outcome.Diagnostic = fmt.Sprintf("selection actor %s has invalid identity continuity: %v", outcome.ID, candidateErr)
			}
		}
		if _, retryErr := ProjectIdentityContinuity(selectedProjectionEvents(acceptedEvents, outcomes)); retryErr != nil {
			return result, replicationPhaseError(selection.Remote, "identity-continuity projection")
		}
	}
	propagateReplicationFailures(outcomes)
	revalidateReplicationMemoryObjectDependencies(selection, quarantineDir, mainGitDir, acceptedEvents, outcomes)
	propagateReplicationFailures(outcomes)

	keys := make([]string, 0, len(outcomes))
	for key := range outcomes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	promotions := make([]replicationPromotion, 0)
	roots := make([]string, 0)
	for _, key := range keys {
		outcome := outcomes[key]
		if outcome.Status == replicationPromotable {
			ref := acceptedActorRef(selection.Remote, outcome.ID)
			if outcome.Kind == replicationProposal {
				ref = acceptedProposalRef(selection.Remote, outcome.ID)
			} else if outcome.Kind == replicationMemory {
				actor, _, ok := parseMemoryRef(outcome.request.SourceRef)
				if !ok {
					return result, replicationPhaseError(selection.Remote, "memory promotion preparation")
				}
				var err error
				ref, err = acceptedMemoryRef(selection.Remote, actor, outcome.ID)
				if err != nil {
					return result, replicationPhaseError(selection.Remote, "memory promotion preparation")
				}
			}
			promotions = append(promotions, replicationPromotion{Ref: ref, NewOID: outcome.request.OID, OldOID: outcome.acceptedOld})
			roots = append(roots, outcome.request.OID)
		}
		result.Outcomes = append(result.Outcomes, *outcome)
	}
	sort.Slice(result.Outcomes, func(i, j int) bool {
		if result.Outcomes[i].Kind != result.Outcomes[j].Kind {
			return result.Outcomes[i].Kind < result.Outcomes[j].Kind
		}
		return result.Outcomes[i].ID < result.Outcomes[j].ID
	})
	requiredObjects, err := reachableObjectIDsAt(quarantineDir, roots)
	if err != nil {
		return result, replicationPhaseError(selection.Remote, "object-copy preparation")
	}
	pendingObjects, err := replicationObjectsAbsentFromMain(mainGitDir, requiredObjects)
	if err != nil {
		return result, replicationPhaseError(selection.Remote, "unaccepted-object inspection")
	}
	result.promotions = append([]replicationPromotion(nil), promotions...)
	result.pendingObjects = append([]string(nil), pendingObjects...)
	result.acceptedObjects = append([]string(nil), requiredObjects...)
	if err := createReplicationPendingAnchor(mainGitDir, result); err != nil {
		return result, replicationPhaseError(selection.Remote, "pending acceptance anchoring")
	}
	if err := runReplicationProcessHook("after-pending-anchor"); err != nil {
		return result, replicationResidueError(selection.Remote, "interrupted after pending anchor before object copy")
	}
	if err := replicationRecordTransaction(mainGitDir, result, "validated"); err != nil {
		return result, replicationPhaseError(selection.Remote, "validated transaction recording")
	}
	if err := replicationCopyObjects(quarantineDir, mainGitDir, sortedUniqueStrings(roots...)); err != nil {
		return result, replicationPhaseError(selection.Remote, "object copy")
	}
	for _, objectID := range requiredObjects {
		if _, err := gitOutputAt(mainGitDir, "cat-file", "-e", objectID+"^{object}"); err != nil {
			return result, replicationResidueError(selection.Remote, fmt.Sprintf("object verification failed at %s", objectID))
		}
	}
	if replicationAfterCopyHook != nil {
		if err := replicationAfterCopyHook(); err != nil {
			return result, replicationResidueError(selection.Remote, "interrupted after object copy")
		}
	}
	if err := replicationRemoveQuarantine(quarantineRoot, quarantineDir); err != nil {
		return result, replicationResidueError(selection.Remote, "quarantine cleanup failed")
	}
	cleanupNeeded = false
	if replicationBeforePromoteHook != nil {
		if err := replicationBeforePromoteHook(); err != nil {
			return result, replicationResidueError(selection.Remote, "interrupted immediately before accepted-ref transaction")
		}
	}
	if err := promoteReplicationRefs(promotions); err != nil {
		return result, replicationResidueError(selection.Remote, "accepted-ref transaction failed")
	}
	for index := range result.Outcomes {
		if result.Outcomes[index].Status == replicationPromotable {
			result.Outcomes[index].Status = replicationPromoted
			result.Promoted++
		}
	}
	if err := replicationReleaseShallow(mainGitDir, promotions); err != nil {
		return result, shallowReleasePartialSuccessError(selection.Remote, mainGitDir, quarantineRoot, quarantineDir, promotions, err)
	}
	if err := runReplicationProcessHook("before-completion-receipt"); err != nil {
		return result, fmt.Errorf("replication promotion succeeded for %d selection(s), but completion recording failed for remote %s; refs and shallow boundaries are committed, trust operations remain fail-closed; retry hn sync %s --recover-shallow", result.Promoted, selection.Remote, selection.Remote)
	}
	if err := replicationRecordTransaction(mainGitDir, result, "complete"); err != nil {
		return result, fmt.Errorf("replication promotion succeeded for %d selection(s), but completion recording failed for remote %s; refs and shallow boundaries are committed, trust operations remain fail-closed; retry hn sync %s --recover-shallow", result.Promoted, selection.Remote, selection.Remote)
	}
	if err := removeReplicationPendingAnchor(mainGitDir, result.ID); err != nil {
		return result, fmt.Errorf("replication promotion succeeded for %d selection(s), but completed pending-anchor cleanup failed for remote %s: %w", result.Promoted, selection.Remote, err)
	}
	if err := reconcileCompletedReplicationTransactions(mainGitDir, result); err != nil {
		return result, fmt.Errorf("replication promotion succeeded for %d selection(s), but pending transaction reconciliation failed for remote %s: %w", result.Promoted, selection.Remote, err)
	}
	return result, nil
}

func replicationObjectsAbsentFromMain(gitDir string, objectIDs []string) ([]string, error) {
	absent := make([]string, 0, len(objectIDs))
	for _, id := range sortedUniqueStrings(objectIDs...) {
		probe, err := probeExactGitObjectAt(gitDir, id)
		if err != nil {
			return nil, err
		}
		if !probe.Exists {
			absent = append(absent, id)
		}
	}
	return absent, nil
}

func replicationResidueError(remote, phase string) error {
	return fmt.Errorf("replication %s for remote %s before accepted-ref commit; accepted refs and trust projection are unchanged; unreferenced object residue may remain in the object database", phase, remote)
}

func shallowReleasePartialSuccessError(remote, mainGitDir, quarantineRoot, quarantineDir string, promotions []replicationPromotion, cause error) error {
	outcomes := make([]string, 0, len(promotions))
	changed := 0
	for _, promotion := range promotions {
		outcome := "same-head"
		if promotion.OldOID != promotion.NewOID {
			outcome = "advanced"
			changed++
		}
		outcomes = append(outcomes, promotion.Ref+"="+outcome)
	}
	sort.Strings(outcomes)
	diagnostic := redactReplicationDiagnostic(cause.Error(), mainGitDir, quarantineRoot, quarantineDir)
	return fmt.Errorf("replication ref transaction committed for remote %s with %d ref value change(s); outcomes: %s; required objects were imported, but shallow boundary release failed: %s; accepted fact projection remains fail-closed; retry hn sync %s --recover-shallow", remote, changed, strings.Join(outcomes, ","), diagnostic, remote)
}

func reachableObjectIDsAt(gitDir string, roots []string) ([]string, error) {
	seen := make(map[string]bool)
	objects := make([]string, 0)
	for _, root := range sortedUniqueStrings(roots...) {
		listed, err := gitTextAt(gitDir, "rev-list", "--objects", root)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(listed, "\n") {
			fields := strings.Fields(line)
			if len(fields) == 0 || seen[fields[0]] {
				continue
			}
			seen[fields[0]] = true
			objects = append(objects, fields[0])
		}
	}
	sort.Strings(objects)
	return objects, nil
}

func resolveReplicationRequests(selection ReplicationSelection, remoteURL string) ([]replicationRequest, []ReplicationOutcome, error) {
	advertisedText, err := gitText("ls-remote", "--refs", "--", remoteURL, "refs/hn/actors/*", "refs/hn/proposals/*", "refs/hn/memory/*/*")
	if err != nil {
		return nil, nil, replicationPhaseError(selection.Remote, "advertisement")
	}
	advertised := make(map[string]string)
	memoryRefs := make(map[string][]string)
	fields := strings.Fields(advertisedText)
	if len(fields)%2 != 0 {
		return nil, nil, replicationPhaseError(selection.Remote, "advertisement validation")
	}
	for index := 0; index < len(fields); index += 2 {
		oid, ref := fields[index], fields[index+1]
		if !validGitOID(oid) {
			return nil, nil, replicationPhaseError(selection.Remote, "advertisement validation")
		}
		advertised[ref] = oid
		if _, stream, ok := parseMemoryRef(ref); ok {
			memoryRefs[stream] = append(memoryRefs[stream], ref)
		}
	}
	wanted := make([]replicationRequest, 0)
	outcomes := make([]ReplicationOutcome, 0)
	if selection.All {
		refs := make([]string, 0, len(advertised))
		for ref := range advertised {
			refs = append(refs, ref)
		}
		sort.Strings(refs)
		reportedAmbiguousMemory := make(map[string]bool)
		for _, ref := range refs {
			switch {
			case strings.HasPrefix(ref, "refs/hn/actors/"):
				id := strings.TrimPrefix(ref, "refs/hn/actors/")
				if validActorFingerprint(id) {
					wanted = append(wanted, newReplicationRequest(replicationActor, id, ref, advertised[ref]))
				}
			case strings.HasPrefix(ref, "refs/hn/proposals/"):
				id := "sha256:" + strings.TrimPrefix(ref, "refs/hn/proposals/")
				if validEventID(id) {
					wanted = append(wanted, newReplicationRequest(replicationProposal, id, ref, advertised[ref]))
				}
			case strings.HasPrefix(ref, memoryRefPrefix):
				_, stream, ok := parseMemoryRef(ref)
				if ok && len(memoryRefs[stream]) == 1 {
					wanted = append(wanted, newReplicationRequest(replicationMemory, stream, ref, advertised[ref]))
				} else if ok && !reportedAmbiguousMemory[stream] {
					reportedAmbiguousMemory[stream] = true
					request := newReplicationRequest(replicationMemory, stream, "", "")
					outcomes = append(outcomes, failedReplicationOutcome(request, replicationStructuralInvalid,
						fmt.Sprintf("advertised memory %s has ambiguous owners on remote %s", stream, selection.Remote)))
				}
			}
		}
	} else {
		for _, actor := range selection.Actors {
			ref := actorRef(actor)
			wanted = append(wanted, newReplicationRequest(replicationActor, actor, ref, advertised[ref]))
		}
		for _, proposal := range selection.Proposals {
			ref := proposalRef(proposal)
			wanted = append(wanted, newReplicationRequest(replicationProposal, proposal, ref, advertised[ref]))
		}
		for _, stream := range selection.Memories {
			refs := memoryRefs[stream]
			if len(refs) == 1 {
				wanted = append(wanted, newReplicationRequest(replicationMemory, stream, refs[0], advertised[refs[0]]))
				continue
			}
			request := newReplicationRequest(replicationMemory, stream, "", "")
			if len(refs) > 1 {
				outcomes = append(outcomes, failedReplicationOutcome(request, replicationStructuralInvalid,
					fmt.Sprintf("selected memory %s has ambiguous owners on remote %s", stream, selection.Remote)))
				continue
			}
			wanted = append(wanted, request)
		}
	}
	requests := make([]replicationRequest, 0, len(wanted))
	for _, request := range wanted {
		if request.OID == "" {
			outcome := failedReplicationOutcome(request, replicationDependencyMissing,
				fmt.Sprintf("selected %s %s is not advertised by remote %s", request.Kind, request.ID, selection.Remote))
			outcome.MissingID = request.ID
			outcome.Recovery = "verify the full selection ID and retry when the exact ref is advertised"
			outcomes = append(outcomes, outcome)
			continue
		}
		requests = append(requests, request)
	}
	return requests, outcomes, nil
}

func newReplicationRequest(kind, id, sourceRef, oid string) replicationRequest {
	destination := "refs/hn/quarantine/actors/" + id
	if kind == replicationProposal {
		destination = "refs/hn/quarantine/proposals/" + strings.TrimPrefix(id, "sha256:")
	} else if kind == replicationMemory {
		actor, stream, ok := parseMemoryRef(sourceRef)
		if ok && stream == id {
			destination = "refs/hn/quarantine/memory/" + actor + "/" + strings.TrimPrefix(stream, "sha256:")
		} else {
			destination = "refs/hn/quarantine/memory/unresolved/" + strings.TrimPrefix(id, "sha256:")
		}
	}
	return replicationRequest{Kind: kind, ID: id, SourceRef: sourceRef, QuarantineRef: destination, OID: oid}
}

func acceptedRefForRequest(remote string, request replicationRequest) string {
	if request.Kind == replicationProposal {
		return acceptedProposalRef(remote, request.ID)
	}
	if request.Kind == replicationMemory {
		actor, stream, ok := parseMemoryRef(request.SourceRef)
		if !ok || stream != request.ID {
			return ""
		}
		ref, err := acceptedMemoryRef(remote, actor, stream)
		if err != nil {
			return ""
		}
		return ref
	}
	return acceptedActorRef(remote, request.ID)
}

func failedReplicationOutcome(request replicationRequest, status, diagnostic string) ReplicationOutcome {
	return ReplicationOutcome{Kind: request.Kind, ID: request.ID, Status: status, Diagnostic: safeDiagnostic(diagnostic), request: request}
}

func redactReplicationDiagnostic(message string, privateValues ...string) string {
	for _, privateValue := range privateValues {
		if privateValue != "" {
			message = strings.ReplaceAll(message, privateValue, "<private>")
		}
	}
	return safeDiagnostic(message)
}

func refValueAt(gitDir, ref string) (string, bool, error) {
	out, err := gitOutputAt(gitDir, "rev-parse", "--verify", "--quiet", ref)
	if err != nil {
		return "", false, nil
	}
	return strings.TrimSpace(string(out)), true, nil
}

func measureQuarantinedSelection(quarantineGitDir, mainGitDir, kind, root string) (ReplicationMeasurements, error) {
	unbounded := ReplicationBudgets{
		MaxEvents: 1 << 62, MaxObjects: 1 << 62, MaxObjectBytes: 1 << 62,
		MaxAttachmentBytes: 1 << 62, MaxTotalBytes: 1 << 62,
	}
	return measureQuarantinedSelectionUnderBudgets(quarantineGitDir, mainGitDir, kind, root, "", unbounded, kind+" "+root)
}

func measureQuarantinedSelectionUnderBudgets(quarantineGitDir, mainGitDir, kind, root, acceptedRoot string, budgets ReplicationBudgets, selectionID string) (ReplicationMeasurements, error) {
	// Budgets are per selected ref. Objects shared with another selection count
	// independently for both. Only objects reachable from this selection's
	// previous accepted value are discounted; unrelated or unreferenced objects
	// in the main object database cannot weaken a later budget check.
	if objectType, err := gitTextAt(quarantineGitDir, "cat-file", "-t", root); err != nil || objectType != "commit" {
		return ReplicationMeasurements{}, fmt.Errorf("selected root %s is not a commit", root)
	}
	measurements := ReplicationMeasurements{}
	if kind == replicationActor || kind == replicationMemory {
		count, err := gitTextAt(quarantineGitDir, "rev-list", "--count", root)
		if err != nil {
			return measurements, fmt.Errorf("count selected %s events: %w", kind, err)
		}
		measurements.Events, err = strconv.ParseInt(count, 10, 64)
		if err != nil || measurements.Events < 1 {
			return measurements, fmt.Errorf("invalid selected actor event count %q", count)
		}
		if err := enforceReplicationBudgets(selectionID, budgets, measurements); err != nil {
			return measurements, replicationBudgetExceeded{err}
		}
	}
	listed, err := gitTextAt(quarantineGitDir, "rev-list", "--objects", root)
	if err != nil {
		return ReplicationMeasurements{}, fmt.Errorf("enumerate selected graph: %w", err)
	}
	objects := make([]string, 0)
	seen := make(map[string]bool)
	previouslyAccepted := make(map[string]bool)
	if acceptedRoot != "" {
		previous, err := reachableObjectIDsAt(mainGitDir, []string{acceptedRoot})
		if err != nil {
			return measurements, fmt.Errorf("enumerate previous accepted graph: %w", err)
		}
		for _, objectID := range previous {
			previouslyAccepted[objectID] = true
		}
	}
	for _, line := range strings.Split(listed, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || seen[fields[0]] {
			continue
		}
		seen[fields[0]] = true
		if previouslyAccepted[fields[0]] {
			continue
		}
		objects = append(objects, fields[0])
	}
	measurements.Objects = int64(len(objects))
	if err := enforceReplicationBudgets(selectionID, budgets, measurements); err != nil {
		return measurements, replicationBudgetExceeded{err}
	}
	if len(objects) > 0 {
		batch, err := gitInputAt(quarantineGitDir, []byte(strings.Join(objects, "\n")+"\n"),
			"cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize)")
		if err != nil {
			return ReplicationMeasurements{}, fmt.Errorf("measure selected graph: %w", err)
		}
		scanner := bufio.NewScanner(strings.NewReader(string(batch)))
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) != 3 || fields[1] == "missing" {
				return ReplicationMeasurements{}, fmt.Errorf("unreadable reachable object %q", scanner.Text())
			}
			size, err := strconv.ParseInt(fields[2], 10, 64)
			if err != nil || size < 0 {
				return ReplicationMeasurements{}, fmt.Errorf("invalid object size for %s", fields[0])
			}
			measurements.TotalBytes += size
			if size > measurements.LargestObjectBytes {
				measurements.LargestObjectBytes = size
			}
			if err := enforceReplicationBudgets(selectionID, budgets, measurements); err != nil {
				return measurements, replicationBudgetExceeded{err}
			}
		}
		if err := scanner.Err(); err != nil {
			return ReplicationMeasurements{}, err
		}
	}
	if kind == replicationActor {
		events, err := loadActorEventsAt(quarantineGitDir, root)
		if err != nil {
			return ReplicationMeasurements{}, err
		}
		for _, event := range events {
			for _, attachment := range event.Attachments {
				size := int64(len(attachment))
				if size > measurements.LargestAttachmentBytes {
					measurements.LargestAttachmentBytes = size
				}
				if err := enforceReplicationBudgets(selectionID, budgets, measurements); err != nil {
					return measurements, replicationBudgetExceeded{err}
				}
			}
		}
	}
	return measurements, nil
}

func validateQuarantinedActor(gitDir, actor, head string) ([]StoredEvent, error) {
	events, err := loadActorEventsAt(gitDir, head)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("actor history is empty")
	}
	for _, event := range events {
		if event.Event.Actor != actor {
			return nil, fmt.Errorf("event %s belongs to actor %s", event.ID, event.Event.Actor)
		}
		if err := validateQuarantinedEventTree(gitDir, event); err != nil {
			return nil, err
		}
	}
	for index, event := range events {
		parents, err := gitTextAt(gitDir, "show", "-s", "--format=%P", event.Commit)
		if err != nil {
			return nil, fmt.Errorf("inspect event commit parents for %s: %w", event.ID, err)
		}
		want := ""
		if index > 0 {
			want = events[index-1].Commit
		}
		if parents != want {
			return nil, fmt.Errorf("event %s commit parent does not match its actor predecessor", event.ID)
		}
	}
	if err := validateActorChains(events); err != nil {
		return nil, err
	}
	return events, nil
}

func validateQuarantinedEventTree(gitDir string, event StoredEvent) error {
	contents, err := gitOutputAt(gitDir, "ls-tree", "-z", event.Commit)
	if err != nil {
		return fmt.Errorf("inspect event tree %s: %w", event.ID, err)
	}
	names := make(map[string]bool)
	for _, entry := range strings.Split(string(contents), "\x00") {
		if entry == "" {
			continue
		}
		separator := strings.IndexByte(entry, '\t')
		if separator < 0 {
			return fmt.Errorf("event %s has malformed tree entry", event.ID)
		}
		metadata, name := strings.Fields(entry[:separator]), entry[separator+1:]
		if len(metadata) != 3 || metadata[0] != "100644" || metadata[1] != "blob" || strings.Contains(name, "/") || names[name] {
			return fmt.Errorf("event %s has unsupported tree entry %q", event.ID, name)
		}
		names[name] = true
	}
	want := map[string]bool{"event.json": true, "signature": true}
	if event.Event.Kind == "run.result" {
		want["log.txt"] = true
	}
	if len(names) != len(want) {
		return fmt.Errorf("event %s has unsupported attachment or missing signed material", event.ID)
	}
	for name := range want {
		if !names[name] {
			return fmt.Errorf("event %s is missing %s", event.ID, name)
		}
	}
	return nil
}

func acceptedProposalHeads() (map[string]string, map[string]string, error) {
	text, err := gitText("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/proposals", "refs/hn/remotes")
	if err != nil {
		return nil, nil, err
	}
	heads := make(map[string]string)
	owners := make(map[string]string)
	fields := strings.Fields(text)
	for index := 0; index+1 < len(fields); index += 2 {
		ref, oid := fields[index], fields[index+1]
		var id string
		if strings.HasPrefix(ref, "refs/hn/proposals/") {
			id = "sha256:" + strings.TrimPrefix(ref, "refs/hn/proposals/")
			if !validEventID(id) {
				continue
			}
		} else if _, parsedID, ok := parseAcceptedProposalRef(ref); ok {
			id = parsedID
		} else {
			continue
		}
		if prior := heads[id]; prior != "" && prior != oid {
			return nil, nil, fmt.Errorf("accepted proposal %s has conflicting code refs", id)
		}
		heads[id] = oid
	}
	return heads, owners, nil
}

func replicationEventDependency(gitDir, acceptedGitDir string, event StoredEvent, byID map[string]StoredEvent, codeHeads, eventOwners, codeOwners map[string]string) (*replicationDependency, error) {
	require := func(id, description string, predicate func(StoredEvent) bool) (*replicationDependency, error) {
		stored, exists := byID[id]
		if !exists {
			return &replicationDependency{EventID: event.ID, EventKind: event.Event.Kind, Missing: id,
				Recovery: "select the full actor history supplying " + id, Key: eventOwners[id], Reason: description + " is absent"}, nil
		}
		if !predicate(stored) {
			return nil, fmt.Errorf("referenced %s %s has the wrong event kind", description, id)
		}
		return nil, nil
	}
	e := event.Event
	if isIssueKind(e.Kind) {
		for _, reference := range issueReferences(e) {
			if _, exists := byID[reference]; !exists {
				return &replicationDependency{EventID: event.ID, EventKind: e.Kind, Missing: reference,
					Recovery: "select the full actor history supplying " + reference, Key: eventOwners[reference], Reason: "issue dependency is absent"}, nil
			}
		}
		return nil, validateIssueReferences(event, byID)
	}
	if isProposalKind(e.Kind) {
		head, exists := codeHeads[event.ID]
		if !exists {
			return &replicationDependency{EventID: event.ID, EventKind: e.Kind, Missing: event.ID,
				Recovery: "add exact proposal selection --proposal " + event.ID, Key: codeOwners[event.ID], Reason: "proposal code ref is absent"}, nil
		}
		if head != e.Head {
			return nil, fmt.Errorf("proposal code ref/head mismatch: event=%s ref=%s", e.Head, head)
		}
		_, quarantineAncestryErr := gitOutputAt(gitDir, "merge-base", "--is-ancestor", e.Base, e.Head)
		_, acceptedAncestryErr := gitOutputAt(acceptedGitDir, "merge-base", "--is-ancestor", e.Base, e.Head)
		if quarantineAncestryErr != nil && acceptedAncestryErr != nil {
			return nil, fmt.Errorf("proposal head %s does not descend from base %s", e.Head, e.Base)
		}
	}
	switch e.Kind {
	case "issue.comment":
		return require(e.Subject, "issue", func(value StoredEvent) bool { return value.Event.Kind == "issue.open" })
	case "proposal.revise":
		dependency, err := require(e.Subject, "proposal predecessor", func(value StoredEvent) bool { return isProposalKind(value.Event.Kind) })
		if err == nil && dependency == nil && byID[e.Subject].Event.Actor != e.Actor {
			return nil, fmt.Errorf("revision is not signed by predecessor author %s", byID[e.Subject].Event.Actor)
		}
		return dependency, err
	case "review.submit":
		return require(e.Subject, "proposal", func(value StoredEvent) bool { return isProposalKind(value.Event.Kind) })
	case "run.request":
		dependency, err := require(e.Subject, "proposal", func(value StoredEvent) bool { return isProposalKind(value.Event.Kind) })
		if err == nil && dependency == nil && byID[e.Subject].Event.Head != e.Commit {
			return nil, fmt.Errorf("run request commit does not match proposal head")
		}
		return dependency, err
	case "run.result":
		dependency, err := require(e.Subject, "run request", func(value StoredEvent) bool { return value.Event.Kind == "run.request" })
		if err == nil && dependency == nil {
			request := byID[e.Subject].Event
			if request.Commit != e.Commit || request.Pipeline != e.Pipeline || request.Definition != e.Definition {
				return nil, fmt.Errorf("run result does not match request")
			}
		}
		return dependency, err
	case "proposal.decision", "proposal.merged":
		dependency, err := require(e.Subject, "proposal", func(value StoredEvent) bool { return isProposalKind(value.Event.Kind) })
		if err != nil || dependency != nil {
			return dependency, err
		}
		for _, evidence := range e.Evidence {
			if _, exists := byID[evidence]; !exists {
				return &replicationDependency{EventID: event.ID, EventKind: e.Kind, Missing: evidence,
					Recovery: "select the full actor history supplying " + evidence, Key: eventOwners[evidence], Reason: "evidence is absent"}, nil
			}
		}
		if e.Kind == "proposal.decision" {
			return nil, validateReplicationDecisionAt(gitDir, acceptedGitDir, event, byID[e.Subject], byID)
		}
		if err := validateProposalMergeBinding(event, byID[e.Subject]); err != nil {
			return nil, err
		}
		return nil, validateReplicationMergeAt(gitDir, acceptedGitDir, event, byID[e.Subject], byID)
	case "identity.accept":
		dependency, err := require(e.Subject, "identity authorization", func(value StoredEvent) bool { return value.Event.Kind == "identity.authorize" })
		if err == nil && dependency == nil {
			authorization := byID[e.Subject].Event
			if e.Actor != authorization.TargetActor || e.PublicKey != authorization.TargetKey {
				return nil, fmt.Errorf("identity acceptance is not signed by authorization target")
			}
		}
		return dependency, err
	}
	return nil, nil
}

func replicationEventReferences(event Event) []string {
	references := append([]string(nil), event.Evidence...)
	references = append(references, issueReferences(event)...)
	switch event.Kind {
	case "issue.comment", "proposal.revise", "review.submit", "run.request", "run.result", "proposal.decision", "proposal.merged", "identity.accept":
		references = append(references, event.Subject)
	}
	return sortedUniqueStrings(references...)
}

func loadReplicationPolicyAt(gitDir, acceptedGitDir, commit string) (PolicyDocument, string, error) {
	encoded, err := gitOutputAt(gitDir, "show", commit+":.hn/policy.json")
	if err != nil {
		encoded, err = gitOutputAt(acceptedGitDir, "show", commit+":.hn/policy.json")
		if err != nil {
			return PolicyDocument{}, "", fmt.Errorf("no .hn/policy.json at commit %s", commit)
		}
	}
	policy, digest, err := parsePolicyBytes(encoded)
	return policy, digest, err
}

func replicationPipelineDigestAt(gitDir, acceptedGitDir, commit, name string) (string, error) {
	if !validPipelineName(name) {
		return "", fmt.Errorf("invalid pipeline name %q", name)
	}
	encoded, err := gitOutputAt(gitDir, "show", commit+":"+".hn/pipelines/"+name+".json")
	if err != nil {
		encoded, err = gitOutputAt(acceptedGitDir, "show", commit+":"+".hn/pipelines/"+name+".json")
		if err != nil {
			return "", fmt.Errorf("pipeline %q does not exist at commit %s", name, commit)
		}
	}
	if len(encoded) > maxPipelineSize {
		return "", fmt.Errorf("pipeline %q exceeds %d bytes", name, maxPipelineSize)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var pipeline PipelineDefinition
	if err := decoder.Decode(&pipeline); err != nil {
		return "", fmt.Errorf("parse pipeline %q: %w", name, err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return "", fmt.Errorf("parse pipeline %q: %w", name, err)
	}
	if err := validatePipeline(pipeline); err != nil {
		return "", fmt.Errorf("pipeline %q: %w", name, err)
	}
	return eventID(encoded), nil
}

func validateReplicationDecisionAt(gitDir, acceptedGitDir string, decision, proposal StoredEvent, byID map[string]StoredEvent) error {
	policy, digest, err := loadReplicationPolicyAt(gitDir, acceptedGitDir, proposal.Event.Base)
	if err != nil {
		return fmt.Errorf("decision %s: %w", decision.ID, err)
	}
	if decision.Event.Policy != digest || !actorListed(decision.Event.Actor, policy.Maintainers) {
		return fmt.Errorf("decision %s is not authorized by exact base policy %s", decision.ID, digest)
	}
	if decision.Event.Verdict == "reject" {
		return nil
	}
	approvers := make(map[string]bool)
	pipelineRunners := make(map[string]map[string]bool)
	pipelineDefinitions := make(map[string]string)
	for name := range policy.Pipelines {
		pipelineRunners[name] = make(map[string]bool)
		definition, err := replicationPipelineDigestAt(gitDir, acceptedGitDir, proposal.Event.Head, name)
		if err != nil {
			return fmt.Errorf("decision %s required pipeline %q: %w", decision.ID, name, err)
		}
		pipelineDefinitions[name] = definition
	}
	for _, evidenceID := range decision.Event.Evidence {
		evidence := byID[evidenceID]
		switch evidence.Event.Kind {
		case "review.submit":
			if evidence.Event.Subject == proposal.ID && evidence.Event.Verdict == "approve" && actorListed(evidence.Event.Actor, policy.Proposals.TrustedReviewers) &&
				(policy.Proposals.AllowAuthorApproval || evidence.Event.Actor != proposal.Event.Actor) {
				approvers[evidence.Event.Actor] = true
			}
		case "run.result":
			request, exists := byID[evidence.Event.Subject]
			if !exists || request.Event.Kind != "run.request" || request.Event.Subject != proposal.ID ||
				!actorListed(request.Event.Actor, policy.Maintainers) || evidence.Event.Outcome != "passed" {
				continue
			}
			pipelinePolicy, exists := policy.Pipelines[request.Event.Pipeline]
			if exists && request.Event.Definition == pipelineDefinitions[request.Event.Pipeline] && actorListed(evidence.Event.Actor, pipelinePolicy.TrustedRunners) {
				pipelineRunners[request.Event.Pipeline][evidence.Event.Actor] = true
			}
		}
	}
	if len(approvers) < policy.Proposals.RequiredApprovals {
		return fmt.Errorf("decision %s lacks required approval evidence", decision.ID)
	}
	for name, pipelinePolicy := range policy.Pipelines {
		if len(pipelineRunners[name]) < pipelinePolicy.RequiredResults {
			return fmt.Errorf("decision %s lacks required %s results", decision.ID, name)
		}
	}
	return nil
}

func validateReplicationMergeAt(gitDir, acceptedGitDir string, merge, proposal StoredEvent, byID map[string]StoredEvent) error {
	policy, digest, err := loadReplicationPolicyAt(gitDir, acceptedGitDir, proposal.Event.Base)
	if err != nil {
		return fmt.Errorf("merge %s: %w", merge.ID, err)
	}
	if merge.Event.Policy != digest || !actorListed(merge.Event.Actor, policy.Maintainers) {
		return fmt.Errorf("merge %s is not authorized by exact base policy %s", merge.ID, digest)
	}
	acceptors := make(map[string]bool)
	for _, evidenceID := range merge.Event.Evidence {
		decision, exists := byID[evidenceID]
		if !exists || decision.Event.Kind != "proposal.decision" || decision.Event.Subject != proposal.ID ||
			decision.Event.Policy != digest || decision.Event.Verdict != "accept" || !actorListed(decision.Event.Actor, policy.Maintainers) {
			continue
		}
		acceptors[decision.Event.Actor] = true
	}
	if len(acceptors) < policy.Proposals.RequiredAccepts {
		return fmt.Errorf("merge %s lacks required acceptance evidence", merge.ID)
	}
	return nil
}

func markReplicationDependency(outcome *ReplicationOutcome, dependency replicationDependency) {
	outcome.Status = replicationDependencyMissing
	outcome.MissingID = dependency.Missing
	outcome.Recovery = dependency.Recovery
	outcome.Diagnostic = fmt.Sprintf("selection %s %s event %s (%s) missing dependency %s: %s; recovery: %s",
		outcome.Kind, outcome.ID, dependency.EventID, dependency.EventKind, dependency.Missing, dependency.Reason, dependency.Recovery)
	if dependency.Key != "" {
		outcome.dependencies = append(outcome.dependencies, dependency.Key)
	}
}

func propagateReplicationFailures(outcomes map[string]*ReplicationOutcome) {
	changed := true
	for changed {
		changed = false
		for _, outcome := range outcomes {
			if outcome.Status != replicationPromotable {
				continue
			}
			for _, dependency := range outcome.dependencies {
				owner := outcomes[dependency]
				if owner != nil && owner.Status != replicationPromotable {
					outcome.Status = replicationDependencyMissing
					outcome.MissingID = owner.ID
					outcome.Recovery = "repair or replace exact selection " + owner.ID
					outcome.Diagnostic = fmt.Sprintf("selection %s %s depends on failed %s %s", outcome.Kind, outcome.ID, owner.Kind, owner.ID)
					changed = true
					break
				}
			}
		}
	}
}

func mapStoredEvents(byID map[string]StoredEvent) []StoredEvent {
	events := make([]StoredEvent, 0, len(byID))
	for _, event := range byID {
		events = append(events, event)
	}
	sortStoredEvents(events)
	return events
}

func selectedProjectionEvents(accepted []StoredEvent, outcomes map[string]*ReplicationOutcome) []StoredEvent {
	byID := make(map[string]StoredEvent, len(accepted))
	for _, event := range accepted {
		byID[event.ID] = event
	}
	for _, outcome := range outcomes {
		if outcome.Status != replicationPromotable || outcome.Kind != replicationActor {
			continue
		}
		for _, event := range outcome.events {
			byID[event.ID] = event
		}
	}
	return mapStoredEvents(byID)
}

type replicationMemoryResolver struct {
	quarantineGitDir string
	acceptedGitDir   string
}

func (resolver replicationMemoryResolver) Probe(object string) (gitObjectProbe, error) {
	probe, err := probeExactGitObjectAt(resolver.quarantineGitDir, object)
	if err != nil || probe.Exists {
		return probe, err
	}
	if validFullGitObjectID(object) {
		unaccepted, err := replicationObjectIsUnaccepted(resolver.acceptedGitDir, object)
		if err != nil || unaccepted {
			return gitObjectProbe{}, err
		}
	}
	return probeExactGitObjectAt(resolver.acceptedGitDir, object)
}

func (resolver replicationMemoryResolver) objectDirectory(object string) (string, bool, error) {
	for _, directory := range []string{resolver.quarantineGitDir, resolver.acceptedGitDir} {
		probe, err := probeExactGitObjectAt(directory, object)
		if err != nil {
			return "", false, err
		}
		if probe.Exists {
			if directory == resolver.acceptedGitDir && validFullGitObjectID(object) {
				unaccepted, err := replicationObjectIsUnaccepted(directory, object)
				if err != nil {
					return "", false, err
				}
				if unaccepted {
					continue
				}
			}
			return directory, true, nil
		}
	}
	return "", false, nil
}

func (resolver replicationMemoryResolver) TreeEntry(commit, path string) (string, bool, error) {
	directory, exists, err := resolver.objectDirectory(commit)
	if err != nil || !exists {
		return "", false, err
	}
	output, err := gitTextAt(directory, "ls-tree", commit, "--", path)
	if err != nil {
		return "", false, err
	}
	if output == "" {
		return "", false, nil
	}
	fields := strings.Fields(output)
	if len(fields) < 3 || fields[1] != "blob" || !validFullGitObjectID(fields[2]) {
		return "", false, fmt.Errorf("invalid Git tree entry for memory anchor")
	}
	return fields[2], true, nil
}

func (resolver replicationMemoryResolver) IsAncestor(ancestor, descendant string) (bool, string, error) {
	pending := []string{descendant}
	seen := make(map[string]bool)
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
		directory, exists, err := resolver.objectDirectory(commit)
		if err != nil {
			return false, "", err
		}
		if !exists {
			if firstMissing == "" {
				firstMissing = commit
			}
			continue
		}
		contents, err := gitTextAt(directory, "cat-file", "commit", commit)
		if err != nil {
			return false, "", err
		}
		for _, line := range strings.Split(contents, "\n") {
			if line == "" {
				break
			}
			if strings.HasPrefix(line, "parent ") {
				parent := strings.TrimPrefix(line, "parent ")
				if !validFullGitObjectID(parent) {
					return false, "", fmt.Errorf("malformed memory anchor ancestry")
				}
				pending = append(pending, parent)
			}
		}
	}
	return false, firstMissing, nil
}

func classifyReplicationMemoryDependencies(selection ReplicationSelection, quarantineGitDir, acceptedGitDir string, acceptedEvents []StoredEvent, outcomes map[string]*ReplicationOutcome) {
	acceptedMemories, err := collectMemories()
	if err != nil {
		for _, outcome := range outcomes {
			if outcome.Status == replicationPromotable && outcome.Kind == replicationMemory {
				outcome.Status = replicationRelationshipBad
				outcome.Diagnostic = fmt.Sprintf("selection memory %s cannot inspect accepted memory projection", outcome.ID)
			}
		}
		return
	}
	memories := append([]StoredMemory(nil), acceptedMemories...)
	ownerByMemory := make(map[string]string)
	ownerByEvent := make(map[string]string)
	for key, outcome := range outcomes {
		if outcome.Status != replicationPromotable || outcome.Kind != replicationActor {
			continue
		}
		for _, stored := range outcome.events {
			ownerByEvent[stored.ID] = key
		}
	}
	for key, outcome := range outcomes {
		if outcome.Status != replicationPromotable || outcome.Kind != replicationMemory {
			continue
		}
		for _, stored := range outcome.memories {
			memories = append(memories, stored)
			ownerByMemory[stored.ID] = key
		}
	}
	for key, outcome := range outcomes {
		if outcome.Status != replicationPromotable || outcome.Kind != replicationMemory {
			continue
		}
		for _, stored := range outcome.memories {
			if targetOwner := ownerByMemory[stored.Envelope.Target]; targetOwner != "" && targetOwner != key {
				outcome.dependencies = append(outcome.dependencies, targetOwner)
			}
			evidence := append([]string(nil), stored.Envelope.Evidence...)
			if stored.Envelope.Record != nil {
				evidence = append(evidence, stored.Envelope.Record.Evidence...)
			}
			for _, typed := range evidence {
				kind, id, ok := strings.Cut(typed, ":")
				if !ok {
					continue
				}
				dependencyOwner := ""
				switch kind {
				case "memory":
					dependencyOwner = ownerByMemory[id]
				case "event":
					dependencyOwner = ownerByEvent[id]
				}
				if dependencyOwner != "" && dependencyOwner != key {
					outcome.dependencies = append(outcome.dependencies, dependencyOwner)
				}
			}
		}
		outcome.dependencies = sortedUniqueStrings(outcome.dependencies...)
	}
	events := selectedProjectionEvents(acceptedEvents, outcomes)
	projection := ProjectMemories(memories, MemoryProjectionContext{
		Events: events,
		Resolver: replicationMemoryResolver{
			quarantineGitDir: quarantineGitDir,
			acceptedGitDir:   acceptedGitDir,
		},
	})
	for _, diagnostic := range projection.Diagnostics {
		key := ownerByMemory[diagnostic.MemoryID]
		outcome := outcomes[key]
		if outcome == nil || outcome.Status != replicationPromotable {
			continue
		}
		outcome.Status = replicationRelationshipBad
		outcome.Diagnostic = fmt.Sprintf("selection memory %s memory %s (%s) has invalid relationship %s", outcome.ID, diagnostic.MemoryID, diagnostic.Operation, diagnostic.Code)
	}
	for _, row := range projection.Rows {
		key := ownerByMemory[row.ID]
		outcome := outcomes[key]
		if outcome == nil || outcome.Status != replicationPromotable {
			continue
		}
		if row.Applicability == memoryApplicabilityAnchorInvalid || row.Evidence == memoryEvidenceInvalid {
			outcome.Status = replicationRelationshipBad
			outcome.Diagnostic = fmt.Sprintf("selection memory %s memory %s has invalid anchor or evidence relationship", outcome.ID, row.ID)
		}
	}
	for _, missing := range projection.MissingDependencies {
		key := ownerByMemory[missing.OwnerID]
		outcome := outcomes[key]
		if outcome == nil || outcome.Status != replicationPromotable {
			continue
		}
		outcome.Status = replicationDependencyMissing
		outcome.DependencyKind = missing.Kind
		outcome.MissingID = missing.MissingID
		outcome.OwnerMemoryID = missing.OwnerID
		outcome.OwnerStream = missing.Stream
		outcome.RequiredSelectors, outcome.RequiredRef = memoryReplicationRequiredSupplier(selection, acceptedGitDir, acceptedEvents, missing)
		outcome.Recovery = memoryReplicationRecovery(selection, missing, outcome.RequiredSelectors)
		outcome.Diagnostic = fmt.Sprintf("selection memory %s memory %s missing %s dependency %s: %s; recovery: %s", outcome.ID, missing.OwnerID, missing.Kind, missing.MissingID, missing.Reason, outcome.Recovery)
		if err := recordReplicationMemoryShallowGap(selection, outcome, missing); err != nil {
			outcome.Diagnostic += "; durable shallow gap recording failed"
		}
	}
}

func revalidateReplicationMemoryObjectDependencies(selection ReplicationSelection, quarantineGitDir, acceptedGitDir string, acceptedEvents []StoredEvent, outcomes map[string]*ReplicationOutcome) {
	available := make(map[string]bool)
	for _, outcome := range outcomes {
		if outcome.Status != replicationPromotable {
			continue
		}
		objects, err := reachableObjectIDsAt(quarantineGitDir, []string{outcome.request.OID})
		if err != nil {
			continue
		}
		for _, object := range objects {
			available[object] = true
		}
	}
	isAvailable := func(object string) bool {
		if available[object] {
			return true
		}
		if validFullGitObjectID(object) {
			unaccepted, err := replicationObjectIsUnaccepted(acceptedGitDir, object)
			if err != nil || unaccepted {
				return false
			}
		}
		probe, err := probeExactGitObjectAt(acceptedGitDir, object)
		return err == nil && probe.Exists
	}
	for _, outcome := range outcomes {
		if outcome.Status != replicationPromotable || outcome.Kind != replicationMemory {
			continue
		}
		required := make(map[string]MemoryDependency)
		for _, stored := range outcome.memories {
			if stored.Envelope.Record != nil {
				record := stored.Envelope.Record
				required[record.Anchor.Commit] = MemoryDependency{Kind: "anchor-commit", OwnerID: stored.ID, Stream: stored.Envelope.Stream, MissingID: record.Anchor.Commit, Reason: "anchor-commit-unavailable"}
				for _, anchored := range record.Anchor.Paths {
					if anchored.Blob != "absent" {
						required[anchored.Blob] = MemoryDependency{Kind: "anchor-path", OwnerID: stored.ID, Stream: stored.Envelope.Stream, MissingID: anchored.Blob, Reason: "anchor-path-object-unavailable"}
					}
				}
				for _, typed := range record.Evidence {
					if strings.HasPrefix(typed, "git:") {
						id := strings.TrimPrefix(typed, "git:")
						required[id] = MemoryDependency{Kind: "evidence-git", OwnerID: stored.ID, Stream: stored.Envelope.Stream, MissingID: id, Reason: "object-unavailable"}
					}
				}
			}
			for _, typed := range stored.Envelope.Evidence {
				if strings.HasPrefix(typed, "git:") {
					id := strings.TrimPrefix(typed, "git:")
					required[id] = MemoryDependency{Kind: "evidence-git", OwnerID: stored.ID, Stream: stored.Envelope.Stream, MissingID: id, Reason: "object-unavailable"}
				}
			}
		}
		objects := make([]string, 0, len(required))
		for object := range required {
			objects = append(objects, object)
		}
		sort.Strings(objects)
		for _, object := range objects {
			if isAvailable(object) {
				continue
			}
			missing := required[object]
			outcome.Status = replicationDependencyMissing
			outcome.DependencyKind = missing.Kind
			outcome.MissingID = object
			outcome.OwnerMemoryID = missing.OwnerID
			outcome.OwnerStream = missing.Stream
			outcome.RequiredSelectors, outcome.RequiredRef = memoryReplicationRequiredSupplier(selection, acceptedGitDir, acceptedEvents, missing)
			outcome.Recovery = memoryReplicationRecovery(selection, missing, outcome.RequiredSelectors)
			outcome.Diagnostic = fmt.Sprintf("selection memory %s has unavailable exact Git dependency %s; recovery: %s", outcome.ID, object, outcome.Recovery)
			if err := recordReplicationMemoryShallowGap(selection, outcome, missing); err != nil {
				outcome.Diagnostic += "; durable shallow gap recording failed"
			}
			break
		}
	}
}

func memoryReplicationRequiredSupplier(selection ReplicationSelection, gitDir string, events []StoredEvent, dependency MemoryDependency) ([]string, string) {
	for _, event := range events {
		if isProposalKind(event.Event.Kind) && event.Event.Head == dependency.MissingID {
			return []string{replicationProposal + ":" + event.ID}, proposalRef(event.ID)
		}
	}
	remoteURL, err := gitText("remote", "get-url", selection.Remote)
	if err != nil {
		return nil, ""
	}
	advertised, err := gitText("ls-remote", "--refs", "--", remoteURL, "refs/hn/actors/*", "refs/hn/memory/*/*")
	if err != nil {
		return nil, ""
	}
	fields := strings.Fields(advertised)
	if len(fields)%2 != 0 {
		return nil, ""
	}
	for index := 0; index < len(fields); index += 2 {
		head, ref := fields[index], fields[index+1]
		probe, err := probeExactGitObjectAt(gitDir, head)
		if err != nil || !probe.Exists || probe.Type != "commit" {
			continue
		}
		switch dependency.Kind {
		case "evidence-event":
			actor := strings.TrimPrefix(ref, "refs/hn/actors/")
			if !validActorFingerprint(actor) || ref != actorRef(actor) {
				continue
			}
			events, err := loadActorEventsAt(gitDir, head)
			if err != nil {
				continue
			}
			for _, event := range events {
				if event.ID == dependency.MissingID {
					return []string{replicationActor + ":" + actor}, ref
				}
			}
		case "lifecycle-target", "evidence-memory":
			actor, stream, ok := parseMemoryRef(ref)
			if !ok {
				continue
			}
			memories, err := loadMemoryStreamAt(gitDir, memoryStreamSource{Ref: ref, Actor: actor, Stream: stream, Head: head})
			if err != nil {
				continue
			}
			for _, memory := range memories {
				if memory.ID == dependency.MissingID {
					return []string{replicationMemory + ":" + stream}, ref
				}
			}
		}
	}
	return nil, ""
}

func memoryReplicationRecovery(selection ReplicationSelection, dependency MemoryDependency, required []string) string {
	shallow, _ := repositoryIsShallow()
	missing := make([]string, 0)
	for _, selector := range required {
		kind, id, ok := strings.Cut(selector, ":")
		if !ok {
			continue
		}
		selected := selection.All
		switch kind {
		case replicationActor:
			selected = selected || stringInSlice(id, selection.Actors)
		case replicationProposal:
			selected = selected || stringInSlice(id, selection.Proposals)
		case replicationMemory:
			selected = selected || stringInSlice(id, selection.Memories)
		}
		if !selected {
			missing = append(missing, "--"+kind+" "+id)
		}
	}
	if len(required) == 0 {
		return "supplier for exact " + dependency.Kind + " dependency " + dependency.MissingID + " is not derivable from signed facts; select its exact supplier, then retry"
	}
	if len(missing) != 0 {
		sync := "hn sync " + selection.Remote
		if shallow {
			sync += " --recover-shallow"
		}
		return "hn replication select " + selection.Remote + " " + strings.Join(missing, " ") + " (preserve existing selectors and budgets), then " + sync
	}
	if shallow {
		return "hn sync " + selection.Remote + " --recover-shallow"
	}
	return "repair the exact advertised dependency, then hn sync " + selection.Remote
}

func recordReplicationMemoryShallowGap(selection ReplicationSelection, outcome *ReplicationOutcome, dependency MemoryDependency) error {
	shallow, err := repositoryIsShallow()
	if err != nil {
		return err
	}
	if !shallow {
		return nil
	}
	gap := &ShallowDependencyGap{
		Operation: "memory replication", Kind: memoryShallowDependencyKind(dependency.Kind),
		MissingID: dependency.MissingID, OwnerKind: replicationMemory, OwnerID: dependency.Stream,
		OwnerMemoryID: dependency.OwnerID, OwnerStream: dependency.Stream,
		Remote: selection.Remote, RequiredRef: outcome.RequiredRef,
		RequiredSelectors: append([]string(nil), outcome.RequiredSelectors...),
		Recovery:          outcome.Recovery, Cause: fmt.Errorf("required exact memory dependency %s is unavailable", dependency.MissingID),
	}
	if dependency.Kind == "anchor-commit" || dependency.Kind == "anchor-path" || dependency.Kind == "evidence-git" {
		gap.Objectish = dependency.MissingID
		gap.ObjectType = "object"
		if dependency.Kind == "anchor-commit" {
			gap.ObjectType = "commit"
		}
	}
	return recordShallowDependencyGap(gap)
}

func removeGeneratedQuarantine(root, quarantine string) error {
	resolvedRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	resolved, err := filepath.Abs(quarantine)
	if err != nil {
		return err
	}
	if filepath.Dir(resolved) != resolvedRoot || !strings.HasPrefix(filepath.Base(resolved), "txn-") {
		return fmt.Errorf("refuse to remove non-generated quarantine path")
	}
	info, err := os.Lstat(resolved)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("generated quarantine path is not a directory")
	}
	return os.RemoveAll(resolved)
}

func promoteReplicationRefs(promotions []replicationPromotion) error {
	if len(promotions) == 0 {
		return nil
	}
	sort.Slice(promotions, func(i, j int) bool { return promotions[i].Ref < promotions[j].Ref })
	var input strings.Builder
	input.WriteString("start\n")
	for _, promotion := range promotions {
		_, _, actorDestination := parseAcceptedActorRef(promotion.Ref)
		_, _, proposalDestination := parseAcceptedProposalRef(promotion.Ref)
		_, _, _, memoryDestination := parseAcceptedMemoryRef(promotion.Ref)
		if (!actorDestination && !proposalDestination && !memoryDestination) || !validGitOID(promotion.NewOID) {
			return fmt.Errorf("invalid replication promotion")
		}
		if promotion.OldOID == "" {
			fmt.Fprintf(&input, "create %s %s\n", promotion.Ref, promotion.NewOID)
		} else {
			if !validGitOID(promotion.OldOID) {
				return fmt.Errorf("invalid expected old object for %s", promotion.Ref)
			}
			fmt.Fprintf(&input, "update %s %s %s\n", promotion.Ref, promotion.NewOID, promotion.OldOID)
		}
	}
	input.WriteString("prepare\ncommit\n")
	if _, err := gitInput([]byte(input.String()), nil, "update-ref", "--stdin"); err != nil {
		return fmt.Errorf("atomically promote accepted refs: %w", err)
	}
	return nil
}

func recordReplicationTransaction(mainGitDir string, result replicationTransactionResult, state string) error {
	if !validReplicationTransactionID(result.ID) || !validReplicationRemote(result.Remote) || (state != "validated" && state != "complete") {
		return fmt.Errorf("invalid replication transaction receipt")
	}
	if err := validateReplicationObjectIDs(result.ID, result.pendingObjects); err != nil {
		return err
	}
	if err := validateReplicationObjectIDs(result.ID, result.acceptedObjects); err != nil {
		return err
	}
	directory := replicationTransactionsPath(mainGitDir)
	if err := ensureReplicationStateDirectory(mainGitDir, directory); err != nil {
		return fmt.Errorf("prepare replication transaction records: %w", err)
	}
	pending := make([]string, len(result.pendingObjects))
	copy(pending, result.pendingObjects)
	accepted := make([]string, len(result.acceptedObjects))
	copy(accepted, result.acceptedObjects)
	record := replicationTransactionRecord{
		Version: replicationTransactionRecordVersion, ID: result.ID, Remote: result.Remote,
		State: state, Outcomes: result.Outcomes,
		Promotions:     append([]replicationPromotion(nil), result.promotions...),
		PendingObjects: &pending, AcceptedObjects: &accepted,
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if len(encoded)+1 > maxReplicationStateBytes {
		return fmt.Errorf("replication transaction %s is too large", result.ID)
	}
	if err := replacePrivateFileAtomic(filepath.Join(directory, result.ID+".json"), append(encoded, '\n')); err != nil {
		return fmt.Errorf("record replication transaction: %w", err)
	}
	return nil
}
