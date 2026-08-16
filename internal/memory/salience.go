// salience.go: giam salience theo thoi gian, archive case cu (khong con dung).
package memory

import (
	"context"
	"fmt"

	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

const (
	decayFactor    = 0.95
	decayAfterDays = 7
	archiveBelow   = 0.10
)

// pinnedSalience is the ceiling ops.go's "pin" action sets (actOnMemory:
// `UPDATE case_memory SET salience = 2.0`) and the exact value the product
// promises decay "can never cross" (README §3, docs/CONTROL_PLANE.md). Before
// this fix that promise was not actually kept: a pinned memory that was never
// recalled again still matched the decay WHERE clause like any other row and
// was multiplied down every cycle — after enough 6-hourly decay runs (roughly
// 58 of them, since 2.0 * 0.95^58 < archiveBelow) it would cross archiveBelow
// and get archived exactly like an unpinned memory, silently forgetting
// something a human explicitly taught the fleet.
const pinnedSalience = 2.0

// recallSalienceCeiling is the ceiling episodic.go's recall boost is capped
// at — deliberately below pinnedSalience so ordinary reuse can never produce
// the exact value the "pin" action (dashboardapi/ops.go) and the decay
// exemption above both treat as "a human chose to protect this". See
// episodic.go's RetrieveCaseMemory for where this is applied.
const recallSalienceCeiling = pinnedSalience - 0.05

func DecaySalience(ctx context.Context, db *cockroach.Client) error {
	_, err := db.Pool.Exec(ctx, fmt.Sprintf(`
		UPDATE case_memory SET salience = salience * %f
		WHERE archived = false AND last_recalled_at < now() - INTERVAL '%d days'
		  AND salience < %f
	`, decayFactor, decayAfterDays, pinnedSalience))
	if err != nil {
		return fmt.Errorf("decaying salience: %w", err)
	}

	_, err = db.Pool.Exec(ctx, fmt.Sprintf(`
		UPDATE case_memory SET archived = true
		WHERE archived = false AND salience < %f
	`, archiveBelow))
	if err != nil {
		return fmt.Errorf("archiving low salience cases: %w", err)
	}

	return nil
}
