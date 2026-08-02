import type { ApiMeta, ApiResponse, Document, ListDocumentsParams, OCRResponse } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const documentsService = () => {
  const documents = ref<Document[]>([]);
  const meta = ref<ApiMeta | null>(null);
  const loading = ref(false);
  const error = ref("");
  let requestSeq = 0;

  const fetchDocuments = async (params: ListDocumentsParams = {}): Promise<void> => {
    const { $api } = useNuxtApp();
    const seq = ++requestSeq;
    loading.value = true;
    try {
      const res = await $api<ApiResponse<Document[]>>("/documents", { query: params });
      if (seq !== requestSeq) return;
      documents.value = res.data;
      meta.value = res.meta ?? null;
      error.value = "";
    } catch (err) {
      if (seq !== requestSeq) return;
      error.value = apiErrorMessage(err);
    } finally {
      if (seq === requestSeq) loading.value = false;
    }
  };

  const fetchDocument = async (id: string): Promise<Document | null> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<Document>>(`/documents/${id}`);
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  const fetchOCR = async (id: string): Promise<OCRResponse | null> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<OCRResponse>>(`/documents/${id}/ocr`);
      return res.data;
    } catch {
      return null;
    }
  };

  const redoOCR = async (id: string): Promise<boolean> => {
    const { $api } = useNuxtApp();
    try {
      await $api<ApiResponse<Document>>(`/documents/${id}/redo-ocr`, { method: "POST" });
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const download = async (id: string, filename = "document"): Promise<void> => {
    const { $api } = useNuxtApp();
    try {
      const blob = await $api<Blob>(`/documents/${id}/download`, { responseType: "blob" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = filename;
      link.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      error.value = apiErrorMessage(err);
    }
  };

  const archive = async (id: string): Promise<boolean> => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/documents/${id}`, { method: "DELETE" });
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const upload = async (file: File, name?: string): Promise<Document | null> => {
    const { $api } = useNuxtApp();
    const formData = new FormData();
    formData.append("file", file);
    if (name) formData.append("name", name);
    try {
      const res = await $api<ApiResponse<Document>>("/documents", {
        method: "POST",
        body: formData,
      });
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  return {
    documents,
    meta,
    loading,
    error,
    fetchDocuments,
    fetchDocument,
    fetchOCR,
    redoOCR,
    download,
    archive,
    upload,
  };
};
