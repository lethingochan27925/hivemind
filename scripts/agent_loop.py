"""
agent_loop.py — HiveMind Agent Worker
Flow: claim task -> input validation -> vector recall (case_memory)
      -> reason (Claude Haiku) -> verdict -> write case_memory -> audit_log
"""

import os
import re
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
EMBED_DIM = 1024

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


def sanitize_field(value: str, max_len: int = 64) -> str:
    value = re.sub(r'[^\w\s.,\-]', '', str(value))
    return value[:max_len]


def embed_text(text: str) -> list[float]:
    response = bedrock_embed.invoke_model(
        modelId=TITAN_MODEL_ID,
        contentType="application/json",
        accept="application/json",
        body=json.dumps({
            "inputText": text,
            "dimensions": EMBED_DIM,
            "normalize": True,
        }),
    )
    body = json.loads(response["body"].read())
    return body["embedding"]


def amount_range(amount: float) -> str:
    if amount < 10_000:
        return "low"
    elif amount < 100_000:
        return "mid"
    return "high"


def sign_label(val: float) -> str:
    if abs(val) < 1.0:
        return "near_zero"
    return "positive" if val > 0 else "negative"


def retrieve_case_memory(cur, txn: dict, top_k: int = 3) -> list:
    summary = (
        f"type={txn.get('type','?')} "
        f"amount={float(txn.get('amount',0)):.2f} "
        f"risk={float(txn.get('risk_score',0)):.3f} "
        f"error_orig={float(txn.get('error_balance_orig',0)):.2f} "
        f"error_dest={float(txn.get('error_balance_dest',0)):.2f}"
    )
    try:
        embedding = embed_text(summary)
        embedding_str = "[" + ",".join(str(x) for x in embedding) + "]"
        cur.execute("""
            SELECT id, summary, verdict, confidence_avg, pattern_type,
                   key_signals, salience, recall_count,
                   embedding <=> %s::vector AS distance
            FROM case_memory
            WHERE archived = false
              AND transaction_type = %s
            ORDER BY embedding <=> %s::vector
            LIMIT %s
        """, (embedding_str, txn.get("type"), embedding_str, top_k))
        hits = cur.fetchall() or []

        if hits:
            ids = [h["id"] for h in hits]
            cur.execute("""
                UPDATE case_memory
                SET salience = LEAST(salience + 0.1, 2.0),
                    recall_count = recall_count + 1,
                    last_recalled_at = now()
                WHERE id = ANY(%s::UUID[])
            """, (ids,))
        return hits
    except Exception as e:
        print(f"  [memory] retrieve error: {e}")
        return []


def classify_pattern(txn: dict) -> str:
    amount = float(txn.get("amount", 0))
    old_orig = float(txn.get("old_balance_orig", 0))
    new_orig = float(txn.get("new_balance_orig", 0))
    err_dest = float(txn.get("error_balance_dest", 0))

    if old_orig == amount and new_orig == 0:
        return "balance_wipe"
    if abs(err_dest) > 1000:
        return "dest_no_update"
    if amount > 200_000:
        return "high_amount_transfer"
    if txn.get("type") == "CASH_OUT":
        return "rapid_cashout"
    return "unclassified"


