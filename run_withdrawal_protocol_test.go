package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"hubnot/internal/migration"
)

func withdrawalEvent(t *testing.T, actor *Identity, request *StoredEvent, reason string) Event {
	t.Helper()
	e, err := nextEvent(actor, "run.withdraw")
	if err != nil {
		t.Fatal(err)
	}
	e.Subject, e.Body = request.ID, reason
	return e
}
func appendWithdrawal(t *testing.T, actor *Identity, request *StoredEvent, reason string) *StoredEvent {
	t.Helper()
	s, err := appendEvent(withdrawalEvent(t, actor, request, reason), actor)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func signWithdrawalBytes(t *testing.T, actor *Identity, payload []byte) []byte {
	t.Helper()
	key, err := actor.privateKey()
	if err != nil {
		t.Fatal(err)
	}
	return ed25519.Sign(key, payload)
}

func TestRunWithdrawalShape(t *testing.T) {
	f := newEfficiencyFixture(t, false)
	r := f.request(f.author, f.proposal, "test", "")
	valid := withdrawalEvent(t, f.author, r, strings.Repeat("é", 2048))
	raw, sig, err := encodeAndSign(valid, f.author)
	if err != nil {
		t.Fatalf("bounded valid withdrawal refused: %v", err)
	}
	if _, _, err = verifyEvent(raw, sig); err != nil {
		t.Fatal(err)
	}
	for _, subject := range []string{"", shortID(r.ID), f.head} {
		e := valid
		e.Subject = subject
		if _, _, err := encodeAndSign(e, f.author); err == nil {
			t.Fatalf("invalid full request ID accepted: %q", subject)
		}
	}
	for _, reason := range []string{"", " \n\t", " padded", "padded ", strings.Repeat("é", 2049), string([]byte{0xff})} {
		e := valid
		e.Body = reason
		if _, _, err := encodeAndSign(e, f.author); err == nil {
			t.Fatalf("invalid reason accepted: %q", reason)
		}
	}
	for _, field := range []string{"pipeline", "head", "exitCode", "evidence", "audit", "unknown", "Kind", "Subject"} {
		for _, value := range []string{"null", `""`, "0", "[]"} {
			hostile := append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`,"`+field+`":`+value+`}`)...)
			if _, _, err := verifyEvent(hostile, signWithdrawalBytes(t, f.author, hostile)); err == nil {
				t.Fatalf("extraneous %s=%s accepted", field, value)
			}
		}
	}
	for _, suffix := range []string{`,"body":"replacement"}`, `,"kind":"run.withdraw"}`, `,"subject":"` + r.ID + `"}`, `,"bo\u0064y":"replacement"}`} {
		hostile := append(append([]byte(nil), raw[:len(raw)-1]...), []byte(suffix)...)
		if _, _, err := verifyEvent(hostile, signWithdrawalBytes(t, f.author, hostile)); err == nil {
			t.Fatalf("duplicate accepted: %s", suffix)
		}
	}
	// New-kind strictness must not reinterpret legacy payloads.
	legacy := newEvent(testIdentity(t, "Legacy"), "issue.open", 1, "")
	legacy.Title = "legacy"
	actor := testIdentity(t, "Legacy bytes")
	legacy.Actor, legacy.PublicKey = actor.Actor, actor.PublicKey
	old, _ := json.Marshal(legacy)
	old = append(old[:len(old)-1], []byte(`,"futureLegacyField":null}`)...)
	if _, _, err := verifyEvent(old, signWithdrawalBytes(t, actor, old)); err != nil {
		t.Fatalf("legacy semantics changed: %v", err)
	}
}

