import type {
  ApiResponse,
  FormComment,
  FormSubmission,
  FormSubmissionDetail,
  FormTemplate,
} from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const formsService = () => {
  const templates = shallowRef<FormTemplate[]>([]);
  const submissions = shallowRef<FormSubmission[]>([]);
  const pending = shallowRef<FormSubmission[]>([]);
  const loading = shallowRef(false);
  const reviewBusy = shallowRef(false);
  const error = shallowRef("");

  const loadTemplates = async () => {
    const { $api } = useNuxtApp();
    loading.value = true;
    try {
      const response = await $api<ApiResponse<FormTemplate[]>>("/forms");
      templates.value = response.data;
      error.value = "";
    } catch (err) {
      error.value = apiErrorMessage(err);
    } finally {
      loading.value = false;
    }
  };
  const loadSubmissions = async () => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<FormSubmission[]>>("/forms/submissions/mine");
      submissions.value = response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
    }
  };
  const createTemplate = async (
    input: Omit<FormTemplate, "id" | "version" | "created_by" | "created_at" | "updated_at">,
  ) => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<FormTemplate>>("/forms", {
        method: "POST",
        body: input,
      });
      templates.value = [response.data, ...templates.value];
      return response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };
  const updateTemplate = async (id: string, input: Partial<FormTemplate>) => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<FormTemplate>>(`/forms/${id}`, {
        method: "PATCH",
        body: input,
      });
      templates.value = templates.value.map((item) => (item.id === id ? response.data : item));
      return response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };
  const setTemplateStatus = async (id: string, status: "published" | "archived") => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<FormTemplate>>(
        `/forms/${id}/${status === "published" ? "publish" : "archive"}`,
        { method: "POST" },
      );
      templates.value = templates.value.map((item) => (item.id === id ? response.data : item));
      return response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };
  const loadPending = async () => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<FormSubmission[]>>("/forms/submissions/pending");
      pending.value = response.data;
      error.value = "";
      return response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return [];
    }
  };
  const submission = async (id: string) => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<FormSubmissionDetail>>(`/forms/submissions/${id}`);
      return response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };
  const addComment = async (id: string, body: string): Promise<FormComment | null> => {
    const { $api } = useNuxtApp();
    reviewBusy.value = true;
    try {
      const response = await $api<ApiResponse<FormComment>>(`/forms/submissions/${id}/comments`, {
        method: "POST",
        body: { body },
      });
      error.value = "";
      return response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    } finally {
      reviewBusy.value = false;
    }
  };
  const decide = async (
    id: string,
    action: "approve" | "reject" | "return",
    comment: string,
  ): Promise<boolean> => {
    const { $api } = useNuxtApp();
    reviewBusy.value = true;
    try {
      await $api(`/forms/submissions/${id}/decision`, {
        method: "POST",
        body: { action, comment },
      });
      pending.value = pending.value.filter((item) => item.id !== id);
      error.value = "";
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    } finally {
      reviewBusy.value = false;
    }
  };
  const submit = async (
    templateId: string,
    departmentId: string,
    values: Record<string, unknown>,
  ) => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<FormSubmission>>(`/forms/${templateId}/submissions`, {
        method: "POST",
        body: { department_id: departmentId, values },
      });
      submissions.value = [response.data, ...submissions.value];
      return response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };
  return {
    templates,
    submissions,
    pending,
    loading,
    reviewBusy,
    error,
    loadTemplates,
    loadSubmissions,
    createTemplate,
    updateTemplate,
    setTemplateStatus,
    loadPending,
    submission,
    addComment,
    decide,
    submit,
  };
};
