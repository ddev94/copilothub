<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useProjectStore } from "@/stores/repo";
import { api } from "@/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const route = useRoute();
const router = useRouter();
const projectStore = useProjectStore();

const projectId = computed(() => route.params.projectId as string);
const project = computed(
  () => projectStore.projects.find((p) => p.id === projectId.value) ?? null,
);

// ── General ──
const projectName = ref("");
const savingName = ref(false);
const nameMsg = ref<{ type: "ok" | "error"; text: string } | null>(null);

// ── Git ──
const repoURL = ref("");
const branch = ref("");
const connecting = ref(false);
const gitError = ref<string | null>(null);
const editingBranch = ref(false);
const newBranch = ref("");
const changingBranch = ref(false);

onMounted(async () => {
  await projectStore.fetch();
  if (project.value) {
    projectName.value = project.value.name;
    repoURL.value = project.value.repoURL ?? "";
    branch.value = project.value.repoBranch ?? "";
  }
});

async function saveName() {
  if (!projectName.value.trim() || projectName.value === project.value?.name)
    return;
  savingName.value = true;
  nameMsg.value = null;
  try {
    await api.projects.update(projectId.value, {
      name: projectName.value.trim(),
    });
    await projectStore.fetch();
    nameMsg.value = { type: "ok", text: "Saved" };
    setTimeout(() => (nameMsg.value = null), 2000);
  } catch (e) {
    nameMsg.value = {
      type: "error",
      text: e instanceof Error ? e.message : "Failed to save",
    };
  } finally {
    savingName.value = false;
  }
}

async function connectRepo() {
  if (!repoURL.value.trim()) return;
  connecting.value = true;
  gitError.value = null;
  try {
    await api.projects.connectRepo(
      projectId.value,
      repoURL.value.trim(),
      branch.value.trim() || undefined,
    );
    await projectStore.fetch();
  } catch (e) {
    gitError.value =
      e instanceof Error ? e.message : "Failed to clone repository";
  } finally {
    connecting.value = false;
  }
}

async function disconnectRepo() {
  connecting.value = true;
  gitError.value = null;
  try {
    await api.projects.disconnectRepo(projectId.value);
    await projectStore.fetch();
    repoURL.value = "";
    branch.value = "";
  } catch (e) {
    gitError.value = e instanceof Error ? e.message : "Failed to disconnect";
  } finally {
    connecting.value = false;
  }
}

function startEditBranch() {
  newBranch.value = project.value?.repoBranch ?? "";
  editingBranch.value = true;
  gitError.value = null;
}

