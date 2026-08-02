<script setup lang="ts">
  import {
    CloudIcon,
    AlertTriangleIcon,
    PauseIcon,
    RefreshCwIcon,
    Settings2Icon,
    CheckCircle2Icon,
    PlusIcon,
  } from "@lucide/vue";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Card, CardContent } from "@/components/ui/card";
  import { Progress } from "@/components/ui/progress";
  import { spSites } from "@/data/mock-data";

  definePageMeta({
    title: "SharePoint",
    description: "Connected sites, library sync, conflicts, and permissions mapping.",
    breadcrumbs: [{ label: "Collaboration", to: "/" }, { label: "SharePoint" }],
  });

  useHead({
    title: "SharePoint Libraries — Lumen DMS",
    meta: [
      {
        name: "description",
        content: "Manage SharePoint site connections, library syncs, and conflict resolution.",
      },
    ],
  });

  const dotMap: Record<string, string> = {
    healthy: "bg-success",
    syncing: "bg-info animate-pulse",
    conflict: "bg-destructive",
    paused: "bg-muted-foreground",
  };

  const statusLabel: Record<string, string> = {
    healthy: "Healthy",
    syncing: "Syncing",
    conflict: "Conflict",
    paused: "Paused",
  };

  const syncTone: Record<string, string> = {
    healthy: "bg-success/10 text-success",
    syncing: "bg-info/10 text-info",
    conflict: "bg-destructive/10 text-destructive",
    paused: "bg-secondary text-muted-foreground",
  };
</script>

<template>
  <Teleport to="#page-actions-target">
    <Button variant="outline" size="sm" class="gap-1.5">
      <RefreshCwIcon class="h-3.5 w-3.5" /> Sync all
    </Button>
    <Button size="sm" class="gap-1.5"> <PlusIcon class="h-4 w-4" /> Connect site </Button>
  </Teleport>

  <Card class="border-info/30 bg-info/5 shadow-elegant mb-4">
    <CardContent class="flex items-center gap-3 p-4">
      <div class="bg-info/10 text-info grid h-10 w-10 place-items-center rounded-lg">
        <CloudIcon class="h-5 w-5" />
      </div>
      <div class="flex-1">
        <p class="text-sm font-medium">All 5 sites connected &middot; 98% healthy</p>
        <p class="text-muted-foreground text-xs">
          34,455 documents indexed &middot; last full sync 2 minutes ago
        </p>
      </div>
      <Badge variant="secondary" class="bg-success/10 text-success border-0">
        <CheckCircle2Icon class="mr-1 h-3 w-3" /> Operational
      </Badge>
    </CardContent>
  </Card>

  <div class="grid grid-cols-1 gap-3">
    <Card
      v-for="s in spSites"
      :key="s.id"
      class="border-border shadow-elegant hover:shadow-glow transition-shadow"
    >
      <CardContent class="p-5">
        <div class="flex items-start gap-4">
          <div class="bg-info/10 text-info grid h-12 w-12 shrink-0 place-items-center rounded-xl">
            <CloudIcon class="h-5 w-5" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <h3 class="truncate text-sm font-semibold">{{ s.name }}</h3>
              <span :class="`h-1.5 w-1.5 rounded-full ${dotMap[s.syncStatus]}`" />
              <span class="text-muted-foreground text-[11px]">{{ statusLabel[s.syncStatus] }}</span>
            </div>
            <p class="text-muted-foreground truncate text-xs">{{ s.url }}</p>

            <div class="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
              <div class="border-border/60 bg-secondary/30 rounded-lg border px-3 py-2">
                <div class="text-muted-foreground text-[10px]">Libraries</div>
                <div class="text-sm font-medium tabular-nums">{{ s.libraries }}</div>
              </div>
              <div class="border-border/60 bg-secondary/30 rounded-lg border px-3 py-2">
                <div class="text-muted-foreground text-[10px]">Documents</div>
                <div class="text-sm font-medium tabular-nums">
                  {{ s.documents.toLocaleString() }}
                </div>
              </div>
              <div class="border-border/60 bg-secondary/30 rounded-lg border px-3 py-2">
                <div class="text-muted-foreground text-[10px]">Last sync</div>
                <div class="text-sm font-medium tabular-nums">{{ s.lastSync }}</div>
              </div>
              <div class="border-border/60 bg-secondary/30 rounded-lg border px-3 py-2">
                <div class="text-muted-foreground text-[10px]">Conflicts</div>
                <div
                  :class="`text-sm font-medium tabular-nums ${s.conflicts ? 'text-warning' : ''}`"
                >
                  {{ s.conflicts ?? 0 }}
                </div>
              </div>
            </div>

            <div v-if="s.syncStatus === 'syncing'" class="mt-3">
              <div class="text-muted-foreground mb-1 flex items-center justify-between text-[11px]">
                <span>Syncing 142 files</span><span>67%</span>
              </div>
              <Progress :model-value="67" class="h-1" />
            </div>

            <div
              v-if="s.syncStatus === 'conflict'"
              class="border-destructive/30 bg-destructive/5 mt-3 flex items-center gap-2 rounded-lg border px-3 py-2"
            >
              <AlertTriangleIcon class="text-destructive h-3.5 w-3.5 shrink-0" />
              <p class="text-destructive text-xs">
                {{ s.conflicts }} sync conflicts need your attention
              </p>
              <Button variant="outline" size="sm" class="ml-auto h-7 text-xs">Resolve</Button>
            </div>
          </div>

          <div class="flex shrink-0 flex-col gap-1.5">
            <Button variant="outline" size="sm" class="h-8 gap-1.5 text-xs">
              <template v-if="s.syncStatus === 'paused'">
                <RefreshCwIcon class="h-3 w-3" /> Resume
              </template>
              <template v-else> <PauseIcon class="h-3 w-3" /> Pause </template>
            </Button>
            <Button variant="ghost" size="sm" class="h-8 gap-1.5 text-xs">
              <Settings2Icon class="h-3 w-3" /> Configure
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
