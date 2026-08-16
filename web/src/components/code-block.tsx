import { cn } from "@/lib/utils";

export function CodeBlock({
  content,
  filename,
  className,
}: {
  content: string;
  filename?: string;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "rounded-md border bg-zinc-950 text-zinc-100 overflow-hidden",
        className,
      )}
    >
      {filename && (
        <div className="border-b border-zinc-800 px-3 py-1.5 font-mono text-xs text-zinc-400">
          {filename}
        </div>
      )}
      <pre className="p-3 overflow-x-auto text-xs leading-relaxed font-mono whitespace-pre-wrap break-words">
        <code>{content}</code>
      </pre>
    </div>
  );
}
