<script setup lang="ts">
  import {
    FilesIcon,
    CogIcon,
    ScanLineIcon,
    SparklesIcon,
    WorkflowIcon,
    HardDriveIcon,
    SearchIcon,
    UploadCloudIcon,
    CloudIcon,
    ArrowUpRightIcon,
    ChevronRightIcon,
    ActivityIcon,
  } from "@lucide/vue";

  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Card, CardContent } from "@/components/ui/card";
  import { Progress } from "@/components/ui/progress";
  import {
    dashboardMetrics as m,
    activityFeed,
    workflows,
    insights,
    uploadSeries,
    processingSeries,
    spSites,
    documents,
  } from "@/data/mock-data";

  definePageMeta({
    title: "Dashboard",
    description:
      "Real-time overview of your document workspace, processing pipelines, and SharePoint sync.",
  });

  useHead({
    title: "Dashboard — Lumen DMS",
    meta: [
      {
        name: "description",
        content:
          "Real-time overview of your document workspace, AI processing, and SharePoint sync health.",
      },
      { property: "og:title", content: "Dashboard — Lumen DMS" },
      {
        property: "og:description",
        content: "Real-time overview of your document workspace and AI processing.",
      },
    ],
  });

  const metricTiles = [
    {
      icon: FilesIcon,
      tone: "text-primary",
      label: "Total documents",
      value: m.totalDocs.toLocaleString(),
      delta: "+184 today",
    },
    {
      icon: CogIcon,
      tone: "text-warning",
      label: "Processing queue",
      value: m.processingQueue,
      delta: "3 stuck",
      deltaTone: "warn" as const,
    },
    {
      icon: ScanLineIcon,
      tone: "text-info",
      label: "OCR pending",
      value: m.ocrPending,
      delta: "all under 1 min",
    },
    {
      icon: SparklesIcon,
      tone: "text-primary",
      label: "AI classified",
      value: m.aiClassified.toLocaleString(),
      delta: "+312 this week",
    },
    {
      icon: WorkflowIcon,
      tone: "text-success",
      label: "Open workflows",
      value: m.workflowsOpen,
      delta: "4 due today",
      deltaTone: "warn" as const,
    },
    {
      icon: SearchIcon,
      tone: "text-info",
      label: "Searches today",
      value: m.searchesToday.toLocaleString(),
      delta: "+22% WoW",
    },
  ];

  const syncDotMap: Record<string, string> = {
    healthy: "bg-success",
    syncing: "bg-info animate-pulse",
    conflict: "bg-destructive",
    paused: "bg-muted-foreground",
  };

  const chartMax = Math.max(...uploadSeries, ...processingSeries);
  const chartW = 100;
  const chartH = 100;

  function chartPaths(series: number[], idx: number) {
    const pts = series
      .map(
        (v, i) =>
          `${(i / (series.length - 1)) * chartW},${chartH - (v / chartMax) * (chartH - 10) - 4}`,
      )
      .join(" ");
    return { pts, idx, color: idx === 0 ? "var(--color-primary)" : "var(--color-info)" };
  }

  const storagePct = computed(() => (m.storageUsedGB / m.storageTotalGB) * 100);

  const storageBuckets = [
    { label: "PDFs", value: "312 GB", tone: "bg-rose-500" },
    { label: "Office", value: "218 GB", tone: "bg-blue-500" },
    { label: "Other", value: "82 GB", tone: "bg-emerald-500" },
  ];

  const visibleSites = computed(() => spSites.slice(0, 4));
  const recentActivities = computed(() => activityFeed.slice(0, 5));
  const recentWorkflows = computed(() => workflows.slice(0, 4));
  const topInsights = computed(() => insights.slice(0, 3));
  const recentDocs = computed(() => documents.slice(0, 6));
</script>

