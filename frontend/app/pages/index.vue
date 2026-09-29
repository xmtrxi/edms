<script setup lang="ts">
  import { UploadCloudIcon } from "@lucide/vue";
  import { computed, onMounted, shallowRef } from "vue";

  import UploadDialog from "@/components/documents/UploadDialog.vue";
  import { Button } from "@/components/ui/button";

  definePageMeta({
    title: "Office desk",
    description: "Live records, incoming requests, and the work moving through your office.",
  });
  useHead({ title: "Office desk — EDMS" });

  const workspace = workspaceService();
  const overview = shallowRef<import("@/types/api").DashboardOverview | null>(null);
  const uploadOpen = shallowRef(false);

  const refresh = async () => {
    overview.value = await workspace.overview();
  };

  const readyCount = computed(() => overview.value?.ready_documents ?? 0);
  const processingCount = computed(
    () =>
      (overview.value?.recent_documents ?? []).filter(
        (item) => item.status === "processing" || item.status === "uploaded",
      ).length,
  );
  const openSubmissionCount = computed(
    () =>
      (overview.value?.recent_submissions ?? []).filter(
        (item) => item.status === "awaiting_approval" || item.status === "returned",
      ).length,
  );

  onMounted(refresh);
</script>

<template>
  <Teleport to="#page-actions-target"
    ><Button size="sm" class="gap-2" @click="uploadOpen = true"
      ><UploadCloudIcon class="size-4" />Add document</Button
    ></Teleport
  >
  <div class="space-y-7">
    <DashboardOfficeOverview
      :total-documents="overview?.total_documents ?? 0"
      :ready-count="readyCount"
      :processing-count="processingCount"
      :open-submissions="openSubmissionCount"
      :loading="workspace.loading.value"
    />
    <div
      v-if="workspace.error.value"
      role="alert"
      class="border-destructive/30 bg-destructive/5 text-destructive rounded-lg border px-4 py-3 text-sm"
    >
      Live data could not be fully loaded.
      {{ workspace.error.value }}
    </div>
    <div class="grid gap-6 xl:grid-cols-[minmax(0,1.45fr)_minmax(20rem,0.75fr)]">
      <DashboardDocumentLedger
        :documents="overview?.recent_documents ?? []"
        :loading="workspace.loading.value"
      />
      <DashboardWorkQueue
        :submissions="overview?.recent_submissions ?? []"
        :templates="[]"
        :loading="workspace.loading.value"
      />
    </div>
  </div>
  <UploadDialog v-model:open="uploadOpen" @uploaded="refresh" />
</template>
