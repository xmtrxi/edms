<script setup lang="ts">
  import { FolderPlusIcon, PlusIcon, SearchIcon } from "@lucide/vue";
  import { toast } from "vue-sonner";

  import CategoryDialog from "@/components/categories/CategoryDialog.vue";
  import CategoryTree from "@/components/categories/CategoryTree.vue";
  import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
  } from "@/components/ui/alert-dialog";
  import { Button } from "@/components/ui/button";
  import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
  import { Skeleton } from "@/components/ui/skeleton";
  import type { Category, CategoryInput } from "@/types/api";

  definePageMeta({
    title: "Categories",
    description: "Organize documents with categories and subcategories.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Categories" }],
  });

  useHead({
    title: "Categories — Lumen DMS",
    meta: [{ name: "description", content: "Manage document categories and subcategories." }],
  });

  const service = categoriesService();
  const query = shallowRef("");
  const dialogOpen = shallowRef(false);
  const editing = shallowRef<Category | null>(null);
  const selectedParentId = shallowRef("");
  const deleteTarget = shallowRef<Category | null>(null);

  const parentCategories = computed(() =>
    service.categories.value.filter((category) => !category.parent_id),
  );

  const openCreate = (parent?: Category) => {
    editing.value = null;
    selectedParentId.value = parent?.id ?? "";
    dialogOpen.value = true;
  };

  const openEdit = (category: Category) => {
    editing.value = category;
    selectedParentId.value = category.parent_id ?? "";
    dialogOpen.value = true;
  };

  const saveCategory = async (input: CategoryInput) => {
    const category = editing.value
      ? await service.updateCategory(editing.value.id, input)
      : await service.createCategory(input);
    if (!category) {
      toast.error(service.error.value || "Could not save category");
      return;
    }
    dialogOpen.value = false;
    toast.success(editing.value ? "Category updated" : "Category created");
  };

  const deleteCategory = async () => {
    if (!deleteTarget.value) return;
    const name = deleteTarget.value.name;
    if (!(await service.deleteCategory(deleteTarget.value.id))) {
      toast.error(service.error.value || "Could not delete category");
      return;
    }
    deleteTarget.value = null;
    toast.success(`Deleted ${name}`);
  };

  await service.fetchCategories();
</script>

<template>
  <Teleport to="#page-actions-target">
    <Button size="sm" @click="openCreate()">
      <PlusIcon data-icon="inline-start" /> New category
    </Button>
  </Teleport>

  <div class="mb-5 flex flex-wrap items-center gap-3">
    <InputGroup class="max-w-sm">
      <InputGroupAddon><SearchIcon /></InputGroupAddon>
      <InputGroupInput v-model="query" placeholder="Search categories" />
    </InputGroup>
    <p class="text-muted-foreground text-xs">
      {{ parentCategories.length }} categories ·
      {{ service.categories.value.length - parentCategories.length }} subcategories
    </p>
  </div>

  <div v-if="service.loading.value" class="flex flex-col gap-2">
    <Skeleton v-for="item in 4" :key="item" class="h-16 w-full" />
  </div>

  <div
    v-else-if="service.error.value"
    class="border-border flex flex-col items-center gap-3 rounded-lg border border-dashed py-12 text-center"
  >
    <FolderPlusIcon class="text-muted-foreground size-6" />
    <div>
      <p class="text-sm font-medium">Could not load categories</p>
      <p class="text-muted-foreground mt-1 text-xs">{{ service.error.value }}</p>
    </div>
    <Button variant="outline" size="sm" @click="service.fetchCategories">Try again</Button>
  </div>

  <CategoryTree
    v-else
    :categories="service.categories.value"
    :query="query"
    @create-root="openCreate()"
    @create-child="openCreate"
    @edit="openEdit"
    @delete="deleteTarget = $event"
  />

  <CategoryDialog
    :open="dialogOpen"
    :category="editing"
    :parent-id="selectedParentId"
    :parent-categories="parentCategories"
    :saving="service.saving.value"
    @update:open="dialogOpen = $event"
    @submit="saveCategory"
  />

  <AlertDialog :open="!!deleteTarget" @update:open="!$event && (deleteTarget = null)">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>Delete {{ deleteTarget?.name }}?</AlertDialogTitle>
        <AlertDialogDescription>
          Categories containing documents or subcategories cannot be deleted until those items are
          moved.
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel @click="deleteTarget = null">Cancel</AlertDialogCancel>
        <AlertDialogAction @click="deleteCategory">Delete category</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
