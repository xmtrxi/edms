<script setup lang="ts">
  import {
    GripVerticalIcon,
    FileTextIcon,
    ClockIcon,
    ArrowRightIcon,
    SendIcon,
    CheckCircle2Icon,
    XCircleIcon,
    PlusIcon,
  } from "@lucide/vue";
  import { ref, computed } from "vue";
  import { useRouter } from "vue-router";
  import { toast } from "vue-sonner";

  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Card, CardContent } from "@/components/ui/card";
  import { Separator } from "@/components/ui/separator";
  import { documents } from "@/data/mock-data";
  import { cn } from "@/lib/utils";

  definePageMeta({
    title: "Routing Kanban",
    description: "Drag documents through your routing pipeline — review, approve, sign, and file.",
    breadcrumbs: [{ label: "Collaboration", to: "/" }, { label: "Routing Kanban" }],
  });

  useHead({
    title: "Routing Kanban — Lumen DMS",
  });

  interface KanbanItem {
    id: string;
    title: string;
    docName: string;
    docId: string;
    priority: string;
    assignee: string;
    initials: string;
    role: string;
    dueDate?: string;
    slipId?: string;
  }

  interface KanbanColumn {
    id: string;
    title: string;
    color: string;
    items: KanbanItem[];
  }

  const dragItem = ref<{ colId: string; itemId: string } | null>(null);
  const dragOverCol = ref<string | null>(null);

  const pendingItems = ref<KanbanItem[]>([
    {
      id: "k1",
      title: "MSA Acme Corp v4",
      docName: "MSA_Acme_Corp_v4.pdf",
      docId: "doc_1",
      priority: "high",
      assignee: "Sofia Reyes",
      initials: "SR",
      role: "Legal Counsel",
      dueDate: "2026-06-02",
    },
    {
      id: "k2",
      title: "NDA Helix Labs",
      docName: "NDA_Helix_Labs.pdf",
      docId: "doc_7",
      priority: "medium",
      assignee: "Sofia Reyes",
      initials: "SR",
      role: "Legal Counsel",
      dueDate: "2026-06-05",
    },
    {
      id: "k3",
      title: "Patent Filing US",
      docName: "Patent_Filing_US.pdf",
      docId: "doc_20",
      priority: "low",
      assignee: "Marcus Bell",
      initials: "MB",
      role: "Director, Operations",
    },
  ]);

  const reviewItems = ref<KanbanItem[]>([
    {
      id: "k4",
      title: "Q3 Financial Report",
      docName: "Q3_Financial_Report.xlsx",
      docId: "doc_2",
      priority: "high",
      assignee: "Marcus Bell",
      initials: "MB",
      role: "Director, Operations",
      dueDate: "2026-06-03",
    },
    {
      id: "k5",
      title: "Employee Handbook 2026",
      docName: "Employee_Handbook_2026.docx",
      docId: "doc_3",
      priority: "medium",
      assignee: "Naledi Okafor",
      initials: "NO",
      role: "Head of HR",
    },
  ]);

  const approvedItems = ref<KanbanItem[]>([
    {
      id: "k6",
      title: "Vendor Risk Vantage",
      docName: "Vendor_Risk_Vantage.pdf",
      docId: "doc_5",
      priority: "medium",
      assignee: "Aiko Tanaka",
      initials: "AT",
      role: "Executive Secretary",
    },
  ]);

  const signedItems = ref<KanbanItem[]>([
    {
      id: "k7",
      title: "RFC-014 Event Driven",
      docName: "RFC-014_Event_Driven.pdf",
      docId: "doc_4",
      priority: "high",
      assignee: "Idris Khan",
      initials: "IK",
      role: "CTO",
      dueDate: "2026-05-30",
    },
  ]);

  const filedItems = ref<KanbanItem[]>([]);

  const columns = computed<KanbanColumn[]>(() => [
    {
      id: "pending",
      title: "To Review",
      color: "bg-muted-foreground/20",
      items: pendingItems.value,
    },
    { id: "review", title: "In Review", color: "bg-info/20", items: reviewItems.value },
    { id: "approved", title: "Approved", color: "bg-success/20", items: approvedItems.value },
    { id: "signed", title: "Signed", color: "bg-primary/20", items: signedItems.value },
    { id: "filed", title: "Filed", color: "bg-secondary/60", items: filedItems.value },
  ]);

  const columnRefs: Record<string, KanbanItem[]> = {
    pending: pendingItems.value,
    review: reviewItems.value,
    approved: approvedItems.value,
    signed: signedItems.value,
    filed: filedItems.value,
  };

  const onDragStart = (colId: string, itemId: string, e: DragEvent) => {
    dragItem.value = { colId, itemId };
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", itemId);
    }
  };

  const onDragOver = (colId: string, e: DragEvent) => {
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
    dragOverCol.value = colId;
  };

  const onDragLeave = (colId: string) => {
    if (dragOverCol.value === colId) dragOverCol.value = null;
  };

  const onDrop = (colId: string) => {
    if (!dragItem.value || dragItem.value.colId === colId) {
      dragItem.value = null;
      dragOverCol.value = null;
      return;
    }

    const { colId: fromCol, itemId } = dragItem.value;
    const fromRef = columnRefs[fromCol];
    const toRef = columnRefs[colId];
    if (!fromRef || !toRef) return;

    const itemIndex = fromRef.findIndex((i) => i.id === itemId);
    if (itemIndex === -1) return;

    const [movedItem] = fromRef.splice(itemIndex, 1);

    if (movedItem) {
      toRef.unshift(movedItem);
      const colNames: Record<string, string> = {
        pending: "To Review",
        review: "In Review",
        approved: "Approved",
        signed: "Signed",
        filed: "Filed",
      };
      toast(`"${movedItem.title}" moved to ${colNames[colId]}`, {
        action: {
          label: "Undo",
          onClick: () => {
            const idx = toRef.findIndex((i) => i.id === itemId);
            if (idx !== -1) toRef.splice(idx, 1);
            fromRef.splice(itemIndex, 0, movedItem);
            toast.success("Moved back");
          },
        },
      });
    }

    dragItem.value = null;
    dragOverCol.value = null;
  };

  const priorityClass = (p: string) => {
    return p === "urgent"
      ? "bg-destructive/15 text-destructive"
      : p === "high"
        ? "bg-warning/15 text-warning"
        : p === "low"
          ? "bg-secondary text-muted-foreground"
          : "bg-info/15 text-info";
  };
