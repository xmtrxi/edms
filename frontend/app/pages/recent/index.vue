<script setup lang="ts">
  import DocumentsBrowser from "@/components/documents/DocumentsBrowser.vue";

  definePageMeta({
    title: "Recent",
    description: "The last 30 days of activity across your workspace.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Recent" }],
  });

  useHead({
    title: "Recent",
    meta: [{ name: "description", content: "Documents changed in the last 30 days." }],
  });
</script>

<template>
  <DocumentsBrowser :show-status-filter="false" :recent-days="30" />
</template>
