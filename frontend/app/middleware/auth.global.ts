import { getToken } from "@/utils/token";

const guestOnlyPages = new Set(["/auth/login", "/auth/register", "/auth/forgot-password"]);
// Reached from emailed links or the identity provider, signed in or not.
const openAuthPages = new Set([
  "/auth/invite",
  "/auth/reset-password",
  "/auth/verify-email",
  "/auth/sso",
]);

export default defineNuxtRouteMiddleware(async (to) => {
  if (import.meta.server) return;

  const auth = useAuthStore();

  if (to.path.startsWith("/visit")) return;
  // Invitation and share links work whether or not the person is signed in.
  if (
    openAuthPages.has(to.path) ||
    to.path.startsWith("/s/") ||
    to.path.startsWith("/r/") ||
    to.path.startsWith("/sign/")
  )
    return;

  if (guestOnlyPages.has(to.path)) {
    if (getToken()) return navigateTo("/");
    return;
  }

  if (!getToken()) return navigateTo({ path: "/auth/login", query: { redirect: to.fullPath } });

  if (!auth.user) {
    if (!(await auth.fetchSession())) return navigateTo("/auth/login");
  }
});
