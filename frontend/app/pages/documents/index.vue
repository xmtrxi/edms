<script setup lang="ts">
  import { FolderPlusIcon, RefreshCwIcon, ScanSearchIcon, UploadCloudIcon } from "@lucide/vue";
  import { toast } from "vue-sonner";

  import DocumentsBrowser from "@/components/documents/DocumentsBrowser.vue";
  import UploadDialog from "@/components/documents/UploadDialog.vue";
  import { Button } from "@/components/ui/button";

  const uploadOpen = shallowRef(false);
  const syncing = shallowRef(false);
  const dryRunning = shallowRef(false);
  const browser = useTemplateRef<InstanceType<typeof DocumentsBrowser>>("browser");
  const service = documentsService();

  definePageMeta({
    title: "My Documents",
    description: "All documents you own.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "My Documents" }],
  });

  useHead({
    title: "My Documents — Lumen DMS",
    meta: [
      {
        name: "description",
        content: "Browse, filter, and preview all of your documents.",
      },
      { property: "og:title", content: "My Documents — Lumen DMS" },
    ],
  });

  const newFolder = () => {
    toast.info("Folder creation — coming soon");
  };

  const onUploaded = () => {
    browser.value?.refresh();
  };

  const syncStorage = async (dryRun = false) => {
    if (dryRun) dryRunning.value = true;
    else syncing.value = true;

    try {
      const result = await service.syncStorage({ dryRun });
      if (!result) {
        toast.error(service.error.value || "Storage sync failed");
        return;
      }

      if (dryRun) {
        const summary = `${result.scanned} provider file(s) checked — ${result.would_import ?? 0} new, ${result.skipped} already known`;
        if (result.failed) {
          toast.warning(
            `Dry run complete — ${summary}, ${result.failed} failed. Nothing was imported.`,
          );
        } else {
          toast.success(`Dry run complete — ${summary}. Nothing was imported or sent to OCR.`);
        }
        return;
      }

      browser.value?.refresh();
      if (result.failed) {
        toast.warning(
          `Sync imported ${result.imported} file(s), skipped ${result.skipped}, and failed ${result.failed}.`,
        );
      } else {
        toast.success(
          `Sync complete — ${result.imported} imported, ${result.skipped} already known.`,
        );
      }
    } finally {
      if (dryRun) dryRunning.value = false;
      else syncing.value = false;
    }
  };
</script>

<template>
  <Teleport to="#page-actions-target">
    <Button
      variant="outline"
      size="sm"
      class="gap-1.5"
      :disabled="syncing || dryRunning"
      title="Check provider files without importing or running OCR"
      @click="syncStorage(true)"
    >
      <ScanSearchIcon class="h-3.5 w-3.5" :class="dryRunning && 'animate-pulse'" />
      {{ dryRunning ? "Checking…" : "Dry run" }}
    </Button>
    <Button
      variant="outline"
      size="sm"
      class="gap-1.5"
      :disabled="syncing || dryRunning"
      @click="syncStorage(false)"
    >
      <RefreshCwIcon class="h-3.5 w-3.5" :class="syncing && 'animate-spin'" />
      {{ syncing ? "Syncing…" : "Sync storage" }}
    </Button>
    <Button variant="outline" size="sm" class="gap-1.5" @click="newFolder">
      <FolderPlusIcon class="h-3.5 w-3.5" /> New folder
    </Button>
    <Button size="sm" class="gap-1.5" @click="uploadOpen = true">
      <UploadCloudIcon class="h-4 w-4" /> Upload
    </Button>
  </Teleport>
  <DocumentsBrowser ref="browser" />
  <UploadDialog :open="uploadOpen" @update:open="uploadOpen = $event" @uploaded="onUploaded" />
</template>
