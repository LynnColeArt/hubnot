package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func init() { NativeResourceCommands[ResourceWorkflow] = cmdWorkflow }

const workflowHelp = `Native signed workflow definitions; foreign source documents stay inert.

  hn workflow open --input FILE|- [--actor ACTOR] [--operation KEY] [--json]
  hn workflow revise|resolve FULL_ID --expect FULL_REV ... --input FILE|- [--actor ACTOR] [--operation KEY] [--json]
  hn workflow list [--limit 1..200] [--cursor TOKEN] [--json]
  hn workflow show|heads|history ID [--limit 1..200] [--cursor TOKEN] [--json]
  hn workflow operation --actor ACTOR --operation KEY [--json]
  hn workflow materialize FULL_ID [--expect FULL_REV] [--overwrite] [--json]
  hn workflow request FULL_ID --proposal FULL_PROPOSAL [--expect FULL_REV] [--actor ACTOR] [--json]

Full-state input uses schema hn.workflow/1 and an optional existing hn.pipeline/0
pipeline. Foreign source_documents preserve exact UTF-8 content and its digest;
source YAML never executes. Enabled native definitions are runnable only through
the existing explicitly invoked, governed runner path. Disabled, foreign-only or
conflicted workflows cannot materialize or request a run.

Materialize writes canonical compact JSON to .hn/pipelines/<name>.json in the
working tree. It refuses symlinks and differing files unless --overwrite is
explicit. It makes no commit, signed event, network request or execution.
Request requires exact canonical definition bytes at the immutable proposal head;
it appends an ordinary run.request using existing policy/code/dependency guards.
Its signed proposal commit and pipeline digest keep their existing authority.
The selected native workflow revision is reported in output, without adding a
new signed authority field. Request never executes the pipeline. Explicit runner
or run commands retain their existing policy and isolation requirements.
`

func cmdWorkflow(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(workflowHelp)
		return nil
	}
	switch args[0] {
	case "show", "materialize", "request":
	default:
		return CmdResource(ResourceWorkflow, args)
	}
	options, err := parseWorkflowAction(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Print(workflowHelp)
		return nil
	}
	if err != nil {
		return renderWorkflowResult(nil, err, workflowMachineRequested(args))
	}
	var data any
	switch options.command {
	case "show":
		data, err = showNativeWorkflow(options)
	case "materialize":
		data, err = materializeNativeWorkflow(options)
	case "request":
		data, err = requestNativeWorkflow(options)
	}
	return renderWorkflowResult(data, err, options.machine)
}

type workflowAction struct {
	command, id, expected, actor, proposal, cursor string
	overwrite, machine                             bool
	limit                                          int
}

func workflowMachineRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--json" || arg == "--json=true" {
			return true
		}
	}
	return false
}
func parseWorkflowAction(args []string) (workflowAction, error) {
	options := workflowAction{command: args[0], limit: 50}
	flags := flag.NewFlagSet("workflow "+options.command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&options.machine, "json", false, "")
	if options.command == "show" {
		flags.IntVar(&options.limit, "limit", 50, "")
		flags.StringVar(&options.cursor, "cursor", "", "")
	} else {
		flags.StringVar(&options.expected, "expect", "", "")
		if options.command == "materialize" {
			flags.BoolVar(&options.overwrite, "overwrite", false, "")
		} else {
			flags.StringVar(&options.proposal, "proposal", "", "")
			flags.StringVar(&options.actor, "actor", "", "")
		}
	}
	rest := args[1:]
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		options.id = rest[0]
		rest = rest[1:]
	}
	if err := flags.Parse(rest); err != nil {
		return options, err
	}
	if flags.NArg() != 0 || options.id == "" {
		return options, usageError("workflow action requires one resource ID before flags; see 'hn workflow help'")
	}
	if options.command != "show" {
		if err := requireFullEventID(options.id); err != nil {
			return options, err
		}
		if options.expected != "" {
			if err := requireFullEventID(options.expected); err != nil {
				return options, err
			}
		}
	}
	if options.command == "request" {
		if err := requireFullEventID(options.proposal); err != nil {
			return options, fmt.Errorf("workflow request requires --proposal FULL_ID: %w", err)
		}
		if options.actor != "" && !validActorFingerprint(options.actor) {
			return options, fmt.Errorf("workflow request --actor requires a full actor fingerprint")
		}
	}
	return options, nil
}
func renderWorkflowResult(data any, resultErr error, machine bool) error {
	if machine {
		envelope := map[string]any{"ok": resultErr == nil, "data": data}
		if resultErr != nil {
			code := "workflow_error"
			var typed *issueCommandError
			if errors.As(resultErr, &typed) {
				code = typed.Code
			}
			envelope["error"] = map[string]string{"code": code, "message": resultErr.Error()}
		}
		if err := json.NewEncoder(os.Stdout).Encode(envelope); err != nil {
			return err
		}
	} else if resultErr == nil {
		raw, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
	}
	return resultErr
}

