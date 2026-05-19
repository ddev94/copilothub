<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogClose,
} from '@/components/ui/dialog'

const open = ref(false)
const saving = ref(false)
const cliFound = ref(false)
const tokenOverride = ref('')
const model = ref('')

onMounted(async () => {
  try {
    const [status, cfg] = await Promise.all([api.auth.status(), api.config.get()])
    cliFound.value = status.cliFound
    model.value = cfg.ai.model ?? ''
  } catch {
    // ignore
  }
})

async function save() {
  saving.value = true
  try {
    await api.config.save({
      ai: { token: tokenOverride.value, model: model.value },
      knowledge: { enabled: true, serviceUrl: 'http://localhost:8001', topK: 6 },
    })
    open.value = false
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogTrigger as-child>
      <Button
        variant="ghost"
        class="w-full justify-start text-xs text-muted-foreground hover:text-foreground h-auto p-0"
      >
        ⚙ Settings
      </Button>
    </DialogTrigger>

    <DialogContent class="max-w-md">
      <DialogHeader>
        <DialogTitle>Settings</DialogTitle>
      </DialogHeader>

      <!-- Auth status -->
      <div
        class="px-3 py-2.5 rounded-md border text-sm flex items-center gap-2"
        :class="cliFound
          ? 'border-green-500/30 bg-green-500/10 text-green-400'
          : 'border-destructive/30 bg-destructive/10 text-destructive'"
      >
        <span>{{ cliFound ? '✓' : '✗' }}</span>
        <span>{{ cliFound ? 'GitHub Copilot CLI detected' : 'Copilot CLI not found' }}</span>
      </div>

      <div class="space-y-3 text-xs text-muted-foreground">
        <p>Authentication is handled automatically via your logged-in GitHub account.</p>
        <p>The CLI is picked up from the VS Code Copilot extension or <code class="bg-muted px-1 rounded">COPILOT_CLI_PATH</code> env var.</p>
      </div>

      <div class="space-y-3">
        <div class="space-y-1">
          <Label class="text-xs font-medium text-muted-foreground">
            Model
            <span class="font-normal">(leave blank for Copilot default)</span>
          </Label>
          <Input
            v-model="model"
            placeholder="e.g. gpt-4o, claude-sonnet-4-5"
          />
        </div>

        <div class="space-y-1">
          <Label class="text-xs font-medium text-muted-foreground">
            GitHub Token Override
            <span class="font-normal">(optional)</span>
          </Label>
          <Input
            v-model="tokenOverride"
            type="password"
            placeholder="ghp_... — only needed if auto-auth fails"
          />
        </div>
      </div>

      <DialogFooter>
        <DialogClose as-child>
          <Button variant="outline" @click="open = false">Cancel</Button>
        </DialogClose>
        <Button :disabled="saving" @click="save">
          {{ saving ? 'Saving...' : 'Save' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
