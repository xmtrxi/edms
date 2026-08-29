<script setup lang="ts">
  import { parseMarkdown } from "comark";
  import { Loader2Icon, RefreshCwIcon, ScanTextIcon } from "lucide-vue-next";

  import StatusBadges from "@/components/StatusBadges.vue";
  import { Button } from "@/components/ui/button";
  import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
  import type { Document } from "@/types/api";

  const props = defineProps<{
    document: Document;
    busy?: boolean;
    fullPage?: boolean;
  }>();

  const emit = defineEmits<{
    rerun: [];
    viewFull: [];
  }>();

  const activeTab = shallowRef("markdown");
  const markdown = computed(() => props.document.ocr?.markdown.replace(/\\n/g, "\n") ?? "");
  const result = await parseMarkdown(markdown.value);

  const plainText = computed(() => props.document.ocr?.text?.trim() ?? "");
  const hasOcr = computed(() => Boolean(markdown.value || plainText.value));
  const isProcessing = computed(() => props.document.status === "processing");
  const contentHeightClass = computed(() =>
    props.fullPage ? "max-h-[calc(100vh-15rem)]" : "max-h-[34rem]",
  );
</script>

<template>
  <section class="border-border bg-card overflow-hidden rounded-xl border">
    <div class="border-border flex flex-wrap items-center gap-3 border-b px-5 py-4">
      <div class="bg-primary/10 text-primary grid h-9 w-9 place-items-center rounded-lg">
        <ScanTextIcon class="h-4 w-4" />
      </div>
      <div class="min-w-0 flex-1">
        <h3 class="text-sm font-semibold">Extracted content</h3>
      </div>
      <StatusBadges :status="document.status" />
      <Button size="sm" variant="outline" class="gap-1.5" :disabled="busy" @click="emit('rerun')">
        <RefreshCwIcon :class="busy ? 'animate-spin' : ''" class="h-3.5 w-3.5" />
        Re-run
      </Button>
      <Button v-if="!fullPage && hasOcr" size="sm" @click="emit('viewFull')">View full OCR</Button>
    </div>
    <Tabs v-if="hasOcr" v-model="activeTab" class="min-w-0">
      <div class="border-border border-b px-5 py-3">
        <TabsList class="h-8">
          <TabsTrigger value="markdown" class="text-xs">Markdown</TabsTrigger>
          <TabsTrigger value="text" class="text-xs">Text</TabsTrigger>
        </TabsList>
      </div>

      <TabsContent value="markdown" class="m-0">
        <div
          :class="[
            'prose lg:prose-lg dark:prose-invert max-w-none overflow-auto px-6 py-6',
            contentHeightClass,
          ]"
        >
          <Markdown v-if="markdown" :value="result" />
          <p v-else class="text-muted-foreground text-sm">No Markdown content is available.</p>
        </div>
      </TabsContent>

      <TabsContent value="text" class="m-0">
        <pre
          :class="[
            'bg-secondary/30 overflow-auto p-5 font-mono text-xs leading-6 whitespace-pre-wrap',
            contentHeightClass,
          ]"
          >{{ plainText }}</pre>
      </TabsContent>
    </Tabs>

    <div v-else-if="isProcessing" class="flex items-center gap-3 p-5">
      <Loader2Icon class="text-primary h-4 w-4 animate-spin" />
      <div>
        <p class="text-sm font-medium">OCR is running in the queue</p>
        <p class="text-muted-foreground text-xs">
          The document will become searchable as soon as Docling finishes.
        </p>
      </div>
    </div>

    <div v-else class="p-6 text-center">
      <div class="bg-secondary mx-auto grid h-10 w-10 place-items-center rounded-lg">
        <ScanTextIcon class="text-muted-foreground h-4 w-4" />
      </div>
      <p class="mt-3 text-sm font-medium">No OCR output yet</p>
      <Button size="sm" variant="outline" class="mt-4 gap-1.5" @click="emit('rerun')">
        <RefreshCwIcon class="h-3.5 w-3.5" />
        Run OCR
      </Button>
    </div>
  </section>
</template>

<style scoped></style>