type workflowShow struct {
	*ResourceView
	Status              string `json:"status"`
	Runnable            bool   `json:"runnable"`
	NativeRevision      string `json:"native_revision,omitempty"`
	DefinitionDigest    string `json:"definition_digest,omitempty"`
	SourceDocumentCount int    `json:"source_document_count"`
}

func showNativeWorkflow(options workflowAction) (*workflowShow, error) {
	data, err := queryResource(ResourceWorkflow, "show", options.id, "", "", options.limit, options.cursor)
	if err != nil {
		return nil, err
	}
	view := data.(*ResourceView)
	shown := &workflowShow{ResourceView: view, Status: "conflicted"}
	if view.Conflict {
		return shown, nil
	}
	var state WorkflowState
	if err = DecodeResourceState(ResourceWorkflow, view.State, &state); err != nil {
		return nil, err
	}
	shown.NativeRevision = view.Heads[0].ID
	shown.SourceDocumentCount = len(state.SourceDocuments)
	switch {
	case state.Pipeline == nil && len(state.SourceDocuments) > 0:
		shown.Status = "inert-source"
	case state.Pipeline == nil:
		shown.Status = "missing-native-definition"
	case !state.Enabled:
		shown.Status = "disabled"
	default:
		shown.Status = "native-runnable"
		shown.Runnable = true
	}
	if state.Pipeline != nil {
		raw, _, err := nativeWorkflowDefinition(state)
		if err != nil {
			return nil, err
		}
		shown.DefinitionDigest = eventID(raw)
	}
	return shown, nil
}

type workflowSelection struct {
	ID, Revision string
	State        WorkflowState
	Events       []StoredEvent
}

func selectNativeWorkflow(id, expected string) (*workflowSelection, error) {
	events, _, err := collectIssueEvents()
	if err != nil {
		return nil, err
	}
	catalog, err := buildGovernedResourceCatalog(events)
	if err != nil {
		return nil, err
	}
	view := catalog.Resources[id]
	if view == nil || view.Family != ResourceWorkflow {
		return nil, issueFailure("not_found", errors.New("native workflow opening was not found"))
	}
	if view.Conflict || len(view.Heads) != 1 {
		return nil, issueFailure("resource_conflict", errors.New("workflow has concurrent heads; inspect and resolve explicitly"))
	}
	revision := view.Heads[0].ID
	if expected != "" && expected != revision {
		return nil, issueFailure("stale_revision", errors.New("expected workflow revision is no longer the current head"))
	}
	var state WorkflowState
	if err = DecodeResourceState(ResourceWorkflow, view.State, &state); err != nil {
		return nil, err
	}
	return &workflowSelection{id, revision, state, events}, nil
}
func nativeWorkflowDefinition(state WorkflowState) ([]byte, string, error) {
	if state.Pipeline == nil {
		return nil, "", fmt.Errorf("workflow has no native pipeline; preserved foreign source is inert and needs explicit mapping")
	}
	if err := validatePipeline(*state.Pipeline); err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(state.Pipeline)
	if err != nil {
		return nil, "", err
	}
	if len(raw) > maxPipelineSize {
		return nil, "", fmt.Errorf("native workflow pipeline exceeds 1 MiB")
	}
	if !validPipelineName(state.Name) {
		return nil, "", fmt.Errorf("invalid workflow pipeline name")
	}
	return raw, eventID(raw), nil
}
func runnableWorkflowDefinition(state WorkflowState) ([]byte, string, error) {
	raw, digest, err := nativeWorkflowDefinition(state)
	if err != nil {
		return nil, "", err
	}
	if !state.Enabled {
		return nil, "", fmt.Errorf("workflow is disabled; revise it explicitly before materializing or requesting a run")
	}
	return raw, digest, nil
}

