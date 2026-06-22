<script setup lang="ts">
  import { FolderPlusIcon, UploadCloudIcon } from "@lucide/vue";
  import { ref } from "vue";
  import { useRouter } from "vue-router";
  import { toast } from "vue-sonner";

  import { Button } from "@/components/ui/button";
  import UploadDialog from "@/components/UploadDialog.vue";

  const router = useRouter();
  const uploadOpen = ref(false);

  definePageMeta({
    title: "My Documents",
    description: "All documents you own, with AI tags and SharePoint sync.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "My Documents" }],
  });

  useHead({
    title: "My Documents — Lumen DMS",
    meta: [
      {
        name: "description",
        content:
          "Browse, filter, and preview all of your documents with AI-assisted classification.",
      },
      { property: "og:title", content: "My Documents — Lumen DMS" },
    ],
  });

  function newFolder() {
    toast.info("Folder creation — picker coming soon");
  }
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
  <DocumentExplorer />
  <UploadDialog :open="uploadOpen" @update:open="uploadOpen = $event" />
</template>
