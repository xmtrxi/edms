<script setup lang="ts">
  import { RotateCwIcon } from "@lucide/vue";
  import { onMounted, shallowRef } from "vue";

  import { Button } from "@/components/ui/button";
  import VisitorAccessPanel from "@/components/visitors/VisitorAccessPanel.vue";
  import VisitorRequestForm from "@/components/visitors/VisitorRequestForm.vue";
  import VisitorRouteTimeline from "@/components/visitors/VisitorRouteTimeline.vue";
  import type { VisitorPortal } from "@/types/api";

  definePageMeta({ layout: "visitor" });
  useHead({ title: "Visitor request — EDMS" });

  const service = visitorsService();
  const portal = shallowRef<VisitorPortal | null>(null);
  const ready = shallowRef(false);
  const attachments = shallowRef<Array<{ id: string; name: string; size: number }>>([]);

  const loadPortal = async () => {
    portal.value = await service.portal();
    if (portal.value) attachments.value = await service.attachments();
    ready.value = true;
  };

  const submit = async (values: Record<string, unknown>) => {
    if (await service.submit(values)) await loadPortal();
  };

  const reset = () => {
    service.clearSession();
    portal.value = null;
  };
  const upload = async (file: File) => {
    const item = await service.uploadAttachment(file);
    if (item) attachments.value = [item, ...attachments.value];
  };

  onMounted(loadPortal);
</script>

<template>
  <div class="pt-5 sm:pt-12">
    <VisitorAccessPanel
      v-if="ready && !portal"
      :loading="service.loading.value"
      :error="service.error.value"
      @unlocked="loadPortal"
    />
    <div v-else-if="portal" class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_22rem]">
      <div>
        <div class="mb-5 flex items-center justify-between gap-4">
          <p class="text-muted-foreground text-sm">
            Welcome,
            <span class="text-foreground font-medium">{{ portal.request.visitor_name }}</span>
          </p>
          <Button variant="ghost" size="sm" class="gap-2" @click="reset"
            ><RotateCwIcon class="size-3.5" />Change request</Button
          >
        </div>
        <VisitorRequestForm
          :portal="portal"
          :loading="service.loading.value"
          :error="service.error.value"
          :attachments="attachments"
          @submit="submit"
          @upload="upload"
        />
      </div>
      <VisitorRouteTimeline :portal="portal" />
    </div>
    <div v-else class="grid min-h-96 place-items-center">
      <span class="text-muted-foreground text-sm">Opening secure visitor desk…</span>
    </div>
  </div>
</template>
