"use client";

type Row = { label: string; value: number };

/**
 * BarList - a horizontal ranked-bar list (top patterns, distributions). Bars are
 * sized relative to the largest value; the value is right-aligned in monospace.
 */
export function BarList({
  rows,
  color = "#bd82e6",
  formatLabel,
}: {
  rows: Row[];
  color?: string;
  formatLabel?: (s: string) => string;
}) {
  const max = Math.max(1, ...rows.map((r) => r.value));
  return (
    <ul className="flex flex-col gap-2.5">
      {rows.map((r) => (
        <li key={r.label} className="flex items-center gap-3 text-[14px]">
          <span className="w-36 shrink-0 text-text-secondary truncate font-medium">
            {formatLabel ? formatLabel(r.label) : r.label}
          </span>
          <div className="flex-1 h-5 rounded bg-bg-inset overflow-hidden">
            <div
              className="h-full rounded transition-[width] duration-500"
              style={{ width: `${(r.value / max) * 100}%`, background: color, opacity: 0.85 }}
            />
          </div>
          <span className="tnum w-10 text-right text-text-primary font-semibold">{r.value}</span>
        </li>
      ))}
    </ul>
  );
}
