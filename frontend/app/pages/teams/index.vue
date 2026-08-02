<script setup lang="ts">
  import { FileTextIcon, PlusIcon, Settings2Icon, UserPlusIcon, UsersIcon } from "@lucide/vue";
  import { ref, shallowRef } from "vue";
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
  import {
    Sheet,
    SheetContent,
    SheetHeader,
    SheetTitle,
    SheetDescription,
  } from "@/components/ui/sheet";
  import { Textarea } from "@/components/ui/textarea";

  definePageMeta({
    title: "Teams",
    description: "Shared workspaces for departments and project teams.",
    breadcrumbs: [{ label: "Collaboration", to: "/" }, { label: "Teams" }],
  });

  useHead({
    title: "Teams — Lumen DMS",
    meta: [
      {
        name: "description",
        content: "Collaborative team workspaces for departments and project groups.",
      },
    ],
  });

  interface Team {
    name: string;
    members: number;
    docs: number;
    color: string;
    description?: string;
  }

  const initialTeams: Team[] = [
    {
      name: "Legal",
      members: 14,
      docs: 4221,
      color: "from-rose-500/20 to-rose-500/0",
      description: "Contracts, NDAs, M&A and compliance.",
    },
    {
      name: "Finance",
      members: 22,
      docs: 9834,
      color: "from-emerald-500/20 to-emerald-500/0",
      description: "Reporting, audits, treasury and FP&A.",
    },
    {
      name: "People Ops",
      members: 9,
      docs: 1820,
      color: "from-blue-500/20 to-blue-500/0",
      description: "HR policies, onboarding, payroll.",
    },
    {
      name: "Engineering",
      members: 48,
      docs: 15470,
      color: "from-violet-500/20 to-violet-500/0",
      description: "RFCs, ADRs, architecture and runbooks.",
    },
    {
      name: "Marketing",
      members: 16,
      docs: 3110,
      color: "from-orange-500/20 to-orange-500/0",
      description: "Campaigns, brand assets, GTM plans.",
    },
    {
      name: "Executive",
      members: 6,
      docs: 412,
      color: "from-primary/20 to-primary/0",
      description: "Board materials and strategy.",
    },
  ];

  const colorOptions = [
    "from-rose-500/20 to-rose-500/0",
    "from-emerald-500/20 to-emerald-500/0",
    "from-blue-500/20 to-blue-500/0",
    "from-violet-500/20 to-violet-500/0",
    "from-orange-500/20 to-orange-500/0",
    "from-primary/20 to-primary/0",
  ];

  const teams = shallowRef<Team[]>(initialTeams);
  const newOpen = ref(false);
  const detail = shallowRef<Team | null>(null);
  const inviteOpen = ref(false);

  const formName = ref("");
  const formDesc = ref("");
  const formColor = ref(colorOptions[0]);
  const inviteEmails = ref("");
  const inviteRole = ref("Member");

  const addTeam = (t: Team) => {
    teams.value = [t, ...teams.value];
    toast.success(`Team "${t.name}" created`);
  };

  const submitForm = () => {
    if (!formName.value.trim()) {
      toast.error("Team name is required");
      return;
    }
    addTeam({
      name: formName.value,
      description: formDesc.value,
      members: 1,
      docs: 0,
      color: formColor.value,
    });
    formName.value = "";
    formDesc.value = "";
    formColor.value = colorOptions[0];
    newOpen.value = false;
  };

  const sendInvites = () => {
    const list = inviteEmails.value.split(/[,\s]+/).filter((e) => e.includes("@"));
    if (!list.length) {
      toast.error("Add at least one valid email");
      return;
    }
    toast.success(
      `Invited ${list.length} ${list.length === 1 ? "person" : "people"} as ${inviteRole.value}`,
    );
    inviteEmails.value = "";
    inviteOpen.value = false;
  };
</script>

