<script setup lang="ts">
  import { CheckCircle2Icon, FileImageIcon, FingerprintIcon, ScanTextIcon } from "lucide-vue-next";
  import { computed } from "vue";

  import { Badge } from "@/components/ui/badge";
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
  import type { Document } from "@/types/api";

  const props = defineProps<{ document: Document }>();
  const metadata = computed(() => props.document.metadata ?? {});
  const imageCount = computed(() =>
    Array.isArray(metadata.value.ocr_image_paths) ? metadata.value.ocr_image_paths.length : 0,
  );
  const checksum = computed(() => String(metadata.value.sha256 ?? ""));
  const shortChecksum = computed(() =>
    checksum.value ? `${checksum.value.slice(0, 12)}…${checksum.value.slice(-8)}` : "Not available",
  );
</script>

<template>
  <Card size="sm">
    <CardHeader class="gap-1 px-5">
      <CardTitle class="text-sm">Document intelligence</CardTitle>
      <CardDescription
        >Useful extraction information, without internal storage paths.</CardDescription
      >
    </CardHeader>
    <CardContent class="flex flex-col gap-3 px-5">
      <div class="flex items-center justify-between gap-3 text-xs">
        <span class="text-muted-foreground">Format</span
        ><Badge variant="secondary">{{
          String(metadata.extension ?? document.mime_type.split("/").pop() ?? "—").toUpperCase()
        }}</Badge>
      </div>
      <div class="flex items-center justify-between gap-3 text-xs">
        <span class="text-muted-foreground">Extraction</span
        ><span class="flex items-center gap-1 font-medium"
          ><CheckCircle2Icon class="text-success size-3.5" />
          {{ metadata.extracted ? "Complete" : "Pending" }}</span
        >
      </div>
      <div class="flex items-center justify-between gap-3 text-xs">
        <span class="text-muted-foreground">OCR engine</span
        ><span class="flex items-center gap-1 font-medium"
          ><ScanTextIcon class="size-3.5" /> {{ metadata.ocr_engine ?? "—" }}</span
        >
      </div>
      <div class="flex items-center justify-between gap-3 text-xs">
        <span class="text-muted-foreground">Extracted images</span
        ><span class="flex items-center gap-1 font-medium"
          ><FileImageIcon class="size-3.5" /> {{ imageCount }}</span
        >
      </div>
      <div class="flex items-center justify-between gap-3 text-xs">
        <span class="text-muted-foreground">Checksum</span
        ><span class="flex items-center gap-1 font-mono text-[11px]"
          ><FingerprintIcon class="size-3.5" /> {{ shortChecksum }}</span
        >
      </div>
    </CardContent>
  </Card>
</template>
