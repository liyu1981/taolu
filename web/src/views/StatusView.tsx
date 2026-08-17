import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Loading, ErrorBox } from "@/components/status";

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
    </div>
  );
}
