<script setup lang="ts">
import { ref, computed, nextTick } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useProjectStore } from "@/stores/repo";
import { api } from "@/api";
import { Button } from "@/components/ui/button";
import { Select, SelectTrigger, SelectContent, SelectItem, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { ClarifyResponse, ToolEvent } from "@/types";
import { marked } from "marked";

function renderMd(text: string): string {
  return marked.parse(text, { async: false }) as string;
}

const route = useRoute();
const router = useRouter();
const projectStore = useProjectStore();
projectStore.fetch();

const availableModels = ref<string[]>([]);
const selectedModel = ref("");
api.models.list().then(({ models, current }) => {
  availableModels.value = models;
  selectedModel.value = current;
});

const projectId = computed(() => route.params.projectId as string);
const currentProject = computed(
  () => projectStore.projects.find((p) => p.id === projectId.value) ?? null,
);
const projectRepos = computed(() => currentProject.value?.repositories ?? []);

// ── Input state ──────────────────────────────────────────────────────
const specText = ref("");
const inputPreview = ref(false);
type ClarifyMode = "source" | "wiki" | "both";
const clarifyMode = ref<ClarifyMode>("source");

// Repos explicitly excluded (empty = use all repos)
const deselectedRepoIds = ref<string[]>([]);

function toggleRepo(repoId: string) {
  const idx = deselectedRepoIds.value.indexOf(repoId);
  if (idx === -1) {
    deselectedRepoIds.value.push(repoId); // exclude this repo
  } else {
    deselectedRepoIds.value.splice(idx, 1); // re-include
  }
}

// IDs to send to API: all repos minus deselected; undefined = use all
const effectiveRepoIds = computed<string[] | undefined>(() => {
  if (deselectedRepoIds.value.length === 0) return undefined;
  return projectRepos.value
    .filter((r) => !deselectedRepoIds.value.includes(r.id))
    .map((r) => r.id);
});


function repoDisplayName(repoURL: string, name?: string) {
  return (
    name ||
    repoURL.replace(/^https?:\/\/github\.com\//, "").replace(/\.git$/, "")
  );
}

// Wiki
const wikiError = ref("");

// ── Result state ─────────────────────────────────────────────────────
const loading = ref(false);
const error = ref("");
const result = ref<ClarifyResponse | null>(null);
const toolEvents = ref<ToolEvent[]>([]);

// ── Chat state ───────────────────────────────────────────────────────
const sessionId = ref<string | null>(null);
type ChatMessage = { role: "user" | "assistant"; content: string };
const chatMessages = ref<ChatMessage[]>([]);
const chatInput = ref("");
const chatLoading = ref(false);
const chatError = ref("");
const chatScrollEl = ref<HTMLElement | null>(null);

function scrollChatToBottom() {
  nextTick(() => {
    if (chatScrollEl.value) chatScrollEl.value.scrollTop = chatScrollEl.value.scrollHeight;
  });
}

async function sendChat() {
  const msg = chatInput.value.trim();
  if (!msg || chatLoading.value || !sessionId.value) return;
  chatInput.value = "";
  chatMessages.value.push({ role: "user", content: msg });
  scrollChatToBottom();
  chatLoading.value = true;
  chatError.value = "";
  try {
    const reply = await api.clarifyChat(
      { sessionId: sessionId.value, message: msg, projectId: projectId.value, repoIds: effectiveRepoIds.value, model: selectedModel.value || undefined },
      () => {},
    );
    chatMessages.value.push({ role: "assistant", content: reply });
    scrollChatToBottom();
  } catch (e) {
    chatError.value = e instanceof Error ? e.message : "Lỗi chat";
  } finally {
    chatLoading.value = false;
  }
}

// ── Computed ─────────────────────────────────────────────────────────
const needsWiki = computed(
  () => clarifyMode.value === "wiki" || clarifyMode.value === "both",
);

const canRun = computed(() => {
  if (loading.value || !specText.value.trim()) return false;
  return true;
});

const totalIssues = computed(() => result.value?.issues.length ?? 0);

// ── Actions ──────────────────────────────────────────────────────────
async function runClarify() {
  if (!canRun.value) return;
  loading.value = true;
  error.value = "";
  wikiError.value = "";
  result.value = null;
  toolEvents.value = [];
  sessionId.value = null;
  chatMessages.value = [];
  chatError.value = "";
  try {
    let wikiContent: string | undefined;
    if (needsWiki.value) {
      const wikiRes = await api.wiki.chat({
        projectId: projectId.value ?? "",
        sectionKey: "spec-clarify",
        question: specText.value.trim(),
        history: [],
      });
      wikiContent = wikiRes.answer;
      if (wikiRes.chunks?.length) {
        wikiContent += "\n\n--- Nguồn tham khảo ---\n";
        for (const chunk of wikiRes.chunks) {
          wikiContent += `\n[${chunk.sourceFile || "unknown"}]:\n${chunk.content}\n`;
        }
      }
    }
    const payload = {
      spec: specText.value,
      mode: clarifyMode.value,
      wikiContent,
      projectId: projectId.value,
      repoIds: effectiveRepoIds.value,
      model: selectedModel.value || undefined,
    };
    if (clarifyMode.value === "source" || clarifyMode.value === "both") {
      result.value = await api.clarifyStream(payload, (ev) => {
        toolEvents.value.push(ev);
      });
    } else {
      result.value = await api.clarify(payload);
    }
    if (result.value?.sessionId) {
      sessionId.value = result.value.sessionId;
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Review thất bại";
  } finally {
    loading.value = false;
  }
}

function clearResults() {
  result.value = null;
  error.value = "";
  toolEvents.value = [];
  sessionId.value = null;
  chatMessages.value = [];
  chatError.value = "";
}

// ── UI helpers ───────────────────────────────────────────────────────

function toolEventIcon(kind: string) {
  switch (kind) {
    case "read":
      return "📖";
    case "write":
      return "✏️";
    case "shell":
      return "⚙️";
    case "url":
      return "🌐";
    case "mcp":
      return "🔌";
    default:
      return "🔧";
  }
}

function toolEventLabel(ev: ToolEvent) {
  if (ev.path) return ev.path;
  if (ev.name) return ev.name;
  return ev.kind;
}

function severityClass(severity: string, category?: string) {
  if (category === "code_wiki_conflict") {
    return "bg-purple-50 border-purple-200 text-purple-700 dark:bg-purple-950/30 dark:border-purple-800 dark:text-purple-400";
  }
  switch (severity) {
    case "high":
      return "bg-red-50 border-red-200 text-red-700 dark:bg-red-950/30 dark:border-red-800 dark:text-red-400";
    case "medium":
      return "bg-amber-50 border-amber-200 text-amber-700 dark:bg-amber-950/30 dark:border-amber-800 dark:text-amber-400";
    default:
      return "bg-blue-50 border-blue-200 text-blue-700 dark:bg-blue-950/30 dark:border-blue-800 dark:text-blue-400";
  }
}

function severityBadgeVariant(severity: string) {
  switch (severity) {
    case "high":
      return "destructive";
    case "medium":
      return "secondary";
    default:
      return "outline";
  }
}

function categoryIcon(category: string) {
  switch (category) {
    case "missing_flow":
      return "🔀";
    case "missing_edge_case":
      return "⚠️";
    case "missing_constraint":
      return "📏";
    case "ambiguity":
      return "❓";
    case "inaccuracy":
      return "✗";
    case "code_wiki_conflict":
      return "⚡";
    // legacy fallbacks
    case "gap":
      return "⚡";
    case "conflict":
      return "⚠️";
    case "suggestion":
      return "💡";
    default:
      return "•";
  }
}

function categoryLabel(category: string) {
  switch (category) {
    case "missing_flow":
      return "Thiếu luồng";
    case "missing_edge_case":
      return "Thiếu edge case";
    case "missing_constraint":
      return "Thiếu ràng buộc";
    case "ambiguity":
      return "Mơ hồ";
    case "inaccuracy":
      return "Sai";
    case "code_wiki_conflict":
      return "Code ≠ Wiki";
    // legacy fallbacks
    case "gap":
      return "Thiếu";
    case "conflict":
      return "Mâu thuẫn";
    case "suggestion":
      return "Gợi ý";
    default:
      return category;
  }
}
</script>

<template>
  <div class="flex h-screen bg-background text-foreground overflow-hidden">
    <!-- Left: Spec Input -->
    <div class="w-[40%] flex flex-col border-r border-border">
      <!-- Header -->
      <div
        class="px-5 py-3 border-b border-border flex items-center justify-between shrink-0"
      >
        <div class="flex items-center gap-3">
          <button
            class="text-xs text-muted-foreground hover:text-foreground transition-colors"
            @click="router.push(`/projects/${projectId}`)"
          >
            ← Project
          </button>
          <h1 class="text-sm font-bold">🔍 Spec Clarify</h1>
        </div>
        <div class="flex items-center gap-2">
          <p
            v-if="projectStore.selectedProject"
            class="text-xs text-muted-foreground truncate max-w-48"
          >
            {{ projectStore.selectedProject.name }}
          </p>
        </div>
      </div>

      <!-- Spec textarea -->
      <div class="flex-1 flex flex-col overflow-hidden p-5 gap-4">
        <div class="flex-1 flex flex-col gap-2 min-h-0">
          <div class="flex items-center justify-between">
            <Label
              class="text-xs font-semibold text-muted-foreground uppercase tracking-wide"
            >
              Spec / Requirement Document
            </Label>
            <div class="flex rounded border border-border overflow-hidden">
              <button
                class="px-2 py-0.5 text-[11px] transition-colors"
                :class="!inputPreview ? 'bg-primary text-primary-foreground' : 'bg-muted/30 text-muted-foreground hover:bg-muted/60'"
                @click="inputPreview = false"
              >
                Edit
              </button>
              <button
                class="px-2 py-0.5 text-[11px] transition-colors"
                :class="inputPreview ? 'bg-primary text-primary-foreground' : 'bg-muted/30 text-muted-foreground hover:bg-muted/60'"
                @click="inputPreview = true"
              >
                Preview
              </button>
            </div>
          </div>
          <Textarea
            v-if="!inputPreview"
            v-model="specText"
            placeholder="Paste nội dung spec hoặc requirement vào đây...&#10;&#10;Ví dụ:&#10;- Hệ thống cho phép user đăng ký bằng email&#10;- Hỗ trợ OAuth (Google, GitHub)&#10;- Reset mật khẩu qua email&#10;- Quản lý session bằng JWT..."
            class="flex-1 resize-none text-sm leading-relaxed font-mono"
          />
          <div
            v-else
            class="flex-1 overflow-y-auto rounded-md border border-input bg-background px-3 py-2 text-sm prose prose-sm max-w-none"
            v-html="renderMd(specText || '*Chưa có nội dung*')"
          />
        </div>

        <!-- Mode selector + Wiki input -->
        <div class="space-y-3 shrink-0">
          <Label
            class="text-xs font-semibold text-muted-foreground uppercase tracking-wide"
          >
            Kiểm tra spec với
          </Label>
          <div class="grid grid-cols-3 gap-2">
            <button
              v-for="m in [
                {
                  value: 'source',
                  label: 'Source Code',
                  icon: '💻',
                  desc: 'Kiểm tra với codebase',
                },
                {
                  value: 'wiki',
                  label: 'Wiki / Docs',
                  icon: '📖',
                  desc: 'Kiểm tra với tài liệu wiki',
                },
                {
                  value: 'both',
                  label: 'Source + Wiki',
                  icon: '🔀',
                  desc: 'Kết hợp cả hai',
                },
              ]"
              :key="m.value"
              class="rounded-lg border px-3 py-2.5 text-left transition-colors"
              :class="
                clarifyMode === m.value
                  ? 'bg-primary text-primary-foreground border-primary'
                  : 'bg-muted/30 border-border text-muted-foreground hover:bg-muted/60'
              "
              :disabled="loading"
              @click="clarifyMode = m.value as ClarifyMode"
            >
              <div class="flex items-center gap-2">
                <span class="text-base">{{ m.icon }}</span>
                <span class="text-sm font-medium">{{ m.label }}</span>
              </div>
              <p class="text-[11px] mt-0.5 opacity-80">{{ m.desc }}</p>
            </button>
          </div>

          <!-- Repo selector (source mode, multi-repo) -->
          <div
            v-if="
              (clarifyMode === 'source' || clarifyMode === 'both') &&
              projectRepos.length > 1
            "
            class="space-y-2"
          >
            <p
              class="text-[11px] text-muted-foreground font-medium uppercase tracking-wide"
            >
              Repositories
              <span class="normal-case font-normal">
                ({{
                  deselectedRepoIds.length === 0
                    ? "all"
                    : projectRepos.length - deselectedRepoIds.length
                }}
                selected)
              </span>
            </p>
            <div class="flex flex-wrap gap-1.5">
              <button
                v-for="repo in projectRepos"
                :key="repo.id"
                class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] border transition-colors"
                :class="
                  !deselectedRepoIds.includes(repo.id)
                    ? 'bg-primary/10 border-primary/40 text-primary'
                    : 'bg-muted/30 border-border text-muted-foreground line-through opacity-60'
                "
                :disabled="loading"
                @click="toggleRepo(repo.id)"
              >
                <span
                  class="w-1.5 h-1.5 rounded-full"
                  :class="
                    !deselectedRepoIds.includes(repo.id)
                      ? 'bg-primary'
                      : 'bg-muted-foreground/40'
                  "
                />
                {{ repoDisplayName(repo.repoURL, repo.name) }}
                <template v-if="repo.repoBranch"
                  >· {{ repo.repoBranch }}</template
                >
              </button>
            </div>
            <p class="text-[10px] text-muted-foreground">
              Click vào repo để bỏ khỏi phân tích. Mặc định dùng tất cả.
            </p>
          </div>

          <!-- Wiki info -->
          <div v-if="needsWiki" class="space-y-2">
            <div
              class="flex items-center gap-2 px-3 py-2 rounded-lg bg-muted/40 border border-border"
            >
              <span class="text-base">📚</span>
              <div class="min-w-0">
                <p class="text-xs font-medium truncate">
                  {{ projectStore.selectedProject?.name || "Project" }}
                </p>
              </div>
            </div>
            <p class="text-[11px] text-muted-foreground">
              Spec sẽ được so sánh tự động với kiến thức Wiki của project hiện
              tại.
            </p>
            <p v-if="wikiError" class="text-[11px] text-destructive">
              {{ wikiError }}
            </p>
          </div>

          <!-- Run button + Model selector -->
          <div class="flex gap-2">
            <Button class="flex-1 h-10" :disabled="!canRun" @click="runClarify">
              <span v-if="loading" class="flex items-center gap-2">
                <span
                  class="w-4 h-4 border-2 border-primary-foreground border-t-transparent rounded-full animate-spin"
                />
                Đang review...
              </span>
              <span v-else>🔎 Review Spec</span>
            </Button>
            <Select v-model="selectedModel">
              <SelectTrigger class="w-36 shrink-0">
                <SelectValue placeholder="Model" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="m in availableModels" :key="m" :value="m">
                  {{ m }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </div>
    </div>

    <!-- Middle: Results -->
    <div class="flex-1 flex flex-col overflow-hidden border-r border-border min-w-0">
      <!-- Header -->
      <div
        class="px-5 py-3 border-b border-border flex items-center justify-between shrink-0"
      >
        <p class="text-sm font-semibold">Kết quả review</p>
        <div v-if="result" class="flex items-center gap-1.5">
          <Badge
            v-if="clarifyMode === 'source'"
            variant="secondary"
            class="text-[10px]"
            >Source Code</Badge
          >
          <Badge v-else variant="secondary" class="text-[10px]">Wiki</Badge>
          <span class="text-[10px] font-medium px-2 py-0.5 rounded-full border bg-muted border-border text-muted-foreground">
            {{ totalIssues }} vấn đề
          </span>
          <Button
            variant="ghost"
            size="sm"
            class="h-6 text-xs"
            @click="clearResults"
            >✕</Button
          >
        </div>
      </div>

      <!-- Content -->
      <ScrollArea class="flex-1">
        <!-- Loading -->
        <div
          v-if="loading"
          class="flex flex-col items-center justify-center h-full min-h-[400px] gap-3 text-center px-5"
        >
          <div
            class="w-8 h-8 border-2 border-primary border-t-transparent rounded-full animate-spin"
          />
          <p class="text-sm text-muted-foreground">AI đang phân tích spec...</p>
          <p class="text-xs text-muted-foreground">Có thể mất vài phút</p>
          <!-- Tool activity feed -->
          <div
            v-if="toolEvents.length"
            class="w-full max-w-xs mt-2 space-y-0.5 text-left"
          >
            <p
              class="text-[10px] font-semibold text-muted-foreground uppercase tracking-wide mb-1"
            >
              Đang đọc codebase
            </p>
            <div class="max-h-48 overflow-y-auto space-y-0.5">
              <div
                v-for="(ev, idx) in toolEvents"
                :key="idx"
                class="flex items-center gap-1.5 text-[11px] text-muted-foreground"
              >
                <span class="shrink-0">{{ toolEventIcon(ev.kind) }}</span>
                <span class="truncate font-mono">{{ toolEventLabel(ev) }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Error -->
        <div
          v-else-if="error"
          class="m-5 px-4 py-3 bg-destructive/10 border border-destructive/20 rounded-lg text-sm text-destructive"
        >
          {{ error }}
        </div>

        <!-- Results -->
        <div v-else-if="result" class="p-5 space-y-5">
          <!-- Summary -->
          <div
            class="bg-muted/40 rounded-lg px-4 py-3 text-sm leading-relaxed prose prose-sm max-w-none"
            v-html="renderMd(result.summary)"
          />

          <!-- Issues -->
          <div v-if="result.issues.length" class="space-y-2">
            <p class="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
              Các vấn đề ({{ totalIssues }})
            </p>
            <div
              v-for="issue in result.issues"
              :key="issue.id"
              class="rounded-lg border p-3 space-y-2"
              :class="severityClass(issue.severity, issue.category)"
            >
              <div class="flex items-start justify-between gap-2">
                <div class="flex items-center gap-2 flex-1 min-w-0">
                  <span class="text-base shrink-0">{{ categoryIcon(issue.category) }}</span>
                  <p class="text-sm font-semibold leading-snug">{{ issue.title }}</p>
                </div>
                <div class="flex gap-1 shrink-0 items-center">
                  <Badge :variant="severityBadgeVariant(issue.severity) as any" class="text-[10px]">
                    {{ issue.severity }}
                  </Badge>
                  <span class="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-background/60 border border-current/20">
                    {{ categoryLabel(issue.category) }}
                  </span>
                </div>
              </div>

              <div class="text-xs leading-relaxed opacity-90 prose prose-xs max-w-none" v-html="renderMd(issue.description)" />
              <div class="border-t border-current/20 pt-2">
                <p class="text-[10px] font-medium opacity-70 mb-0.5">💡 Gợi ý:</p>
                <div class="text-xs leading-relaxed opacity-80 prose prose-xs max-w-none" v-html="renderMd(issue.suggestion)" />
              </div>

              <!-- Referenced files & wiki sections -->
              <div
                v-if="issue.referenced_files?.length || issue.wiki_sections?.length"
                class="border-t border-current/10 pt-2 space-y-1"
              >
                <div v-if="issue.referenced_files?.length" class="flex flex-col gap-0.5">
                  <span class="text-[10px] opacity-50 font-medium">📁 Files tham khảo</span>
                  <template v-for="f in issue.referenced_files" :key="f.path">
                    <a v-if="f.url" :href="f.url" target="_blank" rel="noopener noreferrer"
                      class="text-[10px] font-mono text-blue-600 hover:underline truncate pl-3" :title="f.path">{{ f.path }}</a>
                    <span v-else class="text-[10px] font-mono opacity-60 truncate pl-3" :title="f.path">{{ f.path }}</span>
                  </template>
                </div>
                <div v-if="issue.wiki_sections?.length" class="flex flex-col gap-0.5">
                  <span class="text-[10px] opacity-50 font-medium">📖 Wiki sections</span>
                  <span v-for="s in issue.wiki_sections" :key="s" class="text-[10px] opacity-60 pl-3">{{ s }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- No issues found -->
          <div v-if="!result.issues.length" class="text-center py-8">
            <span class="text-4xl block mb-3">✅</span>
            <p class="text-sm font-medium">Spec rõ ràng, không phát hiện vấn đề nào!</p>
          </div>
        </div>

        <!-- Empty state -->
        <div
          v-else
          class="flex flex-col items-center justify-center h-full min-h-[400px] gap-3 text-center px-8"
        >
          <span class="text-5xl opacity-15">🔍</span>
          <p class="text-sm text-muted-foreground">
            Paste spec vào bên trái, chọn nguồn so sánh,<br />
            rồi nhấn "Review Spec" để bắt đầu.
          </p>
          <p class="text-xs text-muted-foreground/60">
            AI sẽ phân tích spec và chỉ ra các điểm chưa đúng,<br />
            kèm Q&A cho các điểm cần xác nhận.
          </p>
        </div>
      </ScrollArea>
    </div>

    <!-- Right: Chat panel — appears after review when sessionId is available -->
    <div
      v-if="sessionId"
      class="w-80 shrink-0 flex flex-col border-l border-border"
    >
      <!-- Chat header -->
      <div class="px-4 py-3 flex items-center gap-2 shrink-0 border-b border-border bg-muted/20">
        <span class="text-sm font-semibold">💬 Chat với AI</span>
        <span class="text-[10px] text-muted-foreground ml-auto">Cùng session</span>
      </div>

      <!-- Messages -->
      <div ref="chatScrollEl" class="flex-1 overflow-y-auto px-3 py-3 space-y-2">
        <div v-if="chatMessages.length === 0" class="text-center py-6 px-2">
          <p class="text-xs text-muted-foreground">Hỏi bất kỳ điều gì về spec hoặc các vấn đề tìm được...</p>
        </div>
        <div
          v-for="(msg, i) in chatMessages"
          :key="i"
          class="flex gap-2"
          :class="msg.role === 'user' ? 'justify-end' : 'justify-start'"
        >
          <div
            class="max-w-[85%] rounded-lg px-3 py-1.5 text-xs leading-relaxed"
            :class="
              msg.role === 'user'
                ? 'bg-primary text-primary-foreground'
                : 'bg-muted text-foreground prose prose-xs max-w-none'
            "
            v-html="msg.role === 'assistant' ? renderMd(msg.content) : msg.content"
          />
        </div>
        <div v-if="chatLoading" class="flex justify-start">
          <div class="bg-muted rounded-lg px-3 py-1.5 flex items-center gap-1.5">
            <span class="w-1.5 h-1.5 rounded-full bg-muted-foreground animate-bounce" style="animation-delay:0ms" />
            <span class="w-1.5 h-1.5 rounded-full bg-muted-foreground animate-bounce" style="animation-delay:150ms" />
            <span class="w-1.5 h-1.5 rounded-full bg-muted-foreground animate-bounce" style="animation-delay:300ms" />
          </div>
        </div>
        <p v-if="chatError" class="text-xs text-destructive">{{ chatError }}</p>
      </div>

      <!-- Input -->
      <div class="shrink-0 px-3 py-3 border-t border-border flex gap-2">
        <input
          v-model="chatInput"
          type="text"
          placeholder="Nhập câu hỏi..."
          class="flex-1 h-8 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          :disabled="chatLoading"
          @keydown.enter.prevent="sendChat"
        />
        <Button size="sm" class="h-8 px-3" :disabled="!chatInput.trim() || chatLoading" @click="sendChat">
          Gửi
        </Button>
      </div>
    </div>
  </div>
</template>
