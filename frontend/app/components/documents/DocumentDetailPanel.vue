<script setup lang="ts">
  import {
    DownloadIcon,
    Share2Icon,
    XIcon,
    SparklesIcon,
    HistoryIcon,
    FileTextIcon,
    CloudIcon,
    Maximize2Icon,
    GitBranchIcon,
    ShieldAlertIcon,
    SendIcon,
    PenLineIcon,
    StampIcon,
    MoreHorizontalIcon,
  } from "@lucide/vue";
  import { ref, computed } from "vue";
  import { toast } from "vue-sonner";

  import RouteDialog from "@/components/documents/RouteDialog.vue";
  import ShareDialog from "@/components/documents/ShareDialog.vue";
  import SignDialog from "@/components/documents/SignDialog.vue";
  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Dialog, DialogContent } from "@/components/ui/dialog";
  import { Input } from "@/components/ui/input";
  import { Separator } from "@/components/ui/separator";
  import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
  import type { Doc } from "@/data/mock-data";
  import { activityFeed } from "@/data/mock-data";
  import {
    addComment,
    commentsForDoc,
    deleteComment,
    resolveComment,
    signaturesForDoc,
    slipsForDoc,
    stampsForDoc,
    useOfficeState,
    STAMP_META,
    currentUser,
  } from "@/lib/office-store";

  const props = defineProps<{ doc: Doc }>();
  const emit = defineEmits<{ close: [] }>();

  const shareOpen = ref(false);
  const signOpen = ref(false);
  const routeOpen = ref(false);
  const fullscreen = ref(false);

  const office = useOfficeState();
  const docComments = computed(() => commentsForDoc(props.doc.id, office.value));
  const docSignatures = computed(() => signaturesForDoc(props.doc.id, office.value));
  const docStamps = computed(() => stampsForDoc(props.doc.id, office.value));
  const docSlips = computed(() => slipsForDoc(props.doc.id, office.value));

  const me = currentUser();
  const commentText = ref("");
  const replyTextMap = ref<Record<string, string>>({});

  const roots = computed(() => docComments.value.filter((c: any) => !c.parentId));

  const childrenOf = (id: string) => {
    return docComments.value.filter((c: any) => c.parentId === id);
  };

  const submitComment = () => {
    if (!commentText.value.trim()) return;
    addComment(props.doc.id, commentText.value);
    commentText.value = "";
  };

  const submitReply = (parentId: string) => {
    const text = replyTextMap.value[parentId];
    if (!text?.trim()) return;
    addComment(props.doc.id, text, parentId);
    replyTextMap.value[parentId] = "";
  };

  const aiNames = ["MSA_Acme_Corp_v4.pdf", "RFC-014_Event_Driven.pdf", "Vendor_Risk_Vantage.pdf"];
</script>

