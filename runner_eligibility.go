package main

import "fmt"

// runnerSkipReason only schedules automatic work; explicit execution keeps its
// existing authority and evidence checks. All inputs come from one verified
// observation, so this projection is not a claim against simultaneous starts.
func runnerSkipReason(events []StoredEvent, lineage *lineageIndex, request *StoredEvent) (string, error) {
	proposal, err := lineage.candidate(request.Event.Subject)
	if err != nil {
		return "", err
	}
	if proposal.Event.Head != request.Event.Commit {
		return "", fmt.Errorf("run request does not match its signed proposal")
	}
	state, err := lineage.state(proposal.ID)
	if err != nil {
		return "", err
	}
	switch {
	case state.MergeConflict:
		return "lineage merge conflict", nil
	case state.Merged:
		return "proposal merged", nil
	case state.LineageClosed:
		return "lineage closed", nil
	case state.Superseded:
		return "proposal superseded", nil
	}
	policy, _, _, err := loadPolicy(proposal.Event.Base)
	if err != nil {
		return "", err
	}
	if _, required := policy.Pipelines[request.Event.Pipeline]; !required {
		return "", nil
	}
	_, _, definition, err := loadPipeline(proposal.Event.Head, request.Event.Pipeline)
	if err != nil {
		return "", err
	}
	if definition != request.Event.Definition {
		return "", fmt.Errorf("pipeline definition does not match the signed run request")
	}
	evaluation, required := evaluatePipelineResults(proposal.ID, request.Event.Pipeline, definition, events, policy)
	if required && len(evaluation.Passed) >= evaluation.Required {
		return fmt.Sprintf("pipeline satisfied by %d qualifying trusted results (required %d)", len(evaluation.Passed), evaluation.Required), nil
	}
	return "", nil
}
