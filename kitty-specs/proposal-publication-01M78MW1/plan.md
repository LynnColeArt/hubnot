# Native proposal publication plan

Extend the proposal CLI with versioned publish and operation commands. Keep the legacy proposal open path. Add a signed operation binding, reuse native identity and policy checks, and atomically verify the base branch while publishing the actor and candidate refs. A caller retry first resolves the signed operation, compares its exact intent, and returns or repairs the original result. Use structured outcomes throughout. Exercise real Git ref transactions and admission, including existing proposal reads and legacy callers.
