"""
agent_loop.py — HiveMind Agent Worker

Flow: claim task → retrieve episodic memory (vector search) → reason (Claude Haiku)
      → verdict → write episodic memory → audit log

Bedrock: Claude Haiku cho reasoning, Titan Embeddings v2 cho vector search.
"""

import os
import time
import socket
import json

import boto3
import psycopg2
from psycopg2.extras import RealDictCursor
from dotenv import load_dotenv

load_dotenv(".env")

WORKER_ID = socket.gethostname()
POLL_INTERVAL = 2
TITAN_MODEL_ID  = "amazon.titan-embed-text-v2:0"
CLAUDE_MODEL_ID = "anthropic.claude-3-haiku-20240307-v1:0"
# ===========================================================================
# BEDROCK CLIENT
# ===========================================================================

# bedrock = boto3.client(
#     service_name="bedrock-runtime",
#     region_name=os.environ.get("AWS_DEFAULT_REGION", "ap-southeast-1"),
#     aws_access_key_id=os.environ["AWS_ACCESS_KEY_ID"],
#     aws_secret_access_key=os.environ["AWS_SECRET_ACCESS_KEY"],
# )

# Tạo 2 client riêng
bedrock = boto3.client(
    service_name="bedrock-runtime",
    region_name="ap-southeast-1",
    aws_access_key_id=os.environ["AWS_ACCESS_KEY_ID"],
    aws_secret_access_key=os.environ["AWS_SECRET_ACCESS_KEY"],
)

bedrock_embed = boto3.client(
    service_name="bedrock-runtime",
    region_name="us-east-1",
    aws_access_key_id=os.environ["AWS_ACCESS_KEY_ID"],
    aws_secret_access_key=os.environ["AWS_SECRET_ACCESS_KEY"],
)





# ===========================================================================
# EMBEDDING — Titan Embeddings v2
# ===========================================================================

def embed_text(text: str) -> list[float]:
    """Gọi Bedrock Titan Embeddings v2, trả về vector 1536 chiều."""
    response = bedrock_embed.invoke_model(
        modelId=TITAN_MODEL_ID,
        contentType="application/json",
        accept="application/json",
        body=json.dumps({
            "inputText": text,
            # "dimensions": 1536,
            # "normalize": True,
        }),
    )
    body = json.loads(response["body"].read())
    return body["embedding"]


# ===========================================================================
# MEMORY LAYER
# ===========================================================================

def retrieve_episodic_memory(cur, txn: dict, top_k: int = 3) -> list:
    """
    Vector search top-k case tương tự từ episodic_memory.
    Dùng CockroachDB pgvector cosine distance.
    """
    summary = (
        f"type={txn.get('type','?')} "
        f"amount={float(txn.get('amount',0)):.2f} "
        f"risk={float(txn.get('risk_score',0)):.3f}"
    )
    try:
        embedding = embed_text(summary)
        embedding_str = "[" + ",".join(str(x) for x in embedding) + "]"
        cur.execute("""
            SELECT transaction_id, verdict, rationale, confidence
            FROM episodic_memory
            WHERE embedding IS NOT NULL
            ORDER BY embedding <-> %s::vector
            LIMIT %s
        """, (embedding_str, top_k))
        return cur.fetchall() or []
    except Exception as e:
        print(f"  [memory] retrieve error: {e}")
        return []


def write_episodic_memory(cur, txn: dict, verdict: str, confidence: float, rationale: str):
    """Ghi episode mới vào episodic_memory với embedding thật."""
    summary = (
        f"type={txn.get('type','?')} "
        f"amount={float(txn.get('amount',0)):.2f} "
        f"risk={float(txn.get('risk_score',0)):.3f} "
        f"verdict={verdict}"
    )
    try:
        embedding = embed_text(summary)
        embedding_str = "[" + ",".join(str(x) for x in embedding) + "]"
        cur.execute("""
            INSERT INTO episodic_memory (
                transaction_id, summary, verdict, confidence,
                rationale, embedding, created_at
            ) VALUES (%s, %s, %s, %s, %s, %s::vector, NOW())
            ON CONFLICT (transaction_id) DO NOTHING
        """, (
            txn["id"], summary, verdict, confidence,
            rationale, embedding_str,
        ))
    except Exception as e:
        print(f"  [memory] write error: {e}")


def write_audit_log(cur, txn: dict, task: dict, verdict: str,
                    confidence: float, rationale: str, memory_hits: list):
    """Append-only audit log."""
    cur.execute("""
        INSERT INTO audit_log (
            transaction_id, task_id, worker_id,
            verdict, confidence, rationale, memory_hits, decided_at
        ) VALUES (%s, %s, %s, %s, %s, %s, %s, NOW())
    """, (
        txn["id"], task["id"], WORKER_ID,
        verdict, confidence, rationale,
        json.dumps([dict(h) for h in memory_hits]),
    ))


# ===========================================================================
# REASONING — Claude Haiku
# ===========================================================================

