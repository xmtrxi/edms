<script setup lang="ts">
  import { DownloadIcon, HistoryIcon, RefreshCwIcon } from "@lucide/vue";
  import { computed, onMounted, ref, shallowRef } from "vue";

  import type { AuditEvent } from "@/services/operations";
  import { formatDateTime } from "@/utils/format";

  definePageMeta({
    title: "Audit trail",
    description: "A verifiable record of office activity.",
    breadcrumbs: [{ label: "Administration", to: "/" }, { label: "Audit trail" }],
  });
  const operations = operationsService();
  const events = shallowRef<AuditEvent[]>([]);
  const query = ref("");
  const refresh = async () => {
    events.value = await operations.audit();
  };
  const filtered = computed(() =>
    events.value.filter((item) =>
      `${item.action} ${item.resource} ${item.resource_id}`
        .toLowerCase()
        .includes(query.value.toLowerCase()),
    ),
  );
  onMounted(refresh);
</script>
<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center gap-2">
      <Input
        v-model="query"
        class="min-w-60 flex-1"
        placeholder="Filter actions or records…"
      /><Button variant="outline" class="gap-2" @click="refresh"
        ><RefreshCwIcon class="size-4" />Refresh</Button
      ><Button variant="outline" class="gap-2" @click="operations.exportAudit"
        ><DownloadIcon class="size-4" />Export</Button
      >
    </div>
    <div
      v-if="operations.error.value"
      class="bg-destructive/5 text-destructive rounded-xl p-3 text-sm"
    >
      {{ operations.error.value }}
    </div>
    <div class="bg-card overflow-hidden rounded-2xl border shadow-sm">
      <div
        v-for="item in filtered"
        :key="item.id"
        class="flex items-start gap-4 border-b p-4 last:border-0"
      >
        <div class="bg-primary/10 text-primary grid size-9 place-items-center rounded-lg">
          <HistoryIcon class="size-4" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium">{{ item.action.replaceAll(".", " ") }}</p>
          <p class="text-muted-foreground mt-1 truncate text-xs">
            {{ item.resource }} · {{ item.resource_id }}
          </p>
        </div>
        <time class="text-muted-foreground text-xs">{{ formatDateTime(item.created_at) }}</time>
      </div>
      <div
        v-if="!operations.loading.value && filtered.length === 0"
        class="text-muted-foreground py-16 text-center text-sm"
      >
        No matching audit events.
      </div>
    </div>
  </div>
</template>
