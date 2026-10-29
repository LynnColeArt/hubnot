package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	c "hubnot/internal/contribution"
	"hubnot/internal/migration"
	"hubnot/internal/safepath"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// A contribution anchor is an explicit local trust selection. Neither imported
// policy nor an envelope installs it. Adopted is immutable until explicit CAS
// readoption; every policy transition since it is independently verified.
type contributionAuthority struct {
	GitDir       string                    `json:"gitDir"`
	Ref          string                    `json:"ref"`
	Adopted      string                    `json:"adopted"`
	ReadOnly     bool                      `json:"readOnly,omitempty"`
	Digest       string                    `json:"-"`
	Commit       string                    `json:"-"`
	Policy       PolicyDocument            `json:"-"`
	PolicyDigest string                    `json:"-"`
	Checkpoints  map[string]c.AuthorityRef `json:"-"`
}

func contributionDirectory(create bool) (string, error) {
	dir, e := requireGitRepository()
	if e != nil {
		return "", e
	}
	for _, p := range []string{"hn", "contribution"} {
		dir = filepath.Join(dir, p)
		info, e := os.Lstat(dir)
		if errors.Is(e, os.ErrNotExist) && create {
			e = os.Mkdir(dir, 0700)
			if e != nil {
				return "", e
			}
			info, e = os.Lstat(dir)
		}
		if e != nil {
			return "", e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
			return "", fmt.Errorf("unsafe contribution directory")
		}
	}
	return dir, nil
}
func loadContributionAuthority() (*contributionAuthority, error) {
	if _, err := requireGitRepository(); err != nil {
		return nil, nil
	}
	dir, e := contributionDirectory(false)
	if errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	raw, e := readPrivateFileBounded(filepath.Join(dir, "authority.json"), 4096)
	if errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var a contributionAuthority
	if e = contributionStrict(raw, &a); e != nil {
		return nil, e
	}
	a.Digest = eventID(raw)
	private, e := contributionPrivateGitDirectory()
	if e != nil {
		return nil, e
	}
	if a.GitDir != private {
		return nil, fmt.Errorf("contribution authority belongs to another worktree context; explicit local adoption required")
	}
	a.Commit, a.Policy, a.PolicyDigest, e = resolveContributionBranch(a.Ref)
	if e != nil {
		return nil, e
	}
	if !validGitOID(a.Adopted) {
		return nil, fmt.Errorf("invalid adopted contribution checkpoint")
	}
	ctx, cancel, e := contributionContext(context.Background(), contributionAuthorityLimits(&a))
	if e != nil {
		return nil, e
	}
	defer cancel()
	gitdir, e := requireGitRepository()
	if e != nil {
		return nil, e
	}
	if _, e = projectGit(ctx, gitdir, nil, 256, "merge-base", "--is-ancestor", a.Adopted, a.Commit); e != nil {
		return nil, fmt.Errorf("contribution authority replaced or lookup exceeded bounds; explicit CAS readoption required: %w", e)
	}
	if e = verifyContributionPolicyHistory(&a); e != nil {
		return nil, e
	}
	return &a, nil
}
func (a *contributionAuthority) current() c.Authority {
	return c.Authority{Ref: c.AuthorityRef{Commit: a.Commit, Digest: a.PolicyDigest}, Policy: a.Policy.Contributions, Maintainers: sortedStrings(a.Policy.Maintainers)}
}
func contributionAuthorityLimits(a *contributionAuthority) c.Limits {
	if a.Policy.Contributions != nil {
		return a.Policy.Contributions.Limits
	}
	return c.DefaultLimits()
}
func contributionAuthorityEvents(a *contributionAuthority) ([]StoredEvent, error) {
	l := contributionAuthorityLimits(a)
	returnEvents, _, e := collectIssueEventsWithLimits(issueReadLimits{events: int(min(l.Retained.Events, int64(c.MaxGraphRecords))), refs: int(min(l.Retained.Objects, int64(c.MaxGraphRecords))), object: min(l.DecodedBytes, int64(8<<20)), total: min(l.Retained.Bytes, int64(256<<20))})
	return returnEvents, e
}
func contributionPolicyAt(ctx context.Context, dir, commit string) (PolicyDocument, string, error) {
	if !validGitOID(commit) {
		return PolicyDocument{}, "", c.ErrAuthority
	}
	if e := replicationPendingError(dir, commit); e != nil {
		return PolicyDocument{}, "", e
	}
	raw, e := projectGit(ctx, dir, nil, maxPolicySize, "show", commit+":.hn/policy.json")
	if e != nil {
		return PolicyDocument{}, "", e
	}
	p, d, e := parsePolicyBytes(raw)
	return p, d, e
}
func contributionRatifiedTransition(ctx context.Context, dir, commit, priorDigest, digest string, events []StoredEvent) error {
	raw, e := projectGit(ctx, dir, nil, 256, "show", "-s", "--format=%P", commit)
	if e != nil {
		return e
	}
	parents := strings.Fields(string(raw))
	if len(parents) != 2 {
		return fmt.Errorf("policy change must be an exact governed native merge")
	}
	byID := map[string]StoredEvent{}
	for _, v := range events {
		byID[v.ID] = v
	}
	_, parentDigest, e := contributionPolicyAt(ctx, dir, parents[0])
	if e != nil || parentDigest != priorDigest {
		return fmt.Errorf("policy transition base does not match target policy")
	}
	for _, m := range events {
		if e = ctx.Err(); e != nil {
			return e
		}
		if m.Event.Kind != "proposal.merged" || m.Event.Commit != commit || m.Event.Policy != priorDigest {
			continue
		}
		p, ok := byID[m.Event.Subject]
		if !ok || !isProposalKind(p.Event.Kind) || p.Event.Head != parents[1] {
			continue
		}
		_, bd, e := contributionPolicyAt(ctx, dir, p.Event.Base)
		if e != nil || bd != priorDigest {
			continue
		}
		_, hd, e := contributionPolicyAt(ctx, dir, p.Event.Head)
		if e != nil || hd != digest {
			continue
		}
		if e = validateMergeEvent(m, p, byID); e != nil {
			continue
		}
		valid := true
		for _, id := range m.Event.Evidence {
			decision, ok := byID[id]
			if !ok || validateDecisionEvent(decision, p, byID) != nil {
				valid = false
				break
			}
		}
		if valid {
			return nil
		}
	}
	return fmt.Errorf("contribution policy transition %s lacks exact base-policy-governed native merge", commit)
}
func verifyContributionPolicyHistory(a *contributionAuthority) error {
	l := contributionAuthorityLimits(a)
	ctx, cancel, e := contributionContext(context.Background(), l)
	if e != nil {
		return e
	}
	defer cancel()
	dir, e := requireGitRepository()
	if e != nil {
		return e
	}
	bound := min(l.Depth, int64(c.MaxGraphRecords))
	raw, e := projectGit(ctx, dir, nil, (bound+1)*128, "rev-list", "--first-parent", "--max-count="+strconv.FormatInt(bound+1, 10), a.Commit, "^"+a.Adopted)
	if e != nil {
		return e
	}
	commits := strings.Fields(string(raw))
	if int64(len(commits)) > bound {
		return fmt.Errorf("authority history exceeds project depth/graph bound")
	}
	_, prior, e := contributionPolicyAt(ctx, dir, a.Adopted)
	if e != nil {
		return e
	}
	var events []StoredEvent
	for i := len(commits) - 1; i >= 0; i-- {
		commit := commits[i]
		_, digest, e := contributionPolicyAt(ctx, dir, commit)
		if e != nil {
			return e
		}
		if digest == prior {
			continue
		}
		if events == nil {
			events, e = contributionAuthorityEvents(a)
			if e != nil {
				return e
			}
		}
		if e = contributionRatifiedTransition(ctx, dir, commit, prior, digest, events); e != nil {
			return e
		}
		prior = digest
	}
	return verifyContributionCheckpoints(ctx, dir, a, events)
}

