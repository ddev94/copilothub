<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api'
import { useRepoStore } from '@/stores/repo'
import type { FeatureManifest } from '@/types'

const router = useRouter()
const repoStore = useRepoStore()
const features = ref<FeatureManifest[]>([])

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
          <p v-if="repoStore.info" class="text-xs text-muted-foreground mt-0.5">
            {{ repoStore.info.name }}
          </p>
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
