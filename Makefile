# HiveMind — one short command per job.
#
#   make            list every target
#   make check      build + vet + fmt + test  (chay truoc khi push)
#   make ship       check + deploy backend + deploy dashboard
#
# Moi target tu nap credentials tu .env va tu tra URL/ID ha tang - khong hardcode.

SHELL := /bin/bash
.DEFAULT_GOAL := help

# Nap .env cho cac target goi truc tiep aws/terraform
ENV := set -a; [ -f .env ] && . ./.env; set +a;
TF  := terraform -chdir=terraform
API  = $$($(TF) output -raw dashboard_api_url | sed 's:/*$$::')

.PHONY: help
help: ## Liet ke moi lenh
	@echo "HiveMind — make targets"
	@echo
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo

# ---------------------------------------------------------------- build & test

.PHONY: build
build: ## Build toan bo Go
	go build ./...

.PHONY: test
test: ## Test hermetic (khong can AWS/DB)
	go test ./cmd/... ./internal/... ./pkg/... ./test/integration/

.PHONY: test-v
test-v: ## Test co chi tiet tung case
	go test ./cmd/... ./internal/... ./pkg/... ./test/integration/ -v

.PHONY: fmt
fmt: ## gofmt toan bo
	gofmt -w cmd internal pkg test

.PHONY: vet
vet: ## go vet
	go vet ./...

.PHONY: lint
lint: ## ESLint cho dashboard
	cd dashboard && npx eslint app components lib

.PHONY: fuzz
fuzz: ## Chay fuzz 60s cho tung target chinh
	go test ./internal/agent/ -run Fuzz -fuzz FuzzBalanceSignal -fuzztime 60s
	go test ./internal/agent/ -run Fuzz -fuzz FuzzSanitizeField -fuzztime 60s
	go test ./pkg/mcp/ -run Fuzz -fuzz FuzzTransactionDecode -fuzztime 60s

.PHONY: pipeline-test
pipeline-test: ## Kiem tra bat bien cua CI/CD (khong can cloud)
	bash test/integration/pipeline_test.sh

.PHONY: check
check: build vet test pipeline-test ## Tat ca kiem tra truoc khi push
	@gofmt -l cmd internal pkg test | tee /tmp/hm-fmt.txt; \
	 [ ! -s /tmp/hm-fmt.txt ] || { echo "  ^ can 'make fmt'"; exit 1; }
	@echo "[OK] moi kiem tra da qua"

# ---------------------------------------------------------------- deploy

.PHONY: deploy
deploy: ## Deploy Lambda thay doi:  make deploy S=agent-worker  (rong = ca 6 Go)
	./scripts/deploy-lambda.sh $(S)

.PHONY: deploy-api
deploy-api: ## Deploy rieng dashboard-api (nhanh nhat khi sua control plane)
	./scripts/deploy-lambda.sh dashboard-api

.PHONY: deploy-ui
deploy-ui: ## Build + deploy dashboard (S3 + CloudFront + cap nhat URL)
	./scripts/deploy-dashboard.sh

.PHONY: ship
ship: check deploy deploy-ui ## Kiem tra roi deploy tat ca

.PHONY: init
init: ## Dung TOAN BO ha tang tu zero (mot lenh duy nhat)
	./scripts/init.sh

.PHONY: destroy
destroy: ## Xoa ha tang AWS (CockroachDB giu nguyen)
	./scripts/destroy-infra.sh

.PHONY: tf-plan
tf-plan: ## terraform plan
	@$(ENV) source scripts/load-tf-vars.sh >/dev/null && $(TF) plan

.PHONY: tf-apply
tf-apply: ## terraform apply (tu go lock chet roi thu lai mot lan)
	@$(ENV) source scripts/load-tf-vars.sh >/dev/null && $(TF) apply -auto-approve || \
	 { ./scripts/tf-unlock.sh && \
	   $(ENV) source scripts/load-tf-vars.sh >/dev/null && $(TF) apply -auto-approve; }

.PHONY: unlock
unlock: ## Go lock terraform chet (chi go khi lock gia hon 20 phut)
	@$(ENV) ./scripts/tf-unlock.sh

.PHONY: iam
iam: ## Ap lai rieng IAM policy cua dashboard-api
	@$(ENV) source scripts/load-tf-vars.sh >/dev/null && \
	 $(TF) apply -auto-approve -target=module.iam.aws_iam_role_policy.dashboard_api

# ---------------------------------------------------------------- van hanh

.PHONY: urls
urls: ## In URL dashboard + API dang chay
	@$(ENV) echo "Dashboard : $$($(TF) output -raw dashboard_url)"; \
	 echo "API       : $$($(TF) output -raw dashboard_api_url)"

