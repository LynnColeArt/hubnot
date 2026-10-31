package extension

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MessageInspection describes only supplied transport values. These fields never
// attest a live session, authorization, signatures, or semantic acceptance.
type MessageInspection struct {
	FrameBytes         int     `json:"frame_bytes"`
	PayloadBytes       int     `json:"payload_bytes"`
	MessageKind        string  `json:"message_kind"`
	RequestID          *string `json:"request_id"`
	Method             *string `json:"method"`
	Validation         string  `json:"validation"`
	SessionState       string  `json:"session_state"`
	Authorization      string  `json:"authorization"`
	SemanticValidation string  `json:"semantic_validation"`
	SignatureIntegrity string  `json:"signature_integrity"`
	Dispatch           string  `json:"dispatch"`
}

// InspectMessage validates one closed typed message, with the correlation flags
// from host-v1 section 7.1. It has no repository/module selector or dispatch path.
func InspectMessage(payload []byte, kind, expected, method string) (MessageInspection, error) {
	result := MessageInspection{FrameBytes: len(payload) + 4, PayloadBytes: len(payload), MessageKind: kind, Validation: "frame_and_typed_message", SessionState: "not_checked", Authorization: "not_checked", SemanticValidation: "not_checked", SignatureIntegrity: "not_checked", Dispatch: "not_performed"}
	value, err := strictJSON(payload, FrameLimit)
	if err != nil {
		return result, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return result, invalidShape()
	}
	if _, ok := object["protocol"].(string); !ok {
		return result, invalidShape()
	}
	if object["protocol"] != HostProtocol {
		return result, failure("incompatible_version", "unsupported host protocol")
	}
	if err := inventoryRule("message_kinds")(kind); err != nil {
		return result, err
	}
	var shape wireRule
	switch kind {
	case "hello":
		if expected != "" || method != "" {
			return result, invalidShape()
		}
		shape = closed(map[string]wireRule{"protocol": literal(HostProtocol), "hello": sessionFields(false), "limits": closed(map[string]wireRule{"frame_bytes": literalNumber(FrameLimit), "read_timeout_ms": literalNumber(30000), "mutation_timeout_ms": literalNumber(240000), "page_max": literalNumber(200), "one_outstanding": literal(true)}, nil)}, nil)
	case "ready":
		if expected != "" || method != "" {
			return result, invalidShape()
		}
		shape = closed(map[string]wireRule{"protocol": literal(HostProtocol), "ready": literal(true)}, nil)
	case "cancel":
		if expected == "" || method != "" {
			return result, invalidShape()
		}
		shape = closed(map[string]wireRule{"protocol": literal(HostProtocol), "cancel": requestIDRule}, nil)
		id, _ := object["cancel"].(string)
		if id != expected {
			return result, invalidShape()
		}
		result.RequestID = &id
	case "request":
		if expected != "" || method != "" {
			return result, invalidShape()
		}
		selected, ok := object["method"].(string)
		if !ok {
			return result, invalidShape()
		}
		if !methods[selected] {
			return result, failure("unsupported_capability", "unsupported host method")
		}
		shape = closed(map[string]wireRule{"protocol": literal(HostProtocol), "request_id": requestIDRule, "method": literal(selected), "params": paramsRule(selected)}, map[string]wireRule{"timeout_ms": integer(1, 240000, false)})
		id, _ := object["request_id"].(string)
		result.RequestID = &id
		result.Method = &selected
	case "response":
		idValue, exists := object["request_id"]
		if !exists {
			return result, invalidShape()
		}
		success, ok := object["ok"].(bool)
		if !ok {
			return result, invalidShape()
		}
		if idValue == nil {
			if expected != "" || method != "" || success {
				return result, invalidShape()
			}
		} else {
			id, ok := idValue.(string)
			if !ok || expected == "" || id != expected {
				return result, invalidShape()
			}
			result.RequestID = &id
		}
		dataRule := nullRule
		errorRule := errorShape()
		if success {
			if method == "" {
				return result, invalidShape()
			}
			if !methods[method] {
				return result, failure("unsupported_capability", "unsupported host method")
			}
			dataRule = resultRule(method)
			errorRule = nullRule
			result.Method = &method
		} else if method != "" {
			return result, invalidShape()
		}
		shape = closed(map[string]wireRule{"protocol": literal(HostProtocol), "request_id": nullable(requestIDRule), "ok": booleanRule, "data": dataRule, "error": errorRule}, nil)
	default:
		return result, invalidShape()
	}
	if err := shape(value); err != nil {
		return result, err
	}
	return result, nil
}

