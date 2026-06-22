<script setup lang="ts">
  import {
    FolderOpenIcon,
    FolderIcon,
    SearchIcon,
    XIcon,
    FileTextIcon,
    ChevronRightIcon,
    Grid3X3Icon,
  } from "@lucide/vue";
  import { ref, computed } from "vue";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { documents, type Doc } from "@/data/mock-data";

  const emit = defineEmits<{
    selectFolder: [folder: string];
    selectDoc: [doc: Doc];
  }>();

  const query = ref("");
  const selectedFolder = ref<string | null>(null);

  const folders = computed(() => {
    const folderMap = new Map<string, Doc[]>();
    for (const d of documents) {
      const f = d.folder || "Uncategorized";
      if (!folderMap.has(f)) folderMap.set(f, []);
      folderMap.get(f)!.push(d);
    }
    return Array.from(folderMap.entries())
      .map(([name, docs]) => ({ name, docs, count: docs.length }))
      .sort((a, b) => a.name.localeCompare(b.name));
  });

  const filteredFolders = computed(() => {
    if (!query.value.trim()) return folders.value;
    const q = query.value.toLowerCase();
    return folders.value.filter(
      (f) =>
        f.name.toLowerCase().includes(q) || f.docs.some((d) => d.name.toLowerCase().includes(q)),
    );
  });

  const selectedFolderDocs = computed(() => {
    if (!selectedFolder.value) return [];
    return documents.filter((d) => (d.folder || "Uncategorized") === selectedFolder.value);
  });

  const folderDocCounts = computed(() => {
    const counts = new Map<string, number>();
    for (const d of documents) {
      const f = d.folder || "Uncategorized";
      counts.set(f, (counts.get(f) || 0) + 1);
    }
    return counts;
  });

  function selectFolder(folder: string) {
    selectedFolder.value = folder;
    emit("selectFolder", folder);
  }

  function backToFolders() {
    selectedFolder.value = null;
  }

  const typeColors: Record<string, string> = {
    pdf: "text-red-500",
    docx: "text-blue-600",
    xlsx: "text-emerald-600",
    pptx: "text-orange-500",
    image: "text-fuchsia-500",
    txt: "text-stone-500",
  };

  const folderIcons: Record<string, string> = {
    Legal: "text-violet-500",
    Finance: "text-emerald-500",
    HR: "text-rose-500",
    Engineering: "text-sky-500",
  };
</script>

<template>
  <div v-if="!selectedFolder" class="space-y-4">
    <div class="relative">
      <SearchIcon class="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2" />
      <Input
        v-model="query"
        placeholder="Search folders…"
        class="bg-secondary/60 focus-visible:bg-card focus-visible:border-border h-9 rounded-lg border-transparent pl-9"
      />
      <button
        v-if="query"
        @click="query = ''"
        class="hover:bg-secondary text-muted-foreground hover:text-foreground absolute top-1/2 right-2 grid h-5 w-5 -translate-y-1/2 place-items-center rounded"
      >
        <XIcon class="h-3 w-3" />
      </button>
    </div>

    <div
      v-if="filteredFolders.length === 0"
      class="border-border rounded-2xl border border-dashed p-12 text-center"
    >
      <FolderOpenIcon class="text-muted-foreground/40 mx-auto h-10 w-10" />
      <p class="mt-2 text-sm font-medium">No folders found</p>
      <p class="text-muted-foreground mt-1 text-xs">Try a different search term.</p>
    </div>

    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
      <button
        v-for="f in filteredFolders"
        :key="f.name"
        @click="selectFolder(f.name)"
        class="group border-border bg-card shadow-elegant hover:shadow-glow hover:border-primary/30 rounded-xl border p-4 text-left transition-all hover:-translate-y-0.5"
      >
        <div class="flex items-start gap-3">
          <div
            class="bg-secondary grid h-12 w-12 shrink-0 place-items-center rounded-xl"
            :class="folderIcons[f.name] || 'text-muted-foreground'"
          >
            <FolderOpenIcon class="h-6 w-6" />
          </div>
          <div class="min-w-0 flex-1">
            <h3 class="truncate text-sm font-semibold">{{ f.name }}</h3>
            <p class="text-muted-foreground mt-0.5 text-xs">
              {{ f.count }} document{{ f.count > 1 ? "s" : "" }}
            </p>
            <div class="mt-2 flex flex-wrap gap-1">
              <Badge
                v-for="(d, i) in f.docs.slice(0, 3)"
                :key="d.id"
                variant="secondary"
                class="bg-secondary/60 gap-1 border-0 text-[10px]"
              >
                <FileTextIcon class="h-2.5 w-2.5" :class="typeColors[d.type]" />
                <span class="max-w-[80px] truncate">{{ d.name }}</span>
              </Badge>
              <Badge
                v-if="f.docs.length > 3"
                variant="secondary"
                class="bg-secondary/60 border-0 text-[10px]"
              >
                +{{ f.docs.length - 3 }} more
              </Badge>
            </div>
          </div>
          <ChevronRightIcon
            class="text-muted-foreground/40 group-hover:text-muted-foreground mt-1 h-4 w-4 shrink-0 transition-colors"
          />
        </div>
      </button>
    </div>
  </div>

  <div v-else class="space-y-4">
    <div class="flex items-center gap-2">
      <Button variant="ghost" size="sm" class="h-8 gap-1.5" @click="backToFolders">
        <ChevronRightIcon class="h-4 w-4 rotate-180" /> All folders
      </Button>
      <div class="flex items-center gap-2">
        <div class="bg-border h-3 w-px" />
        <FolderOpenIcon class="text-primary h-4 w-4" />
        <h3 class="text-sm font-semibold">{{ selectedFolder }}</h3>
        <Badge variant="secondary" class="border-0 text-[10px]"
          >{{ selectedFolderDocs.length }} documents</Badge
        >
      </div>
    </div>

    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
      <button
        v-for="d in selectedFolderDocs"
        :key="d.id"
        @click="emit('selectDoc', d)"
        class="group border-border bg-card shadow-elegant hover:shadow-glow hover:border-primary/30 rounded-xl border p-3 text-left transition-all"
      >
        <div class="flex items-start gap-3">
          <FileTextIcon
            class="mt-0.5 h-8 w-8 shrink-0"
            :class="typeColors[d.type] || 'text-muted-foreground'"
          />
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium">{{ d.name }}</p>
            <p class="text-muted-foreground mt-0.5 text-xs">
              {{ d.size }} &middot; v{{ d.versions }}
            </p>
            <div class="mt-2 flex flex-wrap gap-1">
              <Badge
                v-for="t in d.tags.slice(0, 2)"
                :key="t"
                variant="secondary"
                class="bg-secondary/60 border-0 text-[10px]"
                >{{ t }}</Badge
              >
            </div>
          </div>
        </div>
      </button>
    </div>

    <div
      v-if="selectedFolderDocs.length === 0"
      class="border-border rounded-2xl border border-dashed p-12 text-center"
    >
      <FileTextIcon class="text-muted-foreground/40 mx-auto h-8 w-8" />
      <p class="mt-1 text-sm font-medium">No documents in this folder</p>
    </div>
  </div>
</template>
