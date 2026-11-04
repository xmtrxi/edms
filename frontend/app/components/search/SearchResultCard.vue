<script setup lang="ts">
  import {
    CalendarClockIcon,
    ChevronDownIcon,
    FileSearchIcon,
    Layers3Icon,
    Loader2Icon,
    MapPinIcon,
  } from "lucide-vue-next";
  import { shallowRef } from "vue";

  import FileTypeIcon from "@/components/FileTypeIcon.vue";
  import StatusBadges from "@/components/StatusBadges.vue";
  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import type { SearchResultDocument } from "@/types/api";
  import { formatBytes, formatDate, fileTypeFromMime, initialsOf } from "@/utils/format";

  const props = defineProps<{
    document: SearchResultDocument;
    passages?: SearchResultDocument[];
    passagesLoading?: boolean;
  }>();

  const emit = defineEmits<{
    open: [id: string];
    expand: [id: string];
  }>();

  const expanded = shallowRef(false);

  const pageLabel = (item: SearchResultDocument): string => {
    if (!item.page_start) return "Page unavailable";
    return item.page_end && item.page_end !== item.page_start
      ? `Pages ${item.page_start}–${item.page_end}`
      : `Page ${item.page_start}`;
  };

  const togglePassages = (): void => {
    expanded.value = !expanded.value;
    if (expanded.value && props.passages === undefined) emit("expand", props.document.id);
  };
</script>

<template>
  <article
    class="border-border bg-card hover:border-primary/35 group rounded-xl border p-4 transition-colors"
  >
    <div class="flex gap-3">
      <FileTypeIcon :type="fileTypeFromMime(document.mime_type)" size="md" class="mt-0.5" />
      <div class="min-w-0 flex-1">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <button
            type="button"
            class="text-foreground hover:text-primary truncate text-left text-sm font-semibold"
            @click="emit('open', document.id)"
          >
            {{ document.name }}
          </button>
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

        <div v-if="document.snippet" class="mt-2 flex flex-wrap items-center gap-2 text-[11px]">
          <span class="text-primary flex items-center gap-1 font-medium">
            <MapPinIcon class="h-3 w-3" />
            {{ pageLabel(document) }}
          </span>
          <span v-if="document.headings?.length" class="text-muted-foreground truncate">
            {{ document.headings.join(" › ") }}
          </span>
        </div>

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

        <button
          type="button"
          class="text-muted-foreground hover:text-foreground mt-3 flex items-center gap-1.5 text-xs font-medium"
          :aria-expanded="expanded"
          @click="togglePassages"
        >
          <Loader2Icon v-if="passagesLoading" class="h-3.5 w-3.5 animate-spin" />
          <Layers3Icon v-else class="h-3.5 w-3.5" />
          {{ expanded ? "Hide matching passages" : "Show matching passages" }}
          <ChevronDownIcon
            class="h-3.5 w-3.5 transition-transform"
            :class="expanded && 'rotate-180'"
          />
        </button>

        <div v-if="expanded" class="border-border mt-3 space-y-2 border-l pl-3">
          <p v-if="passagesLoading" class="text-muted-foreground py-2 text-xs">Finding passages…</p>
          <button
            v-for="passage in passages"
            :key="`${passage.version_id}-${passage.chunk_index}`"
            type="button"
            class="hover:bg-muted/60 block w-full rounded-md px-2 py-2 text-left"
            @click="emit('open', document.id)"
          >
            <span class="text-primary text-[11px] font-medium">{{ pageLabel(passage) }}</span>
            <span v-if="passage.headings?.length" class="text-muted-foreground ml-2 text-[11px]">
              {{ passage.headings.join(" › ") }}
            </span>
            <span class="text-foreground/75 mt-1 line-clamp-2 block text-xs leading-5">
              {{ passage.snippet }}
            </span>
          </button>
          <p v-if="passages && passages.length === 0" class="text-muted-foreground py-2 text-xs">
            No additional matching passages.
          </p>
        </div>
      </div>
    </div>
  </article>
</template>
