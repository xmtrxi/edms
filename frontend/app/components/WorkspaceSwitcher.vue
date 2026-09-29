<script setup lang="ts">
  import { Building2Icon, CheckIcon, ChevronsUpDownIcon, PlusIcon, UserIcon } from "@lucide/vue";
  import { computed, shallowRef } from "vue";
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
  import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
  } from "@/components/ui/dropdown-menu";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import { useSidebar } from "@/components/ui/sidebar";
  import type { Organization } from "@/types/api";
  import { initialsOf } from "@/utils/format";

  const auth = useAuthStore();
  const { state } = useSidebar();
  const collapsed = computed(() => state.value === "collapsed");

  const createOpen = shallowRef(false);
  const newName = shallowRef("");
  const creating = shallowRef(false);

  const current = computed(() => auth.organization);
  const roleLabel: Record<string, string> = {
    owner: "Owner",
    admin: "Admin",
    member: "Member",
    guest: "Guest",
  };
  // Nothing to switch to and nothing to create: show a plain label.
  const interactive = computed(() => auth.organizations.length > 1 || auth.canCreateOrganization);

  const subtitle = (org: Organization | null) => {
    if (!org) return "";
    if (org.kind === "personal") return "Personal workspace";
    return org.role ? roleLabel[org.role] : "Organization";
  };

  const choose = async (org: Organization) => {
    if (org.id === current.value?.id) return;
    const ok = await auth.switchOrganization(org.id);
    if (!ok) toast.error(auth.error || "Could not switch workspace");
  };

  const create = async () => {
    if (!newName.value.trim()) return;
    creating.value = true;
    const ok = await auth.createOrganization(newName.value.trim());
    creating.value = false;
    if (!ok) toast.error(auth.error || "Could not create organization");
  };
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child :disabled="!interactive">
      <button
        type="button"
        class="hover:bg-sidebar-accent focus-visible:ring-sidebar-ring flex w-full items-center gap-2.5 rounded-lg px-1.5 py-1 text-left outline-none focus-visible:ring-2 disabled:cursor-default disabled:hover:bg-transparent"
        :aria-label="`Workspace: ${current?.name ?? 'none'}`"
      >
        <div
          class="from-primary to-primary/60 text-primary-foreground shadow-glow grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-gradient-to-br text-xs font-semibold"
        >
          <UserIcon v-if="current?.kind === 'personal'" class="h-4 w-4" />
          <span v-else>{{ initialsOf(current?.name ?? "Lumen") }}</span>
        </div>
        <div v-if="!collapsed" class="flex min-w-0 flex-1 flex-col leading-tight">
          <span class="truncate text-sm font-semibold tracking-tight">{{
            current?.name ?? "Lumen DMS"
          }}</span>
          <span class="text-muted-foreground truncate text-[11px]">{{ subtitle(current) }}</span>
        </div>
        <ChevronsUpDownIcon
          v-if="!collapsed && interactive"
          class="text-muted-foreground h-4 w-4 shrink-0"
        />
      </button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start" class="w-64">
      <DropdownMenuLabel class="text-muted-foreground text-xs font-medium">
        Workspaces
      </DropdownMenuLabel>
      <DropdownMenuItem
        v-for="org in auth.organizations"
        :key="org.id"
        class="gap-2.5 py-2"
        @select="choose(org)"
      >
        <div
          class="bg-muted text-muted-foreground grid h-7 w-7 shrink-0 place-items-center rounded-md text-[11px] font-semibold"
        >
          <UserIcon v-if="org.kind === 'personal'" class="h-3.5 w-3.5" />
          <Building2Icon v-else-if="!org.name" class="h-3.5 w-3.5" />
          <span v-else>{{ initialsOf(org.name) }}</span>
        </div>
        <div class="flex min-w-0 flex-1 flex-col leading-tight">
          <span class="truncate text-sm">{{ org.name }}</span>
          <span class="text-muted-foreground truncate text-[11px]">{{ subtitle(org) }}</span>
        </div>
        <CheckIcon v-if="org.id === current?.id" class="text-primary h-4 w-4 shrink-0" />
      </DropdownMenuItem>
      <template v-if="auth.canCreateOrganization">
        <DropdownMenuSeparator />
        <DropdownMenuItem class="gap-2.5 py-2" @select="createOpen = true">
          <div class="grid h-7 w-7 shrink-0 place-items-center rounded-md border border-dashed">
            <PlusIcon class="h-3.5 w-3.5" />
          </div>
          <span class="text-sm">Create organization</span>
        </DropdownMenuItem>
      </template>
    </DropdownMenuContent>
  </DropdownMenu>

  <Dialog v-model:open="createOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Create an organization</DialogTitle>
        <DialogDescription>
          Folders and files you add there belong to the organization, and you can invite your team.
        </DialogDescription>
      </DialogHeader>
      <form class="space-y-2" @submit.prevent="create">
        <Label for="org-name">Organization name</Label>
        <Input id="org-name" v-model="newName" placeholder="Acme Ltd" autocomplete="organization" />
      </form>
      <DialogFooter>
        <Button variant="outline" @click="createOpen = false">Cancel</Button>
        <Button :disabled="!newName.trim() || creating" @click="create">
          {{ creating ? "Creating…" : "Create organization" }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
