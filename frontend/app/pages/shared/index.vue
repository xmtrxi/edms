<script setup lang="ts">
  import { onMounted, shallowRef } from "vue";

  import type { Document } from "@/types/api";

  definePageMeta({
    title: "Shared with me",
    description: "Documents others have shared with you.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Shared" }],
  });

  useHead({ title: "Shared with me — Lumen DMS" });
  const workspace = workspaceService();
  const documents = shallowRef<Document[]>([]);
  const refresh = async () => {
    documents.value = await workspace.shared();
  };
  onMounted(refresh);
</script>

<template>
  <DocumentsLiveDocumentCollection
    :documents="documents"
    :loading="workspace.loading.value"
    :error="workspace.error.value"
    empty-title="Nothing has been shared with you"
    @refresh="refresh"
  />
</template>
