import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Check, X } from "lucide-react";
import { Loading, ErrorBox } from "@/components/status";

const toolLabels: Record<string, string> = {
  opencode: "OpenCode",
  claude: "Claude Desktop",
  vscode: "VS Code",
};

export default function StatusView() {
  const { data, isLoading, error } = useQuery({
    queryKey: ["status"],
    queryFn: api.status,
  });

  if (isLoading) return <Loading label="Loading status…" />;
  if (error) return <ErrorBox error={error} />;
  if (!data) return null;

  const cards = [
    { label: "Server", value: `${data.server_name} v${data.server_version}` },
    { label: "Vault path", value: data.vault_path },
    { label: "Project code", value: data.project_code || "—" },
    { label: "Taolus", value: String(data.taolu_count) },
    { label: "Archived", value: String(data.archived_count) },
    { label: "Groups", value: data.groups.length ? data.groups.join(", ") : "—" },
    { label: "taolu-authoring", value: data.authoring || "not seeded" },
    { label: "Uptime", value: data.uptime },
  ];

  const installed = data.installed ?? {};

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">System status</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Vault and server health at a glance.
        </p>
      </div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {cards.map((c) => (
          <Card key={c.label}>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm text-muted-foreground font-medium">
                {c.label}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-sm font-mono break-words">{c.value}</div>
            </CardContent>
          </Card>
        ))}
      </div>

      {Object.keys(installed).length > 0 && (
        <div>
          <h2 className="text-lg font-semibold tracking-tight mb-1">
            Integration status
          </h2>
          <p className="text-sm text-muted-foreground mb-3">
            Global slash-command installation per agent tool (not per-project).
          </p>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {Object.entries(installed).map(([toolID, info]) => (
              <Card key={toolID}>
                <CardContent className="flex items-center gap-3 py-3">
                  {info.installed ? (
                    <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  ) : (
                    <X className="w-4 h-4 text-muted-foreground/50 shrink-0" />
                  )}
                  <div className="min-w-0 flex-1">
                    <div className="text-sm font-medium">
                      {toolLabels[toolID] ?? toolID}
                    </div>
                    <div className="text-xs text-muted-foreground truncate font-mono">
                      {info.installed ? info.path : "not installed"}
                    </div>
                  </div>
                  <Badge variant={info.installed ? "default" : "outline"} className="shrink-0">
                    {info.installed ? "installed" : "missing"}
                  </Badge>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
