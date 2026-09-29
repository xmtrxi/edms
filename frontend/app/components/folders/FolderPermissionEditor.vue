<script setup lang="ts">
  import {
    Building2Icon,
    GlobeIcon,
    ShieldCheckIcon,
    ShieldIcon,
    Trash2Icon,
    UserIcon,
    UsersIcon,
  } from "@lucide/vue";

  import PrincipalPicker from "@/components/sharing/PrincipalPicker.vue";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
  } from "@/components/ui/select";
  import type {
    Department,
    FolderGrant,
    FolderGrantInput,
    FolderRole,
    PrincipalOption,
    PrincipalType,
    User,
  } from "@/types/api";

  const props = withDefaults(
    defineProps<{
      grants: FolderGrant[];
      saving?: boolean;
      /** False hides editing controls for people who can only look. */
      canManage?: boolean;
      // Kept for callers that still pass them; names now come with each grant.
      users?: User[];
      departments?: Department[];
    }>(),
    { saving: false, canManage: true },
  );
  const emit = defineEmits<{ save: [input: FolderGrantInput]; remove: [grantId: string] }>();

  const roles: Array<{ value: FolderRole; label: string; description: string }> = [
    { value: "viewer", label: "Viewer", description: "View and download" },
    { value: "commenter", label: "Commenter", description: "View and comment" },
    { value: "contributor", label: "Contributor", description: "Add files and comment" },
    { value: "editor", label: "Editor", description: "Edit files and create versions" },
    { value: "manager", label: "Manager", description: "Everything, including sharing" },
    { value: "uploader", label: "Uploader", description: "Drop files in without browsing" },
  ];
  const iconFor: Record<PrincipalType, typeof UserIcon> = {
    user: UserIcon,
    department: Building2Icon,
    group: UsersIcon,
    role: ShieldIcon,
    everyone: GlobeIcon,
  };

  const direct = computed(() => props.grants.filter((grant) => grant.source !== "inherited"));
  const inherited = computed(() => props.grants.filter((grant) => grant.source === "inherited"));
  const selectedKeys = computed(() =>
    direct.value.map((grant) => `${grant.principal_type}|${grant.principal_id}`),
  );

  const add = (option: PrincipalOption) =>
    emit("save", { principal_type: option.type, principal_id: option.id, role: "viewer" });

  const changeRole = (grant: FolderGrant, role: FolderRole) =>
    emit("save", { principal_type: grant.principal_type, principal_id: grant.principal_id, role });
</script>

<template>
  <section class="border-border mt-8 border-t pt-7">
    <div class="mb-5 flex items-start gap-3">
      <span class="bg-primary/10 text-primary grid size-9 place-items-center rounded-lg"
        ><ShieldCheckIcon class="size-4"
      /></span>
      <div>
        <h3 class="font-semibold">Who has access</h3>
        <p class="text-muted-foreground mt-0.5 text-sm">
          Access here applies to every file and subfolder that follows this folder.
        </p>
      </div>
    </div>

    <PrincipalPicker v-if="canManage" :selected-keys="selectedKeys" @select="add" />

    <ul v-if="direct.length" class="divide-border border-border mt-4 divide-y border-y">
      <li v-for="grant in direct" :key="grant.id" class="flex items-center gap-3 py-3">
        <span class="bg-secondary grid size-9 shrink-0 place-items-center rounded-full">
          <component :is="iconFor[grant.principal_type]" class="size-4" />
        </span>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium">{{ grant.name }}</p>
          <p class="text-muted-foreground truncate text-xs">
            {{ grant.description ?? (grant.principal_type === "everyone" ? "All members" : "") }}
          </p>
        </div>
        <Select
          v-if="canManage"
          :model-value="grant.role"
          :disabled="saving"
          @update:model-value="(value) => changeRole(grant, value as FolderRole)"
        >
          <SelectTrigger class="h-8 w-36" :aria-label="`Role for ${grant.name}`"
            ><SelectValue
          /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="role in roles" :key="role.value" :value="role.value">
              {{ role.label }}
            </SelectItem>
          </SelectContent>
        </Select>
        <Badge v-else variant="secondary" class="capitalize">{{ grant.role }}</Badge>
        <Button
          v-if="canManage"
          variant="ghost"
          size="icon"
          :disabled="saving"
          :aria-label="`Remove ${grant.name}`"
          @click="emit('remove', grant.id)"
          ><Trash2Icon class="size-4"
        /></Button>
      </li>
    </ul>
    <p v-else class="text-muted-foreground mt-4 text-sm">
      No one has been added to this folder directly.
      {{
        inherited.length ? "It uses the access below." : "Only its creator and admins can see it."
      }}
    </p>

    <div v-if="inherited.length" class="mt-5">
      <p class="text-muted-foreground mb-2 text-xs font-semibold tracking-wider uppercase">
        Inherited
      </p>
      <ul class="divide-border divide-y">
        <li v-for="grant in inherited" :key="grant.id" class="flex items-center gap-3 py-2">
          <component :is="iconFor[grant.principal_type]" class="text-muted-foreground size-4" />
          <span class="min-w-0 flex-1 truncate text-sm">{{ grant.name }}</span>
          <span class="text-muted-foreground truncate text-xs">from {{ grant.source_name }}</span>
          <Badge variant="outline" class="capitalize">{{ grant.role }}</Badge>
        </li>
      </ul>
    </div>
  </section>
</template>