<template>
  <div class="flex min-h-0 flex-col">
    <div class="border-border flex h-12 shrink-0 items-center justify-between border-b px-4">
      <span class="text-muted-foreground text-xs font-medium tracking-wider uppercase"
        >Document details</span
      >
      <div class="flex items-center gap-0.5">
        <Button
          variant="ghost"
          size="icon"
          class="h-7 w-7"
          aria-label="Fullscreen"
          @click="fullscreen = true"
        >
          <Maximize2Icon class="h-3.5 w-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          class="h-7 w-7"
          @click="emit('close')"
          aria-label="Close"
        >
          <XIcon class="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>

    <div class="flex-1 overflow-y-auto">
      <div class="m-4">
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
            <span class="text-muted-foreground text-[10px]">Page 1 / {{ doc.versions ?? 3 }}</span>
          </div>
          <div class="bg-card relative p-5">
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
            <h4 class="text-sm font-semibold tracking-tight">
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
                  class="h-7 max-w-[100px] object-contain"
                />
                <div
                  v-else-if="s.kind === 'initials'"
                  class="border-primary/60 text-primary grid h-7 w-7 place-items-center rounded-full border-2 text-[10px] font-semibold tracking-wider"
                >
                  {{ s.value }}
                </div>
                <div v-else class="text-foreground/90 text-lg" :style="{ fontFamily: s.font }">
                  {{ s.value }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="px-4">
        <h3 class="text-base font-semibold tracking-tight">{{ doc.name }}</h3>
        <p class="text-muted-foreground mt-1 text-xs">
          {{ doc.size }} · v{{ doc.versions }} · modified {{ doc.modified }}
        </p>

        <div class="mt-3 flex flex-wrap gap-1">
          <StatusBadges :status="doc.status" />
          <StatusBadges :source="doc.source" :library="doc.library" />
        </div>

        <div class="mt-4 grid grid-cols-4 gap-1.5">
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
            @click="toast.success(`Downloading ${doc.name}`)"
            class="border-border bg-card hover:bg-secondary/60 hover:border-primary/40 group flex flex-col items-center justify-center gap-1 rounded-lg border px-2 py-2.5 transition"
          >
            <DownloadIcon
              class="text-muted-foreground group-hover:text-primary h-4 w-4 transition"
            />
            <span class="text-[10px] font-medium">Download</span>
          </button>
        </div>
      </div>

      <Separator class="my-4" />

      <div class="px-4">
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
                <div class="mb-2 flex items-center justify-between">
                  <div class="text-xs font-medium">{{ slip.subject }}</div>
                  <Badge variant="secondary" class="border-0 text-[10px] capitalize">{{
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
                    <span class="text-muted-foreground truncate text-[10px]">— {{ s.role }}</span>
                    <span class="text-muted-foreground ml-auto text-[10px] capitalize">{{
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
                      class="h-8 max-w-[120px] object-contain"
                    />
                    <div
                      v-else-if="s.kind === 'initials'"
                      class="border-primary/60 text-primary grid h-8 w-8 place-items-center rounded-full border-2 text-[11px] font-semibold tracking-wider"
                    >
                      {{ s.value }}
                    </div>
                    <div v-else class="text-foreground/90 text-xl" :style="{ fontFamily: s.font }">
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
                  <Avatar class="h-6 w-6"
                    ><AvatarFallback class="bg-secondary text-[10px]">{{
                      c.initials
                    }}</AvatarFallback></Avatar
                  >
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-1.5">
                      <span class="text-xs font-medium">{{ c.author }}</span>
                      <span class="text-muted-foreground text-[10px]">{{ c.time }}</span>
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
                        @click="deleteComment(props.doc.id, c.id)"
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
                    <Avatar class="h-6 w-6"
                      ><AvatarFallback class="bg-secondary text-[10px]">{{
                        ch.initials
                      }}</AvatarFallback></Avatar
                    >
                    <div class="min-w-0 flex-1">
                      <div class="flex items-center gap-1.5">
                        <span class="text-xs font-medium">{{ ch.author }}</span>
                        <span class="text-muted-foreground text-[10px]">{{ ch.time }}</span>
                      </div>
                      <p class="mt-0.5 text-xs">{{ ch.text }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="border-border flex items-start gap-2 border-t pt-1">
                <Avatar class="mt-1 h-7 w-7"
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
              <span class="text-muted-foreground">Owner</span>
              <span class="max-w-[60%] truncate text-right font-medium">{{ doc.owner }}</span>
            </div>
            <div class="flex items-start justify-between gap-3">
              <span class="text-muted-foreground">Folder</span>
              <span class="max-w-[60%] truncate text-right font-medium">{{
                doc.folder ?? "—"
              }}</span>
            </div>
            <div class="flex items-start justify-between gap-3">
              <span class="text-muted-foreground">Type</span>
              <span class="max-w-[60%] truncate text-right font-medium">{{
                doc.type.toUpperCase()
              }}</span>
            </div>
            <div class="flex items-start justify-between gap-3">
              <span class="text-muted-foreground">Tags</span>
              <span class="max-w-[60%] truncate text-right font-medium">{{
                doc.tags.join(", ")
              }}</span>
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
      </div>

      <Separator class="my-4" />

      <div class="px-4 pb-6">
        <div class="text-muted-foreground mb-2 flex items-center gap-1.5 text-xs font-medium">
          <HistoryIcon class="h-3.5 w-3.5" /> Recent activity
        </div>
        <div class="space-y-2.5">
          <div v-for="a in activityFeed.slice(0, 4)" :key="a.id" class="flex items-start gap-2.5">
            <Avatar class="h-6 w-6"
              ><AvatarFallback class="bg-secondary text-[10px]">{{
                a.initials
              }}</AvatarFallback></Avatar
            >
            <div class="text-xs leading-snug">
              <span class="font-medium">{{ a.who }}</span>
              <span class="text-muted-foreground"> {{ a.action }} </span>
              <span class="font-medium">{{ a.target }}</span>
              <div class="text-muted-foreground text-[11px]">{{ a.when }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <ShareDialog :open="shareOpen" :title="doc.name" @update:open="shareOpen = $event" />
    <SignDialog
      :open="signOpen"
      :doc-id="doc.id"
      :doc-name="doc.name"
      @update:open="signOpen = $event"
    />
    <RouteDialog
      :open="routeOpen"
      :doc-id="doc.id"
      :doc-name="doc.name"
      @update:open="routeOpen = $event"
    />

    <Dialog :open="fullscreen" @update:open="fullscreen = $event">
      <DialogContent class="max-w-5xl overflow-hidden p-0">
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
            <Button size="sm" class="gap-1.5" @click="toast.success(`Downloading ${doc.name}`)"
              ><DownloadIcon class="h-3.5 w-3.5" /> Download</Button
            >
          </div>
        </div>
        <div class="bg-secondary/40 max-h-[80vh] overflow-y-auto p-8">
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
              <div class="bg-card relative p-10">
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
                <h4 class="text-2xl font-semibold tracking-tight">
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
  </div>
</template>