def build_prompt(txn: dict, memory_hits: list) -> str:
    """Xây dựng prompt cho Claude Haiku."""
    memory_context = ""
    if memory_hits:
        cases = "\n".join([
            f"  - verdict={h['verdict']} confidence={h['confidence']:.2f}: {h['rationale']}"
            for h in memory_hits
        ])
        memory_context = f"\nSimilar past cases:\n{cases}\n"

    return f"""You are a fraud investigation agent. Analyze this transaction and give a verdict.

Transaction:
  type={txn.get('type','?')}
  amount={float(txn.get('amount',0)):.2f}
  risk_score={float(txn.get('risk_score',0)):.3f}
  error_balance_orig={float(txn.get('error_balance_orig',0)):.2f}
  error_balance_dest={float(txn.get('error_balance_dest',0)):.2f}
{memory_context}
Respond in JSON only:
{{
  "verdict": "fraud" | "escalate" | "legit",
  "confidence": 0.0-1.0,
  "rationale": "one sentence explanation"
}}"""


def call_claude(txn: dict, memory_hits: list) -> dict:
    """Gọi Claude Haiku qua Bedrock, parse JSON response."""
    prompt = build_prompt(txn, memory_hits)
    try:
        response = bedrock.invoke_model(
            modelId=CLAUDE_MODEL_ID,
            contentType="application/json",
            accept="application/json",
            body=json.dumps({
                "anthropic_version": "bedrock-2023-05-31",
                "max_tokens": 256,
                "messages": [{"role": "user", "content": prompt}],
            }),
        )
        body = json.loads(response["body"].read())
        text = body["content"][0]["text"].strip()

        if "```" in text:
            text = text.split("```")[1].replace("json", "").strip()
        result = json.loads(text)

        assert result["verdict"] in ("fraud", "escalate", "legit")
        assert 0.0 <= float(result["confidence"]) <= 1.0
        return {
            "verdict": result["verdict"],
            "confidence": float(result["confidence"]),
            "rationale": result.get("rationale", ""),
            "step": "claude_haiku",
        }
    except Exception as e:
        print(f"  [claude] error: {e}, falling back to rule-based")
        return _rule_based_fallback(txn)


def _rule_based_fallback(txn: dict) -> dict:
    """Fallback nếu Claude call fail."""
    risk = float(txn.get("risk_score", 0))
    if risk >= 0.80:
        return {"verdict": "fraud",    "confidence": 0.90, "rationale": "high risk score", "step": "fallback"}
    elif risk >= 0.50:
        return {"verdict": "escalate", "confidence": 0.70, "rationale": "medium risk score", "step": "fallback"}
    return {"verdict": "legit", "confidence": 0.85, "rationale": "low risk score", "step": "fallback"}


# ===========================================================================
# MAIN LOOP
# ===========================================================================

def connect():
    conn = psycopg2.connect(os.environ["DATABASE_URL"])
    conn.autocommit = False
    return conn


def run():
    conn = connect()
    print(f"[{WORKER_ID}] Worker started (Bedrock Claude Haiku + Titan Embeddings)")

    while True:
        try:
            cur = conn.cursor(cursor_factory=RealDictCursor)

            # --- 1. Claim 1 pending task ---
            cur.execute("""
                SELECT id, transaction_id, risk_score
                FROM tasks
                WHERE status = 'pending'
                ORDER BY created_at
                LIMIT 1
                FOR UPDATE SKIP LOCKED
            """)
            task = cur.fetchone()

            if task is None:
                conn.commit()
                cur.close()
                print("No pending tasks, sleeping...")
                time.sleep(POLL_INTERVAL)
                continue

            cur.execute("""
                UPDATE tasks
                SET status = 'investigating',
                    claimed_by = %s,
                    claimed_at = NOW(),
                    heartbeat_at = NOW()
                WHERE id = %s
            """, (WORKER_ID, task["id"]))
            conn.commit()
            print(f"Claimed task {task['id']}")

            # --- 2. Load transaction ---
            cur.execute("SELECT * FROM transactions WHERE id = %s", (task["transaction_id"],))
            txn = cur.fetchone()

            if txn is None:
                cur.execute("UPDATE tasks SET status='failed', completed_at=NOW() WHERE id=%s", (task["id"],))
                conn.commit()
                cur.close()
                continue

            txn = dict(txn)

            # --- 3. Retrieve episodic memory (vector search) ---
            memory_hits = retrieve_episodic_memory(cur, txn)
            print(f"  Memory hits: {len(memory_hits)}")

            # --- 4. Reason via Claude Haiku ---
            result = call_claude(txn, memory_hits)

            # --- 5. Update task ---
            cur.execute("""
                UPDATE tasks
                SET status = 'done',
                    verdict = %s,
                    confidence = %s,
                    completed_at = NOW(),
                    heartbeat_at = NOW(),
                    step = %s
                WHERE id = %s
            """, (result["verdict"], result["confidence"], result["step"], task["id"]))

            # --- 6. Write episodic memory ---
            write_episodic_memory(cur, txn, result["verdict"],
                                  result["confidence"], result["rationale"])

            # --- 7. Audit log ---
            write_audit_log(cur, txn, dict(task), result["verdict"],
                            result["confidence"], result["rationale"], memory_hits)

            conn.commit()
            cur.close()

            print(
                f"  Done | risk={txn['risk_score']:.3f} | "
                f"verdict={result['verdict']} | confidence={result['confidence']:.2f} | "
                f"step={result['step']}"
            )

        except KeyboardInterrupt:
            print("Stopping worker...")
            break
        except Exception as e:
            conn.rollback()
            print(f"Error: {e}")
            time.sleep(2)

    conn.close()


if __name__ == "__main__":
    run()