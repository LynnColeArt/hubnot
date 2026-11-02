package extension

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestSessionBudgetsMirrorFrozenContract(t *testing.T) {
	if StartupBudget != 10*time.Second || ReadBudget != 30*time.Second || MutationBudget != 240*time.Second || StderrLimit != 65536 || SessionLimit != 4 || RevocationInterval != 250*time.Millisecond || TerminateGrace != 2*time.Second || ExitBudget != 5*time.Second {
		t.Fatal("host-v1 frozen lifecycle bounds drifted")
	}
}

func TestSessionReaderDeadlineAndEOF(t *testing.T) {
	for _, tc := range []struct {
		name        string
		prefix      []byte
		closeOutput bool
		code        string
	}{
		{"first-byte-deadline", nil, false, "cancelled"},
		{"prefix-deadline", []byte{0}, false, "cancelled"},
		{"payload-deadline", []byte{0, 0, 0, 2, '{'}, false, "cancelled"},
		{"clean-eof", nil, true, ""},
		{"prefix-eof", []byte{0}, true, "invalid_input"},
		{"payload-eof", []byte{0, 0, 0, 2, '{'}, true, "invalid_input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, output, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			defer output.Close()
			if len(tc.prefix) != 0 {
				if _, err = output.Write(tc.prefix); err != nil {
					t.Fatal(err)
				}
			}
			if tc.closeOutput {
				if err = output.Close(); err != nil {
					t.Fatal(err)
				}
			}
			reader := &sessionReader{file: input}
			deadline := time.Now().Add(100 * time.Millisecond)
			if tc.name == "first-byte-deadline" {
				// No context/timer select participates: the real pipe's OS deadline
				// must itself produce the host cancellation before any ready frame.
				deadline = time.Now().Add(-time.Second)
			}
			reader.setDeadline(deadline)
			event := reader.read()
			if event.sequence != 1 || event.first.IsZero() || event.payload != nil {
				t.Fatalf("incomplete read observation %#v", event)
			}
			if tc.code == "" {
				if event.err != io.EOF {
					t.Fatalf("clean EOF identity changed: %v", event.err)
				}
				return
			}
			var public *Error
			if !errors.As(event.err, &public) || public.Origin != "host" || public.Code != tc.code || public.Recovery != "do_not_retry" || public.Operation != nil {
				t.Fatalf("%s must be host/%s/do_not_retry without operation: %#v", tc.name, tc.code, event.err)
			}
		})
	}
	t.Run("closed-input", func(t *testing.T) {
		input, output, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer output.Close()
		if err = input.Close(); err != nil {
			t.Fatal(err)
		}
		event := (&sessionReader{file: input}).read()
		var public *Error
		if !errors.Is(event.err, os.ErrClosed) || errors.As(event.err, &public) {
			t.Fatalf("other first-byte I/O error was reclassified: %v", event.err)
		}
	})
}
func TestSessionSynchronousCancellationAndUncertainty(t *testing.T) {
	for _, mode := range []string{"timeout", "cancel", "second", "wrong-cancel", "eof", "read-timeout"} {
		t.Run(mode, func(t *testing.T) {
			cfg, pid := sessionFixture(t, mode)
			entered, settled := make(chan struct{}), make(chan struct{})
			cfg.Dispatch = func(ctx context.Context, call Invocation) (any, error) {
				if err := AuthorizeRequest(call); err != nil {
					return nil, err
				}
				close(entered)
				<-ctx.Done()
				time.Sleep(20 * time.Millisecond)
				close(settled)
				return nil, ctx.Err()
			}
			started := time.Now()
			_, err := Run(context.Background(), cfg)
			if time.Since(started) > 3*time.Second {
				t.Fatal("request watchdog failed")
			}
			select {
			case <-entered:
			default:
				t.Fatal("request never dispatched")
			}
			select {
			case <-settled:
			default:
				t.Fatal("host returned before synchronous writer settled")
			}
			var public *Error
			if !errors.As(err, &public) {
				t.Fatalf("public failure %v", err)
			}
			if mode == "read-timeout" {
				if public.Code != "cancelled" || public.Operation != nil {
					t.Fatalf("read cancellation %#v", public)
				}
			} else {
				if public.Code != "uncertain_operation" || public.Recovery != "inspect_operation" || !reflect.DeepEqual(public.Operation, map[string]any{"protocol": "hn/0", "actor": repeatActor(), "operation": "actual-key"}) {
					t.Fatalf("lost dispatched write %#v", public)
				}
			}
			assertSessionPIDGone(t, pid)
		})
	}
}
func TestSessionNoDispatchAfterRevokedEpoch(t *testing.T) {
	cfg, pid := sessionFixture(t, "idle")
	cfg.Dispatch = func(context.Context, Invocation) (any, error) { t.Error("idle child dispatched"); return nil, nil }
	done := make(chan error, 1)
	go func() { _, e := Run(context.Background(), cfg); done <- e }()
	ready := time.Now().Add(3 * time.Second)
	for {
		if _, e := os.Stat(pid); e == nil {
			break
		}
		if time.Now().After(ready) {
			t.Fatal("child did not establish live revocation fixture")
		}
		time.Sleep(10 * time.Millisecond)
	}
	entry, _, e := cfg.Store.Entry(cfg.Registration)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = cfg.Store.Set(cfg.Registration, false, entry.Grants); e != nil {
		t.Fatal(e)
	}
	select {
	case e = <-done:
		expectGrantError(t, e, "denied_access")
	case <-time.After(2 * time.Second):
		t.Fatal("revocation watchdog failed")
	}
	assertSessionPIDGone(t, pid)
}

