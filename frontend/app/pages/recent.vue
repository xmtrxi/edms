<script setup lang="ts">
  import { documents } from "@/data/mock-data";

  definePageMeta({
    title: "Recent",
    description: "The last 30 days of activity across your workspace.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Recent" }],
  });

  useHead({
    title: "Recent — Lumen DMS",
    meta: [{ name: "description", content: "Recently opened and uploaded documents." }],
  });
</script>

<template>
  <DocumentExplorer
    :docs="documents.slice(0, 12)"
    empty-title="No recent activity"
    empty-action="Browse documents"
    empty-link="/documents"
  />
</template>
