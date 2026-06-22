<script setup lang="ts">
  import {
    PlusIcon,
    TagIcon,
    Trash2Icon,
    FileTextIcon,
    SearchIcon,
    XIcon,
    PencilIcon,
    CheckIcon,
  } from "@lucide/vue";
  import { ref, computed } from "vue";
  import { toast } from "vue-sonner";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Card, CardContent } from "@/components/ui/card";
  import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
    DialogFooter,
  } from "@/components/ui/dialog";
  import { Input } from "@/components/ui/input";
  import { Separator } from "@/components/ui/separator";
  import { Textarea } from "@/components/ui/textarea";
  import { documents } from "@/data/mock-data";
  import {
    useCategories,
    categoriesStore,
    COLOR_OPTIONS,
    colorStyle,
  } from "@/lib/categories-store";

  definePageMeta({
    title: "Categories",
    description:
      "Create and curate dynamic categories. Tag documents to power instant search and workflows.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Categories" }],
  });

  useHead({
    title: "Categories — Lumen DMS",
    meta: [
      {
        name: "description",
        content: "Dynamic categories and tags for documents — power your search index.",
      },
    ],
  });

  const cats = useCategories();
  const open = ref(false);
  const editOpen = ref(false);
  const manageOpen = ref(false);
  const name = ref("");
  const color = ref(COLOR_OPTIONS[0].name);
  const desc = ref("");
  const filter = ref("");
  const editingCat = ref<(typeof cats.value)[number] | null>(null);
  const managingCat = ref<(typeof cats.value)[number] | null>(null);
  const docSearch = ref("");

  const filtered = computed(() =>
    cats.filter((c) => c.name.toLowerCase().includes(filter.value.toLowerCase())),
  );

  function create() {
    if (!name.value.trim()) {
      toast.error("Category name required");
      return;
    }
    categoriesStore.add(name.value.trim(), color.value, desc.value.trim() || undefined);
    toast.success(`Category "${name.value}" created`);
    open.value = false;
    resetForm();
  }

  function resetForm() {
    name.value = "";
    desc.value = "";
    color.value = COLOR_OPTIONS[0].name;
  }

  function remove(id: string, catName: string) {
    categoriesStore.remove(id);
    toast.success(`Removed "${catName}"`);
  }

  function openEdit(cat: (typeof cats.value)[number]) {
    editingCat.value = { ...cat };
    name.value = cat.name;
    desc.value = cat.description ?? "";
    color.value = cat.color;
    editOpen.value = true;
  }

  function saveEdit() {
    if (!editingCat.value || !name.value.trim()) {
      toast.error("Category name required");
      return;
    }
    categoriesStore.update(editingCat.value.id, {
      name: name.value.trim(),
      description: desc.value.trim() || undefined,
      color: color.value,
    });
    toast.success(`Category updated`);
    editOpen.value = false;
    editingCat.value = null;
  }

  function openManage(cat: (typeof cats.value)[number]) {
    managingCat.value = { ...cat };
    docSearch.value = "";
    manageOpen.value = true;
  }

  function toggleDocInCat(docId: string) {
    if (!managingCat.value) return;
    categoriesStore.toggleDoc(managingCat.value.id, docId);
    const has = managingCat.value.docIds.includes(docId);
    if (has) {
      managingCat.value.docIds = managingCat.value.docIds.filter((d) => d !== docId);
    } else {
      managingCat.value.docIds = [...managingCat.value.docIds, docId];
    }
  }

  const availableDocs = computed(() => {
    const q = docSearch.value.toLowerCase();
    return documents.filter(
      (d) =>
        !d.trashed &&
        (q
          ? d.name.toLowerCase().includes(q) || (d.owner && d.owner.toLowerCase().includes(q))
          : true),
    );
  });

  function docInCategory(cat: (typeof cats.value)[number], docId: string) {
    return cat.docIds.includes(docId);
  }
</script>

