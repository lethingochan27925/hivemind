"""
HiveMind — PaySim Adapter
=========================
Đọc PaySim CSV → tính engineered features → score bằng XGBoost
→ routing theo risk_tier → insert vào CockroachDB transactions table

Chạy: python adapter.py --csv PS_20174392719_1491204439457_log.csv
                        --model fraud_scorer.pkl
                        --db postgresql://root@localhost:26257/hivemind
                        --mode replay   # replay | stream
                        --limit 1000    # số row cần replay (demo: 500-1000)
"""

import argparse
import time
import uuid
import joblib
import pandas as pd
import numpy as np
import psycopg2
from psycopg2.extras import execute_batch


# ── Routing thresholds ──────────────────────────────────────
LOW_THRESHOLD  = 0.001  # < 0.001 → low (auto approve)
HIGH_THRESHOLD = 0.999  # > 0.999 → high (auto block)
# 0.001..0.999   → HiveMind agent investigates (~15-25% tổng giao dịch)


def load_paysim(csv_path: str, limit: int = None) -> pd.DataFrame:
    """Load PaySim CSV, rename columns theo notebook, filter TRANSFER+CASH_OUT."""
    df = pd.read_csv(csv_path, nrows=limit)

    # Rename theo notebook (nhất quán với model đã train)
    df = df.rename(columns={
        'oldbalanceOrg':  'oldBalanceOrig',
        'newbalanceOrig': 'newBalanceOrig',
        'oldbalanceDest': 'oldBalanceDest',
        'newbalanceDest': 'newBalanceDest',
    })

    # Chỉ lấy 2 loại có fraud (theo EDA trong notebook)
    df = df[df['type'].isin(['TRANSFER', 'CASH_OUT'])].copy()
    df = df.reset_index(drop=True)

    print(f"[adapter] Loaded {len(df):,} rows (TRANSFER + CASH_OUT only)")
    print(f"[adapter] Fraud rate: {df['isFraud'].mean():.1%}")
    return df


def engineer_features(df: pd.DataFrame) -> pd.DataFrame:
    """Tính engineered features — key insight từ notebook."""
    # Lỗi số dư = dấu hiệu gian lận mạnh nhất trong PaySim
    df['errorBalanceOrig'] = (
        df['newBalanceOrig'] + df['amount'] - df['oldBalanceOrig']
    )
    df['errorBalanceDest'] = (
        df['oldBalanceDest'] + df['amount'] - df['newBalanceDest']
    )
    return df


def score_transactions(df: pd.DataFrame, model) -> pd.DataFrame:
    """Dùng XGBoost model để tính risk_score cho mỗi giao dịch."""
    # Features mà model đã được train (theo notebook)
    feature_cols = [
        'step', 'type',
        'amount',
        'oldBalanceOrig', 'newBalanceOrig',
        'oldBalanceDest', 'newBalanceDest',
        'errorBalanceOrig', 'errorBalanceDest',
    ]

    # Encode type (model cần số)
    df_model = df[feature_cols].copy()
    df_model['type'] = (df_model['type'] == 'TRANSFER').astype(int)

    risk_scores = model.predict_proba(df_model)[:, 1]
    df['risk_score'] = risk_scores

    # Routing
    df['risk_tier'] = 'medium'
    df.loc[df['risk_score'] < LOW_THRESHOLD,  'risk_tier'] = 'low'
    df.loc[df['risk_score'] > HIGH_THRESHOLD, 'risk_tier'] = 'high'

    # Summary
    tier_counts = df['risk_tier'].value_counts()
    print(f"[scorer] low={tier_counts.get('low',0):,} "
          f"medium={tier_counts.get('medium',0):,} "
          f"high={tier_counts.get('high',0):,}")
    return df


def amount_range(amount: float) -> str:
    """Phân loại amount range cho pre-filter vector search."""
    if amount < 10_000:
        return 'low'
    elif amount < 100_000:
        return 'mid'
    return 'high'


