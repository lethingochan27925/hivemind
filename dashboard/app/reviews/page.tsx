"use client";

import { useState } from "react";
import { api, PendingReview } from "@/lib/api";
import { useLive } from "@/lib/use-live";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { Check, X, User } from "lucide-react";

const money = (n: number) =>
  n.toLocaleString(undefined, { maximumFractionDigits: 0 });

export default function ReviewsPage() {
  const { data, error, lastUpdated } = useLive(api.getReviews, 8000);
  const reviews = data ?? [];

  const [selected, setSelected] = useState<PendingReview | null>(null);
  const [reviewerId, setReviewerId] = useState("");
  const [notes, setNotes] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const active = selected && reviews.find((r) => r.task_id === selected.task_id) ? selected : null;

  const decide = async (decision: "approved" | "rejected") => {
    if (!active || !reviewerId.trim()) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      await api.decideReview({
        task_id: active.task_id,
        reviewer_id: reviewerId.trim(),
        decision,
        notes: notes.trim(),
      });
      setSelected(null);
      setNotes("");
    } catch (e) {
      setSubmitError((e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  const verdictTone = (v: string) =>
    v === "fraud" ? "red" : v === "escalate" ? "yellow" : "green";

  return (
    <div>
      <PageHeader
        title="Review Queue"
        description="Cases the agent escalated as uncertain - a human analyst makes the final call"
        lastUpdated={lastUpdated}
        error={error}
        actions={
          <Badge variant={reviews.length > 0 ? "yellow" : "green"} dot>
            {reviews.length} awaiting
          </Badge>
        }
      />

      <div className="p-6 grid grid-cols-1 lg:grid-cols-3 gap-5 hm-enter">
        <Panel title="Escalated cases" className="lg:col-span-2" bodyClassName="p-0">
          {reviews.length === 0 ? (
            <EmptyState
              message="No cases awaiting review"
              hint="The agent auto-resolves clear cases; only genuinely ambiguous ones land here."
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-[14px]">
                <thead>
                  <tr className="text-left text-text-tertiary border-b border-border bg-bg-inset/40">
                    <th className="py-2.5 px-4 font-normal">Task</th>
                    <th className="py-2.5 px-4 font-normal">Type</th>
                    <th className="py-2.5 px-4 font-normal text-right">Amount</th>
                    <th className="py-2.5 px-4 font-normal text-right">Risk</th>
                    <th className="py-2.5 px-4 font-normal">Agent verdict</th>
                    <th className="py-2.5 px-4 font-normal text-right">Confidence</th>
                  </tr>
                </thead>
                <tbody>
                  {reviews.map((r) => {
                    const isSel = active?.task_id === r.task_id;
                    return (
                      <tr
                        key={r.task_id}
                        onClick={() => setSelected(r)}
                        tabIndex={0}
                        onKeyDown={(e) => e.key === "Enter" && setSelected(r)}
                        className={`cursor-pointer border-b border-border/40 outline-none ${
                          isSel ? "bg-blue/10" : "hover:bg-bg-panel-hover/60 focus:bg-bg-panel-hover/60"
                        }`}
                      >
                        <td className="py-2.5 px-4 tnum text-text-secondary">{r.task_id.slice(0, 8)}</td>
                        <td className="py-2.5 px-4 text-text-secondary">{r.txn_type}</td>
                        <td className="py-2.5 px-4 tnum text-right text-text-primary">{money(r.amount)}</td>
                        <td className="py-2.5 px-4 tnum text-right text-text-secondary">
                          {r.risk_score.toFixed(3)}
                        </td>
                        <td className="py-2.5 px-4">
                          <Badge variant={verdictTone(r.verdict)}>{r.verdict}</Badge>
                        </td>
                        <td className="py-2.5 px-4 tnum text-right text-text-secondary">
                          {(r.confidence * 100).toFixed(0)}%
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </Panel>

        <Panel title="Decision">
          {!active ? (
            <EmptyState message="Select a case" hint="Pick a row to review its details and decide." />
          ) : (
            <div className="space-y-4">
              <div className="rounded-md border border-border bg-bg-inset px-3 py-2.5 space-y-2">
                <div className="flex items-center justify-between">
                  <span className="tnum text-[14px] text-text-secondary">{active.task_id.slice(0, 12)}</span>
                  <Badge variant={verdictTone(active.verdict)}>{active.verdict}</Badge>
                </div>
                <div className="grid grid-cols-2 gap-2 text-[13px]">
                  <Field label="Type" value={active.txn_type} />
                  <Field label="Amount" value={money(active.amount)} mono />
                  <Field label="Risk score" value={active.risk_score.toFixed(3)} mono />
                  <Field label="Confidence" value={`${(active.confidence * 100).toFixed(0)}%`} mono />
                </div>
              </div>

              <label className="flex items-center gap-2 rounded-md border border-border bg-bg-panel-hover px-2.5 focus-within:border-blue">
                <User size={13} className="text-text-tertiary" />
                <input
                  type="text"
                  placeholder="Reviewer name"
                  value={reviewerId}
                  onChange={(e) => setReviewerId(e.target.value)}
                  className="flex-1 bg-transparent py-2 text-[14px] text-text-primary placeholder:text-text-tertiary outline-none"
                />
              </label>

              <textarea
                placeholder="Notes (optional) - recorded in the audit trail"
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                rows={3}
                className="w-full bg-bg-panel-hover border border-border rounded-md px-2.5 py-2 text-[14px] text-text-primary placeholder:text-text-tertiary outline-none focus:border-blue resize-none"
              />

              {submitError && <div className="text-red text-[13px]">{submitError}</div>}

              <div className="flex gap-2">
                <button
                  onClick={() => decide("approved")}
                  disabled={submitting || !reviewerId.trim()}
                  className="flex-1 flex items-center justify-center gap-1.5 bg-green/12 text-green border border-green/30 rounded-md py-2 text-[14px] font-medium disabled:opacity-40 hover:bg-green/20 transition-colors"
                >
                  <Check size={14} />
                  Approve
                </button>
                <button
                  onClick={() => decide("rejected")}
                  disabled={submitting || !reviewerId.trim()}
                  className="flex-1 flex items-center justify-center gap-1.5 bg-red/12 text-red border border-red/30 rounded-md py-2 text-[14px] font-medium disabled:opacity-40 hover:bg-red/20 transition-colors"
                >
                  <X size={14} />
                  Reject
                </button>
              </div>
              {!reviewerId.trim() && (
                <p className="text-[12px] text-text-tertiary">
                  Enter a reviewer name - it is written to the audit trail for compliance.
                </p>
              )}
            </div>
          )}
        </Panel>
      </div>
    </div>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex flex-col">
      <span className="text-text-tertiary">{label}</span>
      <span className={`text-text-primary ${mono ? "tnum" : ""}`}>{value}</span>
    </div>
  );
}
