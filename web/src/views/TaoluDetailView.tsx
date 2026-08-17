import { useState } from "react";
import { useParams, Link, useNavigate } from "@tanstack/react-router";
import { useQuery as useReactQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { TaoluDetail, Version, ContentFile } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ModeBadge } from "@/components/mode-badge";
import { CodeBlock } from "@/components/code-block";
import { DiffList } from "@/components/diff-view";
import { Loading, ErrorBox } from "@/components/status";

export default function TaoluDetailView() {
  const { name } = useParams({ from: "/browse/$name" });

  const detail = useReactQuery({
    queryKey: ["taolu", name],
    queryFn: () => api.taolu(name),
  });
  const history = useReactQuery({
    queryKey: ["history", name],
    queryFn: () => api.history(name),
  });

  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["taolu", name] });
    queryClient.invalidateQueries({ queryKey: ["taolus"] });
    queryClient.invalidateQueries({ queryKey: ["status"] });
  };

  const archiveMut = useMutation({
    mutationFn: () => api.archive(name),
    onSuccess: () => { invalidate(); navigate({ to: "/browse" }); },
  });

  const restoreMut = useMutation({
    mutationFn: () => api.restore(name),
    onSuccess: invalidate,
  });

  if (detail.isLoading) return <Loading label="Loading taolu…" />;
  if (detail.error) return <ErrorBox error={detail.error} />;
  if (!detail.data) return null;

  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between">
        <div>
          <Link to="/browse" className="text-sm text-muted-foreground hover:text-foreground">
            ← Browse
          </Link>
          <h1 className="text-2xl font-semibold tracking-tight mt-1 flex items-center gap-2">
            {detail.data.name}
            {detail.data.archived && <Badge variant="destructive">archived</Badge>}
          </h1>
          <p className="text-sm text-muted-foreground">
            {detail.data.group} · <ModeBadge mode={detail.data.mode} /> · latest{" "}
            <span className="font-mono">{detail.data.latest_version || "—"}</span> ·{" "}
            {detail.data.version_count} version{detail.data.version_count === 1 ? "" : "s"}
          </p>
        </div>
        <div className="flex items-center gap-2 mt-1">
          {detail.data.archived ? (
            <Button
              variant="glass"
              size="sm"
              disabled={restoreMut.isPending}
              onClick={() => restoreMut.mutate()}
            >
              Restore
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="sm"
              disabled={archiveMut.isPending}
              onClick={() => archiveMut.mutate()}
            >
              Archive
            </Button>
          )}
        </div>
      </div>

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="history">History</TabsTrigger>
          <TabsTrigger value="content">Content</TabsTrigger>
          <TabsTrigger value="diff">Diff</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          <OverviewTab detail={detail.data} />
        </TabsContent>
        <TabsContent value="history">
          <HistoryTab
            history={history.data}
            isLoading={history.isLoading}
            error={history.error}
          />
        </TabsContent>
        <TabsContent value="content">
          <ContentTab name={name} history={history.data} historyLoading={history.isLoading} />
        </TabsContent>
        <TabsContent value="diff">
          <DiffTab name={name} history={history.data} historyLoading={history.isLoading} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

