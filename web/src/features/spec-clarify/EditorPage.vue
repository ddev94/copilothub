<script setup lang="ts">
import { ref, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useProjectStore } from "@/stores/repo";
import { api } from "@/api";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import SpecDiffView from "@/components/SpecDiffView.vue";
import type { ClarifyResponse, ToolEvent } from "@/types";
import { marked } from "marked";

function renderMd(text: string): string {
  return marked.parse(text, { async: false }) as string;
}

const route = useRoute();
const router = useRouter();
const projectStore = useProjectStore();
projectStore.fetch();

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

// ── Fix / progress state ─────────────────────────────────────────────
const resolvedIssueIds = ref<string[]>([]);
const pendingFix = ref<{
  issueId: string; // '__all__' when fixing all remaining
  revisedSpec: string;
  showDiff: boolean;
} | null>(null);
const fixingIssueId = ref<string | null>(null);
const refining = ref(false);
const refineError = ref("");

// ── Computed ─────────────────────────────────────────────────────────
const needsWiki = computed(
  () => clarifyMode.value === "wiki" || clarifyMode.value === "both",
);

const canRun = computed(() => {
  if (loading.value || !specText.value.trim()) return false;
  return true;
});

const totalIssues = computed(() => result.value?.issues.length ?? 0);
const resolvedCount = computed(() => resolvedIssueIds.value.length);
const specReady = computed(
  () => totalIssues.value > 0 && resolvedCount.value >= totalIssues.value,
);
const unresolvedIssues = computed(
  () =>
    result.value?.issues.filter(
      (i) => !resolvedIssueIds.value.includes(i.id),
    ) ?? [],
);
const pendingFixIssue = computed(() =>
  pendingFix.value?.issueId === "__all__"
    ? null
    : (result.value?.issues.find((i) => i.id === pendingFix.value?.issueId) ??
      null),
);

function isResolved(issueId: string) {
  return resolvedIssueIds.value.includes(issueId);
}

function markResolved(issueId: string) {
  if (!isResolved(issueId)) resolvedIssueIds.value.push(issueId);
}

// ── Actions ──────────────────────────────────────────────────────────
async function runClarify() {
  if (!canRun.value) return;
  loading.value = true;
  error.value = "";
  wikiError.value = "";
  result.value = null;
  toolEvents.value = [];
  resolvedIssueIds.value = [];
  pendingFix.value = null;
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
    };
    // Use SSE streaming when mode involves source code (Copilot SDK tool events)
    if (clarifyMode.value === "source" || clarifyMode.value === "both") {
      result.value = await api.clarifyStream(payload, (ev) => {
        toolEvents.value.push(ev);
      });
    } else {
      result.value = await api.clarify(payload);
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
  resolvedIssueIds.value = [];
  pendingFix.value = null;
  refineError.value = "";
}

// Fix all remaining unresolved issues at once
async function runRefine() {
  if (!result.value || refining.value) return;
  refining.value = true;
  refineError.value = "";
  pendingFix.value = null;
  try {
    const res = await api.refineSpec({
      spec: specText.value,
      issues: unresolvedIssues.value,
    });
    pendingFix.value = {
      issueId: "__all__",
      revisedSpec: res.refinedSpec,
      showDiff: true,
    };
  } catch (e) {
    refineError.value = e instanceof Error ? e.message : "Fix thất bại";
  } finally {
    refining.value = false;
  }
}

// Fix a single issue
async function runFixIssue(issueId: string) {
  if (!result.value || fixingIssueId.value || refining.value) return;
  const issue = result.value.issues.find((i) => i.id === issueId);
  if (!issue) return;

  fixingIssueId.value = issueId;
  refineError.value = "";
  pendingFix.value = null;

  try {
    const res = await api.refineSpec({
      spec: specText.value,
      issues: [issue],
    });
    pendingFix.value = {
      issueId,
      revisedSpec: res.refinedSpec,
      showDiff: true,
    };
  } catch (e) {
    refineError.value = e instanceof Error ? e.message : "Fix thất bại";
  } finally {
    fixingIssueId.value = null;
  }
}

// Accept the pending fix — update spec in-place and mark issue(s) resolved
function acceptFix() {
  if (!pendingFix.value) return;
  specText.value = pendingFix.value.revisedSpec;
  if (pendingFix.value.issueId === "__all__") {
    for (const issue of unresolvedIssues.value) markResolved(issue.id);
  } else {
    markResolved(pendingFix.value.issueId);
  }
  pendingFix.value = null;
}

