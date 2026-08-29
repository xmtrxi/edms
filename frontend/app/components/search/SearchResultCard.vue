<script setup lang="ts">
  import { CalendarClockIcon, FileSearchIcon } from "lucide-vue-next";

  import FileTypeIcon from "@/components/FileTypeIcon.vue";
  import StatusBadges from "@/components/StatusBadges.vue";
  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import type { SearchResultDocument } from "@/types/api";
  import { formatBytes, formatDate, fileTypeFromMime, initialsOf } from "@/utils/format";

  defineProps<{
    document: SearchResultDocument;
  }>();

  const emit = defineEmits<{
    open: [id: string];
  }>();
</script>

<template>
  <article
    class="border-border bg-card hover:border-primary/35 focus-visible:ring-ring group cursor-pointer rounded-xl border p-4 transition-colors focus-visible:ring-2 focus-visible:outline-none"
    tabindex="0"
    role="button"
    @click="emit('open', document.id)"
    @keydown.enter.prevent="emit('open', document.id)"
    @keydown.space.prevent="emit('open', document.id)"
  >
    <div class="flex gap-3">
      <FileTypeIcon :type="fileTypeFromMime(document.mime_type)" size="md" class="mt-0.5" />
      <div class="min-w-0 flex-1">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <h3 class="text-foreground group-hover:text-primary truncate text-sm font-semibold">
            {{ document.name }}
          </h3>
          <StatusBadges :status="document.status" />
        </div>

        <p
          v-if="document.snippet"
          class="text-foreground/80 mt-2 line-clamp-3 max-w-3xl text-sm leading-6"
        >
          {{ document.snippet }}
        </p>
        <p v-else class="text-muted-foreground mt-2 flex items-center gap-1.5 text-xs">
          <FileSearchIcon class="h-3.5 w-3.5" />
          Matched by document metadata.
        </p>

        <div class="mt-3 flex flex-wrap items-center gap-2 text-[11px]">
          <Badge variant="secondary" class="h-5 border-0 px-1.5">
            {{ formatBytes(document.size) }}
          </Badge>
          <span class="text-muted-foreground flex items-center gap-1">
            <CalendarClockIcon class="h-3 w-3" />
            {{ formatDate(document.updated_at) }}
          </span>
          <span class="text-muted-foreground flex min-w-0 items-center gap-1">
            <Avatar class="h-4 w-4">
              <AvatarFallback class="bg-secondary text-[7px]">
                {{ initialsOf(document.owner?.name) }}
              </AvatarFallback>
            </Avatar>
            <span class="truncate">{{ document.owner?.name ?? "Unknown" }}</span>
          </span>
          <span class="text-muted-foreground/70 truncate">{{ document.original_name }}</span>
        </div>
      </div>
    </div>
  </article>
</template>
