package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maxPolicyRules       = 64
	maxPolicyPathBytes   = 4096
	maxChangedPathsBytes = 8 << 20
)

// These sections are independently opt-in. Absence never supplies default rules.
type ProtectedChecksPolicy struct {
	Paths             []string `json:"paths"`
	TrustedReviewers  []string `json:"trustedReviewers"`
	RequiredApprovals int      `json:"requiredApprovals"`
}

type AutomergePolicy struct {
	Rules []AutomergeRule `json:"rules"`
}
type AutomergeRule struct {
	Name         string   `json:"name"`
	TargetBranch string   `json:"targetBranch"`
	Actors       []string `json:"actors"`
	Authors      []string `json:"authors"`
	Paths        []string `json:"paths"`
}

type ProtectedChecksEvaluation struct {
	ChangedPaths []string
	Required     int
	Approvals    []StoredEvent
}

func validateOptionalPolicy(policy PolicyDocument) error {
	if p := policy.ProtectedChecks; p != nil {
		if err := validatePolicyPaths("protectedChecks paths", p.Paths, false); err != nil {
			return err
		}
		if len(p.TrustedReviewers) < 1 || len(p.TrustedReviewers) > maxPolicyRules {
			return fmt.Errorf("protectedChecks requires 1 to %d trustedReviewers", maxPolicyRules)
		}
		if err := validateActorList("protectedChecks trustedReviewers", p.TrustedReviewers); err != nil {
			return err
		}
		if p.RequiredApprovals < 1 || p.RequiredApprovals > len(p.TrustedReviewers) {
			return fmt.Errorf("protectedChecks requiredApprovals must be between 1 and trusted reviewer count")
		}
	}
	if p := policy.Automerge; p != nil {
		if len(p.Rules) < 1 || len(p.Rules) > maxPolicyRules {
			return fmt.Errorf("automerge requires 1 to %d rules", maxPolicyRules)
		}
		names := map[string]bool{}
		for _, r := range p.Rules {
			if !validPipelineName(r.Name) || names[r.Name] {
				return fmt.Errorf("automerge has invalid or duplicate rule name %q", r.Name)
			}
			names[r.Name] = true
			if !validPolicyBranch(r.TargetBranch) {
				return fmt.Errorf("automerge rule %q has invalid targetBranch %q", r.Name, r.TargetBranch)
			}
			for label, actors := range map[string][]string{"actors": r.Actors, "authors": r.Authors} {
				if len(actors) < 1 || len(actors) > maxPolicyRules {
					return fmt.Errorf("automerge rule %q requires 1 to %d %s", r.Name, maxPolicyRules, label)
				}
				if err := validateActorList("automerge "+label, actors); err != nil {
					return err
				}
			}
			for _, actor := range r.Actors {
				if !actorListed(actor, policy.Maintainers) {
					return fmt.Errorf("automerge rule %q actor %s is not a maintainer", r.Name, actor)
				}
			}
			if err := validatePolicyPaths("automerge rule "+r.Name+" paths", r.Paths, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePolicyPaths(label string, paths []string, nonempty bool) error {
	if paths == nil || len(paths) > maxPolicyRules || (nonempty && len(paths) == 0) {
		return fmt.Errorf("%s requires an explicit array of at most %d literal paths", label, maxPolicyRules)
	}
	seen := map[string]bool{}
	for _, p := range paths {
		q := strings.TrimSuffix(p, "/")
		if p == "" || len(p) > maxPolicyPathBytes || !utf8.ValidString(p) || q == "" || q == "." || path.IsAbs(q) || path.Clean(q) != q || strings.ContainsAny(p, `\*?[]`) || strings.Contains(q, ":") || strings.IndexFunc(p, unicode.IsControl) >= 0 {
			return fmt.Errorf("%s contains invalid literal path %q", label, p)
		}
		for _, component := range strings.Split(q, "/") {
			if component == ".." || strings.EqualFold(component, ".git") {
				return fmt.Errorf("%s contains invalid literal path %q", label, p)
			}
		}
		if seen[p] {
			return fmt.Errorf("%s contains duplicate path %q", label, p)
		}
		seen[p] = true
	}
	return nil
}

func validPolicyBranch(branch string) bool {
	if branch == "" || len(branch) > 255 || strings.HasPrefix(branch, "-") || strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") || strings.HasSuffix(branch, ".") || strings.ContainsAny(branch, " ~^:?*[\\") || strings.Contains(branch, "..") || strings.Contains(branch, "@{") || strings.Contains(branch, "//") || strings.IndexFunc(branch, unicode.IsControl) >= 0 || !utf8.ValidString(branch) {
		return false
	}
	for _, part := range strings.Split(branch, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return branch != "@"
}

// Protection includes changed ancestors: replacing scripts/ with a symlink must
// not escape a rule that protects scripts/lint.sh or scripts/.
func protectedPathMatches(protected, changed string) bool {
	literal := strings.TrimSuffix(protected, "/")
	return changed == literal || strings.HasPrefix(literal, changed+"/") || (strings.HasSuffix(protected, "/") && strings.HasPrefix(changed, protected))
}

func protectedPaths(policy PolicyDocument) []string {
	if policy.ProtectedChecks == nil {
		return nil
	}
	paths := append([]string(nil), policy.ProtectedChecks.Paths...)
	paths = append(paths, ".hn/policy.json")
	for name := range policy.Pipelines {
		paths = append(paths, ".hn/pipelines/"+name+".json")
	}
	return sortedUniqueStrings(paths...)
}

// limitedPolicyBuffer bounds bytes as they arrive from Git, not after Output has
// allocated an arbitrarily large diff. Returning an error stops the copy pipe.
type limitedPolicyBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitedPolicyBuffer) Len() int       { return b.buffer.Len() }
func (b *limitedPolicyBuffer) Bytes() []byte  { return b.buffer.Bytes() }
func (b *limitedPolicyBuffer) String() string { return b.buffer.String() }

func (b *limitedPolicyBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		b.exceeded = true
		return 0, fmt.Errorf("Git output exceeds %d bytes", b.limit)
	}
	return b.buffer.Write(p)
}

func boundedPolicyGitOutput(gitDir string, limit int, arguments ...string) ([]byte, error) {
	args := []string{"--no-replace-objects"}
	if gitDir != "" {
		args = append(args, "--git-dir="+gitDir)
	}
	args = append(args, arguments...)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = os.Environ()
	var stdout = limitedPolicyBuffer{limit: limit}
	var stderr = limitedPolicyBuffer{limit: 64 << 10}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stdout.exceeded || stderr.exceeded {
			return nil, fmt.Errorf("policy Git output exceeds bound (%d bytes)", limit)
		}
		return nil, fmt.Errorf("policy Git inspection: %s", safeText(commandFailure(err, stderr.String())))
	}
	return stdout.Bytes(), nil
}

func changedPathsAt(gitDir, base, head string) ([]string, error) {
	// Objects and tree modes only; never consult a worktree or execute diff drivers.
	raw, err := boundedPolicyGitOutput(gitDir, maxChangedPathsBytes, "diff-tree", "--no-commit-id", "--no-ext-diff", "--no-textconv", "--no-renames", "--ignore-submodules=none", "-r", "--name-only", "-z", base, head, "--")
	if err != nil {
		return nil, fmt.Errorf("inspect changed paths: %w", err)
	}

	if len(raw) == 0 {
		return []string{}, nil
	}
	if raw[len(raw)-1] != 0 {
		return nil, fmt.Errorf("changed paths Git output lacks NUL terminator")
	}
	entries := bytes.Split(raw[:len(raw)-1], []byte{0})
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if len(entry) == 0 {
			return nil, fmt.Errorf("changed paths Git output contains empty path")
		}
		paths = append(paths, string(entry))
	}
	sort.Strings(paths)
	return paths, nil
}

func changedProtectedPaths(policy PolicyDocument, changed []string) []string {
	var result []string
	for _, p := range changed {
		for _, protected := range protectedPaths(policy) {
			if protectedPathMatches(protected, p) {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

func protectionReviewQualifies(review StoredEvent, proposal StoredEvent, protection *ProtectedChecksPolicy) bool {
	return protection != nil && review.Event.Kind == "review.submit" && review.Event.Subject == proposal.ID && review.Event.Verdict == "approve" && review.Event.Actor != proposal.Event.Actor && actorListed(review.Event.Actor, protection.TrustedReviewers)
}

func evaluateProtectedChecks(policy PolicyDocument, proposal StoredEvent, events []StoredEvent, changed []string) *ProtectedChecksEvaluation {
	if policy.ProtectedChecks == nil {
		return nil
	}
	result := &ProtectedChecksEvaluation{ChangedPaths: changedProtectedPaths(policy, changed), Required: policy.ProtectedChecks.RequiredApprovals}
	for _, review := range currentReviews(events, proposal.ID) {
		if protectionReviewQualifies(review, proposal, policy.ProtectedChecks) {
			result.Approvals = append(result.Approvals, review)
		}
	}
	return result
}

// Immutable evidence admission examines only the exact event IDs a decision
// signed. Current withdrawals affect later merge eligibility, never old bytes.
func validateProtectedDecisionEvidence(policy PolicyDocument, decision, proposal StoredEvent, byID map[string]StoredEvent, changed []string) error {
	if policy.ProtectedChecks == nil || len(changedProtectedPaths(policy, changed)) == 0 {
		return nil
	}
	actors := map[string]bool{}
	for _, id := range decision.Event.Evidence {
		if review, ok := byID[id]; ok && protectionReviewQualifies(review, proposal, policy.ProtectedChecks) {
			actors[review.Event.Actor] = true
		}
	}
	if len(actors) < policy.ProtectedChecks.RequiredApprovals {
		return fmt.Errorf("decision %s lacks required protected check approval evidence", decision.ID)
	}
	return nil
}

func validateProtectedDecisionAt(gitDir, acceptedGitDir string, policy PolicyDocument, decision, proposal StoredEvent, byID map[string]StoredEvent) error {
	if policy.ProtectedChecks == nil {
		return nil
	}
	changed, err := changedPathsAt(gitDir, proposal.Event.Base, proposal.Event.Head)
	// Selected proposal fetches supply both commits in the isolated quarantine.
	// Otherwise an already accepted proposal may supply both. Require one
	// complete store; never treat a missing side as an empty or partial diff.
	if err != nil && acceptedGitDir != "" && acceptedGitDir != gitDir {
		changed, err = changedPathsAt(acceptedGitDir, proposal.Event.Base, proposal.Event.Head)
	}
	if err != nil {
		return fmt.Errorf("protected checks: %w", err)
	}
	return validateProtectedDecisionEvidence(policy, decision, proposal, byID, changed)
}

func guardTargetProtectedPolicy(target string, evaluation *ProposalEvaluation) error {
	// Do not introduce target-policy parsing into legacy repositories. Inspect
	// exact bytes for the optional section first; opted-in or malformed optional
	// configuration then goes through the normal strict parser.
	encoded, err := boundedPolicyGitOutput("", maxPolicySize, "show", target+":.hn/policy.json")
	if err != nil {
		if evaluation.Policy.ProtectedChecks != nil {
			return fmt.Errorf("target protection policy unavailable: %w", err)
		}
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		if evaluation.Policy.ProtectedChecks != nil {
			return fmt.Errorf("target protection policy: %w", err)
		}
		return nil
	}
	present := false
	for name, raw := range fields {
		if strings.EqualFold(name, "protectedChecks") && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			present = true
			break
		}
	}
	if !present {
		return nil
	}
	policy, digest, err := parsePolicyBytes(encoded)
	if err != nil {
		return fmt.Errorf("target protection policy: %w", err)
	}
	if policy.ProtectedChecks != nil {
		if digest != evaluation.PolicyDigest {
			return fmt.Errorf("target protected checks use a different policy; revise the proposal against the current target before merging")
		}
		changed, err := changedPathsAt("", evaluation.Proposal.Event.Base, target)
		if err != nil {
			return fmt.Errorf("target protected check baseline: %w", err)
		}
		if len(changedProtectedPaths(policy, changed)) > 0 {
			return fmt.Errorf("target protected check baseline changed; revise the proposal against the current target before merging")
		}
	}
	return nil
}
