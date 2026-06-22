<script setup lang="ts">
  import {
    FileTextIcon,
    FileSpreadsheetIcon,
    FileImageIcon,
    PresentationIcon,
    FileType2Icon,
  } from "@lucide/vue";

  import { cn } from "@/lib/utils";

  const props = withDefaults(
    defineProps<{
      type: "pdf" | "docx" | "xlsx" | "pptx" | "image" | "txt";
      size?: "sm" | "md" | "lg";
      class?: string;
    }>(),
    {
      size: "md",
    },
  );

  const map: Record<string, { Icon: typeof FileTextIcon; tone: string }> = {
    pdf: { Icon: FileTextIcon, tone: "bg-rose-500/10 text-rose-500" },
    docx: { Icon: FileType2Icon, tone: "bg-blue-500/10 text-blue-500" },
    xlsx: { Icon: FileSpreadsheetIcon, tone: "bg-emerald-500/10 text-emerald-500" },
    pptx: { Icon: PresentationIcon, tone: "bg-orange-500/10 text-orange-500" },
    image: { Icon: FileImageIcon, tone: "bg-violet-500/10 text-violet-500" },
    txt: { Icon: FileTextIcon, tone: "bg-muted-foreground/10 text-muted-foreground" },
  };

  const meta = computed(() => map[props.type]);
  const sz = computed(() =>
    props.size === "sm" ? "h-7 w-7" : props.size === "lg" ? "h-14 w-14" : "h-9 w-9",
  );
  const ic = computed(() =>
    props.size === "sm" ? "h-3.5 w-3.5" : props.size === "lg" ? "h-6 w-6" : "h-4 w-4",
  );
</script>

<template>
  <div :class="cn('grid shrink-0 place-items-center rounded-lg', sz, meta.tone, props.class)">
    <component :is="meta.Icon" :class="ic" />
  </div>
</template>
