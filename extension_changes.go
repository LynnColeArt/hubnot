package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"hubnot/internal/artifact"
	"hubnot/internal/extension"
	"hubnot/internal/safepath"
)

const extensionChangeEntries = 10000
const extensionChangeBytes = 16 << 20
const extensionChangeLifetime = 10 * time.Minute

// These are eviction budgets for private recoverable state, not public promises
// of continuity. Lost state always requires an explicit new capture.
const extensionChangeCacheBytes = 256 << 20
const extensionChangeCacheCount = 16
const extensionChangeCheckpointCount = 64
const extensionChangeCheckpointBytes = 128 << 20

type extensionChangeSelection struct {
	Families []string `json:"families"`
	Types    []string `json:"types"`
}
type extensionChangeEntry struct {
	Protocol    string   `json:"protocol"`
	Family      *string  `json:"family"`
	TypeID      *string  `json:"type_id"`
	ResourceID  string   `json:"resource_id"`
	RevisionIDs []string `json:"revision_ids"`
	Value       any      `json:"value"`
}
type extensionChangePage struct {
	Capture         string                   `json:"capture"`
	SnapshotID      string                   `json:"snapshot_id"`
	Selection       extensionChangeSelection `json:"selection"`
	SelectionDigest string                   `json:"selection_digest"`
	SourceCut       string                   `json:"source_cut"`
	Mode            string                   `json:"mode"`
	Items           []extensionChangeEntry   `json:"items"`
	NextCursor      *string                  `json:"next_cursor"`
	Complete        bool                     `json:"complete"`
	Checkpoint      *string                  `json:"checkpoint"`
	ExpiresAt       string                   `json:"expires_at"`
}

// Facts are saved only after the shared accepted-history verifiers and governed
// projections succeed. No raw event/body is stored in checkpoint descriptors.
type extensionChangeFacts struct {
	Refs            []extensionNativeRef `json:"refs"`
	AuthorityCommit string               `json:"authority_commit"`
	PolicyDigest    string               `json:"policy_digest"`
	ViewDigest      string               `json:"view_digest"`
	RetainedRefs    []extensionNativeRef `json:"retained_refs"`
}
type extensionChangeCheckpoint struct {
	Repository      string                   `json:"repository"`
	Registration    string                   `json:"registration"`
	Epoch           string                   `json:"epoch"`
	Grants          string                   `json:"grants"`
	Selection       extensionChangeSelection `json:"selection"`
	SelectionDigest string                   `json:"selection_digest"`
	SourceCut       string                   `json:"source_cut"`
	SnapshotID      string                   `json:"snapshot_id"`
	Facts           extensionChangeFacts     `json:"facts"`
	RetainedDigest  string                   `json:"retained_digest"`
}
type extensionChangeObservation struct {
	entries []extensionChangeEntry
	facts   extensionChangeFacts
	cut     string
	// Membership comes from the complete current shared verifiers, not rev-list
	// alone. It proves old source objects remain in a verified accepted history.
	commits map[string]bool
}
type extensionChangeCapture struct {
	token, session, verifier string
	descriptor               extensionChangeCheckpoint
	entries                  []extensionChangeEntry
	bytes                    int
	created                  time.Time
	cursors                  map[string]int
	offsets                  map[int]string
	checkpoint               string
}

var extensionChanges = struct {
	sync.Mutex
	captures map[string]*extensionChangeCapture
}{captures: map[string]*extensionChangeCapture{}}

