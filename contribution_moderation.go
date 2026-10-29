package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	c "hubnot/internal/contribution"
	"hubnot/internal/migration"
	"hubnot/internal/safepath"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// Generated project records travel through a second bounded local intake before
// they acquire public commit refs. Outbound preparation is disposable; retained
// data, failed attempts and immutable Git writes are charged by Receive first.
func contributionStoreRecords(a *contributionAuthority, envs []c.Envelope, intent string) error {
	refs := []string{a.Ref}
	hasSubmission := false
	for _, env := range envs {
		v, e := c.Verify(env.Payload, env.Signature)
		if e != nil {
			return e
		}
		if v.Record().Submission != nil {
			hasSubmission = true
		}
	}
	if hasSubmission {
		var e error
		verified := []c.Verified{}
		for _, env := range envs {
			v, err := c.Verify(env.Payload, env.Signature)
			if err != nil {
				return err
			}
			verified = append(verified, v)
		}
		refs, e = contributionRefsForRecords(verified, nil)
		if e != nil {
			return e
		}
	}
	gitdir, e := contributionGitDirectory()
	if e != nil {
		return e
	}
	dir, e := os.MkdirTemp("", "hn-contribution-control-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "control.hnc")
	digest, e := ExportContributionArtifact(context.Background(), gitdir, a.Policy.Contributions.Project, refs, envs, path, a.Policy.Contributions.Limits)
	if e != nil {
		return e
	}
	x, e := contributionExchange(a)
	if e != nil {
		return e
	}
	if _, e = x.Receive(context.Background(), path, digest, intent, refs); e != nil {
		return e
	}
	if e = contributionValidateLiveRecords(a, envs); e != nil {
		return e
	}
	return contributionRetainIntake(a, x, intent)
}
func contributionReceiptIntake(a *contributionAuthority, x *ContributionExchange, intent string) error {
	identity, e := loadIdentity()
	if e != nil {
		return e
	}
	if actorListed(identity.Actor, a.Policy.Contributions.RevokedModerators) {
		return fmt.Errorf("current intake actor explicitly revoked")
	}
	if !actorListed(identity.Actor, a.Policy.Maintainers) && !actorListed(identity.Actor, a.Policy.Contributions.Moderators) {
		return fmt.Errorf("current project intake signer required; raw submission stays pending")
	}
	unlock, e := x.lock()
	if e != nil {
		return e
	}
	envs, e := func() ([]c.Envelope, error) {
		defer unlock()
		j, e := x.load()
		if e != nil {
			return nil, e
		}
		var attempt *ContributionIntake
		for i := range j.Attempts {
			if j.Attempts[i].Intent == intent {
				attempt = &j.Attempts[i]
				break
			}
		}
		if attempt == nil || attempt.Status != "pending" {
			return nil, fmt.Errorf("completed conflict-free bounded intake required")
		}
		artifact, e := x.inspectAttempt(context.Background(), *attempt)
		if e != nil {
			return nil, e
		}
		defer artifact.Close()
		events, e := collectEvents()
		if e != nil {
			return nil, e
		}
		view, current, e := contributionView(events)
		if e != nil {
			return nil, e
		}
		if current == nil || current.Commit != a.Commit || current.PolicyDigest != a.PolicyDigest || current.ReadOnly {
			return nil, fmt.Errorf("current contribution authority changed or read-only")
		}
		records, e := contributionRecords(a)
		if e != nil {
			return nil, e
		}
		envelopes := []c.Envelope{}
		for _, submission := range artifact.Records {
			r := submission.Record()
			if r.Submission == nil {
				continue
			}
			already := false
			for _, existing := range records {
				if existing.Record().Receipt != nil && existing.Record().Receipt.Submission == submission.ID() && existing.Record().Receipt.Authority.Digest == a.PolicyDigest {
					already = true
					break
				}
			}
			if already {
				continue
			}
			evaluated, e := c.IntakeEvidence(*view, submission)
			if e != nil {
				return nil, e
			}
			permission, e := c.Evaluate(a.current(), c.Request{Operation: r.Submission.Operation, Actor: r.Actor, Author: r.Submission.Content.Author, Verified: true, DependenciesAdmitted: true, Receipt: true})
			if e != nil {
				return nil, e
			}
			if !permission.Allowed && permission.Reason != "project moderator admission required" {
				return nil, fmt.Errorf("contribution operation denied: %s", permission.Reason)
			}
			receipt := c.Receipt{Submission: submission.ID(), Authority: a.current().Ref, Artifact: attempt.Artifact, Accounting: attempt.Accounting, Window: j.Ledger.Window, Cost: attempt.Reserved, Automatic: permission.Allowed && permission.Automatic, Evaluated: evaluated}
			env, e := signContribution(identity, c.Record{Kind: c.ReceiptKind, Project: r.Project, Intent: "receipt-" + c.Digest([]byte(submission.ID() + a.PolicyDigest))[7:], Receipt: &receipt})
			if e != nil {
				return nil, e
			}
			envelopes = append(envelopes, env)
		}
		return envelopes, nil
	}()
	if e != nil {
		return e
	}
	if len(envs) == 0 {
		return nil
	}
	return contributionStoreRecords(a, envs, "receipts-"+c.Digest([]byte(intent + a.PolicyDigest))[7:])
}
func contributionDecide(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("decide admit|reject|hide|restore|block|unblock")
	}
	action := c.Action(args[0])
	flags := quietFlags("contribution decide")
	submission := flags.String("submission", "", "exact submission")
	revision := flags.String("revision", "", "exact native revision")
	actor := flags.String("actor", "", "exact actor")
	expected := flags.String("expect-heads", "", "absent or comma-separated exact decision heads")
	intent := flags.String("operation", "", "actor-local decision intent")
	_ = flags.Bool("json", false, "JSON")
	if e := flags.Parse(args[1:]); e != nil {
		return e
	}
	if flags.NArg() != 0 || *expected == "" {
		return fmt.Errorf("exact --expect-heads required")
	}
	a, e := loadContributionAuthority()
	if e != nil {
		return e
	}
	if a == nil || a.Policy.Contributions == nil || a.ReadOnly {
		return c.ErrAuthority
	}
	id, e := loadIdentity()
	if e != nil {
		return e
	}
	permission, e := c.Evaluate(a.current(), c.Request{Operation: c.Moderate, Actor: id.Actor})
	if e != nil {
		return e
	}
	if !permission.Allowed {
		return fmt.Errorf("moderation denied: %s", permission.Reason)
	}
	target := c.Target{}
	count := 0
	for _, candidate := range []c.Target{{Kind: c.SubmissionTarget, ID: *submission}, {Kind: c.RevisionTarget, ID: *revision}, {Kind: c.ActorTarget, ID: *actor}} {
		if candidate.ID != "" {
			target = candidate
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("one exact typed moderation target required")
	}
	if target.Kind == c.RevisionTarget {
		known, _, err := contributionRetentionKnown(a)
		if err != nil {
			return err
		}
		if !known[target.ID] {
			return fmt.Errorf("raw revision is not locally registered; run hn contribution retained register --revision %s before moderation", target.ID)
		}
	}
	parents := []string{}
	if *expected != "absent" {
		parents = strings.Split(*expected, ",")
		c.SortIDs(parents)
	}
	x, e := contributionExchange(a)
	if e != nil {
		return e
	}
	unlock, e := x.lock()
	if e != nil {
		return e
	}
	replayed := false
	env, e := func() (c.Envelope, error) {
		defer unlock()
		events, e := collectEvents()
		if e != nil {
			return c.Envelope{}, e
		}
		view, current, e := contributionView(events)
		if e != nil {
			return c.Envelope{}, e
		}
		if current.Commit != a.Commit || current.PolicyDigest != a.PolicyDigest {
			return c.Envelope{}, fmt.Errorf("current authority changed")
		}
		if *intent == "" {
			raw, _ := json.Marshal(c.Decision{Action: action, Target: target, Authority: a.current().Ref, Parents: parents})
			*intent = "decision-" + c.Digest(raw)[7:]
		}
		records, e := contributionRecords(a)
		if e != nil {
			return c.Envelope{}, e
		}
		for _, existing := range records {
			r := existing.Record()
			if r.Actor == id.Actor && r.Intent == *intent {
				if r.Decision == nil || r.Decision.Action != action || r.Decision.Target != target || !equalIssueIDs(r.Decision.Parents, parents) {
					return c.Envelope{}, fmt.Errorf("conflicting decision operation intent")
				}
				replayed = true
				return c.Envelope{Payload: existing.Payload(), Signature: existing.Signature()}, nil
			}
		}
		if !equalIssueIDs(parents, view.DecisionHeads[target]) {
			return c.Envelope{}, fmt.Errorf("moderation heads changed; name every exact current head")
		}
		if target.Kind == c.RevisionTarget {
			native, e := classifyContributionFacts(contributionContentEvents(events))
			if e != nil {
				return c.Envelope{}, e
			}
			if _, e = native.Fact(target.ID); e != nil {
				return c.Envelope{}, e
			}
		}
		if *intent == "" {
			raw, _ := json.Marshal(c.Decision{Action: action, Target: target, Authority: a.current().Ref, Parents: parents})
			*intent = "decision-" + c.Digest(raw)[7:]
		}
		record := c.Record{Kind: c.DecisionKind, Project: a.Policy.Contributions.Project, Intent: *intent, Decision: &c.Decision{Action: action, Target: target, Authority: a.current().Ref, Parents: parents}}
		env, e := signContribution(id, record)
		if e != nil {
			return env, e
		}
		v, e := c.Verify(env.Payload, env.Signature)
		if e != nil {
			return env, e
		}
		records, e = contributionRecords(a)
		if e != nil {
			return env, e
		}
		native, e := classifyContributionFacts(contributionContentEvents(events))
		if e != nil {
			return env, e
		}
		next, e := c.Reduce(a.current(), append(records, v), native, contributionAuthoritySource{a})
		if e != nil {
			return env, e
		}
		for _, d := range next.Invalid {
			if d.ID == v.ID() {
				return env, fmt.Errorf("decision refused: %s", d.Reason)
			}
		}
		return env, nil
	}()
	if e != nil {
		return e
	}
	if !replayed {
		if e = contributionStoreRecords(a, []c.Envelope{env}, "control-"+c.Digest(env.Payload)[7:]); e != nil {
			return e
		}
	}
	return contributionPrint(map[string]any{"id": c.Digest(env.Payload), "target": target, "action": action, "replayed": replayed})
}

// This pre-sign check predicts operation eligibility only. It creates no
// admission, receipt or authority. Completed Receive and the signed receipt are
// separate mandatory steps; the reducer never trusts this speculative check.
func contributionAuthorizeCandidate(event Event, events []StoredEvent) error {
	a, e := loadContributionAuthority()
	if e != nil {
		return e
	}
	if a == nil || a.Policy.Contributions == nil {
		return nil
	}
	if a.ReadOnly {
		return fmt.Errorf("read-only contribution authority cannot write")
	}
	raw, e := json.Marshal(event)
	if e != nil {
		return e
	}
	candidate := StoredEvent{ID: eventID(raw), Event: event}
	all := append(append([]StoredEvent{}, contributionContentEvents(events)...), candidate)
	if _, e = validateIssueRelationships(all); e != nil {
		return e
	}
	if _, e = validateResourceRelationships(all); e != nil {
		return e
	}
	native, e := classifyContributionFacts(all)
	if e != nil {
		return e
	}
	fact, e := native.Fact(candidate.ID)
	if e != nil {
		return e
	}
	view, _, e := contributionView(events)
	if e != nil {
		return e
	}
	identity, e := loadIdentity()
	if e != nil {
		return e
	}
	env, e := signContribution(identity, c.Record{Kind: c.SubmissionKind, Project: a.Policy.Contributions.Project, Intent: "preflight", Submission: &c.Submission{Operation: fact.Operation, Content: fact.Content}})
	if e != nil {
		return e
	}
	verified, e := c.Verify(env.Payload, env.Signature)
	if e != nil {
		return e
	}
	if _, e = c.IntakeEvidence(*view, verified); e != nil {
		return e
	}
	permission, e := c.Evaluate(a.current(), c.Request{Operation: fact.Operation, Actor: event.Actor, Author: fact.Content.Author, Verified: true, DependenciesAdmitted: true, Receipt: true})
	if e != nil {
		return e
	}
	if !permission.Allowed && permission.Reason != "project moderator admission required" {
		return fmt.Errorf("contribution operation denied: %s", permission.Reason)
	}
	return nil
}
func contributionRequireProposal(id string) error {
	events, e := collectEvents()
	if e != nil {
		return e
	}
	v, _, e := contributionView(events)
	if e != nil {
		return e
	}
	if v != nil && (!v.Admitted[id] || !v.Visible[id]) {
		return fmt.Errorf("proposal is not admitted and visible under current contribution authority")
	}
	return nil
}

// Public execution must pass the live admission boundary before any backend or
// runner side effect. The daemon runner is deliberately unsupported while this
// first contribution profile is selected; no implicit worker is introduced.
func contributionRunBoundary(family string, args []string) error {
	a, e := loadContributionAuthority()
	if e != nil {
		return e
	}
	if a == nil || a.Policy.Contributions == nil {
		return nil
	}
	if family == "runner" {
		return fmt.Errorf("runner is unsupported under enabled contribution policy; use explicit hn run request/execute")
	}
	if len(args) < 1 {
		return nil
	}
	if args[0] == "list" {
		return fmt.Errorf("run list is raw history; use explicit contribution inspect --raw or hn log under enabled policy")
	}
	if len(args) < 2 {
		return nil
	}
	subject := args[1]
	events, e := collectEvents()
	if e != nil {
		return e
	}
	if args[0] != "request" {
		row, e := resolveEvent(events, subject)
		if e != nil {
			return e
		}
		subject = row.Event.Subject
		if args[0] == "logs" {
			request, e := resolveEvent(events, subject)
			if e != nil {
				return e
			}
			subject = request.Event.Subject
		}
	}
	if e = contributionRequireProposal(subject); e != nil {
		return e
	}
	identity, e := loadIdentity()
	if e != nil {
		return e
	}
	permission, e := c.Evaluate(a.current(), c.Request{Operation: c.NativeExecute, Actor: identity.Actor})
	if e != nil {
		return e
	}
	if !permission.RequiresNativeAuthority {
		return c.ErrAuthority
	}
	return nil
}

func contributionPrintNativeAdmission(id string) error {
	events, e := collectEvents()
	if e != nil {
		return e
	}
	admission, e := contributionAdmission(events, id)
	if e != nil {
		return e
	}
	if admission != "" {
		fmt.Printf("Contribution admission: %s (%s)\n", admission, id)
	}
	return nil
}

// One host operation lock surrounds an entire public command, including the
// exchange's own journal locks. It serializes complete local observations,
// signing and promotion without recursively acquiring the intake lock. Direct
// host/Git writers remain outside the native protocol enforcement boundary.
func contributionCommandLock(args []string) (func(), error) {
	noop := func() {}
	if len(args) == 0 {
		return noop, nil
	}
	if args[0] == "log" {
		return noop, nil
	}
	dir, e := contributionDirectory(false)
	if e != nil && !os.IsNotExist(e) {
		if _, repoErr := requireGitRepository(); repoErr != nil {
			return noop, nil
		}
		return nil, e
	}
	if os.IsNotExist(e) {
		if args[0] != "contribution" {
			return noop, nil
		}
		dir, e = contributionDirectory(true)
		if e != nil {
			return nil, e
		}
	} else if args[0] != "contribution" {
		if _, e = os.Lstat(filepath.Join(dir, "authority.json")); os.IsNotExist(e) {
			return noop, nil
		} else if e != nil {
			return nil, e
		}
	}
	dir, e = contributionSharedDirectory(true)
	if e != nil {
		return nil, e
	}
	fd, e := syscall.Open(filepath.Join(dir, "operation.lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if e != nil {
		return nil, e
	}
	file := os.NewFile(uintptr(fd), "contribution operation lock")
	info, e := file.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		file.Close()
		return nil, fmt.Errorf("unsafe contribution operation lock")
	}
	if e = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		file.Close()
		return nil, fmt.Errorf("contribution operation busy: %w", e)
	}
	return func() { syscall.Flock(fd, syscall.LOCK_UN); file.Close() }, nil
}
func contributionValidateLiveRecords(a *contributionAuthority, envs []c.Envelope) error {
	events, e := collectEvents()
	if e != nil {
		return e
	}
	v, current, e := contributionView(events)
	if e != nil {
		return e
	}
	if current == nil || current.ReadOnly || current.Commit != a.Commit || current.PolicyDigest != a.PolicyDigest {
		return fmt.Errorf("current contribution authority changed before promotion")
	}
	records, e := contributionRecords(a)
	if e != nil {
		return e
	}
	byID := map[string]c.Verified{}
	for _, r := range records {
		byID[r.ID()] = r
	}
	for _, env := range envs {
		v, e := c.Verify(env.Payload, env.Signature)
		if e != nil {
			return e
		}
		byID[v.ID()] = v
	}
	for _, env := range envs {
		verified, e := c.Verify(env.Payload, env.Signature)
		if e != nil {
			return e
		}
		r := verified.Record()
		if r.Receipt != nil {
			submission, ok := byID[r.Receipt.Submission]
			if !ok {
				return c.ErrMissingDependency
			}
			cut, e := c.IntakeEvidence(*v, submission)
			if e != nil {
				return e
			}
			if !equalIssueIDs(cut, r.Receipt.Evaluated) {
				return fmt.Errorf("moderation evidence changed before receipt promotion")
			}
		}
		if r.Decision != nil {
			if !equalIssueIDs(v.DecisionHeads[r.Decision.Target], r.Decision.Parents) {
				return fmt.Errorf("moderation heads changed before decision promotion")
			}
			permission, e := c.Evaluate(current.current(), c.Request{Operation: c.Moderate, Actor: r.Actor})
			if e != nil {
				return e
			}
			if !permission.Allowed {
				return c.ErrAuthority
			}
		}
	}
	return nil
}

