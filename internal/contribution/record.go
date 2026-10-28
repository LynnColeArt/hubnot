// Package contribution defines optional contribution envelopes and policy after
// native authentication. It never reads a keyring, performs I/O or verifies native
// Git/event formats; the host supplies the existing authoritative verifiers.
package contribution

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const Version = "hn.contribution/1"
const MaxRecordBytes = 64 << 10
const MaxLinks = 64
const MaxGraphRecords = 16384

type Kind string

const (
	SubmissionKind Kind = "submission"
	DecisionKind   Kind = "decision"
	ReceiptKind    Kind = "receipt"
)

type Action string

const (
	Admit   Action = "admit"
	Reject  Action = "reject"
	Hide    Action = "hide"
	Restore Action = "restore"
	Block   Action = "block"
	Unblock Action = "unblock"
)

type TargetKind string

const (
	SubmissionTarget TargetKind = "submission"
	RevisionTarget   TargetKind = "revision"
	ActorTarget      TargetKind = "actor"
)

type Target struct {
	Kind TargetKind `json:"kind"`
	ID   string     `json:"id"`
}
type AuthorityRef struct {
	Commit string `json:"commit"`
	Digest string `json:"digest"`
}
type Content struct {
	Root       string   `json:"root"`
	Revision   string   `json:"revision"`
	Author     string   `json:"author"`
	Parents    []string `json:"parents"`
	References []string `json:"references"`
}
type Submission struct {
	Operation Operation `json:"operation"`
	Content   Content   `json:"content"`
}
type Decision struct {
	Action    Action       `json:"action"`
	Target    Target       `json:"target"`
	Authority AuthorityRef `json:"authority"`
	Parents   []string     `json:"parents"`
}

// Receipt attests completed bounded intake; Accounting binds the durable local
// transition, while the project signature supplies portable admission provenance.
type Receipt struct {
	Submission string       `json:"submission"`
	Authority  AuthorityRef `json:"authority"`
	Artifact   string       `json:"artifact"`
	Accounting string       `json:"accounting"`
	Window     string       `json:"window"`
	Cost       Cost         `json:"cost"`
	Automatic  bool         `json:"automatic"`
	Evaluated  []string     `json:"evaluated"`
}
type Record struct {
	Version    string      `json:"version"`
	Kind       Kind        `json:"kind"`
	Project    string      `json:"project"`
	Actor      string      `json:"actor"`
	PublicKey  string      `json:"publicKey"`
	Intent     string      `json:"intent"`
	Submission *Submission `json:"submission,omitempty"`
	Decision   *Decision   `json:"decision,omitempty"`
	Receipt    *Receipt    `json:"receipt,omitempty"`
}
type Envelope struct {
	Payload   []byte
	Signature []byte
}

// Verified cannot be constructed with authenticated content outside this package.
// Accessors copy data, so callers cannot mutate an authenticated record.
type Verified struct {
	payload   []byte
	signature []byte
	id        string
}

