package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"hubnot/internal/safepath"
)

const SessionLimit = 4
const StderrLimit = 65536
const RevocationInterval = 250 * time.Millisecond
const TerminateGrace = 2 * time.Second
const ExitBudget = 5 * time.Second

// A slot is an advisory lock held by the owning host until its child exits and
// its synchronous request settles. Metadata identifies sessions, never PIDs.
type sessionSlot struct {
	file     *os.File
	identity slotIdentity
	once     sync.Once
}
type slotIdentity struct {
	SessionID      string `json:"session_id"`
	RegistrationID string `json:"registration_id"`
	Epoch          string `json:"epoch"`
	Cleanup        string `json:"cleanup"`
}

var ownerStates = map[string]bool{"owned": true, "verified": true, "failed": true}

func readOwner(file *os.File) (slotIdentity, error) {
	var id slotIdentity
	data, err := io.ReadAll(io.NewSectionReader(file, 0, 1025))
	value, shapeErr := strictJSON(data, 1024)
	object, ok := value.(map[string]any)
	if err != nil || shapeErr != nil || !ok || !exactFields(object, "session_id", "registration_id", "epoch", "cleanup") || len(data) > 1024 || json.Unmarshal(data, &id) != nil || !validHex(id.SessionID, 32) || !validDigest(id.RegistrationID) || !validHex(id.Epoch, 32) || !ownerStates[id.Cleanup] {
		return id, failure("repository_error", "cannot verify session ownership")
	}
	return id, nil
}
func writeOwner(file *os.File, id slotIdentity) error {
	data, _ := json.Marshal(id)
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.WriteAt(data, 0); err != nil {
		return err
	}
	return file.Sync()
}
func (s *EnablementStore) slotFile(index int) (*os.File, error) {
	if err := s.directory(true); err != nil {
		return nil, err
	}
	path := filepath.Join(filepath.Dir(s.path), fmt.Sprintf("session-%d.lock", index))
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, failure("repository_error", "cannot open session slot")
	}
	f := os.NewFile(uintptr(fd), "extension session slot")
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		f.Close()
		return nil, failure("invalid_input", "unsafe session slot")
	}
	return f, nil
}
func (s *EnablementStore) acquireSlot(id slotIdentity) (*sessionSlot, error) {
	for i := 0; i < SessionLimit; i++ {
		f, err := s.slotFile(i)
		if err != nil {
			return nil, err
		}
		if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			f.Close()
			if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
				continue
			}
			return nil, failure("repository_error", "cannot lock session slot")
		}
		info, _ := f.Stat()
		if info != nil && info.Size() > 0 {
			prior, e := readOwner(f)
			if e != nil || prior.Cleanup != "verified" {
				f.Close()
				return nil, failure("repository_error", "prior session cleanup remains unverified")
			}
		}
		id.Cleanup = "owned"
		if err = writeOwner(f, id); err != nil {
			f.Close()
			return nil, failure("repository_error", "cannot record session ownership")
		}

		return &sessionSlot{file: f, identity: id}, nil
	}
	return nil, failure("resource_limit", "physical repository already has four sessions")
}
func (s *sessionSlot) close(verified bool) {
	s.once.Do(func() {
		s.identity.Cleanup = "verified"
		if !verified {
			s.identity.Cleanup = "failed"
		}
		_ = writeOwner(s.file, s.identity)
		_ = syscall.Flock(int(s.file.Fd()), syscall.LOCK_UN)
		_ = s.file.Close()
	})
}

