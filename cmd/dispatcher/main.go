// main.go: dispatcher entrypoint - tao task cho transaction medium-tier chua co task,
// roi dieu chinh kich thuoc fleet theo so task dang cho.
// Tren Lambda that: moi invoke la mot chu ky, EventBridge lo viec lap lai.
// Local dev: vong lap tay de giu duoc workflow chay ngoai AWS.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	lambdasvc "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/lethingochan27925/hivemind/internal/budget"
	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/internal/stream"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

const (
	defaultBatchSize        = 100
	defaultMaxWorkerInvokes = 20
	workerAlias             = "live"
	localCycleInterval      = 5 * time.Second
)

// Dispatcher bien so task dang cho thanh kich thuoc fleet: hang doi rong thi
// khong invoke worker nao, tranh dot chi phi Bedrock khi khong co viec.
type Dispatcher struct {
	db               *cockroach.Client
	lambdaClient     *lambdasvc.Client
	workerFunction   string
	batchSize        int
	maxWorkerInvokes int
}

type CycleResult struct {
	TasksCreated   int  `json:"tasks_created"`
	PendingTasks   int  `json:"pending_tasks"`
	WorkersInvoked int  `json:"workers_invoked"`
	BudgetExceeded bool `json:"budget_exceeded,omitempty"`

	SpendTodayUSD float64 `json:"spend_today_usd,omitempty"`
}

func NewDispatcher(ctx context.Context) (*Dispatcher, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	db, err := cockroach.NewClient(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	d := &Dispatcher{
		db:               db,
		workerFunction:   os.Getenv("WORKER_FUNCTION_NAME"),
		batchSize:        envInt("DISPATCHER_BATCH_SIZE", defaultBatchSize),
		maxWorkerInvokes: envInt("DISPATCHER_MAX_WORKER_INVOKES", defaultMaxWorkerInvokes),
	}

	// Local dev khong co worker Lambda de goi - bo qua client thay vi bat
	// nguoi chay phai co AWS credential.
	if d.workerFunction != "" {
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("loading AWS config: %w", err)
		}
		d.lambdaClient = lambdasvc.NewFromConfig(awsCfg)
	}

	return d, nil
}

func (d *Dispatcher) RunOnce(ctx context.Context) (CycleResult, error) {
	created, err := stream.InsertMediumTasks(ctx, d.db, d.batchSize)
	if err != nil {
		return CycleResult{}, err
	}

	pending, err := d.countPendingTasks(ctx)
	if err != nil {
		return CycleResult{TasksCreated: created}, err
	}

	// PHANH NGAN SACH o noi duy nhat sinh chi phi Bedrock. Truoc day guard
	// nam trong GET /cost/budget - chi chay khi co nguoi mo trang Cost, tuc
	// khong phai la phanh. O day no chay moi chu ky du khong ai nhin.
	// Loi kiem tra thi fail-open: mot cu glitch DB khong duoc phep dung fleet.
	spend, limit, exceeded, berr := budget.Exceeded(ctx, d.db)
	if berr != nil {
		log.Printf("budget check failed (fail-open): %v", berr)
	}
	if exceeded {
		log.Printf("daily budget exhausted ($%.4f >= $%.2f) - not invoking workers", spend, limit)
		return CycleResult{
			TasksCreated:   created,
			PendingTasks:   pending,
			BudgetExceeded: true,
			SpendTodayUSD:  spend,
		}, nil
	}

	invoked, err := d.scaleFleet(ctx, pending)
	return CycleResult{
		TasksCreated:   created,
		PendingTasks:   pending,
		WorkersInvoked: invoked,
		SpendTodayUSD:  spend,
	}, err
}

func (d *Dispatcher) countPendingTasks(ctx context.Context) (int, error) {
	var count int
	err := d.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM tasks WHERE status = 'pending'`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting pending tasks: %w", err)
	}
	return count, nil
}

// scaleFleet invoke worker bat dong bo, so luong bang so task dang cho nhung
// khong vuot maxWorkerInvokes. Worker co reserved concurrency nen bi throttle
// la hanh vi binh thuong, khong phai loi: chi bao loi khi khong invoke duoc ai.
//
// Invokes chay SONG SONG, khong tuan tu: voi maxWorkerInvokes mac dinh la 20,
// vong lap tuan tu cu keo dai moi chu ky dispatch bang 20 lan round-trip HTTP
// noi tiep nhau toi Lambda Invoke API - moi Invoke doc lap va fire-and-forget
// (InvocationTypeEvent), nen khong co ly do gi phai doi tung cai. Loi o 1
// goroutine khong duoc phep dung cac goroutine khac lai, giu dung dac tinh
// chiu loi cua vong lap tuan tu ban dau: chi bao loi khi khong invoke duoc ai.
func (d *Dispatcher) scaleFleet(ctx context.Context, pendingTasks int) (int, error) {
	if d.lambdaClient == nil {
		return 0, nil
	}

	fleetSize := min(pendingTasks, d.maxWorkerInvokes)
	if fleetSize == 0 {
		return 0, nil
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		invoked int
		lastErr error
	)
	for i := 0; i < fleetSize; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := d.lambdaClient.Invoke(ctx, &lambdasvc.InvokeInput{
				FunctionName:   aws.String(d.workerFunction),
				Qualifier:      aws.String(workerAlias),
				InvocationType: lambdatypes.InvocationTypeEvent,
				Payload:        []byte("{}"),
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				lastErr = err
				return
			}
			invoked++
		}()
	}
	wg.Wait()

	if invoked == 0 {
		return 0, fmt.Errorf("invoking %d worker(s): %w", fleetSize, lastErr)
	}
	if lastErr != nil {
		log.Printf("fleet partially scaled: %d/%d invoked, last error: %v", invoked, fleetSize, lastErr)
	}
	return invoked, nil
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func main() {
	ctx := context.Background()

	d, err := NewDispatcher(ctx)
	if err != nil {
		log.Fatalf("initializing dispatcher: %v", err)
	}
	defer d.db.Close()

	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(func(ctx context.Context) (CycleResult, error) {
			return d.RunOnce(ctx)
		})
		return
	}

	log.Printf("Dispatcher started (local mode, cycle=%s)", localCycleInterval)
	for {
		res, err := d.RunOnce(ctx)
		if err != nil {
			log.Printf("dispatch cycle error: %v", err)
		} else if res.TasksCreated > 0 || res.WorkersInvoked > 0 {
			log.Printf("created=%d pending=%d workers=%d",
				res.TasksCreated, res.PendingTasks, res.WorkersInvoked)
		}
		time.Sleep(localCycleInterval)
	}
}
