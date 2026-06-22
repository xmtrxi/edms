<script setup lang="ts">
  import { documents } from "@/data/mock-data";

  definePageMeta({
    title: "Archive",
    description: "Documents kept for compliance and long-term reference.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Archive" }],
  });

  useHead({
    title: "Archive — Lumen DMS",
    meta: [{ name: "description", content: "Documents archived from active workspaces." }],
  });

  const archived = documents.filter((d) => d.archived).concat(documents.slice(14, 18));
</script>

<template>
  <DocumentExplorer
    :docs="archived"
    empty-title="Nothing archived yet"
    empty-action="Browse documents"
    empty-link="/documents"
  />
</template>
