"use client";

import { useEffect, useState } from "react";
import { api, InfrastructureData } from "@/lib/api";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";

export default function InfrastructurePage() {
  const [data, setData] = useState<InfrastructureData | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const load = () => {
      api
        .getInfrastructure()
        .then(setData)
        .catch((e) => setError(e.message));
    };
    load();
    const interval = setInterval(load, 15000);
    return () => clearInterval(interval);
  }, []);

  const alarmVariant = (state: string) =>
    state === "OK" ? "green" : state === "ALARM" ? "red" : "default";

  return (
    <div>
      <div className="h-12 flex items-center justify-between px-4 border-b border-border">
        <h1 className="text-[16px] font-semibold text-text-primary tracking-tight">
          Infrastructure
        </h1>
        {error && <Badge variant="red">CloudWatch unreachable</Badge>}
      </div>

      <div className="p-4 space-y-3">
        <Panel title="Service health">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left text-text-tertiary border-b border-border">
                <th className="pb-1.5 font-normal">Service</th>
                <th className="pb-1.5 font-normal">Alarm state</th>
              </tr>
            </thead>
            <tbody>
              {(data?.services ?? []).map((s) => (
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
            <div className="text-text-tertiary text-xs py-4 text-center">
              No incidents in the last 24 hours
            </div>
          )}
        </Panel>
      </div>
    </div>
  );
}
