<script setup lang="ts">
  import {
    ArrowLeftIcon,
    ChevronDownIcon,
    EyeIcon,
    LayoutGridIcon,
    ListIcon,
    SearchIcon,
    XIcon,
  } from "@lucide/vue";

  import FileTypeIcon from "@/components/FileTypeIcon.vue";
  import StatusBadges from "@/components/StatusBadges.vue";
  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
  } from "@/components/ui/dropdown-menu";
  import { Input } from "@/components/ui/input";
  import { cn } from "@/lib/utils";
  import type { Document } from "@/types/api";
  import { formatBytes, formatDate, fileTypeFromMime, initialsOf } from "@/utils/format";
  import { getStatusMeta } from "@/utils/status";

  const router = useRouter();

  const props = withDefaults(
    defineProps<{
      initialStatus?: string;
      showStatusFilter?: boolean;
      showSearch?: boolean;
    }>(),
    {
      initialStatus: "",
      showStatusFilter: true,
      showSearch: true,
    },
  );

  const service = documentsService();
  const { documents, meta, loading, error } = service;

  const PAGE_SIZE = 10;
  const view = ref<"list" | "grid">("list");
  const query = ref("");
  const status = ref(props.initialStatus);
  const page = ref(1);

  const totalPages = computed(() => Math.max(1, Math.ceil((meta.value?.total ?? 0) / PAGE_SIZE)));

  const load = () => {
    service.fetchDocuments({
      q: query.value.trim() || undefined,
      status: status.value || undefined,
      page: page.value,
      page_size: PAGE_SIZE,
    });
  };

  load();

  watch(query, (_value, _oldValue, onCleanup) => {
    const t = setTimeout(() => {
      page.value = 1;
      load();
    }, 300);
    onCleanup(() => clearTimeout(t));
  });

  watch([status, page], () => {
    load();
  });

  const openDocument = (doc: Document) => {
    router.push({ name: "documents-id", params: { id: doc.id } });
  };

  const clearFilters = () => {
    query.value = "";
    status.value = "";
    page.value = 1;
  };

  const hasActiveFilters = computed(() => !!query.value.trim() || !!status.value);

  const statusOptions = ["uploaded", "processing", "ready", "failed", "archived"] as const;

  defineExpose({ refresh: load });
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center gap-2">
      <div v-if="props.showSearch" class="relative min-w-[220px] flex-1">
        <SearchIcon
          class="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2"
        />
        <Input
          v-model="query"
          placeholder="Search your documents…"
          class="bg-secondary/60 focus-visible:bg-card focus-visible:border-border h-9 rounded-lg border-transparent pl-9"
        />
        <button
          v-if="query"
          @click="
            query = '';
            page = 1;
            load();
          "
          class="hover:bg-secondary text-muted-foreground hover:text-foreground absolute top-1/2 right-2 grid h-5 w-5 -translate-y-1/2 place-items-center rounded transition-colors"
        >
          <XIcon class="h-3 w-3" />
        </button>
      </div>

      <DropdownMenu v-if="props.showStatusFilter">
        <DropdownMenuTrigger as-child>
          <Button variant="outline" size="sm" class="h-9 gap-1.5">
            {{ status ? getStatusMeta(status).label : "All statuses" }}
            <ChevronDownIcon class="h-3 w-3 opacity-60" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" :side="'bottom'" :avoidCollisions="true">
          <DropdownMenuLabel>Filter by status</DropdownMenuLabel>
          <DropdownMenuItem
            :class="cn(!status && 'bg-secondary/60')"
            @click="
              status = '';
              page = 1;
            "
            >All statuses</DropdownMenuItem
          >
          <DropdownMenuSeparator />
          <DropdownMenuItem
            v-for="s in statusOptions"
            :key="s"
            :class="cn(status === s && 'bg-secondary/60')"
            @click="
              status = s;
              page = 1;
            "
            >{{ getStatusMeta(s).label }}</DropdownMenuItem
          >
        </DropdownMenuContent>
      </DropdownMenu>

      <div class="border-border bg-card ml-auto flex items-center gap-1 rounded-lg border p-0.5">
        <Button
          :variant="view === 'list' ? 'secondary' : 'ghost'"
          size="icon"
          :class="cn('h-7 w-7', view === 'list' && 'shadow-sm')"
          @click="view = 'list'"
          aria-label="List view"
        >
          <ListIcon class="h-3.5 w-3.5" />
        </Button>
        <Button
          :variant="view === 'grid' ? 'secondary' : 'ghost'"
          size="icon"
          :class="cn('h-7 w-7', view === 'grid' && 'shadow-sm')"
          @click="view = 'grid'"
          aria-label="Grid view"
        >
          <LayoutGridIcon class="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>

    <div class="text-muted-foreground mb-4 flex items-center gap-2 text-xs">
      <span>{{ meta?.total ?? 0 }} items</span>
      <span v-if="meta?.total && meta.total > PAGE_SIZE" class="text-muted-foreground/60"
        >&middot; Page {{ page }} of {{ totalPages }}</span
      >
      <Badge v-if="status" variant="secondary" class="bg-primary/10 text-primary gap-1 border-0">
        {{ getStatusMeta(status).label }}
      </Badge>
      <button
        v-if="hasActiveFilters"
        class="hover:text-foreground inline-flex items-center gap-1 text-[11px] transition-colors"
        @click="clearFilters"
      >
        <XIcon class="h-3 w-3" /> Clear filters
      </button>
    </div>

    <div
      v-if="loading && documents.length === 0"
      class="border-border rounded-2xl border border-dashed p-12 text-center"
    >
      <div
        class="border-border mx-auto h-5 w-5 animate-spin rounded-full border-2 border-t-transparent"
      />
      <p class="text-muted-foreground mt-3 text-xs">Loading documents…</p>
    </div>

    <div v-else-if="error" class="border-border rounded-2xl border p-12 text-center">
      <p class="text-sm font-medium">Could not load documents</p>
      <p class="text-muted-foreground mt-1 text-xs">{{ error }}</p>
      <Button variant="outline" size="sm" class="mt-4" @click="load">Try again</Button>
    </div>

    <div
      v-else-if="documents.length === 0"
      class="border-border rounded-2xl border border-dashed p-12 text-center"
    >
      <p class="text-sm font-medium">No documents</p>
      <p class="text-muted-foreground mt-1 text-xs">Try adjusting filters or upload a new file.</p>
    </div>

    <template v-else>
      <div
        v-if="view === 'grid'"
        class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4"
      >
        <button
          v-for="d in documents"
          :key="d.id"
          @click="openDocument(d)"
          class="border-border bg-card shadow-elegant hover:border-primary/40 group flex flex-col rounded-2xl border p-4 text-left transition-colors"
        >
          <div class="flex items-start justify-between gap-2">
            <FileTypeIcon :type="fileTypeFromMime(d.mime_type)" size="sm" />
            <StatusBadges :status="d.status" />
          </div>
          <div class="mt-3 min-w-0">
            <p class="text-foreground truncate text-sm font-medium">{{ d.name }}</p>
            <p class="text-muted-foreground mt-0.5 truncate text-xs">{{ d.original_name }}</p>
          </div>
          <div class="text-muted-foreground mt-3 flex items-center justify-between text-[11px]">
            <span>{{ formatBytes(d.size) }}</span>
            <span>{{ formatDate(d.updated_at) }}</span>
          </div>
        </button>
      </div>

      <div v-else class="border-border bg-card shadow-elegant overflow-hidden rounded-2xl border">
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="bg-secondary/50 text-muted-foreground text-xs">
              <tr>
                <th class="px-4 py-2.5 text-left font-medium">Name</th>
                <th class="hidden px-3 py-2.5 text-left font-medium md:table-cell">Owner</th>
                <th class="hidden px-3 py-2.5 text-left font-medium md:table-cell">Status</th>
                <th class="hidden px-3 py-2.5 text-right font-medium sm:table-cell">Size</th>
                <th class="px-3 py-2.5 text-right font-medium">Modified</th>
                <th class="w-12 px-2 py-2.5"></th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="d in documents"
                :key="d.id"
                class="border-border/70 hover:bg-secondary/40 border-t transition-colors"
              >
                <td class="px-4 py-2">
                  <button
                    @click="openDocument(d)"
                    class="flex w-full min-w-0 items-center gap-2.5 text-left"
                  >
                    <FileTypeIcon :type="fileTypeFromMime(d.mime_type)" size="sm" />
                    <div class="min-w-0">
                      <div class="text-foreground truncate text-sm font-medium">{{ d.name }}</div>
                      <div class="text-muted-foreground mt-0.5 truncate text-xs md:hidden">
                        {{ d.original_name }}
                      </div>
                    </div>
                  </button>
                </td>
                <td class="hidden px-3 py-2 md:table-cell">
                  <div class="flex items-center gap-2">
                    <Avatar class="h-6 w-6">
                      <AvatarFallback class="bg-secondary text-[10px]">{{
                        initialsOf(d.owner?.name)
                      }}</AvatarFallback>
                    </Avatar>
                    <span class="text-muted-foreground truncate text-xs">{{
                      d.owner?.name ?? "—"
                    }}</span>
                  </div>
                </td>
                <td class="hidden px-3 py-2 md:table-cell">
                  <StatusBadges :status="d.status" />
                </td>
                <td
                  class="text-muted-foreground hidden px-3 py-2 text-right text-xs tabular-nums sm:table-cell"
                >
                  {{ formatBytes(d.size) }}
                </td>
                <td class="text-muted-foreground px-3 py-2 text-right text-xs">
                  {{ formatDate(d.updated_at) }}
                </td>
                <td class="px-2 py-2">
                  <Button
                    variant="ghost"
                    size="icon"
                    class="h-7 w-7"
                    @click="openDocument(d)"
                    aria-label="Open document"
                  >
                    <EyeIcon class="h-3.5 w-3.5" />
                  </Button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div
        v-if="totalPages > 1 && documents.length > 0"
        class="mt-6 flex items-center justify-center gap-1"
      >
        <Button
          variant="outline"
          size="sm"
          class="h-8 w-8"
          :disabled="page <= 1"
          @click="page = Math.max(1, page - 1)"
        >
          <ArrowLeftIcon class="h-3.5 w-3.5" />
        </Button>
        <template v-for="p in totalPages" :key="p">
          <Button
            v-if="p === 1 || p === totalPages || Math.abs(p - page) <= 1"
            :variant="p === page ? 'default' : 'outline'"
            size="sm"
            class="h-8 min-w-[32px]"
            @click="page = p"
            >{{ p }}</Button
          >
          <span
            v-else-if="p === page - 2 || p === page + 2"
            class="text-muted-foreground px-1 text-xs"
            >..</span
          >
        </template>
        <Button
          variant="outline"
          size="sm"
          class="h-8 w-8"
          :disabled="page >= totalPages"
          @click="page = Math.min(totalPages, page + 1)"
        >
          <ArrowLeftIcon class="h-3.5 w-3.5 rotate-180" />
        </Button>
      </div>
    </template>
  </div>
</template>
