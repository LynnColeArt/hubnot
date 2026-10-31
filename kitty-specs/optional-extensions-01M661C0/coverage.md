# Planning coverage map

Date: 2026-10-30. This maps requirements to architectural concerns and future proof;
it is not a test pass, work-package ownership map or implementation status.

| Requirement | Concerns | Planned evidence / outcome |
|---|---|---|
| FR-001 | IC-01, IC-11 | SC-001, SC-002, SC-010 |
| FR-002 | IC-01, IC-05 | SC-001, SC-004 |
| FR-003 | IC-05, IC-06 | SC-004, SC-008 |
| FR-004 | IC-05, IC-07 | SC-004, SC-009 |
| FR-005 | IC-05, IC-06, IC-07 | SC-004 |
| FR-006 | IC-07 | SC-003, SC-006 |
| FR-007 | IC-04, IC-07, IC-09 | SC-003, SC-005 |
| FR-008 | IC-08, IC-09 | SC-003, SC-008 |
| FR-009 | IC-02, IC-04 | SC-005, SC-006 |
| FR-010 | IC-02, IC-03 | SC-005, SC-008 |
| FR-011 | IC-03, IC-04 | SC-005, SC-006 |
| FR-012 | IC-02, IC-03, IC-10 | SC-005, SC-007 |
| FR-013 | IC-04 | SC-005, SC-006 |
| FR-014 | IC-03, IC-11 | SC-006 |
| FR-015 | IC-02, IC-04, IC-11 | SC-001, SC-006, SC-008 |
| FR-016 | IC-02, IC-04 | SC-005 |
| FR-017 | IC-03, IC-07, IC-08 | SC-001, SC-004, SC-008 |
| FR-018 | IC-03, IC-07, IC-09 | SC-003, SC-004, SC-006 |
| FR-019 | IC-06, IC-11 | SC-008 |
| FR-020 | IC-06, IC-07, IC-08 | SC-003, SC-004, SC-008 |
| FR-021 | IC-09 | SC-003 |
| FR-022 | IC-10 | SC-007 |
| FR-023 | IC-10 | SC-005, SC-006 |
| FR-024 | IC-12 | SC-009, SC-010 |
| NFR-001 | IC-01, IC-11 | Default matrix/no-loader controls |
| NFR-002 | IC-01, IC-11 | 30 paired observations and raw metric limits |
| NFR-003 | IC-02, IC-03, IC-06, IC-08, IC-11 | Boundary/refusal cases |
| NFR-004 | IC-06, IC-11 | Child failure/cleanup and unrelated use |
| NFR-005 | IC-01, IC-03, IC-11 | Existing golden hashes and meaningful refusals |
| NFR-006 | IC-05, IC-06, IC-09, IC-11 | Public/private leak cases |
| NFR-007 | IC-11 | Exact package platform/variant matrix |
| NFR-008 | IC-03, IC-09, IC-10, IC-11 | Two independent clones, absent specialized module |
| C-001 | IC-01, IC-12 | Separate mission/prerequisite, no beta change |
| C-002 | IC-01, IC-06, IC-11 | Native dependencies and Go-only packaging |
| C-003 | IC-05, IC-06, IC-10 | Inert declarations and explicit enablement/selection |
| C-004 | IC-10, IC-12 | Representative consumer, no doctrine engine |
| C-005 | IC-09, IC-12 | Fake tracker and connector-owned external state |
| C-006 | IC-02, IC-03, IC-11 | Additive protocol, signed goldens |
| C-007 | IC-06, IC-12 | Host mediation threat boundary |
| C-008 | IC-05, IC-07, IC-12 | Inventory/grants, unsupported privileged surfaces |
| C-009 | IC-06, IC-12 | Single transport and no marketplace/new GUI |
| C-010 | IC-01, IC-02, IC-10 | Memory unchanged, no implicit activation |

All ten success criteria map to FR rows above and IC-11 qualification. Task authoring
must expand this into explicit subtask/WP ownership and concrete acceptance cases.
No tasks or lane state has been finalized by this planning map.
