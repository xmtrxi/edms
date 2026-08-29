import type { ApiResponse, FormSubmission, FormTemplate } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const formsService = () => {
  const templates = shallowRef<FormTemplate[]>([]);
  const submissions = shallowRef<FormSubmission[]>([]);
  const loading = shallowRef(false);
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
    loading,
    error,
    loadTemplates,
    loadSubmissions,
    createTemplate,
    submit,
  };
};
