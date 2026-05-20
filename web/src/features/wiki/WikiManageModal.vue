<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { X } from "lucide-vue-next";
import {
  DialogClose,
  DialogContent,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
  DialogDescription,
} from "reka-ui";
import { Button } from "@/components/ui/button";
import { useKnowledgeStore } from "@/stores/knowledge";
import type { KnowledgeDocument } from "@/types";

const props = defineProps<{
  open: boolean;
  projectId: string;
}>();

const emit = defineEmits<{
  (e: "update:open", val: boolean): void;
  (e: "select", doc: KnowledgeDocument): void;
}>();

const knowledge = useKnowledgeStore();

// ── Upload state ─────────────────────────────────────────────────────────────
let nextQueueId = 0;
const fileInputRef = ref<HTMLInputElement | null>(null);
const uploadQueue = ref<
  Array<{
    id: number;
    file: File;
    status: "queued" | "uploading" | "embedding" | "done" | "error";
    message?: string;
  }>
>([]);

const isProcessing = computed(() =>
  uploadQueue.value.some(
    (i) => i.status === "uploading" || i.status === "embedding",
  ),
);

// Duplicate handling
const showDuplicateConfirm = ref(false);
const pendingFiles = ref<File[]>([]);
const duplicateNames = ref<string[]>([]);

// Delete confirmation
const deletingDoc = ref<KnowledgeDocument | null>(null);
const deleting = ref(false);

// Combined doc list — deduplicate by id (pending first)
const allDocs = computed(() => {
  const seen = new Set<string>();
  const result: KnowledgeDocument[] = [];
  for (const d of knowledge.pendingDocuments) {
    if (!seen.has(d.id)) {
      seen.add(d.id);
      result.push(d);
    }
  }
  for (const d of knowledge.documents) {
    if (!seen.has(d.id)) {
      seen.add(d.id);
      result.push(d);
    }
  }
  return result;
});

// ── Upload logic ─────────────────────────────────────────────────────────────
function triggerUpload() {
  fileInputRef.value?.click();
}

function onFileSelect(e: Event) {
  const input = e.target as HTMLInputElement;
  const files = Array.from(input.files ?? []);
  input.value = "";
  if (files.length === 0) return;

  // Check for duplicates against both approved and pending docs
  const existingNames = new Set([
    ...knowledge.documents.map((d) => d.name),
    ...knowledge.pendingDocuments.map((d) => d.name),
  ]);
  const dupes = files
    .filter((f) => existingNames.has(f.name))
    .map((f) => f.name);
  if (dupes.length > 0) {
    pendingFiles.value = files;
    duplicateNames.value = dupes;
    showDuplicateConfirm.value = true;
  } else {
    startUpload(files, false);
  }
}

function onDuplicateReplace() {
  showDuplicateConfirm.value = false;
  const files = pendingFiles.value;
  pendingFiles.value = [];
  duplicateNames.value = [];
  startUpload(files, true);
}

function onDuplicateSkip() {
  showDuplicateConfirm.value = false;
  const dupeSet = new Set(duplicateNames.value);
  const newFiles = pendingFiles.value.filter((f) => !dupeSet.has(f.name));
  pendingFiles.value = [];
  duplicateNames.value = [];
  if (newFiles.length > 0) {
    startUpload(newFiles, false);
  }
}

function onDuplicateCancel() {
  showDuplicateConfirm.value = false;
  pendingFiles.value = [];
  duplicateNames.value = [];
}

async function startUpload(files: File[], replaceDuplicates: boolean) {
  const items = files.map((file) => ({
    id: nextQueueId++,
    file,
    status: "queued" as const,
  }));
  uploadQueue.value.push(...items);

  for (const item of items) {
    const queueItem = uploadQueue.value.find((q) => q.id === item.id);
    if (!queueItem) continue;

    queueItem.status = "uploading";
    try {
      await new Promise((r) => setTimeout(r, 300));
      queueItem.status = "embedding";
      await knowledge.uploadFiles(
        [queueItem.file],
        replaceDuplicates,
        props.projectId,
      );
      queueItem.status = "done";
    } catch (e) {
      queueItem.status = "error";
      queueItem.message = e instanceof Error ? e.message : "Upload failed";
    }
  }

  setTimeout(() => {
    uploadQueue.value = uploadQueue.value.filter((i) => i.status !== "done");
  }, 1500);
}

