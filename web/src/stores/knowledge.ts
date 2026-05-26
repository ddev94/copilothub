import { defineStore } from "pinia";
import { ref } from "vue";
import { api } from "@/api";
import type { KnowledgeDocument, WikiChatTurn, WikiSessionMeta } from "@/types";

const THREADS_STORAGE_KEY = "wiki_threads_v1";
const SESSIONS_STORAGE_KEY = "wiki_sessions_v1";

export const useKnowledgeStore = defineStore("knowledge", () => {
  const documents = ref<KnowledgeDocument[]>([]);
  const loading = ref(false);
  const uploading = ref(false);
  const error = ref<string | null>(null);
  const threads = ref<Record<string, WikiChatTurn[]>>({});
  const sessions = ref<Record<string, WikiSessionMeta>>({});

  function threadKey(projectId: string, sectionKey: string) {
    return `${projectId}::${sectionKey}`;
  }

  function persistThreads() {
    localStorage.setItem(THREADS_STORAGE_KEY, JSON.stringify(threads.value));
  }

  function persistSessions() {
    localStorage.setItem(SESSIONS_STORAGE_KEY, JSON.stringify(sessions.value));
  }

  function loadPersistedData() {
    try {
      const savedThreads = localStorage.getItem(THREADS_STORAGE_KEY);
      if (savedThreads) threads.value = JSON.parse(savedThreads);
      const savedSessions = localStorage.getItem(SESSIONS_STORAGE_KEY);
      if (savedSessions) sessions.value = JSON.parse(savedSessions);

      for (const key of Object.keys(threads.value)) {
        if (sessions.value[key]) continue;
        const [projectId, sectionKey] = key.split("::");
        if (!projectId || !sectionKey) continue;
        sessions.value[key] = { projectId, sectionKey, title: sectionKey };
      }
      persistSessions();
    } catch {
      threads.value = {};
      sessions.value = {};
    }
  }

  function ensureSession(projectId: string, sectionKey: string) {
    const key = threadKey(projectId, sectionKey);
    if (!sessions.value[key]) {
      sessions.value[key] = { projectId, sectionKey, title: sectionKey };
      persistSessions();
    }
  }

  function getThread(projectId: string, sectionKey: string): WikiChatTurn[] {
    return threads.value[threadKey(projectId, sectionKey)] || [];
  }

  function appendTurn(
    projectId: string,
    sectionKey: string,
    turn: WikiChatTurn,
  ) {
    const key = threadKey(projectId, sectionKey);
    if (!threads.value[key]) threads.value[key] = [];
    threads.value[key].push(turn);
    ensureSession(projectId, sectionKey);
    persistThreads();
  }

  function clearThread(projectId: string, sectionKey: string) {
    delete threads.value[threadKey(projectId, sectionKey)];
    persistThreads();
  }

  function renameSession(projectId: string, sectionKey: string, title: string) {
    const key = threadKey(projectId, sectionKey);
    ensureSession(projectId, sectionKey);
    sessions.value[key].title = title.trim() || sectionKey;
    persistSessions();
  }

  function listSessions(projectId: string): WikiSessionMeta[] {
    return Object.values(sessions.value).filter(
      (s) => s.projectId === projectId,
    );
  }

  async function loadDocuments(projectId: string) {
    if (!projectId) {
      documents.value = [];
      return;
    }
    loading.value = true;
    error.value = null;
    try {
      const docsRes = await api.wiki.listDocuments(projectId);
      documents.value = docsRes.documents;
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Load knowledge failed";
    } finally {
      loading.value = false;
    }
  }

  async function refreshDocuments(projectId: string) {
    if (!projectId) return;
    try {
      const docsRes = await api.wiki.listDocuments(projectId);
      documents.value = docsRes.documents;
    } catch {
      /* silent */
    }
  }

  async function uploadFiles(
    files: File[],
    replaceDuplicates: boolean,
    projectId: string,
  ) {
    if (!projectId || files.length === 0) return;
    uploading.value = true;
    error.value = null;
    try {
      const res = await api.wiki.upload(files, projectId, replaceDuplicates);
      const failed = res.results.filter((r) => !r.ok);
      if (failed.length > 0) {
        error.value = `Upload failed: ${failed.map((f) => `${f.file}${f.message ? ` (${f.message})` : ""}`).join(", ")}`;
      }
      await refreshDocuments(projectId);
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Upload knowledge failed";
    } finally {
      uploading.value = false;
    }
  }

  async function deleteDocument(id: string, projectId: string) {
    error.value = null;
    documents.value = documents.value.filter((d) => d.id !== id);
    try {
      await api.wiki.deleteDocument(id, projectId);
      await refreshDocuments(projectId);
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Delete knowledge failed";
      await refreshDocuments(projectId);
    }
  }

  loadPersistedData();

  return {
    documents,
    loading,
    uploading,
    error,
    threads,
    sessions,
    loadDocuments,
    refreshDocuments,
    uploadFiles,
    deleteDocument,
    getThread,
    appendTurn,
    clearThread,
    renameSession,
    listSessions,
    ensureSession,
  };
});