function rejectFix() {
  pendingFix.value = null;
}

async function copyPendingSpec() {
  if (pendingFix.value) {
    await navigator.clipboard.writeText(pendingFix.value.revisedSpec);
  }
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

function severityClass(severity: string) {
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
    <div class="w-1/2 flex flex-col border-r border-border">
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

          <!-- Run button -->
          <Button class="w-full h-10" :disabled="!canRun" @click="runClarify">
            <span v-if="loading" class="flex items-center gap-2">
              <span
                class="w-4 h-4 border-2 border-primary-foreground border-t-transparent rounded-full animate-spin"
              />
              Đang review...
            </span>
            <span v-else>🔎 Review Spec</span>
          </Button>
        </div>
      </div>
    </div>

    <!-- Right: Results -->
    <div class="w-1/2 flex flex-col overflow-hidden">
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
          <!-- Progress badge -->
          <span
            class="text-[10px] font-medium px-2 py-0.5 rounded-full border"
            :class="
              specReady
                ? 'bg-green-500/10 border-green-500/30 text-green-600'
                : 'bg-muted border-border text-muted-foreground'
            "
          >
            {{
              specReady
                ? "✓ Spec ready"
                : `${resolvedCount}/${totalIssues} resolved`
            }}
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
          <!-- ── Pending fix: diff review ── -->
          <div
            v-if="pendingFix"
            class="space-y-2 rounded-lg border border-primary/30 bg-primary/5 p-3"
          >
            <div class="flex items-center justify-between">
              <p class="text-xs font-semibold text-primary">
                {{
                  pendingFix.issueId === "__all__"
                    ? `✦ Fix cho ${unresolvedIssues.length} vấn đề còn lại`
                    : `✦ Fix: ${pendingFixIssue?.title ?? ""}`
                }}
              </p>
              <div class="flex items-center gap-1.5">
                <div class="flex rounded border border-border overflow-hidden">
                  <button
                    class="px-2 py-0.5 text-[11px] transition-colors"
                    :class="
                      pendingFix.showDiff
                        ? 'bg-primary text-primary-foreground'
                        : 'bg-muted/30 text-muted-foreground'
                    "
                    @click="pendingFix.showDiff = true"
                  >
                    Diff
                  </button>
                  <button
                    class="px-2 py-0.5 text-[11px] transition-colors"
                    :class="
                      !pendingFix.showDiff
                        ? 'bg-primary text-primary-foreground'
                        : 'bg-muted/30 text-muted-foreground'
                    "
                    @click="pendingFix.showDiff = false"
                  >
                    Text
                  </button>
                </div>
                <button
                  class="text-[11px] text-muted-foreground hover:text-foreground px-1.5"
                  @click="copyPendingSpec"
                >
                  Copy
                </button>
              </div>
            </div>
            <SpecDiffView
              v-if="pendingFix.showDiff"
              :original="specText"
              :revised="pendingFix.revisedSpec"
            />
            <div
              v-else
              class="overflow-y-auto max-h-64 rounded-md border border-border bg-background px-3 py-2 text-xs leading-relaxed prose prose-xs max-w-none"
              v-html="renderMd(pendingFix.revisedSpec)"
            />
            <div class="flex gap-2 pt-1">
              <Button size="sm" class="flex-1 h-8" @click="acceptFix"
                >✓ Accept — cập nhật spec</Button
              >
              <Button
                size="sm"
                variant="outline"
                class="h-8 px-3"
                @click="rejectFix"
                >✕ Bỏ qua</Button
              >
            </div>
          </div>

          <!-- Summary -->
          <div
            class="bg-muted/40 rounded-lg px-4 py-3 text-sm leading-relaxed prose prose-sm max-w-none"
            v-html="renderMd(result.summary)"
          />

          <!-- Issues -->
          <div v-if="result.issues.length" class="space-y-2">
            <p
              class="text-xs font-semibold text-muted-foreground uppercase tracking-wide"
            >
              Các vấn đề ({{ resolvedCount }}/{{ totalIssues }} đã xử lý)
            </p>
            <div
              v-for="issue in result.issues"
              :key="issue.id"
              class="rounded-lg border p-3 space-y-2 transition-opacity"
              :class="[
                isResolved(issue.id)
                  ? 'opacity-40 bg-muted/30 border-border'
                  : severityClass(issue.severity),
              ]"
            >
              <div class="flex items-start justify-between gap-2">
                <div class="flex items-center gap-2 flex-1 min-w-0">
                  <!-- Resolved checkmark or category icon -->
                  <span class="text-base shrink-0">
                    {{
                      isResolved(issue.id) ? "✓" : categoryIcon(issue.category)
                    }}
                  </span>
                  <p
                    class="text-sm font-semibold leading-snug"
                    :class="isResolved(issue.id) ? 'line-through' : ''"
                  >
                    {{ issue.title }}
                  </p>
                </div>
                <div class="flex gap-1 shrink-0 items-center">
                  <span
                    v-if="isResolved(issue.id)"
                    class="text-[10px] text-green-600 font-medium"
                    >Fixed</span
                  >
                  <template v-else>
                    <Badge
                      :variant="severityBadgeVariant(issue.severity) as any"
                      class="text-[10px]"
                    >
                      {{ issue.severity }}
                    </Badge>
                    <span
                      class="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-background/60 border border-current/20"
                    >
                      {{ categoryLabel(issue.category) }}
                    </span>
                  </template>
                </div>
              </div>

              <template v-if="!isResolved(issue.id)">
                <div
                  class="text-xs leading-relaxed opacity-90 prose prose-xs max-w-none"
                  v-html="renderMd(issue.description)"
                />
                <div
                  class="border-t border-current/20 pt-2 flex items-start justify-between gap-3"
                >
                  <div class="flex-1 min-w-0">
                    <p class="text-[10px] font-medium opacity-70 mb-0.5">
                      💡 Gợi ý:
                    </p>
                    <div
                      class="text-xs leading-relaxed opacity-80 prose prose-xs max-w-none"
                      v-html="renderMd(issue.suggestion)"
                    />
                  </div>
                  <button
                    class="shrink-0 flex items-center gap-1 text-[11px] font-medium px-2.5 py-1 rounded border transition-colors bg-background/70 border-current/30 hover:bg-background opacity-80 hover:opacity-100 disabled:opacity-40 disabled:cursor-not-allowed"
                    :disabled="!!fixingIssueId || refining || !!pendingFix"
                    @click="runFixIssue(issue.id)"
                  >
                    <span
                      v-if="fixingIssueId === issue.id"
                      class="w-3 h-3 border border-current border-t-transparent rounded-full animate-spin"
                    />
                    <span v-else>✦</span>
                    {{ fixingIssueId === issue.id ? "Đang fix..." : "Fix" }}
                  </button>
                </div>
              </template>
            </div>
          </div>

          <!-- No issues found -->
          <div v-if="!result.issues.length" class="text-center py-8">
            <span class="text-4xl block mb-3">✅</span>
            <p class="text-sm font-medium">
              Spec rõ ràng, không phát hiện vấn đề nào!
            </p>
          </div>

          <!-- Footer actions -->
          <div class="pt-2 border-t border-border space-y-3">
            <!-- Spec ready state -->
            <div v-if="specReady" class="text-center py-4 space-y-2">
              <span class="text-3xl block">✅</span>
              <p class="text-sm font-semibold text-green-600">
                Spec ready — 0 issues remaining
              </p>
              <p class="text-xs text-muted-foreground">
                Tất cả vấn đề đã được xử lý. Spec sẵn sàng giao dev.
              </p>
            </div>

            <!-- Fix remaining button -->
            <template v-else-if="unresolvedIssues.length > 0 && !pendingFix">
              <Button
                class="w-full h-10"
                variant="outline"
                :disabled="refining || !!fixingIssueId"
                @click="runRefine"
              >
                <span v-if="refining" class="flex items-center gap-2">
                  <span
                    class="w-4 h-4 border-2 border-current border-t-transparent rounded-full animate-spin"
                  />
                  Đang fix...
                </span>
                <span v-else
                  >✦ Fix {{ unresolvedIssues.length }} vấn đề còn lại</span
                >
              </Button>
              <p class="text-[10px] text-center text-muted-foreground">
                Hoặc fix từng vấn đề riêng lẻ ở trên
              </p>
            </template>

            <p v-if="refineError" class="text-xs text-destructive">
              {{ refineError }}
            </p>
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
  </div>
</template>
