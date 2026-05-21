<script setup lang="ts">
import { computed } from "vue";
import { diffLines } from "diff";

const props = defineProps<{
  original: string;
  revised: string;
}>();

interface DiffLine {
  type: "added" | "removed" | "unchanged";
  lineNo: { old: number | null; new: number | null };
  text: string;
}

const diffLines_ = computed((): DiffLine[] => {
  const changes = diffLines(props.original, props.revised);
  const lines: DiffLine[] = [];
  let oldLine = 1;
  let newLine = 1;

  for (const change of changes) {
    // change.value may end with a trailing newline — split carefully
    const parts = change.value.split("\n");
    // The last element after split on a trailing \n is an empty string — drop it
    if (parts[parts.length - 1] === "") parts.pop();

    for (const text of parts) {
      if (change.added) {
        lines.push({ type: "added", lineNo: { old: null, new: newLine++ }, text });
      } else if (change.removed) {
        lines.push({ type: "removed", lineNo: { old: oldLine++, new: null }, text });
      } else {
        lines.push({ type: "unchanged", lineNo: { old: oldLine++, new: newLine++ }, text });
      }
    }
  }
  return lines;
});

const stats = computed(() => {
  let added = 0;
  let removed = 0;
  for (const l of diffLines_.value) {
    if (l.type === "added") added++;
    else if (l.type === "removed") removed++;
  }
  return { added, removed };
});
</script>

<template>
  <div class="rounded-md border border-border overflow-hidden font-mono text-xs">
    <!-- Stat bar -->
    <div class="flex items-center gap-3 px-3 py-1.5 bg-muted/40 border-b border-border text-[11px]">
      <span class="text-muted-foreground">Diff</span>
      <span class="text-green-600 font-medium">+{{ stats.added }}</span>
      <span class="text-red-500 font-medium">-{{ stats.removed }}</span>
    </div>

    <!-- Diff lines -->
    <div class="overflow-auto max-h-[480px]">
      <table class="w-full border-collapse">
        <tbody>
          <tr
            v-for="(line, idx) in diffLines_"
            :key="idx"
            :class="{
              'bg-green-500/10': line.type === 'added',
              'bg-red-500/10': line.type === 'removed',
            }"
          >
            <!-- Old line number -->
            <td
              class="select-none text-right px-2 py-0 w-10 border-r border-border text-muted-foreground/50 leading-5"
              :class="{
                'bg-green-500/5': line.type === 'added',
                'bg-red-500/5': line.type === 'removed',
              }"
            >
              {{ line.lineNo.old ?? "" }}
            </td>
            <!-- New line number -->
            <td
              class="select-none text-right px-2 py-0 w-10 border-r border-border text-muted-foreground/50 leading-5"
              :class="{
                'bg-green-500/5': line.type === 'added',
                'bg-red-500/5': line.type === 'removed',
              }"
            >
              {{ line.lineNo.new ?? "" }}
            </td>
            <!-- Sign -->
            <td
              class="select-none text-center px-1 py-0 w-5 leading-5 font-semibold"
              :class="{
                'text-green-600': line.type === 'added',
                'text-red-500': line.type === 'removed',
                'text-transparent': line.type === 'unchanged',
              }"
            >
              {{ line.type === "added" ? "+" : line.type === "removed" ? "-" : " " }}
            </td>
            <!-- Content -->
            <td
              class="px-2 py-0 leading-5 whitespace-pre-wrap break-words"
              :class="{
                'text-green-700 dark:text-green-400': line.type === 'added',
                'text-red-700 dark:text-red-400': line.type === 'removed',
                'text-foreground': line.type === 'unchanged',
              }"
            >{{ line.text }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
