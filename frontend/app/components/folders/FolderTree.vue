<script setup lang="ts">
  import { ChevronRightIcon, FolderIcon, FolderOpenIcon, PlusIcon } from "@lucide/vue";

  import { Button } from "@/components/ui/button";
  import { cn } from "@/lib/utils";
  import type { ManagedFolder } from "@/types/api";

  const props = defineProps<{ folders: ManagedFolder[]; selectedId?: string; loading?: boolean }>();
  const emit = defineEmits<{ select: [folder: ManagedFolder]; create: [] }>();

  const orderedFolders = computed(() =>
    [...props.folders].sort((a, b) => a.depth - b.depth || a.name.localeCompare(b.name)),
  );
</script>

<template>
  <aside class="border-border bg-card flex min-h-0 flex-col border-r">
    <div class="border-border flex items-center justify-between border-b px-4 py-3">
      <div>
        <p class="text-sm font-semibold">Folder structure</p>
        <p class="text-muted-foreground text-xs">{{ folders.length }} folders</p>
      </div>
      <Button variant="ghost" size="icon" aria-label="Create folder" @click="emit('create')">
        <PlusIcon class="size-4" />
      </Button>
    </div>
    <div class="min-h-0 overflow-y-auto p-2">
      <p v-if="loading" class="text-muted-foreground px-2 py-3 text-sm">Loading folders…</p>
      <p v-else-if="!folders.length" class="text-muted-foreground px-2 py-3 text-sm">
        No folders yet.
      </p>
      <button
        v-for="folder in orderedFolders"
        :key="folder.id"
        type="button"
        :style="{ paddingLeft: `${12 + folder.depth * 16}px` }"
        :class="
          cn(
            'hover:bg-accent flex w-full items-center gap-2 rounded-md py-2 pr-2 text-left text-sm transition-colors',
            selectedId === folder.id && 'bg-accent text-accent-foreground font-medium',
          )
        "
        @click="emit('select', folder)"
      >
        <ChevronRightIcon v-if="folder.depth" class="text-muted-foreground size-3 shrink-0" />
        <component
          :is="selectedId === folder.id ? FolderOpenIcon : FolderIcon"
          class="size-4 shrink-0"
          :style="{ color: folder.color || undefined }"
        />
        <span class="truncate">{{ folder.name }}</span>
        <span class="text-muted-foreground ml-auto text-xs tabular-nums">{{
          folder.document_count
        }}</span>
      </button>
    </div>
  </aside>
</template>