func extensionChangesInvalid() error {
	return extensionNativeHostError("checkpoint_invalidated", "capture or checkpoint is unavailable or no longer verifiable; discard incomplete staging and capture the exact selection again", "rescan_required")
}
func extensionChangesCapacity() error {
	return extensionNativeHostError("resource_limit", "complete capture exceeds 10000 entries or 16 MiB; use separate explicit family or type selections, or local full export; one oversized complete state requires export", "do_not_retry")
}
func extensionChangesError(err error, call extension.Invocation) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return extensionNativeHostError("cancelled", "change observation cancelled", "do_not_retry")
	}
	var public *extension.Error
	if errors.As(err, &public) {
		return err
	}
	var native *IssueReadError
	var command *issueCommandError
	var custom *artifact.Error
	if errors.As(err, &native) && native.Code == "invalid_history" || errors.As(err, &command) && command.Code == "invalid_history" || errors.As(err, &custom) && (custom.Code == "invalid_history" || custom.Code == "verification_failure") {
		return extensionNativeHostError("verification_failure", "accepted change history could not be verified", "do_not_retry")
	}
	if errors.As(err, &custom) {
		return extensionArtifactError(err, call)
	}
	return extensionNativeError(err, call.Request.Params)
}

// Values here already passed their native/schema verifier. Normalize structs to
// JSON values, preserving integers, then use the same encoding/json C encoding
// as the shared canonical package. No envelope-parser cap applies to a capture.
func extensionChangesCanonical(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var normalized any
	if err = d.Decode(&normalized); err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}
func extensionChangesBound(entries []extensionChangeEntry) ([]byte, error) {
	if len(entries) > extensionChangeEntries {
		return nil, extensionChangesCapacity()
	}
	encoded, err := extensionChangesCanonical(entries)
	if err != nil {
		return nil, err
	}
	if len(encoded) > extensionChangeBytes {
		return nil, extensionChangesCapacity()
	}
	return encoded, nil
}
func extensionChangesDigest(value any) string {
	raw, _ := extensionChangesCanonical(value)
	return eventID(raw)
}
func extensionChangesRandom() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", extensionNativeHostError("repository_error", "cannot allocate private observation token", "do_not_retry")
	}
	return hex.EncodeToString(b[:]), nil
}
func extensionChangesSelection(p map[string]any) extensionChangeSelection {
	selection := p["selection"].(map[string]any)
	return extensionChangeSelection{Families: artifactIDs(selection["families"]), Types: artifactIDs(selection["types"])}
}
func extensionChangesAuthorize(call extension.Invocation, selection extensionChangeSelection) error {
	if len(selection.Families)+len(selection.Types) == 0 || call.SessionID == "" {
		return extensionNativeHostError("invalid_input", "change observation requires an explicit selection and host session", "do_not_retry")
	}
	for _, family := range selection.Families {
		if err := extension.Authorize(call.Registration, call.Enablement, call.Binding, extension.Access{Method: call.Request.Method, Selector: "native:" + family, Action: "observe"}, false); err != nil {
			return err
		}
	}
	for _, typeID := range selection.Types {
		if err := extension.Authorize(call.Registration, call.Enablement, call.Binding, extension.Access{Method: call.Request.Method, Selector: "artifact:" + typeID, Action: "observe"}, false); err != nil {
			return err
		}
	}
	return nil
}
func extensionChangesCustomPolicy(selection extensionChangeSelection) error {
	if len(selection.Types) == 0 {
		return nil
	}
	authority, err := loadContributionAuthority()
	if err != nil {
		return issueFailure("invalid_history", err)
	}
	if authority != nil && authority.Policy.Contributions != nil {
		return extensionNativeHostError("unsupported_capability", "custom change capture under selected contribution policy is unsupported", "do_not_retry")
	}
	return nil
}
func extensionChangesSortedIDs(ids []string) []string {
	sort.Strings(ids)
	return slices.Compact(ids)
}
func extensionChangesRefs(refs []extensionNativeRef) ([]extensionNativeRef, error) {
	sort.Slice(refs, func(i, j int) bool { return refs[i].Ref < refs[j].Ref })
	out := []extensionNativeRef{}
	for _, ref := range refs {
		if len(out) > 0 && out[len(out)-1].Ref == ref.Ref {
			if out[len(out)-1].OID != ref.OID {
				return nil, extensionChangesInvalid()
			}
			continue
		}
		out = append(out, ref)
	}
	return out, nil
}
func extensionChangesCut(repo string, selection extensionChangeSelection, facts extensionChangeFacts) string {
	return extensionChangesDigest(map[string]any{"repository_id": repo, "refs": facts.Refs, "selection": selection, "authority_commit": facts.AuthorityCommit, "policy_digest": facts.PolicyDigest, "view_digest": facts.ViewDigest})
}
func extensionChangesNativeCommon(a, b extensionNativeCutInputs) bool {
	return a.RepositoryID == b.RepositoryID && a.AuthorityCommit == b.AuthorityCommit && a.PolicyDigest == b.PolicyDigest && reflect.DeepEqual(a.Refs, b.Refs) && reflect.DeepEqual(a.RetainedRefs, b.RetainedRefs)
}

