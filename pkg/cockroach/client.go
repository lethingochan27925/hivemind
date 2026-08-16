// client.go: quan ly connection pool toi CockroachDB, dung chung cho toan bo services.
package cockroach

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	Pool *pgxpool.Pool
}

// defaultMaxConns keeps the previous hardcoded behaviour as a fallback.
// Overridable per-process via COCKROACH_MAX_CONNS: every Lambda in the fleet
// (agent-worker, dispatcher, reaper, salience-decay, dashboard-api...) opens
// its own pool against the same CockroachDB Serverless cluster, so the total
// connection count scales with fleet size, not with any one process's needs -
// a value that could only be changed by editing Go source and redeploying
// was the wrong place to put that knob.
const defaultMaxConns = 10

func maxConnsFromEnv() int32 {
	if v := os.Getenv("COCKROACH_MAX_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return int32(n)
		}
	}
	return defaultMaxConns
}

func NewClient(ctx context.Context, databaseURL string) (*Client, error) {
	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database url: %w", err)
	}
	poolCfg.MaxConns = maxConnsFromEnv()

	connCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connCtx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(connCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &Client{Pool: pool}, nil
}

func (c *Client) Close() {
	if c.Pool != nil {
		c.Pool.Close()
	}
}
