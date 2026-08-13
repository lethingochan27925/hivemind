"use client";

import { PieChart, Pie, Cell, ResponsiveContainer } from "recharts";
import { CHART_CHROME } from "@/lib/palette";

type Slice = { name: string; value: number; color: string };

/**
 * VerdictDonut - a ring showing how the fleet's decisions split across
 * fraud / escalate / legit, with the total agent-handled count in the centre.
 */
export function VerdictDonut({ data }: { data: Slice[] }) {
  const total = data.reduce((s, d) => s + d.value, 0);

  return (
    <div className="flex items-center gap-6">
      <div className="relative w-[150px] h-[150px] shrink-0">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={total > 0 ? data : [{ name: "none", value: 1, color: CHART_CHROME.empty }]}
              dataKey="value"
              innerRadius={52}
              outerRadius={72}
              startAngle={90}
              endAngle={-270}
              paddingAngle={total > 0 ? 2 : 0}
              stroke="none"
            >
              {(total > 0 ? data : [{ color: CHART_CHROME.empty }]).map((d, i) => (
                <Cell key={i} fill={d.color} />
              ))}
            </Pie>
          </PieChart>
        </ResponsiveContainer>
        <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
          <span className="tnum text-[30px] font-bold text-text-primary">{total}</span>
          <span className="text-[11px] text-text-tertiary uppercase tracking-wide font-medium">
            verdicts
          </span>
        </div>
      </div>
      <ul className="flex flex-col gap-3 text-[14px] min-w-0">
        {data.map((d) => (
          <li key={d.name} className="flex items-center gap-2.5">
            <span className="w-3 h-3 rounded-sm shrink-0" style={{ background: d.color }} />
            <span className="text-text-secondary capitalize w-20 font-medium">{d.name}</span>
            <span className="tnum text-text-primary font-semibold w-10 text-right">{d.value}</span>
            <span className="tnum text-text-tertiary w-12 text-right">
              {total > 0 ? `${((d.value / total) * 100).toFixed(0)}%` : "-"}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
