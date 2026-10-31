// fake-tracker is an optional reference consumer of hn.extension/1. It has no
// import of Hubnot internals and never selects a native signing key itself.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const protocol = "hn.extension/1"

type callRecord struct {
	Method   string         `json:"method"`
	Params   any            `json:"params"`
	Response map[string]any `json:"response"`
}
type report struct {
	Hello  map[string]any `json:"hello"`
	Calls  []callRecord   `json:"calls"`
	Status string         `json:"status"`
}
type client struct {
	c        config
	p        plan
	j        *journal
	report   report
	sequence int
	stopped  error
	input    io.Reader // nil selects the private host pipe
	output   io.Writer
	http     *http.Client
}

func main() {
	var err error
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		err = serve(os.Args[2:])
	} else {
		err = consume(os.Args[1:])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake tracker stopped:", safeError(err))
		os.Exit(1)
	}
}
func safeError(err error) string {
	// OS and HTTP errors can contain paths/URLs/credentials; detailed wire outcomes
	// belong only in the explicitly private report.
	for _, code := range []string{"external_uncertain", "external_conflict", "connector_binding_changed", "connector_busy", "checkpoint_invalidated", "stale_revision", "operation_conflict", "denied_access", "resource_limit", "invalid_input", "identity_changed", "native_uncertain", "issue_conflict"} {
		if err.Error() == code {
			return code
		}
	}
	return "connector_failure"
}
func textField(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func readFrame(r io.Reader) (map[string]any, error) {
	var prefix [4]byte
	if _, err := io.ReadFull(r, prefix[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(prefix[:])
	if n == 0 {
		return nil, errors.New("invalid_input")
	}
	if n > frameBytes {
		return nil, errors.New("resource_limit")
	}
	raw := make([]byte, int(n))
	if _, err := io.ReadFull(r, raw); err != nil {
		return nil, err
	}
	var v map[string]any
	if !validWireJSON(raw) || json.Unmarshal(raw, &v) != nil || v == nil {
		return nil, errors.New("invalid_input")
	}
	return v, nil
}
func writeFrame(w io.Writer, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(raw) == 0 || len(raw) > frameBytes {
		return errors.New("resource_limit")
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(raw)))
	if _, err = w.Write(prefix[:]); err != nil {
		return err
	}
	_, err = w.Write(raw)
	return err
}
func consume(args []string) error {
	flags := flag.NewFlagSet("fake-tracker", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	cfg := flags.String("config", "", "private connector configuration")
	script := flags.String("plan", "", "private explicit action")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *cfg == "" || *script == "" {
		return errors.New("invalid_input")
	}
	x := &client{http: &http.Client{Timeout: 10 * time.Second}}
	if err := readPrivate(*cfg, inputBytes, &x.c); err != nil {
		return err
	}
	if err := readPrivate(*script, inputBytes, &x.p); err != nil {
		return err
	}
	if x.c.Token == "" || x.c.Source == "" || x.c.Connection == "" || x.c.API == "" || x.c.Journal == "" || x.c.Report == "" {
		return errors.New("invalid_input")
	}
	hello, err := readFrame(os.Stdin)
	if err != nil {
		return err
	}
	binding, ok := hello["hello"].(map[string]any)
	if !ok || !validHello(hello) || hello["protocol"] != protocol || textField(binding, "module_id") != "example.tracker/tool" || !strings.HasPrefix(textField(binding, "repository_id"), "sha256:") || !strings.HasPrefix(textField(binding, "registration_id"), "sha256:") || textField(binding, "session_id") == "" {
		return errors.New("connector_binding_changed")
	}
	limits, ok := hello["limits"].(map[string]any)
	if !ok || limits["frame_bytes"] != float64(frameBytes) || limits["page_max"] != float64(200) || limits["one_outstanding"] != true {
		return errors.New("invalid_input")
	}
	x.report = report{Hello: hello, Calls: []callRecord{}}
	if err = writeFrame(os.Stdout, map[string]any{"protocol": protocol, "ready": true}); err != nil {
		return err
	}
	defer func() { _ = x.saveReport() }()
	if x.p.Action == "raw" {
		if x.p.Request == nil {
			return errors.New("invalid_input")
		}
		method := textField(x.p.Request, "method")
		_, err = x.call(method, x.p.Request["params"])
		return err
	}
	lock, err := lockJournal(x.c.Journal)
	if err != nil {
		return err
	}
	defer lock.Close()
	x.j, err = loadJournal(x.c, binding)
	if err != nil {
		return err
	}
	identity, err := x.ok("identity.public", map[string]any{})
	if err != nil {
		return err
	}
	if identity["initialized"] != true {
		return errors.New("identity_changed")
	}
	actor, key := textField(identity, "actor"), textField(identity, "public_key")
	if x.j.Actor != "" && (x.j.Actor != actor || x.j.PublicKey != key) {
		return errors.New("identity_changed")
	}
	x.j.Actor = actor
	x.j.PublicKey = key
	if err = x.save(); err != nil {
		return err
	}
	switch x.p.Action {
	case "import":
		err = x.importItem(false)
	case "update":
		err = x.importItem(true)
	case "observe":
		err = x.observe()
	default:
		err = errors.New("invalid_input")
	}
	if err != nil {
		x.report.Status = safeError(err)
	} else {
		x.report.Status = "complete"
	}
	return err
}
func (x *client) save() error       { return writePrivate(x.c.Journal, x.j, privateBytes) }
func (x *client) saveReport() error { return writePrivate(x.c.Report, x.report, privateBytes) }
func (x *client) pause(phase string) error {
	if x.p.Pause != phase {
		return nil
	}
	if x.c.Marker == "" {
		return errors.New("invalid_input")
	}
	if err := writePrivate(x.c.Marker, map[string]any{"phase": phase, "pid": os.Getpid()}, inputBytes); err != nil {
		return err
	}
	// A diagnostic pause is explicit and bounded by the host's normal deadline.
	for {
		if _, err := os.Stat(x.c.Marker + ".continue"); err == nil {
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
}
func (x *client) call(method string, params any) (_ map[string]any, callErr error) {
	if x.stopped != nil {
		return nil, x.stopped
	}
	// Any transport/shape/terminal error makes this session unusable. In
	// particular, a terminal null-ID refusal must never authorize another call.
	defer func() {
		if callErr != nil {
			x.stopped = callErr
		}
	}()
	input, output := x.input, x.output
	if input == nil {
		input = os.Stdin
	}
	if output == nil {
		output = os.Stdout
	}
	x.sequence++
	id := "tracker-" + strconv.Itoa(x.sequence)
	request := map[string]any{"protocol": protocol, "request_id": id, "method": method, "params": params}
	if err := writeFrame(output, request); err != nil {
		return nil, err
	}
	x.report.Calls = append(x.report.Calls, callRecord{Method: method, Params: params})
	index := len(x.report.Calls) - 1
	phase := ""
	if method == "resource.mutate" {
		p, _ := params.(map[string]any)
		intent := textField(p, "intent")
		if intent == "open" {
			phase = "issue"
		} else if intent == "comment" {
			phase = "comment"
		} else {
			phase = "update"
		}
	}
	if phase != "" {
		if err := x.pause("native-sent:" + phase); err != nil {
			return nil, err
		}
	}
	response, err := readFrame(input)
	if err != nil {
		return nil, err
	}
	x.report.Calls[index].Response = response
	if err = x.saveReport(); err != nil {
		return nil, err
	}
	if err = validateResponse(response, id, method, params, x.report.Hello); err != nil {
		return nil, err
	}
	return response, nil
}
func responseData(response map[string]any) (map[string]any, error) {
	if response["ok"] != true {
		e, _ := response["error"].(map[string]any)
		return nil, errors.New(textField(e, "code"))
	}
	data, ok := response["data"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid_input")
	}
	return data, nil
}
func (x *client) ok(method string, params any) (map[string]any, error) {
	r, err := x.call(method, params)
	if err != nil {
		return nil, err
	}
	return responseData(r)
}
func operationKey(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "tracker-" + hex.EncodeToString(sum[:])
}
func (x *client) intent(name string, params map[string]any) (map[string]any, error) {
	prior := x.j.Native[name]
	if prior == nil {
		prior = &nativeIntent{Params: params}
		x.j.Native[name] = prior
		if err := x.save(); err != nil {
			return nil, err
		}
	} else {
		a, _ := json.Marshal(prior.Params)
		b, _ := json.Marshal(params)
		if !bytes.Equal(a, b) {
			return nil, errors.New("operation_conflict")
		}
		if prior.Refusal != nil {
			return nil, errors.New(textField(prior.Refusal, "code"))
		}
		lookup, err := x.ok("resource.operation", map[string]any{"family": "issue", "actor": prior.Params["actor"], "operation": prior.Params["operation"]})
		if err != nil {
			return nil, err
		}
		prior.Lookup = lookup
		switch lookup["status"] {
		case "found":
		case "verified_absent":
			if prior.Result != nil {
				return nil, errors.New("native_uncertain")
			}
		case "uncertain":
			_ = x.save()
			return nil, errors.New("native_uncertain")
		default:
			return nil, errors.New("invalid_input")
		}
		if err = x.save(); err != nil {
			return nil, err
		}
	}
	// Exact replay is intentionally performed even for found: the host verifies the
	// full original request and returns original IDs without another publication.
	response, err := x.call("resource.mutate", prior.Params)
	if err != nil {
		return nil, err
	}
	result, err := responseData(response)
	if err != nil {
		prior.Refusal, _ = response["error"].(map[string]any)
		if e := x.save(); e != nil {
			return nil, e
		}
		return nil, err
	}
	if prior.Lookup != nil && prior.Lookup["status"] == "found" && (prior.Lookup["resource_id"] != result["resource_id"] || prior.Lookup["event_id"] != result["event_id"]) {
		return nil, errors.New("native_uncertain")
	}
	prior.Result = result
	if err = x.save(); err != nil {
		return nil, err
	}
	return result, nil
}

type externalComment struct {
	ID     string `json:"id"`
	Body   string `json:"body"`
	Author string `json:"author"`
	Time   string `json:"time"
	"unicode/utf8"`
}
type externalItem struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Author  string `json:"author"`
	Time    string `json:"time"
	"unicode/utf8"`
	Comments []externalComment `json:"comments"`
}

func (x *client) api(method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			return err
		}
		if len(raw) > frameBytes {
			return errors.New("resource_limit")
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequest(method, x.c.API+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+x.c.Token)
	request.Header.Set("Content-Type", "application/json")
	response, err := x.http.Do(request)
	if err != nil {
		return errors.New("external_uncertain")
	}
	defer response.Body.Close()
	raw, err := readBounded(response.Body, frameBytes)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusConflict {
		return errors.New("external_conflict")
	}
	if response.StatusCode == http.StatusNotFound {
		return errors.New("external_uncertain")
	}
	if response.StatusCode != http.StatusOK {
		return errors.New("external_uncertain")
	}
	if output != nil && json.Unmarshal(raw, output) != nil {
		return errors.New("external_uncertain")
	}
	return nil
}
func issueState(item externalItem, source string) map[string]any {
	return map[string]any{"title": item.Title, "body": item.Body, "status": "open", "criteria": []any{}, "labels": []any{}, "assignees": []any{}, "relations": []any{}, "metadata": map[string]any{"example.tracker/source": source, "example.tracker/author-claim": item.Author, "example.tracker/time-claim": item.Time}}
}
func mutation(actor, key, intent string, id any, parents []string, state any, comment string) map[string]any {
	return map[string]any{"family": "issue", "intent": intent, "id": id, "expected": parents, "actor": actor, "operation": key, "state": state, "comment": comment}
}
func (x *client) importItem(update bool) error {
	var item externalItem
	if !update && x.j.InitialItem != nil {
		// The first source snapshot, including its version and comment claims,
		// belongs to the original immutable import. Recovery needs no fresh API
		// response and never adopts a newer external state as its baseline.
		item = *x.j.InitialItem
	} else {
		if err := x.api("GET", "/item", nil, &item); err != nil {
			return err
		}
		if item.ID == "" || item.Version < 1 {
			return errors.New("invalid_input")
		}
		if !update {
			if len(item.Comments) != 1 {
				return errors.New("invalid_input")
			}
			x.j.InitialItem = &item
			if err := x.save(); err != nil {
				return err
			}
		}
	}
	if update {
		imported := x.j.Native["issue"]
		if imported == nil || imported.Result == nil {
			return errors.New("invalid_input")
		}
		name := "update-" + strconv.Itoa(item.Version)
		params := mutation(x.j.Actor, operationKey(x.c.Connection, x.c.Source, name), "revise", imported.Result["resource_id"], []string{x.j.NativeHead}, issueState(item, x.c.Source), "")
		if prior := x.j.Native[name]; prior != nil {
			params = prior.Params
		}
		result, err := x.intent(name, params)
		if err != nil {
			return err
		}
		x.j.NativeHead = textField(result, "event_id")
		x.j.ExternalVersion = item.Version
		id := textField(result, "resource_id")
		x.j.Baseline[id] = []string{textField(result, "event_id")}
		if comment := x.j.Native["comment"]; comment != nil && comment.Result != nil {
			x.j.Baseline[id] = append(x.j.Baseline[id], textField(comment.Result, "event_id"))
		}
		sort.Strings(x.j.Baseline[id])
		return x.save()
	}
	params := mutation(x.j.Actor, operationKey(x.c.Connection, x.c.Source, "issue"), "open", nil, []string{}, issueState(item, x.c.Source), "")
	if prior := x.j.Native["issue"]; prior != nil {
		params = prior.Params
	}
	result, err := x.intent("issue", params)
	if err != nil {
		return err
	}
	id := textField(result, "resource_id")
	if x.j.NativeHead == "" {
		x.j.NativeHead = textField(result, "event_id")
		x.j.ExternalVersion = item.Version
		x.j.Baseline[id] = []string{textField(result, "event_id")}
	}
	if len(item.Comments) != 1 {
		return errors.New("invalid_input")
	}
	comment := item.Comments[0]
	text := comment.Body + "\n[external author claim: " + comment.Author + "; time claim: " + comment.Time + "; source: " + x.c.Source + "/" + comment.ID + "]"
	cp := mutation(x.j.Actor, operationKey(x.c.Connection, x.c.Source, "comment"), "comment", id, []string{}, nil, text)
	if prior := x.j.Native["comment"]; prior != nil {
		cp = prior.Params
	}
	cr, err := x.intent("comment", cp)
	if err != nil {
		return err
	}
	if len(x.j.Baseline[id]) == 1 {
		x.j.Baseline[id] = append(x.j.Baseline[id], textField(cr, "event_id"))
		sort.Strings(x.j.Baseline[id])
	}
	return x.save()
}
func revisions(entry map[string]any) ([]string, error) {
	values, ok := entry["revision_ids"].([]any)
	if !ok {
		return nil, errors.New("invalid_input")
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("invalid_input")
		}
		out = append(out, s)
	}
	return out, nil
}
func (x *client) settleExternal(key string, intent *externalIntent) error {
	if intent.Receipt == nil {
		var found receipt
		if err := x.api("GET", "/writes/"+key, nil, &found); err != nil {
			return err
		}
		if !reflect.DeepEqual(found.Request, intent.Request) || found.Version != intent.Request.Expected+1 {
			return errors.New("external_conflict")
		}
		intent.Receipt = &found
	}
	x.j.Baseline[intent.Request.Resource] = intent.Request.Revisions
	x.j.ExternalVersion = intent.Receipt.Version
	return x.save()
}
func (x *client) exportEntry(entry map[string]any) error {
	id := textField(entry, "resource_id")
	imported := x.j.Native["issue"]
	// Descriptive metadata alone never creates an external association. The
	// connector owns this exact mapping in its private successful import receipt.
	if imported == nil || imported.Result == nil || imported.Result["resource_id"] != id {
		return nil
	}
	rev, err := revisions(entry)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(rev, x.j.Baseline[id]) {
		// An acknowledged/reconciled export still pins the exact native parent
		// for a later explicit import update; IDs are never treated as ordering.
		view, _ := entry["value"].(map[string]any)
		heads, _ := view["heads"].([]any)
		if len(heads) == 1 {
			head, _ := heads[0].(map[string]any)
			x.j.NativeHead = textField(head, "id")
		}
		return nil
	}
	view, ok := entry["value"].(map[string]any)
	if !ok {
		return errors.New("invalid_input")
	}
	if view["conflict"] == true {
		return errors.New("issue_conflict")
	}
	state, ok := view["state"].(map[string]any)
	if !ok {
		return errors.New("invalid_input")
	}
	key := operationKey(x.j.Repository, id, strings.Join(rev, "\x00"))
	intent := x.j.External[key]
	if intent == nil {
		intent = &externalIntent{Request: externalRequest{Key: key, Expected: x.j.ExternalVersion, Source: x.c.Source, Resource: id, Revisions: rev, Title: textField(state, "title"), Body: textField(state, "body"), LoseAck: x.p.LoseExternalAck}}
		x.j.External[key] = intent
		if err = x.save(); err != nil {
			return err
		}
		var acknowledged receipt
		if err = x.api("POST", "/writes", intent.Request, &acknowledged); err != nil {
			if err.Error() == "external_conflict" {
				intent.Failure = "external_conflict"
				if e := x.save(); e != nil {
					return e
				}
			}
			x.report.Status = safeError(err)
			if e := x.saveReport(); e != nil {
				return e
			}
			if err.Error() == "external_uncertain" {
				if e := x.pause("external-uncertain"); e != nil {
					return e
				}
			}
			return err
		}
		if !reflect.DeepEqual(acknowledged.Request, intent.Request) || acknowledged.Version != intent.Request.Expected+1 {
			return errors.New("external_conflict")
		}
		intent.Receipt = &acknowledged
	}
	if err = x.settleExternal(key, intent); err != nil {
		return err
	}
	heads, _ := view["heads"].([]any)
	if len(heads) == 1 {
		head, _ := heads[0].(map[string]any)
		x.j.NativeHead = textField(head, "id")
	}
	return x.save()
}
func (x *client) observe() error {
	// Reconcile uncertain connector-owned remote effects before any checkpoint can
	// advance. A missing receipt is uncertainty, never permission to blindly write.
	keys := make([]string, 0, len(x.j.External))
	for key := range x.j.External {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		intent := x.j.External[key]
		if intent.Failure != "" {
			return errors.New(intent.Failure)
		}
		if intent.Receipt == nil {
			if err := x.settleExternal(key, intent); err != nil {
				return err
			}
		}
	}
	limit := x.p.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 200 {
		return errors.New("invalid_input")
	}
	selection := map[string]any{"families": []string{"issue"}, "types": []string{}}
	method := "changes.capture"
	params := map[string]any{"selection": selection, "limit": limit}
	if x.j.Checkpoint != "" && !x.p.Fresh {
		method = "changes.since"
		params["checkpoint"] = x.j.Checkpoint
	}
	stage := map[string]map[string]any{}
	snapshot, selected, cut := "", "", ""
	for {
		page, err := x.ok(method, params)
		if err != nil {
			return err
		}
		if page["mode"] != "replace" {
			return errors.New("invalid_input")
		}
		if snapshot == "" {
			snapshot = textField(page, "snapshot_id")
			selected = textField(page, "selection_digest")
			cut = textField(page, "source_cut")
		} else if snapshot != page["snapshot_id"] || selected != page["selection_digest"] || cut != page["source_cut"] {
			return errors.New("checkpoint_invalidated")
		}
		items, ok := page["items"].([]any)
		if !ok {
			return errors.New("invalid_input")
		}
		for _, v := range items {
			entry, ok := v.(map[string]any)
			if !ok {
				return errors.New("invalid_input")
			}
			id := textField(entry, "resource_id")
			if prior := stage[id]; prior != nil && !reflect.DeepEqual(prior, entry) {
				return errors.New("checkpoint_invalidated")
			}
			stage[id] = entry
		}
		if err = x.pause("page"); err != nil {
			return err
		}
		if page["complete"] == true {
			if page["next_cursor"] != nil || textField(page, "checkpoint") == "" {
				return errors.New("invalid_input")
			}
			ids := make([]string, 0, len(stage))
			for id := range stage {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			for _, id := range ids {
				if err = x.exportEntry(stage[id]); err != nil {
					return err
				}
			}
			x.j.Entries = stage
			x.j.Selection = selected
			x.j.Snapshot = snapshot
			x.j.Checkpoint = textField(page, "checkpoint")
			return x.save()
		}
		if page["checkpoint"] != nil || textField(page, "next_cursor") == "" {
			return errors.New("invalid_input")
		}
		method = "changes.page"
		params = map[string]any{"capture": page["capture"], "cursor": page["next_cursor"], "limit": limit}
	}
}

type serviceState struct {
	Item     externalItem       `json:"item"`
	Receipts map[string]receipt `json:"receipts"`
	Writes   int                `json:"writes"`
}
type fakeService struct {
	mu          sync.Mutex
	path, token string
	state       serviceState
}

func (s *fakeService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+s.token {
		http.Error(w, "denied", http.StatusUnauthorized)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" && r.URL.Path == "/item" {
		_ = json.NewEncoder(w).Encode(s.state.Item)
		return
	}
	if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/writes/") {
		receipt, ok := s.state.Receipts[strings.TrimPrefix(r.URL.Path, "/writes/")]
		if !ok {
			http.Error(w, "unconfirmed", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(receipt)
		return
	}
	if r.Method != "POST" || r.URL.Path != "/writes" {
		http.NotFound(w, r)
		return
	}
	raw, err := readBounded(r.Body, frameBytes)
	if err != nil {
		http.Error(w, "resource_limit", http.StatusRequestEntityTooLarge)
		return
	}
	var request externalRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF || request.Key == "" {
		http.Error(w, "invalid_input", http.StatusBadRequest)
		return
	}
	if prior, ok := s.state.Receipts[request.Key]; ok {
		if !reflect.DeepEqual(prior.Request, request) {
			http.Error(w, "external_conflict", http.StatusConflict)
			return
		}
		_ = json.NewEncoder(w).Encode(prior)
		return
	}
	if request.Expected != s.state.Item.Version {
		http.Error(w, "external_conflict", http.StatusConflict)
		return
	}
	next := s.state
	next.Receipts = make(map[string]receipt, len(s.state.Receipts)+1)
	for k, v := range s.state.Receipts {
		next.Receipts[k] = v
	}
	next.Item.Title = request.Title
	next.Item.Body = request.Body
	next.Item.Version++
	next.Writes++
	result := receipt{Request: request, Version: next.Item.Version}
	next.Receipts[request.Key] = result
	if err = writePrivate(s.path, next, privateBytes); err != nil {
		http.Error(w, "unconfirmed", http.StatusServiceUnavailable)
		return
	}
	s.state = next
	// Deliberate response-loss control: the real external mutation and receipt are
	// durable, then the socket closes without any acknowledgement bytes.
	if request.LoseAck {
		if hijacker, ok := w.(http.Hijacker); ok {
			connection, _, err := hijacker.Hijack()
			if err == nil {
				_ = connection.Close()
				return
			}
		}
	}
	_ = json.NewEncoder(w).Encode(result)
}
func serve(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("state", "", "private fake tracker state")
	address := flags.String("listen", "127.0.0.1:0", "local address")
	ready := flags.String("ready", "", "private ready address file")
	credential := flags.String("credential", "", "private credential JSON file")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *path == "" || *ready == "" || *credential == "" {
		return errors.New("invalid_input")
	}
	host, _, err := net.SplitHostPort(*address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("invalid_input")
	}
	var secret struct {
		Token string `json:"token"`
	}
	if err = readPrivate(*credential, inputBytes, &secret); err != nil {
		return err
	}
	if secret.Token == "" {
		return errors.New("invalid_input")
	}
	service := &fakeService{path: *path, token: secret.Token}
	if err = readPrivate(*path, privateBytes, &service.state); err != nil {
		return err
	}
	if service.state.Receipts == nil {
		return errors.New("invalid_input")
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		return err
	}
	defer listener.Close()
	if err = os.WriteFile(*ready, []byte("http://"+listener.Addr().String()), 0600); err != nil {
		return err
	}
	server := &http.Server{Handler: service, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 10 * time.Second}
	return server.Serve(listener)
}

// These private checks cover only the connector's declared/consumed public
// methods. Other successful method/family shapes are refused, never guessed.
// The host remains responsible for grants, semantic validity and signatures.
type wireCheck func(any) bool

func wireObject(fields map[string]wireCheck) wireCheck {
	return func(v any) bool {
		m, ok := v.(map[string]any)
		if !ok || len(m) != len(fields) {
			return false
		}
		for name, check := range fields {
			value, exists := m[name]
			if !exists || !check(value) {
				return false
			}
		}
		return true
	}
}
func wireArray(check wireCheck, max int) wireCheck {
	return func(v any) bool {
		a, ok := v.([]any)
		if !ok || len(a) > max {
			return false
		}
		for _, value := range a {
			if !check(value) {
				return false
			}
		}
		return true
	}
}
func wireDistinctText(bytes, count int) wireCheck {
	return func(v any) bool {
		if !wireArray(wireString(0, bytes), count)(v) {
			return false
		}
		seen := map[string]bool{}
		for _, value := range v.([]any) {
			s := value.(string)
			if seen[s] {
				return false
			}
			seen[s] = true
		}
		return true
	}
}
func wireNullOr(check wireCheck) wireCheck { return func(v any) bool { return v == nil || check(v) } }
func wireString(min, max int) wireCheck {
	return func(v any) bool {
		s, ok := v.(string)
		return ok && utf8.ValidString(s) && len(s) >= min && len(s) <= max
	}
}
func wireEnum(values ...string) wireCheck {
	return func(v any) bool {
		for _, value := range values {
			if v == value {
				return true
			}
		}
		return false
	}
}
func wireBool(v any) bool { _, ok := v.(bool); return ok }
func wireInteger(min, max int64) wireCheck {
	return func(v any) bool {
		n, ok := v.(float64)
		return ok && n >= float64(min) && n <= float64(max) && n == math.Trunc(n)
	}
}
func wireHex(n int) wireCheck {
	return func(v any) bool {
		s, ok := v.(string)
		if !ok || len(s) != n {
			return false
		}
		for _, c := range s {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return false
			}
		}
		return true
	}
}
func wireID(v any) bool {
	s, ok := v.(string)
	return ok && strings.HasPrefix(s, "sha256:") && wireHex(64)(s[7:])
}
func wireKey(v any) bool {
	s, ok := v.(string)
	if !ok || len(s) < 1 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if c < 0x21 || c > 0x7e {
			return false
		}
	}
	return true
}
func wireVersion(v any) bool { s, ok := v.(string); return ok && len(s) <= 64 && wireKey(s) }
func wireSortedIDs(v any) bool {
	if !wireArray(wireID, frameBytes)(v) {
		return false
	}
	last := ""
	for _, value := range v.([]any) {
		s := value.(string)
		if s <= last {
			return false
		}
		last = s
	}
	return true
}
func validHello(v any) bool {
	return wireObject(map[string]wireCheck{
		"protocol": wireEnum(protocol),
		"hello":    wireObject(map[string]wireCheck{"session_id": wireHex(32), "repository_id": wireID, "module_id": wireEnum("example.tracker/tool"), "module_version": wireVersion, "registration_id": wireID}),
		"limits":   wireObject(map[string]wireCheck{"frame_bytes": wireInteger(int64(frameBytes), int64(frameBytes)), "read_timeout_ms": wireInteger(30000, 30000), "mutation_timeout_ms": wireInteger(240000, 240000), "page_max": wireInteger(200, 200), "one_outstanding": func(v any) bool { return v == true }}),
	})(v)
}
func wireRecovery(v any) bool {
	return wireEnum("do_not_retry", "retry_identical", "inspect_operation", "rescan_required")(v)
}
func wireError(v any) bool {
	if !wireObject(map[string]wireCheck{
		"origin": wireEnum("host", "native", "artifact"), "code": wireString(1, 4096), "message": wireString(0, 4096), "recovery": wireRecovery,
		"operation": wireNullOr(wireObject(map[string]wireCheck{"protocol": wireEnum("hn/0", "hn-artifact/1"), "actor": wireHex(64), "operation": wireKey})),
	})(v) {
		return false
	}
	m := v.(map[string]any)
	codes := map[string]string{
		"host":     "denied_access incompatible_version unavailable_module unsupported_capability resource_limit cancelled uncertain_operation checkpoint_invalidated verification_failure invalid_input not_found repository_error",
		"native":   "actor_mismatch admission_pending admission_refused ambiguous_id concurrent_update invalid_cursor invalid_history invalid_input issue_conflict not_found operation_conflict permission_denied repository_error resource_conflict resource_limit stale_cursor stale_revision",
		"artifact": "invalid_input resource_limit verification_failure invalid_history incomplete_dependency unsupported_schema actor_stream_conflict operation_conflict stale_revision resource_conflict concurrent_update not_found repository_error cancelled uncertain_operation",
	}
	return strings.Contains(" "+codes[m["origin"].(string)]+" ", " "+m["code"].(string)+" ")
}
func wireMutation(v any) bool {
	return wireObject(map[string]wireCheck{
		"protocol": wireEnum("hn/0"), "family": wireEnum("issue"), "type_id": func(v any) bool { return v == nil },
		"resource_id": wireID, "event_id": wireID, "replayed": wireBool, "conflict": wireBool, "head_count": wireInteger(0, 9007199254740991),
		"admission": wireNullOr(wireEnum("admitted", "pending", "hidden")), "source_cut": wireID,
	})(v)
}
func wireIssueState(v any) bool {
	raw, err := json.Marshal(v)
	if err != nil || len(raw) > 256*1024 {
		return false
	}
	metadata := func(v any) bool {
		m, ok := v.(map[string]any)
		if !ok || len(m) > 64 {
			return false
		}
		for k, value := range m {
			if !wireString(1, 128)(k) || !wireString(0, 4096)(value) {
				return false
			}
		}
		return true
	}
	return wireObject(map[string]wireCheck{
		"title": wireString(1, 1024), "body": wireString(0, 65536), "status": wireEnum("open", "closed"),
		"criteria": wireArray(wireObject(map[string]wireCheck{"id": wireString(1, 64), "text": wireString(0, 4096)}), 64),
		"labels":   wireDistinctText(256, 64), "assignees": wireDistinctText(256, 64),
		"relations": wireArray(wireObject(map[string]wireCheck{"kind": wireEnum("blocks", "related", "parent"), "target": wireID}), 128), "metadata": metadata,
	})(v)
}
func wireIssueView(v any) bool {
	if !wireObject(map[string]wireCheck{
		"protocol": wireEnum("hn/0"), "family": wireEnum("issue"), "id": wireID, "creator": wireHex(64),
		"heads":    wireArray(wireObject(map[string]wireCheck{"id": wireID, "actor": wireHex(64), "state": wireIssueState}), frameBytes),
		"conflict": wireBool, "state": wireNullOr(wireIssueState),
		"comments": wireArray(wireObject(map[string]wireCheck{"id": wireID, "actor": wireHex(64), "body": wireString(0, 65536)}), frameBytes),
	})(v) {
		return false
	}
	m := v.(map[string]any)
	return (m["conflict"] == true) == (m["state"] == nil)
}
func wireOperation(v any, params any) bool {
	if !wireObject(map[string]wireCheck{
		"status": wireEnum("found", "verified_absent", "uncertain"), "protocol": wireEnum("hn/0"), "actor": wireHex(64), "operation": wireKey,
		"family": wireEnum("issue"), "type_id": func(v any) bool { return v == nil }, "request": wireNullOr(wireID), "resource_id": wireNullOr(wireID), "event_id": wireNullOr(wireID),
		"result": wireNullOr(wireMutation), "source_cut": wireID,
		"reason": wireNullOr(wireEnum("incomplete_selection", "unavailable_history", "private_intent_pending", "visibility_restricted", "publication_unconfirmed")),
	})(v) {
		return false
	}
	m := v.(map[string]any)
	p, ok := params.(map[string]any)
	if !ok || m["actor"] != p["actor"] || m["operation"] != p["operation"] || m["family"] != p["family"] {
		return false
	}
	if m["status"] == "found" {
		if m["request"] == nil || m["resource_id"] == nil || m["event_id"] == nil || m["result"] == nil || m["reason"] != nil {
			return false
		}
		r := m["result"].(map[string]any)
		return r["resource_id"] == m["resource_id"] && r["event_id"] == m["event_id"]
	}
	return m["request"] == nil && m["resource_id"] == nil && m["event_id"] == nil && m["result"] == nil && ((m["status"] == "uncertain") == (m["reason"] != nil))
}
func wireChangePage(v any) bool {
	selection := wireObject(map[string]wireCheck{"families": wireArray(wireEnum("issue"), 1), "types": wireArray(func(any) bool { return false }, 0)})
	entry := func(v any) bool {
		if !wireObject(map[string]wireCheck{"protocol": wireEnum("hn/0"), "family": wireEnum("issue"), "type_id": func(v any) bool { return v == nil }, "resource_id": wireID, "revision_ids": wireSortedIDs, "value": wireIssueView})(v) {
			return false
		}
		m := v.(map[string]any)
		view := m["value"].(map[string]any)
		return m["resource_id"] == view["id"]
	}
	if !wireObject(map[string]wireCheck{
		"capture": wireString(1, 4096), "snapshot_id": wireID, "selection": selection, "selection_digest": wireID, "source_cut": wireID, "mode": wireEnum("replace"),
		"items": wireArray(entry, 200), "next_cursor": wireNullOr(wireString(1, 4096)), "complete": wireBool, "checkpoint": wireNullOr(wireString(1, 4096)),
		"expires_at": func(v any) bool {
			s, ok := v.(string)
			if !ok || !strings.HasSuffix(s, "Z") {
				return false
			}
			_, err := time.Parse(time.RFC3339Nano, s)
			return err == nil
		},
	})(v) {
		return false
	}
	m := v.(map[string]any)
	selected := m["selection"].(map[string]any)
	if len(selected["families"].([]any)) != 1 {
		return false
	}
	return (m["complete"] == true && m["next_cursor"] == nil && m["checkpoint"] != nil) || (m["complete"] == false && m["next_cursor"] != nil && m["checkpoint"] == nil)
}
func wireCapabilities(v any, hello map[string]any) bool {
	if !wireObject(map[string]wireCheck{
		"session":      wireObject(map[string]wireCheck{"session_id": wireHex(32), "module_id": wireEnum("example.tracker/tool"), "module_version": wireVersion, "registration_id": wireID, "repository_id": wireID, "protocol": wireEnum(protocol)}),
		"capabilities": wireArray(wireObject(map[string]wireCheck{"method": wireString(1, 128), "declared": wireBool, "granted": wireBool, "enabled": wireBool, "compatible": wireBool, "availability": wireEnum("observed", "not_probed", "unavailable"), "recovery": wireRecovery}), 14),
		"unsupported":  wireArray(wireObject(map[string]wireCheck{"surface": wireString(1, 128), "status": wireEnum("unsupported"), "recovery": wireEnum("do_not_retry")}), 13),
	})(v) {
		return false
	}
	m := v.(map[string]any)
	session := m["session"].(map[string]any)
	binding, ok := hello["hello"].(map[string]any)
	if !ok {
		return false
	}
	for k, value := range binding {
		if session[k] != value {
			return false
		}
	}
	inventory := func(v any, key, inventory string) bool {
		remaining := map[string]bool{}
		for _, s := range strings.Fields(inventory) {
			remaining[s] = true
		}
		for _, value := range v.([]any) {
			name := textField(value.(map[string]any), key)
			if !remaining[name] {
				return false
			}
			delete(remaining, name)
		}
		return len(remaining) == 0
	}
	return inventory(m["capabilities"], "method", "capabilities.list identity.public resource.read resource.mutate resource.operation artifact.read artifact.mutate artifact.operation artifact.search artifact.type.read artifact.type.publish changes.capture changes.page changes.since") && inventory(m["unsupported"], "surface", "memory.mutate proposal review decide merge policy.mutate identity.mutate run runner audit automerge sync.apply migration.apply")
}
func validateResponse(response map[string]any, id, method string, params any, hello map[string]any) error {
	invalid := errors.New("invalid_input")
	if !wireObject(map[string]wireCheck{"protocol": wireEnum(protocol), "request_id": wireNullOr(wireKey), "ok": wireBool, "data": func(any) bool { return true }, "error": func(any) bool { return true }})(response) {
		return invalid
	}
	if response["request_id"] != id && response["request_id"] != nil {
		return invalid
	}
	if response["ok"] == false {
		if response["data"] != nil || !wireError(response["error"]) {
			return invalid
		}
		if response["request_id"] == nil {
			return errors.New(textField(response["error"].(map[string]any), "code"))
		}
		return nil // ordinary known refusal: the caller must handle its result
	}
	if response["request_id"] != id || response["error"] != nil {
		return invalid
	}
	data := response["data"]
	valid := false
	switch method {
	case "identity.public":
		valid = wireObject(map[string]wireCheck{"initialized": wireBool, "actor": wireNullOr(wireHex(64)), "name": wireNullOr(wireString(0, frameBytes)), "public_key": wireNullOr(func(v any) bool {
			s, ok := v.(string)
			if !ok {
				return false
			}
			raw, err := base64.RawStdEncoding.DecodeString(s)
			return err == nil && len(raw) == 32 && base64.RawStdEncoding.EncodeToString(raw) == s
		})})(data)
		if valid {
			m := data.(map[string]any)
			if m["initialized"] == false {
				valid = m["actor"] == nil && m["name"] == nil && m["public_key"] == nil
			} else {
				valid = m["actor"] != nil && m["name"] != nil && m["public_key"] != nil
				if valid {
					key, _ := base64.RawStdEncoding.DecodeString(m["public_key"].(string))
					sum := sha256.Sum256(key)
					valid = m["actor"] == hex.EncodeToString(sum[:])
				}
			}
		}
	case "resource.mutate":
		valid = wireMutation(data)
		if p, ok := params.(map[string]any); valid && ok {
			m := data.(map[string]any)
			valid = p["family"] == m["family"] && (p["id"] == nil || p["id"] == m["resource_id"])
		} else {
			valid = false
		}
	case "resource.operation":
		valid = wireOperation(data, params)
	case "resource.read":
		valid = wireObject(map[string]wireCheck{"items": wireArray(wireIssueView, 200), "source_cut": wireID, "next_cursor": wireNullOr(wireString(1, 4096))})(data)
	case "changes.capture", "changes.page", "changes.since":
		valid = wireChangePage(data)
	case "capabilities.list":
		valid = wireCapabilities(data, hello)
	}
	if !valid {
		return invalid
	}
	return nil
}

// encoding/json accepts duplicate object names and replaces malformed Unicode.
// Validate the complete JSON value domain before materializing a host frame.
func validWireJSON(raw []byte) bool {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return false
	}
	// Check escaped UTF-16 pairs too: json.Valid permits lone surrogates.
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		n, _ := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func() bool
	value = func() bool {
		token, err := d.Token()
		if err != nil {
			return false
		}
		switch token := token.(type) {
		case json.Delim:
			if token == '{' {
				seen := map[string]bool{}
				for d.More() {
					k, err := d.Token()
					if err != nil {
						return false
					}
					name, ok := k.(string)
					if !ok || seen[name] {
						return false
					}
					seen[name] = true
					if !value() {
						return false
					}
				}
			} else if token == '[' {
				for d.More() {
					if !value() {
						return false
					}
				}
			} else {
				return false
			}
			_, err := d.Token()
			return err == nil
		case json.Number:
			s := token.String()
			if strings.ContainsAny(s, ".eE") || s == "-0" {
				return false
			}
			n, err := strconv.ParseInt(s, 10, 64)
			return err == nil && n >= -9007199254740991 && n <= 9007199254740991
		}
		return true
	}
	return value()
}