func extensionChangesObserve(ctx context.Context, call extension.Invocation, selection extensionChangeSelection) (*extensionChangeObservation, error) {
	if err := extensionChangesCustomPolicy(selection); err != nil {
		return nil, err
	}
	repo, _, err := artifactRepositoryID()
	if err != nil {
		return nil, err
	}
	if repo != call.Binding.RepositoryID {
		return nil, extensionNativeHostError("denied_access", "repository binding changed", "do_not_retry")
	}
	unlock, err := contributionCommandLock([]string{"issue", "list"})
	if err != nil {
		return nil, extensionNativeHostError("repository_error", "change observation is busy or unavailable", "retry_identical")
	}
	defer unlock()
	out := &extensionChangeObservation{entries: []extensionChangeEntry{}, facts: extensionChangeFacts{Refs: []extensionNativeRef{}, RetainedRefs: []extensionNativeRef{}}, commits: map[string]bool{}}
	admitted, visible := []string{}, []string{}
	var first *extensionNativeProjection
	for _, family := range selection.Families {
		p, e := extensionNativeProjectionFor(ctx, family)
		if e != nil {
			return nil, e
		}
		if first == nil {
			first = p
			out.facts.AuthorityCommit = p.cutInputs.AuthorityCommit
			out.facts.PolicyDigest = p.cutInputs.PolicyDigest
			out.facts.Refs = append(out.facts.Refs, p.cutInputs.Refs...)
			out.facts.RetainedRefs = append(out.facts.RetainedRefs, p.cutInputs.RetainedRefs...)
		} else if !extensionChangesNativeCommon(first.cutInputs, p.cutInputs) {
			return nil, extensionChangesInvalid()
		}
		admitted = append(admitted, p.cutInputs.AdmittedIDs...)
		visible = append(visible, p.cutInputs.VisibleIDs...)
		for _, event := range p.events {
			out.commits[event.Commit] = true
		}
		for _, v := range p.views {
			ids := []string{}
			for _, head := range v.Heads {
				ids = append(ids, head.(map[string]any)["id"].(string))
			}
			for _, comment := range v.Comments {
				ids = append(ids, comment.(map[string]any)["id"].(string))
			}
			family := family
			out.entries = append(out.entries, extensionChangeEntry{Protocol: "hn/0", Family: &family, ResourceID: v.ID, RevisionIDs: extensionChangesSortedIDs(ids), Value: v})
		}
	}
	var h *artifactHistory
	if len(selection.Types) > 0 {
		h, err = loadArtifactHistory(ctx)
		if err != nil {
			return nil, err
		}
		catalog, e := buildArtifactCatalog(ctx, h)
		if e != nil {
			return nil, e
		}
		// Consume the current type-wide projection helper as well as its verified
		// catalog. Its diagnostics cover every selected current head, not just a page.
		if _, err = extensionArtifactProjection(ctx, call, catalog, selection.Types); err != nil {
			return nil, err
		}
		for _, source := range h.Sources {
			out.facts.Refs = append(out.facts.Refs, extensionNativeRef{Ref: source.Ref, OID: source.OID})
		}
		selected := map[string]bool{}
		for _, typeID := range selection.Types {
			selected[typeID] = true
		}
		for id, item := range h.ByID {
			out.commits[item.commit] = true
			if item.verified.Kind() == "type.publish" && selected[artifactText(artifactPayload(item), "type_id")] {
				admitted = append(admitted, id)
				visible = append(visible, id)
			}
		}
		for logical, revisions := range catalog.records {
			if !selected[artifactText(artifactPayload(h.ByID[logical]), "type_id")] {
				continue
			}
			admitted = append(admitted, revisions...)
			visible = append(visible, revisions...)
			v, e := extensionArtifactView(ctx, call, catalog, logical)
			if e != nil {
				return nil, e
			}
			ids := []string{}
			for _, head := range v.Heads {
				ids = append(ids, head.ID)
			}
			typeID := v.TypeID
			out.entries = append(out.entries, extensionChangeEntry{Protocol: "hn-artifact/1", TypeID: &typeID, ResourceID: v.ID, RevisionIDs: extensionChangesSortedIDs(ids), Value: v})
		}
	}
	// Sequential projections must share one actual cut. Repeat the first native
	// governed projection after custom reads and independently recheck all custom
	// source witnesses and current binding. Retention is reverified by the helper.
	if first != nil {
		after, e := extensionNativeProjectionFor(ctx, selection.Families[0])
		if e != nil {
			return nil, e
		}
		if !reflect.DeepEqual(first.cutInputs, after.cutInputs) {
			return nil, extensionChangesInvalid()
		}
	}
	if h != nil {
		if err = extensionArtifactStable(ctx, call, h); err != nil {
			return nil, err
		}
	}
	if err = extensionChangesCustomPolicy(selection); err != nil {
		return nil, err
	}
	if err = extensionArtifactBinding(ctx, call); err != nil {
		return nil, err
	}
	out.facts.Refs, err = extensionChangesRefs(out.facts.Refs)
	if err != nil {
		return nil, err
	}
	out.facts.ViewDigest = extensionChangesDigest(map[string]any{"admitted_ids": extensionChangesSortedIDs(admitted), "visible_ids": extensionChangesSortedIDs(visible), "retained_refs": out.facts.RetainedRefs})
	out.cut = extensionChangesCut(repo, selection, out.facts)
	sort.Slice(out.entries, func(i, j int) bool {
		a, b := out.entries[i], out.entries[j]
		if a.Protocol != b.Protocol {
			return a.Protocol < b.Protocol
		}
		scope := func(e extensionChangeEntry) string {
			if e.Family != nil {
				return *e.Family
			}
			return *e.TypeID
		}
		if scope(a) != scope(b) {
			return scope(a) < scope(b)
		}
		return a.ResourceID < b.ResourceID
	})
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// A race detected by a shared reader while verifying an existing capture is
// evidence that its old coherent view is lost. New capture/since reads retain
// their native/artifact retry semantics; other refusals keep their own cause.
func extensionChangesSourceRace(err error) bool {
	// Classify the same public cause as dispatch. In particular an enclosing
	// invalid-history refusal must not be weakened by a nested race diagnostic.
	public, ok := extensionChangesError(err, extension.Invocation{}).(*extension.Error)
	return ok && public.Code == "concurrent_update" && (public.Origin == "native" || public.Origin == "artifact")
}

func extensionChangesDispatch(ctx context.Context, call extension.Invocation) (data any, err error) {
	defer func() { err = extensionChangesError(err, call) }()
	// Bound token lookup before any scope can be inferred from private state.
	if err = extension.Authorize(call.Registration, call.Enablement, call.Binding, extension.Access{Method: "capabilities.list"}, false); err != nil {
		return nil, err
	}
	p := call.Request.Params
	var selection extensionChangeSelection
	var capture *extensionChangeCapture
	var offset int
	if call.Request.Method == "changes.page" {
		token := artifactText(p, "capture")
		extensionChanges.Lock()
		capture = extensionChanges.captures[token]
		extensionChanges.Unlock()
		if capture == nil || capture.session != call.SessionID || capture.descriptor.Repository != call.Binding.RepositoryID || capture.descriptor.Registration != call.Binding.RegistrationID || capture.descriptor.Epoch != call.Binding.Epoch || capture.descriptor.Grants != extensionChangesDigest(call.Enablement.Grants) {
			return nil, extensionChangesInvalid()
		}
		selection = capture.descriptor.Selection
		var found bool
		offset, found = capture.cursors[artifactText(p, "cursor")]
		if !found {
			return nil, extensionChangesInvalid()
		}
	} else {
		selection = extensionChangesSelection(p)
	}
	if err = extensionChangesAuthorize(call, selection); err != nil {
		return nil, err
	}
	if capture != nil && !time.Now().Before(capture.created.Add(extensionChangeLifetime)) {
		extensionChangesForget(capture.token)
		return nil, extensionChangesInvalid()
	}
	observation, err := extensionChangesObserve(ctx, call, selection)
	if err != nil {
		if capture != nil && extensionChangesSourceRace(err) {
			extensionChangesForget(capture.token)
			return nil, extensionChangesInvalid()
		}
		return nil, err
	}
	_, common, err := artifactRepositoryID()
	if err != nil {
		return nil, err
	}
	if capture != nil {
		if observation.cut != capture.descriptor.SourceCut {
			extensionChangesForget(capture.token)
			return nil, extensionChangesInvalid()
		}
		state, e := extensionChangesOpenState(common, false)
		if e != nil {
			return nil, e
		}
		defer state.close()
		if eventID(state.key) != capture.verifier {
			return nil, extensionChangesInvalid()
		}
		return extensionChangesPage(ctx, call, capture, offset, state)
	}
	if call.Request.Method == "changes.since" {
		state, e := extensionChangesOpenState(common, false)
		if e != nil {
			return nil, e
		}
		old, e := state.read(artifactText(p, "checkpoint"))
		state.close()
		if e != nil {
			return nil, e
		}
		if err = extensionChangesCompatible(ctx, call, selection, old, observation); err != nil {
			return nil, err
		}
	}
	encoded, err := extensionChangesBound(observation.entries)
	if err != nil {
		return nil, err
	}
	token, err := extensionChangesRandom()
	if err != nil {
		return nil, err
	}
	cpID, err := extensionChangesRandom()
	if err != nil {
		return nil, err
	}
	descriptor := extensionChangeCheckpoint{Repository: call.Binding.RepositoryID, Registration: call.Binding.RegistrationID, Epoch: call.Binding.Epoch, Grants: extensionChangesDigest(call.Enablement.Grants), Selection: selection, SelectionDigest: extensionChangesDigest(selection), SourceCut: observation.cut, SnapshotID: extensionChangesDigest(map[string]any{"source_cut": observation.cut, "selection_digest": extensionChangesDigest(selection), "entries": observation.entries}), Facts: observation.facts, RetainedDigest: extensionChangesDigest(observation.facts.RetainedRefs)}
	descriptor.Facts.RetainedRefs = []extensionNativeRef{}
	descriptorBytes, _ := json.Marshal(descriptor)
	capture = &extensionChangeCapture{token: token, session: call.SessionID, descriptor: descriptor, entries: observation.entries, bytes: len(encoded) + len(descriptorBytes), created: time.Now(), cursors: map[string]int{}, offsets: map[int]string{}, checkpoint: cpID}
	// Check the actual full wire response before creating any verifier or capture
	// state. A placeholder checkpoint has precisely the final token's wire size.
	if _, err = extensionChangesRender(call, capture, 0, strings.Repeat("0", 129)); err != nil {
		return nil, err
	}
	state, err := extensionChangesOpenState(common, true)
	if err != nil {
		return nil, err
	}
	defer state.close()
	capture.verifier = eventID(state.key)
	result, err := extensionChangesPage(ctx, call, capture, 0, state)
	if err != nil {
		return nil, err
	}
	extensionChangesRetain(capture)
	return result, nil
}

func extensionChangesForget(token string) {
	extensionChanges.Lock()
	defer extensionChanges.Unlock()
	delete(extensionChanges.captures, token)
}
func extensionChangesRetain(capture *extensionChangeCapture) {
	extensionChanges.Lock()
	defer extensionChanges.Unlock()
	total := capture.bytes
	now := time.Now()
	for token, c := range extensionChanges.captures {
		if !now.Before(c.created.Add(extensionChangeLifetime)) {
			delete(extensionChanges.captures, token)
		} else {
			total += c.bytes
		}
	}
	for total > extensionChangeCacheBytes || len(extensionChanges.captures) >= extensionChangeCacheCount {
		var oldest *extensionChangeCapture
		for _, c := range extensionChanges.captures {
			if oldest == nil || c.created.Before(oldest.created) {
				oldest = c
			}
		}
		if oldest == nil {
			break
		}
		total -= oldest.bytes
		delete(extensionChanges.captures, oldest.token)
	}
	extensionChanges.captures[capture.token] = capture
}
func extensionChangesRender(call extension.Invocation, capture *extensionChangeCapture, offset int, checkpoint string) (extensionChangePage, error) {
	var zero extensionChangePage
	limit := int(call.Request.Params["limit"].(float64))
	if limit < 1 {
		return zero, extensionNativeHostError("invalid_input", "page limit must be positive", "do_not_retry")
	}
	if limit > 200 {
		return zero, extensionNativeHostError("resource_limit", "page limit exceeds 200", "do_not_retry")
	}
	if offset < 0 || offset > len(capture.entries) {
		return zero, extensionChangesInvalid()
	}
	end := min(offset+limit, len(capture.entries))
	page := extensionChangePage{Capture: capture.token, SnapshotID: capture.descriptor.SnapshotID, Selection: capture.descriptor.Selection, SelectionDigest: capture.descriptor.SelectionDigest, SourceCut: capture.descriptor.SourceCut, Mode: "replace", Items: capture.entries[offset:end], Complete: end == len(capture.entries), ExpiresAt: capture.created.Add(extensionChangeLifetime).UTC().Format(time.RFC3339)}
	if page.Complete {
		page.Checkpoint = &checkpoint
	} else {
		extensionChanges.Lock()
		cursor := capture.offsets[end]
		if cursor == "" {
			var err error
			cursor, err = extensionChangesRandom()
			if err != nil {
				extensionChanges.Unlock()
				return zero, err
			}
			capture.offsets[end] = cursor
			capture.cursors[cursor] = end
		}
		extensionChanges.Unlock()
		page.NextCursor = &cursor
	}
	wire, err := json.Marshal(map[string]any{"protocol": "hn.extension/1", "request_id": call.Request.ID, "ok": true, "data": page, "error": nil})
	if err != nil {
		return zero, extensionNativeHostError("repository_error", "cannot encode complete change page", "do_not_retry")
	}
	if len(wire) > extension.FrameLimit {
		message := "complete page exceeds 8 MiB; request a smaller page limit"
		if end-offset <= 1 {
			message = "one complete state exceeds 8 MiB; use local full export"
		}
		return zero, extensionNativeHostError("resource_limit", message, "do_not_retry")
	}
	return page, nil
}
func extensionChangesPageValid(ctx context.Context, capture *extensionChangeCapture) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !time.Now().Before(capture.created.Add(extensionChangeLifetime)) {
		extensionChangesForget(capture.token)
		return extensionChangesInvalid()
	}
	return nil
}
func extensionChangesPage(ctx context.Context, call extension.Invocation, capture *extensionChangeCapture, offset int, state *extensionChangeState) (extensionChangePage, error) {
	if err := extensionChangesPageValid(ctx, capture); err != nil {
		return extensionChangePage{}, err
	}
	checkpoint := state.token(capture.checkpoint, capture.descriptor)
	page, err := extensionChangesRender(call, capture, offset, checkpoint)
	if err != nil {
		return page, err
	}
	if err = extensionChangesPageValid(ctx, capture); err != nil {
		return extensionChangePage{}, err
	}
	if page.Complete {
		if err = state.write(capture.checkpoint, capture.descriptor); err != nil {
			return extensionChangePage{}, err
		}
	}
	if err = extensionChangesPageValid(ctx, capture); err != nil {
		return extensionChangePage{}, err
	}
	return page, nil
}
func extensionChangesCompatible(ctx context.Context, call extension.Invocation, selection extensionChangeSelection, old extensionChangeCheckpoint, current *extensionChangeObservation) error {
	if old.Repository != call.Binding.RepositoryID || old.Registration != call.Binding.RegistrationID || old.Epoch != call.Binding.Epoch || old.Grants != extensionChangesDigest(call.Enablement.Grants) || !reflect.DeepEqual(old.Selection, selection) || old.SelectionDigest != extensionChangesDigest(selection) || extensionChangesCut(old.Repository, old.Selection, old.Facts) != old.SourceCut {
		return extensionChangesInvalid()
	}
	if old.Facts.AuthorityCommit != current.facts.AuthorityCommit || old.Facts.PolicyDigest != current.facts.PolicyDigest || old.RetainedDigest != extensionChangesDigest(current.facts.RetainedRefs) {
		return extensionChangesInvalid()
	}
	refs := map[string]string{}
	for _, ref := range current.facts.Refs {
		refs[ref.Ref] = ref.OID
	}
	gitDir, err := requireGitRepository()
	if err != nil {
		return extensionChangesInvalid()
	}
	for _, ref := range old.Facts.Refs {
		if err = ctx.Err(); err != nil {
			return err
		}
		head := refs[ref.Ref]
		if head == "" || !current.commits[ref.OID] {
			return extensionChangesInvalid()
		}
		if head != ref.OID {
			if _, err = projectGit(ctx, gitDir, nil, 1024, "merge-base", "--is-ancestor", ref.OID, head); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return extensionChangesInvalid()
			}
		}
	}
	return nil
}

