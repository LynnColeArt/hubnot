// Package release creates deterministic local Hubnot release assets using Go and Git.
package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Options names the complete release inputs. Output must not already exist.
// Empty Targets/Variants select all supported targets/variants.
type Options struct {
	Repository string
	Revision   string
	Version    string
	Output     string
	Targets    []string
	Variants   []string
}

// BuildProvenance is included beside the executable; it deliberately does not
// claim a digest of its containing archive.
type BuildProvenance struct {
	Schema           string   `json:"schema"`
	SourceCommit     string   `json:"source_commit"`
	SourceTree       string   `json:"source_tree"`
	SourceTimestamp  int64    `json:"source_timestamp"`
	Compiler         string   `json:"compiler"`
	Version          string   `json:"version"`
	Target           string   `json:"target"`
	Variant          string   `json:"variant"`
	Flags            []string `json:"flags"`
	Environment      []string `json:"environment"`
	ExecutableSHA256 string   `json:"executable_sha256"`
}

// Asset binds a generated archive's bytes. Only binary assets have Build set.
type Asset struct {
	Name   string           `json:"name"`
	SHA256 string           `json:"sha256"`
	Build  *BuildProvenance `json:"build,omitempty"`
}

// Provenance inventories archives without making a circular self-digest claim.
// SHA256SUMS separately covers this manifest and every archive.
type Provenance struct {
	Schema          string  `json:"schema"`
	SourceCommit    string  `json:"source_commit"`
	SourceTree      string  `json:"source_tree"`
	SourceTimestamp int64   `json:"source_timestamp"`
	Compiler        string  `json:"compiler"`
	Version         string  `json:"version"`
	Assets          []Asset `json:"assets"`
}

var supportedTargets = []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"}
var supportedVariants = []string{"native-only", "standard"}
var releaseVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

type entry struct {
	name string
	mode int64
	data []byte
}

type packager struct {
	git, goTool string
	env         []string
	repository  string
}

