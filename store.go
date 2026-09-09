package main

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func actorRef(actor string) string {
	return "refs/hn/actors/" + actor
}

func proposalRef(id string) string {
	return "refs/hn/proposals/" + strings.TrimPrefix(id, "sha256:")
}

func createProposalRef(id, head string) error {
	if _, err := gitOutput("update-ref", "-m", "hn: publish proposal code", proposalRef(id), head, ""); err != nil {
		return fmt.Errorf("create proposal ref: %w", err)
	}
	return nil
}

func proposalHead(id string) (string, bool, error) {
	suffix := strings.TrimPrefix(id, "sha256:")
	out, err := gitText(
		"for-each-ref",
		"--format=%(refname) %(objectname)",
		"refs/hn/proposals",
		"refs/hn/remotes",
	)
	if err != nil {
		return "", false, err
	}
	var found string
	fields := strings.Fields(out)
	for index := 0; index+1 < len(fields); index += 2 {
		ref, object := fields[index], fields[index+1]
		matched := ref == "refs/hn/proposals/"+suffix
		if !matched {
			_, acceptedID, ok := parseAcceptedProposalRef(ref)
			matched = ok && acceptedID == id
		}
		if !matched {
			continue
		}
		if found != "" && found != object {
			return "", false, fmt.Errorf("proposal %s has conflicting code refs", shortID(id))
		}
		found = object
	}
	return found, found != "", nil
}

func appendEvent(event Event, identity *Identity) (*StoredEvent, error) {
	return appendEventWithAttachments(event, identity, nil)
}

func appendEventWithAttachments(event Event, identity *Identity, attachments map[string][]byte) (*StoredEvent, error) {
	ref := actorRef(identity.Actor)
	previousCommit, hasPrevious, err := refValue(ref)
	if err != nil {
		return nil, err
	}

	payload, signature, err := encodeAndSign(event, identity)
	if err != nil {
		return nil, err
	}
	id := eventID(payload)
	if event.Kind == "run.result" {
		log, exists := attachments["log.txt"]
		if !exists || len(attachments) != 1 || eventID(log) != event.Log {
			return nil, fmt.Errorf("run result requires exactly one matching log attachment")
		}
	} else if len(attachments) != 0 {
		return nil, fmt.Errorf("event kind %s does not support attachments", event.Kind)
	}
	if hasPrevious {
		previous, err := loadStoredEvent(previousCommit)
		if err != nil {
			return nil, fmt.Errorf("read actor head: %w", err)
		}
		if event.Previous != previous.ID || event.Sequence != previous.Event.Sequence+1 {
			return nil, fmt.Errorf("actor history changed while creating event; retry the command")
		}
	} else if event.Previous != "" || event.Sequence != 1 {
		return nil, fmt.Errorf("first actor event must have sequence 1 and no predecessor")
	}

	eventBlob, err := gitInput(payload, nil, "hash-object", "-w", "--stdin")
	if err != nil {
		return nil, err
	}
	signatureText := []byte(base64.RawStdEncoding.EncodeToString(signature))
	signatureBlob, err := gitInput(signatureText, nil, "hash-object", "-w", "--stdin")
	if err != nil {
		return nil, err
	}
	treeInput := fmt.Sprintf(
		"100644 blob %s\tevent.json\n100644 blob %s\tsignature\n",
		strings.TrimSpace(string(eventBlob)),
		strings.TrimSpace(string(signatureBlob)),
	)
	attachmentNames := make([]string, 0, len(attachments))
	for name := range attachments {
		if name == "" || name == "event.json" || name == "signature" || strings.ContainsAny(name, "/\\\x00\n\r\t") {
			return nil, fmt.Errorf("invalid attachment name %q", name)
		}
		attachmentNames = append(attachmentNames, name)
	}
	sort.Strings(attachmentNames)
	for _, name := range attachmentNames {
		blob, err := gitInput(attachments[name], nil, "hash-object", "-w", "--stdin")
		if err != nil {
			return nil, err
		}
		treeInput += fmt.Sprintf("100644 blob %s\t%s\n", strings.TrimSpace(string(blob)), name)
	}
	tree, err := gitInput([]byte(treeInput), nil, "mktree")
	if err != nil {
		return nil, err
	}

	commitArgs := []string{"commit-tree", strings.TrimSpace(string(tree)), "-m", "hn event " + id}
	if hasPrevious {
		commitArgs = append(commitArgs, "-p", previousCommit)
	}
	email := shortID(identity.Actor) + "@hn.invalid"
	env := []string{
		"GIT_AUTHOR_NAME=" + identity.Name,
		"GIT_AUTHOR_EMAIL=" + email,
		"GIT_COMMITTER_NAME=" + identity.Name,
		"GIT_COMMITTER_EMAIL=" + email,
	}
	commit, err := gitInput(nil, env, commitArgs...)
	if err != nil {
		return nil, err
	}
	commitID := strings.TrimSpace(string(commit))

	updateArgs := []string{"update-ref", "-m", "hn: append " + event.Kind, ref, commitID}
	if hasPrevious {
		updateArgs = append(updateArgs, previousCommit)
	} else {
		// An empty old value means the ref must not already exist, and works in
		// repositories using either SHA-1 or SHA-256 object IDs.
		updateArgs = append(updateArgs, "")
	}
	if _, err := gitOutput(updateArgs...); err != nil {
		return nil, fmt.Errorf("append event: %w", err)
	}

	return &StoredEvent{ID: id, Commit: commitID, Event: event, Payload: payload, Signature: signature, Attachments: attachments}, nil
}

