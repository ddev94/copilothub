<script setup lang="ts">
import { onMounted, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useProjectStore } from "@/stores/repo";

const route = useRoute();
const router = useRouter();
const projectStore = useProjectStore();

const projectId = computed(() => route.params.projectId as string);
const project = computed(
  () => projectStore.projects.find((p) => p.id === projectId.value) ?? null,
);

const features = [
  {
    id: "spec-clarify",
    name: "Spec Clarify",
    icon: "🔍",
    description:
      "Analyze and clarify spec documents against source code or wiki",
  },
  {
    id: "wiki",
    name: "Wiki",
    icon: "📚",
    description: "Chat and manage project knowledge across local projects",
  },
];

onMounted(async () => {
  await projectStore.fetch();
  if (projectId.value) {
    projectStore.selectProject(projectId.value);
  }
});

function openFeature(featureId: string) {
  router.push(`/projects/${projectId.value}/features/${featureId}`);
}
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <header class="border-b border-border px-6 py-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3">
          <button
            class="text-xs text-muted-foreground hover:text-foreground transition-colors"
            @click="router.push('/')"
          >
            ← Home
          </button>
          <div>
            <h1 class="text-xl font-bold">{{ project?.name ?? "Project" }}</h1>
            <p class="text-xs text-muted-foreground mt-0.5">Project features</p>
          </div>
        </div>
        <button
          class="inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground border border-border rounded-md px-3 py-1.5 transition-colors hover:bg-muted"
          @click="router.push('/settings')"
        >
          Settings
        </button>
      </div>
    </header>

    <main class="px-6 py-8 max-w-3xl mx-auto">
      <div
        v-if="!project"
        class="text-center text-muted-foreground py-12 text-sm"
      >
        Project not found.
      </div>

      <div v-else class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div
          v-for="f in features"
          :key="f.id"
          class="border border-border rounded-lg p-6 flex flex-col gap-3 bg-card hover:border-primary/50 transition-colors cursor-pointer"
          @click="openFeature(f.id)"
        >
          <div class="flex items-center gap-3">
            <span class="text-3xl">{{ f.icon }}</span>
            <p class="font-semibold text-base">{{ f.name }}</p>
          </div>
          <p class="text-sm text-muted-foreground flex-1">
            {{ f.description }}
          </p>
          <span class="text-xs text-primary font-medium mt-1 self-end">
            Open →
          </span>
        </div>
      </div>
    </main>
  </div>
</template>
