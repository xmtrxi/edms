import type { ApiResponse, DashboardOverview, Document, DocumentRoute } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const workspaceService = () => {
  const loading = shallowRef(false);
  const error = shallowRef("");

  const request = async <T>(
    path: string,
    options: Record<string, unknown> = {},
  ): Promise<T | null> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    try {
      const response = await $api<ApiResponse<T> | null>(path, options);
      error.value = "";
      return response?.data ?? null;
    } catch (reason) {
      error.value = apiErrorMessage(reason);
      return null;
    } finally {
      loading.value = false;
    }
  };

  const overview = async () => request<DashboardOverview>("/dashboard/overview");
  const favorites = async () => (await request<Document[]>("/favorites")) ?? [];
  const shared = async () => (await request<Document[]>("/shared")) ?? [];
  const trash = async () => (await request<Document[]>("/trash")) ?? [];
  const workflows = async () => (await request<DocumentRoute[]>("/workflows")) ?? [];
  const setFavorite = async (id: string, favorite: boolean) =>
    request<never>(`/documents/${id}/favorite`, { method: favorite ? "PUT" : "DELETE" });
  const moveToTrash = async (id: string) =>
    request<never>(`/documents/${id}/trash`, { method: "POST" });
  const restore = async (id: string) =>
    request<never>(`/documents/${id}/restore`, { method: "POST" });
  const settings = async () => (await request<Record<string, unknown>>("/settings")) ?? {};
  const saveSettings = async (values: Record<string, unknown>) =>
    request<Record<string, unknown>>("/settings", { method: "PUT", body: values });
  const storageStatus = async () =>
    request<{ provider: string; configured: boolean; healthy: boolean; error?: string }>(
      "/integrations/storage",
    );

  return {
    loading,
    error,
    overview,
    favorites,
    shared,
    trash,
    workflows,
    setFavorite,
    moveToTrash,
    restore,
    settings,
    saveSettings,
    storageStatus,
  };
};
