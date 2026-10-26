package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCommandUsageAndInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--version", "1.2.3"}, {"--unknown"}, {"--version", "1.2.3", "--out", "out", "unexpected"}} {
		var out bytes.Buffer
		err := run(context.Background(), args, &out)
		if len(args) == 1 && args[0] == "--help" {
			if err != nil || !strings.Contains(out.String(), "--rev") {
				t.Fatalf("help: %q %v", out.String(), err)
			}
		} else if err == nil {
			t.Fatalf("accepted incomplete arguments %v", args)
		}
	}
}
