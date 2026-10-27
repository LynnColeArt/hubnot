package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"hubnot/internal/migration"
)

func init() { localCommands["migrate"] = cmdMigrate }

const migrationCLIHelp = `Guided, inspectable project migration. No command deletes the source.
  hn migrate                         guided plan, then explicit apply question
  hn migrate plan [--guided] --session NEW_DIRECTORY [selection flags]
  hn migrate capabilities [source/destination configuration flags]
  hn migrate inspect --session DIRECTORY [--blob sha256:DIGEST]
  hn migrate inspect --archive FILE [--blob sha256:DIGEST]
  hn migrate imports --repository REPOSITORY
  hn migrate inspect --repository REPOSITORY --snapshot sha256:DIGEST
  hn migrate apply|resume --session DIRECTORY --confirm-apply [fresh credentials]
Selection flags (all foreign configurations require an explicit profile/mode):
  --source-kind local|archive|saved|imported|PROVIDER --source-repository PATH_OR_SELECTOR
  --destination-kind local|archive|PROVIDER --destination-repository PATH_OR_SELECTOR
  --source-endpoint URL --source-organization NAME --source-project NAME
  --source-api-version PROFILE --source-credential-mode MODE --source-token-env NAME
  --destination-endpoint URL --destination-organization NAME --destination-project NAME
  --destination-api-version PROFILE --destination-credential-mode MODE --destination-token-env NAME
  --kind KIND (repeat; comments require their own explicit selection)
  --ref FULL_REF (repeat) --source-git REPOSITORY (provider/imported companion only)
  --map KEY=VALUE (repeat; empty VALUE explicitly omits that field)
  --decision FINDING_ID=acknowledge|preserve|map (repeat; no blanket yes)
  --git-remote ABSOLUTE_PATH_OR_HTTPS --git-origin HTTPS_ORIGIN --git-auth-env NAME --git-ca-file PATH
  --source-workflow-ref IMMUTABLE_REV --source-planning SELECTOR (repeat)
  --source-attachments JSON_FILE (bounded explicit AttachmentSelection array)
  --snapshot sha256:DIGEST (repeat; explicit imported snapshots)
  --actor ACTOR (optional stable progress binding)
Read-only plan never applies; apply/resume require --confirm-apply or guided APPLY.
Source and destination capabilities describe the selected configuration, not live access.
Profiles: GitHub 2022-11-28/bearer; GitLab v4/private-token;
Forgejo 11|16/token; Gitea 1.24|1.27/token; Bitbucket 2.0/
anonymous|oauth2|api_token_bearer|api_token_basic; Azure services-7.1|server-7.0/bearer|pat.
See docs/migration-cli.md for scope, mappings, bounds and credential handling.
`

type migrationCLIOptions struct {
	command                                                                 string
	source, destination                                                     migration.ProviderConfig
	session, sourceGit, remote, actor, sourceTokenEnv, destinationTokenEnv  string
	archive, repository, blob, attachmentFile, gitOrigin, gitAuthEnv, gitCA string
	refs, kinds, snapshots                                                  []string
	mappings, decisions                                                     map[string]string
	guided, confirm                                                         bool
}

func migrationCLIMap(m map[string]string, input string) error {
	key, value, ok := strings.Cut(input, "=")
	if !ok || key == "" || len(input) > 8192 || !utf8.ValidString(input) || strings.ContainsAny(input, "\x00\r\n") {
		return fmt.Errorf("expected bounded KEY=VALUE, got invalid mapping")
	}
	if _, exists := m[key]; exists {
		return fmt.Errorf("duplicate mapping/decision for %q", key)
	}
	m[key] = value
	return nil
}

