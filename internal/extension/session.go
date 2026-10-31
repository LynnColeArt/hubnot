package extension

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const StartupBudget = 10 * time.Second
const ReadBudget = 30 * time.Second
const MutationBudget = 240 * time.Second

// Session is public identity only. Executable paths, argv and epochs stay private.
type Session struct {
	Protocol       string `json:"protocol"`
	SessionID      string `json:"session_id"`
	RepositoryID   string `json:"repository_id"`
	ModuleID       string `json:"module_id"`
	ModuleVersion  string `json:"module_version"`
	RegistrationID string `json:"registration_id"`
}
type RunResult struct {
	Session           Session `json:"session"`
	Outcome           string  `json:"outcome"`
	CompletedRequests int     `json:"completed_requests"`
	ExitCode          int     `json:"exit_code"`
	PendingOperations []any   `json:"pending_operations"`
}
type Request struct {
	ID        string         `json:"request_id"`
	Method    string         `json:"method"`
	Params    map[string]any `json:"params"`
	TimeoutMS int            `json:"timeout_ms"`
}

// Dispatch executes synchronously in the request context. A returning result is
// settled: it must never schedule publication after returning a refusal.
type Invocation struct {
	Request      Request
	Registration Registration
	Enablement   EnablementEntry
	Binding      Binding
}
type Dispatch func(context.Context, Invocation) (any, error)
type SessionConfig struct {
	Registration Registration
	Store        *EnablementStore
	Directory    string
	Started      time.Time
	Recheck      func(context.Context, Binding) (Registration, EnablementEntry, error)
	Dispatch     Dispatch
}
type wireResponse struct {
	Protocol  string  `json:"protocol"`
	RequestID *string `json:"request_id"`
	OK        bool    `json:"ok"`
	Data      any     `json:"data"`
	Error     *Error  `json:"error"`
}
type frameEvent struct {
	payload []byte
	first   time.Time
	err     error
}

func sortStrings(values []string) { sort.Strings(values) }
func publicError(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return failure("repository_error", "host request failed").(*Error)
}
func writeMessage(file *os.File, deadline time.Time, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return failure("repository_error", "cannot encode host response")
	}
	if len(data) > FrameLimit {
		return failure("resource_limit", "complete response exceeds frame limit")
	}
	_ = file.SetWriteDeadline(deadline)
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(data)))
	for _, part := range [][]byte{prefix[:], data} {
		for len(part) > 0 {
			n, e := file.Write(part)
			if e != nil {
				return failure("cancelled", "module response could not be delivered")
			}
			part = part[n:]
		}
	}
	return nil
}

// sessionReader keeps the current request/idle deadline separate from the
// first-byte partial-frame cap. Extending a mutation budget cannot extend an
// already-started control prefix/payload beyond the frozen thirty seconds.
type sessionReader struct {
	file            *os.File
	mu              sync.Mutex
	deadline, first time.Time
}

