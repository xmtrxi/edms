import { authService } from "@/services/auth";

export const useAuthStore = defineStore("auth", () => {
  const service = authService();
  return { ...service };
});
