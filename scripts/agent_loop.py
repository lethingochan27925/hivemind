"""
agent_loop.py — HiveMind Agent Worker

Flow: claim task → retrieve episodic memory → reason → verdict → write memory → audit log

Bedrock: mock (fake_reasoning). Swap thật sau khi có AWS credentials.
Episodic memory: stub trả về [] — Thuận sẽ enable vector index trên Cloud.
"""

import os
import time
import socket
import json
from datetime import datetime, timezone

import psycopg2
from psycopg2.extras import RealDictCursor, execute_values
from dotenv import load_dotenv

load_dotenv(".env")

WORKER_ID = socket.gethostname()
POLL_INTERVAL = 2


# ===========================================================================
# MEMORY LAYER — 2 hàm này là core của HiveMind, phân biệt với Postgres thường
# ===========================================================================

def retrieve_episodic_memory(cur, risk_score: float, txn_type: str, top_k: int = 3) -> list:
    """
    Tìm top-k case tương tự từ episodic_memory bằng vector search.
    Hiện tại: stub trả về [] vì chưa có embedding.
    
    Khi Thuận enable CockroachDB Distributed Vector Index:
        - Gọi Bedrock Titan embed summary text
        - Query: ORDER BY embedding <-> $1 LIMIT top_k
    """
    # TODO: thay bằng vector search thật
    # summary = f"{txn_type} risk={risk_score:.2f}"
    # embedding = embed_text(summary)  # Bedrock Titan
    # cur.execute("""
    #     SELECT transaction_id, verdict, rationale, confidence
    #     FROM episodic_memory
    #     ORDER BY embedding <-> %s
    #     LIMIT %s
    # """, (embedding, top_k))
    # return cur.fetchall()
    return []  # stub


def write_episodic_memory(cur, txn: dict, verdict: str, confidence: float, rationale: str):
    """
    Ghi episode mới vào episodic_memory sau mỗi investigation.
    Đây là lúc agent 'học' từ case vừa xử lý.
    
    Hiện tại: ghi metadata thật, embedding là NULL (placeholder).
    Khi có Bedrock Titan: embed summary rồi INSERT kèm vector.
    """
    summary = (
        f"type={txn.get('type','?')} "
        f"amount={txn.get('amount',0):.2f} "
        f"risk={txn.get('risk_score',0):.3f} "
        f"verdict={verdict}"
    )
    cur.execute("""
        INSERT INTO episodic_memory (
            transaction_id,
            summary,
            verdict,
            confidence,
            rationale,
            embedding,
            created_at
        ) VALUES (%s, %s, %s, %s, %s, NULL, NOW())
        ON CONFLICT (transaction_id) DO NOTHING
    """, (
        txn["id"],
        summary,
        verdict,
        confidence,
        rationale,
    ))


def write_audit_log(cur, txn: dict, task: dict, verdict: str, confidence: float,
                    rationale: str, memory_hits: list):
    """
    Append-only audit log — không update, chỉ INSERT.
    Dùng cho compliance + demo kill-region recovery.
    """
    cur.execute("""
        INSERT INTO audit_log (
            transaction_id,
            task_id,
            worker_id,
            verdict,
            confidence,
            rationale,
            memory_hits,
            decided_at
        ) VALUES (%s, %s, %s, %s, %s, %s, %s, NOW())
    """, (
        txn["id"],
        task["id"],
        WORKER_ID,
        verdict,
        confidence,
        rationale,
        json.dumps(memory_hits),
    ))


# ===========================================================================
# REASONING — mock Bedrock, swap thật sau
# ===========================================================================

def fake_reasoning(txn: dict, memory_hits: list) -> dict:
    """
    Placeholder cho Bedrock Claude.
    Nhận thêm memory_hits để sau này inject vào prompt.
    
    Swap bằng:
        response = bedrock.invoke_model(
            modelId="anthropic.claude-haiku-20240307-v1:0",
            body=json.dumps({
                "prompt": build_prompt(txn, memory_hits),
                ...
            })
        )
    """
    risk_score = txn.get("risk_score", 0)
    
    # Nếu có memory hits, giả vờ confidence tăng lên một chút
    memory_boost = 0.03 * len(memory_hits)

    if risk_score >= 0.80:
        verdict, confidence, step = "fraud", min(0.96 + memory_boost, 0.99), "risk_score_above_threshold"
    elif risk_score >= 0.50:
        verdict, confidence, step = "escalate", min(0.75 + memory_boost, 0.90), "manual_review"
    else:
        verdict, confidence, step = "legit", min(0.90 + memory_boost, 0.99), "low_risk"

    rationale = (
        f"risk_score={risk_score:.3f} → {verdict} "
        f"(memory_hits={len(memory_hits)}, step={step})"
    )
    return {"verdict": verdict, "confidence": confidence, "step": step, "rationale": rationale}


# ===========================================================================
# MAIN LOOP
# ===========================================================================

def connect():
    conn = psycopg2.connect(os.environ["DATABASE_URL"])
    conn.autocommit = False
    return conn


def run():
    conn = connect()
    print(f"[{WORKER_ID}] Worker started")

    while True:
        try:
            cur = conn.cursor(cursor_factory=RealDictCursor)

            # --- 1. Claim 1 pending task (SKIP LOCKED = no contention khi fleet) ---
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

            # --- 3. Retrieve episodic memory (CORE HiveMind) ---
            memory_hits = retrieve_episodic_memory(
                cur,
                risk_score=float(txn["risk_score"]),
                txn_type=txn.get("type", "UNKNOWN"),
            )
            print(f"  Memory hits: {len(memory_hits)}")

            # --- 4. Reason (mock Bedrock) ---
            result = fake_reasoning(txn, memory_hits)

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

            # --- 6. Write episodic memory (CORE HiveMind) ---
            write_episodic_memory(cur, dict(txn), result["verdict"], result["confidence"], result["rationale"])

            # --- 7. Audit log (append-only) ---
            write_audit_log(cur, dict(txn), dict(task), result["verdict"],
                            result["confidence"], result["rationale"], memory_hits)

            conn.commit()
            cur.close()

            print(
                f"  Done | risk={txn['risk_score']:.3f} | "
                f"verdict={result['verdict']} | confidence={result['confidence']:.2f}"
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