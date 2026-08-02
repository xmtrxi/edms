<script setup lang="ts">
  import {
    SearchIcon,
    FileTextIcon,
    SparklesIcon,
    ClockIcon,
    StarIcon,
    ChevronRightIcon,
    XIcon,
  } from "@lucide/vue";
  import { ref, computed } from "vue";
  import { toast } from "vue-sonner";

  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Card, CardContent } from "@/components/ui/card";
  import { Input } from "@/components/ui/input";

  definePageMeta({
    title: "Audit Log",
    description: "Complete activity trail across the document system.",
    breadcrumbs: [{ label: "System", to: "/" }, { label: "Audit Log" }],
  });

  useHead({
    title: "Audit Log — Lumen DMS",
    meta: [
      {
        name: "description",
        content: "Track all document activities, changes, and system events.",
      },
    ],
  });

  interface AuditEntry {
    id: string;
    who: string;
    initials: string;
    action: string;
    target: string;
    detail: string;
    when: string;
    type: "create" | "edit" | "delete" | "share" | "view" | "sync" | "approve" | "upload";
  }

  const entries = ref<AuditEntry[]>([
    {
      id: "a1",
      who: "Aiko Tanaka",
      initials: "AT",
      action: "Uploaded",
      target: "Patent_Filing_US.pdf",
      detail: "Added to Engineering / Patents",
      when: "2m ago",
      type: "upload",
    },
    {
      id: "a2",
      who: "AI Assistant",
      initials: "AI",
      action: "Classified",
      target: "Acquisition_LOI.pdf",
      detail: "Confidence 98% — Legal category",
      when: "8m ago",
      type: "edit",
    },
    {
      id: "a3",
      who: "Marcus Bell",
      initials: "MB",
      action: "Approved",
      target: "MSA_Acme_Corp_v4.pdf",
      detail: "Workflow: Contract approval",
      when: "15m ago",
      type: "approve",
    },
    {
      id: "a4",
      who: "Sofia Reyes",
      initials: "SR",
      action: "Commented",
      target: "NDA_Helix_Labs.pdf",
      detail: "Section 4 — liability cap review",
      when: "1h ago",
      type: "edit",
    },
    {
      id: "a5",
      who: "SharePoint Sync",
      initials: "SP",
      action: "Synced",
      target: "Finance Hub",
      detail: "142 files synced from SharePoint",
      when: "34m ago",
      type: "sync",
    },
    {
      id: "a6",
      who: "Idris Khan",
      initials: "IK",
      action: "Shared",
      target: "Q3_Financial_Report.xlsx",
      detail: "Shared with Finance team (Editor)",
      when: "2h ago",
      type: "share",
    },
    {
      id: "a7",
      who: "Petra Lindqvist",
      initials: "PL",
      action: "Deleted",
      target: "Draft_Budget_v2.xlsx",
      detail: "Moved to trash",
      when: "3h ago",
      type: "delete",
    },
    {
      id: "a8",
      who: "Jonas Weber",
      initials: "JW",
      action: "Viewed",
      target: "Board_Minutes_May.docx",
      detail: "Opened from Recent",
      when: "4h ago",
      type: "view",
    },
    {
      id: "a9",
      who: "Naledi Okafor",
      initials: "NO",
      action: "Created",
      target: "Onboarding_Checklist.docx",
      detail: "New document from template",
      when: "5h ago",
      type: "create",
    },
    {
      id: "a10",
      who: "Aiko Tanaka",
      initials: "AT",
      action: "Edited",
      target: "Employee_Handbook_2026.docx",
      detail: "Updated PTO policy section",
      when: "6h ago",
      type: "edit",
    },
  ]);

  const query = ref("");
  const filterType = ref<string | null>(null);

  const typeColors: Record<string, string> = {
    upload: "bg-primary/10 text-primary",
    edit: "bg-info/10 text-info",
    delete: "bg-destructive/10 text-destructive",
    share: "bg-warning/10 text-warning",
    view: "bg-secondary/80 text-muted-foreground",
    sync: "bg-success/10 text-success",
    approve: "bg-emerald/10 text-emerald",
    create: "bg-fuchsia/10 text-fuchsia",
  };

  const typeLabels: Record<string, string> = {
    upload: "Upload",
    edit: "Edit",
    delete: "Delete",
    share: "Share",
    view: "View",
    sync: "Sync",
    approve: "Approve",
    create: "Create",
  };

  const filteredEntries = computed(() => {
    let r = entries.value;
    if (filterType.value) r = r.filter((e) => e.type === filterType.value);
    if (query.value.trim()) {
      const q = query.value.toLowerCase();
      r = r.filter(
        (e) =>
          e.who.toLowerCase().includes(q) ||
          e.target.toLowerCase().includes(q) ||
          e.action.toLowerCase().includes(q),
      );
    }
    return r;
  });
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center gap-2">
      <div class="relative min-w-[220px] flex-1">
        <SearchIcon
          class="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2"
        />
        <Input
          v-model="query"
          placeholder="Search activity…"
          class="bg-secondary/60 focus-visible:bg-card focus-visible:border-border h-9 rounded-lg border-transparent pl-9"
        />
        <button
          v-if="query"
          @click="query = ''"
          class="hover:bg-secondary text-muted-foreground hover:text-foreground absolute top-1/2 right-2 grid h-5 w-5 -translate-y-1/2 place-items-center rounded"
        >
          <XIcon class="h-3 w-3" />
        </button>
      </div>
      <div class="flex flex-wrap gap-1">
        <button
          v-for="t in Object.keys(typeLabels)"
          :key="t"
          @click="filterType = filterType === t ? null : t"
          :class="`rounded-full border px-2 py-1 text-[10px] transition ${
            filterType === t
              ? (typeColors[t] || 'bg-secondary text-muted-foreground') + ' border-current'
              : 'border-border text-muted-foreground hover:text-foreground hover:bg-secondary/60'
          }`"
        >
          {{ typeLabels[t] }}
        </button>
      </div>
    </div>

    <Card class="border-border shadow-elegant">
      <CardContent class="p-0">
        <div class="divide-border/50 divide-y">
          <div
            v-for="e in filteredEntries"
            :key="e.id"
            class="hover:bg-secondary/30 flex items-start gap-3 px-4 py-3 transition-colors"
          >
            <Avatar class="mt-0.5 h-8 w-8 shrink-0"
              ><AvatarFallback
                :class="`text-[10px] ${e.initials === 'AI' ? 'bg-primary/10 text-primary' : e.initials === 'SP' ? 'bg-info/10 text-info' : 'bg-secondary'}`"
                >{{ e.initials }}</AvatarFallback
              ></Avatar
            >
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-sm font-medium">{{ e.who }}</span>
                <span class="text-muted-foreground text-xs">{{ e.action }}</span>
                <span class="truncate text-sm font-medium">{{ e.target }}</span>
                <Badge
                  variant="secondary"
                  :class="`border-0 text-[10px] ${typeColors[e.type] || 'bg-secondary text-muted-foreground'}`"
                  >{{ typeLabels[e.type] }}</Badge
                >
              </div>
              <p class="text-muted-foreground mt-0.5 text-xs">{{ e.detail }}</p>
            </div>
            <span class="text-muted-foreground shrink-0 text-[11px]">{{ e.when }}</span>
          </div>
          <div
            v-if="filteredEntries.length === 0"
            class="text-muted-foreground p-8 text-center text-sm"
          >
            No activity matches your filter.
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
