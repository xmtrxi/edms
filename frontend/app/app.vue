<script setup lang="ts">
  import { Toaster } from "@/components/ui/sonner";

  const brand = useBrand();
  useHead({
    titleTemplate: (title) => (title ? `${title} — ${brand.value}` : brand.value),
  });
</script>

<template>
  <NuxtLoadingIndicator :height="3" :color="'var(--primary)'" />
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
  <Toaster />
</template>
