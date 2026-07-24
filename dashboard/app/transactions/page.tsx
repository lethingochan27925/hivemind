"use client";

import { useEffect, useState } from "react";
import { api, Transaction, AuditStep } from "@/lib/api";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";

export default function TransactionsPage() {
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [selected, setSelected] = useState<Transaction | null>(null);
  const [audit, setAudit] = useState<AuditStep[]>([]);
  const [filter, setFilter] = useState<string>("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .getTransactions(filter || undefined)
      .then(setTransactions)
      .catch((e) => setError(e.message));
  }, [filter]);

  const handleSelect = (tx: Transaction) => {
    setSelected(tx);
    api.getTransactionAudit(tx.id).then(setAudit).catch(() => setAudit([]));
  };

  const tierColor = (tier: string) =>
    tier === "high" ? "red" : tier === "medium" ? "yellow" : "green";

  const verdictColor = (v?: string) =>
    v === "fraud" ? "red" : v === "escalate" ? "yellow" : v === "legit" ? "green" : "default";

  return (
    <div>
      <div className="h-12 flex items-center justify-between px-4 border-b border-border">
        <h1 className="text-[16px] font-semibold text-text-primary tracking-tight">
          Transactions
        </h1>
        <div className="flex gap-1.5">
          {["", "low", "medium", "high"].map((t) => (
            <button
              key={t}
              onClick={() => setFilter(t)}
              className={`px-2 py-1 text-xs rounded-sm border ${
                filter === t
                  ? "border-blue text-blue bg-blue/10"
                  : "border-border text-text-secondary"
              }`}
            >
              {t || "all"}
            </button>
          ))}
        </div>
      </div>

      <div className="p-4 grid grid-cols-3 gap-3">
        <Panel title="Recent transactions" className="col-span-2">
          {error && <div className="text-red text-xs mb-2">{error}</div>}
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left text-text-tertiary border-b border-border">
                <th className="pb-1.5 font-normal">ID</th>
                <th className="pb-1.5 font-normal">Type</th>
                <th className="pb-1.5 font-normal">Amount</th>
                <th className="pb-1.5 font-normal">Risk</th>
                <th className="pb-1.5 font-normal">Verdict</th>
              </tr>
            </thead>
            <tbody>
              {transactions.map((tx) => (
                <tr
                  key={tx.id}
                  onClick={() => handleSelect(tx)}
                  className={`border-b border-border/50 cursor-pointer ${
                    selected?.id === tx.id
                      ? "bg-bg-panel-hover"
                      : "hover:bg-bg-panel-hover/50"
                  }`}
                >
                  <td className="py-1.5 text-text-secondary">{tx.id.slice(0, 8)}</td>
                  <td className="py-1.5 text-text-secondary">{tx.type}</td>
                  <td className="py-1.5 text-text-primary">
                    {tx.amount.toLocaleString()}
                  </td>
                  <td className="py-1.5">
                    <Badge variant={tierColor(tx.risk_tier)}>{tx.risk_tier}</Badge>
                  </td>
                  <td className="py-1.5">
                    {tx.verdict ? (
                      <Badge variant={verdictColor(tx.verdict)}>{tx.verdict}</Badge>
                    ) : (
                      <span className="text-text-tertiary">pending</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Panel>

        <Panel title="Audit trail">
          {!selected ? (
            <div className="text-text-tertiary text-xs py-4 text-center">
              Select a transaction
            </div>
          ) : audit.length === 0 ? (
            <div className="text-text-tertiary text-xs py-4 text-center">
              No audit steps recorded
            </div>
          ) : (
            <div className="space-y-2">
              {audit.map((step, i) => (
                <div key={i} className="border-l-2 border-blue/40 pl-2 py-1">
                  <div className="text-xs text-text-primary">{step.action}</div>
                  {step.reasoning && (
                    <div className="text-[11px] text-text-secondary mt-0.5">
                      {step.reasoning}
                    </div>
                  )}
                  <div className="text-[10px] text-text-tertiary mt-0.5 flex gap-2">
                    {step.tokens_in != null && (
                      <span>{step.tokens_in + (step.tokens_out ?? 0)} tok</span>
                    )}
                    {step.latency_ms != null && <span>{step.latency_ms}ms</span>}
                    <span>{new Date(step.created_at).toLocaleTimeString()}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </Panel>
      </div>
    </div>
  );
}
