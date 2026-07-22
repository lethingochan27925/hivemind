"""
scoring_service.py — HTTP wrapper cho XGBoost fraud scorer.
Go scoring-api forward request toi day, service nay chi lo predict.
"""

import joblib
import numpy as np
import pandas as pd
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

app = FastAPI(title="HiveMind Scoring Service", version="1.0")

MODEL_PATH = "data/processed/fraud_scorer.pkl"
model = None


class ScoreRequest(BaseModel):
    step: int
    type: str
    amount: float
    oldBalanceOrig: float
    newBalanceOrig: float
    oldBalanceDest: float
    newBalanceDest: float
    errorBalanceOrig: float
    errorBalanceDest: float


class ScoreResponse(BaseModel):
    risk_score: float


@app.on_event("startup")
def load_model():
    global model
    model = joblib.load(MODEL_PATH)


@app.post("/score", response_model=ScoreResponse)
def score(req: ScoreRequest):
    if model is None:
        raise HTTPException(status_code=503, detail="model not loaded")

    df = pd.DataFrame([{
        "step": req.step,
        "type": 1 if req.type == "TRANSFER" else 0,
        "amount": req.amount,
        "oldBalanceOrig": req.oldBalanceOrig,
        "newBalanceOrig": req.newBalanceOrig,
        "oldBalanceDest": req.oldBalanceDest,
        "newBalanceDest": req.newBalanceDest,
        "errorBalanceOrig": req.errorBalanceOrig,
        "errorBalanceDest": req.errorBalanceDest,
    }])

    risk_score = float(model.predict_proba(df)[:, 1][0])
    return ScoreResponse(risk_score=risk_score)


@app.get("/health")
def health():
    return {"status": "ok", "model_loaded": model is not None}
