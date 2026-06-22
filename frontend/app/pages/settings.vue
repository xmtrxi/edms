<script setup lang="ts">
  import {
    CloudIcon,
    DatabaseIcon,
    HardDriveIcon,
    LockIcon,
    SparklesIcon,
    UserIcon,
    BellIcon,
    KeyRoundIcon,
    PlusIcon,
    CopyIcon,
    Trash2Icon,
    ShieldCheckIcon,
    SmartphoneIcon,
    CheckCircle2Icon,
    XCircleIcon,
    AlertCircleIcon,
    RefreshCwIcon,
    PlugIcon,
    PlugZapIcon,
    PencilIcon,
  } from "@lucide/vue";
  import { ref, shallowRef, computed } from "vue";
  import { toast } from "vue-sonner";

  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Card, CardContent } from "@/components/ui/card";
  import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
    DialogFooter,
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
  import { Separator } from "@/components/ui/separator";
  import { Switch } from "@/components/ui/switch";
  import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
  import { spSites } from "@/data/mock-data";
  import { cn } from "@/lib/utils";

  definePageMeta({
    title: "Settings",
    description: "Manage your workspace, security, AI, and notification preferences.",
    breadcrumbs: [{ label: "System", to: "/" }, { label: "Settings" }],
  });

  useHead({
    title: "Settings — Lumen DMS",
    meta: [
      {
        name: "description",
        content: "Manage your workspace, security, AI, and notification preferences.",
      },
    ],
  });

  type TabKey = "connectors" | "profile" | "notifications" | "security" | "tokens" | "ai";

  const tabs: { key: TabKey; icon: typeof UserIcon; label: string }[] = [
    { key: "connectors", icon: CloudIcon, label: "Connectors" },
    { key: "profile", icon: UserIcon, label: "Profile" },
    { key: "notifications", icon: BellIcon, label: "Notifications" },
    { key: "security", icon: LockIcon, label: "Security" },
    { key: "tokens", icon: KeyRoundIcon, label: "API tokens" },
    { key: "ai", icon: SparklesIcon, label: "AI" },
  ];

  const tab = ref<TabKey>("connectors");
  const tokenList = shallowRef([
    { id: "t1", name: "CI / GitHub Actions", created: "Jan 12", lastUsed: "2h ago" },
    { id: "t2", name: "Data warehouse sync", created: "Nov 2", lastUsed: "Yesterday" },
  ]);

  interface Session {
    device: string;
    where: string;
    current?: boolean;
  }

  const sessions: Session[] = [
    { device: "MacBook Pro &middot; Chrome", where: "Berlin, DE", current: true },
    { device: "iPhone 15 &middot; Safari", where: "Berlin, DE" },
    { device: "Windows &middot; Edge", where: "Munich, DE" },
  ];

  interface Connector {
    id: string;
    name: string;
    type: "s3" | "local" | "sharepoint";
    status: "connected" | "disconnected" | "error";
    config: Record<string, string>;
    lastSync: string;
    icon: typeof CloudIcon;
  }

  const connectors = shallowRef<Connector[]>([
    {
      id: "c1",
      name: "Production S3",
      type: "s3",
      status: "connected",
      config: {
        bucket: "lumen-docs-prod",
        region: "eu-central-1",
        endpoint: "s3.eu-central-1.amazonaws.com",
      },
      lastSync: "2m ago",
      icon: CloudIcon,
    },
    {
      id: "c2",
      name: "Staging S3",
      type: "s3",
      status: "disconnected",
      config: {
        bucket: "lumen-docs-staging",
        region: "eu-west-1",
        endpoint: "s3.eu-west-1.amazonaws.com",
      },
      lastSync: "3d ago",
      icon: CloudIcon,
    },
    {
      id: "c3",
      name: "Local cache",
      type: "local",
      status: "connected",
      config: { path: "/mnt/lumen/docs", mount: "NFS" },
      lastSync: "just now",
      icon: HardDriveIcon,
    },
    {
      id: "c4",
      name: "SharePoint Legal Vault",
      type: "sharepoint",
      status: "connected",
      config: { site: "contoso.sharepoint.com/sites/legal", tenant: "contoso" },
      lastSync: "5m ago",
      icon: DatabaseIcon,
    },
    {
      id: "c5",
      name: "SharePoint Finance Hub",
      type: "sharepoint",
      status: "error",
      config: { site: "contoso.sharepoint.com/sites/finance", tenant: "contoso" },
      lastSync: "12m ago",
      icon: DatabaseIcon,
    },
  ]);

  const editConnector = shallowRef<Connector | null>(null);
  const editOpen = ref(false);
  const editConfig = ref<Record<string, string>>({});
  const addLocalOpen = ref(false);
  const addSpOpen = ref(false);
  const addLocalPath = ref("");
  const addLocalMount = ref("NFS");
  const addSpName = ref("");
  const addSpUrl = ref("");

  const spConnectors = computed(() => connectors.value.filter((c) => c.type === "sharepoint"));
  const s3Connectors = computed(() => connectors.value.filter((c) => c.type === "s3"));
  const localConnectors = computed(() => connectors.value.filter((c) => c.type === "local"));

  function openEdit(c: Connector) {
    editConnector.value = c;
    editConfig.value = { ...c.config };
    editOpen.value = true;
  }

  function saveConnector() {
    if (!editConnector.value) return;
    connectors.value = connectors.value.map((c) =>
      c.id === editConnector.value!.id ? { ...c, config: { ...editConfig.value } } : c,
    );
    toast.success(`"${editConnector.value.name}" updated`);
    editOpen.value = false;
    editConnector.value = null;
  }

  function toggleConnector(c: Connector) {
    const newStatus = c.status === "connected" ? "disconnected" : "connected";
    connectors.value = connectors.value.map((x) =>
      x.id === c.id ? { ...x, status: newStatus } : x,
    );
    toast.success(`"${c.name}" ${newStatus === "connected" ? "connected" : "disconnected"}`);
  }

  function statusIcon(status: Connector["status"]) {
    if (status === "connected") return CheckCircle2Icon;
    if (status === "error") return AlertCircleIcon;
    return XCircleIcon;
  }

  function addLocalConnector() {
    if (!addLocalPath.value.trim()) {
      toast.error("Path is required");
      return;
    }
    const id = `c_local_${Date.now().toString(36)}`;
    connectors.value = [
      ...connectors.value,
      {
        id,
        name: `Local — ${addLocalPath.value}`,
        type: "local",
        status: "connected",
        config: { path: addLocalPath.value, mount: addLocalMount.value },
        lastSync: "just now",
        icon: HardDriveIcon,
      },
    ];
    toast.success("Local storage connected");
    addLocalOpen.value = false;
    addLocalPath.value = "";
    addLocalMount.value = "NFS";
  }

  function addSpConnector() {
    if (!addSpName.value.trim() || !addSpUrl.value.trim()) {
      toast.error("Name and URL are required");
      return;
    }
    const id = `c_sp_${Date.now().toString(36)}`;
    connectors.value = [
      ...connectors.value,
      {
        id,
        name: addSpName.value,
        type: "sharepoint",
        status: "connected",
        config: { site: addSpUrl.value, tenant: addSpUrl.value.split(".")[0] ?? "contoso" },
        lastSync: "just now",
        icon: DatabaseIcon,
      },
    ];
    toast.success("SharePoint site connected");
    addSpOpen.value = false;
    addSpName.value = "";
    addSpUrl.value = "";
  }

  function statusClass(status: Connector["status"]) {
    if (status === "connected") return "text-success";
    if (status === "error") return "text-destructive";
    return "text-muted-foreground";
  }
