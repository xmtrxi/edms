<script setup lang="ts">
  import { toast } from "vue-sonner";

  import FolderDetailsPanel from "@/components/folders/FolderDetailsPanel.vue";
  import FolderTree from "@/components/folders/FolderTree.vue";
  import type { FolderInput, ManagedFolder } from "@/types/api";

  definePageMeta({
    title: "Folders",
    description: "Create and organize the shared document structure.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Folders" }],
  });

  const service = foldersService();
  const {
    folders,
    loading,
    error,
    grants,
    users,
    departments,
    fetchFolders,
    createFolder: create,
    updateFolder,
    deleteFolder,
    fetchGrants,
    saveGrant,
    removeGrant,
    fetchUsers,
    fetchDepartments,
  } = service;
  const selected = shallowRef<ManagedFolder | undefined>();
  const saving = shallowRef(false);

  const selectFolder = (folder: ManagedFolder) => {
    selected.value = folder;
    void fetchGrants(folder.id);
  };
  const createFolder = () => {
    selected.value = undefined;
    grants.value = [];
  };
  const saveFolder = async (input: FolderInput) => {
    const isEditing = Boolean(selected.value);
    saving.value = true;
    const result = selected.value
      ? await updateFolder(selected.value.id, input)
      : await create(input);
    saving.value = false;
    if (!result) {
      toast.error(error.value || "Could not save folder");
      return;
    }
    selected.value = result;
    toast.success(isEditing ? "Folder saved" : "Folder created");
  };
  const removeFolder = async () => {
    if (!selected.value) return;
    const removed = await deleteFolder(selected.value.id);
    if (!removed) {
      toast.error(error.value || "Could not delete folder");
      return;
    }
    selected.value = undefined;
    toast.success("Folder deleted");
  };
  const saveFolderGrant = async (input: import("@/types/api").FolderGrantInput) => {
    if (!selected.value) return;
    if (!(await saveGrant(selected.value.id, input))) {
      toast.error(error.value || "Could not update access");
      return;
    }
    toast.success("Access updated");
  };
  const removeFolderGrant = async (grantId: string) => {
    if (!selected.value) return;
    if (!(await removeGrant(selected.value.id, grantId))) {
      toast.error(error.value || "Could not remove access");
      return;
    }
    toast.success("Access removed");
  };
  onMounted(() => {
    void fetchFolders();
    void fetchUsers();
    void fetchDepartments();
  });
</script>

<template>
  <div
    class="border-border bg-card grid min-h-[620px] overflow-hidden border lg:grid-cols-[minmax(260px,340px)_minmax(0,1fr)]"
  >
    <FolderTree
      :folders="folders"
      :selected-id="selected?.id"
      :loading="loading"
      @select="selectFolder"
      @create="createFolder"
    />
    <FolderDetailsPanel
      :folder="selected"
      :folders="folders"
      :grants="grants"
      :users="users"
      :departments="departments"
      :saving="saving"
      @save="saveFolder"
      @remove="removeFolder"
      @create="createFolder"
      @save-grant="saveFolderGrant"
      @remove-grant="removeFolderGrant"
    />
  </div>
</template>