function OverviewTab({ detail }: { detail: TaoluDetail }) {
  return (
    <div className="space-y-4">
      {detail.archived && (
        <div className="rounded-md border border-destructive/40 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          This taolu is archived and must not be used until restored.
        </div>
      )}
      <div className="grid gap-4 lg:grid-cols-2">
        <CodeBlock filename="SKILL.md" content={detail.skill} />
        <CodeBlock filename="ACTION.md" content={detail.action} />
      </div>
      {detail.assets.length > 0 && (
        <div>
          <h3 className="text-sm font-medium mb-2">files/ assets</h3>
          <div className="rounded-xl glass-control bg-clip-padding px-4 py-3">
            <ul className="space-y-1 font-mono text-xs">
              {detail.assets.map((a) => (
                <li key={a.path} className="text-muted-foreground">
                  files/{a.path}
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}
    </div>
  );
}

function HistoryTab({
  history,
  isLoading,
  error,
}: {
  history: Version[] | undefined;
  isLoading: boolean;
  error: Error | null;
}) {
  if (isLoading) return <Loading label="Loading history…" />;
  if (error) return <ErrorBox error={error} />;
  if (!history || history.length === 0)
    return <p className="text-sm text-muted-foreground">No versions.</p>;

  return (
    <div className="space-y-2">
      {[...history].reverse().map((v) => (
        <div
          key={v.uuid}
          className="flex items-start gap-4 rounded-xl glass-control bg-clip-padding px-4 py-3"
        >
          <Badge variant="outline" className="font-mono shrink-0">
            {v.label}
          </Badge>
          <div className="min-w-0 flex-1">
            <p className="text-sm break-words">{v.message || "—"}</p>
            <p className="text-xs text-muted-foreground mt-0.5">
              {new Date(v.date).toLocaleString()} · {v.user} · {v.uuid}
            </p>
          </div>
        </div>
      ))}
    </div>
  );
}

function ContentTab({
  name,
  history,
  historyLoading,
}: {
  name: string;
  history: Version[] | undefined;
  historyLoading: boolean;
}) {
  const [version, setVersion] = useState("tip");
  const content = useReactQuery({
    queryKey: ["content", name, version],
    queryFn: () => api.content(name, version === "tip" ? undefined : version),
    enabled: true,
  });

  const versions = [...(history ?? [])].reverse();

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <span className="text-sm text-muted-foreground">Version</span>
        {!historyLoading && versions.length > 0 ? (
          <Select value={version} onValueChange={setVersion}>
            <SelectTrigger className="w-44">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="tip">tip (latest)</SelectItem>
              {versions.map((v) => (
                <SelectItem key={v.uuid} value={v.label}>
                  {v.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : (
          <span className="text-sm">tip (latest)</span>
        )}
      </div>

      {content.isLoading && <Loading label="Loading content…" />}
      {content.error && <ErrorBox error={content.error} />}
      {content.data && (
        <div className="space-y-4">
          {content.data.files.map((f: ContentFile) => (
            <CodeBlock key={f.path} filename={f.path} content={f.content} />
          ))}
        </div>
      )}
    </div>
  );
}

function DiffTab({
  name,
  history,
  historyLoading,
}: {
  name: string;
  history: Version[] | undefined;
  historyLoading: boolean;
}) {
  const versions = [...(history ?? [])].reverse();
  const [base, setBase] = useState("");
  const [target, setTarget] = useState("tip");

  const diff = useReactQuery({
    queryKey: ["diff", name, base, target],
    queryFn: () => api.diff(name, base || undefined, target === "tip" ? "tip" : target),
    enabled: !historyLoading && versions.length > 1,
  });

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">Base</span>
          {!historyLoading && versions.length > 1 ? (
            <Select value={base} onValueChange={setBase}>
              <SelectTrigger className="w-44">
                <SelectValue placeholder="previous" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">previous</SelectItem>
                {versions.map((v) => (
                  <SelectItem key={v.uuid} value={v.label}>
                    {v.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <span className="text-sm">previous</span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">Target</span>
          <Select value={target} onValueChange={setTarget}>
            <SelectTrigger className="w-44">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="tip">tip (latest)</SelectItem>
              {versions.map((v) => (
                <SelectItem key={v.uuid} value={v.label}>
                  {v.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {historyLoading && <Loading label="Loading…" />}
      {!historyLoading && versions.length <= 1 && (
        <p className="text-sm text-muted-foreground">
          Need at least two versions to diff.
        </p>
      )}
      {diff.isLoading && versions.length > 1 && <Loading label="Loading diff…" />}
      {diff.error && <ErrorBox error={diff.error} />}
      {diff.data && (
        <DiffList
          files={diff.data.files}
          empty="No differences between the selected versions."
        />
      )}
    </div>
  );
}
