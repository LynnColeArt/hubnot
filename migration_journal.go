package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"hubnot/internal/migration"
	"hubnot/internal/safepath"
)

type migrationEnvelope struct {
	Data json.RawMessage `json:"data"`
	MAC  string          `json:"mac"`
}

func migrationPrivateDir(path string) error {
	return safepath.CheckDir(path, true)
}
func migrationKey(dir string) ([]byte, error) {
	if e := migrationPrivateDir(dir); e != nil {
		return nil, e
	}
	b, e := readPrivateFileBounded(filepath.Join(dir, "key"), 32)
	if e != nil {
		return nil, e
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("invalid migration authentication key")
	}
	return b, nil
}
func migrationSeal(dir, name string, b []byte) error {
	k, e := migrationKey(dir)
	if e != nil {
		return e
	}
	h := hmac.New(sha256.New, k)
	h.Write([]byte(name + "\x00"))
	h.Write(b)
	raw, _ := json.Marshal(migrationEnvelope{b, hex.EncodeToString(h.Sum(nil))})
	return writePrivateFileAtomic(filepath.Join(dir, name), raw)
}
func migrationUnseal(dir, name string, max int64) ([]byte, error) {
	k, e := migrationKey(dir)
	if e != nil {
		return nil, e
	}
	raw, e := readPrivateFileBounded(filepath.Join(dir, name), max+1024)
	if e != nil {
		return nil, e
	}
	var v migrationEnvelope
	if e = migration.StrictJSON(raw, &v); e != nil {
		return nil, e
	}
	h := hmac.New(sha256.New, k)
	h.Write([]byte(name + "\x00"))
	h.Write(v.Data)
	mac, e := hex.DecodeString(v.MAC)
	if e != nil || !hmac.Equal(mac, h.Sum(nil)) {
		return nil, fmt.Errorf("migration checkpoint authentication failed")
	}
	if int64(len(v.Data)) > max {
		return nil, fmt.Errorf("migration checkpoint exceeds bound")
	}
	return v.Data, nil
}
func migrationInitKey(dir string) error {
	if e := migrationNoLinks(filepath.Dir(dir)); e != nil {
		return e
	}
	if e := os.Mkdir(dir, 0700); e != nil {
		return e
	}
	if e := migrationPrivateDir(dir); e != nil {
		return e
	}
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dir, "key"), key, 0600)
}
func migrationLock(dir string) (func(), error) {
	if e := migrationPrivateDir(dir); e != nil {
		return nil, e
	}
	p := filepath.Join(dir, "apply.lock")
	f, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return nil, fmt.Errorf("migration busy or interrupted lock needs manual recovery: %w", e)
	}
	return func() { _ = f.Close(); _ = os.Remove(p) }, nil
}

func migrationNoLinks(path string) error {
	return safepath.CheckDir(path, false)
}
