<script setup lang="ts">
  import { CheckCircle2Icon, FileTextIcon, InboxIcon, UploadIcon, XCircleIcon } from "@lucide/vue";
  import { computed, onMounted, ref, shallowRef } from "vue";

  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import { Progress } from "@/components/ui/progress";
  import type { ApiResponse, PublicFileRequest } from "@/types/api";
  import { apiErrorMessage } from "@/utils/api";
  import { formatBytes, formatDate } from "@/utils/format";

  definePageMeta({ layout: "auth" });
  useHead({ title: "Send files — Lumen DMS", meta: [{ name: "robots", content: "noindex" }] });

  const route = useRoute();
  const token = computed(() => String((route.params as Record<string, string>).token ?? ""));
  const request = shallowRef<PublicFileRequest | null>(null);
  const state = shallowRef<"loading" | "open" | "closed" | "missing">("loading");
  const name = shallowRef("");
  const email = shallowRef("");
  const input = ref<HTMLInputElement | null>(null);
  const dragOver = shallowRef(false);
  const files = ref<
    Array<{
      file: File;
      sent: number;
      status: "waiting" | "sending" | "done" | "failed";
      error?: string;
    }>
  >([]);
  const CHUNK = 8 * 1024 * 1024;

  const base = computed(() => `/public/file-requests/${token.value}`);
  // Public endpoints must not carry a signed-in user's token.
  const api = <T>(path: string, options: Record<string, unknown> = {}) =>
    $fetch<T>(base.value + path, { baseURL: useRuntimeConfig().public.apiBase, ...options });

  onMounted(async () => {
    try {
      const res = await api<ApiResponse<PublicFileRequest>>("");
      request.value = res.data;
      state.value = res.data.remaining > 0 ? "open" : "closed";
    } catch (err) {
      const status = (err as { response?: { status?: number } }).response?.status;
      state.value = status === 410 ? "closed" : "missing";
    }
  });

  const sendOne = async (entry: (typeof files.value)[number]) => {
    entry.status = "sending";
    try {
      const started = await api<ApiResponse<{ id: string }>>("/uploads", {
        method: "POST",
        body: { name: entry.file.name, mime_type: entry.file.type, size: entry.file.size },
      });
      const id = started.data.id;
      while (entry.sent < entry.file.size) {
        const chunk = entry.file.slice(entry.sent, Math.min(entry.sent + CHUNK, entry.file.size));
        const res = await api<ApiResponse<{ received: number }>>(
          `/uploads/${encodeURIComponent(id)}`,
          {
            method: "PUT",
            query: { offset: entry.sent },
            body: chunk,
            headers: { "Content-Type": "application/octet-stream" },
          },
        );
        entry.sent = res.data.received;
      }
      await api(`/uploads/${encodeURIComponent(id)}/complete`, {
        method: "POST",
        body: { name: name.value.trim(), email: email.value.trim() },
      });
      entry.status = "done";
    } catch (err) {
      entry.status = "failed";
      entry.error = apiErrorMessage(err);
    }
  };

  const add = async (list: FileList | File[]) => {
    const added = Array.from(list).map((file) => ({ file, sent: 0, status: "waiting" as const }));
    files.value.push(...added);
    for (const entry of files.value.filter((item) => item.status === "waiting"))
      await sendOne(entry);
  };

  const onPick = () => {
    if (input.value?.files?.length) {
      void add(input.value.files);
      input.value.value = "";
    }
  };
</script>

<template>
  <div class="flex min-h-dvh flex-col">
    <header class="flex items-center gap-2 border-b px-4 py-3 sm:px-6">
      <FileTextIcon class="text-primary size-5" />
      <span class="text-sm font-semibold">Lumen DMS</span>
    </header>
    <main class="flex flex-1 justify-center p-4 sm:p-8">
      <div v-if="state === 'loading'" class="w-full max-w-lg space-y-3">
        <Skeleton class="h-8 w-2/3" />
        <Skeleton class="h-40 w-full" />
      </div>

      <div
        v-else-if="state !== 'open'"
        class="bg-card mt-12 h-fit w-full max-w-sm rounded-2xl border p-6 text-center shadow-sm"
      >
        <h1 class="text-lg font-semibold">
          {{ state === "closed" ? "This request is closed" : "This link isn’t available" }}
        </h1>
        <p class="text-muted-foreground mt-2 text-sm">Ask the person who sent it for a new link.</p>
      </div>

      <div v-else-if="request" class="w-full max-w-lg space-y-5">
        <div class="bg-card rounded-2xl border p-6 shadow-sm">
          <div class="bg-primary/10 text-primary mb-4 grid size-11 place-items-center rounded-xl">
            <InboxIcon class="size-5" />
          </div>
          <h1 class="text-xl font-semibold tracking-tight">{{ request.title }}</h1>
          <p class="text-muted-foreground mt-1 text-sm">
            {{ request.requested_by
            }}{{ request.organization ? ` at ${request.organization}` : "" }} asked you for files.
            Open until {{ formatDate(request.expires_at) }}.
          </p>
          <p
            v-if="request.instructions"
            class="bg-muted mt-4 rounded-lg p-3 text-sm whitespace-pre-line"
          >
            {{ request.instructions }}
          </p>

          <div class="mt-5 grid gap-3 sm:grid-cols-2">
            <div class="space-y-1.5">
              <Label for="sender-name">Your name</Label>
              <Input id="sender-name" v-model="name" autocomplete="name" />
            </div>
            <div class="space-y-1.5">
              <Label for="sender-email">Your email</Label>
              <Input id="sender-email" v-model="email" type="email" autocomplete="email" />
            </div>
          </div>

          <input ref="input" type="file" multiple class="hidden" @change="onPick" />
          <button
            type="button"
            class="mt-5 w-full rounded-xl border-2 border-dashed p-8 text-center transition-colors disabled:opacity-50"
            :class="
              dragOver ? 'border-primary bg-primary/5' : 'border-border hover:border-primary/50'
            "
            :disabled="!name.trim() || !email.includes('@')"
            @dragover.prevent="dragOver = true"
            @dragleave="dragOver = false"
            @drop.prevent="
              dragOver = false;
              add(($event as DragEvent).dataTransfer?.files ?? []);
            "
            @click="input?.click()"
          >
            <UploadIcon class="text-primary mx-auto size-6" />
            <span class="mt-2 block text-sm font-medium">Drop files here or click to choose</span>
            <span class="text-muted-foreground block text-xs">
              {{
                !name.trim() || !email.includes("@")
                  ? "Add your name and email first"
                  : `Up to ${request.remaining} more files`
              }}
            </span>
          </button>
        </div>

        <ul v-if="files.length" class="bg-card divide-y rounded-2xl border shadow-sm">
          <li v-for="(entry, index) in files" :key="index" class="px-4 py-3">
            <div class="flex items-center gap-2">
              <span class="min-w-0 flex-1 truncate text-sm">{{ entry.file.name }}</span>
              <CheckCircle2Icon v-if="entry.status === 'done'" class="text-success size-4" />
              <XCircleIcon v-else-if="entry.status === 'failed'" class="text-destructive size-4" />
            </div>
            <Progress
              v-if="entry.status === 'sending'"
              :model-value="entry.file.size ? (entry.sent / entry.file.size) * 100 : 100"
              class="mt-1.5 h-1.5"
            />
            <p class="text-muted-foreground mt-1 text-xs">
              {{
                entry.status === "failed"
                  ? entry.error
                  : entry.status === "done"
                    ? "Sent"
                    : formatBytes(entry.file.size)
              }}
            </p>
          </li>
        </ul>
      </div>
    </main>
  </div>
</template>