func parseMigrationOptions(args []string) (migrationCLIOptions, error) {
	o := migrationCLIOptions{command: "plan", mappings: map[string]string{}, decisions: map[string]string{}}
	if len(args) == 0 {
		o.guided = true
	} else {
		o.command = args[0]
		args = args[1:]
	}
	switch o.command {
	case "plan", "capabilities", "inspect", "imports", "apply", "resume":
	default:
		return o, fmt.Errorf("unknown migration command %q; use hn migrate help", o.command)
	}
	f := flag.NewFlagSet("migrate "+o.command, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	for _, v := range []struct {
		name  string
		value *string
	}{
		{"session", &o.session}, {"source-git", &o.sourceGit}, {"git-remote", &o.remote}, {"actor", &o.actor},
		{"source-token-env", &o.sourceTokenEnv}, {"destination-token-env", &o.destinationTokenEnv},
		{"archive", &o.archive}, {"repository", &o.repository}, {"blob", &o.blob}, {"source-attachments", &o.attachmentFile},
		{"git-origin", &o.gitOrigin}, {"git-auth-env", &o.gitAuthEnv}, {"git-ca-file", &o.gitCA},
	} {
		f.StringVar(v.value, v.name, "", "")
	}
	for _, side := range []struct {
		name string
		c    *migration.ProviderConfig
	}{{"source", &o.source}, {"destination", &o.destination}} {
		for _, v := range []struct {
			name  string
			value *string
		}{
			{"kind", &side.c.Locator.Kind}, {"endpoint", &side.c.Locator.Endpoint}, {"organization", &side.c.Locator.Organization},
			{"project", &side.c.Locator.Project}, {"repository", &side.c.Locator.Repository},
			{"api-version", &side.c.APIVersion}, {"credential-mode", &side.c.CredentialMode},
		} {
			f.StringVar(v.value, side.name+"-"+v.name, "", "")
		}
	}
	f.StringVar(&o.source.WorkflowRef, "source-workflow-ref", "", "")
	for _, v := range []struct {
		name   string
		values *[]string
	}{{"ref", &o.refs}, {"kind", &o.kinds}, {"snapshot", &o.snapshots}, {"source-planning", &o.source.PlanningSelectors}} {
		values := v.values
		f.Func(v.name, "", func(s string) error {
			if len(*values) >= 10000 {
				return fmt.Errorf("selection exceeds bound")
			}
			*values = append(*values, s)
			return nil
		})
	}
	f.Func("map", "", func(s string) error { return migrationCLIMap(o.mappings, s) })
	f.Func("decision", "", func(s string) error { return migrationCLIMap(o.decisions, s) })
	f.BoolVar(&o.guided, "guided", o.guided, "")
	f.BoolVar(&o.confirm, "confirm-apply", false, "")
	for _, a := range args {
		if !utf8.ValidString(a) || len(a) > 8192 || strings.ContainsRune(a, 0) {
			return o, fmt.Errorf("invalid or oversized migration argument")
		}
	}
	if err := f.Parse(args); err != nil {
		return o, err
	}
	if f.NArg() != 0 {
		return o, fmt.Errorf("unexpected migration arguments; use hn migrate help")
	}
	var inappropriate string
	f.Visit(func(v *flag.Flag) {
		allowed := true
		switch o.command {
		case "inspect":
			allowed = v.Name == "session" || v.Name == "archive" || v.Name == "repository" || v.Name == "snapshot" || v.Name == "blob"
		case "imports":
			allowed = v.Name == "repository"
		case "apply", "resume":
			allowed = v.Name == "session" || v.Name == "guided" || v.Name == "confirm-apply" || v.Name == "actor" || v.Name == "destination-token-env" || v.Name == "git-origin" || v.Name == "git-auth-env" || v.Name == "git-ca-file"
		case "capabilities":
			allowed = strings.HasPrefix(v.Name, "source-") || strings.HasPrefix(v.Name, "destination-")
		case "plan":
			allowed = v.Name != "archive" && v.Name != "repository" && v.Name != "blob" && v.Name != "confirm-apply"
		}
		if !allowed {
			inappropriate = v.Name
		}
	})
	if inappropriate != "" {
		return o, fmt.Errorf("--%s is not valid for migrate %s; saved-plan commands do not accept selection overrides", inappropriate, o.command)
	}
	for id, action := range o.decisions {
		if action != "acknowledge" && action != "preserve" && action != "map" {
			return o, fmt.Errorf("--decision %s requires acknowledge, preserve or map", id)
		}
	}
	if o.command != "plan" && o.confirm && o.command != "apply" && o.command != "resume" {
		return o, fmt.Errorf("--confirm-apply is only valid for apply/resume")
	}
	if o.command == "plan" && o.confirm {
		return o, fmt.Errorf("plan cannot apply; use hn migrate apply --confirm-apply after inspection")
	}
	return o, nil
}

func cmdMigrate(args []string) error {
	return runMigrationCLI(context.Background(), args, os.Stdin, os.Stdout)
}
func migrationCLIForeign(kind string) bool {
	return kind != "local" && kind != "archive" && kind != "saved" && kind != "imported"
}
func migrationCLIProfile(side string, c migration.ProviderConfig) error {
	if c.Locator.Kind == "" {
		return fmt.Errorf("missing --%s-kind; use --guided for questions", side)
	}
	if !migrationCLIForeign(c.Locator.Kind) {
		if c.APIVersion != "" || c.CredentialMode != "" || c.Locator.Endpoint != "" || c.Locator.Organization != "" || c.Locator.Project != "" {
			return fmt.Errorf("%s local/archive/saved/imported inputs do not use foreign profile or endpoint fields", side)
		}
		return nil
	}
	if c.APIVersion == "" {
		return fmt.Errorf("missing --%s-api-version; select an explicit adapter profile (hn migrate help)", side)
	}
	if c.CredentialMode == "" {
		return fmt.Errorf("missing --%s-credential-mode; select an explicit adapter authentication mode", side)
	}
	if c.Locator.Endpoint == "" {
		return fmt.Errorf("missing --%s-endpoint", side)
	}
	return nil
}
func migrationCLICredentials(c *migration.ProviderConfig, environment string) error {
	if !migrationCLIForeign(c.Locator.Kind) {
		if environment != "" {
			return fmt.Errorf("credentials apply only to a selected foreign adapter")
		}
		return nil
	}
	p, ok := migrationProviders.Lookup(c.Locator.Kind)
	if !ok {
		return fmt.Errorf("migration provider %q is excluded or unavailable; use hn migrate capabilities", c.Locator.Kind)
	}
	if environment == "" {
		environment = p.TokenEnv
	}
	if strings.ContainsAny(environment, "=\x00\r\n") {
		return fmt.Errorf("invalid credential environment name")
	}
	if environment != "" {
		c.Token = os.Getenv(environment)
	}
	return nil
}
func migrationCLIJSON(out io.Writer, v any) error {
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
func migrationCLIPrintCapabilities(ctx context.Context, out io.Writer, name string, v interface {
	Capabilities(context.Context) ([]migration.Capability, error)
}) error {
	c, e := v.Capabilities(ctx)
	if e != nil {
		return e
	}
	fmt.Fprintln(out, name+" capabilities (offline metadata; access not verified):")
	return migrationCLIJSON(out, c)
}

func runMigrationCLI(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		_, e := io.WriteString(out, migrationCLIHelp)
		return e
	}
	o, e := parseMigrationOptions(args)
	if e != nil {
		return e
	}
	w := newMigrationWizard(in, out)
	switch o.command {
	case "inspect":
		return inspectMigrationCLI(ctx, o, out)
	case "imports":
		if o.repository == "" {
			return fmt.Errorf("missing --repository")
		}
		dir, e := migrationDestinationGitDir(ctx, o.repository)
		if e != nil {
			return e
		}
		v, e := ListLocalMigrations(dir, migration.DefaultLimits())
		if e != nil {
			return e
		}
		return migrationCLIJSON(out, v)
	case "apply", "resume":
		return applyMigrationCLI(ctx, o, w, out)
	case "capabilities":
		fmt.Fprintln(out, "Installed kinds (no remote access checked): local archive saved imported", strings.Join(MigrationProviderKinds(), " "))
		if o.source.Locator.Kind != "" {
			if e = migrationCLIProfile("source", o.source); e != nil {
				return e
			}
			var s migration.Source
			var e error
			if !migrationCLIForeign(o.source.Locator.Kind) && o.source.Locator.Repository == "" {
				if o.sourceTokenEnv != "" {
					return fmt.Errorf("local metadata does not use credentials")
				}
				s = LocalMigrationSource{}
				if o.source.Locator.Kind == "imported" {
					s = ImportedMigrationSource{}
				}
			} else {
				s, e = migrationCLISource(ctx, &o)
			}
			if e != nil {
				return e
			}
			if e = migrationCLIPrintCapabilities(ctx, out, "Source", s); e != nil {
				return e
			}
		}
		if o.destination.Locator.Kind != "" {
			if e = migrationCLIProfile("destination", o.destination); e != nil {
				return e
			}
			var d migration.Destination
			var e error
			if (o.destination.Locator.Kind == "local" || o.destination.Locator.Kind == "archive") && o.destination.Locator.Repository == "" {
				if o.destinationTokenEnv != "" {
					return fmt.Errorf("local metadata does not use credentials")
				}
				d = LocalMigrationDestination{}
			} else {
				d, e = migrationCLIDestination(ctx, &o)
			}
			if e != nil {
				return e
			}
			return migrationCLIPrintCapabilities(ctx, out, "Destination", d)
		}
		return nil
	}
	if o.guided {
		fmt.Fprintln(out, "Installed provider kinds:", strings.Join(MigrationProviderKinds(), " "), "; local archive saved imported sources; local archive destinations")
		if e = w.configure("source", &o.source); e != nil {
			return e
		}
		if e = w.configure("destination", &o.destination); e != nil {
			return e
		}
		if e = w.fill(&o.session, "new saved session directory", false); e != nil {
			return e
		}
		immutableSource := o.source.Locator.Kind == "archive" || o.source.Locator.Kind == "saved"
		if len(o.kinds) == 0 && !immutableSource {
			prompt := "Resource kinds, comma separated (- for local/imported native selection)"
			if migrationCLIForeign(o.source.Locator.Kind) {
				prompt = "Resource kinds, comma separated (explicit provider scope required)"
			}
			v, e := w.answer(prompt, true)
			if e != nil {
				return e
			}
			if v != "-" {
				o.kinds = strings.Split(v, ",")
			}
		}
		if len(o.refs) == 0 && !immutableSource {
			v, e := w.answer("Full Git refs, comma separated (- for no code/native refs)", true)
			if e != nil {
				return e
			}
			if v != "-" {
				o.refs = strings.Split(v, ",")
			}
		}
		if len(o.refs) > 0 && o.source.Locator.Kind != "local" && o.source.Locator.Kind != "archive" && o.source.Locator.Kind != "saved" {
			if e = w.fill(&o.sourceGit, "already-present source Git repository", false); e != nil {
				return e
			}
		}
		if len(o.snapshots) == 0 && o.source.Locator.Kind == "imported" {
			v, e := w.answer("Imported snapshot digests, comma separated", true)
			if e != nil {
				return e
			}
			o.snapshots = strings.Split(v, ",")
		}
		if len(o.mappings) == 0 {
			if e = w.mappings(o.mappings); e != nil {
				return e
			}
		}
		if len(o.refs) > 0 && migrationCLIForeign(o.destination.Locator.Kind) {
			if e = w.fill(&o.remote, "destination Git remote (absolute path or HTTPS)", false); e != nil {
				return e
			}
		}
	}
	if e = migrationCLIProfile("source", o.source); e != nil {
		return e
	}
	if e = migrationCLIProfile("destination", o.destination); e != nil {
		return e
	}
	if migrationCLIForeign(o.source.Locator.Kind) && len(o.kinds) == 0 {
		return fmt.Errorf("missing --kind: select each foreign resource explicitly, for example --kind issue --kind comment")
	}
	if (o.source.Locator.Kind == "archive" || o.source.Locator.Kind == "saved") && (len(o.kinds) > 0 || len(o.refs) > 0) {
		return fmt.Errorf("archive/saved sources preserve their immutable scope; --kind and --ref cannot silently filter it")
	}
	if (o.source.Locator.Kind == "local" || o.source.Locator.Kind == "imported") && len(o.kinds) > 0 {
		return fmt.Errorf("local/imported sources select native refs or imported snapshot digests; --kind cannot silently filter preserved records")
	}
	if o.session == "" {
		return fmt.Errorf("missing --session NEW_DIRECTORY; its parent must already exist")
	}
	if o.remote != "" && !migrationCLIForeign(o.destination.Locator.Kind) {
		return fmt.Errorf("--git-remote requires a foreign destination; local/archive preserve selected history directly")
	}
	if migrationCLIForeign(o.destination.Locator.Kind) && len(o.refs) > 0 && o.remote == "" {
		return fmt.Errorf("selected refs require explicit --git-remote and --map FULL_SOURCE_REF=FULL_DESTINATION_REF")
	}
	s, e := migrationCLISource(ctx, &o)
	if e != nil {
		return e
	}
	d, e := migrationCLIDestination(ctx, &o)
	if e != nil {
		return e
	}
	if e = migrationCLIPrintCapabilities(ctx, out, "Source", s); e != nil {
		return e
	}
	if e = migrationCLIPrintCapabilities(ctx, out, "Destination", d); e != nil {
		return e
	}
	sel := migration.Selection{Source: o.source.Locator, Destination: d.Locator(), Refs: o.refs, Kinds: o.kinds, Mappings: o.mappings}
	l := migration.DefaultLimits()
	snapshot, blobs, e := s.Snapshot(ctx, sel, l)
	if e != nil {
		return e
	}
	if closer, ok := blobs.(io.Closer); ok {
		defer closer.Close()
	}
	for _, ref := range snapshot.NativeRefs {
		if ref.Kind == "code" && migrationCLIForeign(o.destination.Locator.Kind) && o.remote == "" {
			if !o.guided {
				return fmt.Errorf("saved/selected code requires --git-remote and explicit --map FULL_SOURCE_REF=FULL_DESTINATION_REF")
			}
			if e = w.fill(&o.remote, "destination Git remote (absolute path or HTTPS)", false); e != nil {
				return e
			}
			d, e = migrationCLIDestination(ctx, &o)
			if e != nil {
				return e
			}
			if e = migrationCLIPrintCapabilities(ctx, out, "Composed destination", d); e != nil {
				return e
			}
			break
		}
	}
	for attempt := 0; ; attempt++ {
		if attempt >= 32 {
			return fmt.Errorf("mapping revision bound exceeded; start a new explicit plan")
		}
		intents, findings, e := d.Plan(ctx, snapshot, sel, l)
		if e != nil {
			return e
		}
		findings = append(append([]migration.Finding{}, snapshot.Findings...), findings...)
		fmt.Fprintln(out, "Preview (read only; decisions apply to these findings):")
		if e = migrationCLIJSON(out, struct {
			Intents  []migration.Intent  `json:"intents"`
			Findings []migration.Finding `json:"findings"`
		}{intents, findings}); e != nil {
			return e
		}
		replan := false
		decisions := []migration.Decision{}
		known := map[string]bool{}
		for _, f := range findings {
			known[f.ID] = true
			action, exists := o.decisions[f.ID]
			if !exists && (f.Status == "mapped" || f.Status == "preserved") {
				continue
			}
			if !exists && o.guided {
				action, e = w.answer("Decision for "+f.ID+" ["+f.Status+"]: acknowledge, preserve, or map", true)
				if e != nil {
					return e
				}
				if action == "map" {
					if e = w.mappings(sel.Mappings); e != nil {
						return e
					}
					replan = true
					break
				}
				o.decisions[f.ID] = action
			}
			if action == "" {
				return fmt.Errorf("explicit decision required: --decision %s=preserve (or acknowledge/map); inspect its finding first", f.ID)
			}
			if action != "preserve" && action != "acknowledge" && action != "map" {
				return fmt.Errorf("invalid decision for %s", f.ID)
			}
			if action == "map" && f.Status != "mapped" {
				return fmt.Errorf("finding %s is still %s: supply concrete --map choices that the adapter can represent, or explicitly preserve/acknowledge its omission", f.ID, f.Status)
			}
			if f.ID == "local/governance" && action != "preserve" {
				return fmt.Errorf("local/governance requires preserve; governance activation is a separate explicit native action")
			}
			mapping, _ := json.Marshal(sel.Mappings)
			decisions = append(decisions, migration.Decision{FindingID: f.ID, Action: action, Mapping: mapping})
		}
		if replan {
			continue
		}
		for id := range o.decisions {
			if !known[id] {
				return fmt.Errorf("decision names unknown finding %q", id)
			}
		}
		session, e := PrepareMigration(ctx, o.session, snapshot, blobs, sel, d, l, decisions)
		if e != nil {
			return e
		}
		c := o.destination
		c.Locator = d.Locator()
		if e = saveMigrationCLI(session, c); e != nil {
			return fmt.Errorf("plan saved but CLI metadata unavailable; inspect remains possible: %w", e)
		}
		fmt.Fprintln(out, "Saved plan:", session.Directory)
		if e = migrationCLIJSON(out, session.Plan); e != nil {
			return e
		}
		if !o.guided {
			return nil
		}
		answer, e := w.answer("Apply this saved plan now? Type APPLY, or NO to leave it saved", true)
		if e != nil {
			return e
		}
		if answer == "NO" {
			return nil
		}
		if answer != "APPLY" {
			return fmt.Errorf("apply refused: exact APPLY confirmation required")
		}
		if e = ApplyMigration(ctx, session, d, o.actor); e != nil {
			return e
		}
		_, e = fmt.Fprintln(out, "Saved migration intents confirmed:", session.Directory)
		return e
	}
}

func migrationCLISource(ctx context.Context, o *migrationCLIOptions) (migration.Source, error) {
	c := &o.source
	if e := migrationCLIProfile("source", *c); e != nil {
		return nil, e
	}
	if !migrationCLIForeign(c.Locator.Kind) && o.sourceTokenEnv != "" {
		return nil, fmt.Errorf("--source-token-env applies only to a selected foreign adapter")
	}
	var s migration.Source
	switch c.Locator.Kind {
	case "local", "imported":
		if c.Locator.Repository == "" {
			return nil, fmt.Errorf("missing --source-repository")
		}
		dir, e := migrationDestinationGitDir(ctx, c.Locator.Repository)
		if e != nil {
			return nil, e
		}
		if c.Locator.Kind == "imported" {
			s = ImportedMigrationSource{GitDir: dir, SnapshotDigests: o.snapshots}
		} else {
			s = LocalMigrationSource{GitDir: dir}
		}
	case "archive":
		if c.Locator.Repository == "" {
			return nil, fmt.Errorf("missing --source-repository archive path")
		}
		s = ArchiveMigrationSource{Path: c.Locator.Repository}
	case "saved":
		if c.Locator.Repository == "" {
			return nil, fmt.Errorf("missing --source-repository session directory")
		}
		s = migrationSavedSource{directory: c.Locator.Repository}
	default:
		if e := migrationCLICredentials(c, o.sourceTokenEnv); e != nil {
			return nil, e
		}
		if o.attachmentFile != "" {
			b, e := migrationCLIReadFile(o.attachmentFile, 1<<20)
			if e != nil {
				return nil, e
			}
			if e = migration.StrictJSON(b, &c.Attachments); e != nil {
				return nil, e
			}
		}
		var e error
		s, e = OpenMigrationSource(*c)
		if e != nil {
			return nil, fmt.Errorf("source configuration/credential required or invalid: %w", e)
		}
	}
	if o.sourceGit != "" {
		if len(o.refs) == 0 {
			return nil, fmt.Errorf("--source-git requires explicit --ref selections")
		}
		if c.Locator.Kind == "local" || c.Locator.Kind == "archive" || c.Locator.Kind == "saved" {
			return nil, fmt.Errorf("source already contains Git material; --source-git is only for provider/imported companions")
		}
		dir, e := migrationDestinationGitDir(ctx, o.sourceGit)
		if e != nil {
			return nil, e
		}
		s = ComposeMigrationSources(s, LocalMigrationSource{GitDir: dir})
	} else if len(o.refs) > 0 && migrationCLIForeign(c.Locator.Kind) {
		return nil, fmt.Errorf("provider refs require --source-git with already-present code; migration never clones implicitly")
	}
	return s, nil
}
func migrationCLIGitCredentials(o migrationCLIOptions) (*GitHTTPSCredentials, error) {
	if o.gitAuthEnv == "" && o.gitOrigin == "" && o.gitCA == "" {
		return nil, nil
	}
	if o.gitAuthEnv == "" || o.gitOrigin == "" {
		return nil, fmt.Errorf("Git credentials require --git-origin and --git-auth-env")
	}
	value := os.Getenv(o.gitAuthEnv)
	if value == "" {
		return nil, fmt.Errorf("explicit Git credential environment is empty")
	}
	return &GitHTTPSCredentials{Origin: o.gitOrigin, Authorization: value, CAFile: o.gitCA}, nil
}
func migrationCLIDestination(ctx context.Context, o *migrationCLIOptions) (migration.Destination, error) {
	c := &o.destination
	if e := migrationCLIProfile("destination", *c); e != nil {
		return nil, e
	}
	if !migrationCLIForeign(c.Locator.Kind) && o.destinationTokenEnv != "" {
		return nil, fmt.Errorf("--destination-token-env applies only to a selected foreign adapter")
	}
	var d migration.Destination
	switch c.Locator.Kind {
	case "local", "archive":
		if c.Locator.Repository == "" {
			return nil, fmt.Errorf("missing --destination-repository")
		}
		abs, e := filepath.Abs(c.Locator.Repository)
		if e != nil {
			return nil, e
		}
		c.Locator.Repository = abs
		if c.Locator.Kind == "local" {
			d = LocalMigrationDestination{Repository: abs}
		} else {
			d = ArchiveMigrationDestination{Path: abs}
		}
	default:
		if e := migrationCLICredentials(c, o.destinationTokenEnv); e != nil {
			return nil, e
		}
		var e error
		d, e = OpenMigrationDestination(*c)
		if e != nil {
			return nil, fmt.Errorf("destination configuration/credential required or invalid: %w", e)
		}
	}
	if o.remote != "" {
		auth, e := migrationCLIGitCredentials(*o)
		if e != nil {
			return nil, e
		}
		d = ComposeMigrationDestinations(d, GitMigrationDestination{Remote: o.remote, Destination: d.Locator(), Mappings: o.mappings, Credentials: auth})
	}
	return d, nil
}

// This is adapter reconstruction metadata, never a second plan/progress store.
// The closed allowlist deliberately has no Token or environment/config fields.
type migrationCLIDescriptor struct {
	Schema         string            `json:"schema"`
	PlanDigest     string            `json:"plan_digest"`
	Destination    migration.Locator `json:"destination"`
	APIVersion     string            `json:"api_version"`
	CredentialMode string            `json:"credential_mode"`
}

func saveMigrationCLI(s *MigrationSession, c migration.ProviderConfig) error {
	if c.Locator != s.Plan.Destination {
		return fmt.Errorf("CLI destination differs from plan")
	}
	if e := migrationCLIProfile("destination", c); e != nil {
		return e
	}
	b, e := migration.CanonicalPlan(s.Plan)
	if e != nil {
		return e
	}
	d := migrationCLIDescriptor{"hn.migration-cli/1", migration.Digest(b), c.Locator, c.APIVersion, c.CredentialMode}
	raw, e := json.Marshal(d)
	if e != nil {
		return e
	}
	if len(raw) > 16384 {
		return fmt.Errorf("CLI descriptor exceeds bound")
	}
	return migrationSeal(s.Directory, "cli.json", raw)
}
func loadMigrationCLI(s *MigrationSession) (migration.ProviderConfig, error) {
	raw, e := migrationUnseal(s.Directory, "cli.json", 16384)
	if e != nil {
		return migration.ProviderConfig{}, fmt.Errorf("saved CLI descriptor missing/invalid; inspect plan and recover explicitly: %w", e)
	}
	var d migrationCLIDescriptor
	if e = migration.StrictJSON(raw, &d); e != nil {
		return migration.ProviderConfig{}, e
	}
	canonical, e := json.Marshal(d)
	if e != nil || !bytes.Equal(canonical, raw) {
		return migration.ProviderConfig{}, fmt.Errorf("noncanonical or incomplete CLI descriptor")
	}
	b, e := migration.CanonicalPlan(s.Plan)
	if e != nil {
		return migration.ProviderConfig{}, e
	}
	if d.Schema != "hn.migration-cli/1" || d.PlanDigest != migration.Digest(b) || d.Destination != s.Plan.Destination {
		return migration.ProviderConfig{}, fmt.Errorf("CLI descriptor does not bind saved plan/destination")
	}
	c := migration.ProviderConfig{Locator: d.Destination, APIVersion: d.APIVersion, CredentialMode: d.CredentialMode}
	if e = migrationCLIProfile("destination", c); e != nil {
		return c, e
	}
	return c, nil
}
func applyMigrationCLI(ctx context.Context, o migrationCLIOptions, w *migrationWizard, out io.Writer) error {
	if o.session == "" {
		return fmt.Errorf("missing --session DIRECTORY")
	}
	if o.source.Locator.Kind != "" || o.destination.Locator.Kind != "" || o.remote != "" || len(o.refs) > 0 || len(o.kinds) > 0 || len(o.mappings) > 0 || len(o.decisions) > 0 || o.destination.APIVersion != "" || o.destination.CredentialMode != "" || o.destination.Locator.Endpoint != "" || o.destination.Locator.Repository != "" || o.destination.Locator.Organization != "" || o.destination.Locator.Project != "" {
		return fmt.Errorf("apply/resume uses the exact saved intents and destination profile; selection overrides are refused")
	}
	s, e := OpenMigrationSession(ctx, o.session)
	if e != nil {
		return e
	}
	c, e := loadMigrationCLI(s)
	if e != nil {
		return e
	}
	o.destination = c
	if e = migrationCLIJSON(out, s.Plan); e != nil {
		return e
	}
	if !o.confirm {
		if !o.guided {
			return fmt.Errorf("missing --confirm-apply; inspect saved plan before authorizing effects")
		}
		v, e := w.answer("Apply this exact saved plan? Type APPLY", true)
		if e != nil {
			return e
		}
		if v != "APPLY" {
			return fmt.Errorf("apply refused: exact APPLY confirmation required")
		}
	}
	// GitWrite is the sole saved authority for the transport remote and refs.
	for _, in := range s.Plan.Intents {
		if in.Kind == "git_refs" {
			var g migration.GitWrite
			if e = migration.StrictJSON(in.Payload, &g); e != nil {
				return e
			}
			if o.remote != "" && o.remote != g.Remote {
				return fmt.Errorf("conflicting saved Git remotes")
			}
			o.remote = g.Remote
		}
	}
	d, e := migrationCLIDestination(ctx, &o)
	if e != nil {
		return e
	}
	if e = migrationCLIPrintCapabilities(ctx, out, "Destination", d); e != nil {
		return e
	}
	if e = ApplyMigration(ctx, s, d, o.actor); e != nil {
		return e
	}
	_, e = fmt.Fprintln(out, "Saved migration intents confirmed:", s.Directory)
	return e
}

type migrationSavedSource struct{ directory string }

func (s migrationSavedSource) Capabilities(ctx context.Context) ([]migration.Capability, error) {
	return []migration.Capability{{Kind: "project", Read: true, Preserve: true, Limits: []string{"authenticated saved snapshot, exact immutable scope and verified blobs", "source services and credentials are not required"}, Reason: "inspectable preserved source observation"}}, nil
}
func (s migrationSavedSource) Snapshot(ctx context.Context, _ migration.Selection, _ migration.Limits) (migration.Snapshot, migration.BlobSource, error) {
	v, e := OpenMigrationSession(ctx, s.directory)
	if e != nil {
		return migration.Snapshot{}, nil, e
	}
	return InspectMigration(ctx, v)
}
func migrationCLIReadFile(path string, max int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, max+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("input file exceeds bound")
	}
	return b, nil
}
func inspectMigrationCLI(ctx context.Context, o migrationCLIOptions, out io.Writer) error {
	l := migration.DefaultLimits()
	var s migration.Snapshot
	var b migration.BlobSource
	var p *migration.Plan
	selected := 0
	for _, v := range []bool{o.session != "", o.archive != "", o.repository != ""} {
		if v {
			selected++
		}
	}
	if selected != 1 {
		return fmt.Errorf("inspect requires exactly one --session, --archive, or --repository with --snapshot")
	}
	if o.session != "" {
		v, e := OpenMigrationSession(ctx, o.session)
		if e != nil {
			return e
		}
		s, b, e = InspectMigration(ctx, v)
		if e != nil {
			return e
		}
		p = &v.Plan
	} else if o.archive != "" {
		var e error
		s, b, e = (ArchiveMigrationSource{Path: o.archive}).Snapshot(ctx, migration.Selection{}, l)
		if e != nil {
			return e
		}
	} else {
		if len(o.snapshots) != 1 {
			return fmt.Errorf("inspect --repository requires exactly one --snapshot")
		}
		dir, e := migrationDestinationGitDir(ctx, o.repository)
		if e != nil {
			return e
		}
		a, e := ReadLocalMigration(ctx, dir, o.snapshots[0], l)
		if e != nil {
			return e
		}
		s = a.Snapshot
		b = a
	}
	if closer, ok := b.(io.Closer); ok {
		defer closer.Close()
	}
	if o.blob != "" {
		r, e := b.Open(ctx, o.blob)
		if e != nil {
			return e
		}
		defer r.Close()
		_, e = io.Copy(out, r)
		return e
	}
	return migrationCLIJSON(out, struct {
		Schema   string             `json:"schema"`
		Plan     *migration.Plan    `json:"plan,omitempty"`
		Snapshot migration.Snapshot `json:"snapshot"`
	}{"hn.migration-inspect/1", p, s})
}
