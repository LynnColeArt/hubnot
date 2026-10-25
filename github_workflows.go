//go:build !nogithub

package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"hubnot/internal/forge"
	"hubnot/internal/githubapi"
)

func init() {
	registration := forgeProviders["github"]
	registration.Capabilities = append(registration.Capabilities, forge.Capability{Resource: forge.ResourceWorkflow, Import: true, Export: true, MappedFields: []string{"exact source YAML at pinned commit (inert)", "explicit native pipeline file or exact generated subset mapping", "explicit branch/path/blob-SHA source restore", "explicit runner/trigger/checkout/python3 runtime export"}, Unsupported: []string{"general Actions YAML conversion", "secrets, services, matrices, arbitrary uses and expressions", "provider enabled/disabled state changes", "policy, runner authority and execution equivalence"}})
	forgeProviders["github"] = registration
	forgeResourceExporters[forgeResourceRoute{"github", forge.ResourceWorkflow}] = githubExportWorkflow
}
func githubWorkflowImportMappings(s forge.Selection) ([]string, *PipelineDefinition, bool, error) {
	used := []string{}
	var pipeline *PipelineDefinition
	enable := false
	for key := range s.Mappings {
		switch key {
		case "source-ref", "pipeline-file", "native", "enable":
			used = append(used, key)
		default:
			return nil, nil, false, errors.New("unsupported workflow import mapping")
		}
	}
	if val, ok := s.Mappings["enable"]; ok {
		if val != "true" && val != "false" {
			return nil, nil, false, errors.New("workflow enable mapping must be true or false")
		}
		enable = val == "true"
	}
	if file, ok := s.Mappings["pipeline-file"]; ok {
		if s.Key == "" || s.Mappings["native"] != "" {
			return nil, nil, false, errors.New("pipeline-file requires exactly one selected workflow and no native= mapping")
		}
		f, err := os.Open(file)
		if err != nil {
			return nil, nil, false, errors.New("cannot read explicitly selected native pipeline file")
		}
		defer f.Close()
		raw, err := readBoundedWorkflowMapping(f)
		if err != nil {
			return nil, nil, false, err
		}
		p := PipelineDefinition{}
		if strictResourceJSON(raw, &p, maxPipelineSize) != nil || validatePipeline(p) != nil {
			return nil, nil, false, errors.New("invalid explicit hn.pipeline/0 mapping")
		}
		pipeline = &p
	}
	if n, ok := s.Mappings["native"]; ok && n != "generated" {
		return nil, nil, false, errors.New("native workflow mapping supports only generated")
	}
	if enable && pipeline == nil && s.Mappings["native"] != "generated" {
		return nil, nil, false, errors.New("enable requires an explicit native pipeline mapping")
	}
	sort.Strings(used)
	return used, pipeline, enable, nil
}
func readBoundedWorkflowMapping(f *os.File) ([]byte, error) {
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxPipelineSize {
		return nil, errors.New("native pipeline mapping must be a regular file at most 1 MiB")
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxPipelineSize+1))
	if err != nil {
		return nil, errors.New("cannot read native pipeline mapping")
	}
	if len(raw) > maxPipelineSize {
		return nil, errors.New("native pipeline mapping exceeds 1 MiB")
	}
	return raw, nil
}
func (g *githubForge) ReadWorkflows(ctx context.Context, s forge.Selection) (forge.Graph, error) {
	graph := forge.Graph{Nodes: []forge.Node{}, Findings: []forge.Finding{}}
	used, pipeline, enable, err := githubWorkflowImportMappings(s)
	if err != nil {
		return graph, err
	}
	graph.UsedMappings = used
	definitions, err := g.client.ReadWorkflowDefinitions(ctx, s.Project, s.Mappings["source-ref"], s.Key)
	if err != nil {
		return graph, err
	}
	for _, d := range definitions {
		name := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(d.File.Path), ".yaml"), ".yml")
		if !validPipelineName(name) {
			name = "workflow-" + strconv.FormatInt(d.Workflow.ID, 10)
		}
		p := pipeline
		if bytes.HasPrefix(d.File.Content, []byte(githubWorkflowManifestPrefix)) {
			manifest, published, decodeErr := githubParseGeneratedWorkflow(d.File.Content)
			if decodeErr == nil {
				if err := githubWorkflowRejectNativeEcho(manifest, published); err != nil {
					return graph, err
				}
			}
		}
		title := d.Workflow.Name
		description := "Imported source " + d.File.Path + "; provider state " + d.Workflow.State + " is descriptive and grants no native execution authority."
		if s.Mappings["native"] == "generated" {
			state, err := githubDecodeGeneratedWorkflow(d.File.Content)
			if err != nil {
				return graph, err
			}
			p = state.Pipeline
			name, title, description = state.Name, state.Title, state.Description
		}
		w := &forge.Workflow{Name: name, Title: title, Description: description, Enabled: enable, Sources: []forge.SourceDocument{{Format: "github-actions-yaml", Content: string(d.File.Content), Digest: eventID(d.File.Content)}}}
		if p != nil {
			w.Pipeline, _ = json.Marshal(p)
		}
		graph.Nodes = append(graph.Nodes, forge.Node{Kind: ResourceWorkflow, Record: forge.Record{ID: githubOpaqueID(d.Workflow.NodeID, d.Workflow.ID), Key: strconv.FormatInt(d.Workflow.ID, 10), Revision: d.File.Commit + ":" + d.File.SHA, URL: d.Workflow.HTMLURL, CreatedAt: d.Workflow.CreatedAt, UpdatedAt: d.Workflow.UpdatedAt}, Metadata: map[string]string{"github/workflow-path": d.File.Path, "github/workflow-commit": d.File.Commit, "github/workflow-blob": d.File.SHA, "github/workflow-state": d.Workflow.State}, Workflow: w})
		graph.Findings = append(graph.Findings, forge.Finding{Status: forge.Preserved, Path: d.File.Path, Message: "Exact YAML bytes and digest retained as inert source; provider triggers, secrets, runtime, history and enabled state are not native execution or authority."})
		if p != nil {
			graph.Findings = append(graph.Findings, forge.Finding{Status: forge.Mapped, Path: d.File.Path + "/native", Message: "Explicit hn.pipeline/0 mapping; execution still requires the ordinary native materialize/request/policy/runner path."})
		}
	}
	return graph, nil
}

