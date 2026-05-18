<script setup lang="ts">
import { ref } from "vue";
import { useAIStore } from "@/stores/ai";
import { useSpecStore } from "@/stores/spec";
import { api } from "@/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";

const vAutoResize = {
  mounted(el: HTMLTextAreaElement) {
    el.style.overflow = "hidden";
    el.style.height = "auto";
    el.style.height = el.scrollHeight + "px";
    el.addEventListener("input", () => {
      el.style.height = "auto";
      el.style.height = el.scrollHeight + "px";
    });
  },
  updated(el: HTMLTextAreaElement) {
    el.style.height = "auto";
    el.style.height = el.scrollHeight + "px";
  },
};
import type { ClarifyQuestion } from "@/types";

const aiStore = useAIStore();
const specStore = useSpecStore();
const customPrompt = ref("");

const pendingSuggestion = ref<string | null>(null);
const suggestionMode = ref<"replace" | "append">("append");

// Re-clarify state
const clarifying = ref(false);
const clarifyQuestions = ref<(ClarifyQuestion & { answer: string })[]>([]);
const showClarify = ref(false);

async function regenerateStories() {
  if (!specStore.spec?.requirement) return;
  pendingSuggestion.value = null;
  suggestionMode.value = "replace";
  const result = await aiStore.suggest(specStore.spec.requirement, "");
  if (result) pendingSuggestion.value = result;
}

async function addMoreStories() {
  if (!specStore.spec?.requirement) return;
  const existing = specStore.spec.userStories.map((s) => s.title).join(", ");
  const context = `Existing stories: ${existing}\n\nGenerate additional user stories that are NOT already covered.`;
  pendingSuggestion.value = null;
  suggestionMode.value = "append";
  const result = await aiStore.suggest(specStore.spec.requirement, context);
  if (result) pendingSuggestion.value = result;
}

async function reClarify() {
  if (!specStore.spec) return;
  clarifying.value = true;
  showClarify.value = false;
  clarifyQuestions.value = [];
  try {
    // Build context from current edited content
    const currentContent = buildCurrentContext();
    const res = await api.ai.clarify(currentContent);
    if (res.clear) {
      pendingSuggestion.value =
        "Your current content looks clear and complete. No ambiguities detected.";
    } else {
      clarifyQuestions.value = res.questions.map((q) => ({
        ...q,
        answer: q.suggestion,
      }));
      showClarify.value = true;
    }
  } catch (e) {
    pendingSuggestion.value = `Error: ${e instanceof Error ? e.message : "Failed to analyze"}`;
  } finally {
    clarifying.value = false;
  }
}

function buildCurrentContext(): string {
  const s = specStore.spec;
  if (!s) return "";
  let text = s.requirement ? `Requirement:\n${s.requirement}\n\n` : "";
  for (const story of s.userStories) {
    text += `User Story: ${story.title}\n${story.story}\n`;
    text += `Acceptance Criteria:\n`;
    for (const ac of story.acceptanceCriteria) {
      text += `- ${ac.description}\n`;
    }
    text += `Test Cases:\n`;
    for (const tc of story.testCases) {
      text += `- ${tc.title}: ${tc.steps} → ${tc.expectedResult}\n`;
    }
    text += "\n";
  }
  return text;
}

async function applyReClarify() {
  if (!specStore.spec) return;
  const clarification = clarifyQuestions.value
    .map((q) => `Q: ${q.question}\nA: ${q.answer}`)
    .join("\n\n");
  showClarify.value = false;

  // Regenerate with clarification
  pendingSuggestion.value = null;
  suggestionMode.value = "replace";
  const result = await aiStore.suggest(
    specStore.spec.requirement,
    `Current content has been reviewed. Additional clarification:\n${clarification}\n\nPlease regenerate or refine user stories based on these answers.`,
  );
  if (result) pendingSuggestion.value = result;
}

