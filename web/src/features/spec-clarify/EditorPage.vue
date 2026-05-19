<script setup lang="ts">
import { ref, computed } from "vue";
import { useRouter } from "vue-router";
import { useRepoStore } from "@/stores/repo";
import { api } from "@/api";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { ScrollArea } from "@/components/ui/scroll-area";
import ConfigDialog from "./ConfigDialog.vue";
import type { ClarifyResponse } from "@/types";

const router = useRouter();
const repoStore = useRepoStore();
repoStore.fetch();

// ── Input state ──────────────────────────────────────────────────────
const specText = ref("");
type ClarifyMode = "source" | "wiki";
const clarifyMode = ref<ClarifyMode>("source");

// Wiki
type WikiTab = "url" | "paste";
const wikiTab = ref<WikiTab>("url");
const wikiUrl = ref("");
const wikiContent = ref("");
const fetchingWiki = ref(false);
const wikiError = ref("");

// ── Result state ─────────────────────────────────────────────────────
const loading = ref(false);
const error = ref("");
const result = ref<ClarifyResponse | null>(null);

// Q&A answers
const answers = ref<Record<string, string>>({});

// ── Computed ─────────────────────────────────────────────────────────
const needsWiki = computed(() => clarifyMode.value === "wiki");

const canRun = computed(() => {
  if (loading.value || !specText.value.trim()) return false;
  if (needsWiki.value) return wikiContent.value.trim().length > 0;
  return true;
});

// ── Actions ──────────────────────────────────────────────────────────
async function fetchWikiFromUrl() {
  if (!wikiUrl.value.trim()) return;
  fetchingWiki.value = true;
  wikiError.value = "";
  wikiContent.value = "";
  try {
    const res = await api.fetchWiki(wikiUrl.value.trim());
    wikiContent.value = res.content;
  } catch (e) {
    wikiError.value = e instanceof Error ? e.message : "Không thể tải URL này";
  } finally {
    fetchingWiki.value = false;
  }
}

async function runClarify() {
  if (!canRun.value) return;
  loading.value = true;
  error.value = "";
  result.value = null;
  answers.value = {};
  try {
    const res = await api.clarify({
      spec: specText.value,
      mode: clarifyMode.value,
      wikiContent: needsWiki.value ? wikiContent.value : undefined,
    });
    result.value = res;
    // Pre-fill Q&A answers with defaults
    for (const q of res.questions) {
      answers.value[q.id] = q.defaultAnswer;
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Phân tích thất bại";
  } finally {
    loading.value = false;
  }
}

function clearResults() {
  result.value = null;
  error.value = "";
  answers.value = {};
}

// ── UI helpers ───────────────────────────────────────────────────────
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
    case "gap":
      return "⚡";
    case "conflict":
      return "⚠️";
    case "ambiguity":
      return "❓";
    case "suggestion":
      return "💡";
    default:
      return "•";
  }
}

