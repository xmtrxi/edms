<script setup lang="ts">
  import { CircleAlertIcon, Clock3Icon, Loader2Icon } from "@lucide/vue";
  import { computed } from "vue";

  import type { ProcessingJob } from "@/types/api";
  import { formatDateTime } from "@/utils/format";

  const props = defineProps<{ job: ProcessingJob | null; refreshing?: boolean }>();

  const taskStatus = computed(() => String(props.job?.result?.task_status ?? ""));
  const queuePosition = computed(() => props.job?.result?.task_position);
  const taskMeta = computed(
    () => props.job?.result?.task_meta as Record<string, unknown> | undefined,
  );
  const processedCount = computed(
    () => taskMeta.value?.num_processed ?? taskMeta.value?.num_succeeded,
  );
</script>

<template>
  <div class="border-border bg-card rounded-xl border p-4">
    <div class="mb-3 flex items-center justify-between gap-2">
      <p class="text-muted-foreground text-xs font-semibold tracking-wider uppercase">
        Processing log
      </p>
      <Loader2Icon v-if="refreshing" class="text-primary h-3.5 w-3.5 animate-spin" />
    </div>

    <div v-if="job" class="space-y-2 text-xs">
      <div class="flex items-center gap-2">
        <Loader2Icon
          v-if="job.status === 'running'"
          class="text-primary h-3.5 w-3.5 animate-spin"
        />
        <CircleAlertIcon v-else-if="job.status === 'failed'" class="text-destructive h-3.5 w-3.5" />
        <Clock3Icon v-else class="text-muted-foreground h-3.5 w-3.5" />
        <span class="font-medium capitalize">{{ job.status }}</span>
      </div>
      <div class="space-y-1.5 pl-5.5">
        <p v-if="taskStatus" class="text-muted-foreground">Docling: {{ taskStatus }}</p>
        <p v-if="queuePosition !== undefined" class="text-muted-foreground">
          Queue position: {{ queuePosition }}
        </p>
        <p v-if="processedCount !== undefined" class="text-muted-foreground">
          Processed: {{ processedCount }}{{ taskMeta?.num_docs ? ` / ${taskMeta.num_docs}` : "" }}
        </p>
        <p v-if="job.last_error" class="text-destructive break-words">{{ job.last_error }}</p>
        <p v-else-if="job.status === 'queued'" class="text-muted-foreground">
          Waiting for an OCR worker.
        </p>
        <p class="text-muted-foreground/80">Last update: {{ formatDateTime(job.updated_at) }}</p>
      </div>
    </div>

    <p v-else class="text-muted-foreground text-xs">No processing job has been recorded yet.</p>
  </div>
</template>
