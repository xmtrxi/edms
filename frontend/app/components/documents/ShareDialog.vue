<script setup lang="ts">
  import {
    CheckIcon,
    CopyIcon,
    GlobeIcon,
    LinkIcon,
    LockIcon,
    MailIcon,
    ShieldCheckIcon,
    UsersIcon,
    XIcon,
  } from "@lucide/vue";
  import { ref, shallowRef, computed } from "vue";
  import { toast } from "vue-sonner";

  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
    DialogFooter,
  } from "@/components/ui/dialog";
  import { Input } from "@/components/ui/input";
  import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
  } from "@/components/ui/select";
  import { Separator } from "@/components/ui/separator";
  import { Switch } from "@/components/ui/switch";
  import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
  import { cn } from "@/lib/utils";

  const props = defineProps<{ open: boolean; title?: string }>();
  const emit = defineEmits<{ "update:open": [value: boolean] }>();

  interface Person {
    name: string;
    initials: string;
    role: "Viewer" | "Commenter" | "Editor" | "Owner";
  }

  type Permission = "view" | "comment" | "edit" | "owner";

  const initialPeople: Person[] = [
    { name: "Aiko Tanaka", initials: "AT", role: "Owner" },
    { name: "Marcus Bell", initials: "MB", role: "Editor" },
    { name: "Sofia Reyes", initials: "SR", role: "Viewer" },
  ];

  const copied = ref(false);
  const email = ref("");
  const people = shallowRef(initialPeople);
  const perm = ref<string>("Editor");
  const linkAccess = ref<"restricted" | "anyone">("restricted");
  const linkExpiry = ref<"never" | "7d" | "30d" | "custom">("never");
  const notify = ref(true);
  const tab = ref<"people" | "link">("people");

  const url =
    typeof window !== "undefined"
      ? `${window.location.origin}/share/${(props.title ?? "doc").toLowerCase().replace(/[^a-z0-9]+/g, "-")}`
      : "";

  const permissionMatrix: {
    key: Permission;
    label: string;
    desc: string;
    capabilities: string[];
  }[] = [
    {
      key: "view",
      label: "View",
      desc: "Read & download only",
      capabilities: ["Read", "Download", "Print"],
    },
    {
      key: "comment",
      label: "Comment",
      desc: "View & leave comments",
      capabilities: ["Read", "Download", "Comment", "Suggest"],
    },
    {
      key: "edit",
      label: "Edit",
      desc: "Full edit access",
      capabilities: ["Read", "Download", "Comment", "Edit", "Share"],
    },
    {
      key: "owner",
      label: "Owner",
      desc: "Full control",
      capabilities: ["Read", "Download", "Comment", "Edit", "Share", "Manage", "Delete"],
    },
  ];

  const roleColors: Record<string, string> = {
    Owner: "bg-warning/10 text-warning border-warning/20",
    Editor: "bg-info/10 text-info border-info/20",
    Commenter: "bg-primary/10 text-primary border-primary/20",
    Viewer: "bg-secondary/80 text-muted-foreground border-border/50",
  };

  const allCapabilities = [
    "Read",
    "Download",
    "Print",
    "Comment",
    "Suggest",
    "Edit",
    "Share",
    "Manage",
    "Delete",
  ];

  const hasCapability = (permKey: Permission, cap: string): boolean => {
    const p = permissionMatrix.find((x) => x.key === permKey);
    return p ? p.capabilities.includes(cap) : false;
  };

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(url);
      copied.value = true;
      toast.success("Link copied to clipboard");
      setTimeout(() => (copied.value = false), 1800);
    } catch {
      toast.error("Couldn't copy link");
    }
  };

  const inviteUser = () => {
    if (!email.value.includes("@")) return toast.error("Enter a valid email");
    const initials = email.value.slice(0, 2).toUpperCase();
    people.value = [
      ...people.value,
      { name: email.value, initials, role: perm.value as Person["role"] },
    ];
    toast.success(`Invited ${email.value} as ${perm.value}`);
    email.value = "";
  };

  const updateRole = (name: string, val: string) => {
    if (val === "remove") {
      people.value = people.value.filter((x) => x.name !== name);
      return;
    }
    people.value = people.value.map((x) =>
      x.name === name ? { ...x, role: val as Person["role"] } : x,
    );
  };
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="max-w-lg">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2 text-base">
          <UsersIcon class="text-primary h-4 w-4" />
          Share
          <span v-if="title" class="text-muted-foreground truncate font-normal">— {{ title }}</span>
        </DialogTitle>
        <DialogDescription class="text-xs"
          >Invite people or copy a link with the right permissions.</DialogDescription
        >
      </DialogHeader>

      <Tabs v-model="tab">
        <TabsList class="grid w-full grid-cols-2">
          <TabsTrigger value="people" class="gap-1.5"
            ><UsersIcon class="h-3.5 w-3.5" /> People</TabsTrigger
          >
          <TabsTrigger value="link" class="gap-1.5"
            ><LinkIcon class="h-3.5 w-3.5" /> Link access</TabsTrigger
          >
        </TabsList>

        <TabsContent value="people" class="mt-4 space-y-4">
          <div class="flex items-center gap-2">
            <div class="relative flex-1">
              <MailIcon
                class="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2"
              />
              <Input
                v-model="email"
                placeholder="Add people by email…"
                class="pl-9"
                @keydown.enter="inviteUser"
              />
            </div>
            <Select v-model="perm">
              <SelectTrigger class="w-28"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="Viewer">Viewer</SelectItem>
                <SelectItem value="Commenter">Commenter</SelectItem>
                <SelectItem value="Editor">Editor</SelectItem>
                <SelectItem value="Owner">Owner</SelectItem>
              </SelectContent>
            </Select>
            <Button size="sm" @click="inviteUser">Invite</Button>
          </div>

          <div
            class="border-border bg-secondary/40 flex items-center justify-between rounded-lg border px-3 py-2"
          >
            <div class="flex items-center gap-2 text-xs">
              <Switch v-model="notify" id="notify" />
              <label for="notify" class="text-muted-foreground">Notify people by email</label>
            </div>
            <Badge variant="secondary" class="bg-card border-0 text-[10px]"
              >{{ people.length }} with access</Badge
            >
          </div>

          <div class="max-h-48 space-y-2 overflow-y-auto pr-1">
            <div
              v-for="p in people"
              :key="p.name"
              class="border-border bg-card flex items-center justify-between rounded-lg border p-2.5"
            >
              <div class="flex min-w-0 items-center gap-2">
                <Avatar class="h-8 w-8"
                  ><AvatarFallback class="bg-secondary text-[10px]">{{
                    p.initials
                  }}</AvatarFallback></Avatar
                >
                <div class="min-w-0">
                  <p class="truncate text-sm">{{ p.name }}</p>
                  <Badge
                    variant="secondary"
                    :class="`h-4 border px-1.5 text-[10px] ${roleColors[p.role]}`"
                    >{{ p.role }}</Badge
                  >
                </div>
              </div>
              <Select
                :default-value="p.role"
                @update:model-value="(v: string) => updateRole(p.name, v)"
              >
                <SelectTrigger class="h-7 w-24 text-xs"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="Viewer">Viewer</SelectItem>
                  <SelectItem value="Commenter">Commenter</SelectItem>
                  <SelectItem value="Editor">Editor</SelectItem>
                  <SelectItem value="Owner">Owner</SelectItem>
                  <SelectItem value="remove">Remove</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="link" class="mt-4 space-y-4">
          <div class="border-border bg-card rounded-xl border p-4">
            <div class="mb-3 flex items-center gap-3">
              <div
                :class="
                  cn(
                    'grid h-10 w-10 place-items-center rounded-xl',
                    linkAccess === 'anyone'
                      ? 'bg-warning/10 text-warning'
                      : 'bg-secondary text-muted-foreground',
                  )
                "
              >
                <GlobeIcon v-if="linkAccess === 'anyone'" class="h-5 w-5" />
                <LockIcon v-else class="h-5 w-5" />
              </div>
              <div class="min-w-0">
                <p class="text-sm font-medium">
                  {{ linkAccess === "anyone" ? "Anyone with the link" : "Restricted access" }}
                </p>
                <p class="text-muted-foreground text-xs">
                  {{
                    linkAccess === "anyone"
                      ? "No sign-in required"
                      : "Only invited people can access"
                  }}
                </p>
              </div>
              <Switch
                v-model="linkAccess"
                true-value="anyone"
                false-value="restricted"
                class="ml-auto"
              />
            </div>
            <Separator class="mb-3" />
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-muted-foreground text-xs">Permission level</span>
                <Select :model-value="linkAccess === 'anyone' ? 'Viewer' : 'restricted'">
                  <SelectTrigger class="h-7 w-28 text-xs"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="Viewer">Viewer</SelectItem>
                    <SelectItem value="Commenter">Commenter</SelectItem>
                    <SelectItem value="Editor">Editor</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div v-if="linkAccess === 'anyone'" class="flex items-center justify-between">
                <span class="text-muted-foreground text-xs">Link expiry</span>
                <Select v-model="linkExpiry">
                  <SelectTrigger class="h-7 w-28 text-xs"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="never">Never</SelectItem>
                    <SelectItem value="7d">7 days</SelectItem>
                    <SelectItem value="30d">30 days</SelectItem>
                    <SelectItem value="custom">Custom date</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </div>

          <div class="border-border bg-card rounded-xl border p-4">
            <p class="mb-3 text-xs font-semibold">Permission matrix</p>
            <div class="overflow-x-auto">
              <table class="w-full text-xs">
                <thead>
                  <tr class="border-border border-b">
                    <th class="text-muted-foreground w-24 py-1.5 pr-3 text-left font-medium">
                      Capability
                    </th>
                    <th
                      v-for="p in permissionMatrix"
                      :key="p.key"
                      class="px-1 py-1.5 text-center font-medium"
                    >
                      <div
                        :class="`rounded-md px-1.5 py-0.5 text-[10px] font-semibold ${
                          p.key === 'owner'
                            ? 'bg-warning/10 text-warning'
                            : p.key === 'edit'
                              ? 'bg-info/10 text-info'
                              : p.key === 'comment'
                                ? 'bg-primary/10 text-primary'
                                : 'bg-secondary/60 text-muted-foreground'
                        }`"
                      >
                        {{ p.label }}
                      </div>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="cap in allCapabilities"
                    :key="cap"
                    class="border-border/50 border-b last:border-0"
                  >
                    <td class="text-muted-foreground py-2 pr-3">{{ cap }}</td>
                    <td v-for="p in permissionMatrix" :key="p.key" class="px-1 py-2 text-center">
                      <div
                        v-if="hasCapability(p.key, cap)"
                        class="bg-success/15 text-success inline-flex h-5 w-5 items-center justify-center rounded-full"
                      >
                        <CheckIcon class="h-3 w-3" />
                      </div>
                      <div
                        v-else
                        class="bg-secondary/40 text-muted-foreground/40 inline-flex h-5 w-5 items-center justify-center rounded-full"
                      >
                        <XIcon class="h-3 w-3" />
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <Separator />

          <div>
            <div class="mb-2 flex items-center gap-2">
              <LinkIcon class="text-muted-foreground h-3.5 w-3.5" />
              <span class="text-xs font-medium">Share link</span>
            </div>
            <div class="flex items-center gap-2">
              <div
                class="border-border bg-secondary/40 text-muted-foreground flex flex-1 items-center gap-2 truncate rounded-lg border px-3 py-2 text-xs"
              >
                <ShieldCheckIcon class="h-3.5 w-3.5 shrink-0" />
                <span class="truncate">{{ url }}</span>
              </div>
              <Button size="sm" variant="outline" @click="copyLink" class="shrink-0 gap-1.5">
                <CheckIcon v-if="copied" class="text-success h-3.5 w-3.5" />
                <CopyIcon v-else class="h-3.5 w-3.5" />
                {{ copied ? "Copied" : "Copy" }}
              </Button>
            </div>
          </div>
        </TabsContent>
      </Tabs>

      <DialogFooter>
        <Button variant="ghost" @click="emit('update:open', false)">Close</Button>
        <Button
          @click="
            emit('update:open', false);
            toast.success('Share settings saved');
          "
          >Done</Button
        >
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
