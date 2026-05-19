<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { marked } from "marked";
import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import go from "highlight.js/lib/languages/go";
import json from "highlight.js/lib/languages/json";
import typescript from "highlight.js/lib/languages/typescript";
import xml from "highlight.js/lib/languages/xml";
import "highlight.js/styles/github-dark.css";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Textarea } from "@/components/ui/textarea";
import { useKnowledgeStore } from "@/stores/knowledge";
import { api } from "@/api";
import type { WikiChatChunk } from "@/types";

hljs.registerLanguage("bash", bash);
hljs.registerLanguage("go", go);
hljs.registerLanguage("json", json);
hljs.registerLanguage("typescript", typescript);
hljs.registerLanguage("xml", xml);

marked.setOptions({
  breaks: true,
  gfm: true,
});

function renderMarkdown(text: string) {
  const html = marked.parse(text, {
    async: false,
  }) as string;

  const container = document.createElement("div");
  container.innerHTML = html;

  const codeBlocks = container.querySelectorAll("pre code");
  codeBlocks.forEach((block) => {
    hljs.highlightElement(block as HTMLElement);
  });

  return container.innerHTML;
}

const router = useRouter();
const knowledge = useKnowledgeStore();

const sectionKey = ref("business-flow");
const question = ref("");
const answering = ref(false);
const chatError = ref("");
const currentThread = ref<Array<{ question: string; answer: string; chunks: WikiChatChunk[] }>>([]);

const selectedFiles = ref<File[]>([]);
const fileInputRef = ref<HTMLInputElement | null>(null);
const showDuplicateDialog = ref(false);
const duplicateFiles = ref<string[]>([]);
const showKnowledgeDocs = ref(false);

const editingSessionKey = ref<string | null>(null);
const editingSessionTitle = ref("");

const selectedProject = computed({
  get: () => knowledge.selectedProjectPath,
  set: (value: string) => {
    knowledge.setSelectedProject(value);
    const existingThread = knowledge.getThread(value, sectionKey.value);
    currentThread.value = existingThread.map((t) => ({ question: t.question, answer: t.answer, chunks: [] }));
  },
});

const projectSessions = computed(() => {
  if (!selectedProject.value) return [];
  return knowledge.listSessions(selectedProject.value);
});

const canAsk = computed(() => !!selectedProject.value && question.value.trim().length > 0 && !answering.value);

watch(
  () => knowledge.selectedProjectPath,
  async () => {
    await knowledge.loadDocuments();
  },
);

onMounted(async () => {
  await knowledge.loadProjects();
  await knowledge.loadDocuments();
  if (knowledge.selectedProjectPath) {
    const existingThread = knowledge.getThread(knowledge.selectedProjectPath, sectionKey.value);
    currentThread.value = existingThread.map((t) => ({ question: t.question, answer: t.answer, chunks: [] }));
  }
});

async function askWiki() {
  if (!canAsk.value) return;
  answering.value = true;
  chatError.value = "";
  const q = question.value.trim();
  question.value = "";

  const history = currentThread.value.slice(-3).map((t) => ({ question: t.question, answer: t.answer }));
  const turnIndex = currentThread.value.length;
  currentThread.value.push({ question: q, answer: "", chunks: [] });

  try {
    const res = await api.wiki.chat({
      projectPath: selectedProject.value,
      sectionKey: sectionKey.value,
      question: q,
      history,
    });
    currentThread.value[turnIndex] = { question: q, answer: res.answer, chunks: res.chunks };
    knowledge.appendTurn(selectedProject.value, sectionKey.value, { question: q, answer: res.answer });
  } catch (e) {
    currentThread.value.splice(turnIndex, 1);
    chatError.value = e instanceof Error ? e.message : "Ask wiki failed";
  } finally {
    answering.value = false;
  }
}

function selectFiles() {
  fileInputRef.value?.click();
}

function onFileSelect(e: Event) {
  const input = e.target as HTMLInputElement;
  selectedFiles.value = Array.from(input.files ?? []);
  input.value = "";
}

