/** The product name, as the API reports it from APP_NAME. */
export const useBrand = () => {
  const auth = useAuthStore();
  const config = useRuntimeConfig();
  if (import.meta.client && !auth.meta) void auth.fetchMeta();
  return computed(() => auth.meta?.app_name || String(config.public.appName));
};
