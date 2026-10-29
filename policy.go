package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	c "hubnot/internal/contribution"
	"reflect"
	"sort"
	"strings"
)

const (
	policyVersion = "hn.policy/0"
	maxPolicySize = 1 << 20
)

type ContributionCheckpoint struct {
	ID        string         `json:"id"`
	Authority c.AuthorityRef `json:"authority"`
}

type PolicyDocument struct {
	Contributions           *c.Policy                 `json:"contributions,omitempty"`
	ContributionCheckpoints []ContributionCheckpoint  `json:"contributionCheckpoints,omitempty"`
	Version                 string                    `json:"version"`
	Maintainers             []string                  `json:"maintainers"`
	Proposals               ProposalPolicy            `json:"proposals"`
	Pipelines               map[string]PipelinePolicy `json:"pipelines"`
	Memory                  *MemoryPolicy             `json:"memory,omitempty"`
	ProtectedChecks         *ProtectedChecksPolicy    `json:"protectedChecks,omitempty"`
	Automerge               *AutomergePolicy          `json:"automerge,omitempty"`
	Audits                  *AuditPolicy              `json:"audits,omitempty"`
}

// MemoryPolicy is deliberately optional so repositories with an hn.policy/0
// document predating memory remain valid. Absence is not implicit trust.
type MemoryPolicy struct {
	TrustedActors []string `json:"trustedActors"`
	TrustedKinds  []string `json:"trustedKinds"`
}

type ProposalPolicy struct {
	RequiredApprovals   int      `json:"requiredApprovals"`
	RequiredAccepts     int      `json:"requiredAccepts"`
	TrustedReviewers    []string `json:"trustedReviewers"`
	AllowAuthorApproval bool     `json:"allowAuthorApproval"`
}

type PipelinePolicy struct {
	RequiredResults int      `json:"requiredResults"`
	TrustedRunners  []string `json:"trustedRunners"`
	// Requesters may sign run.request events that count, in addition to
	// maintainers. Absent keeps runs maintainer-requested only.
	Requesters []string `json:"requesters,omitempty"`
}

// mayRequestRun reports whether actor's run.request counts for pipeline.
func mayRequestRun(policy PolicyDocument, pipeline, actor string) bool {
	if actorListed(actor, policy.Maintainers) {
		return true
	}
	pipelinePolicy, exists := policy.Pipelines[pipeline]
	return exists && actorListed(actor, pipelinePolicy.Requesters)
}

type PipelineEvaluation struct {
	Name     string
	Required int
	Passed   []StoredEvent
	Failed   []StoredEvent
}

type ProposalEvaluation struct {
	Proposal        *StoredEvent
	DisplayTitle    string
	Lineage         proposalLineageState
	Policy          PolicyDocument
	PolicyDigest    string
	CodeAvailable   bool
	CodeMatches     bool
	Approvals       []StoredEvent
	ChangeRequests  []StoredEvent
	Pipelines       []PipelineEvaluation
	AcceptDecisions []StoredEvent
	RejectDecisions []StoredEvent
	MergeEvents     []StoredEvent
	Ready           bool
	Accepted        bool
	Rejected        bool
	Merged          bool
	Evidence        []string
	ProtectedChecks *ProtectedChecksEvaluation
}

func loadPolicy(commit string) (PolicyDocument, []byte, string, error) {
	gitDir, err := requireGitRepository()
	if err != nil {
		return PolicyDocument{}, nil, "", err
	}
	if err := replicationPendingError(gitDir, commit); err != nil {
		return PolicyDocument{}, nil, "", err
	}
	encoded, err := gitOutput("show", commit+":.hn/policy.json")
	if err != nil {
		return PolicyDocument{}, nil, "", fmt.Errorf("no .hn/policy.json at commit %s", commit)
	}
	policy, digest, err := parsePolicyBytes(encoded)
	if err != nil {
		return PolicyDocument{}, nil, "", err
	}
	return policy, encoded, digest, nil
}

