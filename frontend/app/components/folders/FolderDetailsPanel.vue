<script setup lang="ts">
  import { FolderPlusIcon, SaveIcon, Trash2Icon } from "@lucide/vue";

  import FolderPermissionEditor from "@/components/folders/FolderPermissionEditor.vue";
  import { Button } from "@/components/ui/button";
  import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
  import { Input } from "@/components/ui/input";
  import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
  } from "@/components/ui/select";
  import { Switch } from "@/components/ui/switch";
  import { Textarea } from "@/components/ui/textarea";
  import type { FolderInput, ManagedFolder } from "@/types/api";

  const props = defineProps<{
    folder?: ManagedFolder;
    folders: ManagedFolder[];
    grants?: import("@/types/api").FolderGrant[];
    users?: import("@/types/api").User[];
    departments?: import("@/types/api").Department[];
    saving?: boolean;
  }>();
  const emit = defineEmits<{
    save: [input: FolderInput];
    remove: [];
    create: [];
    saveGrant: [input: import("@/types/api").FolderGrantInput];
    removeGrant: [grantId: string];
  }>();

  const draft = reactive<FolderInput>({
    name: "",
    description: "",
    color: "#6366f1",
    inherit_permissions: true,
  });
  const ROOT_FOLDER = "__root__";
  const parentChoice = shallowRef(ROOT_FOLDER);
  watch(
    () => props.folder,
    (folder) => {
      draft.name = folder?.name ?? "";
      draft.parent_id = folder?.parent_id ?? "";
      parentChoice.value = folder?.parent_id || ROOT_FOLDER;
      draft.description = folder?.description ?? "";
      draft.color = folder?.color || "#6366f1";
      draft.inherit_permissions = folder?.inherit_permissions ?? true;
    },
    { immediate: true },
  );

  const isNew = computed(() => !props.folder);
  const availableParents = computed(() =>
    props.folders.filter((candidate) => candidate.id !== props.folder?.id),
  );
  const submit = () =>
    emit("save", {
      ...draft,
      parent_id: parentChoice.value === ROOT_FOLDER ? undefined : parentChoice.value,
    });
</script>

<template>
  <section class="mx-auto w-full max-w-2xl px-5 py-6 sm:px-8">
    <div class="mb-7 flex items-start justify-between gap-4">
      <div>
        <p class="text-muted-foreground text-sm">
          {{ isNew ? "Create a logical location for documents" : "Folder settings" }}
        </p>
        <h2 class="mt-1 text-xl font-semibold tracking-tight">
          {{ isNew ? "New folder" : folder?.name }}
        </h2>
      </div>
      <Button
        v-if="!isNew"
        variant="outline"
        size="sm"
        :disabled="saving || !!folder?.document_count"
        @click="emit('remove')"
      >
        <Trash2Icon data-icon="inline-start" /> Delete
      </Button>
    </div>

    <form class="flex flex-col gap-6" @submit.prevent="submit">
      <Field>
        <FieldLabel for="folder-name">Folder name</FieldLabel>
        <Input
          id="folder-name"
          v-model="draft.name"
          required
          maxlength="120"
          placeholder="e.g. Finance"
        />
        <FieldDescription
          >Use a concise name. Folder moves never copy the underlying files.</FieldDescription
        >
      </Field>

      <div class="grid gap-6 sm:grid-cols-[1fr_116px]">
        <Field>
          <FieldLabel>Parent folder</FieldLabel>
          <Select v-model="parentChoice">
            <SelectTrigger class="w-full"><SelectValue placeholder="Library root" /></SelectTrigger>
            <SelectContent>
              <SelectItem :value="ROOT_FOLDER">Library root</SelectItem>
              <SelectItem
                v-for="candidate in availableParents"
                :key="candidate.id"
                :value="candidate.id"
              >
                {{ "— ".repeat(candidate.depth) }}{{ candidate.name }}
              </SelectItem>
            </SelectContent>
          </Select>
          <FieldDescription>{{
            isNew
              ? "Choose where this folder belongs."
              : "Moving this folder updates the whole subtree without moving stored files."
          }}</FieldDescription>
        </Field>
        <Field>
          <FieldLabel for="folder-color">Color</FieldLabel>
          <Input id="folder-color" v-model="draft.color" type="color" class="h-10 p-1" />
        </Field>
      </div>

      <Field>
        <FieldLabel for="folder-description">Description</FieldLabel>
        <Textarea
          id="folder-description"
          v-model="draft.description"
          placeholder="What belongs in this folder?"
        />
      </Field>

      <Field class="border-border flex-row items-center justify-between border-y py-4">
        <div class="pr-5">
          <FieldLabel for="folder-inheritance">Inherit access</FieldLabel>
          <FieldDescription
            >New subfolders inherit their parent’s permissions. Access grants come in the next Phase
            1 slice.</FieldDescription
          >
        </div>
        <Switch id="folder-inheritance" v-model="draft.inherit_permissions" />
      </Field>

      <div class="flex flex-wrap items-center gap-3">
        <Button type="submit" :disabled="saving">
          <SaveIcon data-icon="inline-start" />
          {{ saving ? "Saving…" : isNew ? "Create folder" : "Save changes" }}
        </Button>
        <Button v-if="!isNew" type="button" variant="ghost" @click="emit('create')">
          <FolderPlusIcon data-icon="inline-start" /> New folder
        </Button>
        <span v-if="folder?.document_count" class="text-muted-foreground text-sm"
          >{{ folder.document_count }} document{{ folder.document_count === 1 ? "" : "s" }} in this
          folder</span
        >
      </div>
    </form>
    <FolderPermissionEditor
      v-if="folder"
      :grants="grants ?? []"
      :users="users ?? []"
      :departments="departments ?? []"
      :saving="saving"
      @save="emit('saveGrant', $event)"
      @remove="emit('removeGrant', $event)"
    />
  </section>
</template>
