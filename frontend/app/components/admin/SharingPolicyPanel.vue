<script setup lang="ts">
  import { onMounted, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import { Switch } from "@/components/ui/switch";
  import type { SharePolicy } from "@/types/api";

  const sharing = sharingService();
  const policy = shallowRef<SharePolicy | null>(null);
  const domains = shallowRef("");
  const saving = shallowRef(false);

  onMounted(async () => {
    policy.value = await sharing.policy();
    domains.value = policy.value?.allowed_domains.join(", ") ?? "";
  });

  const update = (changes: Partial<SharePolicy>) => {
    if (policy.value) policy.value = { ...policy.value, ...changes };
  };

  const save = async () => {
    if (!policy.value) return;
    saving.value = true;
    const saved = await sharing.updatePolicy({
      ...policy.value,
      allowed_domains: domains.value
        .split(/[\s,]+/)
        .map((item) => item.trim())
        .filter(Boolean),
    });
    saving.value = false;
    if (!saved) {
      toast.error(sharing.error.value || "Could not save the sharing policy");
      return;
    }
    policy.value = saved;
    domains.value = saved.allowed_domains.join(", ");
    toast.success("Sharing policy saved");
  };
</script>

<template>
  <form
    v-if="policy"
    class="bg-card max-w-2xl space-y-1 rounded-2xl border p-5 shadow-sm"
    @submit.prevent="save"
  >
    <h2 class="text-lg font-semibold">Sharing outside the organization</h2>
    <p class="text-muted-foreground pb-3 text-sm">
      These rules apply to everyone, including admins. Restricted files are never shared outside.
    </p>

    <div class="flex items-start gap-4 border-t py-4">
      <div class="flex-1">
        <Label for="policy-external">People outside the organization</Label>
        <p class="text-muted-foreground mt-1 text-xs">
          Let members share files with email addresses that aren’t part of the organization.
        </p>
      </div>
      <Switch
        id="policy-external"
        :model-value="policy.allow_external"
        @update:model-value="(value) => update({ allow_external: value })"
      />
    </div>

    <div class="border-t py-4">
      <Label for="policy-domains">Allowed email domains</Label>
      <p class="text-muted-foreground mt-1 mb-2 text-xs">
        Leave empty to allow any domain, or list partners, e.g. kpmg.com, partner.co.ke
      </p>
      <Input
        id="policy-domains"
        v-model="domains"
        :disabled="!policy.allow_external"
        placeholder="Any domain"
      />
    </div>

    <div class="flex items-start gap-4 border-t py-4">
      <div class="flex-1">
        <Label for="policy-public">Anyone-with-the-link sharing</Label>
        <p class="text-muted-foreground mt-1 text-xs">
          Public links open without signing in. They are always view-only and never available for
          confidential files.
        </p>
      </div>
      <Switch
        id="policy-public"
        :model-value="policy.allow_public_links"
        @update:model-value="(value) => update({ allow_public_links: value })"
      />
    </div>

    <div class="border-t py-4">
      <Label for="policy-days">Longest a link or external share can last (days)</Label>
      <Input
        id="policy-days"
        :model-value="policy.max_link_days"
        type="number"
        min="1"
        max="3650"
        class="mt-2 w-32"
        @update:model-value="(value) => update({ max_link_days: Number(value) || 1 })"
      />
    </div>

    <div class="border-t pt-4">
      <Button type="submit" :disabled="saving">{{ saving ? "Saving…" : "Save policy" }}</Button>
    </div>
  </form>
  <Skeleton v-else class="h-80 max-w-2xl rounded-2xl" />
</template>
