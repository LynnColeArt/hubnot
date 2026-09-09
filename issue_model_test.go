package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestRichIssueSignedCompatibility(t *testing.T) {
	identity := testIdentity(t, "Alice")
	legacy := newEvent(identity, "issue.open", 1, "")
	legacy.Timestamp = "2026-09-09T00:00:00Z"
	legacy.Title = "Legacy"
	payload, sig, err := encodeAndSign(legacy, identity)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"protocol":"hn/0","kind":"issue.open","actor":"` + identity.Actor + `","actorName":"Alice","publicKey":"` + identity.PublicKey + `","sequence":1,"timestamp":"2026-09-09T00:00:00Z","title":"Legacy"}`
	if string(payload) != expected {
		t.Fatalf("legacy bytes changed: %s", payload)
	}
	if _, _, err = verifyEvent(payload, sig); err != nil {
		t.Fatal(err)
	}
	legacy.Title = strings.Repeat("x", 1025)
	if _, _, err = encodeAndSign(legacy, identity); err != nil {
		t.Fatalf("legacy limit changed: %v", err)
	}
	legacy.Issue = &IssueState{Title: legacy.Title, Status: "open"}
	if _, _, err = encodeAndSign(legacy, identity); err == nil {
		t.Fatal("rich oversized title accepted")
	}
	event := newEvent(identity, "issue.revise", 2, eventID(payload))
	event.Subject = eventID(payload)
	event.Parents = []string{event.Subject}
	event.Intent = "revise"
	state := IssueState{Title: "Rich", Body: "description", Status: "open", Criteria: []IssueCriterion{{ID: "c1", Text: "works"}}, Labels: []string{"z", "a"}, Assignees: []string{"human"}, Metadata: map[string]string{"example/context": "value"}}
	canonical, err := CanonicalIssueState(state)
	if err != nil {
		t.Fatal(err)
	}
	event.Issue = &canonical
	event.Operation = "request-1"
	event.Request, err = IssueRequestDigest(event)
	if err != nil {
		t.Fatal(err)
	}
	rich, sig, err := encodeAndSign(event, identity)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := verifyEvent(rich, sig)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Issue, &canonical) {
		t.Fatalf("state lost: %#v", got.Issue)
	}
	for _, old := range []string{"Rich", "request-1", "revise"} {
		bad := bytes.Replace(rich, []byte(old), []byte(old+"x"), 1)
		if _, _, err := verifyEvent(bad, sig); err == nil {
			t.Fatalf("tampered %s accepted", old)
		}
	}
	if state.Labels[0] != "z" {
		t.Fatal("canonicalization mutated caller")
	}
}

func TestRichIssueShapeAndOperationValidation(t *testing.T) {
	root := eventID([]byte("root"))
	state := IssueState{Title: "Title", Status: "open"}
	base := Event{Kind: "issue.revise", Subject: root, Parents: []string{root}, Issue: &state, Intent: "revise"}
	cases := map[string]func(*Event){
		"no parents": func(e *Event) { e.Parents = nil }, "bad parent": func(e *Event) { e.Parents = []string{"short"} }, "duplicate parents": func(e *Event) { e.Parents = []string{root, root} }, "missing state": func(e *Event) { e.Issue = nil }, "title": func(e *Event) { e.Title = "other" }, "body": func(e *Event) { e.Body = "other" }, "bad intent": func(e *Event) { e.Intent = "open" }, "wrong close state": func(e *Event) { e.Intent = "close" }, "orphan operation": func(e *Event) { e.Operation = "op" }, "orphan digest": func(e *Event) { e.Request = root },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := base
			mutate(&e)
			if err := validateEventContent(e); err == nil {
				t.Fatal("invalid event accepted")
			}
		})
	}
	for _, kind := range []string{"proposal.open", "issue.comment", "issue.open"} {
		e := base
		e.Kind = kind
		if err := validateEventContent(e); err == nil {
			t.Fatalf("wrong variant %s accepted", kind)
		}
	}
	base.Operation = "op"
	base.Request, _ = IssueRequestDigest(base)
	if err := validateEventContent(base); err != nil {
		t.Fatal(err)
	}
	base.Request = root
	if err := validateEventContent(base); err == nil {
		t.Fatal("false semantic digest accepted")
	}
}

func TestRichIssueStateBounds(t *testing.T) {
	cases := map[string]func(*IssueState){
		"status": func(s *IssueState) { s.Status = "done" }, "blank title": func(s *IssueState) { s.Title = " " }, "long body": func(s *IssueState) { s.Body = strings.Repeat("x", 65537) }, "duplicate criteria": func(s *IssueState) { s.Criteria = []IssueCriterion{{"c", "a"}, {"c", "b"}} }, "bad key": func(s *IssueState) { s.Metadata = map[string]string{"plain": "x"} }, "empty prefix": func(s *IssueState) { s.Metadata = map[string]string{"/x": "x"} }, "empty name": func(s *IssueState) { s.Metadata = map[string]string{"x/": "x"} }, "duplicate labels": func(s *IssueState) { s.Labels = []string{"a", "a"} }, "blank assignee": func(s *IssueState) { s.Assignees = []string{"\n"} }, "bad relation": func(s *IssueState) { s.Relations = []IssueRelation{{"unknown", eventID([]byte("target"))}} }, "long metadata": func(s *IssueState) { s.Metadata = map[string]string{"a/b": strings.Repeat("x", 4097)} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := IssueState{Title: "Title", Status: "open"}
			mutate(&s)
			if _, err := CanonicalIssueState(s); err == nil {
				t.Fatal("invalid state accepted")
			}
		})
	}
	s := IssueState{Title: strings.Repeat("x", 1024), Body: strings.Repeat("x", 65536), Status: "closed"}
	if _, err := CanonicalIssueState(s); err != nil {
		t.Fatal(err)
	}
	a := IssueState{Title: "Title", Status: "open", Labels: []string{"b", "a"}, Criteria: []IssueCriterion{{"b", "second"}, {"a", "first"}}}
	b, err := CanonicalIssueState(a)
	if err != nil {
		t.Fatal(err)
	}
	if b.Labels[0] != "a" || b.Criteria[0].ID != "b" {
		t.Fatal("wrong canonical ordering")
	}
	encoded, _ := json.Marshal(b)
	if bytes.Contains(encoded, []byte("null")) {
		t.Fatalf("null optional collections: %s", encoded)
	}
}

func TestRichIssueOperationSemanticIdentity(t *testing.T) {
	s := IssueState{Title: "Title", Status: "open", Labels: []string{"b", "a"}}
	e := Event{Kind: "issue.open", Title: s.Title, Issue: &s, Intent: "open", Actor: "actor-a", Operation: "key"}
	digest, err := IssueRequestDigest(e)
	if err != nil {
		t.Fatal(err)
	}
	e.Actor = "actor-b"
	e.Operation = "other"
	e.Timestamp = "later"
	e.Sequence = 99
	e.Previous = eventID([]byte("prev"))
	e.Issue.Labels = []string{"a", "b"}
	same, err := IssueRequestDigest(e)
	if err != nil || same != digest {
		t.Fatal("transport, scope or canonical set order changed semantic digest")
	}
	e.Issue.Title = "changed"
	e.Title = "changed"
	changed, _ := IssueRequestDigest(e)
	if changed == digest {
		t.Fatal("content not bound")
	}
	e = Event{Kind: "issue.revise", Subject: eventID([]byte("root")), Parents: []string{eventID([]byte("parent"))}, Issue: &IssueState{Title: "x", Status: "closed"}, Intent: "close"}
	closed, _ := IssueRequestDigest(e)
	e.Intent = "revise"
	revised, _ := IssueRequestDigest(e)
	if closed == revised {
		t.Fatal("convenience intent not bound")
	}
}

func TestRichIssueCollectionAndAggregateLimits(t *testing.T) {
	state := IssueState{Title: "x", Status: "open"}
	for i := 0; i < 64; i++ {
		value := fmt.Sprintf("%0256d", i)
		state.Labels = append(state.Labels, value)
	}
	if _, err := CanonicalIssueState(state); err != nil {
		t.Fatal(err)
	}
	state.Labels = append(state.Labels, "extra")
	if _, err := CanonicalIssueState(state); err == nil {
		t.Fatal("65 labels accepted")
	}
	state = IssueState{Title: "x", Status: "open", Metadata: map[string]string{}}
	for i := 0; i < 64; i++ {
		state.Metadata[fmt.Sprintf("test/%d", i)] = strings.Repeat("x", 4096)
	}
	if _, err := CanonicalIssueState(state); err == nil {
		t.Fatal("aggregate limit not enforced")
	}
	state = IssueState{Title: "x", Status: "open"}
	for i := 0; i < 128; i++ {
		state.Relations = append(state.Relations, IssueRelation{"related", eventID([]byte{byte(i)})})
	}
	if _, err := CanonicalIssueState(state); err != nil {
		t.Fatal(err)
	}
	state.Relations = append(state.Relations, IssueRelation{"blocks", eventID([]byte("extra"))})
	if _, err := CanonicalIssueState(state); err == nil {
		t.Fatal("129 relations accepted")
	}
}

func TestRichIssueRawSignedPayloadLimit(t *testing.T) {
	identity := testIdentity(t, "Raw payload")
	key, err := identity.privateKey()
	if err != nil {
		t.Fatal(err)
	}
	opening := newEvent(identity, "issue.open", 1, "")
	opening.Title = "rich"
	opening.Intent = "open"
	opening.Issue = &IssueState{Title: opening.Title, Status: "open"}
	comment := newEvent(identity, "issue.comment", 1, "")
	comment.Subject = eventID([]byte("root"))
	comment.Body = "comment"
	comment.Intent = "comment"
	revision := newEvent(identity, "issue.revise", 1, "")
	revision.Subject = comment.Subject
	revision.Parents = []string{comment.Subject}
	revision.Issue = opening.Issue
	revision.Intent = "revise"
	for _, event := range []Event{opening, comment, revision} {
		t.Run(event.Kind, func(t *testing.T) {
			canonical, _, err := encodeAndSign(event, identity)
			if err != nil {
				t.Fatal(err)
			}
			for _, style := range []string{"unknown-field", "whitespace"} {
				t.Run(style, func(t *testing.T) {
					for _, size := range []int{maxIssuePayload, maxIssuePayload + 1} {
						payload := append([]byte{}, canonical...)
						if style == "unknown-field" {
							payload = append(payload[:len(payload)-1], []byte(`,"unknown":""}`)...)
							padding := bytes.Repeat([]byte("x"), size-len(payload))
							payload = append(append(payload[:len(payload)-2], padding...), '"', '}')
						} else {
							payload = append(payload, bytes.Repeat([]byte(" "), size-len(payload))...)
						}
						if len(payload) != size {
							t.Fatalf("fixture bytes = %d, want %d", len(payload), size)
						}
						signature := ed25519.Sign(key, payload)
						_, _, err := verifyEvent(payload, signature)
						if (err != nil) != (size > maxIssuePayload) {
							t.Fatalf("verify signed %s payload of %d bytes: %v", style, size, err)
						}
					}
				})
			}
		})
	}
	legacy := newEvent(identity, "issue.open", 1, "")
	legacy.Title = "legacy"
	legacy.Body = strings.Repeat("x", maxIssuePayload+1)
	payload, signature, err := encodeAndSign(legacy, identity)
	if err != nil {
		t.Fatal(err)
	}
	verified, id, err := verifyEvent(payload, signature)
	if err != nil {
		t.Fatalf("legacy large signed event rejected: %v", err)
	}
	if verified.Body != legacy.Body || id != eventID(payload) {
		t.Fatal("legacy signed content changed")
	}
}
