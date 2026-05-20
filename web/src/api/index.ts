import type {
  RepoInfo,
  Config,
  AuthStatus,
  ClarifyResponse,
  WikiFetchResponse,
  RefineResponse,
  FeatureManifest,
  KnowledgeDocument,
  KnowledgeUploadResponse,
  LocalProject,
  WikiChatRequest,
  WikiChatResponse,
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
  refineSpec: (payload: {
    spec: string;
    issues: ClarifyResponse["issues"];
    answers: Record<string, string>;
  }) =>
    request<RefineResponse>(`${SPEC_CLARIFY}/refine`, {
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
  wiki: {
    projects: () => request<{ projects: LocalProject[] }>("/features/wiki/projects"),
    chat: (payload: WikiChatRequest) =>
      request<WikiChatResponse>("/features/wiki/chat", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    listDocuments: (projectPath: string) =>
      request<{ documents: KnowledgeDocument[] }>(
        `/features/wiki/knowledge/documents?projectPath=${encodeURIComponent(projectPath)}`,
      ),
    upload: async (files: File[], projectPath: string, replaceDuplicates: boolean) => {
      const form = new FormData();
      for (const file of files) {
        form.append("files", file);
      }
      form.append("projectPath", projectPath);
      form.append("replaceDuplicates", String(replaceDuplicates));
      const res = await fetch(`${BASE}/features/wiki/knowledge/upload`, {
        method: "POST",
        body: form,
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(err.error || res.statusText);
      }
      return res.json() as Promise<KnowledgeUploadResponse>;
    },
    deleteDocument: (id: string, projectPath: string) =>
      request<{ ok: boolean }>(
        `/features/wiki/knowledge/document/${id}?projectPath=${encodeURIComponent(projectPath)}`,
        {
          method: "DELETE",
        },
      ),
    listPending: (projectPath: string) =>
      request<{ documents: KnowledgeDocument[] }>(
        `/features/wiki/knowledge/pending?projectPath=${encodeURIComponent(projectPath)}`,
      ),
    approveDocument: (id: string, projectPath: string, approvedBy = "user") =>
      request<{ ok: boolean }>(
        `/features/wiki/knowledge/document/${id}/approve?projectPath=${encodeURIComponent(projectPath)}&approvedBy=${encodeURIComponent(approvedBy)}`,
        { method: "POST" },
      ),
    rejectDocument: (id: string, projectPath: string) =>
      request<{ ok: boolean }>(
        `/features/wiki/knowledge/document/${id}/reject?projectPath=${encodeURIComponent(projectPath)}`,
        { method: "POST" },
      ),
    approveAll: (projectPath: string, approvedBy = "user") =>
      request<{ ok: boolean; count: number }>(
        `/features/wiki/knowledge/approve-all?projectPath=${encodeURIComponent(projectPath)}&approvedBy=${encodeURIComponent(approvedBy)}`,
        { method: "POST" },
      ),
  },
};
