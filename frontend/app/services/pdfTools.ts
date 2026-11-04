import type { ApiResponse, PdfJob, PdfOperation, PdfOptions } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const pdfToolsService = () => {
  const error = shallowRef("");
  const busy = shallowRef(false);

  const call = async <T>(path: string, options: Record<string, unknown> = {}) => {
    const { $api } = useNuxtApp();
    busy.value = true;
    try {
      const response = await $api<ApiResponse<T> | undefined>(path, options);
      error.value = "";
      return response?.data ?? null;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    } finally {
      busy.value = false;
    }
  };

  /**
   * Queues a tool run. The key makes a double click or a retried request
   * return the first job instead of starting another.
   */
  const run = (
    operation: PdfOperation,
    documentIds: string[],
    options: PdfOptions = {},
    name = "",
  ) =>
    call<PdfJob>("/pdf-tools/jobs", {
      method: "POST",
      body: {
        operation,
        document_ids: documentIds,
        options,
        name,
        idempotency_key: crypto.randomUUID(),
      },
    });

  const jobs = (documentId = "") =>
    call<PdfJob[]>(
      `/pdf-tools/jobs${documentId ? `?document_id=${encodeURIComponent(documentId)}` : ""}`,
    );

  const retry = (id: string) =>
    call<PdfJob>(`/pdf-tools/jobs/${encodeURIComponent(id)}/retry`, { method: "POST" });

  return { error, busy, run, jobs, retry };
};

export const pdfOperationLabel: Record<PdfOperation, string> = {
  merge: "Merge",
  compress: "Compress",
  rotate: "Rotate",
  watermark: "Watermark",
  page_numbers: "Page numbers",
};
