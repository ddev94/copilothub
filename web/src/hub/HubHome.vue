<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useProjectStore } from "@/stores/repo";

const router = useRouter();
const projectStore = useProjectStore();
const showCreateProject = ref(false);
const newProjectName = ref("");
const creating = ref(false);

onMounted(async () => {
  projectStore.fetch();
});

async function createProject() {
  if (!newProjectName.value.trim()) return;
  creating.value = true;
  try {
    const p = await projectStore.create(newProjectName.value.trim());
    showCreateProject.value = false;
    newProjectName.value = "";
    router.push(`/projects/${p.id}`);
  } catch (e) {
    console.error(e);
  } finally {
    creating.value = false;
  }
}

async function deleteProject(id: string) {
  if (!confirm("Bạn có chắc muốn xóa project này?")) return;
  await projectStore.remove(id);
}

function openProject(id: string) {
  router.push(`/projects/${id}`);
}

function projectInitial(name: string) {
  return name.charAt(0).toUpperCase();
}

const colors = [
  "from-violet-500 to-purple-600",
  "from-sky-500 to-blue-600",
  "from-emerald-500 to-teal-600",
  "from-amber-500 to-orange-600",
  "from-rose-500 to-pink-600",
  "from-cyan-500 to-teal-600",
  "from-indigo-500 to-blue-600",
  "from-fuchsia-500 to-purple-600",
];

function projectColor(id: string) {
  let hash = 0;
  for (let i = 0; i < id.length; i++) {
    hash = id.charCodeAt(i) + ((hash << 5) - hash);
  }
  return colors[Math.abs(hash) % colors.length];
}

