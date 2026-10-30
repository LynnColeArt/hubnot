package main

import (
	"fmt"
	"sort"
	"strings"
)

func cmdProposal(args []string) error {
	if len(args) == 0 {
		return usageError("usage: hn proposal <open|revise|list|show|status>")
	}
	switch args[0] {
	case "open":
		return cmdProposalOpen(args[1:])
	case "revise":
		return cmdProposalRevise(args[1:])
	case "list":
		if len(args) != 1 {
			return usageError("usage: hn proposal list")
		}
		if err := prepareShallowVerification(shallowVerificationScope{Operation: "proposal list"}); err != nil {
			return err
		}
		if err := guardShallowEventClosure("proposal list"); err != nil {
			return err
		}
		return cmdProposalList()
	case "show":
		if len(args) != 2 {
			return usageError("usage: hn proposal show PROPOSAL")
		}
		return cmdProposalShow(args[1])
	case "status":
		if len(args) != 2 {
			return usageError("usage: hn proposal status PROPOSAL")
		}
		return cmdProposalStatus(args[1])
	default:
		return fmt.Errorf("unknown proposal command %q", args[0])
	}
}

func cmdProposalRevise(args []string) error {
	if len(args) < 1 {
		return usageError("usage: hn proposal revise PREDECESSOR --base REV --head REV [--body TEXT]")
	}
	predecessorQuery := args[0]
	if err := requireFullEventID(predecessorQuery); err != nil {
		return err
	}
	flags := quietFlags("proposal revise")
	baseRevision := flags.String("base", "", "base Git revision")
	headRevision := flags.String("head", "", "revised Git revision")
	body := flags.String("body", "", "revision description")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *baseRevision == "" || *headRevision == "" {
		return usageError("usage: hn proposal revise PREDECESSOR --base REV --head REV [--body TEXT]")
	}
	if err := prepareShallowVerification(shallowVerificationScope{
		Operation: "proposal revision", Subject: predecessorQuery, Base: *baseRevision, Head: *headRevision,
	}); err != nil {
		return err
	}
	if err := guardShallowEventClosure("proposal revision"); err != nil {
		return err
	}
	base, err := resolveCommitDependency("proposal revision", shallowBaseCommit, *baseRevision, "", "")
	if err != nil {
		return fmt.Errorf("resolve base: %w", err)
	}
	head, err := resolveCommitDependency("proposal revision", shallowProposalCodeRef, *headRevision, "", "")
	if err != nil {
		return fmt.Errorf("resolve head: %w", err)
	}
	if base == head {
		return fmt.Errorf("proposal revision base and head resolve to the same commit")
	}
	if err := guardCandidateCreationDependencies("proposal revision", base, head); err != nil {
		return err
	}
	events, err := collectGovernedEvents()
	if err != nil {
		return err
	}
	predecessor, err := resolveEvent(events, predecessorQuery)
	if err != nil {
		return err
	}
	if !isProposalKind(predecessor.Event.Kind) {
		return fmt.Errorf("%s is not a proposal", shortID(predecessor.ID))
	}
	lineage, err := buildLineageIndex(events)
	if err != nil {
		return err
	}
	state, err := lineage.state(predecessor.ID)
	if err != nil {
		return err
	}
	if state.Merged {
		return fmt.Errorf("proposal %s is already merged; create an independent proposal instead", predecessor.ID)
	}
	identity, err := loadIdentity()
	if err != nil {
		return err
	}
	if identity.Actor != predecessor.Event.Actor {
		return fmt.Errorf("identity %s is not the author of predecessor %s", shortID(identity.Actor), predecessor.ID)
	}
	event, err := nextEvent(identity, "proposal.revise")
	if err != nil {
		return err
	}
	event.Subject = predecessor.ID
	event.Body = *body
	event.Base = base
	event.Head = head
	rawEvents, err := collectEvents()
	if err != nil {
		return err
	}
	if err := contributionAuthorizeCandidate(event, rawEvents); err != nil {
		return err
	}
	localPlan, err := contributionBeforeNative(event, identity, rawEvents)
	if err != nil {
		return err
	}
	stored, err := appendEvent(event, identity)
	if err != nil {
		return err
	}
	if err := createProposalRef(stored.ID, head); err != nil {
		return fmt.Errorf("proposal revision event %s was created but its code ref failed: %w", stored.ID, err)
	}
	if err := contributionFinishNative(stored, localPlan); err != nil {
		return err
	}
	fmt.Printf("Created proposal revision %s\n", stored.ID)
	if e := contributionPrintNativeAdmission(stored.ID); e != nil {
		return e
	}
	fmt.Printf("Predecessor: %s\n", predecessor.ID)
	fmt.Printf("Code: %s..%s\n", base, head)
	return nil
}

