package stream

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCSV(t *testing.T, rows string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "paysim.csv")
	header := "step,type,amount,nameOrig,oldbalanceOrg,newbalanceOrig,nameDest,oldbalanceDest,newbalanceDest,isFraud\n"
	if err := os.WriteFile(path, []byte(header+rows), 0644); err != nil {
		t.Fatalf("writing test csv: %v", err)
	}
	return path
}

func TestLoadPaySim_FiltersNonTransferCashOut(t *testing.T) {
	path := writeCSV(t, "1,PAYMENT,100,C1,100,0,M1,0,0,0\n1,TRANSFER,100,C1,100,0,C2,0,100,0\n")
	txns, err := LoadPaySim(path, 0)
	if err != nil {
		t.Fatalf("LoadPaySim: %v", err)
	}
	if len(txns) != 1 || txns[0].Type != "TRANSFER" {
		t.Errorf("got %d txns %+v, want exactly the 1 TRANSFER row (PAYMENT must be filtered out)", len(txns), txns)
	}
}

// TestLoadPaySim_SkipsRowsWithUnparseableNumbers is the regression test for
// the bug this fix closes: a row with a corrupted numeric cell used to
// silently become amount=0/oldOrig=0/newOrig=0 — which is exactly
// ClassifyPattern's "balance_wipe" fraud signature (OldBalanceOrig == Amount
// && NewBalanceOrig == 0) — instead of being rejected. Bad rows must now be
// skipped, not turned into fabricated data.
func TestLoadPaySim_SkipsRowsWithUnparseableNumbers(t *testing.T) {
	path := writeCSV(t, ""+
		"1,TRANSFER,not-a-number,C1,100,0,C2,0,100,0\n"+ // bad amount
		"garbage,TRANSFER,100,C1,100,0,C2,0,100,0\n"+ // bad step
		"1,TRANSFER,200,C3,300,100,C4,0,200,0\n") // one good row
	txns, err := LoadPaySim(path, 0)
	if err != nil {
		t.Fatalf("LoadPaySim: %v", err)
	}
	if len(txns) != 1 {
		t.Fatalf("got %d txns, want exactly 1 (the two corrupted rows must be skipped, not zero-filled): %+v", len(txns), txns)
	}
	if txns[0].Amount != 200 || txns[0].NameOrig != "C3" {
		t.Errorf("the surviving row is wrong: %+v", txns[0])
	}
}

func TestLoadPaySim_RespectsLimit(t *testing.T) {
	rows := ""
	for i := 0; i < 5; i++ {
		rows += "1,TRANSFER,100,C1,100,0,C2,0,100,0\n"
	}
	path := writeCSV(t, rows)
	txns, err := LoadPaySim(path, 3)
	if err != nil {
		t.Fatalf("LoadPaySim: %v", err)
	}
	if len(txns) != 3 {
		t.Errorf("got %d txns, want 3 (limit)", len(txns))
	}
}

func TestLoadPaySim_MissingFileErrors(t *testing.T) {
	if _, err := LoadPaySim(filepath.Join(t.TempDir(), "does-not-exist.csv"), 0); err == nil {
		t.Error("expected an error for a missing file, got nil")
	}
}
