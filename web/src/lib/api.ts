import type {
  ContentResponse,
  Config,
  DiffResponse,
  Status,
  TaoluDetail,
  TaoluItem,
  Version,
} from "./types";

async function get<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) msg = body.error;
    } catch {
      /* keep statusText */
    }
    throw new Error(msg);
  }
  return (await res.json()) as T;
}

async function post<T>(url: string, body?: Record<string, string>): Promise<T> {
  const res = await fetch(url, {
    method: "POST",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const b = (await res.json()) as { error?: string };
      if (b.error) msg = b.error;
    } catch {
      /* keep statusText */
    }
    throw new Error(msg);
  }
  return (await res.json()) as T;
}

async function put<T>(url: string, body: object): Promise<T> {
  const res = await fetch(url, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const b = (await res.json()) as { error?: string };
      if (b.error) msg = b.error;
    } catch {
      /* keep statusText */
    }
    throw new Error(msg);
  }
  return (await res.json()) as T;
}

export interface MutationResult {
  status: string;
  group: string;
  name: string;
}

export const api = {
  status: () => get<Status>("/api/status"),
  config: () => get<Config>("/api/config"),
  taolus: (params?: {
    query?: string;
    group?: string;
    domain?: string;
    include?: string;
    tag?: string;
    archived?: boolean;
  }) => {
    const q = new URLSearchParams();
    if (params?.query) q.set("query", params.query);
    if (params?.group) q.set("group", params.group);
    if (params?.domain) q.set("domain", params.domain);
    if (params?.include) q.set("include", params.include);
    if (params?.tag) q.set("tag", params.tag);
    if (params?.archived) q.set("archived", "true");
    const s = q.toString();
    return get<TaoluItem[]>(`/api/taolus${s ? `?${s}` : ""}`);
  },
  taolu: (name: string) =>
    get<TaoluDetail>(`/api/taolus/${encodeURIComponent(name)}`),
  history: (name: string) =>
    get<Version[]>(`/api/taolus/${encodeURIComponent(name)}/history`),
  content: (name: string, version?: string) => {
    const v = version ? `?version=${encodeURIComponent(version)}` : "";
    return get<ContentResponse>(
      `/api/taolus/${encodeURIComponent(name)}/content${v}`,
    );
  },
  diff: (name: string, a?: string, b?: string) => {
    const q = new URLSearchParams();
    if (a) q.set("a", a);
    q.set("b", b ?? "tip");
    return get<DiffResponse>(
      `/api/taolus/${encodeURIComponent(name)}/diff?${q.toString()}`,
    );
  },
  archive: (name: string, message?: string) =>
    post<MutationResult>(
      `/api/taolus/${encodeURIComponent(name)}/archive`,
      message ? { message } : undefined,
    ),
  restore: (name: string, message?: string) =>
    post<MutationResult>(
      `/api/taolus/${encodeURIComponent(name)}/restore`,
      message ? { message } : undefined,
    ),
  setConfig: (config: Config) => put<Config>("/api/config", config),
};
