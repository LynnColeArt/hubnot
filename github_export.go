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
	"strings"

	"hubnot/internal/forge"
)

// forgeRemoteIntent is a single exact provider write, embedded in the caller's
// private checkpoint. The payload and result digests bind their actual bytes.
// The journal is recovery state; it never grants native or provider authority.
type forgeRemoteIntent struct {
	Schema, Scope, Provider, Endpoint, Kind, Action                 string
	NativeID, NativeRevision, TargetID, TargetKey, ExpectedRevision string
	Payload                                                         json.RawMessage
	PayloadDigest, Phase                                            string
	ResultID, ResultKey, ResultRevision                             string
	ResultPayload                                                   json.RawMessage
	ResultDigest                                                    string
}
type forgeRemoteResult struct {
	ID, Key, Revision string
	Payload           json.RawMessage
	PayloadDigest     string
}

// forgeExecuteRemoteIntent persists prepared and in_flight before a single
// write. An interrupted in_flight write is never repeated: only an exact
// positive reconciliation can confirm it. Callbacks each do at most one write.
func forgeExecuteRemoteIntent(ctx context.Context, i *forgeRemoteIntent, save func() error,
	write func(context.Context, forgeRemoteIntent) (forgeRemoteResult, error),
	reconcile func(context.Context, forgeRemoteIntent) (forgeRemoteResult, bool, error),
	validate func(forgeRemoteIntent, forgeRemoteResult) error) (forgeRemoteResult, error) {
	if err := forgeValidateRemoteIntent(*i); err != nil {
		return forgeRemoteResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return forgeRemoteResult{}, errors.New("transfer deadline or cancellation")
	}
	var result forgeRemoteResult
	if i.Phase == "confirmed" {
		result = forgeRemoteResult{i.ResultID, i.ResultKey, i.ResultRevision, i.ResultPayload, i.ResultDigest}
	} else if i.Phase == "in_flight" {
		found := false
		var err error
		result, found, err = reconcile(ctx, *i)
		if err != nil {
			return result, errors.New("external write reconciliation failed; inspect the target and recovery checkpoint")
		}
		if !found {
			return result, errors.New("external write outcome remains uncertain; a negative scan does not permit another write; reconcile manually")
		}
	} else {
		if err := save(); err != nil {
			return result, err
		}
		i.Phase = "in_flight"
		if err := save(); err != nil {
			return result, err
		}
		var err error
		result, err = write(ctx, *i)
		if err != nil {
			return result, errors.New("external write failed; outcome may be unknown; retry the same transfer to reconcile")
		}
	}
	if !issueIdentifier(result.ID, 1024) || result.Key != "" && !issueIdentifier(result.Key, 256) || result.Revision != "" && !issueIdentifier(result.Revision, 1024) || len(result.Payload) > forgeCheckpointLimit || !json.Valid(result.Payload) || result.PayloadDigest != eventID(result.Payload) {
		return result, errors.New("invalid external write result; inspect the target before recovery")
	}
	if err := validate(*i, result); err != nil {
		return result, err
	}
	i.Phase = "confirmed"
	i.ResultID = result.ID
	i.ResultKey = result.Key
	i.ResultRevision = result.Revision
	i.ResultPayload = result.Payload
	i.ResultDigest = result.PayloadDigest
	if err := save(); err != nil {
		return result, err
	}
	return result, nil
}
func forgeValidateRemoteIntent(i forgeRemoteIntent) error {
	if i.Schema != "hn.forge-remote-intent/1" || !validEventID(i.Scope) || !issueIdentifier(i.Provider, 64) || !forgeValidLocator(i.Endpoint) || !forge.KnownResource(forge.Resource(i.Kind)) || !issueIdentifier(i.Action, 64) || !validEventID(i.NativeID) || !validEventID(i.NativeRevision) || i.TargetID != "" && !issueIdentifier(i.TargetID, 1024) || i.TargetKey != "" && !issueIdentifier(i.TargetKey, 256) || i.ExpectedRevision != "" && !issueIdentifier(i.ExpectedRevision, 1024) || len(i.Payload) > forgeCheckpointLimit || !json.Valid(i.Payload) || i.PayloadDigest != eventID(i.Payload) || (i.Phase != "prepared" && i.Phase != "in_flight" && i.Phase != "confirmed") {
		return errors.New("invalid exact external write intent")
	}
	return nil
}
func forgeRemoteObserved(id, key, revision string, value any) forgeRemoteResult {
	raw, _ := json.Marshal(value)
	return forgeRemoteResult{id, key, revision, raw, eventID(raw)}
}

type forgeExportProjection struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	State     string   `json:"state"`
	Labels    []string `json:"labels"`
	Assignees []string `json:"assignees"`
}

func forgeExportProjectionOf(v forge.IssueInput) (forgeExportProjection, error) {
	s, err := CanonicalIssueState(IssueState{Title: v.Title, Body: v.Body, Status: v.State, Labels: v.Labels, Assignees: v.Assignees})
	if err != nil {
		return forgeExportProjection{}, errors.New("external issue fields exceed supported transfer bounds")
	}
	return forgeExportProjection{s.Title, s.Body, s.Status, s.Labels, s.Assignees}, nil
}
func (p forgeExportProjection) input() forge.IssueInput {
	return forge.IssueInput{Title: p.Title, Body: p.Body, State: p.State, Labels: p.Labels, Assignees: p.Assignees}
}
func forgeExportRemoteProjection(v forge.Issue) (forgeExportProjection, error) {
	if v.IsChangeRequest || !issueIdentifier(v.ID, 1024) || !issueIdentifier(v.Key, 256) || !forgeValidURL(v.URL, false) || !forgeValidTime(v.CreatedAt) || !forgeValidTime(v.UpdatedAt) {
		return forgeExportProjection{}, errors.New("invalid external issue identity or change-request mapping")
	}
	return forgeExportProjectionOf(forge.IssueInput{Title: v.Title, Body: v.Body, State: v.State, Labels: v.Labels, Assignees: v.Assignees})
}
func forgeExportSame(a, b forgeExportProjection) bool {
	return forgeDigestJSON(a) == forgeDigestJSON(b)
}
func forgeExportNativeState(s IssueState) (IssueState, error) {
	copy := s
	copy.Metadata = map[string]string{}
	for k, v := range s.Metadata {
		if k != "forge/source" && k != "forge/baseline" && k != "forge/observation" {
			copy.Metadata[k] = v
		}
	}
	return CanonicalIssueState(copy)
}
func forgeExportNativeDigest(s IssueState) (string, error) {
	v, err := forgeExportNativeState(s)
	if err != nil {
		return "", err
	}
	return forgeDigestJSON(v), nil
}

