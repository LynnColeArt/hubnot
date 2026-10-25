package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const automergeUsage = "usage: hn automerge PROPOSAL --rule NAME [--execute]"

// Process seams permit deterministic race/recovery tests. They are never loaded
// from repository configuration or exposed as policy authority.
var automergeProcessHook func(string) error

func runAutomergeHook(step string) error {
	if automergeProcessHook != nil {
		return automergeProcessHook(step)
	}
	return nil
}

type automergeInspection struct {
	Proposal          *StoredEvent
	Evaluation        *ProposalEvaluation
	Rule              *AutomergeRule
	Branch, Current   string
	Changed, Blockers []string
}

func cmdAutomerge(args []string) error {
	if len(args) < 1 {
		return usageError(automergeUsage)
	}
	if err := requireFullEventID(args[0]); err != nil {
		return err
	}
	flags := quietFlags("automerge")
	rule := flags.String("rule", "", "explicit automerge rule name")
	execute := flags.Bool("execute", false, "execute a locally eligible native merge")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *rule == "" {
		return usageError(automergeUsage)
	}
	inspection, err := inspectAutomerge(args[0], *rule)
	if err != nil {
		return err
	}
	printAutomergeInspection(inspection)
	if err := inspection.eligible(); err != nil {
		return err
	}
	if !*execute {
		return nil
	}
	identity, err := loadIdentity()
	if err != nil {
		return err
	}
	if !actorListed(identity.Actor, inspection.Rule.Actors) {
		return fmt.Errorf("identity %s is not an executing actor for automerge rule %q", identity.Actor, *rule)
	}
	guard := &proposalMergeGuard{ExpectedActor: identity.Actor}
	guard.Check = func(proposal *StoredEvent, evaluation *ProposalEvaluation, branch, current string) error {
		next, err := inspectAutomerge(proposal.ID, *rule)
		if err != nil {
			return err
		}
		if err := next.eligible(); err != nil {
			return err
		}
		if next.Branch != inspection.Rule.TargetBranch || next.Current != current || next.Proposal.Event.Base != proposal.Event.Base || next.Proposal.Event.Head != proposal.Event.Head || next.Evaluation.PolicyDigest != evaluation.PolicyDigest {
			return fmt.Errorf("automerge candidate, policy or target changed before merge")
		}
		if !actorListed(identity.Actor, next.Rule.Actors) {
			return fmt.Errorf("identity %s is no longer an executing actor for automerge rule %q", identity.Actor, *rule)
		}
		// The common merge path signs this exact latest acceptance snapshot.
		*evaluation = *next.Evaluation
		return nil
	}
	guard.Advance = func(proposal *StoredEvent, evaluation *ProposalEvaluation, branch, current, message string) (string, error) {
		return advanceAutomerge(proposal, evaluation, inspection.Rule.TargetBranch, current, message, guard.Check)
	}
	if err := runAutomergeHook("before-native-merge"); err != nil {
		return err
	}
	return mergeProposal([]string{inspection.Proposal.ID}, guard)
}

// Avoid promisor lazy fetch and recovery-ledger writes in preview without
// changing the behavior of any existing native command. Config is inspected
// before the first object reader, and its values are never echoed.
func checkAutomergeRepository() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "--no-replace-objects", "config", "--null", "--get-regexp", `^(extensions\.partialclone|remote\..*\.(promisor|partialclonefilter))$`)
	stdout, stderr := limitedPolicyBuffer{limit: maxPolicySize}, limitedPolicyBuffer{limit: 64 << 10}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if stdout.exceeded || stderr.exceeded {
		return fmt.Errorf("automerge repository configuration exceeds inspection bound")
	}
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 1 {
			return fmt.Errorf("cannot inspect automerge repository configuration")
		}
	} else {
		for _, entry := range bytes.Split(stdout.Bytes(), []byte{0}) {
			if len(entry) == 0 {
				continue
			}
			key, value, found := strings.Cut(string(entry), "\n")
			if !found {
				return fmt.Errorf("invalid automerge repository configuration result")
			}
			key = strings.ToLower(key)
			value = strings.ToLower(strings.TrimSpace(value))
			if strings.HasSuffix(key, ".promisor") && (value == "false" || value == "no" || value == "off" || value == "0") {
				continue
			}
			return fmt.Errorf("automerge does not support partial/promisor repositories; hydrate a full clone or use manual merge")
		}
	}
	raw, err := boundedPolicyGitOutput("", 128, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(raw)) != "false" {
		return fmt.Errorf("automerge requires a full repository; hydrate shallow history or use manual merge")
	}
	gitDirRaw, err := boundedPolicyGitOutput("", maxPolicyPathBytes, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return err
	}
	gitDir := strings.TrimSuffix(string(gitDirRaw), "\n")
	for _, name := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply", "sequencer", "BISECT_START"} {
		if _, err := os.Lstat(filepath.Join(gitDir, name)); err == nil {
			return fmt.Errorf("automerge refuses an unfinished Git operation (%s); finish or abort it first", name)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("cannot inspect Git operation state: %w", err)
		}
	}
	return nil
}

