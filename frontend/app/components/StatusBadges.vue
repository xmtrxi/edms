<script setup lang="ts">
  import {
    CheckCircle2Icon,
    CloudIcon,
    Loader2Icon,
    ScanLineIcon,
    ShieldAlertIcon,
    SparklesIcon,
  } from "@lucide/vue";

  import { Badge } from "@/components/ui/badge";
  import { cn } from "@/lib/utils";

  const props = withDefaults(
    defineProps<{
      status?: "ready" | "processing" | "ocr" | "synced" | "error" | "review";
      source?: "sharepoint" | "local" | "upload";
      library?: string;
      class?: string;
    }>(),
    {
      status: "ready",
      source: "local",
    },
  );

  const statusMap: Record<string, { label: string; tone: string; Icon: typeof CheckCircle2Icon }> =
    {
      ready: { label: "Ready", tone: "bg-success/10 text-success", Icon: CheckCircle2Icon },
      synced: { label: "Synced", tone: "bg-info/10 text-info", Icon: CloudIcon },
      processing: { label: "Processing", tone: "bg-warning/10 text-warning", Icon: Loader2Icon },
      ocr: { label: "OCR", tone: "bg-primary/10 text-primary", Icon: ScanLineIcon },
      review: { label: "Needs review", tone: "bg-warning/10 text-warning", Icon: ShieldAlertIcon },
      error: { label: "Error", tone: "bg-destructive/10 text-destructive", Icon: ShieldAlertIcon },
    };

  const statusMeta = computed(() => statusMap[props.status]);
</script>

<template>
  <Badge
    v-if="props.status"
    variant="secondary"
    :class="cn('h-5 gap-1 border-0 px-1.5 text-[10px] font-medium', statusMeta.tone, props.class)"
  >
    <component
      :is="statusMeta.Icon"
      :class="cn('h-3 w-3', props.status === 'processing' && 'animate-spin')"
    />
    {{ statusMeta.label }}
  </Badge>
  <Badge
    v-if="props.source === 'sharepoint'"
    variant="outline"
    class="border-info/30 text-info bg-info/5 h-5 gap-1 px-1.5 text-[10px] font-medium"
  >
    <CloudIcon class="h-3 w-3" /> SharePoint{{ props.library ? ` · ${props.library}` : "" }}
  </Badge>
</template>
