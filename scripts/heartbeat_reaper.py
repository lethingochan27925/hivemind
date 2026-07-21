"""
heartbeat_reaper.py — HiveMind Heartbeat Reaper
Phat hien agent worker crash (khong update heartbeat), re-queue task
'investigating' bi stuck. Giu nguyen step/scratchpad de agent khac resume.
"""

import os
import time
import logging
import signal

import psycopg2
from dotenv import load_dotenv

load_dotenv(".env")

DATABASE_URL    = os.environ["DATABASE_URL"]
REAP_INTERVAL   = int(os.environ.get("REAPER_INTERVAL_SECONDS", 10))
STUCK_THRESHOLD = os.environ.get("REAPER_STUCK_THRESHOLD", "30s")

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [reaper] %(levelname)s %(message)s",
)
log = logging.getLogger("reaper")

_shutdown = False


def _handle_shutdown(signum, frame):
    global _shutdown
    log.info("Nhan tin hieu dung (%s) — se thoat sau chu ky hien tai...", signum)
    _shutdown = True


signal.signal(signal.SIGINT, _handle_shutdown)
signal.signal(signal.SIGTERM, _handle_shutdown)


def connect_with_retry(max_attempts=5, base_delay=1.0):
    attempt = 0
    while True:
        attempt += 1
        try:
            conn = psycopg2.connect(DATABASE_URL)
            conn.autocommit = False
            log.info("Da ket noi CockroachDB.")
            return conn
        except psycopg2.OperationalError as e:
            if attempt >= max_attempts:
                raise
            delay = base_delay * (2 ** (attempt - 1))
            log.warning(
                "Ket noi that bai (lan %d/%d): %s — thu lai sau %.1fs",
                attempt, max_attempts, e, delay,
            )
            time.sleep(delay)


def reap_stuck_tasks(conn):
    """
    Re-queue task 'investigating' bi stuck ve 'pending'.
    Khong xoa step/scratchpad — agent moi doc lai de resume dung buoc.
    """
    with conn.cursor() as cur:
        cur.execute(f"""
            UPDATE tasks
            SET status       = 'pending',
                claimed_by   = NULL,
                claimed_at   = NULL,
                heartbeat_at = NULL
            WHERE status = 'investigating'
              AND heartbeat_at < NOW() - INTERVAL '{STUCK_THRESHOLD}'
            RETURNING id, claimed_by, transaction_id
        """)
        reaped = cur.fetchall()

        for task_id, worker_id, txn_id in reaped:
            cur.execute("""
                INSERT INTO audit_log (task_id, transaction_id, agent_id, action, reasoning, created_at)
                VALUES (%s, %s, %s, 'task_requeued', %s, now())
            """, (task_id, txn_id, worker_id or 'unknown', f"Stuck > {STUCK_THRESHOLD}, re-queued"))

    conn.commit()
    return len(reaped), reaped


def main():
    conn = connect_with_retry()
    log.info(
        "Heartbeat Reaper bat dau (interval=%ds, threshold=%s).",
        REAP_INTERVAL, STUCK_THRESHOLD,
    )

    while not _shutdown:
        try:
            count, reaped = reap_stuck_tasks(conn)
            if count > 0:
                for task_id, worker_id, txn_id in reaped:
                    log.warning("Re-queued stuck task %s (worker=%s)", task_id, worker_id)
                log.info("Reaped %d stuck task(s) -> re-queued as pending.", count)
            else:
                log.debug("No stuck tasks found.")

        except psycopg2.Error as e:
            log.error("Loi DB: %s", e)
            conn.rollback()
            try:
                conn.close()
            except Exception:
                pass
            conn = connect_with_retry()

        time.sleep(REAP_INTERVAL)

    conn.close()
    log.info("Heartbeat Reaper da dung.")


if __name__ == "__main__":
    main()
