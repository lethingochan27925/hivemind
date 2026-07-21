"""
mcp_client.py — HiveMind MCP Client
Goi CockroachDB Managed MCP Server (read-only) de lay context cho agent.
3 tools: get_transaction, get_customer_context, search_similar_cases
"""

import os
import json
import urllib.request
import urllib.error
from dotenv import load_dotenv

load_dotenv(".env")

MCP_ENDPOINT  = os.environ.get("COCKROACHDB_MCP_ENDPOINT", "https://cockroachlabs.cloud/mcp")
MCP_API_KEY   = os.environ["COCKROACHDB_MCP_API_KEY"]
CLUSTER_ID    = os.environ["COCKROACHDB_CLUSTER_ID"]
DATABASE      = "hivemind"


def _call_mcp(method: str, params: dict, req_id: int = 1) -> dict:
    payload = json.dumps({
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
        "id": req_id,
    }).encode()

    req = urllib.request.Request(
        MCP_ENDPOINT,
        data=payload,
        headers={
            "Authorization": f"Bearer {MCP_API_KEY}",
            "Content-Type": "application/json",
        },
        method="POST",
    )

    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            raw = resp.read().decode()
            for line in raw.splitlines():
                if line.startswith("data:"):
                    return json.loads(line[5:].strip())
            return json.loads(raw)
    except urllib.error.URLError as e:
        raise RuntimeError(f"MCP request failed: {e}")


def _select(query: str) -> list[dict]:
    result = _call_mcp("tools/call", {
        "name": "select_query",
        "arguments": {
            "cluster_id": CLUSTER_ID,
            "database": DATABASE,
            "query": query,
        },
    })

    if "error" in result:
        raise RuntimeError(f"MCP error: {result['error']}")

    content = result.get("result", {}).get("content", [])
    for block in content:
        if block.get("type") == "text":
            try:
                return json.loads(block["text"])
            except json.JSONDecodeError:
                return [{"raw": block["text"]}]
    return []


def get_transaction(transaction_id: str) -> dict | None:
    rows = _select(f"""
        SELECT
            id, step, type, amount,
            name_orig, old_balance_orig, new_balance_orig,
            name_dest, old_balance_dest, new_balance_dest,
            error_balance_orig, error_balance_dest,
            risk_score, risk_tier, is_fraud_label,
            arrived_at
        FROM transactions
        WHERE id = '{transaction_id}'
        LIMIT 1
    """)
    return rows[0] if rows else None


def get_customer_context(name_orig: str, limit: int = 5) -> list[dict]:
    rows = _select(f"""
        SELECT
            t.id,
            t.type,
            t.amount,
            t.risk_score,
            t.risk_tier,
            t.arrived_at,
            tk.verdict,
            tk.confidence
        FROM transactions t
        LEFT JOIN tasks tk ON tk.transaction_id = t.id
        WHERE t.name_orig = '{name_orig}'
        ORDER BY t.arrived_at DESC
        LIMIT {limit}
    """)
    return rows


def search_similar_cases(
    transaction_type: str,
    amount_range: str,
    verdict_filter: str = None,
    limit: int = 3,
) -> list[dict]:
    verdict_clause = ""
    if verdict_filter:
        verdict_clause = f"AND verdict = '{verdict_filter}'"

    rows = _select(f"""
        SELECT
            id, summary, verdict, confidence_avg,
            pattern_type, key_signals,
            salience, recall_count
        FROM case_memory
        WHERE archived = false
          AND transaction_type = '{transaction_type}'
          AND amount_range = '{amount_range}'
          {verdict_clause}
        ORDER BY salience DESC, recall_count DESC
        LIMIT {limit}
    """)
    return rows


def list_tools() -> list:
    result = _call_mcp("tools/list", {})
    return [t["name"] for t in result.get("result", {}).get("tools", [])]


if __name__ == "__main__":
    print("Testing MCP connection...")
    tools = list_tools()
    print(f"Available tools: {tools}")

    print("\nTesting get_transaction with dummy ID...")
    txn = get_transaction("00000000-0000-0000-0000-000000000000")
    print(f"Result: {txn}")

    print("\nMCP client OK")
