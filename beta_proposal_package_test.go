package main

import (
	"reflect"
	"strings"
	"testing"
)

// This is a transported-executable observation, separate from source regression
// tests. Explicit package refusal must never fall back to a local application build.
func TestBetaProposalAncestryPackages(t *testing.T) {
	if *portablePackageDirectory == "" {
		t.Log("exact proposal-package checks require explicit package inputs")
		return
	}
	for _, tags := range []string{"", "noforge"} {
		t.Run("tags="+tags, func(t *testing.T) {
			f := newBetaContributionCLI(t, portableJourneyBinary(t, tags))
			repo := f.repository()
			f.git(repo, "config", "maintenance.auto", "false")
			f.git(repo, "config", "gc.auto", "0")
			f.hn(repo, true, "init", "--name", "Packaged proposal author")
			actor := f.actor(repo)
			writeTestPolicy(t, repo, PolicyDocument{
				Version: policyVersion, Maintainers: []string{actor},
				Proposals: ProposalPolicy{RequiredAccepts: 1}, Pipelines: map[string]PipelinePolicy{},
			})
			f.commit(repo, "Base policy")
			base := f.git(repo, "rev-parse", "HEAD")
			f.commit(repo, "Feature")
			head := f.git(repo, "rev-parse", "HEAD")
			f.git(repo, "switch", "--detach", "-q", base)
			f.commit(repo, "New base")
			otherBase := f.git(repo, "rev-parse", "HEAD")
			state := &artifactCLIFixture{t: t, repo: repo, home: strings.TrimPrefix(f.env[1], "HOME=")}
			refuse := func(args ...string) {
				t.Helper()
				before := state.snapshot()
				out := f.hn(repo, false, args...)
				betaContributionContains(t, out, "proposal head "+head+" does not descend from base "+otherBase)
				if !reflect.DeepEqual(before, state.snapshot()) {
					t.Fatal("packaged proposal refusal changed objects, refs or private state")
				}
			}
			refuse("proposal", "open", "--base", otherBase, "--head", head, "Invalid candidate")
			f.hn(repo, true, "proposal", "open", "--base", base, "--head", head, "Valid candidate")
			ref := f.git(repo, "for-each-ref", "--format=%(refname)", "refs/hn/proposals")
			if !strings.HasPrefix(ref, "refs/hn/proposals/") || strings.Contains(ref, "\n") {
				t.Fatal("expected exactly one valid proposal ref", ref)
			}
			id := "sha256:" + strings.TrimPrefix(ref, "refs/hn/proposals/")
			refuse("proposal", "revise", id, "--base", otherBase, "--head", head)
			f.hn(repo, true, "proposal", "revise", id, "--base", base, "--head", otherBase)
			// A later signed native command still succeeds for the same actor.
			f.hn(repo, true, "issue", "open", "Author continues after refusal")
			f.hn(repo, true, "proposal", "show", id)
		})
	}
}
