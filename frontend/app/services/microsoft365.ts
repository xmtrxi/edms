import type { ApiResponse } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export interface M365Status {
  open: boolean;
  web_url?: string;
  base_version_id?: string;
  opened_at?: string;
}

export const microsoft365Service = () => {
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

  const base = (id: string) => `/documents/${encodeURIComponent(id)}/microsoft365`;
  const status = async (id: string) => (await call<M365Status>(base(id))).data;
  const open = async (id: string) =>
    (await call<M365Status>(`${base(id)}/open`, { method: "POST" })).data;
  /** Saves the working copy as a version; "changed" is false when nothing was edited. */
  const checkIn = async (id: string, done: boolean) =>
    (await call<{ changed: boolean }>(`${base(id)}/checkin`, { method: "POST", body: { done } }))
      .data;
  const discard = async (id: string) => (await call(base(id), { method: "DELETE" })).ok;

  return { error, busy, status, open, checkIn, discard };
};