func inspectAutomerge(query, ruleName string) (*automergeInspection, error) {
	if err := checkAutomergeRepository(); err != nil {
		return nil, err
	}
	events, err := collectEvents()
	if err != nil {
		return nil, err
	}
	proposal, err := resolveFullEvent(events, query)
	if err != nil {
		return nil, err
	}
	if !isProposalKind(proposal.Event.Kind) {
		return nil, fmt.Errorf("%s is not a proposal", query)
	}
	evaluation, err := evaluateProposal(proposal, events)
	if err != nil {
		return nil, err
	}
	result := &automergeInspection{Proposal: proposal, Evaluation: evaluation}
	branchRaw, branchErr := boundedPolicyGitOutput("", 1024, "symbolic-ref", "--quiet", "HEAD")
	if branchErr != nil {
		result.Blockers = append(result.Blockers, "requires a checked-out target branch")
	} else {
		fullBranch := strings.TrimSpace(string(branchRaw))
		if !strings.HasPrefix(fullBranch, "refs/heads/") {
			result.Blockers = append(result.Blockers, "HEAD must name a local target branch")
		} else {
			result.Branch = strings.TrimPrefix(fullBranch, "refs/heads/")
		}
	}
	current, err := resolveCommit("HEAD")
	if err != nil {
		return nil, err
	}
	result.Current = current
	changed, err := changedPathsAt("", proposal.Event.Base, proposal.Event.Head)
	if err != nil {
		return nil, err
	}
	result.Changed = changed
	if evaluation.Policy.Automerge == nil {
		result.Blockers = append(result.Blockers, "automerge is disabled in the signed base policy")
	} else {
		for _, rule := range evaluation.Policy.Automerge.Rules {
			if rule.Name == ruleName {
				copy := rule
				result.Rule = &copy
				break
			}
		}
		if result.Rule == nil {
			result.Blockers = append(result.Blockers, fmt.Sprintf("automerge rule %q does not exist in the signed base policy", ruleName))
		}
	}
	if err := requireLineageTerminalEligibility("automerge", proposal.ID, evaluation.Lineage); err != nil {
		result.Blockers = append(result.Blockers, err.Error())
	}
	if evaluation.Merged {
		result.Blockers = append(result.Blockers, "proposal is already recorded as merged")
	}
	if !evaluation.CodeMatches {
		result.Blockers = append(result.Blockers, "proposal code ref does not match its signed head")
	}
	if !evaluation.Ready {
		result.Blockers = append(result.Blockers, "current approval, protected check or CI evidence is not ready")
	}
	if !evaluation.Accepted {
		result.Blockers = append(result.Blockers, "current required maintainer acceptances are missing")
	}
	if evaluation.Rejected {
		result.Blockers = append(result.Blockers, "proposal has a current maintainer rejection")
	}
	for _, review := range currentReviews(events, proposal.ID) {
		if review.Event.Verdict == "request-changes" && actorListed(review.Event.Actor, evaluation.Policy.Proposals.TrustedReviewers) {
			result.Blockers = append(result.Blockers, "proposal has a current trusted change request")
			break
		}
	}
	for _, pipeline := range evaluation.Pipelines {
		if len(pipeline.Failed) > 0 {
			result.Blockers = append(result.Blockers, fmt.Sprintf("pipeline %q has a current trusted failure", pipeline.Name))
		}
	}
	if current != proposal.Event.Base {
		result.Blockers = append(result.Blockers, "target no longer equals the signed base; revise the candidate (use hn merge for a missing merge-event repair)")
	}
	if len(changed) == 0 {
		result.Blockers = append(result.Blockers, "proposal has no changed paths")
	}
	for _, p := range changed {
		if protectedPathMatches(".hn/policy.json", p) {
			result.Blockers = append(result.Blockers, "policy amendments require manual merge")
			break
		}
	}
	if len(changedProtectedPaths(evaluation.Policy, changed)) > 0 {
		result.Blockers = append(result.Blockers, "protected check changes require manual merge")
	}
	if result.Rule != nil {
		if result.Branch != result.Rule.TargetBranch {
			result.Blockers = append(result.Blockers, fmt.Sprintf("checked-out branch does not match rule target %q", result.Rule.TargetBranch))
		}
		if !actorListed(proposal.Event.Actor, result.Rule.Authors) {
			result.Blockers = append(result.Blockers, "proposal author is not allowed by the rule")
		}
		for _, p := range changed {
			if !automergePathAllowed(result.Rule.Paths, p) {
				result.Blockers = append(result.Blockers, fmt.Sprintf("changed path %q is outside the rule allowlist", p))
			}
		}
	}
	ancestor, missing, err := exactCommitAncestor(proposal.Event.Base, proposal.Event.Head)
	if err != nil {
		return nil, err
	}
	if !ancestor || missing != "" {
		result.Blockers = append(result.Blockers, "signed head is not a locally verifiable descendant of the signed base")
	}
	_, _, targetDigest, err := loadPolicy(current)
	if err != nil {
		return nil, fmt.Errorf("inspect current target policy: %w", err)
	}
	if targetDigest != evaluation.PolicyDigest {
		result.Blockers = append(result.Blockers, "current target policy differs from the signed base policy")
	}
	status, err := boundedPolicyGitOutput("", maxChangedPathsBytes, "--no-optional-locks", "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	if len(status) > 0 {
		result.Blockers = append(result.Blockers, "worktree and index must be clean before automerge")
	}
	return result, nil
}

