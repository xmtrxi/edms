<script setup lang="ts">
  import { CloudDownloadIcon, FileWarningIcon, Loader2Icon } from "@lucide/vue";
  import { onBeforeUnmount, shallowRef, watch } from "vue";

  import FileTypeIcon from "@/components/FileTypeIcon.vue";
  import { Button } from "@/components/ui/button";
  import PdfViewer from "@/components/viewer/PdfViewer.vue";
  import type { PreviewKind } from "@/types/api";
  import { fileTypeFromMime } from "@/utils/format";

  const props = defineProps<{
    documentId: string;
    name: string;
    mimeType: string;
    /** Changes when a new version is saved, so the viewer reloads. */
    versionId?: string;
    canDownload?: boolean;
  }>();
  const emit = defineEmits<{ download: [] }>();

  const loading = shallowRef(true);
  const kind = shallowRef<PreviewKind | "">("");
  const blob = shallowRef<Blob | null>(null);
  const url = shallowRef("");
  const text = shallowRef("");
  const failure = shallowRef("");

  const reset = () => {
    if (url.value) URL.revokeObjectURL(url.value);
    url.value = "";
    blob.value = null;
    text.value = "";
    failure.value = "";
    kind.value = "";
  };

  const load = async () => {
    reset();
    loading.value = true;
    const { $api } = useNuxtApp();
    try {
      const response = await $api.raw<Blob, "blob">(`/documents/${props.documentId}/preview`, {
        responseType: "blob",
      });
      const body = response._data as Blob;
      kind.value = (response.headers.get("X-Preview-Kind") as PreviewKind | null) ?? "";
      if (kind.value === "text") text.value = (await body.slice(0, 1_000_000).text()) || "";
      else if (kind.value === "pdf") blob.value = body;
      else url.value = URL.createObjectURL(body);
    } catch (err) {
      const status = (err as { response?: { status?: number } }).response?.status;
      failure.value =
        status === 415
          ? "There’s no preview for this file type yet."
          : "The preview couldn’t be loaded.";
    } finally {
      loading.value = false;
    }
  };

  watch(() => [props.documentId, props.versionId], load, { immediate: true });
  onBeforeUnmount(reset);
</script>

<template>
  <div class="bg-card flex h-[75dvh] min-h-96 flex-col overflow-hidden rounded-xl border">
    <div v-if="loading" class="grid flex-1 place-items-center">
      <div class="text-muted-foreground flex items-center gap-2 text-sm">
        <Loader2Icon class="size-4 animate-spin" /> Opening {{ name }}…
      </div>
    </div>

    <div
      v-else-if="failure"
      class="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center"
    >
      <FileTypeIcon :type="fileTypeFromMime(mimeType)" size="lg" />
      <p class="flex items-center gap-2 text-sm font-medium">
        <FileWarningIcon class="text-muted-foreground size-4" />{{ failure }}
      </p>
      <Button v-if="canDownload" variant="outline" size="sm" @click="emit('download')">
        <CloudDownloadIcon class="size-4" />Download instead
      </Button>
    </div>

    <PdfViewer
      v-else-if="kind === 'pdf'"
      :source="blob"
      @error="failure = 'This PDF couldn’t be displayed.'"
    />

    <div v-else-if="kind === 'image'" class="bg-muted/40 flex-1 overflow-auto p-4">
      <img :src="url" :alt="name" class="mx-auto max-w-full object-contain shadow-sm" />
    </div>

    <video v-else-if="kind === 'video'" :src="url" controls class="h-full w-full bg-black" />

    <div v-else-if="kind === 'audio'" class="grid flex-1 place-items-center p-8">
      <audio :src="url" controls class="w-full max-w-md" />
    </div>

    <pre
      v-else-if="kind === 'text'"
      class="flex-1 overflow-auto p-5 font-mono text-xs leading-relaxed whitespace-pre-wrap"
      >{{ text }}</pre>
  </div>
</template>
