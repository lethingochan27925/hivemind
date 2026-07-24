export function Badge({
  children,
  variant = "default",
}: {
  children: React.ReactNode;
  variant?: "default" | "blue" | "green" | "yellow" | "red";
}) {
  const styles = {
    default: "bg-bg-panel-hover text-text-secondary",
    blue: "bg-blue/15 text-blue",
    green: "bg-green/15 text-green",
    yellow: "bg-yellow/15 text-yellow",
    red: "bg-red/15 text-red",
  }[variant];

  return (
    <span className={`inline-flex items-center px-1.5 py-0.5 text-xs rounded ${styles}`}>
      {children}
    </span>
  );
}
