<script setup lang="ts">
  import { ShieldCheckIcon } from "@lucide/vue";
  import { onMounted, reactive, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import DepartmentsPanel from "@/components/admin/DepartmentsPanel.vue";
  import GroupsPanel from "@/components/admin/GroupsPanel.vue";
  import SharingPolicyPanel from "@/components/admin/SharingPolicyPanel.vue";
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
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
  } from "@/components/ui/select";
  import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
  import { Textarea } from "@/components/ui/textarea";
  import { administrationService } from "@/services/administration";
  import type { Role } from "@/types/api";

  definePageMeta({
    title: "Teams & access",
    description: "Departments, groups, roles and the rules for sharing files.",
    breadcrumbs: [{ label: "Administration", to: "/" }, { label: "Teams & access" }],
  });

  const admin = administrationService();
  const { roles, users, error, load, createRole, setPermissions, assignRole } = admin;
  const tab = shallowRef("departments");
  onMounted(() => void load());

  const roleOpen = shallowRef(false);
  const roleForm = reactive({ key: "", name: "", description: "", permissions: "forms.create" });
  const saveRole = async () => {
    const role = await createRole({
      ...roleForm,
      permissions: roleForm.permissions.split(/[\s,]+/).filter(Boolean),
    });
    if (role) {
      toast.success("Role created");
      roleOpen.value = false;
    } else toast.error(error.value || "Could not create role");
  };

  const permissionsRole = shallowRef<Role | null>(null);
  const permissionsText = shallowRef("");
  const openPermissions = (role: Role) => {
    permissionsRole.value = role;
    permissionsText.value = "";
  };
  const savePermissions = async () => {
    if (!permissionsRole.value) return;
    const ok = await setPermissions(
      permissionsRole.value.key,
      permissionsText.value.split(/[\s,]+/).filter(Boolean),
    );
    if (ok) {
      toast.success("Permissions updated");
      permissionsRole.value = null;
    } else toast.error(error.value || "Could not update permissions");
  };

  const assignRoleKey = shallowRef<Role | null>(null);
  const assignUserId = shallowRef("");
  const saveAssignment = async () => {
    if (!assignRoleKey.value || !assignUserId.value) return;
    if (await assignRole(assignRoleKey.value.key, assignUserId.value)) {
      toast.success("Role assigned");
      assignRoleKey.value = null;
      assignUserId.value = "";
    } else toast.error(error.value || "Could not assign role");
  };
</script>

<template>
  <Tabs v-model="tab" class="gap-5">
    <TabsList>
      <TabsTrigger value="departments">Departments</TabsTrigger>
      <TabsTrigger value="groups">Groups</TabsTrigger>
      <TabsTrigger value="roles">Roles</TabsTrigger>
      <TabsTrigger value="sharing">Sharing policy</TabsTrigger>
    </TabsList>

    <TabsContent value="departments"><DepartmentsPanel :admin="admin" /></TabsContent>
    <TabsContent value="groups"><GroupsPanel :admin="admin" /></TabsContent>

    <TabsContent value="roles">
      <div class="bg-card max-w-3xl rounded-2xl border shadow-sm">
        <div class="flex items-center justify-between border-b px-4 py-3">
          <div>
            <h2 class="text-sm font-semibold">Roles</h2>
            <p class="text-muted-foreground text-xs">
              Roles grant app-wide permissions, and files can be shared with everyone holding a
              role.
            </p>
          </div>
          <Button size="sm" variant="outline" @click="roleOpen = true">New role</Button>
        </div>
        <ul class="divide-y">
          <li v-for="role in roles" :key="role.id" class="flex items-center gap-3 px-4 py-3">
            <ShieldCheckIcon class="text-muted-foreground size-4 shrink-0" />
            <div class="min-w-0 flex-1">
              <p class="text-sm font-medium">{{ role.name }}</p>
              <p class="text-muted-foreground truncate text-xs">
                {{ role.description || role.key }}
              </p>
            </div>
            <Badge v-if="role.protected" variant="secondary">Built in</Badge>
            <template v-else>
              <Button size="sm" variant="ghost" @click="openPermissions(role)">Permissions</Button>
              <Button size="sm" variant="ghost" @click="assignRoleKey = role">Assign</Button>
            </template>
          </li>
        </ul>
      </div>
    </TabsContent>

    <TabsContent value="sharing"><SharingPolicyPanel /></TabsContent>
  </Tabs>

  <Dialog v-model:open="roleOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader><DialogTitle>New role</DialogTitle></DialogHeader>
      <form class="space-y-3" @submit.prevent="saveRole">
        <div class="space-y-1.5">
          <Label for="role-name">Name</Label>
          <Input id="role-name" v-model="roleForm.name" placeholder="Procurement Officer" />
        </div>
        <div class="space-y-1.5">
          <Label for="role-key">Key</Label>
          <Input id="role-key" v-model="roleForm.key" placeholder="procurement_officer" />
        </div>
        <div class="space-y-1.5">
          <Label for="role-description">Description</Label>
          <Textarea id="role-description" v-model="roleForm.description" />
        </div>
        <div class="space-y-1.5">
          <Label for="role-permissions">Permissions</Label>
          <Textarea
            id="role-permissions"
            v-model="roleForm.permissions"
            placeholder="forms.create, forms.submit"
          />
        </div>
      </form>
      <DialogFooter>
        <Button variant="outline" @click="roleOpen = false">Cancel</Button>
        <Button :disabled="!roleForm.name.trim() || !roleForm.key.trim()" @click="saveRole"
          >Create role</Button
        >
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <Dialog
    :open="Boolean(permissionsRole)"
    @update:open="(value) => !value && (permissionsRole = null)"
  >
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Permissions for {{ permissionsRole?.name }}</DialogTitle>
        <DialogDescription
          >Replace the role’s permission codes, separated by commas.</DialogDescription
        >
      </DialogHeader>
      <Textarea v-model="permissionsText" placeholder="forms.create, forms.submit" />
      <DialogFooter>
        <Button variant="outline" @click="permissionsRole = null">Cancel</Button>
        <Button @click="savePermissions">Save</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <Dialog :open="Boolean(assignRoleKey)" @update:open="(value) => !value && (assignRoleKey = null)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Assign {{ assignRoleKey?.name }}</DialogTitle>
        <DialogDescription>A member holds one custom role at a time.</DialogDescription>
      </DialogHeader>
      <Select v-model="assignUserId">
        <SelectTrigger class="w-full"><SelectValue placeholder="Choose a member" /></SelectTrigger>
        <SelectContent>
          <SelectItem v-for="user in users" :key="user.id" :value="user.id"
            >{{ user.name }} · {{ user.email }}</SelectItem
          >
        </SelectContent>
      </Select>
      <DialogFooter>
        <Button variant="outline" @click="assignRoleKey = null">Cancel</Button>
        <Button :disabled="!assignUserId" @click="saveAssignment">Assign</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
