import type { ApiResponse, Category, CategoryInput } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const categoriesService = () => {
  const categories = ref<Category[]>([]);
  const loaded = shallowRef(false);
  const loading = shallowRef(false);
  const saving = shallowRef(false);
  const error = shallowRef("");

  const fetchCategories = async (): Promise<void> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    try {
      const response = await $api<ApiResponse<Category[]>>("/categories");
      categories.value = response.data;
      loaded.value = true;
      error.value = "";
    } catch (err) {
      error.value = apiErrorMessage(err);
    } finally {
      loading.value = false;
    }
  };

  const createCategory = async (input: CategoryInput): Promise<Category | null> => {
    const { $api } = useNuxtApp();
    saving.value = true;
    try {
      const response = await $api<ApiResponse<Category>>("/categories", {
        method: "POST",
        body: input,
      });
      categories.value = [...categories.value, response.data];
      error.value = "";
      return response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    } finally {
      saving.value = false;
    }
  };

  const updateCategory = async (id: string, input: CategoryInput): Promise<Category | null> => {
    const { $api } = useNuxtApp();
    saving.value = true;
    try {
      const response = await $api<ApiResponse<Category>>(`/categories/${id}`, {
        method: "PUT",
        body: input,
      });
      categories.value = categories.value.map((category) =>
        category.id === id ? response.data : category,
      );
      error.value = "";
      return response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    } finally {
      saving.value = false;
    }
  };

  const deleteCategory = async (id: string): Promise<boolean> => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/categories/${id}`, { method: "DELETE" });
      categories.value = categories.value.filter((category) => category.id !== id);
      error.value = "";
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  return {
    categories: readonly(categories),
    loaded: readonly(loaded),
    loading: readonly(loading),
    saving: readonly(saving),
    error: readonly(error),
    fetchCategories,
    createCategory,
    updateCategory,
    deleteCategory,
  };
};
