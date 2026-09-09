package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxIssuePayload = 256 * 1024
const maxIssueParents = 200

// IssueState is descriptive work information, never execution or review authority.
type IssueState struct {
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Status    string            `json:"status"`
	Criteria  []IssueCriterion  `json:"criteria"`
	Labels    []string          `json:"labels"`
	Assignees []string          `json:"assignees"`
	Relations []IssueRelation   `json:"relations"`
	Metadata  map[string]string `json:"metadata"`
}
type IssueCriterion struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}
type IssueRelation struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
}

func issueText(value string, limit int, required bool) bool {
	return len(value) <= limit && utf8.ValidString(value) && (!required || strings.TrimSpace(value) != "")
}
func issueIdentifier(value string, limit int) bool {
	return issueText(value, limit, true) && !strings.ContainsFunc(value, unicode.IsControl)
}

// CanonicalIssueState validates and copies input, sorting sets without changing
// criterion order. It never normalizes away user content or duplicate entries.
func CanonicalIssueState(input IssueState) (IssueState, error) {
	if !issueText(input.Title, 1024, true) || !issueText(input.Body, 65536, false) || (input.Status != "open" && input.Status != "closed") {
		return IssueState{}, fmt.Errorf("issue requires title (1..1024 bytes), body <=65536 bytes and open/closed status")
	}
	if len(input.Criteria) > 64 || len(input.Labels) > 64 || len(input.Assignees) > 64 || len(input.Relations) > 128 || len(input.Metadata) > 64 {
		return IssueState{}, fmt.Errorf("issue collection limit exceeded")
	}
	result := input
	result.Criteria = append([]IssueCriterion{}, input.Criteria...)
	result.Labels = append([]string{}, input.Labels...)
	result.Assignees = append([]string{}, input.Assignees...)
	result.Relations = append([]IssueRelation{}, input.Relations...)
	result.Metadata = make(map[string]string, len(input.Metadata))
	seen := map[string]bool{}
	for _, c := range result.Criteria {
		if !issueIdentifier(c.ID, 64) || !issueText(c.Text, 4096, true) || seen[c.ID] {
			return IssueState{}, fmt.Errorf("invalid or duplicate issue criterion %q", c.ID)
		}
		seen[c.ID] = true
	}
	for _, values := range [][]string{result.Labels, result.Assignees} {
		sort.Strings(values)
		for i, v := range values {
			if !issueIdentifier(v, 256) || (i > 0 && v == values[i-1]) {
				return IssueState{}, fmt.Errorf("invalid or duplicate issue label/assignee")
			}
		}
	}
	sort.Slice(result.Relations, func(i, j int) bool {
		a, b := result.Relations[i], result.Relations[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Target < b.Target
	})
	for i, r := range result.Relations {
		if (r.Kind != "blocks" && r.Kind != "related" && r.Kind != "parent") || !validEventID(r.Target) || (i > 0 && r == result.Relations[i-1]) {
			return IssueState{}, fmt.Errorf("invalid or duplicate issue relation")
		}
	}
	for k, v := range input.Metadata {
		prefix, name, found := strings.Cut(k, "/")
		if !found || !issueIdentifier(prefix, 128) || !issueIdentifier(name, 128) || !issueIdentifier(k, 128) || !issueText(v, 4096, false) {
			return IssueState{}, fmt.Errorf("issue metadata requires bounded namespaced keys and string values")
		}
		result.Metadata[k] = v
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return IssueState{}, err
	}
	if len(encoded) > maxIssuePayload {
		return IssueState{}, fmt.Errorf("issue state exceeds %d bytes", maxIssuePayload)
	}
	return result, nil
}

func isIssueKind(kind string) bool {
	return kind == "issue.open" || kind == "issue.revise" || kind == "issue.comment"
}
func hasIssueFields(e Event) bool {
	return e.Issue != nil || len(e.Parents) > 0 || e.Operation != "" || e.Request != "" || e.Intent != ""
}

// IssueRequestDigest binds semantic request content; actor scope and operation
// key are separately signed fields. Transport time/sequence do not affect replay.
func IssueRequestDigest(e Event) (string, error) {
	var state *IssueState
	if e.Issue != nil {
		canonical, err := CanonicalIssueState(*e.Issue)
		if err != nil {
			return "", err
		}
		state = &canonical
	}
	parents := append([]string{}, e.Parents...)
	sort.Strings(parents)
	input := struct {
		Intent  string      `json:"intent"`
		Kind    string      `json:"kind"`
		Subject string      `json:"subject"`
		Parents []string    `json:"parents"`
		Issue   *IssueState `json:"issue"`
		Title   string      `json:"title"`
		Body    string      `json:"body"`
	}{e.Intent, e.Kind, e.Subject, parents, state, e.Title, e.Body}
	encoded, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	if len(encoded) > maxIssuePayload {
		return "", fmt.Errorf("issue request exceeds %d bytes", maxIssuePayload)
	}
	return eventID(encoded), nil
}

func validateIssueEvent(e Event) error {
	if !isIssueKind(e.Kind) {
		if hasIssueFields(e) {
			return fmt.Errorf("issue fields are not allowed on %s", e.Kind)
		}
		return nil
	}
	rich := hasIssueFields(e) || e.Kind == "issue.revise"
	if !rich {
		return nil
	} // Existing signed issue bytes retain their original bounds.
	if e.Kind == "issue.open" {
		if len(e.Parents) > 0 || e.Subject != "" {
			return fmt.Errorf("issue opening cannot have parents or subject")
		}
		if e.Issue != nil && (e.Title != e.Issue.Title || e.Body != e.Issue.Body) {
			return fmt.Errorf("issue opening title/body differ from state")
		}
	}
	if e.Kind == "issue.comment" && (e.Issue != nil || len(e.Parents) > 0) {
		return fmt.Errorf("issue comment cannot contain state or parents")
	}
	if e.Kind == "issue.revise" {
		if !validEventID(e.Subject) || e.Issue == nil || e.Title != "" || e.Body != "" || len(e.Parents) == 0 || len(e.Parents) > maxIssueParents {
			return fmt.Errorf("issue revision requires root, full state and 1..200 parents only")
		}
		for i, p := range e.Parents {
			if !validEventID(p) || (i > 0 && e.Parents[i-1] >= p) {
				return fmt.Errorf("issue revision parents must be sorted unique full IDs")
			}
		}
	}
	if e.Issue != nil {
		if _, err := CanonicalIssueState(*e.Issue); err != nil {
			return err
		}
	}
	if !issueText(e.Title, 1024, false) || !issueText(e.Body, 65536, false) {
		return fmt.Errorf("issue title/body limit exceeded")
	}
	if e.Operation != "" || e.Request != "" {
		if len(e.Operation) == 0 || len(e.Operation) > 128 || !validEventID(e.Request) {
			return fmt.Errorf("issue operation requires key and request digest together")
		}
		for _, c := range e.Operation {
			if c < 33 || c > 126 {
				return fmt.Errorf("issue operation key must be printable non-whitespace ASCII")
			}
		}
	}
	switch e.Intent {
	case "open":
		if e.Kind != "issue.open" {
			return fmt.Errorf("open intent requires issue.open")
		}
	case "comment":
		if e.Kind != "issue.comment" {
			return fmt.Errorf("comment intent requires issue.comment")
		}
	case "revise", "resolve", "close", "reopen":
		if e.Kind != "issue.revise" {
			return fmt.Errorf("revision intent requires issue.revise")
		}
		if e.Intent == "close" && e.Issue.Status != "closed" || e.Intent == "reopen" && e.Issue.Status != "open" {
			return fmt.Errorf("lifecycle intent does not match issue status")
		}
		if e.Intent != "resolve" && len(e.Parents) != 1 {
			return fmt.Errorf("ordinary revision requires exactly one parent")
		}
		if e.Intent == "resolve" && len(e.Parents) < 2 {
			return fmt.Errorf("resolution requires at least two parents")
		}
	default:
		return fmt.Errorf("rich issue requires a valid signed intent")
	}
	if e.Operation != "" {
		digest, err := IssueRequestDigest(e)
		if err != nil {
			return err
		}
		if digest != e.Request {
			return fmt.Errorf("issue operation request digest does not match signed content")
		}
	}
	encoded, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if len(encoded) > maxIssuePayload {
		return fmt.Errorf("issue payload exceeds %d bytes", maxIssuePayload)
	}
	return nil
}
