import type { AIJob, ApiResponse } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const aiService = () => {
  const loading = shallowRef(false);
  const error = shallowRef("");
  const run = async (
    kind: "classify" | "ask" | "summarize" | "extract",
    input: { question?: string; text?: string; document_ids?: string[] },
  ): Promise<AIJob | null> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    try {
      const response = await $api<ApiResponse<AIJob>>(`/ai/${kind}`, {
        method: "POST",
        body: input,
      });
      error.value = "";
      return response.data;
    } catch (reason) {
      error.value = apiErrorMessage(reason);
      return null;
    } finally {
      loading.value = false;
    }
  };
  const job = async (id: string): Promise<AIJob | null> => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<AIJob>>(`/ai/jobs/${id}`);
      return response.data;
    } catch (reason) {
      error.value = apiErrorMessage(reason);
      return null;
    }
  };
  return { loading, error, run, job };
};
