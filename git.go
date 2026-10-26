package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func gitOutput(args ...string) ([]byte, error) {
	return gitInput(nil, nil, args...)
}

func gitText(args ...string) (string, error) {
	out, err := gitOutput(args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitInput(input []byte, extraEnv []string, args ...string) ([]byte, error) {
	return gitInputWithDirectory("", input, extraEnv, args...)
}

func gitOutputAt(gitDir string, args ...string) ([]byte, error) {
	return gitInputAt(gitDir, nil, args...)
}

func gitTextAt(gitDir string, args ...string) (string, error) {
	out, err := gitOutputAt(gitDir, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitInputAt(gitDir string, input []byte, args ...string) ([]byte, error) {
	return gitInputWithDirectory(gitDir, input, nil, args...)
}

func gitInputWithDirectory(gitDir string, input []byte, extraEnv []string, args ...string) ([]byte, error) {
	if gitDir != "" {
		args = append([]string{"--git-dir=" + gitDir}, args...)
	}
	cmd := exec.Command("git", args...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = append(os.Environ(), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
	}
	return stdout.Bytes(), nil
}

// copyGitObjects streams one ordinary, non-thin pack from the quarantine
// object database into the main repository without creating a ref or writing
// FETCH_HEAD. Accepted refs are updated separately after every object is
// verified in the destination.
func copyGitObjects(sourceGitDir, destinationGitDir string, roots []string) error {
	if len(roots) == 0 {
		return nil
	}
	input := strings.NewReader(strings.Join(roots, "\n") + "\n")
	// --local leaves out objects the source borrows through alternates; those
	// are already in the destination when the source is a quarantine that
	// borrows the destination's object database.
	producer := exec.Command("git", "--git-dir="+sourceGitDir, "pack-objects", "--stdout", "--revs", "--local")
	consumer := exec.Command("git", "--git-dir="+destinationGitDir, "index-pack", "--stdin")
	pipe, err := producer.StdoutPipe()
	if err != nil {
		return fmt.Errorf("prepare quarantine object copy: %w", err)
	}
	producer.Stdin = input
	var producerError, consumerError bytes.Buffer
	producer.Stderr = &producerError
	consumer.Stdin = pipe
	consumer.Stdout = io.Discard
	consumer.Stderr = &consumerError
	if err := consumer.Start(); err != nil {
		return fmt.Errorf("start object import: %w", err)
	}
	if err := producer.Start(); err != nil {
		_ = consumer.Process.Kill()
		_ = consumer.Wait()
		return fmt.Errorf("start quarantine object export: %w", err)
	}
	producerErr := producer.Wait()
	consumerErr := consumer.Wait()
	if producerErr != nil {
		return fmt.Errorf("export quarantine objects: %s", commandFailure(producerErr, producerError.String()))
	}
	if consumerErr != nil {
		return fmt.Errorf("import quarantine objects: %s", commandFailure(consumerErr, consumerError.String()))
	}
	return nil
}

func commandFailure(err error, stderr string) string {
	message := strings.TrimSpace(stderr)
	if message == "" {
		message = err.Error()
	}
	return message
}

func requireGitRepository() (string, error) {
	gitDir, err := gitText("rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", fmt.Errorf("not inside a Git repository")
	}
	return gitDir, nil
}

func refValue(ref string) (string, bool, error) {
	out, err := gitOutput("rev-parse", "--verify", "--quiet", ref)
	if err != nil {
		// rev-parse returns status 1 when a ref is absent. Check with show-ref so
		// actual repository errors are still surfaced by callers elsewhere.
		return "", false, nil
	}
	return strings.TrimSpace(string(out)), true, nil
}
