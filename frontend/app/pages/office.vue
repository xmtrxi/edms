<script setup lang="ts">
  import {
    InboxIcon,
    SendIcon,
    CheckCircle2Icon,
    XCircleIcon,
    ArrowRightIcon,
    ClockIcon,
    FileTextIcon,
    PenLineIcon,
    AlertCircleIcon,
    Building2Icon,
    StampIcon,
    AlertTriangleIcon,
    LogInIcon,
    ArchiveIcon,
  } from "@lucide/vue";
  import { computed, ref, shallowRef, watch } from "vue";
  import { toast } from "vue-sonner";

  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Card, CardContent } from "@/components/ui/card";
  import {
    Sheet,
    SheetContent,
    SheetHeader,
    SheetTitle,
    SheetDescription,
  } from "@/components/ui/sheet";
  import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
  import { Textarea } from "@/components/ui/textarea";
  import { useKeyboardShortcuts } from "@/composables/useKeyboardShortcuts";
  import { documents } from "@/data/mock-data";
  import {
    advanceStep,
    currentUser,
    useOfficeState,
    type RoutingSlip,
    type RouteStep,
    addStamp,
  } from "@/lib/office-store";

  definePageMeta({
    title: "Office Desk",
    description:
      "Route documents through your office — sign, stamp, approve, and forward to the next person.",
    breadcrumbs: [{ label: "Collaboration", to: "/" }, { label: "Office" }],
  });

  useHead({
    title: "Office Desk — Lumen DMS",
    meta: [
      {
        name: "description",
        content: "Route documents from secretary to directors, sign, stamp, and track every step.",
      },
    ],
  });

  const office = useOfficeState();
  const me = currentUser();
  const tab = ref("inbox");
  const openSlip = shallowRef<RoutingSlip | null>(null);
  const signFor = shallowRef<{
    slipId: string;
    stepId: string;
    docId: string;
    docName: string;
  } | null>(null);
  const note = ref("");

  const inbox = computed(() =>
    office.value.slips.filter((s) =>
      s.steps.some((st) => st.status === "active" && st.recipient === me.name),
    ),
  );
  const sent = computed(() => office.value.slips.filter((s) => s.origin.includes(me.name)));
  const registry = computed(() =>
    office.value.slips.filter(
      (s) => s.steps[0]?.status === "pending" || s.steps.every((st) => st.status === "pending"),
    ),
  );
  const completed = computed(() =>
    office.value.slips.filter((s) => s.status === "completed" || s.status === "returned"),
  );
  const all = computed(() => office.value.slips);

  const overdue = computed(() =>
    office.value.slips.filter((s) => {
      if (!s.dueDate || s.status === "completed" || s.status === "returned") return false;
      const due = new Date(s.dueDate);
      return due < new Date() && s.steps.some((st) => st.status === "active");
    }),
  );

  const counts = computed(() => ({
    inbox: inbox.value.length,
    sent: sent.value.length,
    completed: completed.value.length,
    registry: registry.value.length,
    overdue: overdue.value.length,
  }));

  const lists: Record<string, ReturnType<typeof computed>> = {
    inbox,
    sent,
    completed,
    all,
    registry,
  };

  function priorityClass(p: RoutingSlip["priority"]) {
    return p === "urgent"
      ? "bg-destructive/15 text-destructive"
      : p === "high"
        ? "bg-warning/15 text-warning"
        : p === "low"
          ? "bg-secondary text-muted-foreground"
          : "bg-info/15 text-info";
  }

  function myStepFor(slip: RoutingSlip) {
    return slip.steps.find((s) => s.status === "active" && s.recipient === me.name);
  }

  function stepCompleted(slip: RoutingSlip) {
    return slip.steps.filter(
      (s) => s.status === "approved" || s.status === "signed" || s.status === "forwarded",
    ).length;
  }

  function refNum(id: string) {
    return `RT-${id.slice(0, 6).toUpperCase()}`;
  }

  function currentDept(slip: RoutingSlip) {
    const active = slip.steps.find((s) => s.status === "active");
    return active ? active.role : "Filed";
  }

  function withUndo(action: () => void, description: string) {
    action();
    toast(description, {
      action: { label: "Undo", onClick: () => toast.info("Rollback not available in demo") },
    });
  }

  function markReceived(slip: RoutingSlip) {
    addStamp(slip.docId, "received", "Logged into registry");
    advanceStep(slip.id, slip.steps[0].id, "forwarded");
    toast.success(`"${slip.docName}" received and logged`);
  }

  const prevSigCount = ref(office.value.signatures.length);
  watch(
    () => office.value.signatures.length,
    (newCount) => {
      if (signFor.value && newCount > prevSigCount.value) {
        advanceStep(signFor.value.slipId, signFor.value.stepId, "signed");
        toast("Signed and forwarded to next recipient", {
          action: { label: "Undo", onClick: () => toast.info("Signature rollback (demo)") },
        });
        signFor.value = null;
      }
      prevSigCount.value = newCount;
    },
  );

  useKeyboardShortcuts([
    {
      key: "a",
      handler: () => {
        const slip = openSlip.value ?? inbox.value[0];
        if (!slip) return;
        const step = myStepFor(slip);
        if (step && step.action === "approve") {
          withUndo(
            () => advanceStep(slip.id, step.id, "approved", note.value || undefined),
            "Approved and forwarded",
          );
          openSlip.value = null;
        }
      },
      enabled: computed(() => inbox.value.length > 0),
    },
    {
      key: "r",
      handler: () => {
        const slip = openSlip.value ?? inbox.value[0];
        if (!slip) return;
        const step = myStepFor(slip);
        if (step && step.action === "approve") {
          withUndo(
            () => advanceStep(slip.id, step.id, "rejected", note.value || undefined),
            "Returned to sender",
          );
          openSlip.value = null;
        }
      },
      enabled: computed(() => inbox.value.length > 0),
    },
    {
      key: "s",
      handler: () => {
        const slip = openSlip.value ?? inbox.value[0];
        if (!slip) return;
        const step = myStepFor(slip);
        if (step && step.action === "sign") {
          signFor.value = {
            slipId: slip.id,
            stepId: step.id,
            docId: slip.docId,
            docName: slip.docName,
          };
        }
      },
      enabled: computed(() => inbox.value.length > 0),
    },
  ]);

  const isEmpty = (key: string) => lists[key]?.value.length === 0;
