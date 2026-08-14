// training.go: so lieu cho Training Lab - noi chay tung me du lieu va do xem
// fleet "hoc" den dau sau moi me.
//
// Ghi chu trung thuc: HiveMind KHONG fine-tune trong so model (Claude Haiku tren
// Bedrock khong cho phep dieu do). Thu that su duoc hinh thanh o day la EPISODIC
// MEMORY: moi case dong lai duoc tom tat, nhung vao vector, va hop nhat voi ky uc
// tuong tu. Trang Training Lab do chinh qua trinh do.
//
// Thiet ke: server chi tra SNAPSHOT tich luy. Trinh duyet lay snapshot truoc va
// sau moi me roi tru nhau -> khong can luu trang thai phien o server, va moi con
// so deu truy nguoc duoc ve audit_log.
package dashboardapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type TrainingSnapshot struct {
	At string `json:"at"`

	// Episodic memory
	MemoriesActive   int `json:"memories_active"`
	MemoriesArchived int `json:"memories_archived"`
	RawCasesAbsorbed int `json:"raw_cases_absorbed"`

	// Cong don de tinh chenh lech giua hai me
	RecallEvents  int `json:"recall_events"`
	RecallHitsSum int `json:"recall_hits_sum"`

	// Ket qua
	VerdictFraud    int `json:"verdict_fraud"`
	VerdictLegit    int `json:"verdict_legit"`
	VerdictEscalate int `json:"verdict_escalate"`
	CorrectVerdicts int `json:"correct_verdicts"`
	GradedVerdicts  int `json:"graded_verdicts"`

	// Chi phi va suy luan
	TokensIn        int `json:"tokens_in"`
	TokensOut       int `json:"tokens_out"`
	ModelDecisions  int `json:"model_decisions"`
	FallbackVerdict int `json:"fallback_verdicts"`

	// Hang doi
	Pending       int `json:"pending"`
	Investigating int `json:"investigating"`
}

func (s *Server) TrainingMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	snap := TrainingSnapshot{At: time.Now().UTC().Format(time.RFC3339)}

	_ = s.DB.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE archived = false),
			COUNT(*) FILTER (WHERE archived = true),
			COALESCE(SUM(merge_count) FILTER (WHERE archived = false), 0)
		FROM case_memory
	`).Scan(&snap.MemoriesActive, &snap.MemoriesArchived, &snap.RawCasesAbsorbed)

	_ = s.DB.Pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(memory_hits), 0)
		FROM audit_log WHERE action = 'memory_recall'
	`).Scan(&snap.RecallEvents, &snap.RecallHitsSum)

	_ = s.DB.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE action = 'bedrock_reasoning' AND bedrock_model IS NOT NULL),
			COUNT(*) FILTER (WHERE action = 'bedrock_reasoning' AND bedrock_model IS NULL),
			COALESCE(SUM(tokens_in), 0),
			COALESCE(SUM(tokens_out), 0)
		FROM audit_log
	`).Scan(&snap.ModelDecisions, &snap.FallbackVerdict, &snap.TokensIn, &snap.TokensOut)

	// Verdict + do chinh xac so voi nhan goc, trong mot lan quet.
	_ = s.DB.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE t.verdict = 'fraud'),
			COUNT(*) FILTER (WHERE t.verdict = 'legit'),
			COUNT(*) FILTER (WHERE t.verdict = 'escalate'),
			COUNT(*) FILTER (WHERE (t.verdict = 'fraud' AND tx.is_fraud_label)
			                    OR (t.verdict = 'legit' AND NOT tx.is_fraud_label)),
			COUNT(*) FILTER (WHERE t.verdict IN ('fraud', 'legit'))
		FROM tasks t JOIN transactions tx ON tx.id = t.transaction_id
		WHERE t.verdict IS NOT NULL
	`).Scan(&snap.VerdictFraud, &snap.VerdictLegit, &snap.VerdictEscalate,
		&snap.CorrectVerdicts, &snap.GradedVerdicts)

	_ = s.DB.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending'),
			COUNT(*) FILTER (WHERE status IN ('claimed', 'investigating'))
		FROM tasks
	`).Scan(&snap.Pending, &snap.Investigating)

	writeJSON(w, snap)
}

// --- Live activity log --------------------------------------------------------

type TrainingLogEntry struct {
	At         string  `json:"at"`
	Action     string  `json:"action"`
	AgentID    string  `json:"agent_id"`
	TaskID     string  `json:"task_id"`
	Reasoning  *string `json:"reasoning,omitempty"`
	MemoryHits *int    `json:"memory_hits,omitempty"`
	LatencyMs  *int    `json:"latency_ms,omitempty"`
	Model      *string `json:"bedrock_model,omitempty"`
}

// TrainingLog tra ve hoat dong gan nhat cua fleet, moi nhat truoc. Day la audit
// trail that - khong phai log ung dung - nen moi dong deu truy nguoc duoc.
func (s *Server) TrainingLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 60
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	rows, err := s.DB.Pool.Query(r.Context(), `
		SELECT created_at, action, agent_id, task_id::STRING,
		       left(COALESCE(reasoning, ''), 160), memory_hits, latency_ms, bedrock_model
		FROM audit_log
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		http.Error(w, "failed to read activity", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	out := []TrainingLogEntry{}
	for rows.Next() {
		var e TrainingLogEntry
		var at time.Time
		var reasoning string
		if err := rows.Scan(&at, &e.Action, &e.AgentID, &e.TaskID,
			&reasoning, &e.MemoryHits, &e.LatencyMs, &e.Model); err != nil {
			continue
		}
		e.At = at.Format(time.RFC3339)
		if reasoning != "" {
			e.Reasoning = &reasoning
		}
		out = append(out, e)
	}
	writeJSON(w, out)
}

