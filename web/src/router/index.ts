import { createRouter, createWebHistory } from "vue-router";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: () => import("@/hub/HubHome.vue") },
    { path: "/settings", component: () => import("@/pages/SettingsPage.vue") },
    {
      path: "/features/spec-clarify",
      component: () => import("@/features/spec-clarify/EditorPage.vue"),
    },
    {
      path: "/features/spec-clarify/:pathMatch(.*)*",
      component: () => import("@/features/spec-clarify/EditorPage.vue"),
    },
    {
      path: "/features/spec-designer",
      component: () => import("@/features/spec-clarify/EditorPage.vue"),
    },
    {
      path: "/features/spec-designer/:pathMatch(.*)*",
      component: () => import("@/features/spec-clarify/EditorPage.vue"),
    },
    {
      path: "/features/wiki",
      component: () => import("@/features/wiki/WikiPage.vue"),
    },
    {
      path: "/features/wiki/:pathMatch(.*)*",
      component: () => import("@/features/wiki/WikiPage.vue"),
    },
  ],
});
