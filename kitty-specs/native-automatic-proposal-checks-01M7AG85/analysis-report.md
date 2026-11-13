---
schema_version: 1
artifact_type: spec-kitty.analysis-report
command: /spec-kitty.analyze
mission_slug: native-automatic-proposal-checks-01M7AG85
mission_id: 01M7AG85QTW0CP6EQ1Z86BQ9H6
generated_at: '2026-11-13T13:08:52.527025+00:00'
analyzer_agent: unknown
input_artifacts:
  spec.md:
    path: kitty-specs/native-automatic-proposal-checks-01M7AG85/spec.md
    sha256: 1a644e7d74a994b1a37eb966021c010ba852b4509f6cd953418c9fe47630a7f0
  plan.md:
    path: kitty-specs/native-automatic-proposal-checks-01M7AG85/plan.md
    sha256: fe26268f018550644b7a4abf2cb1d65464fcba73768e37d3bd3b7f967f954a20
  tasks.md:
    path: kitty-specs/native-automatic-proposal-checks-01M7AG85/tasks.md
    sha256: c162bd7aac2dfb824fdd6f3e9906ac514cde8483bfd40c3ec9300599a9250198
  charter:
    path: .kittify/charter/charter.yaml
    sha256: 3b0165cdbea9e22adf47d441d2e05c0acad4513350b2bce8e87924a03fe2ee5a
verdict: unknown
issue_counts:
  high:
  critical:
  info:
  low:
  medium:
findings: []
---

# Native evidence review

Native implementation and regression are already committed. The paired seven-test run passed against the fixed binary. Review the exact first-parent binding and confirm that the side-branch regression rejects both the side ref and a side commit merged into the canonical target.
