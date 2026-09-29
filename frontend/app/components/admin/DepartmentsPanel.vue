<script setup lang="ts">
  import { Building2Icon, CrownIcon, PlusIcon, Trash2Icon, UserPlusIcon, XIcon } from "@lucide/vue";
  import { computed, shallowRef, watch } from "vue";
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
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
  } from "@/components/ui/select";
  import type { administrationService } from "@/services/administration";
  import type { Department } from "@/types/api";
  import { initialsOf } from "@/utils/format";

  const props = defineProps<{ admin: ReturnType<typeof administrationService> }>();
  const { departments, users } = props.admin;

  // Depth-first order so sub-departments sit under their parent.
  const tree = computed(() => {
    const byParent = new Map<string, Department[]>();
    for (const department of departments.value) {
      const key = department.parent_id ?? "";
      byParent.set(key, [...(byParent.get(key) ?? []), department]);
    }
    const rows: Array<{ department: Department; depth: number }> = [];
    const walk = (parent: string, depth: number) => {
      for (const department of (byParent.get(parent) ?? []).sort((a, b) =>
        a.name.localeCompare(b.name),
      )) {
        rows.push({ department, depth });
        walk(department.id, depth + 1);
      }
    };
    walk("", 0);
    // Departments whose parent no longer exists still need to show up.
    for (const department of departments.value) {
      if (!rows.some((row) => row.department.id === department.id))
        rows.push({ department, depth: 0 });
    }
    return rows;
  });

  const selectedId = shallowRef("");
  const selected = computed(() => departments.value.find((item) => item.id === selectedId.value));
  const members = shallowRef<Array<{ id: string; user_id: string; role: string }>>([]);
  const addUserId = shallowRef("");
  const userName = (id: string) => users.value.find((user) => user.id === id)?.name ?? id;
  const userEmail = (id: string) => users.value.find((user) => user.id === id)?.email ?? "";
  const available = computed(() =>
    users.value.filter((user) => !members.value.some((member) => member.user_id === user.id)),
  );

  const loadMembers = async () => {
    members.value = selectedId.value ? await props.admin.members(selectedId.value) : [];
  };
  watch(selectedId, loadMembers);
  watch(
    tree,
    (rows) => {
      if (!selectedId.value && rows[0]) selectedId.value = rows[0].department.id;
    },
    { immediate: true },
  );

  const addMember = async () => {
    if (!selectedId.value || !addUserId.value) return;
    if (await props.admin.addMember(selectedId.value, addUserId.value)) {
      addUserId.value = "";
      await loadMembers();
    } else toast.error(props.admin.error.value || "Could not add member");
  };

  const toggleHead = async (userId: string, role: string) => {
    const next = role === "head" ? "member" : "head";
    if (await props.admin.addMember(selectedId.value, userId, next)) {
      toast.success(
        next === "head" ? `${userName(userId)} now heads this department` : "Head removed",
      );
      await loadMembers();
    } else toast.error(props.admin.error.value || "Could not update member");
  };

  const removeMember = async (userId: string) => {
    if (await props.admin.removeMember(selectedId.value, userId)) await loadMembers();
    else toast.error(props.admin.error.value || "Could not remove member");
  };

  const dialogOpen = shallowRef(false);
  const editing = shallowRef<Department | null>(null);
  const form = shallowRef({ name: "", description: "", parent_id: "none" });
  const openCreate = (parentId = "") => {
    editing.value = null;
    form.value = { name: "", description: "", parent_id: parentId || "none" };
    dialogOpen.value = true;
  };
  const openEdit = (department: Department) => {
    editing.value = department;
    form.value = {
      name: department.name,
      description: department.description ?? "",
      parent_id: department.parent_id || "none",
    };
    dialogOpen.value = true;
  };
  const save = async () => {
    const input = {
      name: form.value.name.trim(),
      description: form.value.description.trim(),
      parent_id: form.value.parent_id === "none" ? "" : form.value.parent_id,
    };
    const saved = editing.value
      ? await props.admin.updateDepartment(editing.value.id, input)
      : await props.admin.createDepartment(input);
    if (!saved) {
      toast.error(props.admin.error.value || "Could not save department");
      return;
    }
    dialogOpen.value = false;
    selectedId.value = saved.id;
    toast.success(editing.value ? "Department updated" : "Department created");
  };
  const remove = async (department: Department) => {
    if (await props.admin.deleteDepartment(department.id)) {
      toast.success(`${department.name} deleted`);
      selectedId.value = "";
    } else toast.error(props.admin.error.value || "Could not delete department");
  };
</script>

