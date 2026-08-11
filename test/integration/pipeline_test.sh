#!/usr/bin/env bash
# pipeline_test.sh — hermetic assertions on the CI/CD pipeline definitions.
# No cloud, no credentials: it inspects .github/workflows/*.yml (and the OIDC
# module) for the invariants the DevOps design promises — OIDC-only auth, a
# canary with a real rollback branch, the smoke gate wired in, dashboard CD, and
# the security scans. Guards against a refactor silently breaking any of them.
#
#   bash test/integration/pipeline_test.sh
#
# Exits non-zero if any invariant is violated, so it doubles as a CI gate.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WF="$ROOT/.github/workflows"
OIDC="$ROOT/terraform/modules/github-oidc/main.tf"

pass=0
fail=0
ok()  { printf '  [PASS]  %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf '  [FAIL]  %s\n' "$1"; fail=$((fail + 1)); }

file_exists() { [ -f "$WF/$1" ] && ok "workflow present: $1" || bad "workflow MISSING: $1"; }
has() { grep -Eq "$2" "$WF/$1" 2>/dev/null && ok "$3" || bad "$3"; }

echo "== workflows present =="
for w in ci.yml security.yml terraform-infra.yml build-and-push.yml \
         deploy-staging.yml smoke-test.yml deploy-canary.yml deploy-dashboard.yml; do
  file_exists "$w"
done

echo "== OIDC only — no static AWS keys anywhere =="
if grep -rEq 'aws-access-key-id|AKIA[0-9A-Z]{16}|aws_secret_access_key' "$WF" 2>/dev/null; then
  bad "static AWS keys found in a workflow"
else
  ok "no static AWS keys in any workflow (OIDC only)"
fi
has build-and-push.yml 'id-token: write' "build-and-push uses OIDC"
has deploy-canary.yml 'id-token: write' "deploy-canary uses OIDC"

echo "== canary has both promote AND rollback =="
has deploy-canary.yml 'AdditionalVersionWeights' "canary shifts weighted traffic"
has deploy-canary.yml 'describe-alarms'          "canary reads the CloudWatch alarm"
has deploy-canary.yml 'Promote to 100'           "canary promotes when healthy"
has deploy-canary.yml 'Rollback'                 "canary rolls back on alarm"

echo "== smoke gate wired into CD =="
has smoke-test.yml 'api_smoke\.sh' "smoke-test runs api_smoke.sh"

echo "== dashboard continuous delivery =="
has deploy-dashboard.yml 's3 sync'            "dashboard syncs to S3"
has deploy-dashboard.yml 'create-invalidation' "dashboard invalidates CloudFront"

echo "== CI quality gates =="
has ci.yml 'go test'            "CI runs go test"
has ci.yml 'gofmt'             "CI checks gofmt"
has ci.yml 'terraform validate' "CI validates terraform"

echo "== security scans =="
has security.yml 'govulncheck' "security runs govulncheck"
has security.yml 'tfsec'       "security runs tfsec"
has security.yml 'gitleaks'    "security runs gitleaks"

echo "== OIDC role least-privilege =="
if grep -Fq 'repo:${var.github_repo}:*' "$OIDC" 2>/dev/null; then
  ok "OIDC trust scoped to a specific repo (var.github_repo)"
else
  bad "OIDC trust not scoped to a repo"
fi

echo
echo "  $pass passed, $fail failed"
[ "$fail" -eq 0 ]
