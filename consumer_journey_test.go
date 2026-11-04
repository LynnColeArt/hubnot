package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// The full consumer journey belongs to the explicit native integration tier.
// Each child is actually built with its own tags; parent test tags do not select
// an executable variant, and these source tests do not certify release binaries.
func TestConsumerPublicJourney(t *testing.T) {
	for _, native := range []bool{false, true} {
		name := "default"
		if native {
			name = "native-only"
		}
		t.Run(name, func(t *testing.T) {
			cli := newBetaContributionCLI(t, runnerAcceptanceBinary(t, native))
			discovery := consumerJSONProcess(t, cli, t.TempDir(), true, "capabilities", "--json")
			var capabilities map[string]json.RawMessage
			if json.Unmarshal(discovery["capabilities"], &capabilities) != nil {
				t.Fatal("capability discovery")
			}
			for _, key := range []string{"native.discovery", "native.proposal-read", "native.log-read", "native.event-read", "native.review-receipt", "native.decision-receipt", "contribution.existing-json"} {
				if capabilities[key] == nil {
					t.Fatalf("compiled variant missing %s: %v", key, capabilities)
				}
			}
			repo, actor, id, _, _, _ := consumerReceiptFixture(t, cli)
			blocked := consumerReadProposal(t, cli, repo, "status", id)
			shown := consumerReadProposal(t, cli, repo, "show", id)
			if blocked.Readiness || blocked.Requirements.Reviews.Deficit != 1 || !reflect.DeepEqual(blocked, shown) {
				t.Fatalf("initial consumer view: %+v %+v", blocked, shown)
			}
			if !strings.Contains(cli.hn(repo, true, "proposal", "show", id), "Receipt candidate") || !strings.Contains(cli.hn(repo, true, "proposal", "status", id), "0/1 trusted approvals") {
				t.Fatal("default proposal reads unavailable")
			}
			changes := consumerJSONProcess(t, cli, repo, true, "review", id, "--request-changes", "--json")
			changesID := consumerReceiptID(t, changes)
			if changesID != cli.latest(repo, actor) || consumerReadProposal(t, cli, repo, "status", id).Readiness {
				t.Fatal("change request attributed/readiness")
			}
			approval := consumerJSONProcess(t, cli, repo, true, "review", id, "--approve", "--json")
			reviewID := consumerReceiptID(t, approval)
			ready := consumerReadProposal(t, cli, repo, "status", id)
			if !ready.Readiness || !reflect.DeepEqual(ready.Evidence, []string{reviewID}) || ready.CurrentReviews[actor].ID != reviewID {
				t.Fatalf("exact current approval: %+v", ready)
			}
			prefix := strings.TrimPrefix(reviewID, "sha256:")[:12]
			if cli.hn(repo, true, "event", "resolve", prefix) != reviewID+"\n" {
				t.Fatal("default full-ID resolution")
			}
			for _, query := range []string{prefix, reviewID} {
				row := consumerJSONProcess(t, cli, repo, true, "event", "show", query, "--json")
				if string(row["id"]) != fmt.Sprintf("%q", reviewID) {
					t.Fatalf("evidence resolution: %v", row)
				}
			}
			decision := consumerJSONProcess(t, cli, repo, true, "decide", id, "--accept", "--json")
			decisionID := consumerReceiptID(t, decision)
			var boundEvidence []string
			var boundPolicy string
			_ = json.Unmarshal(decision["evidence"], &boundEvidence)
			_ = json.Unmarshal(decision["policy"], &boundPolicy)
			if decisionID != cli.latest(repo, actor) || !reflect.DeepEqual(boundEvidence, ready.Evidence) || boundPolicy != ready.Policy {
				t.Fatalf("exact receipt: %v", decision)
			}
			if out := cli.hn(repo, true, "review", id, "--request-changes"); !strings.HasPrefix(out, "Recorded request-changes review ") {
				t.Fatalf("default review output: %s", out)
			}
			current := consumerReadProposal(t, cli, repo, "status", id)
			if current.Readiness || len(current.Evidence) != 0 {
				t.Fatalf("newer evidence did not replace exact actor review: %+v", current)
			}
			historical := consumerJSONProcess(t, cli, repo, true, "event", "show", decisionID, "--json")
			var signed struct {
				Policy   string
				Evidence []string
			}
			if json.Unmarshal(historical["event"], &signed) != nil || signed.Policy != boundPolicy || !reflect.DeepEqual(signed.Evidence, boundEvidence) {
				t.Fatalf("stored receipt changed with current status: %v", historical)
			}
			if out := cli.hn(repo, true, "decide", id, "--reject", "--body", "later decision"); !strings.HasPrefix(out, "Recorded reject decision ") {
				t.Fatalf("default decide output: %s", out)
			}
			if out := cli.hn(repo, true, "review", id, "--approve"); !strings.HasPrefix(out, "Recorded approve review ") {
				t.Fatalf("default approve output: %s", out)
			}
			rejected := consumerJSONProcess(t, cli, repo, true, "decide", id, "--reject", "--body", "JSON rejection", "--json")
			if string(rejected["verdict"]) != `"reject"` || string(rejected["evidence"]) != "[]" {
				t.Fatalf("rejection receipt: %v", rejected)
			}
			if out := cli.hn(repo, true, "decide", id, "--accept"); !strings.HasPrefix(out, "Recorded accept decision ") || !strings.Contains(out, "Evidence: 1 signed event(s)\n") {
				t.Fatalf("default accept output: %s", out)
			}
			logged := consumerJSONProcess(t, cli, repo, true, "log", "--json")
			if string(logged["history_scope"]) != `"raw_signed_history"` {
				t.Fatal("raw log scope")
			}
			text := cli.hn(repo, true, "log")
			for _, eventID := range []string{id, changesID, reviewID, decisionID} {
				if !strings.Contains(string(logged["events"]), eventID) || !strings.Contains(text, strings.TrimPrefix(eventID, "sha256:")[:12]) {
					t.Fatalf("full/text history lost %s", eventID)
				}
			}
			for _, machine := range []bool{false, true} {
				args := []string{"review", id, "--approve", "--unknown"}
				if machine {
					args = append(args, "--json")
				}
				before := consumerFiles(t, repo)
				if machine {
					consumerJSONProcess(t, cli, repo, false, args...)
				} else {
					cli.hn(repo, false, args...)
				}
				if !reflect.DeepEqual(before, consumerFiles(t, repo)) {
					t.Fatalf("refusal changed state in %s: %v", name, args)
				}
			}
		})
	}
}
