// paysim.go: doc PaySim CSV, tinh engineered features (errorBalanceOrig/Dest).
package stream

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
)

type RawTransaction struct {
	Step             int
	Type             string
	Amount           float64
	NameOrig         string
	OldBalanceOrig   float64
	NewBalanceOrig   float64
	NameDest         string
	OldBalanceDest   float64
	NewBalanceDest   float64
	ErrorBalanceOrig float64
	ErrorBalanceDest float64
	IsFraud          bool
}

func LoadPaySim(csvPath string, limit int) ([]RawTransaction, error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("opening csv: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[h] = i
	}

	var txns []RawTransaction
	skipped := 0
	for {
		if limit > 0 && len(txns) >= limit {
			break
		}
		record, err := reader.Read()
		if err != nil {
			break
		}

		txType := record[colIdx["type"]]
		if txType != "TRANSFER" && txType != "CASH_OUT" {
			continue
		}

		// A discarded parse error here used to become a silent 0.0 - and a row
		// with amount=oldOrig=newOrig=0 exactly matches ClassifyPattern's
		// "balance_wipe" fraud signature (agent.go: OldBalanceOrig == Amount
		// && NewBalanceOrig == 0). One corrupted CSV cell could inject a
		// spurious "fraud" pattern into training/eval data instead of being
		// rejected as the bad input it is.
		step, errStep := strconv.Atoi(record[colIdx["step"]])
		amount, errAmount := strconv.ParseFloat(record[colIdx["amount"]], 64)
		oldOrig, errOldOrig := strconv.ParseFloat(record[colIdx["oldbalanceOrg"]], 64)
		newOrig, errNewOrig := strconv.ParseFloat(record[colIdx["newbalanceOrig"]], 64)
		oldDest, errOldDest := strconv.ParseFloat(record[colIdx["oldbalanceDest"]], 64)
		newDest, errNewDest := strconv.ParseFloat(record[colIdx["newbalanceDest"]], 64)
		if errStep != nil || errAmount != nil || errOldOrig != nil || errNewOrig != nil || errOldDest != nil || errNewDest != nil {
			skipped++
			continue
		}
		isFraud := record[colIdx["isFraud"]] == "1"

		txns = append(txns, RawTransaction{
			Step:             step,
			Type:             txType,
			Amount:           amount,
			NameOrig:         record[colIdx["nameOrig"]],
			OldBalanceOrig:   oldOrig,
			NewBalanceOrig:   newOrig,
			NameDest:         record[colIdx["nameDest"]],
			OldBalanceDest:   oldDest,
			NewBalanceDest:   newDest,
			ErrorBalanceOrig: newOrig + amount - oldOrig,
			ErrorBalanceDest: oldDest + amount - newDest,
			IsFraud:          isFraud,
		})
	}

	if skipped > 0 {
		log.Printf("[warn] LoadPaySim: skipped %d row(s) with unparseable numeric fields", skipped)
	}

	return txns, nil
}
