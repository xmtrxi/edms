<script setup lang="ts">
  import {
    CheckCircle2Icon,
    ChevronDownIcon,
    CopyIcon,
    RotateCwIcon,
    XCircleIcon,
    XIcon,
  } from "@lucide/vue";
  import { computed, shallowRef } from "vue";

  import { Button } from "@/components/ui/button";
  import { Progress } from "@/components/ui/progress";
  import type { UploadItem } from "@/stores/uploads";
  import { formatBytes } from "@/utils/format";

  const uploads = useUploadStore();
  const collapsed = shallowRef(false);

  const title = computed(() => {
    const active = uploads.active.length;
    if (active) return `Uploading ${active} file${active === 1 ? "" : "s"}`;
    const failed = uploads.items.filter((item) => item.status === "failed").length;
    return failed ? `${failed} upload${failed === 1 ? "" : "s"} failed` : "Uploads complete";
  });

  const percent = (item: UploadItem) =>
    item.file.size ? Math.round((item.sent / item.file.size) * 100) : 100;
</script>

<template>
  <section
    v-if="uploads.items.length"
    class="bg-card fixed right-4 bottom-4 z-50 w-[min(24rem,calc(100vw-2rem))] overflow-hidden rounded-xl border shadow-lg"
    aria-live="polite"
    :aria-label="title"
  >
    <header class="flex items-center gap-2 border-b px-3 py-2">
      <h2 class="flex-1 truncate text-sm font-semibold">{{ title }}</h2>
      <Button
        variant="ghost"
        size="icon"
        class="size-7"
        :aria-label="collapsed ? 'Show uploads' : 'Hide uploads'"
        @click="collapsed = !collapsed"
      >
        <ChevronDownIcon class="size-4 transition-transform" :class="collapsed && 'rotate-180'" />
      </Button>
      <Button
        v-if="!uploads.active.length"
        variant="ghost"
        size="icon"
        class="size-7"
        aria-label="Close uploads"
        @click="uploads.clearFinished()"
        ><XIcon class="size-4"
      /></Button>
    </header>
    <ul v-if="!collapsed" class="max-h-72 divide-y overflow-y-auto">
      <li v-for="item in uploads.items" :key="item.key" class="px-3 py-2.5">
        <div class="flex items-center gap-2">
          <span class="min-w-0 flex-1 truncate text-sm">{{ item.file.name }}</span>
          <CheckCircle2Icon v-if="item.status === 'done'" class="text-success size-4 shrink-0" />
          <CopyIcon
            v-else-if="item.status === 'duplicate'"
            class="text-muted-foreground size-4 shrink-0"
          />
          <template v-else-if="item.status === 'failed'">
            <XCircleIcon class="text-destructive size-4 shrink-0" />
            <Button
              variant="ghost"
              size="icon"
              class="size-7"
              :aria-label="`Retry ${item.file.name}`"
              @click="uploads.retry(item.key)"
              ><RotateCwIcon class="size-3.5"
            /></Button>
          </template>
          <Button
            v-else-if="item.status === 'queued' || item.status === 'uploading'"
            variant="ghost"
            size="icon"
            class="size-7"
            :aria-label="`Cancel ${item.file.name}`"
            @click="uploads.cancel(item.key)"
            ><XIcon class="size-3.5"
          /></Button>
        </div>
        <Progress
          v-if="item.status === 'uploading' || item.status === 'queued'"
          :model-value="percent(item)"
          class="mt-1.5 h-1.5"
        />
        <p class="text-muted-foreground mt-1 truncate text-xs">
          <template v-if="item.status === 'failed'">{{ item.error }}</template>
          <template v-else-if="item.status === 'duplicate'"
            >Already in the library, so it wasn’t added again</template
          >
          <template v-else-if="item.status === 'cancelled'">Cancelled</template>
          <template v-else-if="item.status === 'done'">{{ formatBytes(item.file.size) }}</template>
          <template v-else
            >{{ formatBytes(item.sent) }} of {{ formatBytes(item.file.size) }}</template
          >
        </p>
      </li>
    </ul>
  </section>
</template>
