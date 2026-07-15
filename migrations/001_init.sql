-- ============================================================
-- HiveMind — CockroachDB Schema (PaySim dataset)
-- ============================================================
-- Dataset: PaySim Synthetic Financial Dataset
-- License: CC BY 4.0 — dùng được cho commercial/hackathon
-- Attribution: "PaySim dataset (NTNU, CC BY 4.0)"
-- Source: kaggle.com/datasets/ealaxi/paysim1
-- ============================================================
-- Chạy: cockroach sql --url $COCKROACHDB_URL < schema.sql
-- ============================================================

CREATE DATABASE IF NOT EXISTS hivemind;
USE hivemind;

-- ============================================================
-- TABLE 1: transactions
-- Lưu giao dịch gốc từ PaySim replay stream
-- Dữ liệu: số + structured (machine reads)
-- ============================================================
CREATE TABLE IF NOT EXISTS transactions (
  id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),

  -- PaySim raw fields (giữ tên gốc để trace về dataset)
  step              INT         NOT NULL,     -- giờ trong simulation (1..743)
  type              STRING      NOT NULL,     -- TRANSFER | CASH_OUT (hai loại có fraud)
  amount            FLOAT       NOT NULL,     -- số tiền giao dịch (USD)
  name_orig         STRING      NOT NULL,     -- account gốc (C...)
  old_balance_orig  FLOAT       NOT NULL,     -- số dư trước giao dịch — tài khoản gốc
  new_balance_orig  FLOAT       NOT NULL,     -- số dư sau giao dịch — tài khoản gốc
  name_dest         STRING      NOT NULL,     -- account đích (C... hoặc M...)
  old_balance_dest  FLOAT       NOT NULL,     -- số dư trước giao dịch — tài khoản đích
  new_balance_dest  FLOAT       NOT NULL,     -- số dư sau giao dịch — tài khoản đích

  -- Engineered features (tính trước, lưu sẵn để agent đọc nhanh)
  -- Nguồn: notebook "predicting-fraud-in-financial-payment-services"
  -- errorBalanceOrig = newBalanceOrig + amount - oldBalanceOrig
  -- errorBalanceDest = oldBalanceDest + amount - newBalanceDest
  -- Fraud thường để lại "lỗi số dư" — signal mạnh nhất trong PaySim
  error_balance_orig FLOAT      NOT NULL,     -- lỗi số dư tài khoản gốc
  error_balance_dest FLOAT      NOT NULL,     -- lỗi số dư tài khoản đích

  -- Risk scoring output (từ XGBoost model)
  risk_score        FLOAT       NOT NULL,     -- predict_proba()[:, 1] ∈ [0..1]
  risk_tier         STRING      NOT NULL,
  -- low    = risk_score < 0.30  → auto approve
  -- high   = risk_score > 0.85  → auto block
  -- medium = 0.30..0.85         → HiveMind agent investigates

  -- Ground truth (từ PaySim label, dùng để eval verdict accuracy)
  is_fraud_label    BOOLEAN     NOT NULL,     -- isFraud gốc từ dataset

  -- Metadata
  arrived_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at      TIMESTAMPTZ,

  CONSTRAINT type_check CHECK (type IN ('TRANSFER', 'CASH_OUT')),
  CONSTRAINT tier_check CHECK (risk_tier IN ('low', 'medium', 'high'))
);

CREATE INDEX ON transactions (risk_tier, arrived_at DESC);
CREATE INDEX ON transactions (name_orig);
CREATE INDEX ON transactions (name_dest);
CREATE INDEX ON transactions (step, type);


-- ============================================================
-- TABLE 2: tasks  (Working Memory)
-- Hàng đợi điều tra cho agent fleet
-- Dữ liệu: số + JSONB (machine reads, transactional)
-- ============================================================
CREATE TABLE IF NOT EXISTS tasks (
  id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),

  -- Link về giao dịch gốc
  transaction_id    UUID        NOT NULL REFERENCES transactions(id),
  risk_score        FLOAT       NOT NULL,     -- copy để agent đọc nhanh

  -- Task lifecycle
  status            STRING      NOT NULL DEFAULT 'pending',
  -- pending → claimed → investigating → done | failed | escalated

  -- Agent tracking
  claimed_by        STRING,                   -- Lambda request id
  claimed_at        TIMESTAMPTZ,
  heartbeat_at      TIMESTAMPTZ,             -- agent ghi mỗi 10s; quá 30s → re-queue
  completed_at      TIMESTAMPTZ,

  -- Resume-after-crash state
  step              STRING,
  -- mcp_query | memory_recall | bedrock_reasoning | verdict
  scratchpad        JSONB,
  -- {
  --   "mcp_result": { account_history: [...], balance_pattern: "..." },
  --   "recalled_cases": [ { id, summary, similarity, verdict } ],
  --   "partial_reasoning": "...",
  --   "retry_count": 0
  -- }

  -- Final output
  verdict           STRING,                   -- fraud | legit | escalate
  confidence        FLOAT,                    -- agent confidence [0..1]

  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT status_check CHECK (
    status IN ('pending','claimed','investigating','done','failed','escalated')
  ),
  CONSTRAINT verdict_check CHECK (
    verdict IS NULL OR verdict IN ('fraud','legit','escalate')
  )
);

