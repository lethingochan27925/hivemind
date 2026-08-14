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
  "Feed data batch by batch and watch the fleet's memory form — measured, not asserted":
    "Nạp dữ liệu theo từng mẻ và xem bộ nhớ của fleet hình thành — đo được, không phải tuyên bố",
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
  Infrastructure: "Hạ tầng",
  "Control Platform": "Nền tảng điều khiển",
  "Agentic memory control plane": "Control plane cho bộ nhớ agent",
  command: "lệnh",
  "Command palette": "Bảng lệnh",
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
  "CockroachDB — the fleet's shared memory. Inspect tables and run read-only queries.":
    "CockroachDB — bộ nhớ dùng chung của fleet. Xem bảng và chạy truy vấn chỉ-đọc.",
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
