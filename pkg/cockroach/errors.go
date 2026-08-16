// errors.go: classifying errors that are expected on a cold start, shared by
// every package that reads a table dashboard-api creates lazily on first use
// (system_policy) rather than at schema-migration time.
package cockroach

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// pgUndefinedTable is Postgres/CockroachDB's SQLSTATE for "relation does not
// exist" (42P01).
const pgUndefinedTable = "42P01"

// IsColdStartMiss reports whether err is the normal, expected shape of
// "the table this query reads hasn't been created yet" — either the table is
// altogether missing (42P01) or exists but has no row yet (pgx.ErrNoRows).
// system_policy is created lazily by dashboardapi on first use, not by the
// schema migration, so a dispatcher or agent-worker Lambda that starts before
// anyone has opened the dashboard hitting either of these is a normal
// cold-start state, not a fault worth logging. Any other error (a dropped
// connection, a permission error, a timeout) is not — callers should still
// surface those.
func IsColdStartMiss(err error) bool {
	if errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUndefinedTable
}

// ShouldWarn reports whether a system_policy read failure is worth a log
// line: true for a real error (dropped connection, permission, timeout),
// false for nil or the expected cold-start miss. Pulled out because the
// exact expression `err != nil && !IsColdStartMiss(err)` was independently
// hand-written at two call sites (internal/agent/policy.go's LoadPolicy and
// internal/budget/budget.go's Exceeded, both reading the same table for the
// same reason) - one shared boolean means a third caller reuses it instead
// of writing a slightly different version. Callers keep their own log
// function and message wording; only the classification is shared.
func ShouldWarn(err error) bool {
	return err != nil && !IsColdStartMiss(err)
}
