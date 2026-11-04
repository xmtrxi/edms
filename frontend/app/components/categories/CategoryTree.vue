<script setup lang="ts">
  import {
    FolderIcon,
    FolderPlusIcon,
    MoreHorizontalIcon,
    PencilIcon,
    TagIcon,
    Trash2Icon,
  } from "@lucide/vue";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuGroup,
    DropdownMenuItem,
    DropdownMenuTrigger,
  } from "@/components/ui/dropdown-menu";
  import {
    Empty,
    EmptyContent,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
  } from "@/components/ui/empty";
  import type { Category } from "@/types/api";

  const props = defineProps<{
    categories: Category[];
    query: string;
  }>();

  const emit = defineEmits<{
    createRoot: [];
    createChild: [category: Category];
    edit: [category: Category];
    delete: [category: Category];
  }>();

  const roots = computed(() => props.categories.filter((category) => !category.parent_id));
  const childrenByParent = computed(() => {
    const result = new Map<string, Category[]>();
    for (const category of props.categories) {
      if (!category.parent_id) continue;
      const children = result.get(category.parent_id) ?? [];
      children.push(category);
      result.set(category.parent_id, children);
    }
    return result;
  });

  const visibleRoots = computed(() => {
    const query = props.query.trim().toLowerCase();
    if (!query) return roots.value;
    return roots.value.filter((root) => {
      const children = childrenByParent.value.get(root.id) ?? [];
      return [root, ...children].some((category) =>
        `${category.name} ${category.description ?? ""}`.toLowerCase().includes(query),
      );
    });
  });

  const visibleChildren = (parentId: string) => {
    const children = childrenByParent.value.get(parentId) ?? [];
    const query = props.query.trim().toLowerCase();
    if (!query) return children;
    return children.filter((category) =>
      `${category.name} ${category.description ?? ""}`.toLowerCase().includes(query),
    );
  };
</script>

<template>
  <Empty v-if="visibleRoots.length === 0" class="border-border border border-dashed py-12">
    <EmptyHeader>
      <EmptyMedia variant="icon"><TagIcon /></EmptyMedia>
      <EmptyTitle>{{ props.query ? "No matching categories" : "No categories yet" }}</EmptyTitle>
      <EmptyDescription>
        {{
          props.query
            ? "Try a different search term."
            : "Create a category to organize uploaded documents."
        }}
      </EmptyDescription>
    </EmptyHeader>
    <EmptyContent v-if="!props.query">
      <Button size="sm" @click="emit('createRoot')">
        <FolderPlusIcon data-icon="inline-start" /> Create category
      </Button>
    </EmptyContent>
  </Empty>

  <div v-else class="border-border bg-card overflow-hidden rounded-lg border">
    <section v-for="(root, rootIndex) in visibleRoots" :key="root.id">
      <div class="flex min-h-16 items-center gap-3 px-4 py-3">
        <div
          class="bg-secondary text-foreground grid size-9 shrink-0 place-items-center rounded-md"
        >
          <FolderIcon class="size-4" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="truncate text-sm font-semibold">{{ root.name }}</h2>
            <Badge variant="secondary">{{ root.document_count }} documents</Badge>
            <Badge v-if="(childrenByParent.get(root.id)?.length ?? 0) > 0" variant="outline">
              {{ childrenByParent.get(root.id)?.length }} subcategories
            </Badge>
          </div>
          <p v-if="root.description" class="text-muted-foreground mt-0.5 truncate text-xs">
            {{ root.description }}
          </p>
        </div>
        <Button variant="outline" size="sm" @click="emit('createChild', root)">
          <FolderPlusIcon data-icon="inline-start" /> Subcategory
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button variant="ghost" size="icon" :aria-label="`Actions for ${root.name}`">
              <MoreHorizontalIcon />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuGroup>
              <DropdownMenuItem @click="emit('edit', root)"> <PencilIcon /> Edit </DropdownMenuItem>
              <DropdownMenuItem class="text-destructive" @click="emit('delete', root)">
                <Trash2Icon /> Delete
              </DropdownMenuItem>
            </DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <div v-if="visibleChildren(root.id).length" class="bg-secondary/30 px-4 py-2">
        <div
          v-for="child in visibleChildren(root.id)"
          :key="child.id"
          class="border-border flex min-h-12 items-center gap-3 border-t py-2 first:border-t-0"
        >
          <div
            class="bg-background text-muted-foreground grid size-7 shrink-0 place-items-center rounded-md border"
          >
            <TagIcon class="size-3.5" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <p class="truncate text-sm font-medium">{{ child.name }}</p>
              <Badge variant="outline">{{ child.document_count }}</Badge>
            </div>
            <p v-if="child.description" class="text-muted-foreground truncate text-xs">
              {{ child.description }}
            </p>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="icon" :aria-label="`Actions for ${child.name}`">
                <MoreHorizontalIcon />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuGroup>
                <DropdownMenuItem @click="emit('edit', child)"
                  ><PencilIcon /> Edit</DropdownMenuItem
                >
                <DropdownMenuItem class="text-destructive" @click="emit('delete', child)">
                  <Trash2Icon /> Delete
                </DropdownMenuItem>
              </DropdownMenuGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
      <div v-if="rootIndex < visibleRoots.length - 1" class="border-border border-t" />
    </section>
  </div>
</template>