async function uploadFiles() {
  if (selectedFiles.value.length === 0) return;
  const existingNames = new Set(knowledge.documents.map((d) => d.name));
  const dupes = selectedFiles.value.filter((f) => existingNames.has(f.name)).map((f) => f.name);
  if (dupes.length > 0) {
    duplicateFiles.value = dupes;
    showDuplicateDialog.value = true;
  } else {
    await knowledge.uploadFiles(selectedFiles.value, false);
    selectedFiles.value = [];
  }
}

async function confirmUpload(replace: boolean) {
  showDuplicateDialog.value = false;
  await knowledge.uploadFiles(selectedFiles.value, replace);
  selectedFiles.value = [];
  duplicateFiles.value = [];
}

function cancelUpload() {
  showDuplicateDialog.value = false;
  selectedFiles.value = [];
  duplicateFiles.value = [];
}

function newSession() {
  const newKey = `session-${Date.now()}`;
  sectionKey.value = newKey;
  knowledge.ensureSession(selectedProject.value, newKey);
  currentThread.value = [];
  question.value = "";
  chatError.value = "";
}

function switchSession(sessionKey: string) {
  sectionKey.value = sessionKey;
  const existingThread = knowledge.getThread(selectedProject.value, sessionKey);
  currentThread.value = existingThread.map((t) => ({ question: t.question, answer: t.answer, chunks: [] }));
}

function startEditSession(sessionKey: string) {
  const key = `${selectedProject.value}::${sessionKey}`;
  editingSessionKey.value = key;
  editingSessionTitle.value = knowledge.sessions[key]?.title || sessionKey;
}

function saveSessionTitle() {
  if (!editingSessionKey.value) return;
  const [projectPath, sessionKey] = editingSessionKey.value.split("::");
  knowledge.renameSession(projectPath, sessionKey, editingSessionTitle.value);
  editingSessionKey.value = null;
  editingSessionTitle.value = "";
}

function cancelEditSession() {
  editingSessionKey.value = null;
  editingSessionTitle.value = "";
}
</script>

