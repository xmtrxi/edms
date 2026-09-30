<script setup lang="ts">
  import { ArrowLeftIcon, EyeIcon, Loader2Icon, UsersIcon } from "@lucide/vue";
  import { computed, onBeforeUnmount, onMounted, shallowRef } from "vue";

  import { Button } from "@/components/ui/button";
  import type { ApiResponse, EditorSession } from "@/types/api";
  import { apiErrorMessage } from "@/utils/api";
  import { initialsOf } from "@/utils/format";

  definePageMeta({
    title: "Editor",
    breadcrumbs: [
      { label: "Workspace", to: "/" },
      { label: "Documents", to: "/documents" },
      { label: "Editor" },
    ],
  });

  interface DocEditor {
    destroyEditor: () => void;
  }
  type DocsAPI = { DocEditor: new (id: string, config: Record<string, unknown>) => DocEditor };

  const route = useRoute();
  const docId = computed(() => String((route.params as Record<string, string>).id ?? ""));
  const session = shallowRef<EditorSession | null>(null);
  const failure = shallowRef("");
  const ready = shallowRef(false);
  const title = computed(() =>
    String((session.value?.config.document as { title?: string })?.title ?? "Document"),
  );
  const others = usePresence(computed(() => (docId.value ? `document:${docId.value}` : "")));
  let editor: DocEditor | null = null;

  useHead(() => ({ title: `${title.value} — Lumen DMS` }));

  // ONLYOFFICE serves its own API script; it is loaded once per page.
  const loadScript = (src: string) =>
    new Promise<void>((resolve, reject) => {
      if ((window as unknown as { DocsAPI?: DocsAPI }).DocsAPI) return resolve();
      const script = document.createElement("script");
      script.src = src;
      script.async = true;
      script.onload = () => resolve();
      script.onerror = () => reject(new Error("The editor server can’t be reached."));
      document.head.appendChild(script);
    });

  onMounted(async () => {
    const { $api } = useNuxtApp();
    try {
      const view = route.query.mode === "view" ? "?mode=view" : "";
      const res = await $api<ApiResponse<EditorSession>>(`/documents/${docId.value}/editor${view}`);
      session.value = res.data;
    } catch (err) {
      const status = (err as { response?: { status?: number } }).response?.status;
      failure.value =
        status === 503
          ? "Editing in the browser isn’t set up on this installation yet. An administrator needs to connect ONLYOFFICE."
          : status === 415
            ? "This file type can’t be opened in the editor."
            : apiErrorMessage(err);
      return;
    }
    try {
      await loadScript(`${session.value.document_server_url}/web-apps/apps/api/documents/api.js`);
      const api = (window as unknown as { DocsAPI: DocsAPI }).DocsAPI;
      editor = new api.DocEditor("onlyoffice-editor", {
        ...session.value.config,
        width: "100%",
        height: "100%",
        events: {
          onAppReady: () => {
            ready.value = true;
          },
        },
      });
    } catch (err) {
      failure.value = err instanceof Error ? err.message : "The editor couldn’t start.";
    }
  });

  onBeforeUnmount(() => {
    editor?.destroyEditor();
    editor = null;
  });
</script>

<template>
  <Teleport to="#page-actions-target">
    <div
      v-if="others.length"
      class="flex items-center gap-1.5"
      :title="others.map((item) => item.name).join(', ')"
    >
      <UsersIcon class="text-muted-foreground size-4" />
      <span
        v-for="person in others.slice(0, 4)"
        :key="person.id"
        class="bg-primary/10 text-primary -ml-1 grid size-7 place-items-center rounded-full border-2 border-white text-[10px] font-semibold first:ml-0"
        >{{ initialsOf(person.name) }}</span
      >
      <span class="sr-only">{{ others.length }} others here</span>
    </div>
    <span
      v-if="session?.mode === 'view'"
      class="text-muted-foreground flex items-center gap-1 text-xs"
    >
      <EyeIcon class="size-3.5" />View only
    </span>
    <Button variant="outline" size="sm" class="gap-1.5" as-child>
      <NuxtLink :to="`/documents/${docId}`"><ArrowLeftIcon class="size-4" />Done</NuxtLink>
    </Button>
  </Teleport>

  <div v-if="failure" class="bg-card mx-auto max-w-lg rounded-2xl border p-8 text-center">
    <p class="text-sm font-medium">{{ failure }}</p>
    <Button variant="outline" size="sm" class="mt-4" as-child>
      <NuxtLink :to="`/documents/${docId}`">Back to the document</NuxtLink>
    </Button>
  </div>
  <div
    v-else
    class="bg-card relative h-[calc(100dvh-12rem)] min-h-[480px] overflow-hidden rounded-xl border"
  >
    <div
      v-if="!ready"
      class="text-muted-foreground absolute inset-0 z-10 grid place-items-center text-sm"
      role="status"
    >
      <span class="flex items-center gap-2"
        ><Loader2Icon class="size-4 animate-spin" />Opening {{ title }}…</span
      >
    </div>
    <div id="onlyoffice-editor" class="size-full" />
  </div>
</template>
