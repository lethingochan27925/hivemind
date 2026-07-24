export function Stat({
  label,
  value,
  color = "default",
}: {
  label: string;
  value: string | number;
  color?: "default" | "blue" | "green" | "yellow" | "red";
}) {
  const colorClass = {
    default: "text-text-primary",
    blue: "text-blue",
    green: "text-green",
    yellow: "text-yellow",
    red: "text-red",
  }[color];

  return (
    <div className="px-4 py-4">
      <div className="text-[12px] text-text-tertiary mb-1.5">{label}</div>
      <div className={`text-3xl font-semibold ${colorClass}`}>{value}</div>
    </div>
  );
}