// Package resolves one committed tree, builds it in isolation, and writes a new
// directory of archives, provenance and checksums. It never publishes or tags.
// Tracked working/index changes are refused; untracked/ignored files are excluded.
func Package(ctx context.Context, options Options) error {
	if !releaseVersion.MatchString(options.Version) || len(options.Version) > 100 {
		return errors.New("version must be X.Y.Z or X.Y.Z-prerelease (without a leading v)")
	}
	if options.Revision == "" || strings.HasPrefix(options.Revision, "-") || strings.ContainsAny(options.Revision, "\x00\r\n") {
		return errors.New("an explicit committed source revision is required")
	}
	if options.Output == "" {
		return errors.New("an explicit new output directory is required")
	}
	targets, err := selectValues(options.Targets, supportedTargets, "target")
	if err != nil {
		return err
	}
	variants, err := selectValues(options.Variants, supportedVariants, "variant")
	if err != nil {
		return err
	}
	output, err := filepath.Abs(options.Output)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		if err != nil {
			return err
		}
		return fmt.Errorf("output already exists: %s", output)
	}
	repository, err := filepath.Abs(options.Repository)
	if err != nil {
		return err
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("Git is required: %w", err)
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("Go is required: %w", err)
	}
	git, err = filepath.Abs(git)
	if err != nil {
		return err
	}
	goTool, err = filepath.Abs(goTool)
	if err != nil {
		return err
	}
	p := packager{git: git, goTool: goTool, env: controlledEnvironment(), repository: repository}
	status, err := p.gitCommand(ctx, "status", "--porcelain=v1", "--untracked-files=no", "--ignore-submodules=none")
	if err != nil {
		return err
	}
	if len(status) != 0 {
		return errors.New("source has tracked working-tree or index changes; commit or restore them before packaging")
	}
	commitBytes, err := p.gitCommand(ctx, "rev-parse", "--verify", "--end-of-options", options.Revision+"^{commit}")
	if err != nil {
		return fmt.Errorf("resolve source commit: %w", err)
	}
	commit := strings.TrimSpace(string(commitBytes))
	treeBytes, err := p.gitCommand(ctx, "rev-parse", "--verify", commit+"^{tree}")
	if err != nil {
		return err
	}
	timestampBytes, err := p.gitCommand(ctx, "show", "-s", "--no-show-signature", "--no-notes", "--format=%ct", commit)
	if err != nil {
		return err
	}
	timestamp, err := strconv.ParseInt(strings.TrimSpace(string(timestampBytes)), 10, 64)
	if err != nil || timestamp < 0 {
		return errors.New("source has an unsupported commit timestamp")
	}
	compilerBytes, err := p.command(ctx, p.goTool, repository, nil, "env", "GOVERSION")
	if err != nil {
		return err
	}
	compiler := strings.TrimSpace(string(compilerBytes))
	var major, minor int
	if _, err := fmt.Sscanf(compiler, "go%d.%d", &major, &minor); err != nil || major != 1 || minor < 26 {
		return fmt.Errorf("supported Go 1.26+ compiler required, got %q", compiler)
	}
	files, err := p.readTree(ctx, commit)
	if err != nil {
		return err
	}
	if err := checkVersionVariable(files); err != nil {
		return err
	}
	for _, name := range []string{"go.mod", "LICENSE", "README.md"} {
		if !hasEntry(files, name) {
			return fmt.Errorf("source lacks required %s", name)
		}
	}
	work, err := os.MkdirTemp("", "hn-release-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	source := filepath.Join(work, "source")
	if err := extract(source, files); err != nil {
		return err
	}
	// Stage on the destination filesystem. A final exclusive mkdir claims the
	// output only after all builds succeed; collisions never replace old files.
	stage, err := os.MkdirTemp(filepath.Dir(output), ".hn-release-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	manifest := Provenance{Schema: "hn.release/1", SourceCommit: commit, SourceTree: strings.TrimSpace(string(treeBytes)), SourceTimestamp: timestamp, Compiler: compiler, Version: options.Version}
	stamp := time.Unix(timestamp, 0).UTC()
	sourceName := "hubnot_" + options.Version + "_source"
	if err := archive(filepath.Join(stage, sourceName+".tar.gz"), sourceName, files, stamp); err != nil {
		return err
	}
	asset, err := archiveAsset(stage, sourceName+".tar.gz", nil)
	if err != nil {
		return err
	}
	manifest.Assets = append(manifest.Assets, asset)
	for _, target := range targets {
		for _, variant := range variants {
			if err := ctx.Err(); err != nil {
				return err
			}
			build, err := p.build(ctx, source, work, options.Version, target, variant, manifest)
			if err != nil {
				return err
			}
			name := "hubnot_" + options.Version + "_" + strings.ReplaceAll(target, "/", "_")
			if variant == "native-only" {
				name += "_native-only"
			}
			binary, err := os.ReadFile(filepath.Join(work, "hn"))
			if err != nil {
				return err
			}
			metadata, err := jsonBytes(build)
			if err != nil {
				return err
			}
			contents := []entry{{name: "hn", mode: 0755, data: binary}, {name: "BUILD.json", mode: 0644, data: metadata}, {name: "INSTALL.md", mode: 0644, data: installation(options.Version, target, variant)}}
			for _, e := range files {
				if e.name == "LICENSE" || e.name == "README.md" || e.name == "CHANGELOG.md" || strings.HasPrefix(e.name, "docs/") {
					contents = append(contents, e)
				}
			}
			if err := archive(filepath.Join(stage, name+".tar.gz"), name, contents, stamp); err != nil {
				return err
			}
			asset, err := archiveAsset(stage, name+".tar.gz", &build)
			if err != nil {
				return err
			}
			manifest.Assets = append(manifest.Assets, asset)
		}
	}
	sort.Slice(manifest.Assets, func(i, j int) bool { return manifest.Assets[i].Name < manifest.Assets[j].Name })
	metadata, err := jsonBytes(manifest)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "PROVENANCE.json"), metadata, 0644); err != nil {
		return err
	}
	if err := writeChecksums(stage); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Mkdir(output, 0755); err != nil {
		return fmt.Errorf("claim output directory: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(output)
		}
	}()
	assets, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	for _, asset := range assets {
		if err := os.Rename(filepath.Join(stage, asset.Name()), filepath.Join(output, asset.Name())); err != nil {
			return err
		}
	}
	complete = true
	return nil
}

