import type {
  ApiResponse,
  FormSubmission,
  VisitorPortal,
  VisitorRequestCreated,
  VisitorRequest,
} from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

const VISITOR_TOKEN_KEY = "edms.visitor.session";

export const visitorsService = () => {
  const { $api } = useNuxtApp();
  const loading = shallowRef(false);
  const error = shallowRef("");

  const createRequest = async (input: {
    template_id: string;
    visitor_name: string;
    visitor_email: string;
    expires_at?: string;
  }): Promise<VisitorRequestCreated | null> => {
    loading.value = true;
    try {
      const response = await $api<ApiResponse<VisitorRequestCreated>>("/visitor-requests", {
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
  const requests = async (): Promise<VisitorRequest[]> => {
    try {
      const response = await $api<ApiResponse<VisitorRequest[]>>("/visitor-requests");
      return response.data;
    } catch (reason) {
      error.value = apiErrorMessage(reason);
      return [];
    }
  };
  const revoke = async (id: string): Promise<boolean> => {
    try {
      await $api(`/visitor-requests/${id}/revoke`, { method: "POST" });
      return true;
    } catch (reason) {
      error.value = apiErrorMessage(reason);
      return false;
    }
  };
  const resend = async (id: string): Promise<VisitorRequestCreated | null> => {
    try {
      const response = await $api<ApiResponse<VisitorRequestCreated>>(
        `/visitor-requests/${id}/resend`,
        { method: "POST" },
      );
      return response.data;
    } catch (reason) {
      error.value = apiErrorMessage(reason);
      return null;
    }
  };
  const attachments = async () => {
    const token = localStorage.getItem(VISITOR_TOKEN_KEY);
    if (!token) return [];
    try {
      const response = await $api<ApiResponse<Array<{ id: string; name: string; size: number }>>>(
        "/visitor/attachments",
        { headers: { "X-Visitor-Token": token } },
      );
      return response.data;
    } catch (reason) {
      error.value = apiErrorMessage(reason);
      return [];
    }
  };
  const uploadAttachment = async (file: File) => {
    const token = localStorage.getItem(VISITOR_TOKEN_KEY);
    if (!token) return null;
    const body = new FormData();
    body.append("file", file);
    try {
      const response = await $api<ApiResponse<{ id: string; name: string; size: number }>>(
        "/visitor/attachments",
        { method: "POST", headers: { "X-Visitor-Token": token }, body },
      );
      return response.data;
    } catch (reason) {
      error.value = apiErrorMessage(reason);
      return null;
    }
  };

  const requestOTP = async (email: string, accessCode: string): Promise<string> => {
    loading.value = true;
    try {
      const response = await $api<ApiResponse<{ development_otp?: string }>>(
        "/visitor/access/request",
        { method: "POST", body: { email, access_code: accessCode } },
      );
      error.value = "";
      return response.data.development_otp ?? "";
    } catch (reason) {
      error.value = apiErrorMessage(reason);
      return "";
    } finally {
      loading.value = false;
    }
  };

  const verify = async (email: string, accessCode: string, otp: string): Promise<boolean> => {
    loading.value = true;
    try {
      const response = await $api<ApiResponse<{ token: string }>>("/visitor/access/verify", {
        method: "POST",
        body: { email, access_code: accessCode, otp },
      });
      localStorage.setItem(VISITOR_TOKEN_KEY, response.data.token);
      error.value = "";
      return true;
    } catch (reason) {
      error.value = apiErrorMessage(reason);
      return false;
    } finally {
      loading.value = false;
    }
  };

  const portal = async (): Promise<VisitorPortal | null> => {
    const token = localStorage.getItem(VISITOR_TOKEN_KEY);
    if (!token) return null;
    loading.value = true;
    try {
      const response = await $api<ApiResponse<VisitorPortal>>("/visitor/portal", {
        headers: { "X-Visitor-Token": token },
      });
      error.value = "";
      return response.data;
    } catch (reason) {
      localStorage.removeItem(VISITOR_TOKEN_KEY);
      error.value = apiErrorMessage(reason);
      return null;
    } finally {
      loading.value = false;
    }
  };

  const submit = async (values: Record<string, unknown>): Promise<FormSubmission | null> => {
    const token = localStorage.getItem(VISITOR_TOKEN_KEY);
    if (!token) return null;
    loading.value = true;
    try {
      const response = await $api<ApiResponse<FormSubmission>>("/visitor/submit", {
        method: "POST",
        headers: { "X-Visitor-Token": token },
        body: { values },
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

  const clearSession = () => localStorage.removeItem(VISITOR_TOKEN_KEY);

  return {
    loading,
    error,
    createRequest,
    requests,
    revoke,
    resend,
    requestOTP,
    verify,
    portal,
    submit,
    attachments,
    uploadAttachment,
    clearSession,
  };
};
