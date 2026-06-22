<script setup lang="ts">
  import {
    FileTextIcon,
    ArrowLeftIcon,
    SparklesIcon,
    MessageSquareIcon,
    PenLineIcon,
    StampIcon,
    SendIcon,
    HistoryIcon,
    CloudIcon,
    GitBranchIcon,
    ShieldAlertIcon,
    DownloadIcon,
    Share2Icon,
    Trash2Icon,
    Maximize2Icon,
    TagIcon,
  } from "@lucide/vue";
  import { computed, ref } from "vue";
  import { useRoute, useRouter } from "vue-router";
  import { toast } from "vue-sonner";

  import FileTypeIcon from "@/components/FileTypeIcon.vue";
  import RouteDialog from "@/components/RouteDialog.vue";
  import ShareDialog from "@/components/ShareDialog.vue";
  import SignDialog from "@/components/SignDialog.vue";
  import StatusBadges from "@/components/StatusBadges.vue";
  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Dialog, DialogContent } from "@/components/ui/dialog";
  import { Input } from "@/components/ui/input";
  import { Separator } from "@/components/ui/separator";
  import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
  import { Textarea } from "@/components/ui/textarea";
  import { documents, type Doc, activityFeed } from "@/data/mock-data";
  import { downloadDoc } from "@/lib/download";
  import {
    useOfficeState,
    addComment,
    resolveComment,
    deleteComment,
    commentsForDoc,
    signaturesForDoc,
    stampsForDoc,
    slipsForDoc,
    currentUser,
    type Comment,
    STAMP_META,
  } from "@/lib/office-store";

  definePageMeta({
    title: "Document",
    breadcrumbs: [{ label: "Documents", to: "/documents" }, { label: "Document" }],
  });

  useHead({
    title: "Document — Lumen DMS",
  });

  const route = useRoute();
  const router = useRouter();
  const office = useOfficeState();
  const me = currentUser();

  const docId = computed(() => route.params.id as string);
  const doc = computed(() => documents.find((d) => d.id === docId.value));
  const docComments = computed(() => commentsForDoc(docId.value, office.value));
  const docSignatures = computed(() => signaturesForDoc(docId.value, office.value));
  const docStamps = computed(() => stampsForDoc(docId.value, office.value));
  const docSlips = computed(() => slipsForDoc(docId.value, office.value));

  const shareOpen = ref(false);
  const signOpen = ref(false);
  const routeOpen = ref(false);
  const fullscreen = ref(false);

  const roots = computed(() => docComments.value.filter((c) => !c.parentId));

  function childrenOf(id: string) {
    return docComments.value.filter((c) => c.parentId === id);
  }

  const commentText = ref("");
  const replyTextMap = ref<Record<string, string>>({});

  function submitComment() {
    if (!commentText.value.trim()) return;
    addComment(docId.value, commentText.value.trim());
    commentText.value = "";
  }

  function submitReply(parentId: string) {
    const text = replyTextMap.value[parentId];
    if (!text?.trim()) return;
    addComment(docId.value, text.trim(), parentId);
    replyTextMap.value[parentId] = "";
  }
</script>