def write_case_memory(cur, txn: dict, verdict: str, confidence: float,
                       rationale: str, task_id: str):
    summary = (
        f"type={txn.get('type','?')} amount={float(txn.get('amount',0)):.2f} "
        f"error_orig={float(txn.get('error_balance_orig',0)):.2f} "
        f"error_dest={float(txn.get('error_balance_dest',0)):.2f}. "
        f"Verdict: {verdict.upper()}. {rationale}"
    )
    pattern = classify_pattern(txn)
    key_signals = [pattern] if pattern != "unclassified" else []

    try:
        embedding = embed_text(summary)
        embedding_str = "[" + ",".join(str(x) for x in embedding) + "]"

        cur.execute("""
            SELECT id FROM case_memory
            WHERE archived = false AND transaction_type = %s
            ORDER BY embedding <=> %s::vector
            LIMIT 1
        """, (txn.get("type"), embedding_str))
        existing = cur.fetchone()

        if existing:
            cur.execute("""
                SELECT 1 - (embedding <=> %s::vector) AS similarity
                FROM case_memory WHERE id = %s
            """, (embedding_str, existing["id"]))
            sim = cur.fetchone()["similarity"]
        else:
            sim = 0

        if existing and sim > 0.92:
            cur.execute("""
                UPDATE case_memory
                SET summary = %s, merge_count = merge_count + 1,
                    last_merged_at = now()
                WHERE id = %s
            """, (summary, existing["id"]))
        else:
            cur.execute("""
                INSERT INTO case_memory (
                    summary, verdict, confidence_avg, pattern_type, key_signals,
                    transaction_type, amount_range, error_orig_sign, error_dest_sign,
                    embedding, source_task_id, created_at
                ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s::vector, %s, now())
            """, (
                summary, verdict, confidence, pattern, key_signals,
                txn.get("type"), amount_range(float(txn.get("amount", 0))),
                sign_label(float(txn.get("error_balance_orig", 0))),
                sign_label(float(txn.get("error_balance_dest", 0))),
                embedding_str, task_id,
            ))
    except Exception as e:
        print(f"  [memory] write error: {e}")


def write_audit_log(cur, txn: dict, task: dict, action: str,
                     reasoning: str = None, memory_hits: list = None,
                     tokens_in: int = None, tokens_out: int = None,
                     latency_ms: int = None, bedrock_model: str = None):
    cur.execute("""
        INSERT INTO audit_log (
            task_id, transaction_id, agent_id, action, reasoning,
            memory_hits, similarity_scores, tokens_in, tokens_out,
            bedrock_model, latency_ms, created_at
        ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, now())
    """, (
        task["id"], txn["id"], WORKER_ID, action, reasoning,
        len(memory_hits) if memory_hits else None,
        [float(h["distance"]) for h in memory_hits] if memory_hits else None,
        tokens_in, tokens_out, bedrock_model, latency_ms,
    ))


def build_prompt(txn: dict, memory_hits: list) -> str:
    memory_context = ""
    if memory_hits:
        cases = "\n".join([
            f"  - verdict={h['verdict']} pattern={h.get('pattern_type','?')}: {h['summary']}"
            for h in memory_hits
        ])
        memory_context = f"\nSimilar past cases (reference only, do NOT anchor to these):\n{cases}\n"

    name_orig = sanitize_field(txn.get("name_orig", ""))
    name_dest = sanitize_field(txn.get("name_dest", ""))

    return f"""You are a fraud investigation agent. Analyze this transaction independently.

Transaction:
  type={txn.get('type','?')}
  amount={float(txn.get('amount',0)):.2f}
  name_orig={name_orig}
  name_dest={name_dest}
  risk_score={float(txn.get('risk_score',0)):.3f}
  error_balance_orig={float(txn.get('error_balance_orig',0)):.2f}
  error_balance_dest={float(txn.get('error_balance_dest',0)):.2f}
{memory_context}
Scoring rules (follow strictly):
- risk_score < 0.30 AND both errors near 0 -> legit
- risk_score 0.30-0.60 AND uncertain signals -> escalate
- risk_score > 0.60 OR large error_balance -> fraud

Respond in JSON only:
{{
  "verdict": "fraud" | "escalate" | "legit",
  "confidence": 0.0-1.0,
  "rationale": "one sentence explanation"
}}"""


