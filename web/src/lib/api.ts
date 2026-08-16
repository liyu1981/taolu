import type {
  ContentResponse,
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

export const api = {
  status: () => get<Status>("/api/status"),
  taolus: (params?: {
    query?: string;
    group?: string;
    include?: string;
    tag?: string;
    archived?: boolean;
  }) => {
    const q = new URLSearchParams();
    if (params?.query) q.set("query", params.query);
    if (params?.group) q.set("group", params.group);
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
};
