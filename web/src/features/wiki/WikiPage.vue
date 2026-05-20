<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { marked, Renderer } from "marked";
import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import go from "highlight.js/lib/languages/go";
import json from "highlight.js/lib/languages/json";
import typescript from "highlight.js/lib/languages/typescript";
import xml from "highlight.js/lib/languages/xml";
import "highlight.js/styles/github.css";
import { Button } from "@/components/ui/button";
import { useKnowledgeStore } from "@/stores/knowledge";
import { useProjectStore } from "@/stores/repo";
import { api } from "@/api";
import type { KnowledgeDocument } from "@/types";
import WikiManageModal from "./WikiManageModal.vue";

hljs.registerLanguage("bash", bash);
hljs.registerLanguage("go", go);
hljs.registerLanguage("json", json);
hljs.registerLanguage("typescript", typescript);
hljs.registerLanguage("xml", xml);

// ── Marked setup with heading IDs ───────────────────────────────────────────
const renderer = new Renderer();
renderer.heading = function ({ text, depth }: { text: string; depth: number }) {
  const id = text
    .toLowerCase()
    .replace(/<[^>]+>/g, "")
    .replace(/[^\w\s-]/g, "")
    .trim()
    .replace(/\s+/g, "-");
  return `<h${depth} id="${id}">${text}</h${depth}>`;
};
marked.use({ renderer, breaks: true, gfm: true });

function renderMarkdown(text: string): string {
  const html = marked.parse(text, { async: false }) as string;
  const el = document.createElement("div");
  el.innerHTML = html;
  el.querySelectorAll("pre code").forEach((block) =>
    hljs.highlightElement(block as HTMLElement),
  );
  return el.innerHTML;
}

// ── TOC extraction ───────────────────────────────────────────────────────────
interface TocEntry {
  level: number;
  text: string;
  id: string;
}

