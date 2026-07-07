import os
from dotenv import load_dotenv
import psycopg2
from psycopg2.extensions import ISOLATION_LEVEL_AUTOCOMMIT
load_dotenv('.env.example')

DATABASE_URL = os.environ["DATABASE_URL"]

conn = psycopg2.connect(DATABASE_URL)
conn.set_isolation_level(ISOLATION_LEVEL_AUTOCOMMIT)

with open('migrations/001_init.sql', 'r', encoding='utf-8') as f:
    sql = f.read()

with conn.cursor() as cur:
    cur.execute(sql)

print("✅ Đã chạy schema thành công!")
conn.close()