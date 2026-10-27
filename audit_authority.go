package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AuditPolicy is opt-in. These full actor fingerprints grant separate roles;
// workflow authorship, Git author metadata and imported policy are not roles.
type AuditPolicy struct {
	Scope      string   `json:"scope"`
	RuleOwners []string `json:"ruleOwners"`
	Requesters []string `json:"requesters"`
	Executors  []string `json:"executors"`
}

func validateAuditPolicy(p *AuditPolicy) error {
	if p == nil {
		return nil
	}
	if !issueIdentifier(p.Scope, 128) {
		return fmt.Errorf("audits scope must be a bounded identifier")
	}
	for _, role := range []struct {
		name   string
		actors []string
	}{{"ruleOwners", p.RuleOwners}, {"requesters", p.Requesters}, {"executors", p.Executors}} {
		if len(role.actors) == 0 || len(role.actors) > 64 {
			return fmt.Errorf("audits %s requires 1 to 64 actors", role.name)
		}
		if err := validateActorList("audits "+role.name, role.actors); err != nil {
			return err
		}
		for _, actor := range role.actors {
			if !validActorFingerprint(actor) {
				return fmt.Errorf("audits %s requires full actor fingerprints", role.name)
			}
		}
	}
	return nil
}

// AuditAuthority is a private operator selection, never a collaboration fact.
// Digest guards the selection file; PolicyDigest tracks the currently resolved
// policy bytes, so a code-only branch advance does not invalidate requests.
type AuditAuthority struct {
	Ref          string         `json:"ref"`
	Digest       string         `json:"-"`
	Commit       string         `json:"-"`
	PolicyDigest string         `json:"-"`
	Policy       PolicyDocument `json:"-"`
}

func auditAuthorityPath(create bool) (string, error) {
	gitdir, err := requireGitRepository()
	if err != nil {
		return "", err
	}
	directory := filepath.Join(gitdir, "hn")
	for _, part := range []string{"", "audit"} {
		if part != "" {
			directory = filepath.Join(directory, part)
		}
		info, err := os.Lstat(directory)
		if errors.Is(err, os.ErrNotExist) && create {
			if err = os.Mkdir(directory, 0700); err != nil {
				return "", err
			}
			info, err = os.Lstat(directory)
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
			return "", fmt.Errorf("unsafe private audit directory %s", directory)
		}
	}
	return filepath.Join(directory, "authority.json"), nil
}

func resolveAuditBranch(ref string) (string, PolicyDocument, string, error) {
	if !strings.HasPrefix(ref, "refs/heads/") || strings.TrimPrefix(ref, "refs/heads/") == "" {
		return "", PolicyDocument{}, "", fmt.Errorf("audit authority requires a named local branch refs/heads/NAME")
	}
	if _, err := gitOutput("check-ref-format", ref); err != nil {
		return "", PolicyDocument{}, "", err
	}
	// Symbolic aliases are not stable branch selections.
	if _, err := gitOutput("symbolic-ref", "-q", ref); err == nil {
		return "", PolicyDocument{}, "", fmt.Errorf("audit authority must be a direct local branch")
	}
	commit, err := gitText("rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", PolicyDocument{}, "", err
	}
	policy, _, digest, err := loadPolicy(commit)
	if err != nil {
		return "", PolicyDocument{}, "", err
	}
	return commit, policy, digest, nil
}

// AuditAuthoritySet atomically adopts a named branch using absent or the exact
// previous selection digest. It does not change branch, policy or signer.
func AuditAuthoritySet(ref, expect string) (*AuditAuthority, error) {
	if expect != "absent" && !validEventID(expect) {
		return nil, fmt.Errorf("authority update requires --expect absent or exact anchor digest")
	}
	if _, _, _, err := resolveAuditBranch(ref); err != nil {
		return nil, err
	}
	path, err := auditAuthorityPath(true)
	if err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("lock audit authority: %w", err)
	}
	defer func() { _ = lock.Close(); _ = os.Remove(path + ".lock") }()
	current, err := readPrivateFileBounded(path, 4096)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	actual := "absent"
	if err == nil {
		actual = eventID(current)
	}
	if actual != expect {
		return nil, fmt.Errorf("audit authority changed: expected %s, found %s", expect, actual)
	}
	raw, err := json.Marshal(AuditAuthority{Ref: ref})
	if err != nil {
		return nil, err
	}
	if err := writePrivateFileAtomic(path, raw); err != nil {
		return nil, err
	}
	return AuditAuthorityShow()
}

