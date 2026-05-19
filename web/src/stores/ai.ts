import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/api'
import type { SuggestResponse } from '@/types'

export const useAIStore = defineStore('ai', () => {
  const loading = ref(false)
  const lastResult = ref<string | null>(null)
  const error = ref<string | null>(null)

  async function suggest(section: string, context: string): Promise<SuggestResponse | null> {
    loading.value = true
    error.value = null
    lastResult.value = null
    try {
      const res = await api.ai.suggest(section, context)
      lastResult.value = res.content
      return res
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'AI request failed'
      return null
    } finally {
      loading.value = false
    }
  }

  return { loading, lastResult, error, suggest }
})
