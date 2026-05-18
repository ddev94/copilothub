<script setup lang="ts">
import { onMounted, ref, computed } from "vue";
import { useSpecStore } from "@/stores/spec";
import { useRepoStore } from "@/stores/repo";
import DocList from "@/components/DocList.vue";
import SectionEditor from "@/components/SectionEditor.vue";
import AIPanel from "@/components/AIPanel.vue";
import WelcomeScreen from "@/components/WelcomeScreen.vue";
import { Button } from "@/components/ui/button";

const specStore = useSpecStore();
const repoStore = useRepoStore();
const showWelcome = ref(false);

onMounted(async () => {
  repoStore.fetch();
  await specStore.openLatest();
  if (!specStore.spec) showWelcome.value = true;
});

function onWelcomeDone() {
  showWelcome.value = false;
}

function exportMarkdown() {
  const s = specStore.spec;
  if (!s) return;
  let md = `# ${s.title}\n\n`;

  if (s.requirement) {
    md += `## Requirement\n\n${s.requirement}\n\n---\n\n`;
  }

  for (const [idx, story] of s.userStories.entries()) {
    md += `## US-${idx + 1}: ${story.title}\n\n`;
    md += `> ${story.story}\n\n`;

    md += `### Acceptance Criteria\n\n`;
    for (const [acIdx, ac] of story.acceptanceCriteria.entries()) {
      md += `${acIdx + 1}. ${ac.description}\n`;
    }
    md += "\n";

    md += `### Test Cases\n\n`;
    for (const [tcIdx, tc] of story.testCases.entries()) {
      md += `**TC-${tcIdx + 1}: ${tc.title}**\n`;
      md += `- Steps: ${tc.steps}\n`;
      md += `- Expected: ${tc.expectedResult}\n\n`;
    }
    md += "---\n\n";
  }

  const blob = new Blob([md], { type: "text/markdown" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${s.title.replace(/[^a-z0-9]/gi, "-").toLowerCase()}.md`;
  a.click();
  URL.revokeObjectURL(url);
}

const saveLabel = computed(() => {
  if (specStore.saving) return "Saving…";
  if (!specStore.lastSavedAt) return null;
  const diff = Math.floor(
    (Date.now() - specStore.lastSavedAt.getTime()) / 1000,
  );
  if (diff < 5) return "Saved";
  if (diff < 60) return `Saved ${diff}s ago`;
  return `Saved ${Math.floor(diff / 60)}m ago`;
});
</script>

<template>
  <WelcomeScreen v-if="showWelcome" @done="onWelcomeDone" />

  <div
    v-else
    class="flex h-screen bg-background text-foreground overflow-hidden"
  >
    <!-- Left: spec list -->
    <DocList @new-doc="showWelcome = true" />

    <!-- Middle: editor -->
    <main class="flex-1 flex flex-col min-w-0">
      <!-- Top bar -->
      <div
        class="flex items-center justify-between px-4 py-2 border-b border-border shrink-0"
      >
        <span class="text-sm font-medium truncate text-muted-foreground">
          {{ specStore.spec?.title ?? "No document open" }}
        </span>
        <div class="flex items-center gap-2 shrink-0">
          <span v-if="saveLabel" class="text-xs text-muted-foreground">{{
            saveLabel
          }}</span>
          <Button variant="outline" size="sm" @click="exportMarkdown"
            >Export ↓</Button
          >
          <Button as-child size="sm">
            <RouterLink to="/preview">Preview</RouterLink>
          </Button>
        </div>
      </div>

      <div class="flex-1 overflow-hidden">
        <SectionEditor />
      </div>
    </main>

    <!-- Right: AI chat -->
    <AIPanel />
  </div>
</template>
