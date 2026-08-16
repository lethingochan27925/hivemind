-- 003_human_lesson_audit_action.sql: widen audit_log.action_ck to allow
-- 'human_lesson_stored' - the action internal/dashboardapi/learn.go writes
-- right after a human review is embedded and pinned into case_memory, as a
-- distinct signal from 'human_reviewed' (which marks the review DECISION
-- itself, written by the review-decision handlers).
--
-- Found by reading the schema against the code, not by a failing test: this
-- action value was never added to 001_init.sql's CHECK constraint, so every
-- single human_lesson_stored insert has been failing with a 23514
-- check_violation since the feature was written - silently, because the
-- call site does `_, _ = ...Exec(...)`. Reproduced directly against a real
-- CockroachDB container: INSERT ... action = 'human_lesson_stored' fails
-- with "failed to satisfy CHECK constraint action_ck" on the schema exactly
-- as 001_init.sql defines it. The memory write itself (case_memory INSERT +
-- salience pin) was never affected - only this secondary audit-trail row
-- silently never existed.
ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS action_ck;
ALTER TABLE audit_log ADD CONSTRAINT action_ck CHECK (
  action IN (
    'mcp_query', 'memory_recall', 'bedrock_reasoning',
    'verdict_fraud', 'verdict_legit', 'verdict_escalate',
    'auto_approve', 'auto_block',
    'task_claimed', 'task_resumed', 'task_failed', 'task_requeued',
    'human_reviewed', 'human_lesson_stored'
  )
);
