import type {
  ApiResponse,
  Invitation,
  Member,
  MembershipRole,
  MembershipStatus,
} from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

/** Members and invitations of the organization the session is acting in. */
export const organizationsService = () => {
  const members = shallowRef<Member[]>([]);
  const invitations = shallowRef<Invitation[]>([]);
  const loading = shallowRef(false);
  const error = shallowRef("");

  const loadMembers = async () => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<Member[]>>("/org/members");
      members.value = res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
    }
  };

  const loadInvitations = async () => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<Invitation[]>>("/org/invitations");
      invitations.value = res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
    }
  };

  const load = async (includeInvitations: boolean) => {
    loading.value = true;
    error.value = "";
    await Promise.all([loadMembers(), includeInvitations ? loadInvitations() : Promise.resolve()]);
    loading.value = false;
  };

  const invite = async (input: {
    email: string;
    name: string;
    role: MembershipRole;
  }): Promise<string | null> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<{ invite_link: string }>>("/org/invitations", {
        method: "POST",
        body: input,
      });
      await loadInvitations();
      return res.data.invite_link;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  const revokeInvitation = async (id: string) => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/org/invitations/${encodeURIComponent(id)}`, { method: "DELETE" });
      invitations.value = invitations.value.filter((item) => item.id !== id);
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const updateMember = async (
    id: string,
    changes: { role?: MembershipRole; status?: MembershipStatus },
  ) => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/org/members/${encodeURIComponent(id)}`, { method: "PATCH", body: changes });
      members.value = members.value.map((item) =>
        item.id === id ? { ...item, ...changes } : item,
      );
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const removeMember = async (id: string) => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/org/members/${encodeURIComponent(id)}`, { method: "DELETE" });
      members.value = members.value.filter((item) => item.id !== id);
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const rename = async (name: string) => {
    const { $api } = useNuxtApp();
    try {
      await $api("/org", { method: "PATCH", body: { name } });
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  return {
    members,
    invitations,
    loading,
    error,
    load,
    invite,
    revokeInvitation,
    updateMember,
    removeMember,
    rename,
  };
};
