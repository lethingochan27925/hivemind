package memory

import "testing"

// TestSalienceConstantsAreSane catches the class of typo that would otherwise
// only surface hours later as "memory never forgets anything" or "memory
// forgets everything immediately" in production: decayFactor must shrink
// (not grow) salience, archiveBelow must sit inside the valid salience range,
// and pinnedSalience — the ceiling ops.go's "pin" action writes — must be
// excluded from decay by construction (see DecaySalience's WHERE clause).
func TestSalienceConstantsAreSane(t *testing.T) {
	if decayFactor <= 0 || decayFactor >= 1 {
		t.Errorf("decayFactor = %v, want strictly between 0 and 1 (must shrink salience, never grow or freeze it)", decayFactor)
	}
	if decayAfterDays <= 0 {
		t.Errorf("decayAfterDays = %v, want > 0", decayAfterDays)
	}
	if archiveBelow <= 0 || archiveBelow >= pinnedSalience {
		t.Errorf("archiveBelow = %v, want strictly between 0 and pinnedSalience (%v)", archiveBelow, pinnedSalience)
	}
	if pinnedSalience != 2.0 {
		t.Errorf("pinnedSalience = %v, want 2.0 to match the CHECK constraint and ops.go's pin action", pinnedSalience)
	}
	if recallSalienceCeiling >= pinnedSalience {
		t.Errorf("recallSalienceCeiling = %v must be strictly below pinnedSalience = %v, or ordinary recall could reach the exact value DecaySalience treats as pinned", recallSalienceCeiling, pinnedSalience)
	}
}

// TestUnpinnedMemoryWouldEventuallyCrossArchiveThreshold documents *why*
// pinnedSalience must be excluded from decay: repeatedly applying decayFactor
// to an ordinary (unpinned) memory does cross archiveBelow eventually — that
// is the intended forgetting curve. The live SQL behaviour (that a pinned row
// at exactly pinnedSalience is excluded from this multiplication and an
// unpinned one is not) is proven against a real database in
// salience_integration_test.go, since no hermetic test can exercise the WHERE
// clause itself.
func TestUnpinnedMemoryWouldEventuallyCrossArchiveThreshold(t *testing.T) {
	salience := 1.0
	cycles := 0
	for salience >= archiveBelow && cycles < 100_000 {
		salience *= decayFactor
		cycles++
	}
	if cycles == 100_000 {
		t.Fatalf("salience starting at 1.0 never crossed archiveBelow (%v) after 100000 decay cycles — decayFactor is too close to 1", archiveBelow)
	}
}
