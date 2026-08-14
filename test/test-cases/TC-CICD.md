# TC-CICD — DevOps pipeline test cases

Executed by `test/integration/pipeline_test.sh`, which runs in CI on every push
(`ci.yml`, job *Pipeline invariants*) and locally via `make cicd`. Hermetic: no
cloud account, no credentials, no network.

**107 assertions, 0 failures. 12 of 12 mutation tests caught.**

## Why this suite was rewritten

The previous suite reported 25/25 passing while every deploy on `main` was red.
It asserted only that certain strings appeared somewhere in a file. It could not
see that:

- CI validated Terraform with a CLI two minor versions older than the backend
  syntax required — `terraform validate` runs with `-backend=false`, so the
  backend block is never parsed and the failure only appears in CD;
- the smoke test ran concurrently with the canary instead of gating it;
- the dashboard was never linted or built in CI at all.

Every case below is therefore about **consistency, ordering and reachability**,
not presence.

| ID | Case | Method | Expected |
|---|---|---|---|
| CICD-01 | All eight designed workflows exist, and no undocumented workflow file does | file listing vs the declared set | exact match |
| CICD-02 | `TF_VERSION` is identical across all five workflows that run Terraform | parse workflow `env` | all equal |
| CICD-03 | No workflow hardcodes `terraform_version` inline | pattern scan | none |
| CICD-04 | The pinned Terraform version supports `use_lockfile` | parse backend, compare minor version | pinned >= 1.10 |
| CICD-05 | `required_version` declares the floor the backend actually needs | parse `versions.tf` | `>= 1.10.0` |
| CICD-06 | `GO_VERSION` in every workflow matches `go.mod` | compare | equal |
| CICD-07 | `Dockerfile.lambda-go` builds with the same Go minor version | compare `FROM golang:` | equal |
| CICD-08 | Dashboard CD uses the Node version CI built with | compare | equal |
| CICD-09 | Chain order is Build then Staging then Smoke then Canary | parse `workflow_run.workflows` | exact chain |
| CICD-10 | Canary does not trigger directly on Deploy Staging | pattern scan | absent |
| CICD-11 | Every `workflow_run` consumer gates on `conclusion == 'success'` | pattern scan | present in all three |
| CICD-12 | Every `workflow_run` consumer checks out `head_sha` | scan checkout blocks | pinned |
| CICD-13 | Every job in every workflow declares `timeout-minutes` | count jobs vs timeouts | timeouts >= jobs |
| CICD-14 | Every workflow declares `permissions` explicitly | pattern scan | present |
| CICD-15 | Deploy workflows serialize without cancelling in flight | parse concurrency | `cancel-in-progress: false` |
| CICD-16 | CI cancels superseded runs | parse concurrency | `true` |
| CICD-17 | No static AWS credentials in any workflow | scan, excluding scanner lines | none |
| CICD-18 | Every AWS-touching workflow mints an OIDC token and assumes a role | pattern scan | all six |
| CICD-19 | No workflow echoes a secret | pattern scan | none |
| CICD-20 | No `${{ }}` interpolation inside a `github-script` body | indentation-aware block scan | none |
| CICD-21 | No `pull_request_target` anywhere | pattern scan | absent |
| CICD-22 | Fork pull requests are skipped where secrets are required | pattern scan | guard present |
| CICD-23 | Every referenced secret is documented | set difference | empty |
| CICD-24 | Every documented secret is referenced | set difference | empty |
| CICD-25 | Deploy preflights secrets and names the missing one | pattern scan | present |
| CICD-26 | CI and build-and-push build the identical service set | compare parsed matrices | identical |
| CICD-27 | Every matrix `cmd_path` exists on disk | filesystem check | all exist |
| CICD-28 | Every service the canary promotes is declared in Terraform | compare against `var.services` | subset |
| CICD-29 | Every `docker build` disables provenance and SBOM | count vs count | equal |
| CICD-30 | Canary shifts weighted traffic, reads the alarm, promotes, and rolls back | pattern scan | all four |
| CICD-31 | A missing alarm fails instead of promoting | pattern scan | present |
| CICD-32 | Canary is a no-op when the alias already serves the newest version | pattern scan | present |
| CICD-33 | Canary filters `$LATEST` before `to_number` | pattern scan | filtered |
| CICD-34 | CI tests every package including `./cmd` | pattern scan | `go test ./...` |
| CICD-35 | CI lints and builds the dashboard | pattern scan | both |
| CICD-36 | CI runs gofmt, terraform validate, terraform fmt and actionlint | pattern scan | all four |
| CICD-37 | CI runs this invariant suite | pattern scan | present |
| CICD-38 | Smoke gate runs the control-plane suite, and that script exists | pattern and file check | both |
| CICD-39 | Dashboard CD injects the API URL and verifies it landed in the bundle | pattern scan | both |
| CICD-40 | Security runs govulncheck, gitleaks and Trivy; tfsec is not invoked | pattern scan | three present, one absent |
| CICD-41 | govulncheck is pinned to a version | pattern scan | `@v` present |
| CICD-42 | IaC findings reach the Security tab; only CRITICAL blocks the build | pattern scan | both |
| CICD-43 | OIDC trust is scoped to one repository | parse the OIDC module | scoped |
| CICD-44 | Terraform state is encrypted at rest | parse backend | `encrypt = true` |
| CICD-45 | `.gitignore` blocks `.env`, `*.tfvars`, `*.tfstate` | pattern scan | all three |
| CICD-46 | Every workflow is valid YAML | parse with PyYAML | all parse |

