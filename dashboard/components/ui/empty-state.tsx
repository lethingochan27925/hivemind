import { Inbox } from "lucide-react";

export function EmptyState({ message }: { message: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-8 text-text-tertiary">
      <Inbox size={24} strokeWidth={1.2} className="mb-2 opacity-40" />
      <span className="text-xs">{message}</span>
    </div>
  );
}
