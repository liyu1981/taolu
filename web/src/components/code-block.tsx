import { cn } from "@/lib/utils";

// Text surface: frosted glass (glass-control) rather than a dark block. Content
// is rendered as raw text on the translucent material with high-contrast
// foreground text so it stays legible over the ambient backdrop.
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
        "rounded-xl glass-control bg-clip-padding text-foreground shadow-sm overflow-hidden",
        className,
      )}
    >
      {filename && (
        <div className="border-b border-border/40 px-3 py-1.5 font-mono text-xs text-muted-foreground">
          {filename}
        </div>
      )}
      <pre className="p-3 overflow-x-auto text-xs leading-relaxed font-mono whitespace-pre-wrap break-words text-foreground">
        <code>{content}</code>
      </pre>
    </div>
  );
}