function dismissClarify() {
  showClarify.value = false;
  clarifyQuestions.value = [];
}

async function sendCustom() {
  if (!customPrompt.value.trim() || !specStore.spec) return;
  pendingSuggestion.value = null;
  suggestionMode.value = "append";
  const result = await aiStore.suggest(
    customPrompt.value,
    specStore.spec.requirement,
  );
  if (result) pendingSuggestion.value = result;
  customPrompt.value = "";
}

function parseStories(raw: string) {
  const parsed = JSON.parse(raw);
  if (!parsed.userStories || !Array.isArray(parsed.userStories)) return null;
  return parsed.userStories.map(
    (s: Omit<import("@/types").UserStory, "id">) => ({
      id: crypto.randomUUID(),
      ...s,
      acceptanceCriteria: (s.acceptanceCriteria ?? []).map(
        (ac: Omit<import("@/types").AcceptanceCriterion, "id">) => ({
          id: crypto.randomUUID(),
          ...ac,
        }),
      ),
      testCases: (s.testCases ?? []).map(
        (tc: Omit<import("@/types").TestCase, "id">) => ({
          id: crypto.randomUUID(),
          ...tc,
        }),
      ),
    }),
  );
}

function applySuggestion() {
  if (!pendingSuggestion.value || !specStore.spec) return;
  try {
    const newStories = parseStories(pendingSuggestion.value);
    if (!newStories) return;
    if (suggestionMode.value === "replace") {
      specStore.updateUserStories(newStories);
    } else {
      specStore.updateUserStories([
        ...specStore.spec.userStories,
        ...newStories,
      ]);
    }
    pendingSuggestion.value = null;
    aiStore.lastResult = null;
  } catch {
    // not parseable JSON – nothing to apply
  }
}

function dismiss() {
  pendingSuggestion.value = null;
  aiStore.lastResult = null;
}
</script>

