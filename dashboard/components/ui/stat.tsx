import { ReactNode } from "react";

type Tone = "default" | "blue" | "green" | "yellow" | "red" | "purple" | "teal";

const toneText: Record<Tone, string> = {
  default: "text-text-primary",
  blue: "text-blue",
  green: "text-green",
  yellow: "text-yellow",
  red: "text-red",
  purple: "text-purple",
  teal: "text-teal",
};

const toneBar: Record<Tone, string> = {
  default: "bg-border-strong",
  blue: "bg-blue",
  green: "bg-green",
  yellow: "bg-yellow",
  red: "bg-red",
  purple: "bg-purple",
  teal: "bg-teal",
};

/**
 * Stat - a single KPI cell. A left accent bar encodes the metric's state colour;
 * the value is set large in tabular monospace so a row of stats aligns to the digit.
 */
export function Stat({
  label,
  value,
  unit,
  color = "default",
  hint,
  icon,
}: {
  label: string;
  value: string | number;
  unit?: string;
  color?: Tone;
  hint?: string;
  icon?: ReactNode;
}) {
  return (
    <div className="relative flex flex-col gap-2 px-5 py-5">
      <span className={`absolute left-0 top-5 bottom-5 w-1 rounded-full ${toneBar[color]}`} />
      <div className="flex items-center gap-2 text-[13px] font-medium text-text-secondary">
        {icon}
        <span className="uppercase tracking-wide">{label}</span>
      </div>
      <div className="flex items-baseline gap-1.5">
        <span className={`tnum text-[40px] leading-none font-bold ${toneText[color]}`}>
          {value}
        </span>
        {unit && <span className="text-base text-text-tertiary font-medium">{unit}</span>}
      </div>
      {hint && <span className="text-[12px] text-text-tertiary">{hint}</span>}
    </div>
  );
}
