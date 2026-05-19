import { defineStore } from "pinia";
import { ref } from "vue";
import { api } from "@/api";
import type { KnowledgeDocument, LocalProject, WikiChatTurn, WikiSessionMeta } from "@/types";

const THREADS_STORAGE_KEY = "wiki_threads_v1";
const SESSIONS_STORAGE_KEY = "wiki_sessions_v1";

export const useKnowledgeStore = defineStore("knowledge", () => {
  const projects = ref<LocalProject[]>([]);
  const selectedProjectPath = ref("");
  const documents = ref<KnowledgeDocument[]>([]);
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
      if (savedThreads) {
        threads.value = JSON.parse(savedThreads);
      }
      const savedSessions = localStorage.getItem(SESSIONS_STORAGE_KEY);
      if (savedSessions) {
        sessions.value = JSON.parse(savedSessions);
      }

      for (const key of Object.keys(threads.value)) {
        if (sessions.value[key]) continue;
        const [projectPath, sectionKey] = key.split("::");
        if (!projectPath || !sectionKey) continue;
        sessions.value[key] = {
          projectPath,
          sectionKey,
          title: sectionKey,
        };
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
      sessions.value[key] = {
        projectPath,
        sectionKey,
        title: sectionKey,
      };
      persistSessions();
    }
  }

  function getThread(projectPath: string, sectionKey: string): WikiChatTurn[] {
    const key = threadKey(projectPath, sectionKey);
    return threads.value[key] || [];
  }

  function appendTurn(projectPath: string, sectionKey: string, turn: WikiChatTurn) {
    const key = threadKey(projectPath, sectionKey);
    if (!threads.value[key]) {
      threads.value[key] = [];
    }
    threads.value[key].push(turn);
    ensureSession(projectPath, sectionKey);
    persistThreads();
  }

  function clearThread(projectPath: string, sectionKey: string) {
    const key = threadKey(projectPath, sectionKey);
    delete threads.value[key];
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

  async function loadProjects() {
    error.value = null;
    try {
      const res = await api.wiki.projects();
      projects.value = res.projects;
      if (!selectedProjectPath.value && res.projects.length > 0) {
        selectedProjectPath.value = res.projects[0].path;
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Load projects failed";
    }
  }

  async function loadDocuments() {
    if (!selectedProjectPath.value) {
      documents.value = [];
      return;
    }
    loading.value = true;
    error.value = null;
    try {
      const res = await api.wiki.listDocuments(selectedProjectPath.value);
      documents.value = res.documents;
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Load knowledge failed";
    } finally {
      loading.value = false;
    }
  }

  async function uploadFiles(files: File[], replaceDuplicates: boolean) {
    if (!selectedProjectPath.value || files.length === 0) return;
    uploading.value = true;
    error.value = null;
    try {
      const res = await api.wiki.upload(files, selectedProjectPath.value, replaceDuplicates);
      const failed = res.results.filter((r) => !r.ok);
      if (failed.length > 0) {
        error.value = `Upload failed: ${failed.map((f) => `${f.file}${f.message ? ` (${f.message})` : ""}`).join(", ")}`;
      }
      await loadDocuments();
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Upload knowledge failed";
    } finally {
      uploading.value = false;
    }
  }

  async function deleteDocument(id: string) {
    if (!selectedProjectPath.value) return;
    error.value = null;
    try {
      await api.wiki.deleteDocument(id, selectedProjectPath.value);
      await loadDocuments();
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Delete knowledge failed";
    }
  }

  function setSelectedProject(path: string) {
    selectedProjectPath.value = path;
  }

  loadPersistedData();

  return {
    projects,
    selectedProjectPath,
    documents,
    loading,
    uploading,
    error,
    threads,
    sessions,
    loadProjects,
    loadDocuments,
    uploadFiles,
    deleteDocument,
    setSelectedProject,
    getThread,
    appendTurn,
    clearThread,
    renameSession,
    listSessions,
    ensureSession,
  };
});
