"use client";

import { useEffect, useState } from "react";
import { api, Transaction, AuditStep } from "@/lib/api";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";

const money = (n: number) => n.toLocaleString(undefined, { maximumFractionDigits: 0 });

const tierTone = (t: string) => (t === "high" ? "red" : t === "medium" ? "yellow" : "green");
const verdictTone = (v?: string) =>
  v === "fraud" ? "red" : v === "escalate" ? "yellow" : v === "legit" ? "green" : "default";

const actionTone = (a: string) => {
  if (a.startsWith("verdict_fraud") || a === "task_failed") return "border-red/50";
  if (a.startsWith("verdict_legit") || a === "auto_approve") return "border-green/50";
  if (a.startsWith("verdict_escalate")) return "border-yellow/50";
  if (a === "memory_recall") return "border-blue/50";
  if (a === "bedrock_reasoning") return "border-purple/50";
  return "border-border-strong";
};

const FILTERS = ["", "low", "medium", "high"];

export default function TransactionsPage() {
  const [txns, setTxns] = useState<Transaction[]>([]);
  const [selected, setSelected] = useState<Transaction | null>(null);
  const [audit, setAudit] = useState<AuditStep[]>([]);
  const [filter, setFilter] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  // Fetch on filter change. setLoading(true) lives in the filter button handler,
  // not here - calling setState synchronously in an effect body triggers
  // cascading renders (react-hooks/set-state-in-effect).
  useEffect(() => {
    let alive = true;
    api
      .getTransactions(filter || undefined)
      .then((t) => {
        if (!alive) return;
        setTxns(t ?? []);
        setError(null);
      })
      .catch((e) => {
        if (alive) setError(e.message);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [filter]);

  const select = (tx: Transaction) => {
    setSelected(tx);
    setAudit([]);
    api
      .getTransactionAudit(tx.id)
      .then((a) => setAudit(a ?? []))
      .catch(() => setAudit([]));
  };

  return (
    <div>
      <PageHeader
        title="Transactions"
        description="Every scored transaction and the agent's full investigation audit trail"
        error={error}
        actions={
          <div className="flex gap-1">
            {FILTERS.map((t) => (
              <button
                key={t}
                onClick={() => {
                  setFilter(t);
                  setLoading(true);
                }}
                className={`px-2 py-1 text-[13px] rounded border capitalize transition-colors ${
                  filter === t
                    ? "border-blue text-blue bg-blue/10"
                    : "border-border text-text-secondary hover:text-text-primary"
                }`}
              >
                {t || "all"}
              </button>
            ))}
          </div>
        }
      />

      <div className="p-6 grid grid-cols-1 lg:grid-cols-3 gap-5 hm-enter">
        <Panel title="Recent transactions" className="lg:col-span-2" bodyClassName="p-0">
          {loading ? (
            <div className="p-3 space-y-2">
              {Array.from({ length: 8 }).map((_, i) => (
                <div key={i} className="h-6 hm-skeleton" />
              ))}
            </div>
          ) : txns.length === 0 ? (
            <EmptyState message="No transactions match this filter" />
          ) : (
            <div className="overflow-x-auto max-h-[70vh]">
              <table className="w-full text-[14px]">
                <thead className="sticky top-0 bg-bg-panel">
                  <tr className="text-left text-text-tertiary border-b border-border">
                    <th className="py-2.5 px-4 font-normal">ID</th>
                    <th className="py-2.5 px-4 font-normal">Type</th>
                    <th className="py-2.5 px-4 font-normal text-right">Amount</th>
                    <th className="py-2.5 px-4 font-normal text-right">Risk</th>
                    <th className="py-2.5 px-4 font-normal">Tier</th>
                    <th className="py-2.5 px-4 font-normal">Verdict</th>
                  </tr>
                </thead>
                <tbody>
                  {txns.map((tx) => (
                    <tr
                      key={tx.id}
                      onClick={() => select(tx)}
                      tabIndex={0}
                      onKeyDown={(e) => e.key === "Enter" && select(tx)}
                      className={`cursor-pointer border-b border-border/40 outline-none ${
                        selected?.id === tx.id
                          ? "bg-blue/10"
                          : "hover:bg-bg-panel-hover/60 focus:bg-bg-panel-hover/60"
                      }`}
                    >
                      <td className="py-2.5 px-4 tnum text-text-secondary">{tx.id.slice(0, 8)}</td>
                      <td className="py-2.5 px-4 text-text-secondary">{tx.type}</td>
                      <td className="py-2.5 px-4 tnum text-right text-text-primary">
                        {money(tx.amount)}
                      </td>
                      <td className="py-2.5 px-4 tnum text-right text-text-secondary">
                        {tx.risk_score.toFixed(3)}
                      </td>
                      <td className="py-2.5 px-4">
                        <Badge variant={tierTone(tx.risk_tier)}>{tx.risk_tier}</Badge>
                      </td>
                      <td className="py-2.5 px-4">
                        {tx.verdict ? (
                          <Badge variant={verdictTone(tx.verdict)}>{tx.verdict}</Badge>
                        ) : (
                          <span className="text-text-tertiary">-</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>

        <Panel title="Audit trail" subtitle={selected ? selected.id.slice(0, 8) : undefined}>
          {!selected ? (
            <EmptyState
              message="Select a transaction"
              hint="Every agent action - memory recall, MCP query, reasoning, verdict - is recorded."
            />
          ) : audit.length === 0 ? (
            <EmptyState message="No audit steps recorded" />
          ) : (
            <ol className="relative space-y-3 pl-1">
              {audit.map((step, i) => (
                <li key={i} className={`border-l-2 pl-3 pb-1 ${actionTone(step.action)}`}>
                  <div className="text-[14px] text-text-primary font-medium">
                    {step.action.replace(/_/g, " ")}
                  </div>
                  {step.reasoning && (
                    <div className="text-[13px] text-text-secondary mt-0.5 leading-relaxed">
                      {step.reasoning}
                    </div>
                  )}
                  <div className="text-[12px] text-text-tertiary mt-1 flex flex-wrap gap-x-3 gap-y-0.5 tnum">
                    {step.tokens_in != null && (
                      <span>{(step.tokens_in + (step.tokens_out ?? 0)).toLocaleString()} tok</span>
                    )}
                    {step.latency_ms != null && <span>{step.latency_ms} ms</span>}
                    <span>{new Date(step.created_at).toLocaleTimeString()}</span>
                  </div>
                </li>
              ))}
            </ol>
          )}
        </Panel>
      </div>
    </div>
  );
}
