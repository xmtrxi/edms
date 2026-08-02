<script setup lang="ts">
  import {
    ArchiveIcon,
    ArrowLeftIcon,
    CloudDownloadIcon,
    FileTextIcon,
    HistoryIcon,
    Loader2Icon,
    RefreshCwIcon,
    ScanLineIcon,
  } from "@lucide/vue";
  import { computed, ref, watch } from "vue";
  import { useRoute, useRouter } from "vue-router";
  import { toast } from "vue-sonner";

  import FileTypeIcon from "@/components/FileTypeIcon.vue";
  import StatusBadges from "@/components/StatusBadges.vue";
  import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
    AlertDialogTrigger,
  } from "@/components/ui/alert-dialog";
  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Separator } from "@/components/ui/separator";
  import type { Document } from "@/types/api";
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
  const docId = computed(() => route.params.id as string);
  const doc = ref<Document | null>(null);
  const loading = ref(true);
  const error = ref("");
  const archiving = ref(false);
  const redoingOcr = ref(false);

  const load = async () => {
    loading.value = true;
    error.value = "";
    doc.value = await service.fetchDocument(docId.value);
    loading.value = false;
    if (!doc.value) error.value = service.error;
  };

  load();
  watch(docId, load);

  const handleDownload = async () => {
    if (!doc.value) return;
    await service.download(doc.value.id, doc.value.name);
    if (service.error) toast.error(service.error);
  };

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

        <div class="border-border bg-card rounded-xl border p-5">
          <div class="mb-3 flex items-center gap-2">
            <ScanLineIcon class="text-primary h-4 w-4" />
            <h3 class="text-sm font-semibold">Extracted text</h3>
            <StatusBadges :status="doc.status" class="ml-auto" />
          </div>
          <div v-if="doc.ocr?.text" class="max-h-96 overflow-y-auto pr-1">
            <pre
              class="bg-secondary/40 text-foreground/80 rounded-lg p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap"
              >{{ doc.ocr.text }}</pre>
          </div>
          <div
            v-else-if="doc.status === 'processing'"
            class="text-muted-foreground flex items-center gap-2 text-xs"
          >
            <Loader2Icon class="h-3.5 w-3.5 animate-spin" /> OCR is still processing this document…
          </div>
          <div
            v-else
            class="text-muted-foreground rounded-xl border border-dashed p-6 text-center text-xs"
          >
            No extracted text yet.
            <Button size="sm" variant="outline" class="mt-2 gap-1.5" @click="handleRedoOCR">
              <RefreshCwIcon class="h-3.5 w-3.5" /> Run OCR
            </Button>
          </div>
        </div>

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

        <div
          v-if="Object.keys(doc.metadata ?? {}).length"
          class="border-border bg-card rounded-xl border p-4"
        >
          <p class="text-muted-foreground mb-2 text-xs font-semibold tracking-wider uppercase">
            Metadata
          </p>
          <div class="flex flex-wrap gap-1">
            <Badge
              v-for="(value, key) in doc.metadata"
              :key="key"
              variant="secondary"
              class="border-0 text-[10px]"
              >{{ key }}: {{ String(value) }}</Badge
            >
          </div>
        </div>
      </div>
    </div>
  </template>
</template>
