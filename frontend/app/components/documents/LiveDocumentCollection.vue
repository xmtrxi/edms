<script setup lang="ts">
  import {
    FileTextIcon,
    FolderOpenIcon,
    RefreshCwIcon,
    RotateCcwIcon,
    Trash2Icon,
  } from "@lucide/vue";
  import { computed, ref } from "vue";

  import type { Document } from "@/types/api";
  import { formatBytes, formatDate } from "@/utils/format";

  const props = withDefaults(
    defineProps<{
      documents: Document[];
      loading?: boolean;
      error?: string;
      emptyTitle: string;
      mode?: "browse" | "trash";
    }>(),
    { loading: false, error: "", mode: "browse" },
  );
  const emit = defineEmits<{ refresh: []; restore: [id: string]; trash: [id: string] }>();
  const query = ref("");
  const filtered = computed(() => {
    const value = query.value.trim().toLowerCase();
    return value
      ? props.documents.filter((item) =>
          [item.name, item.original_name, item.folder_path].some((text) =>
            text?.toLowerCase().includes(value),
          ),
        )
      : props.documents;
  });
</script>

<template>
  <div class="space-y-4">
    <div class="bg-card flex items-center gap-2 rounded-xl border p-2 shadow-sm">
      <input
        v-model="query"
        class="h-9 min-w-0 flex-1 bg-transparent px-3 text-sm outline-none"
        placeholder="Filter this collection…"
      />
      <Button variant="ghost" size="sm" :disabled="loading" @click="emit('refresh')"
        ><RefreshCwIcon class="size-4" /> Refresh</Button
      >
    </div>
    <div
      v-if="error"
      role="alert"
      class="border-destructive/25 bg-destructive/5 text-destructive rounded-xl border px-4 py-3 text-sm"
    >
      {{ error }}
    </div>
    <div v-if="loading && documents.length === 0" class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
      <Skeleton v-for="index in 6" :key="index" class="h-32 rounded-xl" />
    </div>
    <div v-else-if="filtered.length" class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
      <article
        v-for="item in filtered"
        :key="item.id"
        class="group bg-card rounded-xl border p-4 shadow-sm transition hover:-translate-y-0.5 hover:shadow-md"
      >
        <div class="flex items-start gap-3">
          <div
            class="bg-primary/10 text-primary grid size-10 shrink-0 place-items-center rounded-lg"
          >
            <FileTextIcon class="size-5" />
          </div>
          <div class="min-w-0 flex-1">
            <NuxtLink
              :to="`/document/${item.id}`"
              class="hover:text-primary line-clamp-2 text-sm font-semibold"
              >{{ item.name }}</NuxtLink
            >
            <p class="text-muted-foreground mt-1 truncate text-xs">
              {{ item.folder_path || "Office files" }}
            </p>
          </div>
          <Badge variant="secondary" class="capitalize">{{ item.status }}</Badge>
        </div>
        <div
          class="text-muted-foreground mt-4 flex items-center justify-between border-t pt-3 text-xs"
        >
          <span>{{ formatBytes(item.size) }} · {{ formatDate(item.updated_at) }}</span
          ><Button
            v-if="mode === 'trash'"
            variant="ghost"
            size="sm"
            class="h-7 gap-1"
            @click="emit('restore', item.id)"
            ><RotateCcwIcon class="size-3.5" />Restore</Button
          ><Button
            v-else
            variant="ghost"
            size="icon-sm"
            aria-label="Move to trash"
            @click="emit('trash', item.id)"
            ><Trash2Icon class="size-3.5"
          /></Button>
        </div>
      </article>
    </div>
    <div v-else class="bg-muted/20 rounded-2xl border border-dashed py-20 text-center">
      <div class="bg-background mx-auto grid size-12 place-items-center rounded-xl shadow-sm">
        <FolderOpenIcon class="text-muted-foreground size-5" />
      </div>
      <h2 class="mt-4 text-base font-semibold">{{ emptyTitle }}</h2>
      <p class="text-muted-foreground mt-1 text-sm">
        Files will appear here as your office workflow changes.
      </p>
    </div>
  </div>
</template>
