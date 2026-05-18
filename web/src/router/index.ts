import { createRouter, createWebHistory } from "vue-router";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: () => import("@/hub/HubHome.vue") },
    {
      path: "/features/spec-designer",
      component: () => import("@/features/spec-designer/EditorPage.vue"),
    },
    {
      path: "/features/spec-designer/:pathMatch(.*)*",
      component: () => import("@/features/spec-designer/EditorPage.vue"),
    },
  ],
});
