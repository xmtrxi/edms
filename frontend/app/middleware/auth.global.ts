import { getToken } from "@/utils/token";

export default defineNuxtRouteMiddleware((to) => {
  if (import.meta.server) return;

  const auth = useAuthStore();
  const isAuthPage = to.path === "/auth/login" || to.path === "/auth/register";

  if (isAuthPage) {
    if (getToken()) return navigateTo("/");
    return;
  }

  if (!getToken()) return navigateTo("/auth/login");

  if (!auth.user) {
    return auth.fetchMe().then((ok) => {
      if (!ok) return navigateTo("/auth/login");
    });
  }
});
