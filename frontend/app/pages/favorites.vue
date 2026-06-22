<script setup lang="ts">
  import { documents } from "@/data/mock-data";

  definePageMeta({
    title: "Favorites",
    description: "Starred for quick access.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Favorites" }],
  });

  useHead({
    title: "Favorites — Lumen DMS",
    meta: [{ name: "description", content: "Documents you have starred for quick access." }],
  });
</script>

<template>
  <DocumentExplorer
    :docs="documents.filter((d) => d.starred)"
    empty-title="No favorites yet"
    empty-action="Browse documents"
    empty-link="/documents"
  />
</template>
