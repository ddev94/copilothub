import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/api'
import type { RepoInfo } from '@/types'

export const useRepoStore = defineStore('repo', () => {
  const info = ref<RepoInfo | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetch() {
    loading.value = true
    error.value = null
    try {
      info.value = await api.repo.info()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load repo'
    } finally {
      loading.value = false
    }
  }

  return { info, loading, error, fetch }
})