func nextEvent(identity *Identity, kind string) (Event, error) {
	ref := actorRef(identity.Actor)
	head, exists, err := refValue(ref)
	if err != nil {
		return Event{}, err
	}
	if !exists {
		return newEvent(identity, kind, 1, ""), nil
	}
	previous, err := loadStoredEvent(head)
	if err != nil {
		return Event{}, fmt.Errorf("read previous actor event: %w", err)
	}
	if previous.Event.Actor != identity.Actor {
		return Event{}, fmt.Errorf("actor ref contains an event from another identity")
	}
	return newEvent(identity, kind, previous.Event.Sequence+1, previous.ID), nil
}

func loadStoredEvent(commit string) (*StoredEvent, error) {
	gitDir, err := requireGitRepository()
	if err != nil {
		return nil, err
	}
	if err := replicationPendingError(gitDir, commit); err != nil {
		return nil, err
	}
	return loadStoredEventAt("", commit)
}

func loadStoredEventAt(gitDir, commit string) (*StoredEvent, error) {
	payload, err := gitOutputAt(gitDir, "show", commit+":event.json")
	if err != nil {
		return nil, err
	}
	signatureEncoded, err := gitOutputAt(gitDir, "show", commit+":signature")
	if err != nil {
		return nil, err
	}
	signature, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(signatureEncoded)))
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding in %s", commit)
	}
	event, id, err := verifyEvent(payload, signature)
	if err != nil {
		return nil, fmt.Errorf("verify %s: %w", commit, err)
	}
	attachments := make(map[string][]byte)
	if event.Kind == "run.result" {
		log, err := gitOutputAt(gitDir, "show", commit+":log.txt")
		if err != nil {
			return nil, fmt.Errorf("run result %s has no log attachment", shortID(id))
		}
		if eventID(log) != event.Log {
			return nil, fmt.Errorf("run result %s log digest does not match", shortID(id))
		}
		attachments["log.txt"] = log
	}
	return &StoredEvent{ID: id, Commit: commit, Event: event, Payload: payload, Signature: signature, Attachments: attachments}, nil
}

func loadActorEventsAt(gitDir, head string) ([]StoredEvent, error) {
	chain, err := gitTextAt(gitDir, "rev-list", "--reverse", head)
	if err != nil {
		return nil, err
	}
	commits := strings.Fields(chain)
	events := make([]StoredEvent, 0, len(commits))
	for _, commit := range commits {
		var stored *StoredEvent
		if gitDir == "" {
			stored, err = loadStoredEvent(commit)
		} else {
			stored, err = loadStoredEventAt(gitDir, commit)
		}
		if err != nil {
			return nil, err
		}
		events = append(events, *stored)
	}
	return events, nil
}

