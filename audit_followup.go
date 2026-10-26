package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type auditAssociation struct {
	Finding  string `json:"finding"`
	Identity string `json:"identity"`
	IssueID  string `json:"issue_id"`
	Result   string `json:"result"`
	Existing bool   `json:"existing"`
}
type auditFollowUpResult struct {
	Associations []auditAssociation `json:"associations"`
	Concurrent   bool               `json:"multiple_associations"`
}

func auditDigest(parts ...string) string { raw, _ := json.Marshal(parts); return eventID(raw) }
func auditFindingIdentity(f *AuditFact, key string) string {
	return auditDigest(f.Scope, f.Rule, normalizeAuditFindingKey(key))
}

// auditLinkedFinding authenticates opening metadata against a verified signed
// result and its exact request. Later issue edits cannot erase the association.
func auditLinkedFinding(s StoredEvent, byID map[string]StoredEvent) (string, string, bool) {
	if s.Event.Kind != "issue.open" || s.Event.Issue == nil {
		return "", "", false
	}
	m := s.Event.Issue.Metadata
	result, ok := byID[m["audit/result"]]
	if !ok || result.Event.Kind != "audit.result" || result.Event.Audit.Request != m["audit/request"] {
		return "", "", false
	}
	for _, f := range result.Event.Audit.Findings {
		identity := auditFindingIdentity(result.Event.Audit, f.Key)
		if identity == m["audit/finding"] {
			return identity, result.ID, true
		}
	}
	return "", "", false
}

func followUpAudit(o auditOptions, id *Identity) (*auditFollowUpResult, error) {
	if !validEventID(o.id) {
		return nil, fmt.Errorf("follow-up requires a full result ID")
	}
	if o.operation != "" && (len(o.operation) > 128 || strings.ContainsFunc(o.operation, func(r rune) bool { return r < 33 || r > 126 })) {
		return nil, fmt.Errorf("operation requires 1..128 printable non-whitespace ASCII bytes")
	}
	events, err := collectEvents()
	if err != nil {
		return nil, err
	}
	byID := map[string]StoredEvent{}
	for _, s := range events {
		byID[s.ID] = s
	}
	result, ok := byID[o.id]
	if !ok || result.Event.Kind != "audit.result" {
		return nil, fmt.Errorf("not a verified audit result")
	}
	fact := result.Event.Audit
	selected := []AuditFinding{}
	for _, finding := range fact.Findings {
		if o.finding == "" || normalizeAuditFindingKey(finding.Key) == normalizeAuditFindingKey(o.finding) {
			selected = append(selected, finding)
		}
	}
	if o.finding != "" && len(selected) != 1 {
		return nil, fmt.Errorf("finding was not found in the original result snapshot")
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Key < selected[j].Key })
	keys := []string{}
	for _, f := range selected {
		keys = append(keys, auditFindingIdentity(fact, f.Key))
	}
	selection := auditDigest(append([]string{o.id}, keys...)...)
	operation := o.operation
	if operation == "" {
		operation = selection
	}
	operationID := auditDigest(operation)
	// All intent checks precede any mutation. Native issue operations carry this
	// signed binding through a crash after append but before the caller sees it.
	for _, s := range events {
		if s.Event.Actor != id.Actor || s.Event.Kind != "issue.open" || s.Event.Issue == nil {
			continue
		}
		m := s.Event.Issue.Metadata
		if m["audit/operation"] == operationID && m["audit/selection"] != selection {
			return nil, fmt.Errorf("operation is bound to different intent; retry its original snapshot")
		}
	}
	if err := bindAuditFollowUp(id.Actor, operationID, selection, o.id); err != nil {
		return nil, err
	}
	reply := &auditFollowUpResult{Associations: []auditAssociation{}}
	for _, finding := range selected {
		identity := auditFindingIdentity(fact, finding.Key)
		matches := []auditAssociation{}
		for _, s := range events {
			linked, original, ok := auditLinkedFinding(s, byID)
			if ok && linked == identity {
				matches = append(matches, auditAssociation{finding.Key, identity, s.ID, original, true})
			}
		}
		if len(matches) > 0 {
			sort.Slice(matches, func(i, j int) bool { return matches[i].IssueID < matches[j].IssueID })
			reply.Associations = append(reply.Associations, matches...)
			reply.Concurrent = reply.Concurrent || len(matches) > 1
			continue
		}
		state := IssueState{Title: finding.Title, Body: finding.Body, Status: "open", Metadata: map[string]string{"audit/finding": identity, "audit/request": fact.Request, "audit/result": result.ID, "audit/operation": operationID, "audit/selection": selection}}
		// Location is inert prose; native issue metadata retains authenticated links.
		if finding.Path != "" {
			state.Body += fmt.Sprintf("\n\nAudit location: %s", finding.Path)
			if finding.Line > 0 {
				state.Body += fmt.Sprintf(":%d", finding.Line)
			}
		}
		issueOperation := "audit-followup/" + strings.TrimPrefix(auditDigest(operationID, selection, identity), "sha256:")
		value, err := executeIssueMutation(issueOptions{command: "open", state: &state, operation: issueOperation, actor: id.Actor})
		if err != nil {
			return nil, fmt.Errorf("follow-up interrupted; retry the same result, finding selection and operation: %w", err)
		}
		opened := value.(*issueMutationResult)
		reply.Associations = append(reply.Associations, auditAssociation{finding.Key, identity, opened.IssueID, result.ID, opened.Replayed})
	}
	return reply, nil
}

// This private intent is durable recovery state, not an issue index or authority.
// An atomic hard-link installs it without replacing a concurrent intent. A crash
// before the link leaves no binding; after it, retries retain the exact snapshot.
func bindAuditFollowUp(actor, operation, selection, result string) error {
	raw, _ := json.Marshal(struct {
		Selection string `json:"selection"`
		Result    string `json:"result"`
	}{selection, result})
	return bindAuditIntent(actor, "followups", operation, raw)
}
func bindAuditIntent(actor, family, operation string, raw []byte) error {
	if len(raw) > 32<<10 {
		return fmt.Errorf("audit intent exceeds 32 KiB")
	}
	authority, err := auditAuthorityPath(true)
	if err != nil {
		return err
	}
	directory := filepath.Dir(authority)
	for _, part := range []string{family, actor} {
		directory = filepath.Join(directory, part)
		if err := os.Mkdir(directory, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := os.Lstat(directory)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("unsafe private audit intent directory")
		}
	}
	path := filepath.Join(directory, strings.TrimPrefix(operation, "sha256:")+".json")
	current, err := readPrivateFileBounded(path, 32<<10)
	if err == nil {
		if !bytes.Equal(current, raw) {
			return fmt.Errorf("operation is bound to different intent; retry its original snapshot")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.CreateTemp(directory, ".intent-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Link(file.Name(), path); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		current, err = readPrivateFileBounded(path, 32<<10)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, raw) {
			return fmt.Errorf("concurrent audit operation has different intent")
		}
	}
	return syncPrivateDirectory(directory)
}
