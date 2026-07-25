"use client";

import { useEffect, useState } from "react";
import { api, PendingReview } from "@/lib/api";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { PageHeader } from "@/components/ui/page-header";
import { EmptyState } from "@/components/ui/empty-state";

export default function ReviewsPage() {
  const [reviews, setReviews] = useState<PendingReview[]>([]);
  const [selected, setSelected] = useState<PendingReview | null>(null);
  const [reviewerId, setReviewerId] = useState("");
  const [notes, setNotes] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadReviews = () => {
    api
      .getReviews()
      .then(setReviews)
      .catch((e) => setError(e.message));
  };

  useEffect(() => {
    loadReviews();
    const interval = setInterval(loadReviews, 8000);
    return () => clearInterval(interval);
  }, []);

  const handleDecide = async (decision: "approved" | "rejected") => {
    if (!selected || !reviewerId.trim()) return;
    setSubmitting(true);
    try {
      await api.decideReview({
        task_id: selected.task_id,
        reviewer_id: reviewerId.trim(),
        decision,
        notes: notes.trim(),
      });
      setSelected(null);
      setNotes("");
      loadReviews();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  const verdictColor = (v: string) =>
    v === "fraud" ? "red" : v === "escalate" ? "yellow" : "green";

  return (
    <div>
      <PageHeader
        title="Review Queue"
        description="Cases the agent flagged as uncertain, awaiting human approval"
        actions={
          <Badge variant={reviews.length > 0 ? "yellow" : "default"}>
            {reviews.length} pending
          </Badge>
        }
      />

      <div className="p-4 grid grid-cols-3 gap-3">
        <Panel title="Pending tasks" className="col-span-2">
          {error && <div className="text-red text-xs mb-2">{error}</div>}
          {reviews.length === 0 ? (
            <EmptyState message="No tasks awaiting review" />
          ) : (
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left text-text-tertiary border-b border-border">
                  <th className="pb-1.5 pr-3 font-normal">Task</th>
                  <th className="pb-1.5 pr-3 font-normal">Type</th>
                  <th className="pb-1.5 pr-3 font-normal text-right">Amount</th>
                  <th className="pb-1.5 pr-3 font-normal text-right">Risk</th>
                  <th className="pb-1.5 pr-3 font-normal">Verdict</th>
                  <th className="pb-1.5 font-normal text-right">Confidence</th>
                </tr>
              </thead>
              <tbody>
                {reviews.map((r) => (
                  <tr
                    key={r.task_id}
                    onClick={() => setSelected(r)}
                    className={`border-b border-border/50 cursor-pointer ${
                      selected?.task_id === r.task_id
                        ? "bg-bg-panel-hover"
                        : "hover:bg-bg-panel-hover/50"
                    }`}
                  >
                    <td className="py-1.5 pr-3 text-text-secondary">
                      {r.task_id.slice(0, 8)}
                    </td>
                    <td className="py-1.5 pr-3 text-text-secondary">{r.txn_type}</td>
                    <td className="py-1.5 pr-3 text-right text-text-primary">
                      {r.amount.toLocaleString()}
                    </td>
                    <td className="py-1.5 pr-3 text-right text-text-secondary">
                      {r.risk_score.toFixed(3)}
                    </td>
                    <td className="py-1.5 pr-3">
                      <Badge variant={verdictColor(r.verdict)}>{r.verdict}</Badge>
                    </td>
                    <td className="py-1.5 text-right text-text-secondary">
                      {(r.confidence * 100).toFixed(0)}%
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Panel>

        <Panel title="Decision">
          {!selected ? (
            <EmptyState message="Select a task to review" />
          ) : (
            <div className="space-y-3">
              <div className="text-xs text-text-secondary">
                Task <span className="text-text-primary">{selected.task_id}</span>
              </div>
              <div className="text-xs text-text-secondary">
                {selected.txn_type} · {selected.amount.toLocaleString()} · risk{" "}
                {selected.risk_score.toFixed(3)}
              </div>

              <input
                type="text"
                placeholder="Your name"
                value={reviewerId}
                onChange={(e) => setReviewerId(e.target.value)}
                className="w-full bg-bg-panel-hover border border-border rounded-sm px-2 py-1.5 text-xs text-text-primary placeholder:text-text-tertiary outline-none focus:border-blue"
              />

              <textarea
                placeholder="Notes (optional)"
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                rows={3}
                className="w-full bg-bg-panel-hover border border-border rounded-sm px-2 py-1.5 text-xs text-text-primary placeholder:text-text-tertiary outline-none focus:border-blue resize-none"
              />

              <div className="flex gap-2">
                <button
                  onClick={() => handleDecide("approved")}
                  disabled={submitting || !reviewerId.trim()}
                  className="flex-1 bg-green/15 text-green border border-green/30 rounded-sm py-1.5 text-xs disabled:opacity-40"
                >
                  Approve
                </button>
                <button
                  onClick={() => handleDecide("rejected")}
                  disabled={submitting || !reviewerId.trim()}
                  className="flex-1 bg-red/15 text-red border border-red/30 rounded-sm py-1.5 text-xs disabled:opacity-40"
                >
                  Reject
                </button>
              </div>
            </div>
          )}
        </Panel>
      </div>
    </div>
  );
}
