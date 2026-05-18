<script setup lang="ts">
import { useSpecStore } from '@/stores/spec'
import { useRepoStore } from '@/stores/repo'
import { Button } from '@/components/ui/button'
import ConfigDialog from '@/components/ConfigDialog.vue'

const emit = defineEmits<{ newDoc: [] }>()

const specStore = useSpecStore()
const repoStore = useRepoStore()

function formatDate(d: string) {
  const date = new Date(d)
  const now = new Date()
  const diffDays = Math.floor((now.getTime() - date.getTime()) / 86400000)
  if (diffDays === 0) return 'Today'
  if (diffDays === 1) return 'Yesterday'
  if (diffDays < 7) return `${diffDays}d ago`
  return date.toLocaleDateString('en', { month: 'short', day: 'numeric' })
}
</script>

<template>
  <aside class="w-56 shrink-0 flex flex-col border-r border-border bg-card overflow-hidden">
    <!-- Header -->
    <div class="px-4 py-3 border-b border-border">
      <div class="flex items-center justify-between mb-0.5">
        <span class="text-sm font-bold tracking-tight">spec-designer</span>
        <Button variant="ghost" size="icon-sm" class="h-6 w-6 text-base" title="New SRD" @click="emit('newDoc')">
          +
        </Button>
      </div>
      <p v-if="repoStore.info" class="text-xs text-muted-foreground truncate">
        {{ repoStore.info.name }}
      </p>
    </div>

    <!-- Document list -->
    <div class="flex-1 overflow-y-auto py-1">
      <p class="px-4 pt-2 pb-1 text-xs font-medium text-muted-foreground uppercase tracking-wide">
        Documents
      </p>

      <button
        v-for="meta in specStore.specsList"
        :key="meta.id"
        class="w-full text-left px-3 py-2 mx-1 text-sm rounded-md transition-colors"
        :class="meta.id === specStore.spec?.id
          ? 'bg-primary text-primary-foreground'
          : 'hover:bg-muted text-foreground'"
        style="width: calc(100% - 8px)"
        @click="specStore.openSpec(meta.id)"
      >
        <p class="truncate text-sm leading-snug font-medium">{{ meta.title }}</p>
        <p class="text-xs mt-0.5"
           :class="meta.id === specStore.spec?.id ? 'text-primary-foreground/70' : 'text-muted-foreground'">
          {{ formatDate(meta.updatedAt) }}
        </p>
      </button>

      <div v-if="!specStore.specsList.length"
           class="px-4 py-8 text-xs text-muted-foreground text-center">
        No documents yet
      </div>
    </div>

    <!-- Footer -->
    <div class="px-4 py-2.5 border-t border-border">
      <ConfigDialog />
    </div>
  </aside>
</template>
