<script setup lang="ts">
  import { ArrowRightIcon, ClipboardCheckIcon } from "@lucide/vue";
  import { computed } from "vue";

  import type { FormSubmission, FormTemplate } from "@/types/api";
  import { formatDateTime } from "@/utils/format";

  const props = defineProps<{
    submissions: FormSubmission[];
    templates: FormTemplate[];
    loading: boolean;
  }>();
  const names = computed(
    () => new Map(props.templates.map((template) => [template.id, template.name])),
  );
  const statusTone: Record<string, string> = {
    approved: "text-success",
    rejected: "text-destructive",
    returned: "text-warning",
    awaiting_approval: "text-info",
  };
</script>

<template>
  <section>
    <div class="mb-4 flex items-end justify-between gap-4">
      <div>
        <h2 class="text-lg font-semibold tracking-[-0.02em]">Request route</h2>
        <p class="text-muted-foreground mt-1 text-sm">
          Forms you submitted and their current desk.
        </p>
      </div>
      <NuxtLink
        to="/forms"
        class="text-muted-foreground hover:text-foreground flex items-center gap-1.5 text-sm"
        >Forms <ArrowRightIcon class="size-3.5"
      /></NuxtLink>
    </div>
    <div class="bg-card rounded-xl p-5 shadow-[0_16px_55px_-38px_oklch(0.18_0.03_250/0.45)]">
      <p v-if="loading" class="text-muted-foreground py-8 text-center text-sm">
        Checking request routes…
      </p>
      <div v-else-if="!submissions.length" class="py-8 text-center">
        <ClipboardCheckIcon class="text-muted-foreground/50 mx-auto size-6" />
        <p class="mt-3 text-sm font-medium">No active requests</p>
        <p class="text-muted-foreground mt-1 text-sm">
          Your submitted office forms will appear here.
        </p>
      </div>
      <ol v-else class="space-y-0">
        <li
          v-for="(submission, index) in submissions.slice(0, 6)"
          :key="submission.id"
          class="relative flex gap-4 pb-6 last:pb-0"
        >
          <span
            v-if="index < Math.min(submissions.length, 6) - 1"
            class="bg-border absolute top-7 bottom-0 left-[11px] w-px"
          />
          <span
            class="bg-muted border-card relative z-10 mt-0.5 size-6 shrink-0 rounded-full border-4"
          />
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium">
              {{ names.get(submission.template_id) || "Office request" }}
            </p>
            <div class="mt-1 flex items-center justify-between gap-3">
              <span :class="statusTone[submission.status]" class="text-xs capitalize">{{
                submission.status.replaceAll("_", " ")
              }}</span
              ><time
                class="text-muted-foreground font-mono text-[10px]"
                :datetime="submission.updated_at"
                >{{ formatDateTime(submission.updated_at) }}</time
              >
            </div>
          </div>
        </li>
      </ol>
    </div>
  </section>
</template>