type workflowMaterialization struct {
	WorkflowID       string `json:"workflow_id"`
	NativeRevision   string `json:"native_revision"`
	Path             string `json:"path"`
	DefinitionDigest string `json:"definition_digest"`
	Changed          bool   `json:"changed"`
}

func materializeNativeWorkflow(options workflowAction) (*workflowMaterialization, error) {
	selected, err := selectNativeWorkflow(options.id, options.expected)
	if err != nil {
		return nil, err
	}
	raw, digest, err := runnableWorkflowDefinition(selected.State)
	if err != nil {
		return nil, err
	}
	repository, err := gitText("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("workflow materialization requires a Git working tree: %w", err)
	}
	root, err := os.OpenRoot(repository)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	hn, err := workflowDirectory(root, ".hn")
	if err != nil {
		return nil, err
	}
	defer hn.Close()
	pipelines, err := workflowDirectory(hn, "pipelines")
	if err != nil {
		return nil, err
	}
	defer pipelines.Close()
	filename := selected.State.Name + ".json"
	changed, err := writeWorkflowDefinition(pipelines, filename, raw, options.overwrite)
	if err != nil {
		return nil, err
	}
	return &workflowMaterialization{selected.ID, selected.Revision, ".hn/pipelines/" + filename, digest, changed}, nil
}

// Each opened directory is anchored independently. Lstat + SameFile rejects
// symlink substitutions; os.Root also confines all subsequent operations even
// if an untrusted path is replaced while the command is running.
func workflowDirectory(parent *os.Root, name string) (*os.Root, error) {
	info, err := parent.Lstat(name)
	if os.IsNotExist(err) {
		if err = parent.Mkdir(name, 0755); err != nil && !os.IsExist(err) {
			return nil, err
		}
		info, err = parent.Lstat(name)
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("workflow path %s must be a real directory, not a symlink", name)
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	opened, err := child.Stat(".")
	current, currentErr := parent.Lstat(name)
	if err != nil || currentErr != nil || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, opened) || !os.SameFile(info, current) {
		child.Close()
		return nil, fmt.Errorf("workflow directory changed during materialization")
	}
	return child, nil
}
func readWorkflowDestination(directory *os.Root, name string) ([]byte, bool, error) {
	info, err := directory.Lstat(name)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("workflow destination must be a regular file, not a symlink")
	}
	file, err := directory.Open(name)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, false, fmt.Errorf("workflow destination changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxPipelineSize+1))
	if err != nil {
		return nil, false, err
	}
	current, err := directory.Lstat(name)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(info, current) {
		return nil, false, fmt.Errorf("workflow destination changed while reading")
	}
	return raw, true, nil
}
func writeWorkflowDefinition(directory *os.Root, name string, raw []byte, overwrite bool) (bool, error) {
	existing, exists, err := readWorkflowDestination(directory, name)
	if err != nil {
		return false, err
	}
	if exists && bytes.Equal(existing, raw) {
		return false, nil
	}
	if exists && !overwrite {
		return false, fmt.Errorf("workflow destination differs; inspect it and use --overwrite explicitly")
	}
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		return false, err
	}
	temporary := ".hn-workflow-" + hex.EncodeToString(nonce) + ".tmp"
	file, err := directory.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return false, err
	}
	defer directory.Remove(temporary)
	if _, err = file.Write(raw); err != nil {
		file.Close()
		return false, err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return false, err
	}
	if err = file.Close(); err != nil {
		return false, err
	}
	// Rename replaces a directory entry rather than following a destination
	// symlink. Link publishes absent files atomically without replacing a racer.
	if overwrite {
		if current, checkErr := directory.Lstat(name); checkErr == nil && !current.Mode().IsRegular() {
			return false, fmt.Errorf("workflow destination became a non-regular file")
		} else if checkErr != nil && !os.IsNotExist(checkErr) {
			return false, checkErr
		}
		err = directory.Rename(temporary, name)
	} else {
		err = directory.Link(temporary, name)
	}
	if err != nil {
		return false, fmt.Errorf("publish workflow definition: %w", err)
	}
	return true, nil
}