</script>

<template>
  <Teleport to="#page-actions-target">
    <Button size="sm" class="gap-1.5" @click="toast.info('New routing task — opening dialog')">
      <PlusIcon class="h-4 w-4" /> New task
    </Button>
  </Teleport>

  <div
    class="flex min-h-[calc(100dvh-16rem)] gap-3 overflow-x-auto pb-4"
    @dragend="
      dragItem = null;
      dragOverCol = null;
    "
  >
    <div
      v-for="col in columns"
      :key="col.id"
      @dragover="onDragOver(col.id, $event)"
      @dragleave="onDragLeave(col.id)"
      @drop="onDrop(col.id)"
      :class="
        cn(
          'border-border bg-secondary/20 flex max-w-[300px] min-w-[240px] flex-1 flex-col rounded-xl border transition-colors',
          dragOverCol === col.id ? 'border-primary/50 bg-primary/5' : '',
        )
      "
    >
      <div class="border-border flex items-center justify-between border-b px-3 py-2.5">
        <div class="flex items-center gap-2">
          <div
            :class="
              cn('h-2.5 w-2.5 rounded-full', col.color.replace('bg-', 'bg-').replace('/20', '/60'))
            "
          />
          <span class="text-xs font-semibold">{{ col.title }}</span>
        </div>
        <Badge variant="secondary" class="h-5 min-w-5 border-0 px-1.5 text-[10px]">{{
          col.items.length
        }}</Badge>
      </div>

      <div class="flex-1 space-y-2 overflow-y-auto p-2">
        <div
          v-for="item in col.items"
          :key="item.id"
          :draggable="true"
          @dragstart="onDragStart(col.id, item.id, $event)"
          class="border-border bg-card shadow-elegant hover:shadow-glow cursor-grab rounded-lg border p-3 transition-all hover:-translate-y-0.5 active:cursor-grabbing"
        >
          <div class="flex items-start gap-2">
            <GripVerticalIcon class="text-muted-foreground/40 mt-0.5 h-3.5 w-3.5 shrink-0" />
            <div class="min-w-0 flex-1">
              <div class="mb-1 flex items-center gap-1">
                <Badge
                  variant="secondary"
                  :class="`border-0 px-1.5 py-0 text-[9px] capitalize ${priorityClass(item.priority)}`"
                  >{{ item.priority }}</Badge
                >
              </div>
              <h4 class="line-clamp-2 text-xs leading-snug font-semibold">{{ item.title }}</h4>
              <p class="text-muted-foreground mt-0.5 flex items-center gap-1 truncate text-[10px]">
                <FileTextIcon class="h-2.5 w-2.5" /> {{ item.docName }}
              </p>
            </div>
          </div>

          <Separator class="my-2" />

          <div class="flex items-center justify-between">
            <div class="flex items-center gap-1.5">
              <Avatar class="h-5 w-5"
                ><AvatarFallback class="bg-secondary text-[8px]">{{
                  item.initials
                }}</AvatarFallback></Avatar
              >
              <span class="text-muted-foreground max-w-[80px] truncate text-[10px]">{{
                item.assignee
              }}</span>
            </div>
            <div
              v-if="item.dueDate"
              class="text-muted-foreground flex items-center gap-1 text-[10px]"
            >
              <ClockIcon class="h-2.5 w-2.5" />
              <span>{{ item.dueDate }}</span>
            </div>
          </div>
        </div>

        <div
          v-if="col.items.length === 0"
          class="flex flex-col items-center justify-center py-8 text-center"
        >
          <div class="text-muted-foreground/50 text-xs">Drop items here</div>
        </div>
      </div>
    </div>
  </div>
</template>
