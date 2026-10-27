package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"unicode/utf8"
)

func init() { localCommands["audit"] = cmdAudit }

const auditHelp = `Explicit observational audits (disabled unless current policy opts in).
  hn audit authority set --ref refs/heads/NAME --expect absent|DIGEST [--json]
  hn audit authority show [--json]
  hn audit rule authorize RULE --source pipeline|git --at REV --path PATH [--expect AUTH]... [--policy-ref REV] [--actor ACTOR] [--json]
  hn audit rule authorize RULE --source workflow --workflow FULL_REV [--expect AUTH]... [--policy-ref REV] [--actor ACTOR] [--json]
  hn audit rule revoke RULE --expect AUTH... --reason TEXT [--policy-ref REV] [--actor ACTOR] [--json]
  hn audit rule list|show [RULE] [--limit 1..200] [--offset N] [--json]
  hn audit request --at REV --rule RULE [--intent TEXT] [--policy-ref REV] [--operation KEY] [--actor ACTOR] [--json]
  hn audit execute FULL_REQUEST [--policy-ref REV] [--backend sandbox|host] [--allow-unsafe-host-execution] [--actor ACTOR] [--json]
  hn audit list [--limit 1..200] [--offset N] [--json]
  hn audit show|logs FULL_ID [--json]
  hn audit follow-up FULL_RESULT [--finding KEY] [--operation KEY] [--actor ACTOR] [--json]
No command fetches content, schedules work or supplies merge evidence. Request
never executes. Execute never opens issues. Reads need no private signing key.
`

type auditOptions struct {
	command, id, ref, source, at, path, workflow, policy, rule, reason, intent, operation, actor, backend, finding string
	expected                                                                                                       issueExpected
	machine, allowHost                                                                                             bool
	limit, offset                                                                                                  int
}

func parseAuditOptions(args []string) (auditOptions, error) {
	o := auditOptions{command: args[0], limit: 50}
	for _, argument := range args {
		if !utf8.ValidString(argument) {
			return o, fmt.Errorf("audit arguments must be valid UTF-8")
		}
	}
	rest := args[1:]
	if o.command == "authority" || o.command == "rule" {
		if len(rest) == 0 {
			return o, usageError(auditHelp)
		}
		o.command += " " + rest[0]
		rest = rest[1:]
	}
	f := flag.NewFlagSet("audit "+o.command, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.BoolVar(&o.machine, "json", false, "")
	switch o.command {
	case "authority set":
		f.StringVar(&o.ref, "ref", "", "")
		f.Var(&o.expected, "expect", "")
	case "authority show":
	case "rule authorize":
		f.StringVar(&o.source, "source", "", "")
		f.StringVar(&o.at, "at", "", "")
		f.StringVar(&o.path, "path", "", "")
		f.StringVar(&o.workflow, "workflow", "", "")
		f.Var(&o.expected, "expect", "")
	case "rule revoke":
		f.Var(&o.expected, "expect", "")
		f.StringVar(&o.reason, "reason", "", "")
	case "request":
		f.StringVar(&o.at, "at", "", "")
		f.StringVar(&o.rule, "rule", "", "")
		f.StringVar(&o.intent, "intent", "explicit audit", "")
		f.StringVar(&o.operation, "operation", "", "")
	case "execute":
		f.StringVar(&o.backend, "backend", "sandbox", "")
		f.BoolVar(&o.allowHost, "allow-unsafe-host-execution", false, "")
	case "follow-up":
		f.StringVar(&o.finding, "finding", "", "")
		f.StringVar(&o.operation, "operation", "", "")
	case "list", "rule list", "rule show":
		f.IntVar(&o.limit, "limit", 50, "")
		f.IntVar(&o.offset, "offset", 0, "")
	case "show", "logs":
	default:
		return o, usageError(auditHelp)
	}
	switch o.command {
	case "rule authorize", "rule revoke", "request", "execute":
		f.StringVar(&o.policy, "policy-ref", "", "")
	}
	switch o.command {
	case "rule authorize", "rule revoke", "request", "execute", "follow-up":
		f.StringVar(&o.actor, "actor", "", "")
	}
	needsID := o.command == "rule authorize" || o.command == "rule revoke" || o.command == "rule show" || o.command == "execute" || o.command == "follow-up" || o.command == "show" || o.command == "logs"
	if needsID && len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		o.id = rest[0]
		rest = rest[1:]
	}
	if err := f.Parse(rest); err != nil {
		return o, err
	}
	if f.NArg() != 0 || (needsID && o.id == "") || o.limit < 1 || o.limit > 200 || o.offset < 0 {
		return o, usageError(auditHelp)
	}
	if o.actor != "" && !validActorFingerprint(o.actor) {
		return o, fmt.Errorf("actor requires full fingerprint")
	}
	return o, nil
}
func cmdAudit(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(auditHelp)
		return nil
	}
	o, err := parseAuditOptions(args)
	machine := workflowMachineRequested(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Print(auditHelp)
		return nil
	}
	var data any
	if err == nil {
		machine = o.machine
		data, err = executeAuditCommand(o)
	}
	if machine {
		envelope := issueEnvelope{Schema: "hn.audit/1", OK: err == nil, Data: data}
		if err != nil {
			envelope.Error = &issueCommandError{Code: "audit_refused", Message: oneLine(err.Error())}
		}
		if encodeErr := json.NewEncoder(os.Stdout).Encode(envelope); encodeErr != nil {
			return encodeErr
		}
		if err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}
	if o.command == "logs" {
		fmt.Print(safeText(data.(string)))
		return nil
	}
	raw, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(safeText(string(raw)))
	return nil
}

