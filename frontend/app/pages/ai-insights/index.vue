<script setup lang="ts">
  import { FileTextIcon, FolderIcon, SendIcon, SparklesIcon, XIcon } from "@lucide/vue";
  import { computed, nextTick, onMounted, ref, shallowRef } from "vue";

  import { Button } from "@/components/ui/button";
  import { Textarea } from "@/components/ui/textarea";
  import type { AIAnswer, AICitation } from "@/types/api";

  definePageMeta({
    title: "Ask",
    description: "Ask questions and get answers from the documents you can open, with sources.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Ask" }],
  });
  useHead({ title: "Ask — Lumen DMS" });

  interface Turn {
    question: string;
    result?: AIAnswer;
    error?: string;
  }

  const route = useRoute();
  const router = useRouter();
  const ai = aiService();
  const folderService = foldersService();
  const question = shallowRef("");
  const turns = ref<Turn[]>([]);
  const thread = ref<HTMLElement | null>(null);

  const folderId = computed(() => String(route.query.folder ?? ""));
  const folderName = computed(
    () => folderService.folders.value.find((folder) => folder.id === folderId.value)?.name,
  );

  onMounted(() => void folderService.fetchFolders());

  const suggestions = [
    "What decisions were made about the budget?",
    "Which contracts expire in the next three months?",
    "Summarize the latest board minutes",
  ];

  // Splits an answer into text and [n] citation markers.
  const segments = (turn: Turn) => {
    const text = turn.result?.answer ?? "";
    const parts: Array<{ text: string; citation?: AICitation }> = [];
    let last = 0;
    for (const match of text.matchAll(/\[(\d{1,2})\]/g)) {
      const source = turn.result?.sources[Number(match[1]) - 1];
      if (!source) continue;
      parts.push({ text: text.slice(last, match.index) });
      parts.push({ text: match[1] ?? "", citation: source });
      last = (match.index ?? 0) + match[0].length;
    }
    parts.push({ text: text.slice(last) });
    return parts;
  };

  const pages = (citation: AICitation) =>
    citation.page_start
      ? citation.page_end && citation.page_end > citation.page_start
        ? `pages ${citation.page_start}–${citation.page_end}`
        : `page ${citation.page_start}`
      : "";

  const ask = async (text = question.value) => {
    const value = text.trim();
    if (!value || ai.loading.value) return;
    question.value = "";
    const turn: Turn = { question: value };
    turns.value.push(turn);
    const current = turns.value[turns.value.length - 1]!;
    await nextTick();
    thread.value?.scrollTo({ top: thread.value.scrollHeight, behavior: "smooth" });
    const { data, status } = await ai.answer({
      question: value,
      folder_id: folderId.value || undefined,
    });
    if (data) current.result = data;
    else
      current.error =
        status === 503
          ? "The assistant isn’t set up on this installation yet. An administrator needs to connect an AI provider. Search still works."
          : ai.error.value || "Something went wrong";
    await nextTick();
    thread.value?.scrollTo({ top: thread.value.scrollHeight, behavior: "smooth" });
  };

  const clearScope = () => router.push({ query: {} });
</script>

<template>
  <div class="mx-auto flex h-[calc(100dvh-13rem)] max-w-3xl flex-col">
    <div ref="thread" class="min-h-0 flex-1 space-y-6 overflow-y-auto pb-6">
      <div v-if="!turns.length" class="pt-10 text-center">
        <div class="bg-primary/10 text-primary mx-auto grid size-12 place-items-center rounded-2xl">
          <SparklesIcon class="size-6" />
        </div>
        <h2 class="mt-4 text-lg font-semibold">Ask your documents</h2>
        <p class="text-muted-foreground mx-auto mt-1 max-w-md text-sm">
          Answers come only from files you can open, and every fact links to the page it came from.
        </p>
        <div class="mt-6 flex flex-wrap justify-center gap-2">
          <Button
            v-for="item in suggestions"
            :key="item"
            variant="outline"
            size="sm"
            @click="ask(item)"
          >
            {{ item }}
          </Button>
        </div>
      </div>

      <article v-for="(turn, index) in turns" :key="index" class="space-y-3">
        <p class="bg-muted ml-auto w-fit max-w-[85%] rounded-2xl rounded-br-sm px-4 py-2.5 text-sm">
          {{ turn.question }}
        </p>

        <div
          v-if="!turn.result && !turn.error"
          class="text-muted-foreground flex items-center gap-2 text-sm"
        >
          <SparklesIcon class="size-4 animate-pulse" /> Reading your documents…
        </div>
        <p v-else-if="turn.error" class="bg-destructive/5 text-destructive rounded-xl p-3 text-sm">
          {{ turn.error }}
        </p>
        <template v-else-if="turn.result">
          <p class="text-sm leading-relaxed whitespace-pre-line">
            <template v-for="(part, partIndex) in segments(turn)" :key="partIndex">
              <NuxtLink
                v-if="part.citation"
                :to="`/documents/${part.citation.document_id}`"
                class="bg-primary/10 text-primary hover:bg-primary/20 mx-0.5 inline-grid size-5 place-items-center rounded-full align-text-top text-[10px] font-semibold"
                :aria-label="`Source ${part.text}: ${part.citation.document_name}`"
                >{{ part.text }}</NuxtLink
              >
              <template v-else>{{ part.text }}</template>
            </template>
          </p>
          <ol v-if="turn.result.citations.length" class="grid gap-2 sm:grid-cols-2">
            <li v-for="citation in turn.result.citations" :key="citation.index">
              <NuxtLink
                :to="`/documents/${citation.document_id}`"
                class="bg-card hover:border-primary/40 block rounded-xl border p-3 transition-colors"
              >
                <span class="flex items-center gap-2 text-sm font-medium">
                  <span
                    class="bg-primary/10 text-primary grid size-5 shrink-0 place-items-center rounded-full text-[10px]"
                    >{{ turn.result.sources.indexOf(citation) + 1 || citation.index }}</span
                  >
                  <FileTextIcon class="text-muted-foreground size-4 shrink-0" />
                  <span class="truncate">{{ citation.document_name }}</span>
                </span>
                <span class="text-muted-foreground mt-1 line-clamp-2 block text-xs">
                  <template v-if="pages(citation)">{{ pages(citation) }} · </template
                  >{{ citation.snippet }}
                </span>
              </NuxtLink>
            </li>
          </ol>
        </template>
      </article>
    </div>

    <form class="bg-card rounded-2xl border p-2 shadow-sm" @submit.prevent="ask()">
      <div v-if="folderId" class="mb-1 flex items-center gap-2 px-2 pt-1 text-xs">
        <FolderIcon class="size-3.5" />
        <span class="text-muted-foreground">Only in</span>
        <span class="font-medium">{{ folderName ?? "this folder" }}</span>
        <button
          type="button"
          class="hover:text-foreground text-muted-foreground"
          aria-label="Search the whole drive"
          @click="clearScope"
        >
          <XIcon class="size-3.5" />
        </button>
      </div>
      <div class="flex items-end gap-2">
        <label for="ask-input" class="sr-only">Your question</label>
        <Textarea
          id="ask-input"
          v-model="question"
          rows="1"
          class="max-h-40 min-h-10 resize-none border-0 shadow-none focus-visible:ring-0"
          placeholder="Ask about your documents…"
          @keydown.enter.exact.prevent="ask()"
        />
        <Button
          type="submit"
          size="icon"
          :disabled="!question.trim() || ai.loading.value"
          aria-label="Ask"
        >
          <SendIcon class="size-4" />
        </Button>
      </div>
    </form>
  </div>
</template>
