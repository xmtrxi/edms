import type { ApiResponse, DocumentComment } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const commentsService = () => {
  const error = shallowRef("");
  const busy = shallowRef(false);

  const call = async <T>(path: string, options: Record<string, unknown> = {}) => {
    const { $api } = useNuxtApp();
    busy.value = true;
    try {
      const response = await $api<ApiResponse<T> | undefined>(path, options);
      error.value = "";
      return { ok: true, data: response?.data ?? null };
    } catch (err) {
      error.value = apiErrorMessage(err);
      return { ok: false, data: null };
    } finally {
      busy.value = false;
    }
  };

  const list = async (documentId: string) =>
    (await call<DocumentComment[]>(`/documents/${documentId}/comments`)).data;
  const create = async (
    documentId: string,
    input: { body: string; parent_id?: string; page?: number; quote?: string; mentions?: string[] },
  ) =>
    (
      await call<DocumentComment>(`/documents/${documentId}/comments`, {
        method: "POST",
        body: input,
      })
    ).data;
  const edit = async (id: string, body: string) =>
    (
      await call<DocumentComment>(`/comments/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: { body },
      })
    ).data;
  const resolve = async (id: string, resolved: boolean) =>
    (
      await call<DocumentComment>(`/comments/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: { resolved },
      })
    ).data;
  const remove = async (id: string) =>
    (await call(`/comments/${encodeURIComponent(id)}`, { method: "DELETE" })).ok;

  return { error, busy, list, create, edit, resolve, remove };
};