// Checkpoints require their own native ratification even on first read-only
// adoption. Selecting a branch with copied JSON cannot establish old records.
func verifyContributionCheckpoints(ctx context.Context, dir string, a *contributionAuthority, events []StoredEvent) error {
	a.Checkpoints = map[string]c.AuthorityRef{}
	if len(a.Policy.ContributionCheckpoints) == 0 {
		return nil
	}
	l := contributionAuthorityLimits(a)
	bound := min(l.Depth, int64(c.MaxGraphRecords))
	raw, e := projectGit(ctx, dir, nil, (bound+1)*128, "rev-list", "--first-parent", "--max-count="+strconv.FormatInt(bound+1, 10), a.Commit)
	if e != nil {
		return e
	}
	commits := strings.Fields(string(raw))
	if int64(len(commits)) > bound {
		return fmt.Errorf("checkpoint history exceeds project depth/graph bound")
	}
	if events == nil {
		events, e = contributionAuthorityEvents(a)
		if e != nil {
			return e
		}
	}
	pending := map[string]c.AuthorityRef{}
	for _, cp := range a.Policy.ContributionCheckpoints {
		pending[cp.ID] = cp.Authority
	}
	for i, commit := range commits {
		policy, digest, e := contributionPolicyAt(ctx, dir, commit)
		if e != nil {
			return e
		}
		present := map[string]c.AuthorityRef{}
		for _, cp := range policy.ContributionCheckpoints {
			present[cp.ID] = cp.Authority
		}
		var parentPolicy PolicyDocument
		prior := ""
		if i+1 < len(commits) {
			parentPolicy, prior, e = contributionPolicyAt(ctx, dir, commits[i+1])
			if e != nil {
				return e
			}
		}
		old := map[string]c.AuthorityRef{}
		for _, cp := range parentPolicy.ContributionCheckpoints {
			old[cp.ID] = cp.Authority
		}
		introduced := false
		for id, ref := range pending {
			if present[id] != ref {
				return fmt.Errorf("checkpoint history is discontinuous")
			}
			if old[id] != ref {
				introduced = true
			}
		}
		if introduced {
			if prior == "" {
				return fmt.Errorf("checkpoint lacks native governance predecessor")
			}
			if e = contributionRatifiedTransition(ctx, dir, commit, prior, digest, events); e != nil {
				return e
			}
			for id, ref := range pending {
				if old[id] != ref {
					a.Checkpoints[id] = ref
					delete(pending, id)
				}
			}
		}
		if len(pending) == 0 {
			return nil
		}
	}
	return fmt.Errorf("checkpoint ratification unavailable within project bounds")
}