type forgeIssueExportSnapshot struct {
	ID, Actor, Head, NativeDigest string
	State                         IssueState
	CommentIDs                    []string
	SameSource                    bool
	SourceKey                     string
}
type forgeIssueExportCheckpoint struct {
	Schema, Provider, Endpoint, Project, NativeID string
	Signer, SignerPublicKey, Signature            string
	RemoteID, RemoteKey                           string
	NativeBaseline                                string
	RemoteBaseline                                *forgeExportProjection
	Snapshot                                      *forgeIssueExportSnapshot
	IssueDone, PreserveRemote                     bool
	Comments                                      map[string]string
	Pending                                       *forgeRemoteIntent
}

func init() { forgeExport = forgeExportIssue; forgeSkipExportedComment = forgeSkipNativeExportComment }

func forgeExportCurrentSnapshot(id string) (forgeIssueExportSnapshot, map[string]issueHistoryItem, error) {
	view, err := ghReadIssue(id)
	if err != nil {
		return forgeIssueExportSnapshot{}, nil, err
	}
	if view.Conflict || view.State == nil || len(view.HeadIDs) != 1 {
		return forgeIssueExportSnapshot{}, nil, errors.New("native issue is conflicted; resolve before exporting")
	}
	s := forgeIssueExportSnapshot{ID: view.ID, Actor: view.Creator, Head: view.HeadIDs[0], State: *view.State, CommentIDs: []string{}}
	s.NativeDigest, err = forgeExportNativeDigest(s.State)
	if err != nil {
		return s, nil, err
	}
	comments, err := forgeNativeComments(id)
	if err != nil {
		return s, nil, err
	}
	byID := map[string]issueHistoryItem{}
	for _, c := range comments {
		if byID[c.ID].ID != "" {
			return s, nil, errors.New("duplicate signed native comment")
		}
		byID[c.ID] = c
		s.CommentIDs = append(s.CommentIDs, c.ID)
	}
	return s, byID, nil
}
func forgeValidateExportSnapshot(s forgeIssueExportSnapshot) (map[string]issueHistoryItem, error) {
	if !validEventID(s.ID) || !validEventID(s.Head) || !validActorFingerprint(s.Actor) {
		return nil, errors.New("invalid native export snapshot")
	}
	view, err := ghReadIssue(s.ID)
	if err != nil {
		return nil, err
	}
	if view.Creator != s.Actor || view.Conflict {
		return nil, errors.New("native issue identity changed or is conflicted")
	}
	revisions, err := forgeNativeRevisions(s.ID)
	if err != nil {
		return nil, err
	}
	found := false
	for _, r := range revisions {
		if r.ID == s.Head && r.State != nil && forgeDigestJSON(*r.State) == forgeDigestJSON(s.State) {
			found = true
		}
	}
	digest, err := forgeExportNativeDigest(s.State)
	if err != nil || !found || digest != s.NativeDigest {
		return nil, errors.New("export snapshot does not match verified native revision bytes")
	}
	rows, err := forgeNativeComments(s.ID)
	if err != nil {
		return nil, err
	}
	all := map[string]issueHistoryItem{}
	for _, c := range rows {
		all[c.ID] = c
	}
	selected := map[string]issueHistoryItem{}
	for _, id := range s.CommentIDs {
		c, ok := all[id]
		if !ok || selected[id].ID != "" {
			return nil, errors.New("export comment plan does not match verified native history")
		}
		selected[id] = c
	}
	return selected, nil
}
func forgeExportSource(s forgeIssueExportSnapshot) (*forgeSource, error) {
	rows, err := forgeNativeIssues()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ID == s.ID {
			initial := row.OpeningMetadata["forge/source"]
			if initial == "" {
				if s.State.Metadata["forge/source"] != "" || s.State.Metadata["forge/baseline"] != "" || s.State.Metadata["forge/observation"] != "" {
					return nil, errors.New("native issue contains unverified import metadata")
				}
				return nil, nil
			}
			if s.State.Metadata["forge/source"] != initial {
				return nil, errors.New("source provenance differs from immutable opening")
			}
			if err := forgeVerifyImportObservation(s.ID, &s.State, initial); err != nil {
				return nil, err
			}
			source, err := forgeParseSource(s.State.Metadata["forge/observation"])
			if err != nil {
				return nil, err
			}
			return &source, nil
		}
	}
	return nil, errors.New("native issue missing from verified catalog")
}
func forgeExportRender(s forgeIssueExportSnapshot) (forgeExportProjection, error) {
	state, err := forgeExportNativeState(s.State)
	if err != nil {
		return forgeExportProjection{}, err
	}
	if s.SameSource {
		if len(state.Criteria) > 0 || len(state.Relations) > 0 || len(state.Metadata) > 0 {
			return forgeExportProjection{}, errors.New("native-only criteria, relations or metadata require an explicit source-body mapping; no lossless mapping is configured")
		}
		return forgeExportProjectionOf(forge.IssueInput{Title: state.Title, Body: state.Body, State: state.Status, Labels: state.Labels, Assignees: state.Assignees})
	}
	body := fmt.Sprintf("Hubnot issue %s; source actor %s\n\n", s.ID, s.Actor) + state.Body
	if len(state.Criteria) > 0 || len(state.Relations) > 0 || len(state.Metadata) > 0 {
		extra := struct {
			Criteria  []IssueCriterion  `json:"criteria"`
			Relations []IssueRelation   `json:"relations"`
			Metadata  map[string]string `json:"metadata"`
		}{state.Criteria, state.Relations, state.Metadata}
		raw, _ := json.MarshalIndent(extra, "", "  ")
		body += "\n\nNative Hubnot fields (descriptive):\n```json\n" + string(raw) + "\n```"
	}
	if original := s.State.Metadata["forge/source"]; original != "" {
		body += "\n\nOriginal source claims:\n```json\n" + original + "\n```"
	}
	body, err = forgeFormatIssueExport(forgeIssueProvenance{"hn.forge-issue/1", s.ID, s.Actor, s.Head, eventID([]byte(body))}, body)
	if err != nil {
		return forgeExportProjection{}, err
	}
	return forgeExportProjectionOf(forge.IssueInput{Title: state.Title, Body: body, State: state.Status, Labels: state.Labels, Assignees: state.Assignees})
}

