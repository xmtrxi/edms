import type { ApiResponse, Department, Role, User } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const administrationService = () => {
  const departments = shallowRef<Department[]>([]);
  const roles = shallowRef<Role[]>([]);
  const users = shallowRef<User[]>([]);
  const loading = shallowRef(false);
  const error = shallowRef("");
  const load = async () => {
    const { $api } = useNuxtApp();
    loading.value = true;
    try {
      const [departmentResponse, roleResponse, userResponse] = await Promise.all([
        $api<ApiResponse<Department[]>>("/admin/departments"),
        $api<ApiResponse<Role[]>>("/admin/roles"),
        $api<ApiResponse<User[]>>("/users"),
      ]);
      departments.value = departmentResponse.data;
      roles.value = roleResponse.data;
      users.value = userResponse.data;
      error.value = "";
    } catch (err) {
      error.value = apiErrorMessage(err);
    } finally {
      loading.value = false;
    }
  };
  const createDepartment = async (input: Pick<Department, "name" | "description">) => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<Department>>("/admin/departments", {
        method: "POST",
        body: input,
      });
      departments.value = [...departments.value, response.data];
      return response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };
  const createRole = async (input: {
    key: string;
    name: string;
    description: string;
    permissions: string[];
  }) => {
    const { $api } = useNuxtApp();
    try {
      const r = await $api<ApiResponse<Role>>("/admin/roles", { method: "POST", body: input });
      roles.value = [...roles.value, r.data];
      return r.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };
  const setPermissions = async (key: string, permissions: string[]) => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/admin/roles/${key}/permissions`, { method: "PUT", body: { permissions } });
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };
  const assignRole = async (key: string, userId: string) => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/admin/roles/${key}/users/${userId}`, { method: "PUT" });
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };
  const invite = async (name: string, email: string) => {
    const { $api } = useNuxtApp();
    try {
      const r = await $api<ApiResponse<{ setup_link: string }>>("/admin/invitations", {
        method: "POST",
        body: { name, email },
      });
      return r.data.setup_link;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };
  const members = async (departmentId: string) => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<
        ApiResponse<Array<{ id: string; user_id: string; role: string }>>
      >(`/admin/departments/${departmentId}/members`);
      return response.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return [];
    }
  };
  const addMember = async (departmentId: string, userId: string) => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/admin/departments/${departmentId}/members`, {
        method: "POST",
        body: { user_id: userId },
      });
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };
  return {
    departments,
    roles,
    users,
    loading,
    error,
    load,
    createDepartment,
    createRole,
    setPermissions,
    assignRole,
    invite,
    members,
    addMember,
  };
};
