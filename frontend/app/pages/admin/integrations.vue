<script setup lang="ts">
  import {
    CheckCircle2Icon,
    CircleAlertIcon,
    OctagonXIcon,
    RefreshCwIcon,
    RotateCcwIcon,
    Trash2Icon,
    WrenchIcon,
  } from "@lucide/vue";
  import { onMounted, ref } from "vue";
  import { toast } from "vue-sonner";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import type { DeadTask, IntegrationCheck, QueueStats } from "@/types/api";
  import { formatDateTime } from "@/utils/format";

  definePageMeta({
    title: "Integrations",
    description: "Background work and the outside services this installation depends on.",
    breadcrumbs: [{ label: "Administration", to: "/" }, { label: "Integrations" }],
  });
  useHead({ title: "Integrations" });

  const service = integrationsService();
  const checks = ref<IntegrationCheck[]>([]);
  const queues = ref<QueueStats[]>([]);
  const dead = ref<DeadTask[]>([]);
  const loaded = ref(false);

  const load = async () => {
    [checks.value, queues.value, dead.value] = await Promise.all([
      service.report(),
      service.queues(),
      service.deadTasks(),
    ]);
    loaded.value = true;
  };
  onMounted(load);

  const fixable = computed(() => checks.value.some((check) => check.fixable));

  const reconcile = async (requeueDead: boolean) => {
    const result = await service.reconcile(requeueDead);
    if (!result) {
      toast.error(service.error.value || "Could not retry stuck work");
      return;
    }
    const parts = [
      result.emails && `${result.emails} emails`,
      result.ocr && `${result.ocr} text extractions`,
      result.pdf_jobs && `${result.pdf_jobs} PDF jobs`,
      result.dead_tasks && `${result.dead_tasks} dead tasks`,
    ].filter(Boolean);
    toast.success(parts.length ? `Retried ${parts.join(", ")}` : "Nothing needed retrying");
    await load();
  };

  const retry = async (task: DeadTask) => {
    if (await service.retryTask(task)) {
      dead.value = dead.value.filter((item) => item.id !== task.id);
      toast.success("Task queued again");
    } else toast.error(service.error.value || "Could not retry the task");
  };

  const discard = async (task: DeadTask) => {
    if (await service.discardTask(task)) {
      dead.value = dead.value.filter((item) => item.id !== task.id);
    } else toast.error(service.error.value || "Could not delete the task");
  };

  const taskLabel: Record<string, string> = {
    "document:ocr": "Text extraction",
    "search:reindex": "Search rebuild",
    "email:deliver": "Email",
    "pdf:run": "PDF tool",
  };
  const statusIcon = { ok: CheckCircle2Icon, warning: CircleAlertIcon, failing: OctagonXIcon };
  const statusClass = { ok: "text-primary", warning: "text-warning", failing: "text-destructive" };
</script>

