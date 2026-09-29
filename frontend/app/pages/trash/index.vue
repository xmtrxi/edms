<script setup lang="ts">
  import { onMounted, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import type { Document } from "@/types/api";

  definePageMeta({
    title: "Trash",
    description: "Items here will be permanently deleted after 30 days.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Trash" }],
  });

  useHead({ title: "Trash — Lumen DMS" });
  const workspace = workspaceService();
  const documents = shallowRef<Document[]>([]);
  const refresh = async () => {
    documents.value = await workspace.trash();
  };
  const restore = async (id: string) => {
    await workspace.restore(id);
    if (!workspace.error.value) {
      toast.success("Document restored");
      await refresh();
    }
  };
  onMounted(refresh);
</script>

<template>
  <DocumentsLiveDocumentCollection
    :documents="documents"
    :loading="workspace.loading.value"
    :error="workspace.error.value"
    empty-title="Trash is empty"
    mode="trash"
    @refresh="refresh"
    @restore="restore"
  />
</template>