func TestSessionRequestScopeContractMatrix(t *testing.T) {
	fixtures := paramFixtures()
	fixtures["artifact.read"]["type_id"] = "example.test/item"
	fixtures["artifact.search"]["type_id"] = "example.test/item"
	fixtures["artifact.type.read"]["type_id"] = "example.test/item"
	// Independent complete method mirror from host-v1 section4. The page cursor
	// needs the later capture adapter's resolved selection and currently refuses.
	expected := map[string][]Access{
		"capabilities.list":     {{"capabilities.list", "", ""}},
		"identity.public":       {{"identity.public", "native:identity", "read"}},
		"resource.read":         {{"resource.read", "native:issue", "read"}},
		"resource.operation":    {{"resource.operation", "native:issue", "read"}},
		"resource.mutate":       {{"resource.mutate", "native:issue", "create"}},
		"artifact.read":         {{"artifact.read", "artifact:example.test/item", "read"}},
		"artifact.operation":    {{"artifact.operation", "artifact:example.test/item", "read"}},
		"artifact.mutate":       {{"artifact.mutate", "artifact:example.test/item", "create"}},
		"artifact.search":       {{"artifact.search", "artifact:example.test/item", "read"}},
		"artifact.type.read":    {{"artifact.type.read", "artifact:example.test/item", "read"}},
		"artifact.type.publish": {{"artifact.type.publish", "artifact:example.test/item", "type.publish"}},
		"changes.capture":       {{"changes.capture", "native:issue", "observe"}},
		"changes.since":         {{"changes.since", "native:issue", "observe"}},
		"changes.page":          nil,
	}
	if len(fixtures) != 14 || len(expected) != 14 {
		t.Fatal("closed method matrix size drifted")
	}
	for method, want := range expected {
		params, ok := fixtures[method]
		if !ok {
			t.Fatalf("missing method %s", method)
		}
		request := Request{ID: "r", Method: method, Params: params}
		got, err := requestAccess(request)
		if want == nil {
			expectGrantError(t, err, "unsupported_capability")
		} else if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: scope %v error %v want %v", method, got, err, want)
		}
	}
	for kind, want := range map[string]string{"record.open": "create", "record.revise": "revise", "record.convert": "revise", "record.resolve": "resolve"} {
		params := fixtures["artifact.mutate"]
		params["kind"] = kind
		got, e := requestAccess(Request{Method: "artifact.mutate", Params: params})
		if e != nil || len(got) != 1 || got[0].Action != want {
			t.Fatalf("%s must require %s: %v %v", kind, want, got, e)
		}
	}
	for intent, want := range map[string]string{"open": "create", "revise": "revise", "resolve": "resolve", "comment": "create"} {
		params := fixtures["resource.mutate"]
		params["intent"] = intent
		got, e := requestAccess(Request{Method: "resource.mutate", Params: params})
		if e != nil || len(got) != 1 || got[0].Action != want {
			t.Fatalf("%s must require %s: %v %v", intent, want, got, e)
		}
	}
}

