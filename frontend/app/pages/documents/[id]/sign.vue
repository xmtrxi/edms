<script setup lang="ts">
  import {
    ArrowLeftIcon,
    Loader2Icon,
    MousePointerClickIcon,
    PlusIcon,
    SendIcon,
    Trash2Icon,
    UserPlusIcon,
    XIcon,
  } from "@lucide/vue";
  import { computed, onBeforeUnmount, onMounted, reactive, ref, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import { Button } from "@/components/ui/button";
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
  import PdfViewer from "@/components/viewer/PdfViewer.vue";
  import type { Document, EnvelopeDraft, PdfPageSize, SignatureFieldKind } from "@/types/api";
  import { fieldKinds, fieldMeta, recipientColor } from "@/utils/signing";

  definePageMeta({
    title: "Prepare for signing",
    description: "Add who signs, then place their fields on the document.",
    breadcrumbs: [
      { label: "Workspace", to: "/" },
      { label: "Documents", to: "/documents" },
      { label: "Signing" },
    ],
  });

  interface DraftRecipient {
    key: number;
    name: string;
    email: string;
    role: "signer" | "viewer";
  }
  interface DraftField {
    id: number;
    recipient: number;
    kind: SignatureFieldKind;
    page: number;
    x: number;
    y: number;
    w: number;
    h: number;
    required: boolean;
    label: string;
  }

  const route = useRoute();
  const router = useRouter();
  const auth = useAuthStore();
  const documents = documentsService();
  const signatures = signaturesService();
  const docId = computed(() => String((route.params as Record<string, string>).id ?? ""));

  const doc = shallowRef<Document | null>(null);
  const pdf = shallowRef<Blob | null>(null);
  const loadError = shallowRef("");
  const pages = shallowRef<PdfPageSize[]>([]);

  let nextKey = 1;
  const recipients = ref<DraftRecipient[]>([
    { key: nextKey++, name: "", email: "", role: "signer" },
  ]);
  const fields = ref<DraftField[]>([]);
  const active = shallowRef(recipients.value[0]!.key);
  const placing = shallowRef<SignatureFieldKind | null>(null);
  const selected = shallowRef<number | null>(null);
  const settings = reactive({ title: "", message: "", sequential: false, requireOTP: true });

  useHead(() => ({ title: `Sign ${doc.value?.name ?? "document"} — Lumen DMS` }));

  const indexOf = (key: number) => recipients.value.findIndex((item) => item.key === key);
  const colorOf = (key: number) => recipientColor(indexOf(key));
  const signers = computed(() => recipients.value.filter((item) => item.role === "signer"));
  const activeRecipient = computed(() =>
    recipients.value.find((item) => item.key === active.value),
  );
  const fieldCount = (key: number) => fields.value.filter((item) => item.recipient === key).length;

  onMounted(async () => {
    doc.value = await documents.fetchDocument(docId.value);
    if (!doc.value) {
      loadError.value = "This document couldn’t be opened.";
      return;
    }
    if (!doc.value.permissions?.includes("sign")) {
      loadError.value = "You can’t send this document for signature.";
      return;
    }
    settings.title = doc.value.name.replace(/\.[^.]+$/, "");
    const { $api } = useNuxtApp();
    try {
      const response = await $api.raw<Blob, "blob">(`/documents/${docId.value}/preview`, {
        responseType: "blob",
      });
      if (response.headers.get("X-Preview-Kind") !== "pdf") {
        loadError.value = "Only PDFs and Office documents can be sent for signature.";
        return;
      }
      pdf.value = response._data as Blob;
    } catch {
      loadError.value =
        "The document couldn’t be prepared. Office files need the converter to be running.";
    }
  });

  const addRecipient = (role: "signer" | "viewer" = "signer", name = "", email = "") => {
    const key = nextKey++;
    recipients.value.push({ key, name, email, role });
    if (role === "signer") active.value = key;
  };

  const addMe = () => {
    if (!auth.user) return;
    if (
      recipients.value.some((item) => item.email.toLowerCase() === auth.user!.email.toLowerCase())
    )
      return;
    const blank = recipients.value.find((item) => !item.name && !item.email);
    if (blank) {
      blank.name = auth.user.name;
      blank.email = auth.user.email;
      active.value = blank.key;
    } else addRecipient("signer", auth.user.name, auth.user.email);
  };

  const removeRecipient = (key: number) => {
    recipients.value = recipients.value.filter((item) => item.key !== key);
    fields.value = fields.value.filter((item) => item.recipient !== key);
    if (active.value === key) active.value = signers.value[0]?.key ?? 0;
  };

  const setRole = (recipient: DraftRecipient, role: "signer" | "viewer") => {
    recipient.role = role;
    if (role === "viewer") {
      fields.value = fields.value.filter((item) => item.recipient !== recipient.key);
      if (active.value === recipient.key) active.value = signers.value[0]?.key ?? 0;
    }
  };

  const startPlacing = (kind: SignatureFieldKind) => {
    if (!activeRecipient.value || activeRecipient.value.role !== "signer") {
      toast.info("Choose a signer first");
      return;
    }
    placing.value = placing.value === kind ? null : kind;
  };

  const clamp = (value: number, min: number, max: number) => Math.min(max, Math.max(min, value));

  // Clicking a page while a field type is chosen drops that field there.
  const placeAt = (event: MouseEvent, pageIndex: number) => {
    const kind = placing.value;
    const size = pages.value[pageIndex];
    if (!kind || !size) {
      selected.value = null;
      return;
    }
    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
    const meta = fieldMeta(kind);
    const w = meta.width / size.width;
    const h = meta.height / size.height;
    const x = clamp((event.clientX - rect.left) / rect.width - w / 2, 0, 1 - w);
    const y = clamp((event.clientY - rect.top) / rect.height - h / 2, 0, 1 - h);
    const id = nextKey++;
    fields.value.push({
      id,
      recipient: active.value,
      kind,
      page: pageIndex + 1,
      x,
      y,
      w,
      h,
      required: kind !== "checkbox",
      label: "",
    });
    selected.value = id;
    if (!event.shiftKey) placing.value = null;
  };

  // Dragging moves a field; dragging its corner resizes it.
  let drag: {
    field: DraftField;
    mode: "move" | "resize";
    startX: number;
    startY: number;
    origin: { x: number; y: number; w: number; h: number };
    rect: DOMRect;
  } | null = null;

  const startDrag = (event: PointerEvent, field: DraftField, mode: "move" | "resize") => {
    event.stopPropagation();
    selected.value = field.id;
    const page = (event.currentTarget as HTMLElement).closest("[data-page]") as HTMLElement | null;
    if (!page) return;
    drag = {
      field,
      mode,
      startX: event.clientX,
      startY: event.clientY,
      origin: { x: field.x, y: field.y, w: field.w, h: field.h },
      rect: page.getBoundingClientRect(),
    };
    window.addEventListener("pointermove", onDrag);
    window.addEventListener("pointerup", endDrag, { once: true });
  };

  const onDrag = (event: PointerEvent) => {
    if (!drag) return;
    const dx = (event.clientX - drag.startX) / drag.rect.width;
    const dy = (event.clientY - drag.startY) / drag.rect.height;
    const { field, origin } = drag;
    if (drag.mode === "move") {
      field.x = clamp(origin.x + dx, 0, 1 - field.w);
      field.y = clamp(origin.y + dy, 0, 1 - field.h);
    } else {
      field.w = clamp(origin.w + dx, 0.02, 1 - field.x);
      field.h = clamp(origin.h + dy, 0.015, 1 - field.y);
    }
  };

  const endDrag = () => {
    drag = null;
    window.removeEventListener("pointermove", onDrag);
  };

  const removeField = (id: number) => {
    fields.value = fields.value.filter((item) => item.id !== id);
    if (selected.value === id) selected.value = null;
  };

  const onKey = (event: KeyboardEvent) => {
    const target = event.target as HTMLElement | null;
    if (target && ["INPUT", "TEXTAREA"].includes(target.tagName)) return;
    if ((event.key === "Delete" || event.key === "Backspace") && selected.value !== null) {
      removeField(selected.value);
    }
    if (event.key === "Escape") placing.value = null;
  };
  onMounted(() => window.addEventListener("keydown", onKey));
  onBeforeUnmount(() => {
    window.removeEventListener("keydown", onKey);
    endDrag();
  });

  const selectedField = computed(() => fields.value.find((item) => item.id === selected.value));

  const problems = computed(() => {
    const list: string[] = [];
    if (!signers.value.length) list.push("Add at least one signer.");
    for (const recipient of recipients.value) {
      const who = recipient.name || recipient.email || "A recipient";
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(recipient.email.trim()))
        list.push(`${who} needs a valid email address.`);
      if (recipient.role === "signer" && !fieldCount(recipient.key))
        list.push(`Place at least one field for ${who}.`);
    }
    return list;
  });

  const sending = shallowRef(false);
  const send = async () => {
    if (problems.value.length) {
      toast.error(problems.value[0]!);
      return;
    }
    const indexes = new Map(recipients.value.map((item, index) => [item.key, index]));
    const draft: EnvelopeDraft = {
      title: settings.title.trim(),
      message: settings.message.trim(),
      sequential: settings.sequential,
      require_otp: settings.requireOTP,
      recipients: recipients.value.map((item) => ({
        name: item.name.trim(),
        email: item.email.trim(),
        role: item.role,
      })),
      fields: fields.value.map((item) => ({
        recipient: indexes.get(item.recipient)!,
        kind: item.kind,
        page: item.page,
        x: item.x,
        y: item.y,
        w: item.w,
        h: item.h,
        required: item.required,
        label: item.label.trim() || undefined,
      })),
    };
    sending.value = true;
    const envelope = await signatures.send(docId.value, draft);
    sending.value = false;
    if (!envelope) {
      toast.error(signatures.error.value || "Could not send for signature");
      return;
    }
    const count = envelope.recipients.filter((item) => item.status === "sent").length;
    toast.success(
      settings.sequential
        ? `Sent to ${envelope.recipients[0]?.name}; the others follow in order`
        : `Sent to ${count} ${count === 1 ? "person" : "people"}`,
    );
    await router.push(`/documents/${docId.value}#signatures`);
  };
