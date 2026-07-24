// reviews.go: GET /reviews, POST /reviews/decide - trang Review Queue.
package dashboardapi

import (
	"encoding/json"
	"log"
	"net/http"

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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