type githubWorkflowExportConfig struct {
	Format         string `json:"format"`
	Branch         string `json:"branch"`
	Path           string `json:"path"`
	ExpectedSHA    string `json:"expected_sha"`
	SourceDigest   string `json:"source_digest,omitempty"`
	Runner         string `json:"runner,omitempty"`
	Trigger        string `json:"trigger,omitempty"`
	Checkout       string `json:"checkout,omitempty"`
	Runtime        string `json:"runtime,omitempty"`
	NativeID       string `json:"native_id,omitempty"`
	NativeRevision string `json:"native_revision,omitempty"`
	NativeDigest   string `json:"native_digest,omitempty"`
}

func githubWorkflowConfig(m map[string]string) (githubWorkflowExportConfig, error) {
	c := githubWorkflowExportConfig{Format: m["format"], Branch: m["branch"], Path: m["path"], ExpectedSHA: m["expected-sha"], SourceDigest: m["source-digest"], Runner: m["runner"], Trigger: m["trigger"], Checkout: m["checkout"], Runtime: m["runtime"]}
	for k := range m {
		switch k {
		case "format", "branch", "path", "expected-sha", "source-digest", "runner", "trigger", "checkout", "runtime":
		default:
			return c, errors.New("unsupported workflow export mapping")
		}
	}
	if !githubapi.ValidWorkflowPath(c.Path) || !githubWorkflowSafeBranch(c.Branch) || (c.ExpectedSHA != "absent" && !githubWorkflowSHA(c.ExpectedSHA)) {
		return c, errors.New("workflow export requires explicit safe path, branch and expected-sha=<40 lowercase hex>|absent")
	}
	if c.Format != "source" && c.Format != "actions" {
		return c, errors.New("workflow export requires format=source or format=actions")
	}
	if c.Format == "source" && (c.Runner != "" || c.Trigger != "" || c.Checkout != "" || c.Runtime != "") {
		return c, errors.New("execution mappings require format=actions")
	}
	if c.Format == "actions" && c.SourceDigest != "" {
		return c, errors.New("source-digest requires format=source")
	}
	if c.SourceDigest != "" && !validEventID(c.SourceDigest) {
		return c, errors.New("source-digest requires exact sha256 digest")
	}
	return c, nil
}
func githubWorkflowSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'f' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
func githubWorkflowSafeBranch(s string) bool {
	if s == "" || len(s) > 256 || strings.HasPrefix(s, "-") || strings.Contains(s, "..") || strings.Contains(s, "@{") || strings.HasSuffix(s, ".") {
		return false
	}
	for _, r := range s {
		if r <= 32 || r == 127 || strings.ContainsRune("~^:?*[\\", r) {
			return false
		}
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || strings.HasPrefix(p, ".") || strings.HasSuffix(p, ".lock") {
			return false
		}
	}
	return true
}

