package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

const consumerSchema = "hn.consumer/1"

// consumerError uses the native closed vocabulary documented in extensions.md.
// These CLI intents have no operation key, so recovery never promises a safe
// mutation replay and operation is always null.
type consumerError struct {
	Origin    string `json:"origin"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Recovery  string `json:"recovery"`
	Operation any    `json:"operation"`
}

func (e *consumerError) Error() string { return e.Code + ": " + e.Message }
func consumerFailure(code, message string) error {
	return &consumerError{Origin: "native", Code: code, Message: message, Recovery: "do_not_retry"}
}

func consumerOutput(data any, err error) error {
	var failure *consumerError
	if err != nil {
		failure = &consumerError{Origin: "native", Code: "repository_error", Message: "native repository operation failed", Recovery: "do_not_retry"}
		var public *consumerError
		var command *issueCommandError
		var read *IssueReadError
		var resolution *eventResolutionError
		switch {
		case errors.As(err, &public):
			*failure = *public
		case errors.As(err, &command):
			failure.Code = command.Code
			failure.Message = command.Message
		case errors.As(err, &resolution):
			failure.Code = resolution.NativeCode()
			failure.Message = "event not found in selected history"
			if resolution.ambiguous {
				failure.Message = "event prefix is ambiguous"
			}
		case errors.As(err, &read):
			failure.Code = read.Code
			failure.Message = "native accepted history could not be verified"
		}
		switch failure.Code {
		case "actor_mismatch", "admission_pending", "admission_refused", "ambiguous_id", "concurrent_update", "invalid_cursor", "invalid_history", "invalid_input", "issue_conflict", "not_found", "operation_conflict", "permission_denied", "repository_error", "resource_conflict", "resource_limit", "stale_cursor", "stale_revision":
		default:
			failure.Code = "repository_error"
			failure.Message = "native repository operation failed"
		}
		// Unkeyed commands cannot inspect an original operation or promise replay.
		failure.Origin = "native"
		failure.Recovery = "do_not_retry"
		failure.Operation = nil
		if !utf8.ValidString(failure.Message) || len(failure.Message) > 4096 || strings.ContainsAny(failure.Message, "/\\\r\n") {
			failure.Message = "native operation refused; inspect local native diagnostics"
		}
		data = nil
	}
	if e := json.NewEncoder(os.Stdout).Encode(struct {
		OK    bool           `json:"ok"`
		Data  any            `json:"data"`
		Error *consumerError `json:"error"`
	}{err == nil, data, failure}); e != nil {
		return e
	}
	if err != nil {
		return fmt.Errorf("consumer request refused")
	}
	return nil
}

// Static discovery is dispatched before contribution locks or optional module
// bootstrap. Its data describes only support compiled and implemented here.
func cmdConsumerDiscovery(args []string) error {
	flags := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	machine := false
	flags.BoolVar(&machine, "json", false, "versioned JSON output")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !machine {
		return consumerOutput(nil, consumerFailure("invalid_input", "discovery requires --json and no other arguments"))
	}
	return consumerOutput(map[string]any{
		"schema": consumerSchema, "version": version,
		"capabilities": map[string]any{
			"native.discovery":           map[string]any{"version": 1, "commands": []string{"version --json", "--version --json", "capabilities --json"}},
			"native.review-receipt":      map[string]any{"version": 1, "commands": []string{"review FULL_PROPOSAL --approve|--request-changes [--body TEXT] --json"}, "operation_key": false},
			"native.decision-receipt":    map[string]any{"version": 1, "commands": []string{"decide FULL_PROPOSAL --accept|--reject [--body TEXT] --json"}, "operation_key": false},
			"native.event-read":          map[string]any{"version": 1, "commands": []string{"event show EVENT [--json]", "event resolve EVENT [--json]"}, "minimum_prefix_hex": 12},
			"native.log-read":            map[string]any{"version": 1, "commands": []string{"log --json"}, "history_scope": "raw_signed_history"},
			"native.proposal-read":       map[string]any{"version": 1, "commands": []string{"proposal show PROPOSAL --json", "proposal status PROPOSAL --json"}},
			"contribution.existing-json": map[string]any{"version": 1, "envelope": false, "mode": "existing-default-json", "documentation": "docs/consumer-cli-v1.md"},
		},
	}, nil)
}

// Discovery has no string-valued options. Respect '--' so a positional value
// that happens to be --json never selects the new version output mode.
func consumerDiscoveryJSON(args []string) bool {
	flags := flag.NewFlagSet("discovery", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Bool("json", false, "versioned JSON")
	return issueMachineRequested(args, flags)
}

// consumerProposalOptions recognizes only this command's boolean option grammar.
// No string values are scanned and '--' ends option recognition.
func consumerProposalOptions(args []string) (machine bool, action, query string, err error) {
	if len(args) < 2 || args[0] != "proposal" || (args[1] != "show" && args[1] != "status") {
		return
	}
	action = args[1]
	rest := args[2:]
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		query = rest[0]
		rest = rest[1:]
	}
	var invalid bool
	for i, arg := range rest {
		if arg == "--" {
			if i+1 < len(rest) {
				invalid = true
			}
			break
		}
		switch arg {
		case "--json", "--json=true":
			machine = true
		case "--json=false":
			machine = false
		default:
			invalid = true
		}
	}
	if machine && (invalid || query == "") {
		err = consumerFailure("invalid_input", "proposal read requires PROPOSAL and --json only")
	}
	if machine && action == "status" && !validEventID(query) {
		err = consumerFailure("invalid_input", "proposal status requires a full proposal event ID")
	}
	return
}

func consumerStrings(values []string) []string { return append([]string{}, values...) }
func consumerIDs(events []StoredEvent) []string {
	ids := []string{}
	for _, e := range events {
		ids = append(ids, e.ID)
	}
	return ids
}
func consumerEventReference(s StoredEvent) map[string]any {
	return map[string]any{"id": s.ID, "actor": s.Event.Actor, "actor_name": s.Event.ActorName, "sequence": s.Event.Sequence, "kind": s.Event.Kind, "subject": s.Event.Subject, "verdict": s.Event.Verdict, "commit": s.Event.Commit, "definition": s.Event.Definition, "pipeline": s.Event.Pipeline, "outcome": s.Event.Outcome}
}
func consumerProposalData(action, query string) (any, error) {
	operation := "proposal " + action
	if err := prepareShallowVerification(shallowVerificationScope{Operation: operation, Subject: query}); err != nil {
		return nil, err
	}
	if err := guardProposalQuery(operation, query); err != nil {
		return nil, err
	}
	events, err := collectGovernedEvents()
	if err != nil {
		return nil, err
	}
	proposal, err := resolveEvent(events, query)
	if err != nil {
		return nil, err
	}
	if !isProposalKind(proposal.Event.Kind) {
		return nil, consumerFailure("invalid_input", "event is not a proposal")
	}
	if err = contributionRequireProposal(proposal.ID); err != nil {
		return nil, err
	}
	evaluation, err := evaluateProposal(proposal, events)
	if err != nil {
		return nil, err
	}
	return consumerProposalProjection(evaluation, events), nil
}
func consumerProposalProjection(e *ProposalEvaluation, events []StoredEvent) map[string]any {
	p := e.Proposal
	reviews := map[string]any{}
	observedApprovals, observedChanges := 0, 0
	for _, review := range currentReviews(events, p.ID) {
		eligible := actorListed(review.Event.Actor, e.Policy.Proposals.TrustedReviewers) && (e.Policy.Proposals.AllowAuthorApproval || review.Event.Actor != p.Event.Actor)
		protected := e.Policy.ProtectedChecks != nil && actorListed(review.Event.Actor, e.Policy.ProtectedChecks.TrustedReviewers) && review.Event.Actor != p.Event.Actor
		row := consumerEventReference(review)
		row["policy_eligible"] = eligible
		row["protected_policy_eligible"] = protected
		reviews[review.Event.Actor] = row
		if review.Event.Verdict == "approve" {
			observedApprovals++
		} else {
			observedChanges++
		}
	}
	deficit := func(required, actual int) int {
		if actual >= required {
			return 0
		}
		return required - actual
	}
	satisfied, missing := []string{}, []string{}
	requirement := func(name string, met bool) {
		if met {
			satisfied = append(satisfied, name)
		} else {
			missing = append(missing, name)
		}
	}
	requirement("code", e.CodeMatches)
	requirement("reviews", len(e.Approvals) >= e.Policy.Proposals.RequiredApprovals)
	eligible := []string{}
	for _, actor := range e.Policy.Proposals.TrustedReviewers {
		if e.Policy.Proposals.AllowAuthorApproval || actor != p.Event.Actor {
			eligible = append(eligible, actor)
		}
	}
	pipelines := []any{}
	for _, pipeline := range e.Pipelines {
		policy := e.Policy.Pipelines[pipeline.Name]
		passed, failed := []any{}, []any{}
		for _, r := range pipeline.Passed {
			passed = append(passed, consumerEventReference(r))
		}
		for _, r := range pipeline.Failed {
			failed = append(failed, consumerEventReference(r))
		}
		met := len(pipeline.Passed) >= pipeline.Required
		requirement("pipeline:"+pipeline.Name, met)
		pipelines = append(pipelines, map[string]any{"name": pipeline.Name, "required": pipeline.Required, "passes": len(pipeline.Passed), "deficit": deficit(pipeline.Required, len(pipeline.Passed)), "satisfied": met, "trusted_runners": consumerStrings(policy.TrustedRunners), "requesters": consumerStrings(policy.Requesters), "passed": passed, "failed": failed})
	}
	var protection any
	if pc := e.ProtectedChecks; pc != nil {
		applies := len(pc.ChangedPaths) > 0
		met := !applies || len(pc.Approvals) >= pc.Required
		need := 0
		if applies {
			need = deficit(pc.Required, len(pc.Approvals))
		}
		protectedEligible := []string{}
		for _, actor := range e.Policy.ProtectedChecks.TrustedReviewers {
			if actor != p.Event.Actor {
				protectedEligible = append(protectedEligible, actor)
			}
		}
		protection = map[string]any{"applies": applies, "paths": consumerStrings(e.Policy.ProtectedChecks.Paths), "changed_paths": consumerStrings(pc.ChangedPaths), "eligible_actors": protectedEligible, "trusted_reviewer_pool": consumerStrings(e.Policy.ProtectedChecks.TrustedReviewers), "required": pc.Required, "approvals": len(pc.Approvals), "deficit": need, "satisfied": met, "evidence": consumerIDs(pc.Approvals)}
		requirement("protected_checks", met)
	}
	l := e.Lineage
	lineageTerminal := !l.MergeConflict && !l.Superseded && !l.LineageClosed
	return map[string]any{
		"schema": consumerSchema, "id": p.ID, "kind": p.Event.Kind, "actor": p.Event.Actor, "actor_name": p.Event.ActorName, "title": e.DisplayTitle, "body": p.Event.Body, "base": p.Event.Base, "head": p.Event.Head, "policy": e.PolicyDigest, "status": proposalStatus(e),
		"current_reviews": reviews, "readiness": e.Ready, "evidence": consumerStrings(e.Evidence),
		"requirements":           map[string]any{"satisfied": satisfied, "missing": missing, "code": map[string]any{"available": e.CodeAvailable, "matches": e.CodeMatches}, "reviews": map[string]any{"eligible_actors": eligible, "trusted_reviewer_pool": consumerStrings(e.Policy.Proposals.TrustedReviewers), "allow_author_approval": e.Policy.Proposals.AllowAuthorApproval, "required": e.Policy.Proposals.RequiredApprovals, "approvals": len(e.Approvals), "deficit": deficit(e.Policy.Proposals.RequiredApprovals, len(e.Approvals)), "satisfied": len(e.Approvals) >= e.Policy.Proposals.RequiredApprovals, "evidence": consumerIDs(e.Approvals)}, "pipelines": pipelines, "protected_checks": protection},
		"lineage":                map[string]any{"root": l.RootID, "predecessor": l.PredecessorID, "successors": consumerStrings(l.SuccessorIDs), "siblings": consumerStrings(l.SiblingIDs), "ancestors": consumerStrings(l.AncestorIDs), "descendants": consumerStrings(l.DescendantIDs), "members": consumerStrings(l.MemberIDs), "merge_events": consumerStrings(l.MergeEventIDs), "merged_candidates": consumerStrings(l.MergedCandidateIDs), "merged": l.Merged, "superseded": l.Superseded, "closed": l.LineageClosed, "merge_conflict": l.MergeConflict},
		"disagreements":          map[string]any{"observed_change_requests": observedChanges, "qualified_change_requests": len(e.ChangeRequests), "observed_approvals": observedApprovals, "conflicting_current_verdicts": observedApprovals > 0 && observedChanges > 0},
		"decisions":              map[string]any{"eligible_actors": consumerStrings(e.Policy.Maintainers), "required_accepts": e.Policy.Proposals.RequiredAccepts, "accepts": consumerIDs(e.AcceptDecisions), "rejects": consumerIDs(e.RejectDecisions), "accept_deficit": deficit(e.Policy.Proposals.RequiredAccepts, len(e.AcceptDecisions)), "accepted": e.Accepted, "rejected": e.Rejected, "merged": e.Merged, "merge_events": consumerIDs(e.MergeEvents)},
		"observed_preconditions": map[string]any{"scope": "selected_history_evidence_and_lineage_only", "lineage_terminal": lineageTerminal, "acceptance": map[string]any{"met": e.Ready && lineageTerminal && !e.Merged, "evidence_ready": e.Ready, "not_merged": !e.Merged}, "merge": map[string]any{"met": e.Ready && e.Accepted && !e.Rejected && lineageTerminal && !e.Merged, "evidence_ready": e.Ready, "accepted": e.Accepted, "not_rejected": !e.Rejected, "not_merged": !e.Merged}},
	}
}

func consumerEventOptions(args []string) (machine bool, action, query string, err error) {
	if len(args) == 0 || (args[0] != "event" && args[0] != "log") {
		return
	}
	rest := args[1:]
	action = "log"
	valid := true
	if args[0] == "event" {
		action = ""
		if len(rest) > 0 {
			action = rest[0]
			rest = rest[1:]
		}
		valid = action == "show" || action == "resolve"
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			query = rest[0]
			rest = rest[1:]
		}
	}
	flags := flag.NewFlagSet("event read", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Bool("json", false, "versioned JSON")
	machine = issueMachineRequested(rest, flags)
	parseErr := flags.Parse(rest)
	if parseErr != nil || flags.NArg() != 0 || !valid || (action != "log" && !consumerValidEventQuery(query)) {
		err = consumerFailure("invalid_input", "expected event show or resolve with a full ID or at least 12 hex characters, optionally --json")
	}
	return
}
func consumerValidEventQuery(query string) bool {
	hex := strings.TrimPrefix(query, "sha256:")
	if len(hex) < 12 || len(hex) > 64 {
		return false
	}
	for _, r := range hex {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
func consumerStoredEvent(s StoredEvent) map[string]any {
	return map[string]any{"id": s.ID, "commit": s.Commit, "event": json.RawMessage(s.Payload), "signature": base64.RawStdEncoding.EncodeToString(s.Signature)}
}
func consumerLogData() (any, error) {
	if err := prepareShallowVerification(shallowVerificationScope{Operation: "log"}); err != nil {
		return nil, err
	}
	if err := guardShallowEventClosure("log"); err != nil {
		return nil, err
	}
	events, err := collectEvents()
	if err != nil {
		return nil, err
	}
	rows := []any{}
	for _, event := range events {
		rows = append(rows, consumerStoredEvent(event))
	}
	return map[string]any{"schema": consumerSchema, "history_scope": "raw_signed_history", "events": rows}, nil
}

// Ordinary event reads preserve admission and visibility. Only evidence whose
// native candidate/request is unavailable is excluded; a revision's predecessor
// is lineage, not an authorization parent, and does not hide a visible successor.
func consumerSelectedEvents(events []StoredEvent) ([]StoredEvent, error) {
	view, _, err := contributionView(events)
	if err != nil {
		return nil, err
	}
	selected := contributionOperativeEvidence(events, view)
	if view == nil {
		return selected, nil
	}
	byID := map[string]StoredEvent{}
	for _, s := range selected {
		if (isIssueKind(s.Event.Kind) || isResourceKind(s.Event.Kind) || isProposalKind(s.Event.Kind)) && !view.Visible[s.ID] {
			continue
		}
		byID[s.ID] = s
	}
	for _, s := range selected {
		switch s.Event.Kind {
		case "review.submit", "proposal.decision", "proposal.merged", "run.request":
			candidate, ok := byID[s.Event.Subject]
			if !ok || !isProposalKind(candidate.Event.Kind) {
				delete(byID, s.ID)
			}
		}
	}
	for _, s := range selected {
		if s.Event.Kind == "run.result" || s.Event.Kind == "run.withdraw" {
			request, ok := byID[s.Event.Subject]
			if !ok || request.Event.Kind != "run.request" {
				delete(byID, s.ID)
			}
		}
	}
	result := []StoredEvent{}
	for _, s := range selected {
		if _, ok := byID[s.ID]; ok {
			result = append(result, s)
		}
	}
	return result, nil
}
func cmdConsumerEvent(args []string) error {
	machine, action, query, err := consumerEventOptions(append([]string{"event"}, args...))
	if err != nil {
		return err
	}
	// The event closure is the same verified closure used by raw log; unlike log,
	// the selected ordinary projection below additionally enforces moderation.
	if err = prepareShallowVerification(shallowVerificationScope{Operation: "log"}); err != nil {
		return err
	}
	if err = guardShallowEventClosure("event read"); err != nil {
		return err
	}
	events, err := collectEvents()
	if err != nil {
		return err
	}
	selected, err := consumerSelectedEvents(events)
	if err != nil {
		return err
	}
	stored, err := resolveEvent(selected, query)
	if err != nil {
		return err
	}
	if machine {
		data := consumerStoredEvent(*stored)
		data["schema"] = consumerSchema
		data["history_scope"] = "visible_operative_history"
		return consumerOutput(data, nil)
	}
	if action == "resolve" {
		fmt.Println(stored.ID)
	} else {
		fmt.Printf("%s  %s\nActor: %s (%s)\n", stored.ID, stored.Event.Kind, stored.Event.Actor, oneLine(stored.Event.ActorName))
		fmt.Printf("%s\n", safeText(string(stored.Payload)))
	}
	return nil
}

// Use the registered writer grammar for output selection even when parsing fails.
// In particular, a body value or literal after '--' cannot select machine mode.
func consumerMutationOptions(args []string) (machine bool, err error) {
	if len(args) == 0 || (args[0] != "review" && args[0] != "decide") {
		return
	}
	command := args[0]
	rest := args[1:]
	query := ""
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		query = rest[0]
		rest = rest[1:]
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Bool("json", false, "versioned JSON")
	body := flags.String("body", "", "explanation")
	first, second := new(bool), new(bool)
	if command == "review" {
		first = flags.Bool("approve", false, "approve")
		second = flags.Bool("request-changes", false, "request changes")
	} else {
		first = flags.Bool("accept", false, "accept")
		second = flags.Bool("reject", false, "reject")
	}
	machine = issueMachineRequested(rest, flags)
	parseErr := flags.Parse(rest)
	if machine && (parseErr != nil || flags.NArg() != 0 || !validEventID(query) || *first == *second || (command == "decide" && *second && strings.TrimSpace(*body) == "")) {
		err = consumerFailure("invalid_input", "writer requires a full proposal ID, exactly one verdict, and valid options")
	}
	return
}
func consumerMutationReceipt(stored *StoredEvent) map[string]any {
	return map[string]any{"schema": consumerSchema, "id": stored.ID, "kind": stored.Event.Kind, "subject": stored.Event.Subject, "actor": stored.Event.Actor, "actor_name": stored.Event.ActorName, "sequence": stored.Event.Sequence, "verdict": stored.Event.Verdict, "policy": stored.Event.Policy, "evidence": consumerStrings(stored.Event.Evidence)}
}

// Remove only registered JSON options before the original writer parser. This
// keeps its legacy flag inventory (including unknown-flag usage bytes) intact.
func consumerMutationArguments(command string, args []string) ([]string, bool) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Bool("json", false, "")
	flags.String("body", "", "")
	if command == "review" {
		flags.Bool("approve", false, "")
		flags.Bool("request-changes", false, "")
	} else {
		flags.Bool("accept", false, "")
		flags.Bool("reject", false, "")
	}
	machine := issueMachineRequested(args, flags)
	clean := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || arg == "-" || !strings.HasPrefix(arg, "-") {
			clean = append(clean, args[i:]...)
			break
		}
		name, value, assigned := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "=")
		registered := flags.Lookup(name)
		if registered != nil && name == "json" {
			if !assigned {
				continue
			}
			if _, err := strconv.ParseBool(value); err == nil {
				continue
			}
			// Invalid booleans must reach a refusing parser, never disappear and
			// accidentally turn a rejected request into a text-mode mutation.
		}
		clean = append(clean, arg)
		if registered != nil {
			boolean, isBool := registered.Value.(interface{ IsBoolFlag() bool })
			if (!isBool || !boolean.IsBoolFlag()) && !assigned && i+1 < len(args) {
				i++
				clean = append(clean, args[i])
			}
		}
	}
	return clean, machine
}