</script>

<template>
  <Tabs v-model="tab">
    <TabsList class="bg-secondary/60">
      <TabsTrigger value="inbox" class="gap-1.5">
        <InboxIcon class="h-3.5 w-3.5" />
        <span class="hidden md:inline">Inbox</span>
        <Badge
          variant="secondary"
          class="bg-primary/10 text-primary h-4 border-0 px-1 text-[10px]"
          >{{ counts.inbox }}</Badge
        >
      </TabsTrigger>
      <TabsTrigger value="registry" class="gap-1.5">
        <LogInIcon class="h-3.5 w-3.5" />
        <span class="hidden md:inline">Registry</span>
        <Badge
          v-if="counts.registry"
          variant="secondary"
          class="bg-warning/10 text-warning h-4 border-0 px-1 text-[10px]"
          >{{ counts.registry }}</Badge
        >
      </TabsTrigger>
      <TabsTrigger value="sent" class="gap-1.5">
        <SendIcon class="h-3.5 w-3.5" />
        <span class="hidden md:inline">Sent</span>
      </TabsTrigger>
      <TabsTrigger value="completed" class="gap-1.5">
        <ArchiveIcon class="h-3.5 w-3.5" />
        <span class="hidden md:inline">Filed</span>
      </TabsTrigger>
      <TabsTrigger value="all" class="gap-1.5">
        <Building2Icon class="h-3.5 w-3.5" />
        <span class="hidden md:inline">All</span>
      </TabsTrigger>
    </TabsList>

    <!-- Registry tab -->
    <TabsContent value="registry" class="mt-4">
      <div
        class="border-border/60 bg-secondary/30 mb-3 flex items-center gap-2 rounded-lg border px-3 py-2"
      >
        <StampIcon class="text-muted-foreground h-4 w-4" />
        <span class="text-muted-foreground text-xs"
          >Mailroom — incoming documents are stamped "RECEIVED" and logged into the tracking
          system</span
        >
      </div>
      <div
        v-if="isEmpty('registry')"
        class="border-border rounded-xl border border-dashed p-12 text-center"
      >
        <div
          class="bg-primary/10 text-primary mx-auto mb-3 grid h-12 w-12 place-items-center rounded-full"
        >
          <LogInIcon />
        </div>
        <p class="text-sm font-medium">No incoming documents</p>
        <p class="text-muted-foreground mt-1 text-xs">New routes will appear here for logging.</p>
      </div>
      <div v-else class="grid gap-3 md:grid-cols-2">
        <Card v-for="slip in registry.value" :key="slip.id" class="border-border shadow-sm">
          <CardContent class="p-4">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <span class="text-muted-foreground/60 font-mono text-[10px]">{{
                  refNum(slip.id)
                }}</span>
                <button
                  @click="openSlip = slip"
                  class="hover:text-primary mt-0.5 line-clamp-1 text-left text-sm font-semibold"
                >
                  {{ slip.subject }}
                </button>
                <p
                  class="text-muted-foreground mt-0.5 flex items-center gap-1 truncate text-[11px]"
                >
                  <FileTextIcon class="h-3 w-3" /> {{ slip.docName }}
                </p>
              </div>
              <Badge variant="secondary" class="border-0 text-[10px] capitalize">{{
                slip.priority
              }}</Badge>
            </div>
            <div class="border-border mt-3 flex items-center justify-between border-t pt-3">
              <div class="text-muted-foreground flex items-center gap-1.5 text-[11px]">
                <SendIcon class="h-3 w-3" /> {{ slip.origin }}
              </div>
              <Button size="sm" class="h-7 gap-1 text-xs" @click="markReceived(slip)">
                <StampIcon class="h-3 w-3" /> Log &amp; forward
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </TabsContent>

    <!-- Inbox / Sent / Completed / All tabs -->
    <TabsContent
      v-for="key in ['inbox', 'sent', 'completed', 'all']"
      :key="key"
      :value="key"
      class="mt-4"
    >
      <div
        v-if="counts.overdue > 0 && key === 'inbox'"
        class="border-destructive/20 bg-destructive/5 mb-3 flex items-center gap-2 rounded-lg border px-3 py-2"
      >
        <AlertTriangleIcon class="text-destructive h-4 w-4 shrink-0" />
        <span class="text-destructive text-xs font-medium"
          >{{ counts.overdue }} file{{ counts.overdue > 1 ? "s" : "" }} past due date</span
        >
      </div>
      <div
        v-if="isEmpty(key)"
        class="border-border rounded-xl border border-dashed p-12 text-center"
      >
        <div
          class="bg-primary/10 text-primary mx-auto mb-3 grid h-12 w-12 place-items-center rounded-full"
        >
          <component
            :is="
              { inbox: InboxIcon, sent: SendIcon, completed: ArchiveIcon, all: Building2Icon }[key]
            "
          />
        </div>
        <p class="text-sm font-medium">
          {{
            {
              inbox: "Inbox zero",
              sent: "Nothing sent yet",
              completed: "No filed documents",
              all: "No routing activity",
            }[key]
          }}
        </p>
        <p class="text-muted-foreground mt-1 text-xs">
          {{
            {
              inbox: "No documents waiting for your action.",
              sent: "Route a document from the Documents page to get started.",
              completed: "Finished routing slips will appear here.",
              all: "Open a document and click Route to send it through the office.",
            }[key]
          }}
        </p>
      </div>
      <div v-else class="grid gap-3 md:grid-cols-2">
        <Card
          v-for="slip in lists[key].value"
          :key="slip.id"
          :class="`group transition-all hover:shadow-md ${key === 'inbox' ? (slip.dueDate && new Date(slip.dueDate) < new Date() ? 'border-destructive/30' : 'border-primary/30') : 'border-border'}`"
        >
          <CardContent class="p-4">
            <div class="mb-2 flex items-start justify-between gap-2">
              <div class="min-w-0">
                <div
                  class="text-muted-foreground/60 flex items-center gap-1.5 font-mono text-[10px]"
                >
                  {{ refNum(slip.id) }}
                </div>
                <div class="mt-0.5 flex items-center gap-1.5">
                  <Badge
                    variant="secondary"
                    :class="`border-0 text-[10px] capitalize ${priorityClass(slip.priority)}`"
                    >{{ slip.priority }}</Badge
                  >
                  <Badge variant="secondary" class="border-0 text-[10px] capitalize">{{
                    slip.status.replace("_", " ")
                  }}</Badge>
                  <Badge
                    variant="secondary"
                    class="bg-secondary/50 text-muted-foreground border-0 text-[10px]"
                    >{{ currentDept(slip) }}</Badge
                  >
                </div>
                <button
                  @click="openSlip = slip"
                  class="hover:text-primary mt-0.5 line-clamp-1 text-left text-sm font-semibold"
                >
                  {{ slip.subject }}
                </button>
                <NuxtLink
                  to="/documents"
                  class="text-muted-foreground hover:text-primary mt-0.5 flex items-center gap-1 truncate text-[11px]"
                >
                  <FileTextIcon class="h-3 w-3" /> {{ slip.docName }}
                </NuxtLink>
              </div>
              <div v-if="slip.dueDate" class="shrink-0 text-right">
                <div class="text-muted-foreground text-[10px] tracking-wider uppercase">Due</div>
                <div
                  class="flex items-center gap-1 text-xs font-medium"
                  :class="
                    new Date(slip.dueDate) < new Date() && slip.status === 'in_progress'
                      ? 'text-destructive'
                      : ''
                  "
                >
                  <ClockIcon class="h-3 w-3" /> {{ slip.dueDate }}
                </div>
              </div>
            </div>

            <div class="-mx-1 my-3 flex items-center gap-1 overflow-x-auto pb-1">
              <template v-for="(s, i) in slip.steps" :key="s.id">
                <div class="flex shrink-0 items-center gap-1">
                  <div class="flex flex-col items-center gap-0.5">
                    <Avatar
                      :class="`h-7 w-7 ring-2 ${s.status === 'approved' || s.status === 'signed' || s.status === 'forwarded' ? 'ring-success/40' : s.status === 'active' ? 'ring-primary' : s.status === 'rejected' ? 'ring-destructive/40' : 'ring-transparent'}`"
                    >
                      <AvatarFallback class="bg-secondary text-[10px]">{{
                        s.initials
                      }}</AvatarFallback>
                    </Avatar>
                    <span class="text-muted-foreground text-[9px] capitalize">{{ s.action }}</span>
                  </div>
                </div>
                <ArrowRightIcon
                  v-if="i < slip.steps.length - 1"
                  class="text-muted-foreground/40 h-3 w-3 shrink-0"
                />
              </template>
            </div>

            <div class="border-border flex items-center justify-between gap-2 border-t pt-2">
              <div class="text-muted-foreground text-[11px]">
                {{ stepCompleted(slip) }} / {{ slip.steps.length }} steps
              </div>
              <div v-if="myStepFor(slip)" class="flex gap-1.5">
                <template v-if="myStepFor(slip)!.action === 'sign'">
                  <Button
                    size="sm"
                    class="h-7 gap-1.5"
                    @click="
                      signFor = {
                        slipId: slip.id,
                        stepId: myStepFor(slip)!.id,
                        docId: slip.docId,
                        docName: slip.docName,
                      }
                    "
                  >
                    <PenLineIcon class="h-3 w-3" /> Sign
                  </Button>
                </template>
                <template v-else-if="myStepFor(slip)!.action === 'approve'">
                  <Button
                    variant="outline"
                    size="sm"
                    class="h-7 gap-1.5"
                    @click="
                      withUndo(
                        () => advanceStep(slip.id, myStepFor(slip)!.id, 'rejected'),
                        'Returned to sender',
                      )
                    "
                  >
                    <XCircleIcon class="h-3 w-3" /> Return
                  </Button>
                  <Button
                    size="sm"
                    class="h-7 gap-1.5"
                    @click="
                      withUndo(
                        () => advanceStep(slip.id, myStepFor(slip)!.id, 'approved'),
                        'Approved and forwarded',
                      )
                    "
                  >
                    <CheckCircle2Icon class="h-3 w-3" /> Approve
                  </Button>
                </template>
                <template v-else>
                  <Button
                    size="sm"
                    class="h-7 gap-1.5"
                    @click="
                      withUndo(
                        () => advanceStep(slip.id, myStepFor(slip)!.id, 'forwarded'),
                        'Forwarded to next',
                      )
                    "
                  >
                    <ArrowRightIcon class="h-3 w-3" /> Forward
                  </Button>
                </template>
              </div>
              <Button v-else size="sm" variant="ghost" class="h-7" @click="openSlip = slip"
                >View</Button
              >
            </div>
          </CardContent>
        </Card>
      </div>
    </TabsContent>
  </Tabs>

  <!-- Detail sheet -->
  <Sheet
    :open="!!openSlip"
    @update:open="
      (v: boolean) => {
        if (!v) openSlip = null;
      }
    "
  >
    <SheetContent class="w-full overflow-y-auto p-6 sm:max-w-lg sm:p-8">
      <template v-if="openSlip">
        <SheetHeader>
          <div
            class="text-muted-foreground/60 mb-1 flex items-center gap-1.5 font-mono text-[10px]"
          >
            {{ refNum(openSlip.id) }}
          </div>
          <SheetTitle>{{ openSlip.subject }}</SheetTitle>
          <SheetDescription class="flex items-center gap-2">
            <FileTextIcon class="h-3.5 w-3.5" /> {{ openSlip.docName }}
          </SheetDescription>
        </SheetHeader>

        <div class="mt-4 flex flex-wrap gap-1.5">
          <Badge
            variant="secondary"
            :class="`border-0 text-[10px] capitalize ${priorityClass(openSlip.priority)}`"
            >{{ openSlip.priority }}</Badge
          >
          <Badge variant="secondary" class="border-0 text-[10px] capitalize">{{
            openSlip.status.replace("_", " ")
          }}</Badge>
          <Badge
            variant="secondary"
            class="bg-secondary/50 text-muted-foreground border-0 text-[10px]"
            >{{ currentDept(openSlip) }}</Badge
          >
          <Badge
            v-if="openSlip.dueDate"
            variant="secondary"
            class="border-0 text-[10px]"
            :class="
              new Date(openSlip.dueDate) < new Date() && openSlip.status === 'in_progress'
                ? 'bg-destructive/10 text-destructive'
                : ''
            "
            >Due {{ openSlip.dueDate }}</Badge
          >
        </div>

        <p class="text-muted-foreground mt-3 text-xs">
          From <span class="text-foreground font-medium">{{ openSlip.origin }}</span>
          <span class="text-muted-foreground/60 ml-2 text-[10px]">{{
            new Date(openSlip.createdAt).toLocaleDateString()
          }}</span>
        </p>
        <p
          v-if="openSlip.notes"
          class="bg-secondary/40 border-border mt-2 rounded-lg border p-3 text-sm"
        >
          {{ openSlip.notes }}
        </p>

        <div class="mt-5">
          <h4
            class="text-muted-foreground mb-3 flex items-center gap-1.5 text-xs font-medium tracking-wider uppercase"
          >
            <Building2Icon class="h-3.5 w-3.5" /> File movement
          </h4>
          <ol class="space-y-3">
            <li v-for="(step, idx) in openSlip.steps" :key="step.id" class="flex gap-3">
              <div class="flex flex-col items-center">
                <div
                  :class="`grid h-8 w-8 shrink-0 place-items-center rounded-full text-xs font-semibold ring-2 ${step.status === 'approved' || step.status === 'signed' || step.status === 'forwarded' ? 'bg-success/15 text-success ring-success/30' : step.status === 'active' ? 'bg-primary/15 text-primary ring-primary/40' : step.status === 'rejected' ? 'bg-destructive/15 text-destructive ring-destructive/30' : 'bg-secondary text-muted-foreground ring-transparent'}`"
                >
                  <CheckCircle2Icon
                    v-if="
                      step.status === 'approved' ||
                      step.status === 'signed' ||
                      step.status === 'forwarded'
                    "
                    class="h-4 w-4"
                  />
                  <XCircleIcon v-else-if="step.status === 'rejected'" class="h-4 w-4" />
                  <span v-else>{{ idx + 1 }}</span>
                </div>
                <div class="bg-border mt-1 w-px flex-1" />
              </div>
              <div class="flex-1 pb-2">
                <div class="flex items-center gap-2">
                  <Avatar class="h-6 w-6"
                    ><AvatarFallback class="bg-secondary text-[10px]">{{
                      step.initials
                    }}</AvatarFallback></Avatar
                  >
                  <p class="text-sm font-medium">{{ step.recipient }}</p>
                  <Badge variant="secondary" class="ml-auto border-0 text-[10px] capitalize">{{
                    step.action
                  }}</Badge>
                </div>
                <p class="text-muted-foreground mt-0.5 text-[11px]">{{ step.role }}</p>
                <div class="mt-1 flex items-center gap-2 text-[11px]">
                  <span
                    class="font-medium capitalize"
                    :class="
                      step.status === 'approved' || step.status === 'signed'
                        ? 'text-success'
                        : step.status === 'rejected'
                          ? 'text-destructive'
                          : 'text-muted-foreground'
                    "
                    >{{ step.status }}</span
                  >
                  <span v-if="step.actedAt" class="text-muted-foreground"
                    >&middot; {{ new Date(step.actedAt).toLocaleString() }}</span
                  >
                </div>
                <p
                  v-if="step.note"
                  class="bg-secondary/40 border-border mt-1 rounded border p-2 text-xs"
                >
                  &ldquo;{{ step.note }}&rdquo;
                </p>
              </div>
            </li>
          </ol>
        </div>

        <div v-if="myStepFor(openSlip)" class="border-border mt-5 border-t pt-4">
          <h4 class="mb-2 flex items-center gap-1.5 text-xs font-medium">
            <AlertCircleIcon class="text-primary h-3.5 w-3.5" /> Action required —
            {{ myStepFor(openSlip)!.action }}
          </h4>
          <Textarea
            v-model="note"
            rows="2"
            placeholder="Add a note for the next recipient (optional)…"
            class="mb-2"
          />
          <div class="flex flex-wrap gap-2">
            <template v-if="myStepFor(openSlip)!.action === 'sign'">
              <Button
                class="gap-1.5"
                @click="
                  signFor = {
                    slipId: openSlip!.id,
                    stepId: myStepFor(openSlip)!.id,
                    docId: openSlip!.docId,
                    docName: openSlip!.docName,
                  }
                "
              >
                <PenLineIcon class="h-3.5 w-3.5" /> Sign now
              </Button>
            </template>
            <template v-else-if="myStepFor(openSlip)!.action === 'approve'">
              <Button
                class="gap-1.5"
                @click="
                  withUndo(() => {
                    advanceStep(
                      openSlip!.id,
                      myStepFor(openSlip)!.id,
                      'approved',
                      note || undefined,
                    );
                    openSlip = null;
                  }, 'Approved and forwarded')
                "
              >
                <CheckCircle2Icon class="h-3.5 w-3.5" /> Approve &amp; forward
              </Button>
              <Button
                variant="outline"
                class="gap-1.5"
                @click="
                  withUndo(() => {
                    advanceStep(
                      openSlip!.id,
                      myStepFor(openSlip)!.id,
                      'rejected',
                      note || undefined,
                    );
                    openSlip = null;
                  }, 'Returned to sender')
                "
              >
                <XCircleIcon class="h-3.5 w-3.5" /> Return
              </Button>
            </template>
            <template v-else>
              <Button
                class="gap-1.5"
                @click="
                  withUndo(() => {
                    advanceStep(
                      openSlip!.id,
                      myStepFor(openSlip)!.id,
                      'forwarded',
                      note || undefined,
                    );
                    openSlip = null;
                  }, 'Forwarded to next recipient')
                "
              >
                <ArrowRightIcon class="h-3.5 w-3.5" /> Mark done &amp; forward
              </Button>
            </template>
          </div>
        </div>
      </template>
    </SheetContent>
  </Sheet>

  <Teleport to="body">
    <SignDialog
      v-if="signFor"
      :open="!!signFor"
      :doc-id="signFor.docId"
      :doc-name="signFor.docName"
      @update:open="
        (v: boolean) => {
          if (!v) signFor = null;
        }
      "
    />
  </Teleport>
</template>