</script>

<template>
  <div class="flex flex-col gap-4">
    <Tabs v-model="tab" class="w-full">
      <TabsList class="bg-secondary/40 h-auto w-full flex-wrap gap-0.5 overflow-x-auto p-1">
        <TabsTrigger
          v-for="t in tabs"
          :key="t.key"
          :value="t.key"
          class="data-[state=active]:bg-card shrink-0 gap-1.5 text-xs data-[state=active]:shadow-sm"
        >
          <component :is="t.icon" class="h-3.5 w-3.5" /> {{ t.label }}
        </TabsTrigger>
      </TabsList>

      <div class="mt-6 max-w-2xl space-y-4">
        <TabsContent value="connectors" class="mt-0 space-y-4">
          <div class="space-y-4">
            <div>
              <h3 class="mb-2 flex items-center gap-1.5 text-sm font-semibold">
                <CloudIcon class="text-primary h-4 w-4" /> S3 Buckets
              </h3>
              <div class="grid gap-2">
                <div
                  v-for="c in s3Connectors"
                  :key="c.id"
                  class="border-border bg-card flex items-center gap-3 rounded-lg border p-3.5"
                >
                  <div
                    :class="
                      cn(
                        'grid h-9 w-9 place-items-center rounded-lg',
                        c.status === 'connected'
                          ? 'bg-success/10'
                          : c.status === 'error'
                            ? 'bg-destructive/10'
                            : 'bg-secondary',
                      )
                    "
                  >
                    <component
                      :is="statusIcon(c.status)"
                      :class="cn('h-4 w-4', statusClass(c.status))"
                    />
                  </div>
                  <div class="min-w-0 flex-1">
                    <p class="text-sm font-medium">{{ c.name }}</p>
                    <p class="text-muted-foreground truncate text-[11px]">
                      {{ c.config.bucket }} — {{ c.config.region }}
                    </p>
                    <p class="text-muted-foreground mt-0.5 text-[10px]">
                      Last synced {{ c.lastSync }}
                    </p>
                  </div>
                  <div class="flex items-center gap-1">
                    <Button size="sm" variant="ghost" class="h-7 w-7" @click="openEdit(c)"
                      ><PencilIcon class="h-3.5 w-3.5"
                    /></Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      :class="c.status === 'connected' ? 'text-destructive' : 'text-success'"
                      @click="toggleConnector(c)"
                    >
                      <PlugZapIcon v-if="c.status === 'connected'" class="h-3.5 w-3.5" />
                      <PlugIcon v-else class="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
              </div>
            </div>

            <div>
              <h3 class="mb-2 flex items-center gap-1.5 text-sm font-semibold">
                <HardDriveIcon class="text-primary h-4 w-4" /> Local Storage
              </h3>
              <div class="grid gap-2">
                <div
                  v-for="c in localConnectors"
                  :key="c.id"
                  class="border-border bg-card flex items-center gap-3 rounded-lg border p-3.5"
                >
                  <div class="bg-success/10 grid h-9 w-9 place-items-center rounded-lg">
                    <component
                      :is="statusIcon(c.status)"
                      :class="cn('h-4 w-4', statusClass(c.status))"
                    />
                  </div>
                  <div class="min-w-0 flex-1">
                    <p class="text-sm font-medium">{{ c.name }}</p>
                    <p class="text-muted-foreground truncate text-[11px]">{{ c.config.path }}</p>
                    <p class="text-muted-foreground mt-0.5 text-[10px]">
                      Mount: {{ c.config.mount }} &middot; Last synced {{ c.lastSync }}
                    </p>
                  </div>
                  <div class="flex items-center gap-1">
                    <Button size="sm" variant="ghost" class="h-7 w-7" @click="openEdit(c)"
                      ><PencilIcon class="h-3.5 w-3.5"
                    /></Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      class="text-destructive"
                      @click="toggleConnector(c)"
                      ><PlugZapIcon class="h-3.5 w-3.5"
                    /></Button>
                  </div>
                </div>
              </div>
              <Button size="sm" variant="outline" class="mt-2 gap-1.5" @click="addLocalOpen = true"
                ><PlusIcon class="h-3.5 w-3.5" /> Add local path</Button
              >
            </div>

            <div>
              <h3 class="mb-2 flex items-center gap-1.5 text-sm font-semibold">
                <DatabaseIcon class="text-primary h-4 w-4" /> SharePoint
              </h3>
              <div class="grid gap-2">
                <div
                  v-for="c in spConnectors"
                  :key="c.id"
                  class="border-border bg-card flex items-center gap-3 rounded-lg border p-3.5"
                >
                  <div
                    :class="
                      cn(
                        'grid h-9 w-9 place-items-center rounded-lg',
                        c.status === 'connected'
                          ? 'bg-success/10'
                          : c.status === 'error'
                            ? 'bg-destructive/10'
                            : 'bg-secondary',
                      )
                    "
                  >
                    <component
                      :is="statusIcon(c.status)"
                      :class="cn('h-4 w-4', statusClass(c.status))"
                    />
                  </div>
                  <div class="min-w-0 flex-1">
                    <p class="text-sm font-medium">{{ c.name }}</p>
                    <p class="text-muted-foreground truncate text-[11px]">{{ c.config.site }}</p>
                    <p class="text-muted-foreground mt-0.5 text-[10px]">
                      Last synced {{ c.lastSync }}
                    </p>
                  </div>
                  <div class="flex items-center gap-1">
                    <Button size="sm" variant="ghost" class="h-7 w-7" @click="openEdit(c)"
                      ><PencilIcon class="h-3.5 w-3.5"
                    /></Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      :class="c.status === 'connected' ? 'text-destructive' : 'text-success'"
                      @click="toggleConnector(c)"
                    >
                      <PlugZapIcon v-if="c.status === 'connected'" class="h-3.5 w-3.5" />
                      <PlugIcon v-else class="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
              </div>
              <Button size="sm" variant="outline" class="mt-2 gap-1.5" @click="addSpOpen = true"
                ><PlusIcon class="h-3.5 w-3.5" /> Connect SharePoint site</Button
              >
            </div>
          </div>
        </TabsContent>

        <TabsContent value="profile" class="mt-0 space-y-4">
          <Card class="border-border shadow-elegant">
            <CardContent class="p-6">
              <h3 class="text-base font-semibold">Profile</h3>
              <p class="text-muted-foreground mt-0.5 text-xs">
                How others see you in shared workspaces.
              </p>
              <Separator class="my-4" />
              <div class="mb-4 flex items-center gap-4">
                <Avatar class="h-14 w-14"
                  ><AvatarFallback class="bg-secondary">AT</AvatarFallback></Avatar
                >
                <div class="flex gap-2">
                  <Button size="sm" variant="outline" @click="toast.success('Avatar updated')"
                    >Change photo</Button
                  >
                  <Button size="sm" variant="ghost" @click="toast.success('Avatar removed')"
                    >Remove</Button
                  >
                </div>
              </div>
              <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div class="space-y-1.5">
                  <Label for="name">Display name</Label
                  ><Input id="name" :model-value="'Aiko Tanaka'" />
                </div>
                <div class="space-y-1.5">
                  <Label for="email">Email</Label
                  ><Input id="email" :model-value="'aiko@contoso.com'" />
                </div>
                <div class="space-y-1.5">
                  <Label for="role">Role</Label><Input id="role" :model-value="'Senior Counsel'" />
                </div>
                <div class="space-y-1.5">
                  <Label for="dept">Department</Label><Input id="dept" :model-value="'Legal'" />
                </div>
                <div class="space-y-1.5">
                  <Label>Timezone</Label>
                  <Select :model-value="'utc'">
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="utc">UTC</SelectItem>
                      <SelectItem value="cet">Central European Time</SelectItem>
                      <SelectItem value="est">Eastern Time</SelectItem>
                      <SelectItem value="pst">Pacific Time</SelectItem>
                      <SelectItem value="jst">Japan Standard Time</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div class="space-y-1.5">
                  <Label>Language</Label>
                  <Select :model-value="'en'">
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="en">English</SelectItem>
                      <SelectItem value="ja">日本語</SelectItem>
                      <SelectItem value="de">Deutsch</SelectItem>
                      <SelectItem value="fr">Français</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div class="mt-4 flex justify-end">
                <Button size="sm" @click="toast.success('Profile saved')">Save changes</Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="notifications" class="mt-0 space-y-4">
          <Card class="border-border shadow-elegant">
            <CardContent class="p-6">
              <h3 class="text-base font-semibold">Email</h3>
              <Separator class="my-4" />
              <div class="space-y-1">
                <div class="flex items-center justify-between gap-4 py-2">
                  <div>
                    <p class="text-sm font-medium">Workflow assignments</p>
                    <p class="text-muted-foreground text-xs">
                      Email when something needs your approval.
                    </p>
                  </div>
                  <Switch :default-checked="true" />
                </div>
                <div class="flex items-center justify-between gap-4 py-2">
                  <div>
                    <p class="text-sm font-medium">Sync conflicts</p>
                    <p class="text-muted-foreground text-xs">
                      Alert me when SharePoint sync conflicts arise.
                    </p>
                  </div>
                  <Switch :default-checked="true" />
                </div>
                <div class="flex items-center justify-between gap-4 py-2">
                  <div>
                    <p class="text-sm font-medium">Mentions and comments</p>
                    <p class="text-muted-foreground text-xs">Email when someone @mentions you.</p>
                  </div>
                  <Switch :default-checked="true" />
                </div>
                <div class="flex items-center justify-between gap-4 py-2">
                  <div>
                    <p class="text-sm font-medium">Weekly digest</p>
                    <p class="text-muted-foreground text-xs">
                      A Monday summary of activity in your libraries.
                    </p>
                  </div>
                  <Switch />
                </div>
              </div>
            </CardContent>
          </Card>
          <Card class="border-border shadow-elegant">
            <CardContent class="p-6">
              <h3 class="text-base font-semibold">In-app</h3>
              <Separator class="my-4" />
              <div class="space-y-1">
                <div class="flex items-center justify-between gap-4 py-2">
                  <div>
                    <p class="text-sm font-medium">Desktop push</p>
                    <p class="text-muted-foreground text-xs">
                      Browser notifications for urgent items.
                    </p>
                  </div>
                  <Switch :default-checked="true" />
                </div>
                <div class="flex items-center justify-between gap-4 py-2">
                  <div>
                    <p class="text-sm font-medium">Sound</p>
                    <p class="text-muted-foreground text-xs">Play a chime for new approvals.</p>
                  </div>
                  <Switch />
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="security" class="mt-0 space-y-4">
          <Card class="border-border shadow-elegant">
            <CardContent class="p-6">
              <h3 class="text-base font-semibold">Password</h3>
              <p class="text-muted-foreground mt-0.5 text-xs">Use a strong, unique password.</p>
              <Separator class="my-4" />
              <div class="space-y-3">
                <div class="space-y-1.5">
                  <Label>Current password</Label><Input type="password" placeholder="••••••••" />
                </div>
                <div class="grid grid-cols-2 gap-3">
                  <div class="space-y-1.5">
                    <Label>New password</Label><Input type="password" />
                  </div>
                  <div class="space-y-1.5"><Label>Confirm</Label><Input type="password" /></div>
                </div>
                <div class="flex justify-end">
                  <Button size="sm" @click="toast.success('Password updated')"
                    >Update password</Button
                  >
                </div>
              </div>
            </CardContent>
          </Card>
          <Card class="border-border shadow-elegant">
            <CardContent class="p-6">
              <h3 class="text-base font-semibold">Two-factor authentication</h3>
              <Separator class="my-4" />
              <div
                class="border-success/30 bg-success/5 mb-3 flex items-center justify-between rounded-lg border p-3"
              >
                <div class="flex items-center gap-2 text-sm">
                  <ShieldCheckIcon class="text-success h-4 w-4" /> Authenticator app enabled
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  @click="toast.success('Recovery codes regenerated')"
                  >Regenerate codes</Button
                >
              </div>
              <div class="flex items-center justify-between gap-4 py-1">
                <div>
                  <p class="text-sm font-medium">Require 2FA for sensitive actions</p>
                  <p class="text-muted-foreground text-xs">
                    Re-verify before share, delete, or export.
                  </p>
                </div>
                <Switch :default-checked="true" />
              </div>
            </CardContent>
          </Card>
          <Card class="border-border shadow-elegant">
            <CardContent class="p-6">
              <h3 class="text-base font-semibold">Sessions</h3>
              <Separator class="my-4" />
              <div class="space-y-2">
                <div
                  v-for="s in sessions"
                  :key="s.device"
                  class="border-border bg-secondary/40 flex items-center justify-between rounded-md border px-3 py-2"
                >
                  <div class="flex items-center gap-2">
                    <SmartphoneIcon class="text-muted-foreground h-4 w-4" />
                    <div>
                      <p class="text-sm" v-html="s.device" />
                      <p class="text-muted-foreground text-[11px]">{{ s.where }}</p>
                    </div>
                  </div>
                  <div v-if="s.current">
                    <Badge
                      variant="secondary"
                      class="bg-success/10 text-success border-0 text-[10px]"
                      >Current</Badge
                    >
                  </div>
                  <Button
                    v-else
                    size="sm"
                    variant="ghost"
                    @click="toast.success('Signed out ' + s.device)"
                    >Sign out</Button
                  >
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="tokens" class="mt-0 space-y-4">
          <Card class="border-border shadow-elegant">
            <CardContent class="p-6">
              <h3 class="text-base font-semibold">API tokens</h3>
              <p class="text-muted-foreground mt-0.5 text-xs">
                Personal access tokens for programmatic access.
              </p>
              <Separator class="my-4" />
              <div class="space-y-2">
                <div
                  v-for="t in tokenList"
                  :key="t.id"
                  class="border-border bg-secondary/40 flex items-center justify-between rounded-md border px-3 py-2"
                >
                  <div>
                    <p class="text-sm font-medium">{{ t.name }}</p>
                    <p class="text-muted-foreground text-[11px]">
                      Created {{ t.created }} &middot; Last used {{ t.lastUsed }}
                    </p>
                  </div>
                  <div class="flex items-center gap-1">
                    <Button size="sm" variant="ghost" @click="toast.success('Token copied')"
                      ><CopyIcon class="h-3.5 w-3.5"
                    /></Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      @click="
                        tokenList = tokenList.filter((x) => x.id !== t.id);
                        toast.success('Token revoked');
                      "
                      ><Trash2Icon class="text-destructive h-3.5 w-3.5"
                    /></Button>
                  </div>
                </div>
              </div>
              <div class="mt-4 flex justify-end">
                <Button
                  size="sm"
                  class="gap-1.5"
                  @click="
                    tokenList = [
                      ...tokenList,
                      {
                        id: `t${Date.now()}`,
                        name: 'New token',
                        created: 'Today',
                        lastUsed: 'never',
                      },
                    ];
                    toast.success('Token created — copy it before closing');
                  "
                >
                  <PlusIcon class="h-3.5 w-3.5" /> Generate token
                </Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="ai" class="mt-0 space-y-4">
          <Card class="border-border shadow-elegant">
            <CardContent class="p-6">
              <h3 class="text-base font-semibold">AI preferences</h3>
              <p class="text-muted-foreground mt-0.5 text-xs">
                Control how AI processes your documents.
              </p>
              <Separator class="my-4" />
              <div class="space-y-1">
                <div
                  v-for="opt in [
                    {
                      title: 'Auto-summarize new uploads',
                      desc: 'Generate AI summaries for documents added to your libraries.',
                    },
                    {
                      title: 'Smart classification',
                      desc: 'Suggest tags and confidentiality levels using AI.',
                    },
                    {
                      title: 'Risk detection on contracts',
                      desc: 'Flag liability caps, auto-renewals, and unusual clauses.',
                    },
                    {
                      title: 'Semantic search default',
                      desc: 'Use AI ranking instead of keyword by default.',
                    },
                    {
                      title: 'Allow AI training on internal docs',
                      desc: 'Improve answers using your private corpus only.',
                    },
                  ]"
                  :key="opt.title"
                  class="flex items-center justify-between gap-4 py-2"
                >
                  <div>
                    <p class="text-sm font-medium">{{ opt.title }}</p>
                    <p class="text-muted-foreground text-xs">{{ opt.desc }}</p>
                  </div>
                  <Switch
                    :default-checked="
                      opt.title !== 'Semantic search default' &&
                      opt.title !== 'Allow AI training on internal docs'
                    "
                  />
                </div>
              </div>
              <Separator class="my-4" />
              <div class="max-w-xs space-y-1.5">
                <Label>Default model</Label>
                <Select :model-value="'enterprise'">
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="enterprise">Lumen Enterprise (private)</SelectItem>
                    <SelectItem value="fast">Lumen Fast</SelectItem>
                    <SelectItem value="reasoning">Lumen Reasoning</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </div>
    </Tabs>
  </div>

  <Dialog :open="addLocalOpen" @update:open="(v: boolean) => (addLocalOpen = v)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Add local storage path</DialogTitle>
        <DialogDescription>Mount a local filesystem path as a document source.</DialogDescription>
      </DialogHeader>
      <div class="space-y-3">
        <div class="space-y-1.5">
          <Label>Filesystem path</Label>
          <Input v-model="addLocalPath" placeholder="/mnt/lumen/documents" />
        </div>
        <div class="space-y-1.5">
          <Label>Mount type</Label>
          <Select v-model="addLocalMount">
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="NFS">NFS</SelectItem>
              <SelectItem value="SMB">SMB / CIFS</SelectItem>
              <SelectItem value="Local">Local disk</SelectItem>
              <SelectItem value="SSHFS">SSHFS</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      <DialogFooter>
        <Button variant="ghost" @click="addLocalOpen = false">Cancel</Button>
        <Button @click="addLocalConnector">Connect</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <Dialog :open="addSpOpen" @update:open="(v: boolean) => (addSpOpen = v)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Connect SharePoint site</DialogTitle>
        <DialogDescription>Link a SharePoint site to sync documents into Lumen.</DialogDescription>
      </DialogHeader>
      <div class="space-y-3">
        <div class="space-y-1.5">
          <Label>Site name</Label>
          <Input v-model="addSpName" placeholder="e.g. Legal Vault" />
        </div>
        <div class="space-y-1.5">
          <Label>Site URL</Label>
          <Input v-model="addSpUrl" placeholder="contoso.sharepoint.com/sites/legal" />
        </div>
      </div>
      <DialogFooter>
        <Button variant="ghost" @click="addSpOpen = false">Cancel</Button>
        <Button @click="addSpConnector">Connect</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <Dialog
    :open="editOpen"
    @update:open="
      (v: boolean) => {
        if (!v) {
          editOpen = false;
          editConnector = null;
        }
      }
    "
  >
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Edit {{ editConnector?.name }}</DialogTitle>
        <DialogDescription>Update connector configuration.</DialogDescription>
      </DialogHeader>
      <div class="space-y-3">
        <div v-for="(val, key) in editConfig" :key="key" class="space-y-1.5">
          <Label class="text-xs capitalize">{{ key.replace(/([A-Z])/g, " $1") }}</Label>
          <Input :model-value="val" @update:model-value="(v: string) => (editConfig[key] = v)" />
        </div>
      </div>
      <DialogFooter>
        <Button variant="ghost" @click="editOpen = false">Cancel</Button>
        <Button @click="saveConnector">Save</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
