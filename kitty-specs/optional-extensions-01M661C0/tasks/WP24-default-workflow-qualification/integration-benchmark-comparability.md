# Preserve the frozen baseline and compare like builds

2026-10-31 — root, architect-alphonso. Prospective qualification decision; no measurements or pass are asserted here.

The approved WP01 binaries use Go1.26.4, `-trimpath`, stripped linker flags, normal VCS stamping and `CGO_ENABLED=1`. The existing Go-only packager deliberately uses `CGO_ENABLED=0`, `-buildvcs=false`, an empty build ID and explicit version metadata. Preserve the exact accepted binaries and all original measurements. Do not silently compare different build settings and attribute the result solely to extension code, or replace the frozen first gate with newly built baseline data.

WP24 will retain two separately labelled, paired comparisons:

1. The exact frozen WP01 standard/combined binaries against the final candidate built with the same benchmark compiler/settings, isolating the native workflow change under the accepted measurement protocol.
2. The same frozen baseline source rebuilt with the existing packager settings against the candidate with those same settings, covering the Go-only packaged build. Bind both new executables/source/flags independently; the rebuilt baseline is supplemental evidence, never the original frozen executable. Verify baseline public outputs against the accepted corpus before measuring.

For each comparison, use all nine representative read commands, both variants, registry absent and large unrelated custom histories, and 30 alternating paired observations per command/variant/state. This is 1080 pairs per comparison; a pair contains one baseline and one candidate observation. Preserve every raw observation, environment/load and outcome. Reuse the accepted native GNU Time/monotonic measurement scope or independently establish equivalence. Both comparisons use the unchanged elapsed/RSS thresholds; a failure must be investigated and retained, never hidden by choosing the other build setting or dropping samples. No timing from race builds qualifies.

Use `0.0.0-optional-extensions-qualification.1` as the internal qualification package version for WP24/WP25. It is accepted by the existing packager version grammar and identifies test assets only. Creating a public release or tag is outside this decision. Record benchmark-only version strings and all metadata differences explicitly; WP25 must bind its final package/source identities and demonstrate relevant runtime-code equivalence to any reused measurements. A changed runtime input requires affected requalification.

This changes no product behavior, mandatory dependency, public performance budget, contract, file owner, lane or acceptance requirement. It resolves an observed build-settings confound before measurements begin.
