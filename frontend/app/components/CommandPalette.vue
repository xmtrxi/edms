<script setup lang="ts">
  import {
    FileTextIcon,
    SparklesIcon,
    UsersIcon,
    GitBranchIcon,
    CloudIcon,
    SettingsIcon,
    SearchIcon,
    StarIcon,
    Share2Icon,
    UploadIcon,
    Trash2Icon,
    ArchiveIcon,
    HistoryIcon,
    ClockIcon,
    FolderOpenIcon,
  } from "@lucide/vue";
  import { onMounted, onUnmounted, ref, computed } from "vue";
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
  import { documents } from "@/data/mock-data";
  import { activityFeed } from "@/data/mock-data";

  const props = defineProps<{ open: boolean }>();
  const emit = defineEmits<{ "update:open": [value: boolean] }>();

  const router = useRouter();
  const query = ref("");

  const folders = computed(() => {
    const unique = new Set(documents.map((d) => d.folder).filter(Boolean) as string[]);
    return Array.from(unique).sort();
  });

  const filteredFolders = computed(() => {
    if (!query.value.trim()) return folders.value;
    const q = query.value.toLowerCase();
    return folders.value.filter((f) => f.toLowerCase().includes(q));
  });

  function go(to: string) {
    emit("update:open", false);
    router.push(to);
  }

  function goToFolder(folder: string) {
    emit("update:open", false);
    router.push(`/documents?folder=${encodeURIComponent(folder)}`);
  }

  function onKey(e: KeyboardEvent) {
    if ((e.key === "k" || e.key === "K") && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      emit("update:open", !props.open);
    }
  }

  const recentSearches = [
    "MSA Acme renewal",
    "Q3 financial report",
    "SOC2 audit",
    "Employee handbook",
  ];

  onMounted(() => window.addEventListener("keydown", onKey));
  onUnmounted(() => window.removeEventListener("keydown", onKey));
</script>

<template>
  <CommandDialog :open="open" @update:open="emit('update:open', $event)">
    <CommandInput
      v-model="query"
      placeholder="Search documents, folders, people, libraries… or run a command"
    />
    <CommandList class="max-h-[480px]">
      <CommandEmpty>No results found.</CommandEmpty>

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

      <CommandGroup v-if="filteredFolders.length" heading="Folders">
        <CommandItem v-for="f in filteredFolders" :key="f" :value="f" @select="goToFolder(f)">
          <FolderOpenIcon class="text-primary h-4 w-4" />
          <span>{{ f }}</span>
          <span class="text-muted-foreground ml-auto text-[10px]"
            >{{ documents.filter((d) => d.folder === f).length }} documents</span
          >
        </CommandItem>
      </CommandGroup>

      <CommandSeparator v-if="filteredFolders.length" />

      <CommandGroup heading="Recent documents">
        <CommandItem
          v-for="d in documents.slice(0, 5)"
          :key="d.id"
          :value="`${d.name} ${d.owner} ${d.tags.join(' ')} ${d.folder || ''}`"
          @select="go(`/document/${d.id}`)"
        >
          <FileTextIcon />
          <span class="truncate">{{ d.name }}</span>
          <span class="text-muted-foreground ml-auto flex items-center gap-1 text-[10px]">
            <ClockIcon class="h-2.5 w-2.5" /> {{ d.modified }}
          </span>
        </CommandItem>
      </CommandGroup>

      <CommandSeparator />

      <CommandGroup heading="Recent searches">
        <CommandItem
          v-for="s in recentSearches"
          :key="s"
          :value="s"
          @select="go(`/search?q=${encodeURIComponent(s)}`)"
        >
          <HistoryIcon class="h-4 w-4" />
          <span>{{ s }}</span>
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
    </CommandList>
  </CommandDialog>
</template>
