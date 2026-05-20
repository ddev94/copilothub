<script setup lang="ts">
import { useRouter } from 'vue-router'
import { Button } from '@/components/ui/button'

const router = useRouter()
</script>

<template>
  <Button
    variant="ghost"
    class="w-full justify-start text-xs text-muted-foreground hover:text-foreground h-auto p-0"
    @click="router.push('/')"
  >
    ⚙ Settings
  </Button>
</template>
