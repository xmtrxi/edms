<script setup lang="ts">
  import {
    ArchiveIcon,
    ClockIcon,
    CloudIcon,
    FileTextIcon,
    GitBranchIcon,
    SearchIcon,
    SettingsIcon,
    Share2Icon,
    SparklesIcon,
    StarIcon,
    Trash2Icon,
    UploadIcon,
    UsersIcon,
  } from "@lucide/vue";
  import { onMounted, onUnmounted, ref, watch } from "vue";
  import { useRouter } from "vue-router";

  import {
    CommandDialog,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
    CommandSeparator,
    CommandShortcut,
  } from "@/components/ui/command";
  import { formatDate } from "@/utils/format";

  const props = defineProps<{ open: boolean }>();
  const emit = defineEmits<{ "update:open": [value: boolean] }>();

  const router = useRouter();
  const query = ref("");

  const svc = searchService();
  const { results, loading } = svc;

  watch(query, (_value, _oldValue, onCleanup) => {
    const t = setTimeout(() => {
      const q = query.value.trim();
      if (q) void svc.search(q, { page_size: 5 });
    }, 250);
    onCleanup(() => clearTimeout(t));
  });

  const go = (to: string) => {
    emit("update:open", false);
    router.push(to);
  };

  const onKey = (e: KeyboardEvent) => {
    if ((e.key === "k" || e.key === "K") && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      emit("update:open", !props.open);
    }
  };

  onMounted(() => window.addEventListener("keydown", onKey));
  onUnmounted(() => window.removeEventListener("keydown", onKey));
</script>

<template>
  <CommandDialog :open="open" @update:open="emit('update:open', $event)">
    <CommandInput v-model="query" placeholder="Search documents… or run a command" />
    <CommandList class="max-h-[480px]">
      <CommandEmpty>No results found.</CommandEmpty>

      <template v-if="query.trim()">
        <CommandGroup heading="Documents">
          <CommandItem
            v-for="d in results"
            :key="d.id"
            :value="`${d.name} ${d.original_name} ${d.owner?.name ?? ''}`"
            @select="go(`/documents/${d.id}`)"
          >
            <FileTextIcon />
            <span class="truncate">{{ d.name }}</span>
            <span class="text-muted-foreground ml-auto flex items-center gap-1 text-[10px]">
              <ClockIcon class="h-2.5 w-2.5" /> {{ formatDate(d.updated_at) }}
            </span>
          </CommandItem>
          <CommandItem v-if="loading" disabled>
            <span class="text-muted-foreground">Searching…</span>
          </CommandItem>
        </CommandGroup>
      </template>

      <template v-else>
        <CommandGroup heading="Quick actions">
          <CommandItem @select="go('/search')">
            <SearchIcon /> <span>Open semantic search</span>
            <CommandShortcut>/</CommandShortcut>
          </CommandItem>
          <CommandItem @select="go('/ai-insights')">
            <SparklesIcon /> <span>Ask AI</span>
            <CommandShortcut>⌘I</CommandShortcut>
          </CommandItem>
          <CommandItem @select="go('/documents')">
            <UploadIcon /> <span>Upload to My Documents</span>
            <CommandShortcut>⌘N</CommandShortcut>
          </CommandItem>
        </CommandGroup>

        <CommandSeparator />

        <CommandGroup heading="Navigate">
          <CommandItem @select="go('/')"> <FileTextIcon /> Dashboard </CommandItem>
          <CommandItem @select="go('/workflows')"> <GitBranchIcon /> Workflows </CommandItem>
          <CommandItem @select="go('/teams')"> <UsersIcon /> Teams </CommandItem>
          <CommandItem @select="go('/sharepoint')"> <CloudIcon /> SharePoint </CommandItem>
          <CommandItem @select="go('/favorites')"> <StarIcon /> Favorites </CommandItem>
          <CommandItem @select="go('/shared')"> <Share2Icon /> Shared with me </CommandItem>
          <CommandItem @select="go('/archive')"> <ArchiveIcon /> Archive </CommandItem>
          <CommandItem @select="go('/trash')"> <Trash2Icon /> Trash </CommandItem>
          <CommandItem @select="go('/settings')"> <SettingsIcon /> Settings </CommandItem>
        </CommandGroup>
      </template>
    </CommandList>
  </CommandDialog>
</template>
