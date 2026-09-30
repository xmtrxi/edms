import type { ApiResponse, AutomationRule, AutomationRun } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export type RuleDraft = Omit<AutomationRule, "id" | "run_count" | "last_run_at" | "created_at">;

/** Automation rules, for administrators. */
export const automationService = () => {
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
    rules: async () => (await call<AutomationRule[]>("/admin/automations")).data ?? [],
    saveRule: async (draft: RuleDraft, ruleId?: string) =>
      (
        await call<AutomationRule>(
          ruleId ? `/admin/automations/${id(ruleId)}` : "/admin/automations",
          {
            method: ruleId ? "PATCH" : "POST",
            body: draft,
          },
        )
      ).data,
    deleteRule: async (ruleId: string) =>
      (await call(`/admin/automations/${id(ruleId)}`, { method: "DELETE" })).ok,
    runs: async (ruleId: string) =>
      (await call<AutomationRun[]>(`/admin/automations/${id(ruleId)}/runs`)).data ?? [],
  };
};
