<script setup lang="ts">
  import { CloudDownloadIcon, HistoryIcon, RotateCcwIcon, UsersIcon } from "@lucide/vue";

  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import type { DocumentContributors, DocumentVersion } from "@/types/api";
  import { formatBytes, formatDateTime, initialsOf } from "@/utils/format";

  defineProps<{
    versions: DocumentVersion[];
    contributors: DocumentContributors | null;
    currentVersionId?: string;
    busy?: boolean;
  }>();

  const emit = defineEmits<{
    download: [version: DocumentVersion];
    restore: [version: DocumentVersion];
  }>();
</script>

<template>
  <section class="border-border bg-card rounded-xl border p-5">
    <div class="mb-4 flex flex-wrap items-center gap-2">
      <HistoryIcon class="text-primary h-4 w-4" />
      <h3 class="text-sm font-semibold">Version history</h3>
      <div class="ml-auto flex items-center gap-2">
        <Badge variant="secondary" class="gap-1 border-0 text-[10px]">
          <UsersIcon class="h-3 w-3" />
          {{ contributors?.unique_editor_count ?? 0 }} editors
        </Badge>
        <Badge variant="secondary" class="border-0 text-[10px]">
          {{ contributors?.edit_count ?? 0 }} edits
        </Badge>
      </div>
    </div>

    <div v-if="versions.length" class="space-y-2">
      <article
        v-for="version in versions"
        :key="version.id"
        class="border-border flex flex-col gap-3 rounded-lg border p-3 sm:flex-row sm:items-center"
      >
        <div
          class="bg-primary/10 text-primary flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-xs font-semibold"
        >
          v{{ version.version }}
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <p class="truncate text-xs font-medium">{{ version.change_summary }}</p>
            <Badge v-if="version.id === currentVersionId" class="text-[9px]">Current</Badge>
            <Badge variant="outline" class="text-[9px]">{{
              version.source.replaceAll("_", " ")
            }}</Badge>
          </div>
          <p class="text-muted-foreground mt-1 text-[11px]">
            {{ version.creator?.name ?? "System" }} · {{ formatDateTime(version.created_at) }} ·
            {{ formatBytes(version.size) }}
          </p>
        </div>
        <div class="flex shrink-0 gap-1.5">
          <Button
            size="sm"
            variant="ghost"
            class="h-8 gap-1 text-xs"
            @click="emit('download', version)"
          >
            <CloudDownloadIcon class="h-3.5 w-3.5" /> Download
          </Button>
          <Button
            v-if="version.id !== currentVersionId"
            size="sm"
            variant="outline"
            class="h-8 gap-1 text-xs"
            :disabled="busy"
            @click="emit('restore', version)"
          >
            <RotateCcwIcon class="h-3.5 w-3.5" /> Restore
          </Button>
        </div>
      </article>
    </div>
    <p v-else class="text-muted-foreground rounded-lg border border-dashed p-6 text-center text-xs">
      No version history is available yet.
    </p>

    <div v-if="contributors?.contributors.length" class="border-border mt-4 border-t pt-4">
      <p class="text-muted-foreground mb-2 text-[10px] font-semibold tracking-wider uppercase">
        Contributors
      </p>
      <div class="flex flex-wrap gap-2">
        <div
          v-for="contributor in contributors.contributors"
          :key="contributor.user.id"
          class="bg-secondary/50 flex items-center gap-2 rounded-full px-2 py-1"
        >
          <Avatar class="h-5 w-5"
            ><AvatarFallback class="text-[8px]">{{
              initialsOf(contributor.user.name)
            }}</AvatarFallback></Avatar
          >
          <span class="text-[11px] font-medium">{{ contributor.user.name }}</span>
          <span class="text-muted-foreground text-[10px]">{{ contributor.versions_created }}</span>
        </div>
      </div>
    </div>
  </section>
</template>
