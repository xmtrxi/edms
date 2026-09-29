<script setup lang="ts">
  import {
    ArchiveIcon,
    ArrowLeftIcon,
    CloudDownloadIcon,
    FileTextIcon,
    HistoryIcon,
    Loader2Icon,
    PenLineIcon,
    RefreshCwIcon,
    Share2Icon,
  } from "@lucide/vue";
  import { useIntervalFn } from "@vueuse/core";
  import { computed, ref, watch } from "vue";
  import { useRoute, useRouter } from "vue-router";
  import { toast } from "vue-sonner";

  import DocumentMetadataPanel from "@/components/documents/DocumentMetadataPanel.vue";
  import DocumentOcrViewer from "@/components/documents/DocumentOcrViewer.vue";
  import DocumentProcessingPanel from "@/components/documents/DocumentProcessingPanel.vue";
  import DocumentRoutingPanel from "@/components/documents/DocumentRoutingPanel.vue";
  import DocumentShareDialog from "@/components/documents/DocumentShareDialog.vue";
  import DocumentSignDialog from "@/components/documents/DocumentSignDialog.vue";
  import DocumentVersionHistory from "@/components/documents/DocumentVersionHistory.vue";
  import FileTypeIcon from "@/components/FileTypeIcon.vue";
  import StatusBadges from "@/components/StatusBadges.vue";
  import { workspaceService } from "@/services/workspace";
  import type {
    CreateDocumentRoute,
    Document,
    DocumentContributors,
    DocumentRoute,
    DocumentShare,
    DocumentVersion,
    ProcessingJob,
    RouteDecision,
  } from "@/types/api";
  import { formatBytes, formatDateTime, fileTypeFromMime, initialsOf } from "@/utils/format";

  definePageMeta({
    title: "Document",
    breadcrumbs: [{ label: "Documents", to: "/documents" }, { label: "Document" }],
  });

  useHead({
    title: "Document — Lumen DMS",
  });

  const route = useRoute();
  const router = useRouter();

  const service = documentsService();
  const workspace = workspaceService();
  const docId = computed(() => route.params.id as string);
  const doc = ref<Document | null>(null);
  const versions = ref<DocumentVersion[]>([]);
  const contributors = ref<DocumentContributors | null>(null);
  const documentRoutes = ref<DocumentRoute[]>([]);
  const processingJob = ref<ProcessingJob | null>(null);
  const refreshingProcessingJob = ref(false);
  const loading = ref(true);
  const error = ref("");
  const archiving = ref(false);
  const redoingOcr = ref(false);
  const versionBusy = ref(false);
  const routeBusy = ref(false);
  const shareOpen = ref(false);
  const signOpen = ref(false);
  const shareBusy = ref(false);
  const signBusy = ref(false);
  const documentShares = ref<DocumentShare[]>([]);
  const createdShareToken = ref("");

  const load = async () => {
    loading.value = true;
    error.value = "";
    doc.value = await service.fetchDocument(docId.value);
    if (doc.value) {
      const [loadedVersions, loadedContributors, loadedRoutes] = await Promise.all([
        service.fetchVersions(docId.value),
        service.fetchContributors(docId.value),
        service.fetchRoutes(docId.value),
      ]);
      versions.value = loadedVersions;
      contributors.value = loadedContributors;
      documentRoutes.value = loadedRoutes;
      processingJob.value = await service.fetchProcessingJob(docId.value);
    }
    loading.value = false;
    if (!doc.value) error.value = service.error;
  };

  load();
  watch(docId, load);

  const refreshProcessingJob = async () => {
    if (!doc.value || refreshingProcessingJob.value) return;
    refreshingProcessingJob.value = true;
    processingJob.value = await service.fetchProcessingJob(docId.value);
    const refreshedDocument = await service.fetchDocument(docId.value);
    if (refreshedDocument) doc.value = refreshedDocument;
    refreshingProcessingJob.value = false;
  };

  const { pause: pauseProcessingRefresh, resume: resumeProcessingRefresh } = useIntervalFn(
    refreshProcessingJob,
    3_000,
    { immediate: false },
  );

  watch(
    () => doc.value?.status,
    (status) => {
      if (status === "uploaded" || status === "processing") resumeProcessingRefresh();
      else pauseProcessingRefresh();
    },
    { immediate: true },
  );

  const handleDownload = async () => {
    if (!doc.value) return;
    await service.download(doc.value.id, doc.value.name);
    if (service.error) toast.error(service.error);
  };

  const openFullOcr = () => router.push(`/documents/${docId.value}/ocr`);

  const handleRedoOCR = async () => {
    if (!doc.value) return;
    redoingOcr.value = true;
    const ok = await service.redoOCR(doc.value.id);
    redoingOcr.value = false;
    if (ok) {
      toast.success("OCR re-run queued");
      await load();
    } else {
      toast.error(service.error || "Could not re-run OCR");
    }
  };

  const handleRestoreVersion = async (version: DocumentVersion) => {
    if (!doc.value) return;
    versionBusy.value = true;
    const restored = await service.restoreVersion(
      doc.value.id,
      version.id,
      `Restored version ${version.version}`,
    );
    versionBusy.value = false;
    if (restored) {
      toast.success(`Version ${version.version} restored as a new version`);
      await load();
    } else toast.error(service.error || "Could not restore version");
  };

  const handleCreateRoute = async (input: CreateDocumentRoute) => {
    if (!doc.value) return;
    routeBusy.value = true;
    const created = await service.createRoute(doc.value.id, input);
    routeBusy.value = false;
    if (created) {
      toast.success("Document route started");
      documentRoutes.value = [created, ...documentRoutes.value];
    } else toast.error(service.error || "Could not start route");
  };

  const handleRouteDecision = async (routeId: string, input: RouteDecision) => {
    if (!doc.value) return;
    routeBusy.value = true;
    const updated = await service.decideRoute(doc.value.id, routeId, input);
    routeBusy.value = false;
    if (updated) {
      toast.success("Routing decision recorded");
      documentRoutes.value = documentRoutes.value.map((route) =>
        route.id === updated.id ? updated : route,
      );
    } else toast.error(service.error || "Could not record routing decision");
  };

  const handleArchive = async () => {
    if (!doc.value) return;
    archiving.value = true;
    const ok = await service.archive(doc.value.id);
    archiving.value = false;
    if (ok) {
      toast.success("Document archived");
      router.push("/documents");
    } else {
      toast.error(service.error || "Could not archive document");
    }
  };
  const openShareDialog = async () => {
    shareOpen.value = true;
    createdShareToken.value = "";
    shareBusy.value = true;
    documentShares.value = await workspace.shares(docId.value);
    shareBusy.value = false;
  };
  const createShare = async (input: {
    email: string;
    permission: "view" | "download";
    expires_at?: string;
  }) => {
    shareBusy.value = true;
    const created = await workspace.share(docId.value, input);
    shareBusy.value = false;
    if (!created) {
      toast.error(workspace.error.value || "Could not create share");
      return;
    }
    documentShares.value = [created.share, ...documentShares.value];
    createdShareToken.value = created.token;
    toast.success("Secure share created");
  };
  const revokeShare = async (shareId: string) => {
    shareBusy.value = true;
    await workspace.revokeShare(docId.value, shareId);
    shareBusy.value = false;
    if (workspace.error.value) {
      toast.error(workspace.error.value);
      return;
    }
    documentShares.value = await workspace.shares(docId.value);
    toast.success("Share revoked");
  };
  const copyShareToken = async (token: string) => {
    await navigator.clipboard.writeText(token);
    toast.success("Share token copied");
  };
  const requestSignature = async (input: {
    title: string;
    recipients: Array<{ email: string; name: string; role: string }>;
  }) => {
    signBusy.value = true;
    const request = await service.requestSignature(docId.value, input);
    signBusy.value = false;
    if (!request) {
      toast.error(service.error || "Could not create signature request");
      return;
    }
    signOpen.value = false;
    toast.success("Signature request sent");
  };
