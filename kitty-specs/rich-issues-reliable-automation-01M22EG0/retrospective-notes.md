# Retrospective notes: Rich Issues and Reliable Automation

Recorded by the host after local integration on 2026-09-09. These are process observations reconstructed from retained command logs and independent review records, supplementing the runtime-generated `retrospective.yaml`. They do not replace canonical status or acceptance events.

## Practices to retain

- Independent review found four substantive boundary defects across three work packages: a raw signed-payload limit bypass, loss of a failed Git subprocess's cause, inconsistent machine-mode flag parsing, and lossy malformed-Unicode decoding. Each was reproduced, corrected by its implementer, and independently checked again before approval. Preserve this separation of implementation and review, including regression probes at the public boundary.
- Public CLI consumers and temporary Git replicas exposed assumptions that internal unit tests alone would miss. The acceptance suite now exercises replay, stale writes, conflicting replicas, exact supplier admission, and the documented input format through the shipped commands.
- The 201-head recovery fixture made the interaction between bounded output and conflict resolution concrete. Pagination alone would have left an issue with more than 200 heads impossible to resolve. Explicit staged resolution preserves unconsumed heads and checks the observed snapshot.
- The 1000-issue comparison checked complete verified event equality as well as Git process counts. This provides stronger evidence than timing alone. Keep fixture-specific measurements separate from general latency promises.
- A source fingerprint over all 72 source, test and product documentation paths established that the merged source equals the qualified candidate. Metadata-only merge recovery did not require a redundant full test run.
- General issue concepts and a standalone CLI consumer kept Hubnot useful independently of Go Kitty. Assignment, status and criteria carry descriptive information; execution and integration authority remain with consumers.

## Tooling follow-ups

These are observations for future Spec Kitty/runtime work, not open Hubnot product defects and not changes applied to global doctrine in this mission.

| Observation | Effect and recovery | Suggested improvement |
|---|---|---|
| Inherited planning and status commits triggered lane ownership guards | Source ownership and actual independent verdicts were verified; the documented force exception addressed metadata hygiene only | Exclude runtime-owned inherited metadata from source ownership violations, while retaining checks for real cross-package edits |
| A normal Go build artifact was auto-captured in a lane commit | The executable was removed in an explicit cleanup commit; subsequent builds used an external output directory | Keep generated executables out of automatic staging and teach Go gates to use an explicit output directory |
| Primary and coordination metadata diverged | Ordinary Git integration preserved both histories; the final conflicts involved only a derived artifact index and an append-only workflow journal | Make ownership of runtime journals explicit and use a lossless deterministic merge for event records |
| Local acceptance text said no merge was needed despite four pending packages | The host checked actual pending work and completed consolidation | Derive next-action guidance from pending integration state |
| Coordination cleanup attempted branch deletion while its worktree remained registered | The four lane worktrees were removed by the command. The host preserved the remaining derived snapshot externally, verified ancestry and a clean tree, then removed the coordination worktree and branch | Remove a clean worktree before deleting its checked-out branch; report cleanup failure accurately |
| Automatic retrospective ran before branch integration and contained a generic analysis-report presence finding | The existing canonical record was retained and these concrete notes supplement it | Generate the retrospective after integration and distinguish analysis findings from artifact presence |
| Post-merge `retrospect create --update` failed with an unresolved evidence reference | It rejected finding `n-009` referencing absent `e-018`; no record replacement occurred. The existing record remains valid and synthesis dry-run succeeded with no proposals | Remap finding references when merging retrospective evidence lists; add an update round-trip regression |
| Python architectural coverage was unavailable in this Go repository | Actual Go race, vet, build, formatting and public contract gates passed; unrelated Python gates were not reported as passes | Resolve language-appropriate gates from the project charter |

## Closeout and boundaries

All four implementation packages passed independent review before canonical acceptance. All eighteen acceptance criteria passed. The source was integrated locally into `feat/rich-issues-reliable-automation`; publication and the actual Beads replacement remain outside this mission. Historical branches and worktrees were preserved.

The retrospective update failure is recorded in the portable evidence as `work/retrospective-update.json`. The existing canonical retrospective's synthesis dry-run returned `status=ok`, with no proposals, conflicts, applied changes or emitted events. No global policy or doctrine changes were made.
