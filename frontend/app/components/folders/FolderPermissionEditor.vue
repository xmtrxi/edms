<script setup lang="ts">
  import { ShieldCheckIcon, Trash2Icon, UserPlusIcon } from "@lucide/vue";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
  import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
  } from "@/components/ui/select";
  import type { Department, FolderGrant, FolderGrantInput, FolderRole, User } from "@/types/api";

  const props = withDefaults(
    defineProps<{
      grants: FolderGrant[];
      users: User[];
      departments: Department[];
      saving?: boolean;
    }>(),
    { saving: false },
  );
  const emit = defineEmits<{ save: [input: FolderGrantInput]; remove: [grantId: string] }>();

  const selectedPrincipal = shallowRef("");
  const selectedRole = shallowRef<FolderRole>("viewer");
  const roles: Array<{ value: FolderRole; label: string; description: string }> = [
    { value: "viewer", label: "Viewer", description: "View and download" },
    { value: "contributor", label: "Contributor", description: "Upload and comment" },
    { value: "editor", label: "Editor", description: "Edit and create versions" },
    { value: "manager", label: "Manager", description: "Manage files and folder access" },
    { value: "uploader", label: "Uploader", description: "Upload without browsing" },
  ];
  const userById = computed(() => new Map(props.users.map((user) => [user.id, user])));
  const departmentById = computed(
    () => new Map(props.departments.map((department) => [department.id, department])),
  );
  const roleByValue = computed(() => new Map(roles.map((role) => [role.value, role])));
  const save = () => {
    const [principalType, principalID] = selectedPrincipal.value.split(":", 2) as [
      "user" | "department",
      string,
    ];
    if (!principalID) return;
    emit("save", {
      principal_type: principalType,
      principal_id: principalID,
      role: selectedRole.value,
    });
    selectedPrincipal.value = "";
    selectedRole.value = "viewer";
  };
</script>

<template>
  <section class="border-border mt-8 border-t pt-7">
    <div class="mb-5 flex items-start gap-3">
      <span class="bg-primary/10 text-primary grid size-9 place-items-center rounded-lg"
        ><ShieldCheckIcon class="size-4"
      /></span>
      <div>
        <h3 class="font-semibold">Access</h3>
        <p class="text-muted-foreground mt-0.5 text-sm">
          Direct access is evaluated before inherited parent access.
        </p>
      </div>
    </div>

    <div
      class="bg-muted/35 border-border grid gap-4 border p-4 sm:grid-cols-[minmax(0,1fr)_180px_auto] sm:items-end"
    >
      <Field>
        <FieldLabel>Person</FieldLabel>
        <Select v-model="selectedPrincipal">
          <SelectTrigger class="w-full"
            ><SelectValue placeholder="Choose a person"
          /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="user in users" :key="user.id" :value="`user:${user.id}`"
              >{{ user.name }} · {{ user.email }}</SelectItem
            >
            <SelectItem
              v-for="department in departments"
              :key="department.id"
              :value="`department:${department.id}`"
              >{{ department.name }} · Department</SelectItem
            >
          </SelectContent>
        </Select>
      </Field>
      <Field>
        <FieldLabel>Role</FieldLabel>
        <Select v-model="selectedRole">
          <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="role in roles" :key="role.value" :value="role.value">{{
              role.label
            }}</SelectItem>
          </SelectContent>
        </Select>
      </Field>
      <Button type="button" :disabled="saving || !selectedPrincipal" @click="save"
        ><UserPlusIcon data-icon="inline-start" /> Add</Button
      >
      <FieldDescription class="sm:col-span-3"
        >{{ roleByValue.get(selectedRole)?.description }}. Adding the same person updates their
        existing direct role.</FieldDescription
      >
    </div>

    <div v-if="grants.length" class="divide-border border-border mt-4 divide-y border-y">
      <div v-for="grant in grants" :key="grant.id" class="flex items-center gap-3 py-3">
        <div
          class="bg-secondary text-secondary-foreground grid size-9 shrink-0 place-items-center rounded-full text-xs font-semibold"
        >
          {{
            (grant.principal_type === "department"
              ? departmentById.get(grant.principal_id)?.name
              : userById.get(grant.principal_id)?.name
            )
              ?.slice(0, 2)
              .toUpperCase() ?? "??"
          }}
        </div>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium">
            {{
              grant.principal_type === "department"
                ? (departmentById.get(grant.principal_id)?.name ?? grant.principal_id)
                : (userById.get(grant.principal_id)?.name ?? grant.principal_id)
            }}
          </p>
          <p class="text-muted-foreground truncate text-xs">
            {{
              grant.principal_type === "department"
                ? "Department"
                : (userById.get(grant.principal_id)?.email ?? "User")
            }}
          </p>
        </div>
        <Badge variant="secondary" class="capitalize">{{ grant.role }}</Badge>
        <Button
          variant="ghost"
          size="icon"
          :disabled="saving"
          :aria-label="`Remove ${userById.get(grant.principal_id)?.name ?? 'access'}`"
          @click="emit('remove', grant.id)"
          ><Trash2Icon class="size-4"
        /></Button>
      </div>
    </div>
    <p v-else class="text-muted-foreground mt-4 text-sm">
      No direct access grants. This folder is private unless it inherits access from its parent.
    </p>
  </section>
</template>
