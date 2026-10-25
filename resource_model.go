package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	ResourceProject         = "project"
	ResourceProjectItem     = "project-item"
	ResourceDiscussion      = "discussion"
	ResourceDiscussionReply = "discussion-reply"
	ResourceWorkflow        = "workflow"
	maxResourcePayload      = 256 << 10
	maxWorkflowPayload      = 2 << 20
)

type ProjectState struct {
	Schema   string            `json:"schema"`
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	Archived bool              `json:"archived"`
	Fields   []ProjectField    `json:"fields,omitempty"`
	Views    []ProjectView     `json:"views,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}
type ProjectField struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Options []ProjectOption `json:"options,omitempty"`
}
type ProjectOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type ProjectView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Layout       string `json:"layout"`
	GroupByField string `json:"group_by_field,omitempty"`
}
type ProjectItemState struct {
	Schema          string                  `json:"schema"`
	ProjectID       string                  `json:"project_id"`
	ProjectRevision string                  `json:"project_revision"`
	Target          ProjectTarget           `json:"target"`
	Values          map[string]ProjectValue `json:"values,omitempty"`
	Archived        bool                    `json:"archived"`
	Position        string                  `json:"position"`
	Metadata        map[string]string       `json:"metadata,omitempty"`
}
type ProjectTarget struct {
	Kind  string `json:"kind"`
	ID    string `json:"id,omitempty"`
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	URL   string `json:"url,omitempty"`
}
type ProjectValue struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Number   string `json:"number,omitempty"`
	Date     string `json:"date,omitempty"`
	OptionID string `json:"option_id,omitempty"`
}
type DiscussionState struct {
	Schema                string            `json:"schema"`
	Title                 string            `json:"title"`
	Body                  string            `json:"body"`
	Category              string            `json:"category"`
	Status                string            `json:"status"`
	AcceptedReplyRevision string            `json:"accepted_reply_revision,omitempty"`
	Metadata              map[string]string `json:"metadata,omitempty"`
}
type DiscussionReplyState struct {
	Schema       string            `json:"schema"`
	DiscussionID string            `json:"discussion_id"`
	ReplyTo      string            `json:"reply_to,omitempty"`
	Body         string            `json:"body"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}
type WorkflowState struct {
	Schema          string                   `json:"schema"`
	Name            string                   `json:"name"`
	Title           string                   `json:"title"`
	Description     string                   `json:"description"`
	Enabled         bool                     `json:"enabled"`
	Pipeline        *PipelineDefinition      `json:"pipeline,omitempty"`
	SourceDocuments []WorkflowSourceDocument `json:"source_documents,omitempty"`
	Metadata        map[string]string        `json:"metadata,omitempty"`
}
type WorkflowSourceDocument struct {
	Format  string `json:"format"`
	Content string `json:"content"`
	Digest  string `json:"digest"`
}

