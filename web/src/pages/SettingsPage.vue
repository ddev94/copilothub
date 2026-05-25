<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from "vue";
import { useRouter } from "vue-router";
import { api } from "@/api";
import type { Config, AuthStatus, EmbeddingStatus } from "@/types";

const router = useRouter();
const saving = ref(false);
const saveError = ref("");
const saveOk = ref(false);

// AI settings
const aiProvider = ref("copilot");
const aiModel = ref("");
const aiToken = ref("");
const aiBaseURL = ref("");
const copilotStatus = ref<AuthStatus | null>(null);

// Embedding settings
const embedProvider = ref("cybertron");
const embedModel = ref("");
const embedKey = ref("");
const embedURL = ref("");

// Download progress
const embedStatus = ref<EmbeddingStatus>({
  state: "unknown",
  message: "",
  bytes: 0,
  total: 0,
  percent: 0,
});
let eventSource: EventSource | null = null;

const AI_PROVIDERS = [
  { value: "copilot", label: "GitHub Copilot", icon: "🐙" },
  { value: "openai", label: "OpenAI", icon: "🤖" },
  { value: "anthropic", label: "Anthropic Claude", icon: "🔮" },
];

const EMBED_PROVIDERS = [
  { value: "cybertron", label: "Local (all-MiniLM-L6-v2)", icon: "💻" },
  { value: "openai", label: "OpenAI Embeddings", icon: "🌐" },
  { value: "google", label: "Google Gemini", icon: "🔷" },
  { value: "ollama", label: "Ollama", icon: "🦙" },
];

const OPENAI_MODELS = [
  "gpt-4o",
  "gpt-4o-mini",
  "gpt-4-turbo",
  "gpt-3.5-turbo",
  "o1-mini",
];
const ANTHROPIC_MODELS = [
  "claude-opus-4-7",
  "claude-sonnet-4-6",
  "claude-haiku-4-5",
];
const OPENAI_EMBED_MODELS = [
  "text-embedding-3-small",
  "text-embedding-3-large",
  "text-embedding-ada-002",
];
const OLLAMA_EMBED_MODELS = [
  "nomic-embed-text",
  "all-minilm",
  "mxbai-embed-large",
];
const GOOGLE_EMBED_MODELS = ["gemini-embedding-2", "gemini-embedding-001"];

const modelSuggestions = computed(() => {
  if (aiProvider.value === "openai") return OPENAI_MODELS;
  if (aiProvider.value === "anthropic") return ANTHROPIC_MODELS;
  return [];
});

const embedModelSuggestions = computed(() => {
  if (embedProvider.value === "openai") return OPENAI_EMBED_MODELS;
  if (embedProvider.value === "google") return GOOGLE_EMBED_MODELS;
  if (embedProvider.value === "ollama") return OLLAMA_EMBED_MODELS;
  return [];
});

async function loadSettings() {
  try {
    const [cfg, status] = await Promise.all([
      api.config.get(),
      api.auth.status(),
    ]);
    copilotStatus.value = status;
    aiProvider.value = cfg.ai.provider || "copilot";
    aiModel.value = cfg.ai.model || "";
    aiToken.value = cfg.ai.token === "***" ? "" : cfg.ai.token || "";
    aiBaseURL.value = cfg.ai.baseURL || "";
    embedProvider.value = cfg.knowledge.embeddingProvider || "cybertron";
    embedModel.value = cfg.knowledge.embeddingModel || "";
    embedKey.value =
      cfg.knowledge.embeddingKey === "***"
        ? ""
        : cfg.knowledge.embeddingKey || "";
    embedURL.value = cfg.knowledge.embeddingURL || "";
  } catch {
    // ignore
  }
}

async function saveSettings() {
  saving.value = true;
  saveError.value = "";
  saveOk.value = false;
  try {
    const cfg: Config = {
      ai: {
        provider: aiProvider.value,
        token: aiToken.value,
        model: aiModel.value,
        baseURL: aiBaseURL.value,
      },
      knowledge: {
        enabled: true,
        topK: 6,
        embeddingProvider: embedProvider.value,
        embeddingModel: embedModel.value,
        embeddingKey: embedKey.value,
        embeddingURL: embedURL.value,
      },
    };
    await api.config.save(cfg);
    saveOk.value = true;
    setTimeout(() => (saveOk.value = false), 2000);
  } catch (e: unknown) {
    saveError.value = e instanceof Error ? e.message : "Lỗi khi lưu cài đặt";
  } finally {
    saving.value = false;
  }
}

