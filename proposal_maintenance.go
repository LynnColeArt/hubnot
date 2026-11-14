package main

import "strings"

const mergeRepairBlocker = "The proposal code is already present. Retry the original merge confirmation to recover its record."

type proposalMaintenanceTarget struct {
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}

func maintainProposal(args []string) (any, error) {
	if len(args) == 0 || !validEventID(args[0]) {
		return nil, consumerFailure("invalid_input", "Provide an exact proposal revision.")
	}
	id := args[0]
	flags := quietFlags("proposal maintain")
	action := flags.String("action", "inspect", "")
	reason := flags.String("reason", "", "")
	actor := flags.String("actor", "", "")
	branch := flags.String("branch", "", "")
	commit := flags.String("commit", "", "")
	flags.Bool("json", false, "")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 ||
		(*action != "inspect" && *action != "accept" && *action != "reject" && *action != "merge") ||
		(*action != "inspect" && !validActorID(*actor)) ||
		(*action == "merge" && (*branch == "" || !validGitOID(*commit))) {
		return nil, consumerFailure("invalid_input", "Provide the action, signer and confirmed merge target.")
	}
	*reason = strings.TrimSpace(*reason)
	if *action == "reject" && (*reason == "" || len(*reason) > 4096) {
		return nil, consumerFailure("invalid_input", "Provide a rejection reason of up to 4096 bytes.")
	}
	if *action != "inspect" {
		dir, err := contributionSharedDirectory(true)
		if err != nil {
			return nil, err
		}
		unlock, err := lockRepositoryOperation(dir, "proposal-maintenance")
		if err != nil {
			return nil, consumerFailure("concurrent_update", "A proposal action is in progress. Refresh before retrying.")
		}
		defer unlock()
	}
	if err := prepareShallowVerification(shallowVerificationScope{Operation: "proposal status", Subject: id}); err != nil {
		return nil, err
	}
	if err := guardProposalQuery("proposal status", id); err != nil {
		return nil, err
	}
	events, err := collectGovernedEvents()
	if err != nil {
		return nil, err
	}
	proposal, err := resolveFullEvent(events, id)
	if err != nil {
		return nil, err
	}
	if err := contributionRequireProposal(id); err != nil {
		return nil, err
	}
	evaluation, err := evaluateProposal(proposal, events)
	if err != nil {
		return nil, err
	}
	identity, _ := loadIdentity()
	signer := ""
	if identity != nil {
		signer = identity.Actor
	}
	maintainer := signer != "" && actorListed(signer, evaluation.Policy.Maintainers)
	alreadyAccepted := false
	for _, decision := range evaluation.AcceptDecisions {
		if decision.Event.Actor == signer {
			alreadyAccepted = true
		}
	}
	terminal := requireLineageTerminalEligibility("advance", id, evaluation.Lineage) == nil &&
		!evaluation.Rejected && !evaluation.Merged
	if *action == "inspect" {
		target, blocker := inspectProposalMergeTarget(proposal, evaluation, *branch)
		return map[string]any{
			"actor": signer, "maintainer": maintainer, "target": target, "merge_blocker": blocker,
			"reject": maintainer && terminal,
			"accept": maintainer && terminal && evaluation.Ready && !alreadyAccepted && !evaluation.Accepted,
			"merge": maintainer && terminal && evaluation.Ready && evaluation.Accepted &&
				len(evaluation.BlockingReviewThreads) == 0 && blocker == "",
		}, nil
	}
	if signer == "" || signer != *actor {
		return nil, consumerFailure("actor_mismatch", "The selected signing identity changed.")
	}
	if !maintainer {
		return nil, consumerFailure("permission_denied", "Only a maintainer under this revision’s policy can perform this action.")
	}
	if *action == "reject" {
		for _, prior := range evaluation.RejectDecisions {
			if prior.Event.Actor == signer && prior.Event.Body == *reason {
				return consumerMutationReceipt(&prior), nil
			}
		}
		if !terminal {
			return nil, consumerFailure("stale_revision", "This revision can no longer be rejected.")
		}
		return nil, decideProposal([]string{id, "--reject", "--body", *reason, "--json"}, *actor)
	}
	if *action == "accept" {
		if !terminal {
			return nil, consumerFailure("stale_revision", "This revision can no longer be accepted.")
		}
		for _, prior := range evaluation.AcceptDecisions {
			if prior.Event.Actor == signer {
				return consumerMutationReceipt(&prior), nil
			}
		}
		if !evaluation.Ready || evaluation.Accepted {
			return nil, consumerFailure("stale_revision", "This revision is not awaiting acceptance from this maintainer. Check its requirements.")
		}
		return nil, decideProposal([]string{id, "--accept", "--json"}, *actor)
	}
	target := proposalMaintenanceTarget{Branch: *branch, Commit: *commit}
	if evaluation.Merged && !evaluation.Lineage.MergeConflict {
		// Recover a lost response only for the confirmed target.
		for _, merged := range evaluation.MergeEvents {
			parent, err := resolveCommit(merged.Event.Commit + "^1")
			tip, tipErr := resolveCommit("refs/heads/" + target.Branch)
			if tipErr != nil {
				break
			}
			contained, missing, ancestryErr := exactCommitAncestor(merged.Event.Commit, tip)
			if err == nil && parent == target.Commit && ancestryErr == nil && missing == "" && contained {
				return maintenanceMergeReceipt(&merged), nil
			}
		}
		return nil, consumerFailure("stale_revision", "This revision already merged. Refresh to view its merge record.")
	}
	if evaluation.Rejected {
		return nil, consumerFailure("stale_revision", "A maintainer rejected this revision. Open its rejection reason before proposing a new revision.")
	}
	_, blocker := inspectProposalMergeTarget(proposal, evaluation, target.Branch)
	if blocker != "" && blocker != mergeRepairBlocker {
		return nil, consumerFailure("stale_revision", blocker)
	}
	if !terminal || !evaluation.Ready || !evaluation.Accepted || len(evaluation.BlockingReviewThreads) > 0 {
		return nil, consumerFailure("stale_revision", "This revision has merge blockers. Refresh and check its requirements.")
	}
	guard := &proposalMergeGuard{
		ExpectedActor: signer,
		Check: func(_ *StoredEvent, _ *ProposalEvaluation, branch, commit string) error {
			if branch != target.Branch || commit != target.Commit {
				return consumerFailure("stale_revision", "The confirmed merge target changed.")
			}
			return nil
		},
		RepairCheck: func(branch, merged string) error {
			parent, err := resolveCommit(merged + "^1")
			if err != nil || branch != target.Branch || parent != target.Commit {
				return consumerFailure("stale_revision", "The recorded merge does not match the confirmed target.")
			}
			return nil
		},
		Receipt: func(stored *StoredEvent) error { return consumerOutput(maintenanceMergeReceipt(stored), nil) },
	}
	return nil, mergeProposal([]string{id}, guard)
}