type githubWorkflowManifest struct {
	Schema         string             `json:"schema"`
	Name           string             `json:"name"`
	Title          string             `json:"title"`
	Description    string             `json:"description"`
	Pipeline       PipelineDefinition `json:"pipeline"`
	Runner         string             `json:"runner"`
	Trigger        string             `json:"trigger"`
	Checkout       string             `json:"checkout"`
	Runtime        string             `json:"runtime"`
	NativeID       string             `json:"native_id,omitempty"`
	NativeRevision string             `json:"native_revision,omitempty"`
	NativeDigest   string             `json:"native_digest,omitempty"`
}

const githubWorkflowManifestPrefix = "# hn-workflow/1 "

func githubRenderNativeWorkflow(s WorkflowState, c githubWorkflowExportConfig) ([]byte, error) {
	if s.Pipeline == nil || validatePipeline(*s.Pipeline) != nil {
		return nil, errors.New("native Actions export requires a valid native pipeline")
	}
	if c.Runtime != "python3-posix" || c.Trigger != "workflow_dispatch" && c.Trigger != "push" || c.Runner == "" || len(c.Runner) > 128 {
		return nil, errors.New("native Actions export requires explicit runtime=python3-posix, trigger=workflow_dispatch|push and runner label")
	}
	for _, r := range c.Runner {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return nil, errors.New("invalid explicit runner label")
		}
	}
	if c.Checkout != "none" && (!strings.HasPrefix(c.Checkout, "actions/checkout@") || !githubWorkflowSHA(strings.TrimPrefix(c.Checkout, "actions/checkout@"))) {
		return nil, errors.New("checkout mapping must be none or actions/checkout@<full commit SHA>")
	}
	total := 0
	for _, step := range s.Pipeline.Steps {
		seconds := step.TimeoutSeconds
		if seconds == 0 {
			seconds = defaultTimeout
		}
		total += (seconds+59)/60 + 1
	}
	if total > 350 {
		return nil, errors.New("native timeouts exceed conservative Actions job duration subset")
	}
	manifest := githubWorkflowManifest{Schema: "hn.github-workflow/1", Name: s.Name, Title: s.Title, Description: s.Description, Pipeline: *s.Pipeline, Runner: c.Runner, Trigger: c.Trigger, Checkout: c.Checkout, Runtime: c.Runtime, NativeID: c.NativeID, NativeRevision: c.NativeRevision, NativeDigest: c.NativeDigest}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	runnerYAML, _ := json.Marshal(c.Runner)
	var out strings.Builder
	fmt.Fprintf(&out, "%s%s\nname: Hubnot native workflow\non:\n  %s:\npermissions:\n  contents: read\njobs:\n  native:\n    runs-on: %s\n    timeout-minutes: %d\n    steps:\n", githubWorkflowManifestPrefix, base64.StdEncoding.EncodeToString(raw), c.Trigger, runnerYAML, total+5)
	if c.Checkout != "none" {
		fmt.Fprintf(&out, "      - name: Explicit checkout\n        uses: %s\n        with:\n          persist-credentials: false\n", c.Checkout)
	}
	// User strings only occur in base64 JSON. Actions expressions cannot be
	// interpolated inside command arguments, names, directories or manifests.
	script := "import base64,json,subprocess,sys; s=json.loads(base64.b64decode(sys.argv[1])); subprocess.run([s['command']]+s.get('args',[]),cwd=s.get('workingDirectory') or '.',timeout=s.get('timeoutSeconds') or 300,check=True)"
	for i, step := range s.Pipeline.Steps {
		data, _ := json.Marshal(step)
		seconds := step.TimeoutSeconds
		if seconds == 0 {
			seconds = defaultTimeout
		}
		fmt.Fprintf(&out, "      - name: Native step %d\n        shell: bash --noprofile --norc -e -o pipefail {0}\n        timeout-minutes: %d\n        run: |\n          python3 -c %s %s\n", i+1, (seconds+59)/60+1, githubWorkflowShellQuote(script), githubWorkflowShellQuote(base64.StdEncoding.EncodeToString(data)))
	}
	content := []byte(out.String())
	if len(content) > githubapi.MaxWorkflowBytes {
		return nil, errors.New("rendered workflow exceeds 1 MiB")
	}
	return content, nil
}
func githubWorkflowShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
func githubDecodeGeneratedWorkflow(content []byte) (WorkflowState, error) {
	_, s, err := githubParseGeneratedWorkflow(content)
	return s, err
}
func githubParseGeneratedWorkflow(content []byte) (githubWorkflowManifest, WorkflowState, error) {
	var s WorkflowState
	var m githubWorkflowManifest
	first, _, ok := bytes.Cut(content, []byte("\n"))
	if !ok || !bytes.HasPrefix(first, []byte(githubWorkflowManifestPrefix)) {
		return m, s, errors.New("source has no supported exact native workflow manifest")
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(string(first), githubWorkflowManifestPrefix))
	if err != nil || len(raw) > maxPipelineSize || strictResourceJSON(raw, &m, maxPipelineSize) != nil || m.Schema != "hn.github-workflow/1" {
		return m, s, errors.New("invalid native workflow manifest")
	}
	if (m.NativeID != "" || m.NativeRevision != "" || m.NativeDigest != "") && (!validEventID(m.NativeID) || !validEventID(m.NativeRevision) || !validEventID(m.NativeDigest)) {
		return m, s, errors.New("invalid native workflow publication claim")
	}
	s = WorkflowState{Schema: "hn.workflow/1", Name: m.Name, Title: m.Title, Description: m.Description, Pipeline: &m.Pipeline}
	if validateWorkflowState(s) != nil {
		return m, s, errors.New("invalid native workflow manifest state")
	}
	rendered, err := githubRenderNativeWorkflow(s, githubWorkflowExportConfig{Format: "actions", Runner: m.Runner, Trigger: m.Trigger, Checkout: m.Checkout, Runtime: m.Runtime, NativeID: m.NativeID, NativeRevision: m.NativeRevision, NativeDigest: m.NativeDigest})
	if err != nil || !bytes.Equal(content, rendered) {
		return m, s, errors.New("source differs from supported exact native Actions renderer; provide an explicit pipeline-file mapping")
	}
	return m, s, nil
}
func githubWorkflowRejectNativeEcho(m githubWorkflowManifest, s WorkflowState) error {
	if m.NativeID == "" {
		return nil
	}
	events, _, err := collectIssueEvents()
	if err != nil {
		return err
	}
	cat, err := BuildResourceCatalog(events)
	if err != nil {
		return err
	}
	root := cat.Resources[m.NativeID]
	if root == nil || root.Family != ResourceWorkflow {
		return nil
	}
	for _, e := range root.History {
		if e.ID != m.NativeRevision {
			continue
		}
		canonical, err := CanonicalResourceState(ResourceWorkflow, []byte(e.Event.Body))
		if err != nil {
			return err
		}
		if eventID(canonical) != m.NativeDigest {
			return nil
		}
		var original WorkflowState
		if DecodeResourceState(ResourceWorkflow, canonical, &original) != nil {
			return errors.New("invalid existing native workflow publication witness")
		}
		a, _ := json.Marshal(original.Pipeline)
		b, _ := json.Marshal(s.Pipeline)
		if original.Name == s.Name && original.Title == s.Title && original.Description == s.Description && bytes.Equal(a, b) {
			return errors.New("source is an exact publication of an existing signed native workflow; refusing duplicate native root or automatic source remapping")
		}
	}
	return nil
}
func githubWorkflowContent(snapshot forgeResourceSnapshot, c githubWorkflowExportConfig) ([]byte, error) {
	if snapshot.Resource != forge.ResourceWorkflow || len(snapshot.Graph.Nodes) != 1 || snapshot.Graph.Nodes[0].Workflow == nil {
		return nil, errors.New("workflow export requires one exact native workflow")
	}
	var state WorkflowState
	for _, v := range snapshot.Versions {
		if v.Revision == snapshot.NativeRevision {
			if DecodeResourceState(ResourceWorkflow, v.State, &state) != nil {
				return nil, errors.New("invalid exact workflow snapshot")
			}
		}
	}
	if state.Schema == "" {
		return nil, errors.New("workflow snapshot is missing exact signed revision")
	}
	if c.Format == "actions" {
		c.NativeID = snapshot.NativeID
		c.NativeRevision = snapshot.NativeRevision
		for _, v := range snapshot.Versions {
			if v.Revision == snapshot.NativeRevision {
				c.NativeDigest = eventID(v.State)
			}
		}
		return githubRenderNativeWorkflow(state, c)
	}
	var selected []byte
	count := 0
	for _, d := range state.SourceDocuments {
		if d.Format != "github-actions-yaml" || c.SourceDigest != "" && c.SourceDigest != d.Digest {
			continue
		}
		if eventID([]byte(d.Content)) != d.Digest {
			return nil, errors.New("workflow source digest mismatch")
		}
		selected = []byte(d.Content)
		count++
	}
	if count != 1 {
		return nil, errors.New("source export requires exactly one github-actions-yaml document; select source-digest for multiple documents")
	}
	return selected, nil
}