// --- Luu phien huan luyen ------------------------------------------------------
// Mot phien la mot chuoi me co so do. Luu lai de so sanh giua cac lan, va de
// dinh kem vao evidence. Bang tu tao lan dau dung den - khong can migration.

const ensureRunsSQL = `
CREATE TABLE IF NOT EXISTS training_runs (
	id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	label      STRING,
	config     JSONB,
	batches    JSONB,
	summary    JSONB,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

type TrainingRun struct {
	ID        string          `json:"id"`
	Label     string          `json:"label"`
	Config    json.RawMessage `json:"config"`
	Batches   json.RawMessage `json:"batches"`
	Summary   json.RawMessage `json:"summary"`
	CreatedAt string          `json:"created_at"`
}

func (s *Server) TrainingRuns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if _, err := s.DB.Pool.Exec(ctx, ensureRunsSQL); err != nil {
		http.Error(w, "run store unavailable", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rows, err := s.DB.Pool.Query(ctx, `
			SELECT id::STRING, COALESCE(label, ''), config, batches, summary, created_at
			FROM training_runs ORDER BY created_at DESC LIMIT 20
		`)
		if err != nil {
			http.Error(w, "failed to list runs", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		out := []TrainingRun{}
		for rows.Next() {
			var run TrainingRun
			var at time.Time
			if rows.Scan(&run.ID, &run.Label, &run.Config, &run.Batches, &run.Summary, &at) == nil {
				run.CreatedAt = at.Format(time.RFC3339)
				out = append(out, run)
			}
		}
		writeJSON(w, out)

	case http.MethodPost:
		if !controlAllowed(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body struct {
			Label   string          `json:"label"`
			Config  json.RawMessage `json:"config"`
			Batches json.RawMessage `json:"batches"`
			Summary json.RawMessage `json:"summary"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		var id string
		if err := s.DB.Pool.QueryRow(ctx, `
			INSERT INTO training_runs (label, config, batches, summary)
			VALUES ($1, $2, $3, $4) RETURNING id::STRING
		`, body.Label, body.Config, body.Batches, body.Summary).Scan(&id); err != nil {
			http.Error(w, fmt.Sprintf("saving run: %v", err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok", "id": id})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
