const API_BASE_URL = process.env.NEXT_PUBLIC_DASHBOARD_API_URL || "http://localhost:8090";

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

export interface OverviewData {
  verdicts_today: VerdictCount[] | null;
  pending_reviews: number;
  memory_hits_trend: MemoryHitPoint[] | null;
  live_tasks: LiveTask[] | null;
}

export interface PendingReview {
  task_id: string;
  transaction_id: string;
  verdict: string;
  confidence: number;
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
  reasoning?: string;
  tokens_in?: number;
  tokens_out?: number;
  latency_ms?: number;
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

async function fetchJSON<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    cache: "no-store",
  });
  if (!res.ok) {
    throw new Error(`API error ${res.status}: ${path}`);
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
};
