package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"hubnot/internal/forge"
)

const forgeSourceSchema = "hn.forge-source/1"
const forgeCommentSchema = "hn.forge-comment/1"
const forgeCommentPrefix = "\n\n<!-- hn:forge-comment:"
const forgeExportIssuePrefix = "\n\n<!-- hn:forge-issue:"

type forgeSource struct {
	Schema    string `json:"schema"`
	Provider  string `json:"provider"`
	Endpoint  string `json:"endpoint"`
	Project   string `json:"project"`
	ID        string `json:"id"`
	Key       string `json:"key"`
	URL       string `json:"url"`
	Author    string `json:"author"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func forgeSourceKey(s forgeSource) string { return s.Provider + "\n" + s.Endpoint + "\n" + s.ID }
func forgeValidURL(value string, endpoint bool) bool {
	if value == "" && !endpoint {
		return true
	}
	if !issueIdentifier(value, 2048) {
		return false
	}
	u, err := url.Parse(value)
	if err != nil || u.User != nil || u.Opaque != "" || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return false
	}
	return !endpoint || (u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && !strings.Contains(value, "#"))
}
func forgeValidTime(value string) bool {
	if value == "" {
		return true
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}
func forgeValidateSource(s forgeSource) error {
	if s.Schema != forgeSourceSchema || !issueIdentifier(s.Provider, 64) || !forgeValidURL(s.Endpoint, true) || !issueIdentifier(s.Project, 512) || !issueIdentifier(s.ID, 1024) || !issueIdentifier(s.Key, 256) || !forgeValidURL(s.URL, false) || (s.Author != "" && !issueIdentifier(s.Author, 256)) || !forgeValidTime(s.CreatedAt) || !forgeValidTime(s.UpdatedAt) {
		return errors.New("invalid forge source provenance")
	}
	return nil
}
func forgeParseSource(raw string) (forgeSource, error) {
	var s forgeSource
	if len(raw) > 4096 || forgeStrictJSON([]byte(raw), &s) != nil {
		return s, errors.New("invalid forge source metadata")
	}
	return s, forgeValidateSource(s)
}
func forgeMetadata(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > 4096 {
		return "", errors.New("forge provenance exceeds native metadata bounds")
	}
	return string(raw), nil
}

type forgeManaged struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Status    string   `json:"status"`
	Labels    []string `json:"labels"`
	Assignees []string `json:"assignees"`
}

func forgeManagedProjection(state IssueState) (forgeManaged, error) {
	canonical, err := CanonicalIssueState(state)
	if err != nil {
		return forgeManaged{}, err
	}
	return forgeManaged{canonical.Title, canonical.Body, canonical.Status, canonical.Labels, canonical.Assignees}, nil
}
func forgeManagedDigest(state IssueState) (string, error) {
	value, err := forgeManagedProjection(state)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return eventID(raw), nil
}

type forgeCommentProvenance struct {
	Schema    string `json:"schema"`
	Provider  string `json:"provider"`
	SourceKey string `json:"source_key"`
	ID        string `json:"id"`
	URL       string `json:"url"`
	Author    string `json:"author"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Digest    string `json:"digest"`
}

func forgeCommentHeader(p forgeCommentProvenance) string {
	author, created, updated := p.Author, p.CreatedAt, p.UpdatedAt
	if author == "" {
		author = "(unknown)"
	}
	if created == "" {
		created = "(unknown)"
	}
	if updated == "" {
		updated = "(unknown)"
	}
	return fmt.Sprintf("Imported %s comment; source author %s, created %s, updated %s\n%s\n\n", p.Provider, author, created, updated, p.URL)
}
func forgeFormatComment(p forgeCommentProvenance, body string) (string, error) {
	parts := strings.Split(p.SourceKey, "\n")
	if len(parts) != 3 || parts[0] != p.Provider || !forgeValidURL(parts[1], true) || !issueIdentifier(parts[2], 1024) {
		return "", errors.New("invalid forge comment source binding")
	}
	if p.Schema != forgeCommentSchema || !issueIdentifier(p.Provider, 64) || p.SourceKey == "" || len(p.SourceKey) > 4096 || !issueIdentifier(p.ID, 1024) || !forgeValidURL(p.URL, false) || (p.Author != "" && !issueIdentifier(p.Author, 256)) || !forgeValidTime(p.CreatedAt) || !forgeValidTime(p.UpdatedAt) || p.Digest != eventID([]byte(body)) {
		return "", errors.New("invalid forge comment provenance")
	}
	raw, err := json.Marshal(p)
	if err != nil || len(raw) > 8192 {
		return "", errors.New("forge comment provenance exceeds bounds")
	}

	result := forgeCommentHeader(p) + body + forgeCommentPrefix + base64.RawURLEncoding.EncodeToString(raw) + " -->"
	if !issueText(result, 65536, true) {
		return "", errors.New("attributed forge comment exceeds native body bounds")
	}
	return result, nil
}

