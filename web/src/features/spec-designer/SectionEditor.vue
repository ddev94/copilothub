<script setup lang="ts">
import { ref } from "vue";
import { useSpecStore } from "@/stores/spec";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { AcceptanceCriterion, TestCase } from "@/types";
import { CornerDownLeft, X } from "lucide-vue-next";

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

const specStore = useSpecStore();
const expandedStoryIds = ref<Set<string>>(new Set());

function toggleExpand(id: string) {
  if (expandedStoryIds.value.has(id)) {
    expandedStoryIds.value.delete(id);
  } else {
    expandedStoryIds.value.add(id);
  }
  // trigger reactivity
  expandedStoryIds.value = new Set(expandedStoryIds.value);
}

function removeStory(id: string) {
  specStore.removeUserStory(id);
}

function updateStoryField(id: string, field: string, value: string) {
  specStore.updateUserStory(id, { [field]: value });
}

function updateAC(storyId: string, acId: string, patch: Partial<AcceptanceCriterion>) {
  const story = specStore.spec?.userStories.find((s) => s.id === storyId);
  if (!story) return;
  const updated = story.acceptanceCriteria.map((ac) =>
    ac.id === acId ? { ...ac, ...patch } : ac,
  );
  specStore.updateUserStory(storyId, { acceptanceCriteria: updated });
}

function removeAC(storyId: string, acId: string) {
  const story = specStore.spec?.userStories.find((s) => s.id === storyId);
  if (!story) return;
  specStore.updateUserStory(storyId, {
    acceptanceCriteria: story.acceptanceCriteria.filter((ac) => ac.id !== acId),
  });
}

function addAC(storyId: string) {
  const story = specStore.spec?.userStories.find((s) => s.id === storyId);
  if (!story) return;
  const newAC: AcceptanceCriterion = {
    id: crypto.randomUUID(),
    given: "",
    when: "",
    then: "",
  };
  specStore.updateUserStory(storyId, {
    acceptanceCriteria: [...story.acceptanceCriteria, newAC],
  });
}

function updateTC(storyId: string, tcId: string, patch: Partial<TestCase>) {
  const story = specStore.spec?.userStories.find((s) => s.id === storyId);
  if (!story) return;
  const updated = story.testCases.map((tc) =>
    tc.id === tcId ? { ...tc, ...patch } : tc,
  );
  specStore.updateUserStory(storyId, { testCases: updated });
}

function removeTC(storyId: string, tcId: string) {
  const story = specStore.spec?.userStories.find((s) => s.id === storyId);
  if (!story) return;
  specStore.updateUserStory(storyId, {
    testCases: story.testCases.filter((tc) => tc.id !== tcId),
  });
}

function addTC(storyId: string) {
  const story = specStore.spec?.userStories.find((s) => s.id === storyId);
  if (!story) return;
  const newTC: TestCase = {
    id: crypto.randomUUID(),
    title: "",
    steps: "",
    expectedResult: "",
  };
  specStore.updateUserStory(storyId, {
    testCases: [...story.testCases, newTC],
  });
}

function addNewStory() {
  specStore.addUserStory({
    title: "",
    story: "",
    acceptanceCriteria: [],
    testCases: [],
  });
}
</script>