// WaitRetired observes owner-held locks after an epoch replacement. It never
// signals a PID from disk. The owning host detects revocation and joins its child.
func (s *EnablementStore) WaitRetired(ctx context.Context, registrationID, currentEpoch string) ([]string, error) {
	stopped := map[string]bool{}
	for {
		busy := false
		for i := 0; i < SessionLimit; i++ {
			f, err := s.slotFile(i)
			if err != nil {
				return nil, err
			}
			err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
			if err == nil {
				info, _ := f.Stat()
				if info != nil && info.Size() > 0 {
					id, e := readOwner(f)
					if e != nil {
						f.Close()
						return nil, e
					}
					if id.RegistrationID == registrationID && id.Epoch != currentEpoch && id.Cleanup != "verified" {
						f.Close()
						return nil, failure("repository_error", "prior owner did not verify child cleanup")
					}
				}
				_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
				f.Close()
				continue
			}

			if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
				f.Close()
				return nil, failure("repository_error", "cannot verify session cleanup")
			}
			id, e := readOwner(f)
			f.Close()
			if e != nil {
				busy = true
				continue
			}

			if id.RegistrationID == registrationID && id.Epoch != currentEpoch {
				busy = true
				stopped[id.SessionID] = true
			}
		}
		if !busy {
			ids := make([]string, 0, len(stopped))
			for id := range stopped {
				ids = append(ids, id)
			}
			sortStrings(ids)
			return ids, nil
		}
		select {
		case <-ctx.Done():
			return nil, failure("repository_error", "session cleanup could not be verified")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// childProcess owns its process group and private pipes. Raw stderr is discarded;
// only the bounded byte count influences the public session result.
type childProcess struct {
	command       *exec.Cmd
	input, output *os.File
	done          chan struct{}
	stderrDone    chan struct{}
	overflow      chan struct{}
	err           error
	once          sync.Once
	cleanupErr    error
}

func startChild(r Registration, directory string) (*childProcess, error) {
	cmd := exec.Command(r.Executable, r.Argv...)
	cmd.Dir = directory
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	inputRead, input, err := os.Pipe()
	if err != nil {
		return nil, failure("unavailable_module", "cannot create module input")
	}
	output, outputWrite, err := os.Pipe()
	if err != nil {
		inputRead.Close()
		input.Close()
		return nil, failure("unavailable_module", "cannot create module output")
	}
	stderr, stderrWrite, err := os.Pipe()
	if err != nil {
		inputRead.Close()
		input.Close()
		output.Close()
		outputWrite.Close()
		return nil, failure("unavailable_module", "cannot create module diagnostics")
	}
	cmd.Stdin = inputRead
	cmd.Stdout = outputWrite
	cmd.Stderr = stderrWrite
	p := &childProcess{command: cmd, input: input, output: output, done: make(chan struct{}), stderrDone: make(chan struct{}), overflow: make(chan struct{})}
	err = cmd.Start()
	inputRead.Close()
	outputWrite.Close()
	stderrWrite.Close()
	if err != nil {
		input.Close()
		output.Close()
		stderr.Close()
		return nil, failure("unavailable_module", "cannot launch registered module")
	}
	go func() {
		defer close(p.stderrDone)
		defer stderr.Close()
		var b [4096]byte
		total := 0
		exceeded := false
		for {
			n, e := stderr.Read(b[:])
			if total <= StderrLimit {
				total += n
			}
			if total > StderrLimit && !exceeded {
				close(p.overflow)
				exceeded = true
			}
			if e != nil {
				return
			}
		}
	}()
	go func() { p.err = cmd.Wait(); close(p.done) }()
	return p, nil
}
func (p *childProcess) close() error {
	p.once.Do(func() {
		_ = p.input.Close()
		_ = p.output.Close()
		// Signal only the group created by this handle, including descendants after
		// the leader exits. No process identity is accepted from private metadata.
		_ = syscall.Kill(-p.command.Process.Pid, syscall.SIGTERM)
		grace := time.Now().Add(TerminateGrace)
		for syscall.Kill(-p.command.Process.Pid, 0) == nil && time.Now().Before(grace) {
			time.Sleep(10 * time.Millisecond)
		}
		_ = syscall.Kill(-p.command.Process.Pid, syscall.SIGKILL)

		deadline, deadlineCancel := context.WithTimeout(context.Background(), ExitBudget-TerminateGrace)
		defer deadlineCancel()
		select {
		case <-p.done:
		case <-deadline.Done():
			p.cleanupErr = failure("repository_error", "owned module exit could not be verified")
		}
		select {
		case <-p.stderrDone:
		case <-deadline.Done():
			p.cleanupErr = failure("repository_error", "owned module diagnostics did not close")
		}
	})
	return p.cleanupErr
}

// registrationOwner is a registration-scoped index of handles already owned by
// hosts, across physical repositories. No repository-tree scan or PID signaling
// is involved. A removal tombstone prevents new sessions before cleanup begins.
type registrationOwner struct {
	file          *os.File
	path, control string
	identity      slotIdentity
}

func registrationOwnerDirectory(registrationID string) (string, error) {
	if !validDigest(registrationID) {
		return "", failure("invalid_input", "invalid registration identity")
	}
	registry, err := RegistryPath()
	if err != nil {
		return "", err
	}
	if err = privateDirectory(registry, true); err != nil {
		return "", err
	}
	directory := filepath.Join(filepath.Dir(registry), "owners-"+registrationID[7:])
	if err = os.Mkdir(directory, 0700); err != nil && !os.IsExist(err) {
		return "", failure("repository_error", "cannot create registration ownership directory")
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return "", failure("invalid_input", "unsafe registration ownership directory")
	}
	return directory, nil
}
func privateOwnerFile(path string, create ...bool) (*os.File, error) {
	flags := syscall.O_RDWR | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
	if len(create) == 0 || create[0] {
		flags |= syscall.O_CREAT
	}
	fd, err := syscall.Open(path, flags, 0600)
	if err != nil {
		return nil, failure("repository_error", "cannot open registration ownership")
	}
	file := os.NewFile(uintptr(fd), "registration ownership")
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		file.Close()
		return nil, failure("invalid_input", "unsafe registration ownership")
	}
	return file, nil
}
func lockOwnerControl(directory string) (*os.File, error) {
	file, err := privateOwnerFile(filepath.Join(directory, "control"))
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, failure("repository_error", "registration lifecycle is busy")
	}
	return file, nil
}
func ownerRevoked(control *os.File) (bool, error) {
	var b [2]byte
	n, err := control.ReadAt(b[:], 0)
	if err != nil && err != io.EOF {
		return false, failure("repository_error", "cannot read registration lifecycle")
	}
	if n == 0 {
		return false, nil
	}
	if n != 1 || b[0] != 'R' {
		return false, failure("invalid_input", "invalid registration lifecycle")
	}
	return true, nil
}
func acquireRegistrationOwner(id slotIdentity) (*registrationOwner, error) {
	directory, err := registrationOwnerDirectory(id.RegistrationID)
	if err != nil {
		return nil, err
	}
	control, err := lockOwnerControl(directory)
	if err != nil {
		return nil, err
	}
	defer control.Close()
	revoked, err := ownerRevoked(control)
	if err != nil {
		return nil, err
	}
	if revoked {
		return nil, failure("denied_access", "registration removal revoked this instance")
	}
	path := filepath.Join(directory, id.SessionID+".lock")
	file, err := privateOwnerFile(path)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, failure("repository_error", "session ownership collision")
	}
	id.Cleanup = "owned"
	if err = writeOwner(file, id); err != nil {
		file.Close()
		return nil, failure("repository_error", "cannot record registration ownership")
	}

	return &registrationOwner{file, path, filepath.Join(directory, "control"), id}, nil
}
func (owner *registrationOwner) check() error {
	if err := safepath.CheckDir(filepath.Dir(owner.control), false); err != nil {
		return failure("invalid_input", "unsafe registration ownership directory")
	}
	fd, err := syscall.Open(owner.control, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return failure("repository_error", "registration lifecycle is unavailable")
	}
	file := os.NewFile(uintptr(fd), "registration lifecycle")
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		file.Close()
		return failure("invalid_input", "unsafe registration lifecycle")
	}
	defer file.Close()
	revoked, err := ownerRevoked(file)
	if err != nil {
		return err
	}
	if revoked {
		return failure("denied_access", "registration instance was revoked for removal")
	}
	return nil
}
func (owner *registrationOwner) close(verified bool) {
	if verified {
		owner.identity.Cleanup = "verified"
		_ = writeOwner(owner.file, owner.identity)
		_ = os.Remove(owner.path)
	} else {
		owner.identity.Cleanup = "failed"
		_ = writeOwner(owner.file, owner.identity)
	}
	_ = syscall.Flock(int(owner.file.Fd()), syscall.LOCK_UN)
	_ = owner.file.Close()
}