<template>
  <Teleport to="#page-actions-target">
    <Button size="sm" class="gap-1.5" @click="newOpen = true">
      <PlusIcon class="h-4 w-4" /> New team
    </Button>
  </Teleport>

  <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
    <Card
      v-for="t in teams"
      :key="t.name"
      class="border-border shadow-elegant hover:shadow-glow relative cursor-pointer overflow-hidden transition-all hover:-translate-y-0.5"
      @click="detail = t"
    >
      <div :class="`absolute inset-0 bg-gradient-to-br ${t.color} pointer-events-none`" />
      <CardContent class="relative p-5">
        <div class="flex items-center justify-between">
          <div class="bg-card border-border grid h-10 w-10 place-items-center rounded-xl border">
            <UsersIcon class="text-muted-foreground h-4 w-4" />
          </div>
          <Badge variant="secondary" class="bg-card/80 border-0 text-[10px]">Active</Badge>
        </div>
        <h3 class="mt-4 text-base font-semibold">{{ t.name }}</h3>
        <p class="text-muted-foreground text-xs">
          {{ t.members }} members &middot; {{ t.docs.toLocaleString() }} documents
        </p>
        <div class="mt-4 flex items-center justify-between">
          <div class="flex -space-x-1.5">
            <Avatar v-for="i in ['AT', 'MB', 'SR', 'IK']" :key="i" class="ring-card h-6 w-6 ring-2">
              <AvatarFallback class="bg-secondary text-[10px]">{{ i }}</AvatarFallback>
            </Avatar>
          </div>
          <Button variant="ghost" size="sm" class="h-7 text-xs" @click.stop="detail = t"
            >Open</Button
          >
        </div>
      </CardContent>
    </Card>
  </div>

  <Dialog :open="newOpen" @update:open="(v: boolean) => (newOpen = v)">
    <DialogContent>
      <DialogHeader>
        <DialogTitle class="text-base">Create a new team</DialogTitle>
        <DialogDescription class="text-xs"
          >Teams group people, documents, and workflows in one workspace.</DialogDescription
        >
      </DialogHeader>
      <div class="space-y-3">
        <div class="space-y-1.5">
          <label class="text-xs font-medium" for="tname">Team name</label>
          <Input id="tname" v-model="formName" placeholder="e.g. Procurement" />
        </div>
        <div class="space-y-1.5">
          <label class="text-xs font-medium" for="tdesc">Description</label>
          <Textarea id="tdesc" v-model="formDesc" placeholder="What this team works on" :rows="3" />
        </div>
        <div class="space-y-1.5">
          <label class="text-xs font-medium">Accent color</label>
          <div class="flex gap-2">
            <button
              v-for="c in colorOptions"
              :key="c"
              @click="formColor = c"
              :class="
                'h-8 w-8 rounded-lg bg-gradient-to-br ' +
                c +
                ' ' +
                (formColor === c ? 'border-primary ring-primary/30 ring-2' : 'border-border')
              "
              class="border"
              :aria-label="c"
            />
          </div>
        </div>
      </div>
      <DialogFooter>
        <Button variant="ghost" @click="newOpen = false">Cancel</Button>
        <Button @click="submitForm">Create team</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <Sheet
    :open="!!detail"
    @update:open="
      (v: boolean) => {
        if (!v) detail = null;
      }
    "
  >
    <SheetContent class="w-full overflow-y-auto p-6 sm:max-w-md sm:p-8">
      <template v-if="detail">
        <SheetHeader>
          <div
            :class="`-mx-6 -mt-6 mb-4 h-24 bg-gradient-to-br sm:-mx-8 sm:-mt-8 ${detail.color} relative`"
          >
            <div
              class="bg-card border-border shadow-elegant absolute bottom-3 left-6 grid h-12 w-12 place-items-center rounded-xl border sm:left-8"
            >
              <UsersIcon class="h-5 w-5" />
            </div>
          </div>
          <SheetTitle>{{ detail.name }}</SheetTitle>
          <SheetDescription>{{ detail.description }}</SheetDescription>
        </SheetHeader>

        <div class="mt-4 grid grid-cols-3 gap-2 text-center">
          <div class="border-border bg-secondary/40 rounded-lg border py-2.5">
            <p class="text-muted-foreground text-[10px] tracking-wider uppercase">Members</p>
            <p class="mt-0.5 text-sm font-semibold">{{ detail.members }}</p>
          </div>
          <div class="border-border bg-secondary/40 rounded-lg border py-2.5">
            <p class="text-muted-foreground text-[10px] tracking-wider uppercase">Documents</p>
            <p class="mt-0.5 text-sm font-semibold">{{ detail.docs.toLocaleString() }}</p>
          </div>
          <div class="border-border bg-secondary/40 rounded-lg border py-2.5">
            <p class="text-muted-foreground text-[10px] tracking-wider uppercase">Workflows</p>
            <p class="mt-0.5 text-sm font-semibold">{{ 4 + (detail.members % 7) }}</p>
          </div>
        </div>

        <div class="mt-5 flex items-center gap-2">
          <Button size="sm" class="flex-1 gap-1.5" @click="inviteOpen = true">
            <UserPlusIcon class="h-3.5 w-3.5" /> Invite members
          </Button>
          <Button
            size="sm"
            variant="outline"
            class="gap-1.5"
            @click="toast.info('Team settings opened')"
          >
            <Settings2Icon class="h-3.5 w-3.5" /> Settings
          </Button>
        </div>

        <Separator class="my-5" />

        <div>
          <p class="mb-2 text-xs font-medium">Members</p>
          <div class="space-y-2">
            <div
              v-for="(n, i) in [
                'Aiko Tanaka',
                'Marcus Bell',
                'Sofia Reyes',
                'Idris Khan',
                'Petra Lindqvist',
              ]"
              :key="n"
              class="flex items-center justify-between"
            >
              <div class="flex items-center gap-2">
                <Avatar class="h-7 w-7">
                  <AvatarFallback class="bg-secondary text-[10px]">{{
                    n
                      .split(" ")
                      .map((p) => p[0])
                      .join("")
                  }}</AvatarFallback>
                </Avatar>
                <div>
                  <p class="text-sm">{{ n }}</p>
                  <p class="text-muted-foreground text-[11px]">
                    {{ i === 0 ? "Owner" : i < 2 ? "Admin" : "Member" }}
                  </p>
                </div>
              </div>
              <Badge variant="secondary" class="bg-secondary border-0 text-[10px]">{{
                i === 0 ? "Owner" : i < 2 ? "Admin" : "Member"
              }}</Badge>
            </div>
          </div>
        </div>

        <Separator class="my-5" />

        <div>
          <p class="mb-2 text-xs font-medium">Recent documents</p>
          <div class="space-y-2">
            <div
              v-for="d in [
                'MSA_Acme_Corp_v4.pdf',
                'Q3_Financial_Report.xlsx',
                'Board_Minutes_May.docx',
              ]"
              :key="d"
              class="border-border bg-secondary/40 flex items-center gap-2 rounded-md border px-2.5 py-2"
            >
              <FileTextIcon class="text-muted-foreground h-3.5 w-3.5" />
              <span class="truncate text-xs">{{ d }}</span>
            </div>
          </div>
        </div>
      </template>
    </SheetContent>
  </Sheet>

  <Dialog :open="inviteOpen" @update:open="(v: boolean) => (inviteOpen = v)">
    <DialogContent>
      <DialogHeader>
        <DialogTitle class="text-base">Invite to {{ detail?.name ?? "" }}</DialogTitle>
        <DialogDescription class="text-xs"
          >Add teammates by email. Separate multiple addresses with commas.</DialogDescription
        >
      </DialogHeader>
      <div class="space-y-3">
        <Textarea
          v-model="inviteEmails"
          placeholder="alex@contoso.com, sam@contoso.com"
          :rows="3"
        />
        <Select v-model="inviteRole">
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="Member">Member</SelectItem>
            <SelectItem value="Admin">Admin</SelectItem>
            <SelectItem value="Owner">Owner</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <DialogFooter>
        <Button variant="ghost" @click="inviteOpen = false">Cancel</Button>
        <Button @click="sendInvites">Send invites</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
