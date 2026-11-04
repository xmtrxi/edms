import type { AIAnswer, AIJob, ApiResponse } from "@/types/api";
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
  /** Answers from documents the user can open, with citations. */
  const answer = async (input: {
    question: string;
    folder_id?: string;
    document_ids?: string[];
  }): Promise<{ data: AIAnswer | null; status: number }> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    try {
      const response = await $api<ApiResponse<AIAnswer>>("/ai/answer", {
        method: "POST",
        body: input,
      });
      error.value = "";
      return { data: response.data, status: 200 };
    } catch (reason) {
      error.value = apiErrorMessage(reason);
      const status = (reason as { response?: { status?: number } }).response?.status ?? 0;
      return { data: null, status };
    } finally {
      loading.value = false;
    }
  };
  const label = async (documentId: string) => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/ai/documents/${documentId}/label`, { method: "POST" });
      error.value = "";
      return true;
    } catch (reason) {
      error.value = apiErrorMessage(reason);
      return false;
    }
  };
  return { loading, error, run, job, answer, label };
};
