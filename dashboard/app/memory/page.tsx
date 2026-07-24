"use client";

import { useEffect, useState } from "react";
import { api, MemoryData } from "@/lib/api";
import { Panel } from "@/components/ui/panel";
import { Stat } from "@/components/ui/stat";
import { Badge } from "@/components/ui/badge";

export default function MemoryPage() {
  const [data, setData] = useState<MemoryData | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const load = () => {
      api
        .getMemory()
        .then(setData)
        .catch((e) => setError(e.message));
    };
    load();
    const interval = setInterval(load, 8000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div>
      <div className="h-12 flex items-center justify-between px-4 border-b border-border">
        <h1 className="text-[16px] font-semibold text-text-primary tracking-tight">
          Fleet & Memory
        </h1>
        {error && <Badge variant="red">API unreachable</Badge>}
      </div>

      <div className="p-4 space-y-3">
        <div className="grid grid-cols-4 border border-border rounded-sm divide-x divide-border">
          <Stat
            label="Active cases"
            value={data?.stats.active_cases ?? "—"}
            color="blue"
          />
          <Stat
            label="Archived cases"
            value={data?.stats.archived_cases ?? "—"}
            color="default"
          />
          <Stat
            label="Avg salience"
            value={data?.stats.avg_salience?.toFixed(2) ?? "—"}
            color="green"
          />
          <Stat
            label="Verdict accuracy"
            value={
              data?.impact.verdict_accuracy_pct != null
                ? `${data.impact.verdict_accuracy_pct}%`
                : "—"
            }
            color="green"
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Panel title="Top patterns">
            {data?.stats.top_patterns && data.stats.top_patterns.length > 0 ? (
              <table className="w-full text-xs">
                <thead>
                  <tr className="text-left text-text-tertiary border-b border-border">
                    <th className="pb-1.5 font-normal">Pattern</th>
                    <th className="pb-1.5 font-normal text-right">Count</th>
                  </tr>
                </thead>
                <tbody>
                  {data.stats.top_patterns.map((p) => (
                    <tr key={p.pattern_type} className="border-b border-border/50">
                      <td className="py-1.5 text-text-secondary">
                        {p.pattern_type}
                      </td>
                      <td className="py-1.5 text-right text-text-primary">
                        {p.count}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <div className="text-text-tertiary text-xs py-4 text-center">
                No patterns recorded yet
              </div>
            )}
          </Panel>

          <Panel title="Active agents">
            {data?.active_agents && data.active_agents.length > 0 ? (
              <table className="w-full text-xs">
                <thead>
                  <tr className="text-left text-text-tertiary border-b border-border">
                    <th className="pb-1.5 font-normal">Agent</th>
                    <th className="pb-1.5 font-normal">Status</th>
                    <th className="pb-1.5 font-normal">Task</th>
                  </tr>
                </thead>
                <tbody>
                  {data.active_agents.map((a) => (
                    <tr key={a.agent_id} className="border-b border-border/50">
                      <td className="py-1.5 text-text-secondary">
                        {a.agent_id.slice(0, 12)}
                      </td>
                      <td className="py-1.5">
                        <Badge variant={a.status === "active" ? "blue" : "default"}>
                          {a.status}
                        </Badge>
                      </td>
                      <td className="py-1.5 text-text-secondary">
                        {a.current_task ? a.current_task.slice(0, 8) : "—"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <div className="text-text-tertiary text-xs py-4 text-center">
                No agents active in the last 10 minutes
              </div>
            )}
          </Panel>
        </div>
      </div>
    </div>
  );
}