// protocolInventories are the private closed wire vocabularies frozen by the
// host/artifact contracts. Every typed enum below consumes these same values;
// protocol_test.go mirrors them against independent literal contract copies.
var protocolInventories = map[string][]string{
	"families":                {"issue", "project", "project-item", "discussion", "discussion-reply", "workflow"},
	"shapes":                  {"document", "list", "ticket"},
	"operation_protocols":     {"hn/0", "hn-artifact/1"},
	"recoveries":              {"do_not_retry", "retry_identical", "inspect_operation", "rescan_required"},
	"artifact_read_modes":     {"view", "heads", "history", "inspect"},
	"artifact_mutation_kinds": {"record.open", "record.revise", "record.resolve", "record.convert"},
	"diagnostic_codes":        {"incomplete_dependency", "unsupported_schema", "target_conflict", "missing_optional", "invalid_history", "opaque_unverified"},
	"native_admission":        {"admitted", "pending", "hidden"},
	"operation_statuses":      {"found", "verified_absent", "uncertain"},
	"operation_reasons":       {"incomplete_selection", "unavailable_history", "private_intent_pending", "visibility_restricted", "publication_unconfirmed"},
	"availability":            {"observed", "not_probed", "unavailable"},
	"signed_validation":       {"verified", "unsupported", "incomplete"},
	"artifact_envelope_kinds": {"type.publish", "record.open", "record.revise", "record.resolve", "record.convert"},
	"target_modes":            {"logical", "revision"},
	"schema_node_types":       {"object", "array", "string", "integer", "boolean", "null"},
	"native_statuses":         {"open", "closed"},
	"native_relation_kinds":   {"blocks", "related", "parent"},
	"project_value_types":     {"text", "number", "date", "single_select"},
	"project_layouts":         {"table", "board"},
	"project_target_kinds":    {"issue", "proposal", "discussion", "draft", "external"},
	"message_kinds":           {"hello", "ready", "request", "response", "cancel"},
	"native_intents":          {"open", "revise", "resolve"},
	"issue_intents":           {"open", "revise", "resolve", "comment"},
	"error_origins":           {"host", "native", "artifact"},
	"host_error_codes":        {"denied_access", "incompatible_version", "unavailable_module", "unsupported_capability", "resource_limit", "cancelled", "uncertain_operation", "checkpoint_invalidated", "verification_failure", "invalid_input", "not_found", "repository_error"},
	"native_error_codes":      {"actor_mismatch", "admission_pending", "admission_refused", "ambiguous_id", "concurrent_update", "invalid_cursor", "invalid_history", "invalid_input", "issue_conflict", "not_found", "operation_conflict", "permission_denied", "repository_error", "resource_conflict", "resource_limit", "stale_cursor", "stale_revision"},
	"artifact_error_codes":    {"invalid_input", "resource_limit", "verification_failure", "invalid_history", "incomplete_dependency", "unsupported_schema", "actor_stream_conflict", "operation_conflict", "stale_revision", "resource_conflict", "concurrent_update", "not_found", "repository_error", "cancelled", "uncertain_operation"},
	"unsupported":             {"memory.mutate", "proposal", "review", "decide", "merge", "policy.mutate", "identity.mutate", "run", "runner", "audit", "automerge", "sync.apply", "migration.apply"},
}

func inventoryRule(name string) wireRule { return enumeration(protocolInventories[name]...) }

type wireRule func(any) error

func invalidShape() error { return failure("invalid_input", "invalid typed protocol message") }
func predicate(test func(any) bool) wireRule {
	return func(v any) error {
		if !test(v) {
			return invalidShape()
		}
		return nil
	}
}

var nullRule = predicate(func(v any) bool { return v == nil })
var booleanRule = predicate(func(v any) bool { _, ok := v.(bool); return ok })

