# Selected classic definition credential boundary

The qualified Get Definition response and Microsoft versioned Build models were walked before the final source commit. The documented variable DTO has one service-managed secret flag, and the same DTO appears in both definition variables and variable-group variables. Both paths are sanitized; null/nonboolean flags, duplicate keys and case-aliased field declarations refuse. Ordinary variable values remain observational source.

The documented process data-source binding contains authorization-header objects. Their values are removed while header names and endpoint references remain inert metadata. Explicit source findings disclose excluded values. No endpoint is invoked and no credentials, queues or execution authority are adopted.

The sanitizer uses raw JSON values to retain numeric precision, rejects duplicate JSON object keys before map decoding and bounds JSON nesting. Blob/revision hashes derive only from sanitized bytes. Fixtures verify that changing original excluded values does not change the canonical snapshot, inspect archive bytes and retain ordinary fields plus a numeric value beyond 2^53.

Other process input/source-definition/property-bag fields are retained as selected source, without guessing their runtime meaning or fetching service endpoints. This is a declared service-managed credential boundary, not a promise to discover arbitrary plaintext secrets in YAML, Git or task strings.

Primary references: [Get Definition schema](https://learn.microsoft.com/en-us/rest/api/azure/devops/build/definitions/get?view=azure-devops-rest-7.1), [Microsoft versioned Build models](https://github.com/microsoft/azure-devops-python-api/blob/dev/azure-devops/azure/devops/v7_1/build/models.py). Tests exercise the actual HTTP Snapshot boundary in `TestAzureMigrationClassicServiceManagedCredentialsExcluded` and `TestAzureMigrationClassicMalformedCredentialDeclarationsRefuse`.
