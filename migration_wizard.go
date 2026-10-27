package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"hubnot/internal/migration"
)

// A newline completes an answer. Partial input at EOF is never a decision.
type migrationWizard struct {
	in  *bufio.Reader
	out io.Writer
}

func newMigrationWizard(in io.Reader, out io.Writer) *migrationWizard {
	return &migrationWizard{bufio.NewReader(io.LimitReader(in, 1<<20)), out}
}
func (w *migrationWizard) answer(prompt string, required bool) (string, error) {
	fmt.Fprint(w.out, prompt+": ")
	line, err := w.in.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("%s: EOF or unreadable input; no decision or approval recorded", prompt)
	}
	if len(line) > 8192 {
		return "", fmt.Errorf("answer exceeds 8192 bytes")
	}
	value := strings.TrimSpace(line)
	if required && value == "" {
		return "", fmt.Errorf("%s requires an explicit answer; empty input is not consent", prompt)
	}
	return value, nil
}

func (w *migrationWizard) fill(value *string, name string, optional bool) error {
	if *value != "" {
		return nil
	}
	answer, err := w.answer(name+" ("+map[bool]string{true: "- for none", false: "required"}[optional]+")", true)
	if err != nil {
		return err
	}
	if optional && answer == "-" {
		answer = ""
	}
	*value = answer
	return nil
}

func (w *migrationWizard) configure(side string, c *migration.ProviderConfig) error {
	if err := w.fill(&c.Locator.Kind, side+" kind", false); err != nil {
		return err
	}
	if migrationCLIForeign(c.Locator.Kind) {
		for _, field := range []struct {
			value    *string
			name     string
			optional bool
		}{
			{&c.Locator.Endpoint, "endpoint", false}, {&c.Locator.Organization, "organization/collection", true},
			{&c.Locator.Project, "project", true}, {&c.Locator.Repository, "repository selector", true},
			{&c.APIVersion, "API version/profile (see hn migrate help)", false}, {&c.CredentialMode, "credential mode", false},
		} {
			if err := w.fill(field.value, side+" "+field.name, field.optional); err != nil {
				return err
			}
		}
	} else if err := w.fill(&c.Locator.Repository, side+" repository/archive/session path", false); err != nil {
		return err
	}
	return nil
}

func (w *migrationWizard) mappings(m map[string]string) error {
	for {
		value, err := w.answer("Mapping KEY=VALUE (empty VALUE is explicit omission), or done", true)
		if err != nil {
			return err
		}
		if value == "done" {
			return nil
		}
		choice := map[string]string{}
		if err = migrationCLIMap(choice, value); err != nil {
			return err
		}
		for key, mapped := range choice {
			m[key] = mapped
		}
	}
}
