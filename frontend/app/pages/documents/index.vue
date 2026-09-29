<script setup lang="ts">
  import { FolderPlusIcon, InboxIcon, UploadCloudIcon } from "@lucide/vue";
  import { computed } from "vue";

  import UploadDialog from "@/components/documents/UploadDialog.vue";
  import DriveBrowser from "@/components/drive/DriveBrowser.vue";
  import { Button } from "@/components/ui/button";

  definePageMeta({
    title: "Drive",
    description: "Your files and the organization’s folders you can open.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Drive" }],
  });
  useHead({ title: "Drive — Lumen DMS" });

  const route = useRoute();
  const router = useRouter();
  const uploadOpen = shallowRef(false);
  const browser = useTemplateRef<InstanceType<typeof DriveBrowser>>("browser");

  // The open folder lives in the URL so it survives reloads and can be linked.
  const folderId = computed(() => String(route.query.folder ?? ""));
  const navigate = (id: string) => router.push({ query: id ? { folder: id } : {} });
</script>

<template>
  <Teleport to="#page-actions-target">
    <Button
      v-if="folderId"
      variant="outline"
      size="sm"
      class="gap-1.5"
      @click="browser?.openFileRequest()"
    >
      <InboxIcon class="h-3.5 w-3.5" /> Request files
    </Button>
    <Button variant="outline" size="sm" class="gap-1.5" @click="browser?.openNewFolder()">
      <FolderPlusIcon class="h-3.5 w-3.5" /> New folder
    </Button>
    <Button size="sm" class="gap-1.5" @click="uploadOpen = true">
      <UploadCloudIcon class="h-4 w-4" /> Upload
    </Button>
  </Teleport>
  <DriveBrowser ref="browser" :folder-id="folderId" @navigate="navigate" />
  <UploadDialog v-model:open="uploadOpen" :initial-folder-id="folderId" />
</template>
