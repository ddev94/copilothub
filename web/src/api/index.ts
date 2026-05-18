import type {
  Spec,
  SpecMeta,
  RepoInfo,
  Config,
  AuthStatus,
  ClarifyResponse,
} from "@/types";

const BASE = "/api";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { "Content-Type": "application/json", ...init?.headers },
    ...init,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}

export const api = {
  repo: {
    info: () => request<RepoInfo>("/repo"),
  },
  specs: {
    list: () => request<SpecMeta[]>("/specs"),
    create: (data?: Partial<Spec>) =>
      request<Spec>("/specs", {
        method: "POST",
        body: JSON.stringify(data ?? {}),
      }),
  },
  spec: {
    get: (id: string) => request<Spec>(`/spec/${id}`),
    save: (spec: Spec) =>
      request<Spec>(`/spec/${spec.id}`, {
        method: "PUT",
        body: JSON.stringify(spec),
      }),
    delete: (id: string) =>
      request<{ ok: boolean }>(`/spec/${id}`, { method: "DELETE" }),
  },
  ai: {
    suggest: (requirement: string, context: string) =>
      request<{ content: string }>("/ai/suggest", {
        method: "POST",
        body: JSON.stringify({ requirement, context }),
      }),
    clarify: (requirement: string) =>
      request<ClarifyResponse>("/ai/clarify", {
        method: "POST",
        body: JSON.stringify({ requirement }),
      }),
    generateSpec: (payload: {
      title: string;
      requirement: string;
      clarification?: string;
    }) =>
      request<Spec>("/ai/generate-spec", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
  },
  config: {
    get: () => request<Config>("/config"),
    save: (cfg: Config) =>
      request<{ ok: boolean }>("/config", {
        method: "PUT",
        body: JSON.stringify(cfg),
      }),
  },
  auth: {
    status: () => request<AuthStatus>("/auth/status"),
  },
};
