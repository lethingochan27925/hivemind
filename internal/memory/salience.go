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

func DecaySalience(ctx context.Context, db *cockroach.Client) error {
	_, err := db.Pool.Exec(ctx, fmt.Sprintf(`
		UPDATE case_memory SET salience = salience * %f
		WHERE archived = false AND last_recalled_at < now() - INTERVAL '%d days'
	`, decayFactor, decayAfterDays))
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
