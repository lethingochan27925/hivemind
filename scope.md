HiveMind — Distributed Memory & Control Plane for Production Agent Fleets

Hackathon: CockroachDB × AWS — Build with Agentic Memory
Deadline: 18/08/2026, 5:00 PM ET
Giải thưởng: $8,750 USD
Team: Lê Thị Ngọc Hân (brain/memory: Python, ML, agent logic) & Huỳnh Ngọc Thuận (infra/Go: deployment, Kubernetes-free Lambda stack) 🔄
Pitch một câu: HiveMind là control plane + lớp memory phân tán (CockroachDB) cho agent fleets trong production — sống sót qua agent crash lẫn region failure, có governance & telemetry đầy đủ. Để chứng minh nó chịu được workload khắt khe nhất, chúng tôi vận hành một hệ thống điều tra gian lận thanh toán hai tầng (ML scorer + LLM agent investigator) trên dataset PaySim, nơi mất state = mất tiền và mọi quyết định phải audit được. 🔄


1. Bài toán & Giá trị doanh nghiệp
Bài toán: vận hành agent fleet trong production
Doanh nghiệp đang chuyển từ "một chatbot" sang "nhiều agents chạy tự động trong quy trình thật". Ngay khi fleet vượt quá vài agents, ba vấn đề hạ tầng xuất hiện — và chưa có lớp chuẩn nào giải quyết:

Memory không được phép chết — agent đang giữ trạng thái nghiệp vụ (đang hold giao dịch của khách) mà mất state = tiền treo, việc bỏ dở, hoặc hành động bị lặp lại.
Nhiều agents chạy song song — cần cơ chế claim task không trùng lặp, chia sẻ kinh nghiệm giữa các agents, và không giẫm chân nhau khi ghi state liên tục.
Không ai nhìn thấy fleet đang làm gì — thiếu audit trail (compliance), cost tracking, latency telemetry cho từng agent. "AI quyết định vậy" không phải câu trả lời chấp nhận được với regulator.

Khách hàng mục tiêu: Platform Engineering / CTO của các doanh nghiệp đang đưa agents vào production — thị trường đang bùng nổ nhưng chưa có tiêu chuẩn.
Workload chứng minh: fraud investigation hai tầng 🔄
Để chứng minh HiveMind chịu được workload khắt khe nhất, demo vận hành một hệ thống điều tra fraud hai tầng đúng chuẩn industry:

Tầng 1 — ML Scorer (XGBoost trên PaySim): Nhanh, rẻ, quét mọi giao dịch. Kết luận tự động ở hai đầu (risk < 0.001 → clean, risk > 0.999 → fraud). Không gọi LLM cho hai case này.
Tầng 2 — LLM Agent Investigator: Chỉ nhận case ở vùng xám (0.001 ≤ risk ≤ 0.999). Điều tra bằng Bedrock reasoning + episodic memory, ghi audit, đưa verdict có giải thích.

Đây không phải giải pháp tạm mà là kiến trúc đúng ngành: fraud detection thực tế luôn có scoring layer (rẻ, high-recall) + investigation layer (đắt, cần reasoning). HiveMind cung cấp control plane cho tầng investigation, và tầng scoring vừa là input source vừa là filter cost.
Fraud được chọn có chủ đích vì nó ép control plane thể hiện mọi năng lực:

Mất state = mất tiền thật → bắt buộc durable working memory.
Pattern gian lận lặp lại và tiến hóa → episodic memory chung của fleet tạo giá trị đo được.
Ngành tài chính đòi hỏi audit trail pháp lý → audit memory là điều kiện tồn tại.
Đúng ngành trọng điểm của CockroachDB (fintech: strong consistency, compliance, zero downtime).

Giá trị

Cho doanh nghiệp vận hành agents: một lớp memory + governance dùng chung, thay vì mỗi team tự chế Redis + vector DB + Postgres rời rạc.
Cho nghiệp vụ demo: ML scorer xử lý ~99% traffic tự động, chỉ ~1% vào tầng agent — cost tổng giảm mạnh so với LLM-only. Với case vào tầng agent, kiến thức điều tra tích lũy vào episodic memory chung, không mất khi nhân sự nghỉ. 🔄
Cho compliance: mọi quyết định của agent truy vết được bằng một câu SQL.


