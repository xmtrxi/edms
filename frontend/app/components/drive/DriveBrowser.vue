<script setup lang="ts">
  import {
    ChevronRightIcon,
    CloudDownloadIcon,
    FolderIcon,
    FolderInputIcon,
    FolderPlusIcon,
    InboxIcon,
    LayoutGridIcon,
    ListIcon,
    MoreVerticalIcon,
    PencilIcon,
    SearchIcon,
    Share2Icon,
    ShieldCheckIcon,
    Trash2Icon,
    UploadCloudIcon,
    XIcon,
  } from "@lucide/vue";
  import { useLocalStorage } from "@vueuse/core";
  import { computed, onBeforeUnmount, ref, shallowRef, watch } from "vue";
  import { toast } from "vue-sonner";

  import DocumentShareDialog from "@/components/documents/DocumentShareDialog.vue";
  import FileRequestDialog from "@/components/drive/FileRequestDialog.vue";
  import MoveDialog from "@/components/drive/MoveDialog.vue";
  import FileTypeIcon from "@/components/FileTypeIcon.vue";
  import FolderPermissionEditor from "@/components/folders/FolderPermissionEditor.vue";
  import { Button } from "@/components/ui/button";
  import { Checkbox } from "@/components/ui/checkbox";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
  } from "@/components/ui/dialog";
  import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
  } from "@/components/ui/dropdown-menu";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import { cn } from "@/lib/utils";
  import type { ApiResponse, Document, ManagedFolder } from "@/types/api";
  import { apiErrorMessage } from "@/utils/api";
  import { fileTypeFromMime, formatBytes, formatDate, initialsOf } from "@/utils/format";

  const props = defineProps<{ folderId: string }>();
  const emit = defineEmits<{ navigate: [folderId: string] }>();

  const router = useRouter();
  const uploads = useUploadStore();
  const folderService = foldersService();
  const documentService = documentsService();
  const { folders } = folderService;

  const view = useLocalStorage<"list" | "grid">("drive.view", "list");
  const query = shallowRef("");
  const documents = shallowRef<Document[]>([]);
  const total = shallowRef(0);
  const page = shallowRef(1);
  const loading = shallowRef(false);
  const selected = ref(new Set<string>());
  const dragDepth = shallowRef(0);
  const PAGE_SIZE = 100;

  const current = computed(() => folders.value.find((folder) => folder.id === props.folderId));
  const breadcrumb = computed(() => {
    if (!current.value) return [];
    const chain = (current.value.ancestor_ids ?? [])
      .map((id) => folders.value.find((folder) => folder.id === id))
      .filter((folder): folder is ManagedFolder => Boolean(folder));
    return [...chain, current.value];
  });
  const childFolders = computed(() =>
    query.value.trim()
      ? []
      : folders.value.filter((folder) => (folder.parent_id ?? "") === (props.folderId || "")),
  );
  const canUploadHere = computed(() => !props.folderId || current.value?.can_upload !== false);
  const selectedDocs = computed(() => documents.value.filter((doc) => selected.value.has(doc.id)));

  const loadDocuments = async (append = false) => {
    const { $api } = useNuxtApp();
    loading.value = true;
    try {
      const search = query.value.trim();
      const res = await $api<ApiResponse<Document[]>>("/documents", {
        query: {
          q: search || undefined,
          // Searching covers the whole drive; browsing shows one folder.
          folder_id: search ? undefined : props.folderId || "root",
          page: page.value,
          page_size: PAGE_SIZE,
        },
      });
      documents.value = append ? [...documents.value, ...res.data] : res.data;
      total.value = res.meta?.total ?? res.data.length;
    } catch (err) {
      toast.error(apiErrorMessage(err));
    } finally {
      loading.value = false;
    }
  };

  const refresh = () => {
    page.value = 1;
    selected.value = new Set();
    void folderService.fetchFolders();
    void loadDocuments();
  };

  watch(() => props.folderId, refresh, { immediate: true });
  watch(query, (_value, _old, onCleanup) => {
    const timer = setTimeout(() => {
      page.value = 1;
      void loadDocuments();
    }, 300);
    onCleanup(() => clearTimeout(timer));
  });

  const stop = uploads.onUploaded(() => void loadDocuments());
  onBeforeUnmount(stop);

  const loadMore = () => {
    page.value += 1;
    void loadDocuments(true);
  };

  const toggle = (id: string, value: boolean) => {
    const next = new Set(selected.value);
    if (value) next.add(id);
    else next.delete(id);
    selected.value = next;
  };
  const allSelected = computed(
    () => documents.value.length > 0 && documents.value.every((doc) => selected.value.has(doc.id)),
  );
  const toggleAll = (value: boolean) => {
    selected.value = value ? new Set(documents.value.map((doc) => doc.id)) : new Set();
  };

  const openDocument = (doc: Document) => router.push(`/documents/${doc.id}`);
  const openFolder = (folder: ManagedFolder) => {
    query.value = "";
    emit("navigate", folder.id);
  };

  // Drag and drop: files from the desktop upload here; documents dragged
  // onto a folder move into it.
  const DOC_DRAG = "application/x-edms-documents";
  const hasFiles = (event: DragEvent) => event.dataTransfer?.types.includes("Files") ?? false;
  const onDragEnter = (event: DragEvent) => {
    if (hasFiles(event)) dragDepth.value++;
  };
  const onDragLeave = (event: DragEvent) => {
    if (hasFiles(event)) dragDepth.value = Math.max(0, dragDepth.value - 1);
  };
  const onDrop = (event: DragEvent) => {
    dragDepth.value = 0;
    const files = event.dataTransfer?.files;
    if (!files?.length) return;
    if (!canUploadHere.value) {
      toast.error("You can’t add files to this folder");
      return;
    }
    uploads.add(files, { folderId: props.folderId || undefined });
  };
  const onDocDragStart = (event: DragEvent, doc: Document) => {
    const ids = selected.value.has(doc.id) ? [...selected.value] : [doc.id];
    event.dataTransfer?.setData(DOC_DRAG, JSON.stringify(ids));
    if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
  };
  const onFolderDrop = (event: DragEvent, folder: ManagedFolder) => {
    const raw = event.dataTransfer?.getData(DOC_DRAG);
    if (raw) void moveTo(JSON.parse(raw) as string[], folder.id);
  };

  const moveOpen = shallowRef(false);
  const moveIds = shallowRef<string[]>([]);
  const startMove = (ids: string[]) => {
    moveIds.value = ids;
    moveOpen.value = true;
  };
  const moveTo = async (ids: string[], folderId: string) => {
    moveOpen.value = false;
    let moved = 0;
    for (const id of ids) {
      if (await documentService.update(id, { folder_id: folderId })) moved++;
    }
    if (moved < ids.length)
      toast.error(documentService.error.value || "Some files couldn’t be moved");
    if (moved) toast.success(`Moved ${moved} file${moved === 1 ? "" : "s"}`);
    refresh();
  };

  const trash = async (docs: Document[]) => {
    const workspace = workspaceService();
    let removed = 0;
    for (const doc of docs) {
      await workspace.moveToTrash(doc.id);
      if (!workspace.error.value) removed++;
    }
    if (removed < docs.length) toast.error("Some files couldn’t be moved to trash");
    if (removed) toast.success(`Moved ${removed} file${removed === 1 ? "" : "s"} to trash`);
    refresh();
  };

  const download = async (docs: Document[]) => {
    for (const doc of docs) await documentService.download(doc.id, doc.name);
  };

  const renameTarget = shallowRef<{ kind: "doc" | "folder"; id: string; name: string } | null>(
    null,
  );
  const renameValue = shallowRef("");
  const startRename = (kind: "doc" | "folder", id: string, name: string) => {
    renameTarget.value = { kind, id, name };
    renameValue.value = name;
  };
  const saveRename = async () => {
    const target = renameTarget.value;
    if (!target || !renameValue.value.trim()) return;
    if (target.kind === "doc") {
      if (!(await documentService.update(target.id, { name: renameValue.value.trim() }))) {
        toast.error(documentService.error.value || "Could not rename");
        return;
      }
    } else {
      const folder = folders.value.find((item) => item.id === target.id);
      if (
        !folder ||
        !(await folderService.updateFolder(folder.id, {
          name: renameValue.value.trim(),
          parent_id: folder.parent_id,
          description: folder.description,
          color: folder.color,
          inherit_permissions: folder.inherit_permissions,
        }))
      ) {
        toast.error(folderService.error.value || "Could not rename");
        return;
      }
    }
    renameTarget.value = null;
    refresh();
  };

  const newFolderOpen = shallowRef(false);
  const newFolderName = shallowRef("");
  const createFolder = async () => {
    const name = newFolderName.value.trim();
    if (!name) return;
    const created = await folderService.createFolder({
      name,
      parent_id: props.folderId || undefined,
      inherit_permissions: true,
    });
    if (!created) {
      toast.error(folderService.error.value || "Could not create folder");
      return;
    }
    newFolderOpen.value = false;
    newFolderName.value = "";
    refresh();
  };

  const deleteFolder = async (folder: ManagedFolder) => {
    if (!(await folderService.deleteFolder(folder.id))) {
      toast.error(folderService.error.value || "Only empty folders can be deleted");
      return;
    }
    toast.success(`${folder.name} deleted`);
    refresh();
  };

  const shareDoc = shallowRef<Document | null>(null);
  const accessFolder = shallowRef<ManagedFolder | null>(null);
  const requestFolder = shallowRef<ManagedFolder | null>(null);
  const openFolderAccess = async (folder: ManagedFolder) => {
    accessFolder.value = folder;
    await folderService.fetchGrants(folder.id);
  };
  const saveGrant = async (input: import("@/types/api").FolderGrantInput) => {
    if (accessFolder.value && !(await folderService.saveGrant(accessFolder.value.id, input)))
      toast.error(folderService.error.value || "Could not update access");
  };
  const removeGrant = async (grantId: string) => {
    if (accessFolder.value && !(await folderService.removeGrant(accessFolder.value.id, grantId)))
      toast.error(folderService.error.value || "Could not remove access");
  };

  const openNewFolder = () => (newFolderOpen.value = true);
  const openFileRequest = () => {
    if (current.value) requestFolder.value = current.value;
  };
  defineExpose({ refresh, openNewFolder, openFileRequest });
