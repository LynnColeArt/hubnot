package contribution

import (
	"fmt"
	"sort"
)

// Fact is classified from an already authenticated native event by the host.
// Actor is the event signer; Content.Author is the original resource author.
type Fact struct {
	Actor     string
	Operation Operation
	Content   Content
}
type NativeSource interface {
	Fact(revision string) (Fact, error)
}

// Historical must verify exact policy bytes, commit and accepted authority
// ancestry. Established verifies an exact record checkpoint in that ancestry
// before a policy transition, never a timestamp or source-supplied assertion.
type AuthoritySource interface {
	Historical(AuthorityRef) (Authority, error)
	Established(recordID string, evaluated AuthorityRef) (bool, error)
}
type Conflict struct {
	Key string
	IDs []string
}
type Diagnostic struct {
	ID     string
	Reason string
}
type View struct {
	Roots         map[string]string
	Project       string
	Authority     AuthorityRef
	BlockedActors map[string]bool
	Admitted      map[string]bool
	Visible       map[string]bool
	Heads         map[string][]string
	Pending       []string
	Conflicts     []Conflict
	Revoked       []string
	Invalid       []Diagnostic
	DecisionHeads map[Target][]string
}
type graphRecord struct {
	id        string
	record    Record
	authority Authority
	effective bool
	valid     bool
}