-- Index cho fleet polling: SELECT FOR UPDATE SKIP LOCKED
CREATE INDEX ON tasks (status, created_at ASC) WHERE status = 'pending';
-- Index cho heartbeat monitor (re-queue stale tasks)
CREATE INDEX ON tasks (heartbeat_at) WHERE status IN ('claimed','investigating');
-- Index cho dashboard
CREATE INDEX ON tasks (status, completed_at DESC);
CREATE INDEX ON tasks (verdict, completed_at DESC) WHERE verdict IS NOT NULL;


-- ============================================================
-- TABLE 3: case_memory  (Episodic Memory)
-- Bộ nhớ dài hạn của fleet — hai lớp tách biệt
-- Lớp TEXT  → agent đọc khi recall (ngôn ngữ tự nhiên)
-- Lớp VECTOR → machine search (số, 1024 chiều)
-- ============================================================
CREATE TABLE IF NOT EXISTS case_memory (
  id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),

  -- ── Lớp TEXT (agent đọc khi recall) ──────────────────────
  summary           TEXT        NOT NULL,
  -- Ví dụ:
  -- "TRANSFER pattern: amount=450,000, errorBalanceOrig≈0 (balance wiped),
  --  errorBalanceDest=450,000 (dest never updated), step=183 (late hour).
  --  Verdict: FRAUD. Key signals: balance_wipe, dest_error, high_amount.
  --  Confidence: 0.93. Seen 3 times, last: 2026-07-06."

  verdict           STRING      NOT NULL,     -- fraud | legit | escalate
  confidence_avg    FLOAT,                    -- avg confidence của các case merge

  -- PaySim fraud patterns
  pattern_type      STRING,
  -- balance_wipe      → oldBalanceOrig = amount, newBalanceOrig = 0
  -- dest_no_update    → errorBalanceDest lớn
  -- high_amount_transfer → amount > 200,000
  -- rapid_cashout     → CASH_OUT ngay sau TRANSFER
  key_signals       STRING[],
  -- ["balance_wipe", "dest_error", "high_amount"]

  -- Statistical fingerprint (giúp pre-filter trước vector search)
  transaction_type  STRING,                   -- TRANSFER | CASH_OUT
  amount_range      STRING,                   -- low(<10k) | mid(10k-100k) | high(>100k)
  error_orig_sign   STRING,                   -- positive | negative | near_zero
  error_dest_sign   STRING,                   -- positive | negative | near_zero

  -- ── Lớp VECTOR (machine search) ──────────────────────────
  embedding         VECTOR(1024),
  -- Titan Embeddings v2, sinh từ summary
  -- Construction flow (async, sau khi agent đóng case):
  --   Bedrock tóm tắt → Titan embed →
  --   similarity > 0.92 với case hiện có?
  --     yes → merge (update summary, tăng merge_count)
  --     no  → insert mới

  -- ── GEM-inspired memory management ────────────────────────
  salience          FLOAT       NOT NULL DEFAULT 1.0,
  -- +0.1 mỗi lần được recall thành công
  -- ×0.95 mỗi 7 ngày không được recall
  -- < 0.10 → archived = true

  recall_count      INT         NOT NULL DEFAULT 0,
  merge_count       INT         NOT NULL DEFAULT 1,
  archived          BOOLEAN     NOT NULL DEFAULT false,

  -- Provenance
  source_task_id    UUID,
  data_source       STRING      NOT NULL DEFAULT 'paysim',
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_recalled_at  TIMESTAMPTZ,
  last_merged_at    TIMESTAMPTZ,

  CONSTRAINT verdict_ck  CHECK (verdict IN ('fraud','legit','escalate')),
  CONSTRAINT salience_ck CHECK (salience >= 0.0 AND salience <= 2.0),
  CONSTRAINT type_ck     CHECK (
    transaction_type IS NULL OR transaction_type IN ('TRANSFER','CASH_OUT')
  )
);

-- CockroachDB Distributed Vector Index
-- Chỉ index case active (archived = false) để tránh search trên case stale
CREATE VECTOR INDEX ON case_memory (embedding)
  WHERE archived = false;

