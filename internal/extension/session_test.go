package extension

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestSessionBudgetsMirrorFrozenContract(t *testing.T) {
	if StartupBudget != 10*time.Second || ReadBudget != 30*time.Second || MutationBudget != 240*time.Second || StderrLimit != 65536 || SessionLimit != 4 || RevocationInterval != 250*time.Millisecond || TerminateGrace != 2*time.Second || ExitBudget != 5*time.Second {
		t.Fatal("host-v1 frozen lifecycle bounds drifted")
	}
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
