<script setup lang="ts">
  import { CheckIcon, CopyIcon, InboxIcon } from "@lucide/vue";
  import { shallowRef, watch } from "vue";
  import { toast } from "vue-sonner";

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
  import { Textarea } from "@/components/ui/textarea";
  import type { ApiResponse, FileRequest } from "@/types/api";
  import { apiErrorMessage } from "@/utils/api";
  import { formatDate } from "@/utils/format";

  const open = defineModel<boolean>("open", { required: true });
  const props = defineProps<{ folderId: string; folderName: string }>();

  const title = shallowRef("");
  const instructions = shallowRef("");
  const days = shallowRef(14);
  const busy = shallowRef(false);
  const link = shallowRef("");
  const copied = shallowRef(false);
  const existing = shallowRef<FileRequest[]>([]);

  const load = async () => {
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<FileRequest[]>>(
        `/folders/${props.folderId}/file-requests`,
      );
      existing.value = res.data;
    } catch {
      existing.value = [];
    }
  };

  watch(open, (value) => {
    if (!value) return;
    title.value = "";
    instructions.value = "";
    link.value = "";
    copied.value = false;
    void load();
  });

  const create = async () => {
    const { $api } = useNuxtApp();
    busy.value = true;
    try {
      const res = await $api<ApiResponse<FileRequest>>("/file-requests", {
        method: "POST",
        body: {
          folder_id: props.folderId,
          title: title.value.trim(),
          instructions: instructions.value.trim(),
          expires_at: new Date(Date.now() + days.value * 86_400_000).toISOString(),
        },
      });
      link.value = res.data.url ?? "";
      await load();
    } catch (err) {
      toast.error(apiErrorMessage(err));
    } finally {
      busy.value = false;
    }
  };

  const close = async (id: string) => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/file-requests/${encodeURIComponent(id)}/close`, { method: "POST" });
      await load();
    } catch (err) {
      toast.error(apiErrorMessage(err));
    }
  };

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(link.value);
      copied.value = true;
    } catch {
      toast.error("Copy failed. Select the link and copy it manually.");
    }
  };
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>Request files into “{{ folderName }}”</DialogTitle>
        <DialogDescription>
          Anyone with the link can upload, without an account. They can’t see what’s already in the
          folder.
        </DialogDescription>
      </DialogHeader>

      <div v-if="link" class="bg-muted space-y-2 rounded-lg p-3">
        <p class="text-xs font-medium">Share this link. It can’t be shown again.</p>
        <div class="flex gap-2">
          <Input :model-value="link" readonly class="h-8 font-mono text-xs" />
          <Button size="icon" variant="outline" class="size-8" aria-label="Copy link" @click="copy">
            <CheckIcon v-if="copied" class="size-4" /><CopyIcon v-else class="size-4" />
          </Button>
        </div>
      </div>

      <form v-else class="space-y-3" @submit.prevent="create">
        <div class="space-y-1.5">
          <Label for="request-title">What are you asking for?</Label>
          <Input id="request-title" v-model="title" placeholder="Signed tender documents" />
        </div>
        <div class="space-y-1.5">
          <Label for="request-instructions"
            >Instructions <span class="text-muted-foreground">(optional)</span></Label
          >
          <Textarea id="request-instructions" v-model="instructions" rows="3" />
        </div>
        <div class="space-y-1.5">
          <Label for="request-days">Open for (days)</Label>
          <Input
            id="request-days"
            v-model.number="days"
            type="number"
            min="1"
            max="365"
            class="w-28"
          />
        </div>
      </form>

      <div v-if="existing.length" class="space-y-2">
        <p class="text-muted-foreground text-xs font-semibold tracking-wider uppercase">Requests</p>
        <ul class="divide-y rounded-lg border">
          <li
            v-for="request in existing"
            :key="request.id"
            class="flex items-center gap-3 px-3 py-2 text-sm"
          >
            <InboxIcon class="text-muted-foreground size-4 shrink-0" />
            <span class="min-w-0 flex-1">
              <span class="block truncate">{{ request.title }}</span>
              <span class="text-muted-foreground text-xs"
                >{{ request.received_files }} received ·
                {{
                  request.closed ? "closed" : `open until ${formatDate(request.expires_at)}`
                }}</span
              >
            </span>
            <Button v-if="!request.closed" size="sm" variant="ghost" @click="close(request.id)"
              >Close</Button
            >
          </li>
        </ul>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="open = false">Done</Button>
        <Button v-if="!link" :disabled="!title.trim() || busy" @click="create">Create link</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
