package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

const policyCheckUsage = "usage: hn policy check --base REV <--head REV|--file PATH>"

type policySource struct {
	Policy PolicyDocument
	Bytes  []byte
	Digest string
	Commit string
	File   string
}

type policyChange struct {
	Label string
	Value string
}

type policyAmendmentInspection struct {
	Base     policySource
	Proposed policySource
	Changes  []policyChange
}

func cmdPolicy(args []string) error {
	if len(args) == 0 {
		return usageError("usage: hn policy <show|check>")
	}
	switch args[0] {
	case "show":
		return cmdPolicyShow(args[1:])
	case "check":
		return cmdPolicyCheck(args[1:])
	default:
		return fmt.Errorf("unknown policy command %q", args[0])
	}
}

func cmdPolicyShow(args []string) error {
	if len(args) > 1 {
		return usageError("usage: hn policy show [REV]")
	}
	revision := "HEAD"
	if len(args) == 1 {
		revision = args[0]
	}
	if err := prepareShallowVerification(shallowVerificationScope{Operation: "policy show", Subject: revision}); err != nil {
		return err
	}
	source, err := loadPolicyRevision("policy", revision)
	if err != nil {
		return err
	}
	printPolicySource(source)
	return nil
}

func cmdPolicyCheck(args []string) error {
	flags := quietFlags("policy check")
	baseRevision := flags.String("base", "", "base Git revision")
	headRevision := flags.String("head", "", "proposed Git revision")
	filePath := flags.String("file", "", "proposed policy file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *baseRevision == "" {
		return usageError(policyCheckUsage)
	}
	if (*headRevision == "") == (*filePath == "") {
		return usageError("exactly one of --head and --file is required")
	}
	if err := prepareShallowVerification(shallowVerificationScope{
		Operation: "policy check", Base: *baseRevision, Head: *headRevision, ProposedFile: *filePath,
	}); err != nil {
		return err
	}

	base, err := loadPolicyRevision("base", *baseRevision)
	if err != nil {
		return err
	}
	var proposed policySource
	if *headRevision != "" {
		proposed, err = loadPolicyRevision("proposed", *headRevision)
	} else {
		proposed, err = loadPolicyFile("proposed", *filePath)
	}
	if err != nil {
		return err
	}
	printPolicyCheck(policyAmendmentInspection{
		Base:     base,
		Proposed: proposed,
		Changes:  comparePolicies(base, proposed),
	})
	return nil
}

func loadPolicyRevision(side, revision string) (policySource, error) {
	label := side + " policy"
	if side == "policy" {
		label = side
	}
	kind := shallowBaseCommit
	if side == "proposed" {
		kind = shallowProposalCodeRef
	}
	commit, err := resolveCommitDependency(label, kind, revision, "", "")
	if err != nil {
		return policySource{}, fmt.Errorf("%s: %w", label, err)
	}
	if err := guardBasePolicy(label, commit, "", ""); err != nil {
		return policySource{}, fmt.Errorf("%s: %w", label, err)
	}
	policy, encoded, digest, err := loadPolicy(commit)
	if err != nil {
		return policySource{}, fmt.Errorf("%s at commit %s: %w", label, commit, err)
	}
	return policySource{Policy: policy, Bytes: encoded, Digest: digest, Commit: commit}, nil
}

func loadPolicyFile(side, path string) (policySource, error) {
	file, err := os.Open(path)
	if err != nil {
		return policySource{}, fmt.Errorf("%s policy file %s: %w", side, path, err)
	}
	encoded, readErr := io.ReadAll(io.LimitReader(file, maxPolicySize+1))
	closeErr := file.Close()
	if readErr != nil {
		return policySource{}, fmt.Errorf("%s policy file %s: %w", side, path, readErr)
	}
	if closeErr != nil {
		return policySource{}, fmt.Errorf("%s policy file %s: %w", side, path, closeErr)
	}
	policy, digest, err := parsePolicyBytes(encoded)
	if err != nil {
		return policySource{}, fmt.Errorf("%s policy file %s: %w", side, path, err)
	}
	return policySource{Policy: policy, Bytes: encoded, Digest: digest, File: path}, nil
}

