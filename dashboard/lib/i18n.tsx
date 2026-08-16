"use client";

import { createContext, useCallback, useContext, useSyncExternalStore, ReactNode } from "react";

/**
 * i18n - English <-> Vietnamese for the control platform.
 *
 * Design: the dictionary is keyed by the ENGLISH source string, so `t("...")`
 * is a drop-in wrapper and anything not translated falls back to English
 * instead of showing a missing key. Translation is applied inside the shared
 * primitives (PageHeader, Panel, EmptyState), so every page picks it up.
 *
 * Deliberately NOT translated: technical identifiers that are also API/SQL
 * values - service names, verdicts (fraud/legit/escalate), table and column
 * names, AWS service names. Translating those would make the console lie about
 * what the system actually stores.
 */

export type Lang = "en" | "vi";

const VI: Record<string, string> = {
  // --- System status banner (Mission Control) ---
  "Cannot reach the control plane": "Không gọi được control plane",
  "Nothing is running yet": "Chưa có gì đang chạy",
  "The fleet is paused and the queue is empty. Start it to watch agents claim cases, recall similar past fraud from memory, and decide.":
    "Fleet đang tạm dừng và hàng đợi trống. Khởi động để xem agent nhận case, gợi nhớ các vụ gian lận tương tự từ bộ nhớ, và ra phán quyết.",
  "Start the fleet and feed 100 cases": "Khởi động fleet và nạp 100 case",
  "Fleet is paused": "Fleet đang tạm dừng",
  "The queue is empty and schedules are disabled. Start it again to process more cases.":
    "Hàng đợi trống và lịch chạy đang tắt. Khởi động lại để xử lý thêm case.",
  "{n} cases are waiting but the fleet is paused":
    "{n} case đang chờ nhưng fleet đang tạm dừng",
  "Schedules are disabled, so nothing will pick these up until the fleet is started.":
    "Lịch chạy đang tắt, sẽ không có gì xử lý số case này cho đến khi fleet được khởi động.",
  "Fleet running · queue drained": "Fleet đang chạy · hàng đợi đã rút cạn",
  "Every queued case has been decided. Feed more to keep the fleet working.":
    "Mọi case trong hàng đợi đã có phán quyết. Nạp thêm để fleet tiếp tục làm việc.",
  "Feed 100 more": "Nạp thêm 100 case",
  "Fleet running": "Fleet đang chạy",
  "{a} under investigation · {p} waiting in the queue":
    "{a} đang được điều tra · {p} đang chờ trong hàng đợi",
  "Fleet started · {n} cases queued · {w} workers invoked":
    "Đã khởi động fleet · {n} case vào hàng đợi · {w} worker được gọi",
  "Fleet started · {w} workers invoked": "Đã khởi động fleet · {w} worker được gọi",
  "{n} cases queued": "{n} case đã vào hàng đợi",
  Retry: "Thử lại",
  Failed: "Thất bại",
  "last data": "dữ liệu cuối",
  connecting: "đang kết nối",

  // --- Memory health ---
  "Memory health": "Sức khỏe bộ nhớ",
  "{p}% of learned knowledge is archived and unreachable":
    "{p}% kiến thức đã học đang bị lưu kho và không truy cập được",
  "{a} memories answer recall queries · {z} sit archived outside the vector index":
    "{a} ký ức đang phục vụ truy vấn gợi nhớ · {z} nằm trong kho, ngoài vector index",
  "Restore all archived": "Khôi phục toàn bộ kho",
  "Restored {n} memories to the vector index": "Đã khôi phục {n} ký ức về vector index",

  // --- Navigation / chrome ---
  Operate: "Vận hành",
  Observe: "Quan sát",
  Platform: "Nền tảng",
  "Mission Control": "Trung tâm điều hành",
  "Review Queue": "Hàng đợi duyệt",
  "Training Lab": "Phòng huấn luyện",
  "Feed data batch by batch and watch the fleet's memory form - measured, not asserted":
    "Nạp dữ liệu theo từng mẻ và xem bộ nhớ của fleet hình thành - đo được, không phải tuyên bố",
  "Training run": "Phiên huấn luyện",
  "each batch: feed cases → drain the queue → measure":
    "mỗi mẻ: nạp case → chờ xử lý hết → đo",
  "Start training run": "Bắt đầu huấn luyện",
  Stop: "Dừng",
  "Export run": "Xuất phiên chạy",
  "Memory formation": "Hình thành bộ nhớ",
  "active memories and raw cases absorbed, per batch":
    "số ký ức hoạt động và số case gốc đã hấp thụ, theo từng mẻ",
  "Decision mix": "Cơ cấu quyết định",
  "auto-resolved vs escalated, per batch": "tự quyết so với chuyển người, theo từng mẻ",
  "Batch results": "Kết quả từng mẻ",
  "every row is a measured step of the run": "mỗi dòng là một bước đo được của phiên chạy",
  "Live agent activity": "Hoạt động agent trực tiếp",
  "the append-only audit trail, newest first": "audit trail chỉ-ghi-thêm, mới nhất trước",
  "Cost per batch": "Chi phí mỗi mẻ",
  "No run yet": "Chưa có phiên chạy nào",
  "No batches recorded": "Chưa ghi nhận mẻ nào",
  "No recent activity": "Chưa có hoạt động gần đây",
  Transactions: "Giao dịch",
  "Fleet & Memory": "Fleet & Bộ nhớ",
  Cost: "Chi phí",
  Pipeline: "Pipeline",
  Database: "Cơ sở dữ liệu",
  Architecture: "Kiến trúc",
  "System Architecture": "Kiến trúc hệ thống",
  Infrastructure: "Hạ tầng",
  "Control Platform": "Nền tảng điều khiển",
  "Agentic memory control plane": "Control plane cho bộ nhớ agent",
  command: "lệnh",
  "Command palette": "Bảng lệnh",

  // --- Stat (KPI cell) labels, used across Mission Control / Cost / Memory / Training ---
  "Fraud blocked": "Đã chặn gian lận",
  Escalated: "Đã chuyển người duyệt",
  Cleared: "Đã xác nhận hợp lệ",
  "Pending review": "Đang chờ duyệt",
  "Tokens today": "Token hôm nay",
  "Estimated spend today": "Chi phí ước tính hôm nay",
  "Active cases": "Case đang hoạt động",
  Archived: "Đã lưu trữ",
  "Avg salience": "Salience trung bình",
  "Verdict accuracy": "Độ chính xác phán quyết",
  "Active memories": "Ký ức đang hoạt động",
  "Raw cases absorbed": "Case gốc đã hợp nhất",
  "In flight": "Đang xử lý",
  Verdicts: "Phán quyết",
  Accuracy: "Độ chính xác",

  // --- Reviews page (only keys not already covered elsewhere in this dict) ---
  awaiting: "đang chờ",
  selected: "đã chọn",
  "Approve all selected": "Duyệt toàn bộ đã chọn",
  "Enter a reviewer name first": "Nhập tên người duyệt trước",
  "Send back to the agent instead of deciding manually": "Trả lại agent thay vì tự quyết định",
  Task: "Task",
  Amount: "Số tiền",
  Risk: "Rủi ro",
  "Agent verdict": "Phán quyết của agent",
  "Pick a row to review its details and decide.": "Chọn một dòng để xem chi tiết và ra quyết định.",
  "Risk score": "Điểm rủi ro",
  "Reviewer name": "Tên người duyệt",
  "Notes (optional) - recorded in the audit trail": "Ghi chú (không bắt buộc) - được lưu vào nhật ký kiểm toán",
  "Return this case to the fleet instead of deciding it yourself": "Trả case này về fleet thay vì tự quyết định",
  "Enter a reviewer name - it is written to the audit trail for compliance.":
    "Nhập tên người duyệt - thông tin này được ghi vào nhật ký kiểm toán.",
  "Go to": "Đi tới",
  Actions: "Hành động",
  Data: "Dữ liệu",
  "Go to a page or run an action…": "Mở một trang hoặc chạy một hành động…",
  "No matches.": "Không có kết quả.",
  "Switch to light theme": "Chuyển sang giao diện sáng",
  "Switch to dark theme": "Chuyển sang giao diện tối",
  "Tiếng Việt": "Tiếng Việt",

  // --- Topbar ---
  "FLEET RUNNING": "FLEET ĐANG CHẠY",
  "FLEET PAUSED": "FLEET ĐÃ DỪNG",
  pending: "chờ xử lý",
  active: "đang chạy",
  "systems OK": "hệ thống bình thường",
  alarm: "cảnh báo",
  alarms: "cảnh báo",
  today: "hôm nay",

  // --- Page titles & descriptions ---
  "Fraud-investigation fleet - live verdicts, memory, and throughput":
    "Fleet điều tra gian lận - phán quyết, bộ nhớ và thông lượng theo thời gian thực",
  "Cases the agent escalated as uncertain - a human analyst makes the final call":
    "Các case agent không chắc chắn - chuyên viên quyết định cuối cùng",
  "Every scored transaction and the agent's full investigation audit trail":
    "Mọi giao dịch đã chấm điểm và toàn bộ dấu vết điều tra của agent",
  "Episodic memory in CockroachDB - what the fleet has learned and how it recalls it":
    "Bộ nhớ tình tiết trong CockroachDB - fleet đã học gì và gợi nhớ ra sao",
  "Bedrock token usage and estimated spend, per agent - the fleet runs on cents":
    "Token Bedrock và chi phí ước tính theo từng agent - fleet chạy bằng vài xu",
  "CockroachDB - the fleet's shared memory. Inspect tables and run read-only queries.":
    "CockroachDB - bộ nhớ dùng chung của fleet. Xem bảng và chạy truy vấn chỉ-đọc.",
  "Live topology assembled from the deployed infrastructure - every node is a real resource":
    "Sơ đồ sinh tự động từ hạ tầng đang chạy - mỗi node là một tài nguyên thật",
  "Every deployed resource - health, configuration, and direct control":
    "Toàn bộ tài nguyên đã triển khai - sức khỏe, cấu hình và điều khiển trực tiếp",

  // --- Panel titles ---
  "Fleet control": "Điều khiển fleet",
  "operate the system live": "vận hành hệ thống trực tiếp",
  "Agent policy": "Chính sách agent",
  "how the fleet decides - editable while it runs": "cách fleet ra quyết định - sửa được khi đang chạy",
  "Real-world impact": "Hiệu quả thực tế",
  "Verdict split": "Phân bố phán quyết",
  "Fleet learning curve": "Đường cong học của fleet",
  "as shared memory grows: recall rises, reasoning latency falls":
    "bộ nhớ chung càng lớn: gợi nhớ tăng, độ trễ suy luận giảm",
  "Memory recall": "Gợi nhớ bộ nhớ",
  "avg similar cases retrieved per investigation, last 24h":
    "số case tương tự trung bình mỗi lần điều tra, 24h qua",
  "Live fleet": "Fleet trực tiếp",
  "agents currently investigating": "các agent đang điều tra",
  "Escalated cases": "Case đã chuyển người duyệt",
  Decision: "Quyết định",
  "Recent transactions": "Giao dịch gần đây",
  "Decision trace": "Giải phẫu quyết định",
  "Memory administration": "Quản trị bộ nhớ",
  "pin, archive, forget - operate what the fleet remembers":
    "ghim, lưu trữ, lãng quên - vận hành thứ fleet ghi nhớ",
  "Learned patterns": "Mẫu đã học",
  "Memory impact": "Tác động của bộ nhớ",
  "Active agents": "Agent đang hoạt động",
  "Budget guardrail": "Hạn mức chi phí",
  "daily Bedrock spend cap - enforced, not advisory":
    "trần chi tiêu Bedrock mỗi ngày - cưỡng chế thật, không chỉ cảnh báo",
  "Cloud infrastructure spend": "Chi phí hạ tầng đám mây",
  "AWS Cost Explorer - month-to-date, every service this platform runs on":
    "AWS Cost Explorer - từ đầu tháng, mọi dịch vụ nền tảng đang dùng",
  "Spend by agent": "Chi phí theo agent",
  "Token breakdown": "Chi tiết token",
  "Release chain": "Chuỗi phát hành",
  "Deployment control": "Điều khiển triển khai",
  "manual rollback - move the live alias to the previous version":
    "quay lui thủ công - đưa alias live về phiên bản trước",
  "Recent runs": "Lần chạy gần đây",
  "Query console": "Bảng truy vấn",
  "Compute fleet": "Cụm tính toán",
  "Lambda functions - alarm state, live config, schedule and controls":
    "Các hàm Lambda - cảnh báo, cấu hình, lịch chạy và điều khiển",
  "Infrastructure inventory": "Danh mục hạ tầng",
  "every resource Terraform tagged Project=hivemind - click a service to expand":
    "mọi tài nguyên Terraform gắn thẻ Project=hivemind - bấm để xem chi tiết",
  "Multi-region": "Đa vùng",
  "CockroachDB database regions - survive a region failure with RPO = 0":
    "Các vùng của CockroachDB - sống sót khi mất một vùng với RPO = 0",
  "Chaos test": "Thử nghiệm hỗn loạn",
  "Incident timeline": "Dòng thời gian sự cố",
  "crash re-queues and checkpoint resumes, last 24h":
    "các lần re-queue do crash và khôi phục từ checkpoint, 24h qua",
  "live configuration and direct actions on this component":
    "cấu hình trực tiếp và thao tác trên thành phần này",

  // --- Buttons / common actions ---
  "Start fleet": "Chạy fleet",
  "Pause fleet": "Dừng fleet",
  "Run dispatch cycle": "Chạy một chu kỳ điều phối",
  "Feed cases": "Nạp case",
  "Apply policy": "Áp dụng chính sách",
  Reset: "Đặt lại",
  Approve: "Duyệt",
  Reject: "Từ chối",
  "Approve all": "Duyệt tất cả",
  "Reject all": "Từ chối tất cả",
  "Back to agent": "Trả về agent",
  "Send back to agent": "Trả case về cho agent",
  "Re-investigate": "Điều tra lại",
  Override: "Ghi đè",
  "Run query": "Chạy truy vấn",
  "Run now": "Chạy ngay",
  "Rollback version": "Quay lui phiên bản",
  "Enable schedule": "Bật lịch chạy",
  "Disable schedule": "Tắt lịch chạy",
  "Simulate agent crash": "Mô phỏng agent sập",
  "Run decay": "Chạy suy giảm",
  "Save guardrail": "Lưu hạn mức",
  "Add region": "Thêm vùng",
  "Set primary region": "Đặt vùng chính",
  "Survive region failure": "Chịu được mất một vùng",
  Logs: "Nhật ký",
  clear: "bỏ chọn",
  open: "mở",

  // --- Empty states ---
  "No verdicts yet": "Chưa có phán quyết nào",
  "Start the fleet to populate this.": "Chạy fleet để có dữ liệu.",
  "No cases awaiting review": "Không có case nào chờ duyệt",
  "The agent auto-resolves clear cases; only genuinely ambiguous ones land here.":
    "Agent tự xử lý case rõ ràng; chỉ case thực sự mơ hồ mới vào đây.",
  "Select a case": "Chọn một case",
  "Select a transaction": "Chọn một giao dịch",
  "No transactions match this filter": "Không có giao dịch nào khớp bộ lọc",
  "No agents currently investigating": "Hiện không có agent nào đang điều tra",
  "Not enough history yet": "Chưa đủ dữ liệu lịch sử",
  "No incidents in the last 24 hours": "Không có sự cố nào trong 24 giờ qua",
  "Query returned no rows": "Truy vấn không trả về dòng nào",
  "Loading inventory…": "Đang tải danh mục…",
  "No memories yet": "Chưa có ký ức nào",

  // --- Misc ---
  "verdict accuracy": "độ chính xác phán quyết",
  "Agent auto-resolved": "Agent tự xử lý",
  "Escalated to human review": "Chuyển người duyệt",
  "Fraud auto-blocked": "Gian lận tự chặn",
  Disconnected: "Mất kết nối",
  "s ago": "giây trước",
  Collapse: "Thu gọn",
  Expand: "Mở rộng",
  Maximize: "Phóng to",
  "Restore size": "Khôi phục kích thước",
  "Drag to resize · double-click to reset": "Kéo để đổi kích thước · nhấp đúp để khôi phục",

  // --- Transactions page ---
  all: "tất cả",
  Type: "Loại",
  Tier: "Mức",
  Verdict: "Phán quyết",
  "No audit steps recorded": "Chưa có bước điều tra nào được ghi nhận",
  "Case handed back to the fleet - it will be re-investigated.":
    "Đã trả case về fleet - sẽ được điều tra lại.",
  "Re-investigate: return this case to the queue": "Điều tra lại: trả case này về hàng đợi",
  "Override the verdict": "Ghi đè phán quyết",
  "your name": "tên của bạn",
  "Verdict overridden to {v} - recorded in the audit trail.":
    "Đã ghi đè phán quyết thành {v} - đã lưu vào nhật ký kiểm toán.",

  // --- Cost page ---
  "Unit price for {model}:": "Đơn giá cho {model}:",
  model: "model",
  in: "vào",
  out: "ra",
  "fetched live from the AWS Pricing API": "lấy trực tiếp từ AWS Pricing API",
  "input live from the AWS Pricing API · output from the published list price (the catalog has no output SKU for this model)":
    "giá vào lấy trực tiếp từ AWS Pricing API · giá ra theo bảng giá niêm yết (catalog không có SKU output cho model này)",
  "published list price (AWS Pricing API has no matching SKU)":
    "giá niêm yết công khai (AWS Pricing API không có SKU khớp)",
  Agent: "Agent",
  "Tokens in": "Token vào",
  "Tokens out": "Token ra",
  "No token usage recorded today": "Chưa ghi nhận token nào hôm nay",
  "No agents billed today": "Chưa có agent nào tính phí hôm nay",
  "estimated USD, Claude Haiku pricing": "USD ước tính, theo giá Claude Haiku",

  // --- Memory page ---
  "consolidated, searchable": "đã hợp nhất, tìm kiếm được",
  "decayed below threshold": "đã suy giảm dưới ngưỡng",
  "recall-weighted importance": "mức quan trọng theo tần suất gợi nhớ",
  "vs ground-truth labels": "so với nhãn thực tế",
  "episodic cases by fraud signature": "case tình tiết theo dấu hiệu gian lận",
  "No patterns recorded yet": "Chưa ghi nhận mẫu nào",
  "Each investigation writes a case; similar cases (>0.92) consolidate into one pattern.":
    "Mỗi lần điều tra ghi một case; các case tương tự (>0.92) hợp nhất thành một mẫu.",
  "does recall make the fleet faster?": "gợi nhớ có giúp fleet nhanh hơn không?",
  "With memory hit": "Khi có gợi nhớ trúng",
  "Cold (no hit)": "Không có gợi nhớ (cold)",
  "Recalling a prior case resolves an investigation": "Gợi nhớ một case trước đó giúp giải quyết điều tra",
  "{p}% faster": "nhanh hơn {p}%",
  "- memory turns investigation cost into an accumulating asset.":
    "- bộ nhớ biến chi phí điều tra thành tài sản tích luỹ.",
  "Latency is comparable with and without recall on the current sample.":
    "Độ trễ tương đương dù có hay không có gợi nhớ, trên mẫu hiện tại.",
  "Not enough data to compare": "Chưa đủ dữ liệu để so sánh",
  "Latency-with-hit vs cold-start is measured once the same pattern recurs.":
    "Độ trễ có-gợi-nhớ so với cold-start chỉ đo được khi cùng một mẫu lặp lại.",
  "workers seen in the last 30 minutes": "worker xuất hiện trong 30 phút qua",
  Status: "Trạng thái",
  "Current task": "Task hiện tại",
  "Last activity": "Hoạt động gần nhất",
  "No agents active in the last 30 minutes": "Không có agent nào hoạt động trong 30 phút qua",
  "Agents are ephemeral Lambdas - they appear here only while a stream is running.":
    "Agent là Lambda tồn tại ngắn hạn - chỉ xuất hiện ở đây khi có luồng dữ liệu đang chạy.",

  // --- Database page ---
  "Verdict breakdown": "Phân tích phán quyết",
  "Audit actions": "Hành động kiểm toán",
  "Task status": "Trạng thái task",
  "read-only - SELECT / SHOW / WITH": "chỉ đọc - SELECT / SHOW / WITH",
  rows: "dòng",
  "(truncated at 200)": "(đã cắt ở 200)",
  "Export these rows as CSV": "Xuất các dòng này ra CSV",
  "Mutating statements are rejected server-side.": "Câu lệnh làm thay đổi dữ liệu bị từ chối phía server.",

  // --- Architecture / Infrastructure: AWS service display labels ---
  "Lambda functions": "Hàm Lambda",
  "CloudWatch alarms": "Cảnh báo CloudWatch",
  "Log groups": "Nhóm log",
  "SSM parameters": "Tham số SSM",
  "ECR repositories": "Repository ECR",
  "S3 buckets": "Bucket S3",
  "EventBridge rules": "Rule EventBridge",
  "DynamoDB tables": "Bảng DynamoDB",
  "SNS topics": "Topic SNS",
  CloudFront: "CloudFront",
  "IAM roles": "Vai trò IAM",
  "every resource Terraform tagged Project=hivemind": "mọi tài nguyên Terraform gắn thẻ Project=hivemind",

  // --- Infrastructure page ---
  resources: "tài nguyên",
  healthy: "khoẻ mạnh",
  Function: "Hàm",
  State: "Trạng thái",
  Alarm: "Cảnh báo",
  Schedule: "Lịch chạy",
  Ver: "Bản",
  Memory: "Bộ nhớ",
  Timeout: "Timeout",
  Control: "Điều khiển",
  "on demand": "theo yêu cầu",
  "No tagged resources found.": "Không tìm thấy tài nguyên nào được gắn thẻ.",
  Close: "Đóng",
  control: "điều khiển",
  "Open in AWS console": "Mở trong AWS console",
  "Backdates a running task's heartbeat to simulate an agent crash. The Heartbeat Reaper detects the stale lease and re-queues the task - a fresh agent resumes from the last checkpoint in CockroachDB.":
    "Chỉnh lùi thời gian heartbeat của một task đang chạy để mô phỏng agent sập. Heartbeat Reaper phát hiện lease đã cũ và re-queue task - một agent mới sẽ phục hồi từ checkpoint gần nhất trong CockroachDB.",
  "Simulating…": "Đang mô phỏng…",
  "Recovery of task": "Phục hồi task",
  "watched live from the audit log": "theo dõi trực tiếp từ nhật ký kiểm toán",
  "Agent killed": "Agent bị hạ",
  "heartbeat backdated": "heartbeat bị chỉnh lùi",
  "Reaper re-queued": "Reaper đã re-queue",
  "+{s}s after kill": "+{s}s sau khi hạ",
  "waiting for 30s sweep…": "đang chờ chu kỳ quét 30s…",
  "Resumed from checkpoint": "Đã phục hồi từ checkpoint",
  "+{s}s - scratchpad read, no work lost": "+{s}s - đã đọc scratchpad, không mất công việc",
  "waiting for a worker to claim…": "đang chờ một worker nhận việc…",
  "Crash absorbed in {s}s. Durable working memory made the failure a non-event.":
    "Sự cố được hấp thụ trong {s}s. Bộ nhớ làm việc bền vững biến sự cố thành chuyện không đáng kể.",
  "Task resumed from checkpoint": "Task đã phục hồi từ checkpoint",
  "Task re-queued after crash": "Task đã được re-queue sau sự cố",
  Version: "Phiên bản",
  "Schedule disabled.": "Đã tắt lịch chạy.",
  "Schedule enabled.": "Đã bật lịch chạy.",
  "Invoked {svc}.": "Đã gọi {svc}.",
  "Roll {svc} back to its previous published version?": "Quay lui {svc} về phiên bản đã phát hành trước đó?",
  "{svc} rolled back.": "{svc} đã được quay lui.",
  regions: "vùng",
  region: "vùng",
  "single-region": "một vùng",
  primary: "chính",
  "Drop region {r}? This changes the database's survivability.":
    "Xoá vùng {r}? Việc này thay đổi khả năng sống sót của database.",
  "Dropped {r}.": "Đã xoá {r}.",
  "Drop {r}": "Xoá {r}",
  "Drop region {r}": "Xoá vùng {r}",
  "Database has no region configuration yet - add the primary region to begin.":
    "Database chưa có cấu hình vùng - thêm vùng chính để bắt đầu.",
  "Primary region set: {r}.": "Đã đặt vùng chính: {r}.",
  "Added region {r}.": "Đã thêm vùng {r}.",
  "Survival goal: REGION failure.": "Mục tiêu sống sót: mất một VÙNG.",
  "Cluster must have the region provisioned first - see": "Cluster phải được cấp sẵn vùng đó trước - xem",

  // --- Training page ---
  running: "đang chạy",
  batches: "mẻ",
  idle: "rảnh",
  "This does": "Việc này",
  not: "không",
  "fine-tune model weights - Claude Haiku on Bedrock is not trainable here. What forms is":
    "tinh chỉnh trọng số model - Claude Haiku trên Bedrock không huấn luyện được ở đây. Thứ được hình thành là",
  "episodic memory": "bộ nhớ tình tiết",
  ": every closed case is summarised, embedded, and consolidated with similar ones. Each batch below is a measured step, and every number traces back to the append-only audit log.":
    ": mỗi case đã đóng được tóm tắt, embedding và hợp nhất với các case tương tự. Mỗi mẻ bên dưới là một bước đo được, và mọi con số đều truy ngược về nhật ký kiểm toán chỉ-ghi-thêm.",
  "Fraud share": "Tỷ lệ gian lận",
  "New cases carry a ground-truth label and land straight in the queue. They are scored synthetically inside the investigate band - this feeds the agent and its memory, it does not exercise the XGBoost scorer.":
    "Case mới mang nhãn ground-truth và vào thẳng hàng đợi. Chúng được chấm điểm tổng hợp trong dải điều tra - nuôi agent và bộ nhớ của nó, không chạy qua scorer XGBoost.",
  "Generate & ingest": "Tạo & nạp dữ liệu",
  "Ingest training data": "Nạp dữ liệu huấn luyện",
  "generate new labelled transactions and queue them for the fleet":
    "tạo giao dịch có nhãn mới và đưa vào hàng đợi cho fleet",
  "Agent pipeline": "Pipeline của agent",
  "what the fleet is doing right now, stage by stage": "fleet đang làm gì ngay lúc này, từng giai đoạn",
  "Cases per batch": "Case mỗi mẻ",
  "Parallel workers": "Worker song song",
  "Written to the agent policy as dispatch_batch - more workers, faster batches":
    "Ghi vào policy của agent dưới dạng dispatch_batch - càng nhiều worker, mẻ càng nhanh",
  "Start from a blank memory (archive everything first)": "Bắt đầu từ bộ nhớ trống (lưu kho toàn bộ trước)",
  "Ingested {n} new transactions ({f} fraud / {l} legit) in {s}s - {q} now queued.":
    "Đã nạp {n} giao dịch mới ({f} gian lận / {l} hợp lệ) trong {s}s - {q} đang chờ trong hàng đợi.",
  "Start from a blank memory? This archives every active episodic memory before the run begins.":
    "Bắt đầu từ bộ nhớ trống? Việc này sẽ lưu kho toàn bộ ký ức tình tiết đang hoạt động trước khi phiên chạy bắt đầu.",
  "archiving every memory - starting from a blank slate": "đang lưu kho toàn bộ ký ức - bắt đầu từ trang trắng",
  "baseline snapshot": "chụp ảnh nền",
  "batch {i}/{n} - feeding {c} cases": "mẻ {i}/{n} - đang nạp {c} case",
  stopped: "đã dừng",
  "run complete": "phiên chạy hoàn tất",
  failed: "thất bại",
  "Run saved ({id}).": "Đã lưu phiên chạy ({id}).",
  "Could not save the run:": "Không thể lưu phiên chạy:",
  "stopping after this batch…": "đang dừng sau mẻ này…",
  "processing - {p} pending, {i} in flight": "đang xử lý - {p} chờ, {i} đang chạy",
  cases: "case",
  "active memories": "ký ức đang hoạt động",
  "avg recalled / case": "TB gợi nhớ / case",
  closed: "đã đóng",
  escalated: "đã chuyển người",
  "auto-resolve %": "% tự xử lý",
  "Start a run: the first batch fills an empty memory, later batches show what recall does as it grows.":
    "Bắt đầu một phiên chạy: mẻ đầu lấp đầy bộ nhớ trống, các mẻ sau cho thấy gợi nhớ hoạt động ra sao khi bộ nhớ lớn lên.",
  "Each batch reports how many cases the fleet closed on its own.":
    "Mỗi mẻ báo cáo số case fleet tự đóng được.",
  "A run writes one row per batch: cases closed, auto-resolve rate, accuracy, memory growth and cost.":
    "Một phiên chạy ghi một dòng mỗi mẻ: case đã đóng, tỷ lệ tự xử lý, độ chính xác, tăng trưởng bộ nhớ và chi phí.",
  Batch: "Mẻ",
  Closed: "Đã đóng",
  Auto: "Tự động",
  "Auto-resolve": "Tự xử lý",
  "Avg recall": "TB gợi nhớ",
  Memories: "Ký ức",
  Fallbacks: "Fallback",
  Time: "Thời gian",
  "Saved runs": "Phiên đã lưu",
  "every completed run is stored in CockroachDB for comparison":
    "mỗi phiên hoàn tất được lưu vào CockroachDB để so sánh",
  "No saved runs yet": "Chưa có phiên nào được lưu",
  "A run is saved automatically when it finishes.": "Phiên chạy tự động được lưu khi hoàn tất.",
  When: "Khi nào",
  Run: "Phiên chạy",
  Cases: "Case",
  "Recall first → last": "Gợi nhớ đầu → cuối",
  "Start a run, or feed cases from Mission Control.": "Bắt đầu một phiên chạy, hoặc nạp case từ Trung tâm điều hành.",
  events: "sự kiện",
  recalled: "đã gợi nhớ",
  fallback: "fallback",
  Claimed: "Đã nhận",
  "Customer context": "Bối cảnh khách hàng",
  Reasoning: "Suy luận",
  last: "gần nhất",
  "audited events": "sự kiện đã kiểm toán",
  "avg recalled per case:": "TB gợi nhớ mỗi case:",
  "avg reasoning:": "TB suy luận:",
  "millicents spent vs memories recalled": "millicent chi tiêu so với ký ức được gợi nhớ",
};

