<script setup lang="ts">
  import { ArrowRightIcon, FileTextIcon } from "@lucide/vue";

  import type { Document } from "@/types/api";
  import { formatBytes, formatDateTime } from "@/utils/format";
  import { getStatusMeta } from "@/utils/status";

  defineProps<{ documents: Document[]; loading: boolean }>();
</script>

<template>
  <section>
    <div class="mb-4 flex items-end justify-between gap-4">
      <div>
        <h2 class="text-lg font-semibold tracking-[-0.02em]">Recent records</h2>
        <p class="text-muted-foreground mt-1 text-sm">
          The latest documents returned by the office API.
        </p>
      </div>
      <NuxtLink
        to="/documents"
        class="text-muted-foreground hover:text-foreground flex items-center gap-1.5 text-sm"
        >All records <ArrowRightIcon class="size-3.5"
      /></NuxtLink>
    </div>
    <div
      class="bg-card overflow-hidden rounded-xl shadow-[0_16px_55px_-38px_oklch(0.18_0.03_250/0.45)]"
    >
      <div v-if="loading" class="text-muted-foreground px-5 py-12 text-center text-sm">
        Reading the document register…
      </div>
      <div v-else-if="!documents.length" class="px-6 py-12 text-center">
        <FileTextIcon class="text-muted-foreground/50 mx-auto size-6" />
        <p class="mt-3 text-sm font-medium">The register is empty</p>
        <p class="text-muted-foreground mt-1 text-sm">Upload the first office record to begin.</p>
      </div>
      <template v-else>
        <NuxtLink
          v-for="document in documents.slice(0, 8)"
          :key="document.id"
          :to="`/documents/${document.id}`"
          class="hover:bg-muted/50 grid grid-cols-[minmax(0,1fr)_auto] gap-4 border-b px-5 py-4 transition-colors last:border-b-0 sm:grid-cols-[minmax(0,1fr)_8rem_8rem] sm:items-center"
        >
          <div class="min-w-0">
            <p class="truncate text-sm font-medium">{{ document.name }}</p>
            <p class="text-muted-foreground mt-1 truncate text-xs">
              {{ document.folder_path || "Office library" }} · {{ formatBytes(document.size) }}
            </p>
          </div>
          <span
            :class="getStatusMeta(document.status).tone"
            class="w-fit rounded-full px-2.5 py-1 text-[11px] font-medium capitalize"
            >{{ getStatusMeta(document.status).label }}</span
          >
          <time
            class="text-muted-foreground hidden text-right font-mono text-[11px] sm:block"
            :datetime="document.updated_at"
            >{{ formatDateTime(document.updated_at) }}</time
          >
        </NuxtLink>
      </template>
    </div>
  </section>
</template>
