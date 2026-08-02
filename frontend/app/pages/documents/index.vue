<script setup lang="ts">
  import { FolderPlusIcon, UploadCloudIcon } from "@lucide/vue";
  import { ref } from "vue";
  import { toast } from "vue-sonner";

  import DocumentsBrowser from "@/components/documents/DocumentsBrowser.vue";
  import UploadDialog from "@/components/documents/UploadDialog.vue";
  import { Button } from "@/components/ui/button";

  const uploadOpen = ref(false);
  const browser = ref<InstanceType<typeof DocumentsBrowser> | null>(null);

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
</script>

<template>
  <Teleport to="#page-actions-target">
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
