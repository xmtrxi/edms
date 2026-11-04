<script setup lang="ts">
  import {
    ArrowUpRightIcon,
    CheckCheckIcon,
    FilesIcon,
    InboxIcon,
    ScanLineIcon,
  } from "@lucide/vue";
  import { computed } from "vue";

  const props = defineProps<{
    totalDocuments: number;
    readyCount: number;
    processingCount: number;
    openSubmissions: number;
    loading: boolean;
  }>();
  const stats = computed(() => [
    {
      label: "Records indexed",
      value: props.totalDocuments,
      Icon: FilesIcon,
      detail: "Across the office library",
    },
    {
      label: "Ready to use",
      value: props.readyCount,
      Icon: CheckCheckIcon,
      detail: "Available in this view",
    },
    {
      label: "Being processed",
      value: props.processingCount,
      Icon: ScanLineIcon,
      detail: "Upload and OCR pipeline",
    },
    {
      label: "My open requests",
      value: props.openSubmissions,
      Icon: InboxIcon,
      detail: "Awaiting action or changes",
    },
  ]);
</script>

<template>
  <section
    class="office-overview bg-primary text-primary-foreground relative overflow-hidden rounded-2xl px-6 py-7 sm:px-8 sm:py-9"
  >
    <div class="relative z-10 flex flex-col gap-8 lg:flex-row lg:items-end lg:justify-between">
      <div class="max-w-2xl">
        <h2 class="text-3xl font-semibold tracking-[-0.035em] text-balance sm:text-4xl">
          Every file has a place. Every request has a next step.
        </h2>
        <p class="text-primary-foreground/72 mt-4 max-w-[65ch] text-sm leading-6">
          This desk reflects the live API. Open a record, invite a visitor from Forms, or pick up
          work that has returned to you.
        </p>
      </div>
      <NuxtLink to="/forms" class="group flex shrink-0 items-center gap-3 text-sm font-medium"
        >Prepare visitor intake
        <ArrowUpRightIcon
          class="size-4 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5"
      /></NuxtLink>
    </div>
    <dl
      class="bg-primary-foreground/15 relative z-10 mt-9 grid gap-px overflow-hidden rounded-xl sm:grid-cols-2 xl:grid-cols-4"
    >
      <div v-for="stat in stats" :key="stat.label" class="bg-primary px-5 py-5">
        <div class="flex items-center justify-between gap-3">
          <dt class="text-primary-foreground/65 text-xs">{{ stat.label }}</dt>
          <component :is="stat.Icon" class="text-primary-foreground/55 size-4" />
        </div>
        <dd class="mt-2 font-mono text-3xl font-medium tabular-nums">
          {{ loading ? "—" : stat.value.toLocaleString() }}
        </dd>
        <p class="text-primary-foreground/50 mt-1 text-[11px]">{{ stat.detail }}</p>
      </div>
    </dl>
  </section>
</template>
