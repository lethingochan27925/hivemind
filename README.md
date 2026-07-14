## Project Structure

```text
hivemind/
├── README.md
├── Makefile                        # make dev, make deploy, make demo
├── go.mod / go.sum
├── .env.example
├── .gitignore
│
├── cmd/                            # 3 binary entrypoints
│   ├── dispatcher/main.go          # nhận stream → insert tasks
│   ├── worker/main.go              # agent fleet (claim → reason → audit)
│   └── scoring-api/main.go         # HTTP API: nhận txn → risk_score → route
│
├── internal/                       # business logic, không expose ra ngoài
│   ├── agent/
│   │   ├── agent.go                # vòng lặp chính: claim → investigate → verdict
│   │   ├── reasoning.go            # Bedrock prompt builder + response parser
│   │   └── resume.go               # đọc scratchpad, resume-after-crash
│   ├── memory/
│   │   ├── working.go              # claim task (SKIP LOCKED), heartbeat, re-queue
│   │   ├── episodic.go             # vector search top-k, recall, salience update
│   │   ├── audit.go                # append-only audit log writer
│   │   ├── consolidation.go        # merge case nếu similarity > 0.92
│   │   └── salience.go             # background job: decay + archive
│   ├── scorer/
│   │   ├── scorer.go               # load fraud_scorer.pkl, predict_proba
│   │   └── router.go               # routing: low/medium/high → action
│   └── stream/
│       ├── paysim.go               # đọc PaySim CSV, engineer features
│       └── replay.go               # controlled replay (demo mode + full mode)
│
├── pkg/                            # shared clients, có thể dùng từ nhiều binary
│   ├── cockroach/
│   │   ├── client.go               # connection pool, retry logic
│   │   ├── vector.go               # vector search wrapper
│   │   └── mcp.go                  # CockroachDB MCP Server client
│   ├── bedrock/
│   │   ├── client.go               # AWS Bedrock client (Claude Haiku/Sonnet)
│   │   └── embed.go                # Titan Embeddings v2
│   └── mcp/
│       └── client.go               # MCP protocol client chung
│
├── migrations/                     # chạy theo thứ tự
│   ├── 001_init.sql                # 4 bảng chính (= schema.sql đã có)
│   ├── 002_indexes.sql             # vector index + các index phụ
│   └── 003_views.sql               # agent_performance, task_summary, fraud_accuracy
│
├── scripts/
│   ├── init.sh                     # ./scripts/init.sh dev → dựng toàn bộ hạ tầng
│   ├── setup-vars.sh               # push secrets lên GitLab CI/CD
│   ├── train-scorer.py             # train XGBoost → fraud_scorer.pkl
│   └── demo-stream.py              # adapter.py (= file đã có)
│
├── deployments/
│   ├── terraform/
│   │   ├── main.tf
│   │   ├── variables.tf
│   │   ├── outputs.tf
│   │   └── modules/
│   │       ├── gke/                # hoặc Lambda nếu dùng AWS
│   │       ├── networking/
│   │       └── iam/
│   └── k8s/
│       ├── dispatcher/deployment.yaml
│       ├── worker/deployment.yaml   # replicas: 20 cho fleet mode
│       └── dashboard/deployment.yaml
│
├── data/
│   ├── raw/.gitkeep                # PaySim CSV (không commit, gitignore)
│   └── processed/.gitkeep         # fraud_scorer.pkl (không commit)
│
└── dashboard/                      # React + Vite
    ├── src/
    │   ├── components/             # FleetStatus, TaskThroughput, AuditViewer
    │   ├── pages/
    │   ├── hooks/
    │   └── api/                    # fetch từ scoring-api
    └── public/
→
→
```
→
→