func cmdProposalOpen(args []string) error {
	flags := quietFlags("proposal open")
	baseRevision := flags.String("base", "", "base Git revision")
	headRevision := flags.String("head", "", "proposed Git revision")
	body := flags.String("body", "", "proposal description")
	if err := flags.Parse(args); err != nil {
		return err
	}
	title := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if *baseRevision == "" || *headRevision == "" || title == "" {
		return usageError("usage: hn proposal open --base REV --head REV [--body TEXT] TITLE")
	}
	if err := prepareShallowVerification(shallowVerificationScope{
		Operation: "proposal open", Base: *baseRevision, Head: *headRevision,
	}); err != nil {
		return err
	}
	if err := guardShallowEventClosure("proposal open"); err != nil {
		return err
	}
	base, err := resolveCommitDependency("proposal open", shallowBaseCommit, *baseRevision, "", "")
	if err != nil {
		return fmt.Errorf("resolve base: %w", err)
	}
	head, err := resolveCommitDependency("proposal open", shallowProposalCodeRef, *headRevision, "", "")
	if err != nil {
		return fmt.Errorf("resolve head: %w", err)
	}
	if base == head {
		return fmt.Errorf("proposal base and head resolve to the same commit")
	}
	if err := guardCandidateCreationDependencies("proposal open", base, head); err != nil {
		return err
	}
	policyDiagnostic, err := policyAmendmentDiagnostic(base, head)
	if err != nil {
		return fmt.Errorf("inspect policy amendment: %w", err)
	}
	identity, err := loadIdentity()
	if err != nil {
		return err
	}
	event, err := nextEvent(identity, "proposal.open")
	if err != nil {
		return err
	}
	event.Title = title
	event.Body = *body
	event.Base = base
	event.Head = head
	rawEvents, err := collectEvents()
	if err != nil {
		return err
	}
	if err := contributionAuthorizeCandidate(event, rawEvents); err != nil {
		return err
	}
	localPlan, err := contributionBeforeNative(event, identity, rawEvents)
	if err != nil {
		return err
	}
	stored, err := appendEvent(event, identity)
	if err != nil {
		return err
	}
	if err := createProposalRef(stored.ID, head); err != nil {
		return fmt.Errorf("proposal event %s was created but its code ref failed: %w", shortID(stored.ID), err)
	}
	if err := contributionFinishNative(stored, localPlan); err != nil {
		return err
	}
	fmt.Printf("Opened proposal %s: %s\n", shortID(stored.ID), oneLine(title))
	if e := contributionPrintNativeAdmission(stored.ID); e != nil {
		return e
	}
	fmt.Printf("Code: %s..%s\n", shortOID(base), shortOID(head))
	if policyDiagnostic != "" {
		fmt.Println(policyDiagnostic)
	}
	return nil
}

func resolveCommit(revision string) (string, error) {
	if strings.HasPrefix(revision, "-") {
		return "", fmt.Errorf("invalid revision %q", revision)
	}
	commit, err := gitText("rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("Git revision %q is not a commit", revision)
	}
	return commit, nil
}

func shortOID(oid string) string {
	if len(oid) > 12 {
		return oid[:12]
	}
	return oid
}

func proposalEvents(events []StoredEvent) []StoredEvent {
	proposals := make([]StoredEvent, 0)
	for _, stored := range events {
		if isProposalKind(stored.Event.Kind) {
			proposals = append(proposals, stored)
		}
	}
	return proposals
}