const forgeExportCommentPrefix = "\n\n<!-- hn:forge-export-comment:"

type forgeExportCommentProof struct{ Schema, Provider, Endpoint, RemoteIssueID, IssueID, CommentID, Actor, Timestamp, Digest string }

func forgeExportCommentContent(c issueHistoryItem) (string, error) {
	p, raw, present, err := forgeParseComment(c.Body)
	if err != nil && strings.HasPrefix(c.Operation, "forge-") {
		return "", err
	}
	if err == nil && present && c.Operation == forgeOperation("comment", forgeCommentVersion(p)) {
		return forgeCommentHeader(p) + raw, nil
	}
	return c.Body, nil
}
func forgeFormatExportComment(provider, endpoint, remoteID, issueID string, c issueHistoryItem) (string, error) {
	content, err := forgeExportCommentContent(c)
	if err != nil {
		return "", err
	}
	body := fmt.Sprintf("Hubnot comment %s; actor %s; created %s\n\n", c.ID, c.Actor, c.Timestamp) + content
	p := forgeExportCommentProof{"hn.forge-export-comment/1", provider, endpoint, remoteID, issueID, c.ID, c.Actor, c.Timestamp, eventID([]byte(body))}
	raw, _ := json.Marshal(p)
	result := body + forgeExportCommentPrefix + base64.RawURLEncoding.EncodeToString(raw) + " -->"
	if !issueIdentifier(provider, 64) || !forgeValidURL(endpoint, true) || !issueIdentifier(remoteID, 1024) || !validEventID(issueID) || !validEventID(c.ID) || !validActorFingerprint(c.Actor) || !forgeValidTime(c.Timestamp) || !issueText(result, 65536, true) {
		return "", errors.New("attributed native comment exceeds supported transfer bounds")
	}
	return result, nil
}
func forgeParseExportComment(body string) (forgeExportCommentProof, bool) {
	var p forgeExportCommentProof
	index := strings.LastIndex(body, forgeExportCommentPrefix)
	if index < 0 || !strings.HasSuffix(body, " -->") {
		return p, false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(body[index+len(forgeExportCommentPrefix) : len(body)-4])
	if err != nil || len(raw) > 8192 || forgeStrictJSON(raw, &p) != nil || p.Schema != "hn.forge-export-comment/1" || !issueIdentifier(p.Provider, 64) || !forgeValidURL(p.Endpoint, true) || !issueIdentifier(p.RemoteIssueID, 1024) || !validEventID(p.IssueID) || !validEventID(p.CommentID) || !validActorFingerprint(p.Actor) || !forgeValidTime(p.Timestamp) || p.Digest != eventID([]byte(body[:index])) {
		return p, false
	}
	return p, true
}
func forgeSkipSourceComment(s forgeIssueExportSnapshot, c issueHistoryItem) (bool, error) {
	p, _, present, err := forgeParseComment(c.Body)
	if err != nil && strings.HasPrefix(c.Operation, "forge-") {
		return false, err
	}
	return err == nil && present && s.SameSource && p.SourceKey == s.SourceKey && c.Operation == forgeOperation("comment", forgeCommentVersion(p)), nil
}
func forgeSkipNativeExportComment(source forgeSource, id, body string, comments []issueHistoryItem) (bool, error) {
	p, present := forgeParseExportComment(body)
	if !present || p.Provider != source.Provider || p.Endpoint != source.Endpoint || p.RemoteIssueID != source.ID || p.IssueID != id {
		return false, nil
	}
	for _, c := range comments {
		if c.ID == p.CommentID {
			if c.Actor != p.Actor || c.Timestamp != p.Timestamp {
				return false, nil
			}
			wanted, err := forgeFormatExportComment(source.Provider, source.Endpoint, source.ID, id, c)
			return err == nil && wanted == body, nil
		}
	}
	return false, nil
}

func forgeExportScope(p forge.Provider, o transferOptions) string {
	return eventID([]byte(p.Kind() + "\n" + p.Endpoint() + "\n" + o.project + "\nissue\n" + o.id))
}
func forgeExportCheckpointName(scope string) string {
	return "issue-export-" + strings.TrimPrefix(scope, "sha256:") + ".json"
}
func forgeExportCheckCheckpoint(c forgeIssueExportCheckpoint, p forge.Provider, o transferOptions, scope string) error {
	if c.Schema != "hn.forge-issue-export/1" || c.Provider != p.Kind() || c.Endpoint != p.Endpoint() || c.Project != o.project || c.NativeID != o.id || c.NativeBaseline != "" && !validEventID(c.NativeBaseline) || c.RemoteID != "" && !issueIdentifier(c.RemoteID, 1024) || c.RemoteKey != "" && !issueIdentifier(c.RemoteKey, 256) || c.Comments == nil {
		return errors.New("export checkpoint identity is invalid")
	}
	if (c.RemoteID == "") != (c.RemoteKey == "") {
		return errors.New("incomplete external checkpoint identity")
	}
	if c.RemoteBaseline != nil {
		if _, err := forgeExportProjectionOf(c.RemoteBaseline.input()); err != nil {
			return err
		}
	}
	for native, remote := range c.Comments {
		if !validEventID(native) || !issueIdentifier(remote, 1024) {
			return errors.New("invalid exported comment checkpoint")
		}
	}
	if c.Snapshot != nil && c.Snapshot.ID != c.NativeID {
		return errors.New("checkpoint snapshot belongs to another issue")
	}
	if c.Pending != nil {
		if c.Snapshot == nil || c.Pending.Scope != scope || c.Pending.Provider != p.Kind() || c.Pending.Endpoint != p.Endpoint() || c.Pending.NativeID != o.id || c.Pending.NativeRevision != c.Snapshot.Head {
			return errors.New("checkpoint pending intent belongs to another snapshot")
		}
		return forgeValidateRemoteIntent(*c.Pending)
	}
	return nil
}
func forgeExportList(ctx context.Context, p forge.Provider, project, id string) ([]forge.Issue, error) {
	rows, err := p.ListIssues(ctx, project)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	claims := 0
	issues := make([]forge.Issue, 0, len(rows))
	for _, v := range rows {
		if v.IsChangeRequest {
			if proof, _, ok := forgeParseIssueExport(v.Body); ok && proof.ID == id {
				return nil, errors.New("native issue marker points to an external change request")
			}
			continue
		}
		if _, err := forgeExportRemoteProjection(v); err != nil {
			return nil, err
		}
		if seen[v.ID] {
			return nil, errors.New("duplicate external issue identity")
		}
		seen[v.ID] = true
		issues = append(issues, v)
		if proof, _, ok := forgeParseIssueExport(v.Body); ok && proof.ID == id {
			claims++
		}
	}
	if claims > 1 {
		return nil, errors.New("multiple external tickets claim this native issue; reconcile mappings manually")
	}
	return issues, nil
}
func forgeExportSourceBaseline(s forgeIssueExportSnapshot) (forgeExportProjection, string, error) {
	baseline := s.State.Metadata["forge/baseline"]
	rows, err := forgeNativeRevisions(s.ID)
	if err != nil {
		return forgeExportProjection{}, "", err
	}
	for _, r := range rows {
		if r.State == nil {
			continue
		}
		digest, err := forgeManagedDigest(*r.State)
		if err == nil && digest == baseline && r.State.Metadata["forge/source"] == s.State.Metadata["forge/source"] {
			projection, err := forgeExportProjectionOf(forge.IssueInput{Title: r.State.Title, Body: r.State.Body, State: r.State.Status, Labels: r.State.Labels, Assignees: r.State.Assignees})
			native, err2 := forgeExportNativeDigest(*r.State)
			if err != nil {
				return projection, "", err
			}
			return projection, native, err2
		}
	}
	return forgeExportProjection{}, "", errors.New("cannot reconstruct signed source baseline; import/reconcile before exporting")
}
func forgeExportFindInitial(ctx context.Context, p forge.Provider, o transferOptions, s *forgeIssueExportSnapshot, source *forgeSource, c *forgeIssueExportCheckpoint) error {
	rows, err := forgeExportList(ctx, p, o.project, s.ID)
	if err != nil {
		return err
	}
	var matches []forge.Issue
	for _, v := range rows {
		bySource := source != nil && source.Provider == p.Kind() && source.Endpoint == p.Endpoint() && v.ID == source.ID
		proof, _, byMarker := forgeParseIssueExport(v.Body)
		byMarker = byMarker && proof.ID == s.ID
		if bySource || byMarker {
			matches = append(matches, v)
			if bySource {
				s.SameSource = true
				s.SourceKey = forgeSourceKey(*source)
			}
		}
	}
	if len(matches) > 1 {
		return errors.New("ambiguous external issue/source mapping")
	}
	desired, err := forgeExportRender(*s)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		if source != nil && source.Provider == p.Kind() && source.Endpoint == p.Endpoint() && source.Project == o.project {
			return errors.New("original external source is absent or no longer an issue; reconcile instead of creating a replacement")
		}
		return nil
	}
	v := matches[0]
	remote, err := forgeExportRemoteProjection(v)
	if err != nil {
		return err
	}
	c.RemoteID = v.ID
	c.RemoteKey = v.Key
	if s.SameSource {
		baseline, native, err := forgeExportSourceBaseline(*s)
		if err != nil {
			return err
		}
		c.RemoteBaseline = &baseline
		c.NativeBaseline = native
	} else {
		proof, _, _ := forgeParseIssueExport(v.Body)
		if proof.Actor != s.Actor || proof.Head != s.Head || !forgeExportSame(remote, desired) {
			return errors.New("external native marker lacks an exact recoverable baseline; reconcile mapping manually")
		}
		c.RemoteBaseline = &remote
		c.NativeBaseline = s.NativeDigest
	}
	return nil
}
func forgeExportFindCurrent(ctx context.Context, p forge.Provider, o transferOptions, c *forgeIssueExportCheckpoint) (forge.Issue, error) {
	rows, err := forgeExportList(ctx, p, o.project, o.id)
	if err != nil {
		return forge.Issue{}, err
	}
	for _, v := range rows {
		if v.ID == c.RemoteID {
			c.RemoteKey = v.Key
			return v, nil
		}
	}
	return forge.Issue{}, errors.New("mapped external issue is absent from the selected project; reconcile its identity instead of recreating it")
}

