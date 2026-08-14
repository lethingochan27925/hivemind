"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { useFleet, refreshFleet } from "@/lib/use-fleet";
import { useT } from "@/lib/i18n";
import { Play, Plus, RefreshCw, Loader2, AlertTriangle, CheckCircle2, Activity } from "lucide-react";

/**
 * SystemStatus - one line that answers "what is this system doing, and what
 * should I press", directly above everything else on Mission Control.
 *
 * It exists because an idle HiveMind and a broken HiveMind rendered identically:
 * five zeroed KPIs and a grid of empty panels. A visitor could not tell whether
 * the fleet had nothing to do, or the control plane was unreachable. Now the
 * page states which of those it is, and offers the single action that moves it
 * forward.
 */

type Tone = "red" | "blue" | "yellow" | "green";

const TONE: Record<Tone, { box: string; text: string }> = {
  red: { box: "border-red/40 bg-red/8", text: "text-red" },
  blue: { box: "border-blue/40 bg-blue/8", text: "text-blue" },
  yellow: { box: "border-yellow/40 bg-yellow/8", text: "text-yellow" },
  green: { box: "border-green/35 bg-green/6", text: "text-green" },
};

export function SystemStatus({
  apiError,
  hasVerdicts,
}: {
  apiError?: string | null;
  hasVerdicts: boolean;
}) {
  const t = useT();
  const { data, error, loading } = useFleet();
  const [busy, setBusy] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);

  const run = async (key: string, fn: () => Promise<string>) => {
    setBusy(key);
    setNote(null);
    try {
      setNote(await fn());
    } catch (e) {
      setNote(`${t("Failed")}: ${(e as Error).message}`);
    } finally {
      await refreshFleet();
      setBusy(null);
    }
  };

  // One click for the whole cold start: enable the schedules, put work in the
  // queue, and kick a dispatch cycle so the workers pick it up now rather than
  // at the next scheduled tick.
  const startDemo = () =>
    run("demo", async () => {
      await api.setFleetState("start");
      const fed = await api.feedStream(100);
      const d = await api.runDispatch();
      return t("Fleet started · {n} cases queued · {w} workers invoked")
        .replace("{n}", String(fed.requeued))
        .replace("{w}", String(d.dispatcher_result.workers_invoked));
    });

  // Queue already has work: enable the schedules and kick one cycle, but do not
  // pile more cases on top of a backlog.
  const startOnly = () =>
    run("start", async () => {
      await api.setFleetState("start");
      const d = await api.runDispatch();
      return t("Fleet started · {w} workers invoked").replace(
        "{w}",
        String(d.dispatcher_result.workers_invoked)
      );
    });

  if (loading && !data) return null;

  const down = apiError || error;
  const running = data?.running ?? false;
  const tasks = data?.tasks ?? {};
  const pending = tasks.pending ?? 0;
  const investigating = (tasks.investigating ?? 0) + (tasks.claimed ?? 0);
  const inFlight = pending + investigating;

  let tone: Tone;
  let icon = <Activity size={17} />;
  let headline: string;
  let detail: string;
  let action: React.ReactNode = null;

  // Branch on `running` first and exhaust each side. An earlier version ended in
  // a bare `else` that assumed the fleet was running, so a paused fleet with an
  // empty queue and some history behind it announced "Fleet running" while the
  // command bar three centimetres below read "Fleet paused".
  const startFleetButton = (label: string, feed: boolean) => (
    <button
      onClick={feed ? startDemo : startOnly}
      disabled={busy !== null}
      className={btnClass(feed ? "blue" : "yellow")}
    >
      {busy ? <Loader2 size={14} className="animate-spin" /> : <Play size={14} />}
      {label}
    </button>
  );

  if (down) {
    tone = "red";
    icon = <AlertTriangle size={17} />;
    headline = t("Cannot reach the control plane");
    detail = down;
    action = (
      <button onClick={() => window.location.reload()} className={btnClass("red")}>
        <RefreshCw size={14} />
        {t("Retry")}
      </button>
    );
  } else if (!running) {
    if (inFlight > 0) {
      tone = "yellow";
      icon = <AlertTriangle size={17} />;
      headline = t("{n} cases are waiting but the fleet is paused").replace("{n}", String(inFlight));
      detail = t("Schedules are disabled, so nothing will pick these up until the fleet is started.");
      action = startFleetButton(t("Start fleet"), false);
    } else {
      tone = "blue";
      icon = <Play size={17} />;
      headline = hasVerdicts ? t("Fleet is paused") : t("Nothing is running yet");
      detail = hasVerdicts
        ? t("The queue is empty and schedules are disabled. Start it again to process more cases.")
        : t(
            "The fleet is paused and the queue is empty. Start it to watch agents claim cases, recall similar past fraud from memory, and decide."
          );
      action = startFleetButton(t("Start the fleet and feed 100 cases"), true);
    }
  } else if (inFlight === 0) {
    tone = "green";
    icon = <CheckCircle2 size={17} />;
    headline = t("Fleet running · queue drained");
    detail = t("Every queued case has been decided. Feed more to keep the fleet working.");
    action = (
      <button
        onClick={() =>
          run("feed", async () => {
            const fed = await api.feedStream(100);
            await api.runDispatch();
            return t("{n} cases queued").replace("{n}", String(fed.requeued));
          })
        }
        disabled={busy !== null}
        className={btnClass("green")}
      >
        {busy === "feed" ? <Loader2 size={14} className="animate-spin" /> : <Plus size={14} />}
        {t("Feed 100 more")}
      </button>
    );
  } else {
    tone = "green";
    icon = <Activity size={17} />;
    headline = t("Fleet running");
    detail = t("{a} under investigation · {p} waiting in the queue")
      .replace("{a}", String(investigating))
      .replace("{p}", String(pending));
  }

  const style = TONE[tone];

  return (
    <div className={`flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border px-4 py-3 ${style.box}`}>
      <span className={`shrink-0 ${style.text}`}>{icon}</span>
      <div className="min-w-0 flex-1">
        <div className={`text-[14px] font-semibold ${style.text}`}>{headline}</div>
        <div className="text-[13px] text-text-secondary mt-0.5">{note ?? detail}</div>
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  );
}

function btnClass(tone: Tone) {
  const map: Record<Tone, string> = {
    red: "bg-red/12 text-red border-red/30 hover:bg-red/20",
    blue: "bg-blue/12 text-blue border-blue/30 hover:bg-blue/20",
    yellow: "bg-yellow/12 text-yellow border-yellow/30 hover:bg-yellow/20",
    green: "bg-green/12 text-green border-green/30 hover:bg-green/20",
  };
  return `flex items-center gap-2 px-3.5 py-2 rounded-md text-[14px] font-semibold border transition-colors disabled:opacity-40 ${map[tone]}`;
}