func collectEvents() ([]StoredEvent, error) {
	refOutput, err := gitText("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/actors", "refs/hn/remotes")
	if err != nil {
		return nil, err
	}
	if refOutput == "" {
		return nil, nil
	}

	commits := make(map[string]struct{})
	fields := strings.Fields(refOutput)
	for index := 0; index+1 < len(fields); index += 2 {
		ref, head := fields[index], fields[index+1]
		if strings.HasPrefix(ref, "refs/hn/actors/") {
			actor := strings.TrimPrefix(ref, "refs/hn/actors/")
			if !validActorFingerprint(actor) {
				return nil, fmt.Errorf("invalid local actor ref %s", ref)
			}
		} else if _, _, ok := parseAcceptedActorRef(ref); !ok {
			continue
		}
		chain, err := gitText("rev-list", "--reverse", head)
		if err != nil {
			return nil, err
		}
		for _, commit := range strings.Fields(chain) {
			commits[commit] = struct{}{}
		}
	}

	events := make([]StoredEvent, 0, len(commits))
	for commit := range commits {
		stored, err := loadStoredEvent(commit)
		if err != nil {
			return nil, err
		}
		events = append(events, *stored)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Event.Timestamp != events[j].Event.Timestamp {
			return events[i].Event.Timestamp < events[j].Event.Timestamp
		}
		return events[i].ID < events[j].ID
	})
	if err := validateActorChains(events); err != nil {
		return nil, err
	}
	if err := validateEventRelationships(events); err != nil {
		return nil, err
	}
	return events, nil
}

func validateEventRelationships(events []StoredEvent) error {
	if _, err := validateIssueRelationships(events); err != nil {
		return err
	}
	byID := make(map[string]StoredEvent, len(events))
	for _, stored := range events {
		byID[stored.ID] = stored
	}
	for _, stored := range events {
		event := stored.Event
		switch event.Kind {
		case "issue.comment":
			subject, exists := byID[event.Subject]
			if !exists || subject.Event.Kind != "issue.open" {
				return fmt.Errorf("comment %s does not reference an available issue", shortID(stored.ID))
			}
		case "proposal.revise":
			predecessor, exists := byID[event.Subject]
			if !exists || !isProposalKind(predecessor.Event.Kind) {
				return fmt.Errorf("revision %s does not reference an available proposal", shortID(stored.ID))
			}
			if predecessor.Event.Actor != event.Actor {
				return fmt.Errorf("revision %s is not signed by predecessor author %s", shortID(stored.ID), shortID(predecessor.Event.Actor))
			}
		case "review.submit":
			subject, exists := byID[event.Subject]
			if !exists || !isProposalKind(subject.Event.Kind) {
				return fmt.Errorf("review %s does not reference an available proposal", shortID(stored.ID))
			}
		case "run.request":
			subject, exists := byID[event.Subject]
			if !exists || !isProposalKind(subject.Event.Kind) || subject.Event.Head != event.Commit {
				return fmt.Errorf("run request %s does not match an available proposal", shortID(stored.ID))
			}
		case "run.result":
			request, exists := byID[event.Subject]
			if !exists || request.Event.Kind != "run.request" ||
				request.Event.Commit != event.Commit || request.Event.Pipeline != event.Pipeline || request.Event.Definition != event.Definition {
				return fmt.Errorf("run result %s does not match an available request", shortID(stored.ID))
			}
		case "proposal.decision":
			proposal, exists := byID[event.Subject]
			if !exists || !isProposalKind(proposal.Event.Kind) {
				return fmt.Errorf("decision %s does not reference an available proposal", shortID(stored.ID))
			}
			if err := validateDecisionEvent(stored, proposal, byID); err != nil {
				return err
			}
		case "proposal.merged":
			proposal, exists := byID[event.Subject]
			if !exists {
				return fmt.Errorf("merge %s does not match an available proposal", shortID(stored.ID))
			}
			if err := validateProposalMergeBinding(stored, proposal); err != nil {
				return err
			}
			if err := validateMergeEvent(stored, proposal, byID); err != nil {
				return err
			}
		case "identity.accept":
			authorization, exists := byID[event.Subject]
			if !exists {
				continue
			}
			if authorization.Event.Kind != "identity.authorize" {
				return fmt.Errorf("identity acceptance %s does not reference an authorization", shortID(stored.ID))
			}
			if event.Actor != authorization.Event.TargetActor || event.PublicKey != authorization.Event.TargetKey {
				return fmt.Errorf("identity acceptance %s is not signed by the authorization target", shortID(stored.ID))
			}
		}
	}
	return validateRevisionGraph(events, byID)
}

