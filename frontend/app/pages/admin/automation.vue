<script setup lang="ts">
  import {
    CheckCircle2Icon,
    HistoryIcon,
    MoreHorizontalIcon,
    PlusIcon,
    XCircleIcon,
    ZapIcon,
  } from "@lucide/vue";
  import { onMounted, ref, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import RuleEditor from "@/components/automation/RuleEditor.vue";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
  } from "@/components/ui/dialog";
  import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
  } from "@/components/ui/dropdown-menu";
  import { Switch } from "@/components/ui/switch";
  import type { RuleDraft } from "@/services/automation";
  import type { AutomationRule, AutomationRun } from "@/types/api";
  import { ruleSummary } from "@/utils/automation";
  import { formatDateTime } from "@/utils/format";

  definePageMeta({
    title: "Automation",
    description: "Rules that file, label, share and announce documents for you.",
    breadcrumbs: [{ label: "Administration", to: "/" }, { label: "Automation" }],
  });
  useHead({ title: "Automation — Lumen DMS" });

  const auth = useAuthStore();
  const service = automationService();
  const folderService = foldersService();
  const folders = folderService.folders;
  const rules = ref<AutomationRule[]>([]);
  const loaded = shallowRef(false);

  const folderName = (id?: string) =>
    folders.value.find((folder) => folder.id === id)?.name ?? "a folder";

  const load = async () => {
    rules.value = await service.rules();
    loaded.value = true;
  };
  onMounted(() => {
    void folderService.fetchFolders();
    void load();
  });

  const editorOpen = shallowRef(false);
  const editing = shallowRef<AutomationRule | null>(null);
  const newRule = () => {
    editing.value = null;
    editorOpen.value = true;
  };
  const editRule = (rule: AutomationRule) => {
    editing.value = rule;
    editorOpen.value = true;
  };
  const saveRule = async (draft: RuleDraft) => {
    const saved = await service.saveRule(draft, editing.value?.id);
    if (!saved) {
      toast.error(service.error.value);
      return;
    }
    editorOpen.value = false;
    toast.success(editing.value ? "Rule saved" : "Rule created");
    await load();
  };
  const toggleRule = async (rule: AutomationRule, enabled: boolean) => {
    const draft: RuleDraft = {
      name: rule.name,
      enabled,
      trigger: rule.trigger,
      folder_id: rule.folder_id,
      conditions: rule.conditions,
      actions: rule.actions,
    };
    if (await service.saveRule(draft, rule.id)) {
      rule.enabled = enabled;
      toast.success(enabled ? "Rule turned on" : "Rule paused");
    } else toast.error(service.error.value);
  };
  const deleteRule = async (rule: AutomationRule) => {
    if (await service.deleteRule(rule.id)) {
      rules.value = rules.value.filter((item) => item.id !== rule.id);
      toast.success("Rule deleted");
    } else toast.error(service.error.value);
  };

  const historyFor = shallowRef<AutomationRule | null>(null);
  const history = ref<AutomationRun[]>([]);
  const showHistory = async (rule: AutomationRule) => {
    historyFor.value = rule;
    history.value = await service.runs(rule.id);
  };
</script>

<template>
  <div
    v-if="!auth.isOrgAdmin"
    class="bg-card mx-auto max-w-md rounded-2xl border p-8 text-center text-sm"
  >
    Only organization admins can manage automation.
  </div>

  <template v-else>
    <Teleport to="#page-actions-target">
      <Button size="sm" class="gap-1.5" @click="newRule"
        ><PlusIcon class="size-4" />New rule</Button
      >
    </Teleport>

    <div class="mx-auto max-w-4xl space-y-3">
      <div v-if="loaded && !rules.length" class="bg-card rounded-2xl border p-10 text-center">
        <ZapIcon class="text-muted-foreground mx-auto size-8" />
        <p class="mt-3 font-semibold">No rules yet</p>
        <p class="text-muted-foreground mx-auto mt-1 max-w-md text-sm">
          File contracts into the right folder, mark ID scans confidential, or tell legal when
          something is signed — without anyone remembering to.
        </p>
        <Button size="sm" class="mt-4" @click="newRule">Create a rule</Button>
      </div>
      <article
        v-for="rule in rules"
        :key="rule.id"
        class="bg-card rounded-xl border p-4"
        :class="!rule.enabled && 'opacity-70'"
      >
        <div class="flex items-start gap-3">
          <div class="min-w-0 flex-1">
            <p class="flex items-center gap-2 font-medium">
              {{ rule.name }}
              <Badge v-if="!rule.enabled" variant="outline" class="h-5 px-1.5 text-[10px]"
                >Paused</Badge
              >
            </p>
            <p class="text-muted-foreground mt-1 text-sm">{{ ruleSummary(rule, folderName) }}</p>
            <p class="text-muted-foreground mt-2 text-xs">
              Ran {{ rule.run_count }} {{ rule.run_count === 1 ? "time" : "times"
              }}<template v-if="rule.last_run_at">
                · last {{ formatDateTime(rule.last_run_at) }}</template
              >
            </p>
          </div>
          <Switch
            :model-value="rule.enabled"
            :aria-label="`${rule.name} active`"
            @update:model-value="(value) => toggleRule(rule, Boolean(value))"
          />
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button
                variant="ghost"
                size="icon"
                class="size-8"
                :aria-label="`${rule.name} actions`"
                ><MoreHorizontalIcon class="size-4"
              /></Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem @click="editRule(rule)">Edit</DropdownMenuItem>
              <DropdownMenuItem @click="showHistory(rule)"
                ><HistoryIcon class="size-4" />History</DropdownMenuItem
              >
              <DropdownMenuItem class="text-destructive" @click="deleteRule(rule)"
                >Delete</DropdownMenuItem
              >
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </article>
    </div>
  </template>

  <RuleEditor
    v-model:open="editorOpen"
    :rule="editing"
    :folders="folders"
    :busy="service.busy.value"
    @save="saveRule"
  />

  <Dialog :open="Boolean(historyFor)" @update:open="(value) => !value && (historyFor = null)">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ historyFor?.name }}</DialogTitle>
        <DialogDescription>The last 100 times this rule ran.</DialogDescription>
      </DialogHeader>
      <p v-if="!history.length" class="text-muted-foreground text-sm">It hasn’t run yet.</p>
      <ul v-else class="max-h-96 space-y-2 overflow-y-auto">
        <li v-for="run in history" :key="run.id" class="flex items-start gap-2 text-sm">
          <CheckCircle2Icon
            v-if="run.status === 'ok'"
            class="mt-0.5 size-4 shrink-0 text-emerald-600"
          />
          <XCircleIcon v-else class="text-destructive mt-0.5 size-4 shrink-0" />
          <span class="min-w-0 flex-1">
            <NuxtLink :to="`/documents/${run.document_id}`" class="hover:underline"
              >Open document</NuxtLink
            >
            <span class="text-muted-foreground"> · {{ formatDateTime(run.created_at) }}</span>
            <span v-if="run.detail" class="text-destructive block text-xs">{{ run.detail }}</span>
          </span>
        </li>
      </ul>
    </DialogContent>
  </Dialog>
</template>
