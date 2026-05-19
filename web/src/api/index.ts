import type {
  RepoInfo,
  Config,
  AuthStatus,
  ClarifyResponse,
  WikiFetchResponse,
  FeatureManifest,
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

const SPEC_CLARIFY = "/features/spec-clarify";

export const api = {
  hub: {
    features: () => request<{ features: FeatureManifest[] }>("/hub/features"),
  },
  repo: {
    info: () => request<RepoInfo>("/repo"),
  },
  clarify: (payload: { spec: string; mode: string; wikiContent?: string }) =>
    request<ClarifyResponse>(`${SPEC_CLARIFY}/clarify`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  fetchWiki: (url: string) =>
    request<WikiFetchResponse>(`${SPEC_CLARIFY}/fetch-wiki`, {
      method: "POST",
      body: JSON.stringify({ url }),
    }),
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