2. Đáp ứng yêu cầu cuộc thi (Compliance Matrix)
Yêu cầuCách đáp ứngCockroachDB làm persistent memory layer3 tầng memory: episodic (vector + text), working (transactional), audit (append-only), multi-region (SIN+JKT+BOM) 🔄≥ 2 công cụ CockroachDBDùng cả 4: Managed MCP Server, Distributed Vector Indexing, ccloud CLI, Agent Skills Repo≥ 1 dịch vụ AWSBedrock (reasoning + embeddings), Lambda (Scoring/Dispatcher/Worker/Reaper), EventBridge (schedule reaper), S3 (evidence store) 🔄Repo public, open sourceGitHub public tại lethingochan27925/hivemind, license Apache 2.0, README + setup đầy đủ 🔄Video demo3 phút, kịch bản 2 cú knock-out (xem mục 8)Architecture diagramMermaid trong READMEFeedback về CockroachDB AI toolsGhi nhận trong SUBMISSION.md

3. Kiến trúc tổng thể 🔄
   PaySim CSV ──▶ demo-stream.py (Python dev / Go prod)
                         │ HTTP POST /score
                         ▼
              ┌──────────────────────────────┐
              │  ⚡ Scoring Lambda            │
              │  (XGBoost fraud_scorer.pkl)   │
              │  ┌─ risk < 0.001 ─▶ auto: CLEAN (audit only)
              │  ├─ risk > 0.999 ─▶ auto: FRAUD (audit only)
              │  └─ otherwise  ──▶ ⚡ Dispatcher Lambda
              └──────────────────────────────┘
                                   │ SQL INSERT task
                                   ▼
                    ┌──────────────────────────────────────┐
                    │  CockroachDB Cloud (Serverless,       │
                    │  multi-region: SIN + JKT + BOM)       │
                    │  ┌────────────────────────────────┐   │
                    │  │ Working Memory                  │   │
                    │  │ tasks(status, claimed_by,       │   │
                    │  │       heartbeat_at, scratchpad) │   │
                    │  ├────────────────────────────────┤   │
                    │  │ Episodic Memory                 │   │
                    │  │ case_memory(summary, VECTOR     │   │
                    │  │             (1024), salience)   │   │
                    │  ├────────────────────────────────┤   │
                    │  │ Audit Memory                    │   │
                    │  │ audit_log (append-only)         │   │
                    │  └────────────────────────────────┘   │
                    └───────┬────────────────┬──────────────┘
        claim task          │                │  read-only
   (FOR UPDATE SKIP LOCKED) │                │  business queries
                            ▼                ▼
              ┌──────────────────────┐  ┌─────────────────────┐
              │ ⚡ Agent Worker ×N    │  │ CockroachDB MCP     │
              │ (Lambda, Go prod /    │  │ Server (managed)    │
              │  Python dev)          │  │ get_transaction()   │
              │                       │  │ get_customer_ctx()  │
              │  ├─▶ Bedrock          │  │ search_similar()    │
              │  │  Claude Haiku      │  └─────────────────────┘
              │  │  (ap-southeast-1)  │
              │  └─▶ Bedrock          │──▶ S3 (evidence)
              │     Titan Embed v2    │
              │     (us-east-1 !!)    │
              └──────────────────────┘
                            ▲
                            │ re-queue nếu heartbeat quá hạn
              ┌─────────────────────────────┐
              │ ⚡ Heartbeat Reaper Lambda   │
              │ (trigger: EventBridge       │
              │  Scheduled Rule, mỗi 30s)   │
              └─────────────────────────────┘
                            │ telemetry (CloudWatch + DB)
                            ▼
              ┌──────────────────────────────┐
              │ Mission Control Dashboard    │
              │ (React + Vite, static → S3   │
              │  + CloudFront)               │
              └──────────────────────────────┘
