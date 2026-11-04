<script setup lang="ts">
  import { CheckIcon, InboxIcon, MessageSquareIcon, RotateCcwIcon, XIcon } from "@lucide/vue";
  import { computed, ref, watch } from "vue";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import type { FormSubmission, FormSubmissionDetail } from "@/types/api";
  import { formatDateTime } from "@/utils/format";

  const props = defineProps<{
    submissions: FormSubmission[];
    selected: FormSubmissionDetail | null;
    loading?: boolean;
    busy?: boolean;
    error?: string;
  }>();
  const emit = defineEmits<{
    select: [id: string];
    refresh: [];
    decide: [payload: { id: string; action: "approve" | "reject" | "return"; comment: string }];
    comment: [payload: { id: string; body: string }];
  }>();

  const note = ref("");
  const selectedId = computed(() => props.selected?.submission.id ?? "");
  const activeStep = computed(() => props.selected?.steps.find((step) => step.status === "active"));
  const values = computed(() => Object.entries(props.selected?.submission.values ?? {}));

  watch(selectedId, () => {
    note.value = "";
  });

  const submitDecision = (action: "approve" | "reject" | "return") => {
    if (!selectedId.value) return;
    emit("decide", { id: selectedId.value, action, comment: note.value.trim() });
  };
  const submitComment = () => {
    if (!selectedId.value || !note.value.trim()) return;
    emit("comment", { id: selectedId.value, body: note.value.trim() });
    note.value = "";
  };
</script>

<template>
  <section class="border-border bg-card overflow-hidden rounded-xl border">
    <header
      class="border-border flex flex-wrap items-center justify-between gap-3 border-b px-5 py-4"
    >
      <div>
        <h2 class="text-sm font-semibold">Approval inbox</h2>
        <p class="text-muted-foreground mt-0.5 text-xs">Review requests assigned to you.</p>
      </div>
      <Button variant="outline" size="sm" :disabled="loading" @click="emit('refresh')">
        {{ loading ? "Refreshing…" : "Refresh" }}
      </Button>
    </header>

    <p v-if="error" role="alert" class="text-destructive border-border border-b px-5 py-3 text-sm">
      {{ error }}
    </p>

    <div
      v-if="loading && !submissions.length"
      class="text-muted-foreground p-8 text-center text-sm"
    >
      Loading assigned submissions…
    </div>
    <div v-else-if="!submissions.length" class="p-10 text-center">
      <InboxIcon class="text-muted-foreground/50 mx-auto size-8" />
      <p class="mt-3 text-sm font-medium">Your review queue is clear</p>
      <p class="text-muted-foreground mt-1 text-xs">New approvals will appear here.</p>
    </div>
    <div v-else class="grid min-h-[34rem] lg:grid-cols-[19rem_1fr]">
      <nav
        aria-label="Pending submissions"
        class="border-border border-b lg:border-r lg:border-b-0"
      >
        <button
          v-for="item in submissions"
          :key="item.id"
          type="button"
          class="border-border hover:bg-muted/60 focus-visible:ring-ring block w-full border-b px-4 py-3 text-left outline-none focus-visible:ring-2"
          :class="item.id === selectedId ? 'bg-muted' : ''"
          @click="emit('select', item.id)"
        >
          <div class="flex items-center justify-between gap-2">
            <span class="truncate text-sm font-medium"
              >Submission {{ item.id.split(":").at(-1) }}</span
            >
            <Badge variant="outline">{{ item.status.replaceAll("_", " ") }}</Badge>
          </div>
          <p class="text-muted-foreground mt-1 text-xs">{{ formatDateTime(item.updated_at) }}</p>
        </button>
      </nav>

      <div v-if="!selected" class="grid place-items-center p-8 text-center">
        <div>
          <InboxIcon class="text-muted-foreground/50 mx-auto size-8" />
          <p class="mt-3 text-sm font-medium">Select a submission</p>
          <p class="text-muted-foreground mt-1 text-xs">
            Its answers and approval history will open here.
          </p>
        </div>
      </div>
      <article v-else class="min-w-0 p-5 sm:p-6">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-muted-foreground text-xs">Current step</p>
            <h3 class="mt-1 text-lg font-semibold">
              {{ activeStep?.name ?? "Review submission" }}
            </h3>
          </div>
          <Badge>{{ selected.submission.status.replaceAll("_", " ") }}</Badge>
        </div>

        <dl class="mt-6 grid gap-3 sm:grid-cols-2">
          <div
            v-for="[key, value] in values"
            :key="key"
            class="border-border min-w-0 rounded-lg border p-3"
          >
            <dt class="text-muted-foreground truncate text-xs">{{ key.replaceAll("_", " ") }}</dt>
            <dd class="mt-1 text-sm font-medium break-words">{{ String(value ?? "—") }}</dd>
          </div>
        </dl>

        <div class="mt-6">
          <h4 class="text-sm font-semibold">Review notes</h4>
          <div v-if="selected.comments.length" class="mt-3 space-y-2">
            <div
              v-for="comment in selected.comments"
              :key="comment.id"
              class="bg-muted rounded-lg px-3 py-2.5"
            >
              <p class="text-sm break-words">{{ comment.body }}</p>
              <p class="text-muted-foreground mt-1 text-[11px]">
                {{ formatDateTime(comment.created_at) }}
              </p>
            </div>
          </div>
          <p v-else class="text-muted-foreground mt-2 text-xs">No notes have been added.</p>
          <label for="review-note" class="mt-4 block text-xs font-medium"
            >Add a note or decision reason</label
          >
          <textarea
            id="review-note"
            v-model="note"
            rows="3"
            class="border-input bg-background placeholder:text-muted-foreground focus-visible:ring-ring mt-2 w-full resize-y rounded-lg border px-3 py-2 text-sm outline-none focus-visible:ring-2"
            placeholder="Write context for the requester"
          />
        </div>

        <div class="mt-4 flex flex-wrap gap-2">
          <Button :disabled="busy" @click="submitDecision('approve')"
            ><CheckIcon class="mr-2 size-4" />Approve</Button
          >
          <Button
            variant="outline"
            :disabled="busy || !note.trim()"
            @click="submitDecision('return')"
            ><RotateCcwIcon class="mr-2 size-4" />Return</Button
          >
          <Button
            variant="outline"
            class="text-destructive"
            :disabled="busy"
            @click="submitDecision('reject')"
            ><XIcon class="mr-2 size-4" />Reject</Button
          >
          <Button variant="ghost" :disabled="busy || !note.trim()" @click="submitComment"
            ><MessageSquareIcon class="mr-2 size-4" />Add note</Button
          >
        </div>
      </article>
    </div>
  </section>
</template>