func TestSessionPartialControlKeepsThirtySecondCap(t *testing.T) {
	input, output, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer input.Close()
	defer output.Close()
	reader := &sessionReader{file: input}
	reader.setDeadline(time.Now().Add(MutationBudget))
	done := make(chan frameEvent, 1)
	started := time.Now()
	go func() { done <- reader.read() }()
	if _, e = output.Write([]byte{0}); e != nil {
		t.Fatal(e)
	}
	// Reapplying a long remaining mutation deadline must preserve the partial
	// frame's thirty-second cap, rather than resetting it to 240 seconds.
	time.Sleep(10 * time.Millisecond)
	reader.setDeadline(time.Now().Add(MutationBudget))
	select {
	case event := <-done:
		if d := time.Since(started); d < 29*time.Second || d > 32*time.Second {
			t.Fatalf("partial control deadline %s", d)
		}
		expectGrantError(t, event.err, "cancelled")
	case <-time.After(32 * time.Second):
		t.Fatal("partial control inherited mutation deadline")
	}
}

// Hold the actual monitor check across a valid final response and child exit.
// The CLI's Recheck reports cancellation when its supplied context is cancelled;
// only the host's intentional shutdown should be invisible to a clean result.
func TestSessionInFlightRecheckAtShutdown(t *testing.T) {
	for _, mode := range []string{"clean", "revoked", "repository-error", "parent-cancel", "independent-cancel", "mutation-parent-cancel"} {
		t.Run(mode, func(t *testing.T) {
			asset := "clean-shutdown"
			if mode == "mutation-parent-cancel" {
				asset = "mutation-shutdown"
			}
			cfg, pid := sessionFixture(t, asset)
			parent, parentCancel := context.WithCancel(context.Background())
			defer parentCancel()
			entered, checked := make(chan struct{}), make(chan struct{})
			original := cfg.Recheck
			cfg.Recheck = func(ctx context.Context, binding Binding) (Registration, EnablementEntry, error) {
				// Startup and request checks have deadlines; the periodic monitor does not.
				if _, bounded := ctx.Deadline(); bounded {
					return original(ctx, binding)
				}
				close(entered)
				defer close(checked)
				if mode != "independent-cancel" {
					<-ctx.Done()
				}
				switch mode {
				case "revoked":
					return Registration{}, EnablementEntry{}, failure("denied_access", "module epoch changed")
				case "repository-error":
					return Registration{}, EnablementEntry{}, failure("repository_error", "cannot validate registration")
				default:
					return Registration{}, EnablementEntry{}, failure("cancelled", "host validation cancelled")
				}
			}
			cfg.Dispatch = func(ctx context.Context, call Invocation) (any, error) {
				if e := AuthorizeRequest(call); e != nil {
					return nil, e
				}
				select {
				case <-entered:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				if mode == "parent-cancel" || mode == "mutation-parent-cancel" {
					parentCancel()
				}
				if mode == "parent-cancel" || mode == "mutation-parent-cancel" || mode == "independent-cancel" {
					<-ctx.Done()
					return nil, ctx.Err()
				}
				return map[string]any{"items": []any{}, "source_cut": testID, "next_cursor": nil}, nil
			}
			result, err := Run(parent, cfg)
			select {
			case <-checked:
			default:
				t.Fatal("returned without joining in-flight monitor check")
			}
			switch mode {
			case "clean":
				if err != nil || result.Outcome != "completed" || result.CompletedRequests != 1 || result.ExitCode != 0 {
					t.Fatalf("clean protocol completion became a monitor failure: result=%#v error=%v", result, err)
				}
			case "revoked":
				expectGrantError(t, err, "denied_access")
			case "repository-error":
				expectGrantError(t, err, "repository_error")
			case "mutation-parent-cancel":
				var public *Error
				if !errors.As(err, &public) || public.Code != "uncertain_operation" || public.Recovery != "inspect_operation" || !reflect.DeepEqual(public.Operation, map[string]any{"protocol": "hn/0", "actor": repeatActor(), "operation": "actual-key"}) {
					t.Fatalf("lost dispatched mutation uncertainty: %#v", err)
				}
			default:
				expectGrantError(t, err, "cancelled")
			}
			assertSessionPIDGone(t, pid)
		})
	}
}

func TestSessionInvocationIdentity(t *testing.T) {
	cfg, pid := sessionFixture(t, "identity-handoff")
	t.Setenv("HN_SESSION_OVERRIDE", "")
	var calls []Invocation
	cfg.Dispatch = func(ctx context.Context, call Invocation) (any, error) {
		if e := AuthorizeRequest(call); e != nil {
			return nil, e
		}
		calls = append(calls, call)
		return map[string]any{"items": []any{}, "source_cut": testID, "next_cursor": nil}, nil
	}
	entry, _, err := cfg.Store.Entry(cfg.Registration)
	if err != nil {
		t.Fatal(err)
	}
	expectedBinding := Binding{cfg.Registration.RegistrationID, cfg.Store.RepositoryID(), entry.Epoch}
	expectedRequest := Request{ID: "read", Method: "resource.read", Params: map[string]any{"family": "issue", "id": nil, "limit": float64(20), "cursor": nil}}
	ids := []string{}
	for i := 0; i < 2; i++ {
		result, err := Run(context.Background(), cfg)
		if err != nil || result.CompletedRequests != 2 || result.Outcome != "completed" {
			t.Fatalf("session %d: %#v %v", i, result, err)
		}
		assertSessionPIDGone(t, pid)
		if len(calls) != i+1 {
			t.Fatalf("expected one real dispatch per session, got %d", len(calls))
		}
		call := calls[i]
		if call.Binding != expectedBinding || !reflect.DeepEqual(call.Request, expectedRequest) || !reflect.DeepEqual(call.Registration, cfg.Registration) || !reflect.DeepEqual(call.Enablement, entry) {
			t.Fatalf("dispatch handoff changed request/registration/enablement/binding: %#v", call)
		}
		data, err := os.ReadFile(pid + ".session")
		if err != nil {
			t.Fatal(err)
		}
		var observed struct {
			Hello struct {
				Hello struct {
					SessionID string `json:"session_id"`
				} `json:"hello"`
			} `json:"hello"`
			Capabilities struct {
				Data struct {
					Session Session `json:"session"`
				} `json:"data"`
			} `json:"capabilities"`
			Response struct {
				OK bool `json:"ok"`
			} `json:"response"`
		}
		if json.Unmarshal(data, &observed) != nil || !observed.Response.OK {
			t.Fatalf("actual child observation %s", data)
		}
		// Reflection lets this assertion execute against the old Invocation without
		// turning the missing live handoff into an unrelated compilation failure.
		field := reflect.ValueOf(call).FieldByName("SessionID")
		id := ""
		if !field.IsValid() || field.Kind() != reflect.String {
			t.Error("Invocation.SessionID missing from live Run dispatch")
		} else {
			id = field.String()
		}
		if len(id) != 32 || id != result.Session.SessionID || id != observed.Hello.Hello.SessionID || id != observed.Capabilities.Data.Session.SessionID {
			t.Errorf("session %d dispatch identity %q differs from actual result/hello/capabilities %q/%q/%q", i, id, result.Session.SessionID, observed.Hello.Hello.SessionID, observed.Capabilities.Data.Session.SessionID)
		}
		ids = append(ids, id)
	}
	if ids[0] == ids[1] {
		t.Errorf("distinct sessions with identical binding reused identity %q", ids[0])
	}
	for _, location := range []string{"request", "params"} {
		t.Setenv("HN_SESSION_OVERRIDE", location)
		_, err := Run(context.Background(), cfg)
		expectGrantError(t, err, "invalid_input")
		assertSessionPIDGone(t, pid)
		if len(calls) != 2 {
			t.Fatal("caller-selected session identity reached dispatch")
		}
		data, err := os.ReadFile(pid + ".request")
		if err != nil {
			t.Fatal(err)
		}
		var request map[string]any
		if json.Unmarshal(data, &request) != nil {
			t.Fatalf("actual request marker %s", data)
		}
		target := request
		if location == "params" {
			target = request["params"].(map[string]any)
		}
		if target["session_id"] != "caller-selected" {
			t.Fatalf("missing actual override control %s", data)
		}
	}
}

// A real child consumes each complete, correlated successful response before
// sending the next business request through the existing Run/Dispatch seam.
func TestSessionSequentialResponses(t *testing.T) {
	for _, mode := range []string{"sequential-response", "sequential-exit"} {
		t.Run(mode, func(t *testing.T) {
			cfg, pid := sessionFixture(t, mode)
			calls := []string{}
			cfg.Dispatch = func(ctx context.Context, call Invocation) (any, error) {
				if err := AuthorizeRequest(call); err != nil {
					return nil, err
				}
				calls = append(calls, call.Request.ID)
				return map[string]any{"items": []any{}, "source_cut": testID, "next_cursor": nil}, nil
			}
			result, err := Run(context.Background(), cfg)
			want := []string{"sequence-0", "sequence-1", "sequence-2", "sequence-3"}
			if mode == "sequential-exit" {
				want = want[:1]
			}
			if err != nil || result.CompletedRequests != len(want) || result.Outcome != "completed" || !reflect.DeepEqual(calls, want) {
				t.Fatalf("sequential peer refused: result=%#v calls=%v error=%v", result, calls, err)
			}
			assertSessionPIDGone(t, pid)
			data, err := os.ReadFile(pid + ".responses")
			if err != nil {
				t.Fatal(err)
			}
			var responses []map[string]any
			if json.Unmarshal(data, &responses) != nil || len(responses) != len(want) {
				t.Fatalf("missing complete child response observations: %s", data)
			}
			for i, response := range responses {
				if response["request_id"] != want[i] || response["ok"] != true {
					t.Fatalf("uncorrelated or failed response %d: %v", i, response)
				}
			}
		})
	}
}

func TestSessionPrematureSecondHasOnlyOneWriter(t *testing.T) {
	cfg, pid := sessionFixture(t, "premature-second")
	calls := 0
	settled := make(chan struct{})
	cfg.Dispatch = func(ctx context.Context, call Invocation) (any, error) {
		calls++
		<-ctx.Done()
		defer close(settled)
		return nil, ctx.Err()
	}
	_, err := Run(context.Background(), cfg)
	var public *Error
	if !errors.As(err, &public) || public.Code != "uncertain_operation" || public.Recovery != "inspect_operation" || !reflect.DeepEqual(public.Operation, map[string]any{"protocol": "hn/0", "actor": repeatActor(), "operation": "actual-key"}) {
		t.Fatalf("lost premature mutation uncertainty: %v", err)
	}
	if calls != 1 {
		t.Fatalf("premature second dispatched %d writers", calls)
	}
	select {
	case <-settled:
	default:
		t.Fatal("writer did not settle")
	}
	assertSessionPIDGone(t, pid)
}

func TestSessionBlockedResponseControls(t *testing.T) {
	for _, mode := range []string{"cancel", "wrong-cancel", "second", "timeout", "eof"} {
		t.Run(mode, func(t *testing.T) {
			cfg, pid := sessionFixture(t, "blocked-response-"+mode)
			calls := 0
			cfg.Dispatch = func(ctx context.Context, call Invocation) (any, error) {
				calls++
				comments := make([]any, 32)
				for i := range comments {
					comments[i] = map[string]any{"id": testID, "actor": repeatActor(), "body": strings.Repeat("x", 65536)}
				}
				item := map[string]any{"protocol": "hn/0", "family": "issue", "id": testID, "creator": repeatActor(), "heads": []any{}, "conflict": false, "state": nil, "comments": comments}
				return map[string]any{"items": []any{item}, "source_cut": testID, "next_cursor": nil}, nil
			}
			started := time.Now()
			_, err := Run(context.Background(), cfg)
			code := "cancelled"
			if mode == "second" || mode == "wrong-cancel" {
				code = "invalid_input"
			}
			if mode == "eof" {
				code = "unavailable_module"
			}
			expectGrantError(t, err, code)
			if calls != 1 {
				t.Fatalf("response control dispatched %d writers", calls)
			}
			if time.Since(started) > 4*time.Second {
				t.Fatal("blocked response lost finite budget/cleanup")
			}
			assertSessionPIDGone(t, pid)
		})
	}
}

func TestSessionMutationLossAndSettledRefusal(t *testing.T) {
	for _, mode := range []string{"response-loss", "refusal-response-loss", "unclassified-response-loss", "interrupted-refusal"} {
		t.Run(mode, func(t *testing.T) {
			asset := "lost-response"
			if mode == "interrupted-refusal" {
				asset = "premature-second"
			}
			cfg, pid := sessionFixture(t, asset)
			calls := 0
			known := &Error{Origin: "native", Code: "concurrent_update", Message: "native writer refused", Recovery: "retry_identical"}
			cfg.Dispatch = func(ctx context.Context, call Invocation) (any, error) {
				calls++
				if mode == "interrupted-refusal" {
					<-ctx.Done()
					return nil, known
				}
				if mode == "refusal-response-loss" {
					return nil, known
				}
				if mode == "unclassified-response-loss" {
					return nil, errors.New("unclassified dispatch failure")
				}
				return mutationFixture(), nil
			}
			_, err := Run(context.Background(), cfg)
			if mode == "interrupted-refusal" || mode == "refusal-response-loss" {
				if err != known {
					t.Fatalf("definitive refusal replaced: %v", err)
				}
			} else {
				var public *Error
				if !errors.As(err, &public) || public.Code != "uncertain_operation" || public.Recovery != "inspect_operation" || !reflect.DeepEqual(public.Operation, map[string]any{"protocol": "hn/0", "actor": repeatActor(), "operation": "actual-key"}) {
					t.Fatalf("response loss discarded mutation identity: %v", err)
				}
			}
			if calls != 1 {
				t.Fatalf("expected one writer, got %d", calls)
			}
			assertSessionPIDGone(t, pid)
		})
	}
}

// This peer records the actual TERM delivered to a running optional process.
func TestSessionCadenceAsset(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "--cadence-root" {
			continue
		}
		root := os.Args[i+1]
		path := filepath.Join(root, strconv.Itoa(os.Getpid()))
		term := make(chan os.Signal, 1)
		signal.Notify(term, syscall.SIGTERM)
		_ = assetRead()
		if os.WriteFile(path+".ready", []byte(strconv.FormatInt(time.Now().UnixNano(), 10)), 0600) != nil {
			os.Exit(70)
		}
		assetWrite(map[string]any{"protocol": HostProtocol, "ready": true})
		<-term
		if os.WriteFile(path+".term", []byte(strconv.FormatInt(time.Now().UnixNano(), 10)), 0600) != nil {
			os.Exit(71)
		}
		os.Exit(0)
	}
}

