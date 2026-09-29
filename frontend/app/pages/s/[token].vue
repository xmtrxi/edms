<script setup lang="ts">
  import { CloudDownloadIcon, FileTextIcon, LockIcon, LogInIcon } from "@lucide/vue";
  import { computed, onBeforeUnmount, onMounted, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import FileTypeIcon from "@/components/FileTypeIcon.vue";
  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import type { LinkDocument } from "@/types/api";
  import { fileTypeFromMime, formatBytes, formatDate } from "@/utils/format";

  definePageMeta({ layout: "auth" });

  const route = useRoute();
  const sharing = sharingService();
  const token = computed(() => String((route.params as Record<string, string>).token ?? ""));

  const state = shallowRef<"loading" | "password" | "signin" | "missing" | "ready">("loading");
  const linked = shallowRef<LinkDocument | null>(null);
  const password = shallowRef("");
  const wrongPassword = shallowRef(false);
  const previewUrl = shallowRef("");
  const downloading = shallowRef(false);

  useHead(() => ({
    title: linked.value ? `${linked.value.document.name} — Lumen DMS` : "Shared file — Lumen DMS",
    meta: [{ name: "robots", content: "noindex" }],
  }));

  // Types the browser can show inline without a converter.
  const previewKind = computed(() => {
    const mime = linked.value?.document.mime_type ?? "";
    if (mime === "application/pdf") return "pdf";
    if (mime.startsWith("image/")) return "image";
    if (mime.startsWith("text/")) return "text";
    return "";
  });

  const loadPreview = async () => {
    if (!previewKind.value) return;
    const blob = await sharing.linkFile(token.value, password.value, true);
    if (blob) previewUrl.value = URL.createObjectURL(blob);
  };

  const open = async () => {
    const result = await sharing.openLink(token.value, password.value);
    if (result.data) {
      linked.value = result.data;
      state.value = "ready";
      await loadPreview();
      return;
    }
    if (result.status === 401) {
      wrongPassword.value = Boolean(password.value);
      state.value = "password";
    } else if (result.status === 403) {
      state.value = "signin";
    } else {
      state.value = "missing";
    }
  };

  const download = async () => {
    if (!linked.value) return;
    downloading.value = true;
    const blob = await sharing.linkFile(token.value, password.value, false);
    downloading.value = false;
    if (!blob) {
      toast.error(sharing.error.value || "Download failed");
      return;
    }
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = linked.value.document.name;
    anchor.click();
    URL.revokeObjectURL(url);
  };

  onMounted(open);
  onBeforeUnmount(() => {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value);
  });
</script>

<template>
  <div class="flex min-h-dvh flex-col">
    <header class="border-border flex items-center gap-2 border-b px-4 py-3 sm:px-6">
      <FileTextIcon class="text-primary size-5" />
      <span class="text-sm font-semibold">Lumen DMS</span>
    </header>

    <main class="flex flex-1 items-start justify-center p-4 sm:p-8">
      <div v-if="state === 'loading'" class="w-full max-w-3xl space-y-3">
        <Skeleton class="h-8 w-1/2" />
        <Skeleton class="h-[60dvh] w-full" />
      </div>

      <form
        v-else-if="state === 'password'"
        class="bg-card mt-12 w-full max-w-sm space-y-4 rounded-2xl border p-6 shadow-sm"
        @submit.prevent="open"
      >
        <div class="bg-muted grid size-11 place-items-center rounded-xl">
          <LockIcon class="size-5" />
        </div>
        <div>
          <h1 class="text-lg font-semibold">This link is protected</h1>
          <p class="text-muted-foreground mt-1 text-sm">
            Enter the password you were given to open the file.
          </p>
        </div>
        <div class="space-y-1.5">
          <Label for="link-password">Password</Label>
          <Input
            id="link-password"
            v-model="password"
            type="password"
            autocomplete="off"
            autofocus
            :aria-invalid="wrongPassword"
          />
          <p v-if="wrongPassword" class="text-destructive text-xs" role="alert">
            That password isn’t right.
          </p>
        </div>
        <Button type="submit" class="w-full" :disabled="!password">Open file</Button>
      </form>

      <div
        v-else-if="state === 'signin'"
        class="bg-card mt-12 w-full max-w-sm space-y-4 rounded-2xl border p-6 text-center shadow-sm"
      >
        <h1 class="text-lg font-semibold">Sign in to open this file</h1>
        <p class="text-muted-foreground text-sm">
          This link only works for members of the organization that shared it.
        </p>
        <Button as-child class="w-full">
          <NuxtLink :to="{ path: '/auth/login', query: { redirect: route.fullPath } }"
            ><LogInIcon class="size-4" />Sign in</NuxtLink
          >
        </Button>
      </div>

      <div
        v-else-if="state === 'missing'"
        class="bg-card mt-12 w-full max-w-sm rounded-2xl border p-6 text-center shadow-sm"
      >
        <h1 class="text-lg font-semibold">This link isn’t available</h1>
        <p class="text-muted-foreground mt-2 text-sm">
          It may have expired or been turned off. Ask the person who shared it for a new link.
        </p>
      </div>

      <article v-else-if="linked" class="w-full max-w-5xl space-y-4">
        <div class="flex flex-wrap items-center gap-3">
          <FileTypeIcon :type="fileTypeFromMime(linked.document.mime_type)" class="size-6" />
          <div class="min-w-0 flex-1">
            <h1 class="truncate text-lg font-semibold">{{ linked.document.name }}</h1>
            <p class="text-muted-foreground text-xs">
              {{ formatBytes(linked.document.size) }} · link valid until
              {{ formatDate(linked.expires_at) }}
            </p>
          </div>
          <Button v-if="linked.can_download" :disabled="downloading" @click="download">
            <CloudDownloadIcon class="size-4" />{{ downloading ? "Downloading…" : "Download" }}
          </Button>
        </div>

        <div class="bg-card overflow-hidden rounded-xl border shadow-sm">
          <iframe
            v-if="previewUrl && (previewKind === 'pdf' || previewKind === 'text')"
            :src="previewUrl"
            :title="linked.document.name"
            class="h-[75dvh] w-full"
          />
          <img
            v-else-if="previewUrl && previewKind === 'image'"
            :src="previewUrl"
            :alt="linked.document.name"
            class="mx-auto max-h-[75dvh] object-contain"
          />
          <div v-else class="flex flex-col items-center gap-3 p-12 text-center">
            <FileTypeIcon :type="fileTypeFromMime(linked.document.mime_type)" size="lg" />
            <p class="text-sm font-medium">No preview for this file type yet</p>
            <p v-if="!linked.can_download" class="text-muted-foreground text-xs">
              The person who shared it turned off downloads.
            </p>
          </div>
        </div>
      </article>
    </main>
  </div>
</template>
