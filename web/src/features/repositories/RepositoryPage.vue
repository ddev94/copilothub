<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useProjectStore } from "@/stores/repo";
import { api } from "@/api";
import type { ProjectRepository } from "@/types";
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

// ── Add repo form ──
const showAddForm = ref(false);
const newRepoURL = ref("");
const newBranch = ref("");
const newRepoName = ref("");
const adding = ref(false);
const addError = ref<string | null>(null);

// ── Per-repo state ──
const editingBranchFor = ref<string | null>(null);
const editBranchValue = ref("");
const changingBranchFor = ref<string | null>(null);
const removingRepoId = ref<string | null>(null);
const repoError = ref<string | null>(null);

onMounted(async () => {
  await projectStore.fetch();
  if (projectId.value) {
    projectStore.selectProject(projectId.value);
  }
});

async function addRepo() {
  if (!newRepoURL.value.trim()) return;
  adding.value = true;
  addError.value = null;
  try {
    await api.projects.addRepo(
      projectId.value,
      newRepoURL.value.trim(),
      newBranch.value.trim() || undefined,
      newRepoName.value.trim() || undefined,
    );
    await projectStore.fetch();
    newRepoURL.value = "";
    newBranch.value = "";
    newRepoName.value = "";
    showAddForm.value = false;
  } catch (e) {
    addError.value =
      e instanceof Error ? e.message : "Failed to clone repository";
  } finally {
    adding.value = false;
  }
}

function startEditBranch(repo: ProjectRepository) {
  editingBranchFor.value = repo.id;
  editBranchValue.value = repo.repoBranch ?? "";
  repoError.value = null;
}

async function saveBranch(repoId: string) {
  changingBranchFor.value = repoId;
  repoError.value = null;
  try {
    await api.projects.changeRepoBranch(
      projectId.value,
      repoId,
      editBranchValue.value.trim(),
    );
    await projectStore.fetch();
    editingBranchFor.value = null;
  } catch (e) {
    repoError.value =
      e instanceof Error ? e.message : "Failed to change branch";
  } finally {
    changingBranchFor.value = null;
  }
}

async function removeRepo(repoId: string) {
  removingRepoId.value = repoId;
  repoError.value = null;
  try {
    await api.projects.removeRepo(projectId.value, repoId);
    await projectStore.fetch();
  } catch (e) {
    repoError.value = e instanceof Error ? e.message : "Failed to disconnect";
  } finally {
    removingRepoId.value = null;
  }
}

