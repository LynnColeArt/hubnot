package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestOperationalSelfHostingAlphaCrashRecoveryDeniesUnacceptedFacts(t *testing.T) {
	binary := buildOperationalBinary(t)
	attacks := []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{name: "missing receipt", mutate: func(t *testing.T, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "corrupt receipt", mutate: func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("{not-json\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "receipt symlink", mutate: func(t *testing.T, path string) {
			target := filepath.Join(t.TempDir(), "receipt.json")
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unsafe receipt mode", mutate: func(t *testing.T, path string) {
			if err := os.Chmod(path, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, attack := range attacks {
		t.Run(attack.name, func(t *testing.T) {
			fixture := setupShallowRecoveryFixture(t, defaultReplicationBudgets())
			crashed := runOperationalCommandFailureWithEnv(t, binary, fixture.clone, map[string]string{
				"HN_INTERNAL_TESTING":                 "1",
				"HN_TEST_REPLICATION_INTERRUPT_AFTER": "before-completion-receipt",
			}, "sync", "origin", "--recover-shallow")
			assertOperationalContains(t, crashed, "completion recording failed", "trust operations remain fail-closed")
			assertRefValue(t, acceptedActorRef("origin", fixture.actor.Actor), fixture.second.Commit)

			gitDir := runOperationalGit(t, fixture.clone, "rev-parse", "--absolute-git-dir")
			receipt := onlyOperationalStateFile(t, filepath.Join(gitDir, "hn", "replication", "transactions"))
			anchor := onlyOperationalStateFile(t, filepath.Join(gitDir, "hn", "replication", "anchors"))
			for _, directory := range []string{
				filepath.Join(gitDir, "hn"), filepath.Join(gitDir, "hn", "replication"),
				filepath.Dir(receipt), filepath.Dir(anchor),
			} {
				if info, err := os.Lstat(directory); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
					t.Fatalf("private transaction directory %s: info=%v err=%v", directory, info, err)
				}
			}
			for _, path := range []string{receipt, anchor} {
				if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
					t.Fatalf("private transaction file %s: info=%v err=%v", path, info, err)
				}
			}
			if filepath.Base(receipt) != filepath.Base(anchor) {
				t.Fatalf("receipt %s and pending anchor %s do not bind the same transaction", receipt, anchor)
			}
			anchorBytes, err := os.ReadFile(anchor)
			if err != nil {
				t.Fatal(err)
			}
			transactionID := strings.TrimSuffix(filepath.Base(anchor), ".json")
			assertOperationalContains(t, string(anchorBytes), `"id": "`+transactionID+`"`, fixture.first.Commit)
			if strings.Contains(string(anchorBytes), fixture.second.Commit) {
				t.Fatalf("pre-existing accepted head entered the pending anchor:\n%s", anchorBytes)
			}
			attack.mutate(t, receipt)

			beforeRefs := runOperationalGit(t, fixture.clone, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn")
			beforeHead := runOperationalGit(t, fixture.clone, "rev-parse", "HEAD")
			blockedCommands := [][]string{{"log"}}
			if attack.name == "missing receipt" {
				unknown := "sha256:" + strings.Repeat("d", 64)
				blockedCommands = append(blockedCommands,
					[]string{"policy", "show", "HEAD"},
					[]string{"proposal", "status", unknown},
					[]string{"review", unknown, "--approve"},
					[]string{"run", "request", unknown, "test"},
					[]string{"decide", unknown, "--accept"},
					[]string{"merge", unknown},
				)
			}
			for _, args := range blockedCommands {
				blocked := runOperationalCommandFailure(t, binary, fixture.clone, args...)
				assertOperationalContains(t, blocked, "replication acceptance pending", "state=invalid")
				if refs := runOperationalGit(t, fixture.clone, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn"); refs != beforeRefs {
					t.Fatalf("blocked %q changed refs:\nbefore=%s\nafter=%s", strings.Join(args, " "), beforeRefs, refs)
				}
				if head := runOperationalGit(t, fixture.clone, "rev-parse", "HEAD"); head != beforeHead {
					t.Fatalf("blocked %q changed HEAD from %s to %s", strings.Join(args, " "), beforeHead, head)
				}
			}

			retry := runOperationalCommand(t, binary, fixture.clone, "sync", "origin", "--recover-shallow")
			assertOperationalContains(t, retry, "promoted")
			log := runOperationalCommand(t, binary, fixture.clone, "log")
			assertOperationalContains(t, log, operationalShortID(fixture.first.ID), operationalShortID(fixture.second.ID))
			if entries, err := os.ReadDir(filepath.Dir(anchor)); err != nil || len(entries) != 0 {
				t.Fatalf("successful retry did not clear durable pending anchors: entries=%v err=%v", entries, err)
			}
		})
	}
}

func TestOperationalSelfHostingAlphaPendingAnchorPrecedesObjectCopy(t *testing.T) {
	binary := buildOperationalBinary(t)
	fixture := setupShallowRecoveryFixture(t, defaultReplicationBudgets())
	failure := runOperationalCommandFailureWithEnv(t, binary, fixture.clone, map[string]string{
		"HN_INTERNAL_TESTING":                 "1",
		"HN_TEST_REPLICATION_INTERRUPT_AFTER": "after-pending-anchor",
	}, "sync", "origin", "--recover-shallow")
	assertOperationalContains(t, failure, "interrupted after pending anchor before object copy")
	gitDir := runOperationalGit(t, fixture.clone, "rev-parse", "--absolute-git-dir")
	anchor := onlyOperationalStateFile(t, filepath.Join(gitDir, "hn", "replication", "anchors"))
	if entries, err := os.ReadDir(filepath.Join(gitDir, "hn", "replication", "transactions")); !errors.Is(err, os.ErrNotExist) && (err != nil || len(entries) != 0) {
		t.Fatalf("receipt became durable before the injected pre-copy boundary: entries=%v err=%v", entries, err)
	}
	if _, err := os.Stat(anchor); err != nil {
		t.Fatalf("pending anchor was not durable before object copy: %v", err)
	}
	runOperationalGitFailure(t, fixture.clone, "cat-file", "-e", fixture.first.Commit+"^{object}")
	blocked := runOperationalCommandFailure(t, binary, fixture.clone, "log")
	assertOperationalContains(t, blocked, "shallow dependency gap", fixture.first.ID)
	runOperationalCommand(t, binary, fixture.clone, "sync", "origin", "--recover-shallow")
	log := runOperationalCommand(t, binary, fixture.clone, "log")
	assertOperationalContains(t, log, operationalShortID(fixture.first.ID), operationalShortID(fixture.second.ID))
}

func TestOperationalSelfHostingAlphaTopLevelRoutes(t *testing.T) {
	binary := buildOperationalBinary(t)
	help := runOperationalCommand(t, binary, "", "help")
	for _, contract := range []string{
		"hn identity show|list|public|authorize|accept|rotate",
		"hn replication select|show",
		"hn sync [REMOTE] [--recover-shallow]",
		"full actor fingerprints and event IDs",
	} {
		if !strings.Contains(help, contract) {
			t.Fatalf("help omitted %q:\n%s", contract, help)
		}
	}
	repository := filepath.Join(t.TempDir(), "repo")
	runOperationalGit(t, "", "init", "-q", "-b", "main", repository)
	show := runOperationalCommand(t, binary, repository, "replication", "show", "origin")
	if !strings.Contains(show, "remote: origin") || !strings.Contains(show, "saved: false") {
		t.Fatalf("replication route output:\n%s", show)
	}
}

func TestOperationalSelfHostingAlpha(t *testing.T) {
	started := time.Now()
	binary := buildOperationalBinary(t)
	root := t.TempDir()
	author := filepath.Join(root, "author")
	reviewer := filepath.Join(root, "reviewer")
	verifier := filepath.Join(root, "verifier")
	remote := filepath.Join(root, "project.git")

	initOperationalRepository(t, author, "Author")
	runOperationalCommand(t, binary, author, "init", "--name", "Author device")
	authorIdentity := readOperationalIdentity(t, binary, author)
	baselinePolicyDigest := writeOperationalPolicy(t, author, authorIdentity.Actor, authorIdentity.Actor, true, "", false)
	writeOperationalPipeline(t, author)
	runOperationalGit(t, author, "add", ".hn")
	runOperationalGit(t, author, "commit", "-q", "-m", "baseline policy")
	baseline := runOperationalGit(t, author, "rev-parse", "HEAD")
	runOperationalGit(t, "", "init", "--bare", "-q", remote)
	runOperationalGit(t, author, "remote", "add", "origin", remote)
	runOperationalGit(t, author, "push", "-q", "-u", "origin", "main")
	runOperationalGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")

	runOperationalGit(t, "", "clone", "-q", remote, reviewer)
	configureOperationalGit(t, reviewer, "Reviewer")
	runOperationalCommand(t, binary, reviewer, "init", "--name", "Review device")
	reviewerIdentity := readOperationalIdentity(t, binary, reviewer)
	if authorIdentity.Actor == reviewerIdentity.Actor || authorIdentity.PublicKey == reviewerIdentity.PublicKey {
		t.Fatal("independently initialized clones reused actor or public-key material")
	}
	if _, err := os.Stat(filepath.Join(reviewer, ".git", "hn", "identity.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reviewer unexpectedly received a legacy copied identity: %v", err)
	}

	authorizationOutput := runOperationalCommand(t, binary, author, "identity", "authorize",
		"--relationship", "device", "--actor", reviewerIdentity.Actor, "--public-key", reviewerIdentity.PublicKey)
	authorization := parseParenthesizedFullID(t, authorizationOutput)
	runOperationalCommand(t, binary, author, "sync", "origin")
	selectOperationalReplication(t, binary, reviewer, []string{authorIdentity.Actor}, nil)
	runOperationalCommand(t, binary, reviewer, "sync", "origin")
	acceptanceOutput := runOperationalCommand(t, binary, reviewer, "identity", "accept", authorization)
	acceptance := parseParenthesizedFullID(t, acceptanceOutput)
	runOperationalCommand(t, binary, reviewer, "sync", "origin")
	selectOperationalReplication(t, binary, author, []string{authorIdentity.Actor, reviewerIdentity.Actor}, nil)
	runOperationalCommand(t, binary, author, "sync", "origin")
	for _, repository := range []string{author, reviewer} {
		log := runOperationalCommand(t, binary, repository, "log")
		if !strings.Contains(log, operationalShortID(authorization)) || !strings.Contains(log, operationalShortID(acceptance)) {
			t.Fatalf("clone did not display the same accepted device facts:\n%s", log)
		}
	}

	runOperationalGit(t, author, "switch", "-q", "-c", "policy-amendment")
	amendedPolicyDigest := writeOperationalPolicy(t, author, authorIdentity.Actor, reviewerIdentity.Actor, false, reviewerIdentity.Actor, true)
	draftCheck := runOperationalCommand(t, binary, author, "policy", "check", "--base", baseline, "--file", filepath.Join(author, ".hn", "policy.json"))
	assertOperationalContains(t, draftCheck,
		"Base policy digest: "+baselinePolicyDigest,
		"Proposed policy digest: "+amendedPolicyDigest,
		"The base policy governs this amendment candidate.",
		"trusted reviewers added: "+reviewerIdentity.Actor,
	)
	runOperationalGit(t, author, "add", ".hn/policy.json")
	runOperationalGit(t, author, "commit", "-q", "-m", "amend collaboration policy")
	amendmentHead := runOperationalGit(t, author, "rev-parse", "HEAD")
	runOperationalGit(t, author, "switch", "-q", "main")
	headCheck := runOperationalCommand(t, binary, author, "policy", "check", "--base", baseline, "--head", amendmentHead)
	assertOperationalContains(t, headCheck, "Base commit: "+baseline, "Proposed commit: "+amendmentHead, "The base policy governs this amendment candidate.")
	runOperationalCommand(t, binary, author, "proposal", "open", "--base", baseline, "--head", amendmentHead,
		"--body", "The old policy alone authorizes this amendment.", "Amend operational policy")
	amendment := latestOperationalEvent(t, author, operationalActorRef(authorIdentity.Actor))
	if amendment.Kind != "proposal.open" || amendment.Base != baseline || amendment.Head != amendmentHead {
		t.Fatalf("amendment binding mismatch: %#v", amendment)
	}
	runOperationalCommand(t, binary, author, "sync", "origin")
	selectOperationalReplication(t, binary, reviewer, []string{authorIdentity.Actor, reviewerIdentity.Actor}, []string{amendment.ID})
	runOperationalCommand(t, binary, reviewer, "sync", "origin")
	runOperationalCommand(t, binary, reviewer, "review", amendment.ID, "--approve", "--body", "Valid signed but not old-policy authority")
	untrustedAmendmentReview := latestOperationalEvent(t, reviewer, operationalActorRef(reviewerIdentity.Actor))
	runOperationalCommand(t, binary, reviewer, "sync", "origin")
	selectOperationalReplication(t, binary, author, []string{authorIdentity.Actor, reviewerIdentity.Actor}, []string{amendment.ID})
	runOperationalCommand(t, binary, author, "sync", "origin")
	status := runOperationalCommand(t, binary, author, "proposal", "status", amendment.ID)
	assertOperationalContains(t, status, "Policy:   "+operationalShortID(baselinePolicyDigest), "Reviews:  0/1 trusted approvals", "Status:   blocked")
	runOperationalCommand(t, binary, author, "review", amendment.ID, "--approve", "--body", "Authorized by the old policy")
	trustedAmendmentReview := latestOperationalEvent(t, author, operationalActorRef(authorIdentity.Actor))
	runOperationalCommand(t, binary, author, "decide", amendment.ID, "--accept", "--body", "Old policy requirements satisfied")
	amendmentDecision := latestOperationalEvent(t, author, operationalActorRef(authorIdentity.Actor))
	if amendmentDecision.Policy != baselinePolicyDigest || !containsOperationalID(amendmentDecision.Evidence, trustedAmendmentReview.ID) || containsOperationalID(amendmentDecision.Evidence, untrustedAmendmentReview.ID) {
		t.Fatalf("amendment was not decided exclusively under old-policy evidence: %#v", amendmentDecision)
	}
	runOperationalCommand(t, binary, author, "merge", amendment.ID)
	amendmentMerge := latestOperationalEvent(t, author, operationalActorRef(authorIdentity.Actor))
	runOperationalGit(t, author, "push", "-q", "origin", "main")
	runOperationalCommand(t, binary, author, "sync", "origin")
	if amendmentMerge.Subject != amendment.ID || !containsOperationalID(amendmentMerge.Evidence, amendmentDecision.ID) {
		t.Fatalf("amendment merge fact mismatch: %#v", amendmentMerge)
	}

	amendedBase := runOperationalGit(t, author, "rev-parse", "HEAD")
	shown := runOperationalCommand(t, binary, author, "policy", "show", amendedBase)
	assertOperationalContains(t, shown, "Policy digest: "+amendedPolicyDigest, reviewerIdentity.Actor)
	runOperationalGit(t, author, "switch", "-q", "-c", "role-distinct-change")
	if err := os.WriteFile(filepath.Join(author, "operational-alpha.txt"), []byte("role-distinct self-hosting\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runOperationalGit(t, author, "add", "operational-alpha.txt")
	runOperationalGit(t, author, "commit", "-q", "-m", "exercise operational alpha")
	laterHead := runOperationalGit(t, author, "rev-parse", "HEAD")
	runOperationalGit(t, author, "switch", "-q", "main")
	runOperationalCommand(t, binary, author, "proposal", "open", "--base", amendedBase, "--head", laterHead,
		"--body", "Requires a distinct reviewer and runner.", "Exercise operational self-hosting")
	later := latestOperationalEvent(t, author, operationalActorRef(authorIdentity.Actor))
	runOperationalCommand(t, binary, author, "review", later.ID, "--approve", "--body", "Author evidence must not qualify")
	authorReview := latestOperationalEvent(t, author, operationalActorRef(authorIdentity.Actor))
	runOperationalCommand(t, binary, author, "run", "request", later.ID, "test")
	request := latestOperationalEvent(t, author, operationalActorRef(authorIdentity.Actor))
	blocked := runOperationalCommand(t, binary, author, "proposal", "status", later.ID)
	assertOperationalContains(t, blocked, "Reviews:  0/1 trusted approvals", "Pipeline: test 0/1 trusted passes", "Status:   blocked")
	runOperationalCommand(t, binary, author, "sync", "origin")

	selectOperationalReplication(t, binary, reviewer, []string{authorIdentity.Actor, reviewerIdentity.Actor}, []string{amendment.ID, later.ID})
	runOperationalCommand(t, binary, reviewer, "sync", "origin")
	runOperationalCommand(t, binary, reviewer, "run", "execute", request.ID, "--backend", "host", "--allow-unsafe-host-execution")
	result := latestOperationalEvent(t, reviewer, operationalActorRef(reviewerIdentity.Actor))
	runOperationalCommand(t, binary, reviewer, "review", later.ID, "--approve", "--body", "Distinct review evidence")
	reviewerReview := latestOperationalEvent(t, reviewer, operationalActorRef(reviewerIdentity.Actor))
	runOperationalCommand(t, binary, reviewer, "sync", "origin")
	assertOperationalBudgetRejections(t, binary, root, remote, reviewerIdentity.Actor)

	selectOperationalReplication(t, binary, author, []string{authorIdentity.Actor, reviewerIdentity.Actor}, []string{amendment.ID, later.ID})
	runOperationalCommand(t, binary, author, "sync", "origin")
	ready := runOperationalCommand(t, binary, author, "proposal", "status", later.ID)
	assertOperationalContains(t, ready, "Reviews:  1/1 trusted approvals", "Pipeline: test 1/1 trusted passes", "Status:   ready")
	runOperationalCommand(t, binary, author, "decide", later.ID, "--accept", "--body", "Role-distinct evidence satisfied")
	laterDecision := latestOperationalEvent(t, author, operationalActorRef(authorIdentity.Actor))
	runOperationalCommand(t, binary, author, "merge", later.ID)
	laterMerge := latestOperationalEvent(t, author, operationalActorRef(authorIdentity.Actor))
	mergedCommit := runOperationalGit(t, author, "rev-parse", "HEAD")
	runOperationalGit(t, author, "push", "-q", "origin", "main")
	runOperationalCommand(t, binary, author, "sync", "origin")

	if request.Subject != later.ID || request.Commit != laterHead || result.Subject != request.ID || result.Commit != laterHead || result.Definition != request.Definition || reviewerReview.Subject != later.ID || authorReview.Subject != later.ID {
		t.Fatalf("request/result/review exact binding mismatch: request=%#v result=%#v reviewer=%#v author=%#v", request, result, reviewerReview, authorReview)
	}
	if laterDecision.Policy != amendedPolicyDigest || !containsOperationalID(laterDecision.Evidence, reviewerReview.ID) || !containsOperationalID(laterDecision.Evidence, result.ID) || containsOperationalID(laterDecision.Evidence, authorReview.ID) {
		t.Fatalf("role-distinct decision evidence mismatch: %#v", laterDecision)
	}
	if laterMerge.Subject != later.ID || laterMerge.Head != laterHead || laterMerge.Commit != mergedCommit || !containsOperationalID(laterMerge.Evidence, laterDecision.ID) {
		t.Fatalf("merge fact exact binding mismatch: %#v", laterMerge)
	}
	interruptedRotation := runOperationalCommandFailureWithEnv(t, binary, author, map[string]string{
		"HN_INTERNAL_TESTING":              "1",
		"HN_TEST_ROTATION_INTERRUPT_AFTER": "before-active-switch",
	}, "identity", "rotate", "--name", "Rotated device")
	assertOperationalContains(t, interruptedRotation, "injected rotation interruption after before-active-switch")
	if active := readOperationalIdentity(t, binary, author); active.Actor != authorIdentity.Actor {
		t.Fatalf("interrupted rotation switched active signer from %s to %s", authorIdentity.Actor, active.Actor)
	}
	actorsAfterInterruption := strings.Fields(runOperationalGit(t, author, "for-each-ref", "--format=%(refname:strip=3)", "refs/hn/actors"))
	if len(actorsAfterInterruption) != 2 {
		t.Fatalf("interrupted rotation did not durably retain exactly two actor chains: %v", actorsAfterInterruption)
	}
	rotationOutput := runOperationalCommand(t, binary, author, "identity", "rotate", "--name", "Rotated device")
	predecessorActor, successorActor := parseOperationalRotation(t, rotationOutput)
	if predecessorActor != authorIdentity.Actor || successorActor == predecessorActor || successorActor == reviewerIdentity.Actor {
		t.Fatalf("rotation actors are not distinct: predecessor=%s successor=%s", predecessorActor, successorActor)
	}
	rotationAuthorization := latestOperationalEvent(t, author, operationalActorRef(predecessorActor))
	rotationAcceptance := latestOperationalEvent(t, author, operationalActorRef(successorActor))
	if rotationAuthorization.Kind != "identity.authorize" || rotationAcceptance.Kind != "identity.accept" || rotationAcceptance.Subject != rotationAuthorization.ID || rotationAcceptance.Sequence != 1 {
		t.Fatalf("rotation exact continuity mismatch: authorize=%#v accept=%#v", rotationAuthorization, rotationAcceptance)
	}
	identityList := runOperationalCommand(t, binary, author, "identity", "list")
	assertOperationalContains(t, identityList, predecessorActor+"  retired", successorActor+"  active")
	runOperationalCommand(t, binary, author, "sync", "origin")

	runOperationalGit(t, "", "clone", "-q", "--depth", "1", "file://"+remote, verifier)
	configureOperationalGit(t, verifier, "Verifier")
	selectOperationalReplication(t, binary, verifier, []string{authorIdentity.Actor, reviewerIdentity.Actor, successorActor}, []string{amendment.ID, later.ID})
	runOperationalCommand(t, binary, verifier, "sync", "origin")
	if _, err := os.Stat(filepath.Join(verifier, ".git", "hn", "identities")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("identity-free verifier has private identity records: %v", err)
	}
	if refs := runOperationalGit(t, verifier, "for-each-ref", "--format=%(refname)", "refs/hn/actors"); refs != "" {
		t.Fatalf("identity-free sync published a local actor ref: %s", refs)
	}
	verifyLog := runOperationalCommand(t, binary, verifier, "log")
	for _, event := range []operationalEvent{amendment, trustedAmendmentReview, amendmentDecision, amendmentMerge, later, request, result, reviewerReview, laterDecision, laterMerge, rotationAuthorization, rotationAcceptance} {
		if !strings.Contains(verifyLog, operationalShortID(event.ID)) {
			t.Fatalf("identity-free reconstruction omitted %s (%s):\n%s", event.ID, event.Kind, verifyLog)
		}
	}
	verifyStatus := runOperationalCommand(t, binary, verifier, "proposal", "status", later.ID)
	assertOperationalContains(t, verifyStatus, "Status:   merged", "Policy:   "+operationalShortID(amendedPolicyDigest))
	if got := runOperationalGit(t, verifier, "rev-parse", "origin/main"); got != mergedCommit {
		t.Fatalf("verifier branch=%s want merged commit %s", got, mergedCommit)
	}
	if elapsed := time.Since(started); elapsed >= 120*time.Second {
		t.Fatalf("operational scenario took %s, want <120s", elapsed)
	}
}

func TestOperationalSelfHostingAlphaIdentityAmbiguityRemote(t *testing.T) {
	binary := buildOperationalBinary(t)
	root := t.TempDir()
	firstRepo := filepath.Join(root, "first")
	secondRepo := filepath.Join(root, "second")
	verifier := filepath.Join(root, "verifier")
	remote := filepath.Join(root, "identity-cycle.git")

	initOperationalRepository(t, firstRepo, "First")
	runOperationalCommand(t, binary, firstRepo, "init", "--name", "First signer")
	first := readOperationalIdentity(t, binary, firstRepo)
	writeOperationalPolicy(t, firstRepo, first.Actor, first.Actor, true, "", false)
	runOperationalGit(t, firstRepo, "add", ".hn/policy.json")
	runOperationalGit(t, firstRepo, "commit", "-q", "-m", "identity-cycle policy")
	runOperationalGit(t, "", "init", "--bare", "-q", remote)
	runOperationalGit(t, firstRepo, "remote", "add", "origin", remote)
	runOperationalGit(t, firstRepo, "push", "-q", "-u", "origin", "main")
	runOperationalGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")

	runOperationalGit(t, "", "clone", "-q", remote, secondRepo)
	configureOperationalGit(t, secondRepo, "Second")
	runOperationalCommand(t, binary, secondRepo, "init", "--name", "Second signer")
	second := readOperationalIdentity(t, binary, secondRepo)
	if first.Actor == second.Actor {
		t.Fatal("independent cycle actors are identical")
	}

	firstAuthorizationOutput := runOperationalCommand(t, binary, firstRepo, "identity", "authorize",
		"--relationship", "successor", "--actor", second.Actor, "--public-key", second.PublicKey)
	firstAuthorization := parseParenthesizedFullID(t, firstAuthorizationOutput)
	runOperationalCommand(t, binary, firstRepo, "sync", "origin")
	selectOperationalReplication(t, binary, secondRepo, []string{first.Actor}, nil)
	runOperationalCommand(t, binary, secondRepo, "sync", "origin")
	firstAcceptanceOutput := runOperationalCommand(t, binary, secondRepo, "identity", "accept", firstAuthorization)
	firstAcceptance := parseParenthesizedFullID(t, firstAcceptanceOutput)
	secondAuthorizationOutput := runOperationalCommand(t, binary, secondRepo, "identity", "authorize",
		"--relationship", "successor", "--actor", first.Actor, "--public-key", first.PublicKey)
	secondAuthorization := parseParenthesizedFullID(t, secondAuthorizationOutput)
	runOperationalCommand(t, binary, secondRepo, "sync", "origin")

	selectOperationalReplication(t, binary, firstRepo, []string{first.Actor, second.Actor}, nil)
	runOperationalCommand(t, binary, firstRepo, "sync", "origin")
	secondAcceptanceOutput := runOperationalCommand(t, binary, firstRepo, "identity", "accept", secondAuthorization)
	secondAcceptance := parseParenthesizedFullID(t, secondAcceptanceOutput)
	runOperationalCommand(t, binary, firstRepo, "sync", "origin")

	runOperationalGit(t, "", "clone", "-q", remote, verifier)
	configureOperationalGit(t, verifier, "Identity Verifier")
	selectOperationalReplication(t, binary, verifier, []string{first.Actor, second.Actor}, nil)
	runOperationalCommand(t, binary, verifier, "sync", "origin")
	projection := runOperationalCommand(t, binary, verifier, "identity", "list")
	assertOperationalContains(t, projection,
		"Public actor: "+first.Actor+" state=ambiguous",
		"Public actor: "+second.Actor+" state=ambiguous",
		"Identity conflict: successor-cycle",
		firstAuthorization, firstAcceptance, secondAuthorization, secondAcceptance,
		"authority=not-inferred",
	)
	policy := runOperationalCommand(t, binary, verifier, "policy", "show", "HEAD")
	assertOperationalContains(t, policy, "Maintainers (required accepts 1):", first.Actor)
	if strings.Contains(policy, second.Actor) {
		t.Fatalf("cyclic successor was incorrectly inferred as policy authority:\n%s", policy)
	}
}

func TestOperationalSelfHostingAlphaHostileSelectedReplication(t *testing.T) {
	binary := buildOperationalBinary(t)
	root := t.TempDir()
	goodRepo := filepath.Join(root, "good")
	largeRepo := filepath.Join(root, "large")
	unselectedRepo := filepath.Join(root, "unselected")
	receiver := filepath.Join(root, "receiver")
	dependencyReceiver := filepath.Join(root, "dependency-receiver")
	remote := filepath.Join(root, "hostile.git")

	initOperationalRepository(t, goodRepo, "Good")
	runOperationalCommand(t, binary, goodRepo, "init", "--name", "Good actor")
	goodIdentity := readOperationalIdentity(t, binary, goodRepo)
	writeOperationalPolicy(t, goodRepo, goodIdentity.Actor, goodIdentity.Actor, true, "", false)
	runOperationalGit(t, goodRepo, "add", ".hn/policy.json")
	runOperationalGit(t, goodRepo, "commit", "-q", "-m", "replication base")
	runOperationalGit(t, "", "init", "--bare", "-q", remote)
	runOperationalGit(t, goodRepo, "remote", "add", "origin", remote)
	runOperationalGit(t, goodRepo, "push", "-q", "-u", "origin", "main")
	runOperationalGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	runOperationalCommand(t, binary, goodRepo, "issue", "open", "one valid event")
	goodEvent := latestOperationalEvent(t, goodRepo, operationalActorRef(goodIdentity.Actor))
	runOperationalCommand(t, binary, goodRepo, "sync", "origin")

	runOperationalGit(t, "", "clone", "-q", remote, largeRepo)
	configureOperationalGit(t, largeRepo, "Large")
	runOperationalCommand(t, binary, largeRepo, "init", "--name", "Over-budget actor")
	largeIdentity := readOperationalIdentity(t, binary, largeRepo)
	runOperationalCommand(t, binary, largeRepo, "issue", "open", "event one")
	runOperationalCommand(t, binary, largeRepo, "issue", "open", "event two")
	runOperationalCommand(t, binary, largeRepo, "sync", "origin")

	runOperationalGit(t, "", "clone", "-q", remote, unselectedRepo)
	configureOperationalGit(t, unselectedRepo, "Unselected")
	runOperationalCommand(t, binary, unselectedRepo, "init", "--name", "Unselected actor")
	unselectedIdentity := readOperationalIdentity(t, binary, unselectedRepo)
	runOperationalCommand(t, binary, unselectedRepo, "issue", "open", "must remain unselected")
	runOperationalCommand(t, binary, unselectedRepo, "sync", "origin")

	invalidActor := strings.Repeat("f", 64)
	mismatchedActor := strings.Repeat("e", 64)
	mainCommit := runOperationalGit(t, goodRepo, "rev-parse", "main")
	runOperationalGit(t, remote, "update-ref", operationalActorRef(invalidActor), mainCommit)
	goodActorHead := runOperationalGit(t, goodRepo, "rev-parse", operationalActorRef(goodIdentity.Actor))
	runOperationalGit(t, remote, "update-ref", operationalActorRef(mismatchedActor), goodActorHead)
	runOperationalGit(t, "", "clone", "-q", remote, receiver)
	configureOperationalGit(t, receiver, "Receiver")
	selection := []string{
		"replication", "select", "origin",
		"--actor", goodIdentity.Actor, "--actor", largeIdentity.Actor, "--actor", invalidActor, "--actor", mismatchedActor,
		"--max-events", "1", "--max-objects", "100000", "--max-object-bytes", "16777216",
		"--max-attachment-bytes", "1048576", "--max-total-bytes", "268435456",
	}
	selectionOutput := runOperationalCommand(t, binary, receiver, selection...)
	assertOperationalContains(t, selectionOutput,
		"max-events: 1", "max-objects: 100000", "max-object-bytes: 16777216",
		"max-attachment-bytes: 1048576", "max-total-bytes: 268435456",
	)
	syncFailure := runOperationalCommandFailure(t, binary, receiver, "sync", "origin")
	assertOperationalContains(t, syncFailure,
		"Replication actor "+goodIdentity.Actor+": promoted",
		"Replication actor "+largeIdentity.Actor+": failed (over-budget)",
		"Replication actor "+invalidActor+": failed (structurally-invalid)",
		"Replication actor "+mismatchedActor+": failed (structurally-invalid)",
		"independently valid selections were promoted",
	)
	assertOperationalRef(t, receiver, "refs/hn/remotes/origin/actors/"+goodIdentity.Actor, true)
	for _, actor := range []string{largeIdentity.Actor, invalidActor, mismatchedActor, unselectedIdentity.Actor} {
		assertOperationalRef(t, receiver, "refs/hn/remotes/origin/actors/"+actor, false)
	}
	selectionDirectory := filepath.Join(receiver, ".git", "hn", "replication", "selections")
	selectionFiles, err := os.ReadDir(selectionDirectory)
	if err != nil || len(selectionFiles) != 1 || selectionFiles[0].IsDir() {
		t.Fatalf("saved selection is not exactly one local private file: files=%v err=%v", selectionFiles, err)
	}
	if status := runOperationalGit(t, receiver, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("local replication selection appeared in tracked/untracked worktree state: %s", status)
	}
	if refs := runOperationalGit(t, remote, "for-each-ref", "--format=%(refname)", "refs/hn/replication"); refs != "" {
		t.Fatalf("local replication selection was published to the remote: %s", refs)
	}
	if log := runOperationalCommand(t, binary, receiver, "log"); !strings.Contains(log, operationalShortID(goodEvent.ID)) {
		t.Fatalf("independently valid event unusable after hostile sync:\n%s", log)
	}

	runOperationalGit(t, goodRepo, "switch", "-q", "-c", "missing-dependency")
	if err := os.WriteFile(filepath.Join(goodRepo, "candidate.txt"), []byte("candidate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runOperationalGit(t, goodRepo, "add", "candidate.txt")
	runOperationalGit(t, goodRepo, "commit", "-q", "-m", "candidate")
	head := runOperationalGit(t, goodRepo, "rev-parse", "HEAD")
	runOperationalGit(t, goodRepo, "switch", "-q", "main")
	runOperationalCommand(t, binary, goodRepo, "proposal", "open", "--base", "main", "--head", head, "dependency guidance")
	proposal := latestOperationalEvent(t, goodRepo, operationalActorRef(goodIdentity.Actor))
	runOperationalGit(t, goodRepo, "push", "-q", "origin", operationalActorRef(goodIdentity.Actor)+":"+operationalActorRef(goodIdentity.Actor))
	proposalRef := "refs/hn/proposals/" + strings.TrimPrefix(proposal.ID, "sha256:")
	runOperationalGit(t, goodRepo, "push", "-q", "origin", proposalRef+":"+proposalRef)

	runOperationalGit(t, "", "clone", "-q", remote, dependencyReceiver)
	configureOperationalGit(t, dependencyReceiver, "Dependency Receiver")
	selectOperationalReplication(t, binary, dependencyReceiver, nil, []string{proposal.ID})
	missingFailure := runOperationalCommandFailure(t, binary, dependencyReceiver, "sync", "origin")
	assertOperationalContains(t, missingFailure, proposal.ID, "select the full actor history that contains candidate "+proposal.ID)
	assertOperationalRef(t, dependencyReceiver, "refs/hn/remotes/origin/proposals/"+strings.TrimPrefix(proposal.ID, "sha256:"), false)
}

func TestOperationalSelfHostingAlphaShallowRecoveryCLI(t *testing.T) {
	binary := buildOperationalBinary(t)
	fixture := setupShallowRecoveryFixture(t, defaultReplicationBudgets())
	beforeRefs := runOperationalGit(t, fixture.clone, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/remotes")
	blocked := runOperationalCommandFailure(t, binary, fixture.clone, "log")
	assertOperationalContains(t, blocked, fixture.first.ID, "hn sync origin --recover-shallow")
	if after := runOperationalGit(t, fixture.clone, "for-each-ref", "--format=%(refname) %(objectname)", "refs/hn/remotes"); after != beforeRefs {
		t.Fatalf("blocked shallow command changed accepted refs:\nbefore=%s\nafter=%s", beforeRefs, after)
	}
	runOperationalCommand(t, binary, fixture.clone, "sync", "origin", "--recover-shallow")
	log := runOperationalCommand(t, binary, fixture.clone, "log")
	assertOperationalContains(t, log, operationalShortID(fixture.first.ID), operationalShortID(fixture.second.ID))
	if state := runOperationalGit(t, fixture.clone, "rev-parse", "--is-shallow-repository"); state != "true" {
		t.Fatalf("bounded recovery globally unshallowed repository: %s", state)
	}
	assertOperationalRef(t, fixture.clone, "refs/hn/remotes/origin/actors/"+fixture.unselected.Actor, false)

	unselectedAcceptedRef := "refs/hn/remotes/origin/actors/" + fixture.unselected.Actor
	runOperationalGit(t, fixture.clone, "fetch", "-q", "--depth", "1", "origin",
		operationalActorRef(fixture.unselected.Actor)+":"+unselectedAcceptedRef)
	unselectedHead := runOperationalGit(t, fixture.clone, "rev-parse", unselectedAcceptedRef)
	unselectedBlocked := runOperationalCommandFailure(t, binary, fixture.clone, "log")
	assertOperationalContains(t, unselectedBlocked, fixture.unselectedFirst.ID,
		"hn replication select origin --actor "+fixture.unselected.Actor)
	recoveryBlocked := runOperationalCommandFailure(t, binary, fixture.clone, "sync", "origin", "--recover-shallow")
	assertOperationalContains(t, recoveryBlocked, fixture.unselected.Actor, "is not in the saved exact selection",
		"hn replication select origin --actor "+fixture.unselected.Actor)
	selection := runOperationalCommand(t, binary, fixture.clone, "replication", "show", "origin")
	assertOperationalContains(t, selection, "actor: "+fixture.actor.Actor)
	if strings.Contains(selection, fixture.unselected.Actor) {
		t.Fatalf("unselected shallow supplier entered the saved exact selection:\n%s", selection)
	}
	if got := runOperationalGit(t, fixture.clone, "rev-parse", unselectedAcceptedRef); got != unselectedHead {
		t.Fatalf("blocked unselected recovery changed accepted ref from %s to %s", unselectedHead, got)
	}
}

func TestOperationalSelfHostingAlphaAcceptanceLedgerFailsClosed(t *testing.T) {
	t.Run("missing pending set", func(t *testing.T) {
		gitDir := setupOperationalLedgerRepository(t)
		record := map[string]any{
			"version":  replicationTransactionRecordVersion,
			"id":       "txn-missing-state",
			"remote":   "origin",
			"state":    "validated",
			"outcomes": []any{},
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		writeOperationalTransactionFixture(t, gitDir, "txn-missing-state", encoded)
		if _, err := collectEvents(); err == nil || !strings.Contains(err.Error(), "replication acceptance pending") {
			t.Fatalf("missing pending set did not fail closed: %v", err)
		}
	})

	t.Run("corrupt receipt", func(t *testing.T) {
		gitDir := setupOperationalLedgerRepository(t)
		writeOperationalTransactionFixture(t, gitDir, "txn-corrupt-state", []byte("{not-json\n"))
		if _, err := collectEvents(); err == nil || !strings.Contains(err.Error(), "replication acceptance pending") {
			t.Fatalf("corrupt receipt did not fail closed: %v", err)
		}
	})
}

func TestOperationalSelfHostingAlphaAcceptanceStateRejectsHostileFilesystem(t *testing.T) {
	binary := buildOperationalBinary(t)
	tests := []struct {
		name   string
		mutate func(*testing.T, string, string, string)
	}{
		{name: "transaction directory symlink", mutate: func(t *testing.T, _, transactions, _ string) {
			target := t.TempDir()
			if err := os.RemoveAll(transactions); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, transactions); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "transaction path is file", mutate: func(t *testing.T, _, transactions, _ string) {
			if err := os.RemoveAll(transactions); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(transactions, []byte("not-directory\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unsafe transaction directory mode", mutate: func(t *testing.T, _, transactions, _ string) {
			if err := os.Chmod(transactions, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unsafe replication parent mode", mutate: func(t *testing.T, _, transactions, _ string) {
			if err := os.Chmod(filepath.Dir(transactions), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unsafe hn parent mode", mutate: func(t *testing.T, _, transactions, _ string) {
			if err := os.Chmod(filepath.Dir(filepath.Dir(transactions)), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "anchor directory symlink", mutate: func(t *testing.T, _, _, anchors string) {
			target := t.TempDir()
			if err := os.RemoveAll(anchors); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, anchors); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "anchor path is file", mutate: func(t *testing.T, _, _, anchors string) {
			if err := os.RemoveAll(anchors); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(anchors, []byte("not-directory\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unsafe anchor directory mode", mutate: func(t *testing.T, _, _, anchors string) {
			if err := os.Chmod(anchors, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "anchor file symlink", mutate: func(t *testing.T, _, _, anchors string) {
			path := filepath.Join(anchors, "txn-hostile.json")
			target := filepath.Join(t.TempDir(), "anchor.json")
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unsafe anchor file mode", mutate: func(t *testing.T, _, _, anchors string) {
			if err := os.Chmod(filepath.Join(anchors, "txn-hostile.json"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "oversized anchor", mutate: func(t *testing.T, _, _, anchors string) {
			if err := os.WriteFile(filepath.Join(anchors, "txn-hostile.json"), make([]byte, (16<<20)+1), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "anchor unknown JSON", mutate: func(t *testing.T, _, _, anchors string) {
			path := filepath.Join(anchors, "txn-hostile.json")
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			contents = bytes.Replace(contents, []byte("\n}"), []byte(",\n  \"unknown\": true\n}"), 1)
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "oversized receipt", mutate: func(t *testing.T, receipt, _, _ string) {
			if err := os.WriteFile(receipt, make([]byte, (16<<20)+1), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "receipt unknown JSON", mutate: func(t *testing.T, receipt, _, _ string) {
			contents, err := os.ReadFile(receipt)
			if err != nil {
				t.Fatal(err)
			}
			contents = bytes.Replace(contents, []byte("\n}"), []byte(",\n  \"unknown\": true\n}"), 1)
			if err := os.WriteFile(receipt, contents, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "receipt trailing JSON", mutate: func(t *testing.T, receipt, _, _ string) {
			file, err := os.OpenFile(receipt, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.WriteString("{}\n"); err != nil {
				_ = file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "receipt missing v2 fields", mutate: func(t *testing.T, receipt, _, _ string) {
			if err := os.WriteFile(receipt, []byte("{\"version\":2,\"id\":\"txn-hostile\",\"remote\":\"origin\",\"state\":\"validated\",\"outcomes\":[]}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gitDir := setupOperationalLedgerRepository(t)
			repository := filepath.Dir(gitDir)
			pending := strings.Repeat("1", 40)
			recordOperationalTransactionFixture(t, gitDir, "txn-hostile", "validated", []string{pending}, nil)
			transactions := filepath.Join(gitDir, "hn", "replication", "transactions")
			anchors := filepath.Join(gitDir, "hn", "replication", "anchors")
			receipt := filepath.Join(transactions, "txn-hostile.json")
			test.mutate(t, receipt, transactions, anchors)
			blocked := runOperationalCommandFailure(t, binary, repository, "log")
			assertOperationalContains(t, blocked, "replication acceptance pending", "state=invalid")
		})
	}
}

func TestOperationalSelfHostingAlphaAcceptanceLedgerUnion(t *testing.T) {
	gitDir := setupOperationalLedgerRepository(t)
	preexisting := mustGitText(t, "rev-parse", "HEAD")
	absent, err := replicationObjectsAbsentFromMain(gitDir, []string{preexisting})
	if err != nil || len(absent) != 0 {
		t.Fatalf("pre-existing accepted object was marked pending: ids=%v err=%v", absent, err)
	}

	firstOnly := strings.Repeat("1", 40)
	shared := strings.Repeat("2", 40)
	secondOnly := strings.Repeat("3", 40)
	recordOperationalTransactionFixture(t, gitDir, "txn-union-a", "validated", []string{firstOnly, shared}, nil)
	recordOperationalTransactionFixture(t, gitDir, "txn-union-b", "validated", []string{secondOnly, shared}, nil)
	recordOperationalTransactionFixture(t, gitDir, "txn-union-complete", "complete", nil, []string{shared})

	denied, _, err := loadReplicationAcceptanceState(gitDir)
	if err != nil {
		t.Fatal(err)
	}
	if !denied[firstOnly] || !denied[secondOnly] || denied[shared] || denied[preexisting] {
		t.Fatalf("transaction union mismatch: denied=%v", denied)
	}
	if err := os.Remove(filepath.Join(replicationTransactionsPath(gitDir), "txn-union-a.json")); err != nil {
		t.Fatal(err)
	}
	if _, _, err = loadReplicationAcceptanceState(gitDir); err == nil {
		t.Fatal("deleting one pending receipt was interpreted as acceptance")
	}
	recordOperationalTransactionFixture(t, gitDir, "txn-union-a", "validated", []string{firstOnly, shared}, nil)
	if err := os.Remove(filepath.Join(gitDir, "hn", "replication", "anchors", "txn-union-b.json")); err != nil {
		t.Fatal(err)
	}
	if _, _, err = loadReplicationAcceptanceState(gitDir); err == nil {
		t.Fatal("deleting one pending anchor was interpreted as acceptance")
	}
}

type operationalIdentity struct {
	Actor     string
	PublicKey string
}

type operationalEvent struct {
	ID         string
	Kind       string   `json:"kind"`
	Actor      string   `json:"actor"`
	Sequence   uint64   `json:"sequence"`
	Previous   string   `json:"previous"`
	Subject    string   `json:"subject"`
	Base       string   `json:"base"`
	Head       string   `json:"head"`
	Policy     string   `json:"policy"`
	Pipeline   string   `json:"pipeline"`
	Definition string   `json:"definition"`
	Commit     string   `json:"commit"`
	Evidence   []string `json:"evidence"`
}

func initOperationalRepository(t *testing.T, repository, name string) {
	t.Helper()
	runOperationalGit(t, "", "init", "-q", "-b", "main", repository)
	configureOperationalGit(t, repository, name)
}

func configureOperationalGit(t *testing.T, repository, name string) {
	t.Helper()
	runOperationalGit(t, repository, "config", "user.name", name)
	runOperationalGit(t, repository, "config", "user.email", strings.ToLower(name)+"@hn.invalid")
	runOperationalGit(t, repository, "config", "credential.helper", "")
}

func readOperationalIdentity(t *testing.T, binary, repository string) operationalIdentity {
	t.Helper()
	output := runOperationalCommand(t, binary, repository, "identity", "public")
	identity := operationalIdentity{
		Actor:     operationalLabel(t, output, "Actor:"),
		PublicKey: operationalLabel(t, output, "Public key:"),
	}
	if len(identity.Actor) != 64 || identity.PublicKey == "" {
		t.Fatalf("invalid public identity output:\n%s", output)
	}
	return identity
}

func writeOperationalPolicy(t *testing.T, repository, maintainer, reviewer string, allowAuthor bool, runner string, requirePipeline bool) string {
	t.Helper()
	pipelines := map[string]any{}
	if requirePipeline {
		pipelines["test"] = map[string]any{
			"requiredResults": 1,
			"trustedRunners":  []string{runner},
		}
	}
	policy := map[string]any{
		"version":     "hn.policy/0",
		"maintainers": []string{maintainer},
		"proposals": map[string]any{
			"requiredApprovals":   1,
			"requiredAccepts":     1,
			"trustedReviewers":    []string{reviewer},
			"allowAuthorApproval": allowAuthor,
		},
		"pipelines": pipelines,
	}
	encoded, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	directory := filepath.Join(repository, ".hn")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "policy.json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	return operationalDigest(encoded)
}

func writeOperationalPipeline(t *testing.T, repository string) {
	t.Helper()
	pipeline := map[string]any{
		"version": "hn.pipeline/0",
		"steps": []any{map[string]any{
			"name": "operational-proof", "command": "sh",
			"args": []string{"-c", "printf 'operational-ok\\n'"}, "timeoutSeconds": 5,
		}},
	}
	encoded, err := json.MarshalIndent(pipeline, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(repository, ".hn", "pipelines")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "test.json"), append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func selectOperationalReplication(t *testing.T, binary, repository string, actors, proposals []string) {
	t.Helper()
	args := []string{"replication", "select", "origin"}
	for _, actor := range actors {
		args = append(args, "--actor", actor)
	}
	for _, proposal := range proposals {
		args = append(args, "--proposal", proposal)
	}
	args = append(args,
		"--max-events", "10000", "--max-objects", "100000",
		"--max-object-bytes", "16777216", "--max-attachment-bytes", "1048576",
		"--max-total-bytes", "268435456",
	)
	output := runOperationalCommand(t, binary, repository, args...)
	assertOperationalContains(t, output,
		"max-events: 10000", "max-objects: 100000", "max-object-bytes: 16777216",
		"max-attachment-bytes: 1048576", "max-total-bytes: 268435456",
	)
}

func assertOperationalBudgetRejections(t *testing.T, binary, root, remote, actor string) {
	t.Helper()
	dimensions := []string{
		"max-events",
		"max-objects",
		"max-object-bytes",
		"max-attachment-bytes",
		"max-total-bytes",
	}
	for _, dimension := range dimensions {
		dimension := dimension
		t.Run(dimension, func(t *testing.T) {
			repository := filepath.Join(root, "budget-"+dimension)
			runOperationalGit(t, "", "clone", "-q", remote, repository)
			configureOperationalGit(t, repository, "Budget "+dimension)
			limits := map[string]string{
				"max-events":           "10000",
				"max-objects":          "100000",
				"max-object-bytes":     "16777216",
				"max-attachment-bytes": "1048576",
				"max-total-bytes":      "268435456",
			}
			limits[dimension] = "1"
			args := []string{"replication", "select", "origin", "--actor", actor}
			for _, name := range dimensions {
				args = append(args, "--"+name, limits[name])
			}
			runOperationalCommand(t, binary, repository, args...)
			failure := runOperationalCommandFailure(t, binary, repository, "sync", "origin")
			assertOperationalContains(t, failure,
				"Replication actor "+actor+": failed (over-budget)",
				"exceeded "+dimension+" budget",
			)
			assertOperationalRef(t, repository, "refs/hn/remotes/origin/actors/"+actor, false)
		})
	}
}

func latestOperationalEvent(t *testing.T, repository, ref string) operationalEvent {
	t.Helper()
	commit := runOperationalGit(t, repository, "rev-parse", ref)
	payload := runOperationalGitBytes(t, repository, "show", commit+":event.json")
	var event operationalEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("decode public event at %s: %v\n%s", commit, err, payload)
	}
	event.ID = operationalDigest(payload)
	return event
}

func operationalActorRef(actor string) string { return "refs/hn/actors/" + actor }

func operationalDigest(payload []byte) string {
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func operationalShortID(id string) string {
	trimmed := strings.TrimPrefix(id, "sha256:")
	if len(trimmed) > 12 {
		return trimmed[:12]
	}
	return trimmed
}

func operationalLabel(t *testing.T, output, label string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), label) {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), label))
		}
	}
	t.Fatalf("output omitted stable label %q:\n%s", label, output)
	return ""
}

func parseParenthesizedFullID(t *testing.T, output string) string {
	t.Helper()
	start := strings.LastIndex(output, "(sha256:")
	if start < 0 {
		t.Fatalf("output omitted parenthesized full event ID:\n%s", output)
	}
	start++
	end := strings.Index(output[start:], ")")
	if end < 0 {
		t.Fatalf("output has unterminated full event ID:\n%s", output)
	}
	id := output[start : start+end]
	if len(id) != len("sha256:")+64 {
		t.Fatalf("output event ID is not full: %q", id)
	}
	return id
}

func parseOperationalRotation(t *testing.T, output string) (string, string) {
	t.Helper()
	fields := strings.Fields(output)
	if len(fields) != 6 || fields[0] != "Rotated" || fields[1] != "identity" || fields[2] != "from" || fields[4] != "to" {
		t.Fatalf("rotation output omitted stable full actors:\n%s", output)
	}
	if len(fields[3]) != 64 || len(fields[5]) != 64 {
		t.Fatalf("rotation output shortened actor IDs:\n%s", output)
	}
	return fields[3], fields[5]
}

func containsOperationalID(ids []string, wanted string) bool {
	for _, id := range ids {
		if id == wanted {
			return true
		}
	}
	return false
}

func assertOperationalContains(t *testing.T, output string, values ...string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(output, value) {
			t.Fatalf("output omitted %q:\n%s", value, output)
		}
	}
}

func assertOperationalRef(t *testing.T, repository, ref string, exists bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", ref)
	command.Dir = repository
	command.Env = operationalTestEnvironment(t.TempDir())
	err := command.Run()
	if ctx.Err() != nil {
		t.Fatalf("inspect ref %s timed out: %v", ref, ctx.Err())
	}
	if exists && err != nil {
		t.Fatalf("expected ref %s: %v", ref, err)
	}
	if !exists && err == nil {
		t.Fatalf("unexpected ref %s", ref)
	}
}

func setupOperationalLedgerRepository(t *testing.T) string {
	t.Helper()
	repository := filepath.Join(t.TempDir(), "repo")
	mustGit(t, "init", "-q", "-b", "main", repository)
	mustGit(t, "-C", repository, "config", "user.name", "Ledger Fixture")
	mustGit(t, "-C", repository, "config", "user.email", "ledger@hn.invalid")
	mustGit(t, "-C", repository, "commit", "--allow-empty", "-q", "-m", "base")
	withTestDirectory(t, repository)
	actor := testIdentity(t, "Ledger Actor")
	event := newEvent(actor, "issue.open", 1, "")
	event.Title = "accepted before replication"
	if _, err := appendEvent(event, actor); err != nil {
		t.Fatal(err)
	}
	gitDir, err := requireGitRepository()
	if err != nil {
		t.Fatal(err)
	}
	return gitDir
}

func writeOperationalTransactionFixture(t *testing.T, gitDir, id string, contents []byte) {
	t.Helper()
	directory := replicationTransactionsPath(gitDir)
	if err := ensurePrivateDirectory(filepath.Join(gitDir, "hn")); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateDirectory(filepath.Join(gitDir, "hn", "replication")); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateDirectory(directory); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateFileAtomic(filepath.Join(directory, id+".json"), append(contents, '\n')); err != nil {
		t.Fatal(err)
	}
}

func onlyOperationalStateFile(t *testing.T, directory string) string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].IsDir() {
		t.Fatalf("%s contains %d state entries, want one regular file", directory, len(entries))
	}
	return filepath.Join(directory, entries[0].Name())
}

func recordOperationalTransactionFixture(t *testing.T, gitDir, id, state string, pending, accepted []string) {
	t.Helper()
	for _, directory := range []string{
		filepath.Join(gitDir, "hn"),
		filepath.Join(gitDir, "hn", "replication"),
		replicationTransactionsPath(gitDir),
	} {
		if err := ensurePrivateDirectory(directory); err != nil {
			t.Fatal(err)
		}
	}
	result := replicationTransactionResult{
		ID: id, Remote: "origin",
		pendingObjects: append([]string{}, pending...), acceptedObjects: append([]string{}, accepted...),
	}
	if state == "validated" {
		if err := createReplicationPendingAnchor(gitDir, result); err != nil {
			t.Fatal(err)
		}
	}
	if err := recordReplicationTransaction(gitDir, result, state); err != nil {
		t.Fatal(err)
	}
}

func operationalTestEnvironment(home string) []string {
	return operationalTestEnvironmentWith(home, nil)
}

func operationalTestEnvironmentWith(home string, overrides map[string]string) []string {
	blocked := []string{"HOME=", "GIT_CONFIG_GLOBAL=", "GIT_ASKPASS=", "SSH_ASKPASS=", "GH_TOKEN=", "GITHUB_TOKEN=", "HN_", "SPEC_KITTY_"}
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
		blocked = append(blocked, key+"=")
	}
	sort.Strings(keys)
	environment := make([]string, 0, len(os.Environ())+4)
	for _, entry := range os.Environ() {
		skip := false
		for _, prefix := range blocked {
			if strings.HasPrefix(entry, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			environment = append(environment, entry)
		}
	}
	environment = append(environment,
		"HOME="+home,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
	)
	for _, key := range keys {
		environment = append(environment, key+"="+overrides[key])
	}
	return environment
}

func buildOperationalBinary(t *testing.T) string {
	t.Helper()
	source, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "hn")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = source
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build hn: %v\n%s", err, output)
	}
	return binary
}

func runOperationalCommand(t *testing.T, binary, directory string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = directory
	command.Env = operationalTestEnvironment(t.TempDir())
	command.Stdin = strings.NewReader("")
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("hn %s timed out: %v\n%s", strings.Join(args, " "), ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("hn %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func runOperationalCommandFailure(t *testing.T, binary, directory string, args ...string) string {
	return runOperationalCommandFailureWithEnv(t, binary, directory, nil, args...)
}

func runOperationalCommandFailureWithEnv(t *testing.T, binary, directory string, overrides map[string]string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = directory
	command.Env = operationalTestEnvironmentWith(t.TempDir(), overrides)
	command.Stdin = strings.NewReader("")
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("hn %s timed out: %v\n%s", strings.Join(args, " "), ctx.Err(), output)
	}
	if err == nil {
		t.Fatalf("hn %s unexpectedly succeeded:\n%s", strings.Join(args, " "), output)
	}
	return string(output)
}

func runOperationalGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = directory
	command.Env = operationalTestEnvironment(t.TempDir())
	command.Stdin = strings.NewReader("")
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("git %s timed out: %v\n%s", strings.Join(args, " "), ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func runOperationalGitFailure(t *testing.T, directory string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = directory
	command.Env = operationalTestEnvironment(t.TempDir())
	command.Stdin = strings.NewReader("")
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("git %s timed out: %v\n%s", strings.Join(args, " "), ctx.Err(), output)
	}
	if err == nil {
		t.Fatalf("git %s unexpectedly succeeded:\n%s", strings.Join(args, " "), output)
	}
	return strings.TrimSpace(string(output))
}

func runOperationalGitBytes(t *testing.T, directory string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = directory
	command.Env = operationalTestEnvironment(t.TempDir())
	command.Stdin = strings.NewReader("")
	output, err := command.Output()
	if ctx.Err() != nil {
		t.Fatalf("git %s timed out: %v", strings.Join(args, " "), ctx.Err())
	}
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return output
}

func TestReplicationAcceptanceReadAcrossReceiptPublication(t *testing.T) {
	gitDir := setupOperationalLedgerRepository(t)
	object := strings.Repeat("1", 40)
	txn := replicationTransactionResult{ID: "txn-concurrent", Remote: "origin", pendingObjects: []string{object}}
	if err := createReplicationPendingAnchor(gitDir, txn); err != nil {
		t.Fatal(err)
	}
	staged, err := os.CreateTemp(replicationAnchorsPath(gitDir), ".hn-private-*")
	if err != nil {
		t.Fatal(err)
	}
	if err := staged.Close(); err != nil {
		t.Fatal(err)
	}

	reads := 0
	denied, _, err := readReplicationAcceptanceSnapshot(func() (map[string]bool, map[string]replicationTransactionRecord, error) {
		reads++
		if reads == 2 {
			if err := recordReplicationTransaction(gitDir, txn, "validated"); err != nil {
				t.Fatal(err)
			}
		}
		return loadReplicationAcceptanceSnapshot(gitDir)
	})
	if err != nil || reads != 2 || !denied[object] {
		t.Fatalf("read did not converge to verified pending state: reads=%d denied=%v err=%v", reads, denied, err)
	}
}
