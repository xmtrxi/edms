import type {
  ApiResponse,
  AuthResponse,
  DeploymentMeta,
  InvitationPreview,
  LoginRequest,
  Organization,
  RegisterRequest,
  SessionResponse,
  SwitchOrganizationResponse,
  User,
} from "@/types/api";
import { apiErrorMessage } from "@/utils/api";
import { clearToken, setToken } from "@/utils/token";

export const authService = () => {
  const user = ref<User | null>(null);
  const organization = ref<Organization | null>(null);
  const organizations = ref<Organization[]>([]);
  const meta = ref<DeploymentMeta | null>(null);
  const loading = ref(false);
  const error = ref("");

  const isOrgAdmin = computed(
    () => organization.value?.role === "owner" || organization.value?.role === "admin",
  );
  const isPersonalWorkspace = computed(() => organization.value?.kind === "personal");
  const canCreateOrganization = computed(() => Boolean(meta.value?.multi_org));

  const applyAuth = (data: AuthResponse) => {
    setToken(data.token);
    user.value = data.user;
    organization.value = data.organization?.id ? data.organization : null;
  };

  const fetchMeta = async (): Promise<DeploymentMeta | null> => {
    if (meta.value) return meta.value;
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<DeploymentMeta>>("/meta");
      meta.value = res.data;
    } catch {
      meta.value = null;
    }
    return meta.value;
  };

  const login = async (credentials: LoginRequest): Promise<boolean> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    error.value = "";
    try {
      const res = await $api<ApiResponse<AuthResponse>>("/auth/login", {
        method: "POST",
        body: credentials,
      });
      applyAuth(res.data);
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    } finally {
      loading.value = false;
    }
  };

  /** Creates the account and signs in; the server returns a session token. */
  const register = async (payload: RegisterRequest): Promise<boolean> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    error.value = "";
    try {
      const res = await $api<ApiResponse<AuthResponse>>("/auth/register", {
        method: "POST",
        body: payload,
      });
      applyAuth(res.data);
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    } finally {
      loading.value = false;
    }
  };

  /** Loads the user, the active workspace and every workspace they can switch to. */
  const fetchSession = async (): Promise<boolean> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    try {
      const res = await $api<ApiResponse<SessionResponse>>("/auth/session");
      user.value = res.data.user;
      organization.value = res.data.organization;
      organizations.value = res.data.organizations;
      void fetchMeta();
      return true;
    } catch (err) {
      const status = (err as { response?: { status?: number } }).response?.status;
      if (status === 401 || status === 403) {
        clearToken();
        user.value = null;
      }
      error.value = apiErrorMessage(err);
      return false;
    } finally {
      loading.value = false;
    }
  };

  /**
   * Switching workspaces changes every list in the app, so the page reloads
   * with the new token instead of trying to reset each cached store.
   */
  const reloadInto = (data: SwitchOrganizationResponse, path = "/") => {
    setToken(data.token);
    window.location.assign(path);
  };

  const switchOrganization = async (orgId: string): Promise<boolean> => {
    if (orgId === organization.value?.id) return true;
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<SwitchOrganizationResponse>>("/auth/switch-org", {
        method: "POST",
        body: { org_id: orgId },
      });
      reloadInto(res.data);
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const createOrganization = async (name: string): Promise<boolean> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<SwitchOrganizationResponse>>("/orgs", {
        method: "POST",
        body: { name },
      });
      reloadInto(res.data, "/users");
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    }
  };

  const previewInvitation = async (token: string): Promise<InvitationPreview | null> => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<InvitationPreview>>(
        `/invitations/${encodeURIComponent(token)}`,
      );
      return res.data;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  /** New account from an email invitation. */
  const acceptInvitationAsNewUser = async (
    token: string,
    name: string,
    password: string,
  ): Promise<boolean> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    error.value = "";
    try {
      const res = await $api<ApiResponse<AuthResponse>>("/auth/setup", {
        method: "POST",
        body: { token, name, password },
      });
      applyAuth(res.data);
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    } finally {
      loading.value = false;
    }
  };

  /** Signed-in user joins the organization they were invited to. */
  const acceptInvitation = async (token: string): Promise<boolean> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    error.value = "";
    try {
      const res = await $api<ApiResponse<SwitchOrganizationResponse>>("/invitations/accept", {
        method: "POST",
        body: { token },
      });
      reloadInto(res.data);
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    } finally {
      loading.value = false;
    }
  };

  const logout = (): void => {
    clearToken();
    user.value = null;
    organization.value = null;
    organizations.value = [];
    error.value = "";
  };

  return {
    user,
    organization,
    organizations,
    meta,
    loading,
    error,
    isOrgAdmin,
    isPersonalWorkspace,
    canCreateOrganization,
    fetchMeta,
    login,
    register,
    fetchSession,
    switchOrganization,
    createOrganization,
    previewInvitation,
    acceptInvitationAsNewUser,
    acceptInvitation,
    logout,
  };
};
