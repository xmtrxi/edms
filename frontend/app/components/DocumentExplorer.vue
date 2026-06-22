<script setup lang="ts">
  import {
    FilterIcon,
    LayoutGridIcon,
    ListIcon,
    SearchIcon,
    SlidersHorizontalIcon,
    TagIcon,
    CloudIcon,
    ChevronDownIcon,
    Trash2Icon,
    Share2Icon,
    FolderOpenIcon,
    TagsIcon,
    ArrowLeftIcon,
    XIcon,
  } from "@lucide/vue";
  import { ref, computed, watch } from "vue";
  import { useRouter, useRoute } from "vue-router";
  import { toast } from "vue-sonner";

  import DocumentDetailPanel from "@/components/DocumentDetailPanel.vue";
  import FolderBrowser from "@/components/FolderBrowser.vue";
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
  import { Separator } from "@/components/ui/separator";
  import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
  import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
  import { documents as allDocs, type Doc } from "@/data/mock-data";
  import { cn } from "@/lib/utils";

  const router = useRouter();
  const route = useRoute();

  const props = withDefaults(
    defineProps<{
      docs?: Doc[];
      emptyTitle?: string;
      emptyAction?: string;
      emptyLink?: string;
    }>(),
    {
      docs: () => allDocs,
      emptyTitle: "No documents",
    },
  );

  const view = ref<"grid" | "list" | "folders">("list");
  const query = ref("");
  const selected = ref<Doc | null>(null);
  const quickViewDoc = ref<Doc | null>(null);
  const checked = ref<Set<string>>(new Set());
  const filter = ref<"all" | "sharepoint" | "local" | "ai">("all");

  const PAGE_SIZE = 10;
  const page = ref(1);

  const filterCounts = computed(() => ({
    all: props.docs.length,
    sharepoint: props.docs.filter((d) => d.source === "sharepoint").length,
    local: props.docs.filter((d) => d.source !== "sharepoint").length,
    ai: props.docs.filter((d) => !!d.aiSummary).length,
  }));

  const folderFilter = ref<string | null>(route.query.folder as string | null);

  watch(
    () => route.query.folder,
    (f) => {
      folderFilter.value = (f as string) ?? null;
    },
  );

  const hasActiveFilters = computed(
    () => filter.value !== "all" || !!query.value.trim() || !!folderFilter.value,
  );

  function clearFilters() {
    filter.value = "all";
    query.value = "";
    page.value = 1;
    folderFilter.value = null;
  }

  const filtered = computed(() => {
    let r = props.docs;
    if (filter.value === "sharepoint") r = r.filter((d) => d.source === "sharepoint");
    else if (filter.value === "local") r = r.filter((d) => d.source !== "sharepoint");
    else if (filter.value === "ai") r = r.filter((d) => !!d.aiSummary);
    if (query.value.trim()) {
      const q = query.value.toLowerCase();
      r = r.filter(
        (d) => d.name.toLowerCase().includes(q) || d.tags.join(" ").toLowerCase().includes(q),
      );
    }
    if (folderFilter.value) {
      r = r.filter((d) => (d.folder || "Uncategorized") === folderFilter.value);
    }
    return r;
  });

  function selectFolder(folder: string) {
    view.value = "list";
    folderFilter.value = folder;
    page.value = 1;
    router.push({ query: { folder } });
  }

  const totalPages = computed(() => Math.max(1, Math.ceil(filtered.value.length / PAGE_SIZE)));

  const paginated = computed(() => {
    const start = (page.value - 1) * PAGE_SIZE;
    return filtered.value.slice(start, start + PAGE_SIZE);
  });

  function navigate(doc: Doc) {
    router.push(`/document/${doc.id}`);
  }

  function quickView(doc: Doc) {
    quickViewDoc.value = doc;
  }

  function selectCheck(id: string) {
    const n = new Set(checked.value);
    if (n.has(id)) n.delete(id);
    else n.add(id);
    checked.value = n;
  }

  function batchDelete() {
    const count = checked.value.size;
    const names = [...checked.value]
      .map((id) => props.docs.find((d) => d.id === id)?.name)
      .filter(Boolean);
    checked.value = new Set();
    toast(`Deleted ${count} document${count > 1 ? "s" : ""}`, {
      description:
        names.slice(0, 3).join(", ") + (names.length > 3 ? ` +${names.length - 3} more` : ""),
      action: { label: "Undo", onClick: () => toast.success("Restored") },
    });
  }

  function batchShare() {
    if (checked.value.size === 0) return;
    toast.info(
      `Share dialog for ${checked.value.size} document${checked.value.size > 1 ? "s" : ""}`,
    );
  }

  function batchMove() {
    if (checked.value.size === 0) return;
    toast.info(
      `Move ${checked.value.size} document${checked.value.size > 1 ? "s" : ""} — folder picker`,
    );
  }

  function batchTag() {
    if (checked.value.size === 0) return;
    toast.info(
      `Tag ${checked.value.size} document${checked.value.size > 1 ? "s" : ""} — tag selector`,
    );
  }
