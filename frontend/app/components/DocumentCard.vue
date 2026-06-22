<script setup lang="ts">
  import { EyeIcon, MoreHorizontalIcon, StarIcon } from "@lucide/vue";

  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Button } from "@/components/ui/button";
  import type { Doc } from "@/data/mock-data";
  import { cn } from "@/lib/utils";

  const props = defineProps<{
    doc: Doc;
  }>();

  const emit = defineEmits<{
    navigate: [doc: Doc];
    "quick-view": [doc: Doc];
  }>();
</script>

<template>
  <button
    @click="emit('navigate', doc)"
    :class="
      cn(
        'group bg-card shadow-elegant hover:shadow-glow hover:border-primary/30 rounded-2xl border p-4 text-left transition-all hover:-translate-y-0.5',
        'border-border',
      )
    "
  >
    <div class="flex items-start justify-between">
      <FileTypeIcon :type="doc.type" size="lg" />
      <div class="flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
        <StarIcon v-if="doc.starred" class="fill-warning text-warning h-3.5 w-3.5" />
        <Button
          variant="ghost"
          size="icon"
          class="h-7 w-7"
          @click.stop="emit('quick-view', doc)"
          aria-label="Quick view"
        >
          <EyeIcon class="h-3.5 w-3.5" />
        </Button>
        <Button variant="ghost" size="icon" class="h-7 w-7" @click.stop aria-label="More">
          <MoreHorizontalIcon class="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>

    <div class="mt-3">
      <p class="text-foreground truncate text-sm font-medium">{{ doc.name }}</p>
      <p class="text-muted-foreground mt-0.5 text-xs">{{ doc.size }} · v{{ doc.versions }}</p>
    </div>

    <div class="mt-3 flex flex-wrap items-center gap-1">
      <StatusBadges :status="doc.status" />
      <StatusBadges :source="doc.source" :library="doc.library" />
    </div>

    <div class="mt-4 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <Avatar class="h-6 w-6">
          <AvatarFallback class="bg-secondary text-[10px]">{{ doc.ownerInitials }}</AvatarFallback>
        </Avatar>
        <span class="text-muted-foreground truncate text-xs">{{ doc.modified }}</span>
      </div>
    </div>
  </button>
</template>
