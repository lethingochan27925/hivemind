#!/usr/bin/env bash
set -euo pipefail

log()  { printf '[%s] %s\n' "$(date '+%H:%M:%S')" "$*"; }
ok()   { printf '[%s] OK    %s\n' "$(date '+%H:%M:%S')" "$*"; }
err()  { printf '[%s] ERROR %s\n' "$(date '+%H:%M:%S')" "$*" >&2; exit 1; }

command -v gh >/dev/null || err "gh CLI not found - install from https://cli.github.com"
[ -f .env ] || err ".env not found"

gh auth status >/dev/null 2>&1 || err "gh CLI not authenticated - run 'gh auth login' first"

GITHUB_REPO=$(git config --get remote.origin.url | sed -E 's#^(git@github\.com:|https://github\.com/)##; s#\.git$##')
[ -n "$GITHUB_REPO" ] || err "could not parse GitHub repo from git remote origin url"

log "Target repo: ${GITHUB_REPO}"

set -a
source .env
set +a

[ -n "${DATABASE_URL:-}" ] || err "DATABASE_URL not set in .env"
[ -n "${COCKROACHDB_MCP_ENDPOINT:-}" ] || err "COCKROACHDB_MCP_ENDPOINT not set in .env"

if [ -z "${1:-}" ]; then
  err "usage: $0 <AWS_GITHUB_ACTIONS_ROLE_ARN>  (get it from: terraform output -raw github_actions_role_arn)"
fi
ROLE_ARN="$1"

ALERT_EMAIL_VALUE="${ALERT_EMAIL:-}"
if [ -z "$ALERT_EMAIL_VALUE" ]; then
  err "ALERT_EMAIL not set in .env - add a line like ALERT_EMAIL=you@example.com"
fi

log "Pushing secrets to GitHub"

gh secret set AWS_GITHUB_ACTIONS_ROLE_ARN --body "$ROLE_ARN" --repo "$GITHUB_REPO"
ok "AWS_GITHUB_ACTIONS_ROLE_ARN set"

gh secret set DATABASE_URL --body "$DATABASE_URL" --repo "$GITHUB_REPO"
ok "DATABASE_URL set"

gh secret set COCKROACHDB_MCP_ENDPOINT --body "$COCKROACHDB_MCP_ENDPOINT" --repo "$GITHUB_REPO"
ok "COCKROACHDB_MCP_ENDPOINT set"

gh secret set ALERT_EMAIL --body "$ALERT_EMAIL_VALUE" --repo "$GITHUB_REPO"
ok "ALERT_EMAIL set"

log "All secrets pushed"
gh secret list --repo "$GITHUB_REPO"