// An exact footer and exact header/body digest are required. Marker-looking
// text embedded elsewhere never qualifies as source identity or deduplication.
func forgeParseComment(body string) (forgeCommentProvenance, string, bool, error) {
	var p forgeCommentProvenance
	index := strings.LastIndex(body, forgeCommentPrefix)
	if index < 0 || !strings.HasSuffix(body, " -->") {
		return p, "", false, nil
	}
	encoded := body[index+len(forgeCommentPrefix) : len(body)-4]
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) > 8192 || forgeStrictJSON(raw, &p) != nil {
		return p, "", true, errors.New("malformed native forge comment provenance")
	}
	prefix := body[:index]
	header := forgeCommentHeader(p)
	if !strings.HasPrefix(prefix, header) {
		return p, "", true, errors.New("native forge comment attribution was altered")
	}
	content := strings.TrimPrefix(prefix, header)
	reconstructed, err := forgeFormatComment(p, content)
	if err != nil || reconstructed != body {
		return p, "", true, errors.New("native forge comment content does not match provenance")
	}
	return p, content, true, nil
}
func forgeCommentVersion(p forgeCommentProvenance) string {
	return p.SourceKey + "\n" + p.ID + "\n" + p.UpdatedAt + "\n" + p.Digest
}

type forgePreparedComment struct {
	provenance forgeCommentProvenance
	body       string
}
type forgePreparedImport struct {
	source               forgeSource
	sourceJSON, baseline string
	state                IssueState
	comments             []forgePreparedComment
}

func forgePrepareImport(provider forge.Provider, project string, item forge.Issue, comments []forge.Comment) (forgePreparedImport, error) {
	p := forgePreparedImport{}
	if item.IsChangeRequest {
		return p, errors.New("change requests are not issue imports")
	}
	p.source = forgeSource{forgeSourceSchema, provider.Kind(), provider.Endpoint(), project, item.ID, item.Key, item.URL, item.Author, item.CreatedAt, item.UpdatedAt}
	if err := forgeValidateSource(p.source); err != nil {
		return p, err
	}
	sourceJSON, err := forgeMetadata(p.source)
	if err != nil {
		return p, err
	}
	p.sourceJSON = sourceJSON
	state, err := CanonicalIssueState(IssueState{Title: item.Title, Body: item.Body, Status: item.State, Labels: item.Labels, Assignees: item.Assignees, Metadata: map[string]string{}})
	if err != nil {
		return p, errors.New("external issue cannot be represented within native issue bounds")
	}
	p.baseline, err = forgeManagedDigest(state)
	if err != nil {
		return p, err
	}
	state.Metadata["forge/source"] = sourceJSON
	state.Metadata["forge/observation"] = sourceJSON
	state.Metadata["forge/baseline"] = p.baseline
	p.state = state
	seen := map[string]bool{}
	for _, comment := range comments {
		if seen[comment.ID] {
			return p, errors.New("duplicate external source comment identity")
		}
		seen[comment.ID] = true
		provenance := forgeCommentProvenance{forgeCommentSchema, provider.Kind(), forgeSourceKey(p.source), comment.ID, comment.URL, comment.Author, comment.CreatedAt, comment.UpdatedAt, eventID([]byte(comment.Body))}
		body, err := forgeFormatComment(provenance, comment.Body)
		if err != nil {
			return p, err
		}
		p.comments = append(p.comments, forgePreparedComment{provenance, body})
	}
	return p, nil
}
func forgeSourceMappings() (map[string]issueDetail, error) {
	rows, err := forgeNativeIssues()
	if err != nil {
		return nil, err
	}
	result := map[string]issueDetail{}
	for _, row := range rows {
		raw, ok := row.OpeningMetadata["forge/source"]
		if !ok {
			continue
		}
		source, err := forgeParseSource(raw)
		if err != nil {
			return nil, err
		}
		key := forgeSourceKey(source)
		if _, exists := result[key]; exists {
			return nil, errors.New("duplicate native claims for one external issue; inspect source mappings")
		}
		result[key] = row
	}
	return result, nil
}

