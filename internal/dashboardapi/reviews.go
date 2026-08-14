// reviews.go: GET /reviews, POST /reviews/decide - trang Review Queue.
package dashboardapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/lethingochan27925/hivemind/internal/review"
)

func (s *Server) ListReviews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reviews, err := review.ListPendingReviews(r.Context(), s.DB)
	if err != nil {
		log.Printf("listing reviews: %v", err)
		http.Error(w, "failed to list reviews", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reviews)
}

type decideRequest struct {
	TaskID     string `json:"task_id"`
	ReviewerID string `json:"reviewer_id"`
	Decision   string `json:"decision"`
	Notes      string `json:"notes"`
}

func (s *Server) DecideReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req decideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.TaskID == "" || req.ReviewerID == "" || req.Decision == "" {
		http.Error(w, "task_id, reviewer_id, decision are required", http.StatusBadRequest)
		return
	}

	err := review.SubmitReview(r.Context(), s.DB, req.TaskID, req.ReviewerID, req.Decision, req.Notes)
	if err != nil {
		log.Printf("submitting review: %v", err)
		// Task khong con pending_review (bi re-queue hoac da quyet) -> 409 voi
		// thong diep ro rang, khong phai loi SQL tho.
		if strings.Contains(err.Error(), "no rows") {
			http.Error(w, "task is no longer awaiting review - it was re-queued or already decided", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Vong lap hoc tu con nguoi: phan quyet cua reviewer duoc nhung vao
	// case_memory va ghim salience - lan sau gap case tuong tu, agent recall
	// duoc dieu con nguoi da day. Best-effort: loi hoc khong lam hong review.
	learned := true
	if err := s.LearnFromReview(r.Context(), req.TaskID, req.Decision, req.ReviewerID, req.Notes); err != nil {
		log.Printf("learning from review %s: %v", req.TaskID, err)
		learned = false
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "memory_learned": learned})
}