func currentReviews(events []StoredEvent, proposalID string) []StoredEvent {
	byActor := make(map[string]StoredEvent)
	for _, stored := range events {
		if stored.Event.Kind != "review.submit" || stored.Event.Subject != proposalID {
			continue
		}
		previous, exists := byActor[stored.Event.Actor]
		if !exists || stored.Event.Sequence > previous.Event.Sequence {
			byActor[stored.Event.Actor] = stored
		}
	}
	reviews := make([]StoredEvent, 0, len(byActor))
	for _, stored := range byActor {
		reviews = append(reviews, stored)
	}
	sortStoredEvents(reviews)
	return reviews
}

func sortStoredEvents(events []StoredEvent) {
	// Sequence is actor-local, so timestamp is only a presentation order.
	// Event identity and validity never depend on this ordering.
	sort.Slice(events, func(i, j int) bool {
		if events[i].Event.Timestamp != events[j].Event.Timestamp {
			return events[i].Event.Timestamp < events[j].Event.Timestamp
		}
		return events[i].ID < events[j].ID
	})
}

func cmdProposalList() error {
	events, err := collectGovernedEvents()
	if err != nil {
		return err
	}
	proposals := proposalEvents(events)
	if len(proposals) == 0 {
		fmt.Println("No proposals.")
		return nil
	}
	lineage, err := buildLineageIndex(events)
	if err != nil {
		return err
	}
	for _, proposal := range proposals {
		if err := contributionRequireProposal(proposal.ID); err != nil {
			continue
		}
		state, err := lineage.state(proposal.ID)
		if err != nil {
			return err
		}
		title, err := lineageDisplayTitle(lineage, state)
		if err != nil {
			return err
		}
		reviews := currentReviews(events, proposal.ID)
		approvals, changes := reviewCounts(reviews)
		availability := "code-missing"
		if head, exists, err := proposalHead(proposal.ID); err != nil {
			return err
		} else if exists && head == proposal.Event.Head {
			availability = "code-ready"
		} else if exists {
			availability = "code-mismatch"
		}
		fmt.Printf("%s  %s  +%d/-%d  %s\n", shortID(proposal.ID), oneLine(title), approvals, changes, availability)
		printLineageSummary(state, "  ")
	}
	return nil
}

func cmdProposalShow(query string) error {
	if err := prepareShallowVerification(shallowVerificationScope{Operation: "proposal show", Subject: query}); err != nil {
		return err
	}
	if err := guardShallowEventClosure("proposal show"); err != nil {
		return err
	}
	events, err := collectGovernedEvents()
	if err != nil {
		return err
	}
	proposal, err := resolveEventDependency("proposal show", query, shallowCandidateEvent, events)
	if err != nil {
		return err
	}
	if !isProposalKind(proposal.Event.Kind) {
		return fmt.Errorf("%s is not a proposal", shortID(proposal.ID))
	}
	if err := contributionRequireProposal(proposal.ID); err != nil {
		return err
	}
	lineage, err := buildLineageIndex(events)
	if err != nil {
		return err
	}
	state, err := lineage.state(proposal.ID)
	if err != nil {
		return err
	}
	title, err := lineageDisplayTitle(lineage, state)
	if err != nil {
		return err
	}
	fmt.Printf("%s  %s\n", shortID(proposal.ID), oneLine(title))
	fmt.Printf("Proposed by %s at %s\n", oneLine(proposal.Event.ActorName), proposal.Event.Timestamp)
	fmt.Printf("Base: %s\nHead: %s\n", proposal.Event.Base, proposal.Event.Head)
	printLineageSummary(state, "")
	head, exists, err := proposalHead(proposal.ID)
	if err != nil {
		return err
	}
	switch {
	case !exists:
		fmt.Println("Code: unavailable")
	case head != proposal.Event.Head:
		fmt.Printf("Code: REF MISMATCH (%s)\n", head)
	default:
		fmt.Println("Code: available and matched")
	}
	if proposal.Event.Body != "" {
		fmt.Printf("\n%s\n", safeText(proposal.Event.Body))
	}
	reviews := currentReviews(events, proposal.ID)
	if len(reviews) > 0 {
		fmt.Println("\nCurrent reviews:")
		for _, review := range reviews {
			fmt.Printf("\n%s: %s (%s)\n", oneLine(review.Event.ActorName), review.Event.Verdict, shortID(review.ID))
			if review.Event.Body != "" {
				fmt.Printf("%s\n", safeText(review.Event.Body))
			}
		}
	}
	return nil
}