func forgeImport(ctx context.Context, provider forge.Provider, o transferOptions) error {
	scope := provider.Kind() + "\n" + provider.Endpoint() + "\n" + o.project
	if o.dryRun {
		var pending forgeNativeIntent
		found, err := forgeLoadJSON("native-intent.json", &pending)
		if err != nil {
			return err
		}
		if found {
			return errors.New("pending exact native import intent requires recovery before dry-run")
		}
	} else if err := forgeResumeNative(scope); err != nil {
		return err
	}
	var items []forge.Issue
	var err error
	if o.key != "" {
		item, readErr := provider.GetIssue(ctx, o.project, o.key)
		err = readErr
		if err == nil {
			items = []forge.Issue{item}
		}
	} else {
		items, err = provider.ListIssues(ctx, o.project)
	}
	if err != nil {
		return err
	}
	completed := 0
	seen := map[string]bool{}
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("import stopped after %d issues: command deadline or cancellation", completed)
		}
		if seen[item.ID] {
			return errors.New("duplicate external issue identity")
		}
		seen[item.ID] = true
		comments, err := provider.ListComments(ctx, o.project, item.Key)
		if err != nil {
			return fmt.Errorf("import stopped after %d issues: %w", completed, err)
		}
		prepared, err := forgePrepareImport(provider, o.project, item, comments)
		if err != nil {
			return fmt.Errorf("import stopped after %d issues: %w", completed, err)
		}
		if err := forgeApplyImport(ctx, scope, prepared, o.dryRun); err != nil {
			return fmt.Errorf("import stopped after %d issues: %w", completed, err)
		}
		completed++
	}
	fmt.Printf("%s %d issues from %s\n", map[bool]string{true: "Validated", false: "Imported"}[o.dryRun], completed, oneLine(provider.Kind()))
	return nil
}
func forgeApplyImport(ctx context.Context, scope string, p forgePreparedImport, dryRun bool) error {
	mappings, err := forgeSourceMappings()
	if err != nil {
		return err
	}
	existing, found := mappings[forgeSourceKey(p.source)]
	// Export footers cannot prove a native binding. Refuse unproven reverse
	// remapping rather than creating a second native issue or importing markers.
	if strings.Contains(p.state.Body, forgeExportIssuePrefix) {
		return errors.New("external ticket carries native export provenance; reverse remapping requires verified source association")
	}
	state := p.state
	id := ""
	expected := []string{}
	if found {
		if existing.Conflict || existing.State == nil {
			return errors.New("mapped native issue is conflicted; resolve it before importing")
		}
		initial := existing.OpeningMetadata["forge/source"]
		if existing.State.Metadata["forge/source"] != initial {
			return errors.New("current source metadata differs from immutable opening provenance")
		}
		baseline := existing.State.Metadata["forge/baseline"]
		if !validEventID(baseline) {
			return errors.New("mapped issue has no valid import baseline")
		}
		localDigest, err := forgeManagedDigest(*existing.State)
		if err != nil {
			return err
		}
		if p.baseline != baseline && localDigest != baseline && localDigest != p.baseline {
			return errors.New("local and external issue fields both changed; resolve the competing edits explicitly")
		}
		state = *existing.State
		if p.baseline != baseline {
			state.Title = p.state.Title
			state.Body = p.state.Body
			state.Status = p.state.Status
			state.Labels = p.state.Labels
			state.Assignees = p.state.Assignees
		}
		metadata := map[string]string{}
		for k, v := range state.Metadata {
			metadata[k] = v
		}
		state.Metadata = metadata
		state.Metadata["forge/source"] = initial
		state.Metadata["forge/baseline"] = p.baseline
		state.Metadata["forge/observation"] = p.sourceJSON
		id = existing.ID
		current, err := ghReadIssue(id)
		if err != nil {
			return err
		}
		if current.Conflict || current.State == nil || len(current.HeadIDs) != 1 {
			return errors.New("mapped native issue changed or is conflicted")
		}
		// Do not substitute a refreshed state for the exact catalog observation.
		observed, _ := json.Marshal(existing.State)
		latest, _ := json.Marshal(current.State)
		if string(observed) != string(latest) {
			return errors.New("native issue changed while preparing import; retry without overwriting")
		}
		expected = current.HeadIDs
	}
	canonical, err := CanonicalIssueState(state)
	if err != nil {
		return errors.New("merged import exceeds native state bounds")
	}
	state = canonical
	known := map[string]bool{}
	if found {
		nativeComments, err := forgeNativeComments(id)
		if err != nil {
			return err
		}
		for _, comment := range nativeComments {
			provenance, _, present, err := forgeParseComment(comment.Body)
			if err != nil {
				return err
			}
			if !present {
				continue
			}
			key := forgeCommentVersion(provenance)
			if known[key] {
				return errors.New("duplicate native source comment claims")
			}
			known[key] = true
		}
	}
	var actor string
	var identity *Identity
	if !dryRun {
		identity, err = inspectIdentity()
		if err != nil {
			return errors.New("native signing identity unavailable; inspect identity configuration")
		}
		actor = identity.Actor
	} else {
		identity = &Identity{Actor: strings.Repeat("a", 64), PublicKey: strings.Repeat("A", 43)}
	}
	rootIntent := forgeNativeIntent{Schema: "hn.forge-native-intent/1", Scope: scope, Actor: actor, Command: "open", State: &state, Operation: forgeOperation("open", forgeSourceKey(p.source), p.baseline)}
	if found {
		rootIntent.Command = "revise"
		rootIntent.IssueID = id
		rootIntent.Expected = expected
		rootIntent.Operation = forgeOperation("revise", forgeSourceKey(p.source), strings.Join(expected, ","), forgeDigestJSON(state))
	}
	if dryRun {
		rootIntent.Actor = identity.Actor
	}
	if err := forgeValidateIntent(rootIntent, identity); err != nil {
		return err
	}
	// Prevalidate every complete attributed comment against the actual signer
	// before appending the first event of this issue's multi-event import.
	for _, comment := range p.comments {
		dummyID := id
		if dummyID == "" {
			dummyID = "sha256:" + strings.Repeat("a", 64)
		}
		intent := forgeNativeIntent{Schema: "hn.forge-native-intent/1", Scope: scope, Actor: rootIntent.Actor, Command: "comment", IssueID: dummyID, Body: comment.body, Operation: forgeOperation("comment", forgeCommentVersion(comment.provenance))}
		if err := forgeValidateIntent(intent, identity); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return errors.New("transfer command deadline or cancellation")
	}
	if dryRun {
		return nil
	}
	changed := true
	if found {
		before, _ := json.Marshal(existing.State)
		after, _ := json.Marshal(state)
		changed = string(before) != string(after)
	}
	if changed {
		result, err := forgeRunNative(rootIntent)
		if err != nil {
			return err
		}
		id = result.IssueID
	}
	for _, comment := range p.comments {
		if err := ctx.Err(); err != nil {
			return errors.New("transfer command deadline or cancellation")
		}
		if known[forgeCommentVersion(comment.provenance)] {
			continue
		}
		intent := forgeNativeIntent{Schema: "hn.forge-native-intent/1", Scope: scope, Actor: actor, Command: "comment", IssueID: id, Body: comment.body, Operation: forgeOperation("comment", forgeCommentVersion(comment.provenance))}
		if _, err := forgeRunNative(intent); err != nil {
			return err
		}
	}
	return nil
}
func forgeDigestJSON(value any) string { raw, _ := json.Marshal(value); return eventID(raw) }
func forgeOperation(parts ...string) string {
	return "forge-" + strings.TrimPrefix(eventID([]byte(strings.Join(parts, "\x00"))), "sha256:")
}

