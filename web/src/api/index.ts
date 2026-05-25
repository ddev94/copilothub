import type {
  Config,
  AuthStatus,
  ClarifyResponse,
  RefineResponse,
  ToolEvent,
  FeatureManifest,
  KnowledgeDocument,
  KnowledgeUploadResponse,
  LocalProject,
  ProjectRepository,
  WikiChatRequest,
  WikiChatResponse,
  EmbeddingStatus,
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
  projects: {
    list: () => request<{ projects: LocalProject[] }>("/projects"),
    get: (id: string) => request<LocalProject>(`/projects/${id}`),
    create: (payload: { name: string }) =>
      request<LocalProject>("/projects", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    delete: (id: string) =>
      request<{ ok: boolean }>(`/projects/${id}`, { method: "DELETE" }),
    update: (id: string, payload: { name: string }) =>
      request<LocalProject>(`/projects/${id}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      }),
    addRepo: (id: string, repoURL: string, branch?: string, name?: string) =>
      request<ProjectRepository>(`/projects/${id}/repos`, {
        method: "POST",
        body: JSON.stringify({
          repoURL,
          branch: branch || "",
          name: name || "",
        }),
      }),
    removeRepo: (id: string, repoId: string) =>
      request<{ ok: boolean }>(`/projects/${id}/repos/${repoId}`, {
        method: "DELETE",
      }),
    changeRepoBranch: (id: string, repoId: string, branch: string) =>
      request<LocalProject>(`/projects/${id}/repos/${repoId}/change-branch`, {
        method: "POST",
        body: JSON.stringify({ branch }),
      }),
  },
  clarify: (payload: {
    spec: string;
    mode: string;
    wikiContent?: string;
    projectId?: string;
    repoIds?: string[];
  }) =>
    request<ClarifyResponse>(`${SPEC_CLARIFY}/clarify`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  clarifyStream: async (
    payload: {
      spec: string;
      mode: string;
      wikiContent?: string;
      projectId?: string;
      repoIds?: string[];
    },
    onTool: (event: ToolEvent) => void,
    signal?: AbortSignal,
  ): Promise<ClarifyResponse> => {
    const res = await fetch(`${BASE}${SPEC_CLARIFY}/clarify`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "text/event-stream",
      },
      body: JSON.stringify(payload),
      signal,
    });
    if (!res.ok || !res.body) throw new Error(`HTTP ${res.status}`);

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = "";

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });

      // Parse complete SSE messages (delimited by \n\n)
      let boundary: number;
      while ((boundary = buf.indexOf("\n\n")) !== -1) {
        const block = buf.slice(0, boundary);
        buf = buf.slice(boundary + 2);

        let eventType = "message";
        let dataLine = "";
        for (const line of block.split("\n")) {
          if (line.startsWith("event:")) eventType = line.slice(6).trim();
          else if (line.startsWith("data:")) dataLine = line.slice(5).trim();
        }
        if (!dataLine) continue;

        if (eventType === "tool") {
          onTool(JSON.parse(dataLine) as ToolEvent);
        } else if (eventType === "result") {
          return JSON.parse(dataLine) as ClarifyResponse;
        } else if (eventType === "error") {
          throw new Error((JSON.parse(dataLine) as { error: string }).error);
        }
      }
    }
    throw new Error("Stream ended without result");
  },
  refineSpec: (payload: { spec: string; issues: ClarifyResponse["issues"] }) =>
    request<RefineResponse>(`${SPEC_CLARIFY}/refine`, {
      method: "POST",
      body: JSON.stringify(payload),
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
  embedding: {
    check: () => request<EmbeddingStatus>("/embedding/check"),
    stream: () => new EventSource(`${BASE}/embedding/stream`),
  },
  wiki: {
    projects: () =>
      request<{ projects: LocalProject[] }>("/features/wiki/projects"),
    chat: (payload: WikiChatRequest) =>
      request<WikiChatResponse>("/features/wiki/chat", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    getDocumentContent: (docId: string, projectId: string) =>
      request<{
        content: string;
        name: string;
        sourceFile: string;
        isMarkdown: boolean;
      }>(
        `/features/wiki/knowledge/content?docId=${encodeURIComponent(docId)}&projectId=${encodeURIComponent(projectId)}`,
      ),
    listDocuments: (projectId: string) =>
      request<{ documents: KnowledgeDocument[] }>(
        `/features/wiki/knowledge/documents?projectId=${encodeURIComponent(projectId)}`,
      ),
    upload: async (
      files: File[],
      projectId: string,
      replaceDuplicates: boolean,
    ) => {
      const form = new FormData();
      for (const file of files) {
        form.append("files", file);
      }
      form.append("projectId", projectId);
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
    deleteDocument: (id: string, projectId: string) =>
      request<{ ok: boolean }>(
        `/features/wiki/knowledge/document/${id}?projectId=${encodeURIComponent(projectId)}`,
        {
          method: "DELETE",
        },
      ),
    listPending: (projectId: string) =>
      request<{ documents: KnowledgeDocument[] }>(
        `/features/wiki/knowledge/pending?projectId=${encodeURIComponent(projectId)}`,
      ),
    approveDocument: (id: string, projectId: string, approvedBy = "user") =>
      request<{ ok: boolean }>(
        `/features/wiki/knowledge/document/${id}/approve?projectId=${encodeURIComponent(projectId)}&approvedBy=${encodeURIComponent(approvedBy)}`,
        { method: "POST" },
      ),
    rejectDocument: (id: string, projectId: string) =>
      request<{ ok: boolean }>(
        `/features/wiki/knowledge/document/${id}/reject?projectId=${encodeURIComponent(projectId)}`,
        { method: "POST" },
      ),
    approveAll: (projectId: string, approvedBy = "user") =>
      request<{ ok: boolean; count: number }>(
        `/features/wiki/knowledge/approve-all?projectId=${encodeURIComponent(projectId)}&approvedBy=${encodeURIComponent(approvedBy)}`,
        { method: "POST" },
      ),
  },
};
