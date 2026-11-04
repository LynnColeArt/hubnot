package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

const maxRunWithdrawalReason = 4096

// Only this new kind has the narrow envelope. Legacy signed event decoding is
// deliberately unchanged, including its treatment of undeclared JSON fields.
func strictRunWithdrawalJSON(raw []byte, event *Event) error {
	if err := strictResourceJSON(raw, event, len(raw)); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	allowed := strings.Fields("protocol kind actor actorName publicKey sequence timestamp previous subject body")
	for key := range fields {
		if !stringListed(key, allowed) {
			return fmt.Errorf("field %q is not declared for run.withdraw", key)
		}
	}
	return nil
}

func validateRunWithdrawalContent(event Event) error {
	if !validEventID(event.Subject) {
		return fmt.Errorf("run withdrawal requires a full run request ID")
	}
	if !utf8.ValidString(event.Body) || event.Body == "" || strings.TrimSpace(event.Body) != event.Body || len(event.Body) > maxRunWithdrawalReason {
		return fmt.Errorf("run withdrawal reason must be trimmed nonblank UTF-8 and at most %d bytes", maxRunWithdrawalReason)
	}
	envelope := Event{Protocol: event.Protocol, Kind: event.Kind, Actor: event.Actor, ActorName: event.ActorName, PublicKey: event.PublicKey, Sequence: event.Sequence, Timestamp: event.Timestamp, Previous: event.Previous, Subject: event.Subject, Body: event.Body}
	if !reflect.DeepEqual(event, envelope) {
		return fmt.Errorf("run withdrawal contains extraneous fields")
	}
	return nil
}

// validateRunWithdrawalRelationship is shared by local writes/collection and
// import validation. Callers classify an absent request before calling here
// when their boundary supports selected or shallow dependency recovery.
func validateRunWithdrawalRelationship(withdrawal StoredEvent, byID map[string]StoredEvent) error {
	event := withdrawal.Event
	request, exists := byID[event.Subject]
	if !exists || request.Event.Kind != "run.request" {
		return fmt.Errorf("run withdrawal %s requires an available run request %s", shortID(withdrawal.ID), event.Subject)
	}
	if request.Event.Actor != event.Actor || request.Event.PublicKey != event.PublicKey {
		return fmt.Errorf("run withdrawal must be signed by the original requester actor and public key")
	}
	if request.Event.Sequence >= event.Sequence {
		return fmt.Errorf("run withdrawal must follow its original request sequence")
	}
	for _, other := range byID {
		if other.Event.Kind == "run.withdraw" && other.Event.Subject == event.Subject && other.ID != withdrawal.ID {
			return fmt.Errorf("run request %s already has a withdrawal; duplicate or conflicting withdrawal facts are invalid", shortID(event.Subject))
		}
	}
	return nil
}

// findRunWithdrawal projects verified facts only. It controls future starts;
// historical results and governance evidence never consult this projection.
func findRunWithdrawal(events []StoredEvent, requestID string) *StoredEvent {
	for i := range events {
		if events[i].Event.Kind == "run.withdraw" && events[i].Event.Subject == requestID {
			return &events[i]
		}
	}
	return nil
}
