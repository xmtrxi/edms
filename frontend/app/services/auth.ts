import type { ApiResponse, AuthResponse, LoginRequest, RegisterRequest, User } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";
import { clearToken, setToken } from "@/utils/token";

export const authService = () => {
  const user = ref<User | null>(null);
  const loading = ref(false);
  const error = ref("");

  const login = async (credentials: LoginRequest): Promise<boolean> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    error.value = "";
    try {
      const res = await $api<ApiResponse<AuthResponse>>("/auth/login", {
        method: "POST",
        body: credentials,
      });
      setToken(res.data.token);
      user.value = res.data.user;
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    } finally {
      loading.value = false;
    }
  };

  const register = async (payload: RegisterRequest): Promise<boolean> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    error.value = "";
    try {
      await $api<ApiResponse<User>>("/auth/register", {
        method: "POST",
        body: payload,
      });
      return true;
    } catch (err) {
      error.value = apiErrorMessage(err);
      return false;
    } finally {
      loading.value = false;
    }
  };

  const fetchMe = async (): Promise<boolean> => {
    const { $api } = useNuxtApp();
    loading.value = true;
    try {
      const res = await $api<ApiResponse<User>>("/auth/me");
      user.value = res.data;
      return true;
    } catch (err) {
      const status = (err as { response?: { status?: number } }).response?.status;
      if (status === 401) {
        clearToken();
        user.value = null;
      } else {
        error.value = apiErrorMessage(err);
      }
      return false;
    } finally {
      loading.value = false;
    }
  };

  const logout = (): void => {
    clearToken();
    user.value = null;
    error.value = "";
  };

  return { user, loading, error, login, register, fetchMe, logout };
};
