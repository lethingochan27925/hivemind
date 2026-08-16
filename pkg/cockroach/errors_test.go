package cockroach

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsColdStartMiss(t *testing.T) {
	undefinedTable := &pgconn.PgError{Code: pgUndefinedTable}
	otherPgError := &pgconn.PgError{Code: "42501"} // insufficient_privilege

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"no rows", pgx.ErrNoRows, true},
		{"wrapped no rows", errors.New("querying: " + pgx.ErrNoRows.Error()), false}, // string wrap doesn't satisfy errors.Is
		{"errors.Is-wrapped no rows", fmtWrap(pgx.ErrNoRows), true},
		{"undefined table", undefinedTable, true},
		{"other pg error", otherPgError, false},
		{"plain error", errors.New("connection refused"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsColdStartMiss(c.err); got != c.want {
				t.Errorf("IsColdStartMiss(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

func TestShouldWarn(t *testing.T) {
	undefinedTable := &pgconn.PgError{Code: pgUndefinedTable}

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil - nothing to warn about", nil, false},
		{"cold-start miss (no rows) - expected, don't warn", pgx.ErrNoRows, false},
		{"cold-start miss (undefined table) - expected, don't warn", undefinedTable, false},
		{"real error - must warn", errors.New("connection refused"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ShouldWarn(c.err); got != c.want {
				t.Errorf("ShouldWarn(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

// fmtWrap wraps err the way %w does, so errors.Is can still see through it -
// unlike formatting the error's text into a new error (which loses the
// chain, exercised as the "wrapped no rows" case above).
func fmtWrap(err error) error {
	return &wrapped{err}
}

type wrapped struct{ err error }

func (w *wrapped) Error() string { return "querying: " + w.err.Error() }
func (w *wrapped) Unwrap() error { return w.err }
