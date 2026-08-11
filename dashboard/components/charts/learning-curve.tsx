"use client";

import {
  ResponsiveContainer,
  ComposedChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
} from "recharts";

export interface LearningCurvePoint {
  hour: string;
  latency: number; // avg bedrock reasoning latency (ms)
  hits: number; // avg memory hits per investigation
}

/**
 * LearningCurve - the quantitative proof of agentic memory: as case_memory
 * grows, recall (hits) rises and reasoning latency falls. Two series, two axes.
 */
export function LearningCurve({ data }: { data: LearningCurvePoint[] }) {
  return (
    <div className="h-[220px]">
      <ResponsiveContainer width="100%" height="100%">
        <ComposedChart data={data} margin={{ top: 8, right: 4, bottom: 0, left: 4 }}>
          <CartesianGrid stroke="var(--border)" strokeDasharray="3 3" vertical={false} />
          <XAxis
            dataKey="hour"
            tick={{ fontSize: 11, fill: "var(--text-tertiary)" }}
            tickLine={false}
            axisLine={{ stroke: "var(--border)" }}
          />
          <YAxis
            yAxisId="latency"
            orientation="left"
            tick={{ fontSize: 11, fill: "var(--text-tertiary)" }}
            tickLine={false}
            axisLine={false}
            width={44}
            tickFormatter={(v: number) => `${v}ms`}
          />
          <YAxis
            yAxisId="hits"
            orientation="right"
            tick={{ fontSize: 11, fill: "var(--text-tertiary)" }}
            tickLine={false}
            axisLine={false}
            width={34}
          />
          <Tooltip
            contentStyle={{
              background: "var(--bg-panel)",
              border: "1px solid var(--border-strong)",
              borderRadius: 8,
              fontSize: 13,
            }}
            labelStyle={{ color: "var(--text-secondary)" }}
          />
          <Line
            yAxisId="latency"
            type="monotone"
            dataKey="latency"
            stroke="#ff9830"
            strokeWidth={2}
            dot={false}
            name="avg reasoning latency (ms)"
          />
          <Line
            yAxisId="hits"
            type="monotone"
            dataKey="hits"
            stroke="#5794f2"
            strokeWidth={2}
            dot={false}
            name="avg memory hits"
          />
        </ComposedChart>
      </ResponsiveContainer>
      <div className="flex gap-4 mt-1 text-[12px] text-text-tertiary justify-end">
        <span className="flex items-center gap-1.5">
          <span className="w-3 h-0.5 rounded bg-[#ff9830] inline-block" /> reasoning latency
        </span>
        <span className="flex items-center gap-1.5">
          <span className="w-3 h-0.5 rounded bg-[#5794f2] inline-block" /> memory hits
        </span>
      </div>
    </div>
  );
}
