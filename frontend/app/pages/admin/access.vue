<script setup lang="ts">
  import { Building2Icon, ShieldCheckIcon, UsersIcon } from "@lucide/vue";
  import { computed, onMounted, reactive, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Card, CardContent } from "@/components/ui/card";
  import {
    Dialog,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
  } from "@/components/ui/dialog";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import { Textarea } from "@/components/ui/textarea";
  import { administrationService } from "@/services/administration";

  definePageMeta({
    title: "Access control",
    description: "Departments, users, and Casbin roles.",
    breadcrumbs: [{ label: "Administration", to: "/" }, { label: "Access control" }],
  });
  const {
    departments,
    roles,
    users,
    loading,
    error,
    load,
    createDepartment,
    createRole,
    setPermissions,
    assignRole,
    invite,
  } = administrationService();
  const open = shallowRef(false);
  const roleOpen = shallowRef(false);
  const inviteOpen = shallowRef(false);
  const form = reactive({ name: "", description: "" });
  const roleForm = reactive({
    key: "",
    name: "",
    description: "",
    permissions: "forms.create, forms.submit",
  });
  const inviteForm = reactive({ name: "", email: "" });
  const activeUsers = computed(() => users.value.filter((user) => user.status === "active"));
  onMounted(() => {
    void load();
  });
  const save = async () => {
    const created = await createDepartment(form);
    if (created) {
      toast.success("Department created");
      open.value = false;
      form.name = "";
      form.description = "";
    } else toast.error(error.value || "Could not create department");
  };
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
  const sendInvite = async () => {
    const link = await invite(inviteForm.name, inviteForm.email);
    if (link) {
      await navigator.clipboard?.writeText(link);
      toast.success("Invitation created — setup link copied");
      inviteOpen.value = false;
    } else toast.error(error.value || "Could not create invitation");
  };
  const updateRolePermissions = async (key: string) => {
    const raw = window.prompt("Permission codes, comma-separated", "forms.create, forms.submit");
    if (raw && (await setPermissions(key, raw.split(/[\s,]+/).filter(Boolean))))
      toast.success("Permissions updated");
  };
  const assign = async (key: string) => {
    const id = window.prompt("User ID to assign");
    if (id && (await assignRole(key, id))) toast.success("Role assigned");
  };
</script>
<template>
  <Teleport to="#page-actions-target"
    ><div class="flex gap-2">
      <Button size="sm" variant="outline" @click="inviteOpen = true">Invite user</Button
      ><Button size="sm" variant="outline" @click="roleOpen = true">New role</Button
      ><Button size="sm" @click="open = true">New department</Button>
    </div></Teleport
  >
  <div class="space-y-6">
    <div>
      <h1 class="text-xl font-semibold">Access control</h1>
      <p class="text-muted-foreground mt-1 text-sm">
        Casbin policies power roles, departments, and protected operations.
      </p>
    </div>
    <div class="grid gap-3 md:grid-cols-3">
      <Card
        ><CardContent class="p-5"
          ><Building2Icon class="text-primary h-5 w-5" />
          <p class="mt-3 text-2xl font-semibold">{{ departments.length }}</p>
          <p class="text-muted-foreground text-sm">Departments</p></CardContent
        ></Card
      ><Card
        ><CardContent class="p-5"
          ><UsersIcon class="text-primary h-5 w-5" />
          <p class="mt-3 text-2xl font-semibold">{{ activeUsers.length }}</p>
          <p class="text-muted-foreground text-sm">Active users</p></CardContent
        ></Card
      ><Card
        ><CardContent class="p-5"
          ><ShieldCheckIcon class="text-primary h-5 w-5" />
          <p class="mt-3 text-2xl font-semibold">{{ roles.length }}</p>
          <p class="text-muted-foreground text-sm">Roles</p></CardContent
        ></Card
      >
    </div>
    <div class="grid gap-5 lg:grid-cols-2">
      <Card
        ><CardContent class="p-5"
          ><h2 class="font-semibold">Departments</h2>
          <div class="mt-4 divide-y">
            <div v-for="department in departments" :key="department.id" class="py-3">
              <p class="text-sm font-medium">{{ department.name }}</p>
              <p class="text-muted-foreground text-xs">
                {{ department.description || "No description" }}
              </p>
            </div>
            <p v-if="!loading && !departments.length" class="text-muted-foreground py-6 text-sm">
              No departments yet.
            </p>
          </div></CardContent
        ></Card
      ><Card
        ><CardContent class="p-5"
          ><h2 class="font-semibold">Roles</h2>
          <div class="mt-4 space-y-3">
            <div
              v-for="role in roles"
              :key="role.id"
              class="border-border flex items-center justify-between rounded-lg border p-3"
            >
              <div>
                <p class="text-sm font-medium">{{ role.name }}</p>
                <p class="text-muted-foreground text-xs">{{ role.description || role.key }}</p>
              </div>
              <div class="flex items-center gap-1">
                <Badge v-if="role.protected" variant="secondary">Protected</Badge
                ><Button size="sm" variant="ghost" @click="updateRolePermissions(role.key)"
                  >Permissions</Button
                ><Button size="sm" variant="ghost" @click="assign(role.key)">Assign</Button>
              </div>
            </div>
          </div></CardContent
        ></Card
      >
    </div>
  </div>
  <Dialog v-model:open="open"
    ><DialogContent
      ><DialogHeader><DialogTitle>New department</DialogTitle></DialogHeader>
      <div class="grid gap-4 py-2">
        <div class="grid gap-2">
          <Label for="department-name">Name</Label
          ><Input id="department-name" v-model="form.name" />
        </div>
        <div class="grid gap-2">
          <Label for="department-description">Description</Label
          ><Textarea id="department-description" v-model="form.description" />
        </div>
      </div>
      <DialogFooter
        ><Button variant="outline" @click="open = false">Cancel</Button
        ><Button :disabled="!form.name.trim()" @click="save"
          >Create department</Button
        ></DialogFooter
      ></DialogContent
    ></Dialog
  >
  <Dialog v-model:open="roleOpen"
    ><DialogContent
      ><DialogHeader><DialogTitle>New role</DialogTitle></DialogHeader>
      <div class="grid gap-3 py-2">
        <Input v-model="roleForm.name" placeholder="Role name" /><Input
          v-model="roleForm.key"
          placeholder="role_key"
        /><Textarea v-model="roleForm.description" placeholder="Description" /><Textarea
          v-model="roleForm.permissions"
          placeholder="forms.create, forms.submit"
        />
      </div>
      <DialogFooter><Button @click="saveRole">Create role</Button></DialogFooter></DialogContent
    ></Dialog
  >
  <Dialog v-model:open="inviteOpen"
    ><DialogContent
      ><DialogHeader><DialogTitle>Invite user</DialogTitle></DialogHeader>
      <div class="grid gap-3 py-2">
        <Input v-model="inviteForm.name" placeholder="Full name" /><Input
          v-model="inviteForm.email"
          type="email"
          placeholder="email@company.com"
        />
      </div>
      <DialogFooter
        ><Button @click="sendInvite">Send & copy link</Button></DialogFooter
      ></DialogContent
    ></Dialog
  >
</template>
