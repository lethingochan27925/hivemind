type Tone = "blue" | "green" | "yellow" | "red" | "purple" | "teal";

const barColor: Record<Tone, string> = {
  blue: "bg-blue",
  green: "bg-green",
  yellow: "bg-yellow",
  red: "bg-red",
  purple: "bg-purple",
  teal: "bg-teal",
};

/**
 * Meter — a horizontal proportion bar with a label and a right-aligned value.
 * Used for "X of Y" style figures (auto-resolve rate, precision, coverage).
 */
export function Meter({
  label,
  value,
  max,
  tone = "blue",
  display,
}: {
  label: string;
  value: number;
  max: number;
  tone?: Tone;
  display?: string;
}) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-baseline justify-between text-[13px]">
        <span className="text-text-secondary font-medium">{label}</span>
        <span className="tnum text-text-primary font-semibold">
          {display ?? `${pct.toFixed(0)}%`}
        </span>
      </div>
      <div className="h-2.5 rounded-full bg-bg-inset overflow-hidden">
        <div
          className={`h-full rounded-full ${barColor[tone]} transition-[width] duration-500`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}