// ── Delete logic ─────────────────────────────────────────────────────────────
function confirmDelete(doc: KnowledgeDocument) {
  deletingDoc.value = doc;
}

async function doDelete() {
  if (!deletingDoc.value) return;
  deleting.value = true;
  try {
    await knowledge.deleteDocument(deletingDoc.value.id, props.projectId);
  } finally {
    deleting.value = false;
    deletingDoc.value = null;
  }
}

function cancelDelete() {
  deletingDoc.value = null;
}

// ── Approve / Reject ─────────────────────────────────────────────────────────
async function approve(doc: KnowledgeDocument) {
  await knowledge.approveDocument(doc.id, props.projectId);
}

async function reject(doc: KnowledgeDocument) {
  await knowledge.rejectDocument(doc.id, props.projectId);
}

async function approveAll() {
  await knowledge.approveAll(props.projectId);
}

// ── Helpers ──────────────────────────────────────────────────────────────────
function fileIcon(name: string) {
  if (name.endsWith(".md")) return "📄";
  if (name.endsWith(".pdf")) return "📕";
  if (name.endsWith(".docx")) return "📝";
  return "📎";
}

function statusBadge(doc: KnowledgeDocument) {
  if (doc.status === "pending")
    return {
      label: "Pending",
      cls: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
    };
  if (doc.status === "approved")
    return {
      label: "Approved",
      cls: "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400",
    };
  if (doc.status === "rejected")
    return {
      label: "Rejected",
      cls: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
    };
  return {
    label: doc.status ?? "Unknown",
    cls: "bg-muted text-muted-foreground",
  };
}

