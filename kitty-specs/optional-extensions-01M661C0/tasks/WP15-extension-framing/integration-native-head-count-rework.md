# Native visible-head compatibility correction

Root architectural decision, 2026-10-31. This is a bounded implementation
correction to the frozen host contract, not a new method, grant or authority.

Read-only WP17 preparation identified that `mutationResultRule` at approved host
source `320e212b26a5f151b1af09083895d479b4a8396f` requires `head_count >= 1`.
The approved native issue/resource writers at artifact source
`94503db2a8f914eb5df190348dfa6125bba4defd` legitimately retain zero visible heads
when contribution admission is pending or hidden. The field is a current governed
observation, not a promise that a signed event has become visible. Host-v1 section
4 calls it an integer and explicitly preserves actual admission. Inventing one
visible head would misrepresent the native result.

WP15 must accept a nonnegative integer head count, including zero, while retaining
the exact finite JSON integer domain, all response fields and all admission and
protocol/family/type rules. Negative, fractional, missing and otherwise invalid
values must still refuse. Native pending/hidden mutation results and nested found
operation results must pass unchanged; type publication still reports the actual
writer's one declaration. No native writer or signed bytes change.

Keep the repair in WP15-owned protocol validation/tests. First retain a meaningful
old-validator RED with a complete legal native pending/hidden response, then verify
the corrected validation through its current live framing boundary and independent
literal controls. Reuse unchanged frame/session evidence only after byte equality;
do not repeat unrelated transport or signing-head recovery probes. Run the normal
review gate and independent review with explicit actual identities.

After approval root will refresh WP16 against the new approved protocol source,
preserving its lifecycle production code, then qualify/review that host union before
WP17 consumes it. WP11 and WP12 artifact work is independent of this correction.
Ownership, lane identities, dependency graph, inventories, limits and the default
workflow hard gate remain unchanged. Historical approvals/evidence remain labelled
with their original sources; they are not silently transferred to new binaries.
