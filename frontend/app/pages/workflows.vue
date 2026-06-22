<script setup lang="ts">
import {
  PlusIcon,
  CheckCircle2Icon,
  XCircleIcon,
  ArrowRightIcon,
  FileTextIcon,
  GitBranchIcon,
  SparklesIcon,
  ClockIcon,
  AlertTriangleIcon,
  HashIcon,
  SendIcon,
  InboxIcon,
  Building2Icon,
} from "@lucide/vue"
import { ref, shallowRef, computed } from "vue"
import { toast } from "vue-sonner"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { workflows as seedWorkflows, type Workflow } from "@/data/mock-data"

definePageMeta({
  title: "Workflows",
  description: "Approvals, reviews, and signatures across your organization.",
  breadcrumbs: [{ label: "Collaboration", to: "/" }, { label: "Workflows" }],
})

useHead({
  title: "Workflows — Lumen DMS",
  meta: [{ name: "description", content: "Manage approval workflows, reviews, and document routing across teams." }],
})

const statusMeta: Record<string, { label: string; Icon: typeof ClockIcon; tone: string }> = {
  pending: { label: "Pending", Icon: ClockIcon, tone: "bg-secondary text-muted-foreground" },
  in_review: { label: "In Review", Icon: ClockIcon, tone: "bg-info/10 text-info" },
  approved: { label: "Approved", Icon: CheckCircle2Icon, tone: "bg-success/10 text-success" },
  rejected: { label: "Rejected", Icon: XCircleIcon, tone: "bg-destructive/10 text-destructive" },
}

const templates = [
  { id: "t1", name: "Contract approval", desc: "Legal review → Director sign-off.", stages: 2, defaultTitle: "Approve contract", defaultType: "approval" as const },
  { id: "t2", name: "Procurement PR", desc: "Budget owner → Finance → Procurement.", stages: 3, defaultTitle: "Procurement request", defaultType: "approval" as const },
  { id: "t3", name: "Expense report", desc: "Manager → Accounting.", stages: 2, defaultTitle: "Approve expense report", defaultType: "approval" as const },
  { id: "t4", name: "NDA review", desc: "Counterparty → Legal.", stages: 2, defaultTitle: "NDA review", defaultType: "review" as const },
  { id: "t5", name: "Board resolution", desc: "Secretary → Board → Legal → Archive.", stages: 4, defaultTitle: "Board resolution sign-off", defaultType: "signature" as const },
]

const items = shallowRef<Workflow[]>(seedWorkflows)
const newOpen = ref(false)
const active = shallowRef<Workflow | null>(null)
const newTitle = ref("")
const newType = ref("approval")
const newPriority = ref("medium")
const newDue = ref("")
const newNotes = ref("")

const overdue = computed(() => items.value.filter((w) => (w.status === "pending" || w.status === "in_review") && w.due === "Today"))
const activeItems = computed(() => items.value.filter((w) => w.status === "pending" || w.status === "in_review"))
const mine = computed(() => items.value.filter((w) => w.assignees.some((a) => a.initials === "AT")))
const completed = computed(() => items.value.filter((w) => w.status === "approved" || w.status === "rejected"))

const stages = ["Submitted", "Legal review", "Finance sign-off", "Final approval"]

function updateItem(id: string, patch: Partial<Workflow>) {
  items.value = items.value.map((w) => (w.id === id ? { ...w, ...patch } : w))
}

function approveItem(id: string, title: string) {
  updateItem(id, { status: "approved", progress: 100 })
  toast.success('Approved "' + title + '"')
}

function rejectItem(id: string, title: string) {
  updateItem(id, { status: "rejected", progress: 100 })
  toast.error('Rejected "' + title + '"')
}

const createItem = () => {
  const w: Workflow = {
    id: `wf-${Date.now()}`,
    title: newTitle.value.trim() || "New workflow",
    doc: "No document attached",
    status: "pending",
    type: newType.value as Workflow["type"],
    priority: newPriority.value as Workflow["priority"],
    progress: 0,
    due: newDue.value || "Next week",
    assignees: [{ initials: "AT", name: "Aiko Tanaka" }],
  }
  items.value = [w, ...items.value]
  newOpen.value = false
  newTitle.value = ""
  newDue.value = ""
  newNotes.value = ""
  toast.success(`Workflow "${w.title}" created`)
}

