package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type runnerOptions struct {
	Remote          string
	Pipeline        string
	AcceptedActor   string
	Backend         executionBackend
	Interval        time.Duration
	ReportNoPending bool
}

func cmdRunner(args []string) error {
	if len(args) == 0 {
		return usageError("usage: hn runner <once|watch> --accept-pipeline NAME --accept-actor ACTOR")
	}
	switch args[0] {
	case "once":
		options, err := parseRunnerOptions("runner once", args[1:], false)
		if err != nil {
			return err
		}
		options.ReportNoPending = true
		_, err = runnerOnce(options)
		return err
	case "watch":
		options, err := parseRunnerOptions("runner watch", args[1:], true)
		if err != nil {
			return err
		}
		return runnerWatch(options)
	default:
		return fmt.Errorf("unknown runner command %q", args[0])
	}
}

func parseRunnerOptions(name string, args []string, watching bool) (runnerOptions, error) {
	flags := quietFlags(name)
	remote := flags.String("remote", "origin", "Git remote to synchronize")
	pipeline := flags.String("accept-pipeline", "", "only execute this pipeline name")
	actor := flags.String("accept-actor", "", "only execute requests signed by this actor")
	backendName := flags.String("backend", "sandbox", "execution backend: sandbox or host")
	allowHost := flags.Bool("allow-unsafe-host-execution", false, "permit an unsandboxed host backend")
	interval := flags.Duration("interval", 30*time.Second, "watch polling interval")
	if err := flags.Parse(args); err != nil {
		return runnerOptions{}, err
	}
	if flags.NArg() != 0 || !validPipelineName(*pipeline) || !validActorID(*actor) {
		return runnerOptions{}, usageError("runner requires --accept-pipeline NAME and a full --accept-actor ACTOR fingerprint")
	}
	if watching && *interval < time.Second {
		return runnerOptions{}, fmt.Errorf("runner watch interval must be at least 1s")
	}
	backend, err := selectBackend(*backendName, *allowHost)
	if err != nil {
		return runnerOptions{}, err
	}
	if _, err := loadIdentity(); err != nil {
		return runnerOptions{}, err
	}
	return runnerOptions{
		Remote:        *remote,
		Pipeline:      *pipeline,
		AcceptedActor: *actor,
		Backend:       backend,
		Interval:      *interval,
	}, nil
}

func validActorID(actor string) bool {
	decoded, err := hex.DecodeString(actor)
	return err == nil && len(decoded) == 32
}

func runnerOnce(options runnerOptions) (bool, error) {
	return runnerOnceContext(context.Background(), options)
}

func runnerOnceContext(ctx context.Context, options runnerOptions) (bool, error) {
	if err := runnerSync(options.Remote); err != nil {
		return false, err
	}
	events, err := collectEvents()
	if err != nil {
		return false, err
	}
	identity, err := loadIdentity()
	if err != nil {
		return false, err
	}
	var firstError error
	for index := range events {
		request := &events[index]
		if request.Event.Kind != "run.request" || request.Event.Pipeline != options.Pipeline || request.Event.Actor != options.AcceptedActor {
			continue
		}
		alreadyRan := false
		for _, result := range currentRunResults(events, request.ID) {
			if result.Event.Actor == identity.Actor {
				alreadyRan = true
				break
			}
		}
		if alreadyRan {
			continue
		}
		runnerLog("Runner accepted request %s from %s for pipeline %s", shortID(request.ID), shortID(request.Event.Actor), oneLine(request.Event.Pipeline))
		stored, err := executeRunRequest(ctx, events, request, identity, options.Backend, false)
		if err != nil {
			runnerError("skipped %s: %v", shortID(request.ID), err)
			if firstError == nil {
				firstError = err
			}
			continue
		}
		runnerLog("Runner recorded %s result %s with %s backend", stored.Event.Outcome, shortID(stored.ID), options.Backend.Name())
		if err := runnerSync(options.Remote); err != nil {
			return true, err
		}
		return true, nil
	}
	if firstError != nil {
		return false, firstError
	}
	if options.ReportNoPending {
		runnerLog("No pending %s requests from actor %s.", oneLine(options.Pipeline), shortID(options.AcceptedActor))
	}
	return false, nil
}

func runnerWatch(options runnerOptions) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runnerLog("Watching %s for pipeline %s requests from actor %s with %s backend", safeDiagnostic(options.Remote), oneLine(options.Pipeline), shortID(options.AcceptedActor), options.Backend.Name())
	for {
		ran, err := runnerOnceContext(ctx, options)
		if err != nil {
			if ctx.Err() != nil {
				runnerLog("Runner stopped.")
				return nil
			}
			runnerError("%v", err)
			ran = false
		}
		if ran {
			continue
		}
		timer := time.NewTimer(options.Interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			runnerLog("Runner stopped.")
			return nil
		case <-timer.C:
		}
	}
}

func runnerLog(format string, args ...any) {
	fmt.Printf("%s %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

func runnerError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s hn runner: %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

func runnerSync(remote string) error {
	started := time.Now()
	runnerLog("Runner sync started for %s", safeDiagnostic(remote))
	err := cmdSync([]string{remote})
	if err != nil {
		runnerLog("Runner sync failed for %s after %s", safeDiagnostic(remote), time.Since(started).Round(time.Millisecond))
	} else {
		runnerLog("Runner sync completed for %s after %s", safeDiagnostic(remote), time.Since(started).Round(time.Millisecond))
	}
	return err
}
