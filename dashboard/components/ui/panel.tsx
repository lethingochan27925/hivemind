export function Panel({
  title,
  children,
  className = "",
}: {
  title?: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={`border border-border rounded-sm bg-bg-panel/50 ${className}`}>
      {title && (
        <div className="px-2.5 py-1.5 border-b border-border">
          <h2 className="text-[11px] text-text-secondary">{title}</h2>
        </div>
      )}
      <div className="px-2.5 pb-2.5 pt-1">{children}</div>
    </div>
  );
}
