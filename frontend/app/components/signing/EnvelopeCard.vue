<script setup lang="ts">
  import {
    BellRingIcon,
    CheckIcon,
    ChevronDownIcon,
    CopyIcon,
    FileTextIcon,
    XCircleIcon,
  } from "@lucide/vue";
  import { computed, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import type { SignatureEnvelope } from "@/types/api";
  import { formatDate } from "@/utils/format";
  import { envelopeStatusLabel, recipientColor, recipientStatusLabel } from "@/utils/signing";

  const props = defineProps<{ envelope: SignatureEnvelope; showDocument?: boolean }>();
  const emit = defineEmits<{ changed: [] }>();
  const service = signaturesService();

  const expanded = shallowRef(props.envelope.status === "sent");
  const copied = shallowRef("");
  const signers = computed(() =>
    props.envelope.recipients.filter((item) => item.role === "signer"),
  );
  const signed = computed(() => signers.value.filter((item) => item.status === "signed").length);
  const canManage = computed(
    () => props.envelope.status === "sent" && props.envelope.recipients.some((item) => item.link),
  );

  const statusVariant = computed(() =>
    props.envelope.status === "completed"
      ? "default"
      : props.envelope.status === "sent"
        ? "secondary"
        : "outline",
  );

  const copy = async (id: string, link: string) => {
    try {
      await navigator.clipboard.writeText(link);
      copied.value = id;
      toast.success("Signing link copied. It works only for that person.");
    } catch {
      toast.error("Copy failed");
    }
  };

  const remind = async () => {
    const result = await service.remind(props.envelope.id);
    if (result)
      toast.success(
        `Reminder sent to ${result.reminded} ${result.reminded === 1 ? "signer" : "signers"}`,
      );
    else toast.error(service.error.value);
  };

  const confirming = shallowRef(false);
  const reason = shallowRef("");
  const cancel = async () => {
    if (await service.voidRequest(props.envelope.id, reason.value.trim())) {
      confirming.value = false;
      toast.success("Request cancelled");
      emit("changed");
    } else toast.error(service.error.value);
  };
</script>

<template>
  <article class="rounded-lg border">
    <button
      type="button"
      class="hover:bg-muted/40 flex w-full items-start gap-2 p-3 text-left"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      <div class="min-w-0 flex-1 space-y-1">
        <p class="line-clamp-2 text-sm font-medium">{{ envelope.title }}</p>
        <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
          <Badge :variant="statusVariant" class="h-5 px-1.5 text-[10px]">
            {{ envelopeStatusLabel[envelope.status] }}
          </Badge>
          <span class="text-muted-foreground text-xs">
            {{ signed }} of {{ signers.length }} signed · {{ formatDate(envelope.created_at) }}
          </span>
        </div>
        <p
          v-if="showDocument && envelope.document_name"
          class="text-muted-foreground truncate text-xs"
        >
          {{ envelope.document_name }}
        </p>
      </div>
      <ChevronDownIcon
        class="text-muted-foreground mt-0.5 size-4 shrink-0 transition-transform"
        :class="expanded && 'rotate-180'"
      />
    </button>

    <div v-if="expanded" class="space-y-3 border-t p-3">
      <ol class="space-y-2">
        <li
          v-for="(recipient, index) in envelope.recipients"
          :key="recipient.id"
          class="flex items-center gap-2 text-sm"
        >
          <span
            class="size-2 shrink-0 rounded-full"
            :style="{ backgroundColor: recipientColor(index) }"
          />
          <span class="min-w-0 flex-1">
            <span class="block truncate">{{ recipient.name }}</span>
            <span class="text-muted-foreground block truncate text-xs">
              {{
                recipient.role === "viewer" ? "Gets a copy" : recipientStatusLabel[recipient.status]
              }}
              <template v-if="recipient.signed_at">
                · {{ formatDate(recipient.signed_at) }}</template
              >
              <template v-if="recipient.decline_reason">
                · “{{ recipient.decline_reason }}”</template
              >
            </span>
          </span>
          <Button
            v-if="recipient.link && recipient.role === 'signer' && recipient.status !== 'signed'"
            variant="ghost"
            size="icon"
            class="size-7"
            :aria-label="`Copy ${recipient.name}'s signing link`"
            @click="copy(recipient.id, recipient.link)"
          >
            <CheckIcon v-if="copied === recipient.id" class="size-3.5" />
            <CopyIcon v-else class="size-3.5" />
          </Button>
        </li>
      </ol>
      <div class="flex flex-wrap gap-2">
        <Button v-if="showDocument" variant="outline" size="sm" class="gap-1.5" as-child>
          <NuxtLink :to="`/documents/${envelope.document_id}`"
            ><FileTextIcon class="size-3.5" />Open document</NuxtLink
          >
        </Button>
        <template v-if="canManage">
          <Button
            variant="outline"
            size="sm"
            class="gap-1.5"
            :disabled="service.busy.value"
            @click="remind"
          >
            <BellRingIcon class="size-3.5" />Remind
          </Button>
          <Button
            variant="ghost"
            size="sm"
            class="text-destructive gap-1.5"
            @click="confirming = !confirming"
          >
            <XCircleIcon class="size-3.5" />Cancel request
          </Button>
        </template>
      </div>
      <form v-if="confirming" class="bg-muted/40 space-y-2 rounded-lg p-3" @submit.prevent="cancel">
        <label :for="`void-${envelope.id}`" class="text-sm font-medium">Cancel for everyone?</label>
        <Input
          :id="`void-${envelope.id}`"
          v-model="reason"
          placeholder="Reason for the signers (optional)"
          class="h-8"
        />
        <div class="flex justify-end gap-2">
          <Button type="button" variant="ghost" size="sm" @click="confirming = false"
            >Keep it</Button
          >
          <Button type="submit" variant="destructive" size="sm" :disabled="service.busy.value"
            >Cancel request</Button
          >
        </div>
      </form>
    </div>
  </article>
</template>
