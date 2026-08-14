#!/usr/bin/env bash
# pipeline_test.sh — hermetic assertions on the CI/CD pipeline definitions.
#
#   bash test/integration/pipeline_test.sh
#
# No cloud, no credentials, no network. It reads .github/workflows/*.yml, the
# Terraform sources, go.mod, the Dockerfiles and the filesystem, and asserts the
# invariants the delivery design depends on.
#
# WHY THIS FILE IS SHAPED THIS WAY
# The previous version of this suite passed 25/25 while the deploy chain was red
# on every push. It only asserted that certain strings appeared somewhere. It
# could not see that CI validated Terraform with a CLI two minor versions older
# than the backend syntax required, that the smoke test ran in parallel with the
# canary rather than before it, or that the dashboard was never linted at all.
#
# So the assertions here are about CONSISTENCY and ORDER, not presence:
#   - the same tool version everywhere, and compatible with what the code needs
#   - the delivery chain triggers in the intended sequence
#   - matrices match the filesystem and each other
#   - every job is bounded, permissioned, and serialized where it mutates
#   - secrets never reach a log, a fork, or a JavaScript template
#
# Exits non-zero on the first violated invariant class, so it doubles as a CI job.

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WF="$ROOT/.github/workflows"
TFDIR="$ROOT/terraform"
OIDC="$TFDIR/modules/github-oidc/main.tf"

pass=0
fail=0
ok()   { printf '  [PASS]  %s\n' "$1"; pass=$((pass + 1)); }
bad()  { printf '  [FAIL]  %s\n' "$1"; fail=$((fail + 1)); }
note() { printf '  [ .. ]  %s\n' "$1"; }

# has FILE REGEX DESC — the pattern must appear
has() {
  if grep -Eq "$2" "$WF/$1" 2>/dev/null; then ok "$3"; else bad "$3"; fi
}
# hasnt FILE REGEX DESC — the pattern must NOT appear
hasnt() {
  if grep -Eq "$2" "$WF/$1" 2>/dev/null; then bad "$3"; else ok "$3"; fi
}
eq() {
  if [ "$1" = "$2" ]; then ok "$3"; else bad "$3 (got '$1', want '$2')"; fi
}

ALL_WF="ci.yml security.yml terraform-infra.yml build-and-push.yml
        deploy-staging.yml smoke-test.yml deploy-canary.yml deploy-dashboard.yml"
# The workflows that touch AWS and therefore need OIDC + serialization.
DEPLOY_WF="build-and-push.yml deploy-staging.yml smoke-test.yml
           deploy-canary.yml deploy-dashboard.yml terraform-infra.yml"

# Count job keys (two-space indent) inside the jobs: block.
count_jobs() {
  awk '
    /^jobs:/    { injobs = 1; next }
    injobs && /^[A-Za-z]/ { injobs = 0 }
    injobs && /^  [A-Za-z_][A-Za-z0-9_-]*:[[:space:]]*$/ { n++ }
    END { print n + 0 }
  ' "$1"
}

# Read a workflow-level `env:` value, e.g. env_of ci.yml TF_VERSION
env_of() {
  grep -E "^  $2:" "$WF/$1" 2>/dev/null | head -1 |
    sed -E "s/^  $2:[[:space:]]*//; s/^['\"]//; s/['\"]$//"
}

# ---------------------------------------------------------------------------
echo "== 1. every workflow of the design is present =="
for w in $ALL_WF; do
  [ -f "$WF/$w" ] && ok "present: $w" || bad "MISSING: $w"
done
extra=$(ls "$WF" 2>/dev/null | grep -E '\.ya?ml$' | grep -vE "$(echo "$ALL_WF" | tr -s ' \n' '|' | sed 's/|$//')" || true)
[ -z "$extra" ] && ok "no undocumented workflow files" || bad "undocumented workflows: $extra"

# ---------------------------------------------------------------------------
echo
echo "== 2. tool versions agree across every workflow =="
# A Terraform CLI older than the backend syntax is exactly the failure that took
# the deploy chain down: ci.yml validated fine with -backend=false while every
# real init died on `use_lockfile`.
TFV_CI=$(env_of ci.yml TF_VERSION)
if [ -z "$TFV_CI" ]; then
  bad "ci.yml declares no TF_VERSION"
