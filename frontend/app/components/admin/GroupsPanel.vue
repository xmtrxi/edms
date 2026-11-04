<script setup lang="ts">
  import { PlusIcon, Trash2Icon, UserPlusIcon, UsersIcon, XIcon } from "@lucide/vue";
  import { computed, onMounted, shallowRef, watch } from "vue";
  import { toast } from "vue-sonner";

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
  import type { GroupMember } from "@/types/api";
  import { initialsOf } from "@/utils/format";

  const props = defineProps<{ admin: ReturnType<typeof administrationService> }>();
  const { groups, users } = props.admin;

  const selectedId = shallowRef("");
  const selected = computed(() => groups.value.find((group) => group.id === selectedId.value));
  const members = shallowRef<GroupMember[]>([]);
  const addUserId = shallowRef("");
  const available = computed(() =>
    users.value.filter((user) => !members.value.some((member) => member.user_id === user.id)),
  );

  const loadMembers = async () => {
    members.value = selectedId.value ? await props.admin.groupMembers(selectedId.value) : [];
  };
  watch(selectedId, loadMembers);
  onMounted(async () => {
    await props.admin.loadGroups();
    if (!selectedId.value && groups.value[0]) selectedId.value = groups.value[0].id;
  });

  const add = async () => {
    if (await props.admin.addGroupMember(selectedId.value, addUserId.value)) {
      addUserId.value = "";
      await loadMembers();
    } else toast.error(props.admin.error.value || "Could not add member");
  };
  const removeMember = async (userId: string) => {
    if (await props.admin.removeGroupMember(selectedId.value, userId)) await loadMembers();
    else toast.error(props.admin.error.value || "Could not remove member");
  };

  const createOpen = shallowRef(false);
  const form = shallowRef({ name: "", description: "" });
  const create = async () => {
    const created = await props.admin.createGroup({
      name: form.value.name.trim(),
      description: form.value.description.trim(),
    });
    if (!created) {
      toast.error(props.admin.error.value || "Could not create group");
      return;
    }
    createOpen.value = false;
    form.value = { name: "", description: "" };
    selectedId.value = created.id;
  };
  const remove = async () => {
    const group = selected.value;
    if (group && (await props.admin.deleteGroup(group.id))) {
      toast.success(`${group.name} deleted`);
      selectedId.value = groups.value[0]?.id ?? "";
    } else toast.error(props.admin.error.value || "Could not delete group");
  };
</script>

<template>
  <div class="grid gap-5 lg:grid-cols-[minmax(0,320px)_1fr]">
    <div class="bg-card rounded-2xl border shadow-sm">
      <div class="flex items-center justify-between border-b px-4 py-3">
        <h2 class="text-sm font-semibold">Groups</h2>
        <Button size="sm" variant="ghost" @click="createOpen = true"
          ><PlusIcon class="size-4" />New</Button
        >
      </div>
      <ul v-if="groups.length" class="p-2" role="listbox" aria-label="Groups">
        <li v-for="group in groups" :key="group.id">
          <button
            type="button"
            role="option"
            :aria-selected="group.id === selectedId"
            class="hover:bg-accent aria-selected:bg-accent flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left text-sm"
            @click="selectedId = group.id"
          >
            <UsersIcon class="text-muted-foreground size-4 shrink-0" />
            <span class="min-w-0 flex-1 truncate">{{ group.name }}</span>
            <span class="text-muted-foreground text-xs">{{ group.member_count }}</span>
          </button>
        </li>
      </ul>
      <p v-else class="text-muted-foreground p-6 text-sm">
        Groups gather people across departments, like a committee or a project team.
      </p>
    </div>

    <div v-if="selected" class="bg-card rounded-2xl border p-5 shadow-sm">
      <div class="flex items-start gap-3">
        <div class="min-w-0 flex-1">
          <h2 class="text-lg font-semibold">{{ selected.name }}</h2>
          <p class="text-muted-foreground text-sm">
            {{ selected.description || "No description" }}
          </p>
        </div>
        <Button size="sm" variant="ghost" class="text-destructive" @click="remove"
          ><Trash2Icon class="size-4" /><span class="sr-only">Delete group</span></Button
        >
      </div>
      <form class="mt-5 flex gap-2" @submit.prevent="add">
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
              initialsOf(member.name || member.email)
            }}</AvatarFallback></Avatar
          >
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium">{{ member.name || member.email }}</p>
            <p class="text-muted-foreground truncate text-xs">{{ member.email }}</p>
          </div>
          <Button
            size="icon"
            variant="ghost"
            class="size-8"
            :aria-label="`Remove ${member.name || member.email}`"
            @click="removeMember(member.user_id)"
            ><XIcon class="size-4"
          /></Button>
        </li>
      </ul>
      <p v-else class="text-muted-foreground mt-4 text-sm">No members yet.</p>
    </div>
  </div>

  <Dialog v-model:open="createOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>New group</DialogTitle>
        <DialogDescription
          >Share files with a group instead of listing people one by one.</DialogDescription
        >
      </DialogHeader>
      <form class="space-y-4" @submit.prevent="create">
        <div class="space-y-1.5">
          <Label for="group-name">Name</Label>
          <Input id="group-name" v-model="form.name" placeholder="Tender Committee" />
        </div>
        <div class="space-y-1.5">
          <Label for="group-description">Description</Label>
          <Input id="group-description" v-model="form.description" />
        </div>
      </form>
      <DialogFooter>
        <Button variant="outline" @click="createOpen = false">Cancel</Button>
        <Button :disabled="!form.name.trim()" @click="create">Create group</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
