package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"hubnot/internal/migration"
)

func migrationObservationRef(sourceID string, r migration.NativeRef) (string, error) {
	token := "migration-" + migration.Digest([]byte(sourceID))[7:23]
	switch r.Kind {
	case "actor":
		return "refs/hn/remotes/" + token + "/actors/" + projectActorOwner(r.SourceRef), nil
	case "proposal":
		id := strings.TrimPrefix(r.SourceRef, "refs/hn/proposals/")
		if _, observed, ok := parseAcceptedProposalRef(r.SourceRef); ok {
			id = strings.TrimPrefix(observed, "sha256:")
		}
		return "refs/hn/remotes/" + token + "/proposals/" + id, nil
	case "memory":
		s, e := projectMemorySource(r.SourceRef, r.OID)
		if e != nil {
			return "", e
		}
		return "refs/hn/remotes/" + token + "/memory/" + s.Actor + "/" + s.Stream, nil
	case "code":
		return "refs/hn/remotes/" + token + "/code/" + migration.Digest([]byte(r.SourceRef))[7:], nil
	}
	return "", fmt.Errorf("unsupported native observation")
}
func migrationReadRef(ctx context.Context, dir, ref string) (string, error) {
	if !migration.ValidRef(ref) {
		return "", fmt.Errorf("invalid observation ref")
	}
	if _, e := projectGit(ctx, dir, nil, 2048, "symbolic-ref", "-q", ref); e == nil {
		return "", fmt.Errorf("symbolic destination observation refused")
	}
	out, e := projectGit(ctx, dir, nil, 256, "show-ref", "--verify", "--hash", ref)
	if e != nil { // Distinguish missing from corrupt/unavailable repository.
		if _, e = projectGit(ctx, dir, nil, 256, "rev-parse", "--git-dir"); e != nil {
			return "", e
		}
		return "", nil
	}
	id := strings.TrimSpace(string(out))
	if !migration.ValidOID(id) {
		return "", fmt.Errorf("invalid destination observation")
	}
	return id, nil
}
func migrationDestinationGitDir(ctx context.Context, repo string) (string, error) {
	abs, e := filepath.Abs(repo)
	if e != nil {
		return "", e
	}
	if e = projectSafeEnvironment(); e != nil {
		return "", e
	}
	out, e := projectGit(ctx, "", nil, 4096, "-C", abs, "rev-parse", "--absolute-git-dir")
	if e != nil {
		return "", e
	}
	dir := strings.TrimSpace(string(out))
	physical, e := filepath.EvalSymlinks(dir)
	if e != nil || physical != dir {
		return "", fmt.Errorf("destination Git directory aliases refused")
	}
	return dir, nil
}
func migrationBaseline(ctx context.Context, dir string) (string, error) {
	head, e := projectGit(ctx, dir, nil, 4096, "symbolic-ref", "HEAD")
	if e != nil {
		head, e = projectGit(ctx, dir, nil, 4096, "rev-parse", "--verify", "HEAD")
		if e != nil {
			return "", e
		}
	}
	// Bind destination path, object format and HEAD. Destination policy, private
	// signer and working files are never touched by observation-ref promotion.
	format, e := projectGit(ctx, dir, nil, 256, "rev-parse", "--show-object-format")
	if e != nil {
		return "", e
	}
	b, _ := json.Marshal([]string{dir, string(head), string(format)})
	return migration.Digest(b), nil
}
func migrationCheckRefGuards(ctx context.Context, dir string, refs []migration.RefUpdate) error {
	for _, r := range refs {
		actual, e := migrationReadRef(ctx, dir, r.Ref)
		if e != nil {
			return e
		}
		if actual != r.ExpectedOld {
			return fmt.Errorf("stale destination observation %s", r.Ref)
		}
	}
	return nil
}
func migrationPromote(ctx context.Context, dir string, q *ProjectQuarantine, w migration.ProjectWrite, l migration.Limits) error {
	// The destination mutation must run inside the destination itself: a relative
	// core.hooksPath resolves against the process cwd, which is the caller's, not
	// the destination's. Anchor cwd to the selected repository (-C) and pin the
	// already-resolved git dir (--git-dir) so destination hooks execute in the
	// destination directory and caller-side hooks can never observe this write.
	repo, e := migrationDestinationRepoDir(ctx, dir)
	if e != nil {
		return e
	}
	if e := migrationCheckRefGuards(ctx, dir, w.Refs); e != nil {
		return e
	}
	if e := projectCopyObjectIDs(ctx, q.GitDir, dir, q.ObjectIDs(), l); e != nil {
		return e
	}
	var tx strings.Builder
	tx.WriteString("start\n")
	for _, r := range w.Refs {
		old := r.ExpectedOld
		if old == "" {
			old = strings.Repeat("0", len(r.OID))
		}
		fmt.Fprintf(&tx, "update %s %s %s\n", r.Ref, r.OID, old)
	}
	tx.WriteString("prepare\ncommit\n")
	_, e = projectGit(ctx, dir, []byte(tx.String()), 1<<20, "-C", repo, "update-ref", "--no-deref", "--stdin")
	return e
}

