package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func inIdentityTestRepository(t *testing.T) string {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		identityStorageHook = nil
		if err := os.Chdir(original); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	// Match Git's physical path when the temp directory has an alias, such as
	// /var -> /private/var on macOS.
	root, err = os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, "init", "-q", "-b", "main")
	mustGit(t, "config", "user.name", "Identity Test")
	mustGit(t, "config", "user.email", "identity-test@hn.invalid")
	mustGit(t, "commit", "--allow-empty", "-q", "-m", "base")
	return root
}

func writeLegacyIdentity(t *testing.T, identity *Identity) string {
	t.Helper()
	path, err := identityPath()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func requirePathMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix owner permission bits")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode for %s = %#o, want %#o", path, got, want)
	}
}

func TestIdentityCreationUsesCanonicalKeyring(t *testing.T) {
	inIdentityTestRepository(t)

	identity, path, err := createIdentity("Alice")
	if err != nil {
		t.Fatal(err)
	}
	paths, err := identityKeyringPaths()
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(paths.identities, identity.Actor+".json")
	if path != wantPath {
		t.Fatalf("created path = %q, want %q", path, wantPath)
	}
	if _, err := os.Stat(paths.legacy); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy identity exists after new initialization: %v", err)
	}
	active, err := loadActiveActor()
	if err != nil {
		t.Fatal(err)
	}
	if active != identity.Actor {
		t.Fatalf("active actor = %q, want %q", active, identity.Actor)
	}
	loaded, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if *loaded != *identity {
		t.Fatalf("loaded identity changed: %#v != %#v", loaded, identity)
	}
	requirePathMode(t, paths.root, 0o700)
	requirePathMode(t, paths.identities, 0o700)
	requirePathMode(t, paths.active, 0o600)
	requirePathMode(t, path, 0o600)
}

func TestIdentityMigrationPreservesSignerAndActorChain(t *testing.T) {
	inIdentityTestRepository(t)
	legacy := testIdentity(t, "Legacy Alice")
	legacyPath := writeLegacyIdentity(t, legacy)

	first, err := nextEvent(legacy, "issue.open")
	if err != nil {
		t.Fatal(err)
	}
	first.Title = "Before migration"
	storedFirst, err := appendEvent(first, legacy)
	if err != nil {
		t.Fatal(err)
	}
	refBeforeMigration, exists, err := refValue(actorRef(legacy.Actor))
	if err != nil || !exists {
		t.Fatalf("legacy actor ref = %q, exists=%t, error=%v", refBeforeMigration, exists, err)
	}
	probe := newEvent(legacy, "issue.open", 42, storedFirst.ID)
	probe.Title = "Signature compatibility"
	wantPayload, wantSignature, err := encodeAndSign(probe, legacy)
	if err != nil {
		t.Fatal(err)
	}

	migrated, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if *migrated != *legacy {
		t.Fatalf("migration changed identity: %#v != %#v", migrated, legacy)
	}
	refAfterMigration, exists, err := refValue(actorRef(migrated.Actor))
	if err != nil || !exists || refAfterMigration != refBeforeMigration {
		t.Fatalf("actor ref changed during migration: got %q, want %q, error=%v", refAfterMigration, refBeforeMigration, err)
	}
	gotPayload, gotSignature, err := encodeAndSign(probe, migrated)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotPayload, wantPayload) || !bytes.Equal(gotSignature, wantSignature) {
		t.Fatal("migration changed signed payload or signature")
	}
	if eventID(gotPayload) != eventID(wantPayload) {
		t.Fatal("migration changed event ID")
	}
	next, err := nextEvent(migrated, "issue.open")
	if err != nil {
		t.Fatal(err)
	}
	if next.Sequence != 2 || next.Previous != storedFirst.ID {
		t.Fatalf("next event = sequence %d previous %q; want 2 and %q", next.Sequence, next.Previous, storedFirst.ID)
	}
	next.Title = "After migration"
	if _, err := appendEvent(next, migrated); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy identity was removed: %v", err)
	}

	paths, err := identityKeyringPaths()
	if err != nil {
		t.Fatal(err)
	}
	requirePathMode(t, filepath.Join(paths.identities, legacy.Actor+".json"), 0o600)
	requirePathMode(t, paths.active, 0o600)
}