</script>

<template>
  <div class="-mx-4 flex gap-0 sm:-mx-6 lg:-mx-8">
    <div class="min-w-0 flex-1 px-4 sm:px-6 lg:px-8">
      <div class="mb-4 flex flex-wrap items-center gap-2">
        <div class="relative min-w-[220px] flex-1">
          <SearchIcon
            class="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2"
          />
          <Input
            v-model="query"
            placeholder="Filter in this view…"
            class="bg-secondary/60 focus-visible:bg-card focus-visible:border-border h-9 rounded-lg border-transparent pl-9"
          />
          <button
            v-if="query"
            @click="
              query = '';
              page = 1;
            "
            class="hover:bg-secondary text-muted-foreground hover:text-foreground absolute top-1/2 right-2 grid h-5 w-5 -translate-y-1/2 place-items-center rounded transition-colors"
          >
            <XIcon class="h-3 w-3" />
          </button>
        </div>

        <Tabs
          :default-value="filter"
          @update:model-value="
            (v: string) => {
              filter = v as typeof filter;
              page = 1;
            }
          "
        >
          <TabsList class="bg-secondary/60 h-9">
            <TabsTrigger value="all" class="gap-1.5 text-xs"
              >All
              <Badge variant="secondary" class="bg-background/60 h-4 border-0 px-1 text-[10px]">{{
                filterCounts.all
              }}</Badge></TabsTrigger
            >
            <TabsTrigger value="sharepoint" class="gap-1 text-xs"
              ><CloudIcon class="h-3 w-3" /> SharePoint
              <Badge
                v-if="filterCounts.sharepoint"
                variant="secondary"
                class="bg-background/60 h-4 border-0 px-1 text-[10px]"
                >{{ filterCounts.sharepoint }}</Badge
              ></TabsTrigger
            >
            <TabsTrigger value="local" class="gap-1 text-xs"
              >Local
              <Badge
                v-if="filterCounts.local"
                variant="secondary"
                class="bg-background/60 h-4 border-0 px-1 text-[10px]"
                >{{ filterCounts.local }}</Badge
              ></TabsTrigger
            >
            <TabsTrigger value="ai" class="gap-1 text-xs"
              >AI-tagged
              <Badge
                v-if="filterCounts.ai"
                variant="secondary"
                class="bg-background/60 h-4 border-0 px-1 text-[10px]"
                >{{ filterCounts.ai }}</Badge
              ></TabsTrigger
            >
          </TabsList>
        </Tabs>

        <div class="flex items-center gap-2">
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="h-9 gap-1.5">
                <SlidersHorizontalIcon class="h-3.5 w-3.5" /> Sort
                <ChevronDownIcon class="h-3 w-3 opacity-60" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" :side="'bottom'" :avoidCollisions="true">
              <DropdownMenuLabel>Sort by</DropdownMenuLabel>
              <DropdownMenuItem>Last modified</DropdownMenuItem>
              <DropdownMenuItem>Name (A→Z)</DropdownMenuItem>
              <DropdownMenuItem>Size</DropdownMenuItem>
              <DropdownMenuItem>Owner</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem>AI confidence</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <Button variant="outline" size="sm" class="h-9 gap-1.5"
            ><FilterIcon class="h-3.5 w-3.5" /> Filters</Button
          >
        </div>

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
          <Separator orientation="vertical" class="h-4" />
          <Button
            :variant="view === 'folders' ? 'secondary' : 'ghost'"
            size="icon"
            :class="cn('h-7 w-7', view === 'folders' && 'shadow-sm')"
            @click="view = 'folders'"
            aria-label="Folders view"
          >
            <FolderOpenIcon class="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      <div class="text-muted-foreground mb-4 flex flex-wrap items-center gap-2 text-xs">
        <span v-if="view !== 'folders'">{{ filtered.length }} items</span>
        <Badge
          v-if="folderFilter"
          variant="secondary"
          class="bg-primary/10 text-primary gap-1 border-0"
        >
          <FolderOpenIcon class="h-3 w-3" /> {{ folderFilter }}
        </Badge>
        <span
          v-if="view !== 'folders' && filtered.length > PAGE_SIZE"
          class="text-muted-foreground/60"
          >&middot; Page {{ page }} of {{ totalPages }}</span
        >
        <Badge
          v-if="checked.size > 0"
          variant="secondary"
          class="bg-primary/10 text-primary border-0"
          >{{ checked.size }} selected</Badge
        >
        <button
          v-if="hasActiveFilters"
          class="hover:text-foreground inline-flex items-center gap-1 text-[11px] transition-colors"
          @click="clearFilters"
        >
          <XIcon class="h-3 w-3" /> Clear filters
        </button>
      </div>

      <div
        v-if="checked.size > 0"
        class="bg-primary/5 border-primary/20 mb-4 flex items-center gap-2 rounded-lg border p-2"
      >
        <span class="text-primary text-xs font-medium">{{ checked.size }} selected</span>
        <div class="ml-auto flex items-center gap-1">
          <Button size="sm" variant="ghost" class="h-7 gap-1.5 text-xs" @click="batchDelete">
            <Trash2Icon class="h-3.5 w-3.5" /> Delete
          </Button>
          <Button size="sm" variant="ghost" class="h-7 gap-1.5 text-xs" @click="batchShare">
            <Share2Icon class="h-3.5 w-3.5" /> Share
          </Button>
          <Button size="sm" variant="ghost" class="h-7 gap-1.5 text-xs" @click="batchMove">
            <FolderOpenIcon class="h-3.5 w-3.5" /> Move
          </Button>
          <Button size="sm" variant="ghost" class="h-7 gap-1.5 text-xs" @click="batchTag">
            <TagsIcon class="h-3.5 w-3.5" /> Tag
          </Button>
        </div>
      </div>

      <FolderBrowser
        v-if="view === 'folders'"
        @select-folder="selectFolder"
        @select-doc="quickView"
      />

      <template v-else-if="view !== 'folders'">
        <div
          v-if="filtered.length === 0"
          class="border-border rounded-2xl border border-dashed p-12 text-center"
        >
          <p class="text-sm font-medium">{{ emptyTitle }}</p>
          <p class="text-muted-foreground mt-1 text-xs">
            Try adjusting filters or uploading new files.
          </p>
          <Button
            v-if="emptyAction"
            size="sm"
            variant="outline"
            class="mt-3"
            @click="router.push(emptyLink ?? '/documents')"
            >{{ emptyAction }}</Button
          >
        </div>

        <template v-else>
          <div
            v-if="view === 'grid'"
            class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4"
          >
            <DocumentCard
              v-for="d in paginated"
              :key="d.id"
              :doc="d"
              @navigate="navigate"
              @quick-view="quickView"
            />
          </div>
          <DocumentTable
            v-else
            :docs="paginated"
            @navigate="navigate"
            @quick-view="quickView"
            v-model:checked="checked"
          />
        </template>

        <div
          v-if="totalPages > 1 && filtered.length > 0"
          class="mt-6 flex items-center justify-center gap-1"
        >
          <Button
            variant="outline"
            size="sm"
            class="h-8 w-8"
            :disabled="page <= 1"
            @click="page = Math.max(1, page - 1)"
            ><ArrowLeftIcon class="h-3.5 w-3.5"
          /></Button>
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
            ><ArrowLeftIcon class="h-3.5 w-3.5 rotate-180"
          /></Button>
        </div>
      </template>
    </div>

    <Sheet
      :open="!!quickViewDoc"
      @update:open="
        (v: boolean) => {
          if (!v) quickViewDoc = null;
        }
      "
    >
      <SheetContent class="w-full overflow-y-auto p-0 sm:max-w-lg">
        <DocumentDetailPanel v-if="quickViewDoc" :doc="quickViewDoc" @close="quickViewDoc = null" />
      </SheetContent>
    </Sheet>
  </div>
</template>