</script>

<template>
  <div
    class="relative min-h-[60dvh]"
    @dragenter.prevent="onDragEnter"
    @dragover.prevent
    @dragleave="onDragLeave"
    @drop.prevent="onDrop"
  >
    <div
      v-if="dragDepth > 0"
      class="border-primary bg-primary/5 pointer-events-none absolute inset-0 z-20 grid place-items-center rounded-2xl border-2 border-dashed"
    >
      <p class="bg-card flex items-center gap-2 rounded-full px-4 py-2 text-sm font-medium shadow">
        <UploadCloudIcon class="text-primary size-4" /> Drop to upload to
        {{ current?.name ?? "My files" }}
      </p>
    </div>

    <div class="mb-4 flex flex-wrap items-center gap-2">
      <nav class="flex min-w-0 flex-1 items-center gap-1 text-sm" aria-label="Folder path">
        <button
          type="button"
          class="hover:bg-accent rounded-md px-2 py-1 font-medium"
          @click="emit('navigate', '')"
        >
          Drive
        </button>
        <template v-for="folder in breadcrumb" :key="folder.id">
          <ChevronRightIcon class="text-muted-foreground size-4 shrink-0" />
          <button
            type="button"
            class="hover:bg-accent truncate rounded-md px-2 py-1"
            :class="folder.id === folderId && 'font-semibold'"
            :aria-current="folder.id === folderId ? 'page' : undefined"
            @click="emit('navigate', folder.id)"
          >
            {{ folder.name }}
          </button>
        </template>
      </nav>
      <div class="relative w-full sm:w-64">
        <SearchIcon class="text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2" />
        <Input
          v-model="query"
          class="h-9 pl-9"
          placeholder="Search the drive"
          aria-label="Search the drive"
        />
      </div>
      <div class="flex rounded-lg border p-0.5" role="group" aria-label="Layout">
        <Button
          variant="ghost"
          size="icon"
          class="size-8"
          :aria-pressed="view === 'list'"
          aria-label="List view"
          :class="view === 'list' && 'bg-accent'"
          @click="view = 'list'"
          ><ListIcon class="size-4"
        /></Button>
        <Button
          variant="ghost"
          size="icon"
          class="size-8"
          :aria-pressed="view === 'grid'"
          aria-label="Grid view"
          :class="view === 'grid' && 'bg-accent'"
          @click="view = 'grid'"
          ><LayoutGridIcon class="size-4"
        /></Button>
      </div>
    </div>

    <div
      v-if="selected.size"
      class="bg-primary text-primary-foreground sticky top-2 z-10 mb-3 flex flex-wrap items-center gap-2 rounded-xl px-3 py-2 shadow"
    >
      <span class="text-sm font-medium">{{ selected.size }} selected</span>
      <div class="ml-auto flex flex-wrap gap-1">
        <Button size="sm" variant="secondary" @click="startMove([...selected])"
          ><FolderInputIcon class="size-4" />Move</Button
        >
        <Button size="sm" variant="secondary" @click="download(selectedDocs)"
          ><CloudDownloadIcon class="size-4" />Download</Button
        >
        <Button size="sm" variant="secondary" @click="trash(selectedDocs)"
          ><Trash2Icon class="size-4" />Trash</Button
        >
        <Button
          size="icon"
          variant="ghost"
          class="size-8"
          aria-label="Clear selection"
          @click="selected = new Set()"
          ><XIcon class="size-4"
        /></Button>
      </div>
    </div>

    <section v-if="childFolders.length" class="mb-6" aria-label="Folders">
      <h2 class="text-muted-foreground mb-2 text-xs font-semibold tracking-wider uppercase">
        Folders
      </h2>
      <ul class="grid grid-cols-[repeat(auto-fill,minmax(13rem,1fr))] gap-2">
        <li v-for="folder in childFolders" :key="folder.id">
          <div
            class="group bg-card hover:border-primary/40 flex items-center gap-3 rounded-xl border px-3 py-2.5 transition-colors"
            @dragover.prevent
            @drop.prevent.stop="onFolderDrop($event, folder)"
          >
            <button
              type="button"
              class="flex min-w-0 flex-1 items-center gap-3 text-left"
              @click="openFolder(folder)"
            >
              <FolderIcon class="size-5 shrink-0" :style="{ color: folder.color || undefined }" />
              <span class="min-w-0">
                <span class="block truncate text-sm font-medium">{{ folder.name }}</span>
                <span class="text-muted-foreground text-xs"
                  >{{ folder.document_count }} file{{
                    folder.document_count === 1 ? "" : "s"
                  }}</span
                >
              </span>
            </button>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon"
                  class="size-7"
                  :aria-label="`Actions for ${folder.name}`"
                  ><MoreVerticalIcon class="size-4"
                /></Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem @select="openFolder(folder)"
                  ><FolderIcon class="size-4" />Open</DropdownMenuItem
                >
                <DropdownMenuItem @select="openFolderAccess(folder)"
                  ><ShieldCheckIcon class="size-4" />Who has access</DropdownMenuItem
                >
                <DropdownMenuItem v-if="folder.can_upload" @select="requestFolder = folder"
                  ><InboxIcon class="size-4" />Request files</DropdownMenuItem
                >
                <template v-if="folder.can_manage">
                  <DropdownMenuItem @select="startRename('folder', folder.id, folder.name)"
                    ><PencilIcon class="size-4" />Rename</DropdownMenuItem
                  >
                  <DropdownMenuSeparator />
                  <DropdownMenuItem class="text-destructive" @select="deleteFolder(folder)"
                    ><Trash2Icon class="size-4" />Delete</DropdownMenuItem
                  >
                </template>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </li>
      </ul>
    </section>

    <section aria-label="Files">
      <h2
        v-if="childFolders.length || !folderId"
        class="text-muted-foreground mb-2 text-xs font-semibold tracking-wider uppercase"
      >
        {{ query.trim() ? "Results" : folderId ? "Files" : "My files" }}
      </h2>

      <div
        v-if="!loading && !documents.length"
        class="text-muted-foreground flex flex-col items-center gap-3 rounded-2xl border border-dashed p-12 text-center text-sm"
      >
        <UploadCloudIcon class="size-6" />
        <p>
          {{
            query.trim()
              ? "Nothing matches that search."
              : canUploadHere
                ? "No files here yet. Drop files anywhere on this page to upload."
                : "No files here that you can see."
          }}
        </p>
        <Button
          v-if="!query.trim() && canUploadHere && !folderId"
          size="sm"
          variant="outline"
          @click="openNewFolder"
          ><FolderPlusIcon class="size-4" />New folder</Button
        >
      </div>

      <div v-else-if="view === 'list'" class="bg-card overflow-hidden rounded-xl border">
        <table class="w-full text-sm">
          <thead class="text-muted-foreground border-b text-left text-xs">
            <tr>
              <th class="w-10 px-3 py-2">
                <Checkbox
                  :model-value="allSelected"
                  aria-label="Select all files"
                  @update:model-value="(value) => toggleAll(Boolean(value))"
                />
              </th>
              <th class="px-2 py-2 font-medium">Name</th>
              <th class="hidden px-2 py-2 font-medium md:table-cell">Owner</th>
              <th class="hidden px-2 py-2 font-medium sm:table-cell">Modified</th>
              <th class="hidden px-2 py-2 text-right font-medium sm:table-cell">Size</th>
              <th class="w-10" />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="doc in documents"
              :key="doc.id"
              draggable="true"
              :class="
                cn(
                  'hover:bg-muted/50 border-b last:border-0',
                  selected.has(doc.id) && 'bg-primary/5',
                )
              "
              @dragstart="onDocDragStart($event, doc)"
            >
              <td class="px-3 py-2">
                <Checkbox
                  :model-value="selected.has(doc.id)"
                  :aria-label="`Select ${doc.name}`"
                  @update:model-value="(value) => toggle(doc.id, Boolean(value))"
                />
              </td>
              <td class="max-w-0 px-2 py-2">
                <button
                  type="button"
                  class="flex w-full min-w-0 items-center gap-2.5 text-left"
                  @click="openDocument(doc)"
                >
                  <FileTypeIcon :type="fileTypeFromMime(doc.mime_type)" class="size-5 shrink-0" />
                  <span class="truncate font-medium">{{ doc.name }}</span>
                </button>
              </td>
              <td class="hidden px-2 py-2 md:table-cell">
                <span class="flex items-center gap-2">
                  <Avatar class="size-6"
                    ><AvatarFallback class="text-[10px]">{{
                      initialsOf(doc.owner?.name)
                    }}</AvatarFallback></Avatar
                  >
                  <span class="text-muted-foreground truncate text-xs">{{
                    doc.owner?.name ?? "—"
                  }}</span>
                </span>
              </td>
              <td class="text-muted-foreground hidden px-2 py-2 text-xs sm:table-cell">
                {{ formatDate(doc.updated_at) }}
              </td>
              <td
                class="text-muted-foreground hidden px-2 py-2 text-right text-xs tabular-nums sm:table-cell"
              >
                {{ formatBytes(doc.size) }}
              </td>
              <td class="px-2 py-2">
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="size-7"
                      :aria-label="`Actions for ${doc.name}`"
                      ><MoreVerticalIcon class="size-4"
                    /></Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem @select="shareDoc = doc"
                      ><Share2Icon class="size-4" />Share</DropdownMenuItem
                    >
                    <DropdownMenuItem @select="download([doc])"
                      ><CloudDownloadIcon class="size-4" />Download</DropdownMenuItem
                    >
                    <DropdownMenuItem @select="startRename('doc', doc.id, doc.name)"
                      ><PencilIcon class="size-4" />Rename</DropdownMenuItem
                    >
                    <DropdownMenuItem @select="startMove([doc.id])"
                      ><FolderInputIcon class="size-4" />Move</DropdownMenuItem
                    >
                    <DropdownMenuSeparator />
                    <DropdownMenuItem class="text-destructive" @select="trash([doc])"
                      ><Trash2Icon class="size-4" />Move to trash</DropdownMenuItem
                    >
                  </DropdownMenuContent>
                </DropdownMenu>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <ul v-else class="grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-3">
        <li
          v-for="doc in documents"
          :key="doc.id"
          draggable="true"
          :class="
            cn(
              'group bg-card relative overflow-hidden rounded-xl border transition-colors',
              selected.has(doc.id)
                ? 'border-primary ring-primary/30 ring-2'
                : 'hover:border-primary/40',
            )
          "
          @dragstart="onDocDragStart($event, doc)"
        >
          <Checkbox
            class="bg-card absolute top-2 left-2 z-10 opacity-0 group-hover:opacity-100 data-[state=checked]:opacity-100"
            :model-value="selected.has(doc.id)"
            :aria-label="`Select ${doc.name}`"
            @update:model-value="(value) => toggle(doc.id, Boolean(value))"
          />
          <button type="button" class="block w-full text-left" @click="openDocument(doc)">
            <span class="bg-muted/60 grid h-28 place-items-center">
              <FileTypeIcon :type="fileTypeFromMime(doc.mime_type)" size="lg" />
            </span>
            <span class="block truncate px-3 pt-2 text-sm font-medium">{{ doc.name }}</span>
            <span class="text-muted-foreground block px-3 pb-2.5 text-xs">{{
              formatDate(doc.updated_at)
            }}</span>
          </button>
        </li>
      </ul>

      <div v-if="documents.length < total" class="mt-4 flex justify-center">
        <Button variant="outline" size="sm" :disabled="loading" @click="loadMore">Load more</Button>
      </div>
    </section>
  </div>

  <MoveDialog
    v-model:open="moveOpen"
    :folders="folders"
    :count="moveIds.length"
    :current-folder-id="folderId"
    @move="(target) => moveTo(moveIds, target)"
  />

  <DocumentShareDialog
    v-if="shareDoc"
    :open="Boolean(shareDoc)"
    :document-id="shareDoc.id"
    :title="shareDoc.name"
    @update:open="(value) => !value && (shareDoc = null)"
  />

  <FileRequestDialog
    v-if="requestFolder"
    :open="Boolean(requestFolder)"
    :folder-id="requestFolder.id"
    :folder-name="requestFolder.name"
    @update:open="(value) => !value && (requestFolder = null)"
  />

  <Dialog :open="Boolean(accessFolder)" @update:open="(value) => !value && (accessFolder = null)">
    <DialogContent class="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
      <DialogHeader>
        <DialogTitle>Who has access to “{{ accessFolder?.name }}”</DialogTitle>
        <DialogDescription
          >Access here applies to files and subfolders that follow this folder.</DialogDescription
        >
      </DialogHeader>
      <FolderPermissionEditor
        v-if="accessFolder"
        class="!mt-0 !border-0 !pt-0"
        :grants="folderService.grants.value"
        :can-manage="accessFolder.can_manage !== false"
        @save="saveGrant"
        @remove="removeGrant"
      />
    </DialogContent>
  </Dialog>

  <Dialog :open="Boolean(renameTarget)" @update:open="(value) => !value && (renameTarget = null)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader><DialogTitle>Rename</DialogTitle></DialogHeader>
      <form @submit.prevent="saveRename">
        <Label for="rename-input" class="sr-only">New name</Label>
        <Input id="rename-input" v-model="renameValue" autofocus />
      </form>
      <DialogFooter>
        <Button variant="outline" @click="renameTarget = null">Cancel</Button>
        <Button :disabled="!renameValue.trim()" @click="saveRename">Rename</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <Dialog v-model:open="newFolderOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>New folder</DialogTitle>
        <DialogDescription>
          {{
            current
              ? `Inside “${current.name}”, with the same access.`
              : "A top-level folder in your organization."
          }}
        </DialogDescription>
      </DialogHeader>
      <form @submit.prevent="createFolder">
        <Label for="folder-name" class="sr-only">Folder name</Label>
        <Input id="folder-name" v-model="newFolderName" placeholder="Folder name" autofocus />
      </form>
      <DialogFooter>
        <Button variant="outline" @click="newFolderOpen = false">Cancel</Button>
        <Button :disabled="!newFolderName.trim()" @click="createFolder">Create</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