function startEmbedStream() {
  eventSource?.close();
  eventSource = api.embedding.stream();
  eventSource.onmessage = (e) => {
    try {
      embedStatus.value = JSON.parse(e.data);
    } catch {
      /* ignore */
    }
  };
  eventSource.onerror = () => {
    eventSource?.close();
    eventSource = null;
  };
}

async function checkEmbedStatus() {
  try {
    embedStatus.value = await api.embedding.check();
    if (embedStatus.value.state === "downloading") startEmbedStream();
  } catch {
    /* ignore */
  }
}

watch(embedProvider, (v) => {
  if (v === "cybertron") checkEmbedStatus();
  else {
    eventSource?.close();
    eventSource = null;
  }
});

onMounted(() => {
  loadSettings();
  checkEmbedStatus();
});

onUnmounted(() => {
  eventSource?.close();
});
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <!-- Header -->
    <header class="border-b border-border px-6 py-4">
      <div class="flex items-center gap-3">
        <button
          class="text-muted-foreground hover:text-foreground transition-colors"
          @click="router.push('/')"
        >
          <svg class="w-4 h-4" viewBox="0 0 16 16" fill="currentColor">
            <path
              fill-rule="evenodd"
              d="M15 8a.5.5 0 0 0-.5-.5H2.707l3.147-3.146a.5.5 0 1 0-.708-.708l-4 4a.5.5 0 0 0 0 .708l4 4a.5.5 0 0 0 .708-.708L2.707 8.5H14.5A.5.5 0 0 0 15 8z"
            />
          </svg>
        </button>
        <h1 class="text-lg font-semibold">Settings</h1>
      </div>
    </header>

    <main class="max-w-2xl mx-auto px-6 py-8 space-y-8">
      <!-- AI Provider -->
      <section
        class="border border-border rounded-lg bg-card divide-y divide-border"
      >
        <div class="px-5 py-4">
          <h2 class="font-semibold text-sm">AI Provider</h2>
          <p class="text-xs text-muted-foreground mt-0.5">
            Chọn model AI dùng để phân tích spec
          </p>
        </div>
        <div class="px-5 py-5 space-y-4">
          <!-- Provider tabs -->
          <div class="flex gap-2 flex-wrap">
            <button
              v-for="p in AI_PROVIDERS"
              :key="p.value"
              class="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-md border transition-colors"
              :class="
                aiProvider === p.value
                  ? 'border-primary bg-primary/10 text-primary font-medium'
                  : 'border-border hover:border-primary/50 text-muted-foreground'
              "
              @click="aiProvider = p.value"
            >
              {{ p.icon }} {{ p.label }}
            </button>
          </div>

          <!-- Copilot -->
          <div v-if="aiProvider === 'copilot'" class="space-y-3">
            <div
              class="flex items-center gap-2 text-xs px-3 py-2 rounded-md border"
              :class="
                copilotStatus?.cliFound
                  ? 'border-green-500/30 bg-green-500/10 text-green-600'
                  : 'border-destructive/30 bg-destructive/10 text-destructive'
              "
            >
              <span>{{ copilotStatus?.cliFound ? "✓" : "✗" }}</span>
              <span>{{
                copilotStatus?.cliFound
                  ? "GitHub Copilot CLI detected"
                  : "Copilot CLI not found — cài qua VS Code Copilot Chat extension"
              }}</span>
            </div>
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground">
                Model
                <span class="font-normal"
                  >(để trống = dùng mặc định Copilot)</span
                >
              </label>
              <input
                v-model="aiModel"
                placeholder="e.g. gpt-4.1, gpt-4o"
                class="w-full text-sm rounded-md border border-input bg-background px-3 py-2 outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
          </div>

          <!-- OpenAI -->
          <div v-if="aiProvider === 'openai'" class="space-y-3">
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground"
                >API Key <span class="text-destructive">*</span></label
              >
              <input
                v-model="aiToken"
                type="password"
                placeholder="sk-..."
                class="w-full text-sm rounded-md border border-input bg-background px-3 py-2 outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground">Model</label>
              <input
                v-model="aiModel"
                :placeholder="OPENAI_MODELS[0]"
                class="w-full text-sm rounded-md border border-input bg-background px-3 py-2 outline-none focus:ring-1 focus:ring-ring"
              />
              <div class="flex flex-wrap gap-1 mt-1">
                <button
                  v-for="m in modelSuggestions"
                  :key="m"
                  class="text-xs px-2 py-0.5 rounded border border-border hover:border-primary/50 text-muted-foreground hover:text-foreground transition-colors"
                  @click="aiModel = m"
                >
                  {{ m }}
                </button>
              </div>
            </div>
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground">
                Base URL
                <span class="font-normal">(để trống = api.openai.com)</span>
              </label>
              <input
                v-model="aiBaseURL"
                placeholder="https://api.openai.com"
                class="w-full text-sm rounded-md border border-input bg-background px-3 py-2 outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
          </div>

          <!-- Anthropic -->
          <div v-if="aiProvider === 'anthropic'" class="space-y-3">
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground"
                >API Key <span class="text-destructive">*</span></label
              >
              <input
                v-model="aiToken"
                type="password"
                placeholder="sk-ant-..."
                class="w-full text-sm rounded-md border border-input bg-background px-3 py-2 outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground">Model</label>
              <input
                v-model="aiModel"
                :placeholder="ANTHROPIC_MODELS[0]"
                class="w-full text-sm rounded-md border border-input bg-background px-3 py-2 outline-none focus:ring-1 focus:ring-ring"
              />
              <div class="flex flex-wrap gap-1 mt-1">
                <button
                  v-for="m in modelSuggestions"
                  :key="m"
                  class="text-xs px-2 py-0.5 rounded border border-border hover:border-primary/50 text-muted-foreground hover:text-foreground transition-colors"
                  @click="aiModel = m"
                >
                  {{ m }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Embedding Model -->
      <section
        class="border border-border rounded-lg bg-card divide-y divide-border"
      >
        <div class="px-5 py-4">
          <h2 class="font-semibold text-sm">Embedding Model</h2>
          <p class="text-xs text-muted-foreground mt-0.5">
            Dùng cho tính năng Wiki / knowledge search
          </p>
        </div>
        <div class="px-5 py-5 space-y-4">
          <!-- Provider tabs -->
          <div class="flex gap-2 flex-wrap">
            <button
              v-for="p in EMBED_PROVIDERS"
              :key="p.value"
              class="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-md border transition-colors"
              :class="
                embedProvider === p.value
                  ? 'border-primary bg-primary/10 text-primary font-medium'
                  : 'border-border hover:border-primary/50 text-muted-foreground'
              "
              @click="embedProvider = p.value"
            >
              {{ p.icon }} {{ p.label }}
            </button>
          </div>

          <!-- Cybertron -->
          <div v-if="embedProvider === 'cybertron'" class="space-y-3">
            <p class="text-xs text-muted-foreground">
              Chạy hoàn toàn offline sau khi tải model một lần (~100MB). Không
              cần API key.
            </p>
            <!-- Downloading -->
            <div v-if="embedStatus.state === 'downloading'" class="space-y-2">
              <div class="flex items-center justify-between text-xs">
                <span class="text-muted-foreground">{{
                  embedStatus.message
                }}</span>
                <span class="font-mono text-primary"
                  >{{ embedStatus.percent }}%</span
                >
              </div>
              <div class="w-full h-2 bg-muted rounded-full overflow-hidden">
                <div
                  class="h-full bg-primary transition-all duration-300 rounded-full"
                  :style="{ width: embedStatus.percent + '%' }"
                />
              </div>
            </div>
            <!-- Ready -->
            <div
              v-else-if="embedStatus.state === 'ready'"
              class="flex items-center gap-2 text-xs text-green-600"
            >
              <span>✓</span><span>all-MiniLM-L6-v2 sẵn sàng</span>
            </div>
            <!-- Error -->
            <div
              v-else-if="embedStatus.state === 'error'"
              class="text-xs text-destructive"
            >
              {{ embedStatus.message }}
            </div>
            <!-- Unknown -->
            <div v-else class="text-xs text-muted-foreground">
              Model sẽ được tải tự động khi Wiki feature khởi động lần đầu.
            </div>
          </div>

          <!-- OpenAI Embeddings -->
          <div v-if="embedProvider === 'openai'" class="space-y-3">
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground"
                >API Key <span class="text-destructive">*</span></label
              >
              <input
                v-model="embedKey"
                type="password"
                placeholder="sk-..."
                class="w-full text-sm rounded-md border border-input bg-background px-3 py-2 outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground">Model</label>
              <input
                v-model="embedModel"
                :placeholder="OPENAI_EMBED_MODELS[0]"
                class="w-full text-sm rounded-md border border-input bg-background px-3 py-2 outline-none focus:ring-1 focus:ring-ring"
              />
              <div class="flex flex-wrap gap-1 mt-1">
                <button
                  v-for="m in embedModelSuggestions"
                  :key="m"
                  class="text-xs px-2 py-0.5 rounded border border-border hover:border-primary/50 text-muted-foreground hover:text-foreground transition-colors"
                  @click="embedModel = m"
                >
                  {{ m }}
                </button>
              </div>
            </div>
          </div>

          <!-- Ollama Embeddings -->
          <div v-if="embedProvider === 'ollama'" class="space-y-3">
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground">Model</label>
              <input
                v-model="embedModel"
                :placeholder="OLLAMA_EMBED_MODELS[0]"
                class="w-full text-sm rounded-md border border-input bg-background px-3 py-2 outline-none focus:ring-1 focus:ring-ring"
              />
              <div class="flex flex-wrap gap-1 mt-1">
                <button
                  v-for="m in embedModelSuggestions"
                  :key="m"
                  class="text-xs px-2 py-0.5 rounded border border-border hover:border-primary/50 text-muted-foreground hover:text-foreground transition-colors"
                  @click="embedModel = m"
                >
                  {{ m }}
                </button>
              </div>
            </div>
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground">
                Ollama Endpoint
                <span class="font-normal">(để trống = localhost:11434)</span>
              </label>
              <input
                v-model="embedURL"
                placeholder="http://localhost:11434"
                class="w-full text-sm rounded-md border border-input bg-background px-3 py-2 outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
          </div>

          <!-- Google Embeddings -->
          <div v-if="embedProvider === 'google'" class="space-y-3">
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground"
                >API Key <span class="text-destructive">*</span></label
              >
              <input
                v-model="embedKey"
                type="password"
                placeholder="AIza..."
                class="w-full text-sm rounded-md border border-input bg-background px-3 py-2 outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
            <div class="space-y-1">
              <label class="text-xs text-muted-foreground">Model</label>
              <input
                v-model="embedModel"
                :placeholder="GOOGLE_EMBED_MODELS[0]"
                class="w-full text-sm rounded-md border border-input bg-background px-3 py-2 outline-none focus:ring-1 focus:ring-ring"
              />
              <div class="flex flex-wrap gap-1 mt-1">
                <button
                  v-for="m in embedModelSuggestions"
                  :key="m"
                  class="text-xs px-2 py-0.5 rounded border border-border hover:border-primary/50 text-muted-foreground hover:text-foreground transition-colors"
                  @click="embedModel = m"
                >
                  {{ m }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Save bar -->
      <div class="flex items-center justify-between">
        <p v-if="saveError" class="text-xs text-destructive">{{ saveError }}</p>
        <p v-else-if="saveOk" class="text-xs text-green-600">✓ Đã lưu</p>
        <div v-else />
        <div class="flex gap-2">
          <button
            class="text-sm px-4 py-2 rounded-md border border-border hover:bg-muted transition-colors"
            @click="router.push('/')"
          >
            Huỷ
          </button>
          <button
            class="text-sm px-4 py-2 rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50"
            :disabled="saving"
            @click="saveSettings"
          >
            {{ saving ? "Đang lưu..." : "Lưu cài đặt" }}
          </button>
        </div>
      </div>
    </main>
  </div>
</template>