func (reader *sessionReader) applyDeadline() {
	deadline := reader.deadline
	if !reader.first.IsZero() {
		partial := reader.first.Add(ReadBudget)
		if partial.Before(deadline) {
			deadline = partial
		}
	}
	_ = reader.file.SetReadDeadline(deadline)
}
func (reader *sessionReader) setDeadline(deadline time.Time) {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	reader.deadline = deadline
	reader.applyDeadline()
}
func (reader *sessionReader) read() frameEvent {
	file := reader.file
	reader.mu.Lock()
	reader.first = time.Time{}
	reader.applyDeadline()
	reader.mu.Unlock()

	var prefix [4]byte
	n, err := file.Read(prefix[:1])
	first := time.Now()
	reader.mu.Lock()
	reader.first = first
	reader.applyDeadline()
	reader.mu.Unlock()
	if err != nil {
		return frameEvent{first: first, err: err}
	}
	if n != 1 {
		return frameEvent{first: first, err: io.ErrUnexpectedEOF}
	}
	if _, err = io.ReadFull(file, prefix[1:]); err != nil {
		return frameEvent{first: first, err: sessionReadError(err)}
	}
	size := binary.BigEndian.Uint32(prefix[:])
	if size == 0 {
		return frameEvent{first: first, err: failure("invalid_input", "frame payload is empty")}
	}
	if size > FrameLimit {
		return frameEvent{first: first, err: failure("resource_limit", "frame payload exceeds byte limit")}
	}
	payload := make([]byte, int(size))
	if _, err = io.ReadFull(file, payload); err != nil {
		return frameEvent{first: first, err: sessionReadError(err)}
	}
	return frameEvent{payload: payload, first: first}
}
func sessionReadError(err error) error {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return failure("cancelled", "module read deadline expired")
	}
	return frameReadError(err)
}
func transportFailure(err error) error {
	var public *Error
	if errors.As(err, &public) {
		return public
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return failure("cancelled", "module read deadline expired")
	}
	return failure("unavailable_module", "module disconnected during request")
}
func mutation(method string) bool {
	return method == "resource.mutate" || method == "artifact.mutate" || method == "artifact.type.publish"
}
func requestOperation(r Request) any {
	protocol := "hn-artifact/1"
	if r.Method == "resource.mutate" {
		protocol = "hn/0"
	}
	return map[string]any{"protocol": protocol, "actor": r.Params["actor"], "operation": r.Params["operation"]}
}
func uncertain(r Request) error {
	return &Error{Origin: "host", Code: "uncertain_operation", Message: "dispatched mutation outcome requires inspection", Recovery: "inspect_operation", Operation: requestOperation(r)}
}
func requestAccess(r Request) ([]Access, error) {
	selector, action := "", "read"
	switch r.Method {
	case "capabilities.list":
		return []Access{{Method: r.Method}}, nil
	case "identity.public":
		selector = "native:identity"
	case "resource.read", "resource.operation", "resource.mutate":
		selector = "native:" + r.Params["family"].(string)
		if r.Method == "resource.mutate" {
			var err error
			action, err = NativeAction(r.Params["family"].(string), r.Params["intent"].(string))
			if err != nil {
				return nil, err
			}
		}
	case "artifact.mutate", "artifact.type.publish":
		payload := r.Params["payload"].(map[string]any)
		selector = "artifact:" + payload["type_id"].(string)
		action = "type.publish"
		if r.Method == "artifact.mutate" {
			switch r.Params["kind"] {
			case "record.open":
				action = "create"
			case "record.revise", "record.convert":
				action = "revise"
			case "record.resolve":
				action = "resolve"
			}
		}
	case "artifact.read":
		if r.Params["opaque"] == true {
			selector = "artifact:opaque"
			action = "raw.inspect"
		} else if id, ok := r.Params["type_id"].(string); ok {
			selector = "artifact:" + id
		}
	case "artifact.operation", "artifact.search", "artifact.type.read":
		if id, ok := r.Params["type_id"].(string); ok {
			selector = "artifact:" + id
		}
	case "changes.capture", "changes.since":
		selection := r.Params["selection"].(map[string]any)
		access := []Access{}
		for _, v := range selection["families"].([]any) {
			access = append(access, Access{r.Method, "native:" + v.(string), "observe"})
		}
		for _, v := range selection["types"].([]any) {
			access = append(access, Access{r.Method, "artifact:" + v.(string), "observe"})
		}
		return access, nil
	}
	if selector == "" {
		return nil, failure("unsupported_capability", "request requires an adapter-resolved resource scope")
	}
	return []Access{{r.Method, selector, action}}, nil
}

// AuthorizeRequest is used by the current compiled fallback handler for scopes
// derivable from typed params. Real method handlers resolve their actual targets
// or capture tokens and call Authorize for every selected scope before effects.
func AuthorizeRequest(call Invocation) error {
	reg, entry, binding, r := call.Registration, call.Enablement, call.Binding, call.Request
	accesses, err := requestAccess(r)
	if err != nil {
		return err
	}
	for _, access := range accesses {
		if err = Authorize(reg, entry, binding, access, false); err != nil {
			return err
		}
	}
	return nil
}

type capability struct {
	Method       string `json:"method"`
	Declared     bool   `json:"declared"`
	Granted      bool   `json:"granted"`
	Enabled      bool   `json:"enabled"`
	Compatible   bool   `json:"compatible"`
	Availability string `json:"availability"`
	Recovery     string `json:"recovery"`
}
type unsupportedSurface struct {
	Surface  string `json:"surface"`
	Status   string `json:"status"`
	Recovery string `json:"recovery"`
}
type capabilitiesResult struct {
	Session      Session              `json:"session"`
	Capabilities []capability         `json:"capabilities"`
	Unsupported  []unsupportedSurface `json:"unsupported"`
}