func forgeExportFetchCurrent(ctx context.Context, p forge.Provider, o transferOptions, c *forgeIssueExportCheckpoint) (forge.Issue, error) {
	v, err := forgeExportFindCurrent(ctx, p, o, c)
	if err != nil {
		return v, err
	}
	if c.Snapshot == nil {
		return v, errors.New("external association has no native snapshot")
	}
	return v, forgeExportVerifyAssociation(v, *c.Snapshot)
}

func forgeExportAuthenticateSnapshot(s forgeIssueExportSnapshot, c forgeIssueExportCheckpoint, p forge.Provider) error {
	if !s.SameSource {
		if s.SourceKey != "" {
			return errors.New("non-source export snapshot has an inconsistent source key")
		}
		return nil
	}
	source, err := forgeExportSource(s)
	if err != nil {
		return err
	}
	if source == nil || source.Provider != p.Kind() || source.Endpoint != p.Endpoint() || s.SourceKey != forgeSourceKey(*source) || c.RemoteID == "" || c.RemoteID != source.ID {
		return errors.New("source-body export mode requires verified signed source provenance and the exact provider destination")
	}
	return nil
}

func forgeExportVerifyAssociation(v forge.Issue, snapshot forgeIssueExportSnapshot) error {
	if snapshot.SameSource {
		source, err := forgeExportSource(snapshot)
		if err != nil {
			return err
		}
		if source == nil || source.ID != v.ID || forgeSourceKey(*source) != snapshot.SourceKey {
			return errors.New("external source identity no longer matches verified import provenance")
		}
		return nil
	}
	proof, _, present := forgeParseIssueExport(v.Body)
	if !present || proof.ID != snapshot.ID || proof.Actor != snapshot.Actor {
		return errors.New("external issue association lost its verified native provenance; reconcile manually")
	}
	rows, err := forgeNativeRevisions(snapshot.ID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.ID != proof.Head || row.State == nil {
			continue
		}
		historical := forgeIssueExportSnapshot{ID: snapshot.ID, Actor: snapshot.Actor, Head: row.ID, State: *row.State}
		expected, err := forgeExportRender(historical)
		if err != nil {
			return err
		}
		if expected.Body == v.Body {
			return nil
		}
	}
	return errors.New("external issue marker does not match an actual signed native revision")
}

