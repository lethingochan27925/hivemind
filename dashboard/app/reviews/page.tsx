"use client";

import { useState } from "react";
import { api, PendingReview } from "@/lib/api";
import { useLive } from "@/lib/use-live";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { Check, X, User, CornerUpLeft, Loader2, CheckCheck } from "lucide-react";

const money = (n: number) =>
  n.toLocaleString(undefined, { maximumFractionDigits: 0 });

export default function ReviewsPage() {
  const { data, error, lastUpdated } = useLive(api.getReviews, 8000);

  // Bo row khoi danh sach ngay lap tuc (danh sach live se dong bo o vong sau).
  const [removed, setRemoved] = useState<Set<string>>(new Set());
  const removeLocal = (ids: string[]) => setRemoved((prev) => new Set([...prev, ...ids]));

  const reviews = (data ?? []).filter((r) => !removed.has(r.task_id));

  const [selected, setSelected] = useState<PendingReview | null>(null);
  const [reviewerId, setReviewerId] = useState("");
  const [notes, setNotes] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [bulkMsg, setBulkMsg] = useState<string | null>(null);
  const [bulkBusy, setBulkBusy] = useState<string | null>(null);

  const active = selected && reviews.find((r) => r.task_id === selected.task_id) ? selected : null;

  const toggle = (id: string) => {
    const next = new Set(picked);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setPicked(next);
  };

  // Bulk: quyet nhieu case mot luc, hoac tra ca loat ve cho agent dieu tra lai.
  const bulk = async (kind: "approved" | "rejected" | "sendback") => {
    const ids = [...picked];
    if (ids.length === 0 || (kind !== "sendback" && !reviewerId.trim())) return;
    setBulkBusy(kind);
    setBulkMsg(null);
    try {
      if (kind === "sendback") {
        const results = await Promise.allSettled(ids.map((id) => api.requeueTask(id)));
        const ok = results.filter((r) => r.status === "fulfilled").length;
        setBulkMsg(`Sent ${ok}/${ids.length} case(s) back to the agent for re-investigation.`);
      } else {
        const r = await api.bulkReview(ids, kind, reviewerId.trim(), notes.trim());
        setBulkMsg(`${r.decided} decided${r.failed ? `, ${r.failed} skipped (already resolved)` : ""}.`);
      }
      removeLocal(ids);
      setPicked(new Set());
      setSelected(null);
    } catch (e) {
      setBulkMsg((e as Error).message);
    } finally {
      setBulkBusy(null);
    }
  };

  const sendBackOne = async () => {
    if (!active) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      await api.requeueTask(active.task_id);
      removeLocal([active.task_id]);
      setSelected(null);
      setBulkMsg("Case sent back - the agent will investigate it again.");
    } catch (e) {
      setSubmitError((e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

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
      removeLocal([active.task_id]);
      setSelected(null);
      setNotes("");
    } catch (e) {
      const m = (e as Error).message;
      // Case da bi re-queue/da quyet trong luc dang mo form: bo chon, danh sach
      // live se tu lam moi - khong hien loi API tho.
      if (m.includes("409") || m.toLowerCase().includes("no longer")) {
        setSelected(null);
        setSubmitError("This case was just re-queued or already decided - the queue refreshed itself.");
      } else {
        setSubmitError(m);
      }
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
        <Panel
          title="Escalated cases"
          className="lg:col-span-2"
          bodyClassName="p-0"
          actions={
            picked.size > 0 ? (
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="text-[12px] text-text-tertiary tnum mr-1">{picked.size} selected</span>
                <button
                  disabled={bulkBusy !== null || !reviewerId.trim()}
                  onClick={() => bulk("approved")}
                  className="flex items-center gap-1 px-2.5 py-1 rounded-md border border-green/30 bg-green/10 text-green text-[12px] font-semibold disabled:opacity-40 hover:bg-green/20 transition-colors"
                  title={reviewerId.trim() ? "Approve all selected" : "Enter a reviewer name first"}
                >
                  {bulkBusy === "approved" ? <Loader2 size={12} className="animate-spin" /> : <CheckCheck size={12} />}
                  Approve all
                </button>
                <button
                  disabled={bulkBusy !== null || !reviewerId.trim()}
                  onClick={() => bulk("rejected")}
                  className="flex items-center gap-1 px-2.5 py-1 rounded-md border border-red/30 bg-red/10 text-red text-[12px] font-semibold disabled:opacity-40 hover:bg-red/20 transition-colors"
                >
                  {bulkBusy === "rejected" ? <Loader2 size={12} className="animate-spin" /> : <X size={12} />}
                  Reject all
                </button>
                <button
                  disabled={bulkBusy !== null}
                  onClick={() => bulk("sendback")}
                  className="flex items-center gap-1 px-2.5 py-1 rounded-md border border-blue/30 bg-blue/10 text-blue text-[12px] font-semibold disabled:opacity-40 hover:bg-blue/20 transition-colors"
                  title="Send back to the agent instead of deciding manually"
                >
                  {bulkBusy === "sendback" ? <Loader2 size={12} className="animate-spin" /> : <CornerUpLeft size={12} />}
                  Back to agent
                </button>
                <button
                  onClick={() => setPicked(new Set())}
                  className="px-2 py-1 rounded-md border border-border text-[12px] text-text-tertiary hover:text-text-primary transition-colors"
                >
                  clear
                </button>
              </div>
            ) : null
          }
        >
          {bulkMsg && (
            <div className="px-4 py-2 border-b border-border text-[13px] text-text-secondary">{bulkMsg}</div>
          )}
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
                    <th className="py-2.5 pl-4 pr-1 font-normal w-8">
                      <input
                        type="checkbox"
                        aria-label="Select all"
                        checked={picked.size > 0 && picked.size === reviews.length}
                        onChange={(e) =>
                          setPicked(e.target.checked ? new Set(reviews.map((r) => r.task_id)) : new Set())
                        }
                        className="accent-[#7d9bf0]"
                      />
                    </th>
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
                        <td className="py-2.5 pl-4 pr-1" onClick={(e) => e.stopPropagation()}>
                          <input
                            type="checkbox"
                            aria-label={`Select ${r.task_id.slice(0, 8)}`}
                            checked={picked.has(r.task_id)}
                            onChange={() => toggle(r.task_id)}
                            className="accent-[#7d9bf0]"
                          />
                        </td>
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
              <button
                onClick={sendBackOne}
                disabled={submitting}
                className="w-full flex items-center justify-center gap-1.5 bg-blue/12 text-blue border border-blue/30 rounded-md py-2 text-[14px] font-medium disabled:opacity-40 hover:bg-blue/20 transition-colors"
                title="Return this case to the fleet instead of deciding it yourself"
              >
                <CornerUpLeft size={14} />
                Send back to agent
              </button>

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
