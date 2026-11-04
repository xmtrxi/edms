<script setup lang="ts">
  import { LoaderCircleIcon } from "@lucide/vue";

  import { Button } from "@/components/ui/button";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
  } from "@/components/ui/dialog";
  import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
  import { Input } from "@/components/ui/input";
  import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectTrigger,
    SelectValue,
  } from "@/components/ui/select";
  import { Textarea } from "@/components/ui/textarea";
  import type { Category, CategoryInput } from "@/types/api";

  const props = defineProps<{
    open: boolean;
    category?: Category | null;
    parentId?: string;
    parentCategories: Category[];
    saving: boolean;
  }>();

  const emit = defineEmits<{
    "update:open": [value: boolean];
    submit: [input: CategoryInput];
  }>();

  const name = shallowRef("");
  const description = shallowRef("");
  const selectedParent = shallowRef("none");
  const color = shallowRef("blue");
  const submitted = shallowRef(false);

  const isEditing = computed(() => !!props.category);
  const title = computed(() =>
    isEditing.value
      ? "Edit category"
      : selectedParent.value !== "none"
        ? "New subcategory"
        : "New category",
  );

  watch(
    () => props.open,
    (open) => {
      if (!open) return;
      submitted.value = false;
      name.value = props.category?.name ?? "";
      description.value = props.category?.description ?? "";
      selectedParent.value = props.category?.parent_id || props.parentId || "none";
      color.value = props.category?.color || "blue";
    },
  );

  const submit = () => {
    submitted.value = true;
    if (!name.value.trim()) return;
    emit("submit", {
      name: name.value.trim(),
      description: description.value.trim() || undefined,
      parent_id: selectedParent.value === "none" ? undefined : selectedParent.value,
      color: color.value,
    });
  };
</script>

<template>
  <Dialog :open="props.open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ title }}</DialogTitle>
        <DialogDescription>
          Use a parent category to organize this item as a subcategory.
        </DialogDescription>
      </DialogHeader>

      <FieldGroup>
        <Field :data-invalid="submitted && !name.trim()">
          <FieldLabel for="category-name">Name</FieldLabel>
          <Input
            id="category-name"
            v-model="name"
            placeholder="e.g. Finance"
            :aria-invalid="submitted && !name.trim()"
            autofocus
            @keydown.enter="submit"
          />
          <FieldDescription v-if="submitted && !name.trim()">A name is required.</FieldDescription>
        </Field>

        <Field>
          <FieldLabel for="category-description">Description</FieldLabel>
          <Textarea
            id="category-description"
            v-model="description"
            placeholder="What documents belong here?"
            class="min-h-20"
          />
        </Field>

        <Field>
          <FieldLabel>Parent category</FieldLabel>
          <Select v-model="selectedParent">
            <SelectTrigger class="w-full"><SelectValue placeholder="No parent" /></SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem value="none">No parent (top level)</SelectItem>
                <SelectItem
                  v-for="parent in props.parentCategories.filter(
                    (item) => item.id !== props.category?.id,
                  )"
                  :key="parent.id"
                  :value="parent.id"
                >
                  {{ parent.name }}
                </SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
          <FieldDescription>Subcategories can be nested one level deep.</FieldDescription>
        </Field>
      </FieldGroup>

      <DialogFooter>
        <Button variant="outline" :disabled="props.saving" @click="emit('update:open', false)">
          Cancel
        </Button>
        <Button :disabled="props.saving" @click="submit">
          <LoaderCircleIcon v-if="props.saving" data-icon="inline-start" class="animate-spin" />
          {{ props.saving ? "Saving..." : isEditing ? "Save changes" : "Create" }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
