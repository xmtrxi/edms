<script setup lang="ts">
  import { documents } from "@/data/mock-data";

  definePageMeta({
    title: "Trash",
    description: "Items here will be permanently deleted after 30 days.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Trash" }],
  });

  useHead({
    title: "Trash — Lumen DMS",
    meta: [{ name: "description", content: "Deleted documents, recoverable for 30 days." }],
  });
</script>

<template>
  <DocumentExplorer
    :docs="documents.filter((d) => d.trashed)"
    empty-title="Trash is empty"
    empty-action="Browse documents"
    empty-link="/documents"
  />
</template>