<template>
  <Teleport to="#page-actions-target">
    <Button variant="outline" size="sm" class="gap-1.5">
      <ActivityIcon class="h-3.5 w-3.5" /> Live
    </Button>
    <Button size="sm" class="gap-1.5"> <UploadCloudIcon class="h-4 w-4" /> Upload </Button>
  </Teleport>

  <div class="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
    <Card
      v-for="tile in metricTiles"
      :key="tile.label"
      class="border-border shadow-elegant hover:shadow-glow transition-shadow"
    >
      <CardContent class="p-4">
        <div class="flex items-center justify-between">
          <span class="text-muted-foreground text-xs">{{ tile.label }}</span>
          <component :is="tile.icon" :class="`h-4 w-4 ${tile.tone}`" />
        </div>
        <div class="mt-2 text-2xl font-semibold tracking-tight tabular-nums">{{ tile.value }}</div>
        <div
          v-if="tile.delta"
          :class="`mt-1 text-[11px] ${tile.deltaTone === 'warn' ? 'text-warning' : 'text-success'}`"
        >
          {{ tile.delta }}
        </div>
      </CardContent>
    </Card>
  </div>

  <div class="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-3">
    <Card class="border-border shadow-elegant lg:col-span-2">
      <CardContent class="p-5">
        <div class="mb-4 flex items-center justify-between">
          <div>
            <h3 class="text-sm font-semibold">Realtime processing activity</h3>
            <p class="text-muted-foreground text-xs">
              Uploads vs OCR/AI processing &middot; last 14 days
            </p>
          </div>
          <div class="flex items-center gap-3 text-xs">
            <span class="text-muted-foreground flex items-center gap-1.5">
              <span class="bg-primary h-2 w-2 rounded-full" /> Uploads
            </span>
            <span class="text-muted-foreground flex items-center gap-1.5">
              <span class="bg-info h-2 w-2 rounded-full" /> Processing
            </span>
          </div>
        </div>
        <div class="relative h-44 w-full">
          <svg
            :viewBox="`0 0 ${chartW} ${chartH}`"
            preserveAspectRatio="none"
            class="absolute inset-0 h-full w-full"
          >
            <g
              v-for="(cp, idx) in [chartPaths(uploadSeries, 0), chartPaths(processingSeries, 1)]"
              :key="idx"
            >
              <polyline
                :points="cp.pts"
                fill="none"
                :stroke="cp.color"
                stroke-width="1.5"
                vector-effect="non-scaling-stroke"
              />
              <polyline
                :points="`0,${chartH} ${cp.pts} ${chartW},${chartH}`"
                :fill="cp.color"
                opacity="0.08"
              />
            </g>
          </svg>
          <div
            class="text-muted-foreground absolute inset-x-0 bottom-0 flex justify-between px-1 text-[10px]"
          >
            <span /><span /><span /><span /><span>Now</span>
          </div>
        </div>
      </CardContent>
    </Card>

    <Card class="border-border shadow-elegant">
      <CardContent class="p-5">
        <div class="mb-4 flex items-center justify-between">
          <h3 class="flex items-center gap-1.5 text-sm font-semibold">
            <HardDriveIcon class="text-muted-foreground h-4 w-4" /> Storage
          </h3>
          <Badge variant="secondary" class="bg-secondary border-0 text-[10px]"
            >Enterprise plan</Badge
          >
        </div>
        <div class="flex items-baseline gap-1.5">
          <span class="text-3xl font-semibold tracking-tight tabular-nums">{{
            m.storageUsedGB
          }}</span>
          <span class="text-muted-foreground text-sm">/ {{ m.storageTotalGB }} GB</span>
        </div>
        <Progress :model-value="storagePct" class="mt-3 h-2" />
        <div class="mt-4 grid grid-cols-3 gap-2 text-center">
          <div
            v-for="b in storageBuckets"
            :key="b.label"
            class="border-border/60 rounded-lg border p-2"
          >
            <div class="text-muted-foreground flex items-center justify-center gap-1.5 text-[11px]">
              <span :class="`h-1.5 w-1.5 rounded-full ${b.tone}`" />{{ b.label }}
            </div>
            <div class="mt-1 text-sm font-medium tabular-nums">{{ b.value }}</div>
          </div>
        </div>
      </CardContent>
    </Card>

    <Card class="border-border shadow-elegant lg:col-span-2">
      <CardContent class="p-5">
        <div class="mb-4 flex items-center justify-between">
          <div>
            <h3 class="flex items-center gap-1.5 text-sm font-semibold">
              <CloudIcon class="text-info h-4 w-4" /> SharePoint sync health
            </h3>
            <p class="text-muted-foreground text-xs">
              {{ m.spHealthPct }}% healthy across {{ spSites.length }} connected sites
            </p>
          </div>
          <Button variant="ghost" size="sm" as-child class="text-muted-foreground gap-1">
            <NuxtLink to="/sharepoint">View all <ChevronRightIcon class="h-3.5 w-3.5" /></NuxtLink>
          </Button>
        </div>
        <div class="space-y-2">
          <div
            v-for="site in visibleSites"
            :key="site.id"
            class="border-border/60 hover:bg-secondary/40 flex items-center gap-3 rounded-xl border px-3 py-2.5 transition-colors"
          >
            <div class="bg-info/10 text-info grid h-9 w-9 place-items-center rounded-lg">
              <CloudIcon class="h-4 w-4" />
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span class="truncate text-sm font-medium">{{ site.name }}</span>
                <span
                  :class="`h-1.5 w-1.5 rounded-full ${syncDotMap[site.syncStatus]}`"
                  :aria-label="site.syncStatus"
                />
              </div>
              <p class="text-muted-foreground truncate text-[11px]">
                {{ site.url }} &middot; {{ site.documents.toLocaleString() }} docs
              </p>
            </div>
            <span class="text-muted-foreground shrink-0 text-[11px]">{{ site.lastSync }}</span>
          </div>
        </div>
      </CardContent>
    </Card>

    <Card class="border-border shadow-elegant">
      <CardContent class="p-5">
        <h3 class="mb-4 text-sm font-semibold">Recent activity</h3>
        <div class="space-y-3">
          <div v-for="a in recentActivities" :key="a.id" class="flex items-start gap-2.5">
            <Avatar class="h-7 w-7">
              <AvatarFallback class="bg-secondary text-[10px]">{{ a.initials }}</AvatarFallback>
            </Avatar>
            <div class="text-xs leading-snug">
              <span class="font-medium">{{ a.who }}</span>
              <span class="text-muted-foreground"> {{ a.action }} </span>
              <span class="font-medium">{{ a.target }}</span>
              <div class="text-muted-foreground text-[11px]">{{ a.when }}</div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <Card class="border-border shadow-elegant lg:col-span-2">
      <CardContent class="p-5">
        <div class="mb-4 flex items-center justify-between">
          <h3 class="flex items-center gap-1.5 text-sm font-semibold">
            <WorkflowIcon class="text-success h-4 w-4" /> Workflow approvals
          </h3>
          <Button variant="ghost" size="sm" as-child class="text-muted-foreground gap-1">
            <NuxtLink to="/workflows">Open <ChevronRightIcon class="h-3.5 w-3.5" /></NuxtLink>
          </Button>
        </div>
        <div class="space-y-2.5">
          <div
            v-for="w in recentWorkflows"
            :key="w.id"
            class="border-border/60 hover:bg-secondary/40 flex items-center gap-3 rounded-xl border p-3 transition-colors"
          >
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium">{{ w.title }}</p>
              <p class="text-muted-foreground truncate text-[11px]">
                {{ w.doc }} &middot; due {{ w.due }}
              </p>
            </div>
            <div class="hidden -space-x-1.5 sm:flex">
              <Avatar
                v-for="a in w.assignees.slice(0, 3)"
                :key="a.initials"
                class="ring-card h-6 w-6 ring-2"
              >
                <AvatarFallback class="bg-secondary text-[10px]">{{ a.initials }}</AvatarFallback>
              </Avatar>
            </div>
            <Badge
              variant="secondary"
              :class="`border-0 text-[10px] ${
                w.priority === 'high'
                  ? 'bg-destructive/10 text-destructive'
                  : w.priority === 'medium'
                    ? 'bg-warning/10 text-warning'
                    : 'bg-success/10 text-success'
              }`"
            >
              {{ w.priority }}
            </Badge>
          </div>
        </div>
      </CardContent>
    </Card>

    <Card class="border-border shadow-elegant relative overflow-hidden">
      <div
        class="from-primary/10 pointer-events-none absolute inset-0 bg-gradient-to-br via-transparent to-transparent"
      />
      <CardContent class="relative p-5">
        <div class="mb-4 flex items-center justify-between">
          <h3 class="flex items-center gap-1.5 text-sm font-semibold">
            <SparklesIcon class="text-primary h-4 w-4" /> AI insights
          </h3>
          <Button variant="ghost" size="sm" as-child class="text-muted-foreground gap-1">
            <NuxtLink to="/ai-insights">All <ChevronRightIcon class="h-3.5 w-3.5" /></NuxtLink>
          </Button>
        </div>
        <div class="space-y-3">
          <div
            v-for="i in topInsights"
            :key="i.id"
            class="border-border/60 bg-card/60 rounded-lg border p-2.5"
          >
            <p class="truncate text-xs font-medium">{{ i.title }}</p>
            <p class="text-muted-foreground mt-0.5 truncate text-[11px]">{{ i.doc }}</p>
            <div class="mt-1.5 flex items-center gap-1.5">
              <Progress :model-value="i.confidence * 100" class="h-1 flex-1" />
              <span class="text-muted-foreground text-[10px] tabular-nums"
                >{{ Math.round(i.confidence * 100) }}%</span
              >
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <Card class="border-border shadow-elegant lg:col-span-3">
      <CardContent class="p-5">
        <div class="mb-4 flex items-center justify-between">
          <h3 class="text-sm font-semibold">Recent uploads</h3>
          <Button variant="ghost" size="sm" as-child class="text-muted-foreground gap-1">
            <NuxtLink to="/recent">View all <ChevronRightIcon class="h-3.5 w-3.5" /></NuxtLink>
          </Button>
        </div>
        <div class="grid grid-cols-1 gap-2 md:grid-cols-2 lg:grid-cols-3">
          <NuxtLink
            v-for="d in recentDocs"
            :key="d.id"
            to="/documents"
            class="border-border/60 hover:bg-secondary/40 hover:border-primary/30 group flex items-center gap-3 rounded-xl border p-2.5 transition-all"
          >
            <FileTypeIcon :type="d.type" size="sm" />
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium">{{ d.name }}</p>
              <div class="mt-0.5 flex items-center gap-1.5">
                <span class="text-muted-foreground text-[11px]">{{ d.modified }}</span>
                <StatusBadges :status="d.status" />
              </div>
            </div>
            <ArrowUpRightIcon
              class="text-muted-foreground h-3.5 w-3.5 opacity-0 transition-opacity group-hover:opacity-100"
            />
          </NuxtLink>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
