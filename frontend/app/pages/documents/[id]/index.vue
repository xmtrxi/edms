<script setup lang="ts">
  import {
    ArchiveIcon,
    ArrowLeftIcon,
    CloudDownloadIcon,
    FileTextIcon,
    HistoryIcon,
    Loader2Icon,
    LockIcon,
    PenLineIcon,
    RefreshCwIcon,
    ScanSearchIcon,
    Share2Icon,
    SparklesIcon,
  } from "@lucide/vue";
  import { useIntervalFn } from "@vueuse/core";
  import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
  import { useRoute, useRouter } from "vue-router";
  import { toast } from "vue-sonner";

  import DocumentComments from "@/components/documents/DocumentComments.vue";
  import DocumentMetadataPanel from "@/components/documents/DocumentMetadataPanel.vue";
  import DocumentOcrViewer from "@/components/documents/DocumentOcrViewer.vue";
  import DocumentProcessingPanel from "@/components/documents/DocumentProcessingPanel.vue";
  import DocumentRoutingPanel from "@/components/documents/DocumentRoutingPanel.vue";
  import DocumentShareDialog from "@/components/documents/DocumentShareDialog.vue";
  import DocumentVersionHistory from "@/components/documents/DocumentVersionHistory.vue";
  import FileTypeIcon from "@/components/FileTypeIcon.vue";
  import EnvelopeCard from "@/components/signing/EnvelopeCard.vue";
  import StatusBadges from "@/components/StatusBadges.vue";
  import DocumentViewer from "@/components/viewer/DocumentViewer.vue";
  import type {
    CreateDocumentRoute,
    Document,
    DocumentContributors,
    DocumentRoute,
    DocumentVersion,
    ProcessingJob,
    RouteDecision,
    ShareRole,
    SignatureEnvelope,
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
  const sharing = sharingService();
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
  const signatures = signaturesService();
  const envelopes = ref<SignatureEnvelope[]>([]);
  const auth = useAuthStore();
  const viewers = usePresence(computed(() => (docId.value ? `document:${docId.value}` : "")));

  // Someone else saved a new version (in the editor, by upload or by
  // signing): refresh so this page shows it.
  const { subscribe } = useRealtime();
  let stopLive: (() => void) | undefined;
  onMounted(() => {
    stopLive = subscribe(`document:${docId.value}`, (event) => {
      if (event.type !== "version.created") return;
      void load();
      if (event.actor?.id !== auth.user?.id) toast.info("A new version was saved");
    });
  });
  onBeforeUnmount(() => stopLive?.());
  const noAccess = ref(false);
  const requestRole = ref<ShareRole>("viewer");
  const requestMessage = ref("");
  const requestSent = ref(false);
  const requesting = ref(false);

  const can = (permission: string) => Boolean(doc.value?.permissions?.includes(permission));
  const sensitivityLabel: Record<string, string> = {
    public: "Public",
    internal: "Internal",
    confidential: "Confidential",
    restricted: "Restricted",
  };

  // Labelling runs after OCR and only ever suggests a stricter label.
  const ai = aiService();
  const piiLabels: Record<string, string> = {
    card_number: "card numbers",
    kra_pin: "KRA PINs",
    national_id: "ID numbers",
    passport: "passport numbers",
    iban: "bank account numbers",
    email: "email addresses",
    phone: "phone numbers",
  };
  const suggestedSensitivity = computed(() => {
    const value = doc.value?.metadata?.suggested_sensitivity;
    return typeof value === "string" && value !== doc.value?.sensitivity ? value : "";
  });
  const detected = computed(() => {
    const found = (doc.value?.metadata?.detected_pii ?? {}) as Record<string, number>;
    return Object.entries(found)
      .filter(([kind]) => piiLabels[kind])
      .map(([kind, count]) => `${count} ${piiLabels[kind]}`);
  });
  const aiSummary = computed(() => {
    const value = doc.value?.metadata?.ai_summary;
    return typeof value === "string" ? value : "";
  });
  const labelling = ref(false);
  const applySuggestion = async () => {
    if (!doc.value || !suggestedSensitivity.value) return;
    const updated = await sharing.updateSettings(doc.value.id, {
      sensitivity: suggestedSensitivity.value as "confidential",
    });
    if (!updated) {
      toast.error(sharing.error.value || "Could not change the label");
      return;
    }
    toast.success(`Marked as ${sensitivityLabel[suggestedSensitivity.value]}`);
    await load();
  };
  const scan = async () => {
    if (!doc.value) return;
    labelling.value = true;
    const ok = await ai.label(doc.value.id);
    labelling.value = false;
    if (!ok) {
      toast.error(ai.error.value || "Could not scan this document");
      return;
    }
    await load();
    toast.success(
      detected.value.length ? `Found ${detected.value.join(", ")}` : "No sensitive data found",
    );
  };

  const load = async () => {
    loading.value = true;
    error.value = "";
    noAccess.value = false;
    doc.value = await service.fetchDocument(docId.value);
    noAccess.value = !doc.value && service.lastStatus.value === 403;
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
      envelopes.value = await signatures.forDocument(docId.value);
    }
    loading.value = false;
    if (!doc.value) error.value = service.error.value;
  };

  const sendAccessRequest = async () => {
    requesting.value = true;
    const created = await sharing.requestAccess(docId.value, {
      role: requestRole.value,
      message: requestMessage.value.trim(),
    });
    requesting.value = false;
    if (!created) {
      toast.error(sharing.error.value || "Could not send the request");
      return;
    }
    requestSent.value = true;
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
    if (service.error.value) toast.error(service.error.value);
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
      toast.error(service.error.value || "Could not re-run OCR");
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
    } else toast.error(service.error.value || "Could not restore version");
  };

  const handleCreateRoute = async (input: CreateDocumentRoute) => {
    if (!doc.value) return;
    routeBusy.value = true;
    const created = await service.createRoute(doc.value.id, input);
    routeBusy.value = false;
    if (created) {
      toast.success("Document route started");
      documentRoutes.value = [created, ...documentRoutes.value];
    } else toast.error(service.error.value || "Could not start route");
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
    } else toast.error(service.error.value || "Could not record routing decision");
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
      toast.error(service.error.value || "Could not archive document");
    }
  };
  const openShareDialog = () => {
    shareOpen.value = true;
  };
