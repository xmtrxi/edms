import type { ApiMeta, ApiResponse, ListDocumentsParams, SearchResultDocument } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const searchService = () => {
  const results = ref<SearchResultDocument[]>([]);
  const meta = ref<ApiMeta | null>(null);
  const loading = ref(false);
  const error = ref("");
  let requestSeq = 0;

  const search = async (q: string, params: ListDocumentsParams = {}): Promise<void> => {
    const { $api } = useNuxtApp();
    const seq = ++requestSeq;
    loading.value = true;
    try {
      const res = await $api<ApiResponse<SearchResultDocument[]>>("/search", {
        query: { q, ...params },
      });
      if (seq !== requestSeq) return;
      results.value = res.data;
      meta.value = res.meta ?? null;
      error.value = "";
    } catch (err) {
      if (seq !== requestSeq) return;
      error.value = apiErrorMessage(err);
    } finally {
      if (seq === requestSeq) loading.value = false;
    }
  };

  const reindex = async (): Promise<boolean> => {
    const { $api } = useNuxtApp();
    try {
      await $api<ApiResponse<{ status: string }>>("/search/reindex", { method: "POST" });
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const passages = async (q: string, documentId: string): Promise<SearchResultDocument[]> => {
    const { $api } = useNuxtApp();
    const res = await $api<ApiResponse<SearchResultDocument[]>>("/search", {
      query: { q, mode: "passages", document_id: documentId, page: 1, page_size: 50 },
    });
    return res.data;
  };

  return { results, meta, loading, error, search, passages, reindex };
};