function formatDate(dateStr: string) {
  if (!dateStr) return "";
  return new Date(dateStr).toLocaleDateString("vi-VN", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

// Reload on open, reset transient state on close
watch(
  () => props.open,
  async (val) => {
    if (val && props.projectId) {
      await knowledge.loadDocuments(props.projectId);
    } else {
      deletingDoc.value = null;
      showDuplicateConfirm.value = false;
      pendingFiles.value = [];
      duplicateNames.value = [];
    }
  },
);
</script>

<template>
  <DialogRoot :open="open" @update:open="emit('update:open', $event)">
    <DialogPortal>
      <!-- Transparent overlay — no dark background -->
      <DialogOverlay class="fixed inset-0 z-50" />

      <DialogContent
        class="fixed left-1/2 top-1/2 z-50 w-full max-w-2xl -translate-x-1/2 -translate-y-1/2 border border-border bg-background rounded-lg shadow-xl p-6 flex flex-col max-h-[80vh] duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95"
      >
        <!-- Header -->
        <div class="space-y-1.5 mb-3">
          <DialogTitle
            class="text-lg font-semibold leading-none tracking-tight"
          >
            Quản lý Wiki
          </DialogTitle>
          <DialogDescription class="text-sm text-muted-foreground">
            Upload, xem và quản lý tài liệu wiki cho project.
          </DialogDescription>
        </div>
        <DialogClose
          class="absolute right-4 top-4 rounded-sm opacity-70 ring-offset-background transition-opacity hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
        >
          <X class="w-4 h-4" />
        </DialogClose>

        <!-- Actions bar -->
        <div class="flex items-center gap-2 pb-3">
          <Button size="sm" @click="triggerUpload" :disabled="isProcessing">
            <svg
              class="w-4 h-4 mr-1.5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M12 4v16m8-8H4"
              />
            </svg>
            Upload file
          </Button>
          <input
            ref="fileInputRef"
            type="file"
            accept=".pdf,.md,.docx"
            multiple
            class="hidden"
            @change="onFileSelect"
          />
          <Button
            v-if="knowledge.pendingDocuments.length > 0"
            variant="outline"
            size="sm"
            @click="approveAll"
          >
            Approve all ({{ knowledge.pendingDocuments.length }})
          </Button>
          <div class="flex-1" />
          <span class="text-xs text-muted-foreground">
            {{ allDocs.length }} tài liệu
          </span>
        </div>

        <!-- Duplicate confirmation -->
        <Transition name="slide-up">
          <div
            v-if="showDuplicateConfirm"
            class="border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-950/30 rounded-md p-3 mb-3 space-y-2"
          >
            <p class="text-sm">
              File trùng tên: <strong>{{ duplicateNames.join(", ") }}</strong>
            </p>
            <div class="flex gap-2 justify-end">
              <Button variant="outline" size="sm" @click="onDuplicateCancel"
                >Huỷ</Button
              >
              <Button variant="outline" size="sm" @click="onDuplicateSkip"
                >Bỏ qua trùng</Button
              >
              <Button size="sm" @click="onDuplicateReplace">Ghi đè</Button>
            </div>
          </div>
        </Transition>

        <!-- Upload progress queue -->
        <div v-if="uploadQueue.length > 0" class="space-y-1.5 pb-3">
          <TransitionGroup name="upload-item">
            <div
              v-for="item in uploadQueue"
              :key="item.id"
              class="flex items-center gap-3 rounded-md border px-3 py-2 text-sm transition-all duration-300"
              :class="{
                'border-border': item.status === 'queued',
                'border-blue-200 dark:border-blue-800 bg-blue-50/50 dark:bg-blue-950/20':
                  item.status === 'uploading',
                'border-purple-200 dark:border-purple-800 bg-purple-50/50 dark:bg-purple-950/20':
                  item.status === 'embedding',
                'border-green-200 dark:border-green-800 bg-green-50/50 dark:bg-green-950/20':
                  item.status === 'done',
                'border-red-200 dark:border-red-800 bg-red-50/50 dark:bg-red-950/20':
                  item.status === 'error',
              }"
            >
              <span class="shrink-0">{{ fileIcon(item.file.name) }}</span>
              <span class="truncate flex-1 font-medium">{{
                item.file.name
              }}</span>

              <template v-if="item.status === 'queued'">
                <span class="text-xs text-muted-foreground">Đang chờ…</span>
              </template>
              <template v-else-if="item.status === 'uploading'">
                <span
                  class="flex items-center gap-1.5 text-xs text-blue-600 dark:text-blue-400"
                >
                  <svg
                    class="w-3.5 h-3.5 animate-spin"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      class="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      stroke-width="4"
                    />
                    <path
                      class="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"
                    />
                  </svg>
                  Uploading…
                </span>
              </template>
              <template v-else-if="item.status === 'embedding'">
                <span
                  class="flex items-center gap-1.5 text-xs text-purple-600 dark:text-purple-400"
                >
                  <svg
                    class="w-3.5 h-3.5 animate-spin"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      class="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      stroke-width="4"
                    />
                    <path
                      class="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"
                    />
                  </svg>
                  Embedding…
                </span>
              </template>
              <template v-else-if="item.status === 'done'">
                <span
                  class="text-xs text-green-600 dark:text-green-400 flex items-center gap-1"
                >
                  <svg
                    class="w-3.5 h-3.5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M5 13l4 4L19 7"
                    />
                  </svg>
                  Hoàn tất
                </span>
              </template>
              <template v-else-if="item.status === 'error'">
                <span
                  class="text-xs text-red-600 dark:text-red-400 truncate max-w-[200px]"
                  :title="item.message"
                >
                  ✕ {{ item.message }}
                </span>
              </template>
            </div>
          </TransitionGroup>
        </div>

        <!-- Document list -->
        <div class="flex-1 overflow-y-auto min-h-0 -mx-1">
          <div
            v-if="knowledge.loading"
            class="flex items-center justify-center py-8 text-sm text-muted-foreground"
          >
            Đang tải danh sách…
          </div>

          <div
            v-else-if="allDocs.length === 0 && uploadQueue.length === 0"
            class="flex flex-col items-center justify-center py-12 gap-2"
          >
            <span class="text-3xl opacity-30">📚</span>
            <p class="text-sm text-muted-foreground">
              Chưa có tài liệu nào. Upload file để bắt đầu.
            </p>
          </div>

          <div v-else class="divide-y divide-border">
            <div
              v-for="doc in allDocs"
              :key="doc.id"
              class="flex items-center gap-3 px-3 py-2.5 hover:bg-muted/50 transition-colors group"
            >
              <span class="shrink-0 text-base">{{ fileIcon(doc.name) }}</span>
              <div class="flex-1 min-w-0">
                <button
                  class="text-sm font-medium truncate block text-left hover:underline w-full"
                  @click="
                    emit('select', doc);
                    emit('update:open', false);
                  "
                >
                  {{ doc.name }}
                </button>
                <p class="text-[11px] text-muted-foreground mt-0.5">
                  {{ formatDate(doc.createdAt) }}
                </p>
              </div>

              <span
                class="shrink-0 text-[10px] font-medium px-1.5 py-0.5 rounded"
                :class="statusBadge(doc).cls"
              >
                {{ statusBadge(doc).label }}
              </span>

              <div
                class="shrink-0 flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity"
              >
                <button
                  v-if="doc.status === 'pending'"
                  class="p-1 rounded hover:bg-green-100 dark:hover:bg-green-900/30 text-green-600 transition-colors"
                  title="Approve"
                  @click.stop="approve(doc)"
                >
                  <svg
                    class="w-4 h-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M5 13l4 4L19 7"
                    />
                  </svg>
                </button>
                <button
                  v-if="doc.status === 'pending'"
                  class="p-1 rounded hover:bg-red-100 dark:hover:bg-red-900/30 text-red-500 transition-colors"
                  title="Reject"
                  @click.stop="reject(doc)"
                >
                  <svg
                    class="w-4 h-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M6 18L18 6M6 6l12 12"
                    />
                  </svg>
                </button>
                <button
                  class="p-1 rounded hover:bg-red-100 dark:hover:bg-red-900/30 text-red-500 transition-colors"
                  title="Xoá"
                  @click.stop="confirmDelete(doc)"
                >
                  <svg
                    class="w-4 h-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                    />
                  </svg>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Delete confirmation -->
        <Transition name="slide-up">
          <div
            v-if="deletingDoc"
            class="border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-950/30 rounded-md p-3 mt-3 flex items-center gap-3"
          >
            <span class="text-sm flex-1">
              Xoá <strong>{{ deletingDoc.name }}</strong
              >? Không thể hoàn tác.
            </span>
            <Button
              variant="outline"
              size="sm"
              @click="cancelDelete"
              :disabled="deleting"
              >Huỷ</Button
            >
            <Button
              variant="destructive"
              size="sm"
              @click="doDelete"
              :disabled="deleting"
            >
              {{ deleting ? "Đang xoá…" : "Xoá" }}
            </Button>
          </div>
        </Transition>

        <!-- Error -->
        <p v-if="knowledge.error" class="text-xs text-red-600 mt-2">
          {{ knowledge.error }}
        </p>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<style scoped>
.upload-item-enter-active,
.upload-item-leave-active {
  transition: all 0.3s ease;
}
.upload-item-enter-from {
  opacity: 0;
  transform: translateY(-8px);
}
.upload-item-leave-to {
  opacity: 0;
  transform: translateX(16px);
}
.slide-up-enter-active,
.slide-up-leave-active {
  transition: all 0.2s ease;
}
.slide-up-enter-from,
.slide-up-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>