<template>
  <div class="grid gap-5 lg:grid-cols-[minmax(0,320px)_1fr]">
    <div class="bg-card rounded-2xl border shadow-sm">
      <div class="flex items-center justify-between border-b px-4 py-3">
        <h2 class="text-sm font-semibold">Departments</h2>
        <Button size="sm" variant="ghost" @click="openCreate()"
          ><PlusIcon class="size-4" />New</Button
        >
      </div>
      <ul v-if="tree.length" class="p-2" role="listbox" aria-label="Departments">
        <li v-for="row in tree" :key="row.department.id">
          <button
            type="button"
            role="option"
            :aria-selected="row.department.id === selectedId"
            class="hover:bg-accent aria-selected:bg-accent flex w-full items-center gap-2 rounded-lg py-2 pr-2 text-left text-sm"
            :style="{ paddingLeft: `${0.5 + row.depth * 1.25}rem` }"
            @click="selectedId = row.department.id"
          >
            <Building2Icon class="text-muted-foreground size-4 shrink-0" />
            <span class="truncate">{{ row.department.name }}</span>
          </button>
        </li>
      </ul>
      <p v-else class="text-muted-foreground p-6 text-sm">
        No departments yet. Departments are how files reach the right people.
      </p>
    </div>

    <div v-if="selected" class="bg-card rounded-2xl border p-5 shadow-sm">
      <div class="flex flex-wrap items-start gap-3">
        <div class="min-w-0 flex-1">
          <h2 class="text-lg font-semibold">{{ selected.name }}</h2>
          <p class="text-muted-foreground text-sm">
            {{ selected.description || "No description" }}
            <template v-if="selected.parent_id">
              · part of {{ departments.find((item) => item.id === selected?.parent_id)?.name }}
            </template>
          </p>
        </div>
        <Button size="sm" variant="outline" @click="openCreate(selected.id)"
          ><PlusIcon class="size-4" />Sub-department</Button
        >
        <Button size="sm" variant="outline" @click="openEdit(selected)">Edit</Button>
        <Button size="sm" variant="ghost" class="text-destructive" @click="remove(selected)"
          ><Trash2Icon class="size-4" /><span class="sr-only">Delete department</span></Button
        >
      </div>

      <p class="text-muted-foreground bg-muted mt-4 rounded-lg p-3 text-xs leading-relaxed">
        Members see files shared with this department. Heads also see files shared with every
        department below this one.
      </p>

      <form class="mt-5 flex gap-2" @submit.prevent="addMember">
        <Select v-model="addUserId">
          <SelectTrigger class="flex-1" aria-label="Person to add"
            ><SelectValue placeholder="Add a member"
          /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="user in available" :key="user.id" :value="user.id"
              >{{ user.name }} · {{ user.email }}</SelectItem
            >
          </SelectContent>
        </Select>
        <Button type="submit" :disabled="!addUserId"><UserPlusIcon class="size-4" />Add</Button>
      </form>

      <ul v-if="members.length" class="mt-4 divide-y">
        <li v-for="member in members" :key="member.id" class="flex items-center gap-3 py-3">
          <Avatar class="size-8"
            ><AvatarFallback class="text-xs">{{
              initialsOf(userName(member.user_id))
            }}</AvatarFallback></Avatar
          >
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium">{{ userName(member.user_id) }}</p>
            <p class="text-muted-foreground truncate text-xs">{{ userEmail(member.user_id) }}</p>
          </div>
          <Badge v-if="member.role === 'head'" variant="secondary" class="gap-1"
            ><CrownIcon class="size-3" />Head</Badge
          >
          <Button size="sm" variant="ghost" @click="toggleHead(member.user_id, member.role)">{{
            member.role === "head" ? "Remove head" : "Make head"
          }}</Button>
          <Button
            size="icon"
            variant="ghost"
            class="size-8"
            :aria-label="`Remove ${userName(member.user_id)}`"
            @click="removeMember(member.user_id)"
            ><XIcon class="size-4"
          /></Button>
        </li>
      </ul>
      <p v-else class="text-muted-foreground mt-4 text-sm">No members yet.</p>
    </div>
  </div>

  <Dialog v-model:open="dialogOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ editing ? "Edit department" : "New department" }}</DialogTitle>
        <DialogDescription>Departments can sit inside other departments.</DialogDescription>
      </DialogHeader>
      <form class="space-y-4" @submit.prevent="save">
        <div class="space-y-1.5">
          <Label for="department-name">Name</Label>
          <Input id="department-name" v-model="form.name" />
        </div>
        <div class="space-y-1.5">
          <Label for="department-description">Description</Label>
          <Input id="department-description" v-model="form.description" />
        </div>
        <div class="space-y-1.5">
          <Label>Part of</Label>
          <Select v-model="form.parent_id">
            <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="none">Top level</SelectItem>
              <SelectItem
                v-for="department in departments.filter((item) => item.id !== editing?.id)"
                :key="department.id"
                :value="department.id"
                >{{ department.name }}</SelectItem
              >
            </SelectContent>
          </Select>
        </div>
      </form>
      <DialogFooter>
        <Button variant="outline" @click="dialogOpen = false">Cancel</Button>
        <Button :disabled="!form.name.trim()" @click="save">{{
          editing ? "Save" : "Create"
        }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
