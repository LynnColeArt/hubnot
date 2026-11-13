# Linux integration evidence

| Requirement | Result |
| --- | --- |
| FR-001 | Production issue journey and replicated-ref removal/restoration test passed. CI inventory reads committed configuration and edits route to Settings. |
| FR-002 | Standard and GitHub-disabled builds matched Chrome and MCP. A custom provider advertised an enum that Chrome used to plan and resume. Removing that provider preserved inspection and prevented apply/resume. A server kill before resume and the native lost-acknowledgement test covered both recovery boundaries. |
| FR-003 | Chrome and MCP returned 24 offline PRs in three pages with provider requests blocked in Chrome. The no-GitHub binary returned the same history. The upstream-change fixture verified explicit reimport without duplicate identity. |
| FR-004 | Production Chrome and MCP tests covered Explore/Code and signed issues. Chrome and MCP matched 10-item and 7-item PR pagination. The 375-pixel viewport check covered keyboard filtering and Back. Proposals, Memory and CI use shared listing reads. |

The Workbox production build passed. Hubnot’s focused migration suite passed. The full Hubnot suite hit its ten-minute aggregate timeout. The cancellation test active at that point passed when run on its own. Full OS qualification and the remaining UI and memory coverage are recorded in [Implementation and verification](../implementation-status.md).
