"use client";

import { useEffect, useState } from "react";
import { api, Transaction, AuditStep } from "@/lib/api";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { DecisionTrace } from "@/components/control/decision-trace";
import { RotateCcw, Gavel, Loader2 } from "lucide-react";

const money = (n: number) => n.toLocaleString(undefined, { maximumFractionDigits: 0 });

const tierTone = (t: string) => (t === "high" ? "red" : t === "medium" ? "yellow" : "green");
const verdictTone = (v?: string) =>
  v === "fraud" ? "red" : v === "escalate" ? "yellow" : v === "legit" ? "green" : "default";

const FILTERS = ["", "low", "medium", "high"];

export default function TransactionsPage() {
  const [txns, setTxns] = useState<Transaction[]>([]);
  const [selected, setSelected] = useState<Transaction | null>(null);
  const [audit, setAudit] = useState<AuditStep[]>([]);
  const [filter, setFilter] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [caseMsg, setCaseMsg] = useState<string | null>(null);

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

  // Cap nhat lac quan: bang phai doi ngay khi thao tac thanh cong, khong cho
  // vong poll tiep theo (truoc day row van hien verdict cu vai giay).
  const applyLocalVerdict = (id: string, verdict?: string) => {
    setTxns((prev) => prev.map((t) => (t.id === id ? { ...t, verdict } : t)));
    setSelected((prev) => (prev && prev.id === id ? { ...prev, verdict } : prev));
    api
      .getTransactions(filter || undefined)
      .then((t) => setTxns(t ?? []))
      .catch(() => {});
  };

  const select = (tx: Transaction) => {
    setSelected(tx);
    setAudit([]);
    api
      .getTransactionAudit(tx.id)
      .then((a) => setAudit(a ?? []))
      .catch(() => setAudit([]));
  };

  // Deep-link tu global search: /transactions?id=<uuid> tu chon dung row do.
  useEffect(() => {
    if (selected || txns.length === 0) return;
    const id = new URLSearchParams(window.location.search).get("id");
    if (!id) return;
    const tx = txns.find((t) => t.id === id || t.id.startsWith(id));
    if (tx) select(tx);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [txns]);

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

        <Panel
          title="Decision trace"
          subtitle={selected ? `case ${selected.id.slice(0, 8)} - full anatomy of the verdict` : undefined}
          actions={
            selected ? (
              <CaseActions
                txnId={selected.id}
                onDone={(m) => setCaseMsg(m)}
                onApplied={applyLocalVerdict}
              />
            ) : null
          }
        >
          {caseMsg && (
            <div className="mb-3 text-[13px] text-text-secondary border-b border-border pb-2">{caseMsg}</div>
          )}
          {!selected ? (
            <EmptyState
              message="Select a transaction"
              hint="See the anatomy of a verdict: recalled memories with real vector similarity, model cost, and any crash/resume - all from the append-only audit log."
            />
          ) : audit.length === 0 ? (
            <EmptyState message="No audit steps recorded" />
          ) : (
            <div className="max-h-[70vh] overflow-y-auto pr-1">
              <DecisionTrace steps={audit} />
            </div>
          )}
        </Panel>
      </div>
    </div>
  );
}

/**
 * CaseActions - act on the case you are looking at: hand it back to the fleet
 * for a fresh investigation, or record an operator override (written to the
 * audit trail as human_reviewed, with the reviewer's name).
 */
function CaseActions({
  txnId,
  onDone,
  onApplied,
}: {
  txnId: string;
  onDone: (msg: string) => void;
  onApplied: (id: string, verdict?: string) => void;
}) {
  const [busy, setBusy] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [who, setWho] = useState("");

  const run = async (key: string, fn: () => Promise<unknown>, done: string) => {
    setBusy(key);
    try {
      await fn();
      onDone(done);
      setOpen(false);
    } catch (e) {
      onDone((e as Error).message);
    } finally {
      setBusy(null);
    }
  };

  const btn =
    "flex items-center gap-1.5 px-2.5 py-1 rounded-md border text-[12px] font-semibold disabled:opacity-40 transition-colors";

  return (
    <div className="flex flex-col items-end gap-2">
      <div className="flex items-center gap-1.5">
        <button
          disabled={busy !== null}
          onClick={() =>
            run("requeue", async () => {
              await api.requeueTask(txnId);
              onApplied(txnId, undefined);
            }, "Case handed back to the fleet - it will be re-investigated.")
          }
          className={`${btn} border-blue/30 bg-blue/10 text-blue hover:bg-blue/20`}
          title="Re-investigate: return this case to the queue"
        >
          {busy === "requeue" ? <Loader2 size={12} className="animate-spin" /> : <RotateCcw size={12} />}
          Re-investigate
        </button>
        <button
          onClick={() => setOpen(!open)}
          className={`${btn} border-border text-text-secondary hover:text-text-primary`}
          title="Override the verdict"
        >
          <Gavel size={12} />
          Override
        </button>
      </div>

      {open && (
        <div className="flex items-center gap-1.5">
          <input
            value={who}
            onChange={(e) => setWho(e.target.value)}
            placeholder="your name"
            className="w-28 bg-bg-inset border border-border rounded-md px-2 py-1 text-[12px] text-text-primary outline-none focus:border-blue"
          />
          {(["fraud", "legit"] as const).map((v) => (
            <button
              key={v}
              disabled={busy !== null || !who.trim()}
              onClick={() =>
                run(v, async () => {
                  await api.overrideVerdict(txnId, v, who.trim());
                  onApplied(txnId, v);
                }, `Verdict overridden to ${v} - recorded in the audit trail.`)
              }
              className={`${btn} ${
                v === "fraud"
                  ? "border-red/30 bg-red/10 text-red hover:bg-red/20"
                  : "border-green/30 bg-green/10 text-green hover:bg-green/20"
              }`}
            >
              {busy === v ? <Loader2 size={12} className="animate-spin" /> : null}
              {v}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