func capabilities(session Session, r Registration, e EnablementEntry) capabilitiesResult {
	result := capabilitiesResult{Session: session, Capabilities: []capability{}, Unsupported: []unsupportedSurface{}}
	binding := Binding{r.RegistrationID, e.RepositoryID, e.Epoch}
	for _, method := range methodNames() {
		declared := false
		for _, m := range r.Manifest.Capabilities {
			if m == method {
				declared = true
			}
		}
		granted := method == "capabilities.list"
		for _, g := range e.Grants {
			selector := g.Selector
			if strings.HasSuffix(selector, "/*") {
				selector = strings.TrimSuffix(selector, "*") + "type"
			}
			for _, a := range g.Actions {
				if Authorize(r, e, binding, Access{method, selector, a}, false) == nil {
					granted = true
				}
			}
		}
		result.Capabilities = append(result.Capabilities, capability{method, declared, granted, e.Enabled, true, "observed", "do_not_retry"})
	}
	for _, surface := range unsupportedNames() {
		result.Unsupported = append(result.Unsupported, unsupportedSurface{surface, "unsupported", "do_not_retry"})
	}
	return result
}

// Run is called only by explicit extension run. The watchdog includes validation,
// startup, each frame/execution/response and idle revocation checks.
func Run(parent context.Context, cfg SessionConfig) (result RunResult, runErr error) {
	started := cfg.Started
	if started.IsZero() {
		started = time.Now()
	}
	startup, cancelStartup := context.WithDeadline(parent, started.Add(StartupBudget))
	defer cancelStartup()
	if cfg.Store == nil || cfg.Recheck == nil || cfg.Dispatch == nil {
		return result, failure("invalid_input", "host session configuration is incomplete")
	}
	reg := cfg.Registration
	entry, found, err := cfg.Store.Entry(reg)
	if err != nil {
		return result, err
	}
	if !found {
		return result, failure("denied_access", "module is not enabled in this repository")
	}
	binding := Binding{reg.RegistrationID, cfg.Store.RepositoryID(), entry.Epoch}
	reg, entry, err = cfg.Recheck(startup, binding)
	if err != nil {
		return result, err
	}
	var random [16]byte
	if _, err = io.ReadFull(rand.Reader, random[:]); err != nil {
		return result, failure("repository_error", "cannot obtain session randomness")
	}
	session := Session{HostProtocol, hex.EncodeToString(random[:]), binding.RepositoryID, reg.Manifest.ModuleID, reg.Manifest.ModuleVersion, reg.RegistrationID}
	slot, err := cfg.Store.acquireSlot(slotIdentity{SessionID: session.SessionID, RegistrationID: reg.RegistrationID, Epoch: binding.Epoch})
	if err != nil {
		return result, err
	}
	cleanupVerified := true
	defer func() { slot.close(cleanupVerified) }()
	owner, err := acquireRegistrationOwner(slotIdentity{SessionID: session.SessionID, RegistrationID: reg.RegistrationID, Epoch: binding.Epoch})
	if err != nil {
		return result, err
	}
	defer func() { owner.close(cleanupVerified) }()
	// Recheck after acquiring the slot so a simultaneous management edit cannot
	// return success between launch validation and ownership becoming visible.
	reg, entry, err = cfg.Recheck(startup, binding)
	if err != nil {
		return result, err
	}
	if startup.Err() != nil {
		return result, failure("cancelled", "module startup cancelled")
	}
	child, err := startChild(reg, cfg.Directory)
	if err != nil {
		return result, err
	}
	cleanupVerified = false
	defer func() {
		if e := child.close(); e != nil {
			if runErr != nil && publicError(runErr).Code == "uncertain_operation" {
				publicError(runErr).Message = "dispatched mutation requires inspection; child cleanup could not be verified"
			} else {
				runErr = e
			}
		}
		cleanupVerified = child.cleanupErr == nil
		select {
		case <-child.overflow:
			if runErr == nil {
				runErr = failure("resource_limit", "module diagnostics exceed byte limit")
			}
		default:
		}
	}()
	sessionCtx, cancel := context.WithCancel(parent)
	defer cancel()
	// A separate watchdog remains active during blocked reads AND writes. It
	// closes only owned handles; the request goroutine is still joined below.
	abort := make(chan error, 1)
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		tick := time.NewTicker(RevocationInterval)
		defer tick.Stop()
		childDone := child.done
		var exitGrace <-chan time.Time
		for {
			var cause error
			select {
			case <-sessionCtx.Done():
				if parent.Err() == nil {
					return
				}
				cause = failure("cancelled", "host session cancelled")
			case <-parent.Done():
				cause = failure("cancelled", "host session cancelled")
			case <-child.overflow:
				cause = failure("resource_limit", "module diagnostics exceed byte limit")
			case <-childDone:
				if child.closing.Load() {
					return
				}
				childDone = nil
				if child.err != nil {
					cause = failure("unavailable_module", "module exited unsuccessfully")
				} else {
					exitGrace = time.After(100 * time.Millisecond)
				}
			case <-exitGrace:
				cause = failure("unavailable_module", "module exited with unfinished protocol pipes")
			case <-tick.C:
				cause = owner.check()
				if cause == nil {
					_, _, cause = cfg.Recheck(sessionCtx, binding)
				}
			}
			if cause != nil {
				abort <- cause
				cancel()
				_ = child.close()
				return
			}
		}
	}()
	defer func() { cancel(); <-monitorDone }()
	defer func() {
		select {
		case cause := <-abort:
			if runErr == nil || publicError(runErr).Code != "uncertain_operation" {
				runErr = cause
			}
		default:
		}
	}()

	frames := make(chan frameEvent, 1)
	readerDone := make(chan struct{})
	reader := &sessionReader{file: child.output}
	reader.setDeadline(started.Add(StartupBudget))
	go func() {
		defer close(readerDone)
		for {
			event := reader.read()
			select {
			case frames <- event:
			case <-sessionCtx.Done():
				return
			}
			if event.err != nil {
				return
			}
		}
	}()
	defer func() { cancel(); _ = child.output.Close(); <-readerDone }()
	hello := map[string]any{"protocol": HostProtocol, "hello": map[string]any{"session_id": session.SessionID, "repository_id": session.RepositoryID, "module_id": session.ModuleID, "module_version": session.ModuleVersion, "registration_id": session.RegistrationID}, "limits": map[string]any{"frame_bytes": FrameLimit, "read_timeout_ms": 30000, "mutation_timeout_ms": 240000, "page_max": 200, "one_outstanding": true}}
	if err = writeMessage(child.input, started.Add(StartupBudget), hello); err != nil {
		return result, err
	}
	select {
	case <-startup.Done():
		return result, failure("cancelled", "module startup cancelled")
	case cause := <-abort:
		return result, cause
	case event := <-frames:
		if event.err != nil {
			if e, ok := event.err.(*Error); ok && e.Code == "cancelled" {
				return result, e
			}
			return result, failure("unavailable_module", "module ended before ready")
		}
		if _, err = InspectMessage(event.payload, "ready", "", ""); err != nil {
			return result, err
		}
	}
	if startup.Err() != nil {
		return result, failure("cancelled", "module startup cancelled")
	}
	result = RunResult{session, "completed", 0, 0, []any{}}
	seen := map[string]bool{}
	reader.setDeadline(time.Now().Add(ReadBudget))
	var pending *frameEvent
	for {
		var event frameEvent
		if pending != nil {
			event = *pending
			pending = nil
		} else {
			select {
			case <-parent.Done():
				return result, failure("cancelled", "host session cancelled")
			case <-child.overflow:
				return result, failure("resource_limit", "module diagnostics exceed byte limit")
			case cause := <-abort:
				return result, cause
			case event = <-frames:
			}
		}

		if errors.Is(event.err, io.EOF) {
			select {
			case <-child.done:
			case <-time.After(100 * time.Millisecond):
				return result, failure("unavailable_module", "module did not exit after EOF")
			}
			if child.err != nil {
				return result, failure("unavailable_module", "module exited unsuccessfully")
			}
			select {
			case <-child.overflow:
				return result, failure("resource_limit", "module diagnostics exceed byte limit")
			default:
			}
			return result, nil
		}
		if event.err != nil {
			if e, ok := event.err.(*Error); ok {
				return result, e
			}
			return result, failure("cancelled", "module read deadline expired")
		}
		inspection, e := InspectMessage(event.payload, "request", "", "")
		if e != nil {
			return result, e
		}
		var request Request
		_ = json.Unmarshal(event.payload, &request)
		if seen[request.ID] {
			return result, failure("invalid_input", "request ID was already used")
		}
		seen[request.ID] = true
		budget := ReadBudget
		if mutation(request.Method) {
			budget = MutationBudget
		}
		if request.TimeoutMS != 0 {
			budget = time.Duration(request.TimeoutMS) * time.Millisecond
		}
		deadline := event.first.Add(budget)
		ctx, requestCancel := context.WithDeadline(sessionCtx, deadline)
		reader.setDeadline(deadline)
		e = owner.check()
		if e == nil {
			reg, entry, e = cfg.Recheck(ctx, binding)
		}
		if e == nil {
			if request.Method != "capabilities.list" {
				declared := false
				for _, method := range reg.Manifest.Capabilities {
					if method == request.Method {
						declared = true
						break
					}
				}
				if !declared {
					e = failure("denied_access", "method is not declared")
				}
			}
		}
		if e == nil && ctx.Err() != nil {
			e = failure("cancelled", "request cancelled before dispatch")
		}
		type dispatchResult struct {
			data any
			err  error
		}
		done := make(chan dispatchResult, 1)
		dispatched := false
		if e != nil {
			done <- dispatchResult{nil, e}
		} else {
			dispatched = true
			go func() {
				if request.Method == "capabilities.list" {
					done <- dispatchResult{capabilities(session, reg, entry), nil}
				} else {
					data, e := cfg.Dispatch(ctx, Invocation{Request: request, Registration: reg, Enablement: entry, Binding: binding})
					done <- dispatchResult{data, e}
				}
			}()
		}
		var completed dispatchResult
		interrupted := error(nil)
		waiting := true
		for waiting {
			select {
			case completed = <-done:
				waiting = false
			case <-ctx.Done():
				interrupted = failure("cancelled", "request deadline expired")
				waiting = false
			case <-child.overflow:
				interrupted = failure("resource_limit", "module diagnostics exceed byte limit")
				waiting = false
			case cause := <-abort:
				interrupted = cause
				waiting = false
			case control := <-frames:
				interrupted = failure("invalid_input", "second business request or invalid cancel control")
				if control.err != nil {
					interrupted = transportFailure(control.err)
				} else if _, e = InspectMessage(control.payload, "cancel", request.ID, ""); e == nil {
					interrupted = failure("cancelled", "request cancelled")
				}
				waiting = false
			}
		}
		if interrupted != nil {
			requestCancel()
			_ = child.close()
			if dispatched {
				completed = <-done
			}
			if dispatched && mutation(request.Method) {
				var known *Error
				if errors.As(completed.err, &known) && known.Code != "cancelled" && known.Code != "uncertain_operation" {
					return result, known
				}
				return result, uncertain(request)
			}
			return result, interrupted
		}
		if ctx.Err() != nil {
			requestCancel()
			if dispatched && mutation(request.Method) {
				return result, uncertain(request)
			}
			return result, failure("cancelled", "request deadline expired")
		}
		response := wireResponse{HostProtocol, inspection.RequestID, completed.err == nil, completed.data, nil}
		if completed.err != nil {
			response.Data = nil
			response.Error = publicError(completed.err)
		}
		// Validate actual adapter output with the frozen typed response contract.
		encoded, _ := json.Marshal(response)
		method := ""
		if response.OK {
			method = request.Method
		}
		_, e = InspectMessage(encoded, "response", request.ID, method)
		if e == nil {
			written := make(chan error, 1)
			go func() { written <- writeMessage(child.input, deadline, response) }()
			writing := true
			for writing {
				select {
				case e = <-written:
					writing = false
				case <-ctx.Done():
					e = failure("cancelled", "response deadline expired")
					_ = child.close()
					<-written
					writing = false
				case cause := <-abort:
					e = cause
					_ = child.close()
					<-written
					writing = false
				case control := <-frames:
					// A completion already published by the writer means this is the next
					// request. Otherwise it is a control/second request in the active budget.
					select {
					case e = <-written:
						pending = &control
						writing = false
					default:
						e = failure("invalid_input", "second business request or invalid cancel control")
						if control.err != nil {
							e = transportFailure(control.err)
						} else if _, cancelError := InspectMessage(control.payload, "cancel", request.ID, ""); cancelError == nil {
							e = failure("cancelled", "request cancelled during response")
						}
						requestCancel()
						_ = child.close()
						<-written
						writing = false
					}
				}
			}
		}

		requestCancel()
		if e != nil {
			if dispatched && mutation(request.Method) {
				return result, uncertain(request)
			}
			return result, e
		}
		result.CompletedRequests++
		reader.setDeadline(time.Now().Add(ReadBudget))
	}
}