.PHONY: smoke
smoke: ## Smoke test control plane dang chay (30 kiem tra)
	@$(ENV) bash test/integration/api_smoke.sh "$(API)"

.PHONY: start
start: ## Chay fleet
	@$(ENV) curl -s -XPOST -H 'Content-Type: application/json' -d '{"action":"start"}' "$(API)/control/fleet"; echo

.PHONY: stop
stop: ## Dung fleet
	@$(ENV) curl -s -XPOST -H 'Content-Type: application/json' -d '{"action":"pause"}' "$(API)/control/fleet"; echo

.PHONY: feed
feed: ## Nap case vao hang doi:  make feed N=100
	@$(ENV) curl -s -XPOST -H 'Content-Type: application/json' \
	  -d '{"count":$(or $(N),50)}' "$(API)/control/feed"; echo

.PHONY: dispatch
dispatch: ## Chay mot chu ky dieu phoi
	@$(ENV) curl -s -XPOST "$(API)/control/dispatch"; echo

.PHONY: status
status: ## Trang thai fleet + hang doi
	@$(ENV) curl -s "$(API)/control/fleet" | python3 -m json.tool

.PHONY: logs
logs: ## Tail log agent-worker:  make logs S=dispatcher
	@$(ENV) aws logs tail /aws/lambda/hivemind-dev-$(or $(S),agent-worker) --follow

# ---------------------------------------------------------------- do luong

.PHONY: eval
eval: ## Scorecard tu he thong dang chay
	@$(ENV) go run ./cmd/eval --api "$(API)"

.PHONY: scorecard
scorecard: ## Scorecard + ghi vao evidence/
	@$(ENV) go run ./cmd/eval --api "$(API)" \
	  --out evidence/SCORECARD.md --json evidence/scorecard.json

.PHONY: evidence
evidence: ## Thu bang chung vao evidence/ (+ S3):  make evidence L=crash-test
	@if [ -n "$(L)" ]; then ./scripts/capture-evidence.sh --label $(L); \
	 else ./scripts/capture-evidence.sh; fi

.PHONY: experiment
experiment: ## Thi nghiem A/B ve memory:  make experiment N=60 M=8
	./scripts/memory-experiment.sh $(or $(N),60) $(or $(M),8)

.PHONY: gen-data
gen-data: ## Sinh du lieu co nhan:  make gen-data N=1000 SEED=42
	go run ./cmd/gen-data --count $(or $(N),500) --seed $(or $(SEED),42) \
	  --out data/raw/generated.csv

.PHONY: gen-edge
gen-edge: ## Sinh bo case bien (injection, unicode, bien so hoc)
	go run ./cmd/gen-data --edge-cases --out data/raw/edge.csv

.PHONY: regions
regions: ## Trang thai multi-region cua database
	./scripts/multi-region.sh status

# ---------------------------------------------------------------- tien ich

.PHONY: dev
dev: ## Chay dashboard o localhost:3000 (tro toi API that)
	@$(ENV) cd dashboard && \
	 NEXT_PUBLIC_DASHBOARD_API_URL="$$(cd .. && $(TF) output -raw dashboard_api_url)" npm run dev

.PHONY: ui-build
ui-build: ## Build dashboard (khong deploy)
	cd dashboard && npm run build

.PHONY: sql
sql: ## Chay mot cau SQL chi-doc:  make sql Q="SELECT COUNT(*) FROM tasks"
	@$(ENV) curl -s -XPOST -H 'Content-Type: application/json' \
	  -d "{\"sql\":\"$(Q)\"}" "$(API)/control/query" | python3 -m json.tool

# ---------------------------------------------------------------- ci/cd

.PHONY: cicd
cicd: ## Kiem tra 107 bat bien cua pipeline (khong can cloud)
	bash test/integration/pipeline_test.sh

.PHONY: actionlint
actionlint: ## Lint dinh nghia workflow nhu CI lam
	@command -v actionlint >/dev/null || go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.7
	actionlint -color -shellcheck=

.PHONY: ci-local
ci-local: check cicd lint ## Chay tat ca cong CI ngay tren may
	@echo "[OK] moi cong CI da qua"

.PHONY: runs
runs: ## Trang thai GitHub Actions gan nhat
	@gh run list --limit 12

.PHONY: why
why: ## Log cua lan chay that bai gan nhat:  make why
	@gh run view "$$(gh run list --limit 20 --json databaseId,conclusion \
	  -q '[.[]|select(.conclusion=="failure")][0].databaseId')" --log-failed | tail -60

.PHONY: memory-restore
memory-restore: ## Khoi phuc toan bo ky uc bi archive (sau thi nghiem A/B)
	@$(ENV) curl -s -XPOST -H 'Content-Type: application/json' \
	  -d '{"job":"unarchive_all"}' "$(API)/control/memory/job" | python3 -m json.tool