// Unlike protection matching, a descendant permission never allows modifying an
// ancestor; a directory permission allows only paths below that directory.
func automergePathAllowed(allowed []string, changed string) bool {
	for _, p := range allowed {
		if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(changed, p) {
				return true
			}
		} else if p == changed {
			return true
		}
	}
	return false
}

func (i *automergeInspection) eligible() error {
	if len(i.Blockers) > 0 {
		return fmt.Errorf("automerge blocked: %s", safeText(strings.Join(i.Blockers, "; ")))
	}
	return nil
}

func printAutomergeInspection(i *automergeInspection) {
	fmt.Printf("Proposal: %s\nBase: %s\nHead: %s\nPolicy: %s\nTarget: %s at %s\n", i.Proposal.ID, i.Proposal.Event.Base, i.Proposal.Event.Head, i.Evaluation.PolicyDigest, safeText(i.Branch), i.Current)
	if i.Rule != nil {
		fmt.Printf("Rule: %s\nPermitted executing actors:\n", safeText(i.Rule.Name))
		actors := append([]string(nil), i.Rule.Actors...)
		sort.Strings(actors)
		for _, actor := range actors {
			fmt.Printf("  %s\n", actor)
		}
	}
	fmt.Println("Changed paths:")
	for _, p := range i.Changed {
		fmt.Printf("  %q\n", p)
	}
	if len(i.Blockers) == 0 {
		fmt.Println("Eligible: explicit --execute is required to merge locally.")
	} else {
		fmt.Println("Blocked:")
		for _, b := range i.Blockers {
			fmt.Printf("  %s\n", safeText(b))
		}
	}
}

// A prepared expected-old transaction locks the actual target and HEAD before
// checkout. The passed old OID is the signed base, never a new self-observation.
type automergeRefTransaction struct {
	command    *exec.Cmd
	input      io.WriteCloser
	pipe       io.ReadCloser
	output     *bufio.Scanner
	diagnostic *limitedPolicyBuffer
	cancel     context.CancelFunc
	done       bool
}