func forgeExportValidatePending(cp forgeIssueExportCheckpoint, desired forgeExportProjection, comments map[string]issueHistoryItem) error {
	if cp.Pending == nil {
		return nil
	}
	i := cp.Pending
	if i.TargetID != cp.RemoteID {
		return errors.New("pending external write changed its immutable destination")
	}
	switch i.Action {
	case "create-issue", "update-issue":
		var payload forgeExportProjection
		if forgeStrictJSON(i.Payload, &payload) != nil {
			return errors.New("invalid pending issue payload")
		}
		expected := desired
		if i.Action == "create-issue" {
			expected.State = "open"
			if cp.RemoteID != "" {
				return errors.New("create intent already has a target")
			}
		}
		if !forgeExportSame(payload, expected) {
			return errors.New("pending external issue payload differs from its signed native snapshot")
		}
	case "create-comment":
		var payload struct{ NativeID, Body string }
		if forgeStrictJSON(i.Payload, &payload) != nil {
			return errors.New("invalid pending comment payload")
		}
		comment, exists := comments[payload.NativeID]
		if !exists {
			return errors.New("pending external comment is outside the selected native snapshot")
		}
		expected, err := forgeFormatExportComment(cp.Provider, cp.Endpoint, cp.RemoteID, cp.NativeID, comment)
		if err != nil {
			return err
		}
		if payload.Body != expected {
			return errors.New("pending external comment differs from its exact signed native content")
		}
	default:
		return errors.New("unsupported pending issue export step")
	}
	return nil
}

