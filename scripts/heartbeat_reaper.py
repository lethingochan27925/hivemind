"""
heartbeat_reaper.py — HiveMind Heartbeat Reaper

Vai trò: phát hiện agent worker bị crash (không update heartbeat)
và re-queue các task bị stuck ở trạng thái 'investigating'.

Chạy dạng long-running process song song với dispatcher và agent_loop.
Ctrl+C để dừng an toàn.
"""

import os
import time
import logging
import signal

import psycopg2
from dotenv import load_dotenv

load_dotenv(".env")

DATABASE_URL     = os.environ["DATABASE_URL"]
REAP_INTERVAL    = int(os.environ.get("REAPER_INTERVAL_SECONDS", 10))
STUCK_THRESHOLD  = os.environ.get("REAPER_STUCK_THRESHOLD", "30s")

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [reaper] %(levelname)s %(message)s",
)
log = logging.getLogger("reaper")

_shutdown = False


def _handle_shutdown(signum, frame):
    global _shutdown
    log.info("Nhận tín hiệu dừng (%s) — sẽ thoát sau chu kỳ hiện tại...", signum)
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
            log.info("Đã kết nối CockroachDB.")
            return conn
        except psycopg2.OperationalError as e:
            if attempt >= max_attempts:
                raise
            delay = base_delay * (2 ** (attempt - 1))
            log.warning(
                "Kết nối thất bại (lần %d/%d): %s — thử lại sau %.1fs",
                attempt, max_attempts, e, delay,
            )
            time.sleep(delay)


def reap_stuck_tasks(conn) -> int:
    """
    Tìm các task đang 'investigating' nhưng heartbeat quá cũ
    → reset về 'pending' để agent khác claim lại.

    Đây là cơ chế self-healing của fleet:
    - Agent crash / bị kill → không update heartbeat
    - Reaper phát hiện sau STUCK_THRESHOLD giây
    - Task được re-queue → agent khác pick up
    - Không mất data, không cần manual intervention
    """
    with conn.cursor() as cur:
        cur.execute(f"""
            UPDATE tasks
            SET
                status       = 'pending',
                claimed_by   = NULL,
                claimed_at   = NULL,
                heartbeat_at = NULL
            WHERE
                status = 'investigating'
                AND heartbeat_at < NOW() - INTERVAL '{STUCK_THRESHOLD}'
            RETURNING id, claimed_by
        """)
        reaped = cur.fetchall()
    conn.commit()
    return len(reaped), reaped


def main():
    conn = connect_with_retry()
    log.info(
        "Heartbeat Reaper bắt đầu (interval=%ds, threshold=%s).",
        REAP_INTERVAL, STUCK_THRESHOLD,
    )

    while not _shutdown:
        try:
            count, reaped = reap_stuck_tasks(conn)
            if count > 0:
                for task_id, worker_id in reaped:
                    log.warning(
                        "Re-queued stuck task %s (worker=%s)",
                        task_id, worker_id,
                    )
                log.info("Reaped %d stuck task(s) → re-queued as pending.", count)
            else:
                log.debug("No stuck tasks found.")

        except psycopg2.Error as e:
            log.error("Lỗi DB: %s", e)
            conn.rollback()
            try:
                conn.close()
            except Exception:
                pass
            conn = connect_with_retry()

        time.sleep(REAP_INTERVAL)

    conn.close()
    log.info("Heartbeat Reaper đã dừng.")


if __name__ == "__main__":
    main()