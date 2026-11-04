<script setup lang="ts">
  import { DatabaseIcon, RefreshCwIcon, ScanSearchIcon } from "@lucide/vue";
  import { shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import { Button } from "@/components/ui/button";

  const service = documentsService();
  const running = shallowRef<"" | "dry" | "sync">("");

  // Imports files that were put into storage outside the app. Self-hosted only.
  const run = async (dryRun: boolean) => {
    running.value = dryRun ? "dry" : "sync";
    const result = await service.syncStorage({ dryRun });
    running.value = "";
    if (!result) {
      toast.error(service.error.value || "Storage sync failed");
      return;
    }
    if (dryRun) {
      toast.success(
        `Checked ${result.scanned} files: ${result.would_import ?? 0} new, ${result.skipped} already known${result.failed ? `, ${result.failed} failed` : ""}. Nothing was imported.`,
      );
      return;
    }
    const message = `Imported ${result.imported}, skipped ${result.skipped}${result.failed ? `, ${result.failed} failed` : ""}.`;
    if (result.failed) toast.warning(message);
    else toast.success(message);
  };
</script>

<template>
  <section class="bg-card rounded-2xl border p-6 shadow-sm">
    <h2 class="flex items-center gap-2 text-lg font-semibold">
      <DatabaseIcon class="size-5" /> Storage sync
    </h2>
    <p class="text-muted-foreground mt-1 text-sm">
      Import files that were placed in storage outside the app. Run a dry run first to see what
      would be imported.
    </p>
    <div class="mt-4 flex flex-wrap gap-2">
      <Button variant="outline" :disabled="Boolean(running)" @click="run(true)">
        <ScanSearchIcon class="size-4" :class="running === 'dry' && 'animate-pulse'" />
        {{ running === "dry" ? "Checking…" : "Dry run" }}
      </Button>
      <Button :disabled="Boolean(running)" @click="run(false)">
        <RefreshCwIcon class="size-4" :class="running === 'sync' && 'animate-spin'" />
        {{ running === "sync" ? "Syncing…" : "Sync storage" }}
      </Button>
    </div>
  </section>
</template>