func TestIdentityMigrationIsIdempotentAndResumesAfterRecordDurability(t *testing.T) {
	inIdentityTestRepository(t)
	legacy := testIdentity(t, "Alice")
	writeLegacyIdentity(t, legacy)

	failed := false
	identityStorageHook = func(step string) error {
		if step == "migration-record-durable" && !failed {
			failed = true
			return errors.New("injected migration interruption")
		}
		return nil
	}
	if _, err := loadIdentity(); err == nil || !strings.Contains(err.Error(), "injected migration interruption") {
		t.Fatalf("migration error = %v, want injected interruption", err)
	}
	paths, err := identityKeyringPaths()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.identities, legacy.Actor+".json")); err != nil {
		t.Fatalf("durable record missing after interruption: %v", err)
	}
	if _, err := os.Stat(paths.active); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("active pointer changed before final migration step: %v", err)
	}

	identityStorageHook = nil
	for attempt := 0; attempt < 2; attempt++ {
		loaded, err := loadIdentity()
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Actor != legacy.Actor || loaded.PrivateKey != legacy.PrivateKey {
			t.Fatal("migration retry selected a different signer")
		}
	}
	entries, err := os.ReadDir(paths.identities)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("identity record count = %d, want 1", len(entries))
	}
}

func TestKeyringAuthorityIgnoresModifiedLegacyIdentity(t *testing.T) {
	inIdentityTestRepository(t)
	alice := testIdentity(t, "Alice")
	writeLegacyIdentity(t, alice)
	if _, err := loadIdentity(); err != nil {
		t.Fatal(err)
	}
	bob := testIdentity(t, "Bob")
	writeLegacyIdentity(t, bob)

	loaded, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Actor != alice.Actor {
		t.Fatalf("modified legacy identity replaced active actor: got %s, want %s", loaded.Actor, alice.Actor)
	}
}

