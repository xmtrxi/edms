<script setup lang="ts">
  import { ArrowLeftIcon, ChevronDownIcon, Loader2Icon, SearchIcon, XIcon } from "@lucide/vue";
  import { computed, ref, watch } from "vue";
  import { toast } from "vue-sonner";

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
  import { formatBytes, formatDate, fileTypeFromMime, initialsOf } from "@/utils/format";
  import { getStatusMeta } from "@/utils/status";

  definePageMeta({
    title: "Search",
    description: "Search across your documents, OCR text, and metadata.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Search" }],
  });

  useHead({
    title: "Search — Lumen DMS",
    meta: [
      { name: "description", content: "Search across your documents, OCR text, and metadata." },
    ],
  });

  const router = useRouter();

  const svc = searchService();
  const { results, meta, loading, error } = svc;

  const PAGE_SIZE = 10;
  const query = ref("");
  const status = ref("");
  const page = ref(1);

  const totalPages = computed(() => Math.max(1, Math.ceil((meta.value?.total ?? 0) / PAGE_SIZE)));

  const load = () => {
    const q = query.value.trim();
    if (!q) return;
    svc.search(q, {
      status: status.value || undefined,
      page: page.value,
      page_size: PAGE_SIZE,
    });
  };

  watch(query, (_value, _oldValue, onCleanup) => {
    const t = setTimeout(() => {
      page.value = 1;
      load();
    }, 300);
    onCleanup(() => clearTimeout(t));
  });

  watch([status, page], () => {
    if (query.value.trim()) load();
  });

  const openDocument = (id: string) => {
    router.push({ name: "documents-id", params: { id } });
  };

  const clearSearch = () => {
    query.value = "";
    page.value = 1;
  };

  const reindex = () => {
    svc.reindex().then((ok) => {
      if (ok) toast.success("Search index rebuild queued");
      else toast.error(svc.error || "Could not queue reindex");
    });
  };

  const statusOptions = ["uploaded", "processing", "ready", "failed", "archived"] as const;
  const hasSearched = computed(() => !!query.value.trim());
</script>

<template>
  <div class="mx-auto max-w-3xl">
    <div class="relative">
      <SearchIcon
        class="text-muted-foreground absolute top-1/2 left-3.5 h-4 w-4 -translate-y-1/2"
      />
      <Input
        v-model="query"
        placeholder="Search across all documents&hellip;"
        class="bg-card border-border shadow-elegant h-12 rounded-xl pr-11 pl-10 text-base"
      />
      <div class="absolute top-1/2 right-2 flex -translate-y-1/2 items-center gap-1">
        <Button
          v-if="query"
          variant="ghost"
          size="icon"
          class="h-8 w-8"
          @click="clearSearch"
          aria-label="Clear"
        >
          <XIcon class="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>

    <div class="mt-3 flex flex-wrap items-center gap-2">
      <DropdownMenu>
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
      <Button variant="ghost" size="sm" class="text-muted-foreground h-9 gap-1.5" @click="reindex">
        <Loader2Icon v-if="loading" class="h-3.5 w-3.5 animate-spin" />
        Rebuild index
      </Button>
    </div>
  </div>

  <div class="mt-6">
    <div v-if="loading" class="space-y-2.5">
      <div
        v-for="i in 3"
        :key="i"
        class="border-border bg-card animate-pulse rounded-2xl border p-4"
      >
        <div class="bg-secondary h-4 w-1/3 rounded" />
        <div class="bg-secondary mt-2 h-3 w-1/2 rounded" />
      </div>
    </div>

    <div v-else-if="error" class="border-border rounded-2xl border border-dashed p-12 text-center">
      <p class="text-sm font-medium">Search failed</p>
      <p class="text-muted-foreground mt-1 text-xs">{{ error }}</p>
    </div>

    <div
      v-else-if="!hasSearched"
      class="border-border rounded-2xl border border-dashed p-12 text-center"
    >
      <SearchIcon class="text-muted-foreground/40 mx-auto h-8 w-8" />
      <p class="mt-2 text-sm font-medium">Type to search</p>
      <p class="text-muted-foreground mt-1 text-xs">
        Search by filename or content across your document library.
      </p>
    </div>

    <div
      v-else-if="results.length === 0"
      class="border-border rounded-2xl border border-dashed p-12 text-center"
    >
      <p class="text-sm font-medium">No results for “{{ query.trim() }}”</p>
      <p class="text-muted-foreground mt-1 text-xs">Try different keywords or filters.</p>
    </div>

    <template v-else>
      <p class="text-muted-foreground mb-3 text-xs">
        <span class="text-foreground font-medium">{{ meta?.total ?? results.length }}</span>
        results for “{{ query.trim() }}”<span v-if="status">
          · {{ getStatusMeta(status).label }}</span
        >
      </p>

      <div class="space-y-2.5">
        <div
          v-for="d in results"
          :key="d.id"
          class="border-border bg-card shadow-elegant hover:shadow-glow hover:border-primary/30 cursor-pointer rounded-2xl border p-4 transition-all"
          @click="openDocument(d.id)"
        >
          <div class="flex gap-3">
            <FileTypeIcon :type="fileTypeFromMime(d.mime_type)" size="md" />
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <h3 class="text-foreground truncate text-sm font-semibold">{{ d.name }}</h3>
                <StatusBadges :status="d.status" />
              </div>
              <div class="mt-1.5 flex flex-wrap items-center gap-1.5 text-[11px]">
                <span class="text-muted-foreground">{{ d.original_name }}</span>
                <Badge
                  variant="secondary"
                  class="bg-secondary text-muted-foreground h-4 border-0 px-1 text-[10px]"
                  >{{ formatBytes(d.size) }}</Badge
                >
              </div>
              <div class="mt-2 flex flex-wrap items-center gap-1.5 text-[11px]">
                <span class="text-muted-foreground flex items-center gap-1">
                  <Avatar class="h-4 w-4">
                    <AvatarFallback class="bg-secondary text-[7px]">{{
                      initialsOf(d.owner?.name)
                    }}</AvatarFallback>
                  </Avatar>
                  {{ d.owner?.name ?? "Unknown" }}
                </span>
                <span class="text-muted-foreground/60"
                  >&middot; {{ formatDate(d.updated_at) }}</span
                >
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="totalPages > 1" class="mt-6 flex items-center justify-center gap-1">
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
