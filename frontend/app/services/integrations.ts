import type {
  ApiResponse,
  DeadTask,
  IntegrationCheck,
  QueueStats,
  ReconcileResult,
} from "@/types/api";
import { apiErrorMessage } from "@/utils/api";

export const integrationsService = () => {
  const error = shallowRef("");
  const busy = shallowRef(false);

  const call = async <T>(path: string, options: Record<string, unknown> = {}) => {
    const { $api } = useNuxtApp();
    busy.value = true;
    try {
      const response = await $api<ApiResponse<T> | undefined>(path, options);
      error.value = "";
      return { ok: true, data: response?.data ?? null };
    } catch (err) {
      error.value = apiErrorMessage(err);
      return { ok: false, data: null };
    } finally {
      busy.value = false;
    }
  };

  const base = "/admin/integrations";
  const report = async () => (await call<IntegrationCheck[]>(base)).data ?? [];
  const queues = async () => (await call<QueueStats[]>(`${base}/queues`)).data ?? [];
  const deadTasks = async () => (await call<DeadTask[]>(`${base}/dead-tasks`)).data ?? [];
  const reconcile = async (requeueDead: boolean) =>
    (
      await call<ReconcileResult>(`${base}/reconcile`, {
        method: "POST",
        body: { requeue_dead: requeueDead },
      })
    ).data;
  const taskPath = (task: DeadTask) =>
    `${base}/dead-tasks/${encodeURIComponent(task.queue)}/${encodeURIComponent(task.id)}`;
  const retryTask = async (task: DeadTask) =>
    (await call(`${taskPath(task)}/retry`, { method: "POST" })).ok;
  const discardTask = async (task: DeadTask) =>
    (await call(taskPath(task), { method: "DELETE" })).ok;

  return { error, busy, report, queues, deadTasks, reconcile, retryTask, discardTask };
};