// Only acknowledgements and process exit are timed. A successfully prepared
// lease must not expire while its caller is checking evidence or checkout state.
var automergeRefAckTimeout = 30 * time.Second

func prepareAutomergeRef(ref, next, expected string) (*automergeRefTransaction, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "git", "--no-replace-objects", "update-ref", "--stdin", "-m", "hn: native automerge")
	cmd.WaitDelay = time.Second
	stderr := &limitedPolicyBuffer{limit: 64 << 10}
	cmd.Stderr = stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		in.Close()
		return nil, err
	}
	scan := bufio.NewScanner(out)
	scan.Buffer(make([]byte, 256), 1024)
	txn := &automergeRefTransaction{command: cmd, input: in, pipe: out, output: scan, diagnostic: stderr, cancel: cancel}
	if err := cmd.Start(); err != nil {
		cancel()
		in.Close()
		out.Close()
		return nil, err
	}
	if err := txn.exchange("start", "start: ok"); err != nil {
		txn.close()
		return nil, err
	}
	if _, err := fmt.Fprintf(in, "update %s %s %s\n", ref, next, expected); err != nil {
		txn.close()
		return nil, err
	}
	if err := txn.exchange("prepare", "prepare: ok"); err != nil {
		txn.close()
		return nil, fmt.Errorf("automerge expected-target lease refused: %s", safeText(txn.diagnostic.String()))
	}
	return txn, nil
}

func (t *automergeRefTransaction) exchange(line, want string) error {
	if t.done {
		return fmt.Errorf("Git ref transaction is closed")
	}
	if _, err := io.WriteString(t.input, line+"\n"); err != nil {
		return err
	}
	ack := make(chan bool, 1)
	go func() { ack <- t.output.Scan() && t.output.Text() == want }()
	timer := time.NewTimer(automergeRefAckTimeout)
	defer timer.Stop()
	select {
	case ok := <-ack:
		if !ok {
			return fmt.Errorf("Git ref transaction did not acknowledge %s", line)
		}
		return nil
	case <-timer.C:
		t.cancel()
		t.input.Close()
		t.pipe.Close()
		_ = t.wait()
		<-ack
		return fmt.Errorf("Git ref transaction acknowledgement timed out for %s", line)
	}
}

func (t *automergeRefTransaction) wait() error {
	if t.done {
		return nil
	}
	result := make(chan error, 1)
	go func() { result <- t.command.Wait() }()
	timer := time.NewTimer(automergeRefAckTimeout)
	defer timer.Stop()
	var err error
	select {
	case err = <-result:
	case <-timer.C:
		t.cancel()
		t.input.Close()
		t.pipe.Close()
		err = <-result
		if err == nil {
			err = fmt.Errorf("Git ref transaction process exit timed out")
		}
	}
	t.done = true
	t.cancel()
	return err
}

func (t *automergeRefTransaction) commit() error {
	err := t.exchange("commit", "commit: ok")
	t.input.Close()
	if err != nil {
		t.cancel()
		t.pipe.Close()
	}
	waitErr := t.wait()
	if err != nil || waitErr != nil {
		return fmt.Errorf("Git ref transaction commit was not confirmed")
	}
	return nil
}

func (t *automergeRefTransaction) close() {
	if t.done {
		return
	}
	err := t.exchange("abort", "abort: ok")
	t.input.Close()
	if err != nil {
		t.cancel()
		t.pipe.Close()
	}
	_ = t.wait()
}

