<script setup lang="ts">
  import { Toaster } from "@/components/ui/sonner";
</script>

<template>
  <NuxtLoadingIndicator :height="3" :color="'var(--primary)'" />
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
  <Toaster />
</template>
