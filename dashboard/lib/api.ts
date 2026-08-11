const DEFAULT_API_BASE_URL = "http://localhost:8090";

/**
 * Lambda Function URL luon co dau "/" o cuoi. Ghep thang voi path se ra
 * "//overview": Go ServeMux tra 301 canonical-path, ma response 301 do khong
 * di qua CORS middleware nen browser chan. Chuan hoa base URL tai dung mot cho.
 */
function normalizeBaseURL(raw: string | undefined): string {
  const value = raw?.trim();
  return (value || DEFAULT_API_BASE_URL).replace(/\/+$/, "");
}

const API_BASE_URL = normalizeBaseURL(process.env.NEXT_PUBLIC_DASHBOARD_API_URL);

export interface VerdictCount {
  verdict: string;
  count: number;
}

export interface MemoryHitPoint {
  hour_bucket: string;
  avg_memory_hits: number;
}

export interface LiveTask {
  task_id: string;
  claimed_by: string;
  claimed_at: string;
}

export interface LearningPoint {
  hour_bucket: string;
  avg_memory_hits: number;
  avg_latency_ms: number;
  verdicts: number;
}

export interface OverviewData {
  verdicts_today: VerdictCount[] | null;
  pending_reviews: number;
  memory_hits_trend: MemoryHitPoint[] | null;
  live_tasks: LiveTask[] | null;
  learning_curve: LearningPoint[] | null;
  verdict_accuracy_pct?: number;
}

export interface PendingReview {
  task_id: string;
  transaction_id: string;
  verdict: string;
  confidence: number;
  amount: number;
  risk_score: number;
  txn_type: string;
}

export interface PatternStat {
  pattern_type: string;
  count: number;
}

export interface MemoryStats {
  active_cases: number;
  archived_cases: number;
  avg_salience: number;
  top_patterns: PatternStat[] | null;
}

export interface ActiveAgent {
  agent_id: string;
  status: "active" | "idle";
  current_task?: string;
  last_activity: string;
}

export interface ImpactStats {
  verdict_accuracy_pct?: number;
  avg_latency_with_hit_ms?: number;
  avg_latency_no_hit_ms?: number;
}

export interface MemoryData {
  stats: MemoryStats;
  active_agents: ActiveAgent[] | null;
  impact: ImpactStats;
}

export interface Transaction {
  id: string;
  type: string;
  amount: number;
  risk_score: number;
  risk_tier: string;
  verdict?: string;
  arrived_at: string;
}

export interface AuditStep {
  action: string;
  agent_id?: string;
  reasoning?: string;
  memory_hits?: number;
  similarity_scores?: number[];
  tokens_in?: number;
  tokens_out?: number;
  latency_ms?: number;
  bedrock_model?: string;
  created_at: string;
}

export interface ServiceHealth {
  service: string;
  alarm_state: string;
}

export interface IncidentEvent {
  timestamp: string;
  service: string;
  description: string;
  task_id?: string;
  action?: string;
}

export interface InfrastructureData {
  services: ServiceHealth[];
  incidents: IncidentEvent[] | null;
}

export interface AgentCost {
  agent_id: string;
  total_tokens_in: number;
  total_tokens_out: number;
  estimated_cost_usd: number;
}

export interface CostData {
  total_tokens_today: number;
  estimated_cost_usd_today: number;
  by_agent: AgentCost[] | null;
}

// --- Control plane types -----------------------------------------------------
export interface ScheduleState {
  service: string;
  state: string;
}
export interface FleetStatus {
  running: boolean;
  schedules: ScheduleState[];
  tasks: Record<string, number>;
}
export interface DispatchResult {
  tasks_created: number;
  pending_tasks: number;
  workers_invoked: number;
}
export interface LambdaInfo {
  service: string;
  state: string;
  version: string;
  memory_mb: number;
  timeout_sec: number;
  last_modified: string;
  runtime: string;
}
export interface ResourceInfo {
  service: string;
  name: string;
  arn: string;
}
export interface TableStat {
  table: string;
  rows: number;
}
export interface DbStats {
  database: string;
  tables: TableStat[];
  total_rows: number;
}
export interface QueryResult {
  columns: string[];
  rows: unknown[][];
  row_count: number;
  truncated: boolean;
}

const CONTROL_TOKEN = process.env.NEXT_PUBLIC_CONTROL_TOKEN;
function controlHeaders(): Record<string, string> {
  const h: Record<string, string> = { "Content-Type": "application/json" };
  if (CONTROL_TOKEN) h["X-Control-Token"] = CONTROL_TOKEN;
  return h;
}

async function fetchJSON<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    cache: "no-store",
  });
  if (!res.ok) {
    const detail = await res.text().catch(() => "");
    throw new Error(`API ${res.status} ${path}${detail ? ` \u2014 ${detail.trim()}` : ""}`);
  }
  return res.json();
}

export const api = {
  getOverview: () => fetchJSON<OverviewData>("/overview"),
  getReviews: () => fetchJSON<PendingReview[]>("/reviews"),
  decideReview: (body: {
    task_id: string;
    reviewer_id: string;
    decision: "approved" | "rejected";
    notes?: string;
  }) =>
    fetchJSON<{ status: string }>("/reviews/decide", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }),
  getMemory: () => fetchJSON<MemoryData>("/memory"),
  getTransactions: (riskTier?: string) =>
    fetchJSON<Transaction[]>(
      `/transactions${riskTier ? `?risk_tier=${riskTier}` : ""}`
    ),
  getTransactionAudit: (id: string) =>
    fetchJSON<AuditStep[]>(`/transactions/${id}/audit`),
  getInfrastructure: () => fetchJSON<InfrastructureData>("/infrastructure"),
  getCost: () => fetchJSON<CostData>("/cost"),
  simulateCrash: () =>
    fetchJSON<{ status: string; task_id: string; message: string }>(
      "/infrastructure/simulate-crash",
      { method: "POST" }
    ),
  getLambdas: () => fetchJSON<LambdaInfo[]>("/control/lambdas"),
  getResources: () => fetchJSON<ResourceInfo[]>("/control/resources"),
  getDbStats: () => fetchJSON<DbStats>("/control/db"),
  runQuery: (sql: string) =>
    fetchJSON<QueryResult>("/control/query", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ sql }),
    }),

  getFleetStatus: () => fetchJSON<FleetStatus>("/control/fleet"),
  setFleetState: (action: "start" | "pause") =>
    fetchJSON<{ status: string; running: boolean }>("/control/fleet", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ action }),
    }),
  runDispatch: () =>
    fetchJSON<{ status: string; dispatcher_result: DispatchResult }>("/control/dispatch", {
      method: "POST",
      headers: controlHeaders(),
    }),
  feedStream: (count: number) =>
    fetchJSON<{ status: string; requeued: number }>("/control/feed", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ count }),
    }),
};
