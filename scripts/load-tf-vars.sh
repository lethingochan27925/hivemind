#!/bin/bash
set -a
source .env
set +a

export TF_VAR_cockroachdb_connection_string="$DATABASE_URL"
export TF_VAR_cockroachdb_mcp_endpoint="$COCKROACHDB_MCP_ENDPOINT"
export TF_VAR_alert_email="${ALERT_EMAIL:-your@email.com}"

echo "Terraform vars loaded from .env"
