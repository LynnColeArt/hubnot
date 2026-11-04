package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"hubnot/internal/migration"
)

const runnerTargetTimeout = 10 * time.Second
const runnerTargetOutputLimit = 64 << 10

func validateRunnerTargetOptions(skip bool, target string) error {
	if skip != (target != "") {
		return fmt.Errorf("--skip-stale-base and --target refs/heads/NAME must be supplied together")
	}
	if skip && (!strings.HasPrefix(target, "refs/heads/") || !migration.ValidRef(target)) {
		return fmt.Errorf("runner --target must be a full valid refs/heads/NAME")
	}
	return nil
}

// Keep ordinary config-file credentials, HOME/XDG and the SSH agent. Ambient
// repository/object/config overrides must not redirect discovery or transport.
func runnerTargetEnvironment() []string {
	env := make([]string, 0, len(os.Environ())+6)
	blocked := map[string]bool{"GIT_DIR": true, "GIT_COMMON_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true, "GIT_OBJECT_DIRECTORY": true, "GIT_ALTERNATE_OBJECT_DIRECTORIES": true, "GIT_CEILING_DIRECTORIES": true, "GIT_DISCOVERY_ACROSS_FILESYSTEM": true, "GIT_NAMESPACE": true, "GIT_REPLACE_REF_BASE": true, "GIT_NO_REPLACE_OBJECTS": true, "GIT_EXEC_PATH": true, "GIT_SHALLOW_FILE": true, "GIT_GRAFT_FILE": true, "GIT_QUARANTINE_PATH": true, "GIT_IMPLICIT_WORK_TREE": true, "GIT_CONFIG": true, "GIT_ASKPASS": true, "GIT_TERMINAL_PROMPT": true, "GIT_NO_LAZY_FETCH": true, "SSH_ASKPASS": true, "SSH_ASKPASS_REQUIRE": true}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if blocked[name] || strings.HasPrefix(name, "GIT_CONFIG_") || strings.HasPrefix(name, "GIT_TRACE") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "SSH_ASKPASS=", "SSH_ASKPASS_REQUIRE=never")
}

// One budget covers both pipes. Overflow cancels the owned process group rather
// than retaining an unbounded diagnostic or waiting for a stalled producer.
type runnerTargetBudget struct {
	mu        sync.Mutex
	remaining int
}

type runnerTargetOutput struct {
	budget   *runnerTargetBudget
	stdout   bytes.Buffer
	overflow bool
	cancel   context.CancelFunc
}
type runnerTargetWriter struct {
	output *runnerTargetOutput
	retain bool
}

func (w runnerTargetWriter) Write(p []byte) (int, error) {
	o := w.output
	o.budget.mu.Lock()
	defer o.budget.mu.Unlock()
	if len(p) > o.budget.remaining {
		o.overflow = true
		o.cancel()
		return 0, fmt.Errorf("target probe output limit exceeded")
	}
	o.budget.remaining -= len(p)
	if w.retain {
		return o.stdout.Write(p)
	}
	return len(p), nil
}
func runnerTargetGit(parent context.Context, env []string, budget *runnerTargetBudget, args ...string) ([]byte, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	argv := append([]string{"--no-optional-locks", "-c", "core.askPass=", "-c", "credential.interactive=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", argv...)
	cmd.Env = env
	cmd.Stdin = strings.NewReader("")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 250 * time.Millisecond
	output := &runnerTargetOutput{cancel: cancel, budget: budget}
	cmd.Stdout = runnerTargetWriter{output: output, retain: true}
	cmd.Stderr = runnerTargetWriter{output: output}
	err := cmd.Run()
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	if output.overflow {
		return nil, fmt.Errorf("runner target observation exceeded 64KiB output")
	}
	if parent.Err() != nil {
		return nil, fmt.Errorf("runner target observation cancelled: %w", parent.Err())
	}
	if err != nil {
		return nil, err
	}
	return output.stdout.Bytes(), nil
}
func runnerTargetConfig(ctx context.Context, env []string, budget *runnerTargetBudget, key string) (string, error) {
	raw, err := runnerTargetGit(ctx, env, budget, "config", "--get", key)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("cannot read configured Git transport options")
	}
	return strings.TrimSuffix(string(raw), "\n"), nil
}
func runnerTargetEnvValue(env []string, name string) string {
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		if key == name {
			return value
		}
	}
	return ""
}

