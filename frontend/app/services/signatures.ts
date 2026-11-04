import type { ApiResponse, EnvelopeDraft, SignatureEnvelope, SigningSession } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

/** Built-in signing: sending and tracking requests, and signing by link. */
export const signaturesService = () => {
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

  const send = async (documentId: string, draft: EnvelopeDraft) =>
    (
      await call<SignatureEnvelope>(`/documents/${documentId}/signatures`, {
        method: "POST",
        body: draft,
      })
    ).data;
  const forDocument = async (documentId: string) =>
    (await call<SignatureEnvelope[]>(`/documents/${documentId}/signatures`)).data ?? [];
  const list = async (box: "inbox" | "sent") =>
    (await call<SignatureEnvelope[]>(`/signatures?box=${box}`)).data ?? [];
  const get = async (id: string) =>
    (await call<SignatureEnvelope>(`/signatures/${encodeURIComponent(id)}`)).data;
  const remind = async (id: string) =>
    (
      await call<{ reminded: number }>(`/signatures/${encodeURIComponent(id)}/remind`, {
        method: "POST",
      })
    ).data;
  const voidRequest = async (id: string, reason: string) =>
    (await call(`/signatures/${encodeURIComponent(id)}/void`, { method: "POST", body: { reason } }))
      .ok;

  // Public signing, by the recipient's personal link.
  const base = (token: string) => `/public/sign/${encodeURIComponent(token)}`;
  const open = async (token: string) => (await call<SigningSession>(base(token))).data;
  const sendCode = async (token: string) =>
    (await call(`${base(token)}/code`, { method: "POST" })).ok;
  const verifyCode = async (token: string, code: string) =>
    (await call<SigningSession>(`${base(token)}/verify`, { method: "POST", body: { code } })).data;
  const complete = async (token: string, values: Record<string, string>) =>
    (
      await call<SigningSession>(`${base(token)}/complete`, {
        method: "POST",
        body: {
          values,
          consent: true,
          time_zone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        },
      })
    ).data;
  const decline = async (token: string, reason: string) =>
    (await call(`${base(token)}/decline`, { method: "POST", body: { reason } })).ok;
  const documentBlob = async (token: string) => {
    const { $api } = useNuxtApp();
    try {
      return await $api<Blob>(`${base(token)}/document`, { responseType: "blob" });
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };
  const downloadUrl = (token: string) => {
    const config = useRuntimeConfig();
    return `${String(config.public.apiBase).replace(/\/$/, "")}${base(token)}/document?download=1`;
  };

  return {
    error,
    busy,
    send,
    forDocument,
    list,
    get,
    remind,
    voidRequest,
    open,
    sendCode,
    verifyCode,
    complete,
    decline,
    documentBlob,
    downloadUrl,
  };
};
