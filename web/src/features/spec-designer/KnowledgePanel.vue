<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { useKnowledgeStore } from "@/stores/knowledge";
import { useProjectStore } from "@/stores/repo";
import { Button } from "@/components/ui/button";

const store = useKnowledgeStore();
const projectStore = useProjectStore();
const route = useRoute();
const projectId = computed(
  () =>
    (route.params.projectId as string) ??
    projectStore.selectedProject?.id ??
    "",
);
const fileInput = ref<HTMLInputElement | null>(null);

onMounted(() => store.loadDocuments(projectId.value));

function triggerUpload() {
  fileInput.value?.click();
}

async function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement;
  const files = Array.from(input.files ?? []);
  if (files.length === 0) return;
  await store.uploadFiles(files, false, projectId.value);
  input.value = "";
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("vi-VN", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

function extIcon(name: string) {
  if (name.endsWith(".pdf")) return "📄";
  if (name.endsWith(".docx")) return "📝";
  return "📃";
}
</script>

<template>
  <div
    class="flex flex-col h-full border-l border-border w-64 shrink-0 bg-background"
  >
    <!-- Header -->
    <div
      class="flex items-center justify-between px-3 py-2 border-b border-border"
    >
      <span
        class="text-xs font-semibold text-muted-foreground uppercase tracking-wide"
        >Knowledge</span
      >
      <Button
        variant="ghost"
        size="sm"
        class="h-7 px-2 text-xs"
        :disabled="store.uploading"
        @click="triggerUpload"
      >
        {{ store.uploading ? "Đang xử lý…" : "+ Upload" }}
      </Button>
      <input
        ref="fileInput"
        type="file"
        accept=".pdf,.md,.docx"
        multiple
        class="hidden"
        @change="onFileChange"
      />
    </div>

    <!-- Error -->
    <div v-if="store.error" class="mx-3 mt-2 text-xs text-destructive">
      {{ store.error }}
    </div>

    <!-- List -->
    <div class="flex-1 overflow-y-auto">
      <div v-if="store.loading" class="p-3 text-xs text-muted-foreground">
        Đang tải…
      </div>

      <div
        v-else-if="store.documents.length === 0"
        class="p-4 text-xs text-muted-foreground text-center leading-relaxed"
      >
        Chưa có tài liệu nào.<br />Upload PDF, MD, hoặc DOCX để bổ sung kiến
        thức cho AI.
      </div>

      <ul v-else class="divide-y divide-border">
        <li
          v-for="doc in store.documents"
          :key="doc.id"
          class="flex items-center gap-2 px-3 py-2 hover:bg-muted/50 group"
        >
          <span class="text-base leading-none shrink-0">{{
            extIcon(doc.name)
          }}</span>
          <div class="flex-1 min-w-0">
            <p class="text-xs font-medium truncate">{{ doc.name }}</p>
            <p class="text-[11px] text-muted-foreground">
              {{ formatDate(doc.createdAt) }}
            </p>
          </div>
          <button
            class="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-opacity p-0.5 text-xs shrink-0"
            title="Xoá"
            @click="store.deleteDocument(doc.id, projectId)"
          >
            ✕
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>