async function saveBranch() {
  changingBranch.value = true;
  gitError.value = null;
  try {
    await api.projects.changeBranch(projectId.value, newBranch.value.trim());
    await projectStore.fetch();
    editingBranch.value = false;
  } catch (e) {
    gitError.value = e instanceof Error ? e.message : "Failed to change branch";
  } finally {
    changingBranch.value = false;
  }
}
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <!-- Header -->
    <header class="border-b border-border px-6 py-4">
      <div class="flex items-center gap-3">
        <button
          class="text-muted-foreground hover:text-foreground transition-colors"
          @click="router.push(`/projects/${projectId}`)"
        >
          <svg class="w-4 h-4" viewBox="0 0 16 16" fill="currentColor">
            <path
              fill-rule="evenodd"
              d="M15 8a.5.5 0 0 0-.5-.5H2.707l3.147-3.146a.5.5 0 1 0-.708-.708l-4 4a.5.5 0 0 0 0 .708l4 4a.5.5 0 0 0 .708-.708L2.707 8.5H14.5A.5.5 0 0 0 15 8z"
            />
          </svg>
        </button>
        <h1 class="text-lg font-semibold">
          Project Settings
          <span v-if="project" class="text-muted-foreground font-normal">
            — {{ project.name }}
          </span>
        </h1>
      </div>
    </header>

    <main class="max-w-2xl mx-auto px-6 py-8 space-y-8">
      <div
        v-if="!project"
        class="text-center text-muted-foreground py-12 text-sm"
      >
        Project not found.
      </div>

      <template v-else>
        <!-- General Section -->
        <section
          class="border border-border rounded-lg bg-card divide-y divide-border"
        >
          <div class="px-5 py-4">
            <h2 class="font-semibold text-sm">General</h2>
            <p class="text-xs text-muted-foreground mt-0.5">
              Basic project information
            </p>
          </div>
          <div class="px-5 py-5 space-y-4">
            <div class="space-y-1.5">
              <Label>Project Name</Label>
              <div class="flex gap-2">
                <Input
                  v-model="projectName"
                  placeholder="Project name"
                  :disabled="savingName"
                  @keyup.enter="saveName"
                />
                <Button
                  size="sm"
                  :disabled="
                    savingName ||
                    !projectName.trim() ||
                    projectName === project.name
                  "
                  @click="saveName"
                >
                  {{ savingName ? "Saving…" : "Save" }}
                </Button>
              </div>
              <p
                v-if="nameMsg"
                class="text-xs"
                :class="
                  nameMsg.type === 'ok' ? 'text-green-500' : 'text-destructive'
                "
              >
                {{ nameMsg.text }}
              </p>
            </div>
            <div class="space-y-1.5">
              <Label class="text-muted-foreground">Project ID</Label>
              <p class="text-sm font-mono text-muted-foreground">
                {{ project.id }}
              </p>
            </div>
            <div v-if="project.createdAt" class="space-y-1.5">
              <Label class="text-muted-foreground">Created</Label>
              <p class="text-sm text-muted-foreground">
                {{ new Date(project.createdAt).toLocaleDateString() }}
              </p>
            </div>
          </div>
        </section>

        <!-- Git Repository Section -->
        <section
          class="border border-border rounded-lg bg-card divide-y divide-border"
        >
          <div class="px-5 py-4">
            <h2 class="font-semibold text-sm">Git Repository</h2>
            <p class="text-xs text-muted-foreground mt-0.5">
              Connect a GitHub repository to enable source-code based analysis
            </p>
          </div>
          <div class="px-5 py-5 space-y-4">
            <!-- Connected status -->
            <div v-if="project.repoCloned" class="space-y-3">
              <div
                class="flex items-center gap-2 rounded border border-green-500/30 bg-green-500/5 px-3 py-2.5"
              >
                <span class="text-green-500 text-sm">✓</span>
                <div class="flex-1 min-w-0">
                  <p class="text-sm font-mono truncate">
                    {{ project.repoURL }}
                  </p>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  :disabled="connecting || changingBranch"
                  @click="disconnectRepo"
                >
                  {{ connecting ? "Removing…" : "Disconnect" }}
                </Button>
              </div>

              <!-- Branch -->
              <div class="space-y-1.5">
                <Label>Branch</Label>
                <div v-if="editingBranch" class="flex items-center gap-2">
                  <Input
                    v-model="newBranch"
                    placeholder="main"
                    class="flex-1"
                    :disabled="changingBranch"
                    @keyup.enter="saveBranch"
                  />
                  <Button
                    size="sm"
                    class="shrink-0"
                    :disabled="changingBranch"
                    @click="saveBranch"
                  >
                    {{ changingBranch ? "Cloning…" : "Apply" }}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    class="shrink-0"
                    :disabled="changingBranch"
                    @click="editingBranch = false"
                  >
                    Cancel
                  </Button>
                </div>
                <div
                  v-else
                  class="flex items-center gap-2 h-9 px-3 rounded-md border border-border bg-background"
                >
                  <p class="text-sm font-mono flex-1">
                    {{ project.repoBranch || "(default)" }}
                  </p>
                  <button
                    class="text-xs text-primary hover:underline shrink-0"
                    @click="startEditBranch"
                  >
                    Change
                  </button>
                </div>
              </div>
            </div>

            <!-- Connect form -->
            <template v-else>
              <div class="space-y-1.5">
                <Label>Repository URL</Label>
                <Input
                  v-model="repoURL"
                  placeholder="https://github.com/owner/repo.git"
                  :disabled="connecting"
                  @keyup.enter="connectRepo"
                />
              </div>
              <div class="space-y-1.5">
                <Label>
                  Branch
                  <span class="text-muted-foreground text-xs"
                    >(optional, defaults to main)</span
                  >
                </Label>
                <Input
                  v-model="branch"
                  placeholder="main"
                  :disabled="connecting"
                  @keyup.enter="connectRepo"
                />
              </div>
              <div class="flex justify-end">
                <Button
                  size="sm"
                  :disabled="connecting || !repoURL.trim()"
                  @click="connectRepo"
                >
                  {{ connecting ? "Cloning…" : "Connect & Clone" }}
                </Button>
              </div>
            </template>

            <!-- Error -->
            <p v-if="gitError" class="text-xs text-destructive">
              {{ gitError }}
            </p>
          </div>
        </section>
      </template>
    </main>
  </div>
</template>
