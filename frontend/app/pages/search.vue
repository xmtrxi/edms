<script setup lang="ts">
  import {
    SearchIcon,
    SparklesIcon,
    StarIcon,
    ClockIcon,
    CloudIcon,
    FilterIcon,
    ZapIcon,
    TagIcon,
    XIcon,
  } from "@lucide/vue";
  import { ref, shallowRef, computed } from "vue";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Card, CardContent } from "@/components/ui/card";
  import { Input } from "@/components/ui/input";
  import { documents, type Doc } from "@/data/mock-data";
  import { useCategories, colorStyle } from "@/lib/categories-store";
  import { cn } from "@/lib/utils";

  definePageMeta({
    title: "Search",
    description:
      "Instant typo-tolerant search across documents, OCR text, categories, and metadata.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Search" }],
  });

  useHead({
    title: "Search — Lumen DMS",
    meta: [
      {
        name: "description",
        content:
          "Instant Meilisearch-style search across documents, OCR text, categories, and metadata.",
      },
    ],
  });

  const saved = [
    "Q4 contracts up for renewal",
    "All HR policies updated this year",
    "Engineering RFCs about migration",
  ];

  interface Scored {
    doc: Doc;
    score: number;
    matched: string[];
  }

  function scoreDoc(d: Doc, terms: string[], catNames: string[]): Scored {
    let score = 0;
    const matched: string[] = [];
    const hay =
      `${d.name} ${d.tags.join(" ")} ${d.aiSummary ?? ""} ${d.entities?.join(" ") ?? ""} ${catNames.join(" ")} ${d.library ?? ""} ${d.owner}`.toLowerCase();
    for (const t of terms) {
      const tl = t.toLowerCase();
      if (!tl) continue;
      if (d.name.toLowerCase().startsWith(tl)) {
        score += 40;
        matched.push(t);
      } else if (d.name.toLowerCase().includes(tl)) {
        score += 25;
        matched.push(t);
      }
      if (d.tags.some((x) => x.toLowerCase() === tl)) score += 20;
      else if (d.tags.some((x) => x.toLowerCase().includes(tl))) score += 10;
      if (catNames.some((c) => c.toLowerCase().includes(tl))) score += 15;
      if (d.aiSummary?.toLowerCase().includes(tl)) score += 8;
      if (hay.includes(tl)) score += 2;
    }
    if (d.starred) score += 1;
    return { doc: d, score, matched };
  }

  const q = ref("renewal");
  const debounced = ref(q.value);
  const semantic = ref(true);
  const activeCats = shallowRef<string[]>([]);
  const activeLibs = shallowRef<string[]>([]);
  const activeTypes = shallowRef<string[]>([]);
  const categories = useCategories();

  let debounceTimer: ReturnType<typeof setTimeout> | undefined;
  watch(q, (val) => {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      debounced.value = val;
    }, 90);
  });

  const docCatMap = computed(() => {
    const m = new Map<string, string[]>();
    for (const c of categories)
      for (const id of c.docIds) {
        const list = m.get(id) ?? [];
        list.push(c.name);
        m.set(id, list);
      }
    return m;
  });

  const results = computed(() => {
    const terms = debounced.value.trim().split(/\s+/).filter(Boolean);
    const pool = documents.filter((d) => !d.trashed);
    const filtered = pool.filter((d) => {
      if (activeLibs.value.length && (!d.library || !activeLibs.value.includes(d.library)))
        return false;
      if (activeTypes.value.length && !activeTypes.value.includes(d.type)) return false;
      if (activeCats.value.length) {
        const dc = docCatMap.value.get(d.id) ?? [];
        if (!activeCats.value.every((c) => dc.includes(c))) return false;
      }
      return true;
    });
    if (!terms.length)
      return filtered.slice(0, 50).map((doc) => ({ doc, score: 0, matched: [] as string[] }));
    return filtered
      .map((d) => scoreDoc(d, terms, docCatMap.value.get(d.id) ?? []))
      .filter((r) => r.score > 0)
      .sort((a, b) => b.score - a.score);
  });

  function highlight(s: string) {
    const terms = debounced.value.trim().split(/\s+/).filter(Boolean);
    if (!terms.length) return s;
    const escaped = terms.map((t) => t.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
    const re = new RegExp("(" + escaped.join("|") + ")", "ig");
    return s.replace(
      re,
      "<mark class='bg-warning/30 text-foreground rounded px-0.5 font-medium'>$1</mark>",
    );
  }

  function toggle(list: string[], setter: (v: string[]) => void, v: string) {
    setter(list.includes(v) ? list.filter((x) => x !== v) : [...list, v]);
  }

  const allLibs = computed(() =>
    Array.from(new Set(documents.map((d) => d.library).filter(Boolean) as string[])),
  );
  const allTypes = ["pdf", "docx", "xlsx", "pptx", "image", "txt"];

  const activeFilters = computed(() => [
    ...activeCats.value.map((c) => ({
      k: "c-" + c,
      label: "Category: " + c,
      clear: () => {
        const v = activeCats.value;
        activeCats.value = v.filter((x) => x !== c);
      },
    })),
    ...activeLibs.value.map((l) => ({
      k: "l-" + l,
      label: "Library: " + l,
      clear: () => {
        const v = activeLibs.value;
        activeLibs.value = v.filter((x) => x !== l);
      },
    })),
    ...activeTypes.value.map((t) => ({
      k: "t-" + t,
      label: "Type: " + t.toUpperCase(),
      clear: () => {
        const v = activeTypes.value;
        activeTypes.value = v.filter((x) => x !== t);
      },
    })),
  ]);
</script>

<template>
  <div class="relative max-w-3xl">
    <SearchIcon class="text-muted-foreground absolute top-1/2 left-3.5 h-4 w-4 -translate-y-1/2" />
    <Input
      v-model="q"
      placeholder="Search across all libraries&hellip;"
      class="bg-card border-border shadow-elegant h-12 rounded-xl pr-44 pl-10 text-base"
    />
    <div class="absolute top-1/2 right-2 flex -translate-y-1/2 items-center gap-1">
      <Button
        v-if="q"
        variant="ghost"
        size="icon"
        class="h-8 w-8"
        @click="q = ''"
        aria-label="Clear"
      >
        <XIcon class="h-3.5 w-3.5" />
      </Button>
      <Button
        :variant="semantic ? 'default' : 'ghost'"
        size="sm"
        class="h-8 gap-1.5"
        @click="semantic = !semantic"
      >
        <SparklesIcon class="h-3.5 w-3.5" /> AI semantic
      </Button>
    </div>
  </div>

  <div class="text-muted-foreground mt-3 flex items-center gap-1.5 text-[11px]">
    <ZapIcon class="text-primary h-3 w-3" /> Instant ranking &middot; typo-tolerant &middot;
    Meilisearch-style
  </div>

  <div class="mt-4 flex flex-wrap items-center gap-2 text-xs">
    <Badge variant="secondary" class="bg-secondary gap-1 border-0">
      <FilterIcon class="h-3 w-3" /> Active filters
    </Badge>
    <button
      v-for="f in activeFilters"
      :key="f.k"
      @click="f.clear"
      class="border-primary/30 bg-primary/10 text-primary hover:bg-primary/15 inline-flex items-center gap-1 rounded-full border px-2 py-0.5"
    >
      {{ f.label }} <XIcon class="h-2.5 w-2.5" />
    </button>
    <span
      v-if="!activeCats.length && !activeLibs.length && !activeTypes.length"
      class="text-muted-foreground"
      >none</span
    >
  </div>

  <div class="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-4">
    <div class="space-y-4 lg:col-span-1">
      <Card class="border-border shadow-elegant">
        <CardContent class="p-4">
          <h4 class="mb-3 flex items-center gap-1.5 text-xs font-semibold">
            <TagIcon class="text-primary h-3.5 w-3.5" /> Categories
          </h4>
          <div class="space-y-1">
            <button
              v-for="c in categories"
              :key="c.id"
              @click="toggle(activeCats, (v) => (activeCats = v), c.name)"
              :class="
                cn(
                  'hover:bg-secondary/60 flex w-full items-center justify-between gap-2 rounded-md px-2 py-1.5 text-left text-xs',
                  activeCats.includes(c.name) && 'bg-secondary',
                )
              "
            >
              <span class="inline-flex items-center gap-1.5">
                <span
                  class="inline-block h-2 w-2 rounded-full"
                  :style="{ backgroundColor: colorStyle(c.color).color as string }"
                />
                {{ c.name }}
              </span>
              <span class="text-muted-foreground text-[10px]">{{ c.docIds.length }}</span>
            </button>
          </div>
        </CardContent>
      </Card>

      <Card class="border-border shadow-elegant">
        <CardContent class="p-4">
          <h4 class="mb-3 flex items-center gap-1.5 text-xs font-semibold">
            <FileTypeIcon type="pdf" size="sm" /> File type
          </h4>
          <div class="grid grid-cols-3 gap-1">
            <button
              v-for="t in allTypes"
              :key="t"
              @click="toggle(activeTypes, (v) => (activeTypes = v), t)"
              :class="
                cn(
                  'rounded-md border py-1 text-[11px]',
                  activeTypes.includes(t)
                    ? 'border-primary/40 bg-primary/10 text-primary'
                    : 'border-border text-muted-foreground hover:text-foreground',
                )
              "
            >
              {{ t.toUpperCase() }}
            </button>
          </div>
        </CardContent>
      </Card>

      <Card class="border-border shadow-elegant">
        <CardContent class="p-4">
          <h4 class="mb-3 flex items-center gap-1.5 text-xs font-semibold">
            <CloudIcon class="text-info h-3.5 w-3.5" /> Libraries
          </h4>
          <div class="space-y-1 text-xs">
            <label
              v-for="l in allLibs"
              :key="l"
              class="text-muted-foreground hover:text-foreground flex cursor-pointer items-center gap-2"
            >
              <input
                type="checkbox"
                class="accent-primary h-3.5 w-3.5"
                :checked="activeLibs.includes(l)"
                @change="toggle(activeLibs, (v) => (activeLibs = v), l)"
              />
              {{ l }}
            </label>
          </div>
        </CardContent>
      </Card>

      <Card class="border-border shadow-elegant">
        <CardContent class="p-4">
          <h4 class="mb-3 flex items-center gap-1.5 text-xs font-semibold">
            <StarIcon class="text-warning h-3.5 w-3.5" /> Saved searches
          </h4>
          <div class="space-y-1">
            <button
              v-for="s in saved"
              :key="s"
              @click="q = s"
              class="text-muted-foreground hover:text-foreground hover:bg-secondary/60 w-full rounded-md px-2 py-1.5 text-left text-xs"
            >
              {{ s }}
            </button>
          </div>
        </CardContent>
      </Card>

      <Card class="border-border shadow-elegant">
        <CardContent class="p-4">
          <h4 class="mb-3 flex items-center gap-1.5 text-xs font-semibold">
            <ClockIcon class="h-3.5 w-3.5" /> Recent
          </h4>
          <div class="space-y-1">
            <button
              v-for="s in ['msa acme', 'soc2 audit', 'patent us']"
              :key="s"
              @click="q = s"
              class="text-muted-foreground hover:text-foreground hover:bg-secondary/60 w-full rounded-md px-2 py-1 text-left text-xs"
            >
              {{ s }}
            </button>
          </div>
        </CardContent>
      </Card>
    </div>

    <div class="space-y-2.5 lg:col-span-3">
      <p class="text-muted-foreground text-xs">
        <span class="text-foreground font-medium">{{ results.length }}</span> results
        <span v-if="semantic"> &middot; AI semantic ranking</span>
        <span v-if="debounced"> &middot; for &ldquo;{{ debounced }}&rdquo;</span>
      </p>
      <Card
        v-for="{ doc: d, score } in results"
        :key="d.id"
        class="border-border shadow-elegant hover:shadow-glow hover:border-primary/30 transition-all"
      >
        <CardContent class="p-4">
          <div class="flex gap-3">
            <FileTypeIcon :type="d.type" size="md" />
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-1.5">
                <h3 class="truncate text-sm font-semibold" v-html="highlight(d.name)" />
                <Badge
                  v-if="d.aiSummary"
                  variant="secondary"
                  class="bg-primary/10 text-primary gap-1 border-0 text-[10px]"
                >
                  <SparklesIcon class="h-3 w-3" /> AI
                </Badge>
                <span v-if="score > 0" class="text-muted-foreground ml-auto text-[10px]"
                  >score {{ score }}</span
                >
              </div>
              <p
                v-if="d.aiSummary"
                class="text-foreground/80 mt-1.5 line-clamp-2 text-xs leading-relaxed"
                v-html="highlight(d.aiSummary)"
              />
              <div class="mt-2 flex flex-wrap items-center gap-1.5">
                <StatusBadges :status="d.status" :source="d.source" :library="d.library" />
                <span
                  v-for="(cname, ci) in docCatMap.get(d.id) ?? []"
                  :key="ci"
                  class="inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[10px] font-medium"
                  :style="
                    categories.find((x) => x.name === cname)
                      ? colorStyle(categories.find((x) => x.name === cname)!.color)
                      : undefined
                  "
                >
                  <TagIcon class="h-2.5 w-2.5" /> {{ cname }}
                </span>
                <span class="text-muted-foreground text-[11px]"
                  >&middot; {{ d.owner }} &middot; {{ d.modified }}</span
                >
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <div
        v-if="results.length === 0"
        class="border-border rounded-2xl border border-dashed p-12 text-center"
      >
        <p class="text-sm font-medium">No results</p>
        <p class="text-muted-foreground mt-1 text-xs">
          Try fewer keywords, remove filters, or toggle AI semantic search.
        </p>
      </div>
    </div>
  </div>
</template>