func TestRunWithdrawalLocalRefusalBeforeMutation(t *testing.T) {
	for _, kind := range []string{"wrong-actor", "wrong-key", "wrong-kind", "missing", "ordering", "duplicate", "conflict"} {
		t.Run(kind, func(t *testing.T) {
			f := newEfficiencyFixture(t, false)
			r := f.request(f.author, f.proposal, "test", "")
			signer := f.author
			if kind == "wrong-actor" {
				signer = f.trustedA
			}
			e := withdrawalEvent(t, signer, r, "No longer needed")
			switch kind {
			case "wrong-key":
				e.PublicKey = f.trustedA.PublicKey
			case "wrong-kind":
				e.Subject = f.proposal.ID
			case "missing":
				e.Subject = eventID([]byte("absent request"))
			case "ordering":
				e.Sequence = r.Event.Sequence
			case "duplicate", "conflict":
				appendWithdrawal(t, f.author, r, "No longer needed")
				e = withdrawalEvent(t, signer, r, "No longer needed")
				if kind == "conflict" {
					e.Body = "Changed intent"
				}
			}
			before := f.snapshot()
			expected := map[string]string{"wrong-actor": "original requester", "wrong-key": "original requester", "wrong-kind": "available run request", "missing": "available run request", "ordering": "follow its original request", "duplicate": "already has a withdrawal", "conflict": "already has a withdrawal"}[kind]
			_, refusal := appendEvent(e, signer)
			if refusal == nil || !strings.Contains(refusal.Error(), expected) {
				t.Fatalf("%s refusal = %v, want %q", kind, refusal, expected)
			}
			// An unusable signing key must not hide the same relationship refusal:
			// authorization/dependency/duplicate checks happen before signing.
			unavailable := *signer
			unavailable.PrivateKey = "unavailable"
			if _, err := appendEvent(e, &unavailable); err == nil || err.Error() != refusal.Error() {
				t.Fatalf("%s reached signing before refusal: %v", kind, err)
			}
			if got := f.snapshot(); !reflect.DeepEqual(before, got) {
				t.Fatalf("%s refusal changed refs/objects/signer", kind)
			}
			raw, _ := os.ReadFile(f.marker)
			if len(raw) != 0 {
				t.Fatal("refusal executed backend")
			}
			// Same repository and untouched marker prove a genuine eligible run follows refusal.
			if kind == "duplicate" || kind == "conflict" {
				f.request(f.author, f.proposal, "test", "")
			}
			f.execute(f.options())
		})
	}
}

func TestRunWithdrawalAutomaticAndHistoricalFacts(t *testing.T) {
	f := newEfficiencyFixture(t, true)
	r := f.request(f.author, f.proposal, "test", "")
	// These signed results existed before withdrawal.
	f.result(f.trustedA, r, "passed")
	f.result(f.trustedB, r, "passed")
	facts, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	var evidence []string
	for _, s := range facts {
		if s.Event.Kind == "run.result" {
			evidence = append(evidence, s.ID)
		}
	}
	_, _, digest, err := loadPolicy(f.base)
	if err != nil {
		t.Fatal(err)
	}
	decision := appendRevisionDecision(t, f.author, f.proposal.ID, digest, evidence)
	appendRevisionMerge(t, f.author, f.proposal, digest, decision)
	before, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	w := appendWithdrawal(t, f.author, r, "Work already started elsewhere")
	// A disconnected runner publishes after withdrawal. Publication remains valid.
	f.result(f.trustedA, r, "passed")
	after, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]StoredEvent{}
	for _, s := range after {
		byID[s.ID] = s
	}
	for _, s := range before {
		got := byID[s.ID]
		if !bytes.Equal(s.Payload, got.Payload) || !bytes.Equal(s.Signature, got.Signature) {
			t.Fatal("historical bytes changed")
		}
	}
	if byID[w.ID].Event.Kind != "run.withdraw" {
		t.Fatal("withdrawal absent")
	}
	policy, _, _, err := loadPolicy(f.base)
	if err != nil {
		t.Fatal(err)
	}
	eval, required := evaluatePipelineResults(f.proposal.ID, "test", r.Event.Definition, after, policy)
	if !required || len(eval.Passed) != 2 {
		t.Fatalf("withdrawal invalidated historical/late evidence: %#v", eval)
	}
	// Use another unmerged candidate to isolate observed-withdrawal scheduling.
	p := appendPolicyTestProposal(t, f.author, f.base, f.head, "Withdraw queued work")
	queued := f.request(f.author, p, "test", "")
	appendWithdrawal(t, f.author, queued, "No future starts")
	f.skip(f.options(), "withdrawn")
	fresh := appendPolicyTestProposal(t, f.author, f.base, f.head, "Positive execution")
	f.request(f.author, fresh, "test", "")
	f.execute(f.options())
}