type contributionAuthoritySource struct{ a *contributionAuthority }

func (s contributionAuthoritySource) Historical(ref c.AuthorityRef) (c.Authority, error) {
	if !validGitOID(ref.Commit) || !validEventID(ref.Digest) {
		return c.Authority{}, c.ErrAuthority
	}
	l := contributionAuthorityLimits(s.a)
	ctx, cancel, e := contributionContext(context.Background(), l)
	if e != nil {
		return c.Authority{}, e
	}
	defer cancel()
	dir, e := requireGitRepository()
	if e != nil {
		return c.Authority{}, e
	}
	if _, e := projectGit(ctx, dir, nil, 256, "merge-base", "--is-ancestor", ref.Commit, s.a.Commit); e != nil {
		return c.Authority{}, c.ErrAuthority
	}
	p, digest, e := contributionPolicyAt(ctx, dir, ref.Commit)
	if e != nil || digest != ref.Digest || p.Contributions == nil || s.a.Policy.Contributions == nil || p.Contributions.Project != s.a.Policy.Contributions.Project {
		return c.Authority{}, c.ErrAuthority
	}
	return c.Authority{Ref: ref, Policy: p.Contributions, Maintainers: sortedStrings(p.Maintainers)}, nil
}
func (s contributionAuthoritySource) Established(id string, ref c.AuthorityRef) (bool, error) {
	evaluated, ok := s.a.Checkpoints[id]
	return ok && evaluated == ref, nil
}
func contributionContentEvents(events []StoredEvent) []StoredEvent {
	out := []StoredEvent{}
	for _, s := range events {
		if isIssueKind(s.Event.Kind) || isResourceKind(s.Event.Kind) || isProposalKind(s.Event.Kind) {
			out = append(out, s)
		}
	}
	return out
}
func contributionRecords(a *contributionAuthority) ([]c.Verified, error) {
	dir, e := contributionGitDirectory()
	if e != nil {
		return nil, e
	}
	ctx, cancel, e := contributionContext(context.Background(), a.Policy.Contributions.Limits)
	if e != nil {
		return nil, e
	}
	defer cancel()
	material, e := projectGit(ctx, dir, nil, c.MaxGraphRecords*256, "for-each-ref", "--format=%(refname)", "refs/hn/contributions/"+a.Policy.Contributions.Project[7:]+"/")
	raw := string(material)
	if e != nil {
		return nil, e
	}
	refs := strings.Fields(raw)
	if len(refs) > c.MaxGraphRecords {
		return nil, fmt.Errorf("contribution ref bound")
	}
	out := []c.Verified{}
	for _, ref := range refs {
		actor := ref[strings.LastIndex(ref, "/")+1:]
		records, _, e := ReadContributionManifest(ctx, dir, a.Policy.Contributions.Project, actor, a.Policy.Contributions.Limits)
		if e != nil {
			return nil, e
		}
		out = append(out, records...)
		if len(out) > c.MaxGraphRecords {
			return nil, fmt.Errorf("contribution graph bound")
		}
	}
	return out, nil
}
func contributionView(events []StoredEvent) (*c.View, *contributionAuthority, error) {
	a, e := loadContributionAuthority()
	if e != nil {
		return nil, nil, e
	}
	if a == nil || a.Policy.Contributions == nil {
		return nil, a, nil
	}
	if e := contributionRequireBaseline(a); e != nil {
		return nil, a, e
	}
	native := newContributionNativeGraph(events, a.Policy.Contributions.Limits)
	records, e := contributionRecords(a)
	if e != nil {
		return nil, a, e
	}
	v, e := c.Reduce(a.current(), records, native, contributionAuthoritySource{a})
	if e == nil {
		e = contributionRequireRetention(a, &v)
	}
	return &v, a, e
}