interface I18nValue {
  lang: Lang;
  setLang: (l: Lang) => void;
  t: (s: string) => string;
}

const I18nContext = createContext<I18nValue>({
  lang: "en",
  setLang: () => {},
  t: (s) => s,
});

// localStorage la mot external store: doc no bang useSyncExternalStore thay vi
// setState trong effect. Tranh cascading render, va tranh lech hydration vi
// server luon tra ve snapshot "en".
function subscribeLang(onChange: () => void) {
  window.addEventListener("storage", onChange);
  window.addEventListener("hm-lang-change", onChange);
  return () => {
    window.removeEventListener("storage", onChange);
    window.removeEventListener("hm-lang-change", onChange);
  };
}

function readLang(): Lang {
  try {
    const saved = localStorage.getItem("hm-lang");
    if (saved === "vi" || saved === "en") return saved;
  } catch {}
  return "en";
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const lang = useSyncExternalStore(subscribeLang, readLang, () => "en" as Lang);

  const setLang = useCallback((l: Lang) => {
    try {
      localStorage.setItem("hm-lang", l);
    } catch {}
    document.documentElement.lang = l;
    window.dispatchEvent(new Event("hm-lang-change"));
  }, []);

  const t = useCallback((s: string) => (lang === "vi" ? VI[s] ?? s : s), [lang]);

  return <I18nContext.Provider value={{ lang, setLang, t }}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  return useContext(I18nContext);
}

/** useT - shorthand for components that only need the translate function. */
export function useT() {
  return useContext(I18nContext).t;
}