func forgeExportIssue(ctx context.Context, p forge.Provider, o transferOptions) error {
	if !validEventID(o.id) || o.provider != p.Kind() || o.endpoint != p.Endpoint() || !issueIdentifier(o.project, 512) {
		return errors.New("invalid selected issue export identity")
	}
	var identity *Identity
	if !o.dryRun {
		var err error
		identity, err = loadIdentity()
		if err != nil {
			return errors.New("real issue export requires the active native signing identity; select the original exporter identity to recover")
		}
		if err = validateIdentity(identity); err != nil {
			return err
		}
		if registration, ok := forgeProviders[p.Kind()]; ok && registration.TokenEnv != "" && os.Getenv(registration.TokenEnv) == "" {
			return errors.New("external writes require the selected provider credential")
		}
	}
	scope := forgeExportScope(p, o)
	name := forgeExportCheckpointName(scope)
	var cp forgeIssueExportCheckpoint
	found, err := forgeLoadJSON(name, &cp)
	if err != nil {
		return err
	}
	if found {
		if err := forgeExportVerifyCheckpoint(name, cp, identity); err != nil {
			return err
		}
		if err := forgeExportCheckCheckpoint(cp, p, o, scope); err != nil {
			return err
		}
	} else {
		cp = forgeIssueExportCheckpoint{Schema: "hn.forge-issue-export/1", Provider: p.Kind(), Endpoint: p.Endpoint(), Project: o.project, NativeID: o.id, Comments: map[string]string{}}
	}
	resuming := cp.Snapshot != nil
	var selected map[string]issueHistoryItem
	if resuming {
		selected, err = forgeValidateExportSnapshot(*cp.Snapshot)
		if err != nil {
			return err
		}
		if o.dryRun {
			return errors.New("an exact export snapshot is pending; resume normally to reconcile before previewing a new transfer")
		}
	} else {
		snapshot, comments, err := forgeExportCurrentSnapshot(o.id)
		if err != nil {
			return err
		}
		selected = comments
		source, err := forgeExportSource(snapshot)
		if err != nil {
			return err
		}
		if !found {
			if err := forgeExportFindInitial(ctx, p, o, &snapshot, source, &cp); err != nil {
				return err
			}
		} else if source != nil && cp.RemoteID == source.ID && source.Provider == p.Kind() && source.Endpoint == p.Endpoint() {
			snapshot.SameSource = true
			snapshot.SourceKey = forgeSourceKey(*source)
		}
		cp.Snapshot = &snapshot
		cp.IssueDone = false
		cp.PreserveRemote = false
	}
	s := cp.Snapshot
	if err := forgeExportAuthenticateSnapshot(*s, cp, p); err != nil {
		return err
	}
	desired, err := forgeExportRender(*s)
	if err != nil {
		return err
	}
	if err := forgeExportValidatePending(cp, desired, selected); err != nil {
		return err
	}
	remoteID := cp.RemoteID
	if remoteID == "" {
		// The provider ID is unknown until creation. Its contract permits
		// 1024 bytes; '<' reserves encoding/json's maximum six-byte escape
		// per permitted byte before the first external mutation.
		remoteID = strings.Repeat("<", 1024)
	}
	for _, id := range s.CommentIDs {
		comment := selected[id]
		skip, err := forgeSkipSourceComment(*s, comment)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if _, err := forgeFormatExportComment(p.Kind(), p.Endpoint(), remoteID, s.ID, comment); err != nil {
			return err
		}
	}
	if err := forgeExportBudget(cp, desired, selected); err != nil {
		return err
	}
	if !resuming && cp.RemoteID != "" {
		remote, err := forgeExportFindCurrent(ctx, p, o, &cp)
		if err != nil {
			return err
		}
		projection, err := forgeExportRemoteProjection(remote)
		if err != nil {
			return err
		}
		if associationErr := forgeExportVerifyAssociation(remote, *s); associationErr != nil {
			// An unchanged local issue must not overwrite an external-only body
			// edit. Without the body association, preserve it read-only; don't
			// publish new comments or adopt it as the next overwrite baseline.
			if cp.RemoteBaseline != nil && s.NativeDigest == cp.NativeBaseline && !forgeExportSame(projection, *cp.RemoteBaseline) {
				for _, id := range s.CommentIDs {
					skip, err := forgeSkipSourceComment(*s, selected[id])
					if err != nil {
						return err
					}
					if !skip && cp.Comments[id] == "" {
						return errors.New("external-only body edit preserved; reconcile native provenance before publishing new comments")
					}
				}
				fmt.Println("Preserved external-only issue edits; no provider writes or baseline changes performed.")
				return nil
			}
			return associationErr
		}
		switch {
		case forgeExportSame(projection, desired):
			cp.IssueDone = true
			cp.RemoteBaseline = &projection
		case cp.RemoteBaseline == nil:
			return errors.New("external mapping has no verified baseline")
		case !forgeExportSame(projection, *cp.RemoteBaseline) && s.NativeDigest == cp.NativeBaseline:
			cp.IssueDone = true
			cp.PreserveRemote = true
		case !forgeExportSame(projection, *cp.RemoteBaseline):
			return errors.New("native and external issue fields both changed; resolve competing edits explicitly")
		case s.NativeDigest == cp.NativeBaseline:
			cp.IssueDone = true
		}
	}
	if o.dryRun {
		fmt.Printf("Validated issue export to %s; no writes performed\n", oneLine(p.Kind()))
		return nil
	}
	if !resuming {
		current, err := ghReadIssue(s.ID)
		if err != nil {
			return err
		}
		if current.Conflict || len(current.HeadIDs) != 1 || current.HeadIDs[0] != s.Head {
			return errors.New("native issue changed during export preparation; retry before publishing")
		}
	}
	save := func() error {
		if err := forgeExportSignCheckpoint(&cp, identity); err != nil {
			return err
		}
		return forgeStoreJSON(name, cp)
	}
	if err := save(); err != nil {
		return err
	}
	for !cp.IssueDone {
		if cp.Pending == nil {
			input := desired
			action := "update-issue"
			expected := ""
			if cp.RemoteID == "" {
				action = "create-issue"
				input.State = "open"
			} else {
				remote, err := forgeExportFetchCurrent(ctx, p, o, &cp)
				if err != nil {
					return err
				}
				projection, err := forgeExportRemoteProjection(remote)
				if err != nil {
					return err
				}
				if forgeExportSame(projection, desired) {
					cp.RemoteBaseline = &projection
					cp.IssueDone = true
					break
				}
				if cp.RemoteBaseline == nil || !forgeExportSame(projection, *cp.RemoteBaseline) {
					return errors.New("external issue changed during pending export; reconcile without overwriting")
				}
				expected = forgeDigestJSON(projection)
			}
			raw, _ := json.Marshal(input)
			cp.Pending = &forgeRemoteIntent{Schema: "hn.forge-remote-intent/1", Scope: scope, Provider: p.Kind(), Endpoint: p.Endpoint(), Kind: string(forge.ResourceIssue), Action: action, NativeID: s.ID, NativeRevision: s.Head, TargetID: cp.RemoteID, TargetKey: cp.RemoteKey, ExpectedRevision: expected, Payload: raw, PayloadDigest: eventID(raw), Phase: "prepared"}
		}
		if cp.Pending.Action != "create-issue" && cp.Pending.Action != "update-issue" {
			return errors.New("checkpoint step is inconsistent with issue publication")
		}
		result, err := forgeExecuteRemoteIntent(ctx, cp.Pending, save,
			func(ctx context.Context, i forgeRemoteIntent) (forgeRemoteResult, error) {
				var input forgeExportProjection
				if forgeStrictJSON(i.Payload, &input) != nil {
					return forgeRemoteResult{}, errors.New("invalid issue payload")
				}
				var v forge.Issue
				var err error
				if i.Action == "create-issue" {
					v, err = p.CreateIssue(ctx, o.project, input.input())
				} else {
					current, readErr := forgeExportFetchCurrent(ctx, p, o, &cp)
					if readErr != nil {
						return forgeRemoteResult{}, readErr
					}
					projection, readErr := forgeExportRemoteProjection(current)
					if readErr != nil || current.ID != i.TargetID || forgeDigestJSON(projection) != i.ExpectedRevision {
						return forgeRemoteResult{}, errors.New("external issue changed before the guarded write")
					}
					v, err = p.UpdateIssue(ctx, o.project, current.Key, input.input())
				}
				if err != nil {
					return forgeRemoteResult{}, err
				}
				return forgeExportObservedIssue(v)
			},
			func(ctx context.Context, i forgeRemoteIntent) (forgeRemoteResult, bool, error) {
				rows, err := forgeExportList(ctx, p, o.project, s.ID)
				if err != nil {
					return forgeRemoteResult{}, false, err
				}
				matches := []forge.Issue{}
				for _, v := range rows {
					if i.TargetID != "" && v.ID == i.TargetID {
						matches = append(matches, v)
					} else if i.TargetID == "" {
						proof, _, ok := forgeParseIssueExport(v.Body)
						if ok && proof.ID == s.ID {
							matches = append(matches, v)
						}
					}
				}
				if len(matches) != 1 {
					return forgeRemoteResult{}, false, errors.New("external issue write is not uniquely reconciled")
				}
				v := matches[0]
				observed, err := forgeExportObservedIssue(v)
				return observed, true, err
			},
			func(i forgeRemoteIntent, r forgeRemoteResult) error {
				var wanted forgeExportProjection
				var v forge.Issue
				if forgeStrictJSON(i.Payload, &wanted) != nil || forgeStrictJSON(r.Payload, &v) != nil {
					return errors.New("invalid issue write checkpoint")
				}
				actual, err := forgeExportRemoteProjection(v)
				if err != nil {
					return err
				}
				if r.ID != v.ID || r.Key != v.Key || i.TargetID != "" && i.TargetID != v.ID || !forgeExportSame(wanted, actual) {
					return errors.New("external issue did not preserve the exact requested identity/fields; reconcile manually")
				}
				if !s.SameSource {
					proof, _, ok := forgeParseIssueExport(v.Body)
					if !ok || proof.ID != s.ID || proof.Actor != s.Actor || proof.Head != s.Head {
						return errors.New("external issue proof does not bind the selected native revision")
					}
				}
				return nil
			})
		if err != nil {
			return err
		}
		var remote forge.Issue
		if err := forgeStrictJSON(result.Payload, &remote); err != nil {
			return err
		}
		projection, err := forgeExportRemoteProjection(remote)
		if err != nil {
			return err
		}
		cp.RemoteID = remote.ID
		cp.RemoteKey = remote.Key
		cp.RemoteBaseline = &projection
		cp.Pending = nil
		cp.IssueDone = forgeExportSame(projection, desired)
		if err := save(); err != nil {
			return err
		}
	}
	if err := forgeExportComments(ctx, p, o, scope, &cp, selected, save); err != nil {
		return err
	}
	if !cp.PreserveRemote {
		cp.NativeBaseline = s.NativeDigest
	}
	cp.Snapshot = nil
	cp.Pending = nil
	cp.IssueDone = false
	cp.PreserveRemote = false
	if err := save(); err != nil {
		return err
	}
	if resuming {
		fmt.Println("Recovered the exact pending issue export snapshot; run export again to publish any newer native changes.")
	} else {
		fmt.Printf("Exported issue to %s\n", oneLine(p.Kind()))
	}
	return nil
}