func selectValues(requested, supported []string, kind string) ([]string, error) {
	if len(requested) == 0 {
		return append([]string(nil), supported...), nil
	}
	seen := map[string]bool{}
	for _, value := range requested {
		if seen[value] {
			return nil, fmt.Errorf("duplicate %s %q", kind, value)
		}
		found := false
		for _, allowed := range supported {
			if value == allowed {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unsupported %s %q (supported: %s)", kind, value, strings.Join(supported, ", "))
		}
		seen[value] = true
	}
	result := append([]string(nil), requested...)
	sort.Strings(result)
	return result, nil
}

func controlledEnvironment() []string {
	// Preserve only host executable/cache/temp locations, never ambient Go or Git
	// flags, workspace selection, credentials or repository overrides.
	env := []string{}
	for _, key := range []string{"PATH", "HOME", "USERPROFILE", "LOCALAPPDATA", "APPDATA", "SYSTEMROOT", "WINDIR", "TMP", "TEMP", "TMPDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return append(env, "GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GONOPROXY=none", "GOPRIVATE=none", "CGO_ENABLED=0", "GOFLAGS=", "GOEXPERIMENT=", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_NO_REPLACE_OBJECTS=1")
}

func (p packager) command(ctx context.Context, tool, dir string, extra []string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, tool, args...)
	c.Dir = dir
	c.Env = append(append([]string(nil), p.env...), extra...)
	var stderr bytes.Buffer
	c.Stderr = &stderr
	output, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %v: %w: %s", filepath.Base(tool), args, err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

func (p packager) gitCommand(ctx context.Context, args ...string) ([]byte, error) {
	return p.command(ctx, p.git, p.repository, nil, append([]string{"--no-optional-locks", "-c", "core.fsmonitor=false"}, args...)...)
}

func (p packager) readTree(ctx context.Context, commit string) ([]entry, error) {
	listing, err := p.gitCommand(ctx, "ls-tree", "-r", "-z", "--full-tree", commit)
	if err != nil {
		return nil, err
	}
	files := []entry{}
	seen := map[string]string{}
	for _, raw := range bytes.Split(listing, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		header, name, ok := strings.Cut(string(raw), "\t")
		fields := strings.Fields(header)
		if !ok || len(fields) != 3 {
			return nil, errors.New("malformed Git tree listing")
		}
		if err := portablePath(name); err != nil {
			return nil, err
		}
		// Include directory components in collision detection: a/F and A/G
		// collide on Windows even though their complete paths differ.
		parts := strings.Split(name, "/")
		for i := range parts {
			component := strings.Join(parts[:i+1], "/")
			key := strings.ToLower(component)
			if prior, ok := seen[key]; ok && prior != component {
				return nil, fmt.Errorf("case-colliding source paths %q and %q", prior, component)
			}
			seen[key] = component
		}
		mode := int64(0644)
		switch fields[0] {
		case "100644":
		case "100755":
			mode = 0755
		default:
			return nil, fmt.Errorf("source member %q is not a regular file (mode %s); links and submodules are unsupported", name, fields[0])
		}
		if fields[1] != "blob" {
			return nil, fmt.Errorf("source member %q is not a blob", name)
		}
		data, err := p.gitCommand(ctx, "cat-file", "blob", fields[2])
		if err != nil {
			return nil, err
		}
		files = append(files, entry{name: name, mode: mode, data: data})
	}
	return files, nil
}

func portablePath(name string) error {
	if name == "" || !utf8.ValidString(name) || strings.ContainsAny(name, "\\:\x00\r\n") || path.IsAbs(name) || path.Clean(name) != name {
		return fmt.Errorf("nonportable source path %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "." || part == ".." || part == "" || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || strings.ContainsAny(part, "<>\"|?*") || strings.EqualFold(part, ".git") {
			return fmt.Errorf("nonportable source path %q", name)
		}
		for _, r := range part {
			if r < 32 {
				return fmt.Errorf("nonportable source path %q", name)
			}
		}
		stem := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || (len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9') {
			return fmt.Errorf("Windows-reserved source path %q", name)
		}
	}
	return nil
}

func hasEntry(files []entry, name string) bool {
	for _, e := range files {
		if e.name == name {
			return true
		}
	}
	return false
}

func checkVersionVariable(files []entry) error {
	for _, e := range files {
		if e.name != "main.go" {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), e.name, e.data, 0)
		if err != nil {
			return err
		}
		if file.Name.Name != "main" {
			break
		}
		for _, decl := range file.Decls {
			g, ok := decl.(*ast.GenDecl)
			if !ok || g.Tok != token.VAR {
				continue
			}
			for _, spec := range g.Specs {
				v, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range v.Names {
					if name.Name == "version" {
						if v.Type != nil {
							kind, ok := v.Type.(*ast.Ident)
							if !ok || kind.Name != "string" {
								break
							}
							if len(v.Values) == 0 {
								return nil
							}
						}
						if i < len(v.Values) {
							value, ok := v.Values[i].(*ast.BasicLit)
							if ok && value.Kind == token.STRING {
								return nil
							}
						}
					}
				}
			}
		}
	}
	return errors.New("source must declare a linker-settable version variable in main.go; historical releases are not repackaged implicitly")
}