</script>

<template>
  <div v-if="loading" class="border-border rounded-xl border border-dashed p-12 text-center">
    <Loader2Icon class="text-primary mx-auto h-6 w-6 animate-spin" />
    <p class="text-muted-foreground mt-3 text-xs">Loading document…</p>
  </div>

  <div
    v-else-if="noAccess"
    class="bg-card mx-auto max-w-md rounded-2xl border p-8 text-center shadow-sm"
  >
    <div class="bg-muted mx-auto mb-4 grid size-12 place-items-center rounded-xl">
      <LockIcon class="text-muted-foreground size-6" />
    </div>
    <template v-if="requestSent">
      <h2 class="text-lg font-semibold">Request sent</h2>
      <p class="text-muted-foreground mt-2 text-sm">
        The people who manage this file will see your request. You’ll be able to open it once they
        approve.
      </p>
    </template>
    <template v-else>
      <h2 class="text-lg font-semibold">You need access</h2>
      <p class="text-muted-foreground mt-2 text-sm">
        This file belongs to your organization, but it hasn’t been shared with you.
      </p>
      <form class="mt-5 space-y-3 text-left" @submit.prevent="sendAccessRequest">
        <div class="space-y-1.5">
          <Label for="request-role">Access you need</Label>
          <Select v-model="requestRole">
            <SelectTrigger id="request-role" class="w-full"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="viewer">View</SelectItem>
              <SelectItem value="commenter">Comment</SelectItem>
              <SelectItem value="editor">Edit</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="space-y-1.5">
          <Label for="request-message"
            >Message <span class="text-muted-foreground">(optional)</span></Label
          >
          <Textarea
            id="request-message"
            v-model="requestMessage"
            rows="3"
            placeholder="Why you need it"
          />
        </div>
        <Button type="submit" class="w-full" :disabled="requesting">{{
          requesting ? "Sending…" : "Request access"
        }}</Button>
      </form>
    </template>
    <Button variant="ghost" size="sm" class="mt-3" @click="router.push('/documents')"
      >Back to documents</Button
    >
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
      <Badge
        v-if="doc.sensitivity && doc.sensitivity !== 'internal'"
        :variant="
          doc.sensitivity === 'restricted' || doc.sensitivity === 'confidential'
            ? 'destructive'
            : 'secondary'
        "
        class="gap-1"
        ><LockIcon v-if="doc.sensitivity === 'restricted'" class="size-3" />{{
          sensitivityLabel[doc.sensitivity]
        }}</Badge
      >
      <div class="ml-auto flex items-center gap-1.5">
        <div
          v-if="viewers.length"
          class="mr-1 flex items-center"
          :title="`Also here: ${viewers.map((item) => item.name).join(', ')}`"
        >
          <span
            v-for="person in viewers.slice(0, 3)"
            :key="person.id"
            class="bg-primary/10 text-primary -ml-1.5 grid size-7 place-items-center rounded-full border-2 border-white text-[10px] font-semibold first:ml-0"
            >{{ initialsOf(person.name) }}</span
          >
          <span class="sr-only">{{ viewers.length }} other people have this open</span>
        </div>
        <Button
          size="sm"
          :variant="can('share') ? 'default' : 'outline'"
          class="gap-1.5"
          @click="openShareDialog"
        >
          <Share2Icon class="h-3.5 w-3.5" /> {{ can("share") ? "Share" : "Who has access" }}
        </Button>
        <Button v-if="can('sign')" size="sm" variant="outline" class="gap-1.5" as-child>
          <NuxtLink :to="`/documents/${doc.id}/sign`">
            <PenLineIcon class="h-3.5 w-3.5" /> Request signatures
          </NuxtLink>
        </Button>
        <Button
          v-if="can('edit')"
          size="sm"
          variant="outline"
          class="gap-1.5"
          :disabled="redoingOcr"
          @click="handleRedoOCR"
        >
          <RefreshCwIcon :class="redoingOcr ? 'animate-spin' : ''" class="h-3.5 w-3.5" />
          Re-run OCR
        </Button>
        <Button
          v-if="can('edit')"
          size="sm"
          variant="outline"
          class="gap-1.5"
          :disabled="labelling"
          title="Look for personal and financial data"
          @click="scan"
        >
          <ScanSearchIcon class="h-3.5 w-3.5" :class="labelling && 'animate-pulse'" /> Scan
        </Button>
        <Button
          v-if="can('download')"
          size="sm"
          variant="outline"
          class="gap-1.5"
          @click="handleDownload"
        >
          <CloudDownloadIcon class="h-3.5 w-3.5" /> Download
        </Button>
        <AlertDialog v-if="can('delete')">
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

    <div
      v-if="suggestedSensitivity && can('share')"
      role="status"
      class="border-warning/40 bg-warning/10 mb-4 flex flex-wrap items-center gap-3 rounded-xl border px-4 py-3 text-sm"
    >
      <LockIcon class="size-4 shrink-0" />
      <span class="min-w-0 flex-1">
        This file contains {{ detected.join(", ") }}. Consider marking it
        {{ sensitivityLabel[suggestedSensitivity] }} so it can’t be shared publicly.
      </span>
      <Button size="sm" variant="outline" @click="applySuggestion">
        Mark {{ sensitivityLabel[suggestedSensitivity] }}
      </Button>
    </div>
    <div v-if="aiSummary" class="bg-card mb-4 rounded-xl border px-4 py-3 text-sm">
      <p
        class="text-muted-foreground mb-1 flex items-center gap-1.5 text-xs font-semibold tracking-wider uppercase"
      >
        <SparklesIcon class="size-3.5" /> Summary
      </p>
      <p class="leading-relaxed">{{ aiSummary }}</p>
    </div>

    <div class="grid grid-cols-1 gap-6 lg:grid-cols-[1fr_320px]">
      <div class="min-w-0 space-y-6">
        <DocumentViewer
          :document-id="doc.id"
          :name="doc.name"
          :mime-type="doc.mime_type"
          :version-id="doc.current_version_id"
          :can-download="can('download')"
          @download="handleDownload"
        />

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
        <section
          v-if="envelopes.length"
          id="signatures"
          class="border-border bg-card space-y-3 rounded-xl border p-4"
          aria-label="Signatures"
        >
          <p class="text-muted-foreground text-xs font-semibold tracking-wider uppercase">
            Signatures
          </p>
          <EnvelopeCard
            v-for="envelope in envelopes"
            :key="envelope.id"
            :envelope="envelope"
            @changed="load"
          />
        </section>
        <DocumentComments :document-id="doc.id" :can-comment="can('comment')" />
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
            Access
          </p>
          <dl class="space-y-2 text-xs">
            <div class="flex justify-between gap-3">
              <dt class="text-muted-foreground">Sensitivity</dt>
              <dd class="font-medium">{{ sensitivityLabel[doc.sensitivity ?? "internal"] }}</dd>
            </div>
            <div class="flex justify-between gap-3">
              <dt class="text-muted-foreground">Who can open it</dt>
              <dd class="text-right font-medium">
                {{
                  !doc.folder_id
                    ? "Owner and people it’s shared with"
                    : doc.inherit_permissions
                      ? "Follows its folder"
                      : "Its own access list"
                }}
              </dd>
            </div>
            <div class="flex justify-between gap-3">
              <dt class="text-muted-foreground">Your access</dt>
              <dd class="font-medium">
                {{
                  can("share")
                    ? "Can share"
                    : can("edit")
                      ? "Can edit"
                      : can("comment")
                        ? "Can comment"
                        : "Can view"
                }}
              </dd>
            </div>
          </dl>
          <Separator class="my-3" />
          <Button variant="outline" size="sm" class="w-full gap-1.5" @click="openShareDialog">
            <Share2Icon class="h-3.5 w-3.5" />
            {{ can("share") ? "Manage access" : "See who has access" }}
          </Button>
        </div>

        <DocumentMetadataPanel :document="doc" />
      </div>
    </div>
  </template>
  <DocumentShareDialog
    v-if="doc"
    v-model:open="shareOpen"
    :document-id="doc.id"
    :title="doc.name"
    @changed="load"
  />
</template>
