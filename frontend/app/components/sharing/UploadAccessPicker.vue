<script setup lang="ts">
  import {
    Building2Icon,
    FolderIcon,
    GlobeIcon,
    LockIcon,
    ShieldIcon,
    UserIcon,
    UsersIcon,
    XIcon,
  } from "@lucide/vue";
  import { computed, shallowRef, watch } from "vue";

  import PrincipalPicker from "@/components/sharing/PrincipalPicker.vue";
  import { Button } from "@/components/ui/button";
  import { Label } from "@/components/ui/label";
  import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
  import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
  } from "@/components/ui/select";
  import type {
    ApiResponse,
    FolderGrant,
    FolderRole,
    ManagedFolder,
    PrincipalOption,
    PrincipalType,
    Sensitivity,
    UploadAccess,
  } from "@/types/api";

  const access = defineModel<UploadAccess>({ required: true });
  const props = defineProps<{ folder?: ManagedFolder }>();

  const folderGrants = shallowRef<FolderGrant[]>([]);
  // Names for chosen principals; the upload only sends type, id and role.
  const chosen = shallowRef<Array<PrincipalOption & { role: FolderRole }>>([]);

  const roles: Array<{ value: FolderRole; label: string }> = [
    { value: "viewer", label: "Viewer" },
    { value: "commenter", label: "Commenter" },
    { value: "editor", label: "Editor" },
    { value: "manager", label: "Manager" },
  ];
  const sensitivities: Array<{ value: Sensitivity; label: string }> = [
    { value: "public", label: "Public" },
    { value: "internal", label: "Internal" },
    { value: "confidential", label: "Confidential" },
    { value: "restricted", label: "Restricted" },
  ];
  const iconFor: Record<PrincipalType, typeof UserIcon> = {
    user: UserIcon,
    department: Building2Icon,
    group: UsersIcon,
    role: ShieldIcon,
    everyone: GlobeIcon,
  };

  // Sharing from inside a folder needs the right to manage that folder.
  const canWiden = computed(() => !props.folder || props.folder.can_manage !== false);
  const restricted = computed(() => access.value.sensitivity === "restricted");

  const folderSummary = computed(() => {
    if (!props.folder) return "Only you, until you share it";
    if (!folderGrants.value.length) return `Same people as “${props.folder.name}”`;
    const names = folderGrants.value.slice(0, 3).map((grant) => grant.name);
    const extra = folderGrants.value.length - names.length;
    return `${names.join(", ")}${extra > 0 ? ` and ${extra} more` : ""}`;
  });

  watch(
    () => props.folder?.id,
    async (id) => {
      folderGrants.value = [];
      if (!id) return;
      const { $api } = useNuxtApp();
      try {
        const response = await $api<ApiResponse<FolderGrant[]>>(`/folders/${id}/grants`);
        folderGrants.value = response.data;
      } catch {
        folderGrants.value = [];
      }
    },
    { immediate: true },
  );

  // Restricted files never follow the folder.
  watch(restricted, (value) => {
    if (value && access.value.mode === "inherit")
      access.value = { ...access.value, mode: "private" };
    if (value)
      chosen.value = chosen.value.filter((item) => item.type === "user" || item.type === "group");
  });

  watch(
    chosen,
    (items) => {
      access.value = {
        ...access.value,
        grants: items.map((item) => ({
          principal_type: item.type,
          principal_id: item.id,
          role: item.role,
        })),
      };
    },
    { deep: true },
  );

  const setMode = (mode: unknown) => {
    access.value = { ...access.value, mode: mode as UploadAccess["mode"] };
  };

  const add = (option: PrincipalOption) => {
    chosen.value = [...chosen.value, { ...option, role: "viewer" }];
  };

  const setRole = (index: number, role: FolderRole) => {
    chosen.value = chosen.value.map((item, i) => (i === index ? { ...item, role } : item));
  };

  const remove = (index: number) => {
    chosen.value = chosen.value.filter((_, i) => i !== index);
  };
</script>

