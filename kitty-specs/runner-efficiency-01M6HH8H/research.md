# Runner-efficiency research

The user confirmed native runner efficiency for #27/#30, default-workflow
preservation, explicit stale-base filtering and separate concurrent coordination.
The two issues are related but not duplicates; leave open until delivered.

## Existing seams and decisions

- runner.go runnerOnceContext: sync/read/filter and own-result skip is the
  scheduler boundary. Preserve explicit setup, sandbox and actor selection.
- lineage.go buildLineageIndex/state: reuse superseded/merged/closed/conflict
  semantics; avoid a parallel model.
- policy.go evaluateProposal: extract one shared pipeline evaluator aggregating
  latest trusted results across authorized exact-candidate requests. Per-request
  counting would disagree with governance readiness.
- ci_test.go TestDistributedPipelineRun: no required policy pipeline is
  intentional. Absence must not prohibit existing explicit execution.
- event.go/store.go/quarantine.go: add strict run.withdraw using existing Subject
  (full request ID) and Body (reason). Share requester/ordering/dependency
  validation across local reads and quarantine. Exact retries do not append.
- replicationEventReferences also drives shallow and archive closure; include
  the original request dependency and refuse missing/invalid histories.
- contribution_moderation.go: preserve current runner prohibition. New static
  help/capabilities remain keyless reads, independent of extension RPC discovery.
- project_snapshot.go projectGit supplies bounded contextual Git patterns.
  Ordinary gitOutput is unbounded and inherits ambient Git overrides.
- Proposals do not name target branches and cmdSync does not update code branch
  refs. Explicit stale mode observes only the selected advertised remote ref,
  never checkout HEAD; it is an observation, not a lock.
- Withdrawal prevents future starts without retroactively invalidating prior
  results/decisions/merges. Already-running effect cancellation is separate.
- New withdrawal kinds may cause old readers to reject that actor advancement;
  no existing workflow writes the kind implicitly.
- Threshold suppression does not eliminate simultaneous starts. Claims need a
  separate contract for competing claims, partitions, expiry and process loss.

Independent read-only source investigation preceded implementation. Sources and
findings are recorded in research/source-register.csv and evidence-log.csv.
