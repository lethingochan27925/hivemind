// main.go: Lambda Go lam cong vao cho scoring - forward request sang Python XGBoost service.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/lethingochan27925/hivemind/internal/scorer"
)

type scoreHandler struct {
	pythonClient *scorer.Client
}

func (h *scoreHandler) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req scorer.ScoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	riskScore, err := h.pythonClient.Score(req)
	if err != nil {
		log.Printf("scoring error: %v", err)
		http.Error(w, "scoring failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scorer.ScoreResponse{RiskScore: riskScore})
}

func main() {
	pythonEndpoint := os.Getenv("PYTHON_SCORING_ENDPOINT")
	if pythonEndpoint == "" {
		pythonEndpoint = "http://localhost:8001/score"
	}

	handler := &scoreHandler{
		pythonClient: scorer.NewClient(pythonEndpoint),
	}

	http.HandleFunc("/score", handler.handle)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Scoring API listening on :%s, forwarding to %s", port, pythonEndpoint)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
