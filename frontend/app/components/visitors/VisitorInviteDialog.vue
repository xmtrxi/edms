<script setup lang="ts">
  import { CheckIcon, CopyIcon, ExternalLinkIcon, SendIcon } from "@lucide/vue";
  import { computed, reactive, shallowRef, watch } from "vue";
  import { toast } from "vue-sonner";

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
  import type { FormTemplate, VisitorRequestCreated } from "@/types/api";

  const open = defineModel<boolean>({ required: true });
  const props = defineProps<{ template: FormTemplate | null }>();
  const service = visitorsService();
  const created = shallowRef<VisitorRequestCreated | null>(null);
  const form = reactive({ name: "", email: "", expiresAt: "" });
  const canSend = computed(() =>
    Boolean(props.template && form.name.trim() && form.email.includes("@")),
  );

  watch(open, (value) => {
    if (!value) return;
    created.value = null;
    form.name = "";
    form.email = "";
    form.expiresAt = "";
  });

  const create = async () => {
    if (!props.template) return;
    created.value = await service.createRequest({
      template_id: props.template.id,
      visitor_name: form.name.trim(),
      visitor_email: form.email.trim(),
      expires_at: form.expiresAt ? new Date(form.expiresAt).toISOString() : undefined,
    });
  };

  const copy = async (value: string, label: string) => {
    await navigator.clipboard.writeText(value);
    toast.success(`${label} copied`);
  };
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-lg">
      <template v-if="!created">
        <DialogHeader>
          <DialogTitle>Invite a visitor</DialogTitle>
          <DialogDescription
            >Give one person temporary access to {{ template?.name }}. They will verify by email
            before the form opens.</DialogDescription
          >
        </DialogHeader>
        <div class="space-y-5 py-3">
          <div class="space-y-2">
            <Label for="visitor-name">Visitor name</Label
            ><Input id="visitor-name" v-model="form.name" autocomplete="name" />
          </div>
          <div class="space-y-2">
            <Label for="visitor-invite-email">Email address</Label
            ><Input
              id="visitor-invite-email"
              v-model="form.email"
              type="email"
              autocomplete="email"
            />
          </div>
          <div class="space-y-2">
            <Label for="visitor-expiry">Access expires</Label>
            <Input id="visitor-expiry" v-model="form.expiresAt" type="datetime-local" />
            <p class="text-muted-foreground text-xs">If left blank, access lasts 72 hours.</p>
          </div>
          <p v-if="service.error.value" role="alert" class="text-destructive text-sm">
            {{ service.error.value }}
          </p>
        </div>
        <DialogFooter
          ><Button variant="outline" @click="open = false">Cancel</Button
          ><Button :disabled="!canSend || service.loading.value" @click="create"
            ><SendIcon class="mr-2 size-4" />{{
              service.loading.value ? "Creating…" : "Create visitor access"
            }}</Button
          ></DialogFooter
        >
      </template>
      <template v-else>
        <DialogHeader>
          <div class="bg-success/12 text-success mb-3 grid size-10 place-items-center rounded-full">
            <CheckIcon class="size-5" />
          </div>
          <DialogTitle>Visitor access is ready</DialogTitle>
          <DialogDescription
            >Share the link and code using a trusted channel. The access code is only shown
            once.</DialogDescription
          >
        </DialogHeader>
        <div class="space-y-4 py-3">
          <div class="bg-muted rounded-lg p-4">
            <p class="text-muted-foreground text-xs">Access code</p>
            <div class="mt-1 flex items-center justify-between gap-3">
              <strong class="font-mono text-xl tracking-[0.18em]">{{ created.access_code }}</strong
              ><Button
                variant="ghost"
                size="icon"
                aria-label="Copy access code"
                @click="copy(created.access_code, 'Access code')"
                ><CopyIcon class="size-4"
              /></Button>
            </div>
          </div>
          <div class="border-input flex items-center gap-3 rounded-lg border p-3">
            <ExternalLinkIcon class="text-muted-foreground size-4 shrink-0" />
            <span class="min-w-0 flex-1 truncate text-sm">{{ created.public_url }}</span>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Copy visitor link"
              @click="copy(created.public_url, 'Visitor link')"
              ><CopyIcon class="size-4"
            /></Button>
          </div>
        </div>
        <DialogFooter><Button @click="open = false">Done</Button></DialogFooter>
      </template>
    </DialogContent>
  </Dialog>
</template>
