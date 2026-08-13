# syntax=docker/dockerfile:1.7
FROM public.ecr.aws/lambda/python:3.12 AS builder

COPY services/scoring-python/requirements.txt .

# BuildKit cache mount: wheel cache song giua cac lan build. Lan dau tai binh
# thuong; tu do ve sau chi tai goi moi, khong tai lai pandas/sklearn tu dau.
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install -r requirements.txt --target /opt/python-deps

RUN --mount=type=cache,target=/root/.cache/pip \
    pip install --no-deps xgboost==3.3.0 --target /opt/python-deps

# Cat tests/__pycache__ khoi deps (hang tram MB khong dung o runtime).
# Base image AL2023 khong co `find`, nen dung chinh Python de don.
RUN python -c "import pathlib, shutil; \
    r = pathlib.Path('/opt/python-deps'); \
    [shutil.rmtree(p, ignore_errors=True) for p in list(r.rglob('__pycache__'))]; \
    [shutil.rmtree(p, ignore_errors=True) for p in list(r.rglob('tests')) if p.is_dir()]; \
    [p.unlink() for p in list(r.rglob('*.pyc')) if p.is_file()]"

FROM public.ecr.aws/lambda/python:3.12

COPY --from=builder /opt/python-deps ${LAMBDA_TASK_ROOT}
COPY services/scoring-python/main.py ${LAMBDA_TASK_ROOT}/
COPY data/processed/fraud_scorer.pkl ${LAMBDA_TASK_ROOT}/data/processed/fraud_scorer.pkl

CMD ["main.handler"]
