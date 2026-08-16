# Security & Hardening

Security is a scoring criterion (Production Readiness), and the design treats it as layered defence, not a single wall.

## Least-privilege IAM

Each Lambda gets only the permissions it needs (`terraform/modules/iam`):

| Role | Can |
|------|-----|
| `agent_worker` | Invoke Bedrock (Claude + Titan); read SSM config; **no** infra-mutating rights |
| `dispatcher` | Invoke the worker fleet (`lambda:InvokeFunction` on worker + alias) |
| `dashboard_api` | Read CloudWatch alarms/metrics; `events:Enable/Disable/DescribeRule` on `*-schedule` **only**; `lambda:GetFunctionConfiguration`/`ListFunctions`; `tag:GetResources`; invoke dispatcher |

The `dashboard_api` control role can toggle *schedules* and *read* configuration, but cannot delete functions, change code, or touch data-plane resources. The blast radius of a compromised control plane is "pause the fleet", not "destroy it".

## Secrets never live in the repo

- Runtime config comes from **SSM Parameter Store** (SecureString for the DB URL and MCP key), read at cold start — never baked into an image or committed.
- `.gitignore` blocks `*.tfstate`, `*.tfvars`, and every `.env*` (except examples). A pre-push secret scan checks staged content for connection strings, private keys, and AWS keys. The 470 MB PaySim CSV and DB backups are ignored too — data is regenerable (`cmd/gen-data`, `scripts/demo-stream.py`) rather than stored in git, deliberately not auto-run by `scripts/init.sh` itself (see [Deployment](DEPLOYMENT.md)).

## Prompt-injection defence

Transaction text fields (`name_orig`, `name_dest`) originate from an external feed and flow into a Bedrock prompt — a classic injection surface. `SanitizeField` ([`internal/agent/validation.go`](../internal/agent/validation.go)) strips everything outside `[\w\s.,\-]` and truncates:

```go
var sanitizePattern = regexp.MustCompile(`[^\w\s.,\-]`)
func SanitizeField(value string, maxLen int) string {
    clean := sanitizePattern.ReplaceAllString(value, "")
    if len(clean) > maxLen { return clean[:maxLen] }
    return clean
}
```

Quotes, braces, colons, and angle brackets — the characters that break out of the prompt context or forge JSON — never survive. Numeric fields are never string-interpolated; they're typed (`float64`, `int`). The unit test [`internal/agent/validation_test.go`](../internal/agent/validation_test.go) asserts injection payloads (`{"role":"system",…}`, `IGNORE PREVIOUS INSTRUCTIONS`, `'; DROP TABLE tasks; --`) are neutralised.

The same defence covers the **human-taught memory** path (see [Agentic Memory](AGENTIC_MEMORY.md#human-taught-memories)): a reviewer's free-text notes *and* their name are both `SanitizeField`-cleaned before being embedded into `case_memory.summary` — that text is recalled verbatim into a future Bedrock prompt, so an unsanitised review note or a hostile reviewer name would be a durable, self-propagating injection vector rather than a one-off request.

## SQL injection defence in the MCP tool layer

`pkg/mcp`'s `select_query` tool takes one opaque SQL string over JSON-RPC — the protocol has no bind-parameter slot, so the usual `pgx` parameterised-query defence isn't available for it. Three layers instead, all from the OWASP SQL Injection Prevention Cheat Sheet:

- **Allow-list input validation.** `GetTransaction`'s ID is checked against `uuid.Parse` before it ever reaches query construction — a non-UUID string has no legitimate reason to look up a transaction and is rejected outright. `SearchSimilarCases`'s `transaction_type` and `verdict` filters are checked against the exact CHECK-constrained enum values from the schema.
- **Escaping all user-supplied input.** Free-text filters that can't be allow-listed go through `sqlQuoteLiteral` — the same doubled-single-quote rule Postgres/CockroachDB's own `quote_literal()` applies — before interpolation.
- **Fuzzing.** `FuzzSQLQuoteLiteral` (`pkg/mcp/fuzz_test.go`) runs the escaper against arbitrary byte sequences and asserts the output never contains an unescaped `'`.

## Read-only by construction

- **Agent exploration is read-only.** The agent explores customer context through the CockroachDB **MCP Server**, which exposes only `SELECT`. It physically cannot mutate data while investigating; writes go through a separate, typed `pgx` path.
- **The DB console is read-only.** `/control/query` rejects anything that isn't a single `SELECT`/`SHOW`/`WITH`, blocks mutating keywords, and caps results at 200 rows — with the database role itself read-scoped underneath (defence-in-depth). See [API](API.md).

## Control-plane authentication

Mutating control endpoints (`/control/fleet` POST, `/control/dispatch`, `/control/feed`, `/control/query`) honour a `CONTROL_TOKEN` env: when set, they require a matching `X-Control-Token` header. Read endpoints stay open because the data is non-sensitive synthetic PaySim. For a real deployment this is where SSO/RBAC would attach — called out explicitly in the commercialisation roadmap, not left as an unstated gap.

## Auditability as a security property

Every state change is an append-only `audit_log` row with the acting `agent_id`, the `reasoning`, and — for human decisions — the `reviewer_id`. Because the table is append-only and foreign-keyed inside a strongly-consistent database, the trail cannot be silently rewritten or orphaned. "Who blocked this customer and why" is always answerable, including the human who approved it.

## What's intentionally out of scope (and said so)

End-user authn/authz, multi-tenancy, and automated Bedrock cost circuit-breakers are documented as commercialisation roadmap items in the root README — deliberate scope cuts to hit the deadline, not oversights. Naming them is itself a production-readiness signal.
