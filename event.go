package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const protocolVersion = "hn/0"

type Event struct {
	Protocol     string   `json:"protocol"`
	Kind         string   `json:"kind"`
	Actor        string   `json:"actor"`
	ActorName    string   `json:"actorName"`
	PublicKey    string   `json:"publicKey"`
	Sequence     uint64   `json:"sequence"`
	Timestamp    string   `json:"timestamp"`
	Previous     string   `json:"previous,omitempty"`
	Subject      string   `json:"subject,omitempty"`
	Relationship string   `json:"relationship,omitempty"`
	TargetActor  string   `json:"targetActor,omitempty"`
	TargetKey    string   `json:"targetKey,omitempty"`
	Title        string   `json:"title,omitempty"`
	Body         string   `json:"body,omitempty"`
	Base         string   `json:"base,omitempty"`
	Head         string   `json:"head,omitempty"`
	Verdict      string   `json:"verdict,omitempty"`
	Pipeline     string   `json:"pipeline,omitempty"`
	Definition   string   `json:"definition,omitempty"`
	Commit       string   `json:"commit,omitempty"`
	Outcome      string   `json:"outcome,omitempty"`
	ExitCode     int      `json:"exitCode,omitempty"`
	DurationMS   int64    `json:"durationMs,omitempty"`
	Log          string   `json:"log,omitempty"`
	Backend      string   `json:"backend,omitempty"`
	Platform     string   `json:"platform,omitempty"`
	Runner       string   `json:"runner,omitempty"`
	Policy       string   `json:"policy,omitempty"`
	Evidence     []string `json:"evidence,omitempty"`

	Issue     *IssueState `json:"issue,omitempty"`
	Parents   []string    `json:"parents,omitempty"`
	Operation string      `json:"operation,omitempty"`
	Request   string      `json:"request,omitempty"`
	Intent    string      `json:"intent,omitempty"`
}

type StoredEvent struct {
	ID          string
	Commit      string
	Event       Event
	Payload     []byte
	Signature   []byte
	Attachments map[string][]byte
}

func eventID(payload []byte) string {
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func newEvent(identity *Identity, kind string, sequence uint64, previous string) Event {
	return Event{
		Protocol:  protocolVersion,
		Kind:      kind,
		Actor:     identity.Actor,
		ActorName: identity.Name,
		PublicKey: identity.PublicKey,
		Sequence:  sequence,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Previous:  previous,
	}
}

func encodeAndSign(event Event, identity *Identity) ([]byte, []byte, error) {
	if event.Protocol != protocolVersion || event.Actor != identity.Actor || event.PublicKey != identity.PublicKey {
		return nil, nil, fmt.Errorf("event identity does not match its signer")
	}
	if err := validateEventContent(event); err != nil {
		return nil, nil, err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, nil, err
	}
	privateKey, err := identity.privateKey()
	if err != nil {
		return nil, nil, err
	}
	signature := ed25519.Sign(privateKey, payload)
	return payload, signature, nil
}

func verifyEvent(payload, signature []byte) (Event, string, error) {
	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return Event{}, "", fmt.Errorf("invalid event JSON: %w", err)
	}
	if event.Protocol != protocolVersion {
		return Event{}, "", fmt.Errorf("unsupported protocol %q", event.Protocol)
	}
	publicBytes, err := base64.RawStdEncoding.DecodeString(event.PublicKey)
	if err != nil || len(publicBytes) != ed25519.PublicKeySize {
		return Event{}, "", fmt.Errorf("invalid event public key")
	}
	publicKey := ed25519.PublicKey(publicBytes)
	if actorForPublicKey(publicKey) != event.Actor {
		return Event{}, "", fmt.Errorf("event actor does not match public key")
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return Event{}, "", fmt.Errorf("invalid event signature")
	}
	if event.Sequence == 0 {
		return Event{}, "", fmt.Errorf("event sequence must be positive")
	}
	if _, err := time.Parse(time.RFC3339Nano, event.Timestamp); err != nil {
		return Event{}, "", fmt.Errorf("invalid event timestamp")
	}
	if err := validateEventContent(event); err != nil {
		return Event{}, "", err
	}
	return event, eventID(payload), nil
}

