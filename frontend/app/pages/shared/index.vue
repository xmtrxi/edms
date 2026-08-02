<script setup lang="ts">
  import DocumentExplorer from "@/components/documents/DocumentExplorer.vue";
  import { documents } from "@/data/mock-data";

  definePageMeta({
    title: "Shared with me",
    description: "Documents others have shared with you.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Shared" }],
  });

  useHead({
    title: "Shared with me — Lumen DMS",
    meta: [{ name: "description", content: "Documents shared with you by teammates." }],
  });
</script>

<template>
  <DocumentExplorer
    :docs="documents.filter((d) => d.shared)"
    empty-title="Nothing shared with you yet"
    empty-action="Browse documents"
    empty-link="/documents"
  />
</template>
