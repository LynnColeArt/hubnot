package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	pipelineVersion = "hn.pipeline/0"
	maxPipelineSize = 1 << 20
	maxArchiveSize  = 256 << 20
	maxRunLogSize   = 1 << 20
	defaultTimeout  = 300
	maxStepTimeout  = 3600
)

type PipelineDefinition struct {
	Version string         `json:"version"`
	Steps   []PipelineStep `json:"steps"`
}

type PipelineStep struct {
	Name             string   `json:"name"`
	Command          string   `json:"command"`
	Args             []string `json:"args,omitempty"`
	WorkingDirectory string   `json:"workingDirectory,omitempty"`
	TimeoutSeconds   int      `json:"timeoutSeconds,omitempty"`
}

type executionResult struct {
	Outcome    string
	ExitCode   int
	DurationMS int64
	Log        []byte
}

func validPipelineName(name string) bool {
	if name == "" || len(name) > 64 || name == "." || name == ".." {
		return false
	}
	for _, character := range name {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func loadPipeline(commit, name string) (PipelineDefinition, []byte, string, error) {
	if !validPipelineName(name) {
		return PipelineDefinition{}, nil, "", fmt.Errorf("invalid pipeline name %q", name)
	}
	gitDir, err := requireGitRepository()
	if err != nil {
		return PipelineDefinition{}, nil, "", err
	}
	if err := replicationPendingError(gitDir, commit); err != nil {
		return PipelineDefinition{}, nil, "", err
	}
	path := ".hn/pipelines/" + name + ".json"
	encoded, err := gitOutput("show", commit+":"+path)
	if err != nil {
		return PipelineDefinition{}, nil, "", fmt.Errorf("pipeline %q does not exist at commit %s", name, shortOID(commit))
	}
	if len(encoded) > maxPipelineSize {
		return PipelineDefinition{}, nil, "", fmt.Errorf("pipeline %q exceeds %d bytes", name, maxPipelineSize)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var pipeline PipelineDefinition
	if err := decoder.Decode(&pipeline); err != nil {
		return PipelineDefinition{}, nil, "", fmt.Errorf("parse pipeline %q: %w", name, err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return PipelineDefinition{}, nil, "", fmt.Errorf("parse pipeline %q: %w", name, err)
	}
	if err := validatePipeline(pipeline); err != nil {
		return PipelineDefinition{}, nil, "", fmt.Errorf("pipeline %q: %w", name, err)
	}
	return pipeline, encoded, eventID(encoded), nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("contains more than one JSON value")
	}
	return err
}

func validatePipeline(pipeline PipelineDefinition) error {
	if pipeline.Version != pipelineVersion {
		return fmt.Errorf("unsupported version %q", pipeline.Version)
	}
	if len(pipeline.Steps) == 0 || len(pipeline.Steps) > 64 {
		return fmt.Errorf("must contain between 1 and 64 steps")
	}
	for index, step := range pipeline.Steps {
		if strings.TrimSpace(step.Name) == "" || strings.TrimSpace(step.Command) == "" {
			return fmt.Errorf("step %d requires a name and command", index+1)
		}
		if strings.ContainsRune(step.Command, '\x00') {
			return fmt.Errorf("step %d command contains a null byte", index+1)
		}
		if strings.ContainsAny(step.Command, `/\`) && !safeRelativePath(step.Command) {
			return fmt.Errorf("step %d command path escapes the checkout", index+1)
		}
		if len(step.Args) > 256 {
			return fmt.Errorf("step %d has too many arguments", index+1)
		}
		for _, argument := range step.Args {
			if strings.ContainsRune(argument, '\x00') || len(argument) > 16<<10 {
				return fmt.Errorf("step %d has an invalid argument", index+1)
			}
		}
		if !safeRelativeDirectory(step.WorkingDirectory) {
			return fmt.Errorf("step %d has an unsafe working directory", index+1)
		}
		if step.TimeoutSeconds < 0 || step.TimeoutSeconds > maxStepTimeout {
			return fmt.Errorf("step %d timeout must be between 0 and %d seconds", index+1, maxStepTimeout)
		}
	}
	return nil
}

func safeRelativeDirectory(path string) bool {
	if path == "" || path == "." {
		return true
	}
	cleaned := filepath.Clean(path)
	return !filepath.IsAbs(cleaned) && cleaned != ".." && !strings.HasPrefix(cleaned, ".."+string(filepath.Separator))
}

func safeRelativePath(path string) bool {
	if path == "" {
		return false
	}
	cleaned := filepath.Clean(path)
	return !filepath.IsAbs(cleaned) && cleaned != ".." && !strings.HasPrefix(cleaned, ".."+string(filepath.Separator))
}

func cmdRun(args []string) error {
	if len(args) == 0 {
		return usageError("usage: hn run <request|list|show|execute|logs>")
	}
	switch args[0] {
	case "request":
		return cmdRunRequest(args[1:])
	case "list":
		if len(args) != 1 {
			return usageError("usage: hn run list")
		}
		return cmdRunList()
	case "show":
		if len(args) != 2 {
			return usageError("usage: hn run show REQUEST")
		}
		return cmdRunShow(args[1])
	case "execute":
		return cmdRunExecute(args[1:])
	case "logs":
		if len(args) != 2 {
			return usageError("usage: hn run logs RESULT")
		}
		return cmdRunLogs(args[1])
	default:
		return fmt.Errorf("unknown run command %q", args[0])
	}
}

func cmdRunRequest(args []string) error {
	if len(args) != 2 {
		return usageError("usage: hn run request PROPOSAL PIPELINE")
	}
	if err := requireFullEventID(args[0]); err != nil {
		return err
	}
	if err := prepareShallowVerification(shallowVerificationScope{Operation: "run request", Subject: args[0], Pipeline: args[1]}); err != nil {
		return err
	}
	if err := guardShallowEventClosure("run request"); err != nil {
		return err
	}
	events, err := collectEvents()
	if err != nil {
		return err
	}
	proposal, err := resolveEventDependency("run request", args[0], shallowCandidateEvent, events)
	if err != nil {
		return err
	}
	if !isProposalKind(proposal.Event.Kind) {
		return fmt.Errorf("%s is not a proposal", shortID(proposal.ID))
	}
	if err := guardProposalDependencies("run request", proposal, events); err != nil {
		return err
	}
	if err := guardBasePolicy("run request", proposal.Event.Base, replicationProposal, proposal.ID); err != nil {
		return err
	}
	if err := requireProposalCode(proposal); err != nil {
		return err
	}
	if err := guardPipelineDefinition("run request", proposal.Event.Head, args[1], proposal.ID, ""); err != nil {
		return err
	}
	_, _, definition, err := loadPipeline(proposal.Event.Head, args[1])
	if err != nil {
		return err
	}
	identity, err := loadIdentity()
	if err != nil {
		return err
	}
	event, err := nextEvent(identity, "run.request")
	if err != nil {
		return err
	}
	event.Subject = proposal.ID
	event.Pipeline = args[1]
	event.Definition = definition
	event.Commit = proposal.Event.Head
	stored, err := appendEvent(event, identity)
	if err != nil {
		return err
	}
	fmt.Printf("Requested pipeline %s as run %s\n", oneLine(event.Pipeline), shortID(stored.ID))
	fmt.Printf("Commit: %s  Definition: %s\n", shortOID(event.Commit), shortID(event.Definition))
	fmt.Println("Recorded locally; run 'hn sync REMOTE' to publish this request so remote runners can see it.")
	return nil
}

func requireProposalCode(proposal *StoredEvent) error {
	head, exists, err := proposalHead(proposal.ID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("proposal code is unavailable; sync before requesting or executing a run")
	}
	if head != proposal.Event.Head {
		return fmt.Errorf("proposal code ref does not match the signed proposal")
	}
	return nil
}

func runRequests(events []StoredEvent) []StoredEvent {
	requests := make([]StoredEvent, 0)
	for _, stored := range events {
		if stored.Event.Kind == "run.request" {
			requests = append(requests, stored)
		}
	}
	return requests
}

func currentRunResults(events []StoredEvent, requestID string) []StoredEvent {
	byActor := make(map[string]StoredEvent)
	for _, stored := range events {
		if stored.Event.Kind != "run.result" || stored.Event.Subject != requestID {
			continue
		}
		previous, exists := byActor[stored.Event.Actor]
		if !exists || stored.Event.Sequence > previous.Event.Sequence {
			byActor[stored.Event.Actor] = stored
		}
	}
	results := make([]StoredEvent, 0, len(byActor))
	for _, stored := range byActor {
		results = append(results, stored)
	}
	sortStoredEvents(results)
	return results
}

func runResultCounts(results []StoredEvent) (passed, failed int) {
	for _, result := range results {
		if result.Event.Outcome == "passed" {
			passed++
		} else {
			failed++
		}
	}
	return passed, failed
}

func cmdRunList() error {
	if err := prepareShallowVerification(shallowVerificationScope{Operation: "run list"}); err != nil {
		return err
	}
	if err := guardShallowEventClosure("run list"); err != nil {
		return err
	}
	events, err := collectEvents()
	if err != nil {
		return err
	}
	requests := runRequests(events)
	if len(requests) == 0 {
		fmt.Println("No runs.")
		return nil
	}
	for _, request := range requests {
		results := currentRunResults(events, request.ID)
		passed, failed := runResultCounts(results)
		status := "pending"
		if passed > 0 || failed > 0 {
			status = fmt.Sprintf("passed=%d failed=%d", passed, failed)
		}
		fmt.Printf("%s  %-16s  %s  %s\n", shortID(request.ID), oneLine(request.Event.Pipeline), shortOID(request.Event.Commit), status)
	}
	return nil
}

func cmdRunShow(query string) error {
	if err := prepareShallowVerification(shallowVerificationScope{Operation: "run show", Subject: query}); err != nil {
		return err
	}
	if err := guardShallowEventClosure("run show"); err != nil {
		return err
	}
	events, err := collectEvents()
	if err != nil {
		return err
	}
	request, err := resolveEventDependency("run show", query, shallowRunRequest, events)
	if err != nil {
		return err
	}
	if request.Event.Kind != "run.request" {
		return fmt.Errorf("%s is not a run request", shortID(request.ID))
	}
	fmt.Printf("%s  pipeline %s\n", shortID(request.ID), oneLine(request.Event.Pipeline))
	fmt.Printf("Requested by %s at %s\n", oneLine(request.Event.ActorName), request.Event.Timestamp)
	fmt.Printf("Proposal: %s\nCommit: %s\nDefinition: %s\n", shortID(request.Event.Subject), request.Event.Commit, request.Event.Definition)
	results := currentRunResults(events, request.ID)
	if len(results) == 0 {
		fmt.Println("\nNo results.")
		return nil
	}
	fmt.Println("\nCurrent results:")
	for _, result := range results {
		fmt.Printf("\n%s: %s, exit %d, %dms, %s on %s (%s)\n", oneLine(result.Event.ActorName), result.Event.Outcome, result.Event.ExitCode, result.Event.DurationMS, result.Event.Backend, oneLine(result.Event.Platform), shortID(result.ID))
	}
	return nil
}

func cmdRunLogs(query string) error {
	if err := prepareShallowVerification(shallowVerificationScope{Operation: "run logs", Subject: query}); err != nil {
		return err
	}
	if err := guardShallowEventClosure("run logs"); err != nil {
		return err
	}
	events, err := collectEvents()
	if err != nil {
		return err
	}
	result, err := resolveEventDependency("run logs", query, shallowRunResult, events)
	if err != nil {
		return err
	}
	if result.Event.Kind != "run.result" {
		return fmt.Errorf("%s is not a run result", shortID(result.ID))
	}
	log, exists := result.Attachments["log.txt"]
	if !exists {
		return fmt.Errorf("result has no verified log")
	}
	fmt.Print(safeText(string(log)))
	if len(log) > 0 && log[len(log)-1] != '\n' {
		fmt.Println()
	}
	return nil
}

func cmdRunExecute(args []string) error {
	if len(args) < 1 {
		return usageError("usage: hn run execute REQUEST [--backend sandbox|host] [--allow-unsafe-host-execution] [--rerun]")
	}
	query := args[0]
	if err := requireFullEventID(query); err != nil {
		return err
	}
	flags := quietFlags("run execute")
	backendName := flags.String("backend", "sandbox", "execution backend: sandbox or host")
	allowHost := flags.Bool("allow-unsafe-host-execution", false, "execute untrusted repository code without isolation")
	rerun := flags.Bool("rerun", false, "replace this runner's current result")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("usage: hn run execute REQUEST [--backend sandbox|host] [--allow-unsafe-host-execution] [--rerun]")
	}
	backend, err := selectBackend(*backendName, *allowHost)
	if err != nil {
		return err
	}
	if err := prepareShallowVerification(shallowVerificationScope{Operation: "run execute", Subject: query}); err != nil {
		return err
	}
	if err := guardShallowEventClosure("run execute"); err != nil {
		return err
	}
	events, err := collectEvents()
	if err != nil {
		return err
	}
	request, err := resolveEventDependency("run execute", query, shallowRunRequest, events)
	if err != nil {
		return err
	}
	if request.Event.Kind != "run.request" {
		return fmt.Errorf("%s is not a run request", shortID(request.ID))
	}
	identity, err := loadIdentity()
	if err != nil {
		return err
	}
	stored, err := executeRunRequest(context.Background(), events, request, identity, backend, *rerun)
	if err != nil {
		return err
	}
	fmt.Printf("Recorded %s result %s with %s backend (exit %d, %dms)\n", stored.Event.Outcome, shortID(stored.ID), backend.Name(), stored.Event.ExitCode, stored.Event.DurationMS)
	return nil
}

func executeRunRequest(ctx context.Context, events []StoredEvent, request *StoredEvent, identity *Identity, backend executionBackend, rerun bool) (*StoredEvent, error) {
	proposal, err := resolveEvent(events, request.Event.Subject)
	if err != nil {
		return nil, fmt.Errorf("resolve run proposal: %w", err)
	}
	if !isProposalKind(proposal.Event.Kind) || proposal.Event.Head != request.Event.Commit {
		return nil, fmt.Errorf("run request does not match its signed proposal")
	}
	if err := guardProposalDependencies("run execute", proposal, events); err != nil {
		return nil, err
	}
	if err := guardPipelineDefinition("run execute", request.Event.Commit, request.Event.Pipeline, proposal.ID, request.Event.Definition); err != nil {
		return nil, err
	}
	if err := requireProposalCode(proposal); err != nil {
		return nil, err
	}
	pipeline, _, definition, err := loadPipeline(request.Event.Commit, request.Event.Pipeline)
	if err != nil {
		return nil, err
	}
	if definition != request.Event.Definition {
		return nil, fmt.Errorf("pipeline definition does not match the signed run request")
	}
	if !rerun {
		for _, result := range currentRunResults(events, request.ID) {
			if result.Event.Actor == identity.Actor {
				return nil, fmt.Errorf("this identity already produced result %s; pass --rerun to run again", shortID(result.ID))
			}
		}
	}

	result, err := executePipeline(ctx, request.Event.Commit, request.Event.Pipeline, pipeline, backend)
	if err != nil {
		return nil, err
	}
	event, err := nextEvent(identity, "run.result")
	if err != nil {
		return nil, err
	}
	event.Subject = request.ID
	event.Pipeline = request.Event.Pipeline
	event.Definition = request.Event.Definition
	event.Commit = request.Event.Commit
	event.Outcome = result.Outcome
	event.ExitCode = result.ExitCode
	event.DurationMS = result.DurationMS
	event.Log = eventID(result.Log)
	event.Backend = backend.Name()
	event.Platform = runtime.GOOS + "/" + runtime.GOARCH
	event.Runner = "hn/" + version
	stored, err := appendEventWithAttachments(event, identity, map[string][]byte{"log.txt": result.Log})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func executePipeline(parentContext context.Context, commit, pipelineName string, pipeline PipelineDefinition, backend executionBackend) (executionResult, error) {
	if err := backend.Available(); err != nil {
		return executionResult{}, err
	}
	root, err := extractCommit(commit)
	if err != nil {
		return executionResult{}, err
	}
	defer os.RemoveAll(root)
	home := filepath.Join(root, ".hn-runner-home")
	temp := filepath.Join(root, ".hn-runner-tmp")
	if err := os.MkdirAll(home, 0o700); err != nil {
		return executionResult{}, err
	}
	if err := os.MkdirAll(temp, 0o700); err != nil {
		return executionResult{}, err
	}

	logs := &cappedLog{limit: maxRunLogSize}
	started := time.Now()
	outcome := "passed"
	exitCode := 0
	environment := runnerEnvironment(home, temp, commit, os.Getenv("PATH"))
	if backend.Name() == "sandbox" {
		environment = runnerEnvironment("/home/hn", "/tmp", commit, sandboxPath())
	}
	fmt.Fprintf(logs, "Hubnot pipeline %s\nCommit %s\nBackend %s\n", pipelineName, commit, backend.Name())
	for index, step := range pipeline.Steps {
		fmt.Fprintf(logs, "\n[%d/%d] %s\n", index+1, len(pipeline.Steps), step.Name)
		timeout := step.TimeoutSeconds
		if timeout == 0 {
			timeout = defaultTimeout
		}
		ctx, cancel := context.WithTimeout(parentContext, time.Duration(timeout)*time.Second)
		err := backend.RunStep(ctx, root, step, environment, logs)
		if err != nil {
			if parentContext.Err() != nil {
				cancel()
				return executionResult{}, parentContext.Err()
			}
			outcome = "failed"
			exitCode = commandExitCode(err, ctx)
			fmt.Fprintf(logs, "\nStep failed: %v\n", err)
			cancel()
			break
		}
		cancel()
	}
	duration := time.Since(started).Milliseconds()
	return executionResult{Outcome: outcome, ExitCode: exitCode, DurationMS: duration, Log: logs.Bytes()}, nil
}

func runnerEnvironment(home, temp, commit, path string) []string {
	return []string{
		"PATH=" + path,
		"HOME=" + home,
		"TMPDIR=" + temp,
		"CI=true",
		"HN_COMMIT=" + commit,
		"LANG=C.UTF-8",
	}
}

func commandExitCode(err error, ctx context.Context) int {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return 124
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return 127
}

type cappedLog struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (log *cappedLog) Write(data []byte) (int, error) {
	originalLength := len(data)
	remaining := log.limit - log.buffer.Len()
	if remaining <= 0 {
		log.truncated = true
		return originalLength, nil
	}
	if len(data) > remaining {
		data = data[:remaining]
		log.truncated = true
	}
	_, _ = log.buffer.Write(data)
	return originalLength, nil
}

func (log *cappedLog) Bytes() []byte {
	result := append([]byte(nil), log.buffer.Bytes()...)
	if log.truncated {
		result = append(result, []byte("\n[log truncated by Hubnot]\n")...)
	}
	return result
}

func extractCommit(commit string) (string, error) {
	archive, err := gitOutput("archive", "--format=tar", commit)
	if err != nil {
		return "", fmt.Errorf("archive run commit: %w", err)
	}
	if len(archive) > maxArchiveSize {
		return "", fmt.Errorf("run archive exceeds %d bytes", maxArchiveSize)
	}
	root, err := os.MkdirTemp("", "hn-run-")
	if err != nil {
		return "", err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = os.RemoveAll(root)
		}
	}()
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read run archive: %w", err)
		}
		cleaned := filepath.Clean(header.Name)
		if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("unsafe archive path %q", header.Name)
		}
		target := filepath.Join(root, cleaned)
		switch header.Typeflag {
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || header.Size > maxArchiveSize {
				return "", fmt.Errorf("unsafe archive entry size for %q", header.Name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(header.Mode)&0o777)
			if err != nil {
				return "", err
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			closeErr := file.Close()
			if copyErr != nil {
				return "", copyErr
			}
			if closeErr != nil {
				return "", closeErr
			}
		default:
			return "", fmt.Errorf("unsupported archive entry %q", header.Name)
		}
	}
	succeeded = true
	return root, nil
}
