<script setup lang="ts">
  import {
    Building2Icon,
    CheckIcon,
    ChevronDownIcon,
    CopyIcon,
    FolderIcon,
    GlobeIcon,
    InboxIcon,
    LinkIcon,
    LockIcon,
    ShieldIcon,
    UserIcon,
    UsersIcon,
    XIcon,
  } from "@lucide/vue";
  import { computed, shallowRef, watch } from "vue";
  import { toast } from "vue-sonner";

  import PrincipalPicker from "@/components/sharing/PrincipalPicker.vue";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
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
  import { Switch } from "@/components/ui/switch";
  import type {
    DocumentAccess,
    FolderGrant,
    FolderRole,
    PrincipalOption,
    PrincipalType,
    Sensitivity,
    ShareRole,
    ShareView,
  } from "@/types/api";
  import { formatDate, initialsOf } from "@/utils/format";

  const open = defineModel<boolean>("open", { required: true });
  const props = defineProps<{ documentId: string; title: string }>();
  const emit = defineEmits<{ changed: [] }>();

  const sharing = sharingService();
  const access = shallowRef<DocumentAccess | null>(null);
  const loading = shallowRef(false);
  const busy = shallowRef(false);
  const revealedLink = shallowRef<{ scope: string; url: string } | null>(null);
  const copied = shallowRef(false);

  const grantRoles: Array<{ value: FolderRole; label: string }> = [
    { value: "viewer", label: "Viewer" },
    { value: "commenter", label: "Commenter" },
    { value: "editor", label: "Editor" },
    { value: "manager", label: "Manager" },
  ];
  const shareRoles: Array<{ value: ShareRole; label: string }> = [
    { value: "viewer", label: "Viewer" },
    { value: "commenter", label: "Commenter" },
    { value: "editor", label: "Editor" },
  ];
  const sensitivities: Array<{ value: Sensitivity; label: string; description: string }> = [
    { value: "public", label: "Public", description: "Fine to share with anyone" },
    { value: "internal", label: "Internal", description: "For people in the organization" },
    {
      value: "confidential",
      label: "Confidential",
      description: "No public links; share carefully",
    },
    {
      value: "restricted",
      label: "Restricted",
      description: "Only named people and groups, even admins need access",
    },
  ];
  const roleLabel = (role: string) =>
    grantRoles.find((item) => item.value === role)?.label ??
    (role === "contributor" ? "Contributor" : role === "uploader" ? "Uploader" : role);

  const iconFor: Record<PrincipalType, typeof UserIcon> = {
    user: UserIcon,
    department: Building2Icon,
    group: UsersIcon,
    role: ShieldIcon,
    everyone: GlobeIcon,
  };

  const canShare = computed(() => Boolean(access.value?.can_share));
  const restricted = computed(() => access.value?.sensitivity === "restricted");
  const selectedKeys = computed(() =>
    (access.value?.grants ?? []).map((grant) => `${grant.principal_type}|${grant.principal_id}`),
  );
  const orgLink = computed(() => access.value?.links.find((link) => link.link_scope !== "anyone"));
  const publicLink = computed(() =>
    access.value?.links.find((link) => link.link_scope === "anyone"),
  );
  const publicLinksAllowed = computed(
    () =>
      Boolean(access.value?.policy.allow_public_links) &&
      access.value?.sensitivity !== "confidential" &&
      !restricted.value,
  );

  const load = async () => {
    loading.value = true;
    access.value = await sharing.access(props.documentId);
    loading.value = false;
    if (!access.value) toast.error(sharing.error.value || "Could not load sharing settings");
  };

  watch(open, (value) => {
    if (!value) return;
    revealedLink.value = null;
    copied.value = false;
    void load();
  });

  const run = async (action: () => Promise<unknown>, success?: string) => {
    busy.value = true;
    const result = await action();
    busy.value = false;
    if (result === null || result === false) {
      toast.error(sharing.error.value || "That did not work");
      return false;
    }
    if (success) toast.success(success);
    await load();
    emit("changed");
    return true;
  };

  const addPrincipal = (option: PrincipalOption) =>
    run(
      () =>
        sharing.grant(props.documentId, {
          principal_type: option.type,
          principal_id: option.id,
          role: "viewer",
        }),
      `${option.name} can now view this file`,
    );

  const addEmail = async (email: string) => {
    busy.value = true;
    const result = await sharing.shareWithPerson(props.documentId, {
      email,
      role: "viewer",
      notify: true,
    });
    busy.value = false;
    if (!result) {
      toast.error(sharing.error.value || "Could not share with that address");
      return;
    }
    if (result.link) revealedLink.value = { scope: `Link for ${email}`, url: result.link };
    toast.success(
      result.kind === "grant"
        ? `Shared with ${email}`
        : `Shared with ${email} (outside your organization)`,
    );
    await load();
    emit("changed");
  };

  const changeGrantRole = (grant: FolderGrant, role: FolderRole) =>
    run(() =>
      sharing.grant(props.documentId, {
        principal_type: grant.principal_type,
        principal_id: grant.principal_id,
        role,
      }),
    );

  const removeGrant = (grant: FolderGrant) =>
    run(() => sharing.removeGrant(props.documentId, grant.id), `Removed ${grant.name}`);

  const changeShareRole = (share: ShareView, role: ShareRole) =>
    run(() => sharing.updateShareRole(props.documentId, share.id, role));

  const revoke = (share: ShareView) =>
    run(() => sharing.revokeShare(props.documentId, share.id), "Access removed");

  const setInherit = (value: boolean) =>
    run(
      () => sharing.updateSettings(props.documentId, { inherit_permissions: value }),
      value ? "This file follows its folder again" : "This file now has its own access list",
    );

  const setSensitivity = (value: Sensitivity) =>
    run(
      () => sharing.updateSettings(props.documentId, { sensitivity: value }),
      `Marked as ${value}`,
    );

  const linkPassword = shallowRef("");
  const linkDownload = shallowRef(true);
  const createLink = async (scope: "organization" | "anyone") => {
    busy.value = true;
    const created = await sharing.createLink(props.documentId, {
      scope,
      role: "viewer",
      allow_download: scope === "organization" ? true : linkDownload.value,
      password: scope === "anyone" && linkPassword.value ? linkPassword.value : undefined,
    });
    busy.value = false;
    if (!created) {
      toast.error(sharing.error.value || "Could not create the link");
      return;
    }
    revealedLink.value = {
      scope: scope === "anyone" ? "Anyone with the link" : "People in your organization",
      url: created.url,
    };
    copied.value = false;
    linkPassword.value = "";
    await load();
  };

  const copyLink = async () => {
    if (!revealedLink.value) return;
    try {
      await navigator.clipboard.writeText(revealedLink.value.url);
      copied.value = true;
    } catch {
      toast.error("Copy failed. Select the link and copy it manually.");
    }
  };
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
      <DialogHeader>
        <DialogTitle class="truncate pr-6">Share “{{ title }}”</DialogTitle>
        <DialogDescription>
          Give people, departments, groups or roles access. People outside the organization get
          their own secure link.
        </DialogDescription>
      </DialogHeader>

      <div v-if="loading && !access" class="space-y-3 py-2">
        <Skeleton class="h-9 w-full" />
        <Skeleton class="h-12 w-full" />
        <Skeleton class="h-12 w-full" />
      </div>

      <div v-else-if="access" class="space-y-6">
        <div v-if="canShare" class="space-y-2">
          <PrincipalPicker
            allow-email
            :selected-keys="selectedKeys"
            :exclude="restricted ? ['department', 'role', 'everyone'] : []"
            @select="addPrincipal"
            @email="addEmail"
          />
          <p v-if="restricted" class="text-muted-foreground text-xs">
            Restricted files can only be shared with named people and groups in your organization.
          </p>
        </div>
        <p v-else class="bg-muted text-muted-foreground rounded-lg p-3 text-sm">
          You can see who has access, but you can’t change it. Ask the owner or a manager of this
          file.
        </p>

        <div
          v-if="access.pending_requests > 0"
          class="border-primary/30 bg-primary/5 flex items-center gap-3 rounded-lg border p-3 text-sm"
        >
          <InboxIcon class="text-primary size-4 shrink-0" />
          <span class="flex-1"
            >{{ access.pending_requests }} pending access
            {{ access.pending_requests === 1 ? "request" : "requests" }}</span
          >
          <Button variant="link" size="sm" as-child class="h-auto p-0">
            <NuxtLink to="/shared#requests">Review</NuxtLink>
          </Button>
        </div>

        <section>
          <h3 class="mb-2 text-sm font-semibold">People with access</h3>
          <ul class="divide-y rounded-lg border">
            <li class="flex items-center gap-3 px-3 py-2.5">
              <Avatar class="size-8">
                <AvatarFallback class="text-xs">{{ initialsOf(access.owner_name) }}</AvatarFallback>
              </Avatar>
              <span class="min-w-0 flex-1 truncate text-sm font-medium">{{
                access.owner_name || "Uploader"
              }}</span>
              <span class="text-muted-foreground text-xs">Owner</span>
            </li>

            <li
              v-for="grant in access.grants"
              :key="grant.id"
              class="flex items-center gap-3 px-3 py-2.5"
            >
              <span class="bg-muted grid size-8 shrink-0 place-items-center rounded-full">
                <component :is="iconFor[grant.principal_type]" class="size-4" />
              </span>
              <span class="min-w-0 flex-1">
                <span class="block truncate text-sm font-medium">{{ grant.name }}</span>
                <span class="text-muted-foreground block truncate text-xs">{{
                  grant.description ?? (grant.principal_type === "everyone" ? "All members" : "")
                }}</span>
              </span>
              <Select
                v-if="canShare"
                :model-value="grant.role"
                :disabled="busy"
                @update:model-value="(value) => changeGrantRole(grant, value as FolderRole)"
              >
                <SelectTrigger class="h-8 w-32" :aria-label="`Role for ${grant.name}`"
                  ><SelectValue
                /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="role in grantRoles" :key="role.value" :value="role.value">{{
                    role.label
                  }}</SelectItem>
                </SelectContent>
              </Select>
              <span v-else class="text-muted-foreground text-xs">{{ roleLabel(grant.role) }}</span>
              <Button
                v-if="canShare"
                variant="ghost"
                size="icon"
                class="size-8"
                :disabled="busy"
                :aria-label="`Remove ${grant.name}`"
                @click="removeGrant(grant)"
                ><XIcon class="size-4"
              /></Button>
            </li>

            <li
              v-for="share in access.shares"
              :key="share.id"
              class="flex items-center gap-3 px-3 py-2.5"
            >
              <Avatar class="size-8">
                <AvatarFallback class="text-xs">{{
                  initialsOf(share.name || share.email)
                }}</AvatarFallback>
              </Avatar>
              <span class="min-w-0 flex-1">
                <span class="flex items-center gap-2">
                  <span class="truncate text-sm font-medium">{{ share.name || share.email }}</span>
                  <Badge variant="outline" class="h-5 shrink-0 px-1.5 text-[10px]">External</Badge>
                </span>
                <span class="text-muted-foreground block truncate text-xs"
                  >{{ share.email }} · until {{ formatDate(share.expires_at) }}</span
                >
              </span>
              <Select
                v-if="canShare"
                :model-value="share.role"
                :disabled="busy"
                @update:model-value="(value) => changeShareRole(share, value as ShareRole)"
              >
                <SelectTrigger class="h-8 w-32" :aria-label="`Role for ${share.email}`"
                  ><SelectValue
                /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="role in shareRoles" :key="role.value" :value="role.value">{{
                    role.label
                  }}</SelectItem>
                </SelectContent>
              </Select>
              <Button
                v-if="canShare"
                variant="ghost"
                size="icon"
                class="size-8"
                :disabled="busy"
                :aria-label="`Remove ${share.email}`"
                @click="revoke(share)"
                ><XIcon class="size-4"
              /></Button>
            </li>
          </ul>

          <div v-if="access.folder_id" class="mt-3 rounded-lg border">
            <div class="flex items-center gap-3 px-3 py-2.5">
              <FolderIcon class="text-muted-foreground size-4 shrink-0" />
              <div class="min-w-0 flex-1">
                <Label for="follow-folder" class="text-sm font-medium"
                  >Follow access from “{{ access.folder_name }}”</Label
                >
                <p class="text-muted-foreground text-xs">
                  {{
                    access.inherit_permissions
                      ? "Anyone with access to the folder can open this file."
                      : "Only the people listed above can open this file."
                  }}
                </p>
              </div>
              <Switch
                id="follow-folder"
                :model-value="access.inherit_permissions"
                :disabled="!canShare || busy || restricted"
                @update:model-value="setInherit"
              />
            </div>
            <Collapsible v-if="access.inherit_permissions && access.inherited.length">
              <CollapsibleTrigger
                class="text-muted-foreground hover:text-foreground flex w-full items-center gap-2 border-t px-3 py-2 text-xs"
              >
                <ChevronDownIcon class="size-3.5" />
                {{ access.inherited.length }} from the folder
              </CollapsibleTrigger>
              <CollapsibleContent>
                <ul class="divide-y border-t">
                  <li
                    v-for="grant in access.inherited"
                    :key="grant.id"
                    class="flex items-center gap-3 px-3 py-2 text-sm"
                  >
                    <component
                      :is="iconFor[grant.principal_type]"
                      class="text-muted-foreground size-4"
                    />
                    <span class="min-w-0 flex-1 truncate">{{ grant.name }}</span>
                    <span class="text-muted-foreground text-xs">{{ roleLabel(grant.role) }}</span>
                  </li>
                </ul>
              </CollapsibleContent>
            </Collapsible>
          </div>
        </section>

        <section class="grid gap-4 sm:grid-cols-2">
          <div class="space-y-1.5">
            <Label>Sensitivity</Label>
            <Select
              :model-value="access.sensitivity"
              :disabled="!canShare || busy"
              @update:model-value="(value) => setSensitivity(value as Sensitivity)"
            >
              <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="item in sensitivities" :key="item.value" :value="item.value">{{
                  item.label
                }}</SelectItem>
              </SelectContent>
            </Select>
            <p class="text-muted-foreground text-xs">
              {{ sensitivities.find((item) => item.value === access?.sensitivity)?.description }}
            </p>
          </div>
        </section>

        <section v-if="canShare && !restricted" class="space-y-3">
          <h3 class="text-sm font-semibold">Links</h3>
          <div class="flex flex-wrap items-center gap-3 rounded-lg border px-3 py-2.5">
            <Building2Icon class="text-muted-foreground size-4 shrink-0" />
            <div class="min-w-0 flex-1">
              <p class="text-sm font-medium">People in your organization</p>
              <p class="text-muted-foreground text-xs">
                {{
                  orgLink
                    ? `Link active until ${formatDate(orgLink.expires_at)}`
                    : "Anyone signed in to your organization can open it"
                }}
              </p>
            </div>
            <Button
              v-if="orgLink"
              size="sm"
              variant="ghost"
              :disabled="busy"
              @click="revoke(orgLink)"
              >Turn off</Button
            >
            <Button size="sm" variant="outline" :disabled="busy" @click="createLink('organization')"
              ><LinkIcon class="size-3.5" />{{ orgLink ? "New link" : "Create link" }}</Button
            >
          </div>

          <div class="rounded-lg border px-3 py-2.5">
            <div class="flex flex-wrap items-center gap-3">
              <GlobeIcon class="text-muted-foreground size-4 shrink-0" />
              <div class="min-w-0 flex-1">
                <p class="text-sm font-medium">Anyone with the link</p>
                <p class="text-muted-foreground text-xs">
                  <template v-if="!access.policy.allow_public_links"
                    >Turned off by your organization’s sharing policy</template
                  >
                  <template v-else-if="access.sensitivity === 'confidential'"
                    >Not available for confidential files</template
                  >
                  <template v-else-if="publicLink"
                    >View only{{ publicLink.has_password ? " · password" : ""
                    }}{{ publicLink.allow_download ? "" : " · no download" }} · until
                    {{ formatDate(publicLink.expires_at) }}</template
                  >
                  <template v-else>View only, no sign-in needed</template>
                </p>
              </div>
              <Button
                v-if="publicLink"
                size="sm"
                variant="ghost"
                :disabled="busy"
                @click="revoke(publicLink)"
                >Turn off</Button
              >
            </div>
            <div
              v-if="publicLinksAllowed"
              class="mt-3 flex flex-wrap items-end gap-3 border-t pt-3"
            >
              <div class="min-w-40 flex-1 space-y-1">
                <Label for="link-password" class="text-xs">Password (optional)</Label>
                <div class="relative">
                  <LockIcon
                    class="text-muted-foreground absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2"
                  />
                  <Input
                    id="link-password"
                    v-model="linkPassword"
                    type="password"
                    autocomplete="new-password"
                    class="h-8 pl-8"
                    placeholder="At least 6 characters"
                  />
                </div>
              </div>
              <div class="flex items-center gap-2 pb-1">
                <Switch id="link-download" v-model="linkDownload" />
                <Label for="link-download" class="text-xs">Allow download</Label>
              </div>
              <Button size="sm" variant="outline" :disabled="busy" @click="createLink('anyone')"
                ><LinkIcon class="size-3.5" />{{
                  publicLink ? "Replace link" : "Create link"
                }}</Button
              >
            </div>
          </div>

          <div v-if="revealedLink" class="bg-muted space-y-2 rounded-lg p-3">
            <p class="text-xs font-medium">
              {{ revealedLink.scope }}. Copy it now; for security it can’t be shown again.
            </p>
            <div class="flex gap-2">
              <Input
                :model-value="revealedLink.url"
                readonly
                class="h-8 font-mono text-xs"
                @focus="($event.target as HTMLInputElement).select()"
              />
              <Button
                size="icon"
                variant="outline"
                class="size-8"
                aria-label="Copy link"
                @click="copyLink"
              >
                <CheckIcon v-if="copied" class="size-4" />
                <CopyIcon v-else class="size-4" />
              </Button>
            </div>
          </div>
        </section>
      </div>
    </DialogContent>
  </Dialog>
</template>
