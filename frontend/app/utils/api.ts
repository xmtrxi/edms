export const apiErrorMessage = (error: unknown): string => {
  if (error instanceof Error && "response" in error) {
    const response = (error as { response?: { status?: number; _data?: unknown } }).response;
    const body = response?._data;
    if (body && typeof body === "object" && "error" in body) {
      const message = (body as { error?: unknown }).error;
      if (typeof message === "string" && message) return message;
    }
    if (response?.status) return `Request failed (${response.status})`;
  }
  return error instanceof Error ? error.message : "Request failed";
};