func TestRunWithdrawalGitAndArchiveRoundTrip(t *testing.T) {
	f := newEfficiencyFixture(t, false)
	r := f.request(f.author, f.proposal, "test", "")
	w := appendWithdrawal(t, f.author, r, "Portable cancellation")
	if err := cmdSync([]string{"origin"}); err != nil {
		t.Fatal(err)
	}
	limits := migration.DefaultLimits()
	ctx := context.Background()
	refs := []string{actorRef(f.author.Actor), proposalRef(f.proposal.ID)}
	p, err := ExportProjectSnapshot(ctx, filepath.Join(f.root, ".git"), refs, limits)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	var archive bytes.Buffer
	if err := migration.WriteArchive(ctx, &archive, p.Snapshot, p, limits); err != nil {
		t.Fatal(err)
	}
	a, err := migration.ReadArchive(ctx, bytes.NewReader(archive.Bytes()), limits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	q, err := StageProjectSnapshot(ctx, a.Snapshot, a, limits)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	got, err := loadStoredEventAt(q.GitDir, w.Commit)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != w.ID || !bytes.Equal(got.Payload, w.Payload) || !bytes.Equal(got.Signature, w.Signature) {
		t.Fatal("archive changed signed withdrawal")
	}
	receiver := filepath.Join(t.TempDir(), "receiver")
	mustGit(t, "clone", "-q", f.remote, receiver)
	withTestDirectory(t, receiver)
	if err := cmdReplication([]string{"select", "origin", "--actor", f.author.Actor, "--proposal", f.proposal.ID}); err != nil {
		t.Fatal(err)
	}
	if err := cmdSync([]string{"origin"}); err != nil {
		t.Fatal(err)
	}
	events, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range events {
		if s.ID == w.ID {
			found = bytes.Equal(s.Payload, w.Payload) && bytes.Equal(s.Signature, w.Signature)
		}
	}
	if !found {
		t.Fatal("selected Git replication lost withdrawal")
	}
}

func TestRunWithdrawalMissingDependencyRecovery(t *testing.T) {
	f := newEfficiencyFixture(t, false)
	r := f.request(f.author, f.proposal, "test", "")
	e := withdrawalEvent(t, f.author, r, "Missing dependency")
	e.Subject = eventID([]byte("unavailable request"))
	parent := mustGitText(t, "rev-parse", actorRef(f.author.Actor))
	appendRawSignedEventCommit(t, f.author, e, parent, nil)
	// A real shallow repository reports a recoverable exact request gap.
	shallow := filepath.Join(f.root, ".git", "shallow")
	if err := os.WriteFile(shallow, []byte(mustGitText(t, "rev-parse", "HEAD")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before := f.snapshot()
	err := guardShallowEventClosure("withdrawal recovery")
	var gap *ShallowDependencyGap
	if !errors.As(err, &gap) || gap.Kind != shallowRunRequest || gap.MissingID != e.Subject {
		t.Fatalf("missing withdrawal request not classified: %v", err)
	}
	if got := f.snapshot(); !reflect.DeepEqual(before, got) {
		t.Fatal("recovery classification mutated facts")
	}
	if _, err := collectEvents(); err == nil {
		t.Fatal("orphan withdrawal collected")
	}
}

func TestRunWithdrawalRawExtraneousFieldRefusal(t *testing.T) {
	f := newEfficiencyFixture(t, false)
	r := f.request(f.author, f.proposal, "test", "")
	e := withdrawalEvent(t, f.author, r, "strict")
	raw, _ := json.Marshal(e)
	raw = append(raw[:len(raw)-1], []byte(`,"pipeline":""}`)...)
	sig := signWithdrawalBytes(t, f.author, raw)
	parent := mustGitText(t, "rev-parse", actorRef(f.author.Actor))
	appendRawSignedEventCommit(t, f.author, e, parent, map[string][]byte{"event.json": raw, "signature": []byte(base64.RawStdEncoding.EncodeToString(sig))})
	if _, err := collectEvents(); err == nil {
		t.Fatal("local collection accepted raw extraneous field")
	}
}

// Make hostile object material through ordinary Git reads, without invoking the
// exporter (whose validation must itself refuse it). The archive reader and
// staging boundary still receive real signed Git objects and verify everything.
func withdrawalArchiveMaterial(t *testing.T, snapshot migration.Snapshot) (migration.Snapshot, projectTestBlobs) {
	t.Helper()
	var roots []string
	for _, ref := range snapshot.NativeRefs {
		roots = append(roots, ref.OID)
	}
	gitDir := mustGitText(t, "rev-parse", "--absolute-git-dir")
	ids, err := projectObjectIDs(context.Background(), gitDir, roots, migration.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	raw.WriteString(projectGitSchema)
	for _, id := range ids {
		kind := mustGitText(t, "cat-file", "-t", id)
		data, err := gitOutput("cat-file", kind, id)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&raw, "%s %s %d\n", id, kind, len(data))
		raw.Write(data)
		raw.WriteByte('\n')
	}
	data := raw.Bytes()
	digest := migration.Digest(data)
	snapshot.GitBundle = digest
	snapshot.Blobs = []migration.Blob{{Digest: digest, Size: int64(len(data)), MediaType: "application/vnd.hn.git-objects", Name: "hostile native facts"}}
	return snapshot, projectTestBlobs{digest: data}
}

func TestRunWithdrawalRefusesHostileGitAndArchiveBeforePromotion(t *testing.T) {
	for _, kind := range []string{"wrong-actor", "wrong-key", "wrong-kind", "missing", "ordering", "duplicate", "conflict", "raw-unknown", "raw-duplicate", "raw-zero", "attachment"} {
		t.Run(kind, func(t *testing.T) {
			f := newEfficiencyFixture(t, false)
			r := f.request(f.author, f.proposal, "test", "")
			if kind == "duplicate" || kind == "conflict" {
				appendWithdrawal(t, f.author, r, "Original reason")
			}
			if err := cmdSync([]string{"origin"}); err != nil {
				t.Fatal(err)
			}
			// Save a genuine independently validated archive before hostile advancement.
			ctx := context.Background()
			limits := migration.DefaultLimits()
			refs := []string{actorRef(f.author.Actor), proposalRef(f.proposal.ID)}
			good, err := ExportProjectSnapshot(ctx, filepath.Join(f.root, ".git"), refs, limits)
			if err != nil {
				t.Fatal(err)
			}
			defer good.Close()
			receiver := filepath.Join(t.TempDir(), "receiver")
			mustGit(t, "clone", "-q", f.remote, receiver)
			withTestDirectory(t, receiver)
			worker, keyPath, err := createIdentity("Receiving worker")
			if err != nil {
				t.Fatal(err)
			}
			f.worker, f.keyPath = worker, keyPath
			// The unrelated actor is absent until it attempts an unauthorized withdrawal;
			// initial valid import selects only the actual supplier.
			if err := cmdReplication([]string{"select", "origin", "--actor", f.author.Actor, "--proposal", f.proposal.ID}); err != nil {
				t.Fatal(err)
			}
			if err := cmdSync([]string{"origin"}); err != nil {
				t.Fatal(err)
			}
			accepted := mustGitText(t, "rev-parse", acceptedActorRef("origin", f.author.Actor))
			withTestDirectory(t, f.root)
			signer := f.author
			if kind == "wrong-actor" {
				signer = f.trustedA
			}
			e := withdrawalEvent(t, signer, r, "Original reason")
			switch kind {
			case "wrong-key":
				e.PublicKey = f.trustedA.PublicKey
			case "wrong-kind":
				e.Subject = f.proposal.ID
			case "missing":
				e.Subject = eventID([]byte("request not supplied"))
			case "ordering":
				e.Sequence = r.Event.Sequence
			case "conflict":
				e.Body = "Conflicting intent"
			}
			parent := ""
			if oid, exists, err := refValue(actorRef(signer.Actor)); err != nil {
				t.Fatal(err)
			} else if exists {
				parent = oid
			}
			// Sign hostile bytes directly; bypass the writer only to model a malicious
			// remote, never to bypass any receiving validator.
			raw, _ := json.Marshal(e)
			switch kind {
			case "raw-unknown":
				raw = append(raw[:len(raw)-1], []byte(`,"unknown":true}`)...)
			case "raw-duplicate":
				raw = append(raw[:len(raw)-1], []byte(`,"body":"Original reason"}`)...)
			case "raw-zero":
				raw = append(raw[:len(raw)-1], []byte(`,"exitCode":0}`)...)
			}
			sig := signWithdrawalBytes(t, signer, raw)
			extra := map[string][]byte{"event.json": raw, "signature": []byte(base64.RawStdEncoding.EncodeToString(sig))}
			if kind == "attachment" {
				extra["untrusted.txt"] = []byte("must remain quarantined")
			}
			// The helper encodes a permissible seed; overridden bytes are the attack.
			seed := e
			seed.PublicKey = signer.PublicKey
			bad := appendRawSignedEventCommit(t, signer, seed, parent, extra)
			bad.ID = eventID(raw)
			mustGit(t, "push", "-q", "origin", actorRef(signer.Actor)+":"+actorRef(signer.Actor))
			if _, err := collectEvents(); err == nil {
				t.Fatalf("local collection accepted hostile %s", kind)
			}
			hostile := good.Snapshot
			hostile.NativeRefs = append([]migration.NativeRef(nil), good.Snapshot.NativeRefs...)
			found := false
			for i := range hostile.NativeRefs {
				if hostile.NativeRefs[i].SourceRef == actorRef(signer.Actor) {
					hostile.NativeRefs[i].OID = bad.Commit
					found = true
				}
			}
			if !found {
				hostile.NativeRefs = append(hostile.NativeRefs, migration.NativeRef{SourceRef: actorRef(signer.Actor), OID: bad.Commit, Kind: migration.NativeKind(actorRef(signer.Actor))})
				refs = append(refs, actorRef(signer.Actor))
			}
			hostile, blobs := withdrawalArchiveMaterial(t, hostile)
			beforeSource := f.snapshot()
			if p, err := ExportProjectSnapshot(ctx, filepath.Join(f.root, ".git"), refs, limits); err == nil {
				p.Close()
				t.Fatal("export accepted hostile withdrawal")
			}
			if got := f.snapshot(); !reflect.DeepEqual(beforeSource, got) {
				t.Fatal("export refusal changed source refs/objects/signer")
			}
			// Round-trip the malicious envelope through the real archive reader first.
			var archive bytes.Buffer
			if err := migration.WriteArchive(ctx, &archive, hostile, blobs, limits); err != nil {
				t.Fatal(err)
			}
			a, err := migration.ReadArchive(ctx, bytes.NewReader(archive.Bytes()), limits)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			if q, err := StageProjectSnapshot(ctx, a.Snapshot, a, limits); err == nil {
				q.Close()
				t.Fatal("archive staging accepted hostile withdrawal")
			}
			withTestDirectory(t, receiver)
			if kind == "wrong-actor" {
				if err := cmdReplication([]string{"select", "origin", "--actor", f.author.Actor, "--actor", signer.Actor, "--proposal", f.proposal.ID}); err != nil {
					t.Fatal(err)
				}
			}
			before := f.snapshot()
			output, err := captureTestOutput(t, func() error { return cmdSync([]string{"origin"}) })
			if err == nil {
				t.Fatalf("hostile %s sync succeeded: %s", kind, output)
			}
			assertRefValue(t, acceptedActorRef("origin", f.author.Actor), accepted)
			if kind == "wrong-actor" {
				assertRefAbsent(t, acceptedActorRef("origin", signer.Actor))
			}
			if got := f.snapshot(); !reflect.DeepEqual(before, got) {
				t.Fatalf("%s promoted refs/objects or changed signer before refusal", kind)
			}
			if _, err := gitOutput("cat-file", "-e", bad.Commit+"^{object}"); err == nil {
				t.Fatal("hostile commit escaped quarantine")
			}
			marker, err := os.ReadFile(f.marker)
			if err != nil || len(marker) != 0 {
				t.Fatal("hostile import caused backend effects")
			}
			if _, err := collectEvents(); err != nil {
				t.Fatal(err)
			}
			// The identical material builder must produce a valid archive too, so
			// malformed serialization cannot make every hostile case vacuously pass.
			controlSnapshot, controlBlobs := withdrawalArchiveMaterial(t, good.Snapshot)
			var controlArchive bytes.Buffer
			if err := migration.WriteArchive(ctx, &controlArchive, controlSnapshot, controlBlobs, limits); err != nil {
				t.Fatal(err)
			}
			control, err := migration.ReadArchive(ctx, bytes.NewReader(controlArchive.Bytes()), limits)
			if err != nil {
				t.Fatal(err)
			}
			defer control.Close()
			q, err := StageProjectSnapshot(ctx, control.Snapshot, control, limits)
			if err != nil {
				t.Fatalf("same-builder valid archive control refused: %v", err)
			}
			q.Close()
			if kind == "duplicate" || kind == "conflict" {
				// Its first withdrawal is valid, so use a fresh explicit request for the
				// backend control. Existing malicious remote evidence is never reset.
				writeTestPolicy(t, receiver, PolicyDocument{Version: policyVersion, Maintainers: []string{worker.Actor}, Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{}})
				mustGit(t, "config", "user.name", "Receiver")
				mustGit(t, "config", "user.email", "receiver@hn.invalid")
				mustGit(t, "add", ".hn/policy.json")
				mustGit(t, "commit", "-qm", "Positive control authority")
				controlBase := mustGitText(t, "rev-parse", "HEAD")
				mustGit(t, "commit", "--allow-empty", "-qm", "Positive control candidate")
				controlHead := mustGitText(t, "rev-parse", "HEAD")
				p := appendPolicyTestProposal(t, worker, controlBase, controlHead, "Eligible local control")
				e, err := nextEvent(worker, "run.request")
				if err != nil {
					t.Fatal(err)
				}
				e.Subject, e.Pipeline, e.Definition, e.Commit = p.ID, "test", r.Event.Definition, p.Event.Head
				fresh, err := appendEvent(e, worker)
				if err != nil {
					t.Fatal(err)
				}
				r = fresh
			}
			if err := cmdRunExecute([]string{r.ID, "--backend", "host", "--allow-unsafe-host-execution"}); err != nil {
				t.Fatalf("genuine eligible positive control failed: %v", err)
			}
			marker, err = os.ReadFile(f.marker)
			if err != nil || !bytes.Contains(marker, []byte("executed")) {
				t.Fatal("positive control did not execute backend")
			}
		})
	}
}

func TestRunWithdrawalDoesNotCancelInFlightPublication(t *testing.T) {
	f := newEfficiencyFixture(t, false)
	started, release := filepath.Join(t.TempDir(), "started"), filepath.Join(t.TempDir(), "release")
	pipeline := PipelineDefinition{Version: pipelineVersion, Steps: []PipelineStep{{Name: "In flight", Command: "sh", Args: []string{"-c", `printf started > "$1"; while [ ! -f "$2" ]; do sleep 0.01; done; printf executed >> "$3"`, "hn-test", started, release, f.marker}}}}
	raw, err := json.Marshal(pipeline)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, ".hn", "pipelines", "test.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", ".hn/pipelines/test.json")
	mustGit(t, "commit", "-qm", "In-flight backend control")
	f.head = mustGitText(t, "rev-parse", "HEAD")
	p := appendPolicyTestProposal(t, f.author, f.base, f.head, "In-flight candidate")
	r := f.request(f.author, p, "test", "")
	facts, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	type completion struct {
		result *StoredEvent
		err    error
	}
	done := make(chan completion, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() {
		s, err := executeRunRequest(ctx, facts, r, f.worker, hostBackend{}, false)
		done <- completion{s, err}
	}()
	defer os.WriteFile(release, nil, 0600)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("backend did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	w := appendWithdrawal(t, f.author, r, "Observed after backend started")
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	outcome := <-done
	if outcome.err != nil || outcome.result == nil {
		t.Fatalf("withdrawal invalidated in-flight result: %v", outcome.err)
	}
	after, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(currentRunResults(after, r.ID)) != 1 || findRunWithdrawal(after, r.ID) == nil {
		t.Fatal("late result or withdrawal absent")
	}
	got, err := loadStoredEvent(outcome.result.Commit)
	if err != nil || !bytes.Equal(got.Payload, outcome.result.Payload) {
		t.Fatalf("late signed result unverifiable: %v", err)
	}
	observed, err := loadStoredEvent(w.Commit)
	if err != nil || !bytes.Equal(observed.Payload, w.Payload) {
		t.Fatal("withdrawal changed")
	}
	if err := cmdSync([]string{"origin"}); err != nil {
		t.Fatalf("late result cannot replicate: %v", err)
	}
}

func TestRunWithdrawalSelectedShallowRecovery(t *testing.T) {
	f := newEfficiencyFixture(t, false)
	r := f.request(f.author, f.proposal, "test", "")
	control := f.request(f.author, f.proposal, "test", "")
	w := appendWithdrawal(t, f.author, r, "Recover complete withdrawal history")
	if err := cmdSync([]string{"origin"}); err != nil {
		t.Fatal(err)
	}
	receiver := filepath.Join(t.TempDir(), "receiver")
	mustGit(t, "clone", "-q", "file://"+f.remote, receiver)
	withTestDirectory(t, receiver)
	_, keyPath, err := createIdentity("Recovery worker")
	if err != nil {
		t.Fatal(err)
	}
	f.keyPath = keyPath
	mustGit(t, "fetch", "-q", "origin", proposalRef(f.proposal.ID)+":"+acceptedProposalRef("origin", f.proposal.ID))
	mustGit(t, "fetch", "-q", "--depth", "1", "origin", actorRef(f.author.Actor)+":"+acceptedActorRef("origin", f.author.Actor))
	if err := cmdReplication([]string{"select", "origin", "--actor", f.author.Actor, "--proposal", f.proposal.ID}); err != nil {
		t.Fatal(err)
	}
	before := f.snapshot()
	err = guardShallowEventClosure("run list")
	var gap *ShallowDependencyGap
	if !errors.As(err, &gap) || gap.Kind != shallowActorPredecessor || gap.MissingID != control.ID {
		t.Fatalf("withdrawal's cut actor history not recoverable: %v", err)
	}
	if got := f.snapshot(); !reflect.DeepEqual(before, got) {
		t.Fatal("gap classification promoted incomplete actor history")
	}
	marker, err := os.ReadFile(f.marker)
	if err != nil || len(marker) != 0 {
		t.Fatal("incomplete withdrawal history caused backend effect")
	}
	if err := cmdSync([]string{"origin", "--recover-shallow"}); err != nil {
		t.Fatal(err)
	}
	if err := guardShallowEventClosure("run list"); err != nil {
		t.Fatal(err)
	}
	facts, err := collectEvents()
	if err != nil {
		t.Fatal(err)
	}
	got := findRunWithdrawal(facts, r.ID)
	if got == nil || got.ID != w.ID || !bytes.Equal(got.Payload, w.Payload) || !bytes.Equal(got.Signature, w.Signature) {
		t.Fatal("recovery lost exact signed withdrawal")
	}
	assertRefValue(t, acceptedActorRef("origin", f.author.Actor), w.Commit)
	if err := cmdRunExecute([]string{control.ID, "--backend", "host", "--allow-unsafe-host-execution"}); err != nil {
		t.Fatal(err)
	}
	marker, err = os.ReadFile(f.marker)
	if err != nil || !bytes.Contains(marker, []byte("executed")) {
		t.Fatal("eligible request did not execute after recovery")
	}
}
