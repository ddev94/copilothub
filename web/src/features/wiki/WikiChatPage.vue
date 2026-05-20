<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from "vue";
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
import { Textarea } from "@/components/ui/textarea";
import { useKnowledgeStore } from "@/stores/knowledge";
import { useProjectStore } from "@/stores/repo";
import { api } from "@/api";
import type { WikiChatChunk } from "@/types";

hljs.registerLanguage("bash", bash);
hljs.registerLanguage("go", go);
hljs.registerLanguage("json", json);
hljs.registerLanguage("typescript", typescript);
hljs.registerLanguage("xml", xml);

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

// ── State ────────────────────────────────────────────────────────────────────
const route = useRoute();
const router = useRouter();
const knowledge = useKnowledgeStore();
const projectStore = useProjectStore();

const projectId = computed(() => route.params.projectId as string);
const sectionKey = ref(`wiki-chat-${Date.now()}`);

const messages = ref<
  Array<{ question: string; answer: string; chunks: WikiChatChunk[] }>
>([]);
const input = ref("");
const answering = ref(false);
const chatError = ref("");
const scrollRef = ref<HTMLElement | null>(null);
const inputRef = ref<HTMLTextAreaElement | null>(null);

const canSend = computed(
  () => input.value.trim().length > 0 && !answering.value,
);

async function send(question?: string) {
  const q = (question ?? input.value).trim();
  if (!q || answering.value) return;
  input.value = "";
  answering.value = true;
  chatError.value = "";

  const history = messages.value
    .slice(-4)
    .map((m) => ({ question: m.question, answer: m.answer }));
  const idx = messages.value.length;
  messages.value.push({ question: q, answer: "", chunks: [] });

  await nextTick();
  scrollRef.value?.scrollTo({
    top: scrollRef.value.scrollHeight,
    behavior: "smooth",
  });

  try {
    const res = await api.wiki.chat({
      projectId: projectId.value,
      sectionKey: sectionKey.value,
      question: q,
      history,
    });
    const answer =
      res.answer?.trim() ||
      "Không tìm thấy thông tin liên quan. Hãy upload/approve thêm tài liệu.";
    messages.value[idx] = { question: q, answer, chunks: res.chunks ?? [] };
    knowledge.appendTurn(projectId.value, sectionKey.value, {
      question: q,
      answer,
    });
  } catch (e) {
    messages.value.splice(idx, 1);
    chatError.value = e instanceof Error ? e.message : "Chat failed";
  } finally {
    answering.value = false;
    await nextTick();
    scrollRef.value?.scrollTo({
      top: scrollRef.value.scrollHeight,
      behavior: "smooth",
    });
  }
}

onMounted(async () => {
  await projectStore.fetch();
  const initialQ = route.query.q as string | undefined;
  if (initialQ) {
    await send(initialQ);
  } else {
    inputRef.value?.focus();
  }
});
</script>

