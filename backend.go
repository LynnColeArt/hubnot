package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type executionBackend interface {
	Name() string
	Available() error
	RunStep(context.Context, string, PipelineStep, []string, io.Writer) error
}

type hostBackend struct{}

func (hostBackend) Name() string     { return "host" }
func (hostBackend) Available() error { return nil }

func (hostBackend) RunStep(ctx context.Context, root string, step PipelineStep, environment []string, output io.Writer) error {
	workingDirectory := pipelineWorkingDirectory(root, step.WorkingDirectory)
	commandName := step.Command
	if strings.ContainsAny(commandName, `/\`) && !filepath.IsAbs(commandName) {
		commandName = filepath.Join(workingDirectory, commandName)
	}
	command := exec.CommandContext(ctx, commandName, step.Args...)
	command.Dir = workingDirectory
	command.Env = environment
	command.Stdout = output
	command.Stderr = output
	return command.Run()
}

type bubblewrapBackend struct {
	binary string
}

func newBubblewrapBackend() bubblewrapBackend {
	for _, directory := range sandboxSystemDirectories() {
		candidate := filepath.Join(directory, "bwrap")
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return bubblewrapBackend{binary: candidate}
		}
	}
	return bubblewrapBackend{}
}

func (backend bubblewrapBackend) Name() string { return "sandbox" }

func (backend bubblewrapBackend) Available() error {
	if backend.binary == "" {
		return fmt.Errorf("bubblewrap is not installed; install bwrap or explicitly select --backend host")
	}
	return nil
}

func (backend bubblewrapBackend) RunStep(ctx context.Context, root string, step PipelineStep, environment []string, output io.Writer) error {
	if err := backend.Available(); err != nil {
		return err
	}
	workingDirectory := "/workspace"
	if step.WorkingDirectory != "" && step.WorkingDirectory != "." {
		workingDirectory += "/" + filepath.ToSlash(filepath.Clean(step.WorkingDirectory))
	}
	commandName := step.Command
	if strings.ContainsAny(commandName, `/\`) && !filepath.IsAbs(commandName) {
		commandName = workingDirectory + "/" + filepath.ToSlash(filepath.Clean(commandName))
	}

	arguments := []string{
		"--die-with-parent",
		"--new-session",
		"--unshare-all",
		"--cap-drop", "ALL",
		"--clearenv",
	}
	for _, directory := range []string{"/usr", "/bin", "/lib", "/lib64", "/sbin"} {
		if _, err := os.Stat(directory); err == nil {
			arguments = append(arguments, "--ro-bind", directory, directory)
		}
	}
	arguments = append(arguments,
		"--bind", root, "/workspace",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--tmpfs", "/home",
		"--dir", "/home/hn",
	)
	for _, variable := range environment {
		name, value, found := strings.Cut(variable, "=")
		if found {
			arguments = append(arguments, "--setenv", name, value)
		}
	}
	arguments = append(arguments, "--chdir", workingDirectory, "--", commandName)
	arguments = append(arguments, step.Args...)

	command := exec.CommandContext(ctx, backend.binary, arguments...)
	command.Env = environment
	command.Stdout = output
	command.Stderr = output
	return command.Run()
}

func selectBackend(name string, allowUnsafeHost bool) (executionBackend, error) {
	var backend executionBackend
	switch name {
	case "", "sandbox":
		selected := newBubblewrapBackend()
		backend = selected
	case "host":
		if !allowUnsafeHost {
			return nil, fmt.Errorf("host execution requires --allow-unsafe-host-execution")
		}
		backend = hostBackend{}
	default:
		return nil, fmt.Errorf("unknown execution backend %q; choose sandbox or host", name)
	}
	if err := backend.Available(); err != nil {
		return nil, err
	}
	return backend, nil
}

func pipelineWorkingDirectory(root, relative string) string {
	if relative == "" || relative == "." {
		return root
	}
	return filepath.Join(root, filepath.Clean(relative))
}

func sandboxSystemDirectories() []string {
	return []string{
		"/usr/local/sbin",
		"/usr/local/bin",
		"/usr/sbin",
		"/usr/bin",
		"/sbin",
		"/bin",
	}
}

func sandboxPath() string {
	return strings.Join(append(sandboxSystemDirectories(), "/usr/local/go/bin"), string(os.PathListSeparator))
}