func lineageDisplayTitle(lineage *lineageIndex, state proposalLineageState) (string, error) {
	root, err := lineage.candidate(state.RootID)
	if err != nil {
		return "", err
	}
	if err := contributionRequireProposal(root.ID); err != nil {
		return "[hidden proposal title]", nil
	}
	return root.Event.Title, nil
}

func printLineageSummary(state proposalLineageState, indent string) {
	hasLineage := state.PredecessorID != "" || len(state.SuccessorIDs) > 0 || len(state.MergedCandidateIDs) > 0
	if !hasLineage {
		return
	}
	if state.PredecessorID != "" {
		fmt.Printf("%sPredecessor: %s\n", indent, state.PredecessorID)
	}
	if len(state.SuccessorIDs) > 0 {
		fmt.Printf("%sSuccessors: %s\n", indent, strings.Join(state.SuccessorIDs, ", "))
	}
	if len(state.SiblingIDs) > 0 {
		fmt.Printf("%sSiblings: %s\n", indent, strings.Join(state.SiblingIDs, ", "))
	}
	states := make([]string, 0, 4)
	if state.Merged {
		states = append(states, "merged")
	}
	if state.Superseded {
		states = append(states, "superseded")
	}
	if state.LineageClosed {
		states = append(states, "lineage closed")
	}
	if state.MergeConflict {
		states = append(states, "merge conflict")
	}
	if len(states) == 0 {
		states = append(states, "candidate")
	}
	fmt.Printf("%sState: %s\n", indent, strings.Join(states, ", "))
	if len(state.MergedCandidateIDs) > 0 {
		fmt.Printf("%sMerged lineage members: %s\n", indent, strings.Join(state.MergedCandidateIDs, ", "))
	}
}

func reviewCounts(reviews []StoredEvent) (approvals, changes int) {
	for _, review := range reviews {
		switch review.Event.Verdict {
		case "approve":
			approvals++
		case "request-changes":
			changes++
		}
	}
	return approvals, changes
}

func cmdReview(args []string) error {
	if len(args) < 1 {
		return usageError("usage: hn review PROPOSAL <--approve|--request-changes> [--body TEXT]")
	}
	proposalQuery := args[0]
	if err := requireFullEventID(proposalQuery); err != nil {
		return err
	}
	flags := quietFlags("review")
	approve := flags.Bool("approve", false, "approve the proposal")
	requestChanges := flags.Bool("request-changes", false, "request changes")
	body := flags.String("body", "", "review explanation")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *approve == *requestChanges {
		return usageError("choose exactly one of --approve or --request-changes")
	}
	if err := prepareShallowVerification(shallowVerificationScope{Operation: "proposal review", Subject: proposalQuery}); err != nil {
		return err
	}
	if err := guardShallowEventClosure("proposal review"); err != nil {
		return err
	}
	events, err := collectGovernedEvents()
	if err != nil {
		return err
	}
	proposal, err := resolveEventDependency("proposal review", proposalQuery, shallowCandidateEvent, events)
	if err != nil {
		return err
	}
	if err := contributionRequireProposal(proposal.ID); err != nil {
		return err
	}
	if !isProposalKind(proposal.Event.Kind) {
		return fmt.Errorf("%s is not a proposal", shortID(proposal.ID))
	}
	if err := guardProposalDependencies("proposal review", proposal, events); err != nil {
		return err
	}
	head, exists, err := proposalHead(proposal.ID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("proposal code is unavailable; sync before reviewing")
	}
	if head != proposal.Event.Head {
		return fmt.Errorf("proposal code ref does not match the signed proposal")
	}
	identity, err := loadIdentity()
	if err != nil {
		return err
	}
	event, err := nextEvent(identity, "review.submit")
	if err != nil {
		return err
	}
	event.Subject = proposal.ID
	event.Body = *body
	if *approve {
		event.Verdict = "approve"
	} else {
		event.Verdict = "request-changes"
	}
	stored, err := appendEvent(event, identity)
	if err != nil {
		return err
	}
	fmt.Printf("Recorded %s review %s on proposal %s\n", event.Verdict, shortID(stored.ID), shortID(proposal.ID))
	return nil
}
