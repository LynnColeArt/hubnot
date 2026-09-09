package main

import (
	"errors"
	"fmt"
	"os"
)

const version = "0.0.1-dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "hn: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "version", "--version":
		fmt.Println(version)
		return nil
	case "init":
		return cmdInit(args[1:])
	case "identity":
		return cmdIdentity(args[1:])
	case "memory":
		return cmdMemory(args[1:])
	case "issue":
		return cmdIssue(args[1:])
	case "proposal":
		return cmdProposal(args[1:])
	case "policy":
		return cmdPolicy(args[1:])
	case "replication":
		return cmdReplication(args[1:])
	case "review":
		return cmdReview(args[1:])
	case "run":
		return cmdRun(args[1:])
	case "runner":
		return cmdRunner(args[1:])
	case "decide":
		return cmdDecide(args[1:])
	case "merge":
		return cmdMerge(args[1:])
	case "sync":
		return cmdSync(args[1:])
	case "log":
		return cmdLog(args[1:])
	default:
		return fmt.Errorf("unknown command %q; run 'hn help'", args[0])
	}
}

func printUsage() {
	fmt.Print(`Hubnot distributes collaboration with a Git repository.

Usage:
  hn init [--name NAME]
  hn identity show|list|public|authorize|accept|rotate
  hn identity show [--json]
  hn memory record --kind KIND --at REV --applies MODE [record fields]
  hn memory record --input FILE|- --json
  hn memory handoff --at REV --applies MODE --input FILE|- --json
  hn memory supersede MEMORY [record fields]
  hn memory retract MEMORY --reason REASON
  hn memory challenge MEMORY --reason REASON [--evidence TYPED-ID]...
  hn memory show MEMORY [--json]
  hn memory recall [filters] [bounds] [--json]
  hn memory index rebuild|verify
  hn issue help
  hn issue open [--body TEXT] TITLE | --input FILE|- [--operation KEY] [--json]
  hn issue revise|resolve ISSUE --expect REV ... --input FILE|- [--json]
  hn issue close|reopen ISSUE --expect REV [--operation KEY] [--json]
  hn issue comment ISSUE [--body TEXT] [--operation KEY] [--json] [TEXT]
  hn issue list [--details] [--limit N] [--cursor TOKEN] [--json]
  hn issue show|heads|history|graph ISSUE [--limit N] [--cursor TOKEN] [--json]
  hn issue operation --actor ACTOR --operation KEY [--json]
  Issue mutations accept optional --actor ACTOR to pin the signing identity.
  hn proposal open --base REV --head REV [--body TEXT] TITLE
  hn proposal revise PREDECESSOR --base REV --head REV [--body TEXT]
  hn proposal list
  hn proposal show PROPOSAL
  hn proposal status PROPOSAL
  hn policy show [REV]
  hn policy check --base REV <--head REV|--file PATH>
  hn review PROPOSAL <--approve|--request-changes> [--body TEXT]
  hn run request PROPOSAL PIPELINE
  hn run list
  hn run show REQUEST
  hn run execute REQUEST [--backend sandbox|host]
  hn run logs RESULT
  hn runner once --accept-pipeline NAME --accept-actor ACTOR
  hn runner watch --accept-pipeline NAME --accept-actor ACTOR
  hn decide PROPOSAL <--accept|--reject> [--body TEXT]
  hn merge PROPOSAL
  hn replication select|show [REMOTE] [full actor fingerprints and event IDs] [budgets]
  hn sync [REMOTE] [--recover-shallow]
  hn log
  hn version
`)
}

func usageError(message string) error {
	return errors.New(message)
}