type githubWorkflowDestination struct {
	Provider, Endpoint, Project, NativeID string
	Config                                githubWorkflowExportConfig
}
type githubWorkflowCheckpoint struct {
	Schema      string                    `json:"schema"`
	Scope       string                    `json:"scope"`
	Destination githubWorkflowDestination `json:"destination"`
	Snapshot    forgeResourceSnapshot     `json:"snapshot"`
	Pending     *forgeRemoteIntent        `json:"pending,omitempty"`
}

// This exporter performs exactly one remote mutation. Reserve its largest
// future prepared/in-flight/confirmed form, including payload plus result,
// before saving a checkpoint or dispatching PUT. Result identities are fixed
// by explicit destination path and 40-hex blob/commit SHA, not opaque IDs.
func githubWorkflowCheckpointBudget(c githubWorkflowCheckpoint) error {
	raw, err := json.Marshal(c)
	if err != nil || len(raw) > forgeCheckpointLimit {
		return errors.New("workflow export complete checkpoint exceeds 8 MiB")
	}
	if c.Pending != nil {
		copy := *c.Pending
		copy.Phase = "confirmed"
		copy.ResultID = c.Destination.Project + "/" + c.Destination.Config.Path
		copy.ResultKey = c.Destination.Config.Path
		copy.ResultRevision = strings.Repeat("f", 40)
		result := githubWorkflowResult{Path: c.Destination.Config.Path, SHA: strings.Repeat("f", 40), Commit: strings.Repeat("f", 40)}
		copy.ResultPayload, _ = json.Marshal(result)
		copy.ResultDigest = eventID(copy.ResultPayload)
		c.Pending = &copy
		raw, err = json.Marshal(c)
		if err != nil || len(raw) > forgeCheckpointLimit {
			return errors.New("workflow export future confirmed checkpoint exceeds 8 MiB")
		}
	}
	return nil
}