type forgeNativeIntent struct {
	Schema    string      `json:"schema"`
	Scope     string      `json:"scope"`
	Actor     string      `json:"actor"`
	Command   string      `json:"command"`
	IssueID   string      `json:"issue_id,omitempty"`
	Expected  []string    `json:"expected,omitempty"`
	State     *IssueState `json:"state,omitempty"`
	Body      string      `json:"body,omitempty"`
	Operation string      `json:"operation"`
}

func forgeIntentEvent(i forgeNativeIntent) Event {
	e := Event{Protocol: protocolVersion, Kind: issueEventKind(i.Command), Actor: i.Actor, Intent: i.Command, Subject: i.IssueID, Parents: i.Expected, Issue: i.State, Body: i.Body, Operation: i.Operation}
	if i.Command == "open" && i.State != nil {
		e.Title = i.State.Title
		e.Body = i.State.Body
	}
	e.Request, _ = IssueRequestDigest(e)
	return e
}
func forgeValidateIntent(i forgeNativeIntent, identity *Identity) error {
	if i.Schema != "hn.forge-native-intent/1" || i.Scope == "" || len(i.Scope) > 4096 || !validActorFingerprint(i.Actor) || (i.Command != "open" && i.Command != "revise" && i.Command != "comment") {
		return errors.New("invalid native transfer intent")
	}
	e := forgeIntentEvent(i)
	e.ActorName = identity.Name
	e.PublicKey = identity.PublicKey
	e.Sequence = ^uint64(0)
	e.Timestamp = "2026-10-24T00:00:00.999999999Z"
	e.Previous = "sha256:" + strings.Repeat("f", 64)
	if err := validateEventContent(e); err != nil {
		return errors.New("complete imported event exceeds native protocol bounds")
	}
	return nil
}
func forgeAttemptNative(i forgeNativeIntent) (*issueMutationResult, error) {
	raw, _, _, err := executeIssueQuery(issueOptions{command: "operation", actor: i.Actor, operation: i.Operation, limit: 200})
	if err == nil {
		prior := raw.(*issueOperationResult)
		wanted := forgeIntentEvent(i)
		if prior.Request != wanted.Request || prior.Intent != i.Command || prior.Kind != wanted.Kind || (i.Command != "open" && prior.IssueID != i.IssueID) {
			return nil, errors.New("native transfer operation was bound to another exact intent")
		}
		return &issueMutationResult{IssueID: prior.IssueID, EventID: prior.EventID, Replayed: true}, nil
	}
	var typed *issueCommandError
	if !errors.As(err, &typed) || typed.Code != "not_found" {
		return nil, err
	}
	raw, err = executeIssueMutation(issueOptions{command: i.Command, id: i.IssueID, state: i.State, expected: i.Expected, body: i.Body, actor: i.Actor, operation: i.Operation})
	if err != nil {
		return nil, err
	}
	result := raw.(*issueMutationResult)
	if result.Conflict {
		return nil, errors.New("native import recorded concurrent issue heads; inspect and resolve")
	}
	return result, nil
}
func forgeRunNative(i forgeNativeIntent) (*issueMutationResult, error) {
	var existing forgeNativeIntent
	found, err := forgeLoadJSON("native-intent.json", &existing)
	if err != nil {
		return nil, err
	}
	if found && forgeDigestJSON(existing) != forgeDigestJSON(i) {
		return nil, errors.New("another exact native transfer intent is pending recovery")
	}
	if err := forgeStoreJSON("native-intent.json", i); err != nil {
		return nil, err
	}
	result, err := forgeAttemptNative(i)
	if err != nil {
		return nil, err
	}
	if err := forgeRemoveCheckpoint("native-intent.json"); err != nil {
		return nil, err
	}
	return result, nil
}
func forgeResumeNative(scope string) error {
	var intent forgeNativeIntent
	found, err := forgeLoadJSON("native-intent.json", &intent)
	if err != nil || !found {
		return err
	}
	if intent.Scope != scope {
		return errors.New("pending native import belongs to another explicit provider/project; retry its original command")
	}
	identity, err := inspectIdentity()
	if err != nil {
		identity = &Identity{Actor: intent.Actor, PublicKey: strings.Repeat("A", 43)}
	}
	if err := forgeValidateIntent(intent, identity); err != nil {
		return err
	}
	if _, err := forgeAttemptNative(intent); err != nil {
		return err
	}
	return forgeRemoveCheckpoint("native-intent.json")
}