type auditRecord struct {
	ID    string `json:"id"`
	Event Event  `json:"event"`
}

func auditRecorded(s *StoredEvent) auditRecord { return auditRecord{ID: s.ID, Event: s.Event} }
func auditResolveCommit(rev string) (string, error) {
	if rev == "" || strings.HasPrefix(rev, "-") {
		return "", fmt.Errorf("an explicit already-present Git revision is required")
	}
	return gitText("rev-parse", "--verify", "--end-of-options", rev+"^{commit}")
}
func executeAuditCommand(o auditOptions) (any, error) {
	switch o.command {
	case "authority set", "authority show":
		var a *AuditAuthority
		var err error
		if o.command == "authority set" {
			if len(o.expected) != 1 {
				return nil, fmt.Errorf("authority set requires one --expect")
			}
			a, err = AuditAuthoritySet(o.ref, o.expected[0])
		} else {
			a, err = AuditAuthorityShow()
		}
		if err != nil {
			return nil, err
		}
		return struct {
			Ref          string       `json:"ref"`
			Digest       string       `json:"digest"`
			Commit       string       `json:"commit"`
			PolicyDigest string       `json:"policy_digest"`
			Audits       *AuditPolicy `json:"audits"`
		}{a.Ref, a.Digest, a.Commit, a.PolicyDigest, a.Policy.Audits}, nil
	case "list", "show", "logs", "rule list", "rule show":
		return inspectAudits(o)
	}
	id, err := loadIdentity()
	if err != nil {
		return nil, err
	}
	if o.actor != "" && o.actor != id.Actor {
		return nil, fmt.Errorf("active signing actor differs from --actor")
	}
	var stored *StoredEvent
	switch o.command {
	case "rule authorize":
		source := AuditSource{Kind: o.source}
		switch o.source {
		case "pipeline", "git":
			if o.workflow != "" {
				return nil, fmt.Errorf("Git sources do not accept --workflow")
			}
			source.Commit, err = auditResolveCommit(o.at)
			if err != nil {
				return nil, err
			}
			source.Path = o.path
			if source.Path == "" && o.source == "pipeline" {
				source.Path = ".hn/pipelines/" + o.id + ".json"
			}
		case "workflow":
			if o.at != "" || o.path != "" || !validEventID(o.workflow) {
				return nil, fmt.Errorf("workflow source requires only an exact --workflow revision")
			}
			source.Revision = o.workflow
		default:
			return nil, fmt.Errorf("source must be pipeline, git or workflow")
		}
		stored, err = AuditAuthorize(AuditAuthorizeOptions{Rule: o.id, Source: source, Expected: o.expected, PolicyRef: o.policy}, id)
	case "rule revoke":
		stored, err = AuditRevoke(o.id, o.expected, o.reason, o.policy, id)
	case "request":
		stored, err = requestAudit(o, id)
	case "execute":
		backend, selectErr := selectBackend(o.backend, o.allowHost)
		if selectErr != nil {
			return nil, selectErr
		}
		stored, err = executeAudit(context.Background(), o.id, o.policy, id, backend)
	case "follow-up":
		return followUpAudit(o, id)
	}
	if err != nil {
		return nil, err
	}
	return auditRecorded(stored), nil
}
func requestAudit(o auditOptions, id *Identity) (*StoredEvent, error) {
	at, err := auditResolveCommit(o.at)
	if err != nil {
		return nil, err
	}
	if !validPipelineName(o.rule) || !issueText(o.intent, 4096, true) {
		return nil, fmt.Errorf("request requires a valid rule and intent of 1..4096 bytes")
	}
	if o.operation != "" {
		if len(o.operation) > 256 {
			return nil, fmt.Errorf("audit operation exceeds 256 bytes")
		}
		release, err := lockAuditRequestOperation(id.Actor, o.operation)
		if err != nil {
			return nil, err
		}
		defer release()
		events, err := collectEvents()
		if err != nil {
			return nil, err
		}
		var prior *StoredEvent
		for i := range events {
			s := &events[i]
			if s.Event.Actor == id.Actor && s.Event.Operation == o.operation {
				if prior != nil {
					return nil, fmt.Errorf("operation has multiple signed records")
				}
				prior = s
			}
		}
		if prior != nil {
			if prior.Event.Kind != "audit.request" || prior.Event.Audit.CodeCommit != at || prior.Event.Audit.Rule != o.rule || prior.Event.Audit.Intent != o.intent {
				return nil, fmt.Errorf("operation is bound to different audit intent")
			}
			if o.policy != "" {
				if _, err := currentAuditAuthority(o.policy, id.Actor, "requester"); err != nil {
					return nil, err
				}
			}
			return prior, nil
		}
	}
	if o.operation != "" {
		raw, _ := json.Marshal(struct{ At, Rule, Intent string }{at, o.rule, o.intent})
		if err := bindAuditIntent(id.Actor, "requests", auditDigest(o.operation), raw); err != nil {
			return nil, err
		}
	}
	return AuditRequestWithIntent(at, o.rule, o.policy, o.intent, o.operation, id)
}
func inspectAudits(o auditOptions) (any, error) {
	events, err := collectEvents()
	if err != nil {
		return nil, err
	}
	if o.command == "show" || o.command == "logs" {
		if !validEventID(o.id) {
			return nil, fmt.Errorf("audit inspection requires a full ID")
		}
		s, err := resolveEvent(events, o.id)
		if err != nil {
			return nil, err
		}
		if !isAuditKind(s.Event.Kind) {
			return nil, fmt.Errorf("not an audit fact")
		}
		if o.command == "logs" {
			if s.Event.Kind != "audit.result" {
				return nil, fmt.Errorf("logs requires an audit result")
			}
			return string(s.Attachments["log.txt"]), nil
		}
		return auditRecorded(s), nil
	}
	records := []auditRecord{}
	for i := range events {
		s := &events[i]
		if !isAuditKind(s.Event.Kind) {
			continue
		}
		if strings.HasPrefix(o.command, "rule ") {
			if s.Event.Kind != "audit.rule-authorize" && s.Event.Kind != "audit.rule-revoke" {
				continue
			}
			if o.command == "rule show" && s.Event.Audit.Rule != o.id {
				continue
			}
		}
		records = append(records, auditRecorded(s))
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	total := len(records)
	start := min(o.offset, total)
	end := min(start+o.limit, total)
	next := end
	if end == total {
		next = 0
	}
	// Historical reads work without any local anchor. Active heads, when available,
	// are an additional projection and never an authority inferred from history.
	heads := []string{}
	if o.command == "rule show" {
		if a, e := AuditAuthorityShow(); e == nil && a.Policy.Audits != nil {
			for _, s := range currentAuditRuleHeads(a, o.id, events) {
				heads = append(heads, s.ID)
			}
		}
	}
	return struct {
		Items []auditRecord `json:"items"`
		Total int           `json:"total"`
		Next  int           `json:"next_offset"`
		Heads []string      `json:"current_heads,omitempty"`
	}{records[start:end], total, next, heads}, nil
}

// Hold only a prepared verify transaction: never commit a local lock ref.
// Git releases the ref lock when stdin closes, including client death. Disable
// repository hooks explicitly: taking this local guard must not execute code.
func lockAuditRequestOperation(actor, operation string) (func(), error) {
	directory, err := os.MkdirTemp("", "hn-audit-lock-")
	if err != nil {
		return nil, err
	}
	command := exec.Command("git", "-c", "core.hooksPath="+directory, "-c", "core.filesRefLockTimeout=0", "update-ref", "--no-deref", "--stdin")
	input, err := command.StdinPipe()
	if err != nil {
		_ = os.RemoveAll(directory)
		return nil, err
	}
	output, err := command.StdoutPipe()
	if err != nil {
		_ = input.Close()
		_ = os.RemoveAll(directory)
		return nil, err
	}
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	if err = command.Start(); err != nil {
		_ = input.Close()
		_ = os.RemoveAll(directory)
		return nil, err
	}
	release := func() {
		_, _ = io.WriteString(input, "abort\n")
		_ = input.Close()
		_ = command.Wait()
		_ = os.RemoveAll(directory)
	}
	ref := "refs/hn/local/audit-request/" + actor + "/" + strings.TrimPrefix(auditDigest(operation), "sha256:")
	// Empty old OID asks Git to verify absence, independent of SHA-1/SHA-256.
	_, err = io.WriteString(input, "start\nverify "+ref+" \"\"\nprepare\n")
	reader := bufio.NewReader(output)
	if err == nil {
		var line string
		line, err = reader.ReadString('\n')
		if err == nil && line != "start: ok\n" {
			err = fmt.Errorf("unexpected Git transaction response")
		}
	}
	if err == nil {
		var line string
		line, err = reader.ReadString('\n')
		if err == nil && line != "prepare: ok\n" {
			err = fmt.Errorf("unexpected Git preparation response")
		}
	}
	if err != nil {
		release()
		return nil, fmt.Errorf("audit operation is busy or unavailable; retry the same intent after the other invocation finishes: %s", oneLine(diagnostics.String()))
	}
	return release, nil
}