<template>
  <div v-if="!doc" class="border-border rounded-xl border border-dashed p-12 text-center">
    <FileTextIcon class="text-muted-foreground/40 mx-auto h-8 w-8" />
    <p class="mt-2 text-sm font-medium">Document not found</p>
    <Button variant="outline" size="sm" class="mt-3" @click="router.push('/documents')"
      >Back to documents</Button
    >
  </div>

  <template v-else>
    <div class="mb-4 flex flex-wrap items-center gap-2">
      <Button
        variant="ghost"
        size="icon"
        class="h-8 w-8 shrink-0"
        @click="router.push('/documents')"
      >
        <ArrowLeftIcon class="h-4 w-4" />
      </Button>
      <Separator orientation="vertical" class="h-5" />
      <FileTypeIcon :type="doc.type" class="text-muted-foreground h-5 w-5 shrink-0" />
      <h2 class="min-w-0 truncate text-lg font-semibold">{{ doc.name }}</h2>
      <StatusBadges :status="doc.status" />
      <StatusBadges :source="doc.source" :library="doc.library" />
      <div class="ml-auto flex items-center gap-1">
        <Button
          variant="ghost"
          size="icon"
          class="h-8 w-8"
          aria-label="Fullscreen"
          @click="fullscreen = true"
        >
          <Maximize2Icon class="h-4 w-4" />
        </Button>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-6 lg:grid-cols-[1fr_400px]">
      <div class="min-w-0 space-y-6">
        <div
          class="border-border bg-card shadow-elegant relative overflow-hidden rounded-xl border"
        >
          <div
            class="border-border bg-secondary/40 flex items-center justify-between border-b px-3 py-2"
          >
            <div class="flex min-w-0 items-center gap-2">
              <FileTypeIcon :type="doc.type" size="sm" />
              <span class="text-muted-foreground truncate text-[11px]">{{ doc.name }}</span>
            </div>
            <span class="text-muted-foreground shrink-0 text-[10px]"
              >Page 1 / {{ doc.versions ?? 3 }}</span
            >
          </div>
          <div class="bg-card relative p-5 sm:p-8">
            <div
              v-if="docStamps.length > 0"
              class="absolute top-6 right-6 flex -rotate-6 flex-wrap gap-2"
            >
              <div
                v-for="s in docStamps"
                :key="s.id"
                :class="`rounded px-2 py-0.5 text-[10px] font-bold tracking-widest ring-2 ${STAMP_META[s.kind].ring}`"
              >
                {{ STAMP_META[s.kind].label }}
              </div>
            </div>
            <h4 class="text-base font-semibold tracking-tight sm:text-lg">
              {{ doc.name.replace(/\.[^.]+$/, "").replace(/_/g, " ") }}
            </h4>
            <p class="text-muted-foreground mt-1 text-[11px] tracking-wider uppercase">
              {{ doc.library ?? "Personal" }} · {{ doc.type.toUpperCase() }}
            </p>
            <div class="mt-4 space-y-2">
              <div class="bg-secondary h-1.5 w-11/12 rounded" />
              <div class="bg-secondary h-1.5 w-10/12 rounded" />
              <div class="bg-secondary h-1.5 w-9/12 rounded" />
              <div class="bg-secondary h-1.5 w-11/12 rounded" />
              <div class="bg-secondary h-1.5 w-7/12 rounded" />
            </div>
            <div
              v-if="doc.aiSummary"
              class="border-border bg-secondary/30 mt-4 rounded-lg border p-3"
            >
              <p class="mb-1 text-[11px] font-medium">Excerpt</p>
              <p class="text-foreground/80 line-clamp-4 text-xs leading-relaxed">
                {{ doc.aiSummary }}
              </p>
            </div>
            <div v-if="docSignatures.length > 0" class="mt-4 space-y-2">
              <div
                v-for="s in docSignatures"
                :key="s.id"
                class="border-border flex items-center justify-between gap-3 rounded-lg border p-2"
              >
                <div class="min-w-0">
                  <p class="truncate text-xs font-medium">
                    {{ s.signer }}
                    <span v-if="s.role" class="text-muted-foreground font-normal"
                      >· {{ s.role }}</span
                    >
                  </p>
                  <p class="text-muted-foreground text-[10px]">
                    Signed {{ new Date(s.signedAt).toLocaleString() }}
                  </p>
                </div>
                <img
                  v-if="s.kind === 'drawn'"
                  :src="s.value"
                  alt="signature"
                  class="h-7 max-w-[100px] shrink-0 object-contain"
                />
                <div
                  v-else-if="s.kind === 'initials'"
                  class="border-primary/60 text-primary grid h-7 w-7 shrink-0 place-items-center rounded-full border-2 text-[10px] font-semibold tracking-wider"
                >
                  {{ s.value }}
                </div>
                <div
                  v-else
                  class="text-foreground/90 shrink-0 text-lg"
                  :style="{ fontFamily: s.font }"
                >
                  {{ s.value }}
                </div>
              </div>
            </div>
            <div class="mt-4 flex flex-wrap items-center gap-2">
              <Button size="sm" variant="outline" class="gap-1.5" @click="downloadDoc(doc)"
                ><DownloadIcon class="h-3.5 w-3.5" /> Download</Button
              >
              <Button size="sm" variant="outline" class="gap-1.5"
                ><FileTextIcon class="h-3.5 w-3.5" /> Open</Button
              >
            </div>
          </div>
        </div>

        <div class="border-border bg-card divide-border/50 divide-y rounded-xl border p-4">
          <div class="pb-3">
            <h3 class="text-muted-foreground mb-2 text-xs font-semibold tracking-wider uppercase">
              Metadata
            </h3>
            <div class="grid grid-cols-2 gap-x-4 gap-y-1.5 text-xs">
              <div class="flex justify-between">
                <span class="text-muted-foreground">Type</span>
                <span class="font-medium uppercase">{{ doc.type }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">Size</span>
                <span class="font-medium">{{ doc.size }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">Owner</span>
                <span class="font-medium">{{ doc.owner }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">Modified</span>
                <span class="font-medium">{{ doc.modified }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">Versions</span>
                <span class="font-medium">{{ doc.versions ?? 1 }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">Folder</span>
                <span class="max-w-[100px] truncate font-medium">{{ doc.folder ?? "—" }}</span>
              </div>
            </div>
          </div>
          <div class="pt-3">
            <p class="text-muted-foreground mb-1.5 text-xs">Tags</p>
            <div class="flex flex-wrap gap-1">
              <Badge
                v-for="t in doc.tags"
                :key="t"
                variant="secondary"
                class="gap-1 border-0 text-[10px]"
              >
                <TagIcon class="h-2.5 w-2.5" /> {{ t }}
              </Badge>
            </div>
          </div>
        </div>

        <div class="border-border bg-card rounded-xl border p-5">
          <div class="mb-3 flex items-center gap-2">
            <SparklesIcon class="text-primary h-4 w-4" />
            <h3 class="text-sm font-semibold">AI summary</h3>
          </div>
          <p class="text-muted-foreground text-sm leading-relaxed">{{ doc.aiSummary }}</p>
          <div v-if="doc.entities?.length" class="mt-3 flex flex-wrap gap-1.5">
            <Badge
              v-for="e in doc.entities"
              :key="e"
              variant="secondary"
              class="bg-primary/5 text-primary border-0 text-[10px]"
              >{{ e }}</Badge
            >
          </div>
          <Separator class="my-3" />
          <div class="space-y-2.5">
            <div v-for="a in activityFeed.slice(0, 4)" :key="a.id" class="flex items-start gap-2.5">
              <Avatar class="h-6 w-6 shrink-0"
                ><AvatarFallback class="bg-secondary text-[10px]">{{
                  a.initials
                }}</AvatarFallback></Avatar
              >
              <div class="min-w-0 text-xs leading-snug">
                <span class="font-medium">{{ a.who }}</span>
                <span class="text-muted-foreground"> {{ a.action }} </span>
                <span class="font-medium">{{ a.target }}</span>
                <div class="text-muted-foreground text-[11px]">{{ a.when }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="space-y-4">
        <div class="grid grid-cols-4 gap-1.5">
          <button
            @click="signOpen = true"
            class="border-border bg-card hover:bg-secondary/60 hover:border-primary/40 group flex flex-col items-center justify-center gap-1 rounded-lg border px-2 py-2.5 transition"
          >
            <PenLineIcon
              class="text-muted-foreground group-hover:text-primary h-4 w-4 transition"
            />
            <span class="text-[10px] font-medium">Sign</span>
          </button>
          <button
            @click="routeOpen = true"
            class="border-border bg-card hover:bg-secondary/60 hover:border-primary/40 group flex flex-col items-center justify-center gap-1 rounded-lg border px-2 py-2.5 transition"
          >
            <SendIcon class="text-muted-foreground group-hover:text-primary h-4 w-4 transition" />
            <span class="text-[10px] font-medium">Route</span>
          </button>
          <button
            @click="shareOpen = true"
            class="border-border bg-card hover:bg-secondary/60 hover:border-primary/40 group flex flex-col items-center justify-center gap-1 rounded-lg border px-2 py-2.5 transition"
          >
            <Share2Icon class="text-muted-foreground group-hover:text-primary h-4 w-4 transition" />
            <span class="text-[10px] font-medium">Share</span>
          </button>
          <button
            @click="downloadDoc(doc)"
            class="border-border bg-card hover:bg-secondary/60 hover:border-primary/40 group flex flex-col items-center justify-center gap-1 rounded-lg border px-2 py-2.5 transition"
          >
            <DownloadIcon
              class="text-muted-foreground group-hover:text-primary h-4 w-4 transition"
            />
            <span class="text-[10px] font-medium">Download</span>
          </button>
        </div>

        <Tabs default-value="office" class="w-full">
          <TabsList class="bg-secondary/60 grid h-9 w-full grid-cols-4">
            <TabsTrigger value="office" class="text-xs">Office</TabsTrigger>
            <TabsTrigger value="comments" class="text-xs">
              Comments<span
                v-if="docComments.length > 0"
                class="text-muted-foreground ml-1 text-[10px]"
                >{{ docComments.length }}</span
              >
            </TabsTrigger>
            <TabsTrigger value="ai" class="text-xs">AI</TabsTrigger>
            <TabsTrigger value="meta" class="text-xs">Meta</TabsTrigger>
          </TabsList>

          <TabsContent value="office" class="mt-3 space-y-3">
            <div
              v-if="docSlips.length === 0 && docSignatures.length === 0 && docStamps.length === 0"
              class="border-border rounded-xl border border-dashed p-5 text-center"
            >
              <div
                class="bg-primary/10 text-primary mx-auto mb-2 grid h-10 w-10 place-items-center rounded-full"
              >
                <FileTextIcon class="h-5 w-5" />
              </div>
              <p class="text-xs font-medium">No office actions yet</p>
              <p class="text-muted-foreground mb-3 text-[11px]">
                Send for signature or route through the office.
              </p>
              <div class="flex justify-center gap-2">
                <Button size="sm" variant="outline" class="gap-1.5" @click="signOpen = true"
                  ><PenLineIcon class="h-3.5 w-3.5" /> Sign</Button
                >
                <Button size="sm" class="gap-1.5" @click="routeOpen = true"
                  ><SendIcon class="h-3.5 w-3.5" /> Route</Button
                >
              </div>
            </div>
            <template v-else>
              <div
                v-for="slip in docSlips"
                :key="slip.id"
                class="border-border rounded-xl border p-3"
              >
                <div class="mb-2 flex items-center justify-between gap-2">
                  <div class="truncate text-xs font-medium">{{ slip.subject }}</div>
                  <Badge variant="secondary" class="shrink-0 border-0 text-[10px] capitalize">{{
                    slip.status.replace("_", " ")
                  }}</Badge>
                </div>
                <p class="text-muted-foreground mb-2 text-[11px]">
                  From {{ slip.origin }}{{ slip.dueDate ? ` · due ${slip.dueDate}` : "" }}
                </p>
                <ol class="space-y-1.5">
                  <li v-for="(s, i) in slip.steps" :key="s.id" class="flex items-center gap-2">
                    <div
                      :class="`grid h-5 w-5 shrink-0 place-items-center rounded-full text-[10px] font-medium ${
                        s.status === 'approved' || s.status === 'signed' || s.status === 'forwarded'
                          ? 'bg-success/15 text-success'
                          : s.status === 'active'
                            ? 'bg-primary/15 text-primary ring-primary/30 ring-2'
                            : s.status === 'rejected'
                              ? 'bg-destructive/15 text-destructive'
                              : 'bg-secondary text-muted-foreground'
                      }`"
                    >
                      {{ i + 1 }}
                    </div>
                    <span class="truncate text-xs font-medium">{{ s.recipient }}</span>
                    <span class="text-muted-foreground hidden truncate text-[10px] sm:inline"
                      >— {{ s.role }}</span
                    >
                    <span class="text-muted-foreground ml-auto shrink-0 text-[10px] capitalize">{{
                      s.action
                    }}</span>
                  </li>
                </ol>
              </div>

              <div v-if="docStamps.length > 0" class="border-border rounded-xl border p-3">
                <div class="mb-2 flex items-center gap-1.5 text-xs font-medium">
                  <StampIcon class="h-3.5 w-3.5" /> Stamps
                </div>
                <div class="flex flex-wrap gap-2">
                  <div
                    v-for="s in docStamps"
                    :key="s.id"
                    :class="`-rotate-3 rounded px-2 py-1 text-[10px] font-bold tracking-widest ring-2 ${STAMP_META[s.kind].ring}`"
                  >
                    {{ STAMP_META[s.kind].label }}
                  </div>
                </div>
              </div>

              <div v-if="docSignatures.length > 0" class="border-border rounded-xl border p-3">
                <div class="mb-2 flex items-center gap-1.5 text-xs font-medium">
                  <PenLineIcon class="h-3.5 w-3.5" /> Signatures
                </div>
                <div v-for="s in docSignatures" :key="s.id" class="space-y-2">
                  <div class="flex items-center justify-between gap-3">
                    <div class="min-w-0">
                      <p class="truncate text-xs font-medium">
                        {{ s.signer }}
                        <span v-if="s.role" class="text-muted-foreground font-normal"
                          >· {{ s.role }}</span
                        >
                      </p>
                      <p class="text-muted-foreground text-[10px]">
                        Signed {{ new Date(s.signedAt).toLocaleString() }}
                      </p>
                    </div>
                    <img
                      v-if="s.kind === 'drawn'"
                      :src="s.value"
                      alt="signature"
                      class="h-8 max-w-[120px] shrink-0 object-contain"
                    />
                    <div
                      v-else-if="s.kind === 'initials'"
                      class="border-primary/60 text-primary grid h-8 w-8 shrink-0 place-items-center rounded-full border-2 text-[11px] font-semibold tracking-wider"
                    >
                      {{ s.value }}
                    </div>
                    <div
                      v-else
                      class="text-foreground/90 shrink-0 text-xl"
                      :style="{ fontFamily: s.font }"
                    >
                      {{ s.value }}
                    </div>
                  </div>
                </div>
              </div>
            </template>
          </TabsContent>

          <TabsContent value="comments" class="mt-3 space-y-2">
            <div class="space-y-3">
              <p
                v-if="roots.length === 0"
                class="text-muted-foreground py-4 text-center text-[11px]"
              >
                No comments yet — start the conversation.
              </p>
              <div
                v-for="c in roots"
                :key="c.id"
                :class="
                  'rounded-lg border p-2.5 ' +
                  (c.resolved ? 'border-success/30 bg-success/5' : 'border-border')
                "
              >
                <div class="flex items-start gap-2">
                  <Avatar class="h-6 w-6 shrink-0"
                    ><AvatarFallback class="bg-secondary text-[10px]">{{
                      c.initials
                    }}</AvatarFallback></Avatar
                  >
                  <div class="min-w-0 flex-1">
                    <div class="flex flex-wrap items-center gap-1.5">
                      <span class="text-xs font-medium">{{ c.author }}</span>
                      <span class="text-muted-foreground text-[10px]">{{
                        new Date(c.createdAt).toLocaleDateString()
                      }}</span>
                    </div>
                    <p class="mt-0.5 text-xs">{{ c.text }}</p>
                    <div class="mt-1 flex items-center gap-2">
                      <button
                        class="text-muted-foreground hover:text-foreground text-[10px]"
                        @click="replyTextMap[c.id] = replyTextMap[c.id] ?? ''"
                      >
                        Reply
                      </button>
                      <button
                        v-if="!c.resolved"
                        class="text-muted-foreground hover:text-success text-[10px]"
                        @click="resolveComment(c.id)"
                      >
                        Resolve
                      </button>
                      <button
                        class="text-muted-foreground hover:text-destructive text-[10px]"
                        @click="deleteComment(c.id)"
                      >
                        Delete
                      </button>
                    </div>
                    <div v-if="replyTextMap[c.id] !== undefined" class="mt-2 flex gap-1.5">
                      <Input
                        v-model="replyTextMap[c.id]"
                        placeholder="Reply…"
                        class="h-7 text-xs"
                        @keydown.enter="submitReply(c.id)"
                      />
                      <Button size="icon" class="h-7 w-7 shrink-0" @click="submitReply(c.id)"
                        ><SendIcon class="h-3 w-3"
                      /></Button>
                    </div>
                  </div>
                </div>
                <div
                  v-for="ch in childrenOf(c.id)"
                  :key="ch.id"
                  class="border-border mt-2 border-l-2 pl-3"
                >
                  <div class="flex items-start gap-2">
                    <Avatar class="h-6 w-6 shrink-0"
                      ><AvatarFallback class="bg-secondary text-[10px]">{{
                        ch.initials
                      }}</AvatarFallback></Avatar
                    >
                    <div class="min-w-0 flex-1">
                      <div class="flex items-center gap-1.5">
                        <span class="text-xs font-medium">{{ ch.author }}</span>
                        <span class="text-muted-foreground text-[10px]">{{
                          new Date(ch.createdAt).toLocaleDateString()
                        }}</span>
                      </div>
                      <p class="mt-0.5 text-xs">{{ ch.text }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="border-border flex items-start gap-2 border-t pt-1">
                <Avatar class="mt-1 h-7 w-7 shrink-0"
                  ><AvatarFallback class="bg-primary/10 text-primary text-[10px]">{{
                    me.initials
                  }}</AvatarFallback></Avatar
                >
                <div class="flex flex-1 gap-1.5">
                  <Input
                    v-model="commentText"
                    placeholder="Comment… use @ to mention"
                    class="h-8 text-xs"
                    @keydown.enter="submitComment"
                  />
                  <Button size="icon" class="h-8 w-8 shrink-0" @click="submitComment"
                    ><SendIcon class="h-3.5 w-3.5"
                  /></Button>
                </div>
              </div>
            </div>
          </TabsContent>

          <TabsContent value="ai" class="mt-3 space-y-3">
            <div v-if="doc.aiSummary" class="border-primary/20 bg-primary/5 rounded-xl border p-3">
              <div class="text-primary mb-1.5 flex items-center gap-1.5 text-xs font-medium">
                <SparklesIcon class="h-3.5 w-3.5" /> AI Summary
              </div>
              <p class="text-foreground/90 text-xs leading-relaxed">{{ doc.aiSummary }}</p>
            </div>
            <div
              v-if="doc.entities && doc.entities.length > 0"
              class="border-border rounded-xl border p-3"
            >
              <p class="mb-2 text-xs font-medium">Entities</p>
              <div class="flex flex-wrap gap-1">
                <Badge
                  v-for="e in doc.entities"
                  :key="e"
                  variant="secondary"
                  class="bg-secondary h-5 border-0 px-1.5 text-[10px]"
                  >{{ e }}</Badge
                >
              </div>
            </div>
            <div
              v-if="doc.status === 'review'"
              class="border-warning/30 bg-warning/5 rounded-xl border p-3"
            >
              <div class="text-warning mb-1 flex items-center gap-1.5 text-xs font-medium">
                <ShieldAlertIcon class="h-3.5 w-3.5" /> Contract risk
              </div>
              <p class="text-foreground/80 text-xs">
                Liability cap may be below policy. Confidence 79%.
              </p>
            </div>
            <Button
              variant="outline"
              size="sm"
              class="w-full gap-1.5"
              @click="toast.info('Ask AI — opening conversation')"
            >
              <SparklesIcon class="text-primary h-3.5 w-3.5" /> Ask AI about this document
            </Button>
          </TabsContent>

          <TabsContent value="meta" class="mt-3 space-y-2.5 text-xs">
            <div class="flex items-start justify-between gap-3">
              <span class="text-muted-foreground shrink-0">Owner</span>
              <span class="max-w-[60%] truncate text-right font-medium">{{ doc.owner }}</span>
            </div>
            <div class="flex items-start justify-between gap-3">
              <span class="text-muted-foreground shrink-0">Folder</span>
              <span class="max-w-[60%] truncate text-right font-medium">{{
                doc.folder ?? "—"
              }}</span>
            </div>
            <div class="flex items-start justify-between gap-3">
              <span class="text-muted-foreground shrink-0">Type</span>
              <span class="max-w-[60%] truncate text-right font-medium">{{
                doc.type.toUpperCase()
              }}</span>
            </div>
            <div class="flex items-start justify-between gap-3">
              <span class="text-muted-foreground shrink-0">Size</span>
              <span class="max-w-[60%] truncate text-right font-medium">{{ doc.size }}</span>
            </div>
            <div class="flex items-start justify-between gap-3">
              <span class="text-muted-foreground shrink-0">Modified</span>
              <span class="max-w-[60%] truncate text-right font-medium">{{ doc.modified }}</span>
            </div>
            <div class="flex items-start justify-between gap-3">
              <span class="text-muted-foreground shrink-0">Tags</span>
              <div class="flex max-w-[60%] flex-wrap justify-end gap-1">
                <Badge
                  v-for="t in doc.tags"
                  :key="t"
                  variant="secondary"
                  class="border-0 text-[10px]"
                  >{{ t }}</Badge
                >
              </div>
            </div>
            <template v-if="doc.source === 'sharepoint'">
              <Separator class="my-2" />
              <div class="border-info/20 bg-info/5 rounded-lg border p-2.5">
                <div class="text-info mb-1 flex items-center gap-1.5 text-xs font-medium">
                  <CloudIcon class="h-3 w-3" /> SharePoint sync
                </div>
                <p class="text-muted-foreground text-[11px]">
                  Library: {{ doc.library }} · Synced 2m ago
                </p>
              </div>
            </template>
            <Separator class="my-2" />
            <div class="text-muted-foreground mb-1 flex items-center gap-1.5 text-xs font-medium">
              <GitBranchIcon class="h-3.5 w-3.5" /> Versions
            </div>
            <div
              v-for="(_, i) in Array(doc.versions ?? 3)"
              :key="i"
              class="flex justify-between text-[11px]"
            >
              <span>v{{ (doc.versions ?? 3) - i }}</span>
              <span class="text-muted-foreground">{{
                i === 0 ? doc.modified : `${i * 3}d ago`
              }}</span>
            </div>
          </TabsContent>
        </Tabs>

        <div class="border-border bg-card rounded-xl border p-4">
          <div class="text-muted-foreground mb-3 flex items-center gap-1.5 text-xs font-medium">
            <HistoryIcon class="h-3.5 w-3.5" /> Recent activity
          </div>
          <div class="space-y-2.5">
            <div v-for="a in activityFeed.slice(0, 4)" :key="a.id" class="flex items-start gap-2.5">
              <Avatar class="h-6 w-6 shrink-0"
                ><AvatarFallback class="bg-secondary text-[10px]">{{
                  a.initials
                }}</AvatarFallback></Avatar
              >
              <div class="min-w-0 text-xs leading-snug">
                <span class="font-medium">{{ a.who }}</span>
                <span class="text-muted-foreground"> {{ a.action }} </span>
                <span class="font-medium">{{ a.target }}</span>
                <div class="text-muted-foreground text-[11px]">{{ a.when }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <ShareDialog :open="shareOpen" :title="doc.name" @update:open="shareOpen = $event" />
    <RouteDialog
      :open="routeOpen"
      :doc-id="doc.id"
      :doc-name="doc.name"
      @update:open="routeOpen = $event"
    />
    <SignDialog
      :open="signOpen"
      :doc-id="doc.id"
      :doc-name="doc.name"
      @update:open="signOpen = $event"
    />

    <Dialog :open="fullscreen" @update:open="fullscreen = $event">
      <DialogContent class="w-[calc(100%-1rem)] max-w-5xl overflow-hidden p-0">
        <div class="border-border flex h-12 items-center justify-between border-b px-4">
          <div class="flex min-w-0 items-center gap-2">
            <FileTypeIcon :type="doc.type" size="sm" />
            <span class="truncate text-sm font-medium">{{ doc.name }}</span>
          </div>
          <div class="flex items-center gap-1">
            <Button size="sm" variant="outline" class="gap-1.5" @click="signOpen = true"
              ><PenLineIcon class="h-3.5 w-3.5" /> Sign</Button
            >
            <Button size="sm" variant="outline" class="gap-1.5" @click="routeOpen = true"
              ><SendIcon class="h-3.5 w-3.5" /> Route</Button
            >
            <Button size="sm" variant="outline" class="gap-1.5" @click="shareOpen = true"
              ><Share2Icon class="h-3.5 w-3.5" /> Share</Button
            >
            <Button size="sm" class="gap-1.5" @click="downloadDoc(doc)"
              ><DownloadIcon class="h-3.5 w-3.5" /> Download</Button
            >
          </div>
        </div>
        <div class="bg-secondary/40 max-h-[80vh] overflow-y-auto p-4 sm:p-8">
          <div class="mx-auto max-w-2xl">
            <div
              class="border-border bg-card shadow-elegant relative overflow-hidden rounded-xl border"
            >
              <div
                class="border-border bg-secondary/40 flex items-center justify-between border-b px-3 py-2"
              >
                <div class="flex items-center gap-2">
                  <FileTypeIcon :type="doc.type" size="sm" />
                  <span class="text-muted-foreground truncate text-[11px]">{{ doc.name }}</span>
                </div>
                <span class="text-muted-foreground text-[10px]"
                  >Page 1 / {{ doc.versions ?? 3 }}</span
                >
              </div>
              <div class="bg-card relative p-6 sm:p-10">
                <div
                  v-if="docStamps.length > 0"
                  class="absolute top-8 right-8 flex -rotate-6 flex-wrap gap-2"
                >
                  <div
                    v-for="s in docStamps"
                    :key="s.id"
                    :class="`rounded px-3 py-1 text-xs font-bold tracking-widest ring-2 ${STAMP_META[s.kind].ring}`"
                  >
                    {{ STAMP_META[s.kind].label }}
                  </div>
                </div>
                <h4 class="text-xl font-semibold tracking-tight sm:text-2xl">
                  {{ doc.name.replace(/\.[^.]+$/, "").replace(/_/g, " ") }}
                </h4>
                <p class="text-muted-foreground mt-1 text-[11px] tracking-wider uppercase">
                  {{ doc.library ?? "Personal" }} · {{ doc.type.toUpperCase() }}
                </p>
                <div class="mt-6 space-y-2">
                  <div class="bg-secondary h-2 w-11/12 rounded" />
                  <div class="bg-secondary h-2 w-10/12 rounded" />
                  <div class="bg-secondary h-2 w-9/12 rounded" />
                  <div class="bg-secondary h-2 w-11/12 rounded" />
                  <div class="bg-secondary h-2 w-7/12 rounded" />
                  <div class="bg-secondary h-2 w-8/12 rounded" />
                </div>
                <div
                  v-if="doc.aiSummary"
                  class="border-border bg-secondary/30 mt-6 rounded-lg border p-4"
                >
                  <p class="mb-1 text-sm font-medium">Excerpt</p>
                  <p class="text-foreground/80 text-sm leading-relaxed">{{ doc.aiSummary }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  </template>
</template>