func forgeExportComments(ctx context.Context, p forge.Provider, o transferOptions, scope string, cp *forgeIssueExportCheckpoint, selected map[string]issueHistoryItem, save func() error) error {
	if _, err := forgeExportFetchCurrent(ctx, p, o, cp); err != nil {
		return err
	}
	rows, err := p.ListComments(ctx, o.project, cp.RemoteKey)
	if err != nil {
		return err
	}
	byID := map[string]forge.Comment{}
	claims := map[string][]forge.Comment{}
	for _, c := range rows {
		if !issueIdentifier(c.ID, 1024) || !issueText(c.Body, 65536, false) || !forgeValidURL(c.URL, false) || !forgeValidTime(c.CreatedAt) || !forgeValidTime(c.UpdatedAt) || byID[c.ID].ID != "" {
			return errors.New("invalid external comment listing")
		}
		byID[c.ID] = c
		if proof, ok := forgeParseExportComment(c.Body); ok && proof.Provider == p.Kind() && proof.Endpoint == p.Endpoint() && proof.RemoteIssueID == cp.RemoteID && proof.IssueID == o.id {
			claims[proof.CommentID] = append(claims[proof.CommentID], c)
		}
	}
	for _, values := range claims {
		if len(values) > 1 {
			return errors.New("multiple external comments claim one native comment")
		}
	}
	for _, id := range cp.Snapshot.CommentIDs {
		comment := selected[id]
		skip, err := forgeSkipSourceComment(*cp.Snapshot, comment)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		body, err := forgeFormatExportComment(p.Kind(), p.Endpoint(), cp.RemoteID, o.id, comment)
		if err != nil {
			return err
		}
		if existing := cp.Comments[id]; existing != "" {
			if values := claims[id]; len(values) > 0 && values[0].ID != existing {
				return errors.New("exported comment identity changed; reconcile claims manually")
			}
			continue
		}
		if values := claims[id]; len(values) == 1 {
			if values[0].Body != body {
				return errors.New("external comment claim does not match exact native content")
			}
			cp.Comments[id] = values[0].ID
			if cp.Pending != nil && cp.Pending.Action == "create-comment" {
				var planned struct{ NativeID, Body string }
				if forgeStrictJSON(cp.Pending.Payload, &planned) != nil || planned.NativeID != id || planned.Body != body {
					return errors.New("comment checkpoint belongs to another native event")
				}
				// Exact positive reconciliation also confirms a prepared intent
				// whose publication was completed elsewhere; it never resends it.
				cp.Pending = nil
			}
			if err := save(); err != nil {
				return err
			}
			continue
		}
		if cp.Pending == nil {
			raw, _ := json.Marshal(struct{ NativeID, Body string }{id, body})
			cp.Pending = &forgeRemoteIntent{Schema: "hn.forge-remote-intent/1", Scope: scope, Provider: p.Kind(), Endpoint: p.Endpoint(), Kind: string(forge.ResourceIssue), Action: "create-comment", NativeID: o.id, NativeRevision: cp.Snapshot.Head, TargetID: cp.RemoteID, TargetKey: cp.RemoteKey, Payload: raw, PayloadDigest: eventID(raw), Phase: "prepared"}
		}
		if cp.Pending.Action != "create-comment" {
			return errors.New("pending external step is not a comment")
		}
		result, err := forgeExecuteRemoteIntent(ctx, cp.Pending, save,
			func(ctx context.Context, i forgeRemoteIntent) (forgeRemoteResult, error) {
				var payload struct{ NativeID, Body string }
				if forgeStrictJSON(i.Payload, &payload) != nil || payload.NativeID != id {
					return forgeRemoteResult{}, errors.New("invalid comment intent")
				}
				current, readErr := forgeExportFetchCurrent(ctx, p, o, cp)
				if readErr != nil || current.ID != i.TargetID {
					return forgeRemoteResult{}, errors.New("external comment target identity changed")
				}
				v, err := p.CreateComment(ctx, o.project, current.Key, payload.Body)
				return forgeRemoteObserved(v.ID, "", "", forge.Comment{ID: v.ID, Body: v.Body}), err
			},
			func(ctx context.Context, i forgeRemoteIntent) (forgeRemoteResult, bool, error) {
				current, readErr := forgeExportFetchCurrent(ctx, p, o, cp)
				if readErr != nil || current.ID != i.TargetID {
					return forgeRemoteResult{}, false, errors.New("external comment reconciliation target changed")
				}
				values, err := p.ListComments(ctx, o.project, current.Key)
				if err != nil {
					return forgeRemoteResult{}, false, err
				}
				var found []forge.Comment
				for _, v := range values {
					proof, ok := forgeParseExportComment(v.Body)
					if ok && proof.Provider == p.Kind() && proof.Endpoint == p.Endpoint() && proof.RemoteIssueID == i.TargetID && proof.IssueID == o.id && proof.CommentID == id {
						found = append(found, v)
					}
				}
				if len(found) != 1 {
					return forgeRemoteResult{}, false, nil
				}
				v := found[0]
				return forgeRemoteObserved(v.ID, "", "", forge.Comment{ID: v.ID, Body: v.Body}), true, nil
			},
			func(i forgeRemoteIntent, r forgeRemoteResult) error {
				var payload struct{ NativeID, Body string }
				var v forge.Comment
				if forgeStrictJSON(i.Payload, &payload) != nil || forgeStrictJSON(r.Payload, &v) != nil || payload.NativeID != id || v.ID != r.ID || !issueIdentifier(v.ID, 1024) || v.Body != payload.Body || v.Body != body {
					return errors.New("external comment does not match exact native publication intent")
				}
				return nil
			})
		if err != nil {
			return err
		}
		cp.Comments[id] = result.ID
		cp.Pending = nil
		if err := save(); err != nil {
			return err
		}
	}
	return nil
}

// Receipts retain only bounded immutable identity and the exact managed fields
// checked by reconciliation. Unbounded descriptive provider extras are not a
// recovery authority and cannot make a future confirmed checkpoint overflow.
func forgeExportObservedIssue(v forge.Issue) (forgeRemoteResult, error) {
	p, err := forgeExportRemoteProjection(v)
	if err != nil {
		return forgeRemoteResult{}, err
	}
	receipt := forge.Issue{ID: v.ID, Key: v.Key, Title: p.Title, Body: p.Body, State: p.State, Labels: p.Labels, Assignees: p.Assignees}
	return forgeRemoteObserved(v.ID, v.Key, "", receipt), nil
}

const forgeIssueCheckpointDomain = "hn.forge-issue-checkpoint-signature/1\x00"

