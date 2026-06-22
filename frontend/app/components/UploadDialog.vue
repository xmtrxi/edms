<script setup lang="ts">
  import {
    CheckCircle2Icon,
    FileIcon,
    FileSpreadsheetIcon,
    FileTextIcon,
    ImageIcon,
    PresentationIcon,
    ScanLineIcon,
    ShieldIcon,
    SparklesIcon,
    UploadIcon,
    XIcon,
  } from "@lucide/vue";
  import { ref } from "vue";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
  } from "@/components/ui/dialog";
  import { Progress } from "@/components/ui/progress";
  import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
  import { cn } from "@/lib/utils";

  const props = defineProps<{ open: boolean }>();
  const emit = defineEmits<{ "update:open": [value: boolean] }>();

  interface QueueItem {
    id: string;
    name: string;
    size: string;
    progress: number;
    stage: "uploading" | "virus" | "ocr" | "ai" | "done";
  }

  const seed: QueueItem[] = [
    { id: "u1", name: "Renewal_Memo_Q4.pdf", size: "1.8 MB", progress: 100, stage: "done" },
    { id: "u2", name: "Vendor_SOC2_Report.pdf", size: "4.2 MB", progress: 68, stage: "ocr" },
    { id: "u3", name: "Onboarding_Slides.pptx", size: "12.6 MB", progress: 42, stage: "ai" },
    { id: "u4", name: "Budget_Draft.xlsx", size: "0.6 MB", progress: 22, stage: "virus" },
  ];

  const stageMeta: Record<
    QueueItem["stage"],
    { label: string; icon: typeof FileTextIcon; tone: string }
  > = {
    uploading: { label: "Uploading", icon: UploadIcon, tone: "text-info" },
    virus: { label: "Virus scan", icon: ShieldIcon, tone: "text-warning" },
    ocr: { label: "OCR processing", icon: ScanLineIcon, tone: "text-info" },
    ai: { label: "AI classification", icon: SparklesIcon, tone: "text-primary" },
    done: { label: "Ready", icon: CheckCircle2Icon, tone: "text-success" },
  };

  const stages: QueueItem["stage"][] = ["uploading", "virus", "ocr", "ai", "done"];

  function stageIndex(stage: QueueItem["stage"]) {
    return stages.indexOf(stage);
  }

  const queue = ref<QueueItem[]>(seed);
  const dragOver = ref(false);
  const tab = ref<"upload" | "progress">("upload");

  function addFake() {
    const id = `u${queue.value.length + 1}`;
    queue.value = [
      ...queue.value,
      { id, name: `New_Document_${id}.pdf`, size: "2.4 MB", progress: 0, stage: "uploading" },
    ];
    tab.value = "progress";
  }

  function fileIcon(name: string) {
    if (name.endsWith(".pdf")) return FileTextIcon;
    if (name.endsWith(".xlsx")) return FileSpreadsheetIcon;
    if (name.endsWith(".pptx")) return PresentationIcon;
    if (name.match(/\.(png|jpg|jpeg|gif|svg|webp)/i)) return ImageIcon;
    return FileIcon;
  }
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-xl">
      <DialogHeader>
        <DialogTitle>Upload documents</DialogTitle>
        <DialogDescription
          >Files are scanned, OCR-processed, and classified by AI before being added to your
          library.</DialogDescription
        >
      </DialogHeader>

      <Tabs v-model="tab">
        <TabsList class="grid w-full grid-cols-2">
          <TabsTrigger value="upload" class="gap-1.5"
            ><UploadIcon class="h-3.5 w-3.5" /> Upload files</TabsTrigger
          >
          <TabsTrigger value="progress" class="relative gap-1.5">
            <span>Queue</span>
            <Badge
              v-if="queue.length"
              variant="secondary"
              class="bg-primary/10 text-primary ml-1 h-5 border-0 px-1.5 text-[10px]"
              >{{ queue.length }}</Badge
            >
          </TabsTrigger>
        </TabsList>

        <TabsContent value="upload" class="mt-4 space-y-4">
          <div
            @dragover.prevent="dragOver = true"
            @dragleave="dragOver = false"
            @drop.prevent="
              dragOver = false;
              addFake();
            "
            @click="addFake"
            :class="
              cn(
                'cursor-pointer rounded-xl border-2 border-dashed p-10 text-center transition-all',
                dragOver
                  ? 'border-primary bg-primary/5 scale-[1.01]'
                  : 'border-border hover:border-primary/50 hover:bg-secondary/40',
              )
            "
          >
            <div
              class="bg-primary/10 text-primary ring-primary/20 mx-auto grid h-14 w-14 place-items-center rounded-2xl ring-1"
            >
              <UploadIcon class="h-6 w-6" />
            </div>
            <p class="mt-4 text-sm font-medium">Drop files here or click to browse</p>
            <p class="text-muted-foreground mt-1 text-xs">
              PDF, DOCX, XLSX, PPTX, images &middot; up to 250 MB each
            </p>
          </div>

          <div class="grid grid-cols-2 gap-2">
            <div
              v-for="fmt in [
                { label: 'PDF', icon: FileTextIcon, desc: 'Documents, contracts' },
                { label: 'Excel', icon: FileSpreadsheetIcon, desc: 'Spreadsheets' },
                { label: 'PowerPoint', icon: PresentationIcon, desc: 'Presentations' },
                { label: 'Images', icon: ImageIcon, desc: 'PNG, JPG, WebP' },
              ]"
              :key="fmt.label"
              class="border-border bg-secondary/30 flex items-center gap-2 rounded-lg border px-3 py-2"
            >
              <component :is="fmt.icon" class="text-muted-foreground h-4 w-4" />
              <div>
                <p class="text-xs font-medium">{{ fmt.label }}</p>
                <p class="text-muted-foreground text-[10px]">{{ fmt.desc }}</p>
              </div>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="progress" class="mt-4 space-y-3">
          <div
            v-if="queue.length === 0"
            class="border-border rounded-xl border border-dashed p-8 text-center"
          >
            <CheckCircle2Icon class="text-muted-foreground/40 mx-auto h-8 w-8" />
            <p class="text-muted-foreground mt-2 text-sm">No files in queue</p>
          </div>
          <div v-else class="max-h-80 space-y-2 overflow-y-auto pr-1">
            <div
              v-for="q in queue"
              :key="q.id"
              class="border-border bg-card rounded-lg border p-3.5"
            >
              <div class="flex items-center gap-3">
                <div
                  class="bg-secondary text-muted-foreground grid h-10 w-10 shrink-0 place-items-center rounded-lg"
                >
                  <component :is="fileIcon(q.name)" class="h-5 w-5" />
                </div>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center justify-between gap-2">
                    <span class="truncate text-sm font-medium">{{ q.name }}</span>
                    <span class="text-muted-foreground shrink-0 text-[11px]">{{ q.size }}</span>
                  </div>
                  <div class="mt-2 flex items-center gap-2">
                    <Progress :model-value="q.progress" class="h-1.5 flex-1" />
                    <Badge
                      variant="secondary"
                      :class="
                        cn(
                          'bg-secondary h-5 gap-1 border-0 px-1.5 text-[10px] font-medium',
                          stageMeta[q.stage].tone,
                        )
                      "
                    >
                      <component :is="stageMeta[q.stage].icon" class="h-3 w-3" />
                      {{ stageMeta[q.stage].label }}
                    </Badge>
                  </div>
                  <div class="mt-1.5 flex gap-1">
                    <div v-for="(s, si) in stages" :key="s" class="flex items-center gap-1">
                      <div
                        :class="
                          cn(
                            'h-1.5 w-1.5 rounded-full',
                            si <= stageIndex(q.stage) ? 'bg-primary' : 'bg-secondary',
                          )
                        "
                      />
                      <span
                        v-if="si === stageIndex(q.stage)"
                        class="text-muted-foreground text-[9px]"
                        >{{ stageMeta[s].label }}</span
                      >
                    </div>
                  </div>
                </div>
                <Button variant="ghost" size="icon" class="h-7 w-7 shrink-0" aria-label="Remove">
                  <XIcon class="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          </div>
        </TabsContent>
      </Tabs>

      <DialogFooter>
        <Button variant="ghost" @click="emit('update:open', false)">Close</Button>
        <Button @click="emit('update:open', false)">Done</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
