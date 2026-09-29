import { getToken } from "@/utils/token";

const guestOnlyPages = new Set(["/auth/login", "/auth/register"]);

export default defineNuxtRouteMiddleware(async (to) => {
  if (import.meta.server) return;

  const auth = useAuthStore();

  if (to.path.startsWith("/visit")) return;
  // Invitation links work whether or not the person is signed in.
  if (to.path === "/auth/invite") return;

  if (guestOnlyPages.has(to.path)) {
    if (getToken()) return navigateTo("/");
    return;
  }

  if (!getToken()) return navigateTo({ path: "/auth/login", query: { redirect: to.fullPath } });

  if (!auth.user) {
    if (!(await auth.fetchSession())) return navigateTo("/auth/login");
  }
});
