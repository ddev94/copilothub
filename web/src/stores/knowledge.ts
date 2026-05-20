import { defineStore } from "pinia";
import { ref } from "vue";
import { api } from "@/api";
import type { KnowledgeDocument, WikiChatTurn, WikiSessionMeta } from "@/types";

const THREADS_STORAGE_KEY = "wiki_threads_v1";
const SESSIONS_STORAGE_KEY = "wiki_sessions_v1";

export const useKnowledgeStore = defineStore("knowledge", () => {
  const documents = ref<KnowledgeDocument[]>([]);
  const pendingDocuments = ref<KnowledgeDocument[]>([]);
  const loading = ref(false);
  const uploading = ref(false);
  const error = ref<string | null>(null);
  const threads = ref<Record<string, WikiChatTurn[]>>({});
  const sessions = ref<Record<string, WikiSessionMeta>>({});

  function threadKey(projectPath: string, sectionKey: string) {
    return `${projectPath}::${sectionKey}`;
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
        const [projectPath, sectionKey] = key.split("::");
        if (!projectPath || !sectionKey) continue;
        sessions.value[key] = { projectPath, sectionKey, title: sectionKey };
      }
      persistSessions();
    } catch {
      threads.value = {};
      sessions.value = {};
    }
  }

  function ensureSession(projectPath: string, sectionKey: string) {
    const key = threadKey(projectPath, sectionKey);
    if (!sessions.value[key]) {
      sessions.value[key] = { projectPath, sectionKey, title: sectionKey };
      persistSessions();
    }
  }

  function getThread(projectPath: string, sectionKey: string): WikiChatTurn[] {
    return threads.value[threadKey(projectPath, sectionKey)] || [];
  }

  function appendTurn(projectPath: string, sectionKey: string, turn: WikiChatTurn) {
    const key = threadKey(projectPath, sectionKey);
    if (!threads.value[key]) threads.value[key] = [];
    threads.value[key].push(turn);
    ensureSession(projectPath, sectionKey);
    persistThreads();
  }

  function clearThread(projectPath: string, sectionKey: string) {
    delete threads.value[threadKey(projectPath, sectionKey)];
    persistThreads();
  }

  function renameSession(projectPath: string, sectionKey: string, title: string) {
    const key = threadKey(projectPath, sectionKey);
    ensureSession(projectPath, sectionKey);
    sessions.value[key].title = title.trim() || sectionKey;
    persistSessions();
  }

  function listSessions(projectPath: string): WikiSessionMeta[] {
    return Object.values(sessions.value).filter((s) => s.projectPath === projectPath);
  }

  async function loadDocuments(projectPath: string) {
    if (!projectPath) { documents.value = []; pendingDocuments.value = []; return; }
    loading.value = true;
    error.value = null;
    try {
      const [docsRes, pendingRes] = await Promise.all([
        api.wiki.listDocuments(projectPath),
        api.wiki.listPending(projectPath),
      ]);
      documents.value = docsRes.documents;
      pendingDocuments.value = pendingRes.documents;
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Load knowledge failed";
    } finally {
      loading.value = false;
    }
  }

  async function uploadFiles(files: File[], replaceDuplicates: boolean, projectPath: string) {
    if (!projectPath || files.length === 0) return;
    uploading.value = true;
    error.value = null;
    try {
      const res = await api.wiki.upload(files, projectPath, replaceDuplicates);
      const failed = res.results.filter((r) => !r.ok);
      if (failed.length > 0) {
        error.value = `Upload failed: ${failed.map((f) => `${f.file}${f.message ? ` (${f.message})` : ""}`).join(", ")}`;
      }
      await loadDocuments(projectPath);
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Upload knowledge failed";
    } finally {
      uploading.value = false;
    }
  }

  async function deleteDocument(id: string, projectPath: string) {
    error.value = null;
    try {
      await api.wiki.deleteDocument(id, projectPath);
      await loadDocuments(projectPath);
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Delete knowledge failed";
    }
  }

  async function approveDocument(id: string, projectPath: string) {
    error.value = null;
    try {
      await api.wiki.approveDocument(id, projectPath);
      await loadDocuments(projectPath);
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Approve failed";
    }
  }

  async function rejectDocument(id: string, projectPath: string) {
    error.value = null;
    try {
      await api.wiki.rejectDocument(id, projectPath);
      await loadDocuments(projectPath);
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Reject failed";
    }
  }

  async function approveAll(projectPath: string) {
    error.value = null;
    try {
      await api.wiki.approveAll(projectPath);
      await loadDocuments(projectPath);
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Approve all failed";
    }
  }

  loadPersistedData();

  return {
    documents,
    pendingDocuments,
    loading,
    uploading,
    error,
    threads,
    sessions,
    loadDocuments,
    uploadFiles,
    deleteDocument,
    approveDocument,
    rejectDocument,
    approveAll,
    getThread,
    appendTurn,
    clearThread,
    renameSession,
    listSessions,
    ensureSession,
  };
});
