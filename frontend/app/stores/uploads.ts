import type { ApiResponse, Document, UploadAccess } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

/** Bytes per request. Small enough to retry cheaply on a flaky connection. */
const CHUNK_SIZE = 8 * 1024 * 1024;
const MAX_ATTEMPTS = 5;

export interface UploadTarget {
  folderId?: string;
  categoryId?: string;
  access?: UploadAccess;
}

export interface UploadItem {
  key: string;
  file: File;
  target: UploadTarget;
  sessionId: string;
  sent: number;
  status: "queued" | "uploading" | "done" | "duplicate" | "failed" | "cancelled";
  error?: string;
  document?: Document;
}

interface Progress {
  id: string;
  received: number;
  size: number;
}

const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Resumable uploads shared by every upload entry point. Files go up in
 * chunks; after a dropped connection the upload continues from the last
 * byte the server acknowledged instead of starting over.
 */
export const useUploadStore = defineStore("uploads", () => {
  const items = ref<UploadItem[]>([]);
  const listeners = new Set<(document: Document) => void>();
  let running = false;

  const active = computed(() =>
    items.value.filter((item) => item.status === "queued" || item.status === "uploading"),
  );

  /** Called with each finished document, so open views can refresh. */
  const onUploaded = (listener: (document: Document) => void) => {
    listeners.add(listener);
    return () => listeners.delete(listener);
  };

  const add = (files: FileList | File[], target: UploadTarget = {}) => {
    for (const file of Array.from(files)) {
      items.value.push({
        key: `${Date.now()}-${Math.random().toString(36).slice(2)}-${file.name}`,
        file,
        target,
        sessionId: "",
        sent: 0,
        status: "queued",
      });
    }
    void run();
  };

  const statusOf = (err: unknown) =>
    (err as { response?: { status?: number; _data?: { received?: number } } }).response;

  const sendChunk = async (item: UploadItem) => {
    const { $api } = useNuxtApp();
    const end = Math.min(item.sent + CHUNK_SIZE, item.file.size);
    const body = item.file.slice(item.sent, end);
    const res = await $api<ApiResponse<Progress>>(`/uploads/${item.sessionId}`, {
      method: "PUT",
      query: { offset: item.sent },
      body,
      headers: { "Content-Type": "application/octet-stream" },
    });
    item.sent = res.data.received;
  };

  const upload = async (item: UploadItem) => {
    const { $api } = useNuxtApp();
    item.status = "uploading";
    if (!item.sessionId) {
      const res = await $api<ApiResponse<Progress>>("/uploads", {
        method: "POST",
        body: {
          name: item.file.name,
          mime_type: item.file.type,
          size: item.file.size,
          folder_id: item.target.folderId,
          category_id: item.target.categoryId,
          access: item.target.access,
        },
      });
      item.sessionId = res.data.id;
    }
    let attempts = 0;
    while (item.sent < item.file.size) {
      // cancel() can change the status while this loop awaits.
      if ((item.status as UploadItem["status"]) === "cancelled") return;
      try {
        await sendChunk(item);
        attempts = 0;
      } catch (err) {
        const response = statusOf(err);
        // The server holds a different offset: continue from there.
        if (response?.status === 409 && typeof response._data?.received === "number") {
          item.sent = response._data.received;
          continue;
        }
        if (++attempts >= MAX_ATTEMPTS || (response?.status && response.status < 500)) throw err;
        await wait(1000 * 2 ** attempts);
      }
    }
    const done = await $api<ApiResponse<Document> & { message?: string }>(
      `/uploads/${item.sessionId}/complete`,
      { method: "POST" },
    );
    item.document = done.data;
    item.status = done.message?.startsWith("duplicate") ? "duplicate" : "done";
    for (const listener of listeners) listener(done.data);
  };

  const run = async () => {
    if (running) return;
    running = true;
    try {
      let next: UploadItem | undefined;
      while ((next = items.value.find((item) => item.status === "queued"))) {
        try {
          await upload(next);
        } catch (err) {
          if (next.status !== "cancelled") {
            next.status = "failed";
            next.error = apiErrorMessage(err);
          }
        }
      }
    } finally {
      running = false;
    }
  };

  const retry = (key: string) => {
    const item = items.value.find((entry) => entry.key === key);
    if (!item) return;
    item.status = "queued";
    item.error = undefined;
    void run();
  };

  const cancel = async (key: string) => {
    const item = items.value.find((entry) => entry.key === key);
    if (!item) return;
    item.status = "cancelled";
    if (item.sessionId) {
      const { $api } = useNuxtApp();
      await $api(`/uploads/${item.sessionId}`, { method: "DELETE" }).catch(() => undefined);
    }
  };

  const clearFinished = () => {
    items.value = items.value.filter(
      (item) => item.status === "queued" || item.status === "uploading",
    );
  };

  return { items, active, add, retry, cancel, clearFinished, onUploaded };
});