func literal(want any) wireRule { return predicate(func(v any) bool { return v == want }) }
func literalNumber(n int64) wireRule {
	return predicate(func(v any) bool { return v == json.Number(strconv.FormatInt(n, 10)) })
}
func nullable(rule wireRule) wireRule {
	return func(v any) error {
		if v == nil {
			return nil
		}
		return rule(v)
	}
}
func text(min, max int) wireRule {
	return func(v any) error {
		s, ok := v.(string)
		if !ok || len(s) < min {
			return invalidShape()
		}
		if len(s) > max {
			return failure("resource_limit", "typed text exceeds byte limit")
		}
		return nil
	}
}
func enumeration(values ...string) wireRule {
	return predicate(func(v any) bool {
		s, ok := v.(string)
		if !ok {
			return false
		}
		for _, x := range values {
			if s == x {
				return true
			}
		}
		return false
	})
}
func integer(min, max int64, limit bool) wireRule {
	return func(v any) error {
		n, ok := v.(json.Number)
		if !ok {
			return invalidShape()
		}
		i, err := strconv.ParseInt(string(n), 10, 64)
		if err != nil || i < min {
			return invalidShape()
		}
		if i > max {
			if limit {
				return failure("resource_limit", "typed count exceeds limit")
			}
			return invalidShape()
		}
		return nil
	}
}
func closed(required, optional map[string]wireRule) wireRule {
	return func(v any) error {
		o, ok := v.(map[string]any)
		if !ok {
			return invalidShape()
		}
		for k, rule := range required {
			x, ok := o[k]
			if !ok {
				return invalidShape()
			}
			if err := rule(x); err != nil {
				return err
			}
		}
		for k, x := range o {
			if _, ok := required[k]; ok {
				continue
			}
			rule, ok := optional[k]
			if !ok {
				return invalidShape()
			}
			if err := rule(x); err != nil {
				return err
			}
		}
		return nil
	}
}
func sequence(rule wireRule, min, max int, distinct, sorted bool) wireRule {
	return func(v any) error {
		a, ok := v.([]any)
		if !ok || len(a) < min {
			return invalidShape()
		}
		if len(a) > max {
			return failure("resource_limit", "typed array exceeds count limit")
		}
		seen := map[string]bool{}
		previous := ""
		for _, x := range a {
			if err := rule(x); err != nil {
				return err
			}
			key := string(canonical(x))
			if distinct && seen[key] {
				return invalidShape()
			}
			if sorted && previous != "" && previous >= key {
				return invalidShape()
			}
			seen[key] = true
			previous = key
		}
		return nil
	}
}
func dictionary(rule wireRule, max int, keyRule wireRule) wireRule {
	return func(v any) error {
		o, ok := v.(map[string]any)
		if !ok {
			return invalidShape()
		}
		if len(o) > max {
			return failure("resource_limit", "typed object exceeds field limit")
		}
		for k, x := range o {
			if err := keyRule(k); err != nil {
				return err
			}
			if err := rule(x); err != nil {
				return err
			}
		}
		return nil
	}
}
func matches(pattern string) wireRule {
	r := regexp.MustCompile(pattern)
	return predicate(func(v any) bool { s, ok := v.(string); return ok && r.MatchString(s) })
}