function extractToc(markdown: string): TocEntry[] {
  return markdown
    .split("\n")
    .filter((l) => /^#{1,6} /.test(l))
    .map((l) => {
      const m = l.match(/^(#{1,6}) (.+)/);
      if (!m) return null;
      const text = m[2].trim();
      const id = text
        .toLowerCase()
        .replace(/[^\w\s-]/g, "")
        .trim()
        .replace(/\s+/g, "-");
      return { level: m[1].length, text, id } as TocEntry;
    })
    .filter(Boolean) as TocEntry[];
}

// ── State ────────────────────────────────────────────────────────────────────
const router = useRouter();
const knowledge = useKnowledgeStore();
const projectStore = useProjectStore();

const route = useRoute();
const projectId = computed(() => route.params.projectId as string);

// Document viewer
const selectedDoc = ref<KnowledgeDocument | null>(null);
const docContent = ref("");
const docIsMarkdown = ref(true);
const docLoading = ref(false);
const toc = computed(() =>
  docIsMarkdown.value ? extractToc(docContent.value) : [],
);

// Wiki manage modal
const showManageModal = ref(false);

// Chat
const chatInput = ref("");

const allDocs = computed(() => [
  ...knowledge.pendingDocuments.map((d) => ({ ...d, _pending: true })),
  ...knowledge.documents,
]);

// ── Document loading ─────────────────────────────────────────────────────────
async function selectDoc(doc: KnowledgeDocument) {
  if (selectedDoc.value?.id === doc.id) return;
  selectedDoc.value = doc;
  docContent.value = "";
  docLoading.value = true;
  try {
    const res = await api.wiki.getDocumentContent(doc.id, projectId.value);
    docContent.value = res.content;
    docIsMarkdown.value = res.isMarkdown;
  } catch {
    docContent.value = "Không thể tải nội dung file.";
    docIsMarkdown.value = false;
  } finally {
    docLoading.value = false;
  }
}

function scrollToHeading(id: string) {
  document
    .getElementById(id)
    ?.scrollIntoView({ behavior: "smooth", block: "start" });
}

function onModalSelect(doc: KnowledgeDocument) {
  selectDoc(doc);
}

// ── Chat ─────────────────────────────────────────────────────────────────────
function navigateToChat() {
  const q = chatInput.value.trim();
  if (!q) return;
  router.push({
    path: `/projects/${projectId.value}/features/wiki/chat`,
    query: { q },
  });
}

// ── Lifecycle ────────────────────────────────────────────────────────────────
onMounted(async () => {
  await projectStore.fetch();
  if (projectId.value) {
    projectStore.selectProject(projectId.value);
    await knowledge.loadDocuments(projectId.value);
    if (allDocs.value.length > 0) selectDoc(allDocs.value[0]);
  }
});

watch(projectId, async (p) => {
  if (p) {
    await knowledge.loadDocuments(p);
    if (allDocs.value.length > 0) selectDoc(allDocs.value[0]);
  }
});
</script>

<template>
  <div
    class="flex flex-col h-screen bg-background text-foreground overflow-hidden"
  >
    <!-- ── Header ── -->
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
        <span class="text-sm font-semibold">Wiki</span>
        <span
          v-if="projectStore.selectedProject"
          class="text-xs text-muted-foreground font-mono"
          >{{ projectStore.selectedProject.name }}</span
        >
      </div>
      <div class="flex items-center gap-2">
        <button
          class="inline-flex items-center gap-1.5 text-xs border border-border rounded px-2.5 py-1 hover:bg-muted transition-colors"
          @click="showManageModal = true"
        >
          📚 Quản lý Wiki
        </button>
      </div>
    </header>

    <!-- ── Main 3-column ── -->
    <div class="flex flex-1 overflow-hidden">
      <!-- Left: file list -->
      <aside
        class="w-60 border-r border-border flex flex-col bg-background overflow-hidden shrink-0"
      >
        <div class="px-3 pt-3 pb-1">
          <p
            class="text-[11px] font-semibold text-muted-foreground uppercase tracking-wider"
          >
            Documents
          </p>
        </div>

        <div
          v-if="knowledge.loading"
          class="px-3 py-2 text-xs text-muted-foreground"
        >
          Đang tải…
        </div>

        <nav class="flex-1 overflow-y-auto py-1">
          <div v-if="knowledge.pendingDocuments.length > 0" class="mb-1">
            <p
              class="px-3 py-1 text-[10px] font-semibold text-amber-600 uppercase tracking-wider"
            >
              ⏳ Pending
            </p>
            <button
              v-for="doc in knowledge.pendingDocuments"
              :key="doc.id"
              class="w-full text-left px-3 py-1.5 text-xs rounded-sm mx-1 transition-colors hover:bg-muted flex items-center gap-2"
              :class="
                selectedDoc?.id === doc.id
                  ? 'bg-primary/10 text-primary font-medium'
                  : 'text-muted-foreground'
              "
              @click="selectDoc(doc)"
            >
              <span class="shrink-0">{{
                doc.name.endsWith(".md")
                  ? "📄"
                  : doc.name.endsWith(".pdf")
                    ? "📕"
                    : "📝"
              }}</span>
              <span class="truncate">{{ doc.name }}</span>
            </button>
            <div class="px-3 pt-1 pb-2 flex gap-1">
              <Button
                variant="outline"
                size="sm"
                class="h-6 text-[10px] px-2 flex-1"
                :disabled="knowledge.pendingDocuments.length === 0"
                @click="knowledge.approveAll(projectId)"
                >Approve all</Button
              >
            </div>
          </div>

          <div
            v-if="
              knowledge.documents.filter((d) => d.status === 'approved')
                .length > 0
            "
          >
            <button
              v-for="doc in knowledge.documents.filter(
                (d) => d.status === 'approved',
              )"
              :key="doc.id"
              class="w-full text-left px-3 py-1.5 text-xs rounded-sm mx-1 transition-colors hover:bg-muted flex items-center gap-2"
              :class="
                selectedDoc?.id === doc.id
                  ? 'bg-primary/10 text-primary font-medium'
                  : 'text-foreground'
              "
              @click="selectDoc(doc)"
            >
              <span class="shrink-0">{{
                doc.name.endsWith(".md")
                  ? "📄"
                  : doc.name.endsWith(".pdf")
                    ? "📕"
                    : "📝"
              }}</span>
              <span class="truncate">{{ doc.name }}</span>
            </button>
          </div>

          <div
            v-if="allDocs.length === 0"
            class="px-3 py-4 text-xs text-muted-foreground"
          >
            Chưa có tài liệu. Upload file để bắt đầu.
          </div>
        </nav>
      </aside>

      <!-- Center: document content -->
      <main class="flex-1 overflow-y-auto pb-24">
        <!-- Empty state -->
        <div
          v-if="!selectedDoc"
          class="flex flex-col items-center justify-center h-full text-center px-8 gap-3"
        >
          <span class="text-4xl opacity-30">📚</span>
          <p class="text-sm text-muted-foreground">
            Chọn một tài liệu từ danh sách bên trái để xem nội dung
          </p>
        </div>

        <!-- Loading -->
        <div
          v-else-if="docLoading"
          class="flex items-center justify-center h-32"
        >
          <span class="text-sm text-muted-foreground">Đang tải nội dung…</span>
        </div>

        <!-- Markdown content -->
        <article v-else class="max-w-3xl mx-auto px-8 py-8">
          <h1 class="text-2xl font-bold mb-6 pb-3 border-b border-border">
            {{ selectedDoc.name }}
          </h1>
          <div
            v-if="docIsMarkdown"
            class="wiki-content prose prose-neutral dark:prose-invert max-w-none"
            v-html="renderMarkdown(docContent)"
          />
          <pre
            v-else
            class="text-xs whitespace-pre-wrap font-mono text-muted-foreground"
            >{{ docContent }}</pre
          >
        </article>
      </main>

      <!-- Right: TOC -->
      <aside
        v-if="toc.length > 0"
        class="w-52 border-l border-border overflow-y-auto shrink-0 py-4 px-3 hidden lg:block"
      >
        <p
          class="text-[11px] font-semibold text-muted-foreground uppercase tracking-wider mb-3"
        >
          On this page
        </p>
        <nav class="space-y-0.5">
          <button
            v-for="entry in toc"
            :key="entry.id"
            class="w-full text-left text-xs text-muted-foreground hover:text-foreground transition-colors py-0.5 truncate block"
            :style="{ paddingLeft: `${(entry.level - 1) * 10}px` }"
            @click="scrollToHeading(entry.id)"
          >
            {{ entry.text }}
          </button>
        </nav>
      </aside>
    </div>

    <!-- ── Bottom fixed: chat bar ── -->
    <div
      class="fixed bottom-4 left-0 right-0 z-20 flex justify-center pointer-events-none"
    >
      <div
        class="w-full max-w-[600px] mx-4 pointer-events-auto border border-border rounded-xl bg-background/95 backdrop-blur shadow-lg h-12"
      >
        <div class="h-full flex items-center px-4 gap-3">
          <svg
            class="w-4 h-4 text-muted-foreground shrink-0"
            viewBox="0 0 16 16"
            fill="currentColor"
          >
            <path
              d="M2.678 11.894a1 1 0 0 1 .287.801 10.97 10.97 0 0 1-.398 2c1.395-.323 2.247-.697 2.634-.893a1 1 0 0 1 .71-.074A8.06 8.06 0 0 0 8 14c3.996 0 7-2.807 7-6 0-3.192-3.004-6-7-6S1 4.808 1 8c0 1.468.617 2.83 1.678 3.894zm-.493 3.905a21.682 21.682 0 0 1-.713.129c-.2.032-.352-.176-.273-.362a9.68 9.68 0 0 0 .244-.637l.003-.01c.248-.72.45-1.548.524-2.319C.743 11.37 0 9.76 0 8c0-3.866 3.582-7 8-7s8 3.134 8 7-3.582 7-8 7a9.06 9.06 0 0 1-2.347-.306c-.52.263-1.639.742-3.468 1.105z"
            />
          </svg>
          <input
            v-model="chatInput"
            type="text"
            class="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            :placeholder="`Ask about ${projectStore.selectedProject?.name ?? 'this project'}...`"
            @keydown.enter="navigateToChat"
          />
          <button
            v-if="chatInput.trim()"
            class="text-xs px-3 py-1.5 bg-primary text-primary-foreground rounded-md hover:bg-primary/90 transition-colors"
            @click="navigateToChat"
          >
            Send
          </button>
        </div>
      </div>
    </div>

    <!-- ── Wiki Manage Modal ── -->
    <WikiManageModal
      v-model:open="showManageModal"
      :project-id="projectId"
      @select="onModalSelect"
    />
  </div>
</template>

<style scoped>
.wiki-content :deep(h1) {
  font-size: 1.6em;
  font-weight: 700;
  margin: 1.5rem 0 0.75rem;
}
.wiki-content :deep(h2) {
  font-size: 1.3em;
  font-weight: 600;
  margin: 1.4rem 0 0.6rem;
  padding-bottom: 0.3rem;
  border-bottom: 1px solid hsl(var(--border));
}
.wiki-content :deep(h3) {
  font-size: 1.1em;
  font-weight: 600;
  margin: 1.2rem 0 0.5rem;
}
.wiki-content :deep(h4),
.wiki-content :deep(h5),
.wiki-content :deep(h6) {
  font-weight: 600;
  margin: 1rem 0 0.4rem;
}
.wiki-content :deep(p) {
  margin: 0.6rem 0;
  line-height: 1.7;
}
.wiki-content :deep(ul),
.wiki-content :deep(ol) {
  margin: 0.5rem 0;
  padding-left: 1.5rem;
}
.wiki-content :deep(li) {
  margin: 0.25rem 0;
  line-height: 1.6;
}
.wiki-content :deep(blockquote) {
  border-left: 3px solid hsl(var(--border));
  padding-left: 1rem;
  color: hsl(var(--muted-foreground));
  margin: 0.75rem 0;
}
.wiki-content :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 1rem 0;
  font-size: 0.9em;
}
.wiki-content :deep(th) {
  background: hsl(var(--muted));
  font-weight: 600;
  text-align: left;
  padding: 0.5rem 0.75rem;
  border: 1px solid hsl(var(--border));
}
.wiki-content :deep(td) {
  padding: 0.4rem 0.75rem;
  border: 1px solid hsl(var(--border));
}
.wiki-content :deep(code) {
  background: hsl(var(--muted));
  border-radius: 0.3rem;
  padding: 0.1rem 0.35rem;
  font-size: 0.85em;
  font-family: monospace;
}
.wiki-content :deep(pre) {
  margin: 0.75rem 0;
  border-radius: 0.5rem;
  overflow-x: auto;
}
.wiki-content :deep(pre code) {
  display: block;
  padding: 0.9rem;
  background: transparent;
}
.wiki-content :deep(hr) {
  border: none;
  border-top: 1px solid hsl(var(--border));
  margin: 1.5rem 0;
}
.wiki-content :deep(a) {
  color: hsl(var(--primary));
  text-decoration: underline;
  text-underline-offset: 2px;
}
</style>
