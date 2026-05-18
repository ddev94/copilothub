import { createRouter, createWebHistory } from "vue-router";
import EditorPage from "@/pages/EditorPage.vue";

export const router = createRouter({
  history: createWebHistory(),
  routes: [{ path: "/", component: EditorPage }],
});
