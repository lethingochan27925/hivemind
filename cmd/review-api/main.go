// main.go: HTTP API cho human review - Dashboard goi vao day de hien thi
// review queue va gui quyet dinh approve/reject.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/internal/review"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

type server struct {
	db *cockroach.Client
}

func (s *server) listReviews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reviews, err := review.ListPendingReviews(r.Context(), s.db)
	if err != nil {
		log.Printf("listing reviews: %v", err)
		http.Error(w, "failed to list reviews", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(reviews)
}

type decideRequest struct {
	TaskID     string `json:"task_id"`
	ReviewerID string `json:"reviewer_id"`
	Decision   string `json:"decision"`
	Notes      string `json:"notes"`
}

func (s *server) decideReview(w http.ResponseWriter, r *http.Request) {
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

	err := review.SubmitReview(r.Context(), s.db, req.TaskID, req.ReviewerID, req.Decision, req.Notes)
	if err != nil {
		log.Printf("submitting review: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	ctx := context.Background()
	db, err := cockroach.NewClient(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer db.Close()

	s := &server{db: db}

	http.HandleFunc("/reviews", corsMiddleware(s.listReviews))
	http.HandleFunc("/reviews/decide", corsMiddleware(s.decideReview))

	port := os.Getenv("REVIEW_API_PORT")
	if port == "" {
		port = "8090"
	}

	log.Printf("Review API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
