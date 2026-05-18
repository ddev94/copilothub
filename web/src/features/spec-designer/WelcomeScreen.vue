<script setup lang="ts">
import { ref } from "vue";
import { useSpecStore } from "@/stores/spec";
import { api } from "@/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { ClarifyQuestion } from "@/types";

const emit = defineEmits<{ done: [] }>();

const specStore = useSpecStore();

// Step: 'input' | 'clarify' | 'generating'
const step = ref<"input" | "clarify" | "generating">("input");
const title = ref("");
const requirement = ref("");
const error = ref("");
const progressMsg = ref("");

// Clarification state
const questions = ref<(ClarifyQuestion & { answer: string })[]>([]);
const clarifying = ref(false);

async function analyze() {
  error.value = "";
  clarifying.value = true;
  try {
    const res = await api.ai.clarify(requirement.value);
    if (res.clear) {
      // Requirement is clear enough — go directly to generate
      await generate("");
    } else {
      // Show clarification questions
      questions.value = res.questions.map((q) => ({
        ...q,
        answer: q.suggestion,
      }));
      step.value = "clarify";
    }
  } catch (e) {
    error.value =
      e instanceof Error ? e.message : "Failed to analyze requirement.";
  } finally {
    clarifying.value = false;
  }
}

async function generate(clarification: string) {
  step.value = "generating";
  error.value = "";
  progressMsg.value = "Generating user stories...";
  try {
    const generated = await api.ai.generateSpec({
      title: title.value,
      requirement: requirement.value,
      clarification,
    });
    progressMsg.value = "Saving document...";
    await specStore.saveGenerated(generated);
    emit("done");
  } catch (e) {
    error.value =
      e instanceof Error
        ? e.message
        : "Generation failed. Check your AI token in Settings.";
    step.value = "input";
  } finally {
    progressMsg.value = "";
  }
}

function submitClarification() {
  const clarification = questions.value
    .map((q) => `Q: ${q.question}\nA: ${q.answer}`)
    .join("\n\n");
  generate(clarification);
}

function skipClarification() {
  generate("");
}
</script>

<template>
  <!-- Loading overlay -->
  <Teleport to="body">
    <div
      v-if="step === 'generating' || clarifying"
      class="fixed inset-0 z-50 flex flex-col items-center justify-center bg-background/95 backdrop-blur-sm gap-5"
    >
      <div
        class="w-10 h-10 border-2 border-primary border-t-transparent rounded-full animate-spin"
      />
      <div class="text-center space-y-1">
        <p class="text-sm font-medium">
          {{ clarifying ? "Analyzing requirement..." : progressMsg }}
        </p>
        <p class="text-xs text-muted-foreground">
          {{
            clarifying
              ? "Checking for ambiguities"
              : "AI is generating user stories — this may take 30–60 seconds"
          }}
        </p>
      </div>
    </div>
  </Teleport>

  <!-- Welcome screen -->
  <div class="flex h-screen bg-background text-foreground">
    <div class="m-auto w-full max-w-xl px-6 py-12 space-y-8">
      <!-- Step 1: Input requirement -->
      <template v-if="step === 'input'">
        <div class="space-y-1">
          <h1 class="text-2xl font-bold tracking-tight">
            Generate User Stories
          </h1>
          <p class="text-sm text-muted-foreground">
            Describe your requirement. AI will check for ambiguity and generate
            User Stories with Acceptance Criteria and Test Cases.
          </p>
        </div>

        <div class="space-y-4">
          <div class="space-y-1.5">
            <Label class="text-sm font-medium"
              >Title
              <span class="text-muted-foreground font-normal text-xs ml-1"
                >(optional)</span
              ></Label
            >
            <Input
              v-model="title"
              placeholder="e.g. User Authentication Feature"
            />
          </div>

          <div class="space-y-1.5">
            <Label class="text-sm font-medium">
              Requirement <span class="text-destructive">*</span>
            </Label>
            <Textarea
              v-model="requirement"
              placeholder="Describe your requirement...&#10;&#10;Example:&#10;- Users should be able to register with email and password&#10;- Users can login with email/password or OAuth (Google, GitHub)&#10;- Password reset via email&#10;- Session management with JWT tokens&#10;- Account deactivation"
              :rows="10"
              class="resize-none leading-relaxed"
            />
          </div>
        </div>

        <div
          v-if="error"
          class="px-4 py-3 bg-destructive/10 border border-destructive/20 rounded-md text-sm text-destructive"
        >
          {{ error }}
        </div>

        <div class="flex items-center gap-3">
          <Button
            class="flex-1"
            :disabled="!requirement.trim()"
            @click="analyze"
          >
            Generate User Stories
          </Button>
          <Button variant="outline" @click="emit('done')"> Start Blank </Button>
        </div>

        <p class="text-xs text-center text-muted-foreground">
          Uses your GitHub Copilot subscription via the local CLI
        </p>
      </template>

      <!-- Step 2: Clarification -->
      <template v-if="step === 'clarify'">
        <div class="space-y-1">
          <div class="flex items-center gap-2">
            <h1 class="text-2xl font-bold tracking-tight">
              Clarification Needed
            </h1>
            <Badge variant="secondary">{{ questions.length }} questions</Badge>
          </div>
          <p class="text-sm text-muted-foreground">
            Your requirement has some ambiguous areas. Please clarify or use the
            suggested defaults.
          </p>
        </div>

        <!-- Original requirement (collapsed) -->
        <Card class="bg-muted/30">
          <CardContent class="p-3">
            <p
              class="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-1"
            >
              Your Requirement
            </p>
            <p class="text-sm whitespace-pre-wrap line-clamp-3">
              {{ requirement }}
            </p>
          </CardContent>
        </Card>

        <!-- Questions -->
        <div class="space-y-4">
          <div v-for="(q, idx) in questions" :key="q.id" class="space-y-1.5">
            <Label class="text-sm font-medium">
              {{ idx + 1 }}. {{ q.question }}
            </Label>
            <Textarea
              v-model="q.answer"
              :rows="2"
              class="resize-none text-sm leading-relaxed"
              :placeholder="q.suggestion"
            />
          </div>
        </div>

        <div
          v-if="error"
          class="px-4 py-3 bg-destructive/10 border border-destructive/20 rounded-md text-sm text-destructive"
        >
          {{ error }}
        </div>

        <div class="flex items-center gap-3">
          <Button class="flex-1" @click="submitClarification">
            Generate with Answers
          </Button>
          <Button variant="outline" @click="skipClarification">
            Skip & Generate Anyway
          </Button>
        </div>

        <Button
          variant="ghost"
          class="w-full text-xs text-muted-foreground"
          @click="step = 'input'"
        >
          ← Back to edit requirement
        </Button>
      </template>
    </div>
  </div>
</template>
