package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	c "hubnot/internal/contribution"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func init() { localCommands["contribution"] = cmdContribution }
func contributionExchange(a *contributionAuthority) (*ContributionExchange, error) {
	if a == nil || a.Policy.Contributions == nil {
		return nil, fmt.Errorf("contributions disabled; explicitly select authorized policy")
	}
	dir, e := contributionSharedDirectory(true)
	if e != nil {
		return nil, e
	}
	return OpenContributionExchange(filepath.Join(dir, "inbox-"+a.Policy.Contributions.Project[7:]), *a.Policy.Contributions, "initial")
}
func contributionSetAuthority(args []string) error {
	flags := quietFlags("contribution authority set")
	ref := flags.String("ref", "", "direct local branch")
	expect := flags.String("expect", "", "absent or exact anchor digest")
	readOnly := flags.Bool("read-only", false, "keyless read-only trust selection")
	_ = flags.Bool("json", false, "JSON")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if flags.NArg() != 0 || (*expect != "absent" && !validEventID(*expect)) {
		return fmt.Errorf("authority set requires --ref and --expect absent|DIGEST")
	}
	commit, p, digest, e := resolveContributionBranch(*ref)
	if e != nil {
		return e
	}
	if !*readOnly {
		identity, e := loadIdentity()
		if e != nil {
			return e
		}
		if !actorListed(identity.Actor, p.Maintainers) {
			return fmt.Errorf("explicit activation requires current native maintainer")
		}
	}
	dir, e := contributionDirectory(true)
	if e != nil {
		return e
	}
	lock, e := os.OpenFile(filepath.Join(dir, "authority.lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer func() { lock.Close(); os.Remove(lock.Name()) }()
	path := filepath.Join(dir, "authority.json")
	old, e := readPrivateFileBounded(path, 4096)
	actual := "absent"
	if e == nil {
		actual = eventID(old)
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if actual != *expect {
		return fmt.Errorf("contribution anchor changed")
	}
	private, e := contributionPrivateGitDirectory()
	if e != nil {
		return e
	}
	a := contributionAuthority{GitDir: private, Ref: *ref, Adopted: commit, ReadOnly: *readOnly, Commit: commit, Policy: p, PolicyDigest: digest}
	if p.Contributions != nil {
		events, e := collectEvents()
		if e != nil {
			return e
		}
		native, e := classifyContributionFacts(contributionContentEvents(events))
		if e != nil {
			return e
		}
		if _, e = c.Reduce(a.current(), nil, native, contributionAuthoritySource{&a}); e != nil {
			return e
		}
		if len(p.Contributions.Baseline) > 0 {
			if e = contributionChargeBaseline(&a, events); e != nil {
				return e
			}
		}
	}
	if e = verifyContributionPolicyHistory(&a); e != nil {
		return e
	}
	if p.Contributions != nil {
		if e = contributionRegisterRetained(&a, "", nil); e != nil {
			return fmt.Errorf("authority adoption requires charged retained snapshot: %w", e)
		}
	}
	raw, _ := json.Marshal(a)
	if e = writePrivateFileAtomic(path, raw); e != nil {
		return e
	}
	a.Digest = eventID(raw)
	if p.Contributions != nil {
		x, e := contributionExchange(&a)
		if e != nil {
			return e
		}
		attempts, e := x.Pending()
		if e != nil {
			return e
		}
		for _, attempt := range attempts {
			if attempt.Status == "pending" || attempt.Status == "conflict" {
				if e = contributionRetainIntake(&a, x, attempt.Intent); e != nil {
					return fmt.Errorf("authority selected; raw restore publication pending: %w", e)
				}
			}
		}
	}
	return contributionPrint(map[string]any{"ref": a.Ref, "commit": a.Commit, "digest": a.Digest, "readOnly": a.ReadOnly, "enabled": p.Contributions != nil})
}

// The finite local baseline uses existing native authentication, including
// governance histories outside the restricted public intake profile. It charges
// its complete retained object/actor closure before activation, never rewrites it.
func contributionChargeBaseline(a *contributionAuthority, events []StoredEvent) error {
	x, e := contributionExchange(a)
	if e != nil {
		return e
	}
	l := a.Policy.Contributions.Limits
	ctx, cancel, e := contributionContext(context.Background(), l)
	if e != nil {
		return e
	}
	defer cancel()
	byID := map[string]StoredEvent{}
	for _, s := range events {
		byID[s.ID] = s
	}
	roots := []string{}
	seen := map[string]bool{}
	var walk func(string) error
	walk = func(id string) error {
		if seen[id] {
			return nil
		}
		if int64(len(seen)) >= l.Events || len(seen) >= c.MaxGraphRecords {
			return fmt.Errorf("baseline native event bound")
		}
		s, ok := byID[id]
		if !ok {
			return c.ErrMissingDependency
		}
		seen[id] = true
		roots = append(roots, s.Commit)
		if isProposalKind(s.Event.Kind) {
			roots = append(roots, s.Event.Base, s.Event.Head)
		}
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
	for _, id := range a.Policy.Contributions.Baseline {
		if e = walk(id); e != nil {
			return e
		}
	}
	if len(roots) == 0 {
		return nil
	}
	ml, e := contributionLimits(l)
	if e != nil {
		return e
	}
	gitdir, e := contributionGitDirectory()
	if e != nil {
		return e
	}
	objects, e := projectObjectIDs(ctx, gitdir, roots, ml)
	if e != nil {
		return e
	}
	depthRaw, e := projectGit(ctx, gitdir, []byte(strings.Join(roots, "\n")+"\n"), 256, "rev-list", "--count", "--stdin")
	if e != nil {
		return e
	}
	depth, e := strconv.ParseInt(strings.TrimSpace(string(depthRaw)), 10, 64)
	if e != nil || depth > l.Depth {
		return fmt.Errorf("baseline retained commit depth bound")
	}
	sizes, e := projectGit(ctx, gitdir, []byte(strings.Join(objects, "\n")+"\n"), int64(len(objects))*128, "cat-file", "--batch-check=%(objectsize)")
	if e != nil {
		return e
	}
	var decoded int64
	for _, line := range strings.Fields(string(sizes)) {
		n, e := strconv.ParseInt(line, 10, 64)
		if e != nil || n < 0 || n > l.DecodedBytes-decoded {
			return fmt.Errorf("baseline retained byte bound")
		}
		decoded += n
	}
	cost := c.Cost{WireBytes: 1, DecodedBytes: max(decoded, 1), Objects: int64(len(objects)), Events: max(int64(len(seen)), 1), Depth: max(depth, 1), Milliseconds: 1}
	material, _ := json.Marshal(struct {
		Project           string
		Baseline, Objects []string
	}{a.Policy.Contributions.Project, a.Policy.Contributions.Baseline, objects})
	digest := c.Digest(material)
	intent := "baseline-" + digest[7:]
	unlock, e := x.lock()
	if e != nil {
		return e
	}
	defer unlock()
	j, e := x.load()
	if e != nil {
		return e
	}
	next, replay, e := c.Reserve(j.Ledger, x.policy, intent, digest, cost)
	if e != nil {
		return e
	}
	if replay {
		for _, attempt := range j.Attempts {
			if attempt.Intent == intent {
				return contributionWriteBaselineReceipt(x, a, attempt)
			}
		}
		return fmt.Errorf("baseline reservation lost matching receipt")
	}
	j.Ledger = next
	accounting, _ := json.Marshal(next)
	j.Attempts = append(j.Attempts, ContributionIntake{Intent: intent, Artifact: digest, Refs: []string{a.Ref}, Status: "failed", Failure: "authenticated local baseline retained; no public artifact", Reserved: cost, DecodedBytes: decoded, Objects: cost.Objects, Events: cost.Events, Depth: cost.Depth, Milliseconds: 1, Accounting: c.Digest(accounting)})
	if e = x.save(j); e != nil {
		return e
	}
	return contributionWriteBaselineReceipt(x, a, j.Attempts[len(j.Attempts)-1])
}

// An exact public retry first binds the original operation/digest/ref selection
// through Receive. Reconcile interrupted intake only from immutable local bytes;
// Recover keeps the original reservation and limits and never reopens the source.
func contributionReceiveArtifact(ctx context.Context, x *ContributionExchange, path, digest, intent string, refs []string) (ContributionIntake, error) {
	intake, e := x.Receive(ctx, path, digest, intent, refs)
	if e != nil || !intake.Replayed || intake.Status != "reserved" && intake.Status != "uncertain" {
		return intake, e
	}
	intake, e = x.Recover(ctx, intake.Intent)
	intake.Replayed = true
	return intake, e
}

// Raw restore can precede authority adoption. The already-present branch policy
// supplies only finite local retention bounds; it grants no signing or project
// authority. Native facts remain staged until explicit selection publishes them.
func contributionRestoreUnselected(args []string) error {
	flags := quietFlags("contribution restore")
	file := flags.String("file", "", "regular local file")
	digest := flags.String("digest", "", "exact file digest")
	operation := flags.String("operation", "", "receiver-local operation")
	var refs issueExpected
	flags.Var(&refs, "ref", "exact native selection")
	_ = flags.Bool("json", false, "JSON")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected restore argument")
	}
	commit, e := resolveCommit("HEAD")
	if e != nil {
		return e
	}
	p, _, pd, e := loadPolicy(commit)
	if e != nil || p.Contributions == nil {
		return fmt.Errorf("raw restore needs independently obtained branch policy for finite project retention bounds; no authority is adopted")
	}
	a := &contributionAuthority{Commit: commit, Policy: p, PolicyDigest: pd, ReadOnly: true}
	x, e := contributionExchange(a)
	if e != nil {
		return e
	}
	if e = contributionPublicInput(x, *digest, *operation); e != nil {
		return e
	}
	intake, e := contributionReceiveArtifact(context.Background(), x, *file, *digest, *operation, refs)
	if e != nil {
		return e
	}
	return contributionPrint(map[string]any{"intake": intake, "raw": true, "authorityAdopted": false, "nativePublished": false})
}
func cmdContribution(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: hn contribution authority|prepare|receive|pending|inspect|decide|archive|restore|baseline|retained|intake|rollover")
	}
	if args[0] == "authority" {
		if len(args) < 2 {
			return fmt.Errorf("authority set|show")
		}
		if args[1] == "set" {
			return contributionSetAuthority(args[2:])
		}
		if args[1] == "show" {
			a, e := loadContributionAuthority()
			if e != nil {
				return e
			}
			if a == nil {
				return fmt.Errorf("no contribution authority selected")
			}
			return contributionPrint(map[string]any{"ref": a.Ref, "commit": a.Commit, "digest": a.Digest, "policyDigest": a.PolicyDigest, "readOnly": a.ReadOnly, "enabled": a.Policy.Contributions != nil})
		}
		return fmt.Errorf("unknown authority command")
	}
	if args[0] == "prepare" {
		return contributionPrepare(args[1:])
	}
	if args[0] == "inspect" {
		for _, arg := range args[1:] {
			if arg == "--raw" {
				return contributionInspectRaw(args[1:])
			}
		}
	}
	if args[0] == "decide" {
		return contributionDecide(args[1:])
	}
	a, e := loadContributionAuthority()
	if e != nil {
		return e
	}
	if a == nil && args[0] == "restore" {
		return contributionRestoreUnselected(args[1:])
	}
	if a == nil || a.Policy.Contributions == nil {
		return fmt.Errorf("contributions disabled or no authority selected")
	}
	x, e := contributionExchange(a)
	if e != nil {
		return e
	}
	switch args[0] {
	case "receive", "restore":
		flags := quietFlags("contribution receive")
		file := flags.String("file", "", "local regular file")
		digest := flags.String("digest", "", "exact file digest")
		operation := flags.String("operation", "", "durable operation intent")
		var refs issueExpected
		flags.Var(&refs, "ref", "exact selected native ref (repeat)")
		_ = flags.Bool("json", false, "JSON")
		if e = flags.Parse(args[1:]); e != nil {
			return e
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("unexpected contribution receive argument")
		}
		if e = contributionPublicInput(x, *digest, *operation); e != nil {
			return e
		}
		intake, e := contributionReceiveArtifact(context.Background(), x, *file, *digest, *operation, refs)
		if e != nil {
			if intake.Status == "conflict" {
				if retainErr := contributionRetainIntake(a, x, intake.Intent); retainErr != nil {
					return fmt.Errorf("%v; conflict raw retention: %w", e, retainErr)
				}
			}
			return e
		}
		if intake.Status != "pending" && intake.Status != "conflict" {
			return contributionPrint(intake)
		}
		if e = contributionRetainIntake(a, x, intake.Intent); e != nil {
			return e
		}
		if args[0] == "receive" && !a.ReadOnly {
			if e = contributionReceiptIntake(a, x, intake.Intent); e != nil {
				return fmt.Errorf("raw intake retained; admission pending: %w", e)
			}
		}
		return contributionPrint(intake)
	case "retained":
		if len(args) < 2 {
			return fmt.Errorf("retained show|register")
		}
		flags := quietFlags("contribution retained")
		expected := flags.String("expect", "", "exact inspected snapshot digest")
		var revisions issueExpected
		flags.Var(&revisions, "revision", "explicit raw native revision to register (repeat)")
		_ = flags.Bool("json", false, "JSON")
		if e = flags.Parse(args[2:]); e != nil {
			return e
		}
		for _, id := range revisions {
			if !validEventID(id) {
				return fmt.Errorf("exact retained native revision required")
			}
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("unexpected retained argument")
		}
		plan, e := contributionPlanRetained(a, revisions)
		if e != nil {
			return e
		}
		if args[1] == "show" {
			return contributionPrint(map[string]any{"digest": plan.Digest, "plan": plan})
		}
		if args[1] != "register" {
			return fmt.Errorf("retained show|register")
		}
		if *expected != "" && *expected != plan.Digest {
			return fmt.Errorf("retained snapshot changed")
		}
		if e = contributionRegisterRetained(a, *expected, revisions); e != nil {
			return e
		}
		return contributionPrint(map[string]any{"registered": plan.Digest, "native": len(plan.Native), "heads": plan.Heads})
	case "intake":
		flags := quietFlags("contribution intake")
		intent := flags.String("operation", "", "exact receiver-local operation")
		_ = flags.Bool("json", false, "JSON")
		if e = flags.Parse(args[1:]); e != nil {
			return e
		}
		if *intent == "" || flags.NArg() != 0 {
			return fmt.Errorf("intake inspection requires --operation")
		}
		ledger, attempts, e := x.Accounting()
		if e != nil {
			return e
		}
		for _, attempt := range attempts {
			if attempt.Intent != *intent {
				continue
			}
			plan, e := contributionLoadLocalPlan(a, x, *intent)
			if e != nil {
				return e
			}
			var descriptor any
			if plan != nil {
				descriptor = plan.descriptor
			}
			return contributionPrint(map[string]any{"receiverLocal": true, "attempt": attempt, "ledger": ledger, "descriptor": descriptor, "deadlineSemantics": "bounded synchronous native validation; cancellation checked at boundaries before promotion; no background work"})
		}
		return fmt.Errorf("receiver-local operation unavailable")
	case "pending":
		events, e := collectEvents()
		if e != nil {
			return e
		}
		view, _, e := contributionView(events)
		if e != nil {
			return e
		}
		return contributionPrint(view)
	case "inspect":
		flags := quietFlags("contribution inspect")
		raw := flags.Bool("raw", false, "explicit immutable raw inspection")
		_ = flags.Bool("json", false, "JSON")
		inspectArgs := []string{}
		for _, arg := range args[1:] {
			if arg == "--json" || arg == "--raw" {
				inspectArgs = append([]string{arg}, inspectArgs...)
			} else {
				inspectArgs = append(inspectArgs, arg)
			}
		}
		if e = flags.Parse(inspectArgs); e != nil {
			return e
		}
		events, e := collectEvents()
		if e != nil {
			return e
		}
		if *raw {
			if flags.NArg() != 1 {
				return fmt.Errorf("raw inspection requires exact native revision or contribution record")
			}
			id := flags.Arg(0)
			if !validEventID(id) {
				return fmt.Errorf("exact raw ID required")
			}
			for _, s := range events {
				if s.ID == id {
					return contributionPrint(map[string]any{"raw": true, "id": id, "payload": json.RawMessage(s.Payload), "signature": s.Signature})
				}
			}
			records, e := contributionRecords(a)
			if e != nil {
				return e
			}
			for _, r := range records {
				if r.ID() == id {
					return contributionPrint(map[string]any{"raw": true, "id": id, "payload": json.RawMessage(r.Payload()), "signature": r.Signature()})
				}
			}
			return fmt.Errorf("raw record unavailable")
		}
		v, _, e := contributionView(events)
		if e != nil {
			return e
		}
		if flags.NArg() == 0 {
			records, e := contributionRecords(a)
			if e != nil {
				return e
			}
			rows := []any{}
			for _, r := range records {
				rows = append(rows, map[string]any{"id": r.ID(), "record": r.Record()})
			}
			return contributionPrint(map[string]any{"view": contributionViewDocument(v), "records": rows})
		}
		records, e := contributionRecords(a)
		if e != nil {
			return e
		}
		for _, r := range records {
			if r.ID() == flags.Arg(0) {
				return contributionPrint(map[string]any{"id": r.ID(), "record": r.Record(), "view": contributionViewDocument(v)})
			}
		}
		return fmt.Errorf("submission unavailable")
	case "archive":
		flags := quietFlags("contribution archive")
		output := flags.String("output", "", "outer contribution artifact; native governance history must be backed up separately")
		var refs issueExpected
		flags.Var(&refs, "ref", "exact native ref; repeat")
		_ = flags.Bool("json", false, "JSON")
		if e = flags.Parse(args[1:]); e != nil {
			return e
		}
		records, e := contributionRecords(a)
		if e != nil {
			return e
		}
		if len(refs) == 0 {
			refs, e = contributionRefsForRecords(records, a.Policy.Contributions.Baseline)
			if e != nil {
				return e
			}
		}
		envs := contributionEnvelopes(records)
		gitdir, e := contributionGitDirectory()
		if e != nil {
			return e
		}
		digest, e := ExportContributionArtifact(context.Background(), gitdir, a.Policy.Contributions.Project, refs, envs, *output, a.Policy.Contributions.Limits)
		if e != nil {
			return fmt.Errorf("outer contribution archive refuses unsupported signed history; retain full native governance via explicit native backup: %w", e)
		}
		return contributionPrint(map[string]any{"digest": digest, "refs": refs, "raw": true, "requiresIndependentGovernanceHistory": true})
	case "baseline":
		flags := quietFlags("contribution baseline")
		expected := flags.String("expect-policy", "", "exact current policy digest")
		if e = flags.Parse(args[1:]); e != nil {
			return e
		}
		if *expected != a.PolicyDigest || flags.NArg() != 0 {
			return fmt.Errorf("baseline requires exact current --expect-policy")
		}
		if !a.ReadOnly {
			identity, e := loadIdentity()
			if e != nil {
				return e
			}
			if !actorListed(identity.Actor, a.Policy.Maintainers) {
				return c.ErrAuthority
			}
		}
		events, e := collectEvents()
		if e != nil {
			return e
		}
		native, e := classifyContributionFacts(contributionContentEvents(events))
		if e != nil {
			return e
		}
		if _, e = c.Reduce(a.current(), nil, native, contributionAuthoritySource{a}); e != nil {
			return e
		}
		return contributionChargeBaseline(a, events)
	case "rollover":
		flags := quietFlags("contribution rollover")
		expected := flags.String("expect", "", "old window")
		next := flags.String("window", "", "new explicit window")
		if e = flags.Parse(args[1:]); e != nil {
			return e
		}
		id, e := loadIdentity()
		if e != nil {
			return e
		}
		if a.ReadOnly || !actorListed(id.Actor, a.Policy.Maintainers) {
			return c.ErrAuthority
		}
		return x.Rollover(*expected, *next, true)
	}
	return fmt.Errorf("unknown contribution command %q", args[0])
}
func contributionEnvelopes(records []c.Verified) []c.Envelope {
	envs := []c.Envelope{}
	for _, r := range records {
		envs = append(envs, c.Envelope{Payload: r.Payload(), Signature: r.Signature()})
	}
	return envs
}
func contributionPrepare(args []string) error {
	flags := quietFlags("contribution prepare")
	revision := flags.String("revision", "", "exact signed revision")
	project := flags.String("project", "", "explicit project scope if no local authority")
	output := flags.String("output", "", "local artifact")
	intent := flags.String("intent", "", "actor-local submission intent")
	var refs issueExpected
	flags.Var(&refs, "ref", "exact native closure selection")
	_ = flags.Bool("json", false, "JSON")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if !validEventID(*revision) || *output == "" || flags.NArg() != 0 {
		return fmt.Errorf("prepare requires exact --revision and --output")
	}
	a, e := loadContributionAuthority()
	if e != nil {
		return e
	}
	limits := c.DefaultLimits()
	if a != nil && a.Policy.Contributions != nil {
		if *project != "" && *project != a.Policy.Contributions.Project {
			return fmt.Errorf("project differs from selected authority")
		}
		*project = a.Policy.Contributions.Project
		limits = a.Policy.Contributions.Limits
	}
	id, e := loadIdentity()
	if e != nil {
		return e
	}
	events, e := collectEvents()
	if e != nil {
		return e
	}
	native, e := classifyContributionFacts(contributionContentEvents(events))
	if e != nil {
		return e
	}
	f, e := native.Fact(*revision)
	if e != nil {
		return e
	}
	if f.Actor != id.Actor {
		return fmt.Errorf("only the original native signer may submit this revision")
	}
	if *intent == "" {
		*intent = "submit-" + (*revision)[7:]
	}
	env, e := signContribution(id, c.Record{Kind: c.SubmissionKind, Project: *project, Intent: *intent, Submission: &c.Submission{Operation: f.Operation, Content: f.Content}})
	if e != nil {
		return e
	}
	if len(refs) == 0 {
		v, err := c.Verify(env.Payload, env.Signature)
		if err != nil {
			return err
		}
		refs, e = contributionRefsForRecords([]c.Verified{v}, nil)
		if e != nil {
			return e
		}
	}
	gitdir, e := contributionGitDirectory()
	if e != nil {
		return e
	}
	digest, e := ExportContributionArtifact(context.Background(), gitdir, *project, refs, []c.Envelope{env}, *output, limits)
	if e != nil {
		return e
	}
	return contributionPrint(map[string]any{"digest": digest, "refs": refs, "submission": c.Digest(env.Payload), "revision": *revision})
}
func signContribution(id *Identity, r c.Record) (c.Envelope, error) {
	r.Version = c.Version
	r.Actor = id.Actor
	r.PublicKey = id.PublicKey
	key, e := id.privateKey()
	if e != nil {
		return c.Envelope{}, e
	}
	return c.Sign(r, key)
}

// Raw native promotion is create-only in an artifact-specific namespace. Its
// complete deterministic target/OID plan derives from the durable intake's exact
// digest/selection. Replay cannot overwrite another observed actor branch.
func contributionRetainIntake(a *contributionAuthority, x *ContributionExchange, intent string) error {
	plan, e := contributionLoadLocalPlan(a, x, intent)
	if e != nil {
		return e
	}
	if plan != nil {
		events, e := collectEvents()
		if e != nil {
			return e
		}
		for i := range events {
			if events[i].ID == plan.descriptor.Revision {
				return contributionRetryNative(&events[i])
			}
		}
		return fmt.Errorf("reserved local native result not yet appended")
	}
	artifact, e := x.Inspect(context.Background(), intent)
	if e != nil {
		return e
	}
	defer artifact.Close()
	if e = contributionCoverArtifact(a, x, artifact); e != nil {
		return e
	}
	gitdir, e := contributionGitDirectory()
	if e != nil {
		return e
	}
	ml, e := contributionLimits(a.Policy.Contributions.Limits)
	if e != nil {
		return e
	}
	if e = projectCopyObjectIDs(context.Background(), artifact.Quarantine.GitDir, gitdir, artifact.Quarantine.ObjectIDs(), ml); e != nil {
		return e
	}
	promotions := []replicationPromotion{}
	remote := "contribution-" + artifact.Intake.Artifact[7:]
	for _, ref := range artifact.Refs {
		target := ""
		sourceRemote := remote + "-" + c.Digest([]byte(ref.SourceRef))[7:23]
		switch ref.Kind {
		case "actor":
			target = acceptedActorRef(sourceRemote, projectActorOwner(ref.SourceRef))
		case "proposal":
			id := "sha256:" + strings.TrimPrefix(ref.SourceRef, "refs/hn/proposals/")
			if _, observed, ok := parseAcceptedProposalRef(ref.SourceRef); ok {
				id = observed
			}
			target = acceptedProposalRef(sourceRemote, id)
		case "code":
			continue
		default:
			return fmt.Errorf("unsupported raw contribution ref")
		}
		old, exists, e := refValue(target)
		if e != nil {
			return e
		}
		if exists {
			if old != ref.OID {
				return fmt.Errorf("raw contribution target conflict")
			}
			continue
		}
		promotions = append(promotions, replicationPromotion{Ref: target, NewOID: ref.OID})
	}
	if e = contributionPromoteRawRefs(promotions); e != nil {
		return e
	}
	actors := map[string]bool{}
	for _, record := range artifact.Records {
		actor := record.Record().Actor
		if actors[actor] {
			continue
		}
		actors[actor] = true
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
		for _, plan := range artifact.Intake.ManifestWrites {
			if plan.Actor == actor {
				old = plan.Expected
				break
			}
		}
		if _, e = x.RetainActor(context.Background(), gitdir, intent, actor, old); e != nil {
			return e
		}
	}
	return nil
}

func contributionWriteBaselineReceipt(x *ContributionExchange, a *contributionAuthority, attempt ContributionIntake) error {
	receipt := contributionBaselineReceipt{Baseline: contributionBaselineID(a), Artifact: attempt.Artifact, Intent: attempt.Intent, Accounting: attempt.Accounting}
	raw, e := json.Marshal(receipt)
	if e != nil {
		return e
	}
	return writePrivateFileAtomic(filepath.Join(x.root, "baseline-"+receipt.Baseline[7:]+".json"), raw)
}

// Explicit raw inspection is independent of current policy validity, selected
// authority and admission. It preserves exact signatures and never installs trust.
func contributionInspectRaw(args []string) error {
	ordered := []string{}
	for _, arg := range args {
		if arg == "--raw" || arg == "--json" {
			ordered = append([]string{arg}, ordered...)
		} else {
			ordered = append(ordered, arg)
		}
	}
	flags := quietFlags("contribution inspect --raw")
	_ = flags.Bool("raw", false, "raw signed history")
	_ = flags.Bool("json", false, "JSON")
	project := flags.String("project", "", "exact project scope for raw envelope inspection")
	if e := flags.Parse(ordered); e != nil {
		return e
	}
	if flags.NArg() != 1 || !validEventID(flags.Arg(0)) {
		return fmt.Errorf("raw inspection requires one exact signed record ID")
	}
	id := flags.Arg(0)
	events, e := collectEvents()
	if e != nil {
		return e
	}
	for _, s := range events {
		if s.ID == id {
			return contributionPrint(map[string]any{"raw": true, "id": id, "payload": json.RawMessage(s.Payload), "signature": s.Signature})
		}
	}
	if *project == "" {
		p, _, _, err := loadPolicy("HEAD")
		if err == nil && p.Contributions != nil {
			*project = p.Contributions.Project
		}
	}
	if !validEventID(*project) {
		return fmt.Errorf("raw envelope inspection requires --project sha256:FULL_SCOPE")
	}
	dir, e := contributionGitDirectory()
	if e != nil {
		return e
	}
	ctx, cancel, e := contributionContext(context.Background(), c.DefaultLimits())
	if e != nil {
		return e
	}
	defer cancel()
	raw, e := projectGit(ctx, dir, nil, c.MaxGraphRecords*256, "for-each-ref", "--format=%(refname)", "refs/hn/contributions/"+(*project)[7:]+"/")
	if e != nil {
		return e
	}
	for _, ref := range strings.Fields(string(raw)) {
		actor := ref[strings.LastIndex(ref, "/")+1:]
		records, _, e := ReadContributionManifest(ctx, dir, *project, actor, c.DefaultLimits())
		if e != nil {
			return e
		}
		for _, r := range records {
			if r.ID() == id {
				return contributionPrint(map[string]any{"raw": true, "id": id, "payload": json.RawMessage(r.Payload()), "signature": r.Signature()})
			}
		}
	}
	return fmt.Errorf("raw signed record unavailable")
}