// RetireRegistration marks this exact installation instance as revoking, then
// waits for its recorded owners in every repository to release after child exit.
// On failure the tombstone remains, so uncertainty cannot revive old authority.
func RetireRegistration(ctx context.Context, registrationID string) ([]string, error) {
	directory, err := registrationOwnerDirectory(registrationID)
	if err != nil {
		return nil, err
	}
	control, err := lockOwnerControl(directory)
	if err != nil {
		return nil, err
	}
	if _, err = ownerRevoked(control); err == nil {
		_, err = control.WriteAt([]byte{'R'}, 0)
	}
	if err == nil {
		err = control.Sync()
	}
	control.Close()
	if err != nil {
		return nil, failure("repository_error", "cannot revoke registration instance")
	}
	stopped := map[string]bool{}
	for {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return nil, failure("repository_error", "cannot inspect registration owners")
		}
		busy := false
		for _, entry := range entries {
			if entry.Name() == "control" {
				continue
			}
			name := entry.Name()
			if len(name) != 37 || name[32:] != ".lock" || !validHex(name[:32], 32) {
				return nil, failure("invalid_input", "invalid registration owner entry")
			}
			path := filepath.Join(directory, name)
			file, err := privateOwnerFile(path, false)
			if err != nil {
				if _, e := os.Lstat(path); os.IsNotExist(e) {
					continue
				}
				return nil, err
			}
			err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
			if err == nil {
				id, e := readOwner(file)
				if e != nil || id.RegistrationID != registrationID || id.SessionID != name[:32] || id.Cleanup != "verified" {
					file.Close()
					return nil, failure("repository_error", "prior registration owner did not verify cleanup")
				}
				_ = os.Remove(path)
				file.Close()
				continue
			}
			file.Close()
			if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
				return nil, failure("repository_error", "cannot verify registration cleanup")
			}
			busy = true
			stopped[name[:32]] = true
		}
		if !busy {
			ids := make([]string, 0, len(stopped))
			for id := range stopped {
				ids = append(ids, id)
			}
			sortStrings(ids)
			return ids, nil
		}
		select {
		case <-ctx.Done():
			return nil, failure("repository_error", "registration session cleanup could not be verified")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