func sessionCadenceFixture(t *testing.T) (SessionConfig, string) {
	t.Helper()
	cfg, _ := sessionFixture(t, "idle")
	root := t.TempDir()
	reg, err := NewRegistration(cfg.Registration.Manifest, cfg.Registration.Executable, []string{"-test.run=^TestSessionCadenceAsset$", "--", "--cadence-root", root})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = Register(filepath.Join(t.TempDir(), "extensions", "registry.json"), reg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = cfg.Store.Set(reg, true, []Grant{{"native:issue", []string{"read", "create"}}}); err != nil {
		t.Fatal(err)
	}
	cfg.Registration = reg
	cfg.Dispatch = func(context.Context, Invocation) (any, error) {
		return nil, failure("invalid_input", "idle cadence peer unexpectedly requested dispatch")
	}
	cfg.Recheck = func(ctx context.Context, binding Binding) (Registration, EnablementEntry, error) {
		entry, _, err := cfg.Store.Entry(reg)
		if err == nil {
			err = Authorize(reg, entry, binding, Access{Method: "capabilities.list"}, false)
		}
		return reg, entry, err
	}
	return cfg, root
}

func sessionCadenceReady(t *testing.T, root string, count int) []string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		paths, err := filepath.Glob(filepath.Join(root, "*.ready"))
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) == count {
			return paths
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d real hosts became ready", len(paths), count)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSessionRevocationCadenceHeadroom(t *testing.T) {
	cfg, root := sessionCadenceFixture(t)
	original := cfg.Recheck
	var checked time.Time
	cfg.Recheck = func(ctx context.Context, binding Binding) (Registration, EnablementEntry, error) {
		if _, startupOrRequest := ctx.Deadline(); !startupOrRequest {
			checked = time.Now()
			// Reserve a known, bounded fresh-check cost. The old 250ms initial
			// sleep cannot both do this work and deliver TERM within 250ms.
			time.Sleep(60 * time.Millisecond)
		}
		reg, entry, err := original(ctx, binding)
		if _, bounded := ctx.Deadline(); !bounded && err == nil {
			if _, e := os.Stat(filepath.Join(root, "revoked")); e == nil {
				err = failure("denied_access", "fresh recheck observed revoked registration")
			}
		}
		return reg, entry, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := Run(ctx, cfg); done <- err }()
	joined := false
	defer func() {
		cancel()
		if !joined {
			<-done
		}
	}()
	ready := sessionCadenceReady(t, root, 1)[0]
	revoked := time.Now()
	if err := os.WriteFile(filepath.Join(root, "revoked"), []byte("revoked"), 0600); err != nil {
		t.Fatal(err)
	}
	err := <-done
	joined = true
	expectGrantError(t, err, "denied_access")
	raw, err := os.ReadFile(strings.TrimSuffix(ready, ".ready") + ".term")
	if err != nil {
		t.Fatal("real child never acknowledged TERM", err)
	}
	ns, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	term := time.Unix(0, ns)
	t.Logf("revoked=%d fresh_check=%d actual_TERM=%d notice=%s", revoked.UnixNano(), checked.UnixNano(), ns, term.Sub(revoked))
	if checked.Before(revoked) || term.Sub(revoked) > RevocationInterval {
		t.Fatalf("active host missed unchanged 250ms revocation bound: %s", term.Sub(revoked))
	}
	pid, err := strconv.Atoi(filepath.Base(strings.TrimSuffix(ready, ".ready")))
	if err != nil || syscall.Kill(pid, 0) != syscall.ESRCH {
		t.Fatal("Run returned without reaping its actual host", pid, err)
	}
}

