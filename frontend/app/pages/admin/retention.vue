<script setup lang="ts">
  import { ArchiveIcon, MoreHorizontalIcon, PlayIcon, PlusIcon, ScanEyeIcon } from "@lucide/vue";
  import { onMounted, ref, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import PolicyEditor from "@/components/retention/PolicyEditor.vue";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
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
    DropdownMenuTrigger,
  } from "@/components/ui/dropdown-menu";
  import type { PolicyDraft } from "@/services/retention";
  import type { RetentionPolicy, RetentionPreview } from "@/types/api";
  import { formatDateTime } from "@/utils/format";
  import { policySummary } from "@/utils/retention";

  definePageMeta({
    title: "Retention",
    description: "How long documents are kept, and what happens to them afterwards.",
    breadcrumbs: [{ label: "Administration", to: "/" }, { label: "Retention" }],
  });
  useHead({ title: "Retention — Lumen DMS" });

  const auth = useAuthStore();
  const service = retentionService();
  const folderService = foldersService();
  const folders = folderService.folders;
  const policies = ref<RetentionPolicy[]>([]);
  const loaded = shallowRef(false);

  const folderName = (id?: string) =>
    folders.value.find((folder) => folder.id === id)?.name ?? "a folder";

  const load = async () => {
    policies.value = await service.policies();
    loaded.value = true;
  };
  onMounted(() => {
    void folderService.fetchFolders();
    void load();
  });

  const editorOpen = shallowRef(false);
  const editing = shallowRef<RetentionPolicy | null>(null);
  const newPolicy = () => {
    editing.value = null;
    editorOpen.value = true;
  };
  const editPolicy = (policy: RetentionPolicy) => {
    editing.value = policy;
    editorOpen.value = true;
  };
  const savePolicy = async (draft: PolicyDraft) => {
    const saved = await service.savePolicy(draft, editing.value?.id);
    if (!saved) {
      toast.error(service.error.value);
      return;
    }
    editorOpen.value = false;
    toast.success(editing.value ? "Policy saved" : "Policy created");
    await load();
  };
  const deletePolicy = async (policy: RetentionPolicy) => {
    if (await service.deletePolicy(policy.id)) {
      policies.value = policies.value.filter((item) => item.id !== policy.id);
      toast.success("Policy deleted");
    } else toast.error(service.error.value);
  };

  const previewFor = shallowRef<RetentionPolicy | null>(null);
  const preview = shallowRef<RetentionPreview | null>(null);
  const showPreview = async (policy: RetentionPolicy) => {
    previewFor.value = policy;
    preview.value = null;
    preview.value = await service.preview(policy.id);
  };
  const runNow = async () => {
    const policy = previewFor.value;
    if (!policy) return;
    const result = await service.runPolicy(policy.id);
    if (!result) {
      toast.error(service.error.value);
      return;
    }
    toast.success(`${result.applied} ${result.applied === 1 ? "file" : "files"} handled`);
    previewFor.value = null;
    await load();
  };
</script>

<template>
  <div
    v-if="!auth.isOrgAdmin"
    class="bg-card mx-auto max-w-md rounded-2xl border p-8 text-center text-sm"
  >
    Only organization admins can manage retention.
  </div>

  <template v-else>
    <Teleport to="#page-actions-target">
      <Button size="sm" class="gap-1.5" @click="newPolicy"
        ><PlusIcon class="size-4" />New policy</Button
      >
    </Teleport>

    <div class="mx-auto max-w-4xl space-y-3">
      <div v-if="loaded && !policies.length" class="bg-card rounded-2xl border p-10 text-center">
        <ArchiveIcon class="text-muted-foreground mx-auto size-8" />
        <p class="mt-3 font-semibold">No retention policies</p>
        <p class="text-muted-foreground mx-auto mt-1 max-w-md text-sm">
          Everything is kept until someone deletes it. Add a policy to archive old files or empty
          the trash on a schedule. Files on legal hold are always kept.
        </p>
        <Button size="sm" class="mt-4" @click="newPolicy">Create a policy</Button>
      </div>
      <article
        v-for="policy in policies"
        :key="policy.id"
        class="bg-card rounded-xl border p-4"
        :class="!policy.enabled && 'opacity-70'"
      >
        <div class="flex items-start gap-3">
          <div class="min-w-0 flex-1">
            <p class="flex flex-wrap items-center gap-2 font-medium">
              {{ policy.name }}
              <Badge v-if="!policy.enabled" variant="outline" class="h-5 px-1.5 text-[10px]"
                >Paused</Badge
              >
              <Badge
                v-if="policy.action === 'purge'"
                variant="destructive"
                class="h-5 px-1.5 text-[10px]"
                >Deletes for good</Badge
              >
            </p>
            <p class="text-muted-foreground mt-1 text-sm">
              {{ policySummary(policy, folderName) }}
            </p>
            <p class="text-muted-foreground mt-2 text-xs">
              <template v-if="policy.last_run_at"
                >Last run {{ formatDateTime(policy.last_run_at) }} ·
                {{ policy.last_count }} {{ policy.last_count === 1 ? "file" : "files" }}</template
              >
              <template v-else>Hasn’t run yet</template>
            </p>
          </div>
          <Button variant="outline" size="sm" class="gap-1.5" @click="showPreview(policy)"
            ><ScanEyeIcon class="size-4" />Preview</Button
          >
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button
                variant="ghost"
                size="icon"
                class="size-8"
                :aria-label="`${policy.name} actions`"
                ><MoreHorizontalIcon class="size-4"
              /></Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem @click="editPolicy(policy)">Edit</DropdownMenuItem>
              <DropdownMenuItem class="text-destructive" @click="deletePolicy(policy)"
                >Delete</DropdownMenuItem
              >
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </article>
    </div>
  </template>

  <PolicyEditor
    v-model:open="editorOpen"
    :policy="editing"
    :folders="folders"
    :busy="service.busy.value"
    @save="savePolicy"
  />

  <Dialog :open="Boolean(previewFor)" @update:open="(value) => !value && (previewFor = null)">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ previewFor?.name }}</DialogTitle>
        <DialogDescription>{{
          previewFor && policySummary(previewFor, folderName)
        }}</DialogDescription>
      </DialogHeader>
      <p v-if="!preview" class="text-muted-foreground text-sm">Checking…</p>
      <template v-else>
        <p class="text-sm">
          <span class="font-semibold">{{ preview.count }}</span>
          {{ preview.count === 1 ? "file matches" : "files match" }} right now.
          <template v-if="preview.held">
            {{ preview.held }} more {{ preview.held === 1 ? "is" : "are" }} kept because of a legal
            hold.</template
          >
        </p>
        <ul
          v-if="preview.sample.length"
          class="bg-muted/50 max-h-60 space-y-1 overflow-y-auto rounded-lg p-3 text-sm"
        >
          <li v-for="doc in preview.sample" :key="doc.id" class="truncate">{{ doc.name }}</li>
          <li v-if="preview.count > preview.sample.length" class="text-muted-foreground text-xs">
            and {{ preview.count - preview.sample.length }} more
          </li>
        </ul>
      </template>
      <DialogFooter>
        <Button variant="outline" @click="previewFor = null">Close</Button>
        <Button
          :variant="previewFor?.action === 'purge' ? 'destructive' : 'default'"
          class="gap-1.5"
          :disabled="!preview?.count || service.busy.value"
          @click="runNow"
        >
          <PlayIcon class="size-4" />Run now
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