func contributionRequireVisibleRevision(events []StoredEvent, id string) error {
	view, _, e := contributionView(events)
	if e != nil {
		return e
	}
	if view != nil && !view.Visible[id] {
		return fmt.Errorf("selected native revision is not admitted and visible")
	}
	return nil
}

// contributionBeforeNative reserves the complete local attempt before the raw
// native writer creates blobs, commits or refs. Source facts are the already
// authenticated command snapshot. Only exact source metadata is planned here;
// bounded native staging/measurement follows the durable reservation.
func contributionBeforeNative(event Event, identity *Identity, events []StoredEvent) (*contributionLocalPlan, error) {
	a, e := loadContributionAuthority()
	if e != nil {
		return nil, e
	}
	if a == nil || a.Policy.Contributions == nil {
		return nil, nil
	}
	payload, signature, e := encodeAndSign(event, identity)
	if e != nil {
		return nil, e
	}
	l := a.Policy.Contributions.Limits
	if int64(len(events))+1 > l.Events {
		return nil, fmt.Errorf("trusted local authenticated snapshot exceeds native event bound")
	}
	var nativeBytes int64
	for _, s := range events {
		nativeBytes += int64(len(s.Payload) + len(s.Signature))
		if nativeBytes > l.DecodedBytes {
			return nil, fmt.Errorf("trusted local authenticated snapshot exceeds native byte bound")
		}
	}
	if l.Objects < 9 || int64(len(payload))*3+4096 > l.DecodedBytes {
		return nil, fmt.Errorf("local native write exceeds reserved object/byte bounds")
	}
	revision := c.Digest(payload)
	candidate := StoredEvent{ID: revision, Event: event, Payload: payload, Signature: signature}
	source, e := classifyContributionFacts(contributionContentEvents(append(append([]StoredEvent{}, events...), candidate)))
	if e != nil {
		return nil, e
	}
	fact, e := source.Fact(revision)
	if e != nil {
		return nil, e
	}
	envelope, e := signContribution(identity, c.Record{Kind: c.SubmissionKind, Project: a.Policy.Contributions.Project, Intent: "submit-" + revision[7:], Submission: &c.Submission{Operation: fact.Operation, Content: fact.Content}})
	if e != nil {
		return nil, e
	}
	dependencies := replicationEventReferences(event)
	if event.Previous != "" {
		dependencies = append(dependencies, event.Previous)
	}
	refs := []string{}
	if len(dependencies) > 0 {
		refs, e = contributionSelectNativeRefs(nil, sortedUniqueStrings(dependencies...), events)
		if e != nil {
			return nil, e
		}
	}
	if !stringListed(a.Ref, refs) {
		refs = append(refs, a.Ref)
	}
	sort.Strings(refs)
	sources := []migration.NativeRef{}
	for _, ref := range refs {
		oid, exists, e := refValue(ref)
		if e != nil {
			return nil, e
		}
		if !exists {
			return nil, c.ErrMissingDependency
		}
		sources = append(sources, migration.NativeRef{SourceRef: ref, OID: oid, Kind: migration.NativeKind(ref)})
	}
	descriptor := contributionLocalDescriptor{Version: "hn.contribution-local/1", Project: a.Policy.Contributions.Project, Revision: revision, Actor: event.Actor, Operation: fact.Operation, Authority: a.current().Ref, NativePayload: payload, NativeSignature: signature, Sources: sources, Submission: contributionWireEnvelope{Payload: envelope.Payload, Signature: envelope.Signature}}
	raw, e := json.Marshal(descriptor)
	if e != nil {
		return nil, e
	}
	if int64(len(raw)) > l.DecodedBytes/2 {
		return nil, fmt.Errorf("local descriptor exceeds reserved input bound")
	}
	digest := c.Digest(raw)
	intent := "native-" + revision[7:]
	x, e := contributionExchange(a)
	if e != nil {
		return nil, e
	}
	unlock, e := x.lock()
	if e != nil {
		return nil, e
	}
	defer unlock()
	journal, e := x.load()
	if e != nil {
		return nil, e
	}
	cost := contributionReservation(l)
	next, replayed, e := c.Reserve(journal.Ledger, x.policy, intent, digest, cost)
	if e != nil {
		return nil, e
	}
	if !replayed {
		journal.Ledger = next
		accounting, _ := json.Marshal(next)
		journal.Attempts = append(journal.Attempts, ContributionIntake{Intent: intent, Artifact: digest, Refs: refs, Status: "reserved", Reserved: cost, Accounting: c.Digest(accounting), Failure: "trusted-local-native hn.contribution-local/1"})
		if e = x.save(journal); e != nil {
			return nil, e
		}
	}
	path := filepath.Join(x.root, "local-"+digest[7:]+".json")
	if old, e := readPrivateFileBounded(path, l.DecodedBytes); e == nil {
		if !bytes.Equal(old, raw) {
			return nil, fmt.Errorf("conflicting durable local native descriptor")
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	} else if e = writePrivateFileAtomic(path, raw); e != nil {
		return nil, e
	}
	return &contributionLocalPlan{a: a, x: x, descriptor: descriptor, digest: digest, intent: intent}, nil
}

// Exact private native sources are reconstructed in a disposable object DB. An
// advancing local actor ref cannot silently change the originally reserved input.
// The source is the native command's already authenticated snapshot plus its
// exact new native writer proof. Existing actor readers are reused; the bounded
// exporter performs no new archive import or expansion. Legacy native actor and
// dependency validators are synchronous boundary checks, not hard-cancellable
// public reads. Cancellation is checked before promotion; no work outlives return.
func contributionLocalSnapshot(ctx context.Context, gitdir string, descriptor contributionLocalDescriptor, stored *StoredEvent, l c.Limits) (*contributionSnapshot, []StoredEvent, func(), error) {
	noop := func() {}
	event, id, e := verifyEvent(descriptor.NativePayload, descriptor.NativeSignature)
	if e != nil || id != descriptor.Revision || stored.ID != id || !bytes.Equal(stored.Payload, descriptor.NativePayload) || !bytes.Equal(stored.Signature, descriptor.NativeSignature) {
		return nil, nil, noop, fmt.Errorf("local native result differs from reserved signed intent")
	}
	refs := map[string]migration.NativeRef{}
	for _, ref := range descriptor.Sources {
		refs[ref.SourceRef] = ref
	}
	refs[actorRef(event.Actor)] = migration.NativeRef{SourceRef: actorRef(event.Actor), OID: stored.Commit, Kind: "actor"}
	if isProposalKind(event.Kind) {
		refs[proposalRef(id)] = migration.NativeRef{SourceRef: proposalRef(id), OID: event.Head, Kind: "proposal"}
	}
	selected := []migration.NativeRef{}
	names := []string{}
	roots := []string{}
	for _, ref := range refs {
		selected = append(selected, ref)
		names = append(names, ref.SourceRef)
		roots = append(roots, ref.OID)
	}
	sort.Strings(names)
	sort.Slice(selected, func(i, j int) bool { return selected[i].SourceRef < selected[j].SourceRef })
	ml, e := contributionLimits(l)
	if e != nil {
		return nil, nil, noop, e
	}
	dependencies, e := projectDependencyRoots(gitdir, selected)
	if e != nil {
		return nil, nil, noop, e
	}
	roots = append(roots, dependencies...)
	ids, e := projectObjectIDs(ctx, gitdir, roots, ml)
	if e != nil {
		return nil, nil, noop, e
	}
	pinned, e := safepath.MkdirTemp("hn-local-contribution-")
	if e != nil {
		return nil, nil, noop, e
	}
	cleanup := func() { os.RemoveAll(pinned) }
	format := "sha1"
	if len(stored.Commit) == 64 {
		format = "sha256"
	}
	if _, e = projectGit(ctx, "", nil, 4096, "init", "--bare", "--quiet", "--object-format="+format, pinned); e != nil {
		cleanup()
		return nil, nil, noop, e
	}
	if e = projectCopyObjectIDs(ctx, gitdir, pinned, ids, ml); e != nil {
		cleanup()
		return nil, nil, noop, e
	}
	var updates strings.Builder
	for _, ref := range selected {
		fmt.Fprintf(&updates, "create %s %s\n", ref.SourceRef, ref.OID)
	}
	if _, e = projectGit(ctx, pinned, []byte(updates.String()), 4096, "update-ref", "--stdin"); e != nil {
		cleanup()
		return nil, nil, noop, e
	}
	snapshot, e := exportContributionSnapshot(ctx, pinned, names, ml)
	if e != nil {
		cleanup()
		return nil, nil, noop, e
	}
	if _, _, e = contributionObjectPreflight(ctx, snapshot.data, l); e != nil {
		snapshot.Close()
		cleanup()
		return nil, nil, noop, e
	}
	events := []StoredEvent{}
	seen := map[string]bool{}
	for _, ref := range selected {
		if ref.Kind != "actor" {
			continue
		}
		rows, e := validateQuarantinedActor(pinned, projectActorOwner(ref.SourceRef), ref.OID)
		if e == nil {
			e = ctx.Err()
		}
		if e != nil {
			snapshot.Close()
			cleanup()
			return nil, nil, noop, e
		}
		for _, row := range rows {
			if !seen[row.ID] {
				events = append(events, row)
				seen[row.ID] = true
			}
		}
	}
	return snapshot, events, cleanup, nil
}

func contributionFinishNative(stored *StoredEvent, plan *contributionLocalPlan) error {
	if plan == nil {
		return nil
	}
	a, x := plan.a, plan.x
	ctx, cancel, e := contributionContext(context.Background(), a.Policy.Contributions.Limits)
	if e != nil {
		return e
	}
	defer cancel()
	gitdir, e := contributionGitDirectory()
	if e != nil {
		return e
	}
	unlock, e := x.lock()
	if e != nil {
		return e
	}
	defer unlock()
	j, e := x.load()
	if e != nil {
		return e
	}
	index := -1
	for i, attempt := range j.Attempts {
		if attempt.Intent == plan.intent && attempt.Artifact == plan.digest {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("local native reservation missing")
	}
	attempt := j.Attempts[index]
	l := a.Policy.Contributions.Limits
	resultPath := filepath.Join(x.root, "local-result-"+plan.digest[7:]+".json")
	var result contributionLocalResult
	resultRaw, e := readPrivateFileBounded(resultPath, l.DecodedBytes)
	hasResult := e == nil
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if hasResult {
		if e = contributionStrict(resultRaw, &result); e != nil {
			return e
		}
		if result.Version != "hn.contribution-local-result/1" || result.Artifact != plan.digest || result.NativeCommit != stored.Commit {
			return fmt.Errorf("local native result binding changed")
		}
		complete := len(attempt.ManifestWrites) > 0
		for _, write := range attempt.ManifestWrites {
			complete = complete && write.Complete
		}
		if complete {
			return contributionPinNativeObservation(plan, stored)
		}
	}
	if !hasResult {
		snapshot, events, cleanup, e := contributionLocalSnapshot(ctx, gitdir, plan.descriptor, stored, l)
		if e != nil {
			attempt.Status = "failed"
			attempt.Failure = "trusted local native validation failed"
			j.Attempts[index] = attempt
			_ = x.save(j)
			return e
		}
		defer cleanup()
		defer snapshot.Close()
		reader, e := snapshot.Open(ctx, snapshot.Snapshot.GitBundle)
		if e != nil {
			return e
		}
		nativeRaw, e := io.ReadAll(io.LimitReader(reader, l.DecodedBytes+1))
		reader.Close()
		if e != nil || int64(len(nativeRaw)) > l.DecodedBytes {
			return fmt.Errorf("local native snapshot byte bound")
		}
		objects, depth, e := contributionObjectPreflight(ctx, nativeRaw, l)
		if e != nil {
			return e
		}
		eventDepth, e := contributionEventDepth(ctx, events, l.Depth)
		if e != nil {
			return e
		}
		depth = max(depth, eventDepth)
		if int64(len(events))+2 > l.Events {
			return fmt.Errorf("local native event bound")
		}
		native, e := classifyContributionFacts(contributionContentEvents(events))
		if e != nil {
			return e
		}
		fact, e := native.Fact(stored.ID)
		if e != nil {
			return e
		}
		submission, e := c.Verify(plan.descriptor.Submission.Payload, plan.descriptor.Submission.Signature)
		if e != nil {
			return e
		}
		expected, _ := json.Marshal(fact.Content)
		actual, _ := json.Marshal(submission.Record().Submission.Content)
		if !bytes.Equal(expected, actual) || fact.Operation != plan.descriptor.Operation || fact.Actor != plan.descriptor.Actor {
			return fmt.Errorf("local native classifier differs from reserved submission")
		}
		allEvents, e := collectEvents()
		if e != nil {
			return e
		}
		view, current, e := contributionView(allEvents)
		if e != nil {
			return e
		}
		if current == nil || current.ReadOnly || current.current().Ref != plan.descriptor.Authority {
			return fmt.Errorf("local native authority changed before attestation")
		}
		evaluated, e := c.IntakeEvidence(*view, submission)
		if e != nil {
			return e
		}
		envelopes := []contributionWireEnvelope{plan.descriptor.Submission}
		identity, e := loadIdentity()
		if e != nil {
			return e
		}
		if (actorListed(identity.Actor, a.Policy.Maintainers) || actorListed(identity.Actor, a.Policy.Contributions.Moderators)) && !actorListed(identity.Actor, a.Policy.Contributions.RevokedModerators) {
			permission, e := c.Evaluate(a.current(), c.Request{Operation: fact.Operation, Actor: fact.Actor, Author: fact.Content.Author, Verified: true, DependenciesAdmitted: true, Receipt: true})
			if e != nil {
				return e
			}
			if !permission.Allowed && permission.Reason != "project moderator admission required" {
				return fmt.Errorf("local operation denied: %s", permission.Reason)
			}
			window := ""
			for _, reservation := range j.Ledger.Reservations {
				if reservation.Intent == plan.intent {
					window = reservation.Window
				}
			}
			receipt := c.Receipt{Submission: submission.ID(), Authority: a.current().Ref, Artifact: plan.digest, Accounting: attempt.Accounting, Window: window, Cost: attempt.Reserved, Automatic: permission.Automatic && permission.Allowed, Evaluated: evaluated}
			env, e := signContribution(identity, c.Record{Kind: c.ReceiptKind, Project: plan.descriptor.Project, Intent: "receipt-" + c.Digest([]byte(submission.ID() + a.PolicyDigest))[7:], Receipt: &receipt})
			if e != nil {
				return e
			}
			envelopes = append(envelopes, contributionWireEnvelope{Payload: env.Payload, Signature: env.Signature})
		}
		nativeIDs := []string{}
		for _, row := range events {
			nativeIDs = append(nativeIDs, row.ID)
		}
		coverage, err := contributionCoverageBytes(a.Policy.Contributions.Project, plan.digest, nativeIDs, nil)
		if err != nil {
			return err
		}
		if int64(len(nativeRaw)+len(coverage)+lenMustContributionDescriptor(plan.descriptor)) > attempt.Reserved.DecodedBytes {
			return fmt.Errorf("local coverage metadata exceeds durable reservation")
		}
		if e = contributionWriteCoverage(x, plan.digest, coverage); e != nil {
			return e
		}
		result = contributionLocalResult{Version: "hn.contribution-local-result/1", Artifact: plan.digest, NativeCommit: stored.Commit, Envelopes: envelopes, DecodedBytes: int64(len(nativeRaw)+len(coverage)) + int64(lenMustContributionDescriptor(plan.descriptor)), Objects: objects, Events: int64(len(events) + len(envelopes)), Depth: depth}
		resultRaw, e = json.Marshal(result)
		if e != nil {
			return e
		}
		descriptorRaw, _ := json.Marshal(plan.descriptor)
		measured := int64(len(nativeRaw) + len(descriptorRaw) + len(resultRaw))
		if measured > attempt.Reserved.DecodedBytes || objects > attempt.Reserved.Objects {
			return fmt.Errorf("local native closure exceeds durable reservation")
		}
		attempt.DecodedBytes = measured
		attempt.Objects = objects
		attempt.Events = int64(len(events) + len(envelopes))
		attempt.Depth = depth
		attempt.Status = "pending"
		attempt.Failure = "trusted-local-native hn.contribution-local/1"
		if e = writePrivateFileAtomic(resultPath, resultRaw); e != nil {
			return e
		}
	}
	if result.DecodedBytes < 0 || result.Objects < 0 || result.Events < 0 || result.Depth < 0 || result.DecodedBytes+int64(len(resultRaw)) > attempt.Reserved.DecodedBytes || result.Objects > attempt.Reserved.Objects || result.Events > attempt.Reserved.Events || result.Depth > attempt.Reserved.Depth {
		return fmt.Errorf("local result measurements exceed durable reservation")
	}
	attempt.DecodedBytes = result.DecodedBytes + int64(len(resultRaw))
	attempt.Objects = result.Objects
	attempt.Events = result.Events
	attempt.Depth = result.Depth
	attempt.Status = "pending"
	attempt.Failure = "trusted-local-native hn.contribution-local/1"
	envelopes := []c.Envelope{}
	actors := map[string][]c.Envelope{}
	for _, wire := range result.Envelopes {
		env := c.Envelope{Payload: wire.Payload, Signature: wire.Signature}
		v, e := c.Verify(env.Payload, env.Signature)
		if e != nil {
			return e
		}
		envelopes = append(envelopes, env)
		actors[v.Record().Actor] = append(actors[v.Record().Actor], env)
	}
	if e = contributionValidateLiveRecords(a, envelopes); e != nil {
		return e
	}
	plans := map[string]contributionManifestObjects{}
	for actor, envs := range actors {
		ref, e := contributionRef(a.Policy.Contributions.Project, actor)
		if e != nil {
			return e
		}
		expected, exists, e := refValue(ref)
		if e != nil {
			return e
		}
		if !exists {
			expected = "absent"
		}
		var prior *ContributionManifestWrite
		for i := range attempt.ManifestWrites {
			if attempt.ManifestWrites[i].Actor == actor {
				prior = &attempt.ManifestWrites[i]
				break
			}
		}
		if prior != nil {
			if expected == prior.Head {
				prior.Complete = true
				prior.ObjectsDurable = true
				continue
			}
			if expected != prior.Expected {
				return fmt.Errorf("local manifest retention uncertain; expected old head changed")
			}
			expected = prior.Expected
		}
		objects, e := planContributionManifest(ctx, gitdir, a.Policy.Contributions.Project, actor, expected, envs, l)
		if e != nil {
			return e
		}
		write := ContributionManifestWrite{Actor: actor, Expected: expected, Head: objects.head, Material: objects.material, Objects: int64(len(objects.objects)), Bytes: objects.bytes}
		if prior != nil {
			if prior.Head != write.Head || prior.Material != write.Material || prior.Objects != write.Objects || prior.Bytes != write.Bytes {
				return fmt.Errorf("local immutable manifest plan changed")
			}
		} else {
			attempt.ManifestWrites = append(attempt.ManifestWrites, write)
		}
		plans[actor] = objects
	}
	var addedBytes, addedObjects int64
	for _, write := range attempt.ManifestWrites {
		addedBytes += write.Bytes
		addedObjects += write.Objects
	}
	if attempt.DecodedBytes+addedBytes > attempt.Reserved.DecodedBytes || attempt.Objects+addedObjects > attempt.Reserved.Objects {
		return fmt.Errorf("local native and manifest retention exceed charged capacity")
	}
	j.GitDir = gitdir
	j.Attempts[index] = attempt
	if e = x.save(j); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	for actor, objects := range plans {
		if e = writeContributionManifest(ctx, gitdir, objects, l); e != nil {
			return e
		}
		for i := range attempt.ManifestWrites {
			if attempt.ManifestWrites[i].Actor == actor {
				attempt.ManifestWrites[i].ObjectsDurable = true
			}
		}
	}
	j.Attempts[index] = attempt
	if e = x.save(j); e != nil {
		return e
	}
	for i, write := range attempt.ManifestWrites {
		if write.Complete {
			continue
		}
		ref, e := contributionRef(a.Policy.Contributions.Project, write.Actor)
		if e != nil {
			return e
		}
		old := write.Expected
		if old == "absent" {
			old = strings.Repeat("0", len(write.Head))
		}
		if _, e = projectGit(ctx, gitdir, nil, 256, "-c", "core.hooksPath="+os.DevNull, "update-ref", ref, write.Head, old); e != nil {
			return e
		}
		attempt.ManifestWrites[i].Complete = true
	}
	j.Attempts[index] = attempt
	if e = x.save(j); e != nil {
		return e
	}
	return contributionPinNativeObservation(plan, stored)
}

// Retain the complete native actor observation made by this local transaction.
// This is an observed prefix, not a rewritten/truncated history. Later native
// governance appends cannot erase the exact previously received snapshot.
func contributionPinNativeObservation(plan *contributionLocalPlan, stored *StoredEvent) error {
	remote := "contribution-local-" + plan.digest[7:]
	destinations := []replicationPromotion{{Ref: acceptedActorRef(remote, stored.Event.Actor), NewOID: stored.Commit}}
	if isProposalKind(stored.Event.Kind) {
		destinations = append(destinations, replicationPromotion{Ref: acceptedProposalRef(remote, stored.ID), NewOID: stored.Event.Head})
	}
	pending := []replicationPromotion{}
	for _, p := range destinations {
		old, exists, e := refValue(p.Ref)
		if e != nil {
			return e
		}
		if exists {
			if old != p.NewOID {
				return fmt.Errorf("local native observation conflicts with reserved result")
			}
			continue
		}
		pending = append(pending, p)
	}
	return contributionPromoteRawRefs(pending)
}

func lenMustContributionDescriptor(descriptor contributionLocalDescriptor) int {
	raw, _ := json.Marshal(descriptor)
	return len(raw)
}
func contributionLoadLocalPlan(a *contributionAuthority, x *ContributionExchange, intent string) (*contributionLocalPlan, error) {
	j, e := x.load()
	if e != nil {
		return nil, e
	}
	for _, attempt := range j.Attempts {
		if attempt.Intent != intent {
			continue
		}
		raw, e := readPrivateFileBounded(filepath.Join(x.root, "local-"+attempt.Artifact[7:]+".json"), x.policy.Limits.DecodedBytes)
		if os.IsNotExist(e) {
			return nil, nil
		}
		if e != nil {
			return nil, e
		}
		if c.Digest(raw) != attempt.Artifact {
			return nil, fmt.Errorf("local descriptor digest mismatch")
		}
		var descriptor contributionLocalDescriptor
		if e = contributionStrict(raw, &descriptor); e != nil {
			return nil, e
		}
		event, id, e := verifyEvent(descriptor.NativePayload, descriptor.NativeSignature)
		if e != nil || descriptor.Version != "hn.contribution-local/1" || id != descriptor.Revision || event.Actor != descriptor.Actor || descriptor.Project != x.policy.Project {
			return nil, fmt.Errorf("invalid authenticated local descriptor")
		}
		if _, e = c.Verify(descriptor.Submission.Payload, descriptor.Submission.Signature); e != nil {
			return nil, e
		}
		return &contributionLocalPlan{a: a, x: x, descriptor: descriptor, digest: attempt.Artifact, intent: intent}, nil
	}
	return nil, nil
}
func contributionRetryNative(stored *StoredEvent) error {
	a, e := loadContributionAuthority()
	if e != nil {
		return e
	}
	if a == nil || a.Policy.Contributions == nil {
		return nil
	}
	x, e := contributionExchange(a)
	if e != nil {
		return e
	}
	plan, e := contributionLoadLocalPlan(a, x, "native-"+stored.ID[7:])
	if e != nil || plan == nil {
		return e
	}
	raw, e := readPrivateFileBounded(filepath.Join(x.root, "local-result-"+plan.digest[7:]+".json"), x.policy.Limits.DecodedBytes)
	if e == nil {
		var result contributionLocalResult
		if e = contributionStrict(raw, &result); e != nil {
			return e
		}
		original, e := readStoredEvent(result.NativeCommit)
		if e != nil || original.ID != stored.ID {
			return fmt.Errorf("original local native result unavailable")
		}
		stored = original
	} else if !os.IsNotExist(e) {
		return e
	}
	return contributionFinishNative(stored, plan)
}

// Contribution retention updates only accepted observation refs and suppresses
// Git reference-transaction hooks. Receiving signed data never executes a host
// hook, checks out code or changes a development branch.
func contributionPromoteRawRefs(promotions []replicationPromotion) error {
	if len(promotions) == 0 {
		return nil
	}
	sort.Slice(promotions, func(i, j int) bool { return promotions[i].Ref < promotions[j].Ref })
	var input strings.Builder
	input.WriteString("start\n")
	prior := ""
	for _, p := range promotions {
		_, _, actor := parseAcceptedActorRef(p.Ref)
		_, _, proposal := parseAcceptedProposalRef(p.Ref)
		if (!actor && !proposal) || !validGitOID(p.NewOID) || p.Ref == prior {
			return fmt.Errorf("invalid contribution raw observation promotion")
		}
		prior = p.Ref
		if p.OldOID == "" {
			fmt.Fprintf(&input, "create %s %s\n", p.Ref, p.NewOID)
		} else {
			if !validGitOID(p.OldOID) {
				return fmt.Errorf("invalid expected-old contribution observation")
			}
			fmt.Fprintf(&input, "update %s %s %s\n", p.Ref, p.NewOID, p.OldOID)
		}
	}
	input.WriteString("prepare\ncommit\n")
	dir, e := contributionGitDirectory()
	if e != nil {
		return e
	}
	ctx, cancel, e := contributionContext(context.Background(), c.DefaultLimits())
	if e != nil {
		return e
	}
	defer cancel()
	_, e = projectGit(ctx, dir, []byte(input.String()), 1<<20, "-c", "core.hooksPath="+os.DevNull, "update-ref", "--stdin")
	return e
}

type contributionRetainedPlan struct {
	Version   string         `json:"version"`
	Project   string         `json:"project"`
	Authority c.AuthorityRef `json:"authority"`
	Native    []string       `json:"native"`
	Heads     []string       `json:"heads"`
	Roots     []string       `json:"roots"`
	Digest    string         `json:"-"`
}

// This plan selects only already authenticated local material needed by the
// contribution graph and exact authority history. It never obtains an input
// path, key, remote selection or project role from a received descriptor.
func contributionPlanRetained(a *contributionAuthority, extra []string) (contributionRetainedPlan, error) {
	plan := contributionRetainedPlan{Version: "hn.contribution-retained-plan/1", Project: a.Policy.Contributions.Project, Authority: a.current().Ref}
	events, e := contributionAuthorityEvents(a)
	if e != nil {
		return plan, e
	}
	records, e := contributionRecords(a)
	if e != nil {
		return plan, e
	}
	native := newContributionNativeGraph(events, a.Policy.Contributions.Limits)
	view, e := c.Reduce(a.current(), records, native, contributionAuthoritySource{a})
	if e != nil {
		return plan, e
	}
	heads, e := contributionManifestHeads(a)
	if e != nil {
		return plan, e
	}
	plan.Heads = heads
	needed := []string{}
	for id := range view.Roots {
		needed = append(needed, id)
	}
	needed = append(needed, extra...)
	// Exact selected-branch native merge evidence is part of restored authority,
	// separate from contribution content. Preserve its original actor ancestors.
	ctx, cancel, err := contributionContext(context.Background(), a.Policy.Contributions.Limits)
	if err != nil {
		return plan, err
	}
	defer cancel()
	dir, err := contributionGitDirectory()
	if err != nil {
		return plan, err
	}
	for _, row := range events {
		if row.Event.Kind == "proposal.merged" {
			if _, err := projectGit(ctx, dir, nil, 256, "merge-base", "--is-ancestor", row.Event.Commit, a.Commit); err == nil {
				needed = append(needed, row.ID)
			}
		}
	}
	byID := map[string]StoredEvent{}
	for _, s := range events {
		byID[s.ID] = s
	}
	selected := map[string]bool{}
	roots := map[string]bool{a.Commit: true}
	for _, head := range heads {
		roots[head] = true
	}
	var walk func(string, int64) error
	walk = func(id string, depth int64) error {
		if selected[id] {
			return nil
		}
		if depth > a.Policy.Contributions.Limits.Depth || len(selected)+len(records) >= c.MaxGraphRecords || int64(len(selected)+len(records)) >= a.Policy.Contributions.Limits.Events {
			return fmt.Errorf("retained native snapshot event/depth bound")
		}
		s, ok := byID[id]
		if !ok {
			return c.ErrMissingDependency
		}
		if isAuditKind(s.Event.Kind) {
			return fmt.Errorf("initial retained contribution profile refuses compressed audit predecessors; preserve exact native history separately")
		}
		selected[id] = true
		roots[s.Commit] = true
		for _, oid := range []string{s.Event.Base, s.Event.Head, s.Event.Commit} {
			if oid != "" {
				roots[oid] = true
			}
		}
		dependencies := replicationEventReferences(s.Event)
		if s.Event.Previous != "" {
			dependencies = append(dependencies, s.Event.Previous)
		}
		for _, dep := range dependencies {
			if e := walk(dep, depth+1); e != nil {
				return e
			}
		}
		return nil
	}
	for _, id := range needed {
		if e = walk(id, 0); e != nil {
			return plan, e
		}
	}
	for id := range selected {
		plan.Native = append(plan.Native, id)
	}
	sort.Strings(plan.Native)
	for root := range roots {
		if !validGitOID(root) {
			return plan, fmt.Errorf("invalid retained object root")
		}
		plan.Roots = append(plan.Roots, root)
	}
	sort.Strings(plan.Roots)
	raw, e := json.Marshal(plan)
	if e != nil {
		return plan, e
	}
	plan.Digest = c.Digest(raw)
	return plan, nil
}
func contributionRegisterRetained(a *contributionAuthority, expected string, extra []string) error {
	x, e := contributionExchange(a)
	if e != nil {
		return e
	}
	plan, e := contributionPlanRetained(a, extra)
	if e != nil {
		return e
	}
	if expected != "" && plan.Digest != expected {
		return fmt.Errorf("retained snapshot changed; inspect and name the exact digest")
	}
	knownNative, knownHeads, e := contributionRetentionKnown(a)
	if e != nil {
		return e
	}
	covered := true
	for _, head := range plan.Heads {
		covered = covered && knownHeads[head]
	}
	for _, id := range plan.Native {
		covered = covered && knownNative[id]
	}
	if covered {
		return nil
	}
	unlock, e := x.lock()
	if e != nil {
		return e
	}
	defer unlock()
	j, e := x.load()
	if e != nil {
		return e
	}
	intent := "retained-" + plan.Digest[7:]
	cost := contributionReservation(a.Policy.Contributions.Limits)
	next, replay, e := c.Reserve(j.Ledger, x.policy, intent, plan.Digest, cost)
	if e != nil {
		return e
	}
	index := -1
	for i, attempt := range j.Attempts {
		if attempt.Intent == intent {
			index = i
		}
	}
	if !replay {
		j.Ledger = next
		transition, _ := json.Marshal(next)
		j.Attempts = append(j.Attempts, ContributionIntake{Intent: intent, Artifact: plan.Digest, Refs: []string{a.Ref}, Status: "reserved", Reserved: cost, Accounting: c.Digest(transition), Failure: "explicit trusted retained snapshot registration"})
		index = len(j.Attempts) - 1
		if e = x.save(j); e != nil {
			return e
		}
	}
	if index < 0 {
		return fmt.Errorf("retained reservation missing")
	}
	descriptor, _ := json.Marshal(plan)
	if int64(len(descriptor)) > cost.DecodedBytes {
		return fmt.Errorf("retained descriptor budget")
	}
	if e = writePrivateFileAtomic(filepath.Join(x.root, "retained-"+plan.Digest[7:]+".json"), descriptor); e != nil {
		return e
	}
	ctx, cancel, e := contributionContext(context.Background(), a.Policy.Contributions.Limits)
	if e != nil {
		return e
	}
	defer cancel()
	gitdir, e := contributionGitDirectory()
	if e != nil {
		return e
	}
	limits, e := contributionLimits(a.Policy.Contributions.Limits)
	if e != nil {
		return e
	}
	objects, e := projectObjectIDs(ctx, gitdir, plan.Roots, limits)
	if e != nil {
		return e
	}
	var material bytes.Buffer
	material.WriteString(projectGitSchema)
	for _, oid := range objects {
		meta, e := projectGit(ctx, gitdir, []byte(oid+"\n"), 256, "cat-file", "--batch-check=%(objecttype) %(objectsize)")
		if e != nil {
			return e
		}
		fields := strings.Fields(string(meta))
		if len(fields) != 2 {
			return fmt.Errorf("retained object unavailable")
		}
		size, e := strconv.ParseInt(fields[1], 10, 64)
		if e != nil || size < 0 || size > cost.DecodedBytes-int64(material.Len())-256 {
			return fmt.Errorf("retained snapshot byte bound")
		}
		raw, e := projectGit(ctx, gitdir, nil, size, "cat-file", fields[0], oid)
		if e != nil {
			return e
		}
		fmt.Fprintf(&material, "%s %s %d\n", oid, fields[0], len(raw))
		material.Write(raw)
		material.WriteByte('\n')
	}
	count, depth, e := contributionObjectPreflight(ctx, material.Bytes(), a.Policy.Contributions.Limits)
	if e != nil {
		return e
	}
	coverage, e := contributionCoverageBytes(plan.Project, plan.Digest, plan.Native, plan.Heads)
	if e != nil {
		return e
	}
	decoded := int64(material.Len() + len(descriptor) + len(coverage))
	if decoded > cost.DecodedBytes || count > cost.Objects {
		return fmt.Errorf("retained snapshot including metadata exceeds charged capacity")
	}
	attempt := j.Attempts[index]
	attempt.Status = "failed"
	attempt.Failure = "explicit retained snapshot registered; no public artifact"
	attempt.DecodedBytes = decoded
	attempt.Objects = count
	attempt.Events = int64(len(plan.Native))
	attempt.Depth = depth
	j.Attempts[index] = attempt
	if e = x.save(j); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	return contributionWriteCoverage(x, plan.Digest, coverage)
}

func contributionCoverArtifact(a *contributionAuthority, x *ContributionExchange, artifact *ContributionArtifact) error {
	native := []string{}
	for _, row := range artifact.Events {
		native = append(native, row.ID)
	}
	coverage, e := contributionCoverageBytes(a.Policy.Contributions.Project, artifact.Intake.Artifact, native, nil)
	if e != nil {
		return e
	}
	actors := map[string][]c.Envelope{}
	for _, record := range artifact.Records {
		r := record.Record()
		actors[r.Actor] = append(actors[r.Actor], c.Envelope{Payload: record.Payload(), Signature: record.Signature()})
	}
	gitdir, e := contributionGitDirectory()
	if e != nil {
		return e
	}
	baseBytes := artifact.DecodedBytes - int64(len(actors))*4096 - int64(len(artifact.Records))*512
	objects := artifact.Objects - int64(len(artifact.Records))*2 - int64(len(actors))*3
	for actor, envs := range actors {
		var existing *ContributionManifestWrite
		for i := range artifact.Intake.ManifestWrites {
			if artifact.Intake.ManifestWrites[i].Actor == actor {
				existing = &artifact.Intake.ManifestWrites[i]
			}
		}
		if existing != nil {
			baseBytes += existing.Bytes
			objects += existing.Objects
			continue
		}
		ref, e := contributionRef(a.Policy.Contributions.Project, actor)
		if e != nil {
			return e
		}
		old, exists, e := refValue(ref)
		if e != nil {
			return e
		}
		if !exists {
			old = "absent"
		}
		plan, e := planContributionManifest(context.Background(), gitdir, a.Policy.Contributions.Project, actor, old, envs, a.Policy.Contributions.Limits)
		if e != nil {
			return e
		}
		baseBytes += plan.bytes
		objects += int64(len(plan.objects))
	}
	if baseBytes+int64(len(coverage)) > artifact.Intake.Reserved.DecodedBytes || objects > artifact.Intake.Reserved.Objects {
		return fmt.Errorf("artifact and local retention metadata exceed reserved capacity")
	}
	unlock, e := x.lock()
	if e != nil {
		return e
	}
	defer unlock()
	journal, e := x.load()
	if e != nil {
		return e
	}
	found := false
	for i := range journal.Attempts {
		if journal.Attempts[i].Intent == artifact.Intake.Intent {
			journal.Attempts[i].DecodedBytes = max(journal.Attempts[i].DecodedBytes, artifact.DecodedBytes+int64(len(coverage)))
			if journal.Attempts[i].DecodedBytes > journal.Attempts[i].Reserved.DecodedBytes {
				return fmt.Errorf("local coverage metadata byte bound")
			}
			found = true
		}
	}
	if !found {
		return fmt.Errorf("artifact reservation unavailable")
	}
	if e = x.save(journal); e != nil {
		return e
	}
	return contributionWriteCoverage(x, artifact.Intake.Artifact, coverage)
}
