const KEY = "lumen.token";

export const getToken = (): string => {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(KEY) ?? "";
};

export const setToken = (token: string): void => {
  if (typeof window === "undefined") return;
  if (token) window.localStorage.setItem(KEY, token);
  else window.localStorage.removeItem(KEY);
};

export const clearToken = (): void => {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem(KEY);
};