<template>
  <div class="h-full flex flex-col overflow-hidden">
    <!-- No spec open -->
    <div
      v-if="!specStore.spec"
      class="flex-1 flex flex-col items-center justify-center gap-3 text-center p-8"
    >
      <span class="text-5xl opacity-15">📋</span>
      <p class="text-sm text-muted-foreground">
        Select or create a document to view user stories
      </p>
    </div>

    <template v-else>
      <!-- Requirement (editable) -->
      <div class="px-6 py-4 border-b border-border shrink-0">
        <h3
          class="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2"
        >
          Requirement
        </h3>
        <Textarea
          v-auto-resize
          :model-value="specStore.spec.requirement"
          @update:model-value="specStore.updateRequirement(String($event))"
          placeholder="Describe your requirement here..."
          class="resize-none text-sm leading-relaxed overflow-hidden"
        />
      </div>

      <!-- User Stories list -->
      <ScrollArea class="flex-1">
        <div class="p-6 space-y-6">
          <div
            v-if="!specStore.spec.userStories.length"
            class="text-center py-12 text-muted-foreground text-sm"
          >
            No user stories yet. Generate from a requirement or add manually.
          </div>

          <Card
            v-for="(story, idx) in specStore.spec.userStories"
            :key="story.id"
            class="overflow-hidden"
            :class="{
              'ring-2 ring-primary': specStore.activeStoryId === story.id,
            }"
            @click="specStore.activeStoryId = story.id"
          >
            <CardHeader>
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2 flex-1 min-w-0">
                  <Badge variant="outline" class="shrink-0"
                    >US-{{ idx + 1 }}</Badge
                  >
                  <Input
                    v-if="expandedStoryIds.has(story.id)"
                    :model-value="story.title"
                    @update:model-value="
                      updateStoryField(story.id, 'title', String($event))
                    "
                    @click.stop
                    placeholder="User story title..."
                    class="h-7 text-sm font-semibold border-transparent hover:border-border focus:border-border"
                  />
                  <span
                    v-else
                    class="flex-1 text-sm font-semibold truncate cursor-pointer"
                    @click.stop="toggleExpand(story.id)"
                    >{{ story.title || "Untitled story" }}</span
                  >
                  <Button
                    variant="ghost"
                    size="sm"
                    class="h-6 w-6 p-0 text-muted-foreground hover:text-destructive shrink-0 ml-1"
                    @click.stop="toggleExpand(story.id)"
                  >
                    <CornerDownLeft />
                  </Button>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  class="h-6 w-6 p-0 text-muted-foreground hover:text-destructive shrink-0 ml-1"
                  @click.stop="removeStory(story.id)"
                >
                  <X />
                </Button>
              </div>
              <Textarea
                v-if="expandedStoryIds.has(story.id)"
                v-auto-resize
                :model-value="story.story"
                @update:model-value="
                  updateStoryField(story.id, 'story', String($event))
                "
                @click.stop
                placeholder="As a [role], I want [feature], so that [benefit]"
                class="mt-1 resize-none text-sm italic text-muted-foreground border-transparent hover:border-border focus:border-border overflow-hidden"
              />
            </CardHeader>

            <CardContent
              v-if="expandedStoryIds.has(story.id)"
              class="space-y-4"
              @click.stop
            >
              <!-- Acceptance Criteria -->
              <div>
                <div class="flex items-center justify-between mb-2">
                  <h4
                    class="text-xs font-semibold text-muted-foreground uppercase tracking-wide"
                  >
                    Acceptance Criteria
                  </h4>
                  <Button
                    variant="ghost"
                    size="sm"
                    class="h-5 text-xs px-2 text-muted-foreground"
                    @click="addAC(story.id)"
                    >+ Add</Button
                  >
                </div>
                <div class="space-y-2">
                  <div
                    v-for="(ac, acIdx) in story.acceptanceCriteria"
                    :key="ac.id"
                    class="rounded-md border border-border overflow-hidden group"
                  >
                    <div class="flex items-center justify-between px-2.5 py-1 bg-muted/40 border-b border-border">
                      <span class="text-xs font-medium text-muted-foreground">AC-{{ acIdx + 1 }}</span>
                      <Button
                        variant="ghost"
                        size="sm"
                        class="h-5 w-5 p-0 text-muted-foreground hover:text-destructive opacity-0 group-hover:opacity-100 shrink-0"
                        @click="removeAC(story.id, ac.id)"
                        >×</Button
                      >
                    </div>
                    <div class="divide-y divide-border">
                      <div class="flex items-start gap-0">
                        <span class="text-[10px] font-bold text-emerald-600 bg-emerald-50 px-2 py-2 shrink-0 w-14 text-center leading-relaxed border-r border-border">GIVEN</span>
                        <Textarea
                          v-auto-resize
                          :model-value="ac.given"
                          @update:model-value="updateAC(story.id, ac.id, { given: String($event) })"
                          placeholder="điều kiện / ngữ cảnh ban đầu..."
                          class="flex-1 resize-none text-sm border-0 rounded-none focus-visible:ring-0 overflow-hidden py-1.5 px-2.5"
                        />
                      </div>
                      <div class="flex items-start gap-0">
                        <span class="text-[10px] font-bold text-amber-600 bg-amber-50 px-2 py-2 shrink-0 w-14 text-center leading-relaxed border-r border-border">WHEN</span>
                        <Textarea
                          v-auto-resize
                          :model-value="ac.when"
                          @update:model-value="updateAC(story.id, ac.id, { when: String($event) })"
                          placeholder="hành động của người dùng..."
                          class="flex-1 resize-none text-sm border-0 rounded-none focus-visible:ring-0 overflow-hidden py-1.5 px-2.5"
                        />
                      </div>
                      <div class="flex items-start gap-0">
                        <span class="text-[10px] font-bold text-blue-600 bg-blue-50 px-2 py-2 shrink-0 w-14 text-center leading-relaxed border-r border-border">THEN</span>
                        <Textarea
                          v-auto-resize
                          :model-value="ac.then"
                          @update:model-value="updateAC(story.id, ac.id, { then: String($event) })"
                          placeholder="kết quả mong đợi..."
                          class="flex-1 resize-none text-sm border-0 rounded-none focus-visible:ring-0 overflow-hidden py-1.5 px-2.5"
                        />
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Test Cases -->
              <div>
                <div class="flex items-center justify-between mb-2">
                  <h4
                    class="text-xs font-semibold text-muted-foreground uppercase tracking-wide"
                  >
                    Test Cases
                  </h4>
                  <Button
                    variant="ghost"
                    size="sm"
                    class="h-5 text-xs px-2 text-muted-foreground"
                    @click="addTC(story.id)"
                    >+ Add</Button
                  >
                </div>
                <div class="space-y-3">
                  <div
                    v-for="(tc, tcIdx) in story.testCases"
                    :key="tc.id"
                    class="rounded-md border border-border p-3 text-sm space-y-2 group/tc"
                  >
                    <div class="flex items-center gap-2">
                      <Badge variant="secondary" class="text-xs shrink-0"
                        >TC-{{ tcIdx + 1 }}</Badge
                      >
                      <Input
                        :model-value="tc.title"
                        @update:model-value="
                          updateTC(story.id, tc.id, { title: String($event) })
                        "
                        placeholder="Test case title..."
                        class="h-7 text-sm font-medium border-transparent hover:border-border focus:border-border"
                      />
                      <Button
                        variant="ghost"
                        size="sm"
                        class="h-6 w-6 p-0 text-muted-foreground hover:text-destructive opacity-0 group-hover/tc:opacity-100 shrink-0"
                        @click="removeTC(story.id, tc.id)"
                        >×</Button
                      >
                    </div>
                    <div class="space-y-1">
                      <span class="text-xs font-medium text-muted-foreground"
                        >Steps</span
                      >
                      <Textarea
                        v-auto-resize
                        :model-value="tc.steps"
                        @update:model-value="
                          updateTC(story.id, tc.id, { steps: String($event) })
                        "
                        placeholder="Step-by-step instructions..."
                        class="resize-none text-sm border-transparent hover:border-border focus:border-border overflow-hidden"
                      />
                    </div>
                    <div class="space-y-1">
                      <span class="text-xs font-medium text-muted-foreground"
                        >Expected Result</span
                      >
                      <Textarea
                        v-auto-resize
                        :model-value="tc.expectedResult"
                        @update:model-value="
                          updateTC(story.id, tc.id, {
                            expectedResult: String($event),
                          })
                        "
                        placeholder="Expected outcome..."
                        class="resize-none text-sm border-transparent hover:border-border focus:border-border overflow-hidden"
                      />
                    </div>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          <!-- Add new story button -->
          <Button
            variant="outline"
            class="w-full border-dashed"
            @click="addNewStory"
          >
            + Add User Story
          </Button>
        </div>
      </ScrollArea>
    </template>
  </div>
</template>
