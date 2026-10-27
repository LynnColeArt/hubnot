package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestMigrationWizardNeverConsentsAtEOFOrEmpty(t *testing.T) {
	for _, input := range []string{"", "\n", "yes", "yes\n"} {
		var out bytes.Buffer
		w := newMigrationWizard(strings.NewReader(input), &out)
		answer, err := w.answer("Decision", true)
		if input == "yes\n" {
			if err != nil || answer != "yes" {
				t.Fatalf("answer=%q err=%v", answer, err)
			}
		} else if err == nil {
			t.Fatalf("incomplete input %q accepted as %q", input, answer)
		}
	}
}

func TestMigrationArgumentsPreserveExplicitEmptyMapping(t *testing.T) {
	o, err := parseMigrationOptions([]string{"plan", "--map", "type:issue=", "--map", "state:opened=open", "--decision", "loss=preserve"})
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := o.mappings["type:issue"]; !ok || value != "" {
		t.Fatal("explicit empty mapping lost")
	}
	for _, args := range [][]string{{"plan", "--yes"}, {"resume", "extra"}, {"plan", "--map", "a=b", "--map", "a=c"}, {"plan", "--decision", "x=yes"}} {
		if _, err := parseMigrationOptions(args); err == nil {
			t.Fatalf("accepted invalid input %v", args)
		}
	}
}