type githubWorkflowPayload struct {
	Project, Path, Branch, ExpectedSHA string
	Content                            []byte
}
type githubWorkflowResult struct{ Path, SHA, Commit string }

func githubWorkflowScope(d githubWorkflowDestination) string { return forgeDigestJSON(d) }
func githubWorkflowIntent(c githubWorkflowCheckpoint, content []byte, expected string) forgeRemoteIntent {
	p := githubWorkflowPayload{c.Destination.Project, c.Destination.Config.Path, c.Destination.Config.Branch, expected, content}
	raw, _ := json.Marshal(p)
	return forgeRemoteIntent{Schema: "hn.forge-remote-intent/1", Scope: c.Scope, Provider: "github", Endpoint: c.Destination.Endpoint, Kind: ResourceWorkflow, Action: "put-workflow-file", NativeID: c.Snapshot.NativeID, NativeRevision: c.Snapshot.NativeRevision, TargetID: p.Project + "/" + p.Path, TargetKey: p.Path, ExpectedRevision: expected, Payload: raw, PayloadDigest: eventID(raw), Phase: "prepared"}
}
func githubValidateWorkflowIntent(c githubWorkflowCheckpoint, content []byte) error {
	if c.Schema != "hn.github-workflow-export/1" || c.Scope != githubWorkflowScope(c.Destination) || c.Snapshot.NativeID != c.Destination.NativeID || c.Snapshot.Resource != forge.ResourceWorkflow || !validEventID(c.Destination.NativeID) {
		return errors.New("workflow checkpoint snapshot differs from explicit native resource selection")
	}
	if c.Pending == nil {
		return errors.New("workflow checkpoint missing exact PUT intent")
	}
	i := *c.Pending
	if forgeValidateRemoteIntent(i) != nil {
		return errors.New("invalid workflow PUT intent")
	}
	expectedSHA := c.Destination.Config.ExpectedSHA
	if expectedSHA == "absent" {
		expectedSHA = ""
	}
	expected := githubWorkflowIntent(c, content, expectedSHA)
	if i.Schema != expected.Schema || i.Scope != expected.Scope || i.Provider != expected.Provider || i.Endpoint != expected.Endpoint || i.Kind != expected.Kind || i.Action != expected.Action || i.NativeID != expected.NativeID || i.NativeRevision != expected.NativeRevision || i.TargetID != expected.TargetID || i.TargetKey != expected.TargetKey || i.ExpectedRevision != expected.ExpectedRevision || !bytes.Equal(i.Payload, expected.Payload) || i.PayloadDigest != expected.PayloadDigest || i.ExpectedRevision != "" && !githubWorkflowSHA(i.ExpectedRevision) {
		return errors.New("workflow PUT intent differs from authenticated snapshot/destination")
	}
	if i.Phase == "prepared" || i.Phase == "in_flight" {
		if i.ResultID != "" || i.ResultKey != "" || i.ResultRevision != "" || (len(i.ResultPayload) > 0 && string(i.ResultPayload) != "null") || i.ResultDigest != "" {
			return errors.New("unconfirmed workflow intent contains an invented result")
		}
	}
	if i.Phase == "confirmed" {
		if err := githubValidateWorkflowResult(c.Destination.Project, githubWorkflowPayload{c.Destination.Project, c.Destination.Config.Path, c.Destination.Config.Branch, i.ExpectedRevision, content}, forgeRemoteResult{i.ResultID, i.ResultKey, i.ResultRevision, i.ResultPayload, i.ResultDigest}); err != nil {
			return err
		}
	}
	return githubWorkflowCheckpointBudget(c)
}
func githubValidateWorkflowResult(project string, p githubWorkflowPayload, r forgeRemoteResult) error {
	var result githubWorkflowResult
	if forgeStrictJSON(r.Payload, &result) != nil || r.ID != project+"/"+p.Path || r.Key != p.Path || r.PayloadDigest != eventID(r.Payload) || !githubWorkflowSHA(result.SHA) || !githubWorkflowSHA(result.Commit) || result.Path != p.Path || r.Revision != result.SHA || result.SHA != githubWorkflowBlob(p.Content) {
		return errors.New("workflow PUT result differs from exact intended file")
	}
	return nil
}