<template>
  <div class="mx-auto max-w-5xl space-y-5">
    <div class="flex flex-wrap items-center gap-2">
      <p class="text-muted-foreground flex-1 text-sm">
        Stuck emails, extractions and PDF jobs are retried automatically every hour. You can retry
        them now.
      </p>
      <Button variant="outline" class="gap-1.5" :disabled="service.busy.value" @click="load">
        <RefreshCwIcon class="size-4" :class="service.busy.value && 'animate-spin'" /> Refresh
      </Button>
      <Button class="gap-1.5" :disabled="service.busy.value || !fixable" @click="reconcile(false)">
        <WrenchIcon class="size-4" /> Retry stuck work
      </Button>
    </div>

    <section class="bg-card rounded-2xl border shadow-sm" aria-labelledby="checks-heading">
      <h2 id="checks-heading" class="border-b p-5 text-sm font-semibold">Health</h2>
      <p v-if="!loaded" class="text-muted-foreground p-5 text-sm">Checking…</p>
      <ul v-else class="divide-y">
        <li v-for="check in checks" :key="check.name" class="flex items-start gap-3 p-4">
          <component
            :is="statusIcon[check.status]"
            class="mt-0.5 size-5 shrink-0"
            :class="statusClass[check.status]"
          />
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium">{{ check.name }}</p>
            <p class="text-muted-foreground mt-0.5 text-xs">{{ check.detail }}</p>
          </div>
          <Badge
            :variant="
              check.status === 'ok'
                ? 'secondary'
                : check.status === 'failing'
                  ? 'destructive'
                  : 'outline'
            "
          >
            {{
              check.status === "ok"
                ? "OK"
                : check.status === "failing"
                  ? "Failing"
                  : "Needs attention"
            }}
          </Badge>
        </li>
      </ul>
    </section>

    <section
      v-if="queues.length"
      class="bg-card overflow-x-auto rounded-2xl border shadow-sm"
      aria-labelledby="queues-heading"
    >
      <h2 id="queues-heading" class="border-b p-5 text-sm font-semibold">Task queues</h2>
      <table class="w-full text-sm">
        <thead class="text-muted-foreground text-left text-xs">
          <tr>
            <th class="px-5 py-2 font-medium">Queue</th>
            <th class="px-3 py-2 text-right font-medium">Waiting</th>
            <th class="px-3 py-2 text-right font-medium">Running</th>
            <th class="px-3 py-2 text-right font-medium">Retrying</th>
            <th class="px-3 py-2 text-right font-medium">Dead</th>
            <th class="px-3 py-2 text-right font-medium">Done today</th>
            <th class="px-5 py-2 text-right font-medium">Failed today</th>
          </tr>
        </thead>
        <tbody class="divide-y tabular-nums">
          <tr v-for="q in queues" :key="q.queue">
            <td class="px-5 py-2.5 font-medium">
              {{ q.queue }}<Badge v-if="q.paused" variant="outline" class="ml-2">Paused</Badge>
            </td>
            <td class="px-3 py-2.5 text-right">{{ q.pending + q.scheduled }}</td>
            <td class="px-3 py-2.5 text-right">{{ q.active }}</td>
            <td class="px-3 py-2.5 text-right">{{ q.retry }}</td>
            <td class="px-3 py-2.5 text-right" :class="q.dead && 'text-destructive font-medium'">
              {{ q.dead }}
            </td>
            <td class="px-3 py-2.5 text-right">{{ q.processed_today }}</td>
            <td class="px-5 py-2.5 text-right">{{ q.failed_today }}</td>
          </tr>
        </tbody>
      </table>
    </section>

    <section class="bg-card rounded-2xl border shadow-sm" aria-labelledby="dead-heading">
      <div class="flex flex-wrap items-center gap-2 border-b p-5">
        <div class="flex-1">
          <h2 id="dead-heading" class="text-sm font-semibold">Dead-letter tasks</h2>
          <p class="text-muted-foreground mt-0.5 text-xs">
            Tasks that failed every retry. Fix the cause, then retry them.
          </p>
        </div>
        <Button
          v-if="dead.length"
          size="sm"
          variant="outline"
          :disabled="service.busy.value"
          @click="reconcile(true)"
        >
          Retry all
        </Button>
      </div>
      <p v-if="loaded && !dead.length" class="text-muted-foreground p-5 text-sm">No dead tasks.</p>
      <ul v-else class="divide-y">
        <li
          v-for="task in dead"
          :key="`${task.queue}/${task.id}`"
          class="flex flex-wrap items-start gap-3 p-4"
        >
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium">
              {{ taskLabel[task.type] ?? task.type }}
              <span class="text-muted-foreground font-normal"
                >· failed {{ formatDateTime(task.failed_at) }} after
                {{ task.retried }} retries</span
              >
            </p>
            <p class="text-destructive mt-0.5 font-mono text-xs break-all">{{ task.last_error }}</p>
            <p v-if="task.payload" class="text-muted-foreground mt-0.5 font-mono text-xs break-all">
              {{ JSON.stringify(task.payload) }}
            </p>
          </div>
          <div class="flex gap-1">
            <Button size="sm" variant="outline" class="gap-1.5" @click="retry(task)"
              ><RotateCcwIcon class="size-3.5" />Retry</Button
            >
            <Button
              size="sm"
              variant="ghost"
              class="text-destructive"
              aria-label="Delete task"
              @click="discard(task)"
              ><Trash2Icon class="size-3.5"
            /></Button>
          </div>
        </li>
      </ul>
    </section>
  </div>
</template>
