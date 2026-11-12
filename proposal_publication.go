package main

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

type ProposalPublicationIntent struct {
	Operation string `json:"operation"`
	BaseRef   string `json:"base_ref"`
}

type proposalPublicationRequest struct {
	base, head, baseRef, actor, operation, title, body string
}

type proposalPublicationReceipt struct {
	ID        string `json:"id"`
	Base      string `json:"base"`
	Head      string `json:"head"`
	Actor     string `json:"actor"`
	Operation string `json:"operation"`
	BaseRef   string `json:"base_ref"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CodeReady bool   `json:"code_ready"`
	Ready     bool   `json:"ready"`
	Replayed  bool   `json:"replayed"`
}

func validPublicationOperation(key string) bool {
	if len(key) == 0 || len(key) > 128 {
		return false
	}
	for _, c := range key {
		if c < 33 || c > 126 {
			return false
		}
	}
	return true
}

func validPublicationBaseRef(ref string) bool {
	name, ok := strings.CutPrefix(ref, "refs/heads/")
	if !ok || len(name) == 0 || len(ref) > 1024 || strings.Contains(name, "..") ||
		strings.Contains(name, "@{") || strings.HasSuffix(name, ".") {
		return false
	}
	for _, c := range name {
		if c <= ' ' || c == 127 || strings.ContainsRune("~^:?*[\\", c) {
			return false
		}
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
}

func validPublicationText(title, body string) bool {
	return issueText(title, 1024, true) && issueText(body, 65536, false) &&
		!strings.ContainsRune(title, 0) && !strings.ContainsRune(body, 0)
}

type proposalPublicationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *proposalPublicationError) Error() string { return e.Message }

func publicationFailure(code, message string) error {
	return &proposalPublicationError{Code: code, Message: message}
}

func cmdProposalPublication(action string, args []string) error {
	flags := quietFlags("proposal " + action)
	var request proposalPublicationRequest
	flags.StringVar(&request.actor, "actor", "", "expected active signing actor")
	flags.StringVar(&request.operation, "operation", "", "actor-scoped retry key")
	machine := flags.Bool("json", false, "JSON response")
	if action == "publish" {
		flags.StringVar(&request.base, "base", "", "exact base commit")
		flags.StringVar(&request.head, "head", "", "exact candidate commit")
		flags.StringVar(&request.baseRef, "base-ref", "", "local base branch reference")
		flags.StringVar(&request.title, "title", "", "proposal title")
		flags.StringVar(&request.body, "body", "", "proposal body")
	}
	var receipt *proposalPublicationReceipt
	var result error
	if err := flags.Parse(args); err != nil || !*machine || flags.NArg() != 0 ||
		!validActorFingerprint(request.actor) || !validPublicationOperation(request.operation) ||
		(action == "publish" && !request.valid()) {
		result = publicationFailure("invalid_input", "Provide exact proposal arguments and JSON output.")
	} else if action == "operation" {
		receipt, result = lookupProposalPublication(request)
	} else {
		receipt, result = publishProposalOperation(request)
	}
	var failure *proposalPublicationError
	if result != nil && !errors.As(result, &failure) {
		failure = &proposalPublicationError{Code: "recovery_required", Message: "Publication could not be confirmed. Inspect the same operation before retrying."}
	}
	if err := json.NewEncoder(os.Stdout).Encode(struct {
		OK    bool                        `json:"ok"`
		Data  *proposalPublicationReceipt `json:"data"`
		Error *proposalPublicationError   `json:"error"`
	}{failure == nil, receipt, failure}); err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	return nil
}

func (r proposalPublicationRequest) valid() bool {
	return validGitOID(r.base) && validGitOID(r.head) && r.base != r.head &&
		strings.ToLower(r.base) == r.base && strings.ToLower(r.head) == r.head &&
		validPublicationBaseRef(r.baseRef) && validPublicationText(r.title, r.body)
}

func (r proposalPublicationRequest) matches(event Event) bool {
	return event.Base == r.base && event.Head == r.head && event.Title == r.title && event.Body == r.body &&
		event.Publication != nil && event.Publication.BaseRef == r.baseRef
}

// Search authenticated history. An unreadable or ambiguous history never
// constitutes proof that an operation is absent.
func findProposalPublication(events []StoredEvent, request proposalPublicationRequest) (*StoredEvent, error) {
	var matched *StoredEvent
	for i := range events {
		event := events[i].Event
		if event.Kind != "proposal.open" || event.Actor != request.actor ||
			event.Publication == nil || event.Publication.Operation != request.operation {
			continue
		}
		if matched != nil && matched.ID != events[i].ID {
			return nil, publicationFailure("recovery_required", "Multiple signed proposals use the operation.")
		}
		matched = &events[i]
	}
	return matched, nil
}

func proposalPublicationHistory() ([]StoredEvent, error) {
	if err := guardShallowEventClosure("proposal list"); err != nil {
		return nil, err
	}
	return collectEvents()
}

func proposalPublicationResult(stored *StoredEvent, events []StoredEvent, replayed bool) (*proposalPublicationReceipt, error) {
	event := stored.Event
	receipt := &proposalPublicationReceipt{
		ID: stored.ID, Base: event.Base, Head: event.Head, Actor: event.Actor, Operation: event.Publication.Operation,
		BaseRef: event.Publication.BaseRef, Title: event.Title, Body: event.Body, Replayed: replayed,
	}
	head, exists, err := proposalHead(stored.ID)
	if err != nil || (exists && head != event.Head) {
		return receipt, publicationFailure("recovery_required", "The signed proposal has conflicting or unreadable code references.")
	}
	if !exists {
		return receipt, nil
	}
	if resolved, err := resolveCommitDependency("proposal show", shallowProposalCodeRef, head, "proposal", stored.ID); err != nil || resolved != head {
		return receipt, publicationFailure("recovery_required", "The signed proposal code is unavailable.")
	}
	receipt.CodeReady = true
	view, _, err := contributionView(events)
	if err != nil {
		return receipt, err
	}
	receipt.Ready = receipt.CodeReady && (view == nil || view.Visible[stored.ID])
	return receipt, nil
}

func lookupProposalPublication(request proposalPublicationRequest) (*proposalPublicationReceipt, error) {
	identity, err := inspectIdentity()
	if err != nil || identity.Actor != request.actor {
		return nil, publicationFailure("actor_mismatch", "The expected actor is not the active signing identity.")
	}
	events, err := proposalPublicationHistory()
	if err != nil {
		return nil, err
	}
	found, err := findProposalPublication(events, request)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, publicationFailure("not_found", "No proposal is recorded for this actor and operation.")
	}
	return proposalPublicationResult(found, events, true)
}

func publishProposalOperation(request proposalPublicationRequest) (*proposalPublicationReceipt, error) {
	unlock, err := contributionCommandLock([]string{"proposal", "publish"})
	if err != nil {
		return nil, publicationFailure("concurrent_update", "The repository publication state is busy or unavailable. Check the same operation.")
	}
	defer unlock()
	identity, err := inspectIdentity()
	if err != nil || identity.Actor != request.actor {
		return nil, publicationFailure("actor_mismatch", "The expected actor is not the active signing identity.")
	}
	// Reobserve after actor-ref races. nextEvent must capture its predecessor
	// before the operation lookup, so the append CAS covers the lookup.
	for attempt := 0; attempt < 4; attempt++ {
		event, err := nextEvent(identity, "proposal.open")
		if err != nil {
			return nil, err
		}
		events, err := proposalPublicationHistory()
		if err != nil {
			return nil, err
		}
		found, err := findProposalPublication(events, request)
		if err != nil {
			return nil, err
		}
		if found != nil {
			if !request.matches(found.Event) {
				return nil, publicationFailure("operation_conflict", "This operation is already bound to another proposal.")
			}
			return resumeProposalPublication(found, events)
		}

		if err := checkProposalPublicationCandidate(request); err != nil {
			return nil, err
		}
		event.Base, event.Head, event.Title, event.Body = request.base, request.head, request.title, request.body
		event.Publication = &ProposalPublicationIntent{Operation: request.operation, BaseRef: request.baseRef}
		if err := contributionAuthorizeCandidate(event, events); err != nil {
			return nil, publicationFailure("denied", "Repository contribution policy refused the proposal.")
		}
		plan, err := contributionBeforeNative(event, identity, events)
		if err != nil {
			return nil, err
		}
		stored, err := appendEvent(event, identity)
		if err != nil {
			// A failed Git response can follow a successful commit. Reconcile
			// authenticated history before attempting a fresh append.
			if plan != nil {
				return reconcileReservedPublication(request)
			}
			continue
		}
		events = append(events, *stored)
		if err := contributionFinishNative(stored, plan); err != nil {
			receipt, _ := proposalPublicationResult(stored, events, false)
			return receipt, publicationFailure("recovery_required", "The proposal was signed. Retry the same operation to complete admission.")
		}
		return completeProposalPublication(stored, events, false)
	}
	return nil, publicationFailure("concurrent_update", "Publication could not be confirmed. Retry the same operation.")
}

// Contribution reservations are charged against an exact signed event. Never
// issue another event in this call after that reservation has been made.
func reconcileReservedPublication(request proposalPublicationRequest) (*proposalPublicationReceipt, error) {
	events, err := proposalPublicationHistory()
	if err != nil {
		return nil, err
	}
	found, err := findProposalPublication(events, request)
	if err != nil {
		return nil, err
	}
	if found != nil {
		if !request.matches(found.Event) {
			return nil, publicationFailure("operation_conflict", "This operation is already bound to another proposal.")
		}
		return resumeProposalPublication(found, events)
	}
	return nil, publicationFailure("recovery_required", "The contribution reservation could not be reconciled with signed history.")
}

func checkProposalPublicationCandidate(request proposalPublicationRequest) error {
	current, exists, err := refValue(request.baseRef)
	if err != nil || !exists || current != request.base {
		return publicationFailure("stale_base", "The selected base branch moved. Refresh the proposal preview.")
	}
	if err := prepareShallowVerification(shallowVerificationScope{
		Operation: "proposal open", Base: request.base, Head: request.head,
	}); err != nil {
		return err
	}
	for _, oid := range []string{request.base, request.head} {
		commit, err := resolveCommit(oid)
		if err != nil || commit != oid {
			return publicationFailure("invalid_input", "The proposal requires two available, exact commits.")
		}
	}
	if err := guardCandidateCreationDependencies("proposal open", request.base, request.head); err != nil {
		return err
	}
	_, err = policyAmendmentDiagnostic(request.base, request.head)
	return err
}

func resumeProposalPublication(stored *StoredEvent, events []StoredEvent) (*proposalPublicationReceipt, error) {
	receipt, err := proposalPublicationResult(stored, events, true)
	if err != nil {
		return receipt, err
	}
	if !receipt.CodeReady {
		if commit, err := resolveCommit(stored.Event.Head); err != nil || commit != stored.Event.Head {
			return receipt, publicationFailure("recovery_required", "The signed proposal code is unavailable.")
		}
		if _, err := gitOutput("update-ref", "--no-deref", "-m", "hn: repair proposal code", proposalRef(stored.ID), stored.Event.Head, ""); err != nil {
			head, exists, readErr := proposalHead(stored.ID)
			if readErr != nil || !exists || head != stored.Event.Head {
				return receipt, publicationFailure("recovery_required", "The signed proposal code reference could not be repaired.")
			}
		}
	}
	if err := contributionRetryNative(stored); err != nil {
		return receipt, publicationFailure("recovery_required", "The proposal was signed. Its admission could not be completed.")
	}
	return completeProposalPublication(stored, events, true)
}

func completeProposalPublication(stored *StoredEvent, events []StoredEvent, replayed bool) (*proposalPublicationReceipt, error) {
	receipt, err := proposalPublicationResult(stored, events, replayed)
	if err == nil && !receipt.Ready {
		err = publicationFailure("admission_pending", "The proposal was signed and is awaiting repository admission.")
	}
	return receipt, err
}