-- Index cho pre-filter trước vector search (thu nhỏ search space)
CREATE INDEX ON case_memory (transaction_type, verdict, archived);
CREATE INDEX ON case_memory (pattern_type, verdict) WHERE archived = false;
-- Index cho salience decay background job
CREATE INDEX ON case_memory (salience, last_recalled_at) WHERE archived = false;


-- ============================================================
-- TABLE 4: audit_log  (Audit Memory)
-- Append-only — không UPDATE, không DELETE
-- Lớp TEXT: reasoning của model (compliance đọc)
-- Lớp số:   telemetry (machine reads)
-- ============================================================
CREATE TABLE IF NOT EXISTS audit_log (
  id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id           UUID        NOT NULL REFERENCES tasks(id),
  transaction_id    UUID        NOT NULL REFERENCES transactions(id),
  agent_id          STRING      NOT NULL,

  -- Action type
  action            STRING      NOT NULL,
  -- mcp_query         → agent query customer context qua MCP
  -- memory_recall     → agent tìm similar cases
  -- bedrock_reasoning → agent gọi LLM để reason
  -- verdict_fraud | verdict_legit | verdict_escalate
  -- auto_approve | auto_block    → risk_tier = low/high
  -- task_claimed | task_resumed | task_failed | task_requeued

  -- ── TEXT: lập luận của agent (compliance đọc) ─────────────
  reasoning         TEXT,
  -- Ví dụ:
  -- "Recalled 2 similar cases (similarity: 0.94, 0.89).
  --  Case 1: TRANSFER, balance_wipe pattern, Verdict: FRAUD (conf 0.91).
  --  Case 2: CASH_OUT after TRANSFER same account, Verdict: FRAUD (conf 0.88).
  --  Current case: errorBalanceOrig=450,000 (entire balance wiped),
  --  errorBalanceDest=450,000 (dest not updated). Pattern matches balance_wipe.
  --  Decision: FRAUD, confidence: 0.93."

  -- ── SỐ: telemetry (machine reads) ─────────────────────────
  memory_hits       INT,                      -- số case được recall
  similarity_scores FLOAT[],                  -- top-k scores [0.94, 0.89]
  tokens_in         INT,
  tokens_out        INT,
  bedrock_model     STRING,                   -- claude-haiku-* | claude-sonnet-*
  latency_ms        INT,

  -- Evidence pointer
  evidence_s3_key   STRING,                   -- s3://hivemind-evidence/<task_id>/raw.json

  -- Context snapshot (reproduce quyết định nếu cần audit)
  context_snapshot  JSONB,
  -- {
  --   "risk_score": 0.72,
  --   "error_balance_orig": 450000,
  --   "error_balance_dest": 450000,
  --   "transaction_type": "TRANSFER",
  --   "recalled_case_ids": ["uuid1", "uuid2"],
  --   "amount_range": "high"
  -- }

  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT action_ck CHECK (
    action IN (
      'mcp_query', 'memory_recall', 'bedrock_reasoning',
      'verdict_fraud', 'verdict_legit', 'verdict_escalate',
      'auto_approve', 'auto_block',
      'task_claimed', 'task_resumed', 'task_failed', 'task_requeued'
    )
  )
  -- Không có FOREIGN KEY về case_memory vì recall_case_ids lưu trong JSONB
);

CREATE INDEX ON audit_log (task_id, created_at ASC);
CREATE INDEX ON audit_log (agent_id, created_at DESC);
CREATE INDEX ON audit_log (action, created_at DESC);
CREATE INDEX ON audit_log (bedrock_model, created_at DESC)
  WHERE tokens_in IS NOT NULL;


-- ============================================================
-- VIEW: agent_performance  (Dashboard: cost/latency per agent)
-- ============================================================
CREATE VIEW IF NOT EXISTS agent_performance AS
SELECT
  agent_id,
  DATE_TRUNC('hour', created_at)              AS hour,
  COUNT(*)                                    AS total_actions,
  SUM(tokens_in + tokens_out)                 AS total_tokens,
  ROUND(AVG(latency_ms)::NUMERIC, 0)          AS avg_latency_ms,
  MAX(latency_ms)                             AS max_latency_ms,
  AVG(memory_hits)                            AS avg_memory_hits
FROM audit_log
WHERE tokens_in IS NOT NULL
GROUP BY agent_id, DATE_TRUNC('hour', created_at);


-- ============================================================
-- VIEW: task_summary  (Dashboard: fleet status + throughput)
-- ============================================================
CREATE VIEW IF NOT EXISTS task_summary AS
SELECT
  status,
  verdict,
  COUNT(*)                                    AS count,
  AVG(
    EXTRACT(EPOCH FROM (completed_at - created_at))
  )                                           AS avg_duration_sec,
  AVG(confidence)                             AS avg_confidence,
  DATE_TRUNC('hour', created_at)              AS hour
