export interface AcceptanceCriterion {
  id: string;
  description: string;
}

export interface TestCase {
  id: string;
  title: string;
  steps: string;
  expectedResult: string;
}

export interface UserStory {
  id: string;
  title: string;
  story: string;
  acceptanceCriteria: AcceptanceCriterion[];
  testCases: TestCase[];
}

export interface Spec {
  id: string;
  title: string;
  version: string;
  createdAt: string;
  updatedAt: string;
  requirement: string;
  userStories: UserStory[];
}

export interface SpecMeta {
  id: string;
  title: string;
  version: string;
  updatedAt: string;
}

export interface FileNode {
  name: string;
  path: string;
  isDir: boolean;
  children?: FileNode[];
}

export interface RepoInfo {
  name: string;
  path: string;
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

export interface ClarifyQuestion {
  id: string;
  question: string;
  suggestion: string;
}

export interface ClarifyResponse {
  clear: boolean;
  questions: ClarifyQuestion[];
}
