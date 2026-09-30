<script setup lang="ts">
  import {
    CheckCircle2Icon,
    CornerDownRightIcon,
    MessageSquareIcon,
    MoreHorizontalIcon,
    RotateCcwIcon,
  } from "@lucide/vue";
  import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from "vue";
  import { toast } from "vue-sonner";

  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Button } from "@/components/ui/button";
  import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
  } from "@/components/ui/dropdown-menu";
  import { Textarea } from "@/components/ui/textarea";
  import type { ApiResponse, DocumentComment, Member, PersonRef } from "@/types/api";
  import { formatDateTime, initialsOf } from "@/utils/format";

  const props = defineProps<{ documentId: string; canComment: boolean }>();

  const service = commentsService();
  const { subscribe } = useRealtime();
  const comments = ref<DocumentComment[]>([]);
  const loaded = shallowRef(false);
  const showResolved = shallowRef(false);

  const threads = computed(() =>
    comments.value
      .filter((item) => !item.parent_id)
      .map((thread) => ({
        thread,
        replies: comments.value.filter((item) => item.parent_id === thread.id),
      })),
  );
  const openThreads = computed(() => threads.value.filter((item) => !item.thread.resolved));
  const visibleThreads = computed(() => (showResolved.value ? threads.value : openThreads.value));
  const resolvedCount = computed(() => threads.value.length - openThreads.value.length);

  const load = async () => {
    comments.value = (await service.list(props.documentId)) ?? [];
    loaded.value = true;
  };

  // API responses carry the viewer's own permissions; live events carry the
  // author's, so from those only the content is kept.
  const upsert = (comment: DocumentComment, live = false) => {
    const index = comments.value.findIndex((item) => item.id === comment.id);
    if (index === -1) {
      comments.value = [
        ...comments.value,
        live ? { ...comment, can_edit: false, can_resolve: false } : comment,
      ];
      return;
    }
    comments.value = comments.value.map((item) =>
      item.id !== comment.id
        ? item
        : live
          ? { ...comment, can_edit: item.can_edit, can_resolve: item.can_resolve }
          : comment,
    );
  };

  // Changes from other people arrive live.
  let stop: (() => void) | undefined;
  const listen = () => {
    stop?.();
    stop = subscribe(`document:${props.documentId}`, (event) => {
      if (event.type === "comment.created" || event.type === "comment.updated") {
        upsert(event.data as DocumentComment, true);
      } else if (event.type === "comment.deleted") {
        const { id } = event.data as { id: string };
        comments.value = comments.value.filter((item) => item.id !== id && item.parent_id !== id);
      }
    });
  };

  onMounted(() => {
    void load();
    listen();
  });
  watch(
    () => props.documentId,
    () => {
      void load();
      listen();
    },
  );
  onBeforeUnmount(() => stop?.());

  // ── Mentions ────────────────────────────────────────────────────────────
  const people = ref<PersonRef[]>([]);
  let peopleLoaded = false;
  const loadPeople = async () => {
    if (peopleLoaded) return;
    peopleLoaded = true;
    const { $api } = useNuxtApp();
    try {
      const res = await $api<ApiResponse<Member[]>>("/org/members");
      people.value = res.data
        .filter((item) => item.status === "active")
        .map((item) => ({ id: item.user_id, name: item.name || item.email, email: item.email }));
    } catch {
      people.value = [];
    }
  };

  interface Draft {
    body: string;
    mentions: PersonRef[];
  }
  const newDraft = ref<Draft>({ body: "", mentions: [] });
  const replyDrafts = ref<Record<string, Draft>>({});
  const replying = shallowRef("");
  const mentionQuery = shallowRef<{ key: string; text: string } | null>(null);

  const onInput = (key: string, draft: Draft) => {
    const match = /(?:^|\s)@([\p{L}\p{N}._-]{0,30})$/u.exec(draft.body);
    if (match) {
      void loadPeople();
      mentionQuery.value = { key, text: match[1]!.toLowerCase() };
    } else mentionQuery.value = null;
  };
  const suggestions = computed(() => {
    const query = mentionQuery.value;
    if (!query) return [];
    return people.value
      .filter((person) => `${person.name} ${person.email}`.toLowerCase().includes(query.text))
      .slice(0, 6);
  });
  const pickMention = (draft: Draft, person: PersonRef) => {
    draft.body = draft.body.replace(/@([\p{L}\p{N}._-]{0,30})$/u, `@${person.name} `);
    if (!draft.mentions.some((item) => item.id === person.id)) draft.mentions.push(person);
    mentionQuery.value = null;
  };

  const mentionIds = (draft: Draft) =>
    draft.mentions
      .filter((person) => draft.body.includes(`@${person.name}`))
      .map((person) => person.id);

  const submit = async (parentId?: string) => {
    const draft = parentId ? replyDrafts.value[parentId] : newDraft.value;
    if (!draft?.body.trim()) return;
    const created = await service.create(props.documentId, {
      body: draft.body.trim(),
      parent_id: parentId,
      mentions: mentionIds(draft),
    });
    if (!created) {
      toast.error(service.error.value || "Could not post the comment");
      return;
    }
    upsert(created);
    if (parentId) {
      replyDrafts.value[parentId] = { body: "", mentions: [] };
      replying.value = "";
    } else newDraft.value = { body: "", mentions: [] };
  };

  const startReply = (id: string) => {
    replying.value = id;
    replyDrafts.value[id] ??= { body: "", mentions: [] };
  };

  const editing = shallowRef("");
  const editBody = shallowRef("");
  const startEdit = (comment: DocumentComment) => {
    editing.value = comment.id;
    editBody.value = comment.body;
  };
  const saveEdit = async () => {
    const updated = await service.edit(editing.value, editBody.value);
    if (!updated) {
      toast.error(service.error.value);
      return;
    }
    upsert(updated);
    editing.value = "";
  };

  const toggleResolved = async (comment: DocumentComment) => {
    const updated = await service.resolve(comment.id, !comment.resolved);
    if (updated) upsert(updated);
    else toast.error(service.error.value);
  };

  const remove = async (comment: DocumentComment) => {
    if (!(await service.remove(comment.id))) {
      toast.error(service.error.value);
      return;
    }
    comments.value = comments.value.filter(
      (item) => item.id !== comment.id && item.parent_id !== comment.id,
    );
  };

  // Splits a body into text and highlighted mentions.
  const parts = (comment: DocumentComment) => {
    const names = (comment.mentions ?? []).map((person) => `@${person.name}`);
    if (!names.length) return [{ text: comment.body, mention: false }];
    const escaped = names.map((name) => name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
    return comment.body
      .split(new RegExp(`(${escaped.join("|")})`, "g"))
      .filter(Boolean)
      .map((text) => ({ text, mention: names.includes(text) }));
  };
</script>

<template>
  <section
    id="comments"
    class="border-border bg-card space-y-4 rounded-xl border p-4"
    aria-label="Comments"
  >
    <div class="flex items-center justify-between gap-2">
      <p class="text-muted-foreground text-xs font-semibold tracking-wider uppercase">
        Comments<template v-if="openThreads.length"> · {{ openThreads.length }}</template>
      </p>
      <Button
        v-if="resolvedCount"
        variant="ghost"
        size="sm"
        class="h-7 text-xs"
        :aria-pressed="showResolved"
        @click="showResolved = !showResolved"
      >
        {{ showResolved ? "Hide resolved" : `Show resolved (${resolvedCount})` }}
      </Button>
    </div>

    <form v-if="canComment" class="relative space-y-2" @submit.prevent="submit()">
      <label for="new-comment" class="sr-only">Add a comment</label>
      <Textarea
        id="new-comment"
        v-model="newDraft.body"
        rows="2"
        placeholder="Add a comment… type @ to mention someone"
        class="min-h-16 resize-none text-sm"
        @input="onInput('new', newDraft)"
        @keydown.meta.enter.prevent="submit()"
        @keydown.ctrl.enter.prevent="submit()"
      />
      <ul
        v-if="mentionQuery?.key === 'new' && suggestions.length"
        class="bg-popover absolute inset-x-0 top-full z-20 mt-1 overflow-hidden rounded-lg border shadow-md"
        role="listbox"
        aria-label="People to mention"
      >
        <li v-for="person in suggestions" :key="person.id">
          <button
            type="button"
            role="option"
            @mousedown.prevent
            class="hover:bg-accent flex w-full items-center gap-2 px-3 py-2 text-left text-sm"
            @click="pickMention(newDraft, person)"
          >
            <span class="font-medium">{{ person.name }}</span>
            <span class="text-muted-foreground truncate text-xs">{{ person.email }}</span>
          </button>
        </li>
      </ul>
      <div class="flex justify-end">
        <Button type="submit" size="sm" :disabled="!newDraft.body.trim() || service.busy.value"
          >Comment</Button
        >
      </div>
    </form>

    <p
      v-if="loaded && !visibleThreads.length"
      class="text-muted-foreground flex items-center gap-2 text-sm"
    >
      <MessageSquareIcon class="size-4" />
      {{ threads.length ? "Every thread is resolved." : "No comments yet." }}
    </p>

    <ol class="space-y-3">
      <li
        v-for="{ thread, replies } in visibleThreads"
        :key="thread.id"
        class="rounded-lg border p-3"
        :class="thread.resolved && 'opacity-70'"
      >
        <template v-for="comment in [thread, ...replies]" :key="comment.id">
          <article class="flex gap-2.5" :class="comment.parent_id && 'mt-3 ml-6'">
            <Avatar class="size-7 shrink-0">
              <AvatarFallback class="text-[10px]">{{
                initialsOf(comment.author.name)
              }}</AvatarFallback>
            </Avatar>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <p class="truncate text-sm font-medium">{{ comment.author.name }}</p>
                <time class="text-muted-foreground shrink-0 text-xs" :datetime="comment.created_at">
                  {{ formatDateTime(comment.created_at) }}
                </time>
                <span v-if="comment.edited" class="text-muted-foreground text-xs">(edited)</span>
                <DropdownMenu v-if="comment.can_edit || comment.can_resolve">
                  <DropdownMenuTrigger as-child>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="ml-auto size-6"
                      aria-label="Comment actions"
                    >
                      <MoreHorizontalIcon class="size-3.5" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem v-if="comment.can_edit" @click="startEdit(comment)"
                      >Edit</DropdownMenuItem
                    >
                    <DropdownMenuItem class="text-destructive" @click="remove(comment)">
                      {{ comment.parent_id ? "Delete" : "Delete thread" }}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
              <p
                v-if="comment.quote"
                class="text-muted-foreground mt-1 border-l-2 pl-2 text-xs italic"
              >
                “{{ comment.quote }}”<template v-if="comment.page">
                  · page {{ comment.page }}</template
                >
              </p>
              <form v-if="editing === comment.id" class="mt-1 space-y-2" @submit.prevent="saveEdit">
                <Textarea v-model="editBody" rows="2" class="text-sm" aria-label="Edit comment" />
                <div class="flex justify-end gap-2">
                  <Button type="button" variant="ghost" size="sm" @click="editing = ''"
                    >Cancel</Button
                  >
                  <Button type="submit" size="sm" :disabled="!editBody.trim()">Save</Button>
                </div>
              </form>
              <p v-else class="mt-0.5 text-sm break-words whitespace-pre-line">
                <template v-for="(part, index) in parts(comment)" :key="index">
                  <span
                    v-if="part.mention"
                    class="bg-primary/10 text-primary rounded px-0.5 font-medium"
                    >{{ part.text }}</span
                  >
                  <template v-else>{{ part.text }}</template>
                </template>
              </p>
            </div>
          </article>
        </template>

        <div class="mt-2 flex flex-wrap items-center gap-1 pl-9">
          <Button
            v-if="canComment && !thread.resolved"
            variant="ghost"
            size="sm"
            class="h-7 gap-1 text-xs"
            @click="startReply(thread.id)"
          >
            <CornerDownRightIcon class="size-3.5" />Reply
          </Button>
          <Button
            v-if="thread.can_resolve"
            variant="ghost"
            size="sm"
            class="h-7 gap-1 text-xs"
            @click="toggleResolved(thread)"
          >
            <RotateCcwIcon v-if="thread.resolved" class="size-3.5" />
            <CheckCircle2Icon v-else class="size-3.5" />
            {{ thread.resolved ? "Reopen" : "Resolve" }}
          </Button>
          <span v-if="thread.resolved && thread.resolved_by" class="text-muted-foreground text-xs">
            Resolved by {{ thread.resolved_by.name }}
          </span>
        </div>

        <form
          v-if="replying === thread.id && replyDrafts[thread.id]"
          class="relative mt-2 space-y-2 pl-9"
          @submit.prevent="submit(thread.id)"
        >
          <Textarea
            v-model="replyDrafts[thread.id]!.body"
            rows="2"
            placeholder="Reply…"
            class="min-h-14 resize-none text-sm"
            aria-label="Reply"
            @input="onInput(thread.id, replyDrafts[thread.id]!)"
            @keydown.meta.enter.prevent="submit(thread.id)"
            @keydown.ctrl.enter.prevent="submit(thread.id)"
          />
          <ul
            v-if="mentionQuery?.key === thread.id && suggestions.length"
            class="bg-popover absolute inset-x-0 top-full z-20 mt-1 ml-9 overflow-hidden rounded-lg border shadow-md"
            role="listbox"
            aria-label="People to mention"
          >
            <li v-for="person in suggestions" :key="person.id">
              <button
                type="button"
                role="option"
                @mousedown.prevent
                class="hover:bg-accent flex w-full items-center gap-2 px-3 py-2 text-left text-sm"
                @click="pickMention(replyDrafts[thread.id]!, person)"
              >
                {{ person.name }}
              </button>
            </li>
          </ul>
          <div class="flex justify-end gap-2">
            <Button type="button" variant="ghost" size="sm" @click="replying = ''">Cancel</Button>
            <Button type="submit" size="sm" :disabled="!replyDrafts[thread.id]!.body.trim()"
              >Reply</Button
            >
          </div>
        </form>
      </li>
    </ol>
  </section>
</template>
