CREATE DATABASE IF NOT EXISTS hivemind;
USE hivemind;

CREATE TABLE IF NOT EXISTS transactions (
  id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  step               INT         NOT NULL,
  type               STRING      NOT NULL,
  amount             FLOAT       NOT NULL,
  name_orig          STRING      NOT NULL,
  old_balance_orig   FLOAT       NOT NULL,
  new_balance_orig   FLOAT       NOT NULL,
  name_dest          STRING      NOT NULL,
  old_balance_dest   FLOAT       NOT NULL,
  new_balance_dest   FLOAT       NOT NULL,
  error_balance_orig FLOAT       NOT NULL,
  error_balance_dest FLOAT       NOT NULL,
  risk_score         FLOAT       NOT NULL,
  risk_tier          STRING      NOT NULL,
  is_fraud_label     BOOLEAN     NOT NULL,
  arrived_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at       TIMESTAMPTZ,
  CONSTRAINT type_check CHECK (type IN ('TRANSFER', 'CASH_OUT')),
  CONSTRAINT tier_check CHECK (risk_tier IN ('low', 'medium', 'high'))
);

CREATE INDEX ON transactions (risk_tier, arrived_at DESC);
CREATE INDEX ON transactions (name_orig);
CREATE INDEX ON transactions (name_dest);
CREATE INDEX ON transactions (step, type);

CREATE TABLE IF NOT EXISTS tasks (
  id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  transaction_id   UUID        NOT NULL UNIQUE REFERENCES transactions(id),
  risk_score       FLOAT       NOT NULL,
  status           STRING      NOT NULL DEFAULT 'pending',
  claimed_by       STRING,
  claimed_at       TIMESTAMPTZ,
  heartbeat_at     TIMESTAMPTZ,
  completed_at     TIMESTAMPTZ,
  step             STRING,
  scratchpad       JSONB,
  verdict          STRING,
  confidence       FLOAT,
  reviewed_by      STRING,
  reviewed_at      TIMESTAMPTZ,
  review_decision  STRING,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT status_check CHECK (
    status IN ('pending','claimed','investigating','done','failed','escalated','pending_review')
  ),
  CONSTRAINT verdict_check CHECK (
    verdict IS NULL OR verdict IN ('fraud','legit','escalate')
  ),
  CONSTRAINT review_decision_check CHECK (
    review_decision IS NULL OR review_decision IN ('approved','rejected')
  )
);

CREATE INDEX ON tasks (status, created_at ASC) WHERE status = 'pending';
CREATE INDEX ON tasks (heartbeat_at) WHERE status IN ('claimed','investigating');
CREATE INDEX ON tasks (status, completed_at DESC);
CREATE INDEX ON tasks (verdict, completed_at DESC) WHERE verdict IS NOT NULL;
CREATE INDEX ON tasks (status) WHERE status = 'pending_review';

CREATE TABLE IF NOT EXISTS case_memory (
  id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  summary           TEXT        NOT NULL,
  verdict           STRING      NOT NULL,
  confidence_avg    FLOAT,
  pattern_type      STRING,
  key_signals       STRING[],
  transaction_type  STRING,
  amount_range      STRING,
  error_orig_sign   STRING,
  error_dest_sign   STRING,
  embedding         VECTOR(1024),
  salience          FLOAT       NOT NULL DEFAULT 1.0,
  recall_count      INT         NOT NULL DEFAULT 0,
  merge_count       INT         NOT NULL DEFAULT 1,
  archived          BOOLEAN     NOT NULL DEFAULT false,
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

CREATE VECTOR INDEX ON case_memory (embedding) WHERE archived = false;
CREATE INDEX ON case_memory (transaction_type, verdict, archived);
CREATE INDEX ON case_memory (pattern_type, verdict) WHERE archived = false;
CREATE INDEX ON case_memory (salience, last_recalled_at) WHERE archived = false;

CREATE TABLE IF NOT EXISTS audit_log (
  id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id           UUID        NOT NULL REFERENCES tasks(id),
  transaction_id    UUID        NOT NULL REFERENCES transactions(id),
  agent_id          STRING      NOT NULL,
  action            STRING      NOT NULL,
  reasoning         TEXT,
  memory_hits       INT,
  similarity_scores FLOAT[],
  tokens_in         INT,
  tokens_out        INT,
  bedrock_model     STRING,
  latency_ms        INT,
  reviewer_id       STRING,
  review_notes      TEXT,
  evidence_s3_key   STRING,
  context_snapshot  JSONB,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT action_ck CHECK (
    action IN (
      'mcp_query', 'memory_recall', 'bedrock_reasoning',
      'verdict_fraud', 'verdict_legit', 'verdict_escalate',
      'auto_approve', 'auto_block',
      'task_claimed', 'task_resumed', 'task_failed', 'task_requeued',
      'human_reviewed'
    )
  )
);

CREATE INDEX ON audit_log (task_id, created_at ASC);
CREATE INDEX ON audit_log (agent_id, created_at DESC);
CREATE INDEX ON audit_log (action, created_at DESC);
CREATE INDEX ON audit_log (bedrock_model, created_at DESC) WHERE tokens_in IS NOT NULL;

CREATE VIEW IF NOT EXISTS agent_performance AS
SELECT
  agent_id,
  DATE_TRUNC('hour', created_at)      AS hour,
  COUNT(*)                            AS total_actions,
  SUM(tokens_in + tokens_out)         AS total_tokens,
  ROUND(AVG(latency_ms)::NUMERIC, 0)  AS avg_latency_ms,
  MAX(latency_ms)                     AS max_latency_ms,
  AVG(memory_hits)                    AS avg_memory_hits
FROM audit_log
WHERE tokens_in IS NOT NULL
GROUP BY agent_id, DATE_TRUNC('hour', created_at);

CREATE VIEW IF NOT EXISTS task_summary AS
SELECT
  status,
  verdict,
  COUNT(*)                                       AS count,
  AVG(EXTRACT(EPOCH FROM (completed_at - created_at))) AS avg_duration_sec,
  AVG(confidence)                                AS avg_confidence,
  DATE_TRUNC('hour', created_at)                 AS hour
FROM tasks
GROUP BY status, verdict, DATE_TRUNC('hour', created_at);

CREATE VIEW IF NOT EXISTS fraud_accuracy AS
SELECT
  t.verdict                                AS agent_verdict,
  tx.is_fraud_label                        AS ground_truth,
  COUNT(*)                                 AS count,
  ROUND(COUNT(*) * 100.0 / SUM(COUNT(*)) OVER (), 2) AS pct
FROM tasks t
JOIN transactions tx ON t.transaction_id = tx.id
WHERE t.verdict IS NOT NULL
GROUP BY t.verdict, tx.is_fraud_label
ORDER BY t.verdict, tx.is_fraud_label;
