//go:build nogithub

package main

import (
	"errors"
	"fmt"
)

func cmdGitHub(args []string) error {
	if len(args) == 0 || len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		fmt.Println("GitHub transport is excluded by the nogithub build. Native hn issue and hn gh issue commands remain available offline.")
		return nil
	}
	return errors.New("GitHub transport is excluded by the nogithub build; use native hn issue or hn gh issue commands")
}
