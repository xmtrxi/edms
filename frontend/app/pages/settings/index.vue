<script setup lang="ts">
  import { BellIcon, BotIcon, SaveIcon, Settings2Icon, ShieldIcon } from "@lucide/vue";
  import { onMounted, reactive } from "vue";
  import { toast } from "vue-sonner";

  import ApiTokensPanel from "@/components/settings/ApiTokensPanel.vue";
  import StorageSyncPanel from "@/components/settings/StorageSyncPanel.vue";

  definePageMeta({
    title: "Settings",
    description: "Personal office workflow preferences.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Settings" }],
  });
  useHead({ title: "Settings — Lumen DMS" });
  const workspace = workspaceService();
  const auth = useAuthStore();
  const preferences = reactive({
    email_notifications: true,
    workflow_digest: true,
    ai_summaries: false,
    ai_classification: false,
    compact_rows: false,
  });
  const load = async () => {
    Object.assign(preferences, await workspace.settings());
  };
  const save = async () => {
    const saved = await workspace.saveSettings({ ...preferences });
    if (saved) toast.success("Settings saved");
  };
  onMounted(load);
  const options = [
    {
      key: "email_notifications",
      label: "Email notifications",
      description: "Receive important request and route updates.",
      icon: BellIcon,
    },
    {
      key: "workflow_digest",
      label: "Workflow digest",
      description: "Bundle routine movement into a concise daily summary.",
      icon: Settings2Icon,
    },
    {
      key: "ai_summaries",
      label: "AI summaries",
      description: "Allow configured AI to summarize selected office records.",
      icon: BotIcon,
    },
    {
      key: "ai_classification",
      label: "AI classification",
      description: "Suggest record categories without locking you to a provider.",
      icon: ShieldIcon,
    },
  ] as const;
</script>
<template>
  <div class="mx-auto max-w-3xl space-y-5">
    <section class="bg-card rounded-2xl border shadow-sm">
      <div class="border-b p-6">
        <h1 class="text-lg font-semibold">Workspace preferences</h1>
        <p class="text-muted-foreground mt-1 text-sm">
          Saved to your account and applied anywhere you sign in.
        </p>
      </div>
      <div class="divide-y">
        <label
          v-for="item in options"
          :key="item.key"
          class="hover:bg-muted/25 flex cursor-pointer items-center gap-4 p-5 transition"
          ><div class="bg-primary/10 text-primary grid size-10 place-items-center rounded-xl">
            <component :is="item.icon" class="size-5" />
          </div>
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium">{{ item.label }}</p>
            <p class="text-muted-foreground mt-0.5 text-xs">{{ item.description }}</p>
          </div>
          <Switch v-model="preferences[item.key]"
        /></label>
      </div>
    </section>
    <div
      v-if="workspace.error.value"
      class="border-destructive/25 bg-destructive/5 text-destructive rounded-xl border p-3 text-sm"
    >
      {{ workspace.error.value }}
    </div>
    <div class="flex justify-end">
      <Button class="gap-2" :disabled="workspace.loading.value" @click="save"
        ><SaveIcon class="size-4" />{{
          workspace.loading.value ? "Saving…" : "Save preferences"
        }}</Button
      >
    </div>
    <ApiTokensPanel />
    <StorageSyncPanel v-if="auth.isOrgAdmin && auth.meta?.mode !== 'saas'" />
  </div>
</template>
