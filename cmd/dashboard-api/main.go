// main.go: dashboard-api entrypoint - khoi tao client, dang ky routes cho dashboard
// va control plane. Local dev: HTTP server thuong. Tren Lambda: qua httpadapter.
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

const defaultPort = "8090"

// buildMux chi lo routing. CORS duoc boc o ngoai nen khong lap lai o day.
func buildMux(s *dashboardapi.Server) *http.ServeMux {
	mux := http.NewServeMux()

	// Read-only pages
	mux.HandleFunc("/overview", s.GetOverview)
	mux.HandleFunc("/reviews", s.ListReviews)
	mux.HandleFunc("/reviews/decide", s.DecideReview)
	mux.HandleFunc("/memory", s.GetMemory)
	mux.HandleFunc("/transactions", s.ListTransactions)
	mux.HandleFunc("/transactions/", s.GetTransactionAudit)
	mux.HandleFunc("/infrastructure", s.GetInfrastructure)
	mux.HandleFunc("/infrastructure/simulate-crash", s.SimulateCrash)
	mux.HandleFunc("/cost", s.GetCost)
	mux.HandleFunc("/cost/infrastructure", s.GetCloudCost)

	// Control plane — mutates the live system
	mux.HandleFunc("/control/fleet", s.FleetHandler)
	mux.HandleFunc("/control/dispatch", s.RunDispatch)
	mux.HandleFunc("/control/feed", s.FeedStream)
	mux.HandleFunc("/control/lambdas", s.ListLambdas)
	mux.HandleFunc("/control/resources", s.ListResources)
	mux.HandleFunc("/control/db", s.GetDbStats)
	mux.HandleFunc("/control/query", s.RunQuery)
	mux.HandleFunc("/control/regions", s.RegionsHandler)
	mux.HandleFunc("/control/search", s.Search)
	mux.HandleFunc("/control/schedule", s.ScheduleOne)
	mux.HandleFunc("/control/invoke", s.InvokeService)
	mux.HandleFunc("/control/policy", s.PolicyHandler)
	mux.HandleFunc("/control/budget", s.GetBudget)
	mux.HandleFunc("/control/memory", s.MemoryAdmin)
	mux.HandleFunc("/control/memory/job", s.MemoryJob)
	mux.HandleFunc("/control/training/metrics", s.TrainingMetrics)
	mux.HandleFunc("/control/training/log", s.TrainingLog)
	mux.HandleFunc("/control/training/runs", s.TrainingRuns)
	mux.HandleFunc("/control/ingest", s.IngestData)
	mux.HandleFunc("/control/task", s.TaskControl)
	mux.HandleFunc("/control/rollback", s.RollbackService)
	mux.HandleFunc("/reviews/bulk", s.BulkReview)

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

	handler := dashboardapi.WithCORS(buildMux(dashboardapi.NewServer(db)))

	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		adapter := httpadapter.NewV2(handler)
		lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
			return adapter.ProxyWithContext(ctx, req)
		})
		return
	}

	port := os.Getenv("DASHBOARD_API_PORT")
	if port == "" {
		port = defaultPort
	}

	log.Printf("Dashboard API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