func TestKeyringRejectsUnsafeOrMalformedPrivateState(t *testing.T) {
	t.Run("unsafe record permissions", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not expose Unix owner permission bits")
		}
		inIdentityTestRepository(t)
		identity, path, err := createIdentity("Alice")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadIdentity(); err == nil || !strings.Contains(err.Error(), "unsafe permissions") {
			t.Fatalf("load error = %v, want unsafe permissions", err)
		}
		active, err := loadActiveActor()
		if err != nil {
			t.Fatal(err)
		}
		if active != identity.Actor {
			t.Fatalf("failed load switched active actor to %q", active)
		}
	})

	t.Run("record symlink", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink creation is not generally available")
		}
		inIdentityTestRepository(t)
		identity, path, err := createIdentity("Alice")
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(t.TempDir(), "record.json")
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, contents, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		if _, err := loadIdentity(); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("load error = %v, want symbolic-link rejection", err)
		}
		active, err := loadActiveActor()
		if err != nil {
			t.Fatal(err)
		}
		if active != identity.Actor {
			t.Fatalf("failed load switched active actor to %q", active)
		}
	})

	t.Run("identities directory symlink", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink creation is not generally available")
		}
		inIdentityTestRepository(t)
		identity, _, err := createIdentity("Alice")
		if err != nil {
			t.Fatal(err)
		}
		paths, err := identityKeyringPaths()
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(t.TempDir(), "identities")
		if err := os.Rename(paths.identities, target); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, paths.identities); err != nil {
			t.Fatal(err)
		}
		if _, err := loadIdentity(); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("load error = %v, want directory symbolic-link rejection", err)
		}
		active, err := loadActiveActor()
		if err != nil {
			t.Fatal(err)
		}
		if active != identity.Actor {
			t.Fatalf("failed load switched active actor to %q", active)
		}
	})

	t.Run("missing active record", func(t *testing.T) {
		inIdentityTestRepository(t)
		identity, path, err := createIdentity("Alice")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if _, err := loadIdentity(); err == nil || !strings.Contains(err.Error(), identity.Actor+".json") {
			t.Fatalf("missing-record error = %v, want exact active record path", err)
		}
		active, err := loadActiveActor()
		if err != nil {
			t.Fatal(err)
		}
		if active != identity.Actor {
			t.Fatalf("missing record switched active actor to %q", active)
		}
	})

	t.Run("truncated record", func(t *testing.T) {
		inIdentityTestRepository(t)
		identity, path, err := createIdentity("Alice")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"version":1,"actor":"`+identity.Actor), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadIdentity(); err == nil || !strings.Contains(err.Error(), "unexpected EOF") {
			t.Fatalf("truncated-record error = %v, want JSON truncation", err)
		}
		active, err := loadActiveActor()
		if err != nil {
			t.Fatal(err)
		}
		if active != identity.Actor {
			t.Fatalf("truncated record switched active actor to %q", active)
		}
	})

	tests := []struct {
		name   string
		mutate func(*identityRecord)
	}{
		{name: "unknown version", mutate: func(record *identityRecord) { record.Version++ }},
		{name: "mismatched actor", mutate: func(record *identityRecord) { record.Actor = strings.Repeat("0", 64) }},
		{name: "mismatched keypair", mutate: func(record *identityRecord) { record.PrivateKey = testIdentity(t, "Other").PrivateKey }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inIdentityTestRepository(t)
			identity, path, err := createIdentity("Alice")
			if err != nil {
				t.Fatal(err)
			}
			var record identityRecord
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(contents, &record); err != nil {
				t.Fatal(err)
			}
			test.mutate(&record)
			contents, err = json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err = loadIdentity()
			if err == nil {
				t.Fatal("malformed keyring record unexpectedly loaded")
			}
			if strings.Contains(err.Error(), identity.PrivateKey) {
				t.Fatal("error disclosed private key encoding")
			}
		})
	}
}

func TestRotationStateRequiresBothEventsBeforeActiveSwitch(t *testing.T) {
	inIdentityTestRepository(t)
	alice, _, err := createIdentity("Alice")
	if err != nil {
		t.Fatal(err)
	}
	bob := testIdentity(t, "Bob")
	if _, err := storeIdentityRecord(bob, identityLifecycleAvailable); err != nil {
		t.Fatal(err)
	}
	state := identityRotationState{
		Version:          identityRotationVersion,
		PredecessorActor: alice.Actor,
		TargetActor:      bob.Actor,
		Relationship:     identityRelationshipSuccessor,
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := storeIdentityRotation(state); err != nil {
			t.Fatal(err)
		}
	}
	if err := switchActiveIdentity(state); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete switch error = %v", err)
	}
	loaded, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Actor != alice.Actor {
		t.Fatalf("incomplete rotation selected %s, want %s", loaded.Actor, alice.Actor)
	}

	state.AuthorizationEvent = "sha256:" + strings.Repeat("a", 64)
	for attempt := 0; attempt < 2; attempt++ {
		if err := storeIdentityRotation(state); err != nil {
			t.Fatal(err)
		}
	}
	if err := switchActiveIdentity(state); err == nil {
		t.Fatal("rotation switched without target acceptance")
	}
	state.AcceptanceEvent = "sha256:" + strings.Repeat("b", 64)
	for attempt := 0; attempt < 2; attempt++ {
		if err := storeIdentityRotation(state); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := identityKeyringPaths()
	if err != nil {
		t.Fatal(err)
	}
	requirePathMode(t, paths.rotation, 0o600)
	for attempt := 0; attempt < 2; attempt++ {
		if err := switchActiveIdentity(state); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err = loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Actor != bob.Actor {
		t.Fatalf("completed rotation selected %s, want %s", loaded.Actor, bob.Actor)
	}
	if _, err := os.Stat(paths.rotation); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completed rotation state still exists: %v", err)
	}

	metadata, err := listIdentityRecords()
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 2 {
		t.Fatalf("identity metadata count = %d, want 2", len(metadata))
	}
	for _, item := range metadata {
		if strings.Contains(item.Name+item.Actor+item.PublicKey, alice.PrivateKey) || strings.Contains(item.Name+item.Actor+item.PublicKey, bob.PrivateKey) {
			t.Fatal("public identity metadata disclosed a private key")
		}
	}
}

