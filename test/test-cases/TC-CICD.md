# TC-CICD — CI/CD pipeline & DevOps

Traces to: **Production Readiness**. The pipeline is documented in [`docs/CICD.md`](../../docs/CICD.md). The static invariants (P0/P1 below) are asserted by the runnable `test/integration/pipeline_test.sh` — hermetic, no cloud. The behavioural cases (marked *live*) are verified on a real deployment.

| ID | Priority | Precondition | Steps | Expected |
|----|----------|--------------|-------|----------|
| **TC-CICD-01** | P0 | repo checkout | Inspect every workflow under `.github/workflows/`. | The 8 workflows exist: `ci`, `security`, `terraform-infra`, `build-and-push`, `deploy-staging`, `smoke-test`, `deploy-canary`, `deploy-dashboard`. |
| **TC-CICD-02** | P0 | — | Grep all workflows for static AWS keys (`aws-access-key-id`, `AKIA…`, `aws_secret_access_key`). | **None found** — every AWS action authenticates via OIDC (`id-token: write` + `configure-aws-credentials` role assume). |
| **TC-CICD-03** | P0 | — | Inspect `deploy-canary.yml`. | Shifts 10% traffic (`AdditionalVersionWeights`), waits, reads the CloudWatch error alarm, and has **both** a "Promote to 100%" step (alarm OK) and a "Rollback" step (alarm firing). |
| **TC-CICD-04** | P0 | — | Inspect `smoke-test.yml`. | Runs `test/integration/api_smoke.sh` against the deployed control plane after staging — a broken control plane (or a read-only guard that lets a mutation through) fails the pipeline. |
| **TC-CICD-05** | P1 | — | Inspect `deploy-dashboard.yml`. | Builds the static export, `aws s3 sync … --delete`, and `cloudfront create-invalidation` — CD for the public demo URL. |
| **TC-CICD-06** | P1 | — | Inspect `ci.yml`. | Runs `go build`, `go test ./...`, `go vet`, `gofmt -l`, Docker build of all images, and `terraform validate`/`fmt -check`. |
| **TC-CICD-07** | P1 | — | Inspect `security.yml`. | Runs `govulncheck`, `tfsec`, and `gitleaks`. |
| **TC-CICD-08** | P1 | — | Inspect the OIDC role (`terraform/modules/github-oidc`). | Trust is scoped to `repo:lethingochan27925/hivemind:*`; permissions are least-privilege (ECR, Lambda on `<project>-<env>-*`, dashboard S3 bucket, CloudFront invalidation, TF state) — no blanket admin. |
| **TC-CICD-09** | P2 *live* | main pushed | Push a benign change to main. | CI passes → images build/push → staging applies → smoke passes → canary shifts 10% → alarm OK → promotes to 100%. |
| **TC-CICD-10** | P1 *live* | a deploy that raises errors | Deploy a version that trips the error alarm during the canary window. | Canary detects `ALARM`, moves the alias back to the previous version, and **fails the job** (red). No bad version reaches 100%. |
| **TC-CICD-11** | P2 *live* | `dashboard/**` changed | Push a dashboard change. | `deploy-dashboard` builds + syncs + invalidates; the live URL serves the new build within minutes. |

## Running the hermetic slice

```bash
bash test/integration/pipeline_test.sh
```

Asserts TC-CICD-01 through -07 by inspecting the workflow definitions — no AWS, no credentials. It runs in CI (and locally) and guards against a refactor silently dropping the rollback step, wiring in a static key, or unhooking the smoke gate.

## Why static assertions on a pipeline matter

A CI/CD pipeline is code, and its most dangerous regressions are silent: someone removes the rollback branch, hardcodes a key "just to test", or disables the smoke gate. TC-CICD-02 (no static keys) and TC-CICD-03 (rollback still present) are the two that protect production the most, so they run on every push alongside the unit tests.
