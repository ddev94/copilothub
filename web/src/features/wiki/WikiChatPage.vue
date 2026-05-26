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
import type { WikiChatChunk, WikiThinkingEvent } from "@/types";

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

type ChatMessage = {
  question: string;
  answer: string;
  chunks: WikiChatChunk[];
  steps: WikiThinkingEvent[];
  showThinking: boolean;
  expandedSteps: Set<string>;
};

const route = useRoute();
const router = useRouter();
const knowledge = useKnowledgeStore();
const projectStore = useProjectStore();

const projectId = computed(() => route.params.projectId as string);
const sectionKey = ref(`wiki-chat-${Date.now()}`);

const messages = ref<ChatMessage[]>([]);
const input = ref("");
const answering = ref(false);
const chatError = ref("");
const scrollRef = ref<HTMLElement | null>(null);
const inputRef = ref<HTMLTextAreaElement | null>(null);

const canSend = computed(() => input.value.trim().length > 0 && !answering.value);

function upsertStep(msg: ChatMessage, event: WikiThinkingEvent) {
  const idx = msg.steps.findIndex((s) => s.step === event.step);
  if (idx >= 0) msg.steps[idx] = event;
  else msg.steps.push(event);
}

async function send(question?: string) {
  const q = (question ?? input.value).trim();
  if (!q || answering.value) return;
  input.value = "";
  answering.value = true;
  chatError.value = "";

  const history = messages.value.slice(-4).map((m) => ({ question: m.question, answer: m.answer }));
  const idx = messages.value.length;
  messages.value.push({ question: q, answer: "", chunks: [], steps: [], showThinking: false, expandedSteps: new Set() });

  await nextTick();
  scrollRef.value?.scrollTo({ top: scrollRef.value.scrollHeight, behavior: "smooth" });

  try {
    await api.wiki.chatStream(
      { projectId: projectId.value, sectionKey: sectionKey.value, question: q, history },
      {
        onStep: (event) => {
          upsertStep(messages.value[idx], event);
        },
        onFinal: (res) => {
          const answer = res.answer?.trim() || "Không tìm thấy thông tin liên quan. Hãy upload/approve thêm tài liệu.";
          messages.value[idx].answer = answer;
          messages.value[idx].chunks = res.chunks ?? [];
          knowledge.appendTurn(projectId.value, sectionKey.value, { question: q, answer });
        },
      },
    );
  } catch {
    try {
      const res = await api.wiki.chat({ projectId: projectId.value, sectionKey: sectionKey.value, question: q, history });
      const answer = res.answer?.trim() || "Không tìm thấy thông tin liên quan. Hãy upload/approve thêm tài liệu.";
      messages.value[idx].answer = answer;
      messages.value[idx].chunks = res.chunks ?? [];
      knowledge.appendTurn(projectId.value, sectionKey.value, { question: q, answer });
    } catch (e) {
      messages.value.splice(idx, 1);
      chatError.value = e instanceof Error ? e.message : "Chat failed";
    }
  } finally {
    answering.value = false;
    await nextTick();
    scrollRef.value?.scrollTo({ top: scrollRef.value.scrollHeight, behavior: "smooth" });
  }
}

onMounted(async () => {
  await projectStore.fetch();
  const initialQ = route.query.q as string | undefined;
  if (initialQ) await send(initialQ);
  else inputRef.value?.focus();
});
</script>