## Mutation testing

A suite that passes proves nothing until it has been shown to fail. Each defect
below was injected into a copy of the tree and the suite re-run. All twelve were
caught. The first three had actually reached `main`.

| # | Injected defect | Caught |
|---|---|---|
| 1 | `TF_VERSION` reverted to `1.9.0` — the real outage | yes |
| 2 | Canary triggers on Deploy Staging, bypassing the smoke gate | yes |
| 3 | `npx eslint` removed from CI — how five lint errors shipped | yes |
| 4 | `--provenance=false --sbom=false` dropped from one `docker build` | yes |
| 5 | `timeout-minutes` removed from one job | yes |
| 6 | A matrix `cmd_path` pointing at a directory that does not exist | yes |
| 7 | A literal `AKIA...` access key added to a workflow | yes |
| 8 | `max_by` run over unfiltered `Versions` | yes |
| 9 | `checkout` no longer pinning `head_sha` | yes |
| 10 | CI matrix renaming a service, drifting from build-and-push | yes |
| 11 | `permissions` block removed from `security.yml` | yes |
| 12 | tfsec reintroduced | yes |

Reproduce one:

```bash
cp -r . /tmp/mutant && cd /tmp/mutant
sed -i "s/TF_VERSION: '1.14.3'/TF_VERSION: '1.9.0'/" .github/workflows/deploy-staging.yml
bash test/integration/pipeline_test.sh   # expected: non-zero exit
```

## Defects this suite found on its first run

| Finding | Severity | Fix |
|---|---|---|
| `terraform_version: 1.9.0` cannot parse `use_lockfile` | blocker — every deploy red | pinned to the version that writes the state |
| `required_version = ">= 1.7.0"` understated the floor | latent — a 1.7 user gets a confusing error | raised to `>= 1.10.0` |
| Smoke test did not gate the canary | severe — traffic could shift before any check ran | canary retriggered on Smoke Test |
| `golang.org/x/text v0.29.0`, GO-2026-5970, reachable from `cockroach.NewClient` | high | upgraded to v0.39.0 |
| tfsec archived and panicking on this configuration | blocker — Security red | replaced with Trivy |
| Dashboard never linted or built in CI | high | added to `ci.yml` |
| Plan output interpolated into a `github-script` body | high — code injection | passed through `env` |
| No job had a timeout; no deploy had a concurrency group | medium | added to all |