</script>

<template>
  <div v-if="loading" class="border-border rounded-xl border border-dashed p-12 text-center">
    <Loader2Icon class="text-primary mx-auto h-6 w-6 animate-spin" />
    <p class="text-muted-foreground mt-3 text-xs">Loading document…</p>
  </div>

  <div v-else-if="!doc" class="border-border rounded-xl border border-dashed p-12 text-center">
    <FileTextIcon class="text-muted-foreground/40 mx-auto h-8 w-8" />
    <p class="mt-2 text-sm font-medium">Document not found</p>
    <p v-if="error" class="text-muted-foreground mt-1 text-xs">{{ error }}</p>
    <Button variant="outline" size="sm" class="mt-3" @click="router.push('/documents')"
      >Back to documents</Button
    >
  </div>

  <template v-else>
    <div class="mb-4 flex flex-wrap items-center gap-2">
      <Button
        variant="ghost"
        size="icon"
        class="h-8 w-8 shrink-0"
        @click="router.push('/documents')"
      >
        <ArrowLeftIcon class="h-4 w-4" />
      </Button>
      <Separator orientation="vertical" class="h-5" />
      <FileTypeIcon :type="fileTypeFromMime(doc.mime_type)" class="h-5 w-5 shrink-0" />
      <h2 class="min-w-0 truncate text-lg font-semibold">{{ doc.name }}</h2>
      <StatusBadges :status="doc.status" />
      <div class="ml-auto flex items-center gap-1.5">
        <Button size="sm" variant="outline" class="gap-1.5" @click="openShareDialog">
          <Share2Icon class="h-3.5 w-3.5" /> Share
        </Button>
        <Button size="sm" variant="outline" class="gap-1.5" @click="signOpen = true">
          <PenLineIcon class="h-3.5 w-3.5" /> Sign
        </Button>
        <Button
          size="sm"
          variant="outline"
          class="gap-1.5"
          :disabled="redoingOcr"
          @click="handleRedoOCR"
        >
          <RefreshCwIcon :class="redoingOcr ? 'animate-spin' : ''" class="h-3.5 w-3.5" />
          Re-run OCR
        </Button>
        <Button size="sm" variant="outline" class="gap-1.5" @click="handleDownload">
          <CloudDownloadIcon class="h-3.5 w-3.5" /> Download
        </Button>
        <AlertDialog>
          <AlertDialogTrigger as-child>
            <Button
              size="sm"
              variant="outline"
              class="text-destructive gap-1.5"
              :disabled="archiving"
            >
              <ArchiveIcon class="h-3.5 w-3.5" /> Archive
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Archive this document?</AlertDialogTitle>
              <AlertDialogDescription>
                “{{ doc.name }}” will be archived and hidden from your document list. You can
                restore it later from the archive.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction @click="handleArchive">Archive</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-6 lg:grid-cols-[1fr_320px]">
      <div class="min-w-0 space-y-6">
        <div
          class="border-border bg-card shadow-elegant relative overflow-hidden rounded-xl border"
        >
          <div
            class="border-border bg-secondary/40 flex items-center justify-between border-b px-3 py-2"
          >
            <div class="flex min-w-0 items-center gap-2">
              <FileTypeIcon :type="fileTypeFromMime(doc.mime_type)" size="sm" />
              <span class="text-muted-foreground truncate text-[11px]">{{ doc.name }}</span>
            </div>
            <span class="text-muted-foreground shrink-0 text-[10px] uppercase">{{
              fileTypeFromMime(doc.mime_type)
            }}</span>
          </div>
          <div class="bg-card p-5 sm:p-8">
            <div class="flex justify-center">
              <FileTypeIcon :type="fileTypeFromMime(doc.mime_type)" size="lg" />
            </div>
            <h4 class="mt-4 text-center text-base font-semibold tracking-tight sm:text-lg">
              {{ doc.name }}
            </h4>
            <p class="text-muted-foreground mt-1 text-center text-xs">
              {{ doc.original_name }} · {{ formatBytes(doc.size) }}
            </p>
            <div class="mt-4 flex flex-wrap items-center justify-center gap-2">
              <Button size="sm" variant="outline" class="gap-1.5" @click="handleDownload">
                <CloudDownloadIcon class="h-3.5 w-3.5" /> Download
              </Button>
            </div>
          </div>
        </div>

        <DocumentOcrViewer
          :document="doc"
          :busy="redoingOcr"
          @rerun="handleRedoOCR"
          @view-full="openFullOcr"
        />

        <DocumentProcessingPanel :job="processingJob" :refreshing="refreshingProcessingJob" />

        <DocumentVersionHistory
          :versions="versions"
          :contributors="contributors"
          :current-version-id="doc.current_version_id"
          :busy="versionBusy"
          @download="service.downloadVersion(doc.id, $event)"
          @restore="handleRestoreVersion"
        />

        <DocumentRoutingPanel
          :routes="documentRoutes"
          :busy="routeBusy"
          @create="handleCreateRoute"
          @decide="handleRouteDecision"
        />

        <div class="border-border bg-card rounded-xl border p-5">
          <div class="mb-3 flex items-center gap-2">
            <HistoryIcon class="text-primary h-4 w-4" />
            <h3 class="text-sm font-semibold">Details</h3>
          </div>
          <dl class="grid grid-cols-2 gap-x-4 gap-y-3 text-xs sm:grid-cols-3">
            <div>
              <dt class="text-muted-foreground">Status</dt>
              <dd class="mt-1"><StatusBadges :status="doc.status" /></dd>
            </div>
            <div>
              <dt class="text-muted-foreground">Original name</dt>
              <dd class="mt-1 font-medium">{{ doc.original_name }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">Type</dt>
              <dd class="mt-1 font-medium uppercase">{{ doc.mime_type }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">Size</dt>
              <dd class="mt-1 font-medium">{{ formatBytes(doc.size) }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">Created</dt>
              <dd class="mt-1 font-medium">{{ formatDateTime(doc.created_at) }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">Updated</dt>
              <dd class="mt-1 font-medium">{{ formatDateTime(doc.updated_at) }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">Versions</dt>
              <dd class="mt-1 font-medium">{{ doc.version_count }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">Editors</dt>
              <dd class="mt-1 font-medium">{{ contributors?.unique_editor_count ?? 0 }}</dd>
            </div>
          </dl>
        </div>
      </div>

      <div class="space-y-4">
        <div class="border-border bg-card rounded-xl border p-4">
          <p class="text-muted-foreground mb-3 text-xs font-semibold tracking-wider uppercase">
            Owner
          </p>
          <div class="flex items-center gap-2.5">
            <Avatar class="h-8 w-8">
              <AvatarFallback class="bg-primary/10 text-primary text-xs font-medium">{{
                initialsOf(doc.owner?.name)
              }}</AvatarFallback>
            </Avatar>
            <div class="min-w-0">
              <p class="truncate text-sm font-medium">{{ doc.owner?.name ?? "Unknown" }}</p>
              <p class="text-muted-foreground truncate text-xs">{{ doc.owner?.email }}</p>
            </div>
          </div>
        </div>

        <div class="border-border bg-card rounded-xl border p-4">
          <p class="text-muted-foreground mb-3 text-xs font-semibold tracking-wider uppercase">
            OCR
          </p>
          <div v-if="doc.ocr" class="space-y-2 text-xs">
            <div class="flex justify-between">
              <span class="text-muted-foreground">Pages</span>
              <span class="font-medium">{{ doc.ocr.pages ?? "—" }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-muted-foreground">Language</span>
              <span class="font-medium">{{ doc.ocr.language || "—" }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-muted-foreground">Characters</span>
              <span class="font-medium">{{ doc.ocr.text.length.toLocaleString() }}</span>
            </div>
          </div>
          <p v-else class="text-muted-foreground text-xs">No OCR data for this document.</p>
          <Separator class="my-3" />
          <Button
            variant="outline"
            size="sm"
            class="w-full gap-1.5"
            :disabled="redoingOcr"
            @click="handleRedoOCR"
          >
            <RefreshCwIcon :class="redoingOcr ? 'animate-spin' : ''" class="h-3.5 w-3.5" />
            Re-run OCR
          </Button>
        </div>

        <div class="border-border bg-card rounded-xl border p-4">
          <p class="text-muted-foreground mb-3 text-xs font-semibold tracking-wider uppercase">
            Shared with
          </p>
          <div v-if="doc.shared_with?.length" class="space-y-2">
            <div v-for="u in doc.shared_with" :key="u.id" class="flex items-center gap-2">
              <Avatar class="h-6 w-6">
                <AvatarFallback class="bg-secondary text-[10px]">{{
                  initialsOf(u.name)
                }}</AvatarFallback>
              </Avatar>
              <span class="text-xs font-medium">{{ u.name }}</span>
            </div>
          </div>
          <p v-else class="text-muted-foreground text-xs">Not shared.</p>
        </div>

        <DocumentMetadataPanel :document="doc" />
      </div>
    </div>
  </template>
  <DocumentShareDialog
    v-if="doc"
    v-model:open="shareOpen"
    :title="doc.name"
    :shares="documentShares"
    :created-token="createdShareToken"
    :busy="shareBusy"
    :error="workspace.error.value"
    @submit="createShare"
    @revoke="revokeShare"
    @copy="copyShareToken"
  />
  <DocumentSignDialog
    v-if="doc"
    v-model:open="signOpen"
    :document-title="doc.name"
    :busy="signBusy"
    :error="service.error"
    @submit="requestSignature"
  />
</template>
