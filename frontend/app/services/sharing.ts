import type {
  AccessRequest,
  ApiResponse,
  DocumentAccess,
  FolderGrantInput,
  LinkDocument,
  PrincipalOption,
  Sensitivity,
  SharePolicy,
  ShareResult,
  ShareRole,
  ShareView,
} from "@/types/api";
import { apiErrorMessage } from "@/utils/api";
import { getToken } from "@/utils/token";

/** Who can reach a document: grants, people, links and access requests. */
export const sharingService = () => {
  const error = shallowRef("");

  const call = async <T>(
    path: string,
    options: Record<string, unknown> = {},
  ): Promise<T | null> => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<T>>(path, options);
      error.value = "";
      return response?.data ?? (null as T | null);
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  const searchPrincipals = async (query: string) =>
    (await call<PrincipalOption[]>("/principals", { query: { q: query } })) ?? [];

  const access = (documentId: string) => call<DocumentAccess>(`/documents/${documentId}/access`);

  const updateSettings = (
    documentId: string,
    settings: { inherit_permissions?: boolean; sensitivity?: Sensitivity },
  ) => call<DocumentAccess>(`/documents/${documentId}/access`, { method: "PATCH", body: settings });

  const grant = (documentId: string, input: FolderGrantInput) =>
    call(`/documents/${documentId}/grants`, { method: "PUT", body: input });

  const removeGrant = async (documentId: string, grantId: string) => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/documents/${documentId}/grants/${encodeURIComponent(grantId)}`, {
        method: "DELETE",
      });
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const shareWithPerson = (
    documentId: string,
    input: { email: string; role: ShareRole; message?: string; notify?: boolean },
  ) => call<ShareResult>(`/documents/${documentId}/shares`, { method: "POST", body: input });

  const updateShareRole = async (documentId: string, shareId: string, role: ShareRole) => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/documents/${documentId}/shares/${encodeURIComponent(shareId)}`, {
        method: "PATCH",
        body: { role },
      });
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const revokeShare = async (documentId: string, shareId: string) => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/documents/${documentId}/shares/${encodeURIComponent(shareId)}`, {
        method: "DELETE",
      });
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const createLink = (
    documentId: string,
    input: {
      scope: "organization" | "anyone";
      role?: ShareRole;
      allow_download?: boolean;
      password?: string;
      expires_at?: string;
    },
  ) =>
    call<{ link: ShareView; url: string }>(`/documents/${documentId}/links`, {
      method: "POST",
      body: input,
    });

  const requestAccess = (documentId: string, input: { role: ShareRole; message: string }) =>
    call<AccessRequest>(`/documents/${documentId}/access-requests`, {
      method: "POST",
      body: input,
    });

  const accessRequests = async () => (await call<AccessRequest[]>("/access-requests")) ?? [];

  const decide = (requestId: string, approve: boolean, role?: ShareRole) =>
    call<AccessRequest>(
      `/access-requests/${encodeURIComponent(requestId)}/${approve ? "approve" : "deny"}`,
      { method: "POST", body: approve && role ? { role } : {} },
    );

  const policy = () => call<SharePolicy>("/org/sharing-policy");

  const updatePolicy = (input: SharePolicy) =>
    call<SharePolicy>("/org/sharing-policy", { method: "PUT", body: input });

  /** Opens a share link. Organization links send the session token when present. */
  const openLink = async (
    token: string,
    password: string,
  ): Promise<{ data: LinkDocument | null; status: number }> => {
    const { apiBase } = useRuntimeConfig().public;
    const headers: Record<string, string> = {};
    if (password) headers["X-Link-Password"] = password;
    const session = getToken();
    if (session) headers.Authorization = `Bearer ${session}`;
    try {
      const data = await $fetch<ApiResponse<LinkDocument>>(`/links/${token}`, {
        baseURL: apiBase,
        headers,
      });
      error.value = "";
      return { data: data.data, status: 200 };
    } catch (err) {
      error.value = apiErrorMessage(err);
      const status = (err as { response?: { status?: number } }).response?.status ?? 0;
      return { data: null, status };
    }
  };

  /** Fetches a link's file as a blob, for the viewer or a download. */
  const linkFile = async (token: string, password: string, inline: boolean) => {
    const { apiBase } = useRuntimeConfig().public;
    const headers: Record<string, string> = {};
    if (password) headers["X-Link-Password"] = password;
    const session = getToken();
    if (session) headers.Authorization = `Bearer ${session}`;
    try {
      return await $fetch<Blob>(`/links/${token}/download`, {
        baseURL: apiBase,
        headers,
        query: inline ? { inline: "1" } : undefined,
        responseType: "blob",
      });
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  return {
    error,
    searchPrincipals,
    access,
    updateSettings,
    grant,
    removeGrant,
    shareWithPerson,
    updateShareRole,
    revokeShare,
    createLink,
    requestAccess,
    accessRequests,
    decide,
    policy,
    updatePolicy,
    openLink,
    linkFile,
  };
};
