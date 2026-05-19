import { createRouter, createWebHistory } from "vue-router";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: () => import("@/hub/HubHome.vue") },
    {
      path: "/features/spec-clarify",
      component: () => import("@/features/spec-clarify/EditorPage.vue"),
    },
    {
      path: "/features/spec-clarify/:pathMatch(.*)*",
      component: () => import("@/features/spec-clarify/EditorPage.vue"),
    },
  ],
});