const useTemplate = (t: typeof templates[number]) => {
  newTitle.value = t.defaultTitle
  newType.value = t.defaultType
  newPriority.value = "medium"
  newDue.value = ""
  newNotes.value = ""
  newOpen.value = true
}

const refNum = (id: string) => `WF-${id.slice(-5).toUpperCase()}`
</script>

<template>
  <Teleport to="#page-actions-target">
    <Button size="sm" class="gap-1.5" @click="newOpen = true">
      <PlusIcon class="h-4 w-4" /> New workflow
    </Button>
  </Teleport>

  <Tabs default-value="active">
    <TabsList class="bg-secondary/60">
      <TabsTrigger value="active">
        <InboxIcon class="h-3.5 w-3.5 md:mr-1.5" />
        <span class="hidden md:inline">Active</span>
        <Badge variant="secondary" class="bg-primary/10 text-primary ml-1.5 h-4 border-0 px-1 text-[10px]">{{ activeItems.length }}</Badge>
      </TabsTrigger>
      <TabsTrigger value="my">
        <span class="hidden md:inline">My desk</span>
        <Badge variant="secondary" class="bg-secondary ml-1.5 h-4 border-0 px-1 text-[10px]">{{ mine.length }}</Badge>
      </TabsTrigger>
      <TabsTrigger value="completed">
        <span class="hidden md:inline">Completed</span>
        <Badge variant="secondary" class="bg-secondary ml-1.5 h-4 border-0 px-1 text-[10px]">{{ completed.length }}</Badge>
      </TabsTrigger>
      <TabsTrigger value="templates">Templates</TabsTrigger>
    </TabsList>

    <TabsContent value="active" class="mt-4">
      <div v-if="overdue.length > 0" class="mb-3 flex items-center gap-2 rounded-lg border border-destructive/20 bg-destructive/5 px-3 py-2">
        <AlertTriangleIcon class="h-4 w-4 text-destructive shrink-0" />
        <span class="text-xs font-medium text-destructive">{{ overdue.length }} item{{ overdue.length > 1 ? 's' : '' }} past due</span>
      </div>
      <div v-if="activeItems.length === 0" class="border-border text-muted-foreground rounded-xl border border-dashed p-10 text-center text-sm">
        <InboxIcon class="mx-auto h-6 w-6 mb-2 text-muted-foreground/40" />
        No active workflows.
      </div>
      <div v-else class="grid grid-cols-1 gap-3 lg:grid-cols-2">
        <Card
          v-for="w in activeItems"
          :key="w.id"
          :class="`border-border shadow-sm hover:shadow-md cursor-pointer transition-all group ${w.due === 'Today' ? 'border-destructive/30' : ''}`"
          @click="active = w"
        >
          <CardContent class="p-4">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-1.5">
                  <span class="text-[10px] text-muted-foreground/60 font-mono">{{ refNum(w.id) }}</span>
                  <Badge variant="secondary" class="bg-secondary border-0 text-[10px] capitalize">{{ w.type }}</Badge>
                  <Badge variant="secondary" :class="`border-0 text-[10px] capitalize ${w.priority === 'high' ? 'bg-destructive/10 text-destructive' : w.priority === 'medium' ? 'bg-warning/10 text-warning' : 'bg-success/10 text-success'}`">{{ w.priority }}</Badge>
                </div>
                <h3 class="mt-1.5 text-sm font-semibold leading-snug">{{ w.title }}</h3>
                <p class="text-muted-foreground mt-0.5 flex items-center gap-1 text-xs">
                  <FileTextIcon class="h-3 w-3 shrink-0" />{{ w.doc }}
                </p>
              </div>
              <div class="shrink-0 text-right">
                <Badge variant="secondary" :class="`shrink-0 gap-1 border-0 text-[10px] ${statusMeta[w.status].tone}`">
                  <component :is="statusMeta[w.status].Icon" class="h-3 w-3" />
                  {{ statusMeta[w.status].label }}
                </Badge>
                <div class="mt-1 flex items-center gap-1 text-[10px]" :class="w.due === 'Today' ? 'text-destructive font-medium' : 'text-muted-foreground'">
                  <ClockIcon class="h-3 w-3" />
                  {{ w.due }}
                </div>
              </div>
            </div>
            <div class="mt-2 flex items-center gap-2">
              <div class="flex-1 h-1 rounded-full bg-secondary/60 overflow-hidden">
                <div class="h-full rounded-full bg-primary/60" :style="{ width: `${w.progress}%` }" />
              </div>
              <span class="text-[11px] tabular-nums text-muted-foreground">{{ w.progress }}%</span>
            </div>
            <div class="mt-3 flex items-center justify-between">
              <div class="flex items-center gap-2">
                <div class="flex -space-x-1">
                  <Avatar v-for="a in w.assignees" :key="a.initials" class="ring-card h-6 w-6 ring-2">
                    <AvatarFallback class="bg-secondary text-[9px]">{{ a.initials }}</AvatarFallback>
                  </Avatar>
                </div>
              </div>
              <div class="flex items-center gap-1" @click.stop>
                <Button variant="outline" size="sm" class="h-7 text-xs gap-1" @click="approveItem(w.id, w.title)">
                  <CheckCircle2Icon class="h-3 w-3" /> Approve
                </Button>
                <Button variant="ghost" size="sm" class="h-7 text-xs" @click="rejectItem(w.id, w.title)">
                  <XCircleIcon class="h-3 w-3" />
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </TabsContent>

    <TabsContent value="my" class="mt-4">
      <div v-if="mine.length === 0" class="border-border text-muted-foreground rounded-xl border border-dashed p-10 text-center text-sm">
        Nothing assigned to you.
      </div>
      <div v-else class="grid grid-cols-1 gap-3 lg:grid-cols-2">
        <Card v-for="w in mine" :key="w.id" class="border-border shadow-sm hover:shadow-md cursor-pointer transition-all" @click="active = w">
          <CardContent class="p-4">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-1.5">
                  <span class="text-[10px] text-muted-foreground/60 font-mono">{{ refNum(w.id) }}</span>
                  <Badge variant="secondary" class="bg-secondary border-0 text-[10px] capitalize">{{ w.type }}</Badge>
                  <Badge variant="secondary" :class="`border-0 text-[10px] capitalize ${w.priority === 'high' ? 'bg-destructive/10 text-destructive' : w.priority === 'medium' ? 'bg-warning/10 text-warning' : 'bg-success/10 text-success'}`">{{ w.priority }}</Badge>
                </div>
                <h3 class="mt-1.5 text-sm font-semibold leading-snug">{{ w.title }}</h3>
                <p class="text-muted-foreground mt-0.5 flex items-center gap-1 text-xs"><FileTextIcon class="h-3 w-3 shrink-0" />{{ w.doc }}</p>
              </div>
              <Badge variant="secondary" :class="`shrink-0 gap-1 border-0 text-[10px] ${statusMeta[w.status].tone}`">
                <component :is="statusMeta[w.status].Icon" class="h-3 w-3" />
                {{ statusMeta[w.status].label }}
              </Badge>
            </div>
            <div class="mt-2 flex items-center gap-2">
              <div class="flex-1 h-1 rounded-full bg-secondary/60 overflow-hidden">
                <div class="h-full rounded-full bg-primary/60" :style="{ width: `${w.progress}%` }" />
              </div>
              <span class="text-[11px] tabular-nums text-muted-foreground">{{ w.progress }}%</span>
            </div>
            <div class="mt-3 flex items-center justify-between">
              <div class="flex items-center gap-2 text-xs text-muted-foreground">
                <ClockIcon class="h-3 w-3" /> Due {{ w.due }}
              </div>
              <div class="flex items-center gap-1" @click.stop>
                <Button size="sm" class="h-7 text-xs gap-1" @click="approveItem(w.id, w.title)">
                  <CheckCircle2Icon class="h-3 w-3" /> Approve
                </Button>
                <Button variant="outline" size="sm" class="h-7 text-xs" @click="rejectItem(w.id, w.title)">
                  <XCircleIcon class="h-3 w-3" /> Return
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </TabsContent>

    <TabsContent value="completed" class="mt-4">
      <div v-if="completed.length === 0" class="border-border text-muted-foreground rounded-xl border border-dashed p-10 text-center text-sm">No completed workflows.</div>
      <div v-else class="grid grid-cols-1 gap-2">
        <Card v-for="w in completed" :key="w.id" class="border-border/60 shadow-sm hover:shadow-md cursor-pointer transition-all" @click="active = w">
          <CardContent class="p-3">
            <div class="flex items-center gap-3">
              <div :class="w.status === 'approved' ? 'text-success' : 'text-destructive'" class="shrink-0">
                <component :is="w.status === 'approved' ? CheckCircle2Icon : XCircleIcon" class="h-5 w-5" />
              </div>
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-1.5">
                  <span class="text-[10px] text-muted-foreground/60 font-mono">{{ refNum(w.id) }}</span>
                  <span class="text-sm font-medium truncate">{{ w.title }}</span>
                </div>
                <p class="text-xs text-muted-foreground">{{ w.doc }}</p>
              </div>
              <Badge variant="secondary" class="border-0 text-[10px] capitalize" :class="w.status === 'approved' ? 'bg-success/10 text-success' : 'bg-destructive/10 text-destructive'">{{ w.status }}</Badge>
            </div>
          </CardContent>
        </Card>
      </div>
    </TabsContent>

    <TabsContent value="templates" class="mt-4">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <Card v-for="t in templates" :key="t.id" class="border-border shadow-sm hover:shadow-md transition-all">
          <CardContent class="p-4">
            <div class="bg-primary/10 text-primary grid h-8 w-8 place-items-center rounded-lg"><GitBranchIcon class="h-4 w-4" /></div>
            <h3 class="mt-2.5 text-sm font-semibold">{{ t.name }}</h3>
            <p class="text-muted-foreground text-xs mt-0.5" v-html="t.desc" />
            <div class="mt-3 flex items-center justify-between">
              <Badge variant="secondary" class="bg-secondary border-0 text-[10px]">{{ t.stages }} stages</Badge>
              <Button size="sm" variant="outline" class="h-7 gap-1 text-xs" @click="useTemplate(t)">
                Use <ArrowRightIcon class="h-3 w-3" />
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </TabsContent>
  </Tabs>

  <!-- New workflow dialog -->
  <Dialog :open="newOpen" @update:open="(v: boolean) => (newOpen = v)">
    <DialogContent class="max-w-lg">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2 text-base"><GitBranchIcon class="text-primary h-4 w-4" /> New workflow</DialogTitle>
        <DialogDescription class="text-xs">Route a document through approvals, reviews, or signatures.</DialogDescription>
      </DialogHeader>
      <div class="space-y-3">
        <div class="space-y-1.5">
          <Label for="wtitle">Title</Label>
          <Input id="wtitle" v-model="newTitle" placeholder="e.g. Approve vendor renewal" />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div class="space-y-1.5">
            <Label>Type</Label>
            <Select v-model="newType">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="approval">Approval</SelectItem>
                <SelectItem value="review">Review</SelectItem>
                <SelectItem value="signature">Signature</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="space-y-1.5">
            <Label>Priority</Label>
            <Select v-model="newPriority">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="low">Low</SelectItem>
                <SelectItem value="medium">Medium</SelectItem>
                <SelectItem value="high">High</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <div class="space-y-1.5">
          <Label for="due">Due</Label>
          <Input id="due" v-model="newDue" placeholder="Tomorrow, Friday, 2026-07-15" />
        </div>
        <div class="space-y-1.5">
          <Label for="notes">Notes (optional)</Label>
          <Textarea id="notes" v-model="newNotes" rows="2" placeholder="Context for reviewers…" />
        </div>
        <div class="border-primary/20 bg-primary/5 flex items-start gap-2 rounded-lg border p-3 text-xs">
          <SparklesIcon class="text-primary mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span>AI will suggest reviewers and a routing order based on the document type and library.</span>
        </div>
      </div>
      <DialogFooter>
        <Button variant="ghost" @click="newOpen = false">Cancel</Button>
        <Button @click="createItem()">Create workflow</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <!-- Detail sheet -->
  <Sheet :open="!!active" @update:open="(v: boolean) => { if (!v) active = null }">
    <SheetContent class="w-full overflow-y-auto sm:max-w-md">
      <template v-if="active">
        <SheetHeader>
          <div class="flex items-center gap-1.5 text-[10px] text-muted-foreground/60 font-mono mb-1">{{ refNum(active.id) }}</div>
          <SheetTitle>{{ active.title }}</SheetTitle>
          <SheetDescription class="flex items-center gap-1"><FileTextIcon class="h-3 w-3" /> {{ active.doc }}</SheetDescription>
        </SheetHeader>
        <div class="mt-4 flex flex-wrap items-center gap-1.5">
          <Badge variant="secondary" :class="`border-0 text-[10px] ${statusMeta[active.status].tone}`">{{ statusMeta[active.status].label }}</Badge>
          <Badge variant="secondary" class="bg-secondary border-0 text-[10px] capitalize">{{ active.type }}</Badge>
          <Badge variant="secondary" :class="`border-0 text-[10px] capitalize ${active.priority === 'high' ? 'bg-destructive/10 text-destructive' : active.priority === 'medium' ? 'bg-warning/10 text-warning' : 'bg-success/10 text-success'}`">{{ active.priority }} priority</Badge>
          <Badge variant="secondary" class="border-0 text-[10px]" :class="active.due === 'Today' ? 'bg-destructive/10 text-destructive' : 'bg-secondary'">{{ active.due }}</Badge>
        </div>

        <div class="mt-4">
          <div class="flex items-center justify-between text-[11px] text-muted-foreground mb-1">
            <span>Progress</span><span class="tabular-nums">{{ active.progress }}%</span>
          </div>
          <div class="h-1.5 rounded-full bg-secondary/60 overflow-hidden">
            <div class="h-full rounded-full bg-primary/60" :style="{ width: `${active.progress}%` }" />
          </div>
        </div>

        <Separator class="my-4" />

        <p class="mb-3 text-xs font-medium flex items-center gap-1.5"><Building2Icon class="h-3.5 w-3.5 text-primary" /> Routing stages</p>
        <div class="space-y-2">
          <div v-for="(s, i) in stages" :key="s" class="flex items-center gap-2.5 text-xs">
            <div :class="`grid h-5 w-5 place-items-center rounded-full shrink-0 ${(i / stages.length) * 100 < active.progress ? 'bg-success/15 text-success' : 'bg-secondary text-muted-foreground'}`">
              <CheckCircle2Icon v-if="(i / stages.length) * 100 < active.progress" class="h-3 w-3" />
              <span v-else>{{ i + 1 }}</span>
            </div>
            <span :class="(i / stages.length) * 100 < active.progress ? 'font-medium' : 'text-muted-foreground'">{{ s }}</span>
          </div>
        </div>

        <Separator class="my-4" />

        <p class="mb-2 text-xs font-medium">Assignees</p>
        <div class="space-y-2">
          <div v-for="a in active.assignees" :key="a.initials" class="flex items-center gap-2">
            <Avatar class="h-7 w-7"><AvatarFallback class="bg-secondary text-[10px]">{{ a.initials }}</AvatarFallback></Avatar>
            <span class="text-sm">{{ a.name }}</span>
          </div>
        </div>

        <div v-if="active.status === 'pending' || active.status === 'in_review'" class="mt-6 flex items-center gap-2">
          <Button class="flex-1" size="sm" @click="updateItem(active.id, { status: 'approved', progress: 100 }); active = null; toast.success('Workflow approved')">
            <CheckCircle2Icon class="mr-1.5 h-3.5 w-3.5" /> Approve
          </Button>
          <Button variant="outline" class="flex-1" size="sm" @click="updateItem(active.id, { status: 'rejected', progress: 100 }); active = null; toast.error('Workflow rejected')">
            <XCircleIcon class="mr-1.5 h-3.5 w-3.5" /> Reject
          </Button>
        </div>
      </template>
    </SheetContent>
  </Sheet>
</template>
