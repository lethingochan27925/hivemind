// main.go: dashboard-api entrypoint - khoi tao client, dang ky routes cho 5 trang dashboard.
// Local dev: chay HTTP server thuong. Tren Lambda that: chay qua Lambda Runtime API
// thong qua httpadapter, giu nguyen toan bo http.Handler da viet.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/internal/dashboardapi"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

func buildMux(s *dashboardapi.Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/overview", dashboardapi.CORSMiddleware(s.GetOverview))
	mux.HandleFunc("/reviews", dashboardapi.CORSMiddleware(s.ListReviews))
	mux.HandleFunc("/reviews/decide", dashboardapi.CORSMiddleware(s.DecideReview))
	mux.HandleFunc("/memory", dashboardapi.CORSMiddleware(s.GetMemory))
	mux.HandleFunc("/transactions", dashboardapi.CORSMiddleware(s.ListTransactions))
	mux.HandleFunc("/transactions/", dashboardapi.CORSMiddleware(s.GetTransactionAudit))
	mux.HandleFunc("/infrastructure", dashboardapi.CORSMiddleware(s.GetInfrastructure))
	mux.HandleFunc("/infrastructure/simulate-crash", dashboardapi.CORSMiddleware(s.SimulateCrash))
	mux.HandleFunc("/cost", dashboardapi.CORSMiddleware(s.GetCost))
	return mux
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

	s := dashboardapi.NewServer(db)
	mux := buildMux(s)

	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		adapter := httpadapter.NewV2(mux)
		lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
			return adapter.ProxyWithContext(ctx, req)
		})
		return
	}

	port := os.Getenv("DASHBOARD_API_PORT")
	if port == "" {
		port = "8090"
	}

	log.Printf("Dashboard API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
