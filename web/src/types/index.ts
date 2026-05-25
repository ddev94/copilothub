export interface FileNode {
  name: string;
  path: string;
  isDir: boolean;
  children?: FileNode[];
}

export interface RepoInfo {
  name: string;
  path: string;
  remoteOrigin: string;
  currentBranch: string;
  techStack: string[];
  fileTree: FileNode[];
}

export interface AIConfig {
  provider: string; // "copilot" | "openai" | "anthropic"
  token: string;
  model: string;
  baseURL: string;
}

export interface KnowledgeConfig {
  enabled: boolean;
  topK: number;
  embeddingProvider: string; // "cybertron" | "openai" | "ollama" | "google"
  embeddingModel: string;
  embeddingKey: string;
  embeddingURL: string;
}

export type EmbeddingState = "unknown" | "ready" | "downloading" | "error";

export interface EmbeddingStatus {
  state: EmbeddingState;
  message: string;
  bytes: number;
  total: number;
  percent: number;
}

export interface Config {
  ai: AIConfig;
  knowledge: KnowledgeConfig;
}

export interface KnowledgeDocument {
  id: string;
  name: string;
  sourceFile: string;
  createdAt: string;
  status?: "pending" | "approved" | "rejected";
  verified?: boolean;
  confidence?: number;
  sourceType?: string;
  approvedBy?: string;
  approvedAt?: string;
}

export interface ProjectRepository {
  id: string;
  name?: string;
  repoURL: string;
  repoBranch?: string;
  repoCloned: boolean;
}

export interface RepoIndexStatus {
  state: "none" | "indexing" | "indexed" | "error";
  message?: string;
  totalFiles?: number;
  doneFiles?: number;
  percent?: number;
  indexedAt?: string;
}

export interface LocalProject {
  id: string;
  name: string;
  createdAt?: string;
  repositories?: ProjectRepository[];
}

export interface WikiChatChunk {
  id?: string;
  documentId?: string;
  content: string;
  score: number;
  sourceFile?: string;
}

export interface WikiChatTurn {
  question: string;
  answer: string;
}

export interface WikiSessionMeta {
  projectId: string;
  sectionKey: string;
  title: string;
}

export interface WikiChatRequest {
  projectId: string;
  sectionKey: string;
  question: string;
  history: WikiChatTurn[];
}

export interface WikiChatResponse {
  answer: string;
  chunks: WikiChatChunk[];
}

export interface KnowledgeUploadResult {
  file: string;
  ok: boolean;
  message?: string;
}

export interface KnowledgeUploadResponse {
  results: KnowledgeUploadResult[];
}

export interface AuthStatus {
  cliFound: boolean;
  cliPath: string;
}

export interface ToolEvent {
  kind:
    | "read"
    | "write"
    | "shell"
    | "url"
    | "mcp"
    | "custom-tool"
    | "hook"
    | "memory";
  path?: string;
  name?: string;
}

export type IssueSeverity = "high" | "medium" | "low";
export type IssueCategory =
  | "gap"
  | "conflict"
  | "ambiguity"
  | "suggestion"
  | "missing_flow"
  | "missing_edge_case"
  | "missing_constraint"
  | "inaccuracy"
  | "code_wiki_conflict";

export interface FileRef {
  path: string;
  url?: string;
}

export interface ClarifyIssue {
  id: string;
  category: IssueCategory;
  severity: IssueSeverity;
  title: string;
  description: string;
  suggestion: string;
  referenced_files?: FileRef[];
  wiki_sections?: string[];
}

export interface ClarifyResponse {
  summary: string;
  issues: ClarifyIssue[];
  sessionId?: string;
}

export interface FeatureManifest {
  id: string;
  name: string;
  version: string;
  description: string;
  icon: string;
  category: string;
  author: string;
  type: string;
  frontendRoute: string;
}