// migrationDestinationRepoDir maps the resolved destination git directory to
// the directory Git anchors relative core.hooksPath resolution to for the
// selected repository. Only cwd-independent facts classify the shape:
// is-bare-repository for bare layouts, structural back-links for worktrees.
// Ordinary and linked-worktree repositories anchor at their working-tree root,
// matching the resolution an operator command run inside that repository gets
// (`git -C <toplevel>`). Every unrecognized shape refuses; the mutation is
// never allowed to run unanchored.
func migrationDestinationRepoDir(ctx context.Context, dir string) (string, error) {
	out, e := projectGit(ctx, dir, nil, 256, "rev-parse", "--is-bare-repository")
	if e != nil {
		return "", fmt.Errorf("destination repository anchor unavailable")
	}
	if strings.TrimSpace(string(out)) == "true" {
		return dir, nil
	}
	physical := func(p string) (string, error) {
		q, e := filepath.EvalSymlinks(p)
		if e != nil {
			return "", e
		}
		return q, nil
	}
	// Linked worktree: the resolved git directory carries a gitdir back-pointer
	// file naming its registered working tree side (.git file or directory).
	if raw, e := os.ReadFile(filepath.Join(dir, "gitdir")); e == nil {
		p := strings.TrimSpace(string(raw))
		if p == "" {
			return "", fmt.Errorf("destination worktree anchor invalid")
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		root := p
		if filepath.Base(p) == ".git" {
			root = filepath.Dir(p)
		}
		st, e := os.Lstat(root)
		if e != nil || !st.IsDir() {
			return "", fmt.Errorf("destination worktree anchor invalid")
		}
		// The working-tree side must exist and point back at exactly this git
		// directory; a stale/moved registration (missing .git side, wrong
		// target, unreadable pointer) refuses before promotion. No unverified
		// pointer parent is ever returned as an anchor.
		raw, e := os.ReadFile(p)
		if e != nil {
			return "", fmt.Errorf("destination worktree anchor invalid")
		}
		text := strings.TrimSpace(string(raw))
		target, ok := strings.CutPrefix(text, "gitdir:")
		if !ok {
			return "", fmt.Errorf("destination worktree anchor invalid")
		}
		target = strings.TrimSpace(target)
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(p), target)
		}
		q, e := physical(target)
		if e != nil || q != dir {
			return "", fmt.Errorf("destination worktree anchor invalid")
		}
		if root, e = physical(root); e != nil {
			return "", fmt.Errorf("destination worktree anchor invalid")
		}
		return root, nil
	} else if !os.IsNotExist(e) {
		return "", e
	}
	// Ordinary repository: the git directory's parent is the working tree whose
	// .git directory (or separate-git-dir side) is exactly this git directory.
	parent := filepath.Dir(dir)
	st, e := os.Lstat(filepath.Join(parent, ".git"))
	if e == nil && st.IsDir() {
		q, e := physical(filepath.Join(parent, ".git"))
		if e == nil && q == dir {
			if q, e := physical(parent); e == nil {
				return q, nil
			}
		}
	} else if e == nil && st.Mode()&os.ModeSymlink == 0 && st.Mode().IsRegular() {
		raw, e := os.ReadFile(filepath.Join(parent, ".git"))
		if e == nil {
			text := strings.TrimSpace(string(raw))
			target, ok := strings.CutPrefix(text, "gitdir:")
			if ok {
				target = strings.TrimSpace(target)
				if !filepath.IsAbs(target) {
					target = filepath.Join(parent, target)
				}
				q, e := physical(target)
				if e == nil && q == dir {
					if q, e := physical(parent); e == nil {
						return q, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("destination repository anchor unavailable")
}

// GitMigrationDestination is the shared code transport companion for optional
// communication destinations. It performs no provider API or credential lookup.
// Mappings maps selected source refs to explicit destination branches/tags.
type GitMigrationDestination struct {
	Remote      string
	Destination migration.Locator
	Mappings    map[string]string
	Credentials *GitHTTPSCredentials `json:"-"`
}

func (d GitMigrationDestination) Capabilities(context.Context) ([]migration.Capability, error) {
	return []migration.Capability{{Kind: "code", Read: true, Preserve: true, Create: true, Update: true, Limits: []string{"explicit local or HTTPS Git remote; private HTTPS requires transient matching-origin credentials", "atomic guarded fast-forward only", "no source deletion or workflow activation"}, Reason: "shared Git transport"}}, nil
}
func migrationRemoteRefs(ctx context.Context, remote string, refs []migration.RefUpdate, credentials *GitHTTPSCredentials) (map[string]string, error) {
	if !migration.ValidGitRemote(remote) {
		return nil, fmt.Errorf("unsupported Git remote transport")
	}
	if filepath.IsAbs(remote) {
		if e := migrationNoLinks(remote); e != nil {
			return nil, e
		}
	}
	args := []string{"-c", "protocol.allow=never", "-c", "protocol.file.allow=always", "-c", "protocol.https.allow=always", "ls-remote", "--refs", "--", remote}
	for _, r := range refs {
		args = append(args, r.Ref)
	}
	// Isolate client configuration: only public HTTPS is supported here. No
	// ambient repository credential helper or URL rewrite is consulted.
	client, e := os.MkdirTemp("", "hn-git-transport-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(client)
	if _, e = projectGit(ctx, "", nil, 4096, "init", "--bare", "--quiet", client); e != nil {
		return nil, e
	}
	out, e := migrationTransportGit(ctx, client, remote, credentials, nil, int64(len(refs))*1200+1, args...)
	if e != nil {
		return nil, e
	}
	result := map[string]string{}
	allowed := map[string]bool{}
	for _, r := range refs {
		allowed[r.Ref] = true
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 2 || !allowed[f[1]] || !migration.ValidOID(f[0]) || result[f[1]] != "" {
			return nil, fmt.Errorf("unexpected remote ref advertisement")
		}
		result[f[1]] = f[0]
	}
	return result, nil
}
func (d GitMigrationDestination) Plan(ctx context.Context, s migration.Snapshot, sel migration.Selection, l migration.Limits) ([]migration.Intent, []migration.Finding, error) {
	if sel.Destination != d.Destination || !migration.ValidGitRemote(d.Remote) {
		return nil, nil, fmt.Errorf("Git destination selection mismatch")
	}
	refs := []migration.RefUpdate{}
	seen := map[string]bool{}
	for _, r := range s.NativeRefs {
		if r.Kind != "code" {
			continue
		}
		dest, ok := d.Mappings[r.SourceRef]
		if !ok {
			return nil, nil, fmt.Errorf("explicit code destination mapping required for %s", r.SourceRef)
		}
		if seen[dest] || !migration.ValidRef(dest) || (!strings.HasPrefix(dest, "refs/heads/") && !strings.HasPrefix(dest, "refs/tags/")) {
			return nil, nil, fmt.Errorf("invalid/duplicate code destination ref")
		}
		if strings.HasPrefix(dest, "refs/heads/") != strings.HasPrefix(r.SourceRef, "refs/heads/") {
			return nil, nil, fmt.Errorf("code ref family mapping requires separate operator conversion")
		}
		seen[dest] = true
		refs = append(refs, migration.RefUpdate{Ref: dest, OID: r.OID})
	}
	if len(refs) == 0 {
		return nil, nil, fmt.Errorf("Git destination requires explicitly selected code refs")
	}
	actual, e := migrationRemoteRefs(ctx, d.Remote, refs, d.Credentials)
	if e != nil {
		return nil, nil, e
	}
	for i := range refs {
		refs[i].ExpectedOld = actual[refs[i].Ref]
	}
	raw, e := migration.CanonicalSnapshot(s, l)
	if e != nil {
		return nil, nil, e
	}
	payload, _ := json.Marshal(migration.GitWrite{Schema: "hn.migration.git-write/1", SnapshotDigest: migration.Digest(raw), Remote: d.Remote, Refs: refs})
	in := migration.Intent{ID: "preserve-git-code", Kind: "git_refs", Action: "preserve", Sources: []migration.Key{}, Payload: payload, DependsOn: []string{}}
	if _, e = migration.CanonicalWrite(in, true); e != nil {
		return nil, nil, e
	}
	return []migration.Intent{in}, []migration.Finding{{ID: "git/code", Status: "mapped", Path: "code", Message: "Explicit selected code refs map to guarded atomic fast-forward Git writes."}}, nil
}
func (d GitMigrationDestination) bound(in migration.Intent, ac migration.ApplyContext) (migration.GitWrite, error) {
	b := in
	b.Payload = ac.BoundWrite().Payload
	if _, e := migration.CanonicalWrite(b, true); e != nil {
		return migration.GitWrite{}, e
	}
	var w migration.GitWrite
	_ = json.Unmarshal(b.Payload, &w)
	if w.Remote != d.Remote {
		return w, fmt.Errorf("Git remote changed")
	}
	return w, nil
}
func (d GitMigrationDestination) Apply(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Receipt, error) {
	w, e := d.bound(in, ac)
	if e != nil {
		return migration.Receipt{}, e
	}
	s, e := migrationContextSnapshot(ac, w.SnapshotDigest)
	if e != nil {
		return migration.Receipt{}, e
	}
	q, e := StageProjectSnapshot(ctx, s, migrationContextBlobs{ac}, migrationContextLimits(ac))
	if e != nil {
		return migration.Receipt{}, e
	}
	defer q.Close()
	if e = migrationGitAncestry(ctx, q, w); e != nil {
		return migration.Receipt{}, e
	}
	actual, e := migrationRemoteRefs(ctx, w.Remote, w.Refs, d.Credentials)
	if e != nil {
		return migration.Receipt{}, e
	}
	for _, r := range w.Refs {
		if actual[r.Ref] != r.ExpectedOld {
			return migration.Receipt{}, fmt.Errorf("stale Git destination baseline")
		}
	}

	args := []string{"-c", "protocol.allow=never", "-c", "protocol.file.allow=always", "-c", "protocol.https.allow=always", "push", "--atomic", "--porcelain"}
	for _, r := range w.Refs {
		args = append(args, "--force-with-lease="+r.Ref+":"+r.ExpectedOld)
	}
	args = append(args, "--", w.Remote)
	for _, r := range w.Refs {
		args = append(args, r.OID+":"+r.Ref)
	}
	// Lease is compare-and-swap only: every nonempty old value was independently
	// proven an ancestor above; replacing divergent history/tags is refused.
	if _, e = migrationTransportGit(ctx, q.GitDir, w.Remote, d.Credentials, nil, 1<<20, args...); e != nil {
		return migration.Receipt{}, e
	}
	recon, e := d.Reconcile(ctx, in, ac)
	if e != nil {
		return migration.Receipt{}, e
	}
	if recon.Status != "exact" || recon.Receipt == nil {
		return migration.Receipt{}, fmt.Errorf("Git push lacks exact positive acknowledgement")
	}
	return *recon.Receipt, nil
}
func (d GitMigrationDestination) Reconcile(ctx context.Context, in migration.Intent, ac migration.ApplyContext) (migration.Reconciliation, error) {
	w, e := d.bound(in, ac)
	if e != nil {
		return migration.Reconciliation{}, e
	}
	actual, e := migrationRemoteRefs(ctx, w.Remote, w.Refs, d.Credentials)
	if e != nil {
		return migration.Reconciliation{}, e
	}
	for _, r := range w.Refs {
		if actual[r.Ref] != r.OID {
			return migration.Reconciliation{Status: "no_proof", Reason: "remote refs do not exactly match the saved Git write"}, nil
		}
	}
	raw, _ := json.Marshal(w.Refs)
	r := migrationReceipt(in, ac, d.Destination, w.Remote, migration.Digest(raw))
	return migration.Reconciliation{Status: "exact", Receipt: &r, Reason: "exact positive remote ref advertisement"}, nil
}

func migrationGitAncestry(ctx context.Context, q *ProjectQuarantine, w migration.GitWrite) error {
	allowed := map[string]bool{}
	for _, r := range q.Snapshot.NativeRefs {
		if r.Kind == "code" {
			allowed[r.OID] = true
		}
	}
	for _, r := range w.Refs {
		if !allowed[r.OID] {
			return fmt.Errorf("undeclared code snapshot write")
		}
		if r.ExpectedOld != "" && r.ExpectedOld != r.OID {
			if strings.HasPrefix(r.Ref, "refs/tags/") {
				return fmt.Errorf("existing tag replacement refused")
			}
			if _, e := projectGit(ctx, q.GitDir, nil, 4096, "merge-base", "--is-ancestor", r.ExpectedOld, r.OID); e != nil {
				return fmt.Errorf("non-fast-forward Git write refused")
			}
		}
	}
	return nil
}

// GitHTTPSCredentials is transient operator input, never serialized into a plan,
// progress, URL or argv. Resume requires the operator to supply it again.
// CAFile is optional explicit certificate trust; verification is always enabled.
type GitHTTPSCredentials struct {
	Origin        string `json:"-"`
	Authorization string `json:"-"`
	CAFile        string `json:"-"`
}

func migrationGitAuth(remote string, credentials *GitHTTPSCredentials) ([]string, error) {
	pairs := []string{"http.followRedirects", "false", "credential.helper", "", "http.sslVerify", "true"}
	if credentials == nil {
		return pairs, nil
	}
	parsed, e := url.Parse(remote)
	if e != nil || parsed.Scheme != "https" {
		return nil, fmt.Errorf("Git credentials require the explicitly selected HTTPS remote")
	}
	origin, e := migration.NormalizeInstance(credentials.Origin)
	if e != nil {
		return nil, fmt.Errorf("invalid Git credential origin")
	}
	u, e := url.Parse(origin)
	if e != nil || u.Scheme != "https" || u.Path != "" {
		return nil, fmt.Errorf("Git credential origin must be an HTTPS origin without a path")
	}
	selected, e := migration.NormalizeInstance("https://" + parsed.Host)
	if e != nil || selected != origin {
		return nil, fmt.Errorf("Git credential origin does not match selected remote")
	}
	auth := credentials.Authorization
	if auth != "" {
		if len(auth) > 8192 || (!strings.HasPrefix(auth, "Basic ") && !strings.HasPrefix(auth, "Bearer ")) {
			return nil, fmt.Errorf("Git authorization requires bounded Basic or Bearer input")
		}
		_, token, _ := strings.Cut(auth, " ")
		if token == "" {
			return nil, fmt.Errorf("empty Git authorization")
		}
		for _, c := range token {
			if c < 33 || c > 126 {
				return nil, fmt.Errorf("invalid Git authorization header")
			}
		}

		if strings.HasPrefix(auth, "Basic ") {
			decoded, e := base64.StdEncoding.DecodeString(token)
			if e != nil || !bytes.Contains(decoded, []byte(":")) || bytes.ContainsAny(decoded, "\x00\r\n") {
				return nil, fmt.Errorf("invalid Basic Git authorization")
			}
		} else {
			if strings.TrimRight(token, "=") == "" {
				return nil, fmt.Errorf("invalid Bearer Git authorization")
			}
			padding := false
			for _, c := range token {
				if c == '=' {
					padding = true
					continue
				}
				if padding || !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("-._~+/", c)) {
					return nil, fmt.Errorf("invalid Bearer Git authorization")
				}
			}
		}
		pairs = append(pairs, "http."+origin+"/.extraHeader", "Authorization: "+auth)
	}
	if credentials.CAFile != "" {
		if !filepath.IsAbs(credentials.CAFile) {
			return nil, fmt.Errorf("Git certificate trust requires an explicit absolute file")
		}
		st, e := os.Lstat(credentials.CAFile)
		if e != nil || !st.Mode().IsRegular() {
			return nil, fmt.Errorf("invalid Git certificate trust file")
		}
		pairs = append(pairs, "http.sslCAInfo", credentials.CAFile)
	}
	return pairs, nil
}
func migrationTransportGit(ctx context.Context, dir, remote string, credentials *GitHTTPSCredentials, input []byte, limit int64, args ...string) ([]byte, error) {
	pairs, e := migrationGitAuth(remote, credentials)
	if e != nil {
		return nil, e
	}
	args = append([]string{"--git-dir=" + dir}, args...)
	cmd := exec.CommandContext(ctx, "git", args...)
	env := []string{}
	for _, v := range os.Environ() {
		k, _, _ := strings.Cut(v, "=")
		if !strings.HasPrefix(k, "GIT_") {
			env = append(env, v)
		}
	}
	env = append(env, "GIT_NO_REPLACE_OBJECTS=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1", fmt.Sprintf("GIT_CONFIG_COUNT=%d", len(pairs)/2))
	for i := 0; i < len(pairs); i += 2 {
		env = append(env, fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", i/2, pairs[i]), fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", i/2, pairs[i+1]))
	}
	privateHome, e := os.MkdirTemp("", "hn-git-client-home-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(privateHome)
	clean := []string{}
	for _, v := range env {
		k, _, _ := strings.Cut(v, "=")
		switch strings.ToUpper(k) {
		case "HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CONFIG_DIRS", "NETRC", "CURL_HOME":
			continue
		}
		clean = append(clean, v)
	}
	volume := filepath.VolumeName(privateHome)
	clean = append(clean, "HOME="+privateHome, "USERPROFILE="+privateHome, "HOMEDRIVE="+volume, "HOMEPATH="+strings.TrimPrefix(privateHome, volume), "APPDATA="+filepath.Join(privateHome, "AppData", "Roaming"), "LOCALAPPDATA="+filepath.Join(privateHome, "AppData", "Local"), "XDG_CONFIG_HOME="+filepath.Join(privateHome, "config"), "XDG_CONFIG_DIRS="+filepath.Join(privateHome, "config"))
	cmd.Env = clean
	cmd.Stdin = bytes.NewReader(input)
	var out bytes.Buffer
	cmd.Stdout = &projectBoundWriter{w: &out, left: limit}
	cmd.Stderr = io.Discard
	if e = cmd.Run(); e != nil {
		return nil, fmt.Errorf("Git transport failed; verify remote access and explicitly supply matching-origin HTTPS credentials when required")
	}
	return out.Bytes(), nil
}
func (d GitMigrationDestination) Preflight(ctx context.Context, in migration.Intent, ac migration.ApplyContext) error {
	w, e := d.bound(in, ac)
	if e != nil {
		return e
	}
	actual, e := migrationRemoteRefs(ctx, w.Remote, w.Refs, d.Credentials)
	if e != nil {
		return e
	}
	for _, r := range w.Refs {
		if actual[r.Ref] != r.ExpectedOld {
			return fmt.Errorf("stale Git destination baseline before writes")
		}
	}
	return nil
}

func (d GitMigrationDestination) Locator() migration.Locator { return d.Destination }
