# Diagnostics

`pg-contract` validates queries by asking Postgres to prepare them against the before and after schemas. Postgres parses, analyzes, and rewrites a statement during `PREPARE`, so diagnostics are grounded in Postgres SQLSTATE codes rather than string matching on error messages.

The first compatibility rule is intentionally narrow:

- valid before and valid after: no breakage reported
- valid before and invalid after: breaking schema contract
- invalid before: reported separately because the query did not establish a valid before-contract

## Covered SQLSTATEs

| SQLSTATE | Condition | Typical compatibility signal | False-positive risk |
| --- | --- | --- | --- |
| `22P02` | invalid text representation | Enum value or typed literal no longer fits the target type. | Also covers any invalid literal/cast, not only enums. |
| `3F000` | invalid schema name | Referenced schema was removed or renamed. | May indicate session setup or `search_path` drift instead of a schema migration. |
| `42601` | syntax error | Query text is not valid for the target Postgres parser. | Usually a query bug, not always a schema compatibility break. |
| `42702` | ambiguous column | A schema change made an unqualified column ambiguous. | The fix is query qualification, but the schema change might still be acceptable. |
| `42703` | undefined column | A referenced column was removed, renamed, or hidden behind a changed view. | Can also happen when a view changed shape. |
| `42704` | undefined object | A referenced object no longer exists. | Broad condition; inspect the Postgres message for object kind. |
| `42725` | ambiguous function | Function overloads make a call ambiguous. | Explicit casts may be enough without changing schema rollout. |
| `42804` | datatype mismatch | Expression, return type, or column type changed incompatibly. | Postgres reports many type mismatch shapes under this code. |
| `42809` | wrong object type | A referenced name exists but is not the expected object kind. | Could be caused by search path resolution rather than object replacement. |
| `42846` | cannot coerce | A cast or coercion is invalid after a type change. | Might be a query portability issue rather than a migration issue. |
| `42883` | undefined function | Function/operator signature no longer matches inferred argument types. | Also covers operators and missing explicit casts. |
| `42P01` | undefined table | Table/view is missing or unqualified lookup no longer resolves. | Often ambiguous between object removal and `search_path` change. |
| `42P02` | undefined parameter | Query references a parameter Postgres cannot bind for the prepared statement. | Often fixed in query/config, not necessarily schema rollout. |
| `42P10` | invalid column reference | Column reference or constraint inference does not match target schema. | Cause depends heavily on the SQL construct. |
| `42P18` | indeterminate datatype | Postgres cannot infer a parameter type during prepare. | Usually requires explicit casts or `pg-contract.yaml` parameter types. |

## Output Shape

Text, JSON, and GitHub annotation outputs use the same diagnostic classifier:

- `reason`: plain-English summary for the SQLSTATE
- `suggestion`: conservative next step for preserving or updating the query contract
- raw Postgres fields: SQLSTATE, message, hint, detail, position, and object names when Postgres provides them

When no specific SQLSTATE mapping exists, `pg-contract` falls back to the Postgres message and a conservative suggestion to preserve the previous database contract until application queries are updated.
