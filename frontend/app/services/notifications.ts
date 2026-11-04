import type { ApiResponse, AppNotification, NotificationPage } from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const notificationsService = () => {
  const items = ref<AppNotification[]>([]);
  const unread = ref(0);
  const error = shallowRef("");

  const call = async <T>(path: string, options: Record<string, unknown> = {}) => {
    const { $api } = useNuxtApp();
    try {
      const response = await $api<ApiResponse<T> | undefined>(path, options);
      error.value = "";
      return { ok: true, data: response?.data ?? null };
    } catch (err) {
      error.value = apiErrorMessage(err);
      return { ok: false, data: null };
    }
  };

  const load = async () => {
    const { data } = await call<NotificationPage>("/notifications?limit=30");
    if (!data) return;
    items.value = data.items;
    unread.value = data.unread;
  };

  /** A notification pushed over the realtime socket. */
  const receive = (notification: AppNotification) => {
    if (items.value.some((item) => item.id === notification.id)) return;
    items.value = [notification, ...items.value].slice(0, 50);
    if (!notification.read_at) unread.value++;
  };

  const markRead = async (notification: AppNotification) => {
    if (notification.read_at) return;
    notification.read_at = new Date().toISOString();
    unread.value = Math.max(0, unread.value - 1);
    await call(`/notifications/${encodeURIComponent(notification.id)}/read`, { method: "POST" });
  };

  const markAllRead = async () => {
    const { ok } = await call("/notifications/read-all", { method: "POST" });
    if (!ok) return false;
    const now = new Date().toISOString();
    items.value = items.value.map((item) => ({ ...item, read_at: item.read_at ?? now }));
    unread.value = 0;
    return true;
  };

  /** Removes read notifications; unread ones stay. */
  const clearRead = async () => {
    const { ok } = await call("/notifications", { method: "DELETE" });
    if (ok) items.value = items.value.filter((item) => !item.read_at);
    return ok;
  };

  return { items, unread, error, load, receive, markRead, markAllRead, clearRead };
};
