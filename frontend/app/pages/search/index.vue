<script setup lang="ts">
  import {
    ArrowLeftIcon,
    ChevronDownIcon,
    DatabaseZapIcon,
    Loader2Icon,
    SearchIcon,
    SlidersHorizontalIcon,
    TriangleAlertIcon,
    XIcon,
  } from "lucide-vue-next";
  import { computed, shallowRef, watch } from "vue";
  import { toast } from "vue-sonner";

  import SearchResultCard from "@/components/search/SearchResultCard.vue";
  import { Button } from "@/components/ui/button";
  import {
    Card,
    CardAction,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
  } from "@/components/ui/card";
  import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
  } from "@/components/ui/dropdown-menu";
  import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
  } from "@/components/ui/empty";
  import {
    InputGroup,
    InputGroupAddon,
    InputGroupButton,
    InputGroupInput,
  } from "@/components/ui/input-group";
  import { Skeleton } from "@/components/ui/skeleton";
  import { cn } from "@/lib/utils";
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
  const query = shallowRef("");
  const status = shallowRef("");
  const page = shallowRef(1);
  const reindexing = shallowRef(false);

  const totalPages = computed(() => Math.max(1, Math.ceil((meta.value?.total ?? 0) / PAGE_SIZE)));
  const hasSearched = computed(() => !!query.value.trim());
  const hasBackendMatches = computed(() => (meta.value?.total ?? 0) > 0);
  const activeStatusLabel = computed(() =>
    status.value ? getStatusMeta(status.value).label : "All statuses",
  );
  const resultStart = computed(() => (meta.value?.total ? (page.value - 1) * PAGE_SIZE + 1 : 0));
  const resultEnd = computed(() => Math.min(page.value * PAGE_SIZE, meta.value?.total ?? 0));

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

  const reindex = async () => {
    reindexing.value = true;
    const ok = await svc.reindex();
    reindexing.value = false;
    if (ok) toast.success("Search index rebuild queued");
    else toast.error(svc.error || "Could not queue reindex");
  };

  const statusOptions = ["uploaded", "processing", "ready", "failed", "archived"] as const;
</script>

<template>
  <div class="space-y-6">
    <Card class="overflow-hidden">
      <CardHeader class="border-border gap-4 border-b px-5 py-4 sm:flex-row sm:items-center">
        <div class="flex min-w-0 items-center gap-3">
          <div class="bg-primary/10 text-primary grid h-10 w-10 place-items-center rounded-lg">
            <SearchIcon class="h-4 w-4" />
          </div>
          <div class="min-w-0">
            <CardTitle class="text-base">Search library</CardTitle>
            <CardDescription class="mt-0.5 text-xs">
              Meilisearch across filenames, metadata, and Docling OCR content.
            </CardDescription>
          </div>
        </div>
        <CardAction class="sm:ml-auto">
          <Button
            variant="outline"
            size="sm"
            class="gap-1.5"
            :disabled="reindexing"
            @click="reindex"
          >
            <DatabaseZapIcon v-if="!reindexing" class="h-3.5 w-3.5" />
            <Loader2Icon v-else class="h-3.5 w-3.5 animate-spin" />
            Reindex OCR search
          </Button>
        </CardAction>
      </CardHeader>

      <CardContent class="p-5">
        <InputGroup class="bg-background h-12 rounded-lg border">
          <InputGroupAddon>
            <SearchIcon class="h-4 w-4" />
          </InputGroupAddon>
          <InputGroupInput
            v-model="query"
            placeholder="Search invoices, clauses, names, OCR text..."
            class="h-12 text-base"
          />
          <InputGroupAddon v-if="query" align="inline-end">
            <InputGroupButton size="icon-sm" @click="clearSearch" aria-label="Clear search">
              <XIcon class="h-3.5 w-3.5" />
            </InputGroupButton>
          </InputGroupAddon>
        </InputGroup>

        <div class="mt-3 flex flex-wrap items-center justify-between gap-2">
          <div class="flex flex-wrap items-center gap-2">
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="outline" size="sm" class="h-9 gap-1.5">
                  <SlidersHorizontalIcon class="h-3.5 w-3.5" />
                  {{ activeStatusLabel }}
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
                >
                  All statuses
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  v-for="s in statusOptions"
                  :key="s"
                  :class="cn(status === s && 'bg-secondary/60')"
                  @click="
                    status = s;
                    page = 1;
                  "
                >
                  {{ getStatusMeta(s).label }}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            <Button
              v-if="status"
              variant="ghost"
              size="sm"
              class="text-muted-foreground h-9"
              @click="
                status = '';
                page = 1;
              "
            >
              Clear filters
            </Button>
          </div>
          <p class="text-muted-foreground text-xs">
            Reindex after re-running OCR or importing older documents.
          </p>
        </div>
      </CardContent>
    </Card>

    <div v-if="loading" class="space-y-2.5">
      <Card v-for="i in 4" :key="i">
        <CardContent class="space-y-3 p-4">
          <Skeleton class="h-4 w-1/3" />
          <Skeleton class="h-3 w-11/12" />
          <Skeleton class="h-3 w-8/12" />
        </CardContent>
      </Card>
    </div>

    <Card v-else-if="error" class="border-destructive/30">
      <Empty class="p-10">
        <EmptyHeader>
          <EmptyMedia variant="icon" class="bg-destructive/10 text-destructive">
            <TriangleAlertIcon />
          </EmptyMedia>
          <EmptyTitle class="text-sm tracking-normal normal-case">Search failed</EmptyTitle>
          <EmptyDescription>{{ error }}</EmptyDescription>
        </EmptyHeader>
      </Empty>
    </Card>

    <Card v-else-if="!hasSearched" class="border-dashed">
      <Empty class="p-12">
        <EmptyHeader>
          <EmptyMedia variant="icon"><SearchIcon /></EmptyMedia>
          <EmptyTitle class="text-sm tracking-normal normal-case">Search is ready</EmptyTitle>
          <EmptyDescription>
            Type a filename, phrase, invoice number, clause, or OCR keyword.
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    </Card>

    <Card v-else-if="results.length === 0" class="border-dashed">
      <Empty class="p-12">
        <EmptyHeader>
          <EmptyMedia variant="icon"><SearchIcon /></EmptyMedia>
          <EmptyTitle class="text-sm tracking-normal normal-case">
            {{ hasBackendMatches ? "Results need a refresh" : `No results for “${query.trim()}”` }}
          </EmptyTitle>
          <EmptyDescription>
            {{
              hasBackendMatches
                ? "The index found records that are not available in this response. Rebuild the OCR index, then try again."
                : "Try a shorter phrase, a different keyword, or clear the status filter."
            }}
          </EmptyDescription>
        </EmptyHeader>
        <Button v-if="hasBackendMatches" size="sm" variant="outline" @click="reindex">
          Rebuild index
        </Button>
      </Empty>
    </Card>

    <template v-else>
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <p class="text-muted-foreground text-xs">
          Showing
          <span class="text-foreground font-medium">{{ resultStart }}-{{ resultEnd }}</span>
          of
          <span class="text-foreground font-medium">{{ meta?.total ?? results.length }}</span>
          results for “{{ query.trim() }}”
        </p>
        <p class="text-muted-foreground text-xs">
          {{ activeStatusLabel }}
        </p>
      </div>

      <div class="space-y-2.5">
        <SearchResultCard v-for="d in results" :key="d.id" :document="d" @open="openDocument" />
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
