<script setup lang="ts">
  import { PenLineIcon, PlusIcon, Trash2Icon } from "@lucide/vue";
  import { computed, reactive, watch } from "vue";

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

  interface Recipient {
    email: string;
    name: string;
    role: string;
  }

  const open = defineModel<boolean>("open", { required: true });
  const props = defineProps<{ documentTitle: string; busy?: boolean; error?: string }>();
  const emit = defineEmits<{ submit: [input: { title: string; recipients: Recipient[] }] }>();
  const form = reactive<{ title: string; recipients: Recipient[] }>({ title: "", recipients: [] });
  const addRecipient = () => form.recipients.push({ email: "", name: "", role: "signer" });
  const removeRecipient = (index: number) => form.recipients.splice(index, 1);
  const canSubmit = computed(
    () =>
      !props.busy &&
      form.recipients.length > 0 &&
      form.recipients.every((item) => item.email.includes("@")),
  );
  watch(open, (value) => {
    if (!value) return;
    form.title = props.documentTitle;
    form.recipients = [{ email: "", name: "", role: "signer" }];
  });
  const submit = () => {
    if (!canSubmit.value) return;
    emit("submit", {
      title: form.title.trim() || props.documentTitle,
      recipients: form.recipients.map((item) => ({
        ...item,
        email: item.email.trim(),
        name: item.name.trim(),
      })),
    });
  };
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-xl">
      <DialogHeader>
        <div class="bg-primary/10 text-primary mb-2 grid size-9 place-items-center rounded-lg">
          <PenLineIcon class="size-4" />
        </div>
        <DialogTitle>Request signatures</DialogTitle>
        <DialogDescription
          >The configured signing service will email each signer and return the completed file as a
          new document version.</DialogDescription
        >
      </DialogHeader>
      <div class="space-y-4 py-2">
        <div class="space-y-2">
          <Label for="signature-title">Request title</Label>
          <Input id="signature-title" v-model="form.title" />
        </div>
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <Label>Recipients</Label>
            <Button variant="ghost" size="sm" type="button" @click="addRecipient"
              ><PlusIcon class="mr-2 size-4" />Add signer</Button
            >
          </div>
          <div
            v-for="(recipient, index) in form.recipients"
            :key="index"
            class="border-border grid gap-3 rounded-lg border p-3 sm:grid-cols-[1fr_1.4fr_auto]"
          >
            <Input
              v-model="recipient.name"
              :aria-label="`Signer ${index + 1} name`"
              placeholder="Full name"
            />
            <Input
              v-model="recipient.email"
              :aria-label="`Signer ${index + 1} email`"
              type="email"
              placeholder="name@company.com"
            />
            <Button
              variant="ghost"
              size="icon"
              type="button"
              aria-label="Remove signer"
              :disabled="form.recipients.length === 1"
              @click="removeRecipient(index)"
              ><Trash2Icon class="size-4"
            /></Button>
          </div>
        </div>
        <p v-if="error" role="alert" class="text-destructive text-sm">{{ error }}</p>
      </div>
      <DialogFooter>
        <Button variant="outline" @click="open = false">Cancel</Button>
        <Button :disabled="!canSubmit" @click="submit">{{
          busy ? "Sending…" : "Send for signature"
        }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
