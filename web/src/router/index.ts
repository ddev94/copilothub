import { createRouter, createWebHistory } from "vue-router";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: () => import("@/hub/HubHome.vue") },
    { path: "/settings", component: () => import("@/pages/SettingsPage.vue") },
    {
      path: "/projects/:projectId",
      component: () => import("@/hub/ProjectPage.vue"),
    },
    {
      path: "/projects/:projectId/settings",
      component: () => import("@/hub/ProjectSettingsPage.vue"),
    },
    {
      path: "/projects/:projectId/features/spec-clarify",
      component: () => import("@/features/spec-clarify/EditorPage.vue"),
    },
    {
      path: "/projects/:projectId/features/wiki",
      component: () => import("@/features/wiki/WikiPage.vue"),
    },
    {
      path: "/projects/:projectId/features/wiki/chat",
      component: () => import("@/features/wiki/WikiChatPage.vue"),
    },
  ],
});
