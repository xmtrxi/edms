<script setup lang="ts">
  import { onMounted, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import type { Document } from "@/types/api";

  definePageMeta({
    title: "Favorites",
    description: "Starred for quick access.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Favorites" }],
  });

  useHead({ title: "Favorites — Lumen DMS" });
  const workspace = workspaceService();
  const documents = shallowRef<Document[]>([]);
  const refresh = async () => {
    documents.value = await workspace.favorites();
  };
  const trash = async (id: string) => {
    await workspace.moveToTrash(id);
    if (!workspace.error.value) {
      toast.success("Document moved to trash");
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
    empty-title="No favorites yet"
    @refresh="refresh"
    @trash="trash"
  />
</template>
