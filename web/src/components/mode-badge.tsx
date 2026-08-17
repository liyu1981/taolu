import { cn } from "@/lib/utils";

const modeVariants: Record<string, string> = {
  apply: "bg-blue-500/15 text-blue-600 dark:text-blue-400 border-blue-500/30",
  install: "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/30",
  enforce: "bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/30",
};

export function ModeBadge({ mode }: { mode: string }) {
  const cls = modeVariants[mode] ?? "";
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium",
        cls,
      )}
    >
      {mode}
    </span>
  );
}