func TestRotationInterruptionKeepsPreviousSignerActive(t *testing.T) {
	inIdentityTestRepository(t)
	alice, _, err := createIdentity("Alice")
	if err != nil {
		t.Fatal(err)
	}
	bob := testIdentity(t, "Bob")
	if _, err := storeIdentityRecord(bob, identityLifecycleAvailable); err != nil {
		t.Fatal(err)
	}
	state := identityRotationState{
		Version:            identityRotationVersion,
		PredecessorActor:   alice.Actor,
		TargetActor:        bob.Actor,
		Relationship:       identityRelationshipSuccessor,
		AuthorizationEvent: "sha256:" + strings.Repeat("c", 64),
		AcceptanceEvent:    "sha256:" + strings.Repeat("d", 64),
	}
	if err := storeIdentityRotation(state); err != nil {
		t.Fatal(err)
	}
	identityStorageHook = func(step string) error {
		if step == "before-active-switch" {
			return errors.New("injected active switch interruption")
		}
		return nil
	}
	if err := switchActiveIdentity(state); err == nil {
		t.Fatal("active switch unexpectedly survived injected interruption")
	}
	loaded, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Actor != alice.Actor {
		t.Fatalf("interrupted rotation selected %s, want %s", loaded.Actor, alice.Actor)
	}
	identityStorageHook = nil
	if err := switchActiveIdentity(state); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Actor != bob.Actor {
		t.Fatalf("resumed rotation selected %s, want %s", loaded.Actor, bob.Actor)
	}
}

func TestRotationCleanupInterruptionConvergesAfterPointerSwitch(t *testing.T) {
	inIdentityTestRepository(t)
	alice, _, err := createIdentity("Alice")
	if err != nil {
		t.Fatal(err)
	}
	bob := testIdentity(t, "Bob")
	if _, err := storeIdentityRecord(bob, identityLifecycleAvailable); err != nil {
		t.Fatal(err)
	}
	state := identityRotationState{
		Version:            identityRotationVersion,
		PredecessorActor:   alice.Actor,
		TargetActor:        bob.Actor,
		Relationship:       identityRelationshipSuccessor,
		AuthorizationEvent: "sha256:" + strings.Repeat("e", 64),
		AcceptanceEvent:    "sha256:" + strings.Repeat("f", 64),
	}
	if err := storeIdentityRotation(state); err != nil {
		t.Fatal(err)
	}
	paths, err := identityKeyringPaths()
	if err != nil {
		t.Fatal(err)
	}
	identityStorageHook = func(step string) error {
		if step == "before-rotation-cleanup" {
			return errors.New("injected rotation cleanup interruption")
		}
		return nil
	}
	if err := switchActiveIdentity(state); err == nil || !strings.Contains(err.Error(), "cleanup interruption") {
		t.Fatalf("switch error = %v, want cleanup interruption", err)
	}
	loaded, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Actor != bob.Actor {
		t.Fatalf("cleanup interruption selected %s, want switched target %s", loaded.Actor, bob.Actor)
	}
	if _, err := os.Stat(paths.rotation); err != nil {
		t.Fatalf("cleanup interruption did not retain recoverable state: %v", err)
	}

	identityStorageHook = nil
	for attempt := 0; attempt < 2; attempt++ {
		if err := switchActiveIdentity(state); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(paths.rotation); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rotation state exists after cleanup retry: %v", err)
	}
}

