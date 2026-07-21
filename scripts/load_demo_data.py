import os
import pandas as pd
from dotenv import load_dotenv
import psycopg2
import uuid
from psycopg2.extras import execute_values

load_dotenv('.env')
conn = psycopg2.connect(os.environ["DATABASE_URL"])
cur = conn.cursor()

df = pd.read_csv('./notebooks/demo_stream.csv')

data = []
for _, row in df.iterrows():
    data.append((
        str(uuid.uuid4()),
        int(row["step"]),
        row["type"],
        float(row["amount"]),
        row["nameOrig"],
        float(row["oldBalanceOrig"]),
        float(row["newBalanceOrig"]),
        row["nameDest"],
        float(row["oldBalanceDest"]),
        float(row["newBalanceDest"]),
        float(row["errorBalanceOrig"]),
        float(row["errorBalanceDest"]),
        float(row["risk_score"]),
        row["risk_tier"],
        int(row["isFraud"]) == 1
    ))

sql = """
INSERT INTO transactions (
    id, step, type, amount,
    name_orig, old_balance_orig, new_balance_orig,
    name_dest, old_balance_dest, new_balance_dest,
    error_balance_orig, error_balance_dest,
    risk_score, risk_tier, is_fraud_label
)
VALUES %s
ON CONFLICT (id) DO NOTHING
"""

execute_values(cur, sql, data, page_size=500)
conn.commit()
print(f"Da insert {len(data)} records thanh cong!")
cur.close()
conn.close()