<template>
  <Teleport to="#page-actions-target">
    <div class="relative">
      <SearchIcon
        class="text-muted-foreground absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2"
      />
      <Input v-model="filter" placeholder="Filter&hellip;" class="h-9 w-48 pr-8 pl-8" />
      <button
        v-if="filter"
        @click="filter = ''"
        class="hover:bg-secondary text-muted-foreground hover:text-foreground absolute top-1/2 right-2 grid h-4 w-4 -translate-y-1/2 place-items-center rounded transition-colors"
      >
        <XIcon class="h-3 w-3" />
      </button>
    </div>
    <Button size="sm" class="gap-1.5" @click="open = true">
      <PlusIcon class="h-4 w-4" /> New category
    </Button>
  </Teleport>

  <div
    v-if="filtered.length === 0"
    class="border-border rounded-2xl border border-dashed p-12 text-center"
  >
    <TagIcon class="text-muted-foreground mx-auto h-6 w-6" />
    <p class="mt-2 text-sm font-medium">
      {{ filter ? "No categories match your filter" : "No categories yet" }}
    </p>
    <p class="text-muted-foreground mt-1 text-xs">
      {{
        filter
          ? "Try a different search term."
          : "Create your first category to start tagging documents."
      }}
    </p>
    <Button v-if="filter" variant="outline" size="sm" class="mt-3" @click="filter = ''"
      >Clear filter</Button
    >
    <Button v-else size="sm" class="mt-3 gap-1.5" @click="open = true"
      ><PlusIcon class="h-3.5 w-3.5" /> Create category</Button
    >
  </div>

  <div v-else class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
    <Card
      v-for="c in filtered"
      :key="c.id"
      class="border-border hover:border-primary/30 group cursor-pointer shadow-sm transition-all hover:shadow-md"
      @click="openManage(c)"
    >
      <CardContent class="p-4">
        <div class="flex items-start justify-between">
          <div class="flex items-center gap-2.5">
            <span
              class="grid h-9 w-9 place-items-center rounded-xl border"
              :style="colorStyle(c.color)"
            >
              <TagIcon class="h-4 w-4" />
            </span>
            <div>
              <h3 class="text-sm font-semibold">{{ c.name }}</h3>
              <p class="text-muted-foreground text-[11px]">
                {{ c.docIds.length }} documents · created {{ c.createdAt }}
              </p>
            </div>
          </div>
          <div class="flex items-center gap-0.5" @click.stop>
            <Button variant="ghost" size="icon" class="h-7 w-7" @click="openEdit(c)"
              ><PencilIcon class="text-muted-foreground hover:text-foreground h-3.5 w-3.5"
            /></Button>
            <Button variant="ghost" size="icon" class="h-7 w-7" @click="remove(c.id, c.name)"
              ><Trash2Icon class="text-muted-foreground hover:text-destructive h-3.5 w-3.5"
            /></Button>
          </div>
        </div>
        <p v-if="c.description" class="text-muted-foreground mt-2.5 text-xs leading-relaxed">
          {{ c.description }}
        </p>
        <div v-if="c.docIds.length > 0" class="border-border mt-3 border-t pt-3">
          <div
            v-for="d in c.docIds
              .slice(0, 3)
              .map((id) => documents.find((doc) => doc.id === id))
              .filter(Boolean)"
            :key="d!.id"
            class="-mx-1 flex items-center gap-2 rounded-md px-2 py-1.5"
            :class="d!.trashed ? 'opacity-50' : ''"
          >
            <FileTextIcon class="text-muted-foreground h-3 w-3 shrink-0" />
            <span class="truncate text-xs">{{ d!.name }}</span>
          </div>
          <p v-if="c.docIds.length > 3" class="text-muted-foreground mt-1 pl-1 text-[10px]">
            +{{ c.docIds.length - 3 }} more
          </p>
        </div>
        <div v-else class="border-border mt-3 border-t pt-3">
          <p class="text-muted-foreground/60 py-1 text-center text-[10px]">No documents assigned</p>
        </div>
      </CardContent>
    </Card>
  </div>

  <!-- Create dialog -->
  <Dialog :open="open" @update:open="(v: boolean) => (open = v)">
    <DialogContent>
      <DialogHeader><DialogTitle>Create category</DialogTitle></DialogHeader>
      <div class="space-y-4 pt-2">
        <div class="space-y-1.5">
          <label class="text-xs font-medium">Name</label>
          <Input v-model="name" placeholder="e.g. Procurement, Tax 2026, Board Materials" />
        </div>
        <div class="space-y-1.5">
          <label class="text-xs font-medium">Description (optional)</label>
          <Textarea v-model="desc" placeholder="What belongs here?" class="min-h-20" />
        </div>
        <div class="space-y-1.5">
          <label class="text-xs font-medium">Color</label>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="opt in COLOR_OPTIONS"
              :key="opt.name"
              @click="color = opt.name"
              :class="
                color === opt.name
                  ? 'ring-offset-background scale-110 ring-2 ring-offset-2'
                  : 'border-transparent'
              "
              class="h-7 w-7 rounded-md border-2 transition-all"
              :style="{
                backgroundColor: opt.hex,
                borderColor: color === opt.name ? opt.hex : 'transparent',
              }"
              :aria-label="opt.name"
            />
          </div>
        </div>
      </div>
      <DialogFooter>
        <Button variant="outline" @click="open = false">Cancel</Button>
        <Button @click="create">Create</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <!-- Edit dialog -->
  <Dialog :open="editOpen" @update:open="editOpen = $event">
    <DialogContent>
      <DialogHeader><DialogTitle>Edit category</DialogTitle></DialogHeader>
      <div class="space-y-4 pt-2">
        <div class="space-y-1.5">
          <label class="text-xs font-medium">Name</label>
          <Input v-model="name" placeholder="Category name" />
        </div>
        <div class="space-y-1.5">
          <label class="text-xs font-medium">Description</label>
          <Textarea v-model="desc" placeholder="What belongs here?" class="min-h-20" />
        </div>
        <div class="space-y-1.5">
          <label class="text-xs font-medium">Color</label>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="opt in COLOR_OPTIONS"
              :key="opt.name"
              @click="color = opt.name"
              :class="
                color === opt.name
                  ? 'ring-offset-background scale-110 ring-2 ring-offset-2'
                  : 'border-transparent'
              "
              class="h-7 w-7 rounded-md border-2 transition-all"
              :style="{
                backgroundColor: opt.hex,
                borderColor: color === opt.name ? opt.hex : 'transparent',
              }"
              :aria-label="opt.name"
            />
          </div>
        </div>
      </div>
      <DialogFooter>
        <Button variant="outline" @click="editOpen = false">Cancel</Button>
        <Button @click="saveEdit"><CheckIcon class="mr-1.5 h-3.5 w-3.5" /> Save</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <!-- Manage documents in category -->
  <Dialog :open="manageOpen" @update:open="manageOpen = $event">
    <DialogContent class="max-w-xl">
      <DialogHeader v-if="managingCat">
        <div class="flex items-center gap-2.5">
          <span
            class="grid h-8 w-8 place-items-center rounded-lg border text-xs"
            :style="colorStyle(managingCat.color)"
          >
            <TagIcon class="h-3.5 w-3.5" />
          </span>
          <div>
            <DialogTitle class="text-base">{{ managingCat.name }}</DialogTitle>
            <DialogDescription>{{
              managingCat.description ?? "Manage assigned documents"
            }}</DialogDescription>
          </div>
        </div>
      </DialogHeader>
      <div v-if="managingCat" class="space-y-3">
        <div class="relative">
          <SearchIcon
            class="text-muted-foreground absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2"
          />
          <Input
            v-model="docSearch"
            placeholder="Search documents to assign&hellip;"
            class="h-9 pl-8"
          />
        </div>

        <div class="border-border/60 max-h-72 space-y-0.5 overflow-y-auto rounded-lg border">
          <div
            v-for="d in availableDocs"
            :key="d.id"
            @click="toggleDocInCat(d.id)"
            :class="
              docInCategory(managingCat, d.id)
                ? 'bg-primary/5 border-primary/20'
                : 'hover:bg-secondary/30'
            "
            class="border-border/30 flex cursor-pointer items-center gap-2.5 border-b px-3 py-2 transition-colors last:border-0"
          >
            <div
              :class="
                docInCategory(managingCat, d.id)
                  ? 'bg-primary border-primary'
                  : 'border-border bg-secondary/40'
              "
              class="flex h-4 w-4 shrink-0 items-center justify-center rounded border"
            >
              <CheckIcon
                v-if="docInCategory(managingCat, d.id)"
                class="text-primary-foreground h-3 w-3"
              />
            </div>
            <FileTextIcon class="text-muted-foreground h-3.5 w-3.5 shrink-0" />
            <div class="min-w-0 flex-1">
              <p class="truncate text-xs font-medium">{{ d.name }}</p>
              <p class="text-muted-foreground text-[10px]">
                {{ d.owner }} · {{ d.type.toUpperCase() }} · {{ d.modified }}
              </p>
            </div>
            <Badge v-if="d.trashed" variant="secondary" class="border-0 text-[10px]">trashed</Badge>
          </div>
          <div
            v-if="availableDocs.length === 0"
            class="text-muted-foreground py-6 text-center text-xs"
          >
            No documents found
          </div>
        </div>

        <Separator />

        <div class="text-muted-foreground flex items-center justify-between text-xs">
          <span>{{ managingCat.docIds.length }} documents assigned</span>
          <Button variant="outline" size="sm" class="h-7 gap-1 text-xs" @click="manageOpen = false">
            <CheckIcon class="h-3 w-3" /> Done
          </Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
