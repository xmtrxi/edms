<script setup lang="ts">
  import {
    CheckCircle2Icon,
    CloudIcon,
    RefreshCwIcon,
    ShieldCheckIcon,
    XCircleIcon,
  } from "@lucide/vue";
  import { onMounted, shallowRef } from "vue";

  definePageMeta({
    title: "Storage connection",
    description: "Monitor the office document store.",
    breadcrumbs: [{ label: "Administration", to: "/settings" }, { label: "Storage" }],
  });
  useHead({ title: "Storage connection — Lumen DMS" });
  const workspace = workspaceService();
  const status = shallowRef<{
    provider: string;
    configured: boolean;
    healthy: boolean;
    error?: string;
  } | null>(null);
  const refresh = async () => {
    status.value = await workspace.storageStatus();
  };
  onMounted(refresh);
</script>
<template>
  <div class="mx-auto max-w-3xl space-y-5">
    <section class="bg-card overflow-hidden rounded-2xl border shadow-sm">
      <div class="via-card to-card bg-gradient-to-br from-sky-500/15 p-7">
        <div class="flex items-center gap-4">
          <div
            class="grid size-14 place-items-center rounded-2xl bg-sky-500 text-white shadow-lg shadow-sky-500/20"
          >
            <CloudIcon class="size-7" />
          </div>
          <div>
            <p class="text-xs font-semibold tracking-widest text-sky-600 uppercase">
              Document storage
            </p>
            <h1 class="mt-1 text-2xl font-semibold">
              {{ status?.provider || "Checking connection…" }}
            </h1>
          </div>
        </div>
      </div>
      <div class="grid gap-4 border-t p-6 sm:grid-cols-2">
        <div class="rounded-xl border p-4">
          <p class="text-muted-foreground text-xs">Configuration</p>
          <div class="mt-2 flex items-center gap-2 font-medium">
            <CheckCircle2Icon v-if="status?.configured" class="text-success size-4" /><XCircleIcon
              v-else
              class="text-destructive size-4"
            />{{ status?.configured ? "Configured" : "Needs configuration" }}
          </div>
        </div>
        <div class="rounded-xl border p-4">
          <p class="text-muted-foreground text-xs">Health</p>
          <div class="mt-2 flex items-center gap-2 font-medium">
            <ShieldCheckIcon v-if="status?.healthy" class="text-success size-4" /><XCircleIcon
              v-else
              class="text-destructive size-4"
            />{{ status?.healthy ? "Connected" : "Unavailable" }}
          </div>
        </div>
      </div>
      <div
        v-if="status?.error || workspace.error.value"
        class="border-destructive/25 bg-destructive/5 text-destructive mx-6 mb-6 rounded-xl border p-4 text-sm"
      >
        {{ status?.error || workspace.error.value }}
      </div>
    </section>
    <div class="flex justify-end">
      <Button variant="outline" class="gap-2" :disabled="workspace.loading.value" @click="refresh"
        ><RefreshCwIcon class="size-4" />Run health check</Button
      >
    </div>
  </div>
</template>
