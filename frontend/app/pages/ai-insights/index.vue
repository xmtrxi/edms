<script setup lang="ts">
  import { BotIcon, FileTextIcon, SparklesIcon } from "@lucide/vue";
  import { onMounted, ref, shallowRef } from "vue";

  definePageMeta({
    title: "AI workspace",
    description: "Ask, summarize, classify, or extract from office records.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "AI workspace" }],
  });
  useHead({ title: "AI workspace — Lumen DMS" });
  const ai = aiService();
  const docs = documentsService();
  const question = ref("");
  const selected = ref<string[]>([]);
  const answer = shallowRef("");
  const operation = ref<"ask" | "summarize" | "classify" | "extract">("ask");
  const toggle = (id: string) => {
    selected.value = selected.value.includes(id)
      ? selected.value.filter((item) => item !== id)
      : [...selected.value, id];
  };
  const run = async () => {
    const job = await ai.run(operation.value, {
      question: question.value,
      document_ids: selected.value,
    });
    const result = job?.result ?? {};
    answer.value = String(
      result.answer ??
        result.summary ??
        result.extraction ??
        JSON.stringify(result.classification ?? result, null, 2),
    );
  };
  onMounted(() => docs.fetchDocuments({ page: 1, page_size: 30 }));
</script>

<template>
  <div class="grid gap-5 xl:grid-cols-[minmax(0,1.2fr)_22rem]">
    <section class="bg-card overflow-hidden rounded-2xl border shadow-sm">
      <div class="from-primary/10 via-card to-card border-b bg-gradient-to-br p-6">
        <div class="flex items-center gap-3">
          <div
            class="bg-primary text-primary-foreground shadow-primary/20 grid size-11 place-items-center rounded-xl shadow-lg"
          >
            <BotIcon class="size-5" />
          </div>
          <div>
            <h1 class="text-lg font-semibold">Office intelligence</h1>
            <p class="text-muted-foreground text-sm">
              Provider-neutral analysis over the records you select.
            </p>
          </div>
        </div>
      </div>
      <div class="space-y-4 p-5">
        <div class="flex flex-wrap gap-2">
          <Button
            v-for="item in ['ask', 'summarize', 'classify', 'extract'] as const"
            :key="item"
            size="sm"
            :variant="operation === item ? 'default' : 'outline'"
            class="capitalize"
            @click="operation = item"
            >{{ item }}</Button
          >
        </div>
        <Textarea
          v-model="question"
          :placeholder="
            operation === 'ask'
              ? 'What do you need to know from these files?'
              : 'Optional instructions for this analysis'
          "
          class="min-h-28 resize-none"
        />
        <Button
          class="gap-2"
          :disabled="
            ai.loading.value || selected.length === 0 || (operation === 'ask' && !question.trim())
          "
          @click="run"
          ><SparklesIcon class="size-4" />{{
            ai.loading.value ? "Working…" : `Run ${operation}`
          }}</Button
        >
        <div
          v-if="ai.error.value"
          class="border-destructive/25 bg-destructive/5 text-destructive rounded-xl border p-3 text-sm"
        >
          {{ ai.error.value }}
        </div>
        <div v-if="answer" class="bg-muted/25 rounded-xl border p-5">
          <p class="text-muted-foreground mb-2 text-xs font-semibold tracking-wider uppercase">
            Result
          </p>
          <pre class="font-sans text-sm leading-6 whitespace-pre-wrap">{{ answer }}</pre>
        </div>
      </div>
    </section>
    <aside class="bg-card rounded-2xl border p-4 shadow-sm">
      <div class="mb-3 flex items-center justify-between">
        <div>
          <h2 class="text-sm font-semibold">Source records</h2>
          <p class="text-muted-foreground text-xs">{{ selected.length }} selected</p>
        </div>
        <FileTextIcon class="text-muted-foreground size-4" />
      </div>
      <div class="max-h-[36rem] space-y-2 overflow-y-auto">
        <button
          v-for="item in docs.documents.value"
          :key="item.id"
          class="w-full rounded-lg border p-3 text-left transition"
          :class="selected.includes(item.id) ? 'border-primary bg-primary/5' : 'hover:bg-muted/40'"
          @click="toggle(item.id)"
        >
          <p class="line-clamp-2 text-sm font-medium">{{ item.name }}</p>
          <p class="text-muted-foreground mt-1 text-xs capitalize">{{ item.status }}</p>
        </button>
        <p
          v-if="!docs.loading.value && docs.documents.value.length === 0"
          class="text-muted-foreground py-10 text-center text-sm"
        >
          Upload and process a document first.
        </p>
      </div>
    </aside>
  </div>
</template>
