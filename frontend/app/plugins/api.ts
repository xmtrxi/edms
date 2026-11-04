import { clearToken, getToken } from "@/utils/token";

export default defineNuxtPlugin(() => {
  const { apiBase } = useRuntimeConfig().public;

  const api = $fetch.create({
    baseURL: apiBase,
    onRequest({ options }) {
      const token = getToken();
      if (token) options.headers.set("Authorization", `Bearer ${token}`);
    },
    onResponseError({ response }) {
      if (response.status !== 401) return;
      if (typeof window === "undefined") return;
      if (window.location.pathname.startsWith("/auth/")) return;
      if (window.location.pathname.startsWith("/visit")) return;
      clearToken();
      navigateTo("/auth/login");
    },
  });

  return {
    provide: {
      api,
    },
  };
});
