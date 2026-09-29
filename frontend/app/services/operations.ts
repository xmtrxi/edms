import type { ApiResponse } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export interface AuditEvent {
  id: string;
  actor_id: string;
  action: string;
  resource: string;
  resource_id: string;
  metadata: Record<string, unknown>;
  created_at: string;
}
export const operationsService = () => {
  const loading = shallowRef(false);
  const error = shallowRef("");
  const audit = async (filters: { action?: string; resource?: string; since?: string } = {}) => {
    const { $api } = useNuxtApp();
    loading.value = true;
    try {
      const response = await $api<ApiResponse<AuditEvent[]>>("/admin/audit-events", {
        query: filters,
      });
      error.value = "";
      return response.data;
    } catch (reason) {
      error.value = apiErrorMessage(reason);
      return [];
    } finally {
      loading.value = false;
    }
  };
  const exportAudit = async () => {
    const { $api } = useNuxtApp();
    try {
      const blob = await $api<Blob>("/admin/audit-events/export", { responseType: "blob" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = "audit-events.json";
      link.click();
      URL.revokeObjectURL(url);
    } catch (reason) {
      error.value = apiErrorMessage(reason);
    }
  };
  return { loading, error, audit, exportAudit };
};
