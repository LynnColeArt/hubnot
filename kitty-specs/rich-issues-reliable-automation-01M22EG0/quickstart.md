# Acceptance walkthrough

1. Build hn and initialize two identities in temporary Git clones.
2. Create a simple title-only issue, comment, and confirm existing human views.
3. Create rich state through strict JSON; inspect full issue and revision IDs.
4. Revise with an exact expected revision and operation key; retry and compare event IDs/counts.
5. Attempt a stale revision and divergent operation reuse; compare refs before/after.
6. Make independent revisions in both clones, sync admitted actor histories, and inspect both heads.
7. Resolve the complete head set; inspect immutable history and a single resulting head.
8. Add relationships; inspect incoming/outgoing links and cycle diagnostics.
9. Page through list/history/graph; mutate then confirm old cursor fails.
10. Run the generic consumer example and1000issue verified-reader measurement, then full race/vet/build/diff gates.
