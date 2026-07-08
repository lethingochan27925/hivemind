"""
dispatcher.py — HiveMind Dispatcher

Vai trò: liên tục quét bảng `transactions`, tạo `tasks` tương ứng cho
những giao dịch chưa có task, để Agent Worker claim và điều tra.

Chạy dạng long-running process (daemon), không tự thoát khi hết việc —
vì replay stream có thể vẫn đang insert thêm transaction. Ctrl+C để dừng
an toàn (chờ batch hiện tại xử lý xong).
"""

import os
import time
import logging
import signal

import psycopg2
from psycopg2.extras import execute_values
from dotenv import load_dotenv

# --- Cấu hình -------------------------------------------------------------

load_dotenv('.env')  # đọc file .env THẬT (không phải .env.example — đó chỉ là mẫu)

DATABASE_URL = os.environ["DATABASE_URL"]
BATCH_SIZE = int(os.environ.get("DISPATCHER_BATCH_SIZE", 100))
IDLE_SLEEP_SECONDS = float(os.environ.get("DISPATCHER_IDLE_SLEEP", 2))
BUSY_SLEEP_SECONDS = float(os.environ.get("DISPATCHER_BUSY_SLEEP", 0.2))

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [dispatcher] %(levelname)s %(message)s",
)
log = logging.getLogger("dispatcher")

_shutdown = False


def _handle_shutdown(signum, frame):
    global _shutdown
    log.info("Nhận tín hiệu dừng (%s) — sẽ thoát sau batch hiện tại...", signum)
    _shutdown = True


signal.signal(signal.SIGINT, _handle_shutdown)
signal.signal(signal.SIGTERM, _handle_shutdown)


# --- Kết nối DB có retry (CockroachDB Cloud có thể ngắt kết nối idle) ------

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


# --- Query ------------------------------------------------------------------

# Gợi ý: cần index cho tk.transaction_id và t.arrived_at nếu bảng lớn dần
# CREATE INDEX ON tasks (transaction_id);
# CREATE INDEX ON transactions (arrived_at);
FETCH_QUERY = """
    SELECT
        t.id,
        t.risk_score
    FROM transactions t
    LEFT JOIN tasks tk
        ON tk.transaction_id = t.id
    WHERE tk.transaction_id IS NULL
    ORDER BY t.arrived_at
    LIMIT %s
"""

# transaction_id PHẢI có UNIQUE constraint trong bảng tasks để ON CONFLICT
# hoạt động — đây chính là "idempotency guard" chống hold trùng giao dịch
# khi có nhiều dispatcher instance chạy song song.
INSERT_QUERY = """
    INSERT INTO tasks (
        transaction_id,
        risk_score,
        status
    )
    VALUES %s
    ON CONFLICT (transaction_id) DO NOTHING
"""


def dispatch_batch(conn) -> int:
    """Lấy tối đa BATCH_SIZE transaction chưa có task, tạo task tương ứng.
    Trả về số task thực sự tạo (0 nếu không có gì để làm)."""
    with conn.cursor() as cur:
        cur.execute(FETCH_QUERY, (BATCH_SIZE,))
        rows = cur.fetchall()

        if not rows:
            return 0

        values = [(txn_id, risk_score, "pending") for txn_id, risk_score in rows]

        execute_values(cur, INSERT_QUERY, values)
        inserted = cur.rowcount  # loại trừ các dòng bị ON CONFLICT bỏ qua

    conn.commit()
    return inserted


def main():
    conn = connect_with_retry()
    log.info("Dispatcher bắt đầu chạy (batch_size=%d).", BATCH_SIZE)

    while not _shutdown:
        try:
            inserted = dispatch_batch(conn)
        except psycopg2.Error as e:
            log.error("Lỗi khi dispatch batch: %s", e)
            conn.rollback()
            try:
                conn.close()
            except Exception:
                pass
            conn = connect_with_retry()
            continue

        if inserted:
            log.info("Đã tạo %d task mới.", inserted)
            time.sleep(BUSY_SLEEP_SECONDS)
        else:
            # Không có transaction mới → chờ rồi poll tiếp.
            # KHÔNG break: replay stream có thể vẫn đang insert thêm transaction.
            time.sleep(IDLE_SLEEP_SECONDS)

    conn.close()
    log.info("Dispatcher đã dừng.")


if __name__ == "__main__":
    main()