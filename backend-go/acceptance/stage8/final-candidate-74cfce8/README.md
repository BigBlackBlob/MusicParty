# Stage 8 final-candidate closure

Accepted source: `74cfce8beae89a948a6cba089b8d7897b7117c71`

Release target: `go-v1.0.0-rc2`

The measured Stage 8 acceptance from `96727c13434567152bed889798f1e4803e30b5cf` and the final closure from `5b0cc4abd7ec9655cc96d6c0806458b9f585edf9` are carried forward. The Stage 8 load, browser and platform gates were not rerun.

Since the prior release boundary, the only runtime change is the targeted SQLite index compatibility correction. It ignores table-local SQLite index `ColumnID` differences while retaining indexed key names and order, DESC direction, collation, uniqueness and partial-index semantics. The accompanying tests use real SQLite `InspectSchema` and `index_xinfo` data.

Latest successful remote CI for this candidate: Go Backend run `30737512359` and Quality run `30737512354`, both on the accepted source commit.

## Isolated SQLite evidence

Read-only VPS-derived isolated-copy evidence is recorded without source WAL bundle values or contents. The Java baseline initializer brought the isolated derivative from 21 to 23 application tables. After the compatibility correction, `dbcheck` reported:

```json
{"integrity":["ok"],"foreignKeyViolations":[],"applicationTables":23,"schemaCompatible":true,"schemaDifferences":[]}
```

Local paths and database hashes are intentionally excluded because the complete Stage 9 evidence is not yet complete.

The full Java-first → Go → Java handoff remains pending. It will be performed with the newly published immutable image digest; this closure does not mark Stage 9 complete and did not touch production.

This declarative acceptance contains no credentials, database, or executable material.
