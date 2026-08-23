<script setup lang="ts">
  import {
    AlertCircleIcon,
    CheckCircle2Icon,
    FileIcon,
    FileSpreadsheetIcon,
    FileTextIcon,
    ImageIcon,
    PresentationIcon,
    UploadIcon,
    XIcon,
  } from "@lucide/vue";
  import { ref } from "vue";
  import { toast } from "vue-sonner";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
  } from "@/components/ui/dialog";
  import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
  import { Progress } from "@/components/ui/progress";
  import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectLabel,
    SelectTrigger,
    SelectValue,
  } from "@/components/ui/select";
  import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
  import { cn } from "@/lib/utils";
  import { formatBytes } from "@/utils/format";

  const props = defineProps<{ open: boolean }>();
  const emit = defineEmits<{ "update:open": [value: boolean]; uploaded: [id: string] }>();

  interface QueueItem {
    id: string;
    file: File;
    progress: number;
    stage: "uploading" | "done" | "failed";
    categoryId?: string;
    categoryName?: string;
    error?: string;
  }

  const service = documentsService();
  const categoryService = categoriesService();
  const { categories } = categoryService;
  const queue = ref<QueueItem[]>([]);
  const dragOver = ref(false);
  const tab = ref<"upload" | "progress">("upload");
  const input = ref<HTMLInputElement | null>(null);
  const uploading = ref(false);
  const selectedCategoryId = shallowRef("none");

  const parentCategories = computed(() =>
    categories.value.filter((category) => !category.parent_id),
  );
  const subcategories = computed(() => categories.value.filter((category) => !!category.parent_id));
  const selectedCategory = computed(() =>
    categories.value.find((category) => category.id === selectedCategoryId.value),
  );

  watch(
    () => props.open,
    (open) => {
      if (open && !categoryService.loaded.value) void categoryService.fetchCategories();
    },
    { immediate: true },
  );

  const addFiles = (files: FileList | File[]) => {
    const items: QueueItem[] = Array.from(files).map((file) => ({
      id: `${Date.now()}-${file.name}`,
      file,
      progress: 0,
      stage: "uploading",
      categoryId: selectedCategory.value?.id,
      categoryName: selectedCategory.value?.name,
    }));
    queue.value = [...queue.value, ...items];
    tab.value = "progress";
    if (!uploading.value) void uploadAll();
  };

  const onInputChange = () => {
    if (input.value?.files?.length) {
      addFiles(input.value.files);
      input.value.value = "";
    }
  };

  const uploadAll = async () => {
    uploading.value = true;
    try {
      const pending = queue.value.filter((q) => q.stage === "uploading");
      for (const item of pending) {
        item.progress = 15;
        const doc = await service.upload(item.file, item.file.name, item.categoryId);
        if (doc) {
          item.progress = 100;
          item.stage = "done";
          emit("uploaded", doc.id);
          toast.success(`Processed ${doc.name}`);
        } else {
          item.stage = "failed";
          item.error = service.error;
          toast.error(`Failed to upload ${item.file.name}`);
        }
      }
    } finally {
      uploading.value = false;
    }
  };

  const removeItem = (id: string) => {
    queue.value = queue.value.filter((q) => q.id !== id);
  };

  const fileIcon = (name: string) => {
    if (name.endsWith(".pdf")) return FileTextIcon;
    if (name.endsWith(".xlsx")) return FileSpreadsheetIcon;
    if (name.endsWith(".pptx")) return PresentationIcon;
    if (name.match(/\.(png|jpg|jpeg|gif|svg|webp)/i)) return ImageIcon;
    return FileIcon;
  };

  const pendingCount = computed(() => queue.value.filter((q) => q.stage === "uploading").length);
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-xl">
      <DialogHeader>
        <DialogTitle>Upload documents</DialogTitle>
        <DialogDescription
          >Files are uploaded to your library and processed for OCR.</DialogDescription
        >
      </DialogHeader>

      <Tabs v-model="tab">
        <TabsList class="grid w-full grid-cols-2">
          <TabsTrigger value="upload" class="gap-1.5"
            ><UploadIcon class="h-3.5 w-3.5" /> Upload files</TabsTrigger
          >
          <TabsTrigger value="progress" class="relative gap-1.5">
            <span>Queue</span>
            <Badge
              v-if="queue.length"
              variant="secondary"
              class="bg-primary/10 text-primary ml-1 h-5 border-0 px-1.5 text-[10px]"
              >{{ queue.length }}</Badge
            >
          </TabsTrigger>
        </TabsList>

        <TabsContent value="upload" class="mt-4 space-y-4">
          <input ref="input" type="file" multiple class="hidden" @change="onInputChange" />
          <div
            @dragover.prevent="dragOver = true"
            @dragleave="dragOver = false"
            @drop.prevent="
              dragOver = false;
              addFiles(($event as DragEvent).dataTransfer?.files ?? []);
            "
            @click="input?.click()"
            :class="
              cn(
                'cursor-pointer rounded-xl border-2 border-dashed p-10 text-center transition-all',
                dragOver
                  ? 'border-primary bg-primary/5 scale-[1.01]'
                  : 'border-border hover:border-primary/50 hover:bg-secondary/40',
              )
            "
          >
            <div
              class="bg-primary/10 text-primary ring-primary/20 mx-auto grid h-14 w-14 place-items-center rounded-2xl ring-1"
            >
              <UploadIcon class="h-6 w-6" />
            </div>
            <p class="mt-4 text-sm font-medium">Drop files here or click to browse</p>
            <p class="text-muted-foreground mt-1 text-xs">
              PDF, DOCX, XLSX, PPTX, images &middot; up to 250 MB each
            </p>
          </div>

          <Field>
            <FieldLabel>Category</FieldLabel>
            <Select v-model="selectedCategoryId">
              <SelectTrigger class="w-full">
                <SelectValue placeholder="Select a category" />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value="none">Uncategorized</SelectItem>
                </SelectGroup>
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
                    {{ parentCategories.find((parent) => parent.id === category.parent_id)?.name }}
                    / {{ category.name }}
                  </SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
            <FieldDescription>
              Every file added in this batch will be assigned to this category.
            </FieldDescription>
          </Field>

          <div class="grid grid-cols-2 gap-2">
            <div
              v-for="fmt in [
                { label: 'PDF', icon: FileTextIcon, desc: 'Documents, contracts' },
                { label: 'Excel', icon: FileSpreadsheetIcon, desc: 'Spreadsheets' },
                { label: 'PowerPoint', icon: PresentationIcon, desc: 'Presentations' },
                { label: 'Images', icon: ImageIcon, desc: 'PNG, JPG, WebP' },
              ]"
              :key="fmt.label"
              class="border-border bg-secondary/30 flex items-center gap-2 rounded-lg border px-3 py-2"
            >
              <component :is="fmt.icon" class="text-muted-foreground h-4 w-4" />
              <div>
                <p class="text-xs font-medium">{{ fmt.label }}</p>
                <p class="text-muted-foreground text-[10px]">{{ fmt.desc }}</p>
              </div>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="progress" class="mt-4 space-y-3">
          <div
            v-if="queue.length === 0"
            class="border-border rounded-xl border border-dashed p-8 text-center"
          >
            <UploadIcon class="text-muted-foreground/40 mx-auto h-8 w-8" />
            <p class="text-muted-foreground mt-2 text-sm">No files in queue</p>
          </div>
          <div v-else class="max-h-80 space-y-2 overflow-y-auto pr-1">
            <div
              v-for="q in queue"
              :key="q.id"
              class="border-border bg-card rounded-lg border p-3.5"
            >
              <div class="flex items-center gap-3">
                <div
                  class="bg-secondary text-muted-foreground grid h-10 w-10 shrink-0 place-items-center rounded-lg"
                >
                  <component :is="fileIcon(q.file.name)" class="h-5 w-5" />
                </div>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center justify-between gap-2">
                    <span class="truncate text-sm font-medium">{{ q.file.name }}</span>
                    <span class="text-muted-foreground shrink-0 text-[11px]">{{
                      formatBytes(q.file.size)
                    }}</span>
                  </div>
                  <p class="text-muted-foreground mt-0.5 text-[10px]">
                    {{ q.categoryName ? `Category: ${q.categoryName}` : "Uncategorized" }}
                  </p>
                  <div class="mt-2 flex items-center gap-2">
                    <Progress :model-value="q.progress" class="h-1.5 flex-1" />
                    <Badge
                      variant="secondary"
                      :class="
                        cn(
                          'bg-secondary h-5 gap-1 border-0 px-1.5 text-[10px] font-medium',
                          q.stage === 'done'
                            ? 'text-success bg-success/10'
                            : q.stage === 'failed'
                              ? 'text-destructive bg-destructive/10'
                              : 'text-info bg-info/10',
                        )
                      "
                    >
                      <component
                        :is="
                          q.stage === 'done'
                            ? CheckCircle2Icon
                            : q.stage === 'failed'
                              ? AlertCircleIcon
                              : UploadIcon
                        "
                        class="h-3 w-3"
                      />
                      {{
                        q.stage === "done"
                          ? "Processed"
                          : q.stage === "failed"
                            ? "Failed"
                            : "Uploading"
                      }}
                    </Badge>
                  </div>
                  <p v-if="q.error" class="text-destructive mt-1 text-[11px]">{{ q.error }}</p>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  class="h-7 w-7 shrink-0"
                  aria-label="Remove"
                  @click="removeItem(q.id)"
                >
                  <XIcon class="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          </div>
        </TabsContent>
      </Tabs>

      <DialogFooter>
        <Button variant="ghost" @click="emit('update:open', false)">Close</Button>
        <Button :disabled="uploading || pendingCount === 0" @click="emit('update:open', false)"
          >Done</Button
        >
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
