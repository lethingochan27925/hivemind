FROM public.ecr.aws/lambda/python:3.12 AS builder

COPY services/scoring-python/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt --target /opt/python-deps

RUN pip install --no-cache-dir --no-deps xgboost==3.3.0 --target /opt/python-deps

FROM public.ecr.aws/lambda/python:3.12

COPY --from=builder /opt/python-deps ${LAMBDA_TASK_ROOT}
COPY services/scoring-python/main.py ${LAMBDA_TASK_ROOT}/
COPY data/processed/fraud_scorer.pkl ${LAMBDA_TASK_ROOT}/data/processed/fraud_scorer.pkl

CMD ["main.handler"]