func extract(dir string, files []entry) error {
	for _, e := range files {
		name := filepath.Join(dir, filepath.FromSlash(e.name))
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(name, e.data, os.FileMode(e.mode)); err != nil {
			return err
		}
		if err := os.Chmod(name, os.FileMode(e.mode)); err != nil {
			return err
		}
	}
	return nil
}

func (p packager) build(ctx context.Context, source, work, version, target, variant string, release Provenance) (BuildProvenance, error) {
	flags := []string{"-mod=readonly", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid= -X main.version=" + version}
	if variant == "native-only" {
		flags = append(flags, "-tags=nogithub,noforge")
	}
	platform := strings.Split(target, "/")
	env := []string{"GOOS=" + platform[0], "GOARCH=" + platform[1], "GOAMD64=v1", "GOARM64=v8.0"}
	args := append([]string{"build"}, flags...)
	args = append(args, "-o", filepath.Join(work, "hn"), ".")
	if _, err := p.command(ctx, p.goTool, source, env, args...); err != nil {
		return BuildProvenance{}, err
	}
	digest, err := fileDigest(filepath.Join(work, "hn"))
	if err != nil {
		return BuildProvenance{}, err
	}
	// Record controlled build inputs, excluding host cache/temp paths and the
	// intentionally variable -o path, neither of which is compiled into output.
	buildEnv := append([]string{"CGO_ENABLED=0", "GOENV=off", "GOEXPERIMENT=", "GOFLAGS=", "GONOPROXY=none", "GOPRIVATE=none", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off"}, env...)
	sort.Strings(buildEnv)
	return BuildProvenance{Schema: "hn.release-build/1", SourceCommit: release.SourceCommit, SourceTree: release.SourceTree, SourceTimestamp: release.SourceTimestamp, Compiler: release.Compiler, Version: version, Target: target, Variant: variant, Flags: flags, Environment: buildEnv, ExecutableSHA256: digest}, nil
}

func installation(version, target, variant string) []byte {
	return []byte(fmt.Sprintf("# Install Hubnot %s\n\nTarget: %s. Variant: %s.\n\nInstall the included hn executable in a directory on PATH. Git 2.x is required.\nRun `hn version` to check the version. Product documentation is in README.md and docs/.\nNative-only excludes all network adapters; standard includes installed adapters.\nThese builds are unsigned and are not Apple-notarized. Cross-compilation alone\ndoes not establish runtime qualification; consult release verification evidence.\n", version, target, variant))
}

func archive(filename, prefix string, files []entry, stamp time.Time) (err error) {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	gz := gzip.NewWriter(f)
	gz.Header.OS = 255
	tw := tar.NewWriter(gz)
	ordered := append([]entry(nil), files...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].name < ordered[j].name })
	for i, e := range ordered {
		if i > 0 && ordered[i-1].name == e.name {
			err = fmt.Errorf("duplicate archive member %q", e.name)
			break
		}
		h := tar.Header{Name: prefix + "/" + e.name, Mode: e.mode, Size: int64(len(e.data)), ModTime: stamp, Typeflag: tar.TypeReg, Format: tar.FormatPAX}
		if err = tw.WriteHeader(&h); err != nil {
			break
		}
		if _, err = tw.Write(e.data); err != nil {
			break
		}
	}
	return errors.Join(err, tw.Close(), gz.Close())
}

func jsonBytes(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	return append(data, '\n'), err
}

func fileDigest(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func archiveAsset(dir, name string, build *BuildProvenance) (Asset, error) {
	digest, err := fileDigest(filepath.Join(dir, name))
	return Asset{Name: name, SHA256: digest, Build: build}, err
}

func writeChecksums(dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var sums strings.Builder
	for _, file := range files {
		digest, err := fileDigest(filepath.Join(dir, file.Name()))
		if err != nil {
			return err
		}
		fmt.Fprintf(&sums, "%s  %s\n", digest, file.Name())
	}
	return os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sums.String()), 0644)
}