<template>
  <div
    class="flex flex-col h-screen bg-background text-foreground overflow-hidden"
  >
    <!-- Header -->
    <header
      class="h-11 border-b border-border flex items-center justify-between px-4 shrink-0"
    >
      <div class="flex items-center gap-3">
        <button
          class="text-xs text-muted-foreground hover:text-foreground transition-colors"
          @click="router.push(`/projects/${projectId}/features/wiki`)"
        >
          ← Wiki
        </button>
        <span class="text-sm font-semibold">Chat</span>
        <span
          v-if="projectStore.selectedProject"
          class="text-xs text-muted-foreground font-mono"
          >{{ projectStore.selectedProject.name }}</span
        >
      </div>
    </header>

    <!-- Messages -->
    <div ref="scrollRef" class="flex-1 overflow-y-auto px-4 py-4">
      <div class="max-w-[700px] mx-auto space-y-4">
        <div
          v-if="messages.length === 0"
          class="flex flex-col items-center justify-center h-full min-h-[200px] gap-2 text-center"
        >
          <span class="text-3xl opacity-20">💬</span>
          <p class="text-sm text-muted-foreground">
            Hỏi bất kỳ điều gì về tài liệu trong project này…
          </p>
        </div>

        <template v-for="(msg, idx) in messages" :key="idx">
          <!-- Question -->
          <div class="flex justify-end">
            <div
              class="max-w-[75%] bg-primary text-primary-foreground rounded-2xl rounded-br-sm px-4 py-2.5 text-sm"
            >
              {{ msg.question }}
            </div>
          </div>

          <!-- Answer -->
          <div v-if="msg.answer" class="flex justify-start">
            <div
              class="max-w-[80%] bg-muted border border-border/60 rounded-2xl rounded-bl-sm px-4 py-2.5 text-sm leading-relaxed wiki-content"
              v-html="renderMarkdown(msg.answer)"
            />
          </div>

          <!-- Thinking indicator -->
          <div
            v-else-if="idx === messages.length - 1 && answering"
            class="flex justify-start"
          >
            <div
              class="bg-muted border border-border/60 rounded-2xl rounded-bl-sm px-4 py-3 flex gap-1.5"
            >
              <span class="w-2 h-2 rounded-full bg-primary/60 animate-pulse" />
              <span
                class="w-2 h-2 rounded-full bg-primary/60 animate-pulse [animation-delay:150ms]"
              />
              <span
                class="w-2 h-2 rounded-full bg-primary/60 animate-pulse [animation-delay:300ms]"
              />
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- Input bar -->
    <div class="border-t border-border px-4 py-3 shrink-0">
      <div class="max-w-[700px] mx-auto flex gap-2 items-end">
        <Textarea
          ref="inputRef"
          v-model="input"
          rows="2"
          placeholder="Hỏi về tài liệu trong project này..."
          class="flex-1 resize-none text-sm bg-muted/30"
          :disabled="answering"
          @keydown.enter.exact.prevent="send()"
        />
        <Button size="sm" :disabled="!canSend" class="shrink-0" @click="send()"
          >Gửi</Button
        >
      </div>
      <p
        v-if="chatError"
        class="max-w-[700px] mx-auto text-[11px] text-destructive mt-1"
      >
        {{ chatError }}
      </p>
    </div>
  </div>
</template>

<style scoped>
.wiki-content :deep(h1) {
  font-size: 1.4em;
  font-weight: 700;
  margin: 1rem 0 0.5rem;
}
.wiki-content :deep(h2) {
  font-size: 1.15em;
  font-weight: 600;
  margin: 1rem 0 0.4rem;
}
.wiki-content :deep(h3) {
  font-size: 1em;
  font-weight: 600;
  margin: 0.8rem 0 0.3rem;
}
.wiki-content :deep(p) {
  margin: 0.4rem 0;
  line-height: 1.65;
}
.wiki-content :deep(ul),
.wiki-content :deep(ol) {
  margin: 0.4rem 0;
  padding-left: 1.25rem;
}
.wiki-content :deep(li) {
  margin: 0.2rem 0;
  line-height: 1.55;
}
.wiki-content :deep(blockquote) {
  border-left: 3px solid hsl(var(--border));
  padding-left: 0.75rem;
  color: hsl(var(--muted-foreground));
  margin: 0.5rem 0;
}
.wiki-content :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 0.75rem 0;
  font-size: 0.875em;
}
.wiki-content :deep(th) {
  background: hsl(var(--muted));
  font-weight: 600;
  text-align: left;
  padding: 0.4rem 0.6rem;
  border: 1px solid hsl(var(--border));
}
.wiki-content :deep(td) {
  padding: 0.35rem 0.6rem;
  border: 1px solid hsl(var(--border));
}
.wiki-content :deep(code) {
  background: hsl(var(--muted));
  border-radius: 0.25rem;
  padding: 0.1rem 0.3rem;
  font-size: 0.85em;
  font-family: monospace;
}
.wiki-content :deep(pre) {
  margin: 0.5rem 0;
  border-radius: 0.4rem;
  overflow-x: auto;
}
.wiki-content :deep(pre code) {
  display: block;
  padding: 0.75rem;
  background: transparent;
}
.wiki-content :deep(a) {
  color: hsl(var(--primary));
  text-decoration: underline;
  text-underline-offset: 2px;
}
</style>