// Admission is applied before catalog/lineage construction. The original raw
// event index remains separate for signature, chain and exact-reference checks.
func contributionAdmitted(events []StoredEvent, v *c.View) []StoredEvent {
	if v == nil {
		return events
	}
	out := []StoredEvent{}
	for _, s := range events {
		if isIssueKind(s.Event.Kind) || isResourceKind(s.Event.Kind) || isProposalKind(s.Event.Kind) {
			if !v.Admitted[s.ID] {
				continue
			}
		}
		out = append(out, s)
	}
	return out
}
func buildGovernedIssueCatalog(events []StoredEvent) (*IssueCatalog, error) {
	v, _, e := contributionView(events)
	if e != nil {
		return nil, e
	}
	cat, e := BuildIssueCatalog(contributionAdmitted(events, v))
	if e != nil || v == nil {
		return cat, e
	}
	ids := []string{}
	for _, id := range cat.IDs {
		view := cat.Issues[id]
		visible := len(view.Heads) > 0
		for _, h := range view.Heads {
			if !v.Visible[h.ID] {
				visible = false
			}
		}
		if !visible {
			delete(cat.Issues, id)
			continue
		}
		ids = append(ids, id)
		history := []StoredEvent{}
		for _, s := range view.History {
			if v.Visible[s.ID] {
				history = append(history, s)
			}
		}
		view.History = history
		comments := []StoredEvent{}
		for _, s := range view.Comments {
			if v.Visible[s.ID] {
				comments = append(comments, s)
			}
		}
		view.Comments = comments
	}
	cat.IDs = ids
	edges := []IssueEdge{}
	for _, edge := range cat.Graph {
		if cat.Issues[edge.Source] != nil && cat.Issues[edge.Target] != nil && v.Visible[edge.Revision] {
			edges = append(edges, edge)
		}
	}
	cat.Graph = edges
	cat.Cycles = issueGraphCycles(edges)
	return cat, nil
}
func buildGovernedResourceCatalog(events []StoredEvent) (*ResourceCatalog, error) {
	v, _, e := contributionView(events)
	if e != nil {
		return nil, e
	}
	cat, e := BuildResourceCatalog(contributionAdmitted(events, v))
	if e != nil || v == nil {
		return cat, e
	}
	ids := []string{}
	for _, id := range cat.IDs {
		view := cat.Resources[id]
		visible := len(view.Heads) > 0
		for _, h := range view.Heads {
			if !v.Visible[h.ID] {
				visible = false
			}
		}
		if !visible {
			delete(cat.Resources, id)
			continue
		}
		ids = append(ids, id)
		history := []StoredEvent{}
		for _, s := range view.History {
			if v.Visible[s.ID] {
				history = append(history, s)
			}
		}
		view.History = history
	}
	cat.IDs = ids
	return cat, nil
}
func contributionOperativeEvidence(events []StoredEvent, v *c.View) []StoredEvent {
	out := contributionAdmitted(events, v)
	if v == nil {
		return out
	}
	filtered := []StoredEvent{}
	for _, s := range out {
		if (s.Event.Kind == "proposal.merged" || s.Event.Kind == "proposal.decision" || s.Event.Kind == "review.submit" || s.Event.Kind == "run.request") && !v.Admitted[s.Event.Subject] {
			continue
		}
		filtered = append(filtered, s)
	}
	return filtered
}
func collectGovernedEvents() ([]StoredEvent, error) {
	events, e := collectEvents()
	if e != nil {
		return nil, e
	}
	v, _, e := contributionView(events)
	if e != nil {
		return nil, e
	}
	return contributionOperativeEvidence(events, v), nil
}
func contributionVisibleEvents(events []StoredEvent) ([]StoredEvent, error) {
	v, _, e := contributionView(events)
	if e != nil {
		return nil, e
	}
	if v == nil {
		return events, nil
	}
	out := []StoredEvent{}
	for _, s := range events {
		if (isIssueKind(s.Event.Kind) || isResourceKind(s.Event.Kind) || isProposalKind(s.Event.Kind)) && !v.Visible[s.ID] {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}
func contributionReadSnapshot(raw string) (string, error) {
	a, e := loadContributionAuthority()
	if e != nil {
		return "", e
	}
	if a == nil || a.Policy.Contributions == nil {
		return raw, nil
	}
	refs, e := gitText("for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/contributions/"+a.Policy.Contributions.Project[7:]+"/")
	if e != nil {
		return "", e
	}
	return c.Digest([]byte(raw + "\n" + a.Commit + "\n" + a.PolicyDigest + "\n" + refs)), nil
}
func contributionViewDocument(v *c.View) any {
	if v == nil {
		return nil
	}
	heads := []any{}
	for target, ids := range v.DecisionHeads {
		heads = append(heads, map[string]any{"target": target, "heads": ids})
	}
	return map[string]any{"project": v.Project, "authority": v.Authority, "roots": v.Roots, "admitted": v.Admitted, "visible": v.Visible, "heads": v.Heads, "pending": v.Pending, "conflicts": v.Conflicts, "revoked": v.Revoked, "invalid": v.Invalid, "decisionHeads": heads, "blockedActors": v.BlockedActors}
}
func contributionPrint(v any) error {
	if view, ok := v.(*c.View); ok {
		v = contributionViewDocument(view)
	}
	raw, e := json.Marshal(v)
	if e == nil {
		fmt.Println(string(raw))
	}
	return e
}
func contributionNativeRefs() ([]string, error) {
	raw, e := gitText("for-each-ref", "--format=%(refname)", "refs/hn/actors", "refs/hn/remotes", "refs/hn/proposals")
	if e != nil {
		return nil, e
	}
	out := []string{}
	for _, ref := range strings.Fields(raw) {
		if _, _, ok := parseAcceptedActorRef(ref); ok || strings.HasPrefix(ref, "refs/hn/actors/") {
			out = append(out, ref)
			continue
		}
		if _, _, ok := parseAcceptedProposalRef(ref); ok || strings.HasPrefix(ref, "refs/hn/proposals/") {
			out = append(out, ref)
		}
	}
	sort.Strings(out)
	unique := []string{}
	seen := map[string]bool{}
	for _, ref := range out {
		oid, exists, e := refValue(ref)
		if e != nil {
			return nil, e
		}
		if !exists {
			continue
		}
		kind := "actor"
		if strings.Contains(ref, "/proposals/") {
			kind = "proposal"
		}
		key := kind + ":" + oid
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, ref)
	}
	return unique, nil
}

// Select the smallest already-existing full actor observations containing exact
// required facts. This never rewrites or cuts an actor chain. An unsupported
// predecessor in the necessary observation is refused by the exchange verifier.
func contributionRefsForRecords(records []c.Verified, baseline []string) ([]string, error) {
	events, e := collectEvents()
	if e != nil {
		return nil, e
	}
	return contributionSelectNativeRefs(records, baseline, events)
}
func contributionSelectNativeRefs(records []c.Verified, baseline []string, events []StoredEvent) ([]string, error) {
	var e error
	byID := map[string]StoredEvent{}
	for _, s := range events {
		byID[s.ID] = s
	}
	needed := map[string]bool{}
	var walk func(string) error
	walk = func(id string) error {
		if needed[id] {
			return nil
		}
		if len(needed) >= c.MaxGraphRecords {
			return fmt.Errorf("native selection graph limit")
		}
		s, ok := byID[id]
		if !ok {
			return c.ErrMissingDependency
		}
		needed[id] = true
		for _, dep := range replicationEventReferences(s.Event) {
			if e := walk(dep); e != nil {
				return e
			}
		}
		if s.Event.Previous != "" {
			return walk(s.Event.Previous)
		}
		return nil
	}
	for _, id := range baseline {
		if e = walk(id); e != nil {
			return nil, e
		}
	}
	for _, r := range records {
		if s := r.Record().Submission; s != nil {
			if e = walk(s.Content.Revision); e != nil {
				return nil, e
			}
		}
		if d := r.Record().Decision; d != nil && d.Target.Kind == c.RevisionTarget {
			if e = walk(d.Target.ID); e != nil {
				return nil, e
			}
		}
	}
	refs, e := contributionNativeRefs()
	if e != nil {
		return nil, e
	}
	ctx, cancel, e := contributionContext(context.Background(), c.DefaultLimits())
	if e != nil {
		return nil, e
	}
	defer cancel()
	dir, e := contributionGitDirectory()
	if e != nil {
		return nil, e
	}
	selected := map[string]bool{}
	oidByRef := map[string]string{}
	for _, ref := range refs {
		oid, ok, e := refValue(ref)
		if e != nil {
			return nil, e
		}
		if ok {
			oidByRef[ref] = oid
		}
	}
	for id := range needed {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		s := byID[id]
		best := ""
		bestCount := int64(c.MaxGraphRecords + 1)
		for _, ref := range refs {
			owner := ""
			if strings.HasPrefix(ref, "refs/hn/actors/") {
				owner = strings.TrimPrefix(ref, "refs/hn/actors/")
			} else if _, actor, ok := parseAcceptedActorRef(ref); ok {
				owner = actor
			}
			if owner != s.Event.Actor {
				continue
			}
			head := oidByRef[ref]
			if _, e = projectGit(ctx, dir, nil, 256, "merge-base", "--is-ancestor", s.Commit, head); e != nil {
				continue
			}
			raw, e := projectGit(ctx, dir, nil, 256, "rev-list", "--count", head)
			if e != nil {
				return nil, e
			}
			count, e := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
			if e != nil {
				return nil, e
			}
			if count < bestCount {
				best = ref
				bestCount = count
			}
		}
		if best == "" {
			return nil, fmt.Errorf("no exact native actor observation supplies %s", id)
		}
		selected[best] = true
		if isProposalKind(s.Event.Kind) {
			found := false
			for _, ref := range refs {
				candidate := ""
				if strings.HasPrefix(ref, "refs/hn/proposals/") {
					candidate = "sha256:" + strings.TrimPrefix(ref, "refs/hn/proposals/")
				} else if _, proposal, ok := parseAcceptedProposalRef(ref); ok {
					candidate = proposal
				}
				if candidate == id && oidByRef[ref] == s.Event.Head {
					selected[ref] = true
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("proposal lacks exact native code selection")
			}
		}
	}
	// Earlier observations of one actor are redundant when a selected later full
	// chain already contains them. Keep disconnected observations independently.
	for earlier := range selected {
		if !strings.Contains(earlier, "/actors/") {
			continue
		}
		for later := range selected {
			if earlier == later || !strings.Contains(later, "/actors/") {
				continue
			}
			if oidByRef[earlier] == oidByRef[later] {
				if later < earlier {
					delete(selected, earlier)
					break
				}
				continue
			}
			if _, e = projectGit(ctx, dir, nil, 256, "merge-base", "--is-ancestor", oidByRef[earlier], oidByRef[later]); e == nil {
				delete(selected, earlier)
				break
			}
		}
	}
	out := []string{}
	for ref := range selected {
		out = append(out, ref)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("no contributed native content selected; supply explicit --ref")
	}
	return out, nil
}

func contributionAdmission(events []StoredEvent, id string) (string, error) {
	v, _, e := contributionView(events)
	if e != nil {
		return "", e
	}
	if v == nil {
		return "", nil
	}
	if !v.Admitted[id] {
		return "pending", nil
	}
	if !v.Visible[id] {
		return "hidden", nil
	}
	return "admitted", nil
}

type contributionBaselineReceipt struct {
	Baseline   string `json:"baseline"`
	Artifact   string `json:"artifact"`
	Intent     string `json:"intent"`
	Accounting string `json:"accounting"`
}

func contributionBaselineID(a *contributionAuthority) string {
	raw, _ := json.Marshal(struct {
		Project  string
		Baseline []string
	}{a.Policy.Contributions.Project, a.Policy.Contributions.Baseline})
	return c.Digest(raw)
}
func contributionRequireBaseline(a *contributionAuthority) error {
	if a == nil || a.Policy.Contributions == nil || len(a.Policy.Contributions.Baseline) == 0 {
		return nil
	}
	x, e := contributionExistingExchange(a)
	if e != nil {
		return e
	}
	id := contributionBaselineID(a)
	raw, e := readPrivateFileBounded(filepath.Join(x.root, "baseline-"+id[7:]+".json"), 4096)
	if e != nil {
		return fmt.Errorf("current exact baseline lacks durable local reservation; run hn contribution baseline --expect-policy %s: %w", a.PolicyDigest, e)
	}
	var receipt contributionBaselineReceipt
	if e = contributionStrict(raw, &receipt); e != nil {
		return e
	}
	if receipt.Baseline != id || !validEventID(receipt.Artifact) || !validEventID(receipt.Accounting) {
		return fmt.Errorf("invalid baseline reservation binding")
	}
	journal, e := x.load()
	ledger, attempts := journal.Ledger, journal.Attempts
	if e != nil {
		return e
	}
	for i, reservation := range ledger.Reservations {
		if reservation.Intent == receipt.Intent && reservation.Artifact == receipt.Artifact && attempts[i].Accounting == receipt.Accounting && attempts[i].Failure == "authenticated local baseline retained; no public artifact" {
			return nil
		}
	}
	return fmt.Errorf("baseline reservation missing from retained project ledger")
}

// This private descriptor can only be produced by the authenticated native
// writer before append. It is not an artifact input format or a caller trust flag.
type contributionLocalDescriptor struct {
	Version         string                   `json:"version"`
	Project         string                   `json:"project"`
	Revision        string                   `json:"revision"`
	Actor           string                   `json:"actor"`
	Operation       c.Operation              `json:"operation"`
	Authority       c.AuthorityRef           `json:"authority"`
	NativePayload   []byte                   `json:"nativePayload"`
	NativeSignature []byte                   `json:"nativeSignature"`
	Sources         []migration.NativeRef    `json:"sources"`
	Submission      contributionWireEnvelope `json:"submission"`
}
type contributionLocalResult struct {
	Version      string                     `json:"version"`
	Artifact     string                     `json:"artifact"`
	NativeCommit string                     `json:"nativeCommit"`
	DecodedBytes int64                      `json:"decodedBytes"`
	Objects      int64                      `json:"objects"`
	Events       int64                      `json:"events"`
	Depth        int64                      `json:"depth"`
	Envelopes    []contributionWireEnvelope `json:"envelopes"`
}
type contributionLocalPlan struct {
	a              *contributionAuthority
	x              *ContributionExchange
	descriptor     contributionLocalDescriptor
	digest, intent string
}

func contributionExistingExchange(a *contributionAuthority) (*ContributionExchange, error) {
	dir, e := contributionSharedDirectory(false)
	if e != nil {
		return nil, e
	}
	root := filepath.Join(dir, "inbox-"+a.Policy.Contributions.Project[7:])
	if e = safepath.CheckDir(root, true); e != nil {
		return nil, e
	}
	return &ContributionExchange{root: root, policy: *a.Policy.Contributions}, nil
}

// Shared accounting does not make private native descriptors public artifacts.
// Prevent cross-form digest/intent replay from bypassing the public parser.
func contributionPublicInput(x *ContributionExchange, digest, intent string) error {
	journal, e := x.load()
	if e != nil {
		return e
	}
	for _, attempt := range journal.Attempts {
		if attempt.Artifact != digest && attempt.Intent != intent {
			continue
		}
		local := attempt.Failure == "trusted-local-native hn.contribution-local/1"
		if _, e := os.Lstat(filepath.Join(x.root, "local-"+attempt.Artifact[7:]+".json")); e == nil {
			local = true
		} else if !os.IsNotExist(e) {
			return e
		}
		if local {
			return fmt.Errorf("host-local native descriptor cannot select public artifact intake")
		}
	}
	return nil
}

// Raw authentication and governed native classification have separate indexes.
// A valid but unadmitted raw successor cannot poison unrelated shared content
// merely by requiring a classification outside this project's finite profile.
type contributionNativeGraph struct {
	raw    map[string]StoredEvent
	facts  ContributionNativeSource
	limits c.Limits
}

func newContributionNativeGraph(events []StoredEvent, l c.Limits) *contributionNativeGraph {
	raw := map[string]StoredEvent{}
	for _, s := range events {
		raw[s.ID] = s
	}
	return &contributionNativeGraph{raw: raw, facts: ContributionNativeSource{facts: map[string]c.Fact{}}, limits: l}
}
func (g *contributionNativeGraph) Fact(id string) (c.Fact, error) {
	if _, ok := g.facts.facts[id]; ok {
		return g.facts.Fact(id)
	}
	selected := map[string]StoredEvent{}
	visiting := map[string]bool{}
	var walk func(string, int64) error
	walk = func(id string, depth int64) error {
		if _, ok := selected[id]; ok {
			return nil
		}
		if depth > g.limits.Depth || visiting[id] || len(selected) >= c.MaxGraphRecords || int64(len(selected)) >= g.limits.Retained.Events {
			return fmt.Errorf("classified native dependency bound/cycle")
		}
		s, ok := g.raw[id]
		if !ok {
			return c.ErrMissingDependency
		}
		if !isIssueKind(s.Event.Kind) && !isResourceKind(s.Event.Kind) && !isProposalKind(s.Event.Kind) {
			return fmt.Errorf("native fact has no governed content classification")
		}
		visiting[id] = true
		for _, dep := range replicationEventReferences(s.Event) {
			if e := walk(dep, depth+1); e != nil {
				return e
			}
		}
		delete(visiting, id)
		selected[id] = s
		return nil
	}
	if e := walk(id, 0); e != nil {
		return c.Fact{}, e
	}
	events := []StoredEvent{}
	for _, s := range selected {
		events = append(events, s)
	}
	source, e := classifyContributionFacts(events)
	if e != nil {
		return c.Fact{}, e
	}
	additional := 0
	for key := range source.facts {
		if _, ok := g.facts.facts[key]; !ok {
			additional++
		}
	}
	if len(g.facts.facts)+additional > c.MaxGraphRecords || int64(len(g.facts.facts)+additional) > g.limits.Retained.Events {
		return c.Fact{}, fmt.Errorf("classified native graph bound")
	}
	for key, fact := range source.facts {
		g.facts.facts[key] = fact
	}
	return g.facts.Fact(id)
}

func resolveContributionBranch(ref string) (string, PolicyDocument, string, error) {
	if !strings.HasPrefix(ref, "refs/heads/") || strings.TrimPrefix(ref, "refs/heads/") == "" {
		return "", PolicyDocument{}, "", fmt.Errorf("contribution authority requires direct refs/heads/NAME")
	}
	ctx, cancel, e := contributionContext(context.Background(), c.DefaultLimits())
	if e != nil {
		return "", PolicyDocument{}, "", e
	}
	defer cancel()
	dir, e := requireGitRepository()
	if e != nil {
		return "", PolicyDocument{}, "", e
	}
	if _, e = projectGit(ctx, dir, nil, 256, "check-ref-format", ref); e != nil {
		return "", PolicyDocument{}, "", e
	}
	if _, e = projectGit(ctx, dir, nil, 256, "symbolic-ref", "-q", ref); e == nil {
		return "", PolicyDocument{}, "", fmt.Errorf("contribution authority must be direct local branch")
	}
	raw, e := projectGit(ctx, dir, nil, 256, "rev-parse", "--verify", ref+"^{commit}")
	if e != nil {
		return "", PolicyDocument{}, "", e
	}
	commit := strings.TrimSpace(string(raw))
	policy, digest, e := contributionPolicyAt(ctx, dir, commit)
	return commit, policy, digest, e
}

// Git establishes the current private/common relationship; existing migration
// path checks validate the physical directory and worktree back-links. The
// common directory is never guessed from an anchor or received source metadata.
func contributionPrivateGitDirectory() (string, error) {
	ctx, cancel, e := contributionContext(context.Background(), c.DefaultLimits())
	if e != nil {
		return "", e
	}
	defer cancel()
	cwd, e := os.Getwd()
	if e != nil {
		return "", e
	}
	private, e := migrationDestinationGitDir(ctx, cwd)
	if e != nil {
		return "", e
	}
	if _, e = migrationDestinationRepoDir(ctx, private); e != nil {
		return "", e
	}
	if e = safepath.CheckDir(private, false); e != nil {
		return "", e
	}
	return private, nil
}
func contributionGitDirectory() (string, error) {
	private, e := contributionPrivateGitDirectory()
	if e != nil {
		return "", e
	}
	ctx, cancel, e := contributionContext(context.Background(), c.DefaultLimits())
	if e != nil {
		return "", e
	}
	defer cancel()
	raw, e := projectGit(ctx, private, nil, 4096, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if e != nil {
		return "", e
	}
	common, e := safepath.Canonical(strings.TrimSpace(string(raw)))
	if e != nil {
		return "", e
	}
	if e = safepath.CheckDir(common, false); e != nil {
		return "", e
	}
	if common != private && (filepath.Base(filepath.Dir(private)) != "worktrees" || filepath.Dir(filepath.Dir(private)) != common) {
		return "", fmt.Errorf("unrecognized Git worktree/common-directory relationship")
	}
	if _, e = os.Lstat(filepath.Join(common, "commondir")); !os.IsNotExist(e) {
		return "", fmt.Errorf("common Git directory must physically own its object store")
	}
	if e = contributionSafeGitDir(common); e != nil {
		return "", e
	}
	return common, nil
}
func contributionSharedDirectory(create bool) (string, error) {
	dir, e := contributionGitDirectory()
	if e != nil {
		return "", e
	}
	for _, part := range []string{"hn", "contribution"} {
		dir = filepath.Join(dir, part)
		if _, e = os.Lstat(dir); os.IsNotExist(e) && create {
			if e = os.Mkdir(dir, 0700); e != nil {
				return "", e
			}
		}
		if e = safepath.CheckDir(dir, true); e != nil {
			return "", e
		}
	}
	return dir, nil
}

// Coverage is receiver-local accounting provenance, never project authority.
// Portable signatures determine logical admission; these charged registrations
// determine whether this receiver may use its retained graph in governed views.
type contributionCoverage struct {
	Version  string   `json:"version"`
	Project  string   `json:"project"`
	Artifact string   `json:"artifact"`
	Native   []string `json:"native"`
	Heads    []string `json:"heads"`
	Checksum string   `json:"checksum,omitempty"`
}

func contributionCoverageBytes(project, artifact string, native, heads []string) ([]byte, error) {
	coverage := contributionCoverage{Version: "hn.contribution-retention/1", Project: project, Artifact: artifact, Native: sortedUniqueStrings(native...), Heads: sortedUniqueStrings(heads...)}
	body, e := json.Marshal(coverage)
	if e != nil {
		return nil, e
	}
	coverage.Checksum = c.Digest(body)
	return json.Marshal(coverage)
}
func contributionWriteCoverage(x *ContributionExchange, artifact string, raw []byte) error {
	return writePrivateFileAtomic(filepath.Join(x.root, "coverage-"+artifact[7:]+".json"), raw)
}
func contributionRetentionKnown(a *contributionAuthority) (map[string]bool, map[string]bool, error) {
	native, heads := map[string]bool{}, map[string]bool{}
	x, e := contributionExistingExchange(a)
	if e != nil {
		return nil, nil, e
	}
	journal, e := x.load()
	if e != nil {
		return nil, nil, e
	}
	for _, attempt := range journal.Attempts {
		for _, write := range attempt.ManifestWrites {
			if write.Material != "" {
				heads[write.Head] = true
			}
		}
		raw, e := readPrivateFileBounded(filepath.Join(x.root, "coverage-"+attempt.Artifact[7:]+".json"), min(attempt.Reserved.DecodedBytes, int64(4<<20)))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, nil, e
		}
		var coverage contributionCoverage
		if e = contributionStrict(raw, &coverage); e != nil {
			return nil, nil, e
		}
		checksum := coverage.Checksum
		coverage.Checksum = ""
		body, _ := json.Marshal(coverage)
		if checksum != c.Digest(body) || coverage.Version != "hn.contribution-retention/1" || coverage.Project != journal.Ledger.Project || coverage.Artifact != attempt.Artifact || int64(len(coverage.Native)) > attempt.Reserved.Events || len(coverage.Native) > c.MaxGraphRecords || len(coverage.Heads) > c.MaxGraphRecords {
			return nil, nil, fmt.Errorf("invalid retained snapshot coverage")
		}
		prior := ""
		for _, id := range coverage.Native {
			if !validEventID(id) || id <= prior {
				return nil, nil, fmt.Errorf("invalid retained native coverage")
			}
			prior = id
			native[id] = true
		}
		prior = ""
		for _, head := range coverage.Heads {
			if !validGitOID(head) || head <= prior {
				return nil, nil, fmt.Errorf("invalid retained manifest coverage")
			}
			prior = head
			heads[head] = true
		}
	}
	if len(a.Policy.Contributions.Baseline) > 0 {
		if e = contributionRequireBaseline(a); e != nil {
			return nil, nil, e
		}
		for _, id := range a.Policy.Contributions.Baseline {
			native[id] = true
		}
	}
	return native, heads, nil
}
func contributionManifestHeads(a *contributionAuthority) ([]string, error) {
	dir, e := contributionGitDirectory()
	if e != nil {
		return nil, e
	}
	ctx, cancel, e := contributionContext(context.Background(), a.Policy.Contributions.Limits)
	if e != nil {
		return nil, e
	}
	defer cancel()
	raw, e := projectGit(ctx, dir, nil, c.MaxGraphRecords*128, "for-each-ref", "--format=%(objectname)", "refs/hn/contributions/"+a.Policy.Contributions.Project[7:]+"/")
	if e != nil {
		return nil, e
	}
	return sortedUniqueStrings(strings.Fields(string(raw))...), nil
}
func contributionRequireRetention(a *contributionAuthority, view *c.View) error {
	if view == nil {
		return nil
	}
	native, knownHeads, e := contributionRetentionKnown(a)
	if e != nil {
		return e
	}
	heads, e := contributionManifestHeads(a)
	if e != nil {
		return e
	}
	for _, head := range heads {
		if !knownHeads[head] {
			return fmt.Errorf("retained contribution snapshot is not locally registered; explicitly run hn contribution retained register")
		}
	}
	for id := range view.Roots {
		if !native[id] {
			return fmt.Errorf("retained native dependency %s is not locally registered; explicitly run hn contribution retained register", id)
		}
	}
	return nil
}