<template>
  <div class="flex flex-col h-screen bg-background text-foreground overflow-hidden">
    <header class="h-11 border-b border-border flex items-center justify-between px-4 shrink-0">
      <div class="flex items-center gap-3">
        <button class="text-xs text-muted-foreground hover:text-foreground transition-colors" @click="router.push(`/projects/${projectId}/features/wiki`)">← Wiki</button>
        <span class="text-sm font-semibold">Chat</span>
      </div>
    </header>

    <div ref="scrollRef" class="flex-1 overflow-y-auto px-4 py-6">
      <div class="max-w-[740px] mx-auto space-y-6">
        <template v-for="(msg, idx) in messages" :key="idx">
          <!-- User question -->
          <div class="flex justify-end">
            <div class="max-w-[75%] bg-primary text-primary-foreground rounded-2xl rounded-br-sm px-4 py-2.5 text-sm shadow-sm">{{ msg.question }}</div>
          </div>

          <!-- Thinking steps (collapsible, above answer) -->
          <div v-if="msg.steps.length" class="flex justify-start">
            <div class="max-w-[85%] w-full">
              <button
                class="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors group"
                @click="msg.showThinking = !msg.showThinking"
              >
                <svg :class="['w-3.5 h-3.5 transition-transform duration-200', msg.showThinking ? 'rotate-90' : '']" viewBox="0 0 16 16" fill="currentColor"><path d="M6.22 4.22a.75.75 0 0 1 1.06 0l3.25 3.25a.75.75 0 0 1 0 1.06l-3.25 3.25a.75.75 0 0 1-1.06-1.06L8.94 8 6.22 5.28a.75.75 0 0 1 0-1.06Z"/></svg>
                <span>{{ msg.showThinking ? 'Ẩn' : 'Hiện' }} thinking steps</span>
                <span class="text-[10px] opacity-60">({{ msg.steps.length }})</span>
              </button>
              <div v-if="msg.showThinking" class="mt-2 ml-1 pl-3 border-l-2 border-border/70 space-y-1">
                <div v-for="(step, sidx) in msg.steps" :key="`${step.step}-${sidx}`" class="text-xs">
                  <button
                    class="flex items-start gap-2 py-1 w-full text-left hover:bg-muted/50 rounded px-1 -ml-1 transition-colors"
                    @click="msg.expandedSteps.has(step.step) ? msg.expandedSteps.delete(step.step) : msg.expandedSteps.add(step.step)"
                  >
                    <span v-if="step.status === 'completed'" class="mt-0.5 shrink-0 w-3.5 h-3.5 rounded-full bg-emerald-500/15 text-emerald-600 flex items-center justify-center">
                      <svg class="w-2.5 h-2.5" viewBox="0 0 16 16" fill="currentColor"><path d="M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.75.75 0 0 1 1.06-1.06L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0Z"/></svg>
                    </span>
                    <span v-else-if="step.status === 'started'" class="mt-0.5 shrink-0 w-3.5 h-3.5 rounded-full bg-blue-500/15 text-blue-600 flex items-center justify-center">
                      <svg class="w-2.5 h-2.5 animate-spin" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2"><circle cx="8" cy="8" r="5" stroke-dasharray="20 10" /></svg>
                    </span>
                    <span v-else-if="step.status === 'failed'" class="mt-0.5 shrink-0 w-3.5 h-3.5 rounded-full bg-red-500/15 text-red-600 flex items-center justify-center">
                      <svg class="w-2.5 h-2.5" viewBox="0 0 16 16" fill="currentColor"><path d="M3.72 3.72a.75.75 0 0 1 1.06 0L8 6.94l3.22-3.22a.75.75 0 1 1 1.06 1.06L9.06 8l3.22 3.22a.75.75 0 1 1-1.06 1.06L8 9.06l-3.22 3.22a.75.75 0 0 1-1.06-1.06L6.94 8 3.72 4.78a.75.75 0 0 1 0-1.06Z"/></svg>
                    </span>
                    <span v-else class="mt-0.5 shrink-0 w-3.5 h-3.5 rounded-full bg-muted text-muted-foreground flex items-center justify-center">
                      <span class="w-1.5 h-1.5 rounded-full bg-current" />
                    </span>
                    <div class="min-w-0 flex-1">
                      <span class="font-medium text-foreground/80">{{ step.step }}</span>
                      <span v-if="step.summary" class="text-muted-foreground ml-1.5">{{ step.summary }}</span>
                    </div>
                    <svg v-if="step.data" :class="['w-3 h-3 shrink-0 mt-0.5 text-muted-foreground transition-transform duration-150', msg.expandedSteps.has(step.step) ? 'rotate-90' : '']" viewBox="0 0 16 16" fill="currentColor"><path d="M6.22 4.22a.75.75 0 0 1 1.06 0l3.25 3.25a.75.75 0 0 1 0 1.06l-3.25 3.25a.75.75 0 0 1-1.06-1.06L8.94 8 6.22 5.28a.75.75 0 0 1 0-1.06Z"/></svg>
                  </button>
                  <div v-if="step.data && msg.expandedSteps.has(step.step)" class="ml-6 mt-1 mb-2 rounded-md bg-muted/60 border border-border/50 p-2 overflow-x-auto">
                    <pre class="text-[11px] leading-relaxed text-muted-foreground whitespace-pre-wrap break-words">{{ JSON.stringify(step.data, null, 2) }}</pre>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Answer -->
          <div class="flex justify-start">
            <div class="max-w-[85%] w-full">
              <div v-if="msg.answer" class="wiki-answer prose prose-sm dark:prose-invert max-w-none" v-html="renderMarkdown(msg.answer)" />
              <div v-else-if="idx === messages.length - 1 && answering" class="flex items-center gap-2 py-2">
                <span class="flex gap-1">
                  <span class="w-2 h-2 rounded-full bg-primary/60 animate-bounce" />
                  <span class="w-2 h-2 rounded-full bg-primary/60 animate-bounce [animation-delay:150ms]" />
                  <span class="w-2 h-2 rounded-full bg-primary/60 animate-bounce [animation-delay:300ms]" />
                </span>
                <span class="text-xs text-muted-foreground">Đang suy nghĩ...</span>
              </div>

              <!-- Source references -->
              <div v-if="msg.chunks.length" class="mt-3 pt-2 border-t border-border/40">
                <div class="flex items-center gap-1.5 text-[11px] text-muted-foreground font-medium mb-1.5">
                  <svg class="w-3 h-3" viewBox="0 0 16 16" fill="currentColor"><path d="M1 3.5A1.5 1.5 0 0 1 2.5 2h3.879a1.5 1.5 0 0 1 1.06.44l1.122 1.12A1.5 1.5 0 0 0 9.62 4H13.5A1.5 1.5 0 0 1 15 5.5v7a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 1 12.5v-9Z"/></svg>
                  <span>{{ msg.chunks.length }} nguồn tham khảo</span>
                </div>
                <div class="flex flex-wrap gap-1.5">
                  <span
                    v-for="(chunk, cidx) in msg.chunks"
                    :key="cidx"
                    class="inline-flex items-center gap-1 text-[10px] px-2 py-0.5 rounded-full bg-muted border border-border/50 text-muted-foreground"
                    :title="chunk.content"
                  >
                    <svg class="w-2.5 h-2.5 shrink-0" viewBox="0 0 16 16" fill="currentColor"><path d="M3.75 1.5a.25.25 0 0 0-.25.25v11.5c0 .138.112.25.25.25h8.5a.25.25 0 0 0 .25-.25V6H9.75A1.75 1.75 0 0 1 8 4.25V1.5H3.75ZM9.5 1.793V4.25c0 .138.112.25.25.25h2.457L9.5 1.793ZM2 1.75C2 .784 2.784 0 3.75 0h5.086c.464 0 .909.184 1.237.513l3.414 3.414c.329.328.513.773.513 1.237v8.086A1.75 1.75 0 0 1 12.25 15h-8.5A1.75 1.75 0 0 1 2 13.25V1.75Z"/></svg>
                    {{ chunk.sourceFile || 'unknown' }}
                    <span class="opacity-50">{{ chunk.score?.toFixed?.(2) ?? '' }}</span>
                  </span>
                </div>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>

    <div class="border-t border-border px-4 py-3 shrink-0">
      <div class="max-w-[700px] mx-auto flex gap-2 items-end">
        <Textarea ref="inputRef" v-model="input" rows="2" placeholder="Hỏi về tài liệu trong project này..." class="flex-1 resize-none text-sm bg-muted/30" :disabled="answering" @keydown.enter.exact.prevent="send()" />
        <Button size="sm" :disabled="!canSend" class="shrink-0" @click="send()">Gửi</Button>
      </div>
      <p v-if="chatError" class="max-w-[700px] mx-auto text-[11px] text-destructive mt-1">{{ chatError }}</p>
    </div>
  </div>
