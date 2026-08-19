import { useMemo } from "react";
import hljs from "highlight.js/lib/common";
import { cn } from "@/lib/utils";

// Extensions highlight.js does not register directly but maps cleanly to one of
// the common grammars. hljs handles the rest via its built-in aliases (md, sh,
// yml, rs, py, go, …).
const EXT_LANGUAGE: Record<string, string> = {
  tsx: "typescript",
  mts: "typescript",
  cts: "typescript",
  jsx: "javascript",
  mjs: "javascript",
  cjs: "javascript",
};

function languageFromFilename(filename?: string): string | undefined {
  if (!filename) return undefined;
  const base = filename.split("/").pop() ?? filename;
  const dot = base.lastIndexOf(".");
  if (dot < 0) return undefined;
  const ext = base.slice(dot + 1).toLowerCase();
  const lang = EXT_LANGUAGE[ext] ?? ext;
  return hljs.getLanguage(lang) ? lang : undefined;
}

// Text surface: frosted glass (glass-control) rather than a dark block. Content
// is syntax-highlighted via highlight.js and rendered on the translucent
// material with high-contrast foreground text so it stays legible over the
// ambient backdrop.
export function CodeBlock({
  content,
  filename,
  className,
}: {
  content: string;
  filename?: string;
  className?: string;
}) {
  const { html, language } = useMemo(() => {
    const lang = languageFromFilename(filename);
    if (lang) {
      const result = hljs.highlight(content, {
        language: lang,
        ignoreIllegals: true,
      });
      return { html: result.value, language: lang };
    }
    const result = hljs.highlightAuto(content);
    return { html: result.value, language: result.language };
  }, [content, filename]);

  return (
    <div
      className={cn(
        "rounded-xl glass-control bg-clip-padding text-foreground shadow-sm overflow-hidden",
        className,
      )}
    >
      {filename && (
        <div className="flex items-center justify-between border-b border-border/40 px-3 py-1.5 font-mono text-xs text-muted-foreground">
          <span>{filename}</span>
          {language && (
            <span className="uppercase tracking-wide text-[10px] text-muted-foreground/70">
              {language}
            </span>
          )}
        </div>
      )}
      <pre className="p-3 overflow-x-auto text-xs leading-relaxed font-mono whitespace-pre-wrap break-words text-foreground">
        <code
          className="hljs"
          dangerouslySetInnerHTML={{ __html: html }}
        />
      </pre>
    </div>
  );
}
