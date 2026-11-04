<script setup lang="ts">
  import { MailIcon, MoveIcon, PlusIcon, ShieldIcon, Trash2Icon, UserPlusIcon } from "@lucide/vue";
  import { computed, ref, shallowRef, watch } from "vue";

  import PrincipalPicker from "@/components/sharing/PrincipalPicker.vue";
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
  import { Textarea } from "@/components/ui/textarea";
  import type { RuleDraft } from "@/services/automation";
  import type {
    AutomationAction,
    AutomationActionType,
    AutomationRule,
    ManagedFolder,
  } from "@/types/api";
  import { ruleSummary, sensitivityOptions, triggerOptions } from "@/utils/automation";

  const open = defineModel<boolean>("open", { required: true });
  const props = defineProps<{
    rule?: AutomationRule | null;
    folders: ManagedFolder[];
    busy?: boolean;
  }>();
  const emit = defineEmits<{ save: [draft: RuleDraft] }>();

  const ANYWHERE = "__anywhere__";
  const ANY = "__any__";

  const blank = (): RuleDraft => ({
    name: "",
    enabled: true,
    trigger: "document.created",
    folder_id: undefined,
    conditions: {},
    actions: [],
  });
  const draft = ref<RuleDraft>(blank());
  const extensions = shallowRef("");
  const emailDrafts = ref<Record<number, string>>({});

  watch(open, (value) => {
    if (!value) return;
    const source = props.rule;
    draft.value = source
      ? structuredClone({
          name: source.name,
          enabled: source.enabled,
          trigger: source.trigger,
          folder_id: source.folder_id,
          conditions: { ...source.conditions },
          actions: source.actions.map((action) => ({ ...action })),
        })
      : blank();
    extensions.value = (draft.value.conditions.extensions ?? []).join(", ");
    emailDrafts.value = Object.fromEntries(
      draft.value.actions.map((action, index) => [index, (action.emails ?? []).join(", ")]),
    );
  });

  const folderName = (id?: string) =>
    props.folders.find((folder) => folder.id === id)?.name ?? "a folder";
  const folderLabel = (folder: ManagedFolder) => `${"— ".repeat(folder.depth)}${folder.name}`;

  const addAction = (type: AutomationActionType) => {
    const action: AutomationAction = { type };
    if (type === "set_sensitivity") action.sensitivity = "confidential";
    if (type === "grant_access") action.role = "viewer";
    if (type === "notify") action.emails = [];
    draft.value.actions.push(action);
    emailDrafts.value[draft.value.actions.length - 1] = "";
  };
  const removeAction = (index: number) => {
    draft.value.actions.splice(index, 1);
    const next: Record<number, string> = {};
    draft.value.actions.forEach((action, i) => {
      next[i] = (action.emails ?? []).join(", ");
    });
    emailDrafts.value = next;
  };

  const actionMeta: Record<AutomationActionType, { label: string; icon: typeof MoveIcon }> = {
    move_to_folder: { label: "Move to a folder", icon: MoveIcon },
    set_sensitivity: { label: "Set sensitivity", icon: ShieldIcon },
    grant_access: { label: "Give access", icon: UserPlusIcon },
    notify: { label: "Email people", icon: MailIcon },
  };

  const splitList = (value: string) =>
    value
      .split(/[,\s]+/)
      .map((item) => item.trim())
      .filter(Boolean);

  const prepared = computed<RuleDraft>(() => ({
    ...draft.value,
    name: draft.value.name.trim(),
    folder_id: draft.value.folder_id || undefined,
    conditions: {
      ...draft.value.conditions,
      name_contains: draft.value.conditions.name_contains?.trim() || undefined,
      classification: draft.value.conditions.classification?.trim() || undefined,
      sensitivity: draft.value.conditions.sensitivity || undefined,
      extensions: splitList(extensions.value).map((ext) => ext.replace(/^\./, "").toLowerCase()),
    },
    actions: draft.value.actions.map((action, index) =>
      action.type === "notify"
        ? { ...action, emails: splitList(emailDrafts.value[index] ?? "") }
        : action,
    ),
  }));

  const problem = computed(() => {
    const value = prepared.value;
    if (!value.name) return "Name the rule.";
    if (!value.actions.length) return "Add at least one action.";
    for (const action of value.actions) {
      if (action.type === "move_to_folder" && !action.folder_id)
        return "Choose where to move files.";
      if (action.type === "grant_access" && !action.principal_id) return "Choose who gets access.";
      if (action.type === "notify" && !action.emails?.length)
        return "Add an email address to notify.";
    }
    return "";
  });

  const save = () => {
    if (!problem.value) emit("save", prepared.value);
  };
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
      <DialogHeader>
        <DialogTitle>{{ rule ? "Edit rule" : "New rule" }}</DialogTitle>
        <DialogDescription>
          Rules run with your permissions, so they can only do what you could do yourself.
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-5">
        <div class="space-y-1.5">
          <Label for="rule-name">Name</Label>
          <Input id="rule-name" v-model="draft.name" placeholder="File supplier contracts" />
        </div>

        <fieldset class="space-y-3">
          <legend class="text-sm font-semibold">When</legend>
          <div class="grid gap-3 sm:grid-cols-2">
            <div class="space-y-1.5">
              <Label>Event</Label>
              <Select v-model="draft.trigger">
                <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="item in triggerOptions"
                    :key="item.value"
                    :value="item.value"
                    >{{ item.label }}</SelectItem
                  >
                </SelectContent>
              </Select>
              <p class="text-muted-foreground text-xs">
                {{ triggerOptions.find((item) => item.value === draft.trigger)?.hint }}
              </p>
            </div>
            <div class="space-y-1.5">
              <Label>Where</Label>
              <Select
                :model-value="draft.folder_id ?? ANYWHERE"
                @update:model-value="
                  (value) => (draft.folder_id = value === ANYWHERE ? undefined : String(value))
                "
              >
                <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem :value="ANYWHERE">Anywhere</SelectItem>
                  <SelectItem v-for="folder in folders" :key="folder.id" :value="folder.id">{{
                    folderLabel(folder)
                  }}</SelectItem>
                </SelectContent>
              </Select>
              <p class="text-muted-foreground text-xs">Includes its subfolders.</p>
            </div>
          </div>
        </fieldset>

        <fieldset class="space-y-3">
          <legend class="text-sm font-semibold">
            Only if <span class="text-muted-foreground font-normal">(optional)</span>
          </legend>
          <div class="grid gap-3 sm:grid-cols-2">
            <div class="space-y-1.5">
              <Label for="rule-name-contains">Name contains</Label>
              <Input
                id="rule-name-contains"
                v-model="draft.conditions.name_contains"
                placeholder="contract"
              />
            </div>
            <div class="space-y-1.5">
              <Label for="rule-extensions">File types</Label>
              <Input id="rule-extensions" v-model="extensions" placeholder="pdf, docx" />
            </div>
            <div class="space-y-1.5">
              <Label>Sensitivity is</Label>
              <Select
                :model-value="draft.conditions.sensitivity || ANY"
                @update:model-value="
                  (value) => (draft.conditions.sensitivity = value === ANY ? '' : (value as never))
                "
              >
                <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem :value="ANY">Any</SelectItem>
                  <SelectItem
                    v-for="item in sensitivityOptions"
                    :key="item.value"
                    :value="item.value"
                    >{{ item.label }}</SelectItem
                  >
                </SelectContent>
              </Select>
            </div>
            <div class="space-y-1.5">
              <Label for="rule-classification">AI classification contains</Label>
              <Input
                id="rule-classification"
                v-model="draft.conditions.classification"
                placeholder="invoice"
              />
            </div>
          </div>
          <label class="flex items-center justify-between gap-3 rounded-lg border p-3 text-sm">
            <span>
              <span class="block font-medium">Holds ID, passport, KRA PIN or card numbers</span>
              <span class="text-muted-foreground block text-xs"
                >Use with “A file has been read”.</span
              >
            </span>
            <Switch v-model="draft.conditions.sensitive_data" />
          </label>
        </fieldset>

        <fieldset class="space-y-3">
          <legend class="text-sm font-semibold">Then</legend>
          <ol class="space-y-2">
            <li
              v-for="(action, index) in draft.actions"
              :key="index"
              class="space-y-2 rounded-lg border p-3"
            >
              <div class="flex items-center gap-2 text-sm font-medium">
                <component
                  :is="actionMeta[action.type].icon"
                  class="text-muted-foreground size-4"
                />
                {{ index + 1 }}. {{ actionMeta[action.type].label }}
                <Button
                  variant="ghost"
                  size="icon"
                  class="ml-auto size-7"
                  :aria-label="`Remove step ${index + 1}`"
                  @click="removeAction(index)"
                >
                  <Trash2Icon class="size-3.5" />
                </Button>
              </div>
              <Select v-if="action.type === 'move_to_folder'" v-model="action.folder_id">
                <SelectTrigger class="w-full"
                  ><SelectValue placeholder="Choose a folder"
                /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="folder in folders" :key="folder.id" :value="folder.id">{{
                    folderLabel(folder)
                  }}</SelectItem>
                </SelectContent>
              </Select>
              <Select v-else-if="action.type === 'set_sensitivity'" v-model="action.sensitivity">
                <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="item in sensitivityOptions"
                    :key="item.value"
                    :value="item.value"
                    >{{ item.label }}</SelectItem
                  >
                </SelectContent>
              </Select>
              <div v-else-if="action.type === 'grant_access'" class="space-y-2">
                <p v-if="action.principal_id" class="flex items-center gap-2 text-sm">
                  <span class="bg-muted rounded px-2 py-1">{{
                    action.principal_name || action.principal_id
                  }}</span>
                  <button
                    type="button"
                    class="text-muted-foreground text-xs underline"
                    @click="action.principal_id = undefined"
                  >
                    Change
                  </button>
                </p>
                <PrincipalPicker
                  v-else
                  placeholder="Person, department, group or role"
                  @select="
                    (option) =>
                      Object.assign(action, {
                        principal_type: option.type,
                        principal_id: option.id,
                        principal_name: option.name,
                      })
                  "
                />
                <Select v-model="action.role">
                  <SelectTrigger class="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="viewer">Can view</SelectItem>
                    <SelectItem value="commenter">Can comment</SelectItem>
                    <SelectItem value="editor">Can edit</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div v-else-if="action.type === 'notify'" class="space-y-2">
                <Input
                  v-model="emailDrafts[index]"
                  placeholder="legal@company.com, finance@company.com"
                  aria-label="Email addresses"
                />
                <Textarea
                  v-model="action.message"
                  rows="2"
                  placeholder="Message (optional)"
                  aria-label="Message"
                />
              </div>
            </li>
          </ol>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="gap-1.5"
                ><PlusIcon class="size-4" />Add step</Button
              >
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <DropdownMenuItem
                v-for="(meta, type) in actionMeta"
                :key="type"
                @click="addAction(type)"
              >
                <component :is="meta.icon" class="size-4" />{{ meta.label }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </fieldset>

        <p v-if="!problem" class="bg-muted/60 rounded-lg p-3 text-sm">
          {{ ruleSummary(prepared, folderName) }}
        </p>
      </div>

      <DialogFooter class="items-center gap-2 sm:justify-between">
        <label class="flex items-center gap-2 text-sm"
          ><Switch v-model="draft.enabled" />Active</label
        >
        <div class="flex gap-2">
          <Button variant="outline" @click="open = false">Cancel</Button>
          <Button :disabled="Boolean(problem) || busy" :title="problem" @click="save">{{
            rule ? "Save" : "Create rule"
          }}</Button>
        </div>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