Thành phần
Thành phầnCông nghệVai tròData sourcePaySim (CC BY 4.0) → replay scriptNguồn giao dịch mô phỏng, phân phối bimodal có kiểm soát 🔄Scoring LambdaLambda + XGBoostPredict risk score, tự kết luận 2 đầu, route case xám sang Dispatcher 🔄DispatcherLambdaNhận case xám, INSERT task vào working memoryAgent WorkerLambda ×NClaim → MCP query → recall memory → Bedrock reasoning → verdict → auditHeartbeat ReaperLambda + EventBridge Schedule (30s)Quét task heartbeat_at quá hạn, set lại status='pending' 🔄Memory LayerCockroachDB Cloud Serverless, multi-region SIN+JKT+BOM3 tầng memory: working + episodic + audit 🔄ReasoningBedrock Claude 3 Haiku (dev + demo)Phân tích case, sinh lập luận NL 🔄EmbeddingsBedrock Titan Embed Text v2 (amazon.titan-embed-text-v2:0), 1024-dimVector hóa case summary (async, sau khi đóng case) 🔄MCPCockroachDB Managed MCP ServerAgent tự query business data (read-only, safe by default)Evidence StoreS3Raw transaction logDashboardReact + Vite → S3 + CloudFrontFleet status, cost/latency telemetry, audit trail viewerIaCTerraform + ccloud CLIToàn bộ AWS + CockroachDB cluster từ zeroCI/CDGitHub ActionsBuild, test, deploy tự động
⚠️ Ràng buộc region cần lưu ý 🔄

Titan Embed Text v2 hiện chỉ available ở us-east-1 và us-west-2 (đã verify qua AWS docs). Toàn bộ stack chính đặt ở ap-southeast-1 (gần team, gần CockroachDB SIN), nhưng bắt buộc phải giữ boto3 client Bedrock riêng ở us-east-1 để gọi Titan v2. Cross-region call chấp nhận được vì async construction (không nằm trên hot path của user).
Claude Haiku có sẵn ở ap-southeast-1, dùng client local.

Luồng phát triển & deploy 🔄
Chúng tôi không viết Python và Go song song. Quy trình:

Giai đoạn Python (dev): Ngoc Han viết toàn bộ flow bằng Python (agent_loop.py, dispatcher.py, scoring_api.py, heartbeat_reaper.py) — chạy local hoặc Lambda container image, validate correctness end-to-end với CockroachDB Cloud thật.
Giai đoạn Go (prod): Sau khi flow Python chạy đúng, Thuận port toàn bộ sang Go native Lambda handlers (cmd/scoring-api, cmd/dispatcher, cmd/worker, cmd/reaper). Bản Go là bản duy nhất deploy production; Python nghỉ hưu sau khi port xong (chỉ giữ lại làm reference trong notebooks/ cho hackathon judges).

Python đóng vai trò spec sống trong lúc port, không phải một branch song song vĩnh viễn.

4. Nguồn dữ liệu & Chiến lược Data 🔄
4.1 Vì sao chọn PaySim (không phải IEEE-CIS)
Dataset PaySim (CC BY 4.0, Lopez-Rojas 2016) được chọn thay vì IEEE-CIS vì lý do license: IEEE-CIS trên Kaggle mang điều khoản non-commercial, không tương thích với hackathon có giải thưởng tiền mặt. PaySim license commercial-friendly, an toàn để submit và open-source repo.
Schema PaySim mà agent lý luận được:
TrườngÝ nghĩaAgent dùng đểstepĐơn vị thời gian (giờ)Xác định velocity theo cửa sổ thời giantypeLoại giao dịch (CASH_OUT, TRANSFER, PAYMENT, DEBIT, CASH_IN)Nhận diện loại rủi ro (CASH_OUT + TRANSFER là nơi fraud tập trung)amountSố tiềnAmount pattern, so sánh với balancenameOrig, nameDestID người gửi/nhậnĐịnh danh account, phát hiện destination lặp lạioldbalanceOrg, newbalanceOrigBalance của sender trước/sauPhát hiện "wipe out" (balance về 0 sau giao dịch)oldbalanceDest, newbalanceDestBalance của receiverPhát hiện receiver không hoạt động bỗng nhận lớnisFraudGround truth labelEval accuracy của cả scorer lẫn agent
Câu lập luận mẫu agent có thể sinh: "CASH_OUT amount = 181k, sender wipe-out (oldbalance 181k → newbalance 0), destination account từng có 0 balance và không hoạt động — pattern fraud CASH_OUT điển hình trong PaySim."
4.2 Đặc điểm phân phối bimodal của PaySim
XGBoost train trên PaySim sinh phân phối score bimodal, dồn gần 0 và gần 1 — rất ít case ở vùng giữa với threshold chuẩn (0.5). Đây vừa là điểm mạnh (scorer phân loại dứt khoát) vừa là điểm cần xử lý cho demo:

Threshold sản xuất: low = 0.001, high = 0.999 — đảm bảo vùng xám vẫn có đủ case (~1-2% traffic) để agent xử lý.
Cho demo: inject thêm case biên (mix noise vào các fraud pattern rõ ràng) để bảo đảm agent có case điều tra trong 3 phút quay video, tránh trường hợp fleet đứng nhàn rỗi.

Phân phối thống kê PaySim gốc (fraud rate ~0.13%, chỉ tập trung ở CASH_OUT và TRANSFER) được giữ nguyên trong replay, chỉ tinh chỉnh thứ tự để có narrative kể chuyện cho demo (pattern lần 1 → agent mò → pattern lần 2 → agent recall).
4.3 Ground truth cho eval
isFraud trong PaySim là label cuối cùng → dùng để đo:

Scorer accuracy (precision/recall/F1) trên tầng 1.
Agent verdict accuracy trên vùng xám. Kỳ vọng: ≥ 70% (vì đây là vùng khó nhất — case ML không quyết được).


5. Thiết kế 3 tầng Memory

Nguyên tắc thiết kế từ literature: Memory không phải storage dump. Mỗi tầng có lý do nghiệp vụ riêng, và correctness là property của trajectory, không phải của từng record đơn lẻ (GEM paper, Concordia 2026).

5.1 Working Memory — transactional state
sqlCREATE TABLE tasks (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  txn_id        STRING      NOT NULL UNIQUE,   -- PaySim step + nameOrig + nameDest
  risk_score    FLOAT       NOT NULL,           -- điểm từ XGBoost scorer
  status        STRING      NOT NULL DEFAULT 'pending',
                                                -- pending|claimed|investigating|done|failed
  claimed_by    STRING,                         -- agent worker id
  claimed_at    TIMESTAMPTZ,
  heartbeat_at  TIMESTAMPTZ,                    -- reaper phát hiện quá hạn → re-queue
  step          STRING,                         -- bước điều tra hiện tại (resume point)
  scratchpad    JSONB,                          -- {mcp_result, top_k_cases, partial_reasoning}
  created_at    TIMESTAMPTZ DEFAULT now()
);
Cơ chế quan trọng:

Claim không trùng: SELECT ... FOR UPDATE SKIP LOCKED.
Resume sau crash: heartbeat_at quá hạn 30s → EventBridge-scheduled Reaper Lambda re-queue về pending. Agent mới đọc step + scratchpad, làm tiếp đúng chỗ dở. 🔄
Idempotency guard: txn_id UNIQUE — không thể hold cùng một giao dịch hai lần.

5.2 Episodic Memory — hai lớp text + vector
sqlCREATE TABLE case_memory (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  summary       TEXT        NOT NULL,           -- agent đọc khi recall (NL)
  verdict       STRING      NOT NULL,           -- fraud | legit | escalate
  key_signals   STRING[],                       -- ["wipe_out","cash_out_new_dest"]
  pattern_type  STRING,                         -- cash_out_wipeout | transfer_split | ...
  embedding     VECTOR(1024),                   -- Titan Embed v2 output 🔄
  salience      FLOAT       DEFAULT 1.0,
  recall_count  INT         DEFAULT 0,
  archived      BOOLEAN     DEFAULT false,
  source_task_id UUID,
  data_source   STRING      DEFAULT 'paysim',   -- 🔄
  created_at    TIMESTAMPTZ DEFAULT now()
);
CREATE VECTOR INDEX ON case_memory (embedding) WHERE archived = false;
Hai luồng xử lý tách biệt:

Construction (async, us-east-1): sau khi agent đóng case → Bedrock Haiku tóm tắt case → Titan v2 sinh embedding → similarity check (>0.92 merge, ngược lại insert) → ghi case_memory. 🔄
Query (sync, ap-southeast-1 cho Haiku + us-east-1 cho Titan): embed mô tả alert → vector search top-3 → nhét summary vào prompt Haiku.

