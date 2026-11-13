package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"
)

type automaticAttempt struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Started  string `json:"started"`
	Finished string `json:"finished,omitempty"`
	Request  string `json:"request,omitempty"`
	Result   string `json:"result,omitempty"`
}

func eligiblePipelineRequest(request Event, candidate StoredEvent, manualDefinition string, policy PolicyDocument) bool {
	if request.Automatic == nil {
		return request.Definition == manualDefinition && mayRequestRun(policy, request.Pipeline, request.Actor)
	}
	return automaticDefinition(request, candidate, policy)
}

func automaticDefinition(request Event, candidate StoredEvent, policy PolicyDocument) bool {
	intent := request.Automatic
	if intent == nil || intent.Source == candidate.Event.Head || request.Subject != candidate.ID || request.Commit != candidate.Event.Head ||
		!actorListed(request.Actor, policy.Maintainers) {
		return false
	}
	if _, err := gitOutput("merge-base", "--is-ancestor", candidate.Event.Base, intent.Source); err != nil {
		return false
	}
	sourcePolicy, err := loadPolicyCommit(intent.Source)
	if err != nil || !actorListed(request.Actor, sourcePolicy.Maintainers) {
		return false
	}
	config, digest, err := automaticConfigAt(intent.Source)
	if err != nil || digest != intent.Configuration {
		return false
	}
	_, _, definition, err := loadPipeline(intent.Source, request.Pipeline)
	if err != nil || definition != request.Definition {
		return false
	}
	for _, event := range config.Pipelines[request.Pipeline] {
		if event == candidate.Event.Kind {
			return true
		}
	}
	return false
}

func automaticResultEligible(request, result Event) bool {
	return request.Automatic == nil || result.Backend == "sandbox" && result.Actor == request.Actor
}

func automaticTime() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func automaticRunOnce(ctx context.Context) (bool, error) {
	path, err := automaticLedgerPath()
	if err != nil {
		return false, err
	}
	release, err := lockAutomaticLedger(path)
	if err != nil {
		return false, err
	}
	defer release()
	ledger, err := readAutomaticLedger(path)
	if err != nil {
		return false, err
	}
	identity, err := loadIdentity()
	if err != nil {
		return false, err
	}
	if identity.Actor != ledger.Authority {
		return false, fmt.Errorf("active identity is not the scheduling authority")
	}
	events, err := automaticVisibleEvents()
	if err != nil {
		return false, err
	}
	byID := map[string]StoredEvent{}
	for _, event := range events {
		byID[event.ID] = event
	}
	backend := newBubblewrapBackend()
	for _, check := range ledger.Checks {
		if check.Status == "passed" || check.Status == "failed" || check.Status == "canceled" {
			continue
		}
		var attempt *automaticAttempt
		if len(check.Attempts) > 0 {
			attempt = &check.Attempts[len(check.Attempts)-1]
		}
		// The kernel lock fences a live worker. After a crash, recover the signed
		// result first. An interrupted unsigned attempt remains visible.
		if attempt != nil && attempt.Status == "running" {
			for _, request := range events {
				if request.Event.Actor != identity.Actor || request.Event.Automatic == nil ||
					request.Event.Automatic.Check != check.ID || request.Event.Automatic.Attempt != attempt.ID {
					continue
				}
				attempt.Request = request.ID
				for _, result := range currentRunResults(events, request.ID) {
					if !automaticResultEligible(request.Event, result.Event) {
						continue
					}
					attempt.Result = result.ID
					attempt.Status = result.Event.Outcome
					check.Status = attempt.Status
				}
			}
			attempt.Finished = automaticTime()
			if attempt.Result == "" {
				attempt.Status = "canceled"
				check.Status = "canceled"
				check.Reason = "The runner was interrupted before publishing a result."
			}
			if err := writeAutomaticLedger(path, ledger); err != nil {
				return false, err
			}
			continue
		}
		candidate, exists := byID[check.Candidate]
		if !exists || requireProposalCode(&candidate) != nil {
			check.Status = "waiting"
			check.Reason = "Candidate code is unavailable. Sync the proposal code."
			continue
		}
		policy, err := loadPolicyCommit(candidate.Event.Base)
		if err != nil {
			check.Status = "waiting"
			check.Reason = "Candidate policy is unavailable. Sync the proposal base."
			continue
		}
		if required, ok := policy.Pipelines[check.Pipeline]; ok && !actorListed(identity.Actor, required.TrustedRunners) {
			check.Status = "waiting"
			check.Reason = "Authorize the runner for this pipeline in the repository policy."
			continue
		}
		if err := probeAutomaticSandbox(ctx, backend); err != nil {
			check.Status = "waiting"
			check.Reason = "The isolated Linux runner is unavailable. Configure Bubblewrap and verify namespace support."
			continue
		}
		attemptID := eventID(fmt.Appendf(nil, "%s\n%d", check.ID, len(check.Attempts)+1))
		request, err := nextEvent(identity, "run.request")
		if err != nil {
			return false, err
		}
		request.Subject, request.Commit, request.Pipeline, request.Definition = check.Candidate, check.Commit, check.Pipeline, check.Definition
		request.Automatic = &automaticRunIntent{proposalChecksVersion, check.Configuration, check.Source, check.ID, attemptID}
		if !automaticDefinition(request, candidate, policy) {
			check.Status = "waiting"
			check.Reason = "The scheduled definition or authority could not be verified."
			continue
		}
		check.Status, check.Reason = "running", ""
		check.Attempts = append(check.Attempts, automaticAttempt{ID: attemptID, Status: "running", Started: automaticTime()})
		attempt = &check.Attempts[len(check.Attempts)-1]
		if err := writeAutomaticLedger(path, ledger); err != nil {
			return false, err
		}
		stored, err := appendEvent(request, identity)
		if err != nil {
			return false, err
		}
		attempt.Request = stored.ID
		result, err := executeAutomaticRequest(ctx, candidate, *stored, identity, backend)
		if err != nil {
			return false, err
		}
		attempt.Result, attempt.Status, attempt.Finished = result.ID, result.Event.Outcome, automaticTime()
		check.Status = attempt.Status
		return true, writeAutomaticLedger(path, ledger)
	}
	return false, writeAutomaticLedger(path, ledger)
}

func probeAutomaticSandbox(ctx context.Context, backend bubblewrapBackend) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("automatic checks require Linux isolation")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	workspace, err := os.MkdirTemp("", "hn-check-probe-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspace)
	return backend.RunStep(ctx, workspace, PipelineStep{Command: "/usr/bin/true"}, nil, io.Discard)
}

func executeAutomaticRequest(ctx context.Context, candidate, request StoredEvent, identity *Identity, backend bubblewrapBackend) (*StoredEvent, error) {
	pipeline, _, digest, err := loadPipeline(request.Event.Automatic.Source, request.Event.Pipeline)
	if err != nil || digest != request.Event.Definition {
		return nil, fmt.Errorf("pinned pipeline definition is unavailable")
	}
	result, err := executePipeline(ctx, candidate.Event.Head, request.Event.Pipeline, pipeline, backend)
	if err != nil {
		return nil, err
	}
	return signRunResult(request.Event, request.ID, identity, backend, result)
}
