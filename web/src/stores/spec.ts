import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { api } from "@/api";
import type { Spec, SpecMeta, UserStory } from "@/types";

export const useSpecStore = defineStore("spec", () => {
  const specsList = ref<SpecMeta[]>([]);
  const spec = ref<Spec | null>(null);
  const loading = ref(false);
  const saving = ref(false);
  const lastSavedAt = ref<Date | null>(null);
  const activeStoryId = ref<string | null>(null);

  const activeStory = computed(
    () =>
      spec.value?.userStories.find((s) => s.id === activeStoryId.value) ?? null,
  );

  let autoSaveTimer: ReturnType<typeof setTimeout> | null = null;

  function scheduleAutoSave() {
    if (autoSaveTimer) clearTimeout(autoSaveTimer);
    autoSaveTimer = setTimeout(() => save(), 2000);
  }

  async function loadList() {
    specsList.value = await api.specs.list();
  }

  async function openSpec(id: string) {
    loading.value = true;
    try {
      spec.value = await api.spec.get(id);
      activeStoryId.value = spec.value.userStories[0]?.id ?? null;
      lastSavedAt.value = null;
    } finally {
      loading.value = false;
    }
  }

  async function openLatest() {
    await loadList();
    if (specsList.value.length > 0) {
      await openSpec(specsList.value[0].id);
    }
  }

  async function createBlank(title?: string) {
    const created = await api.specs.create(title ? { title } : undefined);
    await loadList();
    spec.value = created;
    activeStoryId.value = created.userStories[0]?.id ?? null;
    lastSavedAt.value = null;
    return created;
  }

  async function saveGenerated(generated: Spec) {
    const saved = await api.specs.create(generated);
    await loadList();
    spec.value = saved;
    activeStoryId.value = saved.userStories[0]?.id ?? null;
    lastSavedAt.value = new Date();
    return saved;
  }

  async function save() {
    if (!spec.value) return;
    saving.value = true;
    try {
      spec.value = await api.spec.save(spec.value);
      lastSavedAt.value = new Date();
      await loadList();
    } finally {
      saving.value = false;
    }
  }

  async function deleteSpec(id: string) {
    await api.spec.delete(id);
    await loadList();
    if (spec.value?.id === id) {
      spec.value = null;
      activeStoryId.value = null;
    }
  }

  function updateRequirement(requirement: string) {
    if (!spec.value) return;
    spec.value.requirement = requirement;
    scheduleAutoSave();
  }

  function updateUserStories(stories: UserStory[]) {
    if (!spec.value) return;
    spec.value.userStories = stories;
    scheduleAutoSave();
  }

  function addUserStory(story: Omit<UserStory, "id">) {
    if (!spec.value) return;
    const newStory: UserStory = { id: crypto.randomUUID(), ...story };
    spec.value.userStories.push(newStory);
    activeStoryId.value = newStory.id;
    scheduleAutoSave();
  }

  function updateUserStory(id: string, patch: Partial<UserStory>) {
    if (!spec.value) return;
    const s = spec.value.userStories.find((s) => s.id === id);
    if (s) {
      Object.assign(s, patch);
      scheduleAutoSave();
    }
  }

  function removeUserStory(id: string) {
    if (!spec.value) return;
    spec.value.userStories = spec.value.userStories.filter((s) => s.id !== id);
    if (activeStoryId.value === id) {
      activeStoryId.value = spec.value.userStories[0]?.id ?? null;
    }
    scheduleAutoSave();
  }

  return {
    specsList,
    spec,
    loading,
    saving,
    lastSavedAt,
    activeStoryId,
    activeStory,
    loadList,
    openSpec,
    openLatest,
    createBlank,
    saveGenerated,
    save,
    deleteSpec,
    updateRequirement,
    updateUserStories,
    addUserStory,
    updateUserStory,
    removeUserStory,
  };
});
