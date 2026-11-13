package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

type automaticCheck struct {
	Attempts      []automaticAttempt `json:"attempts,omitempty"`
	ID            string             `json:"id"`
	Proposal      string             `json:"proposal"`
	Candidate     string             `json:"candidate"`
	Commit        string             `json:"commit"`
	Pipeline      string             `json:"pipeline"`
	Definition    string             `json:"definition"`
	Source        string             `json:"source"`
	Configuration string             `json:"configuration"`
	Status        string             `json:"status"`
	Reason        string             `json:"reason,omitempty"`
	Created       string             `json:"created"`
}

type automaticLedger struct {
	Version   string                     `json:"version"`
	Authority string                     `json:"authority"`
	Target    string                     `json:"target"`
	Source    string                     `json:"source"`
	Observed  map[string]string          `json:"observed"`
	Checks    map[string]*automaticCheck `json:"checks"`
}

func automaticLedgerPath() (string, error) {
	directory, err := contributionGitDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "hn-automatic-checks.json"), nil
}

func readAutomaticLedger(path string) (*automaticLedger, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, fmt.Errorf("unsafe automatic check ledger")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ledger automaticLedger
	if err := json.Unmarshal(raw, &ledger); err != nil {
		return nil, err
	}
	if ledger.Version != proposalChecksVersion || !validActorID(ledger.Authority) || !validPublicationBaseRef(ledger.Target) || ledger.Observed == nil || ledger.Checks == nil {
		return nil, fmt.Errorf("invalid automatic check ledger")
	}
	return &ledger, nil
}

func writeAutomaticLedger(path string, ledger *automaticLedger) error {
	raw, err := json.Marshal(ledger)
	if err != nil {
		return err
	}
	return replacePrivateFileAtomic(path, raw)
}

func lockAutomaticLedger(path string) (func(), error) {
	fd, err := syscall.Open(path+".lock", syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "automatic check ledger lock")
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		file.Close()
		return nil, fmt.Errorf("unsafe automatic check ledger lock")
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("automatic check ledger busy: %w", err)
	}
	return func() { file.Close() }, nil
}

func automaticTarget(ref string) (string, error) {
	if !validPublicationBaseRef(ref) {
		return "", fmt.Errorf("expected an exact local target branch ref")
	}
	source, exists, err := refValue(ref)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("automatic check target branch is unavailable")
	}
	if _, err := loadPolicyCommit(source); err != nil {
		return "", err
	}
	return source, nil
}

func loadPolicyCommit(source string) (PolicyDocument, error) {
	policy, _, _, err := loadPolicy(source)
	return policy, err
}

func automaticConfigAt(source string) (proposalChecksConfig, string, error) {
	_, exists, err := exactTreeEntry(source, proposalChecksPath)
	if err != nil {
		return proposalChecksConfig{}, "", err
	}
	if !exists {
		return proposalChecksConfig{proposalChecksVersion, map[string][]string{}}, "", nil
	}
	raw, err := gitOutput("show", source+":"+proposalChecksPath)
	if err != nil {
		return proposalChecksConfig{}, "", err
	}
	config, err := parseProposalChecksConfig(raw, func(name string) bool {
		_, _, _, err := loadPipeline(source, name)
		return err == nil
	})
	return config, eventID(raw), err
}

func automaticVisibleEvents() ([]StoredEvent, error) {
	if err := guardShallowEventClosure("proposal list"); err != nil {
		return nil, err
	}
	events, err := collectEvents()
	if err != nil {
		return nil, err
	}
	view, _, err := contributionView(events)
	if err != nil {
		return nil, err
	}
	if view == nil {
		return events, nil
	}
	var visible []StoredEvent
	for _, event := range events {
		if !isProposalKind(event.Event.Kind) || view.Admitted[event.ID] && view.Visible[event.ID] {
			visible = append(visible, event)
		}
	}
	return visible, nil
}

func observeAutomaticChecks(ledger *automaticLedger, source string, events []StoredEvent, baseline bool) error {
	if ledger.Source != "" && ledger.Source != source {
		if _, err := gitOutput("merge-base", "--is-ancestor", ledger.Source, source); err != nil {
			return fmt.Errorf("automatic check target history changed non-linearly")
		}
	}
	config, digest, err := automaticConfigAt(source)
	if err != nil {
		return err
	}
	lineage, err := buildLineageIndex(events)
	if err != nil {
		return err
	}
	for _, candidate := range events {
		if !isProposalKind(candidate.Event.Kind) || ledger.Observed[candidate.ID] != "" {
			continue
		}
		if baseline {
			ledger.Observed[candidate.ID] = "historical"
			continue
		}
		if candidate.Event.Kind == "proposal.revise" {
			predecessor, err := lineage.candidate(candidate.Event.Subject)
			if err != nil {
				return err
			}
			if predecessor.Event.Head == candidate.Event.Head {
				ledger.Observed[candidate.ID] = "unchanged"
				continue
			}
		}
		root, err := lineage.root(candidate.ID)
		if err != nil {
			return err
		}
		ledger.Observed[candidate.ID] = "ineligible"
		for pipeline, rules := range config.Pipelines {
			for _, rule := range rules {
				if rule != candidate.Event.Kind {
					continue
				}
				_, _, definition, err := loadPipeline(source, pipeline)
				if err != nil {
					return err
				}
				// This key is scoped to the registered repository ledger.
				id := eventID([]byte(root + "\n" + candidate.ID + "\n" + pipeline))
				ledger.Checks[id] = &automaticCheck{ID: id, Proposal: root, Candidate: candidate.ID, Commit: candidate.Event.Head,
					Pipeline: pipeline, Definition: definition, Source: source, Configuration: digest, Status: "queued", Created: time.Now().UTC().Format(time.RFC3339Nano)}
				if err := requireProposalCode(&candidate); err != nil {
					ledger.Checks[id].Status = "waiting"
					ledger.Checks[id].Reason = "Candidate code is unavailable. Sync the proposal code."
				}
				ledger.Observed[candidate.ID] = "scheduled"
			}
		}
	}
	ledger.Source = source
	return nil
}

