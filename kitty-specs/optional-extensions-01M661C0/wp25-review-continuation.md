# WP25 independent review continuation — 2026-11-03

The maintainer requested normal independent WP25 review followed by WP26 trials,
with Mac signing explicitly deferred as an accepted beta distribution limitation.

The original lane-x was frozen at source
`3dcbb96091c6191bc5d98cde9dbd1d2b98433f95`, tree
`df57aeda7553e88ab1eb54ab2f0564a7844dee01`, while actual CI maintenance was
integrated on the coordination branch. The installed framework had no lane-x
workspace context; normal readiness used the moving coordination ref and found
no implementation ahead of it. Review-claim auto-sync would advance the frozen
execution worktree. No readiness gate or review verdict is bypassed.

Root therefore explicitly archived the LOCAL old execution branch as
`archive/optional-extensions-mac-3dcbb960` and moved its worktree to
`.worktrees/optional-extensions-01M661C0-frozen-mac-3dcbb960`. Source HEAD/tree,
all 1626 tracked paths' presence/bytes/modes, and every existing qualification/
release ref were independently compared before/after and unchanged. Existing
Mac package assets, aliases, workflow run41465035433 and artifact13149409710
are untouched. This is an explicit local path/branch-name change, not a source
rewrite or a new package certification.

The fresh canonical lane-x is allocated by Spec Kitty with immutable approved
WP24 base `4bb26359dbdb2fbc149911dc99c3a5880a24c031`, then receives the actual
integrated coordination source and reconciled WP25 evidence. This preserves the
full original WP25 review diff, including real implementation commits after
WP24. It does not use an empty ahead commit, force/skip option, manual event edit
or a base limited to the later CI-only delta.

Normal subtask/readiness, for_review regression gate, independent review and
canonical review_result recording remain required. A verdict must bind the exact
reviewed source SHA and report. The original 24 approvals and WP26 dependency on
WP25 are preserved. No main merge or release is authorized by this recovery.

WP26's two trials will use separate fresh AI-agent contexts with only public
user/author guides and actual supplied qualified package/consumer executables.
Record actual commands, source/package/binary identities, observations and
friction. Label these as independent AI-agent trials, never human-adopter
research or implementer/CI replay. Missing observations cannot become passes.

Root's preservation evidence is retained under
`/home/lynn/Documents/Codex/2026-10-24/wa/work/hubnot-independent-acceptance-20261103/frozen-archive-before.json`.
