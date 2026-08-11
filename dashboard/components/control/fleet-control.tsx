"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { useLive } from "@/lib/use-live";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { Play, Pause, Zap, Plus, Loader2 } from "lucide-react";

const TASK_TONES: Record<string, string> = {
  pending: "text-yellow",
  investigating: "text-blue",
  pending_review: "text-yellow",
  done: "text-green",
  failed: "text-red",
};

/**
 * FleetControl - the command bar. Every button is a real mutation on the live
 * system: start/pause the EventBridge schedules, feed cases into the queue, and
 * trigger a dispatch cycle that invokes the worker fleet.
 */
export function FleetControl() {
  const { data } = useLive(api.getFleetStatus, 4000);
  const [busy, setBusy] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [feed, setFeed] = useState(50);

  const running = data?.running ?? false;
  const tasks = data?.tasks ?? {};

  const act = async (key: string, fn: () => Promise<string>) => {
    setBusy(key);
    setMsg(null);
    try {
      setMsg(await fn());
    } catch (e) {
      setMsg(`Failed: ${(e as Error).message}`);
    } finally {
      setBusy(null);
    }
  };

  const btn =
    "flex items-center gap-2 px-3.5 py-2 rounded-md text-[14px] font-semibold border disabled:opacity-40 transition-colors";

  return (
    <Panel
      title="Fleet control"
      subtitle="operate the system live"
      actions={
        <Badge variant={running ? "green" : "default"} dot>
          {running ? "Fleet running" : "Fleet paused"}
        </Badge>
      }
    >
      <div className="flex flex-wrap items-center gap-3">
        <button
          disabled={busy !== null}
          onClick={() =>
            act("toggle", async () => {
              const r = await api.setFleetState(running ? "pause" : "start");
              return r.running ? "Fleet started - schedules enabled." : "Fleet paused - schedules disabled.";
            })
          }
          className={`${btn} ${
            running
              ? "bg-red/12 text-red border-red/30 hover:bg-red/20"
              : "bg-green/12 text-green border-green/30 hover:bg-green/20"
          }`}
        >
          {busy === "toggle" ? <Loader2 size={15} className="animate-spin" /> : running ? <Pause size={15} /> : <Play size={15} />}
          {running ? "Pause fleet" : "Start fleet"}
        </button>

        <button
          disabled={busy !== null}
          onClick={() =>
            act("dispatch", async () => {
              const r = await api.runDispatch();
              const d = r.dispatcher_result;
              return `Dispatch cycle: created ${d.tasks_created}, ${d.pending_tasks} pending, invoked ${d.workers_invoked} workers.`;
            })
          }
          className={`${btn} bg-blue/12 text-blue border-blue/30 hover:bg-blue/20`}
        >
          {busy === "dispatch" ? <Loader2 size={15} className="animate-spin" /> : <Zap size={15} />}
          Run dispatch cycle
        </button>

        <div className="flex items-center gap-2 ml-auto">
          <input
            type="number"
            min={1}
            max={1000}
            value={feed}
            onChange={(e) => setFeed(Math.max(1, Math.min(1000, Number(e.target.value) || 0)))}
            className="w-20 bg-bg-panel-hover border border-border rounded-md px-2 py-2 text-[14px] tnum text-text-primary outline-none focus:border-blue"
          />
          <button
            disabled={busy !== null}
            onClick={() =>
              act("feed", async () => {
                const r = await api.feedStream(feed);
                return `Fed ${r.requeued} cases into the queue - the fleet is picking them up.`;
              })
            }
            className={`${btn} bg-purple/12 text-purple border-purple/30 hover:bg-purple/20`}
          >
            {busy === "feed" ? <Loader2 size={15} className="animate-spin" /> : <Plus size={15} />}
            Feed cases
          </button>
        </div>
      </div>

      <div className="flex flex-wrap gap-x-5 gap-y-1 mt-4 pt-3 border-t border-border text-[13px]">
        {Object.entries(tasks).length === 0 ? (
          <span className="text-text-tertiary">No tasks yet</span>
        ) : (
          Object.entries(tasks).map(([k, v]) => (
            <span key={k} className="flex items-center gap-1.5">
              <span className="text-text-tertiary">{k.replace(/_/g, " ")}</span>
              <span className={`tnum font-semibold ${TASK_TONES[k] ?? "text-text-primary"}`}>{v}</span>
            </span>
          ))
        )}
      </div>

      {msg && (
        <div className="mt-3 text-[13px] text-text-secondary border-t border-border pt-2.5">{msg}</div>
      )}
    </Panel>
  );
}