func targetKey(t Target) string { return string(t.Kind) + ":" + t.ID }
func sameContent(a, b Content) bool {
	return a.Root == b.Root && a.Revision == b.Revision && a.Author == b.Author && equalIDs(a.Parents, b.Parents) && equalReferences(a.References, b.References)
}
func equalReferences(a, b []Reference) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func equalIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Reduce builds governed inputs BEFORE native catalogs, expected heads and
// proposal lineage. It does not mutate raw storage or infer automatic admission.
// Invalid untrusted graph nodes are isolated with diagnostics, never promoted;
// unrelated valid content remains inspectable. Baseline errors fail the policy.
func Reduce(current Authority, input []Verified, native NativeSource, sources AuthoritySource) (View, error) {
	out := View{Roots: map[string]string{}, Project: "", Authority: current.Ref, BlockedActors: map[string]bool{}, Admitted: map[string]bool{}, Visible: map[string]bool{}, Heads: map[string][]string{}, DecisionHeads: map[Target][]string{}}
	if err := current.validate(); err != nil {
		return out, err
	}
	out.Project = current.Policy.Project
	if native == nil || sources == nil {
		return out, fmt.Errorf("native and authority adapters required")
	}
	if len(input) > MaxGraphRecords || int64(len(input)) > current.Policy.Limits.Retained.Events {
		return out, fmt.Errorf("contribution graph budget exceeded")
	}
	nodes := map[string]*graphRecord{}
	ids := []string{}
	intentIDs := map[string][]string{}
	seenInput := map[string]bool{}
	for _, v := range input {
		if v.id == "" || len(v.payload) == 0 {
			return out, fmt.Errorf("unauthenticated contribution record")
		}
		if seenInput[v.id] {
			continue
		}
		seenInput[v.id] = true
		r := v.Record()
		if r.Project != current.Policy.Project {
			out.Invalid = append(out.Invalid, Diagnostic{v.id, "wrong project scope"})
			continue
		}
		nodes[v.id] = &graphRecord{id: v.id, record: r, valid: true, effective: true}
		ids = append(ids, v.id)
		k := r.Actor + ":" + r.Intent
		intentIDs[k] = append(intentIDs[k], v.id)
	}
	sort.Strings(ids)
	invalid := func(n *graphRecord, why string) {
		if n.valid {
			out.Invalid = append(out.Invalid, Diagnostic{n.id, why})
		}
		n.valid = false
		n.effective = false
	}
	for k, group := range intentIDs {
		if len(group) > 1 {
			sort.Strings(group)
			out.Conflicts = append(out.Conflicts, Conflict{"intent:" + k, group})
			for _, id := range group {
				invalid(nodes[id], "conflicting actor/project intent")
			}
		}
	}
	facts := map[string]Fact{}
	factDepth := map[string]int{}
	factErrors := map[string]error{}
	factVisiting := map[string]bool{}
	var getFact func(string, int) (Fact, error)
	getFact = func(id string, depth int) (Fact, error) {
		if f, ok := facts[id]; ok {
			return f, nil
		}
		if err := factErrors[id]; err != nil {
			return Fact{}, err
		}
		if depth > int(current.Policy.Limits.Depth) || len(nodes)+len(facts)+len(factErrors) >= MaxGraphRecords || int64(len(nodes)+len(facts)+len(factErrors)) >= current.Policy.Limits.Retained.Events || int64(len(nodes)+len(facts)+len(factErrors)) >= current.Policy.Limits.Retained.Objects {
			return Fact{}, fmt.Errorf("native dependency budget exceeded")
		}
		if factVisiting[id] {
			return Fact{}, fmt.Errorf("native dependency cycle")
		}
		factVisiting[id] = true
		defer delete(factVisiting, id)
		fail := func(err error) (Fact, error) { factErrors[id] = err; return Fact{}, err }
		f, err := native.Fact(id)
		if err != nil {
			return fail(err)
		}
		if f.Content.Revision != id || !validActor(f.Actor) || !contentOperation(f.Operation) {
			return fail(fmt.Errorf("invalid classified native fact"))
		}
		if err = f.Content.validate(); err != nil {
			return fail(err)
		}
		if f.Content.Root == id {
			if authorOperation(f.Operation) {
				return fail(fmt.Errorf("native edit lacks predecessor lineage"))
			}
			if f.Actor != f.Content.Author || len(f.Content.Parents) != 0 {
				return fail(fmt.Errorf("invalid native root/author"))
			}
		} else {
			if !authorOperation(f.Operation) {
				return fail(fmt.Errorf("native creation/reply cannot revise another root"))
			}
			root, e := getFact(f.Content.Root, depth+1)
			if e != nil {
				return fail(e)
			}
			if root.Content.Root != f.Content.Root || root.Content.Author != f.Content.Author || len(f.Content.Parents) == 0 || !sameFamily(root.Operation, f.Operation) {
				return fail(fmt.Errorf("invalid native root lineage"))
			}
		}
		for _, parent := range f.Content.Parents {
			p, e := getFact(parent, depth+1)
			if e != nil {
				return fail(e)
			}
			if p.Content.Root != f.Content.Root || p.Content.Author != f.Content.Author {
				return fail(fmt.Errorf("cross-root native predecessor"))
			}
		}
		for _, ref := range f.Content.References {
			target, e := getFact(ref.ID, depth+1)
			if e != nil {
				return fail(e)
			}
			if ref.Kind == RootReference && target.Content.Root != ref.ID {
				return fail(fmt.Errorf("root reference names a non-root revision"))
			}
		}
		if (f.Operation == IssueReply || f.Operation == DiscussionReply) && len(f.Content.References) == 0 {
			return fail(fmt.Errorf("reply lacks exact target reference"))
		}
		maxDepth := 0
		dependencies := append(append([]string{}, f.Content.Parents...), referenceIDs(f.Content.References)...)
		if f.Content.Root != id {
			dependencies = append(dependencies, f.Content.Root)
		}
		for _, dep := range dependencies {
			if factDepth[dep]+1 > maxDepth {
				maxDepth = factDepth[dep] + 1
			}
		}
		if int64(maxDepth) > current.Policy.Limits.Depth {
			return fail(fmt.Errorf("native dependency depth exceeded"))
		}
		factDepth[id] = maxDepth
		facts[id] = f
		return f, nil
	}
	for _, id := range current.Policy.Baseline {
		if _, err := getFact(id, 0); err != nil {
			return out, fmt.Errorf("invalid historical baseline: %w", err)
		}
	}
	baselineSet := map[string]bool{}
	for _, id := range current.Policy.Baseline {
		baselineSet[id] = true
	}
	for _, id := range current.Policy.Baseline {
		f := facts[id]
		for _, dep := range append(append([]string{}, f.Content.Parents...), referenceIDs(f.Content.References)...) {
			if !baselineSet[dep] {
				return out, fmt.Errorf("baseline lacks exact structural dependency closure")
			}
		}
	}
	for _, id := range ids {
		n := nodes[id]
		if !n.valid {
			continue
		}
		r := n.record
		if r.Submission != nil {
			f, e := getFact(r.Submission.Content.Revision, 0)
			if e != nil || f.Actor != r.Actor || f.Operation != r.Submission.Operation || !sameContent(f.Content, r.Submission.Content) {
				invalid(n, "submission does not bind exact authenticated native fact")
			}
			continue
		}
		ref := AuthorityRef{}
		if r.Decision != nil {
			ref = r.Decision.Authority
		} else {
			ref = r.Receipt.Authority
		}
		a, e := sources.Historical(ref)
		if e != nil || a.Ref != ref || a.validate() != nil || a.Policy.Project != r.Project {
			invalid(n, "invalid historical authority")
			continue
		}
		if ref.Digest != current.Ref.Digest {
			established, e := sources.Established(id, ref)
			if e != nil || !established {
				invalid(n, "old policy requires established authority checkpoint")
				continue
			}
		}
		n.authority = a
		if r.Decision != nil && !listed(r.Actor, a.Policy.Moderators) {
			invalid(n, "decision signer lacks evaluated moderator authority")
			continue
		}
		if r.Receipt != nil && !listed(r.Actor, a.Maintainers) && !listed(r.Actor, a.Policy.Moderators) {
			invalid(n, "receipt signer lacks evaluated intake authority")
			continue
		}
		if listed(r.Actor, a.Policy.RevokedModerators) {
			invalid(n, "signer revoked by evaluated policy")
			continue
		}
		if listed(r.Actor, current.Policy.RevokedModerators) {
			n.effective = false
			out.Revoked = append(out.Revoked, id)
		}
	}
	// Validate decision DAGs independently by target. Missing/hostile parents poison
	// only the dependent nodes and cannot erase a valid head on another target.
	decisionDepth := map[string]int{}
	visited := map[string]bool{}
	visiting := map[string]bool{}
	var decisionValid func(*graphRecord, int) bool
	decisionValid = func(n *graphRecord, depth int) bool {
		if !n.valid {
			return false
		}
		if visited[n.id] {
			return n.valid
		}
		if depth > int(current.Policy.Limits.Depth) || visiting[n.id] {
			invalid(n, "decision cycle/depth exceeded")
			return false
		}
		visiting[n.id] = true
		defer delete(visiting, n.id)
		d := n.record.Decision
		switch d.Target.Kind {
		case SubmissionTarget:
			s := nodes[d.Target.ID]
			if s == nil || !s.valid || s.record.Submission == nil {
				invalid(n, "missing valid submission target")
			}
		case RevisionTarget:
			if _, e := getFact(d.Target.ID, 0); e != nil {
				invalid(n, "missing authenticated revision target")
			}
		}
		for _, parent := range d.Parents {
			p := nodes[parent]
			if p == nil || p.record.Decision == nil || p.record.Decision.Target != d.Target || !decisionValid(p, depth+1) {
				invalid(n, "missing or invalid same-target decision parent")
			} else if decisionDepth[parent]+1 > decisionDepth[n.id] {
				decisionDepth[n.id] = decisionDepth[parent] + 1
			}
		}
		if int64(decisionDepth[n.id]) > current.Policy.Limits.Depth {
			invalid(n, "decision dependency depth exceeded")
		}
		visited[n.id] = true
		return n.valid
	}
	for _, id := range ids {
		n := nodes[id]
		if n.valid && n.record.Decision != nil {
			decisionValid(n, 0)
		}
	}
	// Completed intake is mandatory for automatic AND manual admission. Receipt
	// signatures attest the accounting transition; local intake still measures and
	// reserves its own retention independently of any replicated receipt.
	receipts := map[string][]*graphRecord{}
	for _, id := range ids {
		n := nodes[id]
		if !n.valid || n.record.Receipt == nil {
			continue
		}
		r := n.record.Receipt
		s := nodes[r.Submission]
		if s == nil || !s.valid || s.record.Submission == nil {
			invalid(n, "receipt lacks exact valid submission")
			continue
		}
		if !r.Cost.within(n.authority.Policy.Limits) {
			invalid(n, "receipt exceeds evaluated intake bounds")
			continue
		}
		for _, evaluated := range r.Evaluated {
			p := nodes[evaluated]
			if p == nil || !p.valid || p.record.Decision == nil || p.record.Decision.Target.Kind == SubmissionTarget {
				invalid(n, "invalid evaluated moderation evidence")
			}
		}
		if !n.valid {
			continue
		}
		evaluatedTargets := map[Target]bool{}
		for _, evaluated := range r.Evaluated {
			p := nodes[evaluated]
			d := p.record.Decision
			if evaluatedTargets[d.Target] {
				invalid(n, "conflicted evaluated moderation heads")
				break
			}
			evaluatedTargets[d.Target] = true
			subject := s.record.Submission.Content
			relevant := d.Target.Kind == ActorTarget && (d.Target.ID == s.record.Actor || d.Target.ID == subject.Author)
			if d.Target.Kind == RevisionTarget {
				target := facts[d.Target.ID]
				for _, parent := range subject.Parents {
					if d.Target.ID == parent || target.Content.Root == facts[parent].Content.Root {
						relevant = true
					}
				}
				for _, ref := range subject.References {
					if d.Target.ID == ref.ID || ref.Kind == RootReference && target.Content.Root == ref.ID {
						relevant = true
					}
				}
			}
			if !relevant {
				invalid(n, "irrelevant evaluated moderation evidence")
				break
			}
			if d.Action == Block || d.Action == Hide {
				invalid(n, "evaluated moderation forbids intake")
				break
			}
		}
		if !n.valid {
			continue
		}
		req := Request{Operation: s.record.Submission.Operation, Actor: s.record.Actor, Author: s.record.Submission.Content.Author, Verified: true, DependenciesAdmitted: true, Receipt: true}
		if r.Automatic {
			permission, e := Evaluate(n.authority, req)
			if e != nil || !permission.Allowed || !permission.Automatic {
				invalid(n, "receipt does not satisfy evaluated automatic rule")
				continue
			}
		}
		if n.effective {
			receipts[r.Submission] = append(receipts[r.Submission], n)
		}
	}
	// Candidate heads delete only effective, valid ancestors. Revoked/invalid
	// descendants cannot suppress surviving legitimate decisions.
	groups := map[Target][]*graphRecord{}
	for _, id := range ids {
		n := nodes[id]
		if n.valid && n.effective && n.record.Decision != nil {
			d := n.record.Decision
			if d.Target.Kind == SubmissionTarget && len(receipts[d.Target.ID]) == 0 {
				invalid(n, "admission decision lacks completed bounded intake")
				continue
			}
			groups[d.Target] = append(groups[d.Target], n)
		}
	}
	chosen := map[Target]*graphRecord{}
	conflicted := map[Target]bool{}
	for target, group := range groups {
		heads := map[string]*graphRecord{}
		for _, n := range group {
			heads[n.id] = n
		}
		for _, n := range group {
			for _, parent := range n.record.Decision.Parents {
				delete(heads, parent)
			}
		}
		headIDs := []string{}
		for id := range heads {
			headIDs = append(headIDs, id)
		}
		sort.Strings(headIDs)
		out.DecisionHeads[target] = headIDs
		if len(headIDs) > 1 {
			conflicted[target] = true
			out.Conflicts = append(out.Conflicts, Conflict{targetKey(target), headIDs})
		} else if len(headIDs) == 1 {
			chosen[target] = heads[headIDs[0]]
		}
	}
	actorBlocked := func(actor string) bool {
		t := Target{ActorTarget, actor}
		if conflicted[t] {
			return true
		}
		n := chosen[t]
		return n != nil && n.record.Decision.Action == Block
	}
	for target := range groups {
		if target.Kind == ActorTarget && actorBlocked(target.ID) {
			out.BlockedActors[target.ID] = true
		}
	}
	candidates := map[string]bool{}
	for _, id := range current.Policy.Baseline {
		candidates[id] = true
	}
	for _, id := range ids {
		n := nodes[id]
		if !n.valid || n.record.Submission == nil {
			continue
		}
		s := n.record.Submission
		t := Target{SubmissionTarget, id}
		admitted := false
		if !conflicted[t] && !actorBlocked(n.record.Actor) {
			if d := chosen[t]; d != nil {
				if d.record.Decision.Action == Admit {
					permission, e := Evaluate(d.authority, Request{Operation: s.Operation, Actor: n.record.Actor, Author: s.Content.Author, Verified: true, DependenciesAdmitted: true, Receipt: true, Manual: true, Moderator: d.record.Actor})
					admitted = e == nil && permission.Allowed
				}
			}
			if chosen[t] == nil {
				for _, r := range receipts[id] {
					if r.record.Receipt.Automatic {
						admitted = true
					}
				}
			}
		}
		if admitted {
			candidates[s.Content.Revision] = true
		} else {
			out.Pending = append(out.Pending, id)
		}
	}
	// Admission is an intersection, never an implicit promotion of raw parents.
	state := map[string]uint8{}
	var admit func(string, int) bool
	admit = func(id string, depth int) bool {
		if state[id] == 2 {
			return out.Admitted[id]
		}
		if state[id] == 1 || depth > int(current.Policy.Limits.Depth) || !candidates[id] {
			return false
		}
		state[id] = 1
		f, ok := facts[id]
		allowed := ok && !actorBlocked(f.Actor) && !actorBlocked(f.Content.Author)
		if allowed {
			for _, dep := range append(append([]string{}, f.Content.Parents...), referenceIDs(f.Content.References)...) {
				if !admit(dep, depth+1) {
					allowed = false
				}
			}
		}
		state[id] = 2
		if allowed {
			out.Admitted[id] = true
		}
		return allowed
	}
	candidateIDs := []string{}
	for id := range candidates {
		candidateIDs = append(candidateIDs, id)
	}
	sort.Strings(candidateIDs)
	for _, id := range candidateIDs {
		admit(id, 0)
	}

	// Heads include hidden admitted facts as tombstones; visible-body selection is
	// separate. Raw unadmitted successors never enter this native lineage input.
	heads := map[string]map[string]bool{}
	for _, id := range candidateIDs {
		if out.Admitted[id] {
			root := facts[id].Content.Root
			if heads[root] == nil {
				heads[root] = map[string]bool{}
			}
			heads[root][id] = true
		}
	}
	for _, id := range candidateIDs {
		if out.Admitted[id] {
			for _, parent := range facts[id].Content.Parents {
				delete(heads[facts[id].Content.Root], parent)
			}
		}
	}
	for root, group := range heads {
		for id := range group {
			out.Heads[root] = append(out.Heads[root], id)
		}
		sort.Strings(out.Heads[root])
	}
	for id, f := range facts {
		out.Roots[id] = f.Content.Root
	}
	// Native exact references form a DAG, but root references resolve to current
	// heads and may create derived cycles (accepted answer -> reply -> thread).
	// Compute the greatest visibility fixed point by propagating false through
	// reverse dependencies. A shared root gate avoids a quadratic heads fanout.
	visible := map[string]bool{}
	reverse := map[string][]string{}
	queue := []string{}
	for _, id := range candidateIDs {
		if out.Admitted[id] {
			t := Target{RevisionTarget, id}
			n := chosen[t]
			visible[id] = !conflicted[t] && (n == nil || n.record.Decision.Action != Hide)
			if !visible[id] {
				queue = append(queue, id)
			}
		}
	}
	for root, headIDs := range out.Heads {
		gate := "root:" + root
		visible[gate] = len(headIDs) > 0
		for _, head := range headIDs {
			reverse[head] = append(reverse[head], gate)
		}
	}
	for _, id := range candidateIDs {
		if !out.Admitted[id] {
			continue
		}
		for _, ref := range facts[id].Content.References {
			if ref.Kind == RevisionReference {
				reverse[ref.ID] = append(reverse[ref.ID], id)
			}
			if ref.Kind == RootReference {
				gate := "root:" + ref.ID
				reverse[gate] = append(reverse[gate], id)
				if _, ok := visible[gate]; !ok {
					visible[gate] = false
					queue = append(queue, gate)
				}
			}
		}
	}
	for cursor := 0; cursor < len(queue); cursor++ {
		for _, dependent := range reverse[queue[cursor]] {
			if visible[dependent] {
				visible[dependent] = false
				queue = append(queue, dependent)
			}
		}
	}
	for _, id := range candidateIDs {
		if out.Admitted[id] && visible[id] {
			out.Visible[id] = true
		}
	}

	for _, id := range ids {
		n := nodes[id]
		if n.valid && n.record.Submission != nil && !out.Admitted[n.record.Submission.Content.Revision] && !listed(id, out.Pending) {
			out.Pending = append(out.Pending, id)
		}
	}
	sort.Strings(out.Pending)
	sort.Strings(out.Revoked)
	sort.Slice(out.Invalid, func(i, j int) bool { return out.Invalid[i].ID < out.Invalid[j].ID })
	sort.Slice(out.Conflicts, func(i, j int) bool { return out.Conflicts[i].Key < out.Conflicts[j].Key })
	return out, nil
}

