export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description?: string;
  actions?: React.ReactNode;
}) {
  return (
    <div className="px-4 py-3 border-b border-border flex items-center justify-between">
      <div>
        <h1 className="text-[16px] font-semibold text-text-primary tracking-tight">
          {title}
        </h1>
        {description && (
          <p className="text-[11px] text-text-tertiary mt-0.5">{description}</p>
        )}
      </div>
      {actions}
    </div>
  );
}
