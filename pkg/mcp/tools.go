// tools.go: 3 tools nghiep vu cua HiveMind dung MCP select_query lam nen tang.
package mcp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// isValidUUID validates a canonical UUID (transactions.id / case_memory ids
// are all UUID PKs) via the same parser the rest of the repo already depends
// on (internal/stream/replay.go uses uuid.New()) instead of a second,
// hand-maintained regex that could drift from what the library considers
// valid. Rejecting anything else outright is the strongest defence against
// injection here (OWASP SQL Injection Prevention Cheat Sheet, "Allow-list
// Input Validation") and it is free: a caller never has a legitimate reason to
// look up a transaction by a non-UUID string.
func isValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// sqlQuoteLiteral escapes a string for safe interpolation into a single-quoted
// SQL literal (doubling every embedded quote, the standard SQL escaping rule -
// the same one Postgres/CockroachDB's own quote_literal() applies). The MCP
// select_query tool takes one opaque SQL string over JSON-RPC; the protocol has
// no bind-parameter slot, so this is the documented fallback (OWASP SQL
// Injection Prevention Cheat Sheet, "Escaping All User-Supplied Input") for
// when a real prepared statement is not available.
func sqlQuoteLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// verdictValues / transactionTypeValues are the CHECK-constrained enums from
// migrations/001_init.sql - allow-listing against them is stronger than
// escaping wherever the valid set is closed.
var verdictValues = map[string]bool{"fraud": true, "legit": true, "escalate": true}
var transactionTypeValues = map[string]bool{"TRANSFER": true, "CASH_OUT": true}

// flexFloat64 parse duoc ca so JSON thuong (0.5) lan JSON string ("1.28e-06"),
// vi MCP server co the tra risk_score o dinh dang khac nhau tuy do lon.
type flexFloat64 float64

func (f *flexFloat64) UnmarshalJSON(data []byte) error {
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		*f = flexFloat64(num)
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("risk_score is neither number nor string: %s", string(data))
	}
	parsed, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("parsing risk_score string %q: %w", s, err)
	}
	*f = flexFloat64(parsed)
	return nil
}

type Transaction struct {
	ID               string      `json:"id"`
	Step             int         `json:"step"`
	Type             string      `json:"type"`
	Amount           float64     `json:"amount"`
	NameOrig         string      `json:"name_orig"`
	OldBalanceOrig   float64     `json:"old_balance_orig"`
	NewBalanceOrig   float64     `json:"new_balance_orig"`
	NameDest         string      `json:"name_dest"`
	OldBalanceDest   float64     `json:"old_balance_dest"`
	NewBalanceDest   float64     `json:"new_balance_dest"`
	ErrorBalanceOrig float64     `json:"error_balance_orig"`
	ErrorBalanceDest float64     `json:"error_balance_dest"`
	RiskScoreRaw     flexFloat64 `json:"risk_score"`
	RiskTier         string      `json:"risk_tier"`
	IsFraudLabel     bool        `json:"is_fraud_label"`
}

// RiskScore tra ve gia tri float64 tien dung, che di chi tiet flexFloat64.
func (t *Transaction) RiskScore() float64 {
	return float64(t.RiskScoreRaw)
}

type CustomerHistoryRow struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Amount     float64  `json:"amount"`
	RiskScore  float64  `json:"risk_score"`
	RiskTier   string   `json:"risk_tier"`
	ArrivedAt  string   `json:"arrived_at"`
	Verdict    *string  `json:"verdict"`
	Confidence *float64 `json:"confidence"`
}

type SimilarCase struct {
	ID            string   `json:"id"`
	Summary       string   `json:"summary"`
	Verdict       string   `json:"verdict"`
	ConfidenceAvg float64  `json:"confidence_avg"`
	PatternType   string   `json:"pattern_type"`
	KeySignals    []string `json:"key_signals"`
	Salience      float64  `json:"salience"`
	RecallCount   int      `json:"recall_count"`
}

type Tools struct {
	client *Client
}

func NewTools(client *Client) *Tools {
	return &Tools{client: client}
}

func (t *Tools) GetTransaction(transactionID string) (*Transaction, error) {
	if !isValidUUID(transactionID) {
		return nil, fmt.Errorf("invalid transaction id %q: must be a UUID", transactionID)
	}
	query := fmt.Sprintf(`
		SELECT
			id, step, type, amount,
			name_orig, old_balance_orig, new_balance_orig,
			name_dest, old_balance_dest, new_balance_dest,
			error_balance_orig, error_balance_dest,
			risk_score, risk_tier, is_fraud_label
		FROM transactions
		WHERE id = '%s'
	`, transactionID)

	raw, err := t.client.Select(query, 1)
	if err != nil {
		return nil, err
	}

	var rows []Transaction
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("unmarshaling transaction (raw=%s): %w", string(raw), err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (t *Tools) GetCustomerContext(nameOrig string, limit int) ([]CustomerHistoryRow, error) {
	query := fmt.Sprintf(`
		SELECT
			t.id, t.type, t.amount, t.risk_score, t.risk_tier, t.arrived_at,
			tk.verdict, tk.confidence
		FROM transactions t
		LEFT JOIN tasks tk ON tk.transaction_id = t.id
		WHERE t.name_orig = '%s'
		ORDER BY t.arrived_at DESC
	`, sqlQuoteLiteral(nameOrig))

	raw, err := t.client.Select(query, limit)
	if err != nil {
		return nil, err
	}

	var rows []CustomerHistoryRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("unmarshaling customer context: %w", err)
	}
	return rows, nil
}

func (t *Tools) SearchSimilarCases(transactionType, amountRange, verdictFilter string, limit int) ([]SimilarCase, error) {
	if transactionType != "" && !transactionTypeValues[transactionType] {
		return nil, fmt.Errorf("invalid transaction type %q", transactionType)
	}
	if verdictFilter != "" && !verdictValues[verdictFilter] {
		return nil, fmt.Errorf("invalid verdict filter %q", verdictFilter)
	}

	verdictClause := ""
	if verdictFilter != "" {
		verdictClause = fmt.Sprintf("AND verdict = '%s'", sqlQuoteLiteral(verdictFilter))
	}

	query := fmt.Sprintf(`
		SELECT
			id, summary, verdict, confidence_avg,
			pattern_type, key_signals, salience, recall_count
		FROM case_memory
		WHERE archived = false
		  AND transaction_type = '%s'
		  AND amount_range = '%s'
		  %s
		ORDER BY salience DESC, recall_count DESC
	`, sqlQuoteLiteral(transactionType), sqlQuoteLiteral(amountRange), verdictClause)

	raw, err := t.client.Select(query, limit)
	if err != nil {
		return nil, err
	}

	var rows []SimilarCase
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("unmarshaling similar cases: %w", err)
	}
	return rows, nil
}