<template>
  <div class="flex h-screen bg-muted/20 text-foreground overflow-hidden">
    <div class="w-64 border-r border-border/60 flex flex-col bg-background">
      <div class="px-4 py-3 border-b border-border/60">
        <h2 class="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Sessions</h2>
      </div>
      <ScrollArea class="flex-1 p-3">
        <div class="space-y-2">
          <div v-for="session in projectSessions" :key="session.sectionKey"
               class="group rounded-lg border p-3 cursor-pointer transition-colors"
               :class="session.sectionKey === sectionKey ? 'border-primary/40 bg-primary/5' : 'border-border/40 hover:bg-muted/30'"
               @click="switchSession(session.sectionKey)">
            <div v-if="editingSessionKey === `${session.projectPath}::${session.sectionKey}`" class="space-y-2" @click.stop>
              <input v-model="editingSessionTitle" class="w-full px-2 py-1 text-xs border border-border rounded bg-background" @keydown.enter="saveSessionTitle" @keydown.esc="cancelEditSession" />
              <div class="flex gap-1">
                <Button variant="outline" size="sm" class="h-6 text-[10px] px-2" @click="saveSessionTitle">Save</Button>
                <Button variant="ghost" size="sm" class="h-6 text-[10px] px-2" @click="cancelEditSession">Cancel</Button>
              </div>
            </div>
            <div v-else class="flex items-center justify-between gap-2">
              <div class="min-w-0 flex-1">
                <p class="text-xs font-medium truncate">{{ session.title }}</p>
                <p class="text-[11px] text-muted-foreground">{{ knowledge.getThread(session.projectPath, session.sectionKey).length }} câu hỏi</p>
              </div>
              <Button variant="ghost" size="sm" class="h-6 px-2 text-[10px] opacity-0 group-hover:opacity-100" @click.stop="startEditSession(session.sectionKey)">✎</Button>
            </div>
          </div>
        </div>
      </ScrollArea>
      <div class="p-3 border-t border-border/60">
        <Button variant="outline" size="sm" class="w-full" @click="newSession">+ New Session</Button>
      </div>
    </div>

    <div class="flex-1 flex flex-col bg-background">
      <div class="px-6 py-4 border-b border-border/60 flex items-center justify-between">
        <div class="flex items-center gap-3">
          <button class="text-xs text-muted-foreground hover:text-foreground" @click="router.push('/')">← Hub</button>
          <h1 class="text-base font-semibold">Wiki Assistant for BA</h1>
        </div>
        <div class="flex items-center gap-3">
          <select v-model="selectedProject" class="h-8 rounded-md border border-border bg-background px-2 text-xs">
            <option v-for="p in knowledge.projects" :key="p.id" :value="p.path">{{ p.name }}</option>
          </select>
          <Button variant="ghost" size="sm" @click="showKnowledgeDocs = !showKnowledgeDocs">
            {{ showKnowledgeDocs ? 'Hide' : 'Show' }} Docs
          </Button>
        </div>
      </div>

      <ScrollArea class="flex-1 px-6 py-5">
        <div class="space-y-4 max-w-4xl mx-auto">
          <div v-if="currentThread.length === 0 && !answering" class="rounded-xl border border-dashed border-border bg-muted/30 p-4">
            <p class="text-sm font-medium mb-1">Gợi ý cho BA</p>
            <p class="text-xs text-muted-foreground">Hãy hỏi theo góc nhìn nghiệp vụ, ví dụ: "Luồng VOID ảnh hưởng đến những quy tắc vận hành nào?"</p>
          </div>

          <template v-for="(turn, idx) in currentThread" :key="idx">
            <div class="flex justify-end">
              <div class="max-w-[80%] rounded-2xl rounded-br-sm bg-primary text-primary-foreground px-4 py-3 shadow-sm">
                <p class="text-[11px] opacity-80 mb-1 uppercase">User</p>
                <p class="text-sm leading-6">{{ turn.question }}</p>
              </div>
            </div>

            <div v-if="turn.answer" class="flex justify-start">
              <div class="max-w-[85%] rounded-2xl rounded-bl-sm bg-muted border border-border/60 px-4 py-3 shadow-sm space-y-3">
                <p class="text-[11px] font-semibold text-muted-foreground uppercase">Assistant</p>
                <div class="wiki-markdown prose prose-sm dark:prose-invert max-w-none text-sm leading-7" v-html="renderMarkdown(turn.answer)" />
                <div v-if="turn.chunks.length > 0" class="pt-2 border-t border-border/60">
                  <p class="text-[11px] font-semibold text-muted-foreground uppercase mb-1">Nguồn tham chiếu</p>
                  <div class="flex flex-wrap gap-1.5">
                    <span v-for="(chunk, cidx) in turn.chunks.slice(0, 4)" :key="cidx" class="text-[11px] px-2 py-1 rounded-full bg-background text-muted-foreground border border-border">
                      {{ chunk.sourceFile || `Chunk ${cidx + 1}` }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </template>

          <div v-if="answering" class="flex justify-start">
            <div class="max-w-[60%] rounded-2xl rounded-bl-sm bg-muted border border-border/60 px-4 py-3 shadow-sm">
              <p class="text-[11px] font-semibold text-muted-foreground uppercase mb-2">Assistant</p>
              <div class="flex items-center gap-2 text-sm text-muted-foreground">
                <span class="w-2 h-2 rounded-full bg-primary/70 animate-pulse" />
                <span class="w-2 h-2 rounded-full bg-primary/70 animate-pulse [animation-delay:120ms]" />
                <span class="w-2 h-2 rounded-full bg-primary/70 animate-pulse [animation-delay:240ms]" />
                <span class="ml-1">Đang xử lý dữ liệu và tổng hợp câu trả lời...</span>
              </div>
            </div>
          </div>
        </div>
      </ScrollArea>

      <div class="border-t border-border/60 px-6 py-4 bg-background">
        <div class="space-y-2 max-w-4xl mx-auto">
          <Textarea v-model="question" rows="3" placeholder="Mô tả câu hỏi nghiệp vụ cần làm rõ..." :disabled="answering" class="bg-muted/20" />
          <div class="flex items-center gap-2">
            <Button class="px-5" :disabled="!canAsk" @click="askWiki">{{ answering ? 'Đang phân tích nghiệp vụ...' : 'Gửi câu hỏi' }}</Button>
            <p v-if="chatError" class="text-xs text-destructive">{{ chatError }}</p>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showKnowledgeDocs" class="w-80 border-l border-border/60 flex flex-col bg-background">
      <div class="px-4 py-3 border-b border-border/60">
        <p class="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Knowledge documents</p>
      </div>

      <div class="p-4 space-y-2 border-b border-border/60 bg-muted/10">
        <div class="flex gap-2">
          <Button variant="outline" size="sm" @click="selectFiles" :disabled="knowledge.uploading || !selectedProject">Select</Button>
          <Button size="sm" @click="uploadFiles" :disabled="selectedFiles.length === 0 || knowledge.uploading">
            {{ knowledge.uploading ? 'Processing...' : `Upload (${selectedFiles.length})` }}
          </Button>
        </div>
        <input ref="fileInputRef" type="file" accept=".pdf,.md,.docx" multiple class="hidden" @change="onFileSelect" />
        <p v-if="selectedFiles.length > 0" class="text-[11px] text-muted-foreground truncate">{{ selectedFiles.map((f) => f.name).join(", ") }}</p>
        <p v-if="knowledge.error" class="text-[11px] text-destructive">{{ knowledge.error }}</p>
      </div>

      <ScrollArea class="flex-1 px-4 py-3">
        <div v-if="knowledge.loading" class="text-xs text-muted-foreground">Đang tải...</div>
        <div v-else-if="knowledge.documents.length === 0" class="text-xs text-muted-foreground">Chưa có tài liệu.</div>
        <div v-else class="space-y-2">
          <div v-for="doc in knowledge.documents" :key="doc.id" class="border border-border/60 rounded-lg p-2 flex items-start justify-between gap-2">
            <div class="min-w-0">
              <p class="text-xs font-medium truncate">{{ doc.name }}</p>
              <p class="text-[10px] text-muted-foreground truncate">{{ doc.sourceFile }}</p>
            </div>
            <Button variant="ghost" size="sm" class="h-6 px-2 text-[10px]" @click="knowledge.deleteDocument(doc.id)">×</Button>
          </div>
        </div>
      </ScrollArea>
    </div>

    <div v-if="showDuplicateDialog" class="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <div class="bg-background border border-border rounded-lg p-5 max-w-md w-full mx-4 space-y-3">
        <h3 class="text-sm font-semibold">File trùng tên</h3>
        <p class="text-xs text-muted-foreground">Các file sau đã tồn tại: <strong>{{ duplicateFiles.join(", ") }}</strong></p>
        <p class="text-xs text-muted-foreground">Bạn muốn replace tất cả hay bỏ qua file trùng?</p>
        <div class="flex gap-2 justify-end">
          <Button variant="outline" size="sm" @click="cancelUpload">Cancel</Button>
          <Button variant="outline" size="sm" @click="confirmUpload(false)">Skip duplicates</Button>
          <Button size="sm" @click="confirmUpload(true)">Replace all</Button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.wiki-markdown :deep(p) {
  margin: 0.5rem 0;
}

.wiki-markdown :deep(ul),
.wiki-markdown :deep(ol) {
  margin: 0.5rem 0;
  padding-left: 1.25rem;
}

.wiki-markdown :deep(code) {
  border-radius: 0.375rem;
  padding: 0.1rem 0.35rem;
  background: hsl(var(--background));
  border: 1px solid hsl(var(--border));
  font-size: 0.8em;
}

.wiki-markdown :deep(pre) {
  margin: 0.75rem 0;
  overflow-x: auto;
  border-radius: 0.5rem;
}

.wiki-markdown :deep(pre code) {
  display: block;
  padding: 0.9rem;
  border: none;
}
</style>