// Git blob identity is used only to verify exact provider file bytes.
func githubWorkflowBlob(content []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	_, _ = h.Write(content)
	return fmt.Sprintf("%x", h.Sum(nil))
}
func githubWorkflowReadDestination(ctx context.Context, g *githubForge, project string, c githubWorkflowExportConfig) (githubapi.WorkflowFile, bool, error) {
	pin, err := g.client.ResolveWorkflowCommit(ctx, project, c.Branch)
	if err != nil {
		return githubapi.WorkflowFile{}, false, err
	}
	file, err := g.client.GetWorkflowFile(ctx, project, c.Path, pin)
	var httpErr *githubapi.HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode == 404 {
		return githubapi.WorkflowFile{}, false, nil
	}
	return file, err == nil, err
}
func githubWorkflowReport(c githubWorkflowCheckpoint, status string) error {
	return json.NewEncoder(os.Stdout).Encode(forge.Plan{Resource: forge.ResourceWorkflow, Direction: "export", Nodes: 1, Findings: []forge.Finding{{Status: forge.Mapped, Path: c.Destination.Config.Path, Message: "Explicit branch " + c.Destination.Config.Branch + ": publishing this workflow file may activate provider triggers; " + status}, {Status: forge.Preserved, Path: "native-authority", Message: "Native signatures, policy, roles, provider enabled state, secrets and runner isolation are not transferred. Runtime mapping is an explicit environment choice, not execution equivalence."}}})
}
func githubExportWorkflow(ctx context.Context, p forge.Identity, o transferOptions, snapshot forgeResourceSnapshot) error {
	g, ok := p.(*githubForge)
	if !ok {
		return errors.New("GitHub workflow exporter requires the selected GitHub transport")
	}
	config, err := githubWorkflowConfig(o.mappings)
	if err != nil {
		return err
	}
	var content []byte
	destination := githubWorkflowDestination{"github", p.Endpoint(), o.project, o.id, config}
	scope := githubWorkflowScope(destination)
	name := "workflow-export-" + strings.TrimPrefix(scope, "sha256:") + ".json"
	// Decode into zero state: absent fields in an older snapshot must not
	// inherit pointers or slices from the newer native snapshot.
	var checkpoint githubWorkflowCheckpoint
	loaded, err := forgeLoadJSON(name, &checkpoint)
	if err != nil {
		return err
	}
	if loaded {
		if checkpoint.Schema != "hn.github-workflow-export/1" || checkpoint.Scope != scope || forgeDigestJSON(checkpoint.Destination) != forgeDigestJSON(destination) || githubWorkflowScope(checkpoint.Destination) != scope {
			return errors.New("workflow checkpoint destination differs from explicit selection")
		}
		if err := forgeAuthenticateResourceSnapshot(checkpoint.Snapshot); err != nil {
			return fmt.Errorf("saved workflow snapshot: %w", err)
		}
		oldContent, err := githubWorkflowContent(checkpoint.Snapshot, config)
		if err != nil {
			return err
		}
		if err := githubValidateWorkflowIntent(checkpoint, oldContent); err != nil {
			return err
		}
		// Finish a previous exact write before considering newer native content.
		if checkpoint.Pending.Phase != "confirmed" {
			if o.dryRun {
				return githubWorkflowReport(checkpoint, "pending exact PUT requires read-only positive reconciliation on retry")
			}
			if err := githubWorkflowExecute(ctx, g, name, &checkpoint, oldContent); err != nil {
				return err
			}
		}
		current, exists, err := githubWorkflowReadDestination(ctx, g, o.project, config)
		if err != nil {
			return err
		}
		if !exists || current.SHA != checkpoint.Pending.ResultRevision || !bytes.Equal(current.Content, oldContent) {
			return errors.New("workflow destination changed since the last confirmed exact export")
		}
		// A newer native projection cannot prevent confirmation of the exact
		// previously dispatched intent. It is considered only after recovery.
		content, err = githubWorkflowContent(snapshot, config)
		if err != nil {
			return err
		}
		if bytes.Equal(content, oldContent) {
			return githubWorkflowReport(checkpoint, "exact intended source already present; no mutation")
		}
		return errors.New("native workflow changed after this export; repeat with current remote expected-sha for a fresh explicit content-CAS transfer")
	} else {
		checkpoint = githubWorkflowCheckpoint{Schema: "hn.github-workflow-export/1", Scope: scope, Destination: destination, Snapshot: snapshot}
		content, err = githubWorkflowContent(snapshot, config)
		if err != nil {
			return err
		}
		current, exists, err := githubWorkflowReadDestination(ctx, g, o.project, config)
		if err != nil {
			return err
		}
		expected := config.ExpectedSHA
		if expected == "absent" {
			expected = ""
		}
		if exists && bytes.Equal(current.Content, content) {
			return githubWorkflowReport(checkpoint, "exact intended source already present; no mutation")
		}
		if exists && current.SHA != expected || !exists && expected != "" {
			return errors.New("workflow destination does not match explicit expected-sha; no mutation")
		}
		if err := githubWorkflowCheckImportedSource(ctx, g, o, snapshot, current, exists); err != nil {
			return err
		}
		intent := githubWorkflowIntent(checkpoint, content, expected)
		checkpoint.Pending = &intent
	}
	if err := githubValidateWorkflowIntent(checkpoint, content); err != nil {
		return err
	}
	if o.dryRun {
		return githubWorkflowReport(checkpoint, "dry-run of "+config.Format+" definition with exact blob-SHA guard; no mutation")
	}
	if err := githubWorkflowExecute(ctx, g, name, &checkpoint, content); err != nil {
		return err
	}
	return githubWorkflowReport(checkpoint, "exact intended source confirmed")
}
func githubWorkflowCheckImportedSource(ctx context.Context, g *githubForge, o transferOptions, snapshot forgeResourceSnapshot, current githubapi.WorkflowFile, exists bool) error {
	hasSource := false
	for _, v := range snapshot.Versions {
		if v.Revision == snapshot.NativeID {
			var opening WorkflowState
			if DecodeResourceState(ResourceWorkflow, v.State, &opening) != nil {
				return errors.New("invalid workflow opening witness")
			}
			hasSource = opening.Metadata[forgeResourceSourceKey] != ""
		}
	}
	if !hasSource {
		return nil
	}
	source, _, version, err := forgeImportedResourceBaseline(snapshot.NativeID)
	if err != nil {
		return errors.New("workflow source association has no valid signed import witness")
	}
	if source.Provider != "github" || source.Endpoint != g.Endpoint() {
		return nil
	}
	var baseline WorkflowState
	if DecodeResourceState(ResourceWorkflow, version.State, &baseline) != nil {
		return errors.New("invalid signed imported workflow baseline")
	}
	originalPath := baseline.Metadata["github/workflow-path"]
	rows, err := g.client.ListWorkflows(ctx, o.project)
	if err != nil {
		return err
	}
	sameIdentity := false
	for _, row := range rows {
		if githubOpaqueID(row.NodeID, row.ID) == source.Record.ID {
			if row.Path == o.mappings["path"] {
				sameIdentity = true
				break
			}
			if originalPath == o.mappings["path"] {
				return errors.New("source workflow moved; map its current path or a distinct explicit destination")
			}
		}
	}
	// Mutable repository/path selectors do not replace immutable workflow identity.
	// An explicitly different target file remains an ordinary cross-project export.
	if !sameIdentity {
		if originalPath == o.mappings["path"] {
			return errors.New("original source workflow identity cannot be re-established")
		}
		return nil
	}
	matched := false
	for _, doc := range baseline.SourceDocuments {
		if doc.Format == "github-actions-yaml" && bytes.Equal([]byte(doc.Content), current.Content) {
			matched = true
		}
	}
	if !exists || !matched {
		return errors.New("source workflow changed since signed import baseline; preserve it and reconcile explicitly")
	}
	return nil
}
func githubWorkflowExecute(ctx context.Context, g *githubForge, name string, c *githubWorkflowCheckpoint, content []byte) error {
	if err := githubValidateWorkflowIntent(*c, content); err != nil {
		return err
	}
	var payload githubWorkflowPayload
	if forgeStrictJSON(c.Pending.Payload, &payload) != nil {
		return errors.New("invalid exact workflow PUT payload")
	}
	save := func() error {
		if err := githubWorkflowCheckpointBudget(*c); err != nil {
			return err
		}
		return forgeStoreJSON(name, c)
	}
	result := func(f githubapi.WorkflowFile) forgeRemoteResult {
		return forgeRemoteObserved(payload.Project+"/"+payload.Path, payload.Path, f.SHA, githubWorkflowResult{f.Path, f.SHA, f.Commit})
	}
	_, err := forgeExecuteRemoteIntent(ctx, c.Pending, save, func(ctx context.Context, i forgeRemoteIntent) (forgeRemoteResult, error) {
		// Recheck immediately before dispatch; Contents PUT additionally performs CAS.
		current, exists, err := githubWorkflowReadDestination(ctx, g, payload.Project, c.Destination.Config)
		if err != nil {
			return forgeRemoteResult{}, err
		}
		if exists && current.SHA != payload.ExpectedSHA || !exists && payload.ExpectedSHA != "" {
			return forgeRemoteResult{}, errors.New("workflow blob changed immediately before PUT")
		}
		options := transferOptions{provider: "github", project: payload.Project, id: c.Snapshot.NativeID, mappings: map[string]string{"path": payload.Path}}
		if err := githubWorkflowCheckImportedSource(ctx, g, options, c.Snapshot, current, exists); err != nil {
			return forgeRemoteResult{}, err
		}
		file, err := g.client.PutWorkflowFile(ctx, payload.Project, githubapi.WorkflowFileInput{Path: payload.Path, Branch: payload.Branch, ExpectedSHA: payload.ExpectedSHA, Content: payload.Content, Message: "Hubnot explicit workflow export " + c.Snapshot.NativeRevision})
		if err != nil {
			return forgeRemoteResult{}, err
		}
		return result(file), nil
	}, func(ctx context.Context, i forgeRemoteIntent) (forgeRemoteResult, bool, error) {
		file, exists, err := githubWorkflowReadDestination(ctx, g, payload.Project, c.Destination.Config)
		if err != nil || !exists {
			return forgeRemoteResult{}, false, err
		}
		if !bytes.Equal(file.Content, payload.Content) {
			return forgeRemoteResult{}, false, nil
		}
		return result(file), true, nil
	}, func(i forgeRemoteIntent, r forgeRemoteResult) error {
		return githubValidateWorkflowResult(payload.Project, payload, r)
	})
	return err
}