function repoDisplayName(repo: ProjectRepository) {
  return (
    repo.name ||
    repo.repoURL
      .replace(/^https?:\/\/github\.com\//, "")
      .replace(/\.git$/, "")
  );
}
</script>

<template>
  <div class="flex flex-col h-screen bg-background text-foreground overflow-hidden">
    <header
      class="h-11 border-b border-border flex items-center justify-between px-4 shrink-0 bg-background z-10"
    >
      <div class="flex items-center gap-3">
        <button
          class="text-xs text-muted-foreground hover:text-foreground transition-colors"
          @click="router.push(`/projects/${projectId}`)"
        >
          ← Project
        </button>
        <span class="text-muted-foreground/40">|</span>
        <span class="text-sm font-medium">Git Repositories</span>
      </div>
      <span
        v-if="project?.repositories && project.repositories.length > 0"
        class="text-xs text-muted-foreground"
      >
        {{ project.repositories.length }}
        {{ project.repositories.length === 1 ? "repository" : "repositories" }}
      </span>
    </header>

    <div class="flex-1 overflow-y-auto">
      <div class="max-w-2xl mx-auto px-6 py-8 space-y-4">
        <div
          v-if="!project"
          class="text-center text-muted-foreground py-12 text-sm"
        >
          Project not found.
        </div>

        <template v-else>
          <!-- Repos list -->
          <section
            class="border border-border rounded-lg bg-card divide-y divide-border"
          >
            <div class="px-5 py-4 flex items-center justify-between">
              <div>
                <h2 class="font-semibold text-sm">Connected Repositories</h2>
                <p class="text-xs text-muted-foreground mt-0.5">
                  Connect GitHub repositories for source-code analysis
                </p>
              </div>
              <Button
                v-if="!showAddForm"
                size="sm"
                variant="outline"
                @click="showAddForm = true"
              >
                + Add Repository
              </Button>
            </div>

            <!-- Existing repos list -->
            <div
              v-if="project.repositories && project.repositories.length > 0"
              class="divide-y divide-border"
            >
              <div
                v-for="repo in project.repositories"
                :key="repo.id"
                class="px-5 py-4 space-y-3"
              >
                <!-- Repo header -->
                <div class="flex items-start gap-3">
                  <span class="text-green-500 text-sm mt-0.5">✓</span>
                  <div class="flex-1 min-w-0">
                    <p class="text-sm font-medium truncate">
                      {{ repoDisplayName(repo) }}
                    </p>
                    <p class="text-xs text-muted-foreground font-mono truncate mt-0.5">
                      {{ repo.repoURL }}
                    </p>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    class="shrink-0 text-destructive hover:text-destructive"
                    :disabled="removingRepoId === repo.id"
                    @click="removeRepo(repo.id)"
                  >
                    {{ removingRepoId === repo.id ? "Removing…" : "Disconnect" }}
                  </Button>
                </div>

                <!-- Branch -->
                <div class="pl-6 space-y-1.5">
                  <Label class="text-xs text-muted-foreground">Branch</Label>
                  <div v-if="editingBranchFor === repo.id" class="flex items-center gap-2">
                    <Input
                      v-model="editBranchValue"
                      placeholder="main"
                      class="flex-1 h-8 text-sm"
                      :disabled="changingBranchFor === repo.id"
                      @keyup.enter="saveBranch(repo.id)"
                    />
                    <Button
                      size="sm"
                      class="shrink-0 h-8"
                      :disabled="changingBranchFor === repo.id"
                      @click="saveBranch(repo.id)"
                    >
                      {{ changingBranchFor === repo.id ? "Cloning…" : "Apply" }}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      class="shrink-0 h-8"
                      :disabled="changingBranchFor === repo.id"
                      @click="editingBranchFor = null"
                    >
                      Cancel
                    </Button>
                  </div>
                  <div
                    v-else
                    class="flex items-center gap-2 h-8 px-3 rounded-md border border-border bg-background"
                  >
                    <p class="text-sm font-mono flex-1">
                      {{ repo.repoBranch || "—" }}
                    </p>
                    <button
                      class="text-xs text-primary hover:underline shrink-0"
                      @click="startEditBranch(repo)"
                    >
                      Change
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Empty state -->
            <div
              v-else-if="!showAddForm"
              class="px-5 py-6 text-center text-sm text-muted-foreground"
            >
              No repositories connected. Add one to enable source-code analysis.
            </div>

            <!-- Add repository form -->
            <div v-if="showAddForm" class="px-5 py-5 space-y-4">
              <p class="text-sm font-medium">Add Repository</p>
              <div class="space-y-1.5">
                <Label>Repository URL</Label>
                <Input
                  v-model="newRepoURL"
                  placeholder="https://github.com/owner/repo.git"
                  :disabled="adding"
                  @keyup.enter="addRepo"
                />
              </div>
              <div class="grid grid-cols-2 gap-3">
                <div class="space-y-1.5">
                  <Label>
                    Branch
                    <span class="text-muted-foreground text-xs">(optional)</span>
                  </Label>
                  <Input
                    v-model="newBranch"
                    placeholder="main"
                    :disabled="adding"
                  />
                </div>
                <div class="space-y-1.5">
                  <Label>
                    Display Name
                    <span class="text-muted-foreground text-xs">(optional)</span>
                  </Label>
                  <Input
                    v-model="newRepoName"
                    placeholder="e.g. Backend API"
                    :disabled="adding"
                  />
                </div>
              </div>
              <p v-if="addError" class="text-xs text-destructive">
                {{ addError }}
              </p>
              <div class="flex justify-end gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  :disabled="adding"
                  @click="showAddForm = false; addError = null"
                >
                  Cancel
                </Button>
                <Button
                  size="sm"
                  :disabled="adding || !newRepoURL.trim()"
                  @click="addRepo"
                >
                  {{ adding ? "Cloning…" : "Connect & Clone" }}
                </Button>
              </div>
            </div>

            <!-- Shared repo error -->
            <div v-if="repoError" class="px-5 py-3">
              <p class="text-xs text-destructive">{{ repoError }}</p>
            </div>
          </section>
        </template>
      </div>
    </div>
  </div>
</template>