function timeAgo(dateStr: string) {
  const now = Date.now();
  const diff = now - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(dateStr).toLocaleDateString();
}
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <!-- Header -->
    <header class="border-b border-border">
      <div
        class="max-w-5xl mx-auto px-6 py-5 flex items-center justify-between"
      >
        <div class="flex items-center gap-3">
          <div
            class="w-8 h-8 rounded-lg bg-gradient-to-br from-primary to-primary/70 flex items-center justify-center"
          >
            <svg
              class="w-4.5 h-4.5 text-primary-foreground"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M12 2L2 7l10 5 10-5-10-5z" />
              <path d="M2 17l10 5 10-5" />
              <path d="M2 12l10 5 10-5" />
            </svg>
          </div>
          <div>
            <h1 class="text-lg font-semibold tracking-tight">CopilotHub</h1>
            <p class="text-[11px] text-muted-foreground leading-none mt-0.5">
              AI-powered development tools
            </p>
          </div>
        </div>
        <button
          class="inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground border border-border rounded-md px-3 py-1.5 transition-colors hover:bg-muted"
          @click="router.push('/settings')"
        >
          <svg class="w-3.5 h-3.5" viewBox="0 0 16 16" fill="currentColor">
            <path
              d="M8 4.754a3.246 3.246 0 1 0 0 6.492 3.246 3.246 0 0 0 0-6.492zM5.754 8a2.246 2.246 0 1 1 4.492 0 2.246 2.246 0 0 1-4.492 0z"
            />
            <path
              d="M9.796 1.343c-.527-1.79-3.065-1.79-3.592 0l-.094.319a.873.873 0 0 1-1.255.52l-.292-.16c-1.64-.892-3.433.902-2.54 2.541l.159.292a.873.873 0 0 1-.52 1.255l-.319.094c-1.79.527-1.79 3.065 0 3.592l.319.094a.873.873 0 0 1 .52 1.255l-.16.292c-.892 1.64.901 3.434 2.541 2.54l.292-.159a.873.873 0 0 1 1.255.52l.094.319c.527 1.79 3.065 1.79 3.592 0l.094-.319a.873.873 0 0 1 1.255-.52l.292.16c1.64.893 3.434-.902 2.54-2.541l-.159-.292a.873.873 0 0 1 .52-1.255l.319-.094c1.79-.527 1.79-3.065 0-3.592l-.319-.094a.873.873 0 0 1-.52-1.255l.16-.292c.893-1.64-.902-3.433-2.541-2.54l-.292.159a.873.873 0 0 1-1.255-.52l-.094-.319zm-2.633.283c.246-.835 1.428-.835 1.674 0l.094.319a1.873 1.873 0 0 0 2.693 1.115l.291-.16c.764-.415 1.6.42 1.184 1.185l-.159.292a1.873 1.873 0 0 0 1.116 2.692l.318.094c.835.246.835 1.428 0 1.674l-.319.094a1.873 1.873 0 0 0-1.115 2.693l.16.291c.415.764-.42 1.6-1.185 1.184l-.291-.159a1.873 1.873 0 0 0-2.693 1.116l-.094.318c-.246.835-1.428.835-1.674 0l-.094-.319a1.873 1.873 0 0 0-2.692-1.115l-.292.16c-.764.415-1.6-.42-1.184-1.185l.159-.291A1.873 1.873 0 0 0 1.945 8.93l-.319-.094c-.835-.246-.835-1.428 0-1.674l.319-.094A1.873 1.873 0 0 0 3.06 4.474l-.16-.292c-.415-.764.42-1.6 1.185-1.184l.292.159a1.873 1.873 0 0 0 2.692-1.115l.094-.319z"
            />
          </svg>
          Settings
        </button>
      </div>
    </header>

    <main class="max-w-5xl mx-auto px-6 py-8">
      <!-- Section header -->
      <div class="flex items-center justify-between mb-6">
        <div>
          <h2 class="text-base font-semibold">Projects</h2>
          <p class="text-xs text-muted-foreground mt-0.5">
            {{ projectStore.projects.length }} project{{
              projectStore.projects.length !== 1 ? "s" : ""
            }}
          </p>
        </div>
        <button
          class="inline-flex items-center gap-1.5 text-sm font-medium px-4 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 transition-colors shadow-sm"
          @click="showCreateProject = true"
        >
          <svg class="w-4 h-4" viewBox="0 0 16 16" fill="currentColor">
            <path
              d="M8 2a.75.75 0 0 1 .75.75v4.5h4.5a.75.75 0 0 1 0 1.5h-4.5v4.5a.75.75 0 0 1-1.5 0v-4.5h-4.5a.75.75 0 0 1 0-1.5h4.5v-4.5A.75.75 0 0 1 8 2z"
            />
          </svg>
          New Project
        </button>
      </div>

      <!-- Create Project Inline -->
      <div
        v-if="showCreateProject"
        class="mb-6 border border-primary/20 rounded-xl p-5 bg-card shadow-sm"
      >
        <div class="flex items-end gap-3">
          <div class="flex-1">
            <label
              class="text-xs font-medium text-muted-foreground block mb-1.5"
              >Project name</label
            >
            <input
              v-model="newProjectName"
              type="text"
              placeholder="e.g. Payment Gateway, User Portal…"
              class="w-full px-3.5 py-2 text-sm rounded-lg border border-border bg-background focus:outline-none focus:ring-2 focus:ring-primary/30 focus:border-primary transition-all"
              autofocus
              @keydown.enter="createProject"
              @keydown.escape="
                showCreateProject = false;
                newProjectName = '';
              "
            />
          </div>
          <button
            class="px-4 py-2 text-sm rounded-lg border border-border hover:bg-muted transition-colors"
            @click="
              showCreateProject = false;
              newProjectName = '';
            "
          >
            Cancel
          </button>
          <button
            class="px-5 py-2 text-sm font-medium rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50"
            :disabled="creating || !newProjectName.trim()"
            @click="createProject"
          >
            {{ creating ? "Creating…" : "Create" }}
          </button>
        </div>
      </div>

      <!-- Empty state -->
      <div
        v-if="projectStore.projects.length === 0 && !showCreateProject"
        class="flex flex-col items-center justify-center py-20 text-center"
      >
        <div
          class="w-16 h-16 rounded-2xl bg-muted/60 flex items-center justify-center mb-4"
        >
          <svg
            class="w-7 h-7 text-muted-foreground/50"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path
              d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"
            />
            <line x1="12" y1="11" x2="12" y2="17" />
            <line x1="9" y1="14" x2="15" y2="14" />
          </svg>
        </div>
        <p class="text-sm font-medium mb-1">No projects yet</p>
        <p class="text-xs text-muted-foreground mb-5">
          Create your first project to get started
        </p>
        <button
          class="inline-flex items-center gap-1.5 text-sm font-medium px-4 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 transition-colors shadow-sm"
          @click="showCreateProject = true"
        >
          <svg class="w-4 h-4" viewBox="0 0 16 16" fill="currentColor">
            <path
              d="M8 2a.75.75 0 0 1 .75.75v4.5h4.5a.75.75 0 0 1 0 1.5h-4.5v4.5a.75.75 0 0 1-1.5 0v-4.5h-4.5a.75.75 0 0 1 0-1.5h4.5v-4.5A.75.75 0 0 1 8 2z"
            />
          </svg>
          Create Project
        </button>
      </div>

      <!-- Project Grid -->
      <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="p in projectStore.projects"
          :key="p.id"
          class="group relative border border-border rounded-xl bg-card hover:border-primary/40 hover:shadow-md transition-all duration-200 cursor-pointer overflow-hidden"
          @click="openProject(p.id)"
        >
          <!-- Color accent bar -->
          <div class="h-1.5 bg-gradient-to-r" :class="projectColor(p.id)" />

          <div class="p-5">
            <div class="flex items-start gap-3.5">
              <!-- Avatar -->
              <div
                class="w-10 h-10 rounded-lg bg-gradient-to-br flex items-center justify-center text-white font-semibold text-sm shrink-0 shadow-sm"
                :class="projectColor(p.id)"
              >
                {{ projectInitial(p.name) }}
              </div>

              <div class="flex-1 min-w-0">
                <h3
                  class="text-sm font-semibold truncate group-hover:text-primary transition-colors"
                >
                  {{ p.name }}
                </h3>
                <p
                  v-if="p.createdAt"
                  class="text-[11px] text-muted-foreground mt-0.5"
                >
                  {{ timeAgo(p.createdAt) }}
                </p>
              </div>
            </div>

            <!-- Feature badges -->
            <div class="flex items-center gap-1.5 mt-4">
              <span
                class="inline-flex items-center gap-1 text-[10px] text-muted-foreground bg-muted/60 rounded-md px-2 py-0.5"
              >
                <span class="text-xs">🔍</span> Spec Clarify
              </span>
              <span
                class="inline-flex items-center gap-1 text-[10px] text-muted-foreground bg-muted/60 rounded-md px-2 py-0.5"
              >
                <span class="text-xs">📚</span> Wiki
              </span>
            </div>
          </div>

          <!-- Delete button (top-right, visible on hover) -->
          <button
            class="absolute top-3 right-3 p-1.5 rounded-md text-muted-foreground/0 group-hover:text-muted-foreground hover:!text-destructive hover:bg-destructive/10 transition-all"
            title="Delete project"
            @click.stop="deleteProject(p.id)"
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 16 16" fill="currentColor">
              <path
                d="M6.5 1.75a.25.25 0 0 1 .25-.25h2.5a.25.25 0 0 1 .25.25V3h-3V1.75zm4.5 0V3h2.25a.75.75 0 0 1 0 1.5H2.75a.75.75 0 0 1 0-1.5H5V1.75C5 .784 5.784 0 6.75 0h2.5C10.216 0 11 .784 11 1.75zM4.496 6.675a.75.75 0 1 0-1.492.15l.66 6.6A1.75 1.75 0 0 0 5.405 15h5.19a1.75 1.75 0 0 0 1.741-1.575l.66-6.6a.75.75 0 0 0-1.492-.15l-.66 6.6a.25.25 0 0 1-.249.225h-5.19a.25.25 0 0 1-.249-.225l-.66-6.6z"
              />
            </svg>
          </button>
        </div>
      </div>
    </main>
  </div>
</template>