Context window luôn giữ nhỏ: system_prompt + top-3 summaries + current case.
Consolidation + Salience decay: giữ nguyên logic từ bản gốc (merge on similarity >0.92, decay salience hằng giờ, archive khi <0.1).
5.3 Audit Memory — append-only + telemetry
sqlCREATE TABLE audit_log (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id       UUID        REFERENCES tasks(id),  -- NULL nếu auto-decided ở scorer
  agent_id      STRING      NOT NULL,               -- 'scorer' hoặc worker-uuid 🔄
  action        STRING      NOT NULL,
  -- scored_auto_clean | scored_auto_fraud | dispatched | queried_mcp
  -- | recalled_memory | bedrock_reasoning | decided | held_txn
  reasoning     TEXT,
  tokens_in     INT,
  tokens_out    INT,
  latency_ms    INT,
  memory_hits   INT,
  similarity_scores FLOAT[],
  evidence_s3_key STRING,
  created_at    TIMESTAMPTZ DEFAULT now()
);
Mọi quyết định — kể cả case scorer tự xử lý ở hai đầu — đều ghi audit. Compliance tra cứu bằng một câu SQL.

6. Why CockroachDB / Why Memory (kịch bản phản biện Q&A)
6.1 "Sao không dùng Postgres?"
Những gì Postgres làm được y hệt: task queue với SKIP LOCKED, pgvector search quy mô nhỏ, audit trail bảng thường.
→ Demo single-region, 20 agents, vài trăm task: RDS Postgres cân được hết.
Ba điểm gãy khi fleet vào production thật:

Region failure với RPO = 0. Postgres failover mất vài giây đến vài phút, replication bất đồng bộ có thể mất bản ghi cuối. Với agent đang hold giao dịch của khách, mất bản ghi cuối = thảm họa. CockroachDB không có khái niệm failover: consensus tiếp tục ở các region còn lại, RPO = 0. → Video phút 2:25.
Scale ghi ngang. Fleet ghi-nặng bẩm sinh. 20 agents Postgres ổn; 2.000 agents single-writer nghẽn. CockroachDB thêm node là xong.
Ba loại data trong một hệ nhất quán. Redis + Pinecone + Postgres = ba failure modes, không có transaction xuyên suốt. Một transaction bao cả ba là lập luận về correctness.

6.2 "Tại sao agent cần memory?"

LLM bẩm sinh không có trí nhớ — memory biến chi phí điều tra thành tài sản tích lũy.
Agent phải chết được mà không mất việc — Lambda timeout, cold start, container recycle: state RAM = mất case.
Fleet cần bộ não chung — N agents không share memory = N bot đơn độc.
Doanh nghiệp cần bằng chứng pháp lý — audit memory.

6.3 "Đã có ML scorer, còn cần LLM agent làm gì?" 🔄 (mới)
Đây là câu chốt của kiến trúc hai tầng:

Scorer tự tin (>99% hoặc <0.1%): tự quyết, không cần LLM. Cost gần bằng 0, latency ms.
Scorer không tự tin (vùng xám): LLM giá trị nhất ở đây — vì đây là chỗ rule-based và ML thuần đều thất bại. Agent kết hợp:

Context động qua MCP (customer history, related txns) — thứ ML không đọc được lúc predict.
Kinh nghiệm tổ chức qua episodic memory — case tương tự trước đây đã xử lý thế nào.
Reasoning có giải thích — audit trail dạng ngôn ngữ tự nhiên, không phải SHAP values regulator không hiểu.



Nói cách khác: ML scorer tối ưu cost + throughput; LLM agent tối ưu correctness + explainability trên vùng khó nhất.
6.4 "Tại sao không full-context thay vì retrieval?"
Stanford (Omri et al. 2026): long-context ~38s/query, retrieval <0.1s/query — chênh 380×. Prefill cost bậc hai với context length. HiveMind dùng top-k retrieval, context window luôn kiểm soát được.

7. Phạm vi (Scope)
✅ In scope 🔄

