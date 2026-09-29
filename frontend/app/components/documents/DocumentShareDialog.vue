<script setup lang="ts">
  import { CopyIcon, LinkIcon, ShieldCheckIcon, Trash2Icon } from "@lucide/vue";
  import { computed, reactive, watch } from "vue";

  import { Button } from "@/components/ui/button";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
  } from "@/components/ui/dialog";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import type { DocumentShare } from "@/types/api";
  import { formatDateTime } from "@/utils/format";

  const open = defineModel<boolean>("open", { required: true });
  const props = defineProps<{
    title: string;
    shares: DocumentShare[];
    createdToken?: string;
    busy?: boolean;
    error?: string;
  }>();
  const emit = defineEmits<{
    submit: [input: { email: string; permission: "view" | "download"; expires_at?: string }];
    revoke: [shareId: string];
    copy: [value: string];
  }>();
  const form = reactive({ email: "", permission: "view" as "view" | "download", expiresAt: "" });
  const canSubmit = computed(() => form.email.includes("@") && !props.busy);

  watch(open, (value) => {
    if (!value) return;
    form.email = "";
    form.permission = "view";
    form.expiresAt = "";
  });
  const submit = () => {
    if (!canSubmit.value) return;
    emit("submit", {
      email: form.email.trim(),
      permission: form.permission,
      expires_at: form.expiresAt ? new Date(form.expiresAt).toISOString() : undefined,
    });
  };
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-xl">
      <DialogHeader>
        <DialogTitle>Share {{ title }}</DialogTitle>
        <DialogDescription
          >Create time-limited access for a specific email address.</DialogDescription
        >
      </DialogHeader>
      <div class="grid gap-4 py-2 sm:grid-cols-2">
        <div class="space-y-2 sm:col-span-2">
          <Label for="share-email">Recipient email</Label>
          <Input id="share-email" v-model="form.email" type="email" autocomplete="email" />
        </div>
        <div class="space-y-2">
          <Label for="share-permission">Permission</Label>
          <select
            id="share-permission"
            v-model="form.permission"
            class="border-input bg-background h-9 w-full rounded-md border px-3 text-sm"
          >
            <option value="view">View only</option>
            <option value="download">View and download</option>
          </select>
        </div>
        <div class="space-y-2">
          <Label for="share-expiry">Expires</Label>
          <Input id="share-expiry" v-model="form.expiresAt" type="datetime-local" />
        </div>
      </div>
      <p v-if="error" role="alert" class="text-destructive text-sm">{{ error }}</p>
      <div v-if="createdToken" class="bg-muted rounded-lg p-4">
        <div class="flex items-start gap-3">
          <LinkIcon class="text-primary mt-0.5 size-4 shrink-0" />
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium">Share token created</p>
            <p class="text-muted-foreground mt-1 text-xs">
              Copy it now. For security, it is not stored in readable form.
            </p>
            <code class="mt-3 block text-xs break-all">{{ createdToken }}</code>
          </div>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Copy share token"
            @click="emit('copy', createdToken)"
            ><CopyIcon class="size-4"
          /></Button>
        </div>
      </div>
      <div>
        <div class="mb-2 flex items-center gap-2">
          <ShieldCheckIcon class="text-primary size-4" />
          <h3 class="text-sm font-semibold">Active and previous shares</h3>
        </div>
        <div
          v-if="shares.length"
          class="border-border max-h-48 divide-y overflow-y-auto rounded-lg border"
        >
          <div v-for="share in shares" :key="share.id" class="flex items-center gap-3 px-3 py-2.5">
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium">{{ share.shared_with_email }}</p>
              <p class="text-muted-foreground text-[11px]">
                {{ share.permission }} · expires {{ formatDateTime(share.expires_at) }}
              </p>
            </div>
            <span v-if="share.revoked_at" class="text-muted-foreground text-xs">Revoked</span>
            <Button
              v-else
              variant="ghost"
              size="icon"
              aria-label="Revoke share"
              :disabled="busy"
              @click="emit('revoke', share.id)"
              ><Trash2Icon class="text-destructive size-4"
            /></Button>
          </div>
        </div>
        <p v-else class="text-muted-foreground text-xs">This document has not been shared.</p>
      </div>
      <DialogFooter>
        <Button variant="outline" @click="open = false">Close</Button>
        <Button :disabled="!canSubmit" @click="submit">{{
          busy ? "Creating…" : "Create share"
        }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
