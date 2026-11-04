import type { ApiResponse, RetentionPolicy, RetentionPreview } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export type PolicyDraft = Omit<RetentionPolicy, "id" | "last_run_at" | "last_count" | "created_at">;

/** Retention policies, for administrators. */
export const retentionService = () => {
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
  const id = (value: string) => encodeURIComponent(value);

  return {
    error,
    busy,
    policies: async () => (await call<RetentionPolicy[]>("/admin/retention")).data ?? [],
    savePolicy: async (draft: PolicyDraft, policyId?: string) =>
      (
        await call<RetentionPolicy>(
          policyId ? `/admin/retention/${id(policyId)}` : "/admin/retention",
          {
            method: policyId ? "PATCH" : "POST",
            body: draft,
          },
        )
      ).data,
    deletePolicy: async (policyId: string) =>
      (await call(`/admin/retention/${id(policyId)}`, { method: "DELETE" })).ok,
    preview: async (policyId: string) =>
      (await call<RetentionPreview>(`/admin/retention/${id(policyId)}/preview`)).data,
    runPolicy: async (policyId: string) =>
      (await call<{ applied: number }>(`/admin/retention/${id(policyId)}/run`, { method: "POST" }))
        .data,
  };
};