func maintenanceMergeReceipt(stored *StoredEvent) map[string]any {
	return map[string]any{"id": stored.ID, "kind": stored.Event.Kind, "subject": stored.Event.Subject, "head": stored.Event.Head, "commit": stored.Event.Commit}
}

func inspectProposalMergeTarget(proposal *StoredEvent, evaluation *ProposalEvaluation, requested string) (proposalMaintenanceTarget, string) {
	var target proposalMaintenanceTarget
	if requested == "" {
		return target, "The project’s default branch is unavailable."
	}
	branch, err := gitText("symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || branch != requested {
		return target, "Check out the project’s default branch to merge this revision."
	}
	commit, err := resolveCommit("HEAD")
	if err != nil {
		return target, "The checked-out target commit is unavailable."
	}
	target = proposalMaintenanceTarget{Branch: branch, Commit: commit}
	status, err := gitText("status", "--porcelain", "--untracked-files=normal")
	if err != nil || strings.TrimSpace(status) != "" {
		return target, "The target checkout must be clean before merging."
	}
	for _, operation := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "REBASE_HEAD"} {
		if _, err := gitText("rev-parse", "--verify", "--quiet", operation); err == nil {
			return target, "Finish the Git operation already in progress before merging."
		}
	}
	if err := guardMergeAncestry("proposal merge", proposal, commit); err != nil {
		return target, "The target branch must descend from this revision’s signed base."
	}
	contained, missing, err := exactCommitAncestorUntil(proposal.Event.Head, commit, proposal.Event.Base)
	if err != nil || missing != "" {
		return target, "The target’s proposal history could not be verified."
	}
	if contained && !evaluation.Merged {
		return target, mergeRepairBlocker
	}
	if err := guardTargetMergePolicy(commit, evaluation); err != nil {
		return target, "The target policy or protected paths changed. Revise against the current target."
	}
	return target, ""
}