FROM tasks
GROUP BY status, verdict, DATE_TRUNC('hour', created_at);


-- ============================================================
-- VIEW: fraud_accuracy  (Eval: so sánh verdict vs ground truth)
-- ============================================================
CREATE VIEW IF NOT EXISTS fraud_accuracy AS
SELECT
  t.verdict                                   AS agent_verdict,
  tx.is_fraud_label                           AS ground_truth,
  COUNT(*)                                    AS count,
  ROUND(
    COUNT(*) * 100.0 / SUM(COUNT(*)) OVER (), 2
  )                                           AS pct
FROM tasks t
JOIN transactions tx ON t.transaction_id = tx.id
WHERE t.verdict IS NOT NULL
GROUP BY t.verdict, tx.is_fraud_label
ORDER BY t.verdict, tx.is_fraud_label;
-- Query này trả về confusion matrix:
-- verdict=fraud  + label=true  → True Positive
-- verdict=fraud  + label=false → False Positive
-- verdict=legit  + label=false → True Negative
-- verdict=legit  + label=true  → False Negative

CREATE TABLE IF NOT EXISTS episodic_memory (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID,
    summary        TEXT NOT NULL,
    verdict        STRING NOT NULL,
    confidence     FLOAT8,
    rationale      TEXT,
    embedding      VECTOR(1536),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT verdict_ck CHECK (verdict IN ('fraud', 'legit', 'escalate'))
);

CREATE VECTOR INDEX ON episodic_memory (embedding)
    WHERE embedding IS NOT NULL;


-- ============================================================
-- IMPORTANT QUERIES (copy vào code)
-- ============================================================

-- 1. Fleet claim task (mỗi agent worker gọi để nhận task mới)
-- SELECT id, transaction_id, risk_score, step, scratchpad
-- FROM tasks
-- WHERE status = 'pending'
-- ORDER BY created_at ASC
-- LIMIT 1
-- FOR UPDATE SKIP LOCKED;

-- 2. Heartbeat monitor (background job, chạy mỗi 15s)
-- UPDATE tasks
-- SET status = 'pending',
--     claimed_by = NULL,
--     step = step
-- WHERE status IN ('claimed', 'investigating')
--   AND heartbeat_at < now() - INTERVAL '30 seconds';

-- 3. Vector search top-3 similar cases
-- SELECT id, summary, verdict, key_signals, pattern_type,
--        salience, recall_count,
--        embedding <=> $1 AS distance
-- FROM case_memory
-- WHERE archived = false
--   AND transaction_type = $2        -- pre-filter: TRANSFER hoặc CASH_OUT
-- ORDER BY embedding <=> $1
-- LIMIT 3;

-- 4. Salience update sau recall thành công
-- UPDATE case_memory
-- SET salience          = LEAST(salience + 0.1, 2.0),
--     recall_count      = recall_count + 1,
--     last_recalled_at  = now()
-- WHERE id = ANY($1::UUID[]);       -- array các case_id vừa recall

-- 5. Salience decay background job (chạy mỗi 6 tiếng)
-- UPDATE case_memory
-- SET salience = salience * 0.95
-- WHERE archived = false
--   AND last_recalled_at < now() - INTERVAL '7 days';
--
-- UPDATE case_memory
-- SET archived = true
-- WHERE archived = false
--   AND salience < 0.10;

-- 6. Compliance query: toàn bộ action của một task
-- SELECT action, reasoning, tokens_in + tokens_out AS total_tokens,
--        latency_ms, similarity_scores, created_at
-- FROM audit_log
-- WHERE task_id = $1
-- ORDER BY created_at ASC;

-- ============================================================
-- NOTES
-- ============================================================
-- PaySim columns mapping từ CSV gốc:
--   step          → step
--   type          → type          (chỉ lấy TRANSFER + CASH_OUT)
--   amount        → amount
--   nameOrig      → name_orig
--   oldbalanceOrg → old_balance_orig  (rename theo notebook)
--   newbalanceOrig→ new_balance_orig
--   nameDest      → name_dest
--   oldbalanceDest→ old_balance_dest
--   newbalanceDest→ new_balance_dest
--   isFraud       → is_fraud_label
--   isFlaggedFraud→ DROP (không dùng, bất nhất như notebook đã phân tích)
--
-- Engineered features tính tại Dispatcher trước khi INSERT:
--   error_balance_orig = new_balance_orig + amount - old_balance_orig
--   error_balance_dest = old_balance_dest + amount - new_balance_dest
-- ============================================================