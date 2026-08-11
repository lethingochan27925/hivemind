import { Inbox } from "lucide-react";

export function EmptyState({ message, hint }: { message: string; hint?: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <Inbox size={26} strokeWidth={1.3} className="mb-2.5 text-text-tertiary opacity-50" />
      <span className="text-[14px] text-text-secondary font-medium">{message}</span>
      {hint && <span className="text-[12px] text-text-tertiary mt-1.5 max-w-sm">{hint}</span>}
    </div>
  );
}
