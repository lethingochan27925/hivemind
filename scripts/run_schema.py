"""Chay toan bo migration trong migrations/ theo thu tu ten file.

Ban cu chi doc 001_init.sql, nen 002_indexes.sql va 003_views.sql chua bao gio
duoc ap dung - DB thieu index va view ma khong ai phat hien.
"""
import os
from pathlib import Path

import psycopg2
from psycopg2.extensions import ISOLATION_LEVEL_AUTOCOMMIT

MIGRATIONS_DIR = Path("migrations")


def load_database_url(env_path: str = ".env") -> str:
    if os.environ.get("DATABASE_URL"):
        return os.environ["DATABASE_URL"]
    for line in Path(env_path).read_text(encoding="utf-8").splitlines():
        if line.strip().startswith("DATABASE_URL"):
            return line.partition("=")[2].strip().strip('"').strip("'")
    raise SystemExit("[FAIL] khong tim thay DATABASE_URL trong env hoac .env")


def main() -> None:
    files = sorted(MIGRATIONS_DIR.glob("*.sql"))
    if not files:
        raise SystemExit(f"[FAIL] khong co migration nao trong {MIGRATIONS_DIR}/")

    conn = psycopg2.connect(load_database_url())
    conn.set_isolation_level(ISOLATION_LEVEL_AUTOCOMMIT)
    try:
        # 001_init.sql creates a VECTOR INDEX; on a fresh cluster (e.g. CI's
        # single-node container, started clean on every run) that syntax is
        # gated behind this cluster setting until enabled once. Idempotent -
        # safe to run on every invocation, including against an existing
        # cluster where it was already turned on.
        with conn.cursor() as cur:
            cur.execute("SET CLUSTER SETTING feature.vector_index.enabled = true")

        for path in files:
            sql = path.read_text(encoding="utf-8").strip()
            if not sql:
                print(f"[BO QUA] {path.name} (rong)")
                continue
            print(f"[CHAY]   {path.name}")
            with conn.cursor() as cur:
                cur.execute(sql)
    finally:
        conn.close()

    print("[DONE] Da chay xong toan bo migration")


if __name__ == "__main__":
    main()
