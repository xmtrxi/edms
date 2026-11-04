import type { ApiResponse, Department, Group, GroupMember, Role, User } from "@/types/api";
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
  /** Runs a request and records its error; returns null on failure. */
  const call = async <T>(
    path: string,
    options: Record<string, unknown> = {},
  ): Promise<T | null> => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<T> | undefined>(path, options);
      error.value = "";
      return (response?.data ?? true) as T;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };
  const groups = shallowRef<Group[]>([]);
  const loadGroups = async () => {
    groups.value = (await call<Group[]>("/admin/groups")) ?? [];
  };
  const updateDepartment = async (
    id: string,
    input: { name: string; description?: string; parent_id?: string },
  ) => {
    const updated = await call<Department>(`/admin/departments/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: input,
    });
    if (updated)
      departments.value = departments.value.map((item) => (item.id === id ? updated : item));
    return updated;
  };
  const deleteDepartment = async (id: string) => {
    const ok = await call(`/admin/departments/${encodeURIComponent(id)}`, { method: "DELETE" });
    if (ok) departments.value = departments.value.filter((item) => item.id !== id);
    return Boolean(ok);
  };
  const removeMember = async (departmentId: string, userId: string) =>
    Boolean(
      await call(
        `/admin/departments/${encodeURIComponent(departmentId)}/members/${encodeURIComponent(userId)}`,
        { method: "DELETE" },
      ),
    );
  const createGroup = async (input: { name: string; description?: string }) => {
    const created = await call<Group>("/admin/groups", { method: "POST", body: input });
    if (created) await loadGroups();
    return created;
  };
  const deleteGroup = async (id: string) => {
    const ok = await call(`/admin/groups/${encodeURIComponent(id)}`, { method: "DELETE" });
    if (ok) groups.value = groups.value.filter((item) => item.id !== id);
    return Boolean(ok);
  };
  const groupMembers = async (id: string) =>
    (await call<GroupMember[]>(`/admin/groups/${encodeURIComponent(id)}/members`)) ?? [];
  const addGroupMember = async (id: string, userId: string) => {
    const ok = await call(`/admin/groups/${encodeURIComponent(id)}/members`, {
      method: "POST",
      body: { user_id: userId },
    });
    if (ok) await loadGroups();
    return Boolean(ok);
  };
  const removeGroupMember = async (id: string, userId: string) => {
    const ok = await call(
      `/admin/groups/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}`,
      { method: "DELETE" },
    );
    if (ok) await loadGroups();
    return Boolean(ok);
  };
  const createDepartment = async (
    input: Pick<Department, "name" | "description"> & { parent_id?: string },
  ) => {
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
  const addMember = async (
    departmentId: string,
    userId: string,
    role: "member" | "head" = "member",
  ) => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/admin/departments/${departmentId}/members`, {
        method: "POST",
        body: { user_id: userId, role },
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
    removeMember,
    updateDepartment,
    deleteDepartment,
    groups,
    loadGroups,
    createGroup,
    deleteGroup,
    groupMembers,
    addGroupMember,
    removeGroupMember,
  };
};
