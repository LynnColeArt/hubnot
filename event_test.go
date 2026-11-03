package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"testing"
)

func testIdentity(t *testing.T, name string) *Identity {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &Identity{
		Actor:      actorForPublicKey(publicKey),
		Name:       name,
		PublicKey:  base64.RawStdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.RawStdEncoding.EncodeToString(privateKey),
	}
}

func TestSignedEventRoundTrip(t *testing.T) {
	identity := testIdentity(t, "Alice")
	event := newEvent(identity, "issue.open", 1, "")
	event.Title = "Distributed issues"

	payload, signature, err := encodeAndSign(event, identity)
	if err != nil {
		t.Fatal(err)
	}
	got, id, err := verifyEvent(payload, signature)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != event.Title {
		t.Fatalf("title = %q, want %q", got.Title, event.Title)
	}
	if len(id) != len("sha256:")+64 {
		t.Fatalf("unexpected event ID %q", id)
	}
}

func TestModifiedEventFailsVerification(t *testing.T) {
	identity := testIdentity(t, "Alice")
	event := newEvent(identity, "issue.open", 1, "")
	event.Title = "Original"
	payload, signature, err := encodeAndSign(event, identity)
	if err != nil {
		t.Fatal(err)
	}
	payload = bytes.Replace(payload, []byte("Original"), []byte("Modified"), 1)
	if _, _, err := verifyEvent(payload, signature); err == nil {
		t.Fatal("modified event unexpectedly verified")
	}
}

const actorChainContractActor = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func actorChainContractEvent(id string, sequence uint64, previous string) StoredEvent {
	return StoredEvent{ID: id, Commit: "commit-" + id,
		Event:   Event{Protocol: protocolVersion, Kind: "issue.open", Actor: actorChainContractActor, Sequence: sequence, Previous: previous, Title: "retained " + id},
		Payload: []byte("payload-" + id), Signature: []byte("signature-" + id), Attachments: map[string][]byte{"proof": []byte("attachment-" + id)}}
}

func TestValidateActorChainsContract(t *testing.T) {
	one := actorChainContractEvent("one", 1, "")
	two := actorChainContractEvent("two", 2, "one")
	three := actorChainContractEvent("three", 3, "two")
	other := actorChainContractEvent("other", 1, "")
	other.Event.Actor = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	actor := shortID(actorChainContractActor)
	cases := []struct {
		name   string
		events []StoredEvent
		want   []string
	}{
		{"nil", nil, []string{""}},
		{"empty", []StoredEvent{}, []string{""}},
		{"one", []StoredEvent{one}, []string{""}},
		{"shuffled", []StoredEvent{three, one, two}, []string{""}},
		{"interleaved actors", []StoredEvent{three, other, one, two}, []string{""}},
		{"zero", []StoredEvent{actorChainContractEvent("zero", 0, "")}, []string{"actor " + actor + " has invalid sequence 0"}},
		{"first gap", []StoredEvent{two}, []string{"actor " + actor + " has invalid sequence 2"}},
		{"later gap", []StoredEvent{one, three}, []string{"actor " + actor + " has invalid sequence 3"}},
		{"uint64 maximum", []StoredEvent{actorChainContractEvent("max", ^uint64(0), "")}, []string{"actor " + actor + " has invalid sequence 18446744073709551615"}},
		{"initial predecessor", []StoredEvent{actorChainContractEvent("one", 1, "unexpected")}, []string{"actor " + actor + " has a broken event chain at sequence 1"}},
		{"later predecessor", []StoredEvent{one, actorChainContractEvent("two", 2, "wrong")}, []string{"actor " + actor + " has a broken event chain at sequence 2"}},
		{"different ID duplicate before gap", []StoredEvent{one, actorChainContractEvent("another", 1, "")}, []string{"actor " + actor + " has conflicting sequence 1"}},
		{"same ID duplicate not deduplicated", []StoredEvent{one, one}, []string{"actor " + actor + " has invalid sequence 1"}},
		{"same ID duplicate later", []StoredEvent{two, one, two}, []string{"actor " + actor + " has invalid sequence 2"}},
		{"earlier gap before later conflict", []StoredEvent{three, actorChainContractEvent("another-three", 3, "two")}, []string{"actor " + actor + " has invalid sequence 3"}},
		{"earlier link before later conflict", []StoredEvent{actorChainContractEvent("one", 1, "wrong"), two, actorChainContractEvent("another-two", 2, "one")}, []string{"actor " + actor + " has a broken event chain at sequence 1"}},
		{"equal sequence first link wins", []StoredEvent{one, actorChainContractEvent("broken-two", 2, "wrong"), two}, []string{"actor " + actor + " has a broken event chain at sequence 2"}},
		{"equal sequence reversed conflict wins", []StoredEvent{one, two, actorChainContractEvent("broken-two", 2, "wrong")}, []string{"actor " + actor + " has conflicting sequence 2"}},
	}
	badOther := other
	badOther.Event.Previous = "unexpected"
	cases = append(cases, struct {
		name   string
		events []StoredEvent
		want   []string
	}{"cross actor priority unspecified", []StoredEvent{badOther, two}, []string{"actor " + actor + " has invalid sequence 2", "actor " + shortID(other.Event.Actor) + " has a broken event chain at sequence 1"}})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, check := range []struct {
				name     string
				validate func([]StoredEvent) error
			}{{"production", validateActorChains}, {"frozen H541 oracle", actorChainOldValidator}} {
				err := check.validate(tc.events)
				got := ""
				if err != nil {
					got = err.Error()
				}
				matched := false
				for _, want := range tc.want {
					if got == want {
						matched = true
					}
				}
				if !matched {
					t.Fatalf("%s error = %q, want one of exact contract outcomes %q", check.name, got, tc.want)
				}
			}
		})
	}
}

