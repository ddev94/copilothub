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
      repoURL.value = project.value?.repoURL ?? "";
      branch.value = project.value?.repoBranch ?? "";
      error.value = null;
    }
  },
);

async function connectRepo() {
  if (!repoURL.value.trim()) return;
  connecting.value = true;
  error.value = null;
  try {
    await api.projects.connectRepo(
      props.projectId,
      repoURL.value.trim(),
      branch.value.trim() || undefined,
    );
    await projectStore.fetch();
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to clone repository";
  } finally {
    connecting.value = false;
  }
}

async function disconnectRepo() {
  connecting.value = true;
  error.value = null;
  try {
    await api.projects.disconnectRepo(props.projectId);
    await projectStore.fetch();
    repoURL.value = "";
    branch.value = "";
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
          Configure repository connection for
          <strong>{{ project?.name }}</strong>
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4 pt-2">
        <!-- Repo URL -->
        <div class="space-y-1.5">
          <Label>Repository URL</Label>
          <Input
            v-model="repoURL"
            placeholder="https://github.com/owner/repo.git"
            :disabled="connecting || !!project?.repoCloned"
          />
        </div>

        <!-- Branch -->
        <div class="space-y-1.5">
          <Label
            >Branch
            <span class="text-muted-foreground text-xs"
              >(optional, defaults to main)</span
            ></Label
          >
          <Input
            v-model="branch"
            placeholder="main"
            :disabled="connecting || !!project?.repoCloned"
          />
        </div>

        <!-- Status -->
        <div
          v-if="project?.repoCloned"
          class="flex items-center gap-2 rounded border border-green-500/30 bg-green-500/5 px-3 py-2"
        >
          <span class="text-green-500 text-sm">✓</span>
          <div class="flex-1 min-w-0">
            <p class="text-sm font-mono truncate">{{ project.repoURL }}</p>
            <p v-if="project.repoBranch" class="text-xs text-muted-foreground">
              branch: {{ project.repoBranch }}
            </p>
          </div>
        </div>

        <!-- Error -->
        <p v-if="error" class="text-xs text-destructive">{{ error }}</p>

        <!-- Actions -->
        <div class="flex justify-end gap-2 pt-2">
          <Button
            v-if="project?.repoCloned"
            variant="outline"
            size="sm"
            :disabled="connecting"
            @click="disconnectRepo"
          >
            {{ connecting ? "Removing…" : "Disconnect" }}
          </Button>
          <Button
            v-else
            size="sm"
            :disabled="connecting || !repoURL.trim()"
            @click="connectRepo"
          >
            {{ connecting ? "Cloning…" : "Connect & Clone" }}
          </Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
