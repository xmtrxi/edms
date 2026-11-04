<script setup lang="ts">
  import { CloudIcon } from "@lucide/vue";

  import { Badge } from "@/components/ui/badge";
  import { cn } from "@/lib/utils";
  import { getStatusMeta } from "@/utils/status";

  const props = withDefaults(
    defineProps<{
      status?: string;
      source?: "sharepoint" | "local" | "upload";
      library?: string;
      class?: string;
    }>(),
    {
      status: "ready",
      source: "local",
    },
  );

  const statusMeta = computed(() => getStatusMeta(props.status));
</script>

<template>
  <Badge
    v-if="props.status"
    variant="secondary"
    :class="cn('h-5 gap-1 border-0 px-1.5 text-[10px] font-medium', statusMeta.tone, props.class)"
  >
    <component :is="statusMeta.Icon" :class="cn('h-3 w-3', statusMeta.spin && 'animate-spin')" />
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