def sign_label(val: float) -> str:
    """Phân loại dấu error balance cho pre-filter."""
    if abs(val) < 1.0:
        return 'near_zero'
    return 'positive' if val > 0 else 'negative'


def insert_transactions(df: pd.DataFrame, conn) -> int:
    """Batch insert vào CockroachDB transactions table."""
    sql = """
    INSERT INTO transactions (
        id, step, type, amount,
        name_orig, old_balance_orig, new_balance_orig,
        name_dest, old_balance_dest, new_balance_dest,
        error_balance_orig, error_balance_dest,
        risk_score, risk_tier, is_fraud_label,
        arrived_at
    ) VALUES (
        %s, %s, %s, %s,
        %s, %s, %s,
        %s, %s, %s,
        %s, %s,
        %s, %s, %s,
        now()
    )
    ON CONFLICT DO NOTHING
    """

    rows = []
    for _, row in df.iterrows():
        rows.append((
            str(uuid.uuid4()),
            int(row['step']),
            row['type'],
            float(row['amount']),
            row['nameOrig'],
            float(row['oldBalanceOrig']),
            float(row['newBalanceOrig']),
            row['nameDest'],
            float(row['oldBalanceDest']),
            float(row['newBalanceDest']),
            float(row['errorBalanceOrig']),
            float(row['errorBalanceDest']),
            float(row['risk_score']),
            row['risk_tier'],
            bool(row['isFraud']),
        ))

    with conn.cursor() as cur:
        execute_batch(cur, sql, rows, page_size=500)
    conn.commit()
    return len(rows)


def insert_medium_tasks(df: pd.DataFrame, conn) -> int:
    """Insert task vào working memory cho các giao dịch risk_tier=medium."""
    medium = df[df['risk_tier'] == 'medium']
    if medium.empty:
        return 0

    # Lấy transaction_id từ DB (vừa insert ở bước trước)
    name_origs = tuple(medium['nameOrig'].tolist())
    with conn.cursor() as cur:
        cur.execute(
            "SELECT id, risk_score FROM transactions "
            "WHERE name_orig = ANY(%s) AND risk_tier = 'medium' "
            "ORDER BY arrived_at DESC",
            (list(name_origs),)
        )
        txn_rows = cur.fetchall()

    if not txn_rows:
        return 0

    task_sql = """
    INSERT INTO tasks (id, transaction_id, risk_score, status, created_at)
    VALUES (%s, %s, %s, 'pending', now())
    ON CONFLICT DO NOTHING
    """
    task_rows = [
        (str(uuid.uuid4()), str(txn_id), float(risk_score))
        for txn_id, risk_score in txn_rows
    ]

    with conn.cursor() as cur:
        execute_batch(cur, task_sql, task_rows, page_size=500)
    conn.commit()
    return len(task_rows)


