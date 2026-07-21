// paysim.go: doc PaySim CSV, tinh engineered features (errorBalanceOrig/Dest).
package stream

import (
	"encoding/csv"
	"fmt"
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

		step, _ := strconv.Atoi(record[colIdx["step"]])
		amount, _ := strconv.ParseFloat(record[colIdx["amount"]], 64)
		oldOrig, _ := strconv.ParseFloat(record[colIdx["oldbalanceOrg"]], 64)
		newOrig, _ := strconv.ParseFloat(record[colIdx["newbalanceOrig"]], 64)
		oldDest, _ := strconv.ParseFloat(record[colIdx["oldbalanceDest"]], 64)
		newDest, _ := strconv.ParseFloat(record[colIdx["newbalanceDest"]], 64)
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

	return txns, nil
}