else
  ok "ci.yml pins TF_VERSION=$TFV_CI"
  for w in deploy-staging.yml smoke-test.yml deploy-dashboard.yml terraform-infra.yml; do
    eq "$(env_of "$w" TF_VERSION)" "$TFV_CI" "$w pins the same TF_VERSION"
  done
fi

# No workflow may pin a literal terraform_version — it must come from env.
for w in $ALL_WF; do
  if grep -Eq "terraform_version:[[:space:]]*['\"]?[0-9]" "$WF/$w" 2>/dev/null; then
    bad "$w hardcodes terraform_version instead of using \${{ env.TF_VERSION }}"
  fi
done
ok "no workflow hardcodes a Terraform version inline"

# use_lockfile is S3 native locking: Terraform 1.10+.
if grep -q 'use_lockfile' "$TFDIR/versions.tf" 2>/dev/null; then
  note "terraform backend uses use_lockfile (needs Terraform >= 1.10)"
  TF_MAJOR=${TFV_CI%%.*}; TF_REST=${TFV_CI#*.}; TF_MINOR=${TF_REST%%.*}
  if [ "${TF_MAJOR:-0}" -gt 1 ] || { [ "${TF_MAJOR:-0}" -eq 1 ] && [ "${TF_MINOR:-0}" -ge 10 ]; }; then
    ok "pinned Terraform $TFV_CI supports use_lockfile"
  else
    bad "pinned Terraform $TFV_CI is too old for use_lockfile (needs >= 1.10)"
  fi
  if grep -Eq 'required_version[[:space:]]*=[[:space:]]*">=[[:space:]]*1\.(1[0-9]|[2-9][0-9])' "$TFDIR/versions.tf"; then
    ok "required_version declares the 1.10+ floor that use_lockfile needs"
  else
    bad "required_version understates the floor — it allows a CLI that cannot parse the backend"
  fi
fi

# Go version: go.mod is the source of truth.
GOMOD_VER=$(grep -E '^go [0-9]' "$ROOT/go.mod" | awk '{print $2}')
GOMOD_MM=$(printf '%s' "$GOMOD_VER" | cut -d. -f1,2)
eq "$(env_of ci.yml GO_VERSION)" "$GOMOD_MM" "ci.yml GO_VERSION matches go.mod ($GOMOD_VER)"
eq "$(env_of security.yml GO_VERSION)" "$GOMOD_MM" "security.yml GO_VERSION matches go.mod"
eq "$(env_of deploy-staging.yml GO_VERSION)" "$GOMOD_MM" "deploy-staging.yml GO_VERSION matches go.mod"
if grep -Eq "^FROM golang:$GOMOD_MM" "$ROOT/Dockerfile.lambda-go" 2>/dev/null; then
  ok "Dockerfile.lambda-go builds with golang:$GOMOD_MM"
else
  bad "Dockerfile.lambda-go does not build with golang:$GOMOD_MM"
fi

eq "$(env_of deploy-dashboard.yml NODE_VERSION)" "$(env_of ci.yml NODE_VERSION)" \
   "dashboard CD uses the same Node version CI built with"

# ---------------------------------------------------------------------------
echo
echo "== 3. the delivery chain triggers in order =="
# Build -> Staging -> Smoke -> Canary. The canary must depend on the smoke test,
# not race it: both used to trigger on "Deploy Staging" and ran concurrently, so
# traffic could shift before the control plane had been checked at all.
has deploy-staging.yml 'workflows:[[:space:]]*\["Build and Push Images"\]' \
    "Deploy Staging triggers on Build and Push Images"
has smoke-test.yml 'workflows:[[:space:]]*\["Deploy Staging"\]' \
    "Smoke Test triggers on Deploy Staging"
has deploy-canary.yml 'workflows:[[:space:]]*\["Smoke Test"\]' \
    "Deploy Canary triggers on Smoke Test (smoke gates the canary)"
hasnt deploy-canary.yml 'workflows:[[:space:]]*\["Deploy Staging"\]' \
    "Deploy Canary does NOT bypass the smoke gate"

# Every workflow_run consumer must check the upstream conclusion...
for w in deploy-staging.yml smoke-test.yml deploy-canary.yml; do
  has "$w" "workflow_run\.conclusion == 'success'" "$w only runs after a successful upstream"
done
# ...and check out the commit that was built, not whatever main points at now.
for w in deploy-staging.yml smoke-test.yml; do
  if grep -A3 'uses: actions/checkout' "$WF/$w" | grep -q 'workflow_run.head_sha'; then
    ok "$w checks out the built commit (head_sha), not moving main"
  else
    bad "$w checks out main — it can deploy a different tree than was built"
  fi
done

# ---------------------------------------------------------------------------
echo
echo "== 4. every job is bounded and least-privileged =="
for w in $ALL_WF; do
  [ -f "$WF/$w" ] || continue
  jobs=$(count_jobs "$WF/$w")
  timeouts=$(grep -c '^[[:space:]]*timeout-minutes:' "$WF/$w")
  if [ "$timeouts" -ge "$jobs" ] && [ "$jobs" -gt 0 ]; then
    ok "$w: all $jobs job(s) have timeout-minutes"
  else
    bad "$w: $jobs job(s) but only $timeouts timeout-minutes"
  fi
  grep -Eq '^permissions:' "$WF/$w" && ok "$w declares explicit permissions" \
                                    || bad "$w has no permissions block (inherits broad defaults)"
done

# Deploys must serialize; a cancelled apply or a cancelled `s3 sync --delete`
# leaves the system in a state nobody described.
for w in deploy-staging.yml deploy-canary.yml deploy-dashboard.yml build-and-push.yml; do
  if grep -Eq '^concurrency:' "$WF/$w"; then
    if grep -A2 '^concurrency:' "$WF/$w" | grep -q 'cancel-in-progress: false'; then
      ok "$w serializes deploys without cancelling in flight"
    else
      bad "$w cancels an in-flight deploy"
    fi
  else
    bad "$w has no concurrency group — two deploys can interleave"
  fi
done
if grep -A2 '^concurrency:' "$WF/ci.yml" | grep -q 'cancel-in-progress: true'; then
  ok "ci.yml cancels superseded runs (cheap, safe for a read-only job)"
else
  bad "ci.yml should cancel superseded runs"
fi

# ---------------------------------------------------------------------------
echo
echo "== 5. secrets never leak =="
# Lines that are themselves scanners are excluded, otherwise security.yml's own
# secret-hygiene patterns would trip this.
if grep -rEn 'aws-access-key-id:|AKIA[0-9A-Z]{16}|aws_secret_access_key[[:space:]]*[:=]' "$WF" 2>/dev/null \
   | grep -v 'grep ' | grep -q .; then
  bad "a static AWS key appears in a workflow"
else
  ok "no static AWS credentials anywhere — OIDC only"
fi
for w in $DEPLOY_WF; do
  has "$w" 'id-token: write' "$w mints an OIDC token"
  has "$w" 'aws-actions/configure-aws-credentials@v4' "$w assumes a role rather than using keys"
done

if grep -rEnq 'echo[^|]*\$\{\{[[:space:]]*secrets\.' "$WF" 2>/dev/null; then
  bad "a workflow echoes a secret into the log"
else
  ok "no workflow echoes a secret"
fi

# `${{ }}` inside a github-script body is substituted before the JS is parsed,
# so any interpolated value is executed as code.
if awk '
      /script:[[:space:]]*\|/ { inscript = 1; ind = match($0, /[^ ]/); next }
      inscript {
        if ($0 ~ /^[[:space:]]*$/) next
        if (match($0, /[^ ]/) <= ind) { inscript = 0; next }
        if ($0 ~ /\$\{\{/) found = 1
      }
      END { exit !found }' "$WF/terraform-infra.yml"; then
  bad "terraform-infra.yml interpolates \${{ }} inside a github-script body (code injection)"
else
  ok "github-script bodies read from env, never from \${{ }} interpolation"
fi

hasnt terraform-infra.yml 'pull_request_target' \
    "no workflow uses pull_request_target (which would expose secrets to fork PRs)"
has terraform-infra.yml 'head\.repo\.full_name == github\.repository' \
    "terraform-infra skips fork PRs, which cannot access OIDC or secrets"

# Every secret referenced must be one we document, and each documented secret
# must actually be referenced — an unused secret is a stale credential.
DOCUMENTED="AWS_GITHUB_ACTIONS_ROLE_ARN DATABASE_URL COCKROACHDB_MCP_ENDPOINT
            COCKROACHDB_MCP_API_KEY COCKROACHDB_CLUSTER_ID ALERT_EMAIL GITHUB_TOKEN"
USED=$(grep -rhoE 'secrets\.[A-Z_]+' "$WF" | sed 's/secrets\.//' | sort -u)
unknown=""
for s in $USED; do
  echo "$DOCUMENTED" | tr -s ' \n' '\n' | grep -qx "$s" || unknown="$unknown $s"
done
[ -z "$unknown" ] && ok "every referenced secret is documented" \
                  || bad "undocumented secrets referenced:$unknown"
unused=""
for s in $DOCUMENTED; do
  echo "$USED" | grep -qx "$s" || unused="$unused $s"
done
[ -z "$unused" ] && ok "every documented secret is actually used" \
                 || bad "documented but unused secrets:$unused"

has deploy-staging.yml 'is not configured' \
    "deploy-staging preflights the secrets and names the missing one"

# ---------------------------------------------------------------------------
echo
echo "== 6. build matrices match each other and the filesystem =="
matrix_pairs() {
  awk '/cmd_path:/ {gsub(/.*cmd_path:[[:space:]]*/, ""); path=$0}
       /- service:/ {gsub(/.*- service:[[:space:]]*/, ""); svc=$0}
       path != "" && svc != "" {print svc "=" path; svc=""; path=""}' "$WF/$1" | sort
}
CI_M=$(matrix_pairs ci.yml)
BP_M=$(matrix_pairs build-and-push.yml)
if [ -n "$CI_M" ] && [ "$CI_M" = "$BP_M" ]; then
  ok "ci.yml and build-and-push.yml build the identical service set"
else
  bad "CI and build-and-push matrices have drifted apart"
fi
missing=""
for pair in $BP_M; do
  p=${pair#*=}
  [ -d "$ROOT/${p#./}" ] || missing="$missing $pair"
done
[ -z "$missing" ] && ok "every matrix cmd_path exists on disk" \
                  || bad "matrix points at directories that do not exist:$missing"

# Everything the canary promotes must be something Terraform actually manages.
TF_SERVICES=$(grep -A4 'variable "services"' "$TFDIR/variables.tf" | grep 'default' |
              grep -oE '"[a-z-]+"' | tr -d '"' | sort)
CANARY_SERVICES=$(awk '/service: \[/ {gsub(/.*\[|\].*/, ""); gsub(/,/, "\n"); print}' \
                  "$WF/deploy-canary.yml" | tr -d ' ' | grep -v '^$' | sort -u)
missing=""
for s in $CANARY_SERVICES; do
  echo "$TF_SERVICES" | grep -qx "$s" || missing="$missing $s"
done
[ -z "$missing" ] && ok "every service the canary deploys is declared in terraform" \
                  || bad "canary references services terraform does not manage:$missing"

# Rebuilding this image without these flags produces a manifest Lambda rejects.
builds=$(grep -rc 'docker build' "$WF" 2>/dev/null | awk -F: '{s+=$2} END{print s+0}')
flagged=$(grep -rc 'docker build --provenance=false --sbom=false' "$WF" 2>/dev/null | awk -F: '{s+=$2} END{print s+0}')
eq "$flagged" "$builds" "all $builds docker build invocations disable provenance and SBOM"

# ---------------------------------------------------------------------------
echo
echo "== 7. the canary can actually fail =="
has deploy-canary.yml 'AdditionalVersionWeights' "canary shifts weighted traffic"
has deploy-canary.yml 'describe-alarms'          "canary reads the CloudWatch alarm"
has deploy-canary.yml 'Promote to 100'           "canary promotes when healthy"
has deploy-canary.yml 'Roll back'                "canary has a rollback path"
has deploy-canary.yml 'does not exist'           "a missing alarm fails instead of promoting"
has deploy-canary.yml "changed=false"            "canary is a no-op when the alias already serves the newest version"
# to_number("$LATEST") is null and max_by then errors — the filter is required.
if grep -q 'max_by(Versions,' "$WF/deploy-canary.yml"; then
  bad "canary runs max_by over unfiltered Versions (\$LATEST breaks to_number)"
else
  ok "canary filters \$LATEST before selecting the newest version"
fi

# ---------------------------------------------------------------------------
echo
echo "== 8. CI covers what has actually broken before =="
has ci.yml 'go test \./\.\.\.'    "CI tests every package, including ./cmd (duplicate routes panic at init)"
has ci.yml 'eslint'               "CI lints the dashboard"
has ci.yml 'npm run build'        "CI builds the dashboard static export"
has ci.yml 'gofmt'                "CI checks gofmt"
has ci.yml 'terraform validate'   "CI validates terraform"
has ci.yml 'terraform fmt -check' "CI checks terraform formatting"
has ci.yml 'actionlint'           "CI lints the workflow definitions themselves"
has ci.yml 'pipeline_test\.sh'    "CI runs these invariants on every push"
has smoke-test.yml 'api_smoke\.sh' "the smoke gate runs the control-plane suite"
[ -x "$ROOT/test/integration/api_smoke.sh" ] || [ -f "$ROOT/test/integration/api_smoke.sh" ] \
  && ok "api_smoke.sh exists" || bad "api_smoke.sh is missing"

# The dashboard bakes its API base URL at build time; a fallback to localhost
# renders a page where every panel says "Failed to fetch".
has deploy-dashboard.yml 'NEXT_PUBLIC_DASHBOARD_API_URL' "dashboard CD injects the API URL at build"
has deploy-dashboard.yml 'does not contain'              "dashboard CD verifies the URL was baked in before publishing"

# ---------------------------------------------------------------------------
echo
echo "== 9. security scanning is present and correctly graded =="
has security.yml 'govulncheck'  "security scans Go dependencies"
has security.yml 'gitleaks'     "security scans for committed secrets"
has security.yml 'trivy'        "security scans the Terraform configuration"
hasnt security.yml '(uses:|run:).*tfsec' "tfsec is no longer invoked (archived upstream; it panics on this config)"
has security.yml 'govulncheck@v' "govulncheck is pinned, so a new CVE is news and not a mystery"
has security.yml 'upload-sarif'  "IaC findings reach the Security tab"
has security.yml 'severity CRITICAL --exit-code 1' "only CRITICAL IaC findings block the build"

# ---------------------------------------------------------------------------
echo
echo "== 10. repository hygiene =="
if grep -Fq 'repo:${var.github_repo}:*' "$OIDC" 2>/dev/null; then
  ok "OIDC trust is scoped to one repository"
else
  bad "OIDC trust is not scoped to a repository"
fi
grep -Eq 'encrypt[[:space:]]*=[[:space:]]*true' "$TFDIR/versions.tf" \
  && ok "terraform state is encrypted at rest" || bad "terraform state is not encrypted"
for pat in '.env' '*.tfvars' '*.tfstate'; do
  grep -qF -- "$pat" "$ROOT/.gitignore" && ok ".gitignore blocks $pat" || bad ".gitignore does not block $pat"
done

# ---------------------------------------------------------------------------
echo
echo "== 11. every workflow is valid YAML =="
if python3 -c 'import yaml' 2>/dev/null; then
  for w in $ALL_WF; do
    if python3 -c "import yaml,sys; yaml.safe_load(open(sys.argv[1]))" "$WF/$w" 2>/dev/null; then
      ok "valid YAML: $w"
    else
      bad "INVALID YAML: $w"
    fi
  done
else
  note "PyYAML unavailable — skipped (actionlint covers this in CI)"
fi

echo
echo "  $pass passed, $fail failed"
[ "$fail" -eq 0 ]
