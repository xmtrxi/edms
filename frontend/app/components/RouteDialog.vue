<script setup lang="ts">
  import {
    ArrowRightIcon,
    CheckCircle2Icon,
    ClockIcon,
    FileTextIcon,
    PlusIcon,
    SendIcon,
    UserIcon,
    XIcon,
  } from "@lucide/vue";
  import { ref } from "vue";
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
  import { Label } from "@/components/ui/label";
  import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
  } from "@/components/ui/select";
  import { Separator } from "@/components/ui/separator";
  import { Textarea } from "@/components/ui/textarea";
  import { DIRECTORY, createRoutingSlip, type RouteStep } from "@/lib/office-store";
  import { cn } from "@/lib/utils";

  const props = defineProps<{ open: boolean; docId: string; docName: string }>();
  const emit = defineEmits<{ "update:open": [value: boolean] }>();

  type Draft = RouteStep & { _key: string };

  const ACTIONS: RouteStep["action"][] = ["review", "approve", "sign", "acknowledge", "file"];

  type StepWizard = "details" | "recipients" | "review";

  const wizard = ref<StepWizard>("details");
  const subject = ref(`${props.docName} — routing`);
  const priority = ref<"low" | "normal" | "high" | "urgent">("normal");
  const dueDate = ref("");
  const notes = ref("");
  const steps = ref<Draft[]>([
    {
      _key: "k1",
      id: "s1",
      recipient: "Sofia Reyes",
      initials: "SR",
      role: "Legal Counsel",
      action: "review",
      status: "pending",
    },
    {
      _key: "k2",
      id: "s2",
      recipient: "Marcus Bell",
      initials: "MB",
      role: "Director, Operations",
      action: "approve",
      status: "pending",
    },
  ]);

  function addStep() {
    const d = DIRECTORY[steps.value.length % DIRECTORY.length];
    steps.value = [
      ...steps.value,
      {
        _key: Math.random().toString(36).slice(2),
        id: `s${steps.value.length + 1}`,
        recipient: d.name,
        initials: d.initials,
        role: d.role,
        action: "review",
        status: "pending",
      },
    ];
  }

  function remove(k: string) {
    steps.value = steps.value.filter((s) => s._key !== k);
  }

  function update(k: string, patch: Partial<Draft>) {
    steps.value = steps.value.map((s) => (s._key === k ? { ...s, ...patch } : s));
  }

  function submit() {
    if (!subject.value.trim()) return toast.error("Subject is required");
    if (steps.value.length === 0) return toast.error("Add at least one recipient");
    createRoutingSlip({
      docId: props.docId,
      docName: props.docName,
      subject: subject.value.trim(),
      priority: priority.value,
      dueDate: dueDate.value || undefined,
      notes: notes.value.trim() || undefined,
      steps: steps.value.map(({ _key: _k, ...s }) => s),
    });
    toast.success(`Routed to ${steps.value[0].recipient}`, {
      description: `${steps.value.length} step${steps.value.length > 1 ? "s" : ""} created.`,
    });
    emit("update:open", false);
  }

  const validSteps = ["details", "recipients", "review"] as const;
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent
      class="max-h-[90vh] w-[calc(100%-1rem)] max-w-2xl overflow-y-auto p-4 sm:w-full sm:p-6"
    >
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2 text-base">
          <SendIcon class="text-primary h-4 w-4 shrink-0" /> Route document
        </DialogTitle>
        <DialogDescription class="text-sm"
          >Send "{{ docName }}" through your office in the right order.</DialogDescription
        >
      </DialogHeader>

      <div class="mb-4 flex flex-wrap items-center gap-1">
        <div v-for="(step, i) in validSteps" :key="step" class="flex items-center gap-1">
          <div
            :class="
              cn(
                'flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium transition-colors',
                wizard === step
                  ? 'bg-primary/10 text-primary'
                  : i < validSteps.indexOf(wizard)
                    ? 'bg-success/10 text-success'
                    : 'bg-secondary text-muted-foreground',
              )
            "
          >
            <CheckCircle2Icon v-if="i < validSteps.indexOf(wizard)" class="h-3 w-3" />
            <div
              v-else
              :class="
                cn(
                  'grid h-4 w-4 place-items-center rounded-full text-[10px] font-bold',
                  wizard === step
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-secondary text-muted-foreground',
                )
              "
            >
              {{ i + 1 }}
            </div>
            <span class="whitespace-nowrap capitalize">{{ step }}</span>
          </div>
          <ArrowRightIcon
            v-if="i < validSteps.length - 1"
            class="text-muted-foreground/30 h-3 w-3 shrink-0"
          />
        </div>
      </div>

      <template v-if="wizard === 'details'">
        <div class="grid gap-3">
          <div class="grid gap-1.5">
            <Label class="text-xs font-medium">Subject</Label>
            <Input v-model="subject" class="h-9" />
          </div>

          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div class="grid gap-1.5">
              <Label class="text-xs font-medium">Priority</Label>
              <Select v-model="priority">
                <SelectTrigger class="h-9"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="low">Low</SelectItem>
                  <SelectItem value="normal">Normal</SelectItem>
                  <SelectItem value="high">High</SelectItem>
                  <SelectItem value="urgent">Urgent</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="grid gap-1.5">
              <Label class="text-xs font-medium">Due date</Label>
              <Input v-model="dueDate" type="date" class="h-9" />
            </div>
          </div>

          <div class="grid gap-1.5">
            <Label class="text-xs font-medium"
              >Cover note <span class="text-muted-foreground font-normal">(optional)</span></Label
            >
            <Textarea
              v-model="notes"
              rows="3"
              placeholder="Instructions for the recipients…"
              class="text-sm"
            />
          </div>
        </div>
      </template>

      <template v-if="wizard === 'recipients'">
        <div>
          <div class="mb-3 flex items-center justify-between">
            <Label class="text-xs font-medium">Routing sequence</Label>
            <Button size="sm" variant="outline" class="h-7 gap-1.5" @click="addStep"
              ><PlusIcon class="h-3.5 w-3.5" /> Add step</Button
            >
          </div>
          <div class="space-y-2">
            <div v-for="(s, i) in steps" :key="s._key">
              <div
                class="border-border bg-card flex flex-col gap-2 rounded-lg border p-3 sm:flex-row sm:items-center"
              >
                <div class="flex min-w-0 flex-1 items-center gap-2">
                  <div
                    class="bg-secondary text-muted-foreground grid h-7 w-7 shrink-0 place-items-center rounded-full text-xs font-semibold"
                  >
                    {{ i + 1 }}
                  </div>
                  <Avatar class="h-8 w-8 shrink-0"
                    ><AvatarFallback class="bg-primary/10 text-primary text-[10px]">{{
                      s.initials
                    }}</AvatarFallback></Avatar
                  >
                  <div class="min-w-0 flex-1">
                    <Select
                      :model-value="s.recipient"
                      @update:model-value="
                        (v: string) => {
                          const d = DIRECTORY.find((x) => x.name === v)!;
                          update(s._key, { recipient: d.name, initials: d.initials, role: d.role });
                        }
                      "
                    >
                      <SelectTrigger class="h-8 w-full sm:w-44"><SelectValue /></SelectTrigger>
                      <SelectContent>
                        <SelectItem v-for="d in DIRECTORY" :key="d.name" :value="d.name"
                          >{{ d.name }}
                          <span class="text-muted-foreground">— {{ d.role }}</span></SelectItem
                        >
                      </SelectContent>
                    </Select>
                    <p class="text-muted-foreground mt-0.5 truncate text-[10px]">{{ s.role }}</p>
                  </div>
                </div>
                <div class="flex items-center gap-2 sm:gap-1.5">
                  <Select
                    :model-value="s.action"
                    @update:model-value="
                      (v: string) => update(s._key, { action: v as RouteStep['action'] })
                    "
                  >
                    <SelectTrigger class="h-8 w-full sm:w-[120px]"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem v-for="a in ACTIONS" :key="a" :value="a" class="capitalize">{{
                        a
                      }}</SelectItem>
                    </SelectContent>
                  </Select>
                  <Button
                    variant="ghost"
                    size="icon"
                    class="h-7 w-7 shrink-0"
                    @click="remove(s._key)"
                    ><XIcon class="h-3.5 w-3.5"
                  /></Button>
                </div>
              </div>
              <div v-if="i < steps.length - 1" class="my-1.5 ml-3.5 flex items-center gap-2">
                <div class="bg-border ml-[14px] h-6 w-px" />
                <ArrowRightIcon class="text-muted-foreground/40 h-3 w-3" />
              </div>
            </div>
          </div>
        </div>
      </template>

      <template v-if="wizard === 'review'">
        <div class="border-border bg-secondary/30 rounded-xl border p-4">
          <div class="mb-3 flex items-center gap-3">
            <div
              class="bg-primary/10 text-primary grid h-10 w-10 shrink-0 place-items-center rounded-xl"
            >
              <FileTextIcon class="h-5 w-5" />
            </div>
            <div class="min-w-0">
              <p class="truncate text-sm font-medium">{{ subject }}</p>
              <p class="text-muted-foreground truncate text-xs">{{ props.docName }}</p>
            </div>
          </div>
          <Separator class="mb-3" />
          <div class="text-muted-foreground mb-3 flex flex-wrap items-center gap-3 text-xs">
            <Badge variant="secondary" class="border-0 text-[10px] capitalize">{{
              priority
            }}</Badge>
            <span v-if="dueDate" class="flex items-center gap-1"
              ><ClockIcon class="h-3 w-3" /> Due {{ dueDate }}</span
            >
            <span>{{ steps.length }} step{{ steps.length > 1 ? "s" : "" }}</span>
          </div>
          <div class="space-y-2">
            <div v-for="(s, i) in steps" :key="s._key" class="flex items-center gap-2">
              <div class="flex min-w-0 flex-1 items-center gap-2">
                <Avatar class="h-6 w-6 shrink-0"
                  ><AvatarFallback class="bg-secondary text-[9px]">{{
                    s.initials
                  }}</AvatarFallback></Avatar
                >
                <span class="truncate text-xs font-medium">{{ s.recipient }}</span>
                <span class="text-muted-foreground shrink-0 text-[10px]">— {{ s.action }}</span>
              </div>
              <ArrowRightIcon
                v-if="i < steps.length - 1"
                class="text-muted-foreground/30 h-3 w-3 shrink-0"
              />
            </div>
          </div>
          <p v-if="notes" class="bg-card border-border mt-3 rounded-lg border p-2.5 text-xs">
            {{ notes }}
          </p>
        </div>
      </template>

      <div class="mt-4 flex items-center justify-between">
        <Button
          v-if="wizard !== 'details'"
          variant="ghost"
          size="sm"
          @click="wizard = validSteps[validSteps.indexOf(wizard) - 1]"
          >Back</Button
        >
        <div v-else />
        <Button
          v-if="wizard !== 'review'"
          size="sm"
          @click="wizard = validSteps[validSteps.indexOf(wizard) + 1]"
          >Continue</Button
        >
      </div>

      <Separator class="my-2" />

      <DialogFooter class="flex-col-reverse gap-2 sm:flex-row">
        <Button variant="outline" class="w-full sm:w-auto" @click="emit('update:open', false)"
          >Cancel</Button
        >
        <Button v-if="wizard === 'review'" @click="submit" class="w-full gap-1.5 sm:w-auto"
          ><SendIcon class="h-3.5 w-3.5" /> Send for routing</Button
        >
        <Button v-else disabled class="w-full opacity-40 sm:w-auto">Send for routing</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
