<script setup lang="ts">
  import { FileQuestionIcon, AlertTriangleIcon, RefreshCwIcon, ArrowLeftIcon } from "@lucide/vue";

  import { useNuxtApp } from "#app";
  import { Button } from "@/components/ui/button";

  const props = defineProps<{
    error: { statusCode?: number; message?: string; statusMessage?: string };
  }>();

  const is404 = props.error?.statusCode === 404;
  const code = props.error?.statusCode ?? 500;
  const title = is404 ? "Page not found" : "Something went wrong";
  const description = is404
    ? "The page you're looking for doesn't exist or has been moved."
    : (props.error?.message ?? props.error?.statusMessage ?? "An unexpected error occurred.");

  function goHome() {
    const { $router } = useNuxtApp();
    $router.push("/");
  }

  function reload() {
    window.location.reload();
  }
</script>

<template>
  <div class="bg-background flex min-h-screen flex-col items-center justify-center p-6">
    <div class="flex max-w-md flex-col items-center text-center">
      <div
        class="border-border bg-card shadow-elegant mb-6 grid h-20 w-20 place-items-center rounded-2xl border"
      >
        <FileQuestionIcon v-if="is404" class="text-muted-foreground h-10 w-10" />
        <AlertTriangleIcon v-else class="text-destructive h-10 w-10" />
      </div>
      <span class="text-foreground/10 text-5xl font-bold tracking-tight">{{ code }}</span>
      <h1 class="mt-2 text-xl font-semibold tracking-tight">{{ title }}</h1>
      <p class="text-muted-foreground mt-2 text-sm leading-relaxed">{{ description }}</p>
      <div class="mt-6 flex items-center gap-3">
        <Button variant="outline" class="gap-2" @click="goHome">
          <ArrowLeftIcon class="h-4 w-4" /> Go home
        </Button>
        <Button variant="outline" class="gap-2" @click="reload">
          <RefreshCwIcon class="h-4 w-4" /> Try again
        </Button>
      </div>
    </div>
  </div>
</template>
