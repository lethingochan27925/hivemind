"use client";

import { useEffect, useState } from "react";
import { api, OverviewData } from "@/lib/api";
import { Panel } from "@/components/ui/panel";
import { Stat } from "@/components/ui/stat";
import { Badge } from "@/components/ui/badge";
import { PageHeader } from "@/components/ui/page-header";
import { EmptyState } from "@/components/ui/empty-state";
import { RefreshCw } from "lucide-react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from "recharts";

export default function OverviewPage() {
  const [data, setData] = useState<OverviewData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null);
  const [secondsAgo, setSecondsAgo] = useState(0);

  useEffect(() => {
    const load = () => {
      api
        .getOverview()
        .then((d) => {
          setData(d);
          setLastRefresh(new Date());
        })
        .catch((e) => setError(e.message));
    };
    load();
    const interval = setInterval(load, 5000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    if (!lastRefresh) return;
    const tick = () => {
      setSecondsAgo(Math.max(0, Math.round((Date.now() - lastRefresh.getTime()) / 1000)));
    };
    tick();
    const interval = setInterval(tick, 1000);
    return () => clearInterval(interval);
  }, [lastRefresh]);

  const fraudCount =
    data?.verdicts_today?.find((v) => v.verdict === "fraud")?.count ?? 0;
  const legitCount =
    data?.verdicts_today?.find((v) => v.verdict === "legit")?.count ?? 0;
  const escalateCount =
    data?.verdicts_today?.find((v) => v.verdict === "escalate")?.count ?? 0;

  const uniqueAgents = new Set(
    data?.live_tasks?.map((t) => t.claimed_by) ?? []
  ).size;

  return (
    <div>
      <PageHeader
        title="Overview"
        description="Fleet health, verdict breakdown, and live agent activity"
        actions={
          <div className="flex items-center gap-3 text-xs text-text-tertiary">
            {error && <Badge variant="red">API unreachable</Badge>}
            {lastRefresh && (
              <span className="flex items-center gap-1.5">
                <RefreshCw size={12} />
                Refreshed {secondsAgo}s ago
              </span>
            )}
          </div>
        }
      />

      <div className="p-4 space-y-3">
        <div className="grid grid-cols-5 border border-border rounded-sm divide-x divide-border">
          <Stat label="Fraud detected" value={fraudCount} color="red" />
          <Stat label="Escalated" value={escalateCount} color="yellow" />
          <Stat label="Legit" value={legitCount} color="green" />
          <Stat
            label="Pending review"
            value={data?.pending_reviews ?? "—"}
            color={(data?.pending_reviews ?? 0) > 0 ? "yellow" : "default"}
          />
          <Stat label="Active agents" value={uniqueAgents} color="blue" />
        </div>

        {data?.verdict_accuracy_pct != null && (
          <Panel title="Real-world impact">
            <div className="flex items-baseline gap-2">
              <span className="text-2xl font-semibold text-green">
                {data.verdict_accuracy_pct}%
              </span>
              <span className="text-xs text-text-tertiary">
                verdict accuracy against ground truth labels
              </span>
            </div>
          </Panel>
        )}

        <Panel title="Memory recall hits">
          <div className="h-36">
            {data?.memory_hits_trend && data.memory_hits_trend.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={data.memory_hits_trend}>
                  <CartesianGrid stroke="#2e2e2e" strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="hour_bucket"
                    stroke="#6C6C6C"
                    fontSize={10}
                    tickLine={false}
                    axisLine={{ stroke: "#2e2e2e" }}
                  />
                  <YAxis
                    stroke="#6C6C6C"
                    fontSize={10}
                    tickLine={false}
                    axisLine={{ stroke: "#2e2e2e" }}
                    width={24}
                  />
                  <Tooltip
                    contentStyle={{
                      background: "#1e1e1e",
                      border: "1px solid #2e2e2e",
                      fontSize: 11,
                      borderRadius: 4,
                    }}
                  />
                  <Line
                    type="monotone"
                    dataKey="avg_memory_hits"
                    stroke="#5794F2"
                    strokeWidth={1.5}
                    dot={{ r: 2, fill: "#5794F2" }}
                  />
                </LineChart>
              </ResponsiveContainer>
            ) : (
              <EmptyState message="No memory recall data yet" />
            )}
          </div>
        </Panel>

        <Panel title="Live fleet">
          {data?.live_tasks && data.live_tasks.length > 0 ? (
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left text-text-tertiary border-b border-border">
                  <th className="pb-1.5 font-normal">Task</th>
                  <th className="pb-1.5 font-normal">Agent</th>
                  <th className="pb-1.5 font-normal">Claimed</th>
                </tr>
              </thead>
              <tbody>
                {data.live_tasks.slice(0, 10).map((task) => (
                  <tr key={task.task_id} className="border-b border-border/50 text-text-secondary">
                    <td className="py-1.5">{task.task_id.slice(0, 8)}</td>
                    <td className="py-1.5">{task.claimed_by.slice(0, 12)}</td>
                    <td className="py-1.5">
                      {new Date(task.claimed_at).toLocaleTimeString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <EmptyState message="No agents currently investigating" />
          )}
        </Panel>
      </div>
    </div>
  );
}