func TestValidateActorChainsDoesNotMutateInput(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(strconv.FormatBool(invalid), func(t *testing.T) {
			events := []StoredEvent{actorChainContractEvent("three", 3, "two"), actorChainContractEvent("one", 1, ""), actorChainContractEvent("two", 2, "one")}
			events[1].Event.Issue = &IssueState{Title: "deep state", Labels: []string{"retained"}, Metadata: map[string]string{"key": "value"}}
			if invalid {
				events[2].Event.Previous = "wrong"
			}
			before, err := json.Marshal(events)
			if err != nil {
				t.Fatal(err)
			}
			err = validateActorChains(events)
			if (err != nil) != invalid {
				t.Fatalf("error = %v, invalid=%v", err, invalid)
			}
			after, err := json.Marshal(events)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("validator mutated event order or nested payload/signature/state/attachment data")
			}
		})
	}
}

func BenchmarkActorChainTransientStorage(b *testing.B) {
	for _, count := range []int{8, 100000} {
		events := make([]StoredEvent, count)
		previous := ""
		for i := range events {
			id := fmt.Sprintf("sha256:%064x", i+1)
			events[i] = StoredEvent{ID: id, Event: Event{Actor: actorChainContractActor, Sequence: uint64(i + 1), Previous: previous}}
			previous = id
		}
		b.Run(fmt.Sprintf("events%d", count), func(b *testing.B) {
			for _, variant := range []struct {
				name     string
				validate func([]StoredEvent) error
			}{{"old-H541-map-copy", actorChainOldValidator}, {"production", validateActorChains}} {
				b.Run(variant.name, func(b *testing.B) {
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if err := variant.validate(events); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}

// actorChainOldValidator freezes the H541 validator for behavior/allocation comparison only.
func actorChainOldValidator(events []StoredEvent) error {
	byActor := make(map[string][]StoredEvent)
	for _, event := range events {
		byActor[event.Event.Actor] = append(byActor[event.Event.Actor], event)
	}
	for actor, chain := range byActor {
		sort.Slice(chain, func(i, j int) bool { return chain[i].Event.Sequence < chain[j].Event.Sequence })
		seen := make(map[uint64]string)
		for index, stored := range chain {
			if existing, ok := seen[stored.Event.Sequence]; ok && existing != stored.ID {
				return fmt.Errorf("actor %s has conflicting sequence %s", shortID(actor), strconv.FormatUint(stored.Event.Sequence, 10))
			}
			seen[stored.Event.Sequence] = stored.ID
			wantSequence := uint64(index + 1)
			if stored.Event.Sequence != wantSequence {
				return fmt.Errorf("actor %s has invalid sequence %s", shortID(actor), strconv.FormatUint(stored.Event.Sequence, 10))
			}
			wantPrevious := ""
			if index > 0 {
				wantPrevious = chain[index-1].ID
			}
			if stored.Event.Previous != wantPrevious {
				return fmt.Errorf("actor %s has a broken event chain at sequence %s", shortID(actor), strconv.FormatUint(stored.Event.Sequence, 10))
			}
		}
	}
	return nil
}

func TestValidateActorChainsLongTiePermutations(t *testing.T) {
	actor := shortID(actorChainContractActor)
	for _, variant := range []string{"different ID", "same ID", "wrong link tie", "earlier gap", "earlier broken link"} {
		base := make([]StoredEvent, 32)
		for i := range base {
			previous := ""
			if i > 0 {
				previous = fmt.Sprintf("event%d", i)
			}
			base[i] = actorChainContractEvent(fmt.Sprintf("event%d", i+1), uint64(i+1), previous)
		}
		duplicate := actorChainContractEvent("another17", 17, "event16")
		if variant == "same ID" {
			duplicate.ID = "event17"
		}
		if variant == "wrong link tie" {
			duplicate.Event.Previous = "wrong"
		}
		if variant == "earlier gap" {
			base = append(base[:6], base[7:]...)
		}
		if variant == "earlier broken link" {
			base[6].Event.Previous = "wrong"
		}
		base = append(base, duplicate)
		for _, order := range []string{"ordered", "reversed", "rotated", "odd then even", "affine"} {
			t.Run(variant+"/"+order, func(t *testing.T) {
				events := make([]StoredEvent, 0, len(base))
				switch order {
				case "ordered":
					events = append(events, base...)
				case "reversed":
					for i := len(base) - 1; i >= 0; i-- {
						events = append(events, base[i])
					}
				case "rotated":
					events = append(events, base[11:]...)
					events = append(events, base[:11]...)
				case "odd then even":
					for parity := 1; parity >= 0; parity-- {
						for i := parity; i < len(base); i += 2 {
							events = append(events, base[i])
						}
					}
				case "affine":
					for i := range base {
						events = append(events, base[(5*i+3)%len(base)])
					}
				}
				// Both32 and33 rows exceed sort.Slice's small insertion-sort case.
				before, err := json.Marshal(events)
				if err != nil {
					t.Fatal(err)
				}
				old := actorChainOldValidator(events)
				if old == nil {
					t.Fatal("invalid long fixture unexpectedly accepted by frozen validator")
				}
				want := "actor " + actor + " has conflicting sequence 17"
				switch variant {
				case "same ID":
					want = "actor " + actor + " has invalid sequence 17"
				case "earlier gap":
					want = "actor " + actor + " has invalid sequence 8"
				case "earlier broken link":
					want = "actor " + actor + " has a broken event chain at sequence 7"
				case "wrong link tie":
					broken := "actor " + actor + " has a broken event chain at sequence 17"
					if old.Error() != want && old.Error() != broken {
						t.Fatalf("frozen validator error %q outside exact tie-priority outcomes", old)
					}
					// Unstable tie order is frozen per input permutation, not replaced
					// with an ID tie-break or a global conflict prescan.
					want = old.Error()
				}
				if old.Error() != want {
					t.Fatalf("frozen validator = %q, want %q", old, want)
				}
				got := validateActorChains(events)
				if got == nil || got.Error() != want {
					t.Fatalf("production = %v, want exact %q", got, want)
				}
				after, err := json.Marshal(events)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before, after) {
					t.Fatal("long-sort validation mutated input")
				}
				t.Logf("frozen and production outcome: %s", want)
			})
		}
	}
}
