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
  token: string;
  model: string;
}

export interface Config {
  ai: AIConfig;
}

export interface AuthStatus {
  cliFound: boolean;
  cliPath: string;
}

export type IssueSeverity = "high" | "medium" | "low";
export type IssueCategory = "gap" | "conflict" | "ambiguity" | "suggestion";

export interface ClarifyIssue {
  id: string;
  category: IssueCategory;
  severity: IssueSeverity;
  title: string;
  description: string;
  suggestion: string;
}

export interface ClarifyQuestion {
  id: string;
  question: string;
  context: string;
  options: string[];
  defaultAnswer: string;
}

export interface ClarifyResponse {
  issues: ClarifyIssue[];
  questions: ClarifyQuestion[];
  summary: string;
}

export interface WikiFetchResponse {
  content: string;
  title: string;
}

export interface RefineResponse {
  refinedSpec: string;
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
