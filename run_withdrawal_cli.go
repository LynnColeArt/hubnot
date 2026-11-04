package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Only these exact static forms bypass repository/authority inspection and
// operation locks. Discovery is implementation metadata, never authorization.
func isStaticRunDiscovery(args []string) bool {
	return len(args) == 1 && args[0] == "help" || len(args) == 2 && args[0] == "capabilities" && args[1] == "--json"
}

const runHelp = `Usage:
  hn run request PROPOSAL PIPELINE
  hn run list
  hn run show REQUEST
  hn run execute FULL_REQUEST [--backend sandbox|host] [--allow-unsafe-host-execution] [--rerun]
  hn run logs RESULT
  hn run withdraw FULL_REQUEST --reason TEXT [--json]
  hn run help
  hn run capabilities --json

Withdrawal is an explicit original-requester-only signed fact. The request ID
must be full; the trimmed reason must be nonblank UTF-8, at most 4096 bytes.
Exact normalized retries return the original fact. Different reasons or actors
refuse; concurrent writers may need to retry after observing the winning fact.
Only future starts are prevented. Already-running/disconnected runs may finish
and publish valid results; historical results, decisions and merges remain valid.
A new explicit request is distinct intent and may be executed manually, while
automatic runners may skip work already qualified by trusted results.
Run 'hn sync REMOTE' explicitly to publish local requests and withdrawals.
Writing run.withdraw advances hn/0 actor history; older readers may reject this
new event kind and that actor advancement. No new fact is written implicitly.
Help/capabilities are static, keyless reads and grant no execution, trust,
moderation or extension RPC permission. No daemon or policy is enabled.
`

type runWithdrawalCapabilities struct {
	EventKind             string `json:"event_kind"`
	Scope                 string `json:"scope"`
	RequesterOnly         bool   `json:"requester_only"`
	ExactRetry            bool   `json:"exact_retry"`
	CancelsRunning        bool   `json:"cancels_running"`
	PreservesEvidence     bool   `json:"preserves_evidence"`
	ReasonMaxBytes        int    `json:"reason_max_bytes"`
	Publication           string `json:"publication"`
	OlderReadersMayReject bool   `json:"older_readers_may_reject"`
}

func cmdRunCapabilities(args []string) error {
	if len(args) != 1 || args[0] != "--json" {
		return usageError("usage: hn run capabilities --json")
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Schema          string                    `json:"schema"`
		EventProtocol   string                    `json:"event_protocol"`
		GrantsAuthority bool                      `json:"grants_authority"`
		Commands        []string                  `json:"commands"`
		Withdrawal      runWithdrawalCapabilities `json:"withdrawal"`
	}{
		Schema: "hn.run-capabilities/1", EventProtocol: protocolVersion,
		Commands:   []string{"request", "list", "show", "execute", "logs", "withdraw", "help", "capabilities"},
		Withdrawal: runWithdrawalCapabilities{EventKind: "run.withdraw", Scope: "future-starts", RequesterOnly: true, ExactRetry: true, PreservesEvidence: true, ReasonMaxBytes: maxRunWithdrawalReason, Publication: "explicit-sync", OlderReadersMayReject: true},
	})
}

func guardRunNotWithdrawn(events []StoredEvent, requestID string) error {
	if withdrawal := findRunWithdrawal(events, requestID); withdrawal != nil {
		return fmt.Errorf("request %s was withdrawn by its requester (%s); create a new explicit request for new intent", requestID, withdrawal.ID)
	}
	return nil
}

type runWithdrawalReceipt struct {
	Schema       string `json:"schema"`
	RequestID    string `json:"request_id"`
	WithdrawalID string `json:"withdrawal_id"`
	Replayed     bool   `json:"replayed"`
	Publication  string `json:"publication"`
}

func printRunWithdrawal(stored *StoredEvent, replayed, machine bool) error {
	receipt := runWithdrawalReceipt{Schema: "hn.run-withdrawal/1", RequestID: stored.Event.Subject, WithdrawalID: stored.ID, Replayed: replayed, Publication: "explicit-sync"}
	if machine {
		return json.NewEncoder(os.Stdout).Encode(receipt)
	}
	if replayed {
		fmt.Printf("Withdrawal already recorded: %s\n", stored.ID)
	} else {
		fmt.Printf("Recorded withdrawal: %s\n", stored.ID)
	}
	fmt.Printf("Request: %s\nReason: %s\n", stored.Event.Subject, safeText(stored.Event.Body))
	fmt.Println("Recorded locally; run 'hn sync REMOTE' to publish this withdrawal. Only future starts are prevented.")
	return nil
}
func cmdRunWithdraw(args []string) error {
	const usage = "usage: hn run withdraw FULL_REQUEST --reason TEXT [--json]"
	if len(args) < 1 {
		return usageError(usage)
	}
	requestID := args[0]
	if err := requireFullEventID(requestID); err != nil {
		return err
	}
	flags := quietFlags("run withdraw")
	reasonFlag := flags.String("reason", "", "reason for withdrawing future starts")
	machine := flags.Bool("json", false, "emit the exact request and withdrawal IDs")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError(usage)
	}
	reason := strings.TrimSpace(*reasonFlag)
	if err := validateRunWithdrawalContent(Event{Kind: "run.withdraw", Subject: requestID, Body: reason}); err != nil {
		return err
	}
	// The wire layer uses this same complete-history guard before signing. Unlike
	// run execute, withdrawal requires no available code, pipeline or backend.
	if err := guardShallowEventClosure("run withdraw"); err != nil {
		return err
	}
	events, err := collectEvents()
	if err != nil {
		return err
	}
	request, err := resolveEventDependency("run withdraw", requestID, shallowRunRequest, events)
	if err != nil {
		return err
	}
	if request.Event.Kind != "run.request" {
		return fmt.Errorf("%s is not a run request", requestID)
	}
	// Inspection does not migrate a legacy keyring. Refusals and exact retries
	// must not change signer state merely to discover who would sign.
	identity, err := inspectIdentity()
	if err != nil {
		return err
	}
	if identity.Actor != request.Event.Actor || identity.PublicKey != request.Event.PublicKey {
		return fmt.Errorf("run withdrawal requires the original requester identity")
	}
	if previous := findRunWithdrawal(events, requestID); previous != nil {
		if previous.Event.Body != reason {
			return fmt.Errorf("request already withdrawn with a different reason; exact retry must preserve the original reason")
		}
		return printRunWithdrawal(previous, true, *machine)
	}
	identity, err = loadIdentity()
	if err != nil {
		return err
	}
	if identity.Actor != request.Event.Actor || identity.PublicKey != request.Event.PublicKey {
		return fmt.Errorf("requester identity changed before withdrawal; retry")
	}
	event, err := nextEvent(identity, "run.withdraw")
	if err != nil {
		return err
	}
	event.Subject, event.Body = requestID, reason
	stored, err := appendEvent(event, identity)
	if err != nil {
		return err
	}
	return printRunWithdrawal(stored, false, *machine)
}