// Bounded development evidence: the real monitor invokes fresh Git and hashes
// the executable on every check, for one and four simultaneously active hosts.
// This does not replace native package H14 or default/RSS qualification.
func TestSessionActiveRecheckIO(t *testing.T) {
	for _, count := range []int{1, 4} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			cfg, root := sessionCadenceFixture(t)
			if out, err := exec.Command("git", "-C", cfg.Directory, "init", "--quiet").CombinedOutput(); err != nil {
				t.Fatal(err, string(out))
			}
			original := cfg.Recheck
			var mu sync.Mutex
			checks, gitBytes := 0, 0
			var ioTime time.Duration
			cfg.Recheck = func(ctx context.Context, binding Binding) (Registration, EnablementEntry, error) {
				start := time.Now()
				out, err := exec.CommandContext(ctx, "git", "-C", cfg.Directory, "rev-parse", "--absolute-git-dir").Output()
				if err != nil {
					return Registration{}, EnablementEntry{}, err
				}
				if _, _, err = cfg.Registration.Availability(); err != nil {
					return Registration{}, EnablementEntry{}, err
				}
				if _, bounded := ctx.Deadline(); !bounded {
					mu.Lock()
					checks++
					gitBytes += len(out)
					ioTime += time.Since(start)
					mu.Unlock()
				}
				return original(ctx, binding)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, count)
			pending := 0
			defer func() {
				cancel()
				for i := 0; i < pending; i++ {
					<-done
				}
			}()
			for i := 0; i < count; i++ {
				pending++
				go func() { _, err := Run(ctx, cfg); done <- err }()
				sessionCadenceReady(t, root, i+1)
			}
			ready := sessionCadenceReady(t, root, count)
			start := time.Now()
			time.Sleep(800 * time.Millisecond)
			cancel()
			for i := 0; i < count; i++ {
				err := <-done
				pending--
				expectGrantError(t, err, "cancelled")
			}
			info, err := os.Stat(cfg.Registration.Executable)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("active_hosts=%d observed=%s monitor_checks=%d actual_Git_stdout_bytes=%d executable_bytes_hashed=%d cumulative_Git_and_hash_time=%s", count, time.Since(start), checks, gitBytes, int64(checks)*info.Size(), ioTime)
			if checks < count*2 {
				t.Fatal("active monitor did not perform repeated real Git/hash checks")
			}
			for _, path := range ready {
				pid, err := strconv.Atoi(filepath.Base(strings.TrimSuffix(path, ".ready")))
				if err != nil || syscall.Kill(pid, 0) != syscall.ESRCH {
					t.Fatal("active I/O host not reaped", pid, err)
				}
			}
		})
	}
}
