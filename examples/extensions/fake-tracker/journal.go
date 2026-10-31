package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

const frameBytes = 8388608
const privateBytes = 16777216
const inputBytes = 131072

type config struct {
	API        string `json:"api"`
	Token      string `json:"token"`
	Connection string `json:"connection"`
	Source     string `json:"source"`
	Journal    string `json:"journal"`
	Report     string `json:"report"`
	Marker     string `json:"marker"`
}
type plan struct {
	Action          string         `json:"action"`
	Request         map[string]any `json:"request"`
	Pause           string         `json:"pause"`
	Fresh           bool           `json:"fresh"`
	Limit           int            `json:"limit"`
	LoseExternalAck bool           `json:"lose_external_ack"`
}
type nativeIntent struct {
	Params  map[string]any `json:"params"`
	Result  map[string]any `json:"result"`
	Lookup  map[string]any `json:"lookup"`
	Refusal map[string]any `json:"refusal"`
}
type externalRequest struct {
	Key       string   `json:"key"`
	Expected  int      `json:"expected_version"`
	Source    string   `json:"source"`
	Resource  string   `json:"resource"`
	Revisions []string `json:"revisions"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	LoseAck   bool     `json:"lose_ack"`
}
type receipt struct {
	Request externalRequest `json:"request"`
	Version int             `json:"version"`
}
type externalIntent struct {
	Request externalRequest `json:"request"`
	Receipt *receipt        `json:"receipt"`
	Failure string          `json:"failure"`
}
type journal struct {
	Repository      string                     `json:"repository"`
	Module          string                     `json:"module"`
	Registration    string                     `json:"registration"`
	Connection      string                     `json:"connection"`
	Source          string                     `json:"source"`
	API             string                     `json:"api"`
	Credential      string                     `json:"credential"`
	Actor           string                     `json:"actor"`
	PublicKey       string                     `json:"public_key"`
	Native          map[string]*nativeIntent   `json:"native"`
	External        map[string]*externalIntent `json:"external"`
	ExternalVersion int                        `json:"external_version"`
	InitialItem     *externalItem              `json:"initial_item"`
	NativeHead      string                     `json:"native_head"`
	Baseline        map[string][]string        `json:"baseline"`
	Entries         map[string]map[string]any  `json:"entries"`
	Selection       string                     `json:"selection"`
	Snapshot        string                     `json:"snapshot"`
	Checkpoint      string                     `json:"checkpoint"`
}

func readBounded(r io.Reader, limit int) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > limit {
		return nil, errors.New("resource_limit")
	}
	return raw, nil
}
func readPrivate(path string, limit int, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		return errors.New("private_file_permissions")
	}
	raw, err := readBounded(f, limit)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err = dec.Decode(v); err != nil {
		return errors.New("invalid_private_input")
	}
	if dec.Decode(new(any)) != io.EOF {
		return errors.New("invalid_private_input")
	}
	return nil
}

// Rename plus file/directory sync makes each transaction durable before effects.
// No journal or credential is placed in repository metadata or public objects.
func writePrivate(path string, v any, limit int) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(raw) > limit {
		return errors.New("resource_limit")
	}
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tracker-transaction-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func lockJournal(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("connector_busy")
	}
	return f, nil
}
func loadJournal(c config, binding map[string]any) (*journal, error) {
	j := &journal{}
	err := readPrivate(c.Journal, privateBytes, j)
	if os.IsNotExist(err) {
		return &journal{Repository: textField(binding, "repository_id"), Module: textField(binding, "module_id"), Registration: textField(binding, "registration_id"), Connection: c.Connection, Source: c.Source, API: c.API, Credential: c.Token, Native: map[string]*nativeIntent{}, External: map[string]*externalIntent{}, Baseline: map[string][]string{}, Entries: map[string]map[string]any{}}, nil
	}
	if err != nil {
		return nil, err
	}
	if j.Repository != binding["repository_id"] || j.Module != binding["module_id"] || j.Registration != binding["registration_id"] || j.Connection != c.Connection || j.Source != c.Source || j.API != c.API || j.Credential != c.Token {
		return nil, errors.New("connector_binding_changed")
	}
	if j.Native == nil || j.External == nil || j.Baseline == nil || j.Entries == nil {
		return nil, fmt.Errorf("invalid_private_journal")
	}
	return j, nil
}