</script>

<template>
  <Teleport to="#page-actions-target">
    <Button variant="outline" size="sm" class="gap-1.5" :to="`/documents/${docId}`" as-child>
      <NuxtLink :to="`/documents/${docId}`"><ArrowLeftIcon class="size-4" />Back</NuxtLink>
    </Button>
    <Button size="sm" class="gap-1.5" :disabled="sending || !pdf" @click="send">
      <Loader2Icon v-if="sending" class="size-4 animate-spin" />
      <SendIcon v-else class="size-4" />Send for signature
    </Button>
  </Teleport>

  <div v-if="loadError" class="bg-card mx-auto max-w-lg rounded-2xl border p-8 text-center">
    <p class="text-sm font-medium">{{ loadError }}</p>
    <Button variant="outline" size="sm" class="mt-4" as-child>
      <NuxtLink :to="`/documents/${docId}`">Back to the document</NuxtLink>
    </Button>
  </div>

  <div v-else class="grid gap-4 lg:h-[calc(100dvh-12rem)] lg:grid-cols-[1fr_340px]">
    <div class="bg-card relative min-h-[60dvh] overflow-hidden rounded-xl border lg:min-h-0">
      <div v-if="!pdf" class="text-muted-foreground grid h-full place-items-center text-sm">
        <span class="flex items-center gap-2"
          ><Loader2Icon class="size-4 animate-spin" />Opening the document…</span
        >
      </div>
      <PdfViewer v-else :source="pdf" :initial-scale="1.1" @loaded="pages = $event">
        <template #page="{ index }">
          <div
            :data-page="index + 1"
            class="absolute inset-0"
            :class="placing && 'cursor-crosshair'"
            @click="placeAt($event, index)"
          >
            <div
              v-for="field in fields.filter((item) => item.page === index + 1)"
              :key="field.id"
              class="group absolute flex cursor-move items-center gap-1 overflow-hidden rounded-[3px] border-2 px-1 text-[11px] font-medium select-none"
              :class="selected === field.id && 'ring-2 ring-offset-1'"
              :style="{
                left: `${field.x * 100}%`,
                top: `${field.y * 100}%`,
                width: `${field.w * 100}%`,
                height: `${field.h * 100}%`,
                borderColor: colorOf(field.recipient),
                backgroundColor: `${colorOf(field.recipient)}1f`,
                color: colorOf(field.recipient),
                '--tw-ring-color': colorOf(field.recipient),
              }"
              role="button"
              tabindex="0"
              :aria-label="`${fieldMeta(field.kind).label} for ${recipients[indexOf(field.recipient)]?.name || 'recipient'}`"
              @click.stop="selected = field.id"
              @pointerdown="startDrag($event, field, 'move')"
            >
              <component
                :is="fieldMeta(field.kind).icon"
                v-if="field.kind !== 'checkbox'"
                class="size-3 shrink-0"
              />
              <span v-if="field.kind !== 'checkbox'" class="truncate">
                {{ field.label || fieldMeta(field.kind).label
                }}{{ field.required ? "" : " (optional)" }}
              </span>
              <button
                type="button"
                class="absolute -top-0 -right-0 hidden size-4 place-items-center rounded-bl bg-white/90 group-hover:grid"
                aria-label="Remove field"
                @pointerdown.stop
                @click.stop="removeField(field.id)"
              >
                <XIcon class="size-3" />
              </button>
              <span
                class="absolute right-0 bottom-0 size-2.5 cursor-se-resize"
                :style="{ backgroundColor: colorOf(field.recipient) }"
                @pointerdown="startDrag($event, field, 'resize')"
              />
            </div>
          </div>
        </template>
      </PdfViewer>
      <div
        v-if="placing"
        class="bg-foreground text-background pointer-events-none absolute bottom-4 left-1/2 flex -translate-x-1/2 items-center gap-2 rounded-full px-4 py-2 text-xs shadow-lg"
      >
        <MousePointerClickIcon class="size-4" />
        Click the page to place {{ fieldMeta(placing).label.toLowerCase() }} for
        {{ activeRecipient?.name || "this signer" }} · Shift-click to place several · Esc to stop
      </div>
    </div>

    <aside class="space-y-4 overflow-y-auto lg:pr-1">
      <section class="bg-card space-y-3 rounded-xl border p-4">
        <div class="flex items-center justify-between">
          <h2 class="text-sm font-semibold">Recipients</h2>
          <Button variant="ghost" size="sm" class="h-7 text-xs" @click="addMe">Add me</Button>
        </div>
        <div
          v-for="(recipient, index) in recipients"
          :key="recipient.key"
          class="space-y-2 rounded-lg border p-3 transition-colors"
          :class="active === recipient.key ? 'bg-muted/60' : ''"
          :style="{ borderLeft: `4px solid ${recipientColor(index)}` }"
          @click="recipient.role === 'signer' && (active = recipient.key)"
        >
          <div class="flex items-center gap-2">
            <span class="text-muted-foreground text-xs tabular-nums">{{ index + 1 }}</span>
            <Input
              v-model="recipient.name"
              placeholder="Name"
              class="h-8"
              :aria-label="`Recipient ${index + 1} name`"
            />
            <Button
              variant="ghost"
              size="icon"
              class="size-8 shrink-0"
              :aria-label="`Remove recipient ${index + 1}`"
              :disabled="recipients.length === 1"
              @click.stop="removeRecipient(recipient.key)"
            >
              <Trash2Icon class="size-4" />
            </Button>
          </div>
          <Input
            v-model="recipient.email"
            type="email"
            placeholder="name@company.com"
            class="h-8"
            :aria-label="`Recipient ${index + 1} email`"
          />
          <div class="flex items-center justify-between gap-2">
            <Select
              :model-value="recipient.role"
              @update:model-value="(value) => setRole(recipient, value as 'signer' | 'viewer')"
            >
              <SelectTrigger class="h-8 w-36 text-xs" @click.stop><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="signer">Needs to sign</SelectItem>
                <SelectItem value="viewer">Gets a copy</SelectItem>
              </SelectContent>
            </Select>
            <span v-if="recipient.role === 'signer'" class="text-muted-foreground text-xs">
              {{ fieldCount(recipient.key) }} field{{ fieldCount(recipient.key) === 1 ? "" : "s" }}
            </span>
          </div>
        </div>
        <div class="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            class="flex-1 gap-1.5"
            @click="addRecipient('signer')"
          >
            <UserPlusIcon class="size-4" />Signer
          </Button>
          <Button
            variant="outline"
            size="sm"
            class="flex-1 gap-1.5"
            @click="addRecipient('viewer')"
          >
            <PlusIcon class="size-4" />Copy
          </Button>
        </div>
        <label class="flex items-center justify-between gap-3 text-sm">
          <span>
            <span class="block font-medium">Sign in order</span>
            <span class="text-muted-foreground block text-xs"
              >Each signer is emailed after the one before.</span
            >
          </span>
          <Switch v-model="settings.sequential" />
        </label>
      </section>

      <section class="bg-card space-y-3 rounded-xl border p-4">
        <div>
          <h2 class="text-sm font-semibold">Fields</h2>
          <p class="text-muted-foreground text-xs">
            For
            <span class="font-medium" :style="{ color: colorOf(active) }">{{
              activeRecipient?.name || activeRecipient?.email || "the selected signer"
            }}</span>
            — pick one, then click the page.
          </p>
        </div>
        <div class="grid grid-cols-2 gap-2">
          <Button
            v-for="item in fieldKinds"
            :key="item.kind"
            variant="outline"
            size="sm"
            class="justify-start gap-2"
            :class="placing === item.kind && 'border-primary bg-primary/5'"
            :aria-pressed="placing === item.kind"
            :title="item.hint"
            @click="startPlacing(item.kind)"
          >
            <component :is="item.icon" class="size-4" />{{ item.label }}
          </Button>
        </div>
        <div v-if="selectedField" class="space-y-2 border-t pt-3">
          <p class="text-xs font-medium">
            Selected: {{ fieldMeta(selectedField.kind).label }} on page {{ selectedField.page }}
          </p>
          <Input
            v-if="selectedField.kind === 'text'"
            v-model="selectedField.label"
            class="h-8"
            placeholder="Label, e.g. Job title"
            aria-label="Field label"
          />
          <label class="flex items-center justify-between text-sm">
            Required
            <Switch v-model="selectedField.required" />
          </label>
          <Button
            variant="ghost"
            size="sm"
            class="text-destructive w-full gap-1.5"
            @click="removeField(selectedField.id)"
          >
            <Trash2Icon class="size-4" />Remove field
          </Button>
        </div>
      </section>

      <section class="bg-card space-y-3 rounded-xl border p-4">
        <h2 class="text-sm font-semibold">Message</h2>
        <div class="space-y-1.5">
          <Label for="envelope-title">Subject</Label>
          <Input id="envelope-title" v-model="settings.title" class="h-8" />
        </div>
        <div class="space-y-1.5">
          <Label for="envelope-message"
            >Note to signers <span class="text-muted-foreground">(optional)</span></Label
          >
          <Textarea
            id="envelope-message"
            v-model="settings.message"
            rows="3"
            placeholder="Please sign by Friday."
          />
        </div>
        <label class="flex items-center justify-between gap-3 text-sm">
          <span>
            <span class="block font-medium">Email code check</span>
            <span class="text-muted-foreground block text-xs"
              >Signers confirm a code sent to their email.</span
            >
          </span>
          <Switch v-model="settings.requireOTP" />
        </label>
      </section>
    </aside>
  </div>
</template>
