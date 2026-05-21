<script setup lang="ts">
import { ref, watch, computed } from "vue";
import { api } from "@/api";
import { useProjectStore } from "@/stores/repo";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const props = defineProps<{
  projectId: string;
  open: boolean;
}>();

const emit = defineEmits<{
  (e: "update:open", val: boolean): void;
}>();

const projectStore = useProjectStore();
const project = computed(
  () => projectStore.projects.find((p) => p.id === props.projectId) ?? null,
);

const repoURL = ref("");
const branch = ref("");
const connecting = ref(false);
const error = ref<string | null>(null);

watch(
  () => props.open,
  (v) => {
    if (v) {
      error.value = null;
      repoURL.value = "";
      branch.value = "";
    }
  },
);

const hasRepos = computed(
  () => (project.value?.repositories?.length ?? 0) > 0,
);

async function addRepo() {
  if (!repoURL.value.trim()) return;
  connecting.value = true;
  error.value = null;
  try {
    await api.projects.addRepo(
      props.projectId,
      repoURL.value.trim(),
      branch.value.trim() || undefined,
    );
    await projectStore.fetch();
    repoURL.value = "";
    branch.value = "";
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to clone repository";
  } finally {
    connecting.value = false;
  }
}

async function removeRepo(repoId: string) {
  connecting.value = true;
  error.value = null;
  try {
    await api.projects.removeRepo(props.projectId, repoId);
    await projectStore.fetch();
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to disconnect";
  } finally {
    connecting.value = false;
  }
}
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>Project Settings</DialogTitle>
        <DialogDescription>
          Manage repositories for
          <strong>{{ project?.name }}</strong>
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4 pt-2">
        <!-- Connected repos -->
        <div v-if="hasRepos" class="space-y-2">
          <Label>Connected Repositories</Label>
          <div class="space-y-1">
            <div
              v-for="repo in project!.repositories"
              :key="repo.id"
              class="flex items-center gap-2 rounded border border-green-500/30 bg-green-500/5 px-3 py-2"
            >
              <span class="text-green-500 text-sm">✓</span>
              <div class="flex-1 min-w-0">
                <p class="text-xs font-mono truncate">{{ repo.repoURL }}</p>
                <p v-if="repo.repoBranch" class="text-xs text-muted-foreground">
                  {{ repo.repoBranch }}
                </p>
              </div>
              <Button
                variant="ghost"
                size="sm"
                class="text-destructive hover:text-destructive shrink-0"
                :disabled="connecting"
                @click="removeRepo(repo.id)"
              >
                Remove
              </Button>
            </div>
          </div>
        </div>

        <!-- Add repo form -->
        <div class="space-y-3">
          <Label>Add Repository</Label>
          <Input
            v-model="repoURL"
            placeholder="https://github.com/owner/repo.git"
            :disabled="connecting"
          />
          <Input
            v-model="branch"
            placeholder="Branch (optional, defaults to main)"
            :disabled="connecting"
          />
        </div>

        <p v-if="error" class="text-xs text-destructive">{{ error }}</p>

        <div class="flex justify-end gap-2 pt-2">
          <Button
            size="sm"
            :disabled="connecting || !repoURL.trim()"
            @click="addRepo"
          >
            {{ connecting ? "Cloning…" : "Connect & Clone" }}
          </Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
