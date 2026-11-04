<script setup lang="ts">
  import {
    CheckCircle2Icon,
    ChevronDownIcon,
    FileDownIcon,
    HashIcon,
    Loader2Icon,
    RotateCwIcon,
    StampIcon,
    TriangleAlertIcon,
    WrenchIcon,
  } from "@lucide/vue";
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
    DropdownMenuTrigger,
  } from "@/components/ui/dropdown-menu";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import type { PdfJob, PdfOperation, PdfOptions } from "@/types/api";
  import { formatRelative } from "@/utils/format";

  const props = defineProps<{ documentId: string }>();
  // A job finished and stored a new version, so the page should reload it.
  const emit = defineEmits<{ changed: [] }>();

  const pdf = pdfToolsService();
  const auth = useAuthStore();
  const { subscribe } = useRealtime();
  const jobs = ref<PdfJob[]>([]);

  const dialog = ref<PdfOperation | null>(null);
  const options = ref<PdfOptions>({});
  const titles: Record<string, { title: string; description: string }> = {
    compress: {
      title: "Compress PDF",
      description: "Make the file smaller. Stronger levels may lower image quality.",
    },
    rotate: { title: "Rotate pages", description: "Turn every page clockwise." },
    watermark: {
      title: "Add a watermark",
      description: "Stamp text diagonally across every page.",
    },
    page_numbers: { title: "Add page numbers", description: "Number every page." },
  };

  const load = async () => {
    jobs.value = (await pdf.jobs(props.documentId)) ?? [];
  };

  let stop: (() => void) | undefined;
  onMounted(() => {
    void load();
    const id = auth.user?.id;
    if (!id) return;
    stop = subscribe(`user:${id}`, (event) => {
      if (event.type !== "pdf_job") return;
      const job = event.data as PdfJob;
      if (!job.document_ids.includes(props.documentId)) return;
      const index = jobs.value.findIndex((item) => item.id === job.id);
      const previous = index >= 0 ? jobs.value[index] : undefined;
      jobs.value =
        index >= 0
          ? jobs.value.map((item) => (item.id === job.id ? job : item))
          : [job, ...jobs.value];
      if (
        previous?.status !== job.status &&
        job.status === "succeeded" &&
        job.operation !== "merge"
      ) {
        toast.success(`${pdfOperationLabel[job.operation]} finished — saved as a new version`);
        emit("changed");
      }
      if (previous?.status !== job.status && job.status === "failed") {
        toast.error(`${pdfOperationLabel[job.operation]} failed`, { description: job.error });
      }
    });
  });
  onBeforeUnmount(() => stop?.());

  const open = (operation: PdfOperation) => {
    options.value =
      operation === "compress"
        ? { level: 5 }
        : operation === "rotate"
          ? { angle: 90 }
          : operation === "watermark"
            ? { text: "CONFIDENTIAL", opacity: 0.3 }
            : { position: 8 };
    dialog.value = operation;
  };

  const submit = async () => {
    if (!dialog.value) return;
    const job = await pdf.run(dialog.value, [props.documentId], options.value);
    if (!job) {
      toast.error(pdf.error.value || "Could not start the PDF tool");
      return;
    }
    jobs.value = [job, ...jobs.value.filter((item) => item.id !== job.id)];
    dialog.value = null;
    toast("Working on it — the result will be saved as a new version.");
  };

  const retry = async (job: PdfJob) => {
    const updated = await pdf.retry(job.id);
    if (!updated) toast.error(pdf.error.value || "Could not retry");
  };

  const active = computed(() => jobs.value.filter((job) => job.status !== "succeeded").slice(0, 5));
  const statusText: Record<string, string> = {
    queued: "Waiting",
    running: "Working…",
    retrying: "Retrying",
    failed: "Failed",
  };
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <Button size="sm" variant="outline" class="gap-1.5">
        <WrenchIcon class="h-3.5 w-3.5" /> PDF tools <ChevronDownIcon class="h-3 w-3 opacity-60" />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end">
      <DropdownMenuItem @click="open('compress')"
        ><FileDownIcon class="mr-2 h-4 w-4" />Compress</DropdownMenuItem
      >
      <DropdownMenuItem @click="open('rotate')"
        ><RotateCwIcon class="mr-2 h-4 w-4" />Rotate pages</DropdownMenuItem
      >
      <DropdownMenuItem @click="open('watermark')"
        ><StampIcon class="mr-2 h-4 w-4" />Watermark</DropdownMenuItem
      >
      <DropdownMenuItem @click="open('page_numbers')"
        ><HashIcon class="mr-2 h-4 w-4" />Page numbers</DropdownMenuItem
      >
    </DropdownMenuContent>
  </DropdownMenu>

  <Teleport to="body">
    <div
      v-if="active.length"
      class="bg-card fixed right-4 bottom-4 z-40 w-72 space-y-2 rounded-xl border p-3 shadow-lg"
      role="status"
      aria-live="polite"
    >
      <p class="text-muted-foreground text-xs font-semibold tracking-wider uppercase">PDF tools</p>
      <div v-for="job in active" :key="job.id" class="flex items-start gap-2 text-sm">
        <TriangleAlertIcon
          v-if="job.status === 'failed'"
          class="text-destructive mt-0.5 size-4 shrink-0"
        />
        <CheckCircle2Icon
          v-else-if="job.status === 'succeeded'"
          class="text-primary mt-0.5 size-4 shrink-0"
        />
        <Loader2Icon v-else class="text-muted-foreground mt-0.5 size-4 shrink-0 animate-spin" />
        <div class="min-w-0 flex-1">
          <p class="font-medium">
            {{ pdfOperationLabel[job.operation] }} · {{ statusText[job.status] }}
          </p>
          <p v-if="job.error" class="text-muted-foreground line-clamp-2 text-xs">{{ job.error }}</p>
          <p v-else class="text-muted-foreground text-xs">
            Started {{ formatRelative(job.created_at) }}
          </p>
        </div>
        <Button
          v-if="job.status === 'failed'"
          size="sm"
          variant="ghost"
          class="h-7"
          @click="retry(job)"
          >Retry</Button
        >
        <button
          v-if="job.status === 'failed'"
          class="text-muted-foreground hover:text-foreground text-xs"
          aria-label="Dismiss"
          @click="jobs = jobs.filter((item) => item.id !== job.id)"
        >
          ×
        </button>
      </div>
    </div>
  </Teleport>

  <Dialog :open="dialog !== null" @update:open="(value) => !value && (dialog = null)">
    <DialogContent v-if="dialog">
      <DialogHeader>
        <DialogTitle>{{ titles[dialog]?.title }}</DialogTitle>
        <DialogDescription>
          {{ titles[dialog]?.description }} The result is saved as a new version; earlier versions
          stay available.
        </DialogDescription>
      </DialogHeader>
      <form id="pdf-tool" class="space-y-4" @submit.prevent="submit">
        <div v-if="dialog === 'compress'" class="space-y-1.5">
          <Label for="pdf-level">Strength: {{ options.level }}</Label>
          <input
            id="pdf-level"
            v-model.number="options.level"
            type="range"
            min="1"
            max="9"
            class="w-full"
          />
          <div class="text-muted-foreground flex justify-between text-xs">
            <span>Lighter</span><span>Smaller file</span>
          </div>
        </div>
        <div
          v-else-if="dialog === 'rotate'"
          class="flex gap-2"
          role="radiogroup"
          aria-label="Rotation"
        >
          <Button
            v-for="angle in [90, 180, 270]"
            :key="angle"
            type="button"
            :variant="options.angle === angle ? 'default' : 'outline'"
            :aria-pressed="options.angle === angle"
            @click="options.angle = angle"
            >{{ angle }}°</Button
          >
        </div>
        <template v-else-if="dialog === 'watermark'">
          <div class="space-y-1.5">
            <Label for="pdf-text">Text</Label>
            <Input id="pdf-text" v-model="options.text" maxlength="200" />
          </div>
          <div class="space-y-1.5">
            <Label for="pdf-opacity"
              >Opacity: {{ Math.round((options.opacity ?? 0.3) * 100) }}%</Label
            >
            <input
              id="pdf-opacity"
              v-model.number="options.opacity"
              type="range"
              min="0.05"
              max="1"
              step="0.05"
              class="w-full"
            />
          </div>
        </template>
        <div v-else class="space-y-1.5">
          <Label for="pdf-position">Position</Label>
          <select
            id="pdf-position"
            v-model.number="options.position"
            class="border-input bg-background h-9 w-full rounded-md border px-3 text-sm"
          >
            <option :value="8">Bottom centre</option>
            <option :value="9">Bottom right</option>
            <option :value="7">Bottom left</option>
            <option :value="2">Top centre</option>
            <option :value="3">Top right</option>
          </select>
        </div>
      </form>
      <DialogFooter>
        <Button variant="ghost" @click="dialog = null">Cancel</Button>
        <Button type="submit" form="pdf-tool" :disabled="pdf.busy.value">{{
          pdf.busy.value ? "Starting…" : "Start"
        }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