type workflowRunRequest struct {
	WorkflowID       string `json:"workflow_id"`
	NativeRevision   string `json:"native_revision"`
	ProposalID       string `json:"proposal_id"`
	Commit           string `json:"commit"`
	Pipeline         string `json:"pipeline"`
	DefinitionDigest string `json:"definition_digest"`
	RequestID        string `json:"request_id"`
}

func requestNativeWorkflow(options workflowAction) (*workflowRunRequest, error) {
	selected, err := selectNativeWorkflow(options.id, options.expected)
	if err != nil {
		return nil, err
	}
	canonical, digest, err := runnableWorkflowDefinition(selected.State)
	if err != nil {
		return nil, err
	}
	// These are the existing cmdRunRequest guards, in the same order. This
	// specialized entry point adds exact native-workflow byte binding before the
	// existing request fact is signed; it adds no runner or policy authority.
	if err = prepareShallowVerification(shallowVerificationScope{Operation: "run request", Subject: options.proposal, Pipeline: selected.State.Name}); err != nil {
		return nil, err
	}
	if err = guardShallowEventClosure("run request"); err != nil {
		return nil, err
	}
	if err := contributionRequireProposal(options.proposal); err != nil {
		return nil, err
	}
	proposal, err := resolveEventDependency("run request", options.proposal, shallowCandidateEvent, selected.Events)
	if err != nil {
		return nil, err
	}
	if !isProposalKind(proposal.Event.Kind) {
		return nil, fmt.Errorf("selected subject is not a proposal")
	}
	if err = guardProposalDependencies("run request", proposal, selected.Events); err != nil {
		return nil, err
	}
	if err = guardBasePolicy("run request", proposal.Event.Base, replicationProposal, proposal.ID); err != nil {
		return nil, err
	}
	if err = requireProposalCode(proposal); err != nil {
		return nil, err
	}
	if err = guardPipelineDefinition("run request", proposal.Event.Head, selected.State.Name, proposal.ID, ""); err != nil {
		return nil, err
	}
	_, actual, actualDigest, err := loadPipeline(proposal.Event.Head, selected.State.Name)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(actual, canonical) || actualDigest != digest {
		return nil, fmt.Errorf("proposal pipeline bytes differ from the exact canonical signed workflow definition; materialize and commit the selected revision explicitly")
	}
	identity, err := loadIdentity()
	if err != nil {
		return nil, err
	}
	if options.actor != "" && options.actor != identity.Actor {
		return nil, issueFailure("actor_mismatch", errors.New("active signing actor differs from expected actor"))
	}
	event, err := nextEvent(identity, "run.request")
	if err != nil {
		return nil, err
	}
	event.Subject = proposal.ID
	event.Pipeline = selected.State.Name
	event.Definition = digest
	event.Commit = proposal.Event.Head
	stored, err := appendEvent(event, identity)
	if err != nil {
		return nil, err
	}
	return &workflowRunRequest{selected.ID, selected.Revision, proposal.ID, proposal.Event.Head, selected.State.Name, digest, stored.ID}, nil
}
