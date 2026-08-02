<script setup lang="ts">
  import DocumentsBrowser from "@/components/documents/DocumentsBrowser.vue";

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
  <DocumentsBrowser :show-status-filter="false" />
</template>
