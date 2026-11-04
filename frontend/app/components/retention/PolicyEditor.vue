<script setup lang="ts">
  import { computed, ref, watch } from "vue";

  import { Button } from "@/components/ui/button";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
  } from "@/components/ui/dialog";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
  } from "@/components/ui/select";
  import { Switch } from "@/components/ui/switch";
  import type { PolicyDraft } from "@/services/retention";
  import type { ManagedFolder, RetentionPolicy } from "@/types/api";
  import { sensitivityOptions } from "@/utils/automation";
  import { policySummary, retentionActions } from "@/utils/retention";

  const open = defineModel<boolean>("open", { required: true });
  const props = defineProps<{
    policy?: RetentionPolicy | null;
    folders: ManagedFolder[];
    busy?: boolean;
  }>();
  const emit = defineEmits<{ save: [draft: PolicyDraft] }>();

  const ALL = "__all__";
  const blank = (): PolicyDraft => ({
    name: "",
    enabled: true,
    folder_id: undefined,
    sensitivity: undefined,
    age_basis: "created",
    keep_days: 365 * 7,
    action: "archive",
  });
  const draft = ref<PolicyDraft>(blank());

  watch(open, (value) => {
    if (!value) return;
    const source = props.policy;
    draft.value = source
      ? {
          name: source.name,
          enabled: source.enabled,
          folder_id: source.folder_id,
          sensitivity: source.sensitivity,
          age_basis: source.age_basis,
          keep_days: source.keep_days,
          action: source.action,
        }
      : blank();
  });

  const folderName = (id?: string) =>
    props.folders.find((folder) => folder.id === id)?.name ?? "a folder";
  const minimum = computed(() => (draft.value.action === "purge" ? 7 : 1));
  const problem = computed(() => {
    if (!draft.value.name.trim()) return "Name the policy.";
    if (!Number.isInteger(draft.value.keep_days) || draft.value.keep_days < minimum.value)
      return `Keep files for at least ${minimum.value} day${minimum.value === 1 ? "" : "s"}.`;
    return "";
  });
  const save = () => {
    if (!problem.value) emit("save", { ...draft.value, name: draft.value.name.trim() });
  };
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ policy ? "Edit retention policy" : "New retention policy" }}</DialogTitle>
        <DialogDescription>
          Policies run every hour. Files on legal hold are always kept.
        </DialogDescription>
      </DialogHeader>
      <div class="space-y-4">
        <div class="space-y-1.5">
          <Label for="policy-name">Name</Label>
          <Input
            id="policy-name"
            v-model="draft.name"
            placeholder="Keep invoices for seven years"
          />
        </div>
        <div class="space-y-1.5">
          <Label>What happens</Label>
          <Select v-model="draft.action">
            <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="item in retentionActions" :key="item.value" :value="item.value">{{
                item.label
              }}</SelectItem>
            </SelectContent>
          </Select>
          <p class="text-muted-foreground text-xs">
            {{ retentionActions.find((item) => item.value === draft.action)?.hint }}
          </p>
        </div>
        <div class="grid gap-3 sm:grid-cols-2">
          <div class="space-y-1.5">
            <Label for="policy-days">After (days)</Label>
            <Input
              id="policy-days"
              v-model.number="draft.keep_days"
              type="number"
              :min="minimum"
              max="36500"
            />
          </div>
          <div v-if="draft.action !== 'purge'" class="space-y-1.5">
            <Label>Counted from</Label>
            <Select v-model="draft.age_basis">
              <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="created">When added</SelectItem>
                <SelectItem value="modified">Last change</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <div class="grid gap-3 sm:grid-cols-2">
          <div class="space-y-1.5">
            <Label>Folder</Label>
            <Select
              :model-value="draft.folder_id ?? ALL"
              @update:model-value="
                (value) => (draft.folder_id = value === ALL ? undefined : String(value))
              "
            >
              <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem :value="ALL">Everywhere</SelectItem>
                <SelectItem v-for="folder in folders" :key="folder.id" :value="folder.id">
                  {{ "— ".repeat(folder.depth) }}{{ folder.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="space-y-1.5">
            <Label>Sensitivity</Label>
            <Select
              :model-value="draft.sensitivity ?? ALL"
              @update:model-value="
                (value) => (draft.sensitivity = value === ALL ? undefined : (value as never))
              "
            >
              <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem :value="ALL">Any</SelectItem>
                <SelectItem
                  v-for="item in sensitivityOptions"
                  :key="item.value"
                  :value="item.value"
                  >{{ item.label }}</SelectItem
                >
              </SelectContent>
            </Select>
          </div>
        </div>
        <p v-if="!problem" class="bg-muted/60 rounded-lg p-3 text-sm">
          {{ policySummary(draft, folderName) }}
        </p>
      </div>
      <DialogFooter class="items-center gap-2 sm:justify-between">
        <label class="flex items-center gap-2 text-sm"
          ><Switch v-model="draft.enabled" />Active</label
        >
        <div class="flex gap-2">
          <Button variant="outline" @click="open = false">Cancel</Button>
          <Button :disabled="Boolean(problem) || busy" :title="problem" @click="save">{{
            policy ? "Save" : "Create policy"
          }}</Button>
        </div>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