PaySim replay stream — 3 fraud patterns: CASH_OUT wipeout, TRANSFER split, dormant destination.
XGBoost fraud scorer (fraud_scorer.pkl) — train trên PaySim, exposed qua Scoring Lambda, tự quyết vùng cực trị.
Dispatcher Lambda — nhận case xám từ scorer, INSERT vào working memory.
Agent Worker Lambda ×N — vòng điều tra đầy đủ, hỗ trợ resume-after-crash.
Heartbeat Reaper Lambda + EventBridge Schedule — re-queue stuck tasks (giả định thiết kế: schedule 30s, threshold heartbeat quá hạn 30s).
Memory management — consolidation on insert (similarity > 0.92 → merge), salience decay + archiving.
Fleet mode — ≥ 20 workers đồng thời, chứng minh không trùng claim.
Mission Control Dashboard — 4 khối: fleet status, task throughput, cost/latency per agent, audit trail viewer.
Terraform + ccloud CLI — ./scripts/init.sh dựng toàn bộ hạ tầng từ zero.
Multi-region live — SIN+JKT+BOM đã bật, kịch bản kill-region.
Agent Skills — nạp skills từ CockroachDB Agent Skills Repo; đóng góp ngược 1 skill (audit-trail schema với salience-driven forgetting cho agentic workflows).
Repo hoàn chỉnh — README, architecture diagram, Apache 2.0, setup guide, eval metrics, video demo 3 phút.
Go port hoàn tất cho production deploy; Python giữ lại chỉ trong notebooks/ làm reference.

❌ Out of scope 🔄

Train ML classifier riêng (đã đưa vào scope: XGBoost tầng 1).
Tích hợp payment gateway thật (Stripe/Adyen).
Authentication/multi-tenant cho dashboard.
Human-in-the-loop UI phê duyệt (future work).
Kafka/event streaming (Lambda + SQL queue là đủ).
Parametric memory / fine-tuning model.

⚠️ Rủi ro & phương án 🔄
Rủi roMứcPhương ánTitan v2 cross-region latency (us-east-1 từ ap-southeast-1)Trung bìnhAsync construction, không nằm hot path; batch nếu cầnPaySim vùng xám quá ít, agent thiếu case demoTrung bìnhInject noise vào fraud pattern rõ để tạo case biên có kiểm soátMCP Server read-only không đủ cho luồngThấpRead qua MCP, write qua SQL driver — đúng thiết kếMulti-region đã bật sớm → ăn credits nhanhTrung bìnhMonitor CockroachDB Cloud usage; sẵn phương án tạm scale-down 2 region nếu credits cạnBedrock chi phí vượt dự kiếnThấpDùng Haiku xuyên suốt; prompt caching; scorer đã lọc ~99% trafficTrễ tiến độ Go portTrung bìnhNếu tuần T5 chưa port xong, giữ Python trên Lambda container image cho demo (vẫn đạt DoD)Verdict accuracy < 60% trên vùng xámThấpTune prompt + few-shot examples; thêm rule hints cho 3 pattern chính

8. Kịch bản video demo 3 phút 🔄
Thời điểmCảnhThông điệp0:00–0:20"Agent fleet không memory là gì? 380× chậm hơn, không biết gì về ngày hôm qua, chết là mất việc."Hook0:20–0:45PaySim stream chạy → Scoring Lambda tự quyết ~99% (dashboard hiện tỉ lệ auto-decided), 1% vào DispatcherTwo-tier: ML rẻ + LLM đắt, đúng cost-model production0:45–1:20Fleet 20 agents claim task vùng xám không trùng; case CASH_OUT wipeout lần 1 — agent mò ~90sHigh-concurrency + serializable isolation1:20–1:50Case CASH_OUT wipeout lần 2 → agent recall top-2 case từ episodic memory, verdict trong 10s; show SQL: summary, similarity_score, reasoningMemory biến fleet thành tổ chức biết học1:50–2:15kill -9 10 agents Lambda giữa chừng → EventBridge Reaper re-queue trong <30s → agent khác đọc scratchpad resume đúng bước; audit trail liền mạchDurable agent state + reaper2:15–2:45Kill region primary (SIN) → fleet tiếp tục ở JKT+BOM, zero data loss, audit trail không đứt. Show CockroachDB console: SIN sập, consensus 2 region, cluster healthy"Memory that never goes down"2:45–3:00Architecture recap 1 slide + "One command to deploy. Apache 2.0. Link repo."Chốt

