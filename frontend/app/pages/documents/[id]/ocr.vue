<script setup lang="ts">
  import { ArrowLeftIcon, FileTextIcon, Loader2Icon } from "@lucide/vue";
  import { computed, ref, watch } from "vue";
  import { useRoute, useRouter } from "vue-router";

  import DocumentOcrViewer from "@/components/documents/DocumentOcrViewer.vue";
  import { Button } from "@/components/ui/button";
  import type { Document } from "@/types/api";

  definePageMeta({
    title: "Full OCR",
    breadcrumbs: [{ label: "Documents", to: "/documents" }, { label: "Full OCR" }],
  });

  const route = useRoute();
  const router = useRouter();
  const service = documentsService();
  const docId = computed(() => route.params.id as string);
  const document = ref<Document | null>(null);
  const loading = ref(true);

  const load = async () => {
    loading.value = true;
    document.value = await service.fetchDocument(docId.value);
    const ocr = await service.fetchOCR(docId.value, true);
    if (document.value && ocr) document.value.ocr = ocr;
    loading.value = false;
  };

  load();
  watch(docId, load);
</script>

<template>
  <div class="mx-auto flex max-w-6xl flex-col gap-5">
    <div class="flex items-center gap-3">
      <Button variant="ghost" size="icon" @click="router.push(`/documents/${docId}`)"
        ><ArrowLeftIcon
      /></Button>
      <div class="min-w-0">
        <h1 class="truncate text-lg font-semibold">Full OCR</h1>
        <p class="text-muted-foreground truncate text-sm">{{ document?.name }}</p>
      </div>
    </div>
    <div
      v-if="loading"
      class="text-muted-foreground flex min-h-72 items-center justify-center gap-2"
    >
      <Loader2Icon class="animate-spin" /> Loading complete extraction…
    </div>
    <DocumentOcrViewer v-else-if="document" :document="document" full-page @rerun="load" />
    <div
      v-else
      class="text-muted-foreground flex min-h-72 flex-col items-center justify-center gap-2"
    >
      <FileTextIcon />
      <p>Document not found.</p>
    </div>
  </div>
</template>