// policyAmendmentDiagnostic is the read-only seam used by proposal creation.
// An empty message means the exact policy bytes did not change.
func policyAmendmentDiagnostic(baseRevision, headRevision string) (string, error) {
	baseCommit, err := resolveCommitDependency("base policy", shallowBaseCommit, baseRevision, "", "")
	if err != nil {
		return "", fmt.Errorf("base policy: %w", err)
	}
	headCommit, err := resolveCommitDependency("proposed policy", shallowProposalCodeRef, headRevision, "", "")
	if err != nil {
		return "", fmt.Errorf("proposed policy: %w", err)
	}
	if err := guardBasePolicy("base policy", baseCommit, "", ""); err != nil {
		return "", err
	}
	changedPath, err := gitText("diff", "--name-only", baseCommit, headCommit, "--", ".hn/policy.json")
	if err != nil {
		return "", fmt.Errorf("compare policy paths: %w", err)
	}
	if changedPath == "" {
		return "", nil
	}
	base, err := loadPolicyRevision("base", baseCommit)
	if err != nil {
		return "", err
	}
	proposed, err := loadPolicyRevision("proposed", headCommit)
	if err != nil {
		return "", err
	}
	if bytes.Equal(base.Bytes, proposed.Bytes) {
		return "", nil
	}
	return fmt.Sprintf("Policy amendment: base digest %s governs this candidate; proposed digest %s applies only to later candidates based on commit %s.", base.Digest, proposed.Digest, proposed.Commit), nil
}

func comparePolicies(base, proposed policySource) []policyChange {
	changes := []policyChange{{Label: "exact policy bytes", Value: changedLabel(!bytes.Equal(base.Bytes, proposed.Bytes))}}
	changes = append(changes, actorChanges("maintainers", base.Policy.Maintainers, proposed.Policy.Maintainers)...)
	changes = append(changes,
		policyChange{Label: "required accepts", Value: scalarChange(base.Policy.Proposals.RequiredAccepts, proposed.Policy.Proposals.RequiredAccepts)},
	)
	changes = append(changes, actorChanges("trusted reviewers", base.Policy.Proposals.TrustedReviewers, proposed.Policy.Proposals.TrustedReviewers)...)
	changes = append(changes,
		policyChange{Label: "required approvals", Value: scalarChange(base.Policy.Proposals.RequiredApprovals, proposed.Policy.Proposals.RequiredApprovals)},
		policyChange{Label: "author approval", Value: boolChange(base.Policy.Proposals.AllowAuthorApproval, proposed.Policy.Proposals.AllowAuthorApproval)},
	)

	baseNames := sortedPipelineNames(base.Policy.Pipelines)
	proposedNames := sortedPipelineNames(proposed.Policy.Pipelines)
	added, removed := sortedDifference(baseNames, proposedNames)
	changes = append(changes,
		policyChange{Label: "pipelines added", Value: joinedOrNone(added)},
		policyChange{Label: "pipelines removed", Value: joinedOrNone(removed)},
	)
	for _, name := range sortedUnion(baseNames, proposedNames) {
		basePipeline, inBase := base.Policy.Pipelines[name]
		proposedPipeline, inProposed := proposed.Policy.Pipelines[name]
		changes = append(changes,
			policyChange{Label: "pipeline " + name + " required results", Value: optionalScalarChange(basePipeline.RequiredResults, inBase, proposedPipeline.RequiredResults, inProposed)},
		)
		baseRunners := basePipeline.TrustedRunners
		if !inBase {
			baseRunners = nil
		}
		proposedRunners := proposedPipeline.TrustedRunners
		if !inProposed {
			proposedRunners = nil
		}
		changes = append(changes, actorChanges("pipeline "+name+" trusted runners", baseRunners, proposedRunners)...)
	}
	changes = append(changes, compareOptionalPolicies(base.Policy, proposed.Policy)...)
	return changes
}

func printPolicySource(source policySource) {
	fmt.Printf("Policy commit: %s\n", source.Commit)
	fmt.Printf("Policy digest: %s\n", source.Digest)
	printActors(fmt.Sprintf("Maintainers (required accepts %d):", source.Policy.Proposals.RequiredAccepts), source.Policy.Maintainers)
	printActors(fmt.Sprintf("Trusted reviewers (required approvals %d; author approval %t):", source.Policy.Proposals.RequiredApprovals, source.Policy.Proposals.AllowAuthorApproval), source.Policy.Proposals.TrustedReviewers)
	printOptionalPolicy(source.Policy)
	fmt.Println("Pipelines:")
	if len(source.Policy.Pipelines) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, name := range sortedPipelineNames(source.Policy.Pipelines) {
		pipeline := source.Policy.Pipelines[name]
		fmt.Printf("  %s (required results %d):\n", name, pipeline.RequiredResults)
		for _, actor := range sortedStrings(pipeline.TrustedRunners) {
			fmt.Printf("    %s\n", actor)
		}
	}
}