func validateProposalMergeBinding(merge, proposal StoredEvent) error {
	if !isProposalKind(proposal.Event.Kind) || merge.Event.Subject != proposal.ID || merge.Event.Head != proposal.Event.Head {
		return fmt.Errorf("merge %s does not match proposal %s immutable head %s", merge.ID, proposal.ID, proposal.Event.Head)
	}
	return nil
}

func validateRevisionGraph(events []StoredEvent, byID map[string]StoredEvent) error {
	const (
		visiting = 1
		visited  = 2
	)
	state := make(map[string]uint8)
	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case visiting:
			return fmt.Errorf("revision lineage contains a cycle at %s", shortID(id))
		case visited:
			return nil
		}
		stored, exists := byID[id]
		if !exists || stored.Event.Kind != "proposal.revise" {
			return nil
		}
		state[id] = visiting
		if err := visit(stored.Event.Subject); err != nil {
			return err
		}
		state[id] = visited
		return nil
	}
	for _, stored := range events {
		if stored.Event.Kind == "proposal.revise" {
			if err := visit(stored.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveEvent(events []StoredEvent, query string) (*StoredEvent, error) {
	query = strings.TrimPrefix(query, "sha256:")
	var match *StoredEvent
	for i := range events {
		candidate := strings.TrimPrefix(events[i].ID, "sha256:")
		if strings.HasPrefix(candidate, query) {
			if match != nil && match.ID != events[i].ID {
				return nil, fmt.Errorf("event prefix %q is ambiguous", query)
			}
			match = &events[i]
		}
	}
	if match == nil {
		return nil, fmt.Errorf("event %q not found", query)
	}
	return match, nil
}

func requireFullEventID(query string) error {
	if !validEventID(query) {
		return fmt.Errorf("event ID must be full and valid (sha256:<64-hex>)")
	}
	return nil
}

func validateActorChains(events []StoredEvent) error {
	byActor := make(map[string][]StoredEvent)
	for _, event := range events {
		byActor[event.Event.Actor] = append(byActor[event.Event.Actor], event)
	}
	for actor, chain := range byActor {
		sort.Slice(chain, func(i, j int) bool { return chain[i].Event.Sequence < chain[j].Event.Sequence })
		seen := make(map[uint64]string)
		for index, stored := range chain {
			if existing, ok := seen[stored.Event.Sequence]; ok && existing != stored.ID {
				return fmt.Errorf("actor %s has conflicting sequence %s", shortID(actor), strconv.FormatUint(stored.Event.Sequence, 10))
			}
			seen[stored.Event.Sequence] = stored.ID
			wantSequence := uint64(index + 1)
			if stored.Event.Sequence != wantSequence {
				return fmt.Errorf("actor %s has invalid sequence %s", shortID(actor), strconv.FormatUint(stored.Event.Sequence, 10))
			}
			wantPrevious := ""
			if index > 0 {
				wantPrevious = chain[index-1].ID
			}
			if stored.Event.Previous != wantPrevious {
				return fmt.Errorf("actor %s has a broken event chain at sequence %s", shortID(actor), strconv.FormatUint(stored.Event.Sequence, 10))
			}
		}
	}
	return nil
}
