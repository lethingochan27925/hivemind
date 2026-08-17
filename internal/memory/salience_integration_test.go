//go:build integration

package memory

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

// insertTestMemory inserts a minimal case_memory row with a controllable
// salience and last_recalled_at, and returns its id. embedding is a
// zero-vector of the right dimension — cheap and sufficient, since these
// tests never run a vector search.
func insertTestMemory(t *testing.T, db *cockroach.Client, salience float64, lastRecalledAt time.Time, archived bool) string {
	t.Helper()
	ctx := context.Background()
	id := uuid.New().String()
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO case_memory (
			id, summary, verdict, confidence_avg, pattern_type,
			transaction_type, amount_range, embedding,
			salience, last_recalled_at, archived, created_at
		) VALUES ($1, 'test memory', 'fraud', 0.9, 'test_pattern',
			'TRANSFER', 'low', $2, $3, $4, $5, now())
	`, id, cockroach.EncodeVector(make([]float32, 1024)), salience, lastRecalledAt, archived)
	if err != nil {
		t.Fatalf("inserting test case_memory: %v", err)
	}
	return id
}

func readSalience(t *testing.T, db *cockroach.Client, id string) (float64, bool) {
	t.Helper()
	ctx := context.Background()
	var salience float64
	var archived bool
	if err := db.Pool.QueryRow(ctx,
		`SELECT salience, archived FROM case_memory WHERE id = $1`, id,
	).Scan(&salience, &archived); err != nil {
		t.Fatalf("reading back case_memory %s: %v", id, err)
	}
	return salience, archived
}

func cleanupTestMemory(db *cockroach.Client, id string) {
	db.Pool.Exec(context.Background(), `DELETE FROM case_memory WHERE id = $1`, id)
}

var longAgo = time.Now().Add(-30 * 24 * time.Hour) // well past decayAfterDays
var justNow = time.Now().Add(-1 * time.Hour)       // well inside decayAfterDays

// TestDecaySalience_PinnedMemorySurvives is the regression test for the exact
// bug this change fixes: before the WHERE clause excluded salience >=
// pinnedSalience, a pinned (salience = 2.0) memory that had not been recalled
// in decayAfterDays would be multiplied down by decayFactor exactly like any
// other row, silently breaking the "decay can never erase what a human
// taught it" guarantee documented in README §5 and docs/CONTROL_PLANE.md.
func TestDecaySalience_PinnedMemorySurvives(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	id := insertTestMemory(t, db, pinnedSalience, longAgo, false)
	defer cleanupTestMemory(db, id)

	if err := DecaySalience(context.Background(), db); err != nil {
		t.Fatalf("DecaySalience failed: %v", err)
	}

	salience, archived := readSalience(t, db, id)
	if salience != pinnedSalience {
		t.Errorf("pinned salience after decay = %v, want unchanged %v", salience, pinnedSalience)
	}
	if archived {
		t.Error("pinned memory was archived by decay - it must never be")
	}
}

// TestDecaySalience_UnpinnedOldMemoryShrinks is the ordinary forgetting path:
// an unpinned memory not recalled in decayAfterDays must shrink by exactly
// decayFactor on one decay cycle.
func TestDecaySalience_UnpinnedOldMemoryShrinks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	id := insertTestMemory(t, db, 1.0, longAgo, false)
	defer cleanupTestMemory(db, id)

	if err := DecaySalience(context.Background(), db); err != nil {
		t.Fatalf("DecaySalience failed: %v", err)
	}

	salience, _ := readSalience(t, db, id)
	want := 1.0 * decayFactor
	if diff := salience - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("salience after one decay cycle = %v, want %v", salience, want)
	}
}

// TestDecaySalience_RecentlyRecalledMemoryUntouched: a memory recalled inside
// the decayAfterDays window must not move at all, pinned or not — recall
// activity is what "still useful" means here.
func TestDecaySalience_RecentlyRecalledMemoryUntouched(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	id := insertTestMemory(t, db, 1.0, justNow, false)
	defer cleanupTestMemory(db, id)

	if err := DecaySalience(context.Background(), db); err != nil {
		t.Fatalf("DecaySalience failed: %v", err)
	}

	salience, archived := readSalience(t, db, id)
	if salience != 1.0 {
		t.Errorf("recently-recalled salience after decay = %v, want unchanged 1.0", salience)
	}
	if archived {
		t.Error("recently-recalled memory was archived - it must not be")
	}
}

// TestDecaySalience_ArchivesBelowThreshold: an old, low-salience memory that
// decays below archiveBelow must come out archived in the same run — the two
// UPDATEs in DecaySalience are meant to compose into one atomic-looking step
// from the caller's point of view.
func TestDecaySalience_ArchivesBelowThreshold(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// archiveBelow / decayFactor sits just above the threshold pre-decay, so
	// one multiplication by decayFactor reliably pushes it under archiveBelow.
	// The epsilon must be subtracted, not added: archiveBelow/decayFactor*decayFactor
	// == archiveBelow exactly, and decayFactor < 1, so adding epsilon before
	// multiplying leaves the result epsilon*decayFactor *above* archiveBelow
	// instead of below it.
	id := insertTestMemory(t, db, archiveBelow/decayFactor-0.001, longAgo, false)
	defer cleanupTestMemory(db, id)

	if err := DecaySalience(context.Background(), db); err != nil {
		t.Fatalf("DecaySalience failed: %v", err)
	}

	_, archived := readSalience(t, db, id)
	if !archived {
		t.Error("memory decayed below archiveBelow but was not archived")
	}
}

// TestRetrieveCaseMemory_RecallNeverReachesPinnedSalience is the regression
// test for the bug fixed alongside DecaySalience's pinnedSalience exemption:
// a memory recalled many times (never pinned by a human) must never reach
// exactly pinnedSalience, or it would become permanently exempt from decay
// exactly like an intentionally pinned one.
func TestRetrieveCaseMemory_RecallNeverReachesPinnedSalience(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	id := insertTestMemory(t, db, 1.0, justNow, false)
	defer cleanupTestMemory(db, id)

	// Recall it far more times than it would take to reach 2.0 at +0.1/recall
	// if uncapped (10x), to prove the ceiling holds under repeated reuse.
	for i := 0; i < 30; i++ {
		if _, err := RetrieveCaseMemory(context.Background(), db, cockroach.EncodeVector(make([]float32, 1024)), "TRANSFER", 5); err != nil {
			t.Fatalf("RetrieveCaseMemory failed on iteration %d: %v", i, err)
		}
	}

	salience, _ := readSalience(t, db, id)
	if salience >= pinnedSalience {
		t.Errorf("salience after 30 recalls = %v, must stay strictly below pinnedSalience (%v) since this memory was never pinned", salience, pinnedSalience)
	}
	if salience != recallSalienceCeiling {
		t.Errorf("salience after 30 recalls = %v, want exactly recallSalienceCeiling (%v) once the cap is reached", salience, recallSalienceCeiling)
	}
}

// TestDecaySalience_DoesNotResurrectAlreadyArchived: an already-archived row
// must be left alone entirely (both UPDATEs filter on archived = false) -
// decay is not a way to un-archive something.
func TestDecaySalience_DoesNotResurrectAlreadyArchived(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	id := insertTestMemory(t, db, 0.01, longAgo, true)
	defer cleanupTestMemory(db, id)

	if err := DecaySalience(context.Background(), db); err != nil {
		t.Fatalf("DecaySalience failed: %v", err)
	}

	salience, archived := readSalience(t, db, id)
	if salience != 0.01 {
		t.Errorf("already-archived salience changed to %v, want unchanged 0.01", salience)
	}
	if !archived {
		t.Error("already-archived memory came back unarchived - decay must never do that")
	}
}
