// search.go: GET /control/search?q= - tim kiem toan cuc cua control platform.
// Mot query cham vao ca 4 loai du lieu: transactions, tasks, episodic memory,
// agents - de tu command palette (Ctrl+K) nhay thang den thu can xem.
package dashboardapi

import (
	"context"
	"net/http"
	"strings"
)

type SearchTxn struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	Amount  float64 `json:"amount"`
	Tier    string  `json:"risk_tier"`
	Verdict *string `json:"verdict,omitempty"`
}

type SearchTask struct {
	ID            string  `json:"id"`
	TransactionID string  `json:"transaction_id"`
	Status        string  `json:"status"`
	Verdict       *string `json:"verdict,omitempty"`
}

type SearchMemory struct {
	ID          string  `json:"id"`
	Summary     string  `json:"summary"`
	Verdict     string  `json:"verdict"`
	PatternType *string `json:"pattern_type,omitempty"`
	RecallCount int     `json:"recall_count"`
}

type SearchAgent struct {
	AgentID string `json:"agent_id"`
	Actions int    `json:"actions"`
}

type SearchResults struct {
	Query        string         `json:"query"`
	Transactions []SearchTxn    `json:"transactions"`
	Tasks        []SearchTask   `json:"tasks"`
	Memories     []SearchMemory `json:"memories"`
	Agents       []SearchAgent  `json:"agents"`
}

const searchLimit = 5

func (s *Server) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 || len(q) > 80 {
		http.Error(w, "q must be 2-80 characters", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	res := SearchResults{
		Query:        q,
		Transactions: []SearchTxn{},
		Tasks:        []SearchTask{},
		Memories:     []SearchMemory{},
		Agents:       []SearchAgent{},
	}

	// Moi nhanh la mot prepared query voi tham so hoa day du - khong noi suy
	// chuoi nguoi dung vao SQL. Loi tung nhanh chi lam nhanh do rong.
	s.searchTransactions(ctx, q, &res)
	s.searchTasks(ctx, q, &res)
	s.searchMemories(ctx, q, &res)
	s.searchAgents(ctx, q, &res)

	writeJSON(w, res)
}

func (s *Server) searchTransactions(ctx context.Context, q string, res *SearchResults) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT tx.id, tx.type, tx.amount, tx.risk_tier, t.verdict
		FROM transactions tx
		LEFT JOIN tasks t ON t.transaction_id = tx.id
		WHERE tx.id::STRING ILIKE $1 || '%'
		   OR tx.name_orig ILIKE '%' || $1 || '%'
		   OR tx.name_dest ILIKE '%' || $1 || '%'
		ORDER BY tx.arrived_at DESC
		LIMIT $2
	`, q, searchLimit)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t SearchTxn
		if rows.Scan(&t.ID, &t.Type, &t.Amount, &t.Tier, &t.Verdict) == nil {
			res.Transactions = append(res.Transactions, t)
		}
	}
}

func (s *Server) searchTasks(ctx context.Context, q string, res *SearchResults) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT id, transaction_id, status, verdict
		FROM tasks
		WHERE id::STRING ILIKE $1 || '%'
		   OR status = lower($1)
		   OR verdict = lower($1)
		ORDER BY created_at DESC
		LIMIT $2
	`, q, searchLimit)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t SearchTask
		if rows.Scan(&t.ID, &t.TransactionID, &t.Status, &t.Verdict) == nil {
			res.Tasks = append(res.Tasks, t)
		}
	}
}

func (s *Server) searchMemories(ctx context.Context, q string, res *SearchResults) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT id, left(summary, 140), verdict, pattern_type, recall_count
		FROM case_memory
		WHERE archived = false
		  AND (summary ILIKE '%' || $1 || '%' OR pattern_type ILIKE '%' || $1 || '%')
		ORDER BY recall_count DESC
		LIMIT $2
	`, q, searchLimit)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var m SearchMemory
		if rows.Scan(&m.ID, &m.Summary, &m.Verdict, &m.PatternType, &m.RecallCount) == nil {
			res.Memories = append(res.Memories, m)
		}
	}
}

func (s *Server) searchAgents(ctx context.Context, q string, res *SearchResults) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT agent_id, COUNT(*) AS actions
		FROM audit_log
		WHERE agent_id ILIKE '%' || $1 || '%'
		GROUP BY agent_id
		ORDER BY actions DESC
		LIMIT $2
	`, q, searchLimit)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var a SearchAgent
		if rows.Scan(&a.AgentID, &a.Actions) == nil {
			res.Agents = append(res.Agents, a)
		}
	}
}