<template>
  <fieldset class="space-y-3 rounded-xl border p-4">
    <legend class="px-1 text-sm font-semibold">Who can see this?</legend>

    <RadioGroup :model-value="access.mode" class="gap-2" @update:model-value="setMode">
      <label
        class="has-[button[data-state=checked]]:border-primary/50 has-[button[data-state=checked]]:bg-primary/5 flex cursor-pointer items-start gap-3 rounded-lg border p-3"
        :class="restricted ? 'pointer-events-none opacity-50' : ''"
      >
        <RadioGroupItem value="inherit" class="mt-0.5" :disabled="restricted" />
        <span class="min-w-0 flex-1">
          <span class="flex items-center gap-1.5 text-sm font-medium">
            <FolderIcon class="size-3.5" />{{ folder ? "Same as folder" : "Keep in My files" }}
          </span>
          <span class="text-muted-foreground block truncate text-xs">{{ folderSummary }}</span>
        </span>
      </label>

      <label
        v-if="folder"
        class="has-[button[data-state=checked]]:border-primary/50 has-[button[data-state=checked]]:bg-primary/5 flex cursor-pointer items-start gap-3 rounded-lg border p-3"
      >
        <RadioGroupItem value="private" class="mt-0.5" />
        <span class="min-w-0 flex-1">
          <span class="flex items-center gap-1.5 text-sm font-medium"
            ><LockIcon class="size-3.5" />Only me</span
          >
          <span class="text-muted-foreground block text-xs"
            >Stored in the folder, but hidden from the folder’s members</span
          >
        </span>
      </label>

      <label
        class="has-[button[data-state=checked]]:border-primary/50 has-[button[data-state=checked]]:bg-primary/5 flex cursor-pointer items-start gap-3 rounded-lg border p-3"
        :class="canWiden ? '' : 'pointer-events-none opacity-50'"
      >
        <RadioGroupItem value="custom" class="mt-0.5" :disabled="!canWiden" />
        <span class="min-w-0 flex-1">
          <span class="flex items-center gap-1.5 text-sm font-medium"
            ><UsersIcon class="size-3.5" />Choose people and teams</span
          >
          <span class="text-muted-foreground block text-xs">{{
            canWiden
              ? "Pick departments, groups, roles or people"
              : "You can add files to this folder, but not share them with others"
          }}</span>
        </span>
      </label>
    </RadioGroup>

    <div v-if="access.mode === 'custom'" class="space-y-2">
      <PrincipalPicker
        :selected-keys="chosen.map((item) => `${item.type}|${item.id}`)"
        :exclude="restricted ? ['department', 'role', 'everyone'] : []"
        @select="add"
      />
      <ul v-if="chosen.length" class="divide-y rounded-lg border">
        <li
          v-for="(item, index) in chosen"
          :key="`${item.type}|${item.id}`"
          class="flex items-center gap-3 px-3 py-2"
        >
          <component :is="iconFor[item.type]" class="text-muted-foreground size-4 shrink-0" />
          <span class="min-w-0 flex-1 truncate text-sm">{{ item.name }}</span>
          <Select
            :model-value="item.role"
            @update:model-value="(value) => setRole(index, value as FolderRole)"
          >
            <SelectTrigger class="h-8 w-32" :aria-label="`Role for ${item.name}`"
              ><SelectValue
            /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="role in roles" :key="role.value" :value="role.value">{{
                role.label
              }}</SelectItem>
            </SelectContent>
          </Select>
          <Button
            variant="ghost"
            size="icon"
            class="size-8"
            :aria-label="`Remove ${item.name}`"
            @click="remove(index)"
          >
            <XIcon class="size-4" />
          </Button>
        </li>
      </ul>
      <p v-else class="text-muted-foreground text-xs">
        Until you add someone, only you can see these files.
      </p>
    </div>

    <div class="flex items-center gap-3">
      <Label for="upload-sensitivity" class="text-xs">Sensitivity</Label>
      <Select
        :model-value="access.sensitivity ?? 'internal'"
        @update:model-value="(value) => (access = { ...access, sensitivity: value as Sensitivity })"
      >
        <SelectTrigger id="upload-sensitivity" class="h-8 w-40"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem v-for="item in sensitivities" :key="item.value" :value="item.value">{{
            item.label
          }}</SelectItem>
        </SelectContent>
      </Select>
      <span v-if="restricted" class="text-muted-foreground text-xs"
        >Named people and groups only</span
      >
    </div>
  </fieldset>
</template>
