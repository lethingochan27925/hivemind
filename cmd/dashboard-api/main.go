// main.go: dashboard-api entrypoint - khoi tao client, dang ky routes cho 5 trang dashboard.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/internal/dashboardapi"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

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

	http.HandleFunc("/overview", dashboardapi.CORSMiddleware(s.GetOverview))
	http.HandleFunc("/reviews", dashboardapi.CORSMiddleware(s.ListReviews))
	http.HandleFunc("/reviews/decide", dashboardapi.CORSMiddleware(s.DecideReview))
	http.HandleFunc("/memory", dashboardapi.CORSMiddleware(s.GetMemory))
	http.HandleFunc("/transactions", dashboardapi.CORSMiddleware(s.ListTransactions))
	http.HandleFunc("/transactions/", dashboardapi.CORSMiddleware(s.GetTransactionAudit))
	http.HandleFunc("/infrastructure", dashboardapi.CORSMiddleware(s.GetInfrastructure))
	http.HandleFunc("/infrastructure/simulate-crash", dashboardapi.CORSMiddleware(s.SimulateCrash))
	http.HandleFunc("/cost", dashboardapi.CORSMiddleware(s.GetCost))

	port := os.Getenv("DASHBOARD_API_PORT")
	if port == "" {
		port = "8090"
	}

	log.Printf("Dashboard API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
