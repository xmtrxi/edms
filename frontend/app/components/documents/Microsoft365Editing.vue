<script setup lang="ts">
  import { ExternalLinkIcon, SaveIcon } from "@lucide/vue";
  import { toast } from "vue-sonner";

  import { Button } from "@/components/ui/button";
  import type { M365Status } from "@/services/microsoft365";

  const props = defineProps<{ documentId: string }>();
  // A check-in stored a new version, so the page should reload it.
  const emit = defineEmits<{ changed: [] }>();

  const m365 = microsoft365Service();
  const state = ref<M365Status | null>(null);

  onMounted(async () => {
    state.value = await m365.status(props.documentId);
  });

  const open = async () => {
    // Open the tab before waiting on the API, or popup blockers stop it.
    const tab = window.open("about:blank", "_blank");
    const opened = await m365.open(props.documentId);
    if (!opened?.web_url) {
      tab?.close();
      toast.error(m365.error.value || "Could not open in Microsoft 365");
      return;
    }
    state.value = opened;
    if (tab) tab.location.href = opened.web_url;
    else window.location.assign(opened.web_url);
  };

  const save = async (done: boolean) => {
    const result = await m365.checkIn(props.documentId, done);
    if (!result) {
      toast.error(m365.error.value || "Could not save the changes");
      return;
    }
    if (result.changed) {
      toast.success("Your Microsoft 365 changes were saved as a new version");
      emit("changed");
    } else {
      toast(
        done
          ? "No changes to save"
          : "No changes yet — Office saves as you type, so try again in a moment",
      );
    }
    if (done) state.value = { open: false };
  };

  const discard = async () => {
    if (!(await m365.discard(props.documentId))) {
      toast.error(m365.error.value || "Could not discard the working copy");
      return;
    }
    state.value = { open: false };
    toast("Microsoft 365 copy discarded");
  };

  defineExpose({ open, state });
</script>

<template>
  <Button size="sm" variant="outline" class="gap-1.5" :disabled="m365.busy.value" @click="open">
    <ExternalLinkIcon class="h-3.5 w-3.5" />
    {{ state?.open ? "Continue in Microsoft 365" : "Open in Microsoft 365" }}
  </Button>
  <Teleport v-if="state?.open" defer to="#document-banners">
    <div
      role="status"
      class="bg-primary/5 border-primary/20 mb-4 flex flex-wrap items-center gap-3 rounded-xl border px-4 py-3 text-sm"
    >
      <ExternalLinkIcon class="text-primary size-4 shrink-0" />
      <span class="min-w-0 flex-1">
        You&apos;re editing a copy in Microsoft 365. Save it here to keep a version; edits are also
        saved every hour.
      </span>
      <Button size="sm" class="gap-1.5" :disabled="m365.busy.value" @click="save(false)">
        <SaveIcon class="size-3.5" /> Save changes
      </Button>
      <Button size="sm" variant="outline" :disabled="m365.busy.value" @click="save(true)"
        >Done editing</Button
      >
      <Button
        size="sm"
        variant="ghost"
        class="text-destructive"
        :disabled="m365.busy.value"
        @click="discard"
      >
        Discard
      </Button>
    </div>
  </Teleport>
</template>
