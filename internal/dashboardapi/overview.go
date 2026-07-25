// overview.go: GET /overview - trang Command Center, so lieu tong quan he thong.
package dashboardapi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type VerdictCount struct {
	Verdict string `json:"verdict"`
	Count   int    `json:"count"`
}

type MemoryHitPoint struct {
	HourBucket string  `json:"hour_bucket"`
	AvgHits    float64 `json:"avg_memory_hits"`
}

type LiveTask struct {
	TaskID    string    `json:"task_id"`
	ClaimedBy string    `json:"claimed_by"`
	ClaimedAt time.Time `json:"claimed_at"`
}

type OverviewResponse struct {
	VerdictsToday      []VerdictCount   `json:"verdicts_today"`
	PendingReviews     int              `json:"pending_reviews"`
	MemoryHitsTrend    []MemoryHitPoint `json:"memory_hits_trend"`
	LiveTasks          []LiveTask       `json:"live_tasks"`
	VerdictAccuracyPct *float64         `json:"verdict_accuracy_pct,omitempty"`
}

func (s *Server) GetOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()

	verdicts, err := s.queryVerdictsToday(ctx)
	if err != nil {
		log.Printf("querying verdicts: %v", err)
		http.Error(w, "failed to load overview", http.StatusInternalServerError)
		return
	}

	pendingCount, err := s.queryPendingReviewCount(ctx)
	if err != nil {
		log.Printf("querying pending reviews: %v", err)
		http.Error(w, "failed to load overview", http.StatusInternalServerError)
		return
	}

	trend, err := s.queryMemoryHitsTrend(ctx)
	if err != nil {
		log.Printf("querying memory hits trend: %v", err)
		http.Error(w, "failed to load overview", http.StatusInternalServerError)
		return
	}

	liveTasks, err := s.queryLiveTasks(ctx)
	if err != nil {
		log.Printf("querying live tasks: %v", err)
		http.Error(w, "failed to load overview", http.StatusInternalServerError)
		return
	}

	accuracy, err := s.queryVerdictAccuracy(ctx)
	if err != nil {
		log.Printf("querying verdict accuracy: %v", err)
	}

	resp := OverviewResponse{
		VerdictsToday:      verdicts,
		PendingReviews:     pendingCount,
		MemoryHitsTrend:    trend,
		LiveTasks:          liveTasks,
		VerdictAccuracyPct: accuracy,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) queryVerdictsToday(ctx context.Context) ([]VerdictCount, error) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT verdict, COUNT(*) AS count
		FROM tasks
		WHERE verdict IS NOT NULL
		  AND completed_at >= current_date()
		GROUP BY verdict
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []VerdictCount
	for rows.Next() {
		var v VerdictCount
		if err := rows.Scan(&v.Verdict, &v.Count); err != nil {
			return nil, err
		}
		results = append(results, v)
	}
	return results, rows.Err()
}

func (s *Server) queryPendingReviewCount(ctx context.Context) (int, error) {
	var count int
	err := s.DB.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM tasks WHERE status = 'pending_review'
	`).Scan(&count)
	return count, err
}

func (s *Server) queryMemoryHitsTrend(ctx context.Context) ([]MemoryHitPoint, error) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT
			to_char(created_at, 'HH24:00') AS hour_bucket,
			AVG(memory_hits) AS avg_hits
		FROM audit_log
		WHERE action = 'memory_recall'
		  AND created_at >= now() - INTERVAL '24 hours'
		GROUP BY hour_bucket
		ORDER BY hour_bucket
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MemoryHitPoint
	for rows.Next() {
		var p MemoryHitPoint
		if err := rows.Scan(&p.HourBucket, &p.AvgHits); err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

func (s *Server) queryLiveTasks(ctx context.Context) ([]LiveTask, error) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT id, claimed_by, claimed_at
		FROM tasks
		WHERE status = 'investigating'
		ORDER BY claimed_at DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LiveTask
	for rows.Next() {
		var t LiveTask
		if err := rows.Scan(&t.TaskID, &t.ClaimedBy, &t.ClaimedAt); err != nil {
			return nil, err
		}
		results = append(results, t)
	}
	return results, rows.Err()
}

func (s *Server) queryVerdictAccuracy(ctx context.Context) (*float64, error) {
	var pct *float64
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
	`).Scan(&pct)
	return pct, err
}
