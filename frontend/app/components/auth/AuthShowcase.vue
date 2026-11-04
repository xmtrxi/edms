<script setup lang="ts">
  import {
    FileTextIcon,
    PenLineIcon,
    RouteIcon,
    ScanTextIcon,
    SearchIcon,
    ShieldCheckIcon,
    SparklesIcon,
  } from "@lucide/vue";
  import { computed } from "vue";

  const brand = useBrand();
  const features = useFeatures();

  // Only what this installation has switched on; routing is always there.
  const highlights = computed(() => {
    const on = features.value;
    return [
      on?.ai && { icon: SparklesIcon, title: "AI classification", detail: "Auto-tag & summarize" },
      on?.ocr && { icon: ScanTextIcon, title: "Text recognition", detail: "Scans become text" },
      on?.search && { icon: SearchIcon, title: "Full-text search", detail: "Find any word" },
      on?.esign && { icon: PenLineIcon, title: "E-signatures", detail: "Sign without printing" },
      on?.malware_scan && {
        icon: ShieldCheckIcon,
        title: "Virus scanning",
        detail: "Every upload",
      },
      { icon: RouteIcon, title: "Smart routing", detail: "Approval workflows" },
    ]
      .filter((item) => Boolean(item))
      .slice(0, 3) as Array<{ icon: typeof FileTextIcon; title: string; detail: string }>;
  });
</script>

<template>
  <div
    class="from-primary/5 via-primary/[0.02] to-background relative hidden flex-1 items-center justify-center overflow-hidden bg-gradient-to-br lg:flex"
  >
    <div
      class="from-primary/10 absolute inset-0 bg-[radial-gradient(ellipse_at_top_right,_var(--tw-gradient-stops))] via-transparent to-transparent"
    />
    <div class="relative max-w-md px-8 text-center">
      <div
        class="bg-primary/10 text-primary ring-primary/20 mx-auto mb-6 grid h-20 w-20 place-items-center rounded-2xl ring-1"
      >
        <FileTextIcon class="h-10 w-10" />
      </div>
      <h2 class="text-3xl font-bold tracking-tight">{{ brand }}</h2>
      <p class="text-muted-foreground mt-3 leading-relaxed">
        One workspace for capturing, classifying, routing, and retrieving your documents.
      </p>
      <div v-if="features" class="mt-8 grid grid-cols-3 gap-4 text-left">
        <div
          v-for="item in highlights"
          :key="item.title"
          class="border-border bg-card/60 rounded-xl border p-3"
        >
          <component :is="item.icon" class="text-primary mb-1 h-5 w-5" />
          <p class="text-xs font-medium">{{ item.title }}</p>
          <p class="text-muted-foreground mt-0.5 text-[10px]">{{ item.detail }}</p>
        </div>
      </div>
    </div>
  </div>
</template>