func TestRotationRetryConvergesWhenCleanupWasDurable(t *testing.T) {
	inIdentityTestRepository(t)
	alice, _, err := createIdentity("Alice")
	if err != nil {
		t.Fatal(err)
	}
	bob := testIdentity(t, "Bob")
	if _, err := storeIdentityRecord(bob, identityLifecycleAvailable); err != nil {
		t.Fatal(err)
	}
	state := identityRotationState{
		Version:            identityRotationVersion,
		PredecessorActor:   alice.Actor,
		TargetActor:        bob.Actor,
		Relationship:       identityRelationshipSuccessor,
		AuthorizationEvent: "sha256:" + strings.Repeat("1", 64),
		AcceptanceEvent:    "sha256:" + strings.Repeat("2", 64),
	}
	if err := storeIdentityRotation(state); err != nil {
		t.Fatal(err)
	}
	paths, err := identityKeyringPaths()
	if err != nil {
		t.Fatal(err)
	}
	identityStorageHook = func(step string) error {
		if step == "after-rotation-cleanup" {
			return errors.New("injected post-cleanup interruption")
		}
		return nil
	}
	if err := switchActiveIdentity(state); err == nil || !strings.Contains(err.Error(), "post-cleanup interruption") {
		t.Fatalf("switch error = %v, want post-cleanup interruption", err)
	}
	if _, err := os.Stat(paths.rotation); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rotation cleanup was not durable before interruption: %v", err)
	}
	identityStorageHook = nil
	for attempt := 0; attempt < 2; attempt++ {
		if err := switchActiveIdentity(state); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := loadIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Actor != bob.Actor {
		t.Fatalf("post-cleanup retry selected %s, want %s", loaded.Actor, bob.Actor)
	}
}

func TestIdentityInspectionAndTrackedFilesDoNotExposePrivateKey(t *testing.T) {
	inIdentityTestRepository(t)
	if err := os.MkdirAll(".hn", 0o755); err != nil {
		t.Fatal(err)
	}
	const trackedFixture = ".hn/identity.json"
	if err := os.WriteFile(trackedFixture, []byte("tracked identity-path sentinel\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", trackedFixture)
	mustGit(t, "commit", "-q", "-m", "tracked identity-path sentinel")
	identity, _, err := createIdentity("Alice")
	if err != nil {
		t.Fatal(err)
	}
	output, err := captureTestOutput(t, func() error { return cmdIdentity([]string{"show"}) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output, identity.PrivateKey) {
		t.Fatal("identity inspection disclosed private key encoding")
	}
	count, err := scanTrackedFilesForSecret(identity.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("tracked secret scan examined no files")
	}
	status, err := gitText("status", "--porcelain", "--untracked-files=all")
	if err != nil {
		t.Fatal(err)
	}
	if status != "" {
		t.Fatalf("identity creation changed tracked or worktree-visible state: %s", status)
	}

	if err := os.WriteFile(trackedFixture, []byte(identity.PrivateKey+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scanTrackedFilesForSecret(identity.PrivateKey); err == nil || !strings.Contains(err.Error(), trackedFixture) {
		t.Fatalf("tracked secret mutation was not detected: %v", err)
	}
	if err := os.WriteFile(trackedFixture, []byte("tracked identity-path sentinel\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scanTrackedFilesForSecret(secret string) (int, error) {
	tracked, err := gitText("ls-files")
	if err != nil {
		return 0, err
	}
	paths := strings.Fields(tracked)
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			return 0, err
		}
		if bytes.Contains(contents, []byte(secret)) {
			return len(paths), fmt.Errorf("tracked file %s contains private key material", path)
		}
	}
	return len(paths), nil
}
