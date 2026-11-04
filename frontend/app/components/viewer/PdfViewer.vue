<script setup lang="ts">
  import { MinusIcon, PlusIcon } from "@lucide/vue";
  import type { PDFDocumentProxy } from "pdfjs-dist";
  import { onBeforeUnmount, shallowRef, watch } from "vue";

  import { Button } from "@/components/ui/button";
  import type { PdfPageSize } from "@/types/api";

  const props = withDefaults(defineProps<{ source: Blob | null; initialScale?: number }>(), {
    initialScale: 1.2,
  });
  const emit = defineEmits<{ loaded: [pages: PdfPageSize[]]; error: [message: string] }>();
  defineSlots<{ page(props: { index: number; size: PdfPageSize; scale: number }): unknown }>();

  const scale = shallowRef(props.initialScale);
  const pages = shallowRef<PdfPageSize[]>([]);
  const canvases = new Map<number, HTMLCanvasElement>();
  const rendered = new Map<number, number>();
  let pdf: PDFDocumentProxy | null = null;
  let observer: IntersectionObserver | null = null;

  // pdf.js is large, so it loads only when a PDF is actually shown.
  const loadLibrary = async () => {
    const pdfjs = await import("pdfjs-dist");
    const worker = await import("pdfjs-dist/build/pdf.worker.min.mjs?url");
    pdfjs.GlobalWorkerOptions.workerSrc = worker.default;
    return pdfjs;
  };

  const renderPage = async (index: number) => {
    const canvas = canvases.get(index);
    if (!pdf || !canvas || rendered.get(index) === scale.value) return;
    rendered.set(index, scale.value);
    const page = await pdf.getPage(index + 1);
    const viewport = page.getViewport({ scale: scale.value });
    const ratio = window.devicePixelRatio || 1;
    canvas.width = Math.floor(viewport.width * ratio);
    canvas.height = Math.floor(viewport.height * ratio);
    canvas.style.width = `${viewport.width}px`;
    canvas.style.height = `${viewport.height}px`;
    const context = canvas.getContext("2d");
    if (!context) return;
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
    await page.render({ canvas, canvasContext: context, viewport }).promise;
  };

  const observe = () => {
    observer?.disconnect();
    observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting)
            void renderPage(Number((entry.target as HTMLElement).dataset.index));
        }
      },
      { rootMargin: "600px 0px" },
    );
    for (const canvas of canvases.values()) observer.observe(canvas);
  };

  const setCanvas = (index: number, element: unknown) => {
    if (element instanceof HTMLCanvasElement) canvases.set(index, element);
  };

  watch(
    () => props.source,
    async (source) => {
      pages.value = [];
      canvases.clear();
      rendered.clear();
      await pdf?.destroy();
      pdf = null;
      if (!source) return;
      try {
        const pdfjs = await loadLibrary();
        pdf = await pdfjs.getDocument({ data: new Uint8Array(await source.arrayBuffer()) }).promise;
        const sizes: PdfPageSize[] = [];
        for (let i = 1; i <= pdf.numPages; i++) {
          const viewport = (await pdf.getPage(i)).getViewport({ scale: 1 });
          sizes.push({ width: viewport.width, height: viewport.height });
        }
        pages.value = sizes;
        emit("loaded", sizes);
        // Wait for the canvases to mount before watching them.
        await nextTick();
        observe();
      } catch (err) {
        emit("error", err instanceof Error ? err.message : "Could not open this PDF");
      }
    },
    { immediate: true },
  );

  watch(scale, async () => {
    rendered.clear();
    await nextTick();
    observe();
  });

  onBeforeUnmount(() => {
    observer?.disconnect();
    void pdf?.destroy();
  });

  const zoom = (delta: number) => {
    scale.value = Math.min(3, Math.max(0.5, Math.round((scale.value + delta) * 10) / 10));
  };
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div
      class="bg-card/90 sticky top-0 z-10 flex items-center gap-2 border-b px-3 py-2 backdrop-blur"
    >
      <span class="text-muted-foreground text-xs"
        >{{ pages.length }} page{{ pages.length === 1 ? "" : "s" }}</span
      >
      <div class="ml-auto flex items-center gap-1">
        <Button variant="ghost" size="icon" class="size-7" aria-label="Zoom out" @click="zoom(-0.2)"
          ><MinusIcon class="size-4"
        /></Button>
        <span class="w-12 text-center text-xs tabular-nums">{{ Math.round(scale * 100) }}%</span>
        <Button variant="ghost" size="icon" class="size-7" aria-label="Zoom in" @click="zoom(0.2)"
          ><PlusIcon class="size-4"
        /></Button>
      </div>
    </div>
    <div class="bg-muted/40 min-h-0 flex-1 overflow-auto p-4">
      <div
        v-for="(size, index) in pages"
        :key="index"
        class="relative mx-auto mb-4 bg-white shadow-sm"
        :style="{ width: `${size.width * scale}px`, height: `${size.height * scale}px` }"
      >
        <canvas
          :ref="(element) => setCanvas(index, element)"
          :data-index="index"
          class="block"
          :aria-label="`Page ${index + 1}`"
        />
        <div class="absolute inset-0">
          <slot name="page" :index="index" :size="size" :scale="scale" />
        </div>
      </div>
    </div>
  </div>
</template>