func printPolicyCheck(inspection policyAmendmentInspection) {
	fmt.Printf("Base commit: %s\n", inspection.Base.Commit)
	fmt.Printf("Base policy digest: %s\n", inspection.Base.Digest)
	if inspection.Proposed.Commit != "" {
		fmt.Printf("Proposed commit: %s\n", inspection.Proposed.Commit)
	} else {
		fmt.Printf("Proposed file: %s\n", inspection.Proposed.File)
	}
	fmt.Printf("Proposed policy digest: %s\n", inspection.Proposed.Digest)
	fmt.Println("The base policy governs this amendment candidate.")
	fmt.Println("Changes:")
	for _, change := range inspection.Changes {
		fmt.Printf("  %s: %s\n", change.Label, change.Value)
	}
}

func printActors(label string, actors []string) {
	fmt.Println(label)
	if len(actors) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, actor := range sortedStrings(actors) {
		fmt.Printf("  %s\n", actor)
	}
}

func actorChanges(label string, base, proposed []string) []policyChange {
	added, removed := sortedDifference(sortedStrings(base), sortedStrings(proposed))
	return []policyChange{
		{Label: label + " added", Value: joinedOrNone(added)},
		{Label: label + " removed", Value: joinedOrNone(removed)},
	}
}

func sortedDifference(base, proposed []string) (added, removed []string) {
	baseSet := stringSet(base)
	proposedSet := stringSet(proposed)
	for value := range proposedSet {
		if !baseSet[value] {
			added = append(added, value)
		}
	}
	for value := range baseSet {
		if !proposedSet[value] {
			removed = append(removed, value)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func sortedUnion(first, second []string) []string {
	values := stringSet(first)
	for _, value := range second {
		values[value] = true
	}
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sortedPipelineNames(pipelines map[string]PipelinePolicy) []string {
	names := make([]string, 0, len(pipelines))
	for name := range pipelines {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func joinedOrNone(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, ", ")
}

func changedLabel(changed bool) string {
	if changed {
		return "changed"
	}
	return "unchanged"
}

func scalarChange(base, proposed int) string {
	if base == proposed {
		return fmt.Sprintf("unchanged (%d)", base)
	}
	return fmt.Sprintf("changed (%d -> %d)", base, proposed)
}

func boolChange(base, proposed bool) string {
	if base == proposed {
		return fmt.Sprintf("unchanged (%t)", base)
	}
	return fmt.Sprintf("changed (%t -> %t)", base, proposed)
}

func optionalScalarChange(base int, inBase bool, proposed int, inProposed bool) string {
	switch {
	case !inBase:
		return "added (" + strconv.Itoa(proposed) + ")"
	case !inProposed:
		return "removed (" + strconv.Itoa(base) + ")"
	default:
		return scalarChange(base, proposed)
	}
}

func printOptionalPolicy(policy PolicyDocument) {
	if p := policy.Audits; p != nil {
		fmt.Printf("Audits: enabled (scope %s; observational)\n", p.Scope)
		printActors("Audit rule owners:", p.RuleOwners)
		printActors("Audit requesters:", p.Requesters)
		printActors("Audit executors:", p.Executors)
	} else {
		fmt.Println("Audits: disabled")
	}
	if p := policy.ProtectedChecks; p != nil {
		fmt.Printf("Protected checks: enabled (required designated approvals %d; author excluded)\n", p.RequiredApprovals)
		printActors("Designated reviewers:", p.TrustedReviewers)
		fmt.Println("Protected paths (including policy and required pipelines):")
		for _, path := range protectedPaths(policy) {
			fmt.Printf("  %q\n", path)
		}
	} else {
		fmt.Println("Protected checks: disabled")
	}
	if p := policy.Automerge; p != nil {
		fmt.Println("Automerge: enabled (explicit execution only)")
		rules := append([]AutomergeRule(nil), p.Rules...)
		sort.Slice(rules, func(i, j int) bool { return rules[i].Name < rules[j].Name })
		for _, r := range rules {
			fmt.Printf("Rule %s; target branch %s:\n", r.Name, r.TargetBranch)
			printActors("  Executing maintainers:", r.Actors)
			printActors("  Proposal authors:", r.Authors)
			fmt.Println("  Allowed literal paths:")
			for _, path := range sortedStrings(r.Paths) {
				fmt.Printf("    %q\n", path)
			}
		}
	} else {
		fmt.Println("Automerge: disabled")
	}
}

func compareOptionalPolicies(base, proposed PolicyDocument) []policyChange {
	changes := []policyChange{{Label: "protected checks enabled", Value: boolChange(base.ProtectedChecks != nil, proposed.ProtectedChecks != nil)}, {Label: "automerge enabled", Value: boolChange(base.Automerge != nil, proposed.Automerge != nil)}}
	changes = append(changes, policyChange{Label: "audits enabled", Value: boolChange(base.Audits != nil, proposed.Audits != nil)})
	var oldAudit, newAudit AuditPolicy
	if base.Audits != nil {
		oldAudit = *base.Audits
	}
	if proposed.Audits != nil {
		newAudit = *proposed.Audits
	}
	changes = append(changes, policyChange{Label: "audit scope", Value: fmt.Sprintf("%q -> %q", oldAudit.Scope, newAudit.Scope)})
	changes = append(changes, actorChanges("audit rule owners", oldAudit.RuleOwners, newAudit.RuleOwners)...)
	changes = append(changes, actorChanges("audit requesters", oldAudit.Requesters, newAudit.Requesters)...)
	changes = append(changes, actorChanges("audit executors", oldAudit.Executors, newAudit.Executors)...)
	var a, b ProtectedChecksPolicy
	if base.ProtectedChecks != nil {
		a = *base.ProtectedChecks
	}
	if proposed.ProtectedChecks != nil {
		b = *proposed.ProtectedChecks
	}
	threshold := "unchanged (disabled)"
	if base.ProtectedChecks != nil || proposed.ProtectedChecks != nil {
		threshold = optionalScalarChange(a.RequiredApprovals, base.ProtectedChecks != nil, b.RequiredApprovals, proposed.ProtectedChecks != nil)
	}
	changes = append(changes, policyChange{Label: "protected checks required approvals", Value: threshold})
	changes = append(changes, actorChanges("protected checks designated reviewers", a.TrustedReviewers, b.TrustedReviewers)...)
	added, removed := sortedDifference(sortedStrings(a.Paths), sortedStrings(b.Paths))
	changes = append(changes, policyChange{Label: "protected paths added", Value: joinedOrNone(added)}, policyChange{Label: "protected paths removed", Value: joinedOrNone(removed)})
	oldRules, newRules := map[string]AutomergeRule{}, map[string]AutomergeRule{}
	if base.Automerge != nil {
		for _, r := range base.Automerge.Rules {
			oldRules[r.Name] = r
		}
	}
	if proposed.Automerge != nil {
		for _, r := range proposed.Automerge.Rules {
			newRules[r.Name] = r
		}
	}
	names := []string{}
	for name := range oldRules {
		names = append(names, name)
	}
	for name := range newRules {
		names = append(names, name)
	}
	for _, name := range sortedUniqueStrings(names...) {
		old, was := oldRules[name]
		next, is := newRules[name]
		changes = append(changes, policyChange{Label: "automerge rule " + name + " enabled", Value: boolChange(was, is)}, policyChange{Label: "automerge rule " + name + " target branch", Value: fmt.Sprintf("%q -> %q", old.TargetBranch, next.TargetBranch)})
		changes = append(changes, actorChanges("automerge rule "+name+" actors", old.Actors, next.Actors)...)
		changes = append(changes, actorChanges("automerge rule "+name+" authors", old.Authors, next.Authors)...)
		added, removed := sortedDifference(sortedStrings(old.Paths), sortedStrings(next.Paths))
		changes = append(changes, policyChange{Label: "automerge rule " + name + " paths added", Value: joinedOrNone(added)}, policyChange{Label: "automerge rule " + name + " paths removed", Value: joinedOrNone(removed)})
	}
	return changes
}
