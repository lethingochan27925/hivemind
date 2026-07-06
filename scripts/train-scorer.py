"""
HiveMind — Train XGBoost Fraud Scorer
======================================
Dựa trên notebook: predicting-fraud-in-financial-payment-services
Dataset: PaySim (CC BY 4.0) — kaggle.com/datasets/ealaxi/paysim1

Chạy: python train_scorer.py --csv PS_20174392719_1491204439457_log.csv
Output: fraud_scorer.pkl (XGBoost model)
        scorer_eval.txt  (metrics: AUPRC, confusion matrix)

Thời gian train: ~5-10 phút trên CPU thường với 6M rows
                 ~1-2 phút nếu dùng Kaggle GPU notebook
"""

import argparse
import json
import joblib
import pandas as pd
import numpy as np
from sklearn.model_selection import train_test_split
from sklearn.metrics import (
    average_precision_score, roc_auc_score,
    precision_score, recall_score, f1_score,
    confusion_matrix
)
from xgboost import XGBClassifier


def load_and_prepare(csv_path: str) -> tuple:
    """Load PaySim, engineer features, return X, y."""
    print(f"[train] Loading {csv_path}...")
    df = pd.read_csv(csv_path)

    # Rename theo notebook
    df = df.rename(columns={
        'oldbalanceOrg':  'oldBalanceOrig',
        'newbalanceOrig': 'newBalanceOrig',
        'oldbalanceDest': 'oldBalanceDest',
        'newbalanceDest': 'newBalanceDest',
    })

    # Chỉ dùng 2 loại có fraud
    df = df[df['type'].isin(['TRANSFER', 'CASH_OUT'])].copy()

    print(f"[train] Rows after filter: {len(df):,}")
    print(f"[train] Fraud rate: {df['isFraud'].mean():.2%}")

    # Engineer features (key insight từ notebook)
    df['errorBalanceOrig'] = (
        df['newBalanceOrig'] + df['amount'] - df['oldBalanceOrig']
    )
    df['errorBalanceDest'] = (
        df['oldBalanceDest'] + df['amount'] - df['newBalanceDest']
    )

    # Encode type
    df['type_encoded'] = (df['type'] == 'TRANSFER').astype(int)

    feature_cols = [
        'step', 'type_encoded',
        'amount',
        'oldBalanceOrig', 'newBalanceOrig',
        'oldBalanceDest', 'newBalanceDest',
        'errorBalanceOrig', 'errorBalanceDest',
    ]

    X = df[feature_cols]
    y = df['isFraud']
    return X, y


def train(X, y) -> XGBClassifier:
    """Train XGBoost với scale_pos_weight để xử lý imbalanced data."""
    # Tính weight tự động (theo notebook)
    weights = (y == 0).sum() / (1.0 * (y == 1).sum())
    print(f"[train] scale_pos_weight = {weights:.1f} "
          f"(fraud: {(y==1).sum():,} / non-fraud: {(y==0).sum():,})")

    clf = XGBClassifier(
        max_depth        = 3,
        scale_pos_weight = weights,
        n_jobs           = -1,
        random_state     = 42,
        eval_metric      = 'aucpr',
        # Tăng max_depth lên 5-7 nếu muốn accuracy cao hơn
        # nhưng train lâu hơn
    )
    return clf


def evaluate(clf, X_test, y_test) -> dict:
    """Tính metrics và in ra."""
    proba = clf.predict_proba(X_test)[:, 1]

    # Dùng threshold 0.5 cho classification metrics
    pred = (proba >= 0.5).astype(int)

    auprc  = average_precision_score(y_test, proba)
    auroc  = roc_auc_score(y_test, proba)
    prec   = precision_score(y_test, pred, zero_division=0)
    rec    = recall_score(y_test, pred, zero_division=0)
    f1     = f1_score(y_test, pred, zero_division=0)
    cm     = confusion_matrix(y_test, pred)

    metrics = {
        'auprc':     round(auprc, 4),
        'auroc':     round(auroc, 4),
        'precision': round(prec, 4),
        'recall':    round(rec, 4),
        'f1':        round(f1, 4),
        'confusion_matrix': cm.tolist(),
        'threshold': 0.5,
    }

    print(f"\n[eval] ── Metrics ──────────────────────────")
    print(f"  AUPRC     : {auprc:.4f}  (notebook target: 0.997)")
    print(f"  AUROC     : {auroc:.4f}")
    print(f"  Precision : {prec:.4f}")
    print(f"  Recall    : {rec:.4f}")
    print(f"  F1        : {f1:.4f}")
    print(f"  Confusion matrix:")
    print(f"    TN={cm[0][0]:>7,}  FP={cm[0][1]:>6,}")
    print(f"    FN={cm[1][0]:>7,}  TP={cm[1][1]:>6,}")

    # Routing distribution ở các threshold HiveMind
    for thresh_name, thresh in [('low(<0.30)', 0.30), ('high(>0.85)', 0.85)]:
        if 'low' in thresh_name:
            n = (proba < thresh).sum()
        else:
            n = (proba > thresh).sum()
        print(f"  {thresh_name}: {n:,} ({n/len(proba):.1%})")
    medium = ((proba >= 0.30) & (proba <= 0.85)).sum()
    print(f"  medium(agent): {medium:,} ({medium/len(proba):.1%})")

    return metrics


def main(csv_path: str, output_model: str, output_eval: str) -> None:
    # Load & prepare
    X, y = load_and_prepare(csv_path)

    # Split (time-aware: giữ thứ tự step)
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42, shuffle=False
    )
    print(f"\n[train] Train: {len(X_train):,} | Test: {len(X_test):,}")

    # Train
    clf = train(X_train, y_train)
    print(f"\n[train] Fitting model...")
    clf.fit(X_train, y_train,
            eval_set=[(X_test, y_test)],
            verbose=False)

    # Evaluate
    metrics = evaluate(clf, X_test, y_test)

    # Save model
    joblib.dump(clf, output_model)
    print(f"\n[train] Model saved → {output_model}")

    # Save metrics
    with open(output_eval, 'w') as f:
        json.dump(metrics, f, indent=2)
    print(f"[train] Metrics saved → {output_eval}")

    # Quick smoke test: predict 3 samples
    print(f"\n[train] Smoke test (3 samples):")
    samples = X_test.iloc[:3]
    scores  = clf.predict_proba(samples)[:, 1]
    labels  = y_test.iloc[:3].values
    for i, (score, label) in enumerate(zip(scores, labels)):
        tier = 'low' if score < 0.30 else ('high' if score > 0.85 else 'medium')
        print(f"  [{i}] risk_score={score:.4f} → {tier:6s} | ground_truth={'FRAUD' if label else 'legit'}")


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Train HiveMind fraud scorer')
    parser.add_argument('--csv',   required=True,
                        help='PaySim CSV path')
    parser.add_argument('--model', default='fraud_scorer.pkl',
                        help='Output model path')
    parser.add_argument('--eval',  default='scorer_eval.json',
                        help='Output metrics JSON path')
    args = parser.parse_args()

    main(args.csv, args.model, args.eval)