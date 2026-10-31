package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"hubnot/internal/artifact"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

func init() {
	artifactCommands["fetch"] = func(args []string) error { return artifactGitCommand("fetch", args) }
	artifactCommands["push"] = func(args []string) error { return artifactGitCommand("push", args) }
}

func artifactGitCommand(action string, args []string) error {
	machine := hasJSON(args)
	remote := ""
	ids := []string{}
	seen := map[string]bool{}
	fail := func(message string) error {
		return artifactOutput(nil, artifactFailure("invalid_input", message), machine)
	}
	for i := 0; i < len(args); i++ {
		name, value, eq := strings.Cut(args[i], "=")
		if seen[name] && name != "--id" {
			return fail("duplicate transport flag")
		}
		seen[name] = true
		switch name {
		case "--json":
			if eq && value != "true" && value != "false" {
				return fail("invalid json flag")
			}
			machine = !eq || value == "true"
		case "--remote", "--id":
			if !eq {
				i++
				if i == len(args) {
					return fail("transport flag requires a value")
				}
				value = args[i]
			}
			if name == "--remote" {
				remote = value
			} else {
				ids = append(ids, value)
			}
		default:
			return fail("expected --remote NAME and explicit --id ID selections")
		}
	}
	if !validArtifactRemote(remote) || len(ids) == 0 {
		return fail("requires a valid remote and full artifact IDs")
	}
	sort.Strings(ids)
	for i, id := range ids {
		if !validMemoryDigestID(id) || i > 0 && ids[i-1] == id {
			return fail("requires distinct full artifact IDs")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, err := replicateArtifacts(ctx, action, remote, ids)
	return artifactGitOutput(data, err, machine)
}

func artifactGitOutput(data any, err error, machine bool) error {
	var ae *artifact.Error
	if errors.As(err, &ae) && ae.Code == "concurrent_update" {
		return artifactStorageOutput(data, err, machine, "", "")
	}
	if machine && errors.As(err, &ae) && ae.Code == "uncertain_operation" {
		// Transport has no mutation actor/key. Never manufacture an operation identity
		// for a public ref transaction whose acknowledgement is unconfirmed.
		e := json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": false, "data": nil, "error": map[string]any{"origin": "artifact", "code": ae.Code, "message": ae.Message, "recovery": "inspect_operation", "operation": nil}})
		if e != nil {
			return e
		}
		return errors.New("artifact request refused")
	}
	if _, ok := err.(*artifactStorageError); ok {
		return artifactStorageOutput(data, err, machine, "", "")
	}
	return artifactOutput(data, err, machine)
}

func validArtifactRemote(remote string) bool {
	return len(remote) <= 64 && len(remote) > 0 && remote[0] != '_' && validMemoryRemote(remote)
}

func validArtifactTransportRef(ref string) bool {
	if _, ok := parseArtifactRef(ref); !ok {
		return false
	}
	if strings.HasPrefix(ref, "refs/hn/artifacts/") {
		return true
	}
	return validArtifactRemote(strings.Split(ref, "/")[3])
}

func artifactTransportError(ctx context.Context, code, message string) error {
	if ctx.Err() != nil {
		return artifactFailure("cancelled", "artifact transport cancelled")
	}
	return artifactFailure(code, message)
}

func artifactRemoteSources(ctx context.Context, dir, url string) ([]artifactSource, error) {
	b, err := projectGit(ctx, dir, nil, 32<<20, "ls-remote", "--refs", "--", url, "refs/hn/artifacts/*/*", "refs/hn/remotes/*/artifacts/*/*")
	if err != nil {
		return nil, artifactTransportError(ctx, "repository_error", "cannot read artifact remote advertisement")
	}
	sources := []artifactSource{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		p := strings.Fields(line)
		if len(p) != 2 {
			return nil, artifactFailure("invalid_history", "invalid artifact advertisement")
		}
		if !validArtifactTransportRef(p[1]) || !validGitOID(p[0]) {
			return nil, artifactFailure("invalid_history", "invalid artifact advertisement ref")
		}
		sources = append(sources, artifactSource{p[1], p[0]})
		if len(sources) > artifactReadEntries {
			return nil, artifactFailure("resource_limit", "artifact advertisement exceeds selected ref bound")
		}
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Ref < sources[j].Ref })
	return sources, nil
}

func artifactStage(ctx context.Context, main string) (string, func(), error) {
	root := filepath.Join(main, "hn", "replication", "quarantine")
	created := []string{}
	for _, dir := range []string{filepath.Join(main, "hn"), filepath.Join(main, "hn", "replication"), root} {
		if _, e := os.Lstat(dir); os.IsNotExist(e) {
			created = append(created, dir)
		}
		if err := ensurePrivateDirectory(dir); err != nil {
			return "", nil, artifactFailure("repository_error", "cannot prepare artifact quarantine")
		}
	}
	dir, err := os.MkdirTemp(root, "txn-")
	if err != nil {
		return "", nil, artifactFailure("repository_error", "cannot create artifact quarantine")
	}
	cleanup := func() {
		_ = removeGeneratedQuarantine(root, dir)
		for i := len(created) - 1; i >= 0; i-- {
			_ = os.Remove(created[i])
		}
	}
	format, err := projectGit(ctx, main, nil, 32, "rev-parse", "--show-object-format")
	if err == nil {
		_, err = projectGit(ctx, "", nil, 1024, "init", "--bare", "--quiet", "--object-format="+strings.TrimSpace(string(format)), dir)
	}
	if err != nil {
		cleanup()
		return "", nil, artifactTransportError(ctx, "repository_error", "cannot initialize artifact quarantine")
	}
	return dir, cleanup, nil
}

// Transport consumes the shared verified inventory; it neither parses another
// envelope/schema nor changes the ordinary reader's larger inspection bounds.
func artifactTransferInventory(ctx context.Context, h *artifactHistory, c *artifactClosure) (int64, error) {
	if e := artifactTransferLimit(len(c.Objects), len(c.Items), 0, 0, 0); e != nil {
		return 0, e
	}
	for _, source := range c.Sources {
		actor, _ := parseArtifactRef(source.Ref)
		depth := 0
		for _, item := range h.Chains[actor] {
			depth++
			if item.commit == source.OID {
				break
			}
		}
		if e := artifactTransferLimit(len(c.Objects), len(c.Items), depth, 0, 0); e != nil {
			return 0, e
		}
	}
	var total, wire int64
	// Count the equivalent canonical exchange object table while reading bounded
	// raw objects. Git's network download itself has no pre-download byte quota.
	refs, _ := json.Marshal(c.Sources)
	wire = int64(len(`{"mode":"accepted","objects":[],"protocol":"hn-artifact-exchange/1","refs":}`) + len(refs))
	for i, oid := range c.Objects {
		kind, raw, err := artifactTransportObject(ctx, h.gitDir, oid)
		if err != nil {
			return 0, err
		}
		total += int64(len(raw))
		if e := artifactTransferLimit(len(c.Objects), len(c.Items), 0, total, wire); e != nil {
			return 0, e
		}
		entry, _ := json.Marshal(map[string]any{"oid": oid, "type": kind, "data": base64.RawStdEncoding.EncodeToString(raw)})
		wire += int64(len(entry))
		if i > 0 {
			wire++
		}
		if e := artifactTransferLimit(len(c.Objects), len(c.Items), 0, total, wire); e != nil {
			return 0, e
		}
	}
	return wire, nil
}

// These limits belong to accepted transport, never ordinary catalog inspection.
func artifactTransferLimit(objects, records, depth int, decoded, wire int64) error {
	switch {
	case objects > 4096:
		return artifactFailure("resource_limit", "artifact exchange object bound exceeded")
	case records > 2048:
		return artifactFailure("resource_limit", "artifact exchange signed record bound exceeded")
	case depth > 256:
		return artifactFailure("resource_limit", "artifact exchange ancestry depth exceeded")
	case decoded > 33554432:
		return artifactFailure("resource_limit", "artifact exchange decoded aggregate exceeded")
	case wire > 16777216:
		return artifactFailure("resource_limit", "artifact exchange wire bound exceeded")
	}
	return nil
}

func artifactTransportObject(ctx context.Context, dir, oid string) (string, []byte, error) {
	kind, err := projectGit(ctx, dir, nil, 16, "cat-file", "-t", oid)
	if err != nil {
		return "", nil, artifactTransportError(ctx, "verification_failure", "selected artifact object unavailable")
	}
	typ := strings.TrimSpace(string(kind))
	if typ != "blob" && typ != "tree" && typ != "commit" {
		return "", nil, artifactFailure("invalid_history", "unsupported artifact object type")
	}
	raw, err := projectGit(ctx, dir, nil, artifactReadObjectBytes, "cat-file", typ, oid)
	if err != nil {
		return "", nil, artifactTransportError(ctx, "verification_failure", "selected artifact object unavailable")
	}
	return typ, raw, nil
}

func copyArtifactInventory(ctx context.Context, src, dst string, c *artifactClosure) error {
	for _, oid := range c.Objects {
		kind, raw, err := artifactTransportObject(ctx, src, oid)
		if err != nil {
			return err
		}
		// The shared reader already verified this bounded public object. Preserve
		// its raw Git body: hash-object's extra metadata fsck is not artifact
		// validity, and must not rewrite or reject accepted transport metadata.
		got, err := projectGit(ctx, dst, raw, 128, "hash-object", "-w", "--literally", "-t", kind, "--stdin")
		if err != nil || strings.TrimSpace(string(got)) != oid {
			return artifactTransportError(ctx, "verification_failure", "artifact object copy failed verification")
		}
	}
	return nil
}

func replicateArtifacts(ctx context.Context, action, remote string, ids []string) (any, error) {
	if err := artifactMutationPolicy(); err != nil {
		return nil, err
	}
	main, err := requireGitRepository()
	if err != nil {
		return nil, artifactFailure("repository_error", "artifact transport requires a Git repository")
	}
	url, err := projectGit(ctx, main, nil, 4096, "remote", "get-url", remote)
	if err != nil {
		return nil, artifactTransportError(ctx, "repository_error", "artifact remote unavailable")
	}
	remoteURL := strings.TrimSpace(string(url))
	stage, cleanup, err := artifactStage(ctx, main)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	var h *artifactHistory
	var sources []artifactSource
	if action == "fetch" {
		sources, err = artifactRemoteSources(ctx, main, remoteURL)
		if err != nil {
			return nil, err
		}
		for _, source := range sources {
			_, err = projectGit(ctx, stage, nil, 1024, "fetch", "--no-tags", "--no-write-fetch-head", "--", remoteURL, "+"+source.Ref+":"+source.Ref)
			if err != nil {
				return nil, artifactTransportError(ctx, "verification_failure", "exact artifact quarantine fetch failed")
			}
		}
		h, err = loadArtifactHistoryFromSources(ctx, stage, sources)
	} else {
		h, err = loadArtifactHistory(ctx)
	}
	if err != nil {
		return nil, err
	}
	closure, err := selectArtifactClosure(ctx, h, ids)
	if err != nil {
		return nil, err
	}
	wire, err := artifactTransferInventory(ctx, h, closure)
	if err != nil {
		return nil, err
	}
	if err = artifactCheckSourceWitnesses(ctx, h.gitDir, h.Sources); err != nil {
		return nil, err
	}
	if action == "push" {
		after, e := artifactSources(ctx, main)
		if e != nil {
			return nil, e
		}
		if !reflect.DeepEqual(after, h.Sources) {
			return nil, artifactFailure("concurrent_update", "artifact sources changed during transport")
		}
		if err = copyArtifactInventory(ctx, main, stage, closure); err != nil {
			return nil, err
		}
		for _, source := range closure.Sources {
			if _, err = projectGit(ctx, stage, nil, 1024, "update-ref", source.Ref, source.OID); err != nil {
				return nil, artifactTransportError(ctx, "repository_error", "cannot stage artifact export refs")
			}
		}
		advertised, e := artifactRemoteSources(ctx, main, remoteURL)
		if e != nil {
			return nil, e
		}
		old := map[string]string{}
		for _, source := range advertised {
			old[source.Ref] = source.OID
		}
		for _, source := range closure.Sources {
			if previous := old[source.Ref]; previous != "" && previous != source.OID {
				if _, e := projectGit(ctx, stage, nil, 1024, "merge-base", "--is-ancestor", previous, source.OID); e != nil {
					return nil, artifactTransportError(ctx, "concurrent_update", "artifact push would rewind or diverge remote history")
				}
			}
		}
		args := []string{"push", "--atomic", "--no-verify"}
		for _, source := range closure.Sources {
			args = append(args, "--force-with-lease="+source.Ref+":"+old[source.Ref])
		}
		args = append(args, "--", remoteURL)
		for _, source := range closure.Sources {
			args = append(args, source.Ref+":"+source.Ref)
		}
		if _, err = projectGit(ctx, stage, nil, 4096, args...); err != nil {
			// A transport failure can lose the acknowledgement after remote CAS.
			// Never infer remote absence from a nonzero client exit status.
			return nil, artifactFailure("uncertain_operation", "artifact push dispatched; remote publication is unconfirmed")
		}
		if ctx.Err() != nil {
			return nil, artifactFailure("uncertain_operation", "artifact push dispatched; remote publication is unconfirmed")
		}
		descriptorRefs := []map[string]string{}
		for _, ref := range closure.Sources {
			descriptorRefs = append(descriptorRefs, map[string]string{"ref": ref.Ref, "oid": ref.OID})
		}
		descriptor, _ := json.Marshal(map[string]any{"protocol": "hn-artifact-transfer/1", "remote": remote, "refs": descriptorRefs, "ids": ids, "mode": "accepted"})
		return map[string]any{"mode": "accepted", "output_digest": eventID(descriptor), "records": len(closure.Items), "objects": len(closure.Objects), "refs": closure.Sources, "opaque_ids": []string{}, "bytes": wire}, nil
	}
	promotions := []replicationPromotion{}
	accepted := []artifactSource{}
	for _, source := range closure.Sources {
		actor, _ := parseArtifactRef(source.Ref)
		ref := "refs/hn/remotes/" + remote + "/artifacts/" + actor + "/" + strings.TrimPrefix(artifactStream(actor), "sha256:")
		old, _, e := refValue(ref)
		if e != nil {
			return nil, artifactFailure("repository_error", "cannot inspect accepted artifact ref")
		}
		// Never rewind an already accepted prefix on an older exact selection.
		if old != "" && old != source.OID {
			if _, e = projectGit(ctx, stage, nil, 1024, "merge-base", "--is-ancestor", old, source.OID); e != nil {
				return nil, artifactTransportError(ctx, "concurrent_update", "artifact fetch would rewind or diverge accepted history")
			}
		}
		promotions = append(promotions, replicationPromotion{ref, source.OID, old})
		accepted = append(accepted, artifactSource{ref, source.OID})
	}
	// Validate the complete projected accepted view before touching destination
	// objects. Borrowing permits reads only; explicit witness refs live in stage.
	existing, e := artifactSources(ctx, main)
	if e != nil {
		return nil, e
	}
	if e = borrowMainObjects(stage, main); e != nil {
		return nil, artifactFailure("repository_error", "cannot verify destination artifact projection")
	}
	projected := map[string]string{}
	for _, source := range existing {
		projected[source.Ref] = source.OID
	}
	for _, source := range accepted {
		projected[source.Ref] = source.OID
	}
	projectedSources := []artifactSource{}
	for ref, oid := range projected {
		if _, e = projectGit(ctx, stage, nil, 1024, "update-ref", ref, oid); e != nil {
			return nil, artifactTransportError(ctx, "verification_failure", "cannot stage destination artifact projection")
		}
		projectedSources = append(projectedSources, artifactSource{ref, oid})
	}
	if _, e = loadArtifactHistoryFromSources(ctx, stage, projectedSources); e != nil {
		return nil, e
	}
	after, e := artifactSources(ctx, main)
	if e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(existing, after) {
		return nil, artifactFailure("concurrent_update", "accepted artifact sources changed")
	}
	if ctx.Err() != nil {
		return nil, artifactFailure("cancelled", "artifact transport cancelled")
	}
	if replicationBeforePromoteHook != nil {
		if err = replicationBeforePromoteHook(); err != nil {
			return nil, artifactFailure("concurrent_update", "artifact promotion interrupted")
		}
	}
	if err = copyArtifactInventory(ctx, stage, main, closure); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, artifactFailure("cancelled", "artifact transport cancelled")
	}
	if err = promoteReplicationRefs(promotions); err != nil {
		return nil, artifactFailure("concurrent_update", "artifact accepted-ref expected-old transaction failed")
	}
	// A quarantine Cut is internal-only. Publish the destination's actual accepted
	// view after promotion, including its physical repository identity and refs.
	acceptedHistory, e := loadArtifactHistory(ctx)
	if e != nil {
		return nil, artifactFailure("uncertain_operation", "artifact refs promoted but accepted view acknowledgement unavailable")
	}
	return map[string]any{"mode": "accepted", "accepted_refs": accepted, "imported_ids": closure.IDs, "opaque_ids": []string{}, "diagnostics": []any{}, "source_cut": acceptedHistory.Cut}, nil
}
