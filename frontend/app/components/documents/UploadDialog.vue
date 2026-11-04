<script setup lang="ts">
  import { UploadIcon } from "@lucide/vue";
  import { onBeforeUnmount, ref, shallowRef, watch } from "vue";

  import UploadAccessPicker from "@/components/sharing/UploadAccessPicker.vue";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
  } from "@/components/ui/dialog";
  import { Field, FieldLabel } from "@/components/ui/field";
  import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectLabel,
    SelectTrigger,
    SelectValue,
  } from "@/components/ui/select";
  import { cn } from "@/lib/utils";
  import type { UploadAccess } from "@/types/api";

  const props = withDefaults(defineProps<{ open: boolean; initialFolderId?: string }>(), {
    initialFolderId: "",
  });
  const emit = defineEmits<{ "update:open": [value: boolean]; uploaded: [id: string] }>();

  const uploads = useUploadStore();
  const categoryService = categoriesService();
  const { categories } = categoryService;
  const folderService = foldersService();
  const { folders } = folderService;

  const dragOver = shallowRef(false);
  const input = ref<HTMLInputElement | null>(null);
  const selectedCategoryId = shallowRef("none");
  const selectedFolderId = shallowRef("none");
  const access = ref<UploadAccess>({ mode: "inherit", sensitivity: "internal", grants: [] });

  const parentCategories = computed(() => categories.value.filter((item) => !item.parent_id));
  const subcategories = computed(() => categories.value.filter((item) => !!item.parent_id));
  const selectedFolder = computed(() =>
    folders.value.find((folder) => folder.id === selectedFolderId.value),
  );
  // Only folders the user can add to are offered as destinations.
  const uploadableFolders = computed(() =>
    folders.value.filter((folder) => folder.can_upload !== false),
  );

  watch(
    () => props.open,
    (open) => {
      if (!open) return;
      selectedFolderId.value = props.initialFolderId || "none";
      if (!categoryService.loaded.value) void categoryService.fetchCategories();
      void folderService.fetchFolders();
    },
    { immediate: true },
  );

  const stop = uploads.onUploaded((document) => emit("uploaded", document.id));
  onBeforeUnmount(stop);

  const addFiles = (files: FileList | File[]) => {
    if (!files.length) return;
    uploads.add(files, {
      folderId: selectedFolderId.value !== "none" ? selectedFolderId.value : undefined,
      categoryId: selectedCategoryId.value !== "none" ? selectedCategoryId.value : undefined,
      access: JSON.parse(JSON.stringify(access.value)) as UploadAccess,
    });
    emit("update:open", false);
  };

  const onInputChange = () => {
    if (input.value?.files?.length) {
      addFiles(input.value.files);
      input.value.value = "";
    }
  };
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
      <DialogHeader>
        <DialogTitle>Upload files</DialogTitle>
        <DialogDescription
          >Choose where files go and who can see them, then drop them in. Large files resume if your
          connection drops.</DialogDescription
        >
      </DialogHeader>

      <div class="space-y-4">
        <Field>
          <FieldLabel>Destination</FieldLabel>
          <Select v-model="selectedFolderId">
            <SelectTrigger class="w-full"
              ><SelectValue placeholder="Choose a folder"
            /></SelectTrigger>
            <SelectContent>
              <SelectItem value="none">My files</SelectItem>
              <SelectItem v-for="folder in uploadableFolders" :key="folder.id" :value="folder.id">
                {{ "— ".repeat(folder.depth) }}{{ folder.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </Field>

        <UploadAccessPicker v-model="access" :folder="selectedFolder" />

        <Field>
          <FieldLabel>Category</FieldLabel>
          <Select v-model="selectedCategoryId">
            <SelectTrigger class="w-full"
              ><SelectValue placeholder="Uncategorized"
            /></SelectTrigger>
            <SelectContent>
              <SelectGroup><SelectItem value="none">Uncategorized</SelectItem></SelectGroup>
              <SelectGroup v-if="parentCategories.length">
                <SelectLabel>Categories</SelectLabel>
                <SelectItem
                  v-for="category in parentCategories"
                  :key="category.id"
                  :value="category.id"
                >
                  {{ category.name }}
                </SelectItem>
              </SelectGroup>
              <SelectGroup v-if="subcategories.length">
                <SelectLabel>Subcategories</SelectLabel>
                <SelectItem
                  v-for="category in subcategories"
                  :key="category.id"
                  :value="category.id"
                >
                  {{ parentCategories.find((parent) => parent.id === category.parent_id)?.name }} /
                  {{ category.name }}
                </SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>

        <input ref="input" type="file" multiple class="hidden" @change="onInputChange" />
        <button
          type="button"
          :class="
            cn(
              'w-full cursor-pointer rounded-xl border-2 border-dashed p-10 text-center transition-all',
              dragOver
                ? 'border-primary bg-primary/5'
                : 'border-border hover:border-primary/50 hover:bg-secondary/40',
            )
          "
          @dragover.prevent="dragOver = true"
          @dragleave="dragOver = false"
          @drop.prevent="
            dragOver = false;
            addFiles(($event as DragEvent).dataTransfer?.files ?? []);
          "
          @click="input?.click()"
        >
          <span
            class="bg-primary/10 text-primary ring-primary/20 mx-auto grid h-14 w-14 place-items-center rounded-2xl ring-1"
          >
            <UploadIcon class="h-6 w-6" />
          </span>
          <span class="mt-4 block text-sm font-medium">Drop files here or click to browse</span>
          <span class="text-muted-foreground mt-1 block text-xs"
            >Any file type, up to 10 GB each</span
          >
        </button>
      </div>
    </DialogContent>
  </Dialog>
</template>