def build_demo_stream(df: pd.DataFrame, n: int = 500) -> pd.DataFrame:
    """
    Controlled replay stream cho demo:
    - 400 legit cases (data thật)
    - Pattern card_testing lần 1 tại t=50  → agent mò mẫm
    - Pattern card_testing lần 2 tại t=150 → agent recall memory, xử lý nhanh
    """
    legit = df[df['isFraud'] == 0].head(n - 2).copy()
    fraud_pool = df[df['isFraud'] == 1]

    # Lấy 2 case fraud giống nhau nhất (cùng loại, amount gần nhau)
    transfer_fraud = fraud_pool[fraud_pool['type'] == 'TRANSFER']
    if len(transfer_fraud) >= 2:
        case1 = transfer_fraud.iloc[0:1].copy()
        case2 = transfer_fraud.iloc[1:2].copy()
    else:
        case1 = fraud_pool.iloc[0:1].copy()
        case2 = fraud_pool.iloc[1:2].copy()

    # Ép fraud về medium để agent xử lý (PaySim score fraud ≈ 1.0 → auto block)
    case1['risk_score'] = 0.50
    case1['risk_tier']  = 'medium'
    case2['risk_score'] = 0.52
    case2['risk_tier']  = 'medium'

    # Thêm uncertain cases để fleet có việc làm
    legit_sample = df[df['isFraud'] == 0].sample(n=20, random_state=42).copy()
    legit_sample['risk_score'] = np.random.uniform(0.1, 0.8, len(legit_sample))
    legit_sample['risk_tier']  = 'medium'

    # Build stream với controlled insertion points
    stream_parts = []
    stream_parts.append(legit.iloc[:50])    # 0-49: legit
    stream_parts.append(case1)              # 50: fraud pattern lần 1
    stream_parts.append(legit.iloc[50:150]) # 51-149: legit
    stream_parts.append(case2)              # 150: fraud pattern lần 2 (same pattern)
    stream_parts.append(legit_sample)
    stream_parts.append(legit.iloc[150:])   # 151+: legit

    stream = pd.concat(stream_parts, ignore_index=True)
    print(f"[demo] Stream built: {len(stream):,} rows")
    print(f"[demo] Fraud injected at positions 50 and 151")
    return stream


def replay(csv_path: str, model_path: str, db_url: str,
           mode: str = 'replay', limit: int = 1000,
           delay_ms: int = 50) -> None:
    """Main replay loop."""
    print(f"\n[adapter] Loading model from {model_path}")
    model = joblib.load(model_path)

    print(f"[adapter] Loading PaySim from {csv_path} (limit={limit})")
    df = load_paysim(csv_path, limit=limit * 3)  # load thêm để filter TRANSFER+CASH_OUT
    df = engineer_features(df)
    df = score_transactions(df, model)

    if mode == 'demo':
        df = build_demo_stream(df, n=min(limit, 500))

    print(f"\n[adapter] Connecting to CockroachDB...")
    conn = psycopg2.connect(db_url)

    total_inserted = 0
    total_tasks = 0
    batch_size = 100

    print(f"[adapter] Starting {'demo' if mode=='demo' else 'replay'} stream...\n")
    for i in range(0, len(df), batch_size):
        batch = df.iloc[i:i+batch_size]

        n_inserted = insert_transactions(batch, conn)
        n_tasks = insert_medium_tasks(batch, conn)

        total_inserted += n_inserted
        total_tasks += n_tasks

        print(f"  [{i+len(batch):>5}/{len(df)}] "
              f"inserted={n_inserted} tasks_queued={n_tasks} "
              f"(total_tasks={total_tasks})")

        if delay_ms > 0:
            time.sleep(delay_ms / 1000)

    conn.close()
    print(f"\n[adapter] Done.")
    print(f"  Total transactions inserted : {total_inserted:,}")
    print(f"  Total tasks queued (medium) : {total_tasks:,}")
    print(f"  → Fleet agents can now start claiming tasks\n")


# ── CLI ──────────────────────────────────────────────────────
if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='HiveMind PaySim Adapter')
    parser.add_argument('--csv',   required=True,
                        help='Path to PaySim CSV file')
    parser.add_argument('--model', required=True,
                        help='Path to trained XGBoost model (.pkl)')
    parser.add_argument('--db',    required=True,
                        help='CockroachDB connection URL (postgresql://...)')
    parser.add_argument('--mode',  default='replay',
                        choices=['replay', 'demo'],
                        help='replay=all data, demo=controlled stream for video')
    parser.add_argument('--limit', type=int, default=1000,
                        help='Max rows to process')
    parser.add_argument('--delay', type=int, default=50,
                        help='Delay between batches in ms (0=max speed)')
    args = parser.parse_args()

    replay(
        csv_path=args.csv,
        model_path=args.model,
        db_url=args.db,
        mode=args.mode,
        limit=args.limit,
        delay_ms=args.delay,
    )