<template>
  <aside
    class="w-80 shrink-0 flex flex-col border-l border-border bg-card overflow-hidden"
  >
    <div class="px-4 py-3 border-b border-border">
      <p class="font-semibold text-sm">AI Assistant</p>
      <p class="text-xs text-muted-foreground mt-0.5">
        Powered by GitHub Copilot
      </p>
    </div>

    <div class="px-4 py-4 border-b border-border space-y-2">
      <Button
        variant="outline"
        class="w-full h-auto flex items-center gap-3 px-3 py-2.5 text-sm text-left justify-start"
        :disabled="!specStore.spec?.requirement || aiStore.loading"
        @click="regenerateStories"
      >
        <span class="text-base shrink-0">🔄</span>
        <div>
          <p class="font-medium leading-none mb-0.5">Regenerate stories</p>
          <p class="text-xs text-muted-foreground">
            Generate new user stories from the requirements
          </p>
        </div>
      </Button>

      <Button
        variant="outline"
        class="w-full h-auto flex items-center gap-3 px-3 py-2.5 text-sm text-left justify-start"
        :disabled="!specStore.spec?.userStories?.length || aiStore.loading"
        @click="addMoreStories"
      >
        <span class="text-base shrink-0">➕</span>
        <div>
          <p class="font-medium leading-none mb-0.5">Add more stories</p>
          <p class="text-xs text-muted-foreground">
            Generate additional stories not yet covered
          </p>
        </div>
      </Button>

      <Button
        variant="outline"
        class="w-full h-auto flex items-center gap-3 px-3 py-2.5 text-sm text-left justify-start"
        :disabled="
          !specStore.spec?.userStories?.length || aiStore.loading || clarifying
        "
        @click="reClarify"
      >
        <span class="text-base shrink-0">🔍</span>
        <div>
          <p class="font-medium leading-none mb-0.5">Re-clarify</p>
          <p class="text-xs text-muted-foreground">
            Check edits for ambiguity and refine
          </p>
        </div>
      </Button>
    </div>

    <div class="flex-1 overflow-y-auto px-4 py-3">
      <div
        v-if="aiStore.loading || clarifying"
        class="flex flex-col items-center justify-center h-full gap-3 text-center"
      >
        <div
          class="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin"
        />
        <p class="text-xs text-muted-foreground">
          {{
            clarifying ? "Analyzing for ambiguities..." : "AI is generating..."
          }}
        </p>
      </div>

      <!-- Re-clarify Q&A -->
      <div v-else-if="showClarify" class="space-y-3">
        <div class="flex items-center justify-between">
          <p
            class="text-xs font-medium text-muted-foreground uppercase tracking-wide"
          >
            Clarification Needed
          </p>
          <Badge variant="secondary" class="text-xs"
            >{{ clarifyQuestions.length }} questions</Badge
          >
        </div>
        <p class="text-xs text-muted-foreground">
          Some parts of your content are ambiguous. Answer below to improve the
          output.
        </p>

        <div class="space-y-3">
          <Card
            v-for="(q, idx) in clarifyQuestions"
            :key="q.id"
            class="overflow-hidden"
          >
            <CardContent class="p-3 space-y-1.5">
              <Label class="text-xs font-medium"
                >{{ idx + 1 }}. {{ q.question }}</Label
              >
              <Textarea
                v-auto-resize
                v-model="q.answer"
                :placeholder="q.suggestion"
                class="resize-none text-xs leading-relaxed overflow-hidden"
              />
            </CardContent>
          </Card>
        </div>

        <div class="flex gap-2">
          <Button class="flex-1 h-8 text-xs" @click="applyReClarify">
            Apply & Regenerate
          </Button>
          <Button
            variant="outline"
            class="h-8 text-xs px-3"
            @click="dismissClarify"
          >
            Dismiss
          </Button>
        </div>
      </div>

      <div
        v-else-if="aiStore.error"
        class="px-3 py-3 bg-destructive/10 border border-destructive/20 rounded-lg text-xs text-destructive"
      >
        {{ aiStore.error }}
      </div>

      <div v-else-if="pendingSuggestion" class="space-y-3">
        <div class="flex items-center justify-between">
          <p
            class="text-xs font-medium text-muted-foreground uppercase tracking-wide"
          >
            AI Response
          </p>
          <Badge variant="secondary" class="text-xs">
            {{ suggestionMode === "replace" ? "Replace all" : "Append" }}
          </Badge>
        </div>
        <div
          class="bg-muted/50 border border-border rounded-lg p-3 text-xs leading-relaxed max-h-64 overflow-y-auto whitespace-pre-wrap"
        >
          {{ pendingSuggestion }}
        </div>
        <div class="flex gap-2">
          <Button class="flex-1 h-8 text-xs" @click="applySuggestion">
            Apply
          </Button>
          <Button variant="outline" class="h-8 text-xs px-3" @click="dismiss">
            Dismiss
          </Button>
        </div>
      </div>

      <div
        v-else
        class="flex flex-col items-center justify-center h-full gap-2 text-center"
      >
        <span class="text-3xl opacity-20">🤖</span>
        <p class="text-xs text-muted-foreground">
          Choose an action above or ask a custom question.<br />
          AI will help generate and refine user stories.
        </p>
      </div>
    </div>

    <div class="px-4 py-3 border-t border-border space-y-2">
      <p class="text-xs text-muted-foreground font-medium">
        Ask a custom question
      </p>
      <Textarea
        v-auto-resize
        v-model="customPrompt"
        placeholder="e.g. Add edge cases for authentication..."
        class="text-xs resize-none leading-relaxed overflow-hidden"
        @keydown.ctrl.enter="sendCustom"
        @keydown.meta.enter="sendCustom"
      />
      <Button
        class="w-full h-8 text-xs"
        :disabled="aiStore.loading || !customPrompt.trim()"
        @click="sendCustom"
      >
        Send
      </Button>
    </div>
  </aside>
</template>