func forgeExportCheckpointBytes(cp forgeIssueExportCheckpoint) ([]byte, error) {
	cp.Signature = ""
	raw, err := json.Marshal(cp)
	if err != nil || len(raw) > forgeCheckpointLimit {
		return nil, errors.New("issue checkpoint signature payload exceeds supported bounds")
	}
	return append([]byte(forgeIssueCheckpointDomain), raw...), nil
}
func forgeExportSignCheckpoint(cp *forgeIssueExportCheckpoint, identity *Identity) error {
	if identity == nil {
		return errors.New("real issue export requires the active native signing identity")
	}
	cp.Signer = identity.Actor
	cp.SignerPublicKey = identity.PublicKey
	payload, err := forgeExportCheckpointBytes(*cp)
	if err != nil {
		return err
	}
	key, err := identity.privateKey()
	if err != nil {
		return err
	}
	cp.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(key, payload))
	return nil
}
func forgeExportVerifyCheckpoint(name string, cp forgeIssueExportCheckpoint, identity *Identity) error {
	path, err := forgeCheckpointPath(name)
	if err != nil {
		return err
	}
	raw, err := readPrivateFileBounded(path, forgeCheckpointLimit)
	if err != nil {
		return errors.New("issue checkpoint cannot be authenticated")
	}
	canonical, err := json.Marshal(cp)
	if err != nil || !bytes.Equal(raw, canonical) {
		return errors.New("issue checkpoint is not canonical signed JSON; explicit recovery required")
	}
	public, err := base64.RawStdEncoding.DecodeString(cp.SignerPublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize || base64.RawStdEncoding.EncodeToString(public) != cp.SignerPublicKey || actorForPublicKey(public) != cp.Signer {
		return errors.New("issue checkpoint signer is invalid; explicit recovery required")
	}
	if identity != nil && (identity.Actor != cp.Signer || identity.PublicKey != cp.SignerPublicKey) {
		return errors.New("issue checkpoint belongs to a different native identity; select the original exporter identity for recovery")
	}
	signature, err := base64.RawStdEncoding.DecodeString(cp.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize || base64.RawStdEncoding.EncodeToString(signature) != cp.Signature {
		return errors.New("issue checkpoint signature missing or invalid; explicit recovery required")
	}
	payload, err := forgeExportCheckpointBytes(cp)
	if err != nil {
		return err
	}
	if !ed25519.Verify(public, payload, signature) {
		return errors.New("issue checkpoint signature does not match complete recovery state; explicit recovery required")
	}
	return nil
}

// Reserve the complete largest future journal, not merely today's snapshot.
// Known receipt IDs stay exact; all unpublished IDs reserve the contract's
// 1024-byte maximum with encoding/json's largest permitted-byte escape.
func forgeExportBudget(cp forgeIssueExportCheckpoint, desired forgeExportProjection, selected map[string]issueHistoryItem) error {
	if cp.Snapshot == nil {
		return errors.New("issue export budget requires selected snapshot")
	}
	worst := strings.Repeat("<", 1024)
	key := strings.Repeat("<", 256)
	if cp.RemoteID == "" {
		cp.RemoteID = worst
		cp.RemoteKey = key
	}
	cp.Signer = strings.Repeat("a", 64)
	cp.SignerPublicKey = strings.Repeat("a", 43)
	cp.Signature = strings.Repeat("a", 86)
	receipts := map[string]string{}
	for id, remote := range cp.Comments {
		receipts[id] = remote
	}
	cp.Comments = receipts
	largest := cp.Pending
	maxPending := 0
	consider := func(i forgeRemoteIntent) {
		raw, _ := json.Marshal(i)
		if len(raw) > maxPending {
			copy := i
			largest = &copy
			maxPending = len(raw)
		}
	}
	if largest != nil {
		consider(*largest)
	}
	raw, _ := json.Marshal(desired)
	result := forge.Issue{ID: cp.RemoteID, Key: cp.RemoteKey, Title: desired.Title, Body: desired.Body, State: desired.State, Labels: desired.Labels, Assignees: desired.Assignees}
	resultRaw, _ := json.Marshal(result)
	consider(forgeRemoteIntent{Schema: "hn.forge-remote-intent/1", Scope: eventID([]byte("budget")), Provider: cp.Provider, Endpoint: cp.Endpoint, Kind: string(forge.ResourceIssue), Action: "update-issue", NativeID: cp.NativeID, NativeRevision: cp.Snapshot.Head, TargetID: cp.RemoteID, TargetKey: cp.RemoteKey, ExpectedRevision: eventID(nil), Payload: raw, PayloadDigest: eventID(raw), Phase: "confirmed", ResultID: cp.RemoteID, ResultKey: cp.RemoteKey, ResultPayload: resultRaw, ResultDigest: eventID(resultRaw)})
	for _, id := range cp.Snapshot.CommentIDs {
		comment, ok := selected[id]
		if !ok {
			return errors.New("selected comment missing during journal preflight")
		}
		skip, err := forgeSkipSourceComment(*cp.Snapshot, comment)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if cp.Comments[id] == "" {
			cp.Comments[id] = worst
		}
		body, err := forgeFormatExportComment(cp.Provider, cp.Endpoint, cp.RemoteID, cp.NativeID, comment)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(struct{ NativeID, Body string }{id, body})
		observed, _ := json.Marshal(forge.Comment{ID: worst, Body: body})
		consider(forgeRemoteIntent{Schema: "hn.forge-remote-intent/1", Scope: eventID([]byte("budget")), Provider: cp.Provider, Endpoint: cp.Endpoint, Kind: string(forge.ResourceIssue), Action: "create-comment", NativeID: cp.NativeID, NativeRevision: cp.Snapshot.Head, TargetID: cp.RemoteID, TargetKey: cp.RemoteKey, Payload: payload, PayloadDigest: eventID(payload), Phase: "confirmed", ResultID: worst, ResultPayload: observed, ResultDigest: eventID(observed)})
	}
	before, _ := json.Marshal(cp.RemoteBaseline)
	after, _ := json.Marshal(desired)
	if cp.RemoteBaseline == nil || len(after) > len(before) {
		cp.RemoteBaseline = &desired
	}
	cp.NativeBaseline = cp.Snapshot.NativeDigest
	cp.Pending = largest
	encoded, err := json.Marshal(cp)
	if err != nil || len(encoded) > forgeCheckpointLimit {
		return errors.New("whole future issue export checkpoint including all comment receipts and confirmed results exceeds 8MiB; no writes performed")
	}
	return nil
}