func (v Verified) ID() string          { return v.id }
func (v Verified) Payload() []byte     { return bytes.Clone(v.payload) }
func (v Verified) Signature() []byte   { return bytes.Clone(v.signature) }
func (v Verified) Record() Record      { var r Record; _ = json.Unmarshal(v.payload, &r); return r }
func Digest(b []byte) string           { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func Actor(k ed25519.PublicKey) string { h := sha256.Sum256(k); return hex.EncodeToString(h[:]) }
func PublicKey(k ed25519.PrivateKey) string {
	if len(k) != ed25519.PrivateKeySize {
		return ""
	}
	return base64.RawStdEncoding.EncodeToString(k.Public().(ed25519.PublicKey))
}
func SortIDs(ids []string) { sort.Strings(ids) }
func validHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
func validID(s string) bool    { return strings.HasPrefix(s, "sha256:") && validHex(s[7:], 64) }
func validActor(s string) bool { return validHex(s, 64) }
func validOID(s string) bool   { return validHex(s, 40) || validHex(s, 64) }
func boundedText(s string, n int) bool {
	if s == "" || len(s) > n || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
func validIDs(ids []string, max int) bool {
	if len(ids) > max {
		return false
	}
	for i, id := range ids {
		if !validID(id) || (i > 0 && ids[i-1] >= id) {
			return false
		}
	}
	return true
}
func validActors(ids []string, max int) bool {
	if len(ids) > max {
		return false
	}
	for i, id := range ids {
		if !validActor(id) || (i > 0 && ids[i-1] >= id) {
			return false
		}
	}
	return true
}
func (r AuthorityRef) validate() error {
	if !validOID(r.Commit) || !validID(r.Digest) {
		return ErrAuthority
	}
	return nil
}
func (c Content) validate() error {
	if !validID(c.Root) || !validID(c.Revision) || !validActor(c.Author) || !validIDs(c.Parents, MaxLinks) || !validIDs(c.References, MaxLinks) {
		return fmt.Errorf("invalid exact content binding")
	}
	for _, id := range append(append([]string{}, c.Parents...), c.References...) {
		if id == c.Revision {
			return fmt.Errorf("self-referencing content")
		}
	}
	return nil
}
func (t Target) validate() error {
	switch t.Kind {
	case SubmissionTarget, RevisionTarget:
		if validID(t.ID) {
			return nil
		}
	case ActorTarget:
		if validActor(t.ID) {
			return nil
		}
	}
	return fmt.Errorf("invalid typed target")
}
func (d Decision) validate() error {
	if err := d.Target.validate(); err != nil {
		return err
	}
	if err := d.Authority.validate(); err != nil {
		return err
	}
	if !validIDs(d.Parents, MaxLinks) {
		return fmt.Errorf("invalid decision parents")
	}
	switch d.Action {
	case Admit, Reject:
		if d.Target.Kind == SubmissionTarget {
			return nil
		}
	case Hide, Restore:
		if d.Target.Kind == RevisionTarget {
			return nil
		}
	case Block, Unblock:
		if d.Target.Kind == ActorTarget {
			return nil
		}
	}
	return fmt.Errorf("action does not match typed target")
}
func (r Record) Validate() error {
	if r.Version != Version || !validID(r.Project) || !validActor(r.Actor) || !boundedText(r.Intent, 128) {
		return fmt.Errorf("invalid contribution envelope")
	}
	public, err := base64.RawStdEncoding.DecodeString(r.PublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize || base64.RawStdEncoding.EncodeToString(public) != r.PublicKey || Actor(public) != r.Actor {
		return fmt.Errorf("actor does not match native public key")
	}
	switch r.Kind {
	case SubmissionKind:
		if r.Submission == nil || r.Decision != nil || r.Receipt != nil || !contentOperation(r.Submission.Operation) {
			return fmt.Errorf("invalid submission shape")
		}
		return r.Submission.Content.validate()
	case DecisionKind:
		if r.Decision == nil || r.Submission != nil || r.Receipt != nil {
			return fmt.Errorf("invalid decision shape")
		}
		return r.Decision.validate()
	case ReceiptKind:
		if r.Receipt == nil || r.Submission != nil || r.Decision != nil {
			return fmt.Errorf("invalid receipt shape")
		}
		x := r.Receipt
		if !validID(x.Submission) || !validID(x.Artifact) || !validID(x.Accounting) || !boundedText(x.Window, 128) || !validIDs(x.Evaluated, MaxLinks) {
			return fmt.Errorf("invalid intake receipt binding")
		}
		if err := x.Authority.validate(); err != nil {
			return err
		}
		return x.Cost.validate()
	}
	return fmt.Errorf("unsupported contribution kind")
}
func Sign(r Record, key ed25519.PrivateKey) (Envelope, error) {
	if err := r.Validate(); err != nil {
		return Envelope{}, err
	}
	if len(key) != ed25519.PrivateKeySize || PublicKey(key) != r.PublicKey {
		return Envelope{}, fmt.Errorf("signer mismatch")
	}
	// Reject malformed private-key encodings whose embedded public key differs.
	canonical := ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])
	if !bytes.Equal(canonical, key) {
		return Envelope{}, fmt.Errorf("invalid private key")
	}
	payload, err := json.Marshal(r)
	if err != nil || len(payload) > MaxRecordBytes {
		return Envelope{}, fmt.Errorf("envelope exceeds format bound")
	}
	return Envelope{payload, ed25519.Sign(key, payload)}, nil
}
func Verify(payload, signature []byte) (Verified, error) {
	if len(payload) == 0 || len(payload) > MaxRecordBytes || !utf8.Valid(payload) || len(signature) != ed25519.SignatureSize {
		return Verified{}, fmt.Errorf("invalid envelope size/encoding")
	}
	var r Record
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return Verified{}, fmt.Errorf("invalid envelope JSON: %w", err)
	}
	canonical, err := json.Marshal(r)
	if err != nil || !bytes.Equal(canonical, payload) {
		return Verified{}, fmt.Errorf("noncanonical, duplicate or trailing envelope fields")
	}
	if err = r.Validate(); err != nil {
		return Verified{}, err
	}
	public, _ := base64.RawStdEncoding.DecodeString(r.PublicKey)
	if !ed25519.Verify(public, payload, signature) {
		return Verified{}, fmt.Errorf("invalid contribution signature")
	}
	return Verified{bytes.Clone(payload), bytes.Clone(signature), Digest(payload)}, nil
}

var ErrMissingDependency = errors.New("missing exact native dependency")
var ErrAuthority = errors.New("invalid, stale or unestablished authority")