</template>

<style scoped>
.wiki-answer :deep(h1),
.wiki-answer :deep(h2),
.wiki-answer :deep(h3),
.wiki-answer :deep(h4) {
  margin-top: 1rem;
  margin-bottom: 0.4rem;
  font-weight: 600;
  line-height: 1.4;
}
.wiki-answer :deep(h1) { font-size: 1.25rem; }
.wiki-answer :deep(h2) { font-size: 1.1rem; }
.wiki-answer :deep(h3) { font-size: 1rem; }
.wiki-answer :deep(p) { margin: 0.4rem 0; line-height: 1.65; font-size: 0.875rem; }
.wiki-answer :deep(ul),
.wiki-answer :deep(ol) { margin: 0.4rem 0; padding-left: 1.5rem; font-size: 0.875rem; }
.wiki-answer :deep(li) { margin: 0.2rem 0; line-height: 1.6; }
.wiki-answer :deep(pre) {
  margin: 0.6rem 0;
  border-radius: 0.5rem;
  overflow-x: auto;
  background: hsl(var(--muted));
  padding: 0.75rem 1rem;
  font-size: 0.8rem;
  border: 1px solid hsl(var(--border) / 0.5);
}
.wiki-answer :deep(code) {
  font-size: 0.8rem;
  background: hsl(var(--muted));
  padding: 0.15rem 0.35rem;
  border-radius: 0.25rem;
}
.wiki-answer :deep(pre code) {
  background: none;
  padding: 0;
}
.wiki-answer :deep(blockquote) {
  border-left: 3px solid hsl(var(--primary) / 0.4);
  padding-left: 0.75rem;
  margin: 0.5rem 0;
  color: hsl(var(--muted-foreground));
  font-style: italic;
}
.wiki-answer :deep(table) {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8rem;
  margin: 0.5rem 0;
}
.wiki-answer :deep(th),
.wiki-answer :deep(td) {
  border: 1px solid hsl(var(--border));
  padding: 0.4rem 0.6rem;
  text-align: left;
}
.wiki-answer :deep(th) {
  background: hsl(var(--muted));
  font-weight: 600;
}
.wiki-answer :deep(strong) { font-weight: 600; }
.wiki-answer :deep(a) { color: hsl(var(--primary)); text-decoration: underline; }
</style>
