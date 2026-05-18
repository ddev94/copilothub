<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api'
import { useRepoStore } from '@/stores/repo'
import type { FeatureManifest } from '@/types'

const router = useRouter()
const repoStore = useRepoStore()
const features = ref<FeatureManifest[]>([])

function shortRemote(url: string): string {
  return url
    .replace(/^https?:\/\//, '')
    .replace(/^git@([^:]+):/, '$1/')
    .replace(/\.git$/, '')
}

onMounted(async () => {
  repoStore.fetch()
  try {
    const res = await api.hub.features()
    features.value = res.features
  } catch {
    // fallback: show built-in
    features.value = [{
      id: 'spec-designer',
      name: 'Spec Designer',
      version: '1.0.0',
      description: 'Generate SRD documents and user stories from requirements',
      icon: '📄',
      category: 'documentation',
      author: 'copilothub',
      type: 'builtin',
      frontendRoute: '/features/spec-designer',
    }]
  }
})

function open(feature: FeatureManifest) {
  if (feature.type === 'external') {
    window.open(`/features/${feature.id}/`, '_blank')
  } else {
    router.push(feature.frontendRoute)
  }
}
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <header class="border-b border-border px-6 py-4">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-xl font-bold">AI Hub</h1>
          <div v-if="repoStore.info" class="mt-1 space-y-1">
            <p class="text-xs text-muted-foreground font-mono">{{ repoStore.info.path }}</p>
            <div class="flex items-center gap-3 flex-wrap">
            <span v-if="repoStore.info.currentBranch"
              class="inline-flex items-center gap-1 text-xs bg-muted px-2 py-0.5 rounded-full text-foreground">
              <svg class="w-3 h-3 shrink-0" viewBox="0 0 16 16" fill="currentColor">
                <path d="M11.75 2.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5zm-2.25.75a2.25 2.25 0 1 1 3 2.122V6A2.5 2.5 0 0 1 10 8.5H6a1 1 0 0 0-1 1v1.128a2.251 2.251 0 1 1-1.5 0V5.372a2.25 2.25 0 1 1 1.5 0v1.836A2.493 2.493 0 0 1 6 7h4a1 1 0 0 0 1-1v-.628A2.25 2.25 0 0 1 9.5 3.25z"/>
              </svg>
              {{ repoStore.info.currentBranch }}
            </span>
            <a v-if="repoStore.info.remoteOrigin"
              :href="repoStore.info.remoteOrigin.startsWith('http') ? repoStore.info.remoteOrigin : '#'"
              target="_blank"
              class="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
              :title="repoStore.info.remoteOrigin">
              <svg class="w-3 h-3 shrink-0" viewBox="0 0 16 16" fill="currentColor">
                <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8z"/>
              </svg>
              {{ shortRemote(repoStore.info.remoteOrigin) }}
            </a>
            </div>
          </div>
        </div>
      </div>
    </header>

    <main class="px-6 py-8 max-w-4xl mx-auto">
      <section>
        <h2 class="text-sm font-semibold text-muted-foreground uppercase tracking-wide mb-4">
          Built-in Features
        </h2>
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          <div
            v-for="f in features.filter(f => f.type === 'builtin')"
            :key="f.id"
            class="border border-border rounded-lg p-5 flex flex-col gap-3 bg-card hover:border-primary/50 transition-colors"
          >
            <div class="flex items-center gap-3">
              <span class="text-2xl">{{ f.icon }}</span>
              <div class="min-w-0">
                <p class="font-semibold text-sm">{{ f.name }}</p>
                <p class="text-xs text-muted-foreground">v{{ f.version }}</p>
              </div>
            </div>
            <p class="text-sm text-muted-foreground flex-1">{{ f.description }}</p>
            <button
              class="w-full py-2 px-4 text-sm font-medium rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors"
              @click="open(f)"
            >
              Open →
            </button>
          </div>

          <div
            class="border border-dashed border-border rounded-lg p-5 flex flex-col items-center justify-center gap-2 text-center text-muted-foreground cursor-default"
          >
            <span class="text-2xl opacity-40">+</span>
            <p class="text-sm font-medium">Install a feature</p>
            <p class="text-xs">Run: <code class="bg-muted px-1 rounded">copilothub install &lt;repo&gt;</code></p>
          </div>
        </div>
      </section>

      <section v-if="features.some(f => f.type === 'external')" class="mt-10">
        <h2 class="text-sm font-semibold text-muted-foreground uppercase tracking-wide mb-4">
          Installed Features
        </h2>
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          <div
            v-for="f in features.filter(f => f.type === 'external')"
            :key="f.id"
            class="border border-border rounded-lg p-5 flex flex-col gap-3 bg-card"
          >
            <div class="flex items-center gap-3">
              <span class="text-2xl">{{ f.icon || '🔧' }}</span>
              <div class="min-w-0">
                <p class="font-semibold text-sm">{{ f.name }}</p>
                <p class="text-xs text-muted-foreground">v{{ f.version }} · {{ f.author }}</p>
              </div>
            </div>
            <p class="text-sm text-muted-foreground flex-1">{{ f.description }}</p>
            <button
              class="w-full py-2 px-4 text-sm font-medium rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors"
              @click="open(f)"
            >
              Open →
            </button>
          </div>
        </div>
      </section>
    </main>
  </div>
</template>