func validateEventContent(event Event) error {
	if err := validateIssueEvent(event); err != nil {
		return err
	}
	if event.Previous != "" && !validEventID(event.Previous) {
		return fmt.Errorf("invalid previous event ID")
	}
	switch event.Kind {
	case "issue.open":
		if strings.TrimSpace(event.Title) == "" {
			return fmt.Errorf("issue title cannot be empty")
		}
	case "issue.revise":
		// The complete issue-specific shape is validated above.
	case "issue.comment":
		if !validEventID(event.Subject) || strings.TrimSpace(event.Body) == "" {
			return fmt.Errorf("issue comment requires a subject and body")
		}
	case "proposal.open":
		if strings.TrimSpace(event.Title) == "" || !validGitOID(event.Base) || !validGitOID(event.Head) || event.Base == event.Head {
			return fmt.Errorf("proposal requires a title and distinct Git base/head commits")
		}
	case "proposal.revise":
		if !validEventID(event.Subject) || !validGitOID(event.Base) || !validGitOID(event.Head) || event.Base == event.Head {
			return fmt.Errorf("proposal revision requires a predecessor and distinct Git base/head commits")
		}
		if event.Title != "" {
			return fmt.Errorf("proposal revision inherits its title from its predecessor")
		}
	case "review.submit":
		if !validEventID(event.Subject) || (event.Verdict != "approve" && event.Verdict != "request-changes") {
			return fmt.Errorf("review requires a proposal subject and valid verdict")
		}
	case "run.request":
		if !validEventID(event.Subject) || !validPipelineName(event.Pipeline) || !validEventID(event.Definition) || !validGitOID(event.Commit) {
			return fmt.Errorf("run request requires a proposal, pipeline digest, and commit")
		}
	case "run.result":
		if !validEventID(event.Subject) || !validPipelineName(event.Pipeline) || !validEventID(event.Definition) || !validGitOID(event.Commit) {
			return fmt.Errorf("run result does not match a valid request shape")
		}
		if (event.Outcome != "passed" && event.Outcome != "failed") || !validEventID(event.Log) || event.DurationMS < 0 {
			return fmt.Errorf("run result requires an outcome, duration, and log digest")
		}
		if (event.Backend != "sandbox" && event.Backend != "host") || strings.TrimSpace(event.Platform) == "" || strings.TrimSpace(event.Runner) == "" {
			return fmt.Errorf("run result requires a supported backend, platform, and runner")
		}
	case "proposal.decision":
		if !validEventID(event.Subject) || !validEventID(event.Policy) || (event.Verdict != "accept" && event.Verdict != "reject") {
			return fmt.Errorf("proposal decision requires a proposal, policy digest, and verdict")
		}
		if event.Verdict == "reject" && strings.TrimSpace(event.Body) == "" {
			return fmt.Errorf("proposal rejection requires an explanation")
		}
		if !validEvidenceIDs(event.Evidence) {
			return fmt.Errorf("proposal decision has invalid evidence IDs")
		}
	case "proposal.merged":
		if !validEventID(event.Subject) || !validEventID(event.Policy) || !validGitOID(event.Commit) || !validGitOID(event.Head) || !validEvidenceIDs(event.Evidence) {
			return fmt.Errorf("proposal merge requires a proposal, policy, head, commit, and decision evidence")
		}
	case "identity.authorize":
		if event.Relationship != identityRelationshipDevice && event.Relationship != identityRelationshipSuccessor {
			return fmt.Errorf("identity authorization requires relationship device or successor")
		}
		if !validActorFingerprint(event.TargetActor) {
			return fmt.Errorf("identity authorization requires a full target actor")
		}
		targetKey, err := base64.RawStdEncoding.DecodeString(event.TargetKey)
		if err != nil || len(targetKey) != ed25519.PublicKeySize {
			return fmt.Errorf("identity authorization has an invalid target public key")
		}
		if actorForPublicKey(ed25519.PublicKey(targetKey)) != event.TargetActor {
			return fmt.Errorf("identity authorization target actor does not match target public key")
		}
		if event.TargetActor == event.Actor {
			return fmt.Errorf("identity authorization cannot target its signer")
		}
	case "identity.accept":
		if !validEventID(event.Subject) {
			return fmt.Errorf("identity acceptance requires a full authorization event ID")
		}
	default:
		return fmt.Errorf("unsupported event kind %q", event.Kind)
	}
	return nil
}

func isProposalKind(kind string) bool {
	return kind == "proposal.open" || kind == "proposal.revise"
}

func validEvidenceIDs(evidence []string) bool {
	seen := make(map[string]bool)
	for _, id := range evidence {
		if !validEventID(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func validEventID(id string) bool {
	if !strings.HasPrefix(id, "sha256:") {
		return false
	}
	encoded := strings.TrimPrefix(id, "sha256:")
	decoded, err := hex.DecodeString(encoded)
	return err == nil && len(decoded) == sha256.Size
}

func validGitOID(id string) bool {
	decoded, err := hex.DecodeString(id)
	return err == nil && (len(decoded) == 20 || len(decoded) == 32)
}

func shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
