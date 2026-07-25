"use client";

import { useEffect, useState } from "react";
import { api, InfrastructureData } from "@/lib/api";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { PageHeader } from "@/components/ui/page-header";
import { EmptyState } from "@/components/ui/empty-state";
import { Zap } from "lucide-react";

export default function InfrastructurePage() {
  const [data, setData] = useState<InfrastructureData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [simResult, setSimResult] = useState<string | null>(null);
  const [simulating, setSimulating] = useState(false);

  const load = () => {
    api
      .getInfrastructure()
      .then(setData)
      .catch((e) => setError(e.message));
  };

  useEffect(() => {
    load();
    const interval = setInterval(load, 15000);
    return () => clearInterval(interval);
  }, []);

  const handleSimulate = async () => {
    setSimulating(true);
    setSimResult(null);
    try {
      const res = await api.simulateCrash();
      setSimResult(`Task ${res.task_id.slice(0, 8)} — ${res.message}`);
    } catch (e) {
      setSimResult(`Failed: ${(e as Error).message}`);
    } finally {
      setSimulating(false);
    }
  };

  const alarmVariant = (state: string) =>
    state === "OK" ? "green" : state === "ALARM" ? "red" : "default";

  return (
    <div>
      <PageHeader
        title="Infrastructure"
        description="Service health and resilience — how the fleet recovers from failure"
        actions={error && <Badge variant="red">CloudWatch unreachable</Badge>}
      />

      <div className="p-4 space-y-3">
        <Panel title="Chaos test">
          <div className="flex items-center justify-between">
            <div className="text-xs text-text-secondary max-w-md">
              Backdates the heartbeat of a running task to simulate an agent
              crash. The Heartbeat Reaper will detect and re-queue it on its
              next cycle — watch the Live Fleet on Overview to see it resume.
            </div>
            <button
              onClick={handleSimulate}
              disabled={simulating}
              className="flex items-center gap-1.5 bg-yellow/15 text-yellow border border-yellow/30 rounded-sm px-3 py-1.5 text-xs disabled:opacity-40 shrink-0"
            >
              <Zap size={13} />
              {simulating ? "Simulating..." : "Simulate agent crash"}
            </button>
          </div>
          {simResult && (
            <div className="mt-2 text-[11px] text-text-secondary border-t border-border pt-2">
              {simResult}
            </div>
          )}
        </Panel>

        <Panel title="Service health">
          {data?.services && data.services.length > 0 ? (
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left text-text-tertiary border-b border-border">
                  <th className="pb-1.5 font-normal">Service</th>
                  <th className="pb-1.5 font-normal">Alarm state</th>
                </tr>
              </thead>
              <tbody>
                {data.services.map((s) => (
                  <tr key={s.service} className="border-b border-border/50">
                    <td className="py-1.5 text-text-secondary">{s.service}</td>
                    <td className="py-1.5">
                      <Badge variant={alarmVariant(s.alarm_state)}>
                        {s.alarm_state}
                      </Badge>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <EmptyState message="No service data available" />
          )}
        </Panel>

        <Panel title="Incident timeline — last 24h">
          {data?.incidents && data.incidents.length > 0 ? (
            <div className="space-y-2">
              {data.incidents.map((inc, i) => (
                <div key={i} className="border-l-2 border-yellow/40 pl-2 py-1">
                  <div className="text-xs text-text-primary">
                    Task re-queued — {inc.service}
                  </div>
                  {inc.description && (
                    <div className="text-[11px] text-text-secondary mt-0.5">
                      {inc.description}
                    </div>
                  )}
                  <div className="text-[10px] text-text-tertiary mt-0.5">
                    {new Date(inc.timestamp).toLocaleString()}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState message="No incidents in the last 24 hours" />
          )}
        </Panel>
      </div>
    </div>
  );
}
