import type {
  ApiResponse,
  Department,
  FolderGrant,
  FolderGrantInput,
  FolderInput,
  ManagedFolder,
  User,
} from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const foldersService = () => {
  const folders = shallowRef<ManagedFolder[]>([]);
  const loading = shallowRef(false);
  const error = shallowRef("");
  const grants = shallowRef<FolderGrant[]>([]);
  const users = shallowRef<User[]>([]);
  const departments = shallowRef<Department[]>([]);

  const fetchFolders = async (): Promise<void> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    try {
      const response = await $api<ApiResponse<ManagedFolder[]>>("/folders");
      folders.value = response.data;
      error.value = "";
    } catch (err) {
      error.value = apiErrorMessage(err);
    } finally {
      loading.value = false;
    }
  };

  const createFolder = async (input: FolderInput): Promise<ManagedFolder | null> => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<ManagedFolder>>("/folders", {
        method: "POST",
        body: input,
      });
      folders.value = [...folders.value, response.data];
      error.value = "";
      return response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  const updateFolder = async (id: string, input: FolderInput): Promise<ManagedFolder | null> => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<ManagedFolder>>(`/folders/${id}`, {
        method: "PUT",
        body: input,
      });
      folders.value = folders.value.map((folder) => (folder.id === id ? response.data : folder));
      error.value = "";
      return response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  const deleteFolder = async (id: string): Promise<boolean> => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/folders/${id}`, { method: "DELETE" });
      folders.value = folders.value.filter((folder) => folder.id !== id);
      error.value = "";
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const fetchGrants = async (folderId: string): Promise<void> => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<FolderGrant[]>>(`/folders/${folderId}/grants`);
      grants.value = response.data;
      error.value = "";
    } catch (err) {
      error.value = apiErrorMessage(err);
      grants.value = [];
    }
  };

  const saveGrant = async (folderId: string, input: FolderGrantInput): Promise<boolean> => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/folders/${folderId}/grants`, { method: "PUT", body: input });
      // Reload so the list carries names and inherited grants from the server.
      await fetchGrants(folderId);
      error.value = "";
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const removeGrant = async (folderId: string, grantId: string): Promise<boolean> => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/folders/${folderId}/grants/${grantId}`, { method: "DELETE" });
      grants.value = grants.value.filter((grant) => grant.id !== grantId);
      error.value = "";
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const fetchUsers = async (): Promise<void> => {
    if (users.value.length) return;
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<User[]>>("/users");
      users.value = response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
    }
  };

  const fetchDepartments = async (): Promise<void> => {
    if (departments.value.length) return;
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<Department[]>>("/departments");
      departments.value = response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
    }
  };

  return {
    folders,
    loading,
    error,
    grants,
    users,
    departments,
    fetchFolders,
    createFolder,
    updateFolder,
    deleteFolder,
    fetchGrants,
    saveGrant,
    removeGrant,
    fetchUsers,
    fetchDepartments,
  };
};