func sameFamily(root, revision Operation) bool {
	switch revision {
	case IssueEdit:
		return root == IssueCreate
	case DiscussionEdit:
		return root == DiscussionCreate || root == DiscussionReply
	case ProposalRevise:
		return root == ProposalCreate
	case ProjectEdit:
		return root == ProjectCreate
	case ProjectItemEdit:
		return root == ProjectItemCreate
	case WorkflowEdit:
		return root == WorkflowCreate
	}
	return false
}

// IntakeEvidence supplies the complete receiver-known moderation cut for a new
// receipt. The host must compare this cut and current authority under its intake
// lock before signing. A receipt signer attests the cut was complete at intake;
// an offline observer cannot infer global absence/order of undisclosed decisions.
func IntakeEvidence(view View, submission Verified) ([]string, error) {
	if submission.id == "" {
		return nil, fmt.Errorf("authenticated submission required")
	}
	r := submission.Record()
	if r.Submission == nil || r.Project != view.Project {
		return nil, fmt.Errorf("wrong project or record kind")
	}
	body := r.Submission.Content
	if view.BlockedActors[r.Actor] || view.BlockedActors[body.Author] {
		return nil, fmt.Errorf("blocked actor cannot authorize intake")
	}
	heads := map[string]bool{}
	add := func(target Target) {
		for _, id := range view.DecisionHeads[target] {
			heads[id] = true
		}
	}
	add(Target{ActorTarget, r.Actor})
	add(Target{ActorTarget, body.Author})
	requireExact := func(id string) error {
		if !view.Admitted[id] || !view.Visible[id] {
			return fmt.Errorf("dependency is not admitted and visible")
		}
		add(Target{RevisionTarget, id})
		return nil
	}
	requireRoot := func(root string) error {
		heads := view.Heads[root]
		if len(heads) == 0 {
			return fmt.Errorf("resource lacks admitted state heads")
		}
		for _, head := range heads {
			if err := requireExact(head); err != nil {
				return err
			}
		}
		return nil
	}
	for _, parent := range body.Parents {
		if err := requireExact(parent); err != nil {
			return nil, err
		}
		if err := requireRoot(view.Roots[parent]); err != nil {
			return nil, err
		}
	}
	for _, ref := range body.References {
		if !view.Admitted[ref.ID] {
			return nil, fmt.Errorf("reference identity is not admitted")
		}
		if ref.Kind == RootReference {
			if err := requireRoot(ref.ID); err != nil {
				return nil, err
			}
		} else if err := requireExact(ref.ID); err != nil {
			return nil, err
		}
	}

	result := []string{}
	for id := range heads {
		result = append(result, id)
	}
	sort.Strings(result)
	if len(result) > MaxLinks {
		return nil, fmt.Errorf("evaluated moderation evidence limit exceeded")
	}
	return result, nil
}
