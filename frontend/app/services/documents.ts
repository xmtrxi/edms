import type {
  ApiMeta,
  ApiResponse,
  CreateDocumentRoute,
  Document,
  DocumentFolder,
  DocumentContributors,
  DocumentHistoryEvent,
  DocumentRoute,
  DocumentVersion,
  ListDocumentsParams,
  OCRResponse,
  ProcessingJob,
  RouteDecision,
  StorageSyncResult,
  UploadAccess,
} from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const documentsService = () => {
  const documents = ref<Document[]>([]);
  const meta = ref<ApiMeta | null>(null);
  const loading = ref(false);
  const error = ref("");
  /** HTTP status of the last failed fetchDocument, e.g. 403 when access can be requested. */
  const lastStatus = ref(0);
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
      lastStatus.value = 0;
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      lastStatus.value = (err as { response?: { status?: number } }).response?.status ?? 0;
      return null;
    }
  };

  const fetchFolders = async (): Promise<DocumentFolder[]> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<DocumentFolder[]>>("/documents/folders");
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return [];
    }
  };

  const fetchVersions = async (id: string): Promise<DocumentVersion[]> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<DocumentVersion[]>>(`/documents/${id}/versions`);
      error.value = "";
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return [];
    }
  };

  const fetchContributors = async (id: string): Promise<DocumentContributors | null> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<DocumentContributors>>(`/documents/${id}/contributors`);
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  const fetchHistory = async (id: string): Promise<DocumentHistoryEvent[]> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<DocumentHistoryEvent[]>>(`/documents/${id}/history`);
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return [];
    }
  };

  const downloadVersion = async (documentId: string, version: DocumentVersion): Promise<void> => {
    const { $api } = useNuxtApp();
    try {
      const blob = await $api<Blob>(`/documents/${documentId}/versions/${version.id}/download`, {
        responseType: "blob",
      });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = version.file_name;
      link.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      error.value = apiErrorMessage(err);
    }
  };

  const restoreVersion = async (
    documentId: string,
    versionId: string,
    changeSummary = "",
  ): Promise<DocumentVersion | null> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<DocumentVersion>>(
        `/documents/${documentId}/versions/${versionId}/restore`,
        {
          method: "POST",
          body: { change_summary: changeSummary },
        },
      );
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  const fetchRoutes = async (id: string): Promise<DocumentRoute[]> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<DocumentRoute[]>>(`/documents/${id}/routes`);
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return [];
    }
  };

  const createRoute = async (
    documentId: string,
    input: CreateDocumentRoute,
  ): Promise<DocumentRoute | null> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<DocumentRoute>>(`/documents/${documentId}/routes`, {
        method: "POST",
        body: input,
      });
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  const decideRoute = async (
    documentId: string,
    routeId: string,
    input: RouteDecision,
  ): Promise<DocumentRoute | null> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<DocumentRoute>>(
        `/documents/${documentId}/routes/${routeId}/decisions`,
        { method: "POST", body: input },
      );
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  const fetchOCR = async (id: string, full = false): Promise<OCRResponse | null> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<OCRResponse>>(`/documents/${id}/ocr`, {
        query: full ? { full: "true" } : undefined,
      });
      return res.data;
    } catch {
      return null;
    }
  };

  const fetchProcessingJob = async (id: string): Promise<ProcessingJob | null> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<ProcessingJob>>(`/documents/${id}/processing-job`);
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

  const upload = async (
    file: File,
    name?: string,
    categoryId?: string,
    options: {
      folderId?: string;
      folderPath?: string;
      folderColor?: string;
      folderCategoryIds?: string[];
      access?: UploadAccess;
    } = {},
  ): Promise<Document | null> => {
    const { $api } = useNuxtApp();
    const formData = new FormData();
    formData.append("file", file);
    if (name) formData.append("name", name);
    if (categoryId) formData.append("category_id", categoryId);
    if (options.folderId) formData.append("folder_id", options.folderId);
    if (options.folderPath) formData.append("folder_path", options.folderPath);
    if (options.folderColor) formData.append("folder_color", options.folderColor);
    if (options.folderCategoryIds?.length) {
      formData.append("folder_category_ids", JSON.stringify(options.folderCategoryIds));
    }
    if (options.access && options.access.mode !== "inherit") {
      formData.append("access", JSON.stringify(options.access));
    } else if (options.access?.sensitivity && options.access.sensitivity !== "internal") {
      formData.append("access", JSON.stringify(options.access));
    }
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

  const syncStorage = async (
    options: { workers?: number; dryRun?: boolean } = {},
  ): Promise<StorageSyncResult | null> => {
    const { $api } = useNuxtApp();
    const { workers = 4, dryRun = false } = options;
    try {
      const res = await $api<ApiResponse<StorageSyncResult>>("/documents/sync", {
        method: "POST",
        query: { workers, dry_run: dryRun },
      });
      error.value = "";
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  // Passing an empty categoryId clears the document's category.
  /** Renames or moves a document; folder_id "root" moves it to My files. */
  const update = async (
    id: string,
    input: { name?: string; folder_id?: string },
  ): Promise<Document | null> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<Document>>(`/documents/${id}`, {
        method: "PATCH",
        body: input,
      });
      error.value = "";
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  const setCategory = async (id: string, categoryId: string): Promise<Document | null> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<Document>>(`/documents/${id}/category`, {
        method: "PUT",
        body: { category_id: categoryId },
      });
      documents.value = documents.value.map((doc) => (doc.id === id ? res.data : doc));
      error.value = "";
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };
  const requestSignature = async (
    id: string,
    input: { title?: string; recipients: Array<{ email: string; name?: string; role?: string }> },
  ) => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<Record<string, unknown>>>(`/documents/${id}/sign`, {
        method: "POST",
        body: input,
      });
      error.value = "";
      return response.data;
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
    lastStatus,
    fetchDocuments,
    fetchDocument,
    fetchFolders,
    fetchVersions,
    fetchContributors,
    fetchHistory,
    downloadVersion,
    restoreVersion,
    fetchRoutes,
    createRoute,
    decideRoute,
    fetchOCR,
    fetchProcessingJob,
    redoOCR,
    download,
    archive,
    upload,
    syncStorage,
    setCategory,
    update,
    requestSignature,
  };
};
