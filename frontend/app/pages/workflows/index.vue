<script setup lang="ts">
  import {
    ArrowRightIcon,
    CheckCircle2Icon,
    Clock3Icon,
    RefreshCwIcon,
    RouteIcon,
  } from "@lucide/vue";
  import { computed, onMounted, shallowRef } from "vue";

  import type { DocumentRoute } from "@/types/api";
  import { formatDate } from "@/utils/format";

  definePageMeta({
    title: "Workflows",
    description: "Follow papers as they move through the office.",
    breadcrumbs: [{ label: "Office", to: "/" }, { label: "Workflows" }],
  });
  useHead({ title: "Workflows — Lumen DMS" });
  const workspace = workspaceService();
  const workflows = shallowRef<DocumentRoute[]>([]);
  const refresh = async () => {
    workflows.value = await workspace.workflows();
  };
  const active = computed(() =>
    workflows.value.filter((item) => !["completed", "rejected", "canceled"].includes(item.status)),
  );
  const completed = computed(() => workflows.value.filter((item) => item.status === "completed"));
  onMounted(refresh);
</script>
<template>
  <div class="space-y-5">
    <div class="grid gap-3 sm:grid-cols-3">
      <div class="bg-card rounded-xl border p-4 shadow-sm">
        <p class="text-muted-foreground text-xs font-medium tracking-wider uppercase">In motion</p>
        <p class="mt-2 text-3xl font-semibold">{{ active.length }}</p>
      </div>
      <div class="bg-card rounded-xl border p-4 shadow-sm">
        <p class="text-muted-foreground text-xs font-medium tracking-wider uppercase">Completed</p>
        <p class="mt-2 text-3xl font-semibold">{{ completed.length }}</p>
      </div>
      <div class="bg-primary text-primary-foreground rounded-xl border p-4 shadow-sm">
        <p class="text-xs font-medium tracking-wider uppercase opacity-70">All routes</p>
        <p class="mt-2 text-3xl font-semibold">{{ workflows.length }}</p>
      </div>
    </div>
    <div class="bg-card rounded-2xl border shadow-sm">
      <div class="flex items-center justify-between border-b p-4">
        <div>
          <h2 class="font-semibold">Paper trail</h2>
          <p class="text-muted-foreground text-sm">
            Live document routes and their current office stage.
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          class="gap-2"
          :disabled="workspace.loading.value"
          @click="refresh"
          ><RefreshCwIcon class="size-4" />Refresh</Button
        >
      </div>
      <div
        v-if="workspace.error.value"
        class="bg-destructive/5 text-destructive m-4 rounded-lg p-3 text-sm"
      >
        {{ workspace.error.value }}
      </div>
      <div v-if="workflows.length" class="divide-y">
        <NuxtLink
          v-for="item in workflows"
          :key="item.id"
          :to="`/document/${item.document_id}`"
          class="hover:bg-muted/35 flex items-center gap-4 p-4 transition"
          ><div
            class="grid size-10 place-items-center rounded-xl"
            :class="
              item.status === 'completed'
                ? 'bg-success/10 text-success'
                : 'bg-primary/10 text-primary'
            "
          >
            <CheckCircle2Icon v-if="item.status === 'completed'" class="size-5" /><RouteIcon
              v-else
              class="size-5"
            />
          </div>
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-semibold">{{ item.title }}</p>
            <p class="text-muted-foreground mt-1 text-xs">
              Step {{ item.current_step + 1 }} · Updated {{ formatDate(item.updated_at) }}
            </p>
          </div>
          <Badge variant="secondary" class="capitalize">{{
            item.status.replaceAll("_", " ")
          }}</Badge
          ><ArrowRightIcon class="text-muted-foreground size-4"
        /></NuxtLink>
      </div>
      <div v-else-if="!workspace.loading.value" class="py-20 text-center">
        <Clock3Icon class="text-muted-foreground/50 mx-auto size-8" />
        <h3 class="mt-3 font-semibold">No routes yet</h3>
        <p class="text-muted-foreground mt-1 text-sm">
          Open a document and route it to start its paper trail.
        </p>
      </div>
    </div>
  </div>
</template>
