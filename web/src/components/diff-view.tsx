import { Fragment } from "react";
import type { DiffFile } from "@/lib/types";
import { cn } from "@/lib/utils";

function DiffRows({ unified }: { unified: string }) {
  const lines = unified.split("\n");
  return (
    <pre className="p-3 overflow-x-auto text-xs leading-relaxed font-mono whitespace-pre">
      {lines.map((line, i) => {
        let kind = "context" as "add" | "del" | "context";
        let cls = "text-zinc-300";
        if (line.startsWith("+")) {
          kind = "add";
          cls = "bg-emerald-500/15 text-emerald-200";
        } else if (line.startsWith("-")) {
          kind = "del";
          cls = "bg-red-500/15 text-red-200";
        } else if (line.startsWith("@")) {
          cls = "bg-zinc-800 text-zinc-300";
        } else if (line.startsWith("+++") || line.startsWith("---")) {
          cls = "text-zinc-500";
        }
        return (
          <div key={i} className={cn("flex", cls)}>
            <span className="select-none w-4 shrink-0 text-zinc-600">
              {kind === "add" ? "+" : kind === "del" ? "-" : ""}
            </span>
            <span className="whitespace-pre-wrap break-words">{line}</span>
          </div>
        );
      })}
    </pre>
  );
}

export function DiffBlock({ file }: { file: DiffFile }) {
  return (
    <div className="rounded-md border bg-zinc-950 overflow-hidden">
      <div className="border-b border-zinc-800 px-3 py-1.5 font-mono text-xs text-zinc-400">
        {file.path}
      </div>
      <DiffRows unified={file.unified} />
    </div>
  );
}

export function DiffList({
  files,
  empty,
}: {
  files: DiffFile[];
  empty: string;
}) {
  if (files.length === 0) {
    return <p className="text-sm text-muted-foreground">{empty}</p>;
  }
  return (
    <div className="space-y-4">
      {files.map((f) => (
        <DiffBlock key={f.path} file={f} />
      ))}
    </div>
  );
}

export { Fragment };