9. Kế hoạch theo tuần (07/07 → 18/08) 🔄
TuầnMục tiêuDeliverablePhân côngT1 (07–13/07)Nền móng + spikeTerraform (Lambda/S3/IAM/EventBridge), CockroachDB multi-region cluster, schema 3 tầng, spike vector index, spike MCP, spike Titan v2 từ ap-southeast-1 → us-east-1Ngoc Han: schema + Titan spike; Thuận: Terraform + IAMT2 (14–20/07)ML scorer + Python single-modeTrain XGBoost trên PaySim → fraud_scorer.pkl; Scoring API Python; PaySim replay script; agent worker Python end-to-endNgoc Han: scorer + agent logic; Thuận: dispatcher + CI/CDT3 (21–27/07)Fleet + memory + Python-Go port bắt đầuFleet ≥ 20 workers Python, resume-after-crash, consolidation + salience decay, EventBridge Reaper. Thuận bắt đầu port worker sang Go song song. Checkpoint cắt scope tại đâyNgoc Han: memory logic + eval; Thuận: bắt đầu Go portT4 (28/07–03/08)Dashboard + eval + Go port tiếp tục4 khối dashboard real-time; verdict accuracy trên 200 labeled cases; Go port worker + dispatcher hoàn tấtNgoc Han: dashboard + eval; Thuận: Go portT5 (04–10/08)Multi-region demo + polish + Go port hoàn tấtKịch bản kill-region SIN, đóng góp skill lên Agent Skills Repo, security IAM least-privilege. Deploy Go binaries lên Lambda production.Cả haiT6 (11–18/08)SubmissionQuay video, README hoàn chỉnh, SUBMISSION.md, nộp ≥ 24h trước deadlineCả hai

10. Ngân sách 🔄
KhoảnƯớc tínhGhi chúCockroachDB Cloud$0$400 trial credits; multi-region đã bật từ T1 → monitor sát, có phương án scale-down nếu cầnBedrock Titan Embed v2 (us-east-1)$1–5Async construction, ~10K case summaries × ~$0.02/1M tokensBedrock Claude 3 Haiku (ap-southeast-1)$10–25Chỉ ~1% traffic vào tầng agent (nhờ scorer lọc) → cost thấp hơn kế hoạch cũLambda / EventBridge / S3 / CloudWatch$0–3Trong free tierTổng≈ $11–33Trần rủi ro < $100
Ngày đầu tiên: billing alert $50 trên AWS + $100 trên CockroachDB Cloud.

11. Tiêu chí thành công nội bộ (Definition of Done)

 ./scripts/init.sh dựng toàn bộ hạ tầng từ zero, không thao tác tay
 PaySim stream chạy được, 3 fraud patterns có kịch bản controlled replay 🔄
 Scoring Lambda auto-decide đúng ở 2 đầu, route xám sang Dispatcher, ghi audit đủ 3 nhánh 🔄
 Fleet 20+ agents xử lý 500 tasks vùng xám không trùng claim, không mất task
 Kill agent bất kỳ → task resume đúng bước trong < 30 giây (EventBridge Reaper hoạt động) 🔄
 Vector recall trả case liên quan (kiểm tra tay 20 mẫu + cosine similarity > 0.85)
 Consolidation: case tương tự (>0.92) được merge
 Salience decay job chạy được, archived case không xuất hiện trong default search
 Scorer F1 ≥ 0.95 trên PaySim test set; verdict accuracy agent ≥ 70% trên 200 labeled cases vùng xám 🔄
 Kill region SIN → fleet JKT+BOM tiếp tục, audit trail không đứt 🔄
 IAM least-privilege: mỗi Lambda chỉ có quyền tối thiểu; DB user riêng
 Go binaries build và deploy được lên Lambda; Python retired khỏi production path 🔄
 Video ≤ 3 phút, repo public + Apache 2.0, README có diagram + eval metrics
 Nộp bài trước deadline ≥ 24 giờ


12. Tài liệu tham khảo & Nền tảng lý thuyết
PaperĐóng góp cho HiveMindOrogat & Mansour, "Is Agent Memory a Database?" (Concordia, 2026)GEM framework → salience-driven forgetting, consolidation on insert, retrieval-induced adaptationOmri et al., "Agent Memory: Characterization and System Implications" (Stanford, 2026)Construction vs query cost separation; async construction; top-k retrieval; verdict accuracy là metric chínhZhang et al., "A Survey on the Memory Mechanism of LLM-based Agents" (RUC, 2024)Taxonomy memory sources; cross-trial information; memory management operationsPaySim (Lopez-Rojas et al., 2016)Nguồn giao dịch mô phỏng CC BY 4.0, có ground truth label, phân phối bimodal 🔄