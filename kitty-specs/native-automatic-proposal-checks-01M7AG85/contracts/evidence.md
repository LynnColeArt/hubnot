# Automatic proposal check evidence

An automatic run request signs its target ref, configuration source commit, trigger digest, check ID and attempt ID. The result signs its exact request ID. Policy accepts the source only on the canonical target's first-parent history. This preserves checks across ordinary target advancement and excludes commits reachable only through a merged side branch. History verification is bounded to 10,000 first-parent steps and fails closed if the target or source cannot be verified.

The scheduling clone uses its registered target. A receiving clone uses the local branch corresponding to origin's configured default branch, or main when that symbolic ref is absent. Update that local target through the project's governed workflow before evaluating evidence. Keyed proposals also bind the target to the signed publication base ref.

An opted-in Linux runner executes the pinned definition against the exact candidate using Bubblewrap. Manual run requests retain their existing definition and policy semantics.
