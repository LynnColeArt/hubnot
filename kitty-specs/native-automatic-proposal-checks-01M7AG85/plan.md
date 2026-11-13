# Native implementation plan

## Technical Context

**Language/Version**: Go 1.26.
**Primary Dependencies**: Git and the existing Bubblewrap execution backend.

Keep config validation, durable observation and execution in Hubnot. Sign the target ref with automatic requests and verify the source against canonical first-parent history. Use one evaluator for native policy and synced status. Verify a historical target tip, a merged side branch, and a wrong target in the native regression. Verify Workbox against the resulting binary.