var idRule = matches(`^sha256:[0-9a-f]{64}$`)
var actorRule = matches(`^[0-9a-f]{64}$`)
var sessionIDRule = matches(`^[0-9a-f]{32}$`)
var requestIDRule = matches(`^[!-~]{1,128}$`)
var moduleVersionRule = matches(`^[!-~]{1,64}$`)
var typeIDRule = predicate(func(v any) bool { s, ok := v.(string); return ok && ValidModuleID(s) })
var familyRule = inventoryRule("families")
var shapeRule = inventoryRule("shapes")
var protocolRule = inventoryRule("operation_protocols")
var recoveryRule = inventoryRule("recoveries")
var tokenRule = text(0, 4096)
var decimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]*[1-9])?$`)
var decimalRule = predicate(func(v any) bool {
	s, ok := v.(string)
	return ok && len(s) <= 64 && s != "-0" && decimalPattern.MatchString(s)
})
var dateRule = predicate(func(v any) bool {
	s, ok := v.(string)
	if !ok || len(s) != 10 {
		return false
	}
	d, err := time.Parse("2006-01-02", s)
	return err == nil && d.Year() > 0 && d.Format("2006-01-02") == s
})
var urlRule = predicate(func(v any) bool {
	s, ok := v.(string)
	if !ok || len(s) > 4096 {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
})
var pageLimit = integer(1, 200, true)
var idsRule = sequence(idRule, 0, 1024, true, true)
var nativeIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
var nativeIdentifier = predicate(func(v any) bool {
	s, ok := v.(string)
	return ok && s != "." && s != ".." && nativeIdentifierPattern.MatchString(s)
})
var fieldNameRule = matches(`^[A-Za-z_][A-Za-z0-9_.-]{0,63}$`)
var timestampRule = predicate(func(v any) bool {
	s, ok := v.(string)
	if !ok || len(s) != 20 {
		return false
	}
	t, e := time.Parse("2006-01-02T15:04:05Z", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02T15:04:05Z") == s
})

func sessionFields(protocol bool) wireRule {
	fields := map[string]wireRule{"session_id": sessionIDRule, "repository_id": idRule, "module_id": typeIDRule, "module_version": moduleVersionRule, "registration_id": idRule}
	if protocol {
		fields["protocol"] = literal(HostProtocol)
	}
	return closed(fields, nil)
}
func selectionRule() wireRule {
	return func(v any) error {
		rule := closed(map[string]wireRule{"families": sequence(familyRule, 0, 6, true, true), "types": sequence(typeIDRule, 0, FrameLimit, true, true)}, nil)
		if err := rule(v); err != nil {
			return err
		}
		o := v.(map[string]any)
		if len(o["families"].([]any))+len(o["types"].([]any)) == 0 {
			return invalidShape()
		}
		return nil
	}
}
func paramsRule(method string) wireRule {
	fields := map[string]wireRule{}
	switch method {
	case "capabilities.list", "identity.public":
	case "resource.read":
		fields = map[string]wireRule{"family": familyRule, "id": nullable(idRule), "limit": pageLimit, "cursor": nullable(tokenRule)}
	case "resource.mutate":
		return nativeMutationRule()
	case "resource.operation":
		fields = map[string]wireRule{"family": familyRule, "actor": actorRule, "operation": requestIDRule}
	case "artifact.read":
		fields = map[string]wireRule{"id": nullable(idRule), "opaque_id": nullable(idRule), "type_id": nullable(typeIDRule), "limit": pageLimit, "cursor": nullable(tokenRule), "mode": inventoryRule("artifact_read_modes"), "opaque": booleanRule}
	case "artifact.mutate":
		fields = map[string]wireRule{"kind": inventoryRule("artifact_mutation_kinds"), "record": nullable(idRule), "parents": idsRule, "payload": recordPayloadRule(), "actor": actorRule, "operation": requestIDRule}
	case "artifact.operation":
		fields = map[string]wireRule{"type_id": typeIDRule, "actor": actorRule, "operation": requestIDRule}
	case "artifact.search":
		fields = map[string]wireRule{"type_id": nullable(typeIDRule), "shape": nullable(shapeRule), "literal": nullable(text(0, FrameLimit)), "regex": nullable(text(0, 4096)), "limit": pageLimit, "cursor": nullable(tokenRule)}
	case "artifact.type.read":
		fields = map[string]wireRule{"id": nullable(idRule), "type_id": nullable(typeIDRule), "limit": pageLimit, "cursor": nullable(tokenRule)}
	case "artifact.type.publish":
		fields = map[string]wireRule{"parents": idsRule, "payload": typePayloadRule(), "actor": actorRule, "operation": requestIDRule}
	case "changes.capture":
		fields = map[string]wireRule{"selection": selectionRule(), "limit": pageLimit}
	case "changes.page":
		fields = map[string]wireRule{"capture": tokenRule, "cursor": tokenRule, "limit": pageLimit}
	case "changes.since":
		fields = map[string]wireRule{"checkpoint": tokenRule, "selection": selectionRule(), "limit": pageLimit}
	}
	return func(v any) error {
		if err := closed(fields, nil)(v); err != nil {
			return err
		}
		o := v.(map[string]any)
		switch method {
		case "artifact.read":
			if o["opaque"].(bool) {
				if o["mode"] != "inspect" || o["id"] != nil || o["type_id"] != nil || o["opaque_id"] == nil {
					return invalidShape()
				}
			} else {
				if o["opaque_id"] != nil || (o["id"] == nil && o["type_id"] == nil) || (o["mode"] == "history" && o["id"] == nil) {
					return invalidShape()
				}
			}
		case "artifact.search":
			if (o["literal"] == nil) == (o["regex"] == nil) {
				return invalidShape()
			}
		case "artifact.type.read":
			if o["id"] == nil && o["type_id"] == nil {
				return invalidShape()
			}
		}
		return nil
	}
}
func nativeMutationRule() wireRule {
	return func(v any) error {
		o, ok := v.(map[string]any)
		if !ok {
			return invalidShape()
		}
		family, ok := o["family"].(string)
		if !ok {
			return invalidShape()
		}
		intents := protocolInventories["native_intents"]
		if family == "issue" {
			intents = protocolInventories["issue_intents"]
		}
		if err := closed(map[string]wireRule{"family": familyRule, "intent": enumeration(intents...), "id": nullable(idRule), "expected": sequence(idRule, 0, 200, true, true), "actor": actorRule, "operation": requestIDRule, "state": nullable(nativeStateRule(family)), "comment": text(0, 65536)}, nil)(v); err != nil {
			return err
		}
		if o["intent"] == "open" {
			if o["id"] != nil || len(o["expected"].([]any)) != 0 {
				return invalidShape()
			}
		} else if o["id"] == nil {
			return invalidShape()
		}
		if o["intent"] == "comment" {
			if o["state"] != nil || o["comment"] == "" || len(o["expected"].([]any)) != 0 {
				return invalidShape()
			}
		} else if o["state"] == nil || o["comment"] != "" {
			return invalidShape()
		}
		return nil
	}
}
func errorShape() wireRule {
	return func(v any) error {
		o, ok := v.(map[string]any)
		if !ok {
			return invalidShape()
		}
		origin, ok := o["origin"].(string)
		if !ok {
			return invalidShape()
		}
		if err := inventoryRule("error_origins")(origin); err != nil {
			return err
		}
		codes := protocolInventories[origin+"_error_codes"]

		return closed(map[string]wireRule{"origin": literal(origin), "code": enumeration(codes...), "message": text(0, 4096), "recovery": recoveryRule, "operation": nullable(closed(map[string]wireRule{"protocol": protocolRule, "actor": actorRule, "operation": requestIDRule}, nil))}, nil)(v)
	}
}
func diagnosticRule() wireRule {
	return closed(map[string]wireRule{"code": inventoryRule("diagnostic_codes"), "id": nullable(idRule), "message": text(0, 4096)}, nil)
}
func resultRule(method string) wireRule {
	switch method {
	case "capabilities.list":
		return capabilitiesRule()
	case "identity.public":
		return closed(map[string]wireRule{"initialized": booleanRule, "actor": nullable(actorRule), "name": nullable(text(0, FrameLimit)), "public_key": nullable(base64Rule(32))}, nil)
	case "resource.read":
		return page(nativeViewRule())
	case "resource.mutate", "artifact.mutate", "artifact.type.publish":
		return mutationResultRule()
	case "resource.operation", "artifact.operation":
		return operationResultRule()
	case "artifact.read":
		return page(either(recordViewRule(), signedArtifactRule()))
	case "artifact.search":
		return page(recordViewRule())
	case "artifact.type.read":
		return page(signedArtifactRule())
	default:
		return changePageRule()
	}
}
func either(a, b wireRule) wireRule {
	return func(v any) error {
		if err := a(v); err == nil {
			return nil
		}
		return b(v)
	}
}
func page(item wireRule) wireRule {
	return closed(map[string]wireRule{"items": sequence(item, 0, 200, false, false), "source_cut": idRule, "next_cursor": nullable(tokenRule)}, nil)
}
func mutationResultRule() wireRule {
	return func(v any) error {
		if err := closed(map[string]wireRule{"protocol": protocolRule, "family": nullable(familyRule), "type_id": nullable(typeIDRule), "resource_id": idRule, "event_id": idRule, "replayed": booleanRule, "conflict": booleanRule, "head_count": integer(0, 9007199254740991, false), "admission": nullable(inventoryRule("native_admission")), "source_cut": idRule}, nil)(v); err != nil {
			return err
		}
		o := v.(map[string]any)
		if o["protocol"] == "hn/0" {
			if o["family"] == nil || o["type_id"] != nil {
				return invalidShape()
			}
		} else {
			if o["family"] != nil || o["type_id"] == nil || o["admission"] != nil {
				return invalidShape()
			}
		}
		return nil
	}
}
func operationResultRule() wireRule {
	return func(v any) error {
		if err := closed(map[string]wireRule{"status": inventoryRule("operation_statuses"), "protocol": protocolRule, "actor": actorRule, "operation": requestIDRule, "family": nullable(familyRule), "type_id": nullable(typeIDRule), "request": nullable(idRule), "resource_id": nullable(idRule), "event_id": nullable(idRule), "result": nullable(mutationResultRule()), "source_cut": idRule, "reason": nullable(inventoryRule("operation_reasons"))}, nil)(v); err != nil {
			return err
		}
		o := v.(map[string]any)
		switch o["status"] {
		case "found":
			for _, key := range []string{"request", "resource_id", "event_id", "result"} {
				if o[key] == nil {
					return invalidShape()
				}
			}
			if o["reason"] != nil {
				return invalidShape()
			}
		case "verified_absent":
			for _, key := range []string{"request", "resource_id", "event_id", "result", "reason"} {
				if o[key] != nil {
					return invalidShape()
				}
			}
		case "uncertain":
			if o["result"] != nil || o["reason"] == nil {
				return invalidShape()
			}
		}
		return nil
	}
}
func capabilitiesRule() wireRule {
	return func(v any) error {
		capability := closed(map[string]wireRule{"method": enumeration(methodNames()...), "declared": booleanRule, "granted": booleanRule, "enabled": booleanRule, "compatible": booleanRule, "availability": inventoryRule("availability"), "recovery": recoveryRule}, nil)
		unsupported := closed(map[string]wireRule{"surface": enumeration(unsupportedNames()...), "status": literal("unsupported"), "recovery": literal("do_not_retry")}, nil)
		err := closed(map[string]wireRule{"session": sessionFields(true), "capabilities": sequence(capability, 14, 14, false, false), "unsupported": sequence(unsupported, 13, 13, false, false)}, nil)(v)
		if err != nil {
			return err
		}
		o := v.(map[string]any)
		for key, field := range map[string]string{"capabilities": "method", "unsupported": "surface"} {
			seen := map[string]bool{}
			for _, entry := range o[key].([]any) {
				name := entry.(map[string]any)[field].(string)
				if seen[name] {
					return invalidShape()
				}
				seen[name] = true
			}
		}
		return nil
	}
}
func methodNames() []string {
	names := make([]string, 0, len(methods))
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func unsupportedNames() []string {
	return protocolInventories["unsupported"]
}
func changePageRule() wireRule {
	return closed(map[string]wireRule{"capture": tokenRule, "snapshot_id": idRule, "selection": selectionRule(), "selection_digest": idRule, "source_cut": idRule, "mode": literal("replace"), "items": sequence(closed(map[string]wireRule{"protocol": protocolRule, "family": nullable(familyRule), "type_id": nullable(typeIDRule), "resource_id": idRule, "revision_ids": idsRule, "value": either(nativeViewRule(), recordViewRule())}, nil), 0, 200, false, false), "next_cursor": nullable(tokenRule), "complete": booleanRule, "checkpoint": nullable(tokenRule), "expires_at": timestampRule}, nil)
}
func nativeViewRule() wireRule {
	return func(v any) error {
		o, ok := v.(map[string]any)
		if !ok {
			return invalidShape()
		}
		family, ok := o["family"].(string)
		if !ok {
			return invalidShape()
		}
		state := nativeStateRule(family)
		return closed(map[string]wireRule{"protocol": literal("hn/0"), "family": familyRule, "id": idRule, "creator": actorRule, "heads": sequence(closed(map[string]wireRule{"id": idRule, "actor": actorRule, "state": state}, nil), 0, FrameLimit, false, false), "conflict": booleanRule, "state": nullable(state), "comments": sequence(closed(map[string]wireRule{"id": idRule, "actor": actorRule, "body": text(0, 65536)}, nil), 0, FrameLimit, false, false)}, nil)(v)
	}
}
func recordViewRule() wireRule {
	return closed(map[string]wireRule{"protocol": literal("hn-artifact/1"), "id": idRule, "type_id": typeIDRule, "shape": shapeRule, "heads": sequence(closed(map[string]wireRule{"id": idRule, "actor": actorRule, "type_revision": idRule, "payload": recordPayloadRule()}, nil), 0, FrameLimit, false, false), "conflict": booleanRule, "state": nullable(recordPayloadRule()), "diagnostics": sequence(diagnosticRule(), 0, FrameLimit, false, false)}, nil)
}
func base64Rule(size int) wireRule {
	return predicate(func(v any) bool {
		s, ok := v.(string)
		if !ok {
			return false
		}
		b, e := base64.RawStdEncoding.Strict().DecodeString(s)
		return e == nil && (size < 0 || len(b) == size) && base64.RawStdEncoding.EncodeToString(b) == s
	})
}
func signedArtifactRule() wireRule {
	return func(v any) error {
		o, ok := v.(map[string]any)
		if !ok {
			return invalidShape()
		}
		diagnostics := sequence(diagnosticRule(), 0, FrameLimit, false, false)
		if o["validation"] == "opaque" {
			return closed(map[string]wireRule{"id": nullRule, "opaque_id": idRule, "envelope": nullRule, "signature": nullRule, "validation": literal("opaque"), "raw": closed(map[string]wireRule{"oid": text(1, 128), "type": text(1, 128), "data": base64Rule(-1)}, nil), "diagnostics": diagnostics}, nil)(v)
		}
		return closed(map[string]wireRule{"id": idRule, "envelope": artifactEnvelopeRule(), "signature": base64Rule(64), "validation": inventoryRule("signed_validation"), "diagnostics": diagnostics}, nil)(v)
	}
}
func artifactEnvelopeRule() wireRule {
	return func(v any) error {
		o, ok := v.(map[string]any)
		if !ok {
			return invalidShape()
		}
		payload := recordPayloadRule()
		if o["kind"] == "type.publish" {
			payload = typePayloadRule()
		}
		emptyID := either(literal(""), idRule)
		if err := closed(map[string]wireRule{"protocol": literal("hn-artifact/1"), "kind": inventoryRule("artifact_envelope_kinds"), "actor": actorRule, "public_key": base64Rule(32), "stream": idRule, "sequence": integer(1, 9007199254740991, false), "previous": emptyID, "timestamp": timestampRule, "operation": requestIDRule, "request": idRule, "record": emptyID, "parents": idsRule, "payload": payload}, nil)(v); err != nil {
			return err
		}
		if len(canonical(v)) > 1048576 {
			return failure("resource_limit", "artifact envelope exceeds byte limit")
		}
		return nil
	}
}
func recordPayloadRule() wireRule {
	target := map[string]wireRule{"target": idRule, "target_mode": inventoryRule("target_modes"), "required": booleanRule}
	member := map[string]wireRule{"member_id": matches(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)}
	relation := map[string]wireRule{"relation": typeIDRule}
	for k, r := range target {
		member[k] = r
		relation[k] = r
	}
	return func(v any) error {
		if err := closed(map[string]wireRule{"shape": shapeRule, "type_id": typeIDRule, "type_revision": idRule, "title": predicate(func(v any) bool {
			s, ok := v.(string)
			return ok && len(s) > 0 && len(s) <= 1024 && strings.TrimSpace(s) != ""
		}), "content": text(0, FrameLimit), "fields": fieldsValueRule(1), "members": sequence(closed(member, nil), 0, 1024, false, false), "relationships": sequence(closed(relation, nil), 0, 256, true, false), "conversion_sources": idsRule}, nil)(v); err != nil {
			return err
		}
		o := v.(map[string]any)
		members := o["members"].([]any)
		if o["shape"] != "list" && len(members) > 0 {
			return invalidShape()
		}
		seen := map[string]bool{}
		for _, value := range members {
			name := value.(map[string]any)["member_id"].(string)
			if seen[name] {
				return invalidShape()
			}
			seen[name] = true
		}
		return nil
	}
}
func fieldsValueRule(depth int) wireRule {
	return func(v any) error {
		o, ok := v.(map[string]any)
		if !ok {
			return invalidShape()
		}
		return dictionary(fieldValueRule(depth), 64, fieldNameRule)(o)
	}
}
func fieldValueRule(depth int) wireRule {
	return func(v any) error {
		switch x := v.(type) {
		case map[string]any:
			if depth >= 8 {
				return failure("resource_limit", "modeled value depth exceeds limit")
			}
			return fieldsValueRule(depth + 1)(x)
		case []any:
			if depth >= 8 {
				return failure("resource_limit", "modeled value depth exceeds limit")
			}
			return sequence(fieldValueRule(depth+1), 0, 1024, false, false)(x)
		default:
			return nil
		}
	}
}
func typePayloadRule() wireRule {
	return boundedObject(closed(map[string]wireRule{"type_id": typeIDRule, "schema_version": integer(1, 2147483647, false), "shape": shapeRule, "schema": boundedObject(closed(map[string]wireRule{"language": text(1, 64), "root": schemaRootRule()}, nil), 65536)}, nil), 131072)
}

// This is the frozen schema's syntactic shape only. It does not evaluate fields,
// resolve declarations or accept a schema language as supported semantics.
func schemaNodeRule(depth int) wireRule {
	return func(v any) error {
		o, ok := v.(map[string]any)
		if !ok {
			return invalidShape()
		}
		required := map[string]wireRule{"type": inventoryRule("schema_node_types")}
		optional := map[string]wireRule{}
		switch o["type"] {
		case "object":
			if depth > 8 {
				return failure("resource_limit", "modeled schema depth exceeds limit")
			}
			required["properties"] = dictionary(schemaNodeRule(depth+1), 64, fieldNameRule)
			required["required"] = sequence(fieldNameRule, 0, 1024, true, false)
			required["additionalProperties"] = literal(false)
		case "array":
			if depth > 8 {
				return failure("resource_limit", "modeled schema depth exceeds limit")
			}
			required["items"] = schemaNodeRule(depth + 1)
			required["maxItems"] = integer(0, 1024, true)
		case "string":
			required["maxBytes"] = integer(0, 1048576, true)
			optional["enum"] = sequence(text(0, 1048576), 1, 1024, true, false)
		case "integer":
			optional["minimum"] = integer(-9007199254740991, 9007199254740991, false)
			optional["maximum"] = optional["minimum"]
			optional["enum"] = sequence(optional["minimum"], 1, 1024, true, false)
		case "boolean":
			optional["enum"] = sequence(booleanRule, 1, 1024, true, false)
		}
		return closed(required, optional)(v)
	}
}
func nativeStateRule(family string) wireRule {
	metadata := dictionary(text(0, 4096), 64, text(1, 128))
	required := map[string]wireRule{}
	optional := map[string]wireRule{"metadata": metadata}
	maxBytes := 256 * 1024
	switch family {
	case "issue":
		required = map[string]wireRule{"title": text(1, 1024), "body": text(0, 65536), "status": inventoryRule("native_statuses"), "criteria": sequence(closed(map[string]wireRule{"id": text(1, 64), "text": text(0, 4096)}, nil), 0, 64, false, false), "labels": sequence(text(0, 256), 0, 64, true, false), "assignees": sequence(text(0, 256), 0, 64, true, false), "relations": sequence(closed(map[string]wireRule{"kind": inventoryRule("native_relation_kinds"), "target": idRule}, nil), 0, 128, false, false), "metadata": metadata}
		optional = nil
	case "project":
		required = map[string]wireRule{"schema": literal("hn.project/1"), "title": text(1, 1024), "body": text(0, 65536), "archived": booleanRule}
		optional["fields"] = sequence(closed(map[string]wireRule{"id": nativeIdentifier, "name": text(1, 256), "type": inventoryRule("project_value_types")}, map[string]wireRule{"options": sequence(closed(map[string]wireRule{"id": nativeIdentifier, "name": text(1, 256)}, nil), 0, 64, false, false)}), 0, 64, false, false)
		optional["views"] = sequence(closed(map[string]wireRule{"id": nativeIdentifier, "name": text(1, 256), "layout": inventoryRule("project_layouts")}, map[string]wireRule{"group_by_field": nativeIdentifier}), 0, 16, false, false)
	case "project-item":
		required = map[string]wireRule{"schema": literal("hn.project-item/1"), "project_id": idRule, "project_revision": idRule, "target": projectTargetRule(), "archived": booleanRule, "position": either(literal(""), decimalRule)}
		optional["values"] = dictionary(projectValueRule(), 64, nativeIdentifier)
	case "discussion":
		required = map[string]wireRule{"schema": literal("hn.discussion/1"), "title": text(1, 1024), "body": text(0, 65536), "category": text(1, 256), "status": inventoryRule("native_statuses")}
		optional["accepted_reply_revision"] = idRule
	case "discussion-reply":
		required = map[string]wireRule{"schema": literal("hn.discussion-reply/1"), "discussion_id": idRule, "body": text(1, 65536)}
		optional["reply_to"] = idRule
	case "workflow":
		maxBytes = 2 * 1024 * 1024
		required = map[string]wireRule{"schema": literal("hn.workflow/1"), "name": nativeIdentifier, "title": text(1, 1024), "description": text(0, 65536), "enabled": booleanRule}
		optional["pipeline"] = pipelineRule()
		optional["source_documents"] = sequence(closed(map[string]wireRule{"format": text(1, 128), "content": text(0, 1048576), "digest": idRule}, nil), 0, 8, false, false)
	default:
		return predicate(func(any) bool { return false })
	}
	return boundedObject(closed(required, optional), maxBytes)
}
func boundedObject(rule wireRule, max int) wireRule {
	return func(v any) error {
		if err := rule(v); err != nil {
			return err
		}
		if len(canonical(v)) > max {
			return failure("resource_limit", "typed object exceeds byte limit")
		}
		return nil
	}
}
func projectTargetRule() wireRule {
	return func(v any) error {
		o, ok := v.(map[string]any)
		if !ok {
			return invalidShape()
		}
		required := map[string]wireRule{"kind": inventoryRule("project_target_kinds")}
		switch o["kind"] {
		case "issue", "proposal", "discussion":
			required["id"] = idRule
		case "draft":
			required["title"] = text(1, 1024)
			required["body"] = text(0, 65536)
		case "external":
			required["url"] = urlRule
		}
		return closed(required, nil)(v)
	}
}
func projectValueRule() wireRule {
	return func(v any) error {
		o, ok := v.(map[string]any)
		if !ok {
			return invalidShape()
		}
		required := map[string]wireRule{"type": inventoryRule("project_value_types")}
		switch o["type"] {
		case "text":
			required["text"] = text(0, 4096)
		case "number":
			required["number"] = decimalRule
		case "date":
			required["date"] = dateRule
		case "single_select":
			required["option_id"] = nativeIdentifier
		}
		return closed(required, nil)(v)
	}
}
func pipelineRule() wireRule {
	return boundedObject(closed(map[string]wireRule{"version": literal("hn.pipeline/0"), "steps": sequence(closed(map[string]wireRule{"name": text(1, 256), "command": noNUL(text(1, FrameLimit))}, map[string]wireRule{"args": sequence(noNUL(text(0, 16384)), 0, 256, false, false), "workingDirectory": noNUL(text(0, FrameLimit)), "timeoutSeconds": integer(0, 3600, false)}), 1, 64, false, false)}, nil), 1048576)
}
func noNUL(rule wireRule) wireRule {
	return func(v any) error {
		if err := rule(v); err != nil {
			return err
		}
		if strings.ContainsRune(v.(string), 0) {
			return invalidShape()
		}
		return nil
	}
}

func schemaRootRule() wireRule {
	return func(v any) error {
		if err := schemaNodeRule(1)(v); err != nil {
			return err
		}
		if v.(map[string]any)["type"] != "object" {
			return invalidShape()
		}
		return nil
	}
}
