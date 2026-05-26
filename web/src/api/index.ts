import type {
  Config,
  AuthStatus,
  ClarifyIssue,
  ClarifyResponse,
  ToolEvent,
  FeatureManifest,
  KnowledgeDocument,
  KnowledgeUploadResponse,
  LocalProject,
  ProjectRepository,
  RepoIndexStatus,
  WikiChatRequest,
  WikiChatResponse,
  EmbeddingStatus,
  WikiThinkingEvent,
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
    indexRepo: (id: string, repoId: string) =>
      request<{ status: string }>(`/projects/${id}/repos/${repoId}/index`, {
        method: "POST",
      }),
    indexRepoStatus: (id: string, repoId: string) =>
      request<RepoIndexStatus>(`/projects/${id}/repos/${repoId}/index-status`),
    deleteRepoIndex: (id: string, repoId: string) =>
      request<{ ok: boolean }>(`/projects/${id}/repos/${repoId}/index`, {
        method: "DELETE",
      }),
  },
  clarify: (payload: {
    spec: string;
    mode: string;
    wikiContent?: string;
    projectId?: string;
    repoIds?: string[];
    model?: string;
  }) =>
    request<ClarifyResponse>(`${SPEC_CLARIFY}/clarify`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  clarifyStream: async (
    payload: {
      spec: string;
      mode: string;
      projectId?: string;
      repoIds?: string[];
      model?: string;
    },
    callbacks: {
      onStart?: (totalFiles: number) => void;
      onScanning?: (file: string, language: string, index: number, total: number) => void;
      onIssues?: (file: string, issues: ClarifyIssue[]) => void;
      onDone?: (totalIssues: number, sessionId?: string) => void;
    },
  ): Promise<void> => {
    const res = await fetch(`${BASE}${SPEC_CLARIFY}/clarify-stream`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    if (!res.ok || !res.body) throw new Error(`HTTP ${res.status}`);

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = "";

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });

      let boundary: number;
      while ((boundary = buf.indexOf("\n\n")) !== -1) {
        const block = buf.slice(0, boundary);
        buf = buf.slice(boundary + 2);

        let eventType = "";
        let dataLine = "";
        for (const line of block.split("\n")) {
          if (line.startsWith("event:")) eventType = line.slice(6).trim();
          else if (line.startsWith("data:")) dataLine = line.slice(5).trim();
        }
        if (!dataLine || !eventType) continue;

        const data = JSON.parse(dataLine);
        switch (eventType) {
          case "start":
            callbacks.onStart?.(data.totalFiles);
            break;
          case "scanning":
            callbacks.onScanning?.(data.file, data.language ?? "", data.index, data.total);
            break;
          case "issues":
            callbacks.onIssues?.(data.file, data.issues ?? []);
            break;
          case "done":
            callbacks.onDone?.(data.totalIssues, data.sessionId ?? undefined);
            break;
          case "error":
            throw new Error(data.error);
        }
      }
    }
  },
  clarifyChat: async (
    payload: {
      sessionId: string;
      message: string;
      projectId?: string;
      repoIds?: string[];
      model?: string;
    },
    onTool: (event: ToolEvent) => void,
  ): Promise<string> => {
    const res = await fetch(`${BASE}${SPEC_CLARIFY}/chat`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    if (!res.ok || !res.body) throw new Error(`HTTP ${res.status}`);

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = "";

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });

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
        } else if (eventType === "message") {
          return (JSON.parse(dataLine) as { content: string }).content;
        } else if (eventType === "error") {
          throw new Error((JSON.parse(dataLine) as { error: string }).error);
        }
      }
    }
    throw new Error("Stream ended without response");
  },
  config: {
    get: () => request<Config>("/config"),
    save: (cfg: Config) =>
      request<{ ok: boolean }>("/config", {
        method: "PUT",
        body: JSON.stringify(cfg),
      }),
  },
  models: {
    list: () => request<{ models: string[]; current: string }>("/models"),
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
    ingestProgress: () =>
      request<{
        state: "idle" | "running" | "completed" | "failed" | "converting" | "graphing";
        docId?: string;
        fileName?: string;
        message: string;
        chunksDone: number;
        chunksTotal: number;
        percent: number;
      }>("/features/wiki/knowledge/ingest-progress"),
    chat: (payload: WikiChatRequest) =>
      request<WikiChatResponse>("/features/wiki/chat", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    chatStream: async (
      payload: WikiChatRequest,
      handlers: {
        onStep: (event: WikiThinkingEvent) => void;
        onFinal: (result: WikiChatResponse) => void;
      },
    ): Promise<void> => {
      const res = await fetch(`${BASE}/features/wiki/chat/stream`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "text/event-stream",
        },
        body: JSON.stringify(payload),
      });
      if (!res.ok || !res.body) throw new Error(`HTTP ${res.status}`);

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });

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

          if (eventType === "step") {
            handlers.onStep(JSON.parse(dataLine) as WikiThinkingEvent);
          } else if (eventType === "final") {
            handlers.onFinal(JSON.parse(dataLine) as WikiChatResponse);
            return;
          } else if (eventType === "error") {
            throw new Error((JSON.parse(dataLine) as { error: string }).error);
          }
        }
      }
      throw new Error("Stream ended without final response");
    },
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
  },
};
