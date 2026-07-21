"""
mcp_client.py — HiveMind MCP Client
Wrapper cho CockroachDB Managed MCP Server.
Expose 3 read-only tools cho agent_loop su dung.
"""

import os
import json
import urllib.request
import urllib.error
import logging
from dataclasses import dataclass
from typing import Any
from dotenv import load_dotenv

load_dotenv(".env")

log = logging.getLogger(__name__)

MCP_PROTOCOL_VERSION = "2024-11-05"
MCP_CLIENT_INFO = {"name": "hivemind-agent", "version": "1.0"}
MCP_DEFAULT_LIMIT = 25


@dataclass
class MCPConfig:
    endpoint: str
    api_key: str
    cluster_id: str
    database: str
    timeout: int = 30

    @classmethod
    def from_env(cls) -> "MCPConfig":
        return cls(
            endpoint=os.environ.get(
                "COCKROACHDB_MCP_ENDPOINT", "https://cockroachlabs.cloud/mcp"
            ),
            api_key=os.environ["COCKROACHDB_MCP_API_KEY"],
            cluster_id=os.environ["COCKROACHDB_CLUSTER_ID"],
            database=os.environ.get("COCKROACHDB_DATABASE", "hivemind"),
            timeout=int(os.environ.get("MCP_TIMEOUT_SECONDS", "30")),
        )


class MCPError(Exception):
    pass


class MCPClient:
    def __init__(self, config: MCPConfig):
        self.config = config
        self._session_id: str | None = None

    def _base_headers(self) -> dict:
        headers = {
            "Authorization": f"Bearer {self.config.api_key}",
            "Content-Type": "application/json",
            "Accept": "application/json, text/event-stream",
        }
        if self._session_id:
            headers["Mcp-Session-Id"] = self._session_id
        return headers

    def _post(self, payload: dict) -> dict:
        req = urllib.request.Request(
            self.config.endpoint,
            data=json.dumps(payload).encode(),
            headers=self._base_headers(),
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=self.config.timeout) as resp:
                sid = resp.headers.get("Mcp-Session-Id")
                if sid:
                    self._session_id = sid
                raw = resp.read().decode()
                for line in raw.splitlines():
                    if line.startswith("data:"):
                        return json.loads(line[5:].strip())
                return json.loads(raw)
        except urllib.error.HTTPError as e:
            body = e.read().decode()
            raise MCPError(f"HTTP {e.code}: {body}")
        except urllib.error.URLError as e:
            raise MCPError(f"Connection error: {e}")

    def _jsonrpc(self, method: str, params: dict, req_id: int = 1) -> dict:
        return self._post({
            "jsonrpc": "2.0",
            "method": method,
            "params": params,
            "id": req_id,
        })

    def initialize(self) -> None:
        self._session_id = None
        result = self._jsonrpc("initialize", {
            "protocolVersion": MCP_PROTOCOL_VERSION,
            "capabilities": {},
            "clientInfo": MCP_CLIENT_INFO,
        })
        if "error" in result:
            raise MCPError(f"Init failed: {result['error']}")
        log.debug("MCP session initialized: %s", self._session_id)

    def list_tools(self) -> list[str]:
        self.initialize()
        result = self._jsonrpc("tools/list", {}, req_id=2)
        return [t["name"] for t in result.get("result", {}).get("tools", [])]

    def call_tool(self, tool_name: str, arguments: dict) -> Any:
        result = self._jsonrpc("tools/call", {
            "name": tool_name,
            "arguments": arguments,
        }, req_id=2)

        if "error" in result:
            raise MCPError(f"Tool '{tool_name}' error: {result['error']}")

        content = result.get("result", {}).get("content", [])
        texts = []
        for block in content:
            if block.get("type") == "text":
                texts.append(block["text"])

        if not texts:
            return []

        raw_text = "\n".join(texts)
        try:
            parsed = json.loads(raw_text)
        except json.JSONDecodeError:
            return [{"raw": raw_text}]

        # select_query co the tra ve: list truc tiep, hoac dict co key "rows"/"result"
        if isinstance(parsed, list):
            return parsed
        if isinstance(parsed, dict):
            for key in ("rows", "result", "data"):
                if key in parsed and isinstance(parsed[key], list):
                    return parsed[key]
            # dict don le (1 row) -> wrap thanh list
            return [parsed]
        return [{"raw": raw_text}]

    def select(self, query: str, limit: int = MCP_DEFAULT_LIMIT) -> list[dict]:
        self.initialize()

        safe_query = query.strip().rstrip(";")
        if "limit" not in safe_query.lower():
            safe_query = f"{safe_query} LIMIT {limit}"

        return self.call_tool("select_query", {
            "cluster_id": self.config.cluster_id,
            "database": self.config.database,
            "query": safe_query,
        })


class HiveMindMCPTools:
    """
    3 read-only tools cho agent_loop su dung.
    Moi tool la 1 method rieng biet, de mo rong them tool moi.
    """

    def __init__(self, client: MCPClient):
        self.client = client

    def get_transaction(self, transaction_id: str) -> dict | None:
        rows = self.client.select(f"""
            SELECT
                id, step, type, amount,
                name_orig, old_balance_orig, new_balance_orig,
                name_dest, old_balance_dest, new_balance_dest,
                error_balance_orig, error_balance_dest,
                risk_score, risk_tier, is_fraud_label
            FROM transactions
            WHERE id = '{transaction_id}'
        """, limit=1)
        return rows[0] if rows else None

    def get_customer_context(self, name_orig: str, limit: int = 5) -> list[dict]:
        return self.client.select(f"""
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
        """, limit=limit)

    def search_similar_cases(
        self,
        transaction_type: str,
        amount_range: str,
        verdict_filter: str | None = None,
        limit: int = 3,
    ) -> list[dict]:
        verdict_clause = (
            f"AND verdict = '{verdict_filter}'" if verdict_filter else ""
        )
        return self.client.select(f"""
            SELECT
                id,
                summary,
                verdict,
                confidence_avg,
                pattern_type,
                key_signals,
                salience,
                recall_count
            FROM case_memory
            WHERE archived = false
              AND transaction_type = '{transaction_type}'
              AND amount_range = '{amount_range}'
              {verdict_clause}
            ORDER BY salience DESC, recall_count DESC
        """, limit=limit)


def build_mcp_tools() -> HiveMindMCPTools:
    config = MCPConfig.from_env()
    client = MCPClient(config)
    return HiveMindMCPTools(client)


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)

    tools = build_mcp_tools()

    print("Testing MCP connection...")
    available = tools.client.list_tools()
    print(f"Available tools ({len(available)}): {available}")

    print("\nTesting select_query raw (real transaction if exists)...")
    raw = tools.client.select("SELECT id, type, amount FROM transactions", limit=3)
    print(f"Raw result: {raw}")

    print("\nTesting get_customer_context (should return empty list, not error)...")
    ctx = tools.get_customer_context("C1000001")
    print(f"Result: {ctx}")

    print("\nMCP client OK")
