import { useState, useMemo } from "react";
import { File, Folder, FolderOpen, ChevronRight, ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";

interface FileNode {
  name: string;
  path: string;
  isDir: boolean;
  children?: FileNode[];
}

function buildTree(files: { path: string }[]): FileNode[] {
  const root: FileNode[] = [];

  for (const f of files) {
    const parts = f.path.split("/");
    let current = root;

    for (let i = 0; i < parts.length; i++) {
      const name = parts[i];
      const isDir = i < parts.length - 1;
      const path = parts.slice(0, i + 1).join("/");

      let existing = current.find((n) => n.name === name && n.isDir === isDir);
      if (!existing) {
        existing = { name, path, isDir, children: isDir ? [] : undefined };
        current.push(existing);
      }
      if (isDir && existing.children) {
        current = existing.children;
      }
    }
  }

  return root;
}

function sortTree(nodes: FileNode[]): FileNode[] {
  return nodes
    .sort((a, b) => {
      if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
      return a.name.localeCompare(b.name);
    })
    .map((n) => (n.children ? { ...n, children: sortTree(n.children) } : n));
}

function FileTreeNode({
  node,
  selected,
  onSelect,
  depth,
}: {
  node: FileNode;
  selected: string;
  onSelect: (path: string) => void;
  depth: number;
}) {
  const [expanded, setExpanded] = useState(true);

  if (node.isDir) {
    return (
      <div>
        <button
          onClick={() => setExpanded(!expanded)}
          className={cn(
            "flex items-center gap-1.5 w-full text-left px-2 py-1 text-xs rounded-md",
            "hover:bg-accent/50 text-muted-foreground hover:text-foreground transition-colors",
          )}
          style={{ paddingLeft: `${depth * 12 + 8}px` }}
        >
          {expanded ? (
            <ChevronDown className="w-3 h-3 shrink-0" />
          ) : (
            <ChevronRight className="w-3 h-3 shrink-0" />
          )}
          {expanded ? (
            <FolderOpen className="w-3.5 h-3.5 shrink-0 text-muted-foreground/70" />
          ) : (
            <Folder className="w-3.5 h-3.5 shrink-0 text-muted-foreground/70" />
          )}
          <span className="truncate">{node.name}</span>
        </button>
        {expanded && node.children && (
          <div>
            {node.children.map((child) => (
              <FileTreeNode
                key={child.path}
                node={child}
                selected={selected}
                onSelect={onSelect}
                depth={depth + 1}
              />
            ))}
          </div>
        )}
      </div>
    );
  }

  return (
    <button
      onClick={() => onSelect(node.path)}
      className={cn(
        "flex items-center gap-1.5 w-full text-left px-2 py-1 text-xs rounded-md transition-colors",
        node.path === selected
          ? "bg-accent text-foreground font-medium"
          : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
      )}
      style={{ paddingLeft: `${depth * 12 + 8}px` }}
    >
      <File className="w-3.5 h-3.5 shrink-0" />
      <span className="truncate">{node.name}</span>
    </button>
  );
}

export function FileTree({
  files,
  selected,
  onSelect,
}: {
  files: { path: string }[];
  selected: string;
  onSelect: (path: string) => void;
}) {
  const tree = useMemo(() => sortTree(buildTree(files)), [files]);

  return (
    <div className="rounded-xl glass-control bg-clip-padding p-1 overflow-auto">
      <div className="px-2 py-1.5 text-xs font-medium text-muted-foreground border-b border-border/30 mb-1">
        Files
      </div>
      {tree.map((node) => (
        <FileTreeNode
          key={node.path}
          node={node}
          selected={selected}
          onSelect={onSelect}
          depth={0}
        />
      ))}
    </div>
  );
}