function categoryLabel(category: string) {
  switch (category) {
    case "gap":
      return "Thiếu";
    case "conflict":
      return "Mâu thuẫn";
    case "ambiguity":
      return "Không rõ";
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
            @click="router.push('/')"
          >
            ← Hub
          </button>
          <h1 class="text-sm font-bold">🔍 Spec Clarify</h1>
        </div>
        <div class="flex items-center gap-2">
          <p
            v-if="repoStore.info"
            class="text-xs text-muted-foreground truncate max-w-48"
          >
            {{ repoStore.info.name }}
          </p>
          <ConfigDialog />
        </div>
      </div>

      <!-- Spec textarea -->
      <div class="flex-1 flex flex-col overflow-hidden p-5 gap-4">
        <div class="flex-1 flex flex-col gap-2 min-h-0">
          <Label
            class="text-xs font-semibold text-muted-foreground uppercase tracking-wide"
          >
            Spec / Requirement Document
          </Label>
          <Textarea
            v-model="specText"
            placeholder="Paste nội dung spec hoặc requirement vào đây...&#10;&#10;Ví dụ:&#10;- Hệ thống cho phép user đăng ký bằng email&#10;- Hỗ trợ OAuth (Google, GitHub)&#10;- Reset mật khẩu qua email&#10;- Quản lý session bằng JWT..."
            class="flex-1 resize-none text-sm leading-relaxed"
          />
        </div>

        <!-- Mode selector + Wiki input -->
        <div class="space-y-3 shrink-0">
          <Label
            class="text-xs font-semibold text-muted-foreground uppercase tracking-wide"
          >
            So sánh với
          </Label>
          <div class="grid grid-cols-2 gap-2">
            <button
              v-for="m in [
                {
                  value: 'source',
                  label: 'Source Code',
                  icon: '💻',
                  desc: 'So sánh spec với codebase hiện tại',
                },
                {
                  value: 'wiki',
                  label: 'Wiki / Docs',
                  icon: '📖',
                  desc: 'So sánh spec với tài liệu wiki',
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

          <!-- Wiki input -->
          <div v-if="needsWiki" class="space-y-2">
            <div class="flex rounded-md border border-border overflow-hidden">
              <button
                class="flex-1 py-1.5 text-xs font-medium transition-colors"
                :class="
                  wikiTab === 'url'
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-muted/30 text-muted-foreground hover:bg-muted/50'
                "
                @click="wikiTab = 'url'"
              >
                Fetch URL
              </button>
              <button
                class="flex-1 py-1.5 text-xs font-medium transition-colors border-l border-border"
                :class="
                  wikiTab === 'paste'
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-muted/30 text-muted-foreground hover:bg-muted/50'
                "
                @click="wikiTab = 'paste'"
              >
                Paste
              </button>
            </div>

            <div v-if="wikiTab === 'url'" class="flex gap-1.5">
              <Input
                v-model="wikiUrl"
                placeholder="https://wiki.example.com/..."
                class="flex-1 h-8 text-xs"
                :disabled="fetchingWiki"
                @keydown.enter="fetchWikiFromUrl"
              />
              <Button
                variant="outline"
                class="h-8 px-3 text-xs shrink-0"
                :disabled="!wikiUrl.trim() || fetchingWiki"
                @click="fetchWikiFromUrl"
              >
                {{ fetchingWiki ? "..." : "Fetch" }}
              </Button>
            </div>

            <Textarea
              v-if="wikiTab === 'paste'"
              v-model="wikiContent"
              placeholder="Paste nội dung wiki vào đây..."
              class="text-xs resize-none leading-relaxed"
              rows="4"
            />

            <p v-if="wikiError" class="text-[11px] text-destructive">
              {{ wikiError }}
            </p>
            <p
              v-else-if="wikiContent"
              class="text-[11px] text-green-600 dark:text-green-400"
            >
              ✓ Wiki sẵn sàng ({{ (wikiContent.length / 1000).toFixed(1) }}k ký
              tự)
            </p>
          </div>

          <!-- Run button -->
          <Button class="w-full h-10" :disabled="!canRun" @click="runClarify">
            <span v-if="loading" class="flex items-center gap-2">
              <span
                class="w-4 h-4 border-2 border-primary-foreground border-t-transparent rounded-full animate-spin"
              />
              Đang phân tích...
            </span>
            <span v-else>🔎 Phân tích Spec</span>
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
        <p class="text-sm font-semibold">Kết quả phân tích</p>
        <div v-if="result" class="flex items-center gap-1.5">
          <Badge
            v-if="clarifyMode === 'source'"
            variant="secondary"
            class="text-[10px]"
            >Source Code</Badge
          >
          <Badge v-else variant="secondary" class="text-[10px]">Wiki</Badge>
          <Badge variant="outline" class="text-[10px]">
            {{ result.issues.length }} vấn đề
          </Badge>
          <Badge
            v-if="result.questions.length"
            variant="outline"
            class="text-[10px]"
          >
            {{ result.questions.length }} câu hỏi
          </Badge>
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
          class="flex flex-col items-center justify-center h-full min-h-[400px] gap-3 text-center"
        >
          <div
            class="w-8 h-8 border-2 border-primary border-t-transparent rounded-full animate-spin"
          />
          <p class="text-sm text-muted-foreground">AI đang phân tích spec...</p>
          <p class="text-xs text-muted-foreground">Có thể mất 30–60 giây</p>
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
          <div class="bg-muted/40 rounded-lg px-4 py-3">
            <p class="text-sm leading-relaxed">{{ result.summary }}</p>
          </div>

          <!-- Issues -->
          <div v-if="result.issues.length" class="space-y-3">
            <p
              class="text-xs font-semibold text-muted-foreground uppercase tracking-wide"
            >
              Các vấn đề phát hiện
            </p>
            <div
              v-for="issue in result.issues"
              :key="issue.id"
              class="rounded-lg border p-3 space-y-2"
              :class="severityClass(issue.severity)"
            >
              <div class="flex items-start justify-between gap-2">
                <div class="flex items-center gap-2 flex-1 min-w-0">
                  <span class="text-base shrink-0">{{
                    categoryIcon(issue.category)
                  }}</span>
                  <p class="text-sm font-semibold leading-snug">
                    {{ issue.title }}
                  </p>
                </div>
                <div class="flex gap-1 shrink-0">
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
                </div>
              </div>
              <p class="text-xs leading-relaxed opacity-90">
                {{ issue.description }}
              </p>
              <div class="border-t border-current/20 pt-2">
                <p class="text-[10px] font-medium opacity-70 mb-0.5">
                  💡 Gợi ý:
                </p>
                <p class="text-xs leading-relaxed opacity-80 italic">
                  {{ issue.suggestion }}
                </p>
              </div>
            </div>
          </div>

          <!-- Q&A Section -->
          <div v-if="result.questions.length" class="space-y-3">
            <div class="flex items-center gap-2">
              <p
                class="text-xs font-semibold text-muted-foreground uppercase tracking-wide"
              >
                Câu hỏi cần xác nhận
              </p>
              <Badge variant="secondary" class="text-[10px]">
                {{ result.questions.length }} câu hỏi
              </Badge>
            </div>
            <p class="text-xs text-muted-foreground">
              Spec có một số điểm chưa rõ. Vui lòng xem xét và trả lời để hoàn
              thiện spec.
            </p>

            <Card
              v-for="(q, idx) in result.questions"
              :key="q.id"
              class="overflow-hidden"
            >
              <CardContent class="p-4 space-y-2">
                <Label class="text-sm font-medium">
                  {{ idx + 1 }}. {{ q.question }}
                </Label>
                <p
                  v-if="q.context"
                  class="text-xs text-muted-foreground leading-relaxed bg-muted/40 rounded px-2.5 py-1.5"
                >
                  {{ q.context }}
                </p>
                <!-- Options -->
                <div v-if="q.options?.length" class="flex flex-wrap gap-1.5">
                  <button
                    v-for="opt in q.options"
                    :key="opt"
                    class="px-2.5 py-1 text-xs rounded-md border transition-colors"
                    :class="
                      answers[q.id] === opt
                        ? 'bg-primary text-primary-foreground border-primary'
                        : 'bg-muted/30 border-border text-foreground hover:bg-muted/60'
                    "
                    @click="answers[q.id] = opt"
                  >
                    {{ opt }}
                  </button>
                </div>
                <!-- Free-text answer -->
                <Textarea
                  :model-value="answers[q.id] ?? ''"
                  @update:model-value="answers[q.id] = String($event)"
                  :placeholder="q.defaultAnswer || 'Nhập câu trả lời...'"
                  class="resize-none text-xs leading-relaxed"
                  rows="2"
                />
              </CardContent>
            </Card>
          </div>

          <!-- No issues found -->
          <div
            v-if="!result.issues.length && !result.questions.length"
            class="text-center py-8"
          >
            <span class="text-4xl block mb-3">✅</span>
            <p class="text-sm font-medium">
              Spec rõ ràng, không phát hiện vấn đề nào!
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
            rồi nhấn "Phân tích Spec" để bắt đầu.
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