func resourceFamily(kind string) string {
	family, operation, ok := strings.Cut(kind, ".")
	if ok && (operation == "open" || operation == "revise") && validResourceFamily(family) {
		return family
	}
	return ""
}
func validResourceFamily(family string) bool {
	switch family {
	case ResourceProject, ResourceProjectItem, ResourceDiscussion, ResourceDiscussionReply, ResourceWorkflow:
		return true
	}
	return false
}
func isResourceKind(kind string) bool { return resourceFamily(kind) != "" }
func resourcePayloadLimit(family string) int {
	if family == ResourceWorkflow {
		return maxWorkflowPayload
	}
	return maxResourcePayload
}
func resourceIdentifier(value string) bool {
	if len(value) < 1 || len(value) > 64 || value == "." || value == ".." {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

var resourceDecimal = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]*[1-9])?$`)

func validResourceDecimal(value string) bool {
	return len(value) <= 64 && value != "-0" && resourceDecimal.MatchString(value)
}
func resourceMetadata(metadata map[string]string) error {
	if len(metadata) > 64 {
		return fmt.Errorf("resource metadata exceeds 64 entries")
	}
	for k, v := range metadata {
		prefix, name, ok := strings.Cut(k, "/")
		if !ok || !issueIdentifier(prefix, 128) || !issueIdentifier(name, 128) || !issueIdentifier(k, 128) || !issueText(v, 4096, false) {
			return fmt.Errorf("resource metadata requires bounded namespaced keys and UTF-8 string values")
		}
	}
	return nil
}
func strictResourceJSON(raw []byte, out any, limit int) error {
	if len(raw) > limit {
		return fmt.Errorf("resource JSON exceeds %d bytes", limit)
	}
	if err := validateIssueJSONUnicode(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := parseIssueJSONValue(decoder, 0)
	if err != nil {
		return err
	}
	if err := resourceJSONKeys(value, reflect.TypeOf(out)); err != nil {
		return err
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return err
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	return ensureJSONEnd(decoder)
}

// CanonicalResourceState strictly validates typed state and returns deterministic
// JSON. Descriptive metadata never grants identity, moderation or runner rights.
func CanonicalResourceState(family string, raw []byte) (json.RawMessage, error) {
	if !validResourceFamily(family) {
		return nil, fmt.Errorf("unsupported resource family %q", family)
	}
	var state any
	switch family {
	case ResourceProject:
		state = &ProjectState{}
	case ResourceProjectItem:
		state = &ProjectItemState{}
	case ResourceDiscussion:
		state = &DiscussionState{}
	case ResourceDiscussionReply:
		state = &DiscussionReplyState{}
	case ResourceWorkflow:
		state = &WorkflowState{}
	}
	if err := strictResourceJSON(raw, state, resourcePayloadLimit(family)); err != nil {
		return nil, err
	}
	var err error
	switch s := state.(type) {
	case *ProjectState:
		err = validateProjectState(*s)
		if s.Fields == nil {
			s.Fields = []ProjectField{}
		}
		if s.Views == nil {
			s.Views = []ProjectView{}
		}
		if s.Metadata == nil {
			s.Metadata = map[string]string{}
		}
	case *ProjectItemState:
		err = validateProjectItemState(*s)
		if s.Position == "" {
			s.Position = "0"
		}
		if s.Values == nil {
			s.Values = map[string]ProjectValue{}
		}
		if s.Metadata == nil {
			s.Metadata = map[string]string{}
		}
	case *DiscussionState:
		err = validateDiscussionState(*s)
		if s.Metadata == nil {
			s.Metadata = map[string]string{}
		}
	case *DiscussionReplyState:
		err = validateReplyState(*s)
		if s.Metadata == nil {
			s.Metadata = map[string]string{}
		}
	case *WorkflowState:
		err = validateWorkflowState(*s)
		if s.SourceDocuments == nil {
			s.SourceDocuments = []WorkflowSourceDocument{}
		}
		if s.Metadata == nil {
			s.Metadata = map[string]string{}
		}
	}
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(state)
	if err == nil && len(encoded) > resourcePayloadLimit(family) {
		err = fmt.Errorf("resource state exceeds authenticated payload limit")
	}
	return encoded, err
}
func DecodeResourceState(family string, raw []byte, out any) error {
	canonical, err := CanonicalResourceState(family, raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(canonical, out)
}
func validateProjectState(s ProjectState) error {
	if s.Schema != "hn.project/1" || !issueText(s.Title, 1024, true) || !issueText(s.Body, 65536, false) || len(s.Fields) > 64 || len(s.Views) > 16 {
		return fmt.Errorf("invalid project schema, title/body or field/view bounds")
	}
	fields := map[string]ProjectField{}
	for _, f := range s.Fields {
		if !resourceIdentifier(f.ID) || !issueText(f.Name, 256, true) || fields[f.ID].ID != "" || len(f.Options) > 64 {
			return fmt.Errorf("invalid or duplicate project field")
		}
		switch f.Type {
		case "text", "number", "date":
			if len(f.Options) != 0 {
				return fmt.Errorf("only single_select fields have options")
			}
		case "single_select":
		default:
			return fmt.Errorf("unsupported project field type")
		}
		seen := map[string]bool{}
		for _, o := range f.Options {
			if !resourceIdentifier(o.ID) || !issueText(o.Name, 256, true) || seen[o.ID] {
				return fmt.Errorf("invalid or duplicate project option")
			}
			seen[o.ID] = true
		}
		fields[f.ID] = f
	}
	views := map[string]bool{}
	for _, v := range s.Views {
		if !resourceIdentifier(v.ID) || !issueText(v.Name, 256, true) || views[v.ID] || (v.Layout != "table" && v.Layout != "board") {
			return fmt.Errorf("invalid or duplicate project view")
		}
		views[v.ID] = true
		if v.GroupByField != "" {
			f, exists := fields[v.GroupByField]
			if !exists || f.Type != "single_select" {
				return fmt.Errorf("view grouping requires a single_select field")
			}
		}
	}
	return resourceMetadata(s.Metadata)
}
func validateProjectItemState(s ProjectItemState) error {
	if s.Schema != "hn.project-item/1" || !validEventID(s.ProjectID) || !validEventID(s.ProjectRevision) || len(s.Values) > 64 || (s.Position != "" && !validResourceDecimal(s.Position)) {
		return fmt.Errorf("invalid project item schema, project reference, position or values")
	}
	t := s.Target
	switch t.Kind {
	case "issue", "proposal", "discussion":
		if !validEventID(t.ID) || t.Title != "" || t.Body != "" || t.URL != "" {
			return fmt.Errorf("native target requires only full ID")
		}
	case "draft":
		if t.ID != "" || t.URL != "" || !issueText(t.Title, 1024, true) || !issueText(t.Body, 65536, false) {
			return fmt.Errorf("draft target requires bounded title/body")
		}
	case "external":
		u, err := url.Parse(t.URL)
		if err != nil || len(t.URL) > 4096 || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || t.ID != "" || t.Title != "" || t.Body != "" || !issueIdentifier(t.URL, 4096) {
			return fmt.Errorf("external target requires explicit HTTP(S) URL only")
		}
	default:
		return fmt.Errorf("invalid project target type")
	}
	for id, v := range s.Values {
		if !resourceIdentifier(id) {
			return fmt.Errorf("invalid field ID")
		}
		if err := validateProjectValue(v); err != nil {
			return err
		}
	}
	return resourceMetadata(s.Metadata)
}
func validateProjectValue(v ProjectValue) error {
	switch v.Type {
	case "text":
		if !issueText(v.Text, 4096, false) || v.Number != "" || v.Date != "" || v.OptionID != "" {
			return fmt.Errorf("invalid text field value")
		}
	case "number":
		if !validResourceDecimal(v.Number) || v.Text != "" || v.Date != "" || v.OptionID != "" {
			return fmt.Errorf("number must be a canonical decimal string of at most 64 bytes")
		}
	case "date":
		date, err := time.Parse("2006-01-02", v.Date)
		if err != nil || date.Format("2006-01-02") != v.Date || len(v.Date) != 10 || v.Text != "" || v.Number != "" || v.OptionID != "" {
			return fmt.Errorf("date must be YYYY-MM-DD")
		}
	case "single_select":
		if !resourceIdentifier(v.OptionID) || v.Text != "" || v.Number != "" || v.Date != "" {
			return fmt.Errorf("invalid single_select value")
		}
	default:
		return fmt.Errorf("unsupported typed field value")
	}
	return nil
}
func validateDiscussionState(s DiscussionState) error {
	if s.Schema != "hn.discussion/1" || !issueText(s.Title, 1024, true) || !issueText(s.Body, 65536, false) || !issueIdentifier(s.Category, 256) || (s.Status != "open" && s.Status != "closed") || (s.AcceptedReplyRevision != "" && !validEventID(s.AcceptedReplyRevision)) {
		return fmt.Errorf("invalid discussion schema, title/body, category, status or answer")
	}
	return resourceMetadata(s.Metadata)
}
func validateReplyState(s DiscussionReplyState) error {
	if s.Schema != "hn.discussion-reply/1" || !validEventID(s.DiscussionID) || (s.ReplyTo != "" && !validEventID(s.ReplyTo)) || !issueText(s.Body, 65536, true) {
		return fmt.Errorf("invalid reply schema, thread, parent or body")
	}
	return resourceMetadata(s.Metadata)
}
func validateWorkflowState(s WorkflowState) error {
	if s.Schema != "hn.workflow/1" || !validPipelineName(s.Name) || !issueText(s.Title, 1024, true) || !issueText(s.Description, 65536, false) || len(s.SourceDocuments) > 8 {
		return fmt.Errorf("invalid workflow schema, name, title/description or source bounds")
	}
	if s.Enabled && s.Pipeline == nil {
		return fmt.Errorf("enabled runnable workflow requires a native pipeline")
	}
	if s.Pipeline != nil {
		if err := validatePipeline(*s.Pipeline); err != nil {
			return err
		}
		encoded, _ := json.Marshal(s.Pipeline)
		if len(encoded) > maxPipelineSize {
			return fmt.Errorf("workflow pipeline exceeds 1 MiB")
		}
	}
	seen := map[string]bool{}
	for _, d := range s.SourceDocuments {
		if !resourceIdentifier(d.Format) || !issueText(d.Content, 1<<20, true) || !validEventID(d.Digest) || eventID([]byte(d.Content)) != d.Digest || seen[d.Digest] {
			return fmt.Errorf("invalid, duplicate or digest-mismatched workflow source document")
		}
		seen[d.Digest] = true
	}
	return resourceMetadata(s.Metadata)
}

func ResourceRequestDigest(e Event) (string, error) {
	family := resourceFamily(e.Kind)
	state, err := CanonicalResourceState(family, []byte(e.Body))
	if err != nil {
		return "", err
	}
	parents := append([]string{}, e.Parents...)
	sort.Strings(parents)
	raw, err := json.Marshal(struct {
		Kind    string          `json:"kind"`
		Intent  string          `json:"intent"`
		Subject string          `json:"subject"`
		Parents []string        `json:"parents"`
		State   json.RawMessage `json:"state"`
	}{e.Kind, e.Intent, e.Subject, parents, state})
	if err != nil {
		return "", err
	}
	return eventID(raw), nil
}
func validateResourceEvent(e Event) error {
	family := resourceFamily(e.Kind)
	if family == "" {
		return nil
	}
	if e.Issue != nil || e.Title != "" || e.Relationship != "" || e.TargetActor != "" || e.TargetKey != "" || e.Base != "" || e.Head != "" || e.Verdict != "" || e.Pipeline != "" || e.Definition != "" || e.Commit != "" || e.Outcome != "" || e.ExitCode != 0 || e.DurationMS != 0 || e.Log != "" || e.Backend != "" || e.Platform != "" || e.Runner != "" || e.Policy != "" || len(e.Evidence) != 0 {
		return fmt.Errorf("resource event contains unrelated identity, issue, policy or CI fields")
	}
	if _, err := CanonicalResourceState(family, []byte(e.Body)); err != nil {
		return err
	}
	if strings.HasSuffix(e.Kind, ".open") {
		if e.Subject != "" || len(e.Parents) != 0 || e.Intent != "open" {
			return fmt.Errorf("resource opening requires open intent and no subject/parents")
		}
	} else {
		if !validEventID(e.Subject) || len(e.Parents) < 1 || len(e.Parents) > maxIssueParents || (e.Intent != "revise" && e.Intent != "resolve") {
			return fmt.Errorf("resource revision requires root, 1..200 parents and revise/resolve intent")
		}
		if e.Intent == "revise" && len(e.Parents) != 1 || e.Intent == "resolve" && len(e.Parents) < 2 {
			return fmt.Errorf("resource intent and parent count disagree")
		}
		for i, p := range e.Parents {
			if !validEventID(p) || i > 0 && e.Parents[i-1] >= p {
				return fmt.Errorf("resource parents require sorted unique full IDs")
			}
		}
	}
	if e.Operation != "" || e.Request != "" {
		if len(e.Operation) < 1 || len(e.Operation) > 128 || !validEventID(e.Request) {
			return fmt.Errorf("resource operation requires bounded key and digest together")
		}
		for _, c := range e.Operation {
			if c < 33 || c > 126 {
				return fmt.Errorf("resource operation key must be printable non-whitespace ASCII")
			}
		}
		digest, err := ResourceRequestDigest(e)
		if err != nil {
			return err
		}
		if digest != e.Request {
			return fmt.Errorf("resource request digest mismatch")
		}
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if len(raw) > resourcePayloadLimit(family) {
		return fmt.Errorf("resource authenticated payload exceeds %d bytes", resourcePayloadLimit(family))
	}
	return nil
}

func resourceJSONKeys(value any, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("typed state must be an object")
		}
		fields := map[string]reflect.Type{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			if tag != "" && tag != "-" {
				fields[tag] = f.Type
			}
		}
		for k, v := range object {
			ft, ok := fields[k]
			if !ok {
				return fmt.Errorf("unknown field %q", k)
			}
			if err := resourceJSONKeys(v, ft); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		array, ok := value.([]any)
		if !ok {
			return fmt.Errorf("typed collection must be an array")
		}
		for _, v := range array {
			if err := resourceJSONKeys(v, t.Elem()); err != nil {
				return err
			}
		}
	case reflect.Map:
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("typed map must be an object")
		}
		for _, v := range object {
			if err := resourceJSONKeys(v, t.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}

func resourceInputStrings(v reflect.Value, depth int) error {
	if depth > 16 {
		return fmt.Errorf("resource input nesting exceeds 16")
	}
	if !v.IsValid() {
		return nil
	}
	switch v.Kind() {
	case reflect.String:
		if !utf8.ValidString(v.String()) {
			return fmt.Errorf("resource strings must contain valid UTF-8")
		}
	case reflect.Interface, reflect.Pointer:
		if !v.IsNil() {
			return resourceInputStrings(v.Elem(), depth+1)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := resourceInputStrings(v.Field(i), depth+1); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := resourceInputStrings(v.Index(i), depth+1); err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			if err := resourceInputStrings(iter.Key(), depth+1); err != nil {
				return err
			}
			if err := resourceInputStrings(iter.Value(), depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}
