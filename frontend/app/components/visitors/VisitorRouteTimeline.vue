<script setup lang="ts">
  import { CheckIcon, Clock3Icon, LockKeyholeIcon } from "@lucide/vue";
  import { computed } from "vue";

  import type { VisitorPortal } from "@/types/api";

  const props = defineProps<{ portal: VisitorPortal }>();
  const stages = computed(() => [
    {
      id: "received",
      name: "Information received",
      status: props.portal.submission ? "approved" : "active",
    },
    ...props.portal.steps.map((step) => ({ id: step.id, name: step.name, status: step.status })),
    {
      id: "filed",
      name: "Filed and complete",
      status: props.portal.submission?.status === "approved" ? "approved" : "pending",
    },
  ]);
</script>

<template>
  <aside class="bg-primary text-primary-foreground rounded-2xl p-6 sm:p-8">
    <div class="flex items-center justify-between gap-3">
      <h2 class="font-semibold">Route through the office</h2>
      <span class="text-primary-foreground/60 font-mono text-[10px] tracking-[0.14em] uppercase">{{
        portal.request.id.split(":").at(-1)
      }}</span>
    </div>
    <ol class="mt-7 space-y-0">
      <li
        v-for="(stage, index) in stages"
        :key="stage.id"
        class="relative flex gap-4 pb-7 last:pb-0"
      >
        <span
          v-if="index < stages.length - 1"
          class="bg-primary-foreground/20 absolute top-7 bottom-0 left-3 w-px"
        />
        <span
          class="relative z-10 grid size-6 shrink-0 place-items-center rounded-full"
          :class="
            stage.status === 'approved'
              ? 'bg-success text-success-foreground'
              : stage.status === 'active'
                ? 'bg-primary-foreground text-primary'
                : 'bg-primary-foreground/10 text-primary-foreground/55'
          "
        >
          <CheckIcon v-if="stage.status === 'approved'" class="size-3.5" />
          <Clock3Icon v-else-if="stage.status === 'active'" class="size-3.5" />
          <LockKeyholeIcon v-else class="size-3" />
        </span>
        <div>
          <p class="text-sm font-medium">{{ stage.name }}</p>
          <p class="text-primary-foreground/60 mt-0.5 text-xs capitalize">
            {{ stage.status === "active" ? "In progress" : stage.status }}
          </p>
        </div>
      </li>
    </ol>
  </aside>
</template>
