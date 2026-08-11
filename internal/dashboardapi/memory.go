// memory.go: GET /memory - trang Fleet & Memory, thong ke case_memory va agent hoat dong.
package dashboardapi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type PatternStat struct {
	PatternType string `json:"pattern_type"`
	Count       int    `json:"count"`
}

type MemoryStats struct {
	ActiveCases   int           `json:"active_cases"`
	ArchivedCases int           `json:"archived_cases"`
	AvgSalience   float64       `json:"avg_salience"`
	TopPatterns   []PatternStat `json:"top_patterns"`
}

type ActiveAgent struct {
	AgentID      string `json:"agent_id"`
	Status       string `json:"status"`
	CurrentTask  string `json:"current_task,omitempty"`
	LastActivity string `json:"last_activity"`
}

type ImpactStats struct {
	VerdictAccuracyPct  *float64 `json:"verdict_accuracy_pct,omitempty"`
	AvgLatencyWithHitMs *float64 `json:"avg_latency_with_hit_ms,omitempty"`
	AvgLatencyNoHitMs   *float64 `json:"avg_latency_no_hit_ms,omitempty"`
}

type MemoryResponse struct {
	Stats        MemoryStats   `json:"stats"`
	ActiveAgents []ActiveAgent `json:"active_agents"`
	Impact       ImpactStats   `json:"impact"`
}

func (s *Server) GetMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()

	stats, err := s.queryMemoryStats(ctx)
	if err != nil {
		log.Printf("querying memory stats: %v", err)
		http.Error(w, "failed to load memory data", http.StatusInternalServerError)
		return
	}

	agents, err := s.queryActiveAgents(ctx)
	if err != nil {
		log.Printf("querying active agents: %v", err)
		http.Error(w, "failed to load memory data", http.StatusInternalServerError)
		return
	}

	impact, err := s.queryImpactStats(ctx)
	if err != nil {
		log.Printf("querying impact stats: %v", err)
		http.Error(w, "failed to load memory data", http.StatusInternalServerError)
		return
	}

	resp := MemoryResponse{
		Stats:        stats,
		ActiveAgents: agents,
		Impact:       impact,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) queryMemoryStats(ctx context.Context) (MemoryStats, error) {
	var stats MemoryStats

	err := s.DB.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE archived = false), COUNT(*) FILTER (WHERE archived = true)
		FROM case_memory
	`).Scan(&stats.ActiveCases, &stats.ArchivedCases)
	if err != nil {
		return stats, err
	}

	err = s.DB.Pool.QueryRow(ctx, `
		SELECT COALESCE(AVG(salience), 0) FROM case_memory WHERE archived = false
	`).Scan(&stats.AvgSalience)
	if err != nil {
		return stats, err
	}

	rows, err := s.DB.Pool.Query(ctx, `
		SELECT pattern_type, COUNT(*) AS count
		FROM case_memory
		WHERE archived = false AND pattern_type IS NOT NULL
		GROUP BY pattern_type
		ORDER BY count DESC
		LIMIT 5
	`)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var p PatternStat
		if err := rows.Scan(&p.PatternType, &p.Count); err != nil {
			return stats, err
		}
		stats.TopPatterns = append(stats.TopPatterns, p)
	}

	return stats, rows.Err()
}

func (s *Server) queryActiveAgents(ctx context.Context) ([]ActiveAgent, error) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT DISTINCT ON (claimed_by)
			claimed_by,
			status,
			id,
			claimed_at
		FROM tasks
		WHERE claimed_by IS NOT NULL
		  AND claimed_at >= now() - INTERVAL '30 minutes'
		ORDER BY claimed_by, claimed_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ActiveAgent
	for rows.Next() {
		var a ActiveAgent
		var status, taskID string
		var lastActivity time.Time
		if err := rows.Scan(&a.AgentID, &status, &taskID, &lastActivity); err != nil {
			return nil, err
		}
		a.LastActivity = lastActivity.Format(time.RFC3339)
		if status == "investigating" {
			a.Status = "active"
			a.CurrentTask = taskID
		} else {
			a.Status = "idle"
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

func (s *Server) queryImpactStats(ctx context.Context) (ImpactStats, error) {
	var stats ImpactStats

	err := s.DB.Pool.QueryRow(ctx, `
		SELECT
			ROUND(
				100.0 * COUNT(*) FILTER (
					WHERE (t.verdict = 'fraud' AND tx.is_fraud_label = true)
					   OR (t.verdict = 'legit' AND tx.is_fraud_label = false)
				) / NULLIF(COUNT(*) FILTER (WHERE t.verdict IN ('fraud', 'legit')), 0),
				1
			)
		FROM tasks t
		JOIN transactions tx ON t.transaction_id = tx.id
		WHERE t.verdict IS NOT NULL
	`).Scan(&stats.VerdictAccuracyPct)
	if err != nil {
		return stats, err
	}

	return stats, nil
}
