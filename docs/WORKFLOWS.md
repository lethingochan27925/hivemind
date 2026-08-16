# Workflows

Every routine on this project is one `make` target. This page lists all of them,
grouped by the situation you are in. Run `make` with no arguments to get the same
list from the terminal.

Each target loads credentials from `.env` and resolves infrastructure URLs and IDs
from Terraform outputs at call time — nothing is hardcoded, so the commands keep
working after the stack is destroyed and rebuilt.

## Prerequisites

| Need | Used by |
|------|---------|
| Go 1.25+ | `build test fmt vet fuzz eval gen-data` |
| Node 22+ | `lint ui-build dev deploy-ui` |
| Docker | `deploy deploy-api init` (Lambda container images) |
| AWS CLI v2 + credentials in `.env` | anything that touches the cloud |
| Terraform 1.6+ | `init tf-plan tf-apply destroy iam` |
| A CockroachDB Cloud cluster (`DATABASE_URL` in `.env`) | the whole system |

`make build test fmt vet lint fuzz pipeline-test ui-build gen-data gen-edge` are
hermetic — they need no cloud account and no database.

## 1. First run on a new machine

```bash
cp .env.example .env      # fill in AWS keys + DATABASE_URL
make init
```

`make init` provisions everything from zero in one command: Terraform apply, build
and push seven Lambda container images to ECR, deploy them behind a `live` alias,
build the dashboard, sync it to S3, invalidate CloudFront, and print the URLs.

## 2. Daily development

| Situation | Command |
|-----------|---------|
| Compile everything | `make build` |
| Run the test suite (no cloud needed) | `make test` |
| See each case as it runs | `make test-v` |
| Format Go sources | `make fmt` |
| Static analysis | `make vet` |
| Lint the dashboard | `make lint` |
| **Gate before committing** | `make check` |

`make check` runs build + vet + test + pipeline invariants + a `gofmt` diff check.
It mirrors what CI enforces, so a green `check` means a green pipeline.

## 3. Deploying

| Situation | Command |
|-----------|---------|
| One Lambda changed | `make deploy S=agent-worker` |
| Control plane changed (most common) | `make deploy-api` |
| All six Go services changed | `make deploy` |
| Dashboard changed | `make deploy-ui` |
| Everything, verified | `make ship` |

`make ship` is `check` + `deploy` + `deploy-ui`. Use it before a demo.

Service names for `S=`: `agent-worker`, `dispatcher`, `reaper`,
`salience-decay`, `scoring-api`, `dashboard-api` (`scoring-python` deploys
separately — see `scripts/deploy-lambda.sh scoring-python`).

## 4. Operating the fleet

```bash
make urls           # dashboard + API URLs of the running stack
make start          # resume the fleet
make feed N=100     # queue 100 cases
make dispatch       # trigger one dispatch cycle instead of waiting for the schedule
make status         # fleet state and queue depth
make stop           # pause the fleet (stops spend when not demoing)
```

## 5. Diagnosing a problem

```bash
make logs S=agent-worker
make smoke
make sql Q="SELECT status, COUNT(*) FROM tasks GROUP BY 1"
```

`make sql` goes through the control plane's read-only endpoint. A word-boundary
guard rejects any mutating statement, so this is safe to hand to anyone.

## 6. Measurement and evidence

```bash
make eval                    # scorecard to stdout
make scorecard               # + write evidence/SCORECARD.md and scorecard.json
make evidence                # capture SQL results, logs and state into evidence/ and S3
make evidence L=crash-test   # label the capture for a specific scenario
make experiment N=60 M=8     # controlled A/B on episodic memory
make memory-restore          # undo an experiment's cold start: unarchive everything
make regions                 # multi-region status of the database
```

`make eval` reads only the read-only control endpoint — it needs no AWS
credentials, so a judge can reproduce the scorecard against the live URL.

## 7. Generating test data

```bash
make gen-data N=1000 SEED=42   # labelled, deterministic, PaySim-compatible
make gen-edge                  # adversarial rows: injection, unicode, arithmetic edges
```

## 8. Infrastructure

```bash
make tf-plan       # preview
make tf-apply      # apply
make iam           # re-apply only the dashboard-api IAM policy (fast path when adding a permission)
make unlock        # break a stale Terraform state lock (only if it's genuinely stuck, >20 minutes old)
make destroy       # tear down AWS resources; the CockroachDB cluster is left alone
```

`make destroy` empties versioned S3 buckets (evidence, dashboard, artifacts) before
deleting them — S3 refuses to drop a bucket that still holds object versions or
delete markers.

## 9. Running the dashboard locally

```bash
make dev           # localhost:3000, pointed at the real deployed API
make ui-build      # static build, no deploy
```

## 10. Heavier checks before submitting

```bash
make fuzz          # three fuzz targets, 60s each
make pipeline-test # 100+ CI/CD invariants, no cloud needed (exact count: run it)
```

## The three sequences worth memorising

```bash
# rebuild from nothing
make init && make start && make feed N=200

# ordinary change
make check && make deploy-api

# before submitting
make ship && make smoke && make scorecard && make evidence
```
