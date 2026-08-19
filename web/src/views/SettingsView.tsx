import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ErrorBox, Loading } from "@/components/status";

export default function SettingsView() {
  const queryClient = useQueryClient();
  const [domain, setDomain] = useState("");
  const config = useQuery({
    queryKey: ["config"],
    queryFn: api.config,
  });

  useEffect(() => {
    if (config.data) setDomain(config.data.user_domain);
  }, [config.data]);

  const save = useMutation({
    mutationFn: () => api.setConfig({ user_domain: domain }),
    onSuccess: (next) => {
      setDomain(next.user_domain);
      queryClient.setQueryData(["config"], next);
      queryClient.invalidateQueries({ queryKey: ["status"] });
    },
  });

  if (config.isLoading) return <Loading label="Loading settings…" />;
  if (config.error) return <ErrorBox error={config.error} />;

  const valid = /^@[a-z0-9]+(?:-[a-z0-9]+)*$/.test(domain);
  const unchanged = domain === (config.data?.user_domain ?? "");

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Settings</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Configure how shorthand taolu IDs resolve in this vault.
        </p>
      </div>

      <Card className="max-w-2xl">
        <CardHeader>
          <CardTitle>Your domain</CardTitle>
          <p className="text-sm text-muted-foreground" id="domain-description">
            Use this domain when a taolu ID omits its domain. It must start with @,
            such as @liyu1981.
          </p>
        </CardHeader>
        <CardContent>
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              if (valid && !unchanged) save.mutate();
            }}
          >
            <label className="block space-y-1.5">
              <span className="text-sm font-medium">Default domain</span>
              <input
                id="default-domain"
                value={domain}
                onChange={(event) => setDomain(event.target.value)}
                placeholder="@liyu1981"
                className="h-10 w-full rounded-lg glass-control bg-clip-padding px-3 text-sm text-popover-foreground placeholder:text-muted-foreground shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
                aria-invalid={domain.length > 0 && !valid}
                aria-describedby="domain-description"
              />
            </label>
            {domain.length > 0 && !valid && (
              <p className="text-sm text-destructive">
                Enter @ followed by lowercase letters, numbers, or single hyphens.
              </p>
            )}
            {save.error && <ErrorBox error={save.error} />}
            {save.isSuccess && (
              <p className="text-sm text-emerald-600">Domain saved.</p>
            )}
            <Button type="submit" disabled={!valid || unchanged || save.isPending}>
              {save.isPending ? "Saving…" : "Save domain"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