func AuditAuthorityShow() (*AuditAuthority, error) {
	path, err := auditAuthorityPath(false)
	if err != nil {
		return nil, fmt.Errorf("no audit authority anchor; explicitly adopt a named branch with hn audit authority set: %w", err)
	}
	raw, err := readPrivateFileBounded(path, 4096)
	if err != nil {
		return nil, fmt.Errorf("read audit authority; explicitly adopt with hn audit authority set: %w", err)
	}
	var a AuditAuthority
	if err := strictResourceJSON(raw, &a, 4096); err != nil {
		return nil, err
	}
	a.Digest = eventID(raw)
	a.Commit, a.Policy, a.PolicyDigest, err = resolveAuditBranch(a.Ref)
	return &a, err
}

func currentAuditAuthority(assertion, actor, role string) (*AuditAuthority, error) {
	a, err := AuditAuthorityShow()
	if err != nil {
		return nil, err
	}
	if assertion != "" {
		asserted, err := gitText("rev-parse", "--verify", assertion+"^{commit}")
		if err != nil || asserted != a.Commit {
			return nil, fmt.Errorf("policy-ref must match current anchored authority %s", a.Ref)
		}
	}
	if a.Policy.Audits == nil {
		return nil, fmt.Errorf("audits disabled by current authority policy")
	}
	var actors []string
	switch role {
	case "owner":
		actors = a.Policy.Audits.RuleOwners
	case "requester":
		actors = a.Policy.Audits.Requesters
	case "executor":
		actors = a.Policy.Audits.Executors
	default:
		return nil, fmt.Errorf("unknown audit role")
	}
	if !actorListed(actor, actors) {
		return nil, fmt.Errorf("actor is not a current audit %s", role)
	}
	return a, nil
}

// currentAuditRuleHeads restricts rule graphs to accepted authority ancestry and
// the current digest/owners. A fork cannot poison a same-scope active rule.
func currentAuditRuleHeads(a *AuditAuthority, rule string, events []StoredEvent) []StoredEvent {
	candidates := map[string]StoredEvent{}
	for _, s := range events {
		f := s.Event.Audit
		if (s.Event.Kind != "audit.rule-authorize" && s.Event.Kind != "audit.rule-revoke") || f == nil || f.Scope != a.Policy.Audits.Scope || f.Rule != rule || f.PolicyDigest != a.PolicyDigest || !actorListed(s.Event.Actor, a.Policy.Audits.RuleOwners) {
			continue
		}
		if _, err := gitOutput("merge-base", "--is-ancestor", f.PolicyCommit, a.Commit); err != nil {
			continue
		}
		candidates[s.ID] = s
	}
	heads := map[string]StoredEvent{}
	for id, s := range candidates {
		heads[id] = s
	}
	for _, s := range candidates {
		for _, parent := range s.Event.Audit.Parents {
			delete(heads, parent)
		}
	}
	result := []StoredEvent{}
	for _, s := range heads {
		result = append(result, s)
	}
	sortStoredEvents(result)
	return result
}

func usableAuditRule(a *AuditAuthority, rule string, events []StoredEvent) (*StoredEvent, error) {
	heads := currentAuditRuleHeads(a, rule, events)
	if len(heads) != 1 || heads[0].Event.Kind != "audit.rule-authorize" {
		return nil, fmt.Errorf("audit rule %q is missing, revoked or conflicted", rule)
	}
	return &heads[0], nil
}
