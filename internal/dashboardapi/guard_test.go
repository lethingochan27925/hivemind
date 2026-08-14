package dashboardapi

import "testing"

// TestMutatingPatternMatchesWordsNotSubstrings pins the fix for a real defect:
// the read-only guard used strings.Contains, so "created_at" - an ordinary
// read-only column - matched the keyword "create" and every time-filtered query
// was rejected with 400. The console could not answer "what happened in the last
// hour", and the evidence capture script was failing silently for the same
// reason. Matching on word boundaries keeps DDL blocked without blocking columns
// that merely start with a keyword.
func TestMutatingPatternMatchesWordsNotSubstrings(t *testing.T) {
	mustAllow := []string{
		"SELECT action, created_at FROM audit_log ORDER BY created_at DESC",
		"SELECT * FROM tasks WHERE completed_at >= now() - INTERVAL '1 hour'",
		"SELECT updated_at FROM system_policy",
		"SELECT last_recalled_at, last_merged_at FROM case_memory",
		"SELECT COUNT(*) FROM audit_log WHERE created_at > '2026-01-01'",
		"WITH recent AS (SELECT id FROM tasks WHERE created_at > now()) SELECT * FROM recent",
		"SELECT insert_ts FROM some_view",
		"SELECT * FROM t WHERE dropped_flag = false",
	}
	for _, q := range mustAllow {
		if mutatingRe.MatchString(q) {
			t.Errorf("read-only query wrongly rejected: %s", q)
		}
	}

	mustBlock := []string{
		"DELETE FROM tasks",
		"delete from tasks",
		"DROP TABLE tasks",
		"UPDATE tasks SET status = 'done'",
		"INSERT INTO tasks (id) VALUES ('x')",
		"UPSERT INTO tasks (id) VALUES ('x')",
		"TRUNCATE tasks",
		"ALTER TABLE tasks ADD COLUMN x INT",
		"CREATE TABLE t (id INT)",
		"GRANT SELECT ON tasks TO analyst",
		"REVOKE SELECT ON tasks FROM analyst",
		"COMMENT ON TABLE tasks IS 'x'",
		"COMMENT   ON TABLE tasks IS 'x'",
		"SELECT 1; DROP TABLE tasks",
		"WITH x AS (DELETE FROM tasks RETURNING id) SELECT * FROM x",
	}
	for _, q := range mustBlock {
		if !mutatingRe.MatchString(q) {
			t.Errorf("mutating statement slipped through: %s", q)
		}
	}
}
