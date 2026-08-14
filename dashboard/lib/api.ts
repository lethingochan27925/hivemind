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
  input_per_1k?: number;
  output_per_1k?: number;
  price_source?: string;
  priced_model?: string;
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
export interface ServiceCost {
  service: string;
  usd: number;
}
export interface CloudCost {
  available: boolean;
  note?: string;
  period_start: string;
  period_end: string;
  total_usd: number;
  by_service: ServiceCost[] | null;
  refreshed_at?: string;
  cache_minutes: number;
}
export interface TrainingSnapshot {
  at: string;
  memories_active: number;
  memories_archived: number;
  raw_cases_absorbed: number;
  recall_events: number;
  recall_hits_sum: number;
  verdict_fraud: number;
  verdict_legit: number;
  verdict_escalate: number;
  correct_verdicts: number;
  graded_verdicts: number;
  tokens_in: number;
  tokens_out: number;
  model_decisions: number;
  fallback_verdicts: number;
  pending: number;
  investigating: number;
}
export interface TrainingLogEntry {
  at: string;
  action: string;
  agent_id: string;
  task_id: string;
  reasoning?: string;
  memory_hits?: number;
  latency_ms?: number;
  bedrock_model?: string;
}
export interface IngestResult {
  status: string;
  inserted: number;
  fraud: number;
  legit: number;
  seed: number;
  took_ms: number;
  queued_now: number;
}
export interface TrainingRun {
  id: string;
  label: string;
  config: unknown;
  batches: unknown;
  summary: unknown;
  created_at: string;
}
export interface Policy {
  risk_low: number;
  risk_high: number;
  dispatch_batch: number;
  fallback_action: "escalate" | "requeue";
  daily_budget_usd: number;
  auto_pause_on_budget: boolean;
  updated_by?: string;
  updated_at?: string;
}
export interface BudgetStatus {
  spend_today: number;
  daily_budget_usd: number;
  used_pct: number;
  exceeded: boolean;
  auto_pause_on_budget: boolean;
  paused_by_rule: boolean;
}
export interface MemoryRow {
  id: string;
  summary: string;
  verdict: string;
  pattern_type?: string;
  salience: number;
  recall_count: number;
  merge_count: number;
  archived: boolean;
}
export interface SearchTxn {
  id: string;
  type: string;
  amount: number;
  risk_tier: string;
  verdict?: string;
}
export interface SearchTask {
  id: string;
  transaction_id: string;
  status: string;
  verdict?: string;
}
export interface SearchMemory {
  id: string;
  summary: string;
  verdict: string;
  pattern_type?: string;
  recall_count: number;
}
export interface SearchAgent {
  agent_id: string;
  actions: number;
}
export interface SearchResults {
  query: string;
  transactions: SearchTxn[];
  tasks: SearchTask[];
  memories: SearchMemory[];
  agents: SearchAgent[];
}
export interface RegionInfo {
  region: string;
  primary: boolean;
}
export interface RegionsStatus {
  database: string;
  multi_region: boolean;
  survival_goal: string;
  regions: RegionInfo[];
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
  search: (q: string) => fetchJSON<SearchResults>(`/control/search?q=${encodeURIComponent(q)}`),
  scheduleOne: (service: string, action: "enable" | "disable") =>
    fetchJSON<{ status: string }>("/control/schedule", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ service, action }),
    }),
  invokeService: (service: string) =>
    fetchJSON<{ status: string; service: string }>("/control/invoke", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ service }),
    }),
  // Policy & budget
  getPolicy: () => fetchJSON<Policy>("/control/policy"),
  setPolicy: (p: Policy) =>
    fetchJSON<Policy>("/control/policy", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify(p),
    }),
  getBudget: () => fetchJSON<BudgetStatus>("/control/budget"),
  getCloudCost: () => fetchJSON<CloudCost>("/cost/infrastructure"),

  // Episodic memory administration
  getMemories: (opts?: { limit?: number; archived?: boolean }) =>
    fetchJSON<MemoryRow[]>(
      `/control/memory?limit=${opts?.limit ?? 50}&archived=${opts?.archived ? "true" : "false"}`
    ),
  memoryAction: (action: "pin" | "unpin" | "archive" | "unarchive" | "delete", id: string) =>
    fetchJSON<{ status: string }>("/control/memory", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ action, id }),
    }),
  ingest: (count: number, fraudRate: number, seed = 0) =>
    fetchJSON<IngestResult>("/control/ingest", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ count, fraud_rate: fraudRate, seed }),
    }),
  listTrainingRuns: () => fetchJSON<TrainingRun[]>("/control/training/runs"),
  saveTrainingRun: (label: string, config: unknown, batches: unknown, summary: unknown) =>
    fetchJSON<{ status: string; id: string }>("/control/training/runs", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ label, config, batches, summary }),
    }),
  trainingMetrics: () => fetchJSON<TrainingSnapshot>("/control/training/metrics"),
  trainingLog: (limit = 60) => fetchJSON<TrainingLogEntry[]>(`/control/training/log?limit=${limit}`),
  memoryJob: (job: "decay" | "archive_below" | "archive_all" | "unarchive_all", threshold?: number) =>
    fetchJSON<{ status: string; archived?: number }>("/control/memory/job", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ job, threshold: threshold ?? 0 }),
    }),

  // Task-level control
  requeueTask: (task_id: string) =>
    fetchJSON<{ status: string }>("/control/task", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ action: "requeue", task_id }),
    }),
  overrideVerdict: (task_id: string, verdict: string, reviewer_id: string, notes?: string) =>
    fetchJSON<{ status: string }>("/control/task", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ action: "override", task_id, verdict, reviewer_id, notes: notes ?? "" }),
    }),
  bulkReview: (task_ids: string[], decision: "approved" | "rejected", reviewer_id: string, notes?: string) =>
    fetchJSON<{ decided: number; failed: number }>("/reviews/bulk", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ task_ids, decision, reviewer_id, notes: notes ?? "" }),
    }),

  rollbackService: (service: string) =>
    fetchJSON<{ status: string; from_version: number; to_version: number }>("/control/rollback", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ service }),
    }),

  getRegions: () => fetchJSON<RegionsStatus>("/control/regions"),
  alterRegion: (action: "add" | "drop" | "set_primary" | "survive_region" | "survive_zone", region?: string) =>
    fetchJSON<{ status: string; action: string; region: string }>("/control/regions", {
      method: "POST",
      headers: controlHeaders(),
      body: JSON.stringify({ action, region: region ?? "" }),
    }),
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
