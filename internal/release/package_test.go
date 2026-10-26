package release_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"hubnot/internal/release"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-01-02T03:04:05Z", "GIT_COMMITTER_DATE=2026-01-02T03:04:05Z")
	b, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}

func fixture(t *testing.T) (string, string) {
	t.Helper()
	d := t.TempDir()
	files := map[string]string{
		"go.mod":  "module fixture\n\ngo 1.26\n",
		"main.go": "package main\nimport (\"fmt\";\"os\")\nvar version = \"0.0.1-dev\"\nfunc main(){if len(os.Args)>1 && os.Args[1]==\"version\" {fmt.Println(version)}}\n",
		"LICENSE": "test license\n", "README.md": "test product documentation\n",
		"docs/guide.md": "product guide\n", ".gitignore": "ignored\n",
		".gitattributes": "README.md export-ignore\n",
		"run":            "committed executable\n",
	}
	for name, data := range files {
		p := filepath.Join(d, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, d, "init", "-q")
	if err := os.Chmod(filepath.Join(d, "run"), 0755); err != nil {
		t.Fatal(err)
	}
	git(t, d, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "add", ".")
	git(t, d, "update-index", "--chmod=+x", "run")
	git(t, d, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	return d, git(t, d, "rev-parse", "HEAD")
}

type member struct {
	header tar.Header
	data   []byte
}

func inspect(t *testing.T, file string) map[string]member {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	if !gz.ModTime.IsZero() || gz.Name != "" || gz.Comment != "" || gz.OS != 255 {
		t.Fatalf("unnormalized gzip header: %+v", gz.Header)
	}
	tr := tar.NewReader(gz)
	got := map[string]member{}
	previous := ""
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Name <= previous || strings.Contains(h.Name, "\\") || strings.HasPrefix(h.Name, "/") {
			t.Fatalf("noncanonical archive path %q after %q", h.Name, previous)
		}
		if h.Typeflag != tar.TypeReg || h.Uid != 0 || h.Gid != 0 || h.Uname != "" || h.Gname != "" || !h.AccessTime.IsZero() || !h.ChangeTime.IsZero() {
			t.Fatalf("unnormalized tar header: %+v", h)
		}
		if !h.ModTime.Equal(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)) {
			t.Fatalf("wrong timestamp %s", h.ModTime)
		}
		if h.Mode != 0644 && h.Mode != 0755 {
			t.Fatalf("wrong mode %o", h.Mode)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		got[h.Name] = member{*h, data}
		previous = h.Name
	}
	return got
}

func TestPackageReproducibleExactSourceAndProvenance(t *testing.T) {
	d, rev := fixture(t)
	// Untracked/ignored material and hostile ambient build flags are not inputs.
	for _, name := range []string{"untracked-secret", "ignored"} {
		if err := os.WriteFile(filepath.Join(d, name), []byte("private"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GOFLAGS", "-this-flag-must-not-reach-build")
	t.Setenv("GOOS", "invalid")
	t.Setenv("GOWORK", filepath.Join(d, "missing.work"))
	root := t.TempDir()
	opts := release.Options{Repository: d, Revision: rev, Version: "1.2.3-rc.1", Targets: []string{"linux/amd64"}, Variants: []string{"standard", "native-only"}}
	for _, name := range []string{"first", "different-path"} {
		opts.Output = filepath.Join(root, name)
		if err := release.Package(context.Background(), opts); err != nil {
			t.Fatal(err)
		}
	}
	first := filepath.Join(root, "first")
	entries, err := os.ReadDir(first)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("got %d assets, want two binaries + source + provenance + checksums", len(entries))
	}
	for _, e := range entries {
		a, err := os.ReadFile(filepath.Join(first, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(root, "different-path", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("nonreproducible asset %s", e.Name())
		}
	}
	source := inspect(t, filepath.Join(first, "hubnot_1.2.3-rc.1_source.tar.gz"))
	prefix := "hubnot_1.2.3-rc.1_source/"
	if _, ok := source[prefix+"README.md"]; !ok {
		t.Fatal("export-ignore changed exact source selection")
	}
	if source[prefix+"run"].header.Mode != 0755 {
		t.Fatal("committed executable mode lost")
	}
	if _, ok := source[prefix+"untracked-secret"]; ok {
		t.Fatal("untracked data leaked")
	}
	if _, ok := source[prefix+"ignored"]; ok {
		t.Fatal("ignored data leaked")
	}
	for _, variant := range []string{"standard", "native-only"} {
		suffix := ""
		if variant == "native-only" {
			suffix = "_native-only"
		}
		name := "hubnot_1.2.3-rc.1_linux_amd64" + suffix
		members := inspect(t, filepath.Join(first, name+".tar.gz"))
		for _, p := range []string{"hn", "LICENSE", "README.md", "docs/guide.md", "INSTALL.md", "BUILD.json"} {
			if _, ok := members[name+"/"+p]; !ok {
				t.Fatalf("%s absent from %s", p, name)
			}
		}
		if members[name+"/hn"].header.Mode != 0755 {
			t.Fatal("binary must be executable")
		}
		var provenance release.BuildProvenance
		if err := json.Unmarshal(members[name+"/BUILD.json"].data, &provenance); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(members[name+"/hn"].data)
		if provenance.SourceCommit != rev || provenance.Version != opts.Version || provenance.Target != "linux/amd64" || provenance.Variant != variant || provenance.ExecutableSHA256 != hex.EncodeToString(digest[:]) {
			t.Fatalf("unbound provenance: %+v", provenance)
		}
		if !strings.HasPrefix(provenance.Compiler, "go1.26") && provenance.Compiler != runtime.Version() {
			t.Fatalf("wrong compiler %q", provenance.Compiler)
		}
		if variant == "native-only" && !strings.Contains(strings.Join(provenance.Flags, " "), "nogithub,noforge") {
			t.Fatal("native-only must exclude GitHub before noforge integration")
		}
		if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
			binary := filepath.Join(t.TempDir(), "hn")
			if err := os.WriteFile(binary, members[name+"/hn"].data, 0755); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(binary, "version").CombinedOutput()
			if err != nil || strings.TrimSpace(string(out)) != opts.Version {
				t.Fatalf("reported version %q: %v", out, err)
			}
		}
	}
	sums, err := os.ReadFile(filepath.Join(first, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(sums)), "\n")
	if len(lines) != 4 {
		t.Fatalf("checksum inventory: %s", sums)
	}
	previous := ""
	for _, line := range lines {
		parts := strings.Split(line, "  ")
		if len(parts) != 2 || parts[1] <= previous {
			t.Fatalf("bad checksum line %q", line)
		}
		data, err := os.ReadFile(filepath.Join(first, parts[1]))
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(data)
		if parts[0] != fmt.Sprintf("%x", h) {
			t.Fatalf("wrong checksum for %s", parts[1])
		}
		previous = parts[1]
	}
	var p release.Provenance
	data, err := os.ReadFile(filepath.Join(first, "PROVENANCE.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	if p.SourceCommit != rev || len(p.Assets) != 3 {
		t.Fatalf("wrong release inventory: %+v", p)
	}
	for _, a := range p.Assets {
		data, err := os.ReadFile(filepath.Join(first, a.Name))
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(data)
		if a.SHA256 != fmt.Sprintf("%x", h) {
			t.Fatal("unbound archive digest")
		}
	}
}

func TestPackageRefusalsPreserveOutput(t *testing.T) {
	d, rev := fixture(t)
	root := t.TempDir()
	base := release.Options{Repository: d, Revision: rev, Version: "1.2.3", Targets: []string{"linux/amd64"}, Variants: []string{"standard"}}
	cases := []struct {
		name   string
		change func(*release.Options)
	}{
		{"version", func(o *release.Options) { o.Version = "1.2;bad" }},
		{"leading-v", func(o *release.Options) { o.Version = "v1.2.3" }},
		{"target", func(o *release.Options) { o.Targets = []string{"windows/amd64"} }},
		{"duplicate-target", func(o *release.Options) { o.Targets = []string{"linux/amd64", "linux/amd64"} }},
		{"variant", func(o *release.Options) { o.Variants = []string{"nogithub"} }},
		{"duplicate-variant", func(o *release.Options) { o.Variants = []string{"standard", "standard"} }},
		{"missing-rev", func(o *release.Options) { o.Revision = "not-a-revision" }},
		{"option-rev", func(o *release.Options) { o.Revision = "--help" }},
		{"no-version", func(o *release.Options) { o.Version = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := base
			o.Output = filepath.Join(root, tc.name)
			tc.change(&o)
			if err := release.Package(context.Background(), o); err == nil {
				t.Fatal("accepted invalid input")
			}
			if _, err := os.Lstat(o.Output); !os.IsNotExist(err) {
				t.Fatal("refusal left output")
			}
		})
	}
	base.Output = filepath.Join(root, "existing")
	if err := os.Mkdir(base.Output, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(base.Output, "keep")
	if err := os.WriteFile(marker, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := release.Package(context.Background(), base); err == nil {
		t.Fatal("overwrote output")
	}
	if b, err := os.ReadFile(marker); err != nil || string(b) != "original" {
		t.Fatal("collision changed output")
	}
	base.Output = filepath.Join(root, "dirty")
	if err := os.WriteFile(filepath.Join(d, "README.md"), []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := release.Package(context.Background(), base); err == nil {
		t.Fatal("accepted tracked dirt")
	}
	if _, err := os.Lstat(base.Output); !os.IsNotExist(err) {
		t.Fatal("dirty refusal left output")
	}
}

func repositoryBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !e.IsDir() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			files[p] = string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestPackageAndCLIRefuseConfiguredFiltersBeforeExecution(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "hn-release")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", binary, "../../cmd/hn-release").CombinedOutput(); err != nil {
		t.Fatalf("build command: %v: %s", err, out)
	}
	for _, boundary := range []string{"Package", "CLI"} {
		for _, tc := range []struct{ driver, scope string }{{"review.clean", "local"}, {"other.driver.process", "include"}, {"worktree.process", "worktree"}} {
			t.Run(boundary+"/"+tc.scope+"/"+tc.driver, func(t *testing.T) {
				driver := tc.driver
				d, _ := fixture(t)
				name := driver[:strings.LastIndex(driver, ".")]
				if err := os.WriteFile(filepath.Join(d, ".gitattributes"), []byte("README.md filter="+name+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
				git(t, d, "add", ".gitattributes")
				git(t, d, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "filter attribute")
				root := t.TempDir()
				marker := filepath.Join(root, "unexpected-execution")
				// Install only after commit; no test setup operation invokes the filter.
				command := fmt.Sprintf("printf executed > %q; cat", filepath.ToSlash(marker))
				switch tc.scope {
				case "include":
					included := filepath.Join(d, ".git", "included-config")
					git(t, d, "config", "--file", included, "filter."+driver, command)
					git(t, d, "config", "include.path", included)
				case "worktree":
					git(t, d, "config", "extensions.worktreeConfig", "true")
					git(t, d, "config", "--worktree", "filter."+driver, command)
				default:
					git(t, d, "config", "filter."+driver, command)
				}
				changed := time.Now().Add(2 * time.Hour)
				if err := os.Chtimes(filepath.Join(d, "README.md"), changed, changed); err != nil {
					t.Fatal(err)
				}
				before := repositoryBytes(t, d)
				var err error
				var message string
				if boundary == "Package" {
					err = release.Package(context.Background(), release.Options{Repository: d, Revision: "HEAD", Version: "1.2.3", Output: filepath.Join(root, "out"), Targets: []string{"linux/amd64"}, Variants: []string{"standard"}})
					if err != nil {
						message = err.Error()
					}
				} else {
					var out []byte
					out, err = exec.Command(binary, "--source", d, "--rev", "HEAD", "--version", "1.2.3", "--out", filepath.Join(root, "out"), "--targets", "linux/amd64", "--variants", "standard").CombinedOutput()
					message = string(out)
				}
				if err == nil || !strings.Contains(message, "filter") {
					t.Errorf("expected filter refusal, got %v: %s", err, message)
				}
				entries, readErr := os.ReadDir(root)
				if readErr != nil || len(entries) != 0 {
					t.Errorf("filter ran or refusal left output/staging: %v %v", entries, readErr)
				}
				after := repositoryBytes(t, d)
				if !reflect.DeepEqual(before, after) {
					t.Error("refusal changed repository bytes, including index/config")
				}
			})
		}
	}
}

func TestPackageRefusesSourcesThatCouldStartNestedHelpers(t *testing.T) {
	for _, kind := range []string{"partial-clone", "promisor", "submodule"} {
		t.Run(kind, func(t *testing.T) {
			d, rev := fixture(t)
			switch kind {
			case "partial-clone":
				git(t, d, "config", "extensions.partialClone", "origin")
			case "promisor":
				git(t, d, "config", "remote.other.promisor", "true")
			case "submodule":
				git(t, d, "update-index", "--add", "--cacheinfo", "160000,"+rev+",nested")
				git(t, d, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "gitlink")
			}
			before := repositoryBytes(t, d)
			root := t.TempDir()
			err := release.Package(context.Background(), release.Options{Repository: d, Revision: "HEAD", Version: "1.2.3", Output: filepath.Join(root, "out"), Targets: []string{"linux/amd64"}, Variants: []string{"standard"}})
			if err == nil || (!strings.Contains(err.Error(), "promisor") && !strings.Contains(err.Error(), "submodule")) {
				t.Errorf("expected helper-boundary refusal: %v", err)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatal("refusal left output/staging")
			}
			if !reflect.DeepEqual(before, repositoryBytes(t, d)) {
				t.Fatal("refusal mutated source")
			}
		})
	}
}

func TestPackageDisablesFilesystemMonitor(t *testing.T) {
	d, rev := fixture(t)
	root := t.TempDir()
	marker := filepath.Join(root, "unexpected-execution")
	git(t, d, "config", "core.fsmonitor", fmt.Sprintf("printf executed > %q", filepath.ToSlash(marker)))
	changed := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(filepath.Join(d, "README.md"), changed, changed); err != nil {
		t.Fatal(err)
	}
	if err := release.Package(context.Background(), release.Options{Repository: d, Revision: rev, Version: "1.2.3", Output: filepath.Join(root, "out"), Targets: []string{"linux/amd64"}, Variants: []string{"standard"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(marker); !os.IsNotExist(err) {
		t.Fatal("filesystem monitor executed")
	}
}

func TestPackageRefusesStagedChanges(t *testing.T) {
	d, rev := fixture(t)
	if err := os.WriteFile(filepath.Join(d, "README.md"), []byte("staged change"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, d, "add", "README.md")
	root := t.TempDir()
	if err := release.Package(context.Background(), release.Options{Repository: d, Revision: rev, Version: "1.2.3", Output: filepath.Join(root, "out")}); err == nil {
		t.Fatal("accepted staged change")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("staged-change refusal left output/staging")
	}
}

func TestPackageRejectsNonportableCommittedTrees(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture names require a Unix host; packager refusal is platform independent")
	}
	for _, name := range []string{"back\\slash", "CON", "trailing.", "case", "link"} {
		t.Run(name, func(t *testing.T) {
			d, _ := fixture(t)
			if name == "link" {
				if err := os.Symlink("README.md", filepath.Join(d, name)); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(filepath.Join(d, name), []byte("hostile"), 0644); err != nil {
					t.Fatal(err)
				}
				if name == "case" {
					if err := os.WriteFile(filepath.Join(d, "CASE"), []byte("collision"), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			git(t, d, "add", ".")
			git(t, d, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "nonportable")
			o := release.Options{Repository: d, Revision: "HEAD", Version: "1.2.3", Output: filepath.Join(t.TempDir(), "release"), Targets: []string{"linux/amd64"}}
			if err := release.Package(context.Background(), o); err == nil {
				t.Fatal("accepted nonportable committed tree")
			}
			if _, err := os.Lstat(o.Output); !os.IsNotExist(err) {
				t.Fatal("tree refusal left output")
			}
		})
	}
}

func TestPackagePinsRequestedCommitDespiteHEADAndReplaceRef(t *testing.T) {
	d, original := fixture(t)
	if err := os.WriteFile(filepath.Join(d, "later-file"), []byte("later commit"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, d, "add", ".")
	git(t, d, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "later")
	later := git(t, d, "rev-parse", "HEAD")
	git(t, d, "replace", original, later)
	out := filepath.Join(t.TempDir(), "pinned")
	if err := release.Package(context.Background(), release.Options{Repository: d, Revision: original, Version: "1.2.3", Output: out, Targets: []string{"linux/amd64"}, Variants: []string{"standard"}}); err != nil {
		t.Fatal(err)
	}
	members := inspect(t, filepath.Join(out, "hubnot_1.2.3_source.tar.gz"))
	if _, ok := members["hubnot_1.2.3_source/later-file"]; ok {
		t.Fatal("HEAD or replace ref silently changed declared source")
	}
}

func TestPackageBuildFailureAndCancellationLeaveNoAssets(t *testing.T) {
	d, _ := fixture(t)
	if err := os.WriteFile(filepath.Join(d, "main.go"), []byte("package main\nvar version = \"dev\"\nfunc main(){missing()}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, d, "add", ".")
	git(t, d, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "broken source")
	root := t.TempDir()
	o := release.Options{Repository: d, Revision: "HEAD", Version: "1.2.3", Output: filepath.Join(root, "output"), Targets: []string{"linux/amd64"}}
	if err := release.Package(context.Background(), o); err == nil {
		t.Fatal("accepted compiler failure")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failure left output or staging: %v %v", entries, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := release.Package(ctx, o); err == nil {
		t.Fatal("ignored cancellation")
	}
	entries, err = os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("cancellation left state")
	}
}

func TestPackageRefusesNonLinkerSettableVersion(t *testing.T) {
	for _, declaration := range []string{`const version = "old"`, `var version = func() string { return "old" }()`, `var version = 1`} {
		t.Run(declaration, func(t *testing.T) {
			d, _ := fixture(t)
			if err := os.WriteFile(filepath.Join(d, "main.go"), []byte("package main\n"+declaration+"\nfunc main(){}\n"), 0644); err != nil {
				t.Fatal(err)
			}
			git(t, d, "add", ".")
			git(t, d, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "noninjectable")
			out := filepath.Join(t.TempDir(), "release")
			err := release.Package(context.Background(), release.Options{Repository: d, Revision: "HEAD", Version: "1.2.3", Output: out, Targets: []string{"linux/amd64"}})
			if err == nil || !strings.Contains(err.Error(), "linker-settable") {
				t.Fatalf("version refusal: %v", err)
			}
			if _, err := os.Lstat(out); !os.IsNotExist(err) {
				t.Fatal("version refusal created output")
			}
		})
	}
}
