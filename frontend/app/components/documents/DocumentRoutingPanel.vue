<script setup lang="ts">
  import { CheckIcon, GitPullRequestArrowIcon, RotateCcwIcon, XIcon } from "@lucide/vue";
  import { computed, reactive, shallowRef } from "vue";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Textarea } from "@/components/ui/textarea";
  import type { CreateDocumentRoute, DocumentRoute, RouteDecision } from "@/types/api";

  const props = defineProps<{ routes: DocumentRoute[]; busy?: boolean }>();
  const emit = defineEmits<{
    create: [input: CreateDocumentRoute];
    decide: [routeId: string, input: RouteDecision];
  }>();

  const showCreate = shallowRef(false);
  const comment = shallowRef("");
  const title = shallowRef("");
  const assignee = shallowRef("");
  const activeRoute = computed(() =>
    props.routes.find((route) =>
      ["active", "waiting_for_changes", "waiting_for_signature"].includes(route.status),
    ),
  );
  const activeStep = computed(() => activeRoute.value?.steps[activeRoute.value.current_step]);
  const draft = reactive({ actionType: "approve" as "review" | "approve" | "sign" | "edit" });

  const createRoute = () => {
    if (!title.value.trim() || !assignee.value.trim()) return;
    emit("create", {
      title: title.value.trim(),
      priority: "medium",
      steps: [
        {
          name:
            draft.actionType === "approve"
              ? "Approval"
              : draft.actionType === "sign"
                ? "Signature"
                : draft.actionType === "edit"
                  ? "Revision"
                  : "Review",
          action_type: draft.actionType,
          assignment_type: "user",
          assigned_user_id: assignee.value.trim(),
          require_comment: draft.actionType === "review",
          require_new_version: draft.actionType === "edit",
        },
      ],
    });
    showCreate.value = false;
    title.value = "";
    assignee.value = "";
  };

  const decide = (action: RouteDecision["action"]) => {
    if (!activeRoute.value) return;
    emit("decide", activeRoute.value.id, {
      action,
      comment: comment.value.trim(),
      return_to_step:
        action === "return_for_changes"
          ? Math.max(0, activeRoute.value.current_step - 1)
          : undefined,
    });
    comment.value = "";
  };
</script>

<template>
  <section class="border-border bg-card rounded-xl border p-5">
    <div class="mb-4 flex items-center gap-2">
      <GitPullRequestArrowIcon class="text-primary h-4 w-4" />
      <h3 class="text-sm font-semibold">Document routing</h3>
      <Button
        v-if="!activeRoute"
        size="sm"
        variant="outline"
        class="ml-auto h-8 text-xs"
        @click="showCreate = !showCreate"
      >
        {{ showCreate ? "Cancel" : "Route document" }}
      </Button>
    </div>

    <form
      v-if="showCreate"
      class="mb-4 grid gap-3 rounded-lg border border-dashed p-4"
      @submit.prevent="createRoute"
    >
      <Input v-model="title" placeholder="Route title, e.g. Contract approval" required />
      <select
        v-model="draft.actionType"
        class="border-input bg-background h-10 rounded-md border px-3 text-sm"
      >
        <option value="review">Review</option>
        <option value="approve">Approve</option>
        <option value="edit">Edit</option>
        <option value="sign">Sign</option>
      </select>
      <Input v-model="assignee" placeholder="User record ID, e.g. users:abc" required />
      <Button type="submit" size="sm" :disabled="busy">Start route</Button>
    </form>

    <div v-if="activeRoute" class="space-y-4">
      <div class="flex flex-wrap items-center gap-2">
        <p class="text-sm font-medium">{{ activeRoute.title }}</p>
        <Badge variant="secondary" class="text-[10px]">{{
          activeRoute.status.replaceAll("_", " ")
        }}</Badge>
        <Badge variant="outline" class="text-[10px]">{{ activeRoute.priority }}</Badge>
      </div>
      <ol class="space-y-2">
        <li
          v-for="step in activeRoute.steps"
          :key="step.id"
          class="border-border flex items-center gap-3 rounded-lg border p-3"
        >
          <span
            class="bg-secondary flex h-6 w-6 items-center justify-center rounded-full text-[10px] font-semibold"
            >{{ step.sequence + 1 }}</span
          >
          <div class="min-w-0 flex-1">
            <p class="text-xs font-medium">{{ step.name }}</p>
            <p class="text-muted-foreground text-[10px]">
              {{ step.assignment_type }} ·
              {{ step.assigned_user_id || step.assigned_department_id || step.external_email }}
            </p>
          </div>
          <Badge :variant="step.status === 'active' ? 'default' : 'outline'" class="text-[9px]">{{
            step.status
          }}</Badge>
        </li>
      </ol>
      <div v-if="activeStep" class="space-y-2">
        <Textarea
          v-model="comment"
          placeholder="Decision note or reason for returning changes…"
          :rows="2"
        />
        <div class="flex flex-wrap gap-2">
          <Button
            v-if="activeRoute.status === 'waiting_for_changes'"
            size="sm"
            :disabled="busy"
            class="gap-1"
            @click="decide('resubmit')"
            ><RotateCcwIcon class="h-3.5 w-3.5" /> Resubmit revised version</Button
          >
          <template v-else>
            <Button
              size="sm"
              :disabled="busy"
              class="gap-1"
              @click="
                decide(
                  activeStep.action_type === 'sign'
                    ? 'sign'
                    : activeStep.action_type === 'file'
                      ? 'file'
                      : 'approve',
                )
              "
              ><CheckIcon class="h-3.5 w-3.5" /> Complete</Button
            >
            <Button
              size="sm"
              variant="outline"
              :disabled="busy"
              class="gap-1"
              @click="decide('return_for_changes')"
              ><RotateCcwIcon class="h-3.5 w-3.5" /> Return for changes</Button
            >
            <Button
              size="sm"
              variant="destructive"
              :disabled="busy"
              class="gap-1"
              @click="decide('reject')"
              ><XIcon class="h-3.5 w-3.5" /> Reject</Button
            >
          </template>
        </div>
      </div>
    </div>
    <p
      v-else-if="!showCreate"
      class="text-muted-foreground rounded-lg border border-dashed p-6 text-center text-xs"
    >
      This document has no active route.
    </p>
  </section>
</template>