func automaticSchedule(action, target string) error {
	path, err := automaticLedgerPath()
	if err != nil {
		return err
	}
	release, err := lockAutomaticLedger(path)
	if err != nil {
		return err
	}
	defer release()
	identity, err := inspectIdentity()
	if err != nil {
		return err
	}
	ledger, err := readAutomaticLedger(path)
	baseline := action == "init"
	if baseline {
		if err == nil {
			return fmt.Errorf("automatic check authority is already registered")
		}
		if !os.IsNotExist(err) {
			return err
		}
		ledger = &automaticLedger{Version: proposalChecksVersion, Authority: identity.Actor, Target: target, Observed: map[string]string{}, Checks: map[string]*automaticCheck{}}
	} else if err != nil {
		return err
	}
	if ledger.Authority != identity.Actor {
		return fmt.Errorf("active identity is not the registered scheduling authority")
	}
	source, err := automaticTarget(ledger.Target)
	if err != nil {
		return err
	}
	policy, err := loadPolicyCommit(source)
	if err != nil {
		return err
	}
	if !actorListed(identity.Actor, policy.Maintainers) {
		return fmt.Errorf("scheduling authority must be a target policy maintainer")
	}
	events, err := automaticVisibleEvents()
	if err != nil {
		return err
	}
	if err := observeAutomaticChecks(ledger, source, events, baseline); err != nil {
		return err
	}
	return writeAutomaticLedger(path, ledger)
}

func automaticCheckStatus(candidate string) error {
	path, err := automaticLedgerPath()
	if err != nil {
		return consumerOutput(nil, err)
	}
	ledger, err := readAutomaticLedger(path)
	checks := make([]*automaticCheck, 0)
	if os.IsNotExist(err) {
		events, readErr := automaticVisibleEvents()
		if readErr != nil {
			return consumerOutput(nil, readErr)
		}
		lineage, readErr := buildLineageIndex(events)
		if readErr != nil {
			return consumerOutput(nil, readErr)
		}
		revision, rootErr := lineage.candidate(candidate)
		if rootErr != nil {
			return consumerOutput(nil, consumerFailure("not_found", "Proposal revision is unavailable."))
		}
		policy, policyErr := loadPolicyCommit(revision.Event.Base)
		if policyErr != nil {
			return consumerOutput(nil, policyErr)
		}
		root, rootErr := lineage.root(candidate)
		if rootErr != nil {
			return consumerOutput(nil, consumerFailure("not_found", "Proposal revision is unavailable."))
		}
		index := map[string]*automaticCheck{}
		for _, request := range events {
			intent := request.Event.Automatic
			if intent == nil || request.Event.Subject != candidate || !automaticDefinition(request.Event, revision, policy) {
				continue
			}
			check := index[intent.Check]
			if check == nil {
				check = &automaticCheck{ID: intent.Check, Proposal: root, Candidate: candidate, Commit: request.Event.Commit,
					Pipeline: request.Event.Pipeline, Definition: request.Event.Definition, Source: intent.Source,
					Configuration: intent.Configuration, Status: "waiting", Created: request.Event.Timestamp}
				index[intent.Check] = check
			}
			attempt := automaticAttempt{ID: intent.Attempt, Status: "waiting", Started: request.Event.Timestamp, Request: request.ID}
			for _, result := range currentRunResults(events, request.ID) {
				if !automaticResultEligible(request.Event, result.Event) {
					continue
				}
				attempt.Status, attempt.Finished, attempt.Result = result.Event.Outcome, result.Event.Timestamp, result.ID
				check.Status = attempt.Status
			}
			check.Attempts = append(check.Attempts, attempt)
		}
		for _, check := range index {
			checks = append(checks, check)
		}
		sort.Slice(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
		return consumerOutput(map[string]any{"availability": "no_authority", "checks": checks}, nil)
	}
	if err != nil {
		return consumerOutput(nil, err)
	}
	for _, check := range ledger.Checks {
		if check.Candidate == candidate {
			checks = append(checks, check)
		}
	}
	sort.Slice(checks, func(i, j int) bool { return checks[i].Pipeline < checks[j].Pipeline })
	return consumerOutput(map[string]any{"availability": "local", "authority": ledger.Authority, "decision": ledger.Observed[candidate], "checks": checks}, nil)
}
