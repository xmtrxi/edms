<script setup lang="ts">
  import {
    CheckIcon,
    CopyIcon,
    MailIcon,
    MoreHorizontalIcon,
    SearchIcon,
    UserIcon,
    UserPlusIcon,
    UsersIcon,
  } from "@lucide/vue";
  import { computed, onMounted, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import type { Member, MembershipRole } from "@/types/api";
  import { formatDate, initialsOf } from "@/utils/format";

  definePageMeta({
    title: "Members",
    description: "People in this organization, their roles, and pending invitations.",
    breadcrumbs: [{ label: "Administration", to: "/" }, { label: "Members" }],
  });

  const auth = useAuthStore();
  const org = organizationsService();

  const roleOptions: { value: MembershipRole; label: string; description: string }[] = [
    { value: "owner", label: "Owner", description: "Full control, including owners and billing" },
    { value: "admin", label: "Admin", description: "Manage members, departments and settings" },
    { value: "member", label: "Member", description: "Works with the files shared with them" },
    { value: "guest", label: "Guest", description: "Sees only what is shared with them directly" },
  ];
  const roleLabel = (role: string) =>
    roleOptions.find((item) => item.value === role)?.label ?? role;
  const isOwner = computed(() => auth.organization?.role === "owner");
  // Only owners may hand out or take away the owner role.
  const assignableRoles = computed(() =>
    roleOptions.filter((item) => item.value !== "owner" || isOwner.value),
  );

  const tab = shallowRef<"members" | "invitations">("members");
  const query = shallowRef("");
  const filtered = computed(() => {
    const needle = query.value.trim().toLowerCase();
    if (!needle) return org.members.value;
    return org.members.value.filter((item) =>
      `${item.name} ${item.email}`.toLowerCase().includes(needle),
    );
  });

  const canManage = (member: Member) =>
    auth.isOrgAdmin &&
    member.user_id !== auth.user?.id &&
    (member.role !== "owner" || isOwner.value);

  const changeRole = async (member: Member, role: MembershipRole) => {
    if (role === member.role) return;
    if (await org.updateMember(member.id, { role })) {
      toast.success(`${member.name || member.email} is now ${roleLabel(role).toLowerCase()}`);
    } else toast.error(org.error.value);
  };

  const toggleSuspended = async (member: Member) => {
    const status = member.status === "active" ? "suspended" : "active";
    if (await org.updateMember(member.id, { status })) {
      toast.success(status === "suspended" ? "Member suspended" : "Member reactivated");
    } else toast.error(org.error.value);
  };

  const removing = shallowRef<Member | null>(null);
  const confirmRemove = async () => {
    const member = removing.value;
    removing.value = null;
    if (!member) return;
    if (await org.removeMember(member.id)) toast.success("Member removed");
    else toast.error(org.error.value);
  };

  const inviteOpen = shallowRef(false);
  const inviteEmail = shallowRef("");
  const inviteName = shallowRef("");
  const inviteRole = shallowRef<MembershipRole>("member");
  const inviting = shallowRef(false);
  const inviteLink = shallowRef("");
  const copied = shallowRef(false);

  const openInvite = () => {
    inviteEmail.value = "";
    inviteName.value = "";
    inviteRole.value = "member";
    inviteLink.value = "";
    copied.value = false;
    inviteOpen.value = true;
  };

  const sendInvite = async () => {
    inviting.value = true;
    const link = await org.invite({
      email: inviteEmail.value.trim(),
      name: inviteName.value.trim(),
      role: inviteRole.value,
    });
    inviting.value = false;
    if (!link) {
      toast.error(org.error.value || "Could not send invitation");
      return;
    }
    inviteLink.value = link;
    toast.success(`Invitation sent to ${inviteEmail.value.trim()}`);
  };

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(inviteLink.value);
      copied.value = true;
    } catch {
      toast.error("Copy failed. Select the link and copy it manually.");
    }
  };

  const revoke = async (id: string) => {
    if (await org.revokeInvitation(id)) toast.success("Invitation revoked");
    else toast.error(org.error.value);
  };

  onMounted(() => {
    if (!auth.isPersonalWorkspace) void org.load(auth.isOrgAdmin);
  });