func advanceAutomerge(proposal *StoredEvent, evaluation *ProposalEvaluation, branch, current, message string, check func(*StoredEvent, *ProposalEvaluation, string, string) error) (string, error) {
	if current != proposal.Event.Base {
		return "", fmt.Errorf("automerge target must equal the signed base")
	}
	tree, err := boundedPolicyGitOutput("", 128, "rev-parse", "--verify", proposal.Event.Head+"^{tree}")
	if err != nil {
		return "", err
	}
	treeID := strings.TrimSpace(string(tree))
	if !validFullGitObjectID(treeID) {
		return "", fmt.Errorf("invalid automerge tree identity")
	}
	commitRaw, err := automergeGitInput([]byte(message+"\n"), "commit-tree", treeID, "-p", current, "-p", proposal.Event.Head)
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(string(commitRaw))
	if !validFullGitObjectID(commit) {
		return "", fmt.Errorf("invalid automerge commit identity")
	}
	if err := runAutomergeHook("before-prepare"); err != nil {
		return "", err
	}
	txn, err := prepareAutomergeRef("refs/heads/"+branch, commit, proposal.Event.Base)
	if err != nil {
		return "", err
	}
	defer txn.close()
	if err := runAutomergeHook("lease-prepared"); err != nil {
		return "", err
	}
	if err := check(proposal, evaluation, branch, current); err != nil {
		return "", err
	}
	if err := automergeIgnoredCollisions(proposal.Event.Base, proposal.Event.Head); err != nil {
		return "", err
	}
	if _, err := automergeGitInput(nil, "read-tree", "-m", "-u", current, commit); err != nil {
		return "", err
	}
	commitErr := txn.commit()
	// A lost acknowledgement is not proof that a ref update failed. Observe its
	// actual value before deciding whether a safe checkout rollback is needed.
	actualRaw, readErr := boundedPolicyGitOutput("", 128, "rev-parse", "--verify", "refs/heads/"+branch)
	if readErr != nil {
		return "", fmt.Errorf("automerge ref outcome is uncertain; inspect branch %q and run 'hn merge %s' if code merged", branch, proposal.ID)
	}
	actual := strings.TrimSpace(string(actualRaw))
	if actual != commit {
		if actual == current {
			if _, restoreErr := automergeGitInput(nil, "read-tree", "-m", "-u", commit, current); restoreErr != nil {
				return "", fmt.Errorf("automerge did not advance target; checkout could not be safely restored, preserve local edits and inspect index: %w", restoreErr)
			}
		}
		return "", fmt.Errorf("automerge target was not advanced to the prepared merge; current target %s (commit confirmation: %v)", actual, commitErr)
	}
	if err := runAutomergeHook("merge-committed"); err != nil {
		return "", fmt.Errorf("code merged at %s but event recording interrupted; run 'hn merge %s': %w", commit, proposal.ID, err)
	}
	return commit, nil
}

func automergeGitInput(input []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-replace-objects"}, args...)...)
	cmd.Stdin = bytes.NewReader(input)
	stdout, stderr := limitedPolicyBuffer{limit: maxChangedPathsBytes}, limitedPolicyBuffer{limit: 64 << 10}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stdout.exceeded || stderr.exceeded {
			return nil, fmt.Errorf("automerge Git output exceeds bound")
		}
		return nil, fmt.Errorf("automerge Git operation: %s", safeText(commandFailure(err, stderr.String())))
	}
	return stdout.Bytes(), nil
}

func automergeIgnoredCollisions(base, head string) error {
	changed, err := changedPathsAt("", base, head)
	if err != nil {
		return err
	}
	raw, err := boundedPolicyGitOutput("", maxChangedPathsBytes, "ls-files", "--full-name", "--others", "--ignored", "--exclude-standard", "-z", "--", ":(top)**")
	if err != nil {
		return err
	}
	if len(raw) > 0 && raw[len(raw)-1] != 0 {
		return fmt.Errorf("invalid ignored-path inspection")
	}
	for _, entry := range bytes.Split(raw, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		p := strings.TrimSuffix(string(entry), "/")
		if automergeIgnoredPathCollision(changed, p) {
			return fmt.Errorf("automerge would overwrite ignored user path %q", p)
		}
	}
	return nil
}

// changed is sorted by changedPathsAt. Search exact/descendant names, then each
// real ancestor component; a lexical predecessor alone can hide an ancestor.
func automergeIgnoredPathCollision(changed []string, p string) bool {
	i := sort.SearchStrings(changed, p)
	if i < len(changed) && changed[i] == p {
		return true
	}
	prefix := p + "/"
	i = sort.SearchStrings(changed, prefix)
	if i < len(changed) && strings.HasPrefix(changed[i], prefix) {
		return true
	}
	for offset := 0; offset < len(p); {
		relative := strings.IndexByte(p[offset:], '/')
		if relative < 0 {
			break
		}
		end := offset + relative
		ancestor := p[:end]
		i = sort.SearchStrings(changed, ancestor)
		if i < len(changed) && changed[i] == ancestor {
			return true
		}
		offset = end + 1
	}
	return false
}
