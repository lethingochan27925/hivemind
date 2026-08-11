// main.go: Lambda Go lam cong vao cho scoring - forward request sang Python XGBoost service.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/lethingochan27925/hivemind/internal/scorer"
)

const defaultPort = "8080"

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
	riskScore, err := h.pythonClient.Score(r.Context(), req)
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

// newScoringClient chon che do xac thuc theo moi truong: local goi thang service
// Python khong qua IAM; tren Lambda thi Function URL cua scoring-python dung auth
// AWS_IAM nen request phai duoc ky bang credential cua execution role.
func newScoringClient(ctx context.Context, endpoint string) (*scorer.Client, error) {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") == "" {
		return scorer.NewClient(endpoint), nil
	}

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config for request signing: %w", err)
	}
	return scorer.NewSignedClient(awsCfg, endpoint), nil
}

func buildMux(h *scoreHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/score", h.handle)
	return mux
}

func main() {
	pythonEndpoint := resolveScoringEndpoint()

	client, err := newScoringClient(context.Background(), pythonEndpoint)
	if err != nil {
		log.Fatalf("initializing scoring client: %v", err)
	}
	mux := buildMux(&scoreHandler{pythonClient: client})

	// Tren Lambda that: chay qua Lambda Runtime API bang httpadapter, giu nguyen
	// http.Handler da viet. Truoc day ham nay goi ListenAndServe nen Function URL
	// khong bao gio nhan duoc handler nao - moi request deu that bai.
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		adapter := httpadapter.NewV2(mux)
		log.Printf("Scoring API on Lambda, forwarding to %s", pythonEndpoint)
		lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
			return adapter.ProxyWithContext(ctx, req)
		})
		return
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	log.Printf("Scoring API listening on :%s, forwarding to %s", port, pythonEndpoint)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
