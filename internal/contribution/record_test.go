package contribution_test

import (
	"bytes"
	"crypto/ed25519"
	c "hubnot/internal/contribution"
	"testing"
)

func TestStrictSignedEnvelope(t *testing.T) {
	s := submission(t, "one", content("issue"), c.IssueCreate)
	raw := s.Payload()
	sig := s.Signature()
	if _, err := c.Verify(raw, sig); err != nil {
		t.Fatal(err)
	}
	for name, payload := range map[string][]byte{"trailing": append(append([]byte{}, raw...), []byte(" {}")...), "whitespace": append([]byte(" "), raw...), "unknown": bytes.Replace(raw, []byte(`"version":`), []byte(`"extra":true,"version":`), 1), "duplicate": bytes.Replace(raw, []byte(`"kind":"submission"`), []byte(`"kind":"decision","kind":"submission"`), 1), "case": bytes.Replace(raw, []byte(`"actor":`), []byte(`"Actor":`), 1), "wrong-project": bytes.Replace(raw, []byte(policy().Project), []byte(digest("other")), 1)} {
		newSig := ed25519.Sign(key(1), payload)
		if name == "wrong-project" {
			newSig = sig
		}
		if _, err := c.Verify(payload, newSig); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	bad := append([]byte{}, sig...)
	bad[0] ^= 1
	if _, err := c.Verify(raw, bad); err == nil {
		t.Fatal("altered signature accepted")
	}
	if _, err := c.Verify(raw, ed25519.Sign(key(3), raw)); err == nil {
		t.Fatal("different key accepted")
	}
	if _, err := c.Verify(bytes.Repeat([]byte("x"), c.MaxRecordBytes+1), sig); err == nil {
		t.Fatal("oversize accepted")
	}
	clone := s.Record()
	clone.Submission.Content.Parents = append(clone.Submission.Content.Parents, digest("fake"))
	if len(s.Record().Submission.Content.Parents) != 0 {
		t.Fatal("verified record mutable through accessor")
	}
}
func TestDecisionTypedTargetsAndStaticValidation(t *testing.T) {
	for _, entry := range []struct {
		action c.Action
		target c.Target
	}{{c.Hide, c.Target{Kind: c.ActorTarget, ID: signer(1)}}, {c.Block, c.Target{Kind: c.RevisionTarget, ID: digest("issue")}}, {c.Admit, c.Target{Kind: c.RevisionTarget, ID: digest("issue")}}} {
		r := c.Record{Version: c.Version, Kind: c.DecisionKind, Project: policy().Project, Actor: signer(2), PublicKey: c.PublicKey(key(2)), Intent: "bad", Decision: &c.Decision{Action: entry.action, Target: entry.target, Authority: authority().Ref}}
		if _, err := c.Sign(r, key(2)); err == nil {
			t.Fatal("mismatched decision action/target accepted")
		}
	}
	r := submission(t, "valid", content("native"), c.IssueCreate).Record()
	r.Actor = signer(3)
	if _, err := c.Sign(r, key(1)); err == nil {
		t.Fatal("signer mismatch accepted")
	}
}
