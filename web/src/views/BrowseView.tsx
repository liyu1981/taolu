import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import type { TaoluItem } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ModeBadge } from "@/components/mode-badge";
import { Loading, ErrorBox } from "@/components/status";
import { cn } from "@/lib/utils";

const MODES = ["apply", "install", "enforce"] as const;

export default function BrowseView() {
  const [query, setQuery] = useState("");
  const [group, setGroup] = useState("all");
  const [mode, setMode] = useState("all");
  const [showArchived, setShowArchived] = useState(false);

  const { data, isLoading, error } = useQuery({
    queryKey: ["taolus", query, group, mode, showArchived],
    queryFn: () =>
      api.taolus({
        query: query || undefined,
        group: group === "all" ? undefined : group,
        include: mode === "all" ? undefined : mode,
        archived: showArchived,
      }),
  });

  const groups = useQuery({
    queryKey: ["status"],
    queryFn: api.status,
    select: (s) => s.groups,
  });

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-semibold">Browse taolus</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Search and inspect the taolus in the vault.
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search name / description / tags…"
          className="h-9 w-64 rounded-md border border-input bg-background px-3 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
        />
        <Select value={group} onValueChange={setGroup}>
          <SelectTrigger className="w-40">
            <SelectValue placeholder="Group" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All groups</SelectItem>
            {(groups.data ?? []).map((g) => (
              <SelectItem key={g} value={g}>
                {g}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={mode} onValueChange={setMode}>
          <SelectTrigger className="w-40">
            <SelectValue placeholder="Mode" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All modes</SelectItem>
            {MODES.map((m) => (
              <SelectItem key={m} value={m}>
                {m}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button
          variant={showArchived ? "secondary" : "outline"}
          size="sm"
          onClick={() => setShowArchived((v) => !v)}
        >
          Show archived
        </Button>
      </div>

      {isLoading && <Loading label="Loading taolus…" />}
      {error && <ErrorBox error={error} />}
      {data && data.length === 0 && !isLoading && (
        <p className="text-sm text-muted-foreground">No taolus match.</p>
      )}
      {data && data.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Group</TableHead>
              <TableHead>Mode</TableHead>
              <TableHead>Version</TableHead>
              <TableHead>Description</TableHead>
              <TableHead>Tags</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.map((t: TaoluItem) => (
              <TableRow key={t.name}>
                <TableCell className="font-medium">
                  <Link
                    to="/taolu/$name"
                    params={{ name: t.name }}
                    className="text-primary underline-offset-4 hover:underline"
                  >
                    {t.name}
                  </Link>
                  {t.archived && (
                    <Badge variant="destructive" className="ml-2">
                      archived
                    </Badge>
                  )}
                </TableCell>
                <TableCell>{t.group}</TableCell>
                <TableCell>
                  <ModeBadge mode={t.mode} />
                </TableCell>
                <TableCell className={cn("font-mono text-xs")}>
                  {t.latest_version}
                </TableCell>
                <TableCell className="max-w-xs text-muted-foreground truncate">
                  {t.description}
                </TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {t.tags.map((tag) => (
                      <Badge key={tag} variant="outline">
                        {tag}
                      </Badge>
                    ))}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