</script>

<template>
  <Teleport v-if="auth.isOrgAdmin && !auth.isPersonalWorkspace" to="#page-actions-target">
    <Button size="sm" class="gap-2" @click="openInvite">
      <UserPlusIcon class="size-4" />Invite people
    </Button>
  </Teleport>

  <div
    v-if="auth.isPersonalWorkspace"
    class="bg-card mx-auto max-w-lg rounded-2xl border p-8 text-center shadow-sm"
  >
    <div class="bg-primary/10 text-primary mx-auto mb-4 grid size-12 place-items-center rounded-xl">
      <UserIcon class="size-6" />
    </div>
    <h2 class="text-lg font-semibold">This is your personal workspace</h2>
    <p class="text-muted-foreground mt-2 text-sm leading-relaxed">
      Only you can see it. To work with other people, create an organization and invite them. You
      can still share single files from here.
    </p>
    <p v-if="auth.canCreateOrganization" class="text-muted-foreground mt-4 text-sm">
      Use the workspace menu at the top of the sidebar to create one.
    </p>
  </div>

  <div v-else class="space-y-4">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <Tabs v-if="auth.isOrgAdmin" v-model="tab">
        <TabsList>
          <TabsTrigger value="members">
            Members
            <Badge variant="secondary" class="ml-1.5 h-5 px-1.5">{{
              org.members.value.length
            }}</Badge>
          </TabsTrigger>
          <TabsTrigger value="invitations">
            Pending invitations
            <Badge variant="secondary" class="ml-1.5 h-5 px-1.5">{{
              org.invitations.value.length
            }}</Badge>
          </TabsTrigger>
        </TabsList>
      </Tabs>
      <div v-if="tab === 'members'" class="relative w-full sm:max-w-xs">
        <SearchIcon class="text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2" />
        <Input v-model="query" class="pl-9" placeholder="Find by name or email" />
      </div>
    </div>

    <div
      v-if="org.error.value && !org.loading.value && !org.members.value.length"
      class="bg-destructive/5 text-destructive rounded-xl p-3 text-sm"
    >
      {{ org.error.value }}
    </div>

    <div v-if="tab === 'members'" class="bg-card overflow-hidden rounded-2xl border shadow-sm">
      <div v-if="org.loading.value && !org.members.value.length" class="space-y-3 p-4">
        <Skeleton v-for="n in 4" :key="n" class="h-12 w-full" />
      </div>
      <div
        v-else-if="!filtered.length"
        class="text-muted-foreground flex flex-col items-center gap-2 p-10 text-sm"
      >
        <UsersIcon class="size-5" />
        {{ query ? "No one matches that search." : "No members yet." }}
      </div>
      <div
        v-for="member in filtered"
        :key="member.id"
        class="flex items-center gap-4 border-b p-4 last:border-0"
      >
        <Avatar>
          <AvatarFallback>{{ initialsOf(member.name || member.email) }}</AvatarFallback>
        </Avatar>
        <div class="min-w-0 flex-1">
          <p class="flex items-center gap-2 text-sm font-semibold">
            <span class="truncate">{{ member.name || member.email }}</span>
            <Badge v-if="member.user_id === auth.user?.id" variant="outline" class="h-5 px-1.5">
              You
            </Badge>
            <Badge v-if="member.status === 'suspended'" variant="destructive" class="h-5 px-1.5">
              Suspended
            </Badge>
          </p>
          <p class="text-muted-foreground truncate text-xs">
            {{ member.email }} · joined {{ formatDate(member.joined_at) }}
          </p>
        </div>

        <Select
          v-if="canManage(member)"
          :model-value="member.role"
          @update:model-value="(value) => changeRole(member, value as MembershipRole)"
        >
          <SelectTrigger class="h-8 w-32" :aria-label="`Role for ${member.email}`">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="role in assignableRoles" :key="role.value" :value="role.value">
              {{ role.label }}
            </SelectItem>
          </SelectContent>
        </Select>
        <Badge v-else variant="secondary">{{ roleLabel(member.role) }}</Badge>

        <DropdownMenu v-if="canManage(member)">
          <DropdownMenuTrigger as-child>
            <Button variant="ghost" size="icon" class="size-8" aria-label="Member actions">
              <MoreHorizontalIcon class="size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem @select="toggleSuspended(member)">
              {{ member.status === "active" ? "Suspend access" : "Reactivate" }}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem class="text-destructive" @select="removing = member">
              Remove from organization
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <div v-else-if="auth.isOrgAdmin" class="size-8" />
      </div>
    </div>

    <div v-else class="bg-card overflow-hidden rounded-2xl border shadow-sm">
      <div
        v-if="!org.invitations.value.length"
        class="text-muted-foreground flex flex-col items-center gap-2 p-10 text-sm"
      >
        <MailIcon class="size-5" />
        No pending invitations.
      </div>
      <div
        v-for="invite in org.invitations.value"
        :key="invite.id"
        class="flex items-center gap-4 border-b p-4 last:border-0"
      >
        <div class="bg-muted text-muted-foreground grid size-10 place-items-center rounded-full">
          <MailIcon class="size-4" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-semibold">{{ invite.name || invite.email }}</p>
          <p class="text-muted-foreground truncate text-xs">
            {{ invite.email }} · expires {{ formatDate(invite.expires_at) }}
          </p>
        </div>
        <Badge variant="secondary">{{ roleLabel(invite.role) }}</Badge>
        <Button variant="ghost" size="sm" @click="revoke(invite.id)">Revoke</Button>
      </div>
    </div>
  </div>

  <Dialog v-model:open="inviteOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Invite people to {{ auth.organization?.name }}</DialogTitle>
        <DialogDescription>
          They get an email with a link. People who already have an account join by signing in.
        </DialogDescription>
      </DialogHeader>

      <div v-if="inviteLink" class="space-y-3">
        <p class="text-sm">
          Invitation sent. You can also share this link directly; it works once and expires in 7
          days.
        </p>
        <div class="flex gap-2">
          <Input
            :model-value="inviteLink"
            readonly
            class="font-mono text-xs"
            @focus="($event.target as HTMLInputElement).select()"
          />
          <Button variant="outline" size="icon" aria-label="Copy invitation link" @click="copyLink">
            <CheckIcon v-if="copied" class="size-4" />
            <CopyIcon v-else class="size-4" />
          </Button>
        </div>
      </div>

      <form v-else class="space-y-4" @submit.prevent="sendInvite">
        <div class="space-y-1.5">
          <Label for="invite-email">Email</Label>
          <Input
            id="invite-email"
            v-model="inviteEmail"
            type="email"
            placeholder="name@company.com"
            autocomplete="off"
          />
        </div>
        <div class="space-y-1.5">
          <Label for="invite-name"
            >Name <span class="text-muted-foreground">(optional)</span></Label
          >
          <Input
            id="invite-name"
            v-model="inviteName"
            placeholder="Jane Wanjiru"
            autocomplete="off"
          />
        </div>
        <div class="space-y-1.5">
          <Label>Role</Label>
          <Select v-model="inviteRole">
            <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="role in assignableRoles" :key="role.value" :value="role.value">
                {{ role.label }}
              </SelectItem>
            </SelectContent>
          </Select>
          <p class="text-muted-foreground text-xs">
            {{ roleOptions.find((item) => item.value === inviteRole)?.description }}
          </p>
        </div>
      </form>

      <DialogFooter>
        <template v-if="inviteLink">
          <Button variant="outline" @click="openInvite">Invite someone else</Button>
          <Button @click="inviteOpen = false">Done</Button>
        </template>
        <template v-else>
          <Button variant="outline" @click="inviteOpen = false">Cancel</Button>
          <Button :disabled="!inviteEmail.includes('@') || inviting" @click="sendInvite">
            {{ inviting ? "Sending…" : "Send invitation" }}
          </Button>
        </template>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <AlertDialog :open="Boolean(removing)" @update:open="(open) => !open && (removing = null)">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>Remove {{ removing?.name || removing?.email }}?</AlertDialogTitle>
        <AlertDialogDescription>
          They lose access to this organization straight away. Files they added stay with the
          organization.
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>Cancel</AlertDialogCancel>
        <AlertDialogAction
          class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          @click="confirmRemove"
        >
          Remove
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
