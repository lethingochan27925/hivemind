import { ReactNode } from "react";

type Variant = "default" | "blue" | "green" | "yellow" | "red" | "purple" | "teal";

const styles: Record<Variant, string> = {
  default: "bg-bg-panel-hover text-text-secondary border-border",
  blue: "bg-blue/15 text-blue border-blue/30",
  green: "bg-green/15 text-green border-green/30",
  yellow: "bg-yellow/15 text-yellow border-yellow/30",
  red: "bg-red/15 text-red border-red/30",
  purple: "bg-purple/15 text-purple border-purple/30",
  teal: "bg-teal/15 text-teal border-teal/30",
};

export function Badge({
  children,
  variant = "default",
  dot = false,
}: {
  children: ReactNode;
  variant?: Variant;
  dot?: boolean;
}) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2 py-0.5 text-[12px] font-medium rounded-md border ${styles[variant]}`}
    >
      {dot && <span className="w-1.5 h-1.5 rounded-full bg-current" />}
      {children}
    </span>
  );
}