func parsePolicyBytes(encoded []byte) (PolicyDocument, string, error) {
	if len(encoded) > maxPolicySize {
		return PolicyDocument{}, "", fmt.Errorf("policy exceeds %d bytes", maxPolicySize)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var policy PolicyDocument
	if err := decoder.Decode(&policy); err != nil {
		return PolicyDocument{}, "", fmt.Errorf("parse policy: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return PolicyDocument{}, "", fmt.Errorf("parse policy: %w", err)
	}
	var policyFields map[string]json.RawMessage
	_ = json.Unmarshal(encoded, &policyFields)
	hasContributionFields := false
	for name := range policyFields {
		if strings.EqualFold(name, "contributions") || strings.EqualFold(name, "contributionCheckpoints") {
			hasContributionFields = true
		}
	}
	if hasContributionFields {
		if err := contributionPolicyJSONShape(encoded); err != nil {
			return PolicyDocument{}, "", fmt.Errorf("parse contribution policy: %w", err)
		}
	}
	if err := validatePolicy(policy); err != nil {
		return PolicyDocument{}, "", err
	}
	return policy, eventID(encoded), nil
}

func validatePolicy(policy PolicyDocument) error {
	if policy.Contributions != nil {
		if err := policy.Contributions.Validate(); err != nil {
			return err
		}
	}
	if len(policy.ContributionCheckpoints) > c.MaxGraphRecords {
		return fmt.Errorf("contribution checkpoint limit")
	}
	prior := ""
	for _, cp := range policy.ContributionCheckpoints {
		if !validEventID(cp.ID) || cp.ID <= prior || !validEventID(cp.Authority.Digest) || !validGitOID(cp.Authority.Commit) {
			return fmt.Errorf("invalid sorted exact contribution checkpoint")
		}
		prior = cp.ID
	}
	if err := validateAuditPolicy(policy.Audits); err != nil {
		return err
	}
	if policy.Version != policyVersion {
		return fmt.Errorf("unsupported policy version %q", policy.Version)
	}
	if len(policy.Maintainers) == 0 {
		return fmt.Errorf("policy requires at least one maintainer")
	}
	if err := validateActorList("maintainers", policy.Maintainers); err != nil {
		return err
	}
	if policy.Proposals.RequiredApprovals < 0 || policy.Proposals.RequiredApprovals > 64 {
		return fmt.Errorf("requiredApprovals must be between 0 and 64")
	}
	if policy.Proposals.RequiredAccepts < 1 || policy.Proposals.RequiredAccepts > len(policy.Maintainers) {
		return fmt.Errorf("requiredAccepts must be between 1 and the number of maintainers")
	}
	if err := validateActorList("trustedReviewers", policy.Proposals.TrustedReviewers); err != nil {
		return err
	}
	if policy.Proposals.RequiredApprovals > len(policy.Proposals.TrustedReviewers) {
		return fmt.Errorf("requiredApprovals exceeds the number of trusted reviewers")
	}
	for name, pipeline := range policy.Pipelines {
		if !validPipelineName(name) {
			return fmt.Errorf("policy has invalid pipeline name %q", name)
		}
		if pipeline.RequiredResults < 1 || pipeline.RequiredResults > 64 {
			return fmt.Errorf("pipeline %q requiredResults must be between 1 and 64", name)
		}
		if err := validateActorList("pipeline "+name+" trustedRunners", pipeline.TrustedRunners); err != nil {
			return err
		}
		if pipeline.RequiredResults > len(pipeline.TrustedRunners) {
			return fmt.Errorf("pipeline %q requiredResults exceeds its trusted runner count", name)
		}
		if pipeline.Requesters != nil {
			if len(pipeline.Requesters) < 1 || len(pipeline.Requesters) > 64 {
				return fmt.Errorf("pipeline %q requesters must list 1 to 64 actors when present", name)
			}
			if err := validateActorList("pipeline "+name+" requesters", pipeline.Requesters); err != nil {
				return err
			}
		}
	}
	if policy.Memory != nil {
		if policy.Memory.TrustedActors == nil {
			return fmt.Errorf("memory trustedActors is required")
		}
		if policy.Memory.TrustedKinds == nil {
			return fmt.Errorf("memory trustedKinds is required")
		}
		if err := validateSortedActorList("memory trustedActors", policy.Memory.TrustedActors); err != nil {
			return err
		}
		if err := validateMemoryKindList(policy.Memory.TrustedKinds); err != nil {
			return err
		}
	}
	return validateOptionalPolicy(policy)
}

func validateActorList(name string, actors []string) error {
	seen := make(map[string]bool)
	for _, actor := range actors {
		if !validActorID(actor) {
			return fmt.Errorf("%s contains invalid actor %q", name, actor)
		}
		if seen[actor] {
			return fmt.Errorf("%s contains duplicate actor %s", name, actor)
		}
		seen[actor] = true
	}
	return nil
}

func validateSortedActorList(name string, actors []string) error {
	if err := validateActorList(name, actors); err != nil {
		return err
	}
	if !sort.StringsAreSorted(actors) {
		return fmt.Errorf("%s must be sorted", name)
	}
	return nil
}

func validMemoryKind(kind string) bool {
	switch kind {
	case memoryKindObservation, memoryKindDecision, memoryKindAssumption,
		memoryKindAttempt, memoryKindVerification, memoryKindHandoff:
		return true
	default:
		return false
	}
}

func validateMemoryKindList(kinds []string) error {
	seen := make(map[string]bool, len(kinds))
	for _, kind := range kinds {
		if !validMemoryKind(kind) {
			return fmt.Errorf("memory trustedKinds contains invalid kind %q", kind)
		}
		if seen[kind] {
			return fmt.Errorf("memory trustedKinds contains duplicate kind %s", kind)
		}
		seen[kind] = true
	}
	if !sort.StringsAreSorted(kinds) {
		return fmt.Errorf("memory trustedKinds must be sorted")
	}
	return nil
}

// classifyMemoryTrust keeps policy qualification separate from signatures,
// evidence, applicability, and lifecycle. Actor failure takes deterministic
// precedence when both policy dimensions fail.
func classifyMemoryTrust(policy *MemoryPolicy, actor, kind string) string {
	if policy == nil {
		return memoryTrustPolicyMissing
	}
	if !actorListed(actor, policy.TrustedActors) {
		return memoryTrustActorUntrusted
	}
	if !stringListed(kind, policy.TrustedKinds) {
		return memoryTrustKindUntrusted
	}
	return memoryTrustQualified
}

func stringListed(value string, values []string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func actorListed(actor string, actors []string) bool {
	for _, candidate := range actors {
		if candidate == actor {
			return true
		}
	}
	return false
}

func evaluateProposal(proposal *StoredEvent, events []StoredEvent) (*ProposalEvaluation, error) {
	view, _, err := contributionView(events)
	if err != nil {
		return nil, err
	}
	if view != nil {
		if !view.Admitted[proposal.ID] || !view.Visible[proposal.ID] {
			return nil, fmt.Errorf("proposal is not admitted and visible under current contribution authority")
		}
		events = contributionOperativeEvidence(events, view)
	}
	if !isProposalKind(proposal.Event.Kind) {
		return nil, fmt.Errorf("%s is not a proposal", shortID(proposal.ID))
	}
	lineage, err := buildLineageIndex(events)
	if err != nil {
		return nil, err
	}
	lineageState, err := lineage.state(proposal.ID)
	if err != nil {
		return nil, err
	}
	root, err := lineage.candidate(lineageState.RootID)
	if err != nil {
		return nil, err
	}
	policy, _, digest, err := loadPolicy(proposal.Event.Base)
	if err != nil {
		return nil, err
	}
	evaluation := &ProposalEvaluation{Proposal: proposal, DisplayTitle: root.Event.Title, Lineage: lineageState, Policy: policy, PolicyDigest: digest}
	head, exists, err := proposalHead(proposal.ID)
	if err != nil {
		return nil, err
	}
	evaluation.CodeAvailable = exists
	evaluation.CodeMatches = exists && head == proposal.Event.Head

	for _, review := range currentReviews(events, proposal.ID) {
		if !actorListed(review.Event.Actor, policy.Proposals.TrustedReviewers) {
			continue
		}
		if !policy.Proposals.AllowAuthorApproval && review.Event.Actor == proposal.Event.Actor {
			continue
		}
		if review.Event.Verdict == "approve" {
			evaluation.Approvals = append(evaluation.Approvals, review)
		} else {
			evaluation.ChangeRequests = append(evaluation.ChangeRequests, review)
		}
	}

	pipelineNames := make([]string, 0, len(policy.Pipelines))
	for name := range policy.Pipelines {
		pipelineNames = append(pipelineNames, name)
	}
	sort.Strings(pipelineNames)
	for _, name := range pipelineNames {
		pipelinePolicy := policy.Pipelines[name]
		expectedDefinition := ""
		if evaluation.CodeMatches {
			_, _, definition, err := loadPipeline(proposal.Event.Head, name)
			if err != nil {
				return nil, fmt.Errorf("required pipeline %q: %w", name, err)
			}
			expectedDefinition = definition
		}
		latestByRunner := make(map[string]StoredEvent)
		for _, request := range events {
			if request.Event.Kind != "run.request" || request.Event.Subject != proposal.ID || request.Event.Pipeline != name ||
				request.Event.Definition != expectedDefinition || !mayRequestRun(policy, name, request.Event.Actor) {
				continue
			}
			for _, result := range currentRunResults(events, request.ID) {
				if !actorListed(result.Event.Actor, pipelinePolicy.TrustedRunners) {
					continue
				}
				previous, found := latestByRunner[result.Event.Actor]
				if !found || result.Event.Sequence > previous.Event.Sequence {
					latestByRunner[result.Event.Actor] = result
				}
			}
		}
		pipelineEvaluation := PipelineEvaluation{Name: name, Required: pipelinePolicy.RequiredResults}
		for _, result := range latestByRunner {
			if result.Event.Outcome == "passed" {
				pipelineEvaluation.Passed = append(pipelineEvaluation.Passed, result)
			} else {
				pipelineEvaluation.Failed = append(pipelineEvaluation.Failed, result)
			}
		}
		sortStoredEvents(pipelineEvaluation.Passed)
		sortStoredEvents(pipelineEvaluation.Failed)
		evaluation.Pipelines = append(evaluation.Pipelines, pipelineEvaluation)
	}

	currentDecisions := make(map[string]StoredEvent)
	for _, stored := range events {
		if stored.Event.Kind != "proposal.decision" || stored.Event.Subject != proposal.ID || stored.Event.Policy != digest || !actorListed(stored.Event.Actor, policy.Maintainers) {
			continue
		}
		previous, exists := currentDecisions[stored.Event.Actor]
		if !exists || stored.Event.Sequence > previous.Event.Sequence {
			currentDecisions[stored.Event.Actor] = stored
		}
	}
	for _, decision := range currentDecisions {
		if decision.Event.Verdict == "accept" {
			evaluation.AcceptDecisions = append(evaluation.AcceptDecisions, decision)
		} else {
			evaluation.RejectDecisions = append(evaluation.RejectDecisions, decision)
		}
	}
	sortStoredEvents(evaluation.AcceptDecisions)
	sortStoredEvents(evaluation.RejectDecisions)
	for _, stored := range events {
		if stored.Event.Kind == "proposal.merged" && stored.Event.Subject == proposal.ID && stored.Event.Policy == digest {
			evaluation.MergeEvents = append(evaluation.MergeEvents, stored)
		}
	}
	sortStoredEvents(evaluation.MergeEvents)

	evaluation.Ready = evaluation.CodeMatches && len(evaluation.Approvals) >= policy.Proposals.RequiredApprovals
	for _, pipeline := range evaluation.Pipelines {
		if len(pipeline.Passed) < pipeline.Required {
			evaluation.Ready = false
		}
	}
	for _, approval := range evaluation.Approvals {
		evaluation.Evidence = append(evaluation.Evidence, approval.ID)
	}
	for _, pipeline := range evaluation.Pipelines {
		for _, result := range pipeline.Passed {
			evaluation.Evidence = append(evaluation.Evidence, result.ID)
		}
	}
	if policy.ProtectedChecks != nil {
		changed, err := changedPathsAt("", proposal.Event.Base, proposal.Event.Head)
		if err != nil {
			return nil, fmt.Errorf("protected checks: %w", err)
		}
		evaluation.ProtectedChecks = evaluateProtectedChecks(policy, *proposal, events, changed)
		if len(evaluation.ProtectedChecks.ChangedPaths) > 0 {
			if len(evaluation.ProtectedChecks.Approvals) < evaluation.ProtectedChecks.Required {
				evaluation.Ready = false
			}
			for _, review := range evaluation.ProtectedChecks.Approvals {
				evaluation.Evidence = append(evaluation.Evidence, review.ID)
			}
		}
	}
	evaluation.Evidence = sortedUniqueStrings(evaluation.Evidence...)
	evaluation.Rejected = len(evaluation.RejectDecisions) > 0
	evaluation.Accepted = !evaluation.Rejected && len(evaluation.AcceptDecisions) >= policy.Proposals.RequiredAccepts
	evaluation.Merged = len(evaluation.MergeEvents) > 0
	return evaluation, nil
}

func validateDecisionEvent(decision, proposal StoredEvent, byID map[string]StoredEvent) error {
	policy, _, digest, err := loadPolicy(proposal.Event.Base)
	if err != nil {
		return fmt.Errorf("decision %s: %w", shortID(decision.ID), err)
	}
	if decision.Event.Policy != digest || !actorListed(decision.Event.Actor, policy.Maintainers) {
		return fmt.Errorf("decision %s is not authorized by the proposal policy", shortID(decision.ID))
	}
	if decision.Event.Verdict == "reject" {
		return nil
	}
	if err := validateProtectedDecisionAt("", "", policy, decision, proposal, byID); err != nil {
		return err
	}
	approvers := make(map[string]bool)
	pipelineRunners := make(map[string]map[string]bool)
	pipelineDefinitions := make(map[string]string)
	for name := range policy.Pipelines {
		pipelineRunners[name] = make(map[string]bool)
		_, _, definition, err := loadPipeline(proposal.Event.Head, name)
		if err != nil {
			return fmt.Errorf("decision %s required pipeline %q: %w", shortID(decision.ID), name, err)
		}
		pipelineDefinitions[name] = definition
	}
	for _, evidenceID := range decision.Event.Evidence {
		evidence, exists := byID[evidenceID]
		if !exists {
			return fmt.Errorf("decision %s references unavailable evidence %s", shortID(decision.ID), shortID(evidenceID))
		}
		switch evidence.Event.Kind {
		case "review.submit":
			if evidence.Event.Subject == proposal.ID && evidence.Event.Verdict == "approve" && actorListed(evidence.Event.Actor, policy.Proposals.TrustedReviewers) &&
				(policy.Proposals.AllowAuthorApproval || evidence.Event.Actor != proposal.Event.Actor) {
				approvers[evidence.Event.Actor] = true
			}
		case "run.result":
			request, exists := byID[evidence.Event.Subject]
			if !exists || request.Event.Kind != "run.request" || request.Event.Subject != proposal.ID || !mayRequestRun(policy, request.Event.Pipeline, request.Event.Actor) || evidence.Event.Outcome != "passed" {
				continue
			}
			pipelinePolicy, exists := policy.Pipelines[request.Event.Pipeline]
			if exists && request.Event.Definition == pipelineDefinitions[request.Event.Pipeline] && actorListed(evidence.Event.Actor, pipelinePolicy.TrustedRunners) {
				pipelineRunners[request.Event.Pipeline][evidence.Event.Actor] = true
			}
		}
	}
	if len(approvers) < policy.Proposals.RequiredApprovals {
		return fmt.Errorf("decision %s lacks required approval evidence", shortID(decision.ID))
	}
	for name, pipelinePolicy := range policy.Pipelines {
		if len(pipelineRunners[name]) < pipelinePolicy.RequiredResults {
			return fmt.Errorf("decision %s lacks required %s results", shortID(decision.ID), name)
		}
	}
	return nil
}

func validateMergeEvent(merge, proposal StoredEvent, byID map[string]StoredEvent) error {
	policy, _, digest, err := loadPolicy(proposal.Event.Base)
	if err != nil {
		return fmt.Errorf("merge %s: %w", shortID(merge.ID), err)
	}
	if merge.Event.Policy != digest || !actorListed(merge.Event.Actor, policy.Maintainers) {
		return fmt.Errorf("merge %s is not authorized by the proposal policy", shortID(merge.ID))
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
		return fmt.Errorf("merge %s lacks required acceptance evidence", shortID(merge.ID))
	}
	return nil
}

func proposalStatus(evaluation *ProposalEvaluation) string {
	switch {
	case evaluation.Lineage.MergeConflict:
		return "merge conflict"
	case evaluation.Merged:
		return "merged"
	case evaluation.Lineage.LineageClosed:
		return "lineage closed"
	case evaluation.Lineage.Superseded:
		return "superseded"
	case evaluation.Rejected:
		return "rejected"
	case evaluation.Accepted:
		return "accepted"
	case evaluation.Ready:
		return "ready for decision"
	default:
		return "blocked"
	}
}

func cmdProposalStatus(query string) error {
	if err := requireFullEventID(query); err != nil {
		return err
	}
	if err := prepareShallowVerification(shallowVerificationScope{Operation: "proposal status", Subject: query}); err != nil {
		return err
	}
	if err := guardProposalQuery("proposal status", query); err != nil {
		return err
	}
	events, err := collectGovernedEvents()
	if err != nil {
		return err
	}
	proposal, err := resolveEvent(events, query)
	if err != nil {
		return err
	}
	if err := contributionRequireProposal(proposal.ID); err != nil {
		return err
	}
	evaluation, err := evaluateProposal(proposal, events)
	if err != nil {
		return err
	}
	fmt.Printf("Proposal: %s  %s\n", shortID(proposal.ID), oneLine(evaluation.DisplayTitle))
	if evaluation.Lineage.PredecessorID != "" {
		fmt.Printf("Predecessor: %s\n", evaluation.Lineage.PredecessorID)
	}
	if len(evaluation.Lineage.SuccessorIDs) > 0 {
		fmt.Printf("Successors: %s\n", strings.Join(evaluation.Lineage.SuccessorIDs, ", "))
	}
	if len(evaluation.Lineage.SiblingIDs) > 0 {
		fmt.Printf("Siblings: %s\n", strings.Join(evaluation.Lineage.SiblingIDs, ", "))
	}
	if len(evaluation.Lineage.MergedCandidateIDs) > 0 {
		fmt.Printf("Merged lineage members: %s\n", strings.Join(evaluation.Lineage.MergedCandidateIDs, ", "))
	}
	fmt.Printf("Policy:   %s from base %s\n", shortID(evaluation.PolicyDigest), shortOID(proposal.Event.Base))
	if evaluation.CodeMatches {
		fmt.Printf("Code:     ✓ signed head available and matched\n")
	} else if evaluation.CodeAvailable {
		fmt.Printf("Code:     ✗ proposal ref does not match signed head\n")
	} else {
		fmt.Printf("Code:     ✗ proposal code unavailable\n")
	}
	fmt.Printf("Reviews:  %d/%d trusted approvals", len(evaluation.Approvals), evaluation.Policy.Proposals.RequiredApprovals)
	if len(evaluation.ChangeRequests) > 0 {
		fmt.Printf("; %d trusted change request(s)", len(evaluation.ChangeRequests))
	}
	fmt.Println()
	for _, pipeline := range evaluation.Pipelines {
		fmt.Printf("Pipeline: %s %d/%d trusted passes", oneLine(pipeline.Name), len(pipeline.Passed), pipeline.Required)
		if len(pipeline.Failed) > 0 {
			fmt.Printf("; %d trusted failure(s)", len(pipeline.Failed))
		}
		fmt.Println()
	}
	if protection := evaluation.ProtectedChecks; protection != nil {
		fmt.Printf("Protected checks: enabled; %d changed path(s); %d/%d designated approvals\n", len(protection.ChangedPaths), len(protection.Approvals), protection.Required)
		for _, p := range protection.ChangedPaths {
			fmt.Printf("  %q\n", p)
		}
	}
	fmt.Printf("Decision: %d/%d accepts; %d rejects\n", len(evaluation.AcceptDecisions), evaluation.Policy.Proposals.RequiredAccepts, len(evaluation.RejectDecisions))
	fmt.Printf("Status:   %s\n", proposalStatus(evaluation))
	return nil
}

func sortedDecisionIDs(decisions []StoredEvent) []string {
	ids := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		ids = append(ids, decision.ID)
	}
	sort.Strings(ids)
	return ids
}

func policySummary(policy PolicyDocument) string {
	pipelines := make([]string, 0, len(policy.Pipelines))
	for name := range policy.Pipelines {
		pipelines = append(pipelines, name)
	}
	sort.Strings(pipelines)
	return strings.Join(pipelines, ",")
}

// The opt-in policy uses exact field spelling and unique members while keeping
// legacy optional/null role lists valid. Native policy bytes remain unmodified.
func contributionPolicyJSONShape(raw []byte) error {
	if err := validateIssueJSONUnicode(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var parse func(int) (any, error)
	parse = func(depth int) (any, error) {
		if depth > 32 {
			return nil, fmt.Errorf("policy JSON depth limit")
		}
		token, e := d.Token()
		if e != nil {
			return nil, e
		}
		delim, container := token.(json.Delim)
		if !container {
			return token, nil
		}
		switch delim {
		case '{':
			object := map[string]any{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return nil, e
				}
				name, ok := key.(string)
				if !ok {
					return nil, fmt.Errorf("invalid policy key")
				}
				if _, ok := object[name]; ok {
					return nil, fmt.Errorf("duplicate policy field %q", name)
				}
				value, e := parse(depth + 1)
				if e != nil {
					return nil, e
				}
				object[name] = value
			}
			_, e = d.Token()
			return object, e
		case '[':
			array := []any{}
			for d.More() {
				value, e := parse(depth + 1)
				if e != nil {
					return nil, e
				}
				array = append(array, value)
			}
			_, e = d.Token()
			return array, e
		}
		return nil, fmt.Errorf("invalid policy JSON")
	}
	value, e := parse(0)
	if e != nil {
		return e
	}
	if e = ensureJSONEnd(d); e != nil {
		return e
	}
	var check func(any, reflect.Type) error
	check = func(value any, t reflect.Type) error {
		if value == nil {
			return nil
		}
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		switch t.Kind() {
		case reflect.Struct:
			object, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("typed policy value must be object")
			}
			fields := map[string]reflect.Type{}
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				name := strings.Split(f.Tag.Get("json"), ",")[0]
				if name != "" && name != "-" {
					fields[name] = f.Type
				}
			}
			for name, value := range object {
				typ, ok := fields[name]
				if !ok {
					return fmt.Errorf("unknown policy field %q", name)
				}
				if e := check(value, typ); e != nil {
					return e
				}
			}
		case reflect.Map:
			object, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("typed policy value must be map")
			}
			for _, value := range object {
				if e := check(value, t.Elem()); e != nil {
					return e
				}
			}
		case reflect.Slice:
			array, ok := value.([]any)
			if !ok {
				return fmt.Errorf("typed policy value must be array")
			}
			for _, value := range array {
				if e := check(value, t.Elem()); e != nil {
					return e
				}
			}
		}
		return nil
	}
	return check(value, reflect.TypeOf(PolicyDocument{}))
}