type extensionChangeState struct {
	root *os.Root
	lock *os.File
	key  []byte
}

func (s *extensionChangeState) close() {
	if s.lock != nil {
		_ = syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
		s.lock.Close()
	}
	s.root.Close()
}
func extensionChangesStateError() error {
	return extensionNativeHostError("repository_error", "private observation state is unavailable or unsafe", "do_not_retry")
}
func extensionChangesOpenState(common string, create bool) (*extensionChangeState, error) {
	if err := safepath.CheckDir(common, false); err != nil {
		return nil, extensionChangesStateError()
	}
	path := common
	for _, part := range []string{"hn", "extensions", "changes"} {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			if !create {
				return nil, extensionChangesInvalid()
			}
			if err = os.Mkdir(path, 0700); err != nil && !os.IsExist(err) {
				return nil, extensionChangesStateError()
			}
			info, err = os.Lstat(path)
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
			return nil, extensionChangesStateError()
		}
	}
	if err := safepath.CheckDir(path, true); err != nil {
		return nil, extensionChangesStateError()
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, extensionChangesStateError()
	}
	state := &extensionChangeState{root: root}
	fail := func(e error) (*extensionChangeState, error) { state.close(); return nil, e }
	flags := os.O_RDWR | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
	if create {
		flags |= os.O_CREATE
	}
	state.lock, err = root.OpenFile("lock", flags, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			return fail(extensionChangesInvalid())
		}
		return fail(extensionChangesStateError())
	}
	info, err := state.lock.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return fail(extensionChangesStateError())
	}
	if err = syscall.Flock(int(state.lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fail(extensionNativeHostError("repository_error", "private observation state is busy", "retry_identical"))
	}
	key, err := state.readFile("key", 32)
	if os.IsNotExist(err) && create {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return fail(extensionChangesStateError())
		}
		f, e := root.OpenFile("key", os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
		if e != nil {
			return fail(extensionChangesStateError())
		}
		_, err = f.Write(key)
		if err == nil {
			err = f.Sync()
		}
		if e = f.Close(); err == nil {
			err = e
		}
	}
	if err != nil || len(key) != 32 {
		return fail(extensionChangesInvalid())
	}
	state.key = key
	return state, nil
}
func (s *extensionChangeState) readFile(name string, limit int64) ([]byte, error) {
	f, err := s.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, extensionChangesStateError()
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, extensionChangesStateError()
	}
	return raw, nil
}
func (s *extensionChangeState) token(id string, descriptor extensionChangeCheckpoint) string {
	raw, _ := json.Marshal(descriptor)
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte("hn-extension-checkpoint-v1\x00" + id + "\x00"))
	mac.Write(raw)
	return id + "." + hex.EncodeToString(mac.Sum(nil))
}
func (s *extensionChangeState) read(token string) (extensionChangeCheckpoint, error) {
	var descriptor extensionChangeCheckpoint
	if len(token) != 129 || token[64] != '.' {
		return descriptor, extensionChangesInvalid()
	}
	id := token[:64]
	if _, err := hex.DecodeString(id); err != nil {
		return descriptor, extensionChangesInvalid()
	}
	raw, err := s.readFile(id+".json", extensionChangeCheckpointBytes)
	if err != nil {
		return descriptor, extensionChangesInvalid()
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&descriptor) != nil || d.Decode(new(any)) != io.EOF {
		return descriptor, extensionChangesInvalid()
	}
	expected := s.token(id, descriptor)
	if !hmac.Equal([]byte(expected), []byte(token)) {
		return extensionChangeCheckpoint{}, extensionChangesInvalid()
	}
	return descriptor, nil
}
func (s *extensionChangeState) write(id string, descriptor extensionChangeCheckpoint) error {
	raw, err := json.Marshal(descriptor)
	if err != nil {
		return extensionChangesStateError()
	}
	// The shared history verifier already has finite ref/name bounds. If its
	// authenticated proof alone cannot fit the recoverable-state budget, refuse
	// checkpoint completion explicitly rather than issue an unverifiable token.
	if len(raw) > extensionChangeCheckpointBytes {
		return extensionChangesInvalid()
	}
	dir, err := s.root.Open(".")
	if err != nil {
		return extensionChangesStateError()
	}
	defer dir.Close()
	files := []os.FileInfo{}
	var total int64
	for {
		batch, e := dir.Readdir(64)
		for _, info := range batch {
			if info.Name() == "key" || info.Name() == "lock" {
				continue
			}
			if len(info.Name()) != 69 || !strings.HasSuffix(info.Name(), ".json") || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
				return extensionChangesStateError()
			}
			if _, e := hex.DecodeString(strings.TrimSuffix(info.Name(), ".json")); e != nil {
				return extensionChangesStateError()
			}
			if info.Name() == id+".json" {
				continue
			}
			files = append(files, info)
			total += info.Size()
			if len(files) > extensionChangeCheckpointCount || total > extensionChangeCheckpointBytes {
				return extensionChangesStateError()
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return extensionChangesStateError()
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ModTime().Before(files[j].ModTime()) })
	for len(files) >= extensionChangeCheckpointCount || total+int64(len(raw)) > extensionChangeCheckpointBytes {
		if len(files) == 0 {
			return extensionChangesInvalid()
		}
		oldest := files[0]
		if err = s.root.Remove(oldest.Name()); err != nil {
			return extensionChangesStateError()
		}
		total -= oldest.Size()
		files = files[1:]
	}
	// O_EXCL avoids following an existing descriptor. Idempotent final-page replay
	// accepts only exactly the same authenticated immutable descriptor.
	existing, e := s.readFile(id+".json", extensionChangeCheckpointBytes)
	if e == nil {
		if bytes.Equal(raw, existing) {
			return nil
		}
		return extensionChangesInvalid()
	}
	if !os.IsNotExist(e) {
		return extensionChangesStateError()
	}
	f, err := s.root.OpenFile(id+".json", os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return extensionChangesStateError()
	}
	_, err = f.Write(raw)
	if err == nil {
		err = f.Sync()
	}
	if e = f.Close(); err == nil {
		err = e
	}
	if err != nil {
		_ = s.root.Remove(id + ".json")
		return extensionChangesStateError()
	}
	return nil
}
