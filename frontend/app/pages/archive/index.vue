<script setup lang="ts">
  import DocumentsBrowser from "@/components/documents/DocumentsBrowser.vue";

  definePageMeta({
    title: "Archive",
    description: "Documents kept for compliance and long-term reference.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Archive" }],
  });

  useHead({
    title: "Archive — Lumen DMS",
    meta: [{ name: "description", content: "Documents archived from active workspaces." }],
  });
</script>

<template>
  <DocumentsBrowser initial-status="archived" :show-status-filter="false" :show-search="true" />
</template>