// Accept literal OpenSSH argv (including quoted identity/proxy arguments), not
// shell programs. Refusing a wrapper is preferable to silently dropping auth.
func runnerSSHWords(command string) ([]string, error) {
	var words []string
	var word strings.Builder
	var quote rune
	active, escape := false, false
	for _, c := range command {
		if escape {
			if c == '\n' || c == '\r' {
				return nil, fmt.Errorf("unsupported SSH command")
			}
			if quote == '"' && !strings.ContainsRune("$`\"\\", c) {
				word.WriteByte('\\')
			}
			word.WriteRune(c)
			active = true
			escape = false
			continue
		}
		if c == '\\' && quote != '\'' {
			escape = true
			active = true
			continue
		}
		if c == '\n' || c == '\r' || c == 0 {
			return nil, fmt.Errorf("unsupported SSH command")
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				if quote == '"' && (c == '$' || c == '`') {
					return nil, fmt.Errorf("unsupported SSH command")
				}
				word.WriteRune(c)
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			active = true
		case ' ', '\t':
			if active {
				words = append(words, word.String())
				word.Reset()
				active = false
			}
		case '$', '`', ';', '|', '&', '<', '>', '(', ')', '*', '?', '[', ']':
			return nil, fmt.Errorf("unsupported SSH command")
		default:
			word.WriteRune(c)
			active = true
		}
	}
	if escape || quote != 0 {
		return nil, fmt.Errorf("unsupported SSH command")
	}
	if active {
		words = append(words, word.String())
	}
	if len(words) == 0 || filepath.Base(words[0]) != "ssh" {
		return nil, fmt.Errorf("unsupported SSH command")
	}
	return words, nil
}
func runnerSSHCommand(words []string) string {
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = "'" + strings.ReplaceAll(w, "'", "'\\''") + "'"
	}
	return strings.Join(quoted, " ")
}
func runnerTargetSSH(ctx context.Context, env []string, budget *runnerTargetBudget) ([]string, error) {
	command := runnerTargetEnvValue(env, "GIT_SSH_COMMAND")
	if command == "" {
		var err error
		command, err = runnerTargetConfig(ctx, env, budget, "core.sshCommand")
		if err != nil {
			return nil, err
		}
	}
	if command == "" {
		command = runnerTargetEnvValue(env, "GIT_SSH")
		if command != "" {
			command = runnerSSHCommand([]string{command})
		}
	}
	if command == "" {
		command = "ssh"
	}
	variant := runnerTargetEnvValue(env, "GIT_SSH_VARIANT")
	if variant == "" {
		var err error
		variant, err = runnerTargetConfig(ctx, env, budget, "ssh.variant")
		if err != nil {
			return nil, err
		}
	}
	words, err := runnerSSHWords(command)
	if err != nil || (variant != "" && variant != "auto" && variant != "ssh") {
		return nil, fmt.Errorf("runner target probe requires standard OpenSSH; configure core.sshCommand as ssh with literal identity/proxy arguments or use a supported remote transport")
	}
	words = append([]string{words[0], "-oBatchMode=yes"}, words[1:]...)
	filtered := make([]string, 0, len(env)+2)
	for _, entry := range env {
		if !strings.HasPrefix(entry, "GIT_SSH_COMMAND=") && !strings.HasPrefix(entry, "GIT_SSH_VARIANT=") {
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, "GIT_SSH_COMMAND="+runnerSSHCommand(words), "GIT_SSH_VARIANT=ssh"), nil
}
func runnerTargetUsesSSH(remote string) bool {
	if strings.Contains(remote, "://") {
		return strings.HasPrefix(remote, "ssh://") || strings.HasPrefix(remote, "git+ssh://") || strings.HasPrefix(remote, "ssh+git://")
	}
	colon := strings.IndexByte(remote, ':')
	slash := strings.IndexByte(remote, '/')
	return colon > 0 && (slash < 0 || colon < slash)
}
func observeRunnerTarget(parent context.Context, remote, target string) (string, error) {
	if err := validateRunnerTargetOptions(true, target); err != nil {
		return "", err
	}
	if remote == "" || strings.HasPrefix(remote, "-") || strings.ContainsAny(remote, "\x00\r\n") {
		return "", fmt.Errorf("runner target probe requires a configured Git remote name")
	}
	ctx, cancel := context.WithTimeout(parent, runnerTargetTimeout)
	defer cancel()
	env := runnerTargetEnvironment()
	budget := &runnerTargetBudget{remaining: runnerTargetOutputLimit}
	raw, err := runnerTargetGit(ctx, env, budget, "remote", "get-url", "--", remote)
	if err != nil {
		return "", fmt.Errorf("cannot resolve configured runner remote: %w", err)
	}
	address := strings.TrimSuffix(string(raw), "\n")
	if address == "" || strings.ContainsAny(address, "\x00\r\n") {
		return "", fmt.Errorf("runner remote has an invalid fetch URL")
	}
	if runnerTargetUsesSSH(address) {
		env, err = runnerTargetSSH(ctx, env, budget)
		if err != nil {
			return "", err
		}
	}
	raw, err = runnerTargetGit(ctx, env, budget, "ls-remote", "--refs", "--exit-code", "--", remote, target)
	if err != nil {
		return "", fmt.Errorf("cannot observe exact runner target (check remote/authentication and retry): %w", err)
	}
	line := strings.TrimSuffix(string(raw), "\n")
	oid, ref, ok := strings.Cut(line, "\t")
	if !ok || ref != target || !migration.ValidOID(oid) || strings.ContainsAny(line, "\r\n") {
		return "", fmt.Errorf("runner target advertisement is missing, malformed or ambiguous")
	}
	return oid, nil
}
