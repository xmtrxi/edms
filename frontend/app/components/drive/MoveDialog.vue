<script setup lang="ts">
  import { FolderIcon, UserIcon } from "@lucide/vue";
  import { computed, shallowRef, watch } from "vue";

  import { Button } from "@/components/ui/button";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
  } from "@/components/ui/dialog";
  import type { ManagedFolder } from "@/types/api";

  const open = defineModel<boolean>("open", { required: true });
  const props = defineProps<{
    folders: ManagedFolder[];
    count: number;
    currentFolderId?: string;
  }>();
  const emit = defineEmits<{ move: [folderId: string] }>();

  // "root" is My files.
  const target = shallowRef("");
  watch(open, (value) => {
    if (value) target.value = "";
  });

  const options = computed(() => props.folders.filter((folder) => folder.can_upload !== false));
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Move {{ count }} item{{ count === 1 ? "" : "s" }}</DialogTitle>
        <DialogDescription>
          Files take on the access of the folder they move into, unless they have their own.
        </DialogDescription>
      </DialogHeader>
      <ul
        class="max-h-80 overflow-y-auto rounded-lg border p-1"
        role="listbox"
        aria-label="Destination"
      >
        <li>
          <button
            type="button"
            role="option"
            :aria-selected="target === 'root'"
            class="hover:bg-accent aria-selected:bg-accent flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-sm"
            @click="target = 'root'"
          >
            <UserIcon class="text-muted-foreground size-4" /> My files
          </button>
        </li>
        <li v-for="folder in options" :key="folder.id">
          <button
            type="button"
            role="option"
            :aria-selected="target === folder.id"
            :disabled="folder.id === currentFolderId"
            class="hover:bg-accent aria-selected:bg-accent flex w-full items-center gap-2 rounded-md py-2 pr-2 text-left text-sm disabled:opacity-50"
            :style="{ paddingLeft: `${0.5 + folder.depth * 1.1}rem` }"
            @click="target = folder.id"
          >
            <FolderIcon class="size-4" :style="{ color: folder.color || undefined }" />
            <span class="truncate">{{ folder.name }}</span>
          </button>
        </li>
      </ul>
      <DialogFooter>
        <Button variant="outline" @click="open = false">Cancel</Button>
        <Button :disabled="!target" @click="emit('move', target)">Move here</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
