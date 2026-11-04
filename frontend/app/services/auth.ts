import type {
  ApiResponse,
  AuthResponse,
  DeploymentMeta,
  AccountSession,
  InvitationPreview,
  LoginOutcome,
  LoginRequest,
  MFASetup,
  Organization,
  RegisterRequest,
  SessionResponse,
  SwitchOrganizationResponse,
  User,
} from "@/types/api";
import { apiErrorMessage } from "@/utils/api";
import { clearToken, getToken, setToken } from "@/utils/token";

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

  /** Pending two-step sign-in, kept only in memory. */
  const mfaToken = ref("");

  const login = async (credentials: LoginRequest): Promise<LoginOutcome> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    error.value = "";
    try {
      const res = await $api<ApiResponse<AuthResponse>>("/auth/login", {
        method: "POST",
        body: credentials,
      });
      if (res.data.mfa_required && res.data.mfa_token) {
        mfaToken.value = res.data.mfa_token;
        return "mfa";
      }
      applyAuth(res.data);
      return "signed-in";
    } catch (err) {
      error.value = apiErrorMessage(err);
      const body = (err as { response?: { _data?: { code?: string } } }).response?._data;
      return body?.code === "email_not_verified" ? "unverified" : "failed";
    } finally {
      loading.value = false;
    }
  };

  /** Second step: a code from the authenticator app or a recovery code. */
  const completeMfa = async (code: string): Promise<boolean> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    error.value = "";
    try {
      const res = await $api<ApiResponse<AuthResponse>>("/auth/login/mfa", {
        method: "POST",
        body: { mfa_token: mfaToken.value, code },
      });
      mfaToken.value = "";
      applyAuth(res.data);
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    } finally {
      loading.value = false;
    }
  };

  /** Token handed back by single sign-on in the URL fragment. */
  const acceptSsoToken = async (token: string): Promise<boolean> => {
    setToken(token);
    return fetchSession();
  };

  /** Where the browser goes to start single sign-on. */
  const ssoStartUrl = (returnTo = "/"): string => {
    const { apiBase } = useRuntimeConfig().public;
    return `${apiBase}/auth/sso/start?return=${encodeURIComponent(returnTo)}`;
  };

  /** Runs a request and keeps its error message; returns null on failure. */
  const call = async <T>(
    path: string,
    options: Parameters<typeof $fetch>[1] = {},
  ): Promise<T | null> => {
    const { $api } = useNuxtApp();
    error.value = "";
    try {
      return (await $api<T>(path, options as never)) as T;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return null;
    }
  };

  const requestPasswordReset = async (email: string): Promise<boolean> =>
    (await call("/auth/password/forgot", { method: "POST", body: { email } })) !== null;

  const resetPassword = async (token: string, password: string): Promise<boolean> =>
    (await call("/auth/password/reset", { method: "POST", body: { token, password } })) !== null;

  const verifyEmail = async (token: string): Promise<boolean> => {
    const res = await call<ApiResponse<User>>("/auth/email/verify", {
      method: "POST",
      body: { token },
    });
    if (res && user.value) user.value = res.data;
    return res !== null;
  };

  const resendVerification = async (email: string): Promise<boolean> =>
    (await call("/auth/email/resend", { method: "POST", body: { email } })) !== null;

  const sendVerification = async (): Promise<boolean> =>
    (await call("/auth/email/send-verification", { method: "POST" })) !== null;

  const updateProfile = async (name: string): Promise<boolean> => {
    const res = await call<ApiResponse<User>>("/auth/me", { method: "PATCH", body: { name } });
    if (res) user.value = res.data;
    return res !== null;
  };

  const changePassword = async (currentPassword: string, newPassword: string): Promise<boolean> =>
    (await call("/auth/password", {
      method: "POST",
      body: { current_password: currentPassword, new_password: newPassword },
    })) !== null;

  const listSessions = async (): Promise<AccountSession[]> =>
    (await call<ApiResponse<AccountSession[]>>("/auth/sessions"))?.data ?? [];

  const revokeSession = async (id: string): Promise<boolean> =>
    (await call(`/auth/sessions/${encodeURIComponent(id)}`, { method: "DELETE" })) !== null;

  const revokeOtherSessions = async (): Promise<boolean> =>
    (await call("/auth/sessions/revoke-others", { method: "POST" })) !== null;

  const beginMfa = async (): Promise<MFASetup | null> =>
    (await call<ApiResponse<MFASetup>>("/auth/mfa/setup", { method: "POST" }))?.data ?? null;

  const enableMfa = async (code: string): Promise<string[] | null> => {
    const res = await call<ApiResponse<{ recovery_codes: string[] }>>("/auth/mfa/enable", {
      method: "POST",
      body: { code },
    });
    if (res && user.value) user.value = { ...user.value, mfa_enabled: true };
    return res?.data.recovery_codes ?? null;
  };

  const disableMfa = async (password: string, code: string): Promise<boolean> => {
    const ok =
      (await call("/auth/mfa/disable", { method: "POST", body: { password, code } })) !== null;
    if (ok && user.value) user.value = { ...user.value, mfa_enabled: false };
    return ok;
  };

  const regenerateRecoveryCodes = async (code: string): Promise<string[] | null> =>
    (
      await call<ApiResponse<{ recovery_codes: string[] }>>("/auth/mfa/recovery-codes", {
        method: "POST",
        body: { code },
      })
    )?.data.recovery_codes ?? null;

  /**
   * Creates the account and signs in. Returns "unverified" when the account
   * must confirm its email address first.
   */
  const register = async (
    payload: RegisterRequest,
  ): Promise<"signed-in" | "unverified" | false> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    error.value = "";
    try {
      const res = await $api<ApiResponse<AuthResponse>>("/auth/register", {
        method: "POST",
        body: payload,
      });
      if (res.data.verification_required) return "unverified";
      applyAuth(res.data);
      return "signed-in";
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

  /** Ends the session on the server too, so the token stops working. */
  const logout = async (): Promise<void> => {
    if (getToken()) {
      const { $api } = useNuxtApp();
      await $api("/auth/logout", { method: "POST" }).catch(() => undefined);
    }
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
    completeMfa,
    acceptSsoToken,
    ssoStartUrl,
    requestPasswordReset,
    resetPassword,
    verifyEmail,
    resendVerification,
    sendVerification,
    updateProfile,
    changePassword,
    listSessions,
    revokeSession,
    revokeOtherSessions,
    beginMfa,
    enableMfa,
    disableMfa,
    regenerateRecoveryCodes,
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