def call_claude(txn: dict, memory_hits: list) -> dict:
    prompt = build_prompt(txn, memory_hits)
    start = time.monotonic()
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
        latency_ms = int((time.monotonic() - start) * 1000)
        body = json.loads(response["body"].read())
        text = body["content"][0]["text"].strip()

        usage = body.get("usage", {})
        tokens_in = usage.get("input_tokens")
        tokens_out = usage.get("output_tokens")

        if "```" in text:
            text = text.split("```")[1].replace("json", "").strip()
        result = json.loads(text)

        assert result["verdict"] in ("fraud", "escalate", "legit")
        assert 0.0 <= float(result["confidence"]) <= 1.0

        return {
            "verdict": result["verdict"],
            "confidence": float(result["confidence"]),
            "rationale": result.get("rationale", ""),
            "step": "bedrock_reasoning",
            "tokens_in": tokens_in,
            "tokens_out": tokens_out,
            "latency_ms": latency_ms,
        }
    except Exception as e:
        print(f"  [claude] error: {e}, falling back to rule-based")
        fallback = _rule_based_fallback(txn)
        fallback["latency_ms"] = int((time.monotonic() - start) * 1000)
        return fallback


def _rule_based_fallback(txn: dict) -> dict:
    risk = float(txn.get("risk_score", 0))
    if risk >= 0.80:
        return {"verdict": "fraud", "confidence": 0.90, "rationale": "high risk score",
                "step": "fallback", "tokens_in": None, "tokens_out": None}
    elif risk >= 0.50:
        return {"verdict": "escalate", "confidence": 0.70, "rationale": "medium risk score",
                "step": "fallback", "tokens_in": None, "tokens_out": None}
    return {"verdict": "legit", "confidence": 0.85, "rationale": "low risk score",
            "step": "fallback", "tokens_in": None, "tokens_out": None}


def connect():
    conn = psycopg2.connect(os.environ["DATABASE_URL"])
    conn.autocommit = False
    return conn


def run():
    conn = connect()
    print(f"[{WORKER_ID}] Worker started (Bedrock Claude Haiku + Titan Embeddings v2)")

    while True:
        try:
            cur = conn.cursor(cursor_factory=RealDictCursor)

            cur.execute("""
                SELECT id, transaction_id, risk_score, step, scratchpad
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

            cur.execute("SELECT * FROM transactions WHERE id = %s", (task["transaction_id"],))
            txn = cur.fetchone()

            if txn is None:
                cur.execute("UPDATE tasks SET status='failed', completed_at=NOW() WHERE id=%s", (task["id"],))
                write_audit_log(cur, {"id": task["transaction_id"]}, dict(task), "task_failed")
                conn.commit()
                cur.close()
                continue

            txn = dict(txn)

            memory_hits = retrieve_case_memory(cur, txn)
            write_audit_log(cur, txn, dict(task), "memory_recall", memory_hits=memory_hits)
            print(f"  Memory hits: {len(memory_hits)}")

            result = call_claude(txn, memory_hits)
            write_audit_log(
                cur, txn, dict(task), "bedrock_reasoning",
                reasoning=result["rationale"],
                tokens_in=result.get("tokens_in"),
                tokens_out=result.get("tokens_out"),
                latency_ms=result.get("latency_ms"),
                bedrock_model=CLAUDE_MODEL_ID,
            )

            new_status = "pending_review" if result["verdict"] == "escalate" else "done"

            cur.execute("""
                UPDATE tasks
                SET status = %s,
                    verdict = %s,
                    confidence = %s,
                    completed_at = NOW(),
                    heartbeat_at = NOW(),
                    step = %s
                WHERE id = %s
            """, (new_status, result["verdict"], result["confidence"], result["step"], task["id"]))

            write_case_memory(cur, txn, result["verdict"], result["confidence"],
                               result["rationale"], task["id"])

            write_audit_log(cur, txn, dict(task), f"verdict_{result['verdict']}",
                             reasoning=result["rationale"])

            conn.commit()
            cur.close()

            print(
                f"  Done | risk={txn['risk_score']:.3f} | "
                f"verdict={result['verdict']} | confidence={result['confidence']:.2f} | "
                f"status={new_status}"
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
