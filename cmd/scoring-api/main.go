// main.go: Lambda Go lam cong vao cho scoring - forward request sang Python XGBoost service.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
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

// resolveScoringEndpoint uu tien PYTHON_SCORING_ENDPOINT (local dev).
// Neu chay tren Lambda that (co AWS_LAMBDA_FUNCTION_NAME) va bien env chua set,
// doc endpoint tu SSM - gia tri nay chi biet duoc SAU khi Terraform apply
// xong Lambda scoring-python, nen khong the truyen qua Terraform luc build.
func resolveScoringEndpoint() string {
	if v := os.Getenv("PYTHON_SCORING_ENDPOINT"); v != "" {
		return v
	}

	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") == "" {
		return "http://localhost:8001/score"
	}

	ssmPrefix := os.Getenv("SSM_PREFIX")
	if ssmPrefix == "" {
		ssmPrefix = "/hivemind/dev"
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Printf("loading AWS config for SSM: %v, falling back to localhost", err)
		return "http://localhost:8001/score"
	}

	client := ssm.NewFromConfig(cfg)
	out, err := client.GetParameter(context.Background(), &ssm.GetParameterInput{
		Name: strPtr(ssmPrefix + "/scoring/python_endpoint"),
	})
	if err != nil {
		log.Printf("reading scoring endpoint from SSM: %v, falling back to localhost", err)
		return "http://localhost:8001/score"
	}

	return *out.Parameter.Value
}

func strPtr(s string) *string { return &s }

func main() {
	pythonEndpoint := resolveScoringEndpoint()

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
