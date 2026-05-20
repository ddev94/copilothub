import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { api } from "@/api";
import type { LocalProject } from "@/types";

export const useProjectStore = defineStore("project", () => {
  const projects = ref<LocalProject[]>([]);
  const selectedProjectId = ref<string | null>(null);
  const loading = ref(false);
  const error = ref<string | null>(null);

  const selectedProject = computed(
    () => projects.value.find((p) => p.id === selectedProjectId.value) ?? null,
  );

  async function fetch() {
    loading.value = true;
    error.value = null;
    try {
      const res = await api.projects.list();
      projects.value = res.projects;
      // Auto-select first project if none selected
      if (!selectedProjectId.value && res.projects.length > 0) {
        selectedProjectId.value = res.projects[0].id;
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Failed to load projects";
    } finally {
      loading.value = false;
    }
  }

  function selectProject(id: string) {
    selectedProjectId.value = id;
  }

  async function create(name: string) {
    const project = await api.projects.create({ name });
    await fetch();
    selectedProjectId.value = project.id;
    return project;
  }

  async function remove(id: string) {
    await api.projects.delete(id);
    if (selectedProjectId.value === id) {
      selectedProjectId.value = null;
    }
    await fetch();
  }

  return {
    projects,
    selectedProject,
    selectedProjectId,
    loading,
    error,
    fetch,
    selectProject,
    create,
    remove,
  };
});

// Backward compatibility alias
export const useRepoStore = useProjectStore;
