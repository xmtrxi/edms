<script setup lang="ts">
  import {
    ArrowDownIcon,
    CheckCircle2Icon,
    CircleSlashIcon,
    CloudDownloadIcon,
    Loader2Icon,
    MailCheckIcon,
    ShieldCheckIcon,
  } from "@lucide/vue";
  import { computed, onMounted, reactive, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import SignaturePad from "@/components/signing/SignaturePad.vue";
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
  import { Textarea } from "@/components/ui/textarea";
  import PdfViewer from "@/components/viewer/PdfViewer.vue";
  import type { PdfPageSize, SignatureField, SigningSession } from "@/types/api";
  import { fieldMeta } from "@/utils/signing";

  definePageMeta({ layout: "auth" });

  const route = useRoute();
  const service = signaturesService();
  const token = computed(() => String((route.params as Record<string, string>).token ?? ""));

  const session = shallowRef<SigningSession | null>(null);
  const missing = shallowRef("");
  const pdf = shallowRef<Blob | null>(null);
  const pages = shallowRef<PdfPageSize[]>([]);
  const values = reactive<Record<string, string>>({});
  const consent = shallowRef(false);
  const submitting = shallowRef(false);

  useHead(() => ({
    title: session.value ? `Sign ${session.value.envelope.title} — Lumen DMS` : "Sign — Lumen DMS",
    meta: [{ name: "robots", content: "noindex" }],
  }));

  const envelope = computed(() => session.value?.envelope);
  const state = computed(() => {
    const current = session.value;
    if (!current) return "loading";
    if (current.envelope.status === "completed") return "completed";
    if (current.envelope.status === "voided") return "voided";
    if (current.envelope.status === "declined") return "declined";
    if (current.recipient.status === "signed") return "signed";
    if (current.recipient.role === "viewer") return "copy";
    if (!current.can_sign) return "waiting";
    if (!current.verified) return "verify";
    return "sign";
  });

  const loadPdf = async () => {
    pdf.value = await service.documentBlob(token.value);
  };

  const apply = (next: SigningSession) => {
    session.value = next;
    for (const field of next.fields) {
      if (values[field.id] !== undefined) continue;
      if (field.kind === "name") values[field.id] = field.value || next.recipient.name;
      else if (field.kind === "checkbox") values[field.id] = field.value === "true" ? "true" : "";
      else values[field.id] = field.value ?? "";
    }
  };

  onMounted(async () => {
    const opened = await service.open(token.value);
    if (!opened) {
      missing.value = service.error.value || "This signing link isn’t available.";
      return;
    }
    apply(opened);
    await loadPdf();
  });

  // ── Email code ────────────────────────────────────────────────────────────
  const codeSent = shallowRef(false);
  const code = shallowRef("");
  const sendCode = async () => {
    if (await service.sendCode(token.value)) {
      codeSent.value = true;
      toast.success(`Code sent to ${session.value?.recipient.email}`);
    } else toast.error(service.error.value);
  };
  const verify = async () => {
    const next = await service.verifyCode(token.value, code.value.trim());
    if (next) {
      apply(next);
      toast.success("Thanks — you can sign now");
    } else toast.error(service.error.value || "That code didn’t work");
  };

  // ── Fields ────────────────────────────────────────────────────────────────
  const today = new Intl.DateTimeFormat("en-GB", {
    day: "numeric",
    month: "long",
    year: "numeric",
  }).format(new Date());
  const isDone = (field: SignatureField) => {
    if (field.kind === "date") return true;
    if (field.kind === "checkbox") return !field.required || values[field.id] === "true";
    return Boolean(values[field.id]?.trim());
  };
  const required = computed(() => (session.value?.fields ?? []).filter((field) => field.required));
  const remaining = computed(() => required.value.filter((field) => !isDone(field)));
  const pageFields = (index: number) =>
    (session.value?.fields ?? []).filter((field) => field.page === index + 1);

  const padOpen = shallowRef(false);
  const padField = shallowRef<SignatureField | null>(null);
  const adopted = reactive<{ signature: string; initials: string }>({
    signature: "",
    initials: "",
  });
  const padAspect = computed(() => {
    const field = padField.value;
    const size = field ? pages.value[field.page - 1] : undefined;
    return field && size ? (field.w * size.width) / (field.h * size.height) : 3;
  });

  const openField = (field: SignatureField) => {
    if (state.value !== "sign") return;
    if (field.kind === "signature" || field.kind === "initials") {
      // Reuse an adopted signature for the rest of the document.
      const saved = adopted[field.kind];
      if (saved && !values[field.id]) {
        values[field.id] = saved;
        return;
      }
      padField.value = field;
      padOpen.value = true;
    } else if (field.kind === "checkbox") {
      values[field.id] = values[field.id] === "true" ? "" : "true";
    }
  };

  const adopt = (dataUrl: string) => {
    const field = padField.value;
    if (!field) return;
    const kind = field.kind as "signature" | "initials";
    adopted[kind] = dataUrl;
    values[field.id] = dataUrl;
  };

  const goToNext = () => {
    const next = remaining.value[0];
    if (!next) return;
    document
      .querySelector(`[data-field="${next.id}"]`)
      ?.scrollIntoView({ behavior: "smooth", block: "center" });
  };

  const finish = async () => {
    if (remaining.value.length) {
      goToNext();
      toast.error("Complete the highlighted fields first");
      return;
    }
    submitting.value = true;
    const payload: Record<string, string> = {};
    for (const field of session.value?.fields ?? []) {
      if (field.kind !== "date" && values[field.id]) payload[field.id] = values[field.id]!;
    }
    const next = await service.complete(token.value, payload);
    submitting.value = false;
    if (!next) {
      toast.error(service.error.value || "Signing didn’t go through");
      return;
    }
    apply(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
    await loadPdf();
  };

  // ── Decline ───────────────────────────────────────────────────────────────
  const declineOpen = shallowRef(false);
  const declineReason = shallowRef("");
  const decline = async () => {
    if (!(await service.decline(token.value, declineReason.value.trim()))) {
      toast.error(service.error.value);
      return;
    }
    declineOpen.value = false;
    const reopened = await service.open(token.value);
    if (reopened) apply(reopened);
  };
</script>

<template>
  <div class="bg-muted/40 min-h-dvh">
    <header class="bg-card/95 sticky top-0 z-20 border-b backdrop-blur">
      <div class="mx-auto flex max-w-6xl flex-wrap items-center gap-3 px-4 py-3">
        <div
          class="bg-primary text-primary-foreground grid size-8 place-items-center rounded-lg font-bold"
        >
          L
        </div>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-semibold">{{ envelope?.title ?? "Signature request" }}</p>
          <p v-if="envelope" class="text-muted-foreground truncate text-xs">
            From {{ envelope.sender_name
            }}<template v-if="session?.organization"> · {{ session.organization }}</template>
          </p>
        </div>
        <template v-if="state === 'sign'">
          <span class="text-muted-foreground hidden text-xs sm:inline">
            {{ required.length - remaining.length }} of {{ required.length }} required done
          </span>
          <Button
            v-if="remaining.length"
            variant="outline"
            size="sm"
            class="gap-1.5"
            @click="goToNext"
          >
            <ArrowDownIcon class="size-4" />Next field
          </Button>
          <Button variant="ghost" size="sm" @click="declineOpen = true">Decline</Button>
        </template>
        <Button v-if="state === 'completed'" size="sm" class="gap-1.5" as-child>
          <a :href="service.downloadUrl(token)"
            ><CloudDownloadIcon class="size-4" />Download signed copy</a
          >
        </Button>
      </div>
    </header>

    <main class="mx-auto max-w-6xl space-y-4 px-4 py-6">
      <div v-if="missing" class="bg-card mx-auto max-w-md rounded-2xl border p-8 text-center">
        <CircleSlashIcon class="text-muted-foreground mx-auto size-8" />
        <p class="mt-3 font-semibold">{{ missing }}</p>
        <p class="text-muted-foreground mt-1 text-sm">Ask the sender for a new link.</p>
      </div>
      <div
        v-else-if="state === 'loading'"
        class="text-muted-foreground flex justify-center gap-2 py-20 text-sm"
      >
        <Loader2Icon class="size-4 animate-spin" />Opening…
      </div>

      <template v-else>
        <section
          v-if="state !== 'sign'"
          class="bg-card flex items-start gap-3 rounded-xl border p-4"
          role="status"
        >
          <CheckCircle2Icon
            v-if="['completed', 'signed'].includes(state)"
            class="mt-0.5 size-5 shrink-0 text-emerald-600"
          />
          <MailCheckIcon
            v-else-if="state === 'verify'"
            class="text-primary mt-0.5 size-5 shrink-0"
          />
          <CircleSlashIcon v-else class="text-muted-foreground mt-0.5 size-5 shrink-0" />
          <div class="min-w-0 flex-1 text-sm">
            <template v-if="state === 'completed'">
              <p class="font-semibold">Everyone has signed</p>
              <p class="text-muted-foreground">
                The signed copy below is sealed, with a certificate of completion on the last page.
              </p>
            </template>
            <template v-else-if="state === 'signed'">
              <p class="font-semibold">You’ve signed</p>
              <p class="text-muted-foreground">
                We’ll email you the finished copy once everyone has signed.
              </p>
            </template>
            <template v-else-if="state === 'waiting'">
              <p class="font-semibold">Not your turn yet</p>
              <p class="text-muted-foreground">
                {{ session?.waiting_on }} signs first. We’ll email you when it’s your turn.
              </p>
            </template>
            <template v-else-if="state === 'copy'">
              <p class="font-semibold">You’re getting a copy</p>
              <p class="text-muted-foreground">
                There’s nothing to sign. You’ll receive the finished document.
              </p>
            </template>
            <template v-else-if="state === 'declined'">
              <p class="font-semibold">This request was declined</p>
              <p class="text-muted-foreground">No one needs to sign it any more.</p>
            </template>
            <template v-else-if="state === 'voided'">
              <p class="font-semibold">The sender cancelled this request</p>
              <p v-if="envelope?.void_reason" class="text-muted-foreground">
                “{{ envelope.void_reason }}”
              </p>
            </template>
            <template v-else-if="state === 'verify'">
              <p class="font-semibold">First, confirm it’s you</p>
              <p class="text-muted-foreground">
                We’ll email a 6-digit code to
                <span class="text-foreground font-medium">{{ session?.recipient.email }}</span
                >.
              </p>
              <form v-if="codeSent" class="mt-3 flex max-w-xs gap-2" @submit.prevent="verify">
                <Label for="signing-code" class="sr-only">Code</Label>
                <Input
                  id="signing-code"
                  v-model="code"
                  inputmode="numeric"
                  autocomplete="one-time-code"
                  maxlength="6"
                  placeholder="123456"
                  class="font-mono tracking-widest"
                />
                <Button type="submit" :disabled="code.trim().length !== 6 || service.busy.value"
                  >Confirm</Button
                >
              </form>
              <div class="mt-3 flex gap-2">
                <Button v-if="!codeSent" size="sm" :disabled="service.busy.value" @click="sendCode"
                  >Email me a code</Button
                >
                <Button
                  v-else
                  variant="ghost"
                  size="sm"
                  :disabled="service.busy.value"
                  @click="sendCode"
                  >Send another code</Button
                >
              </div>
            </template>
          </div>
        </section>
        <p
          v-if="envelope?.message && state === 'sign'"
          class="bg-card rounded-xl border p-4 text-sm whitespace-pre-line"
        >
          <span class="text-muted-foreground block text-xs"
            >Message from {{ envelope.sender_name }}</span
          >{{ envelope.message }}
        </p>

        <div class="bg-card h-[78dvh] overflow-hidden rounded-xl border">
          <div v-if="!pdf" class="text-muted-foreground grid h-full place-items-center text-sm">
            <span class="flex items-center gap-2"
              ><Loader2Icon class="size-4 animate-spin" />Loading the document…</span
            >
          </div>
          <PdfViewer v-else :source="pdf" @loaded="pages = $event">
            <template #page="{ index }">
              <template v-if="state === 'sign' || state === 'verify'">
                <button
                  v-for="field in pageFields(index)"
                  :key="field.id"
                  type="button"
                  :data-field="field.id"
                  class="absolute flex items-center overflow-hidden rounded-[3px] text-left transition-colors"
                  :class="[
                    isDone(field)
                      ? 'border border-transparent hover:border-amber-400'
                      : 'animate-pulse border-2 border-amber-500 bg-amber-100/70 hover:animate-none',
                    field.kind === 'text' || field.kind === 'name'
                      ? 'cursor-text'
                      : 'cursor-pointer',
                  ]"
                  :style="{
                    left: `${field.x * 100}%`,
                    top: `${field.y * 100}%`,
                    width: `${field.w * 100}%`,
                    height: `${field.h * 100}%`,
                  }"
                  :aria-label="`${field.label || fieldMeta(field.kind).label}${field.required ? ' (required)' : ''}`"
                  :disabled="state !== 'sign'"
                  @click="openField(field)"
                >
                  <img
                    v-if="
                      (field.kind === 'signature' || field.kind === 'initials') && values[field.id]
                    "
                    :src="values[field.id]"
                    alt=""
                    class="size-full object-contain"
                  />
                  <span v-else-if="field.kind === 'date'" class="px-1 text-[11px] text-slate-900">{{
                    today
                  }}</span>
                  <input
                    v-else-if="field.kind === 'text' || field.kind === 'name'"
                    v-model="values[field.id]"
                    class="size-full bg-transparent px-1 text-[11px] text-slate-900 outline-none placeholder:text-amber-700"
                    :placeholder="field.label || fieldMeta(field.kind).label"
                    :disabled="state !== 'sign'"
                    maxlength="500"
                    @click.stop
                  />
                  <span
                    v-else-if="field.kind === 'checkbox'"
                    class="grid size-full place-items-center text-sm font-bold text-slate-900"
                  >
                    {{ values[field.id] === "true" ? "✕" : "" }}
                  </span>
                  <span
                    v-else
                    class="flex items-center gap-1 px-1 text-[11px] font-medium text-amber-800"
                  >
                    <component :is="fieldMeta(field.kind).icon" class="size-3" />{{
                      fieldMeta(field.kind).label
                    }}
                  </span>
                </button>
              </template>
            </template>
          </PdfViewer>
        </div>

        <section
          v-if="state === 'sign'"
          class="bg-card sticky bottom-4 flex flex-wrap items-center gap-3 rounded-xl border p-4 shadow-lg"
        >
          <label class="flex min-w-0 flex-1 items-start gap-2 text-sm">
            <input v-model="consent" type="checkbox" class="accent-primary mt-1 size-4" />
            <span>
              I agree to sign electronically. My electronic signature is as valid as one on paper.
            </span>
          </label>
          <Button :disabled="!consent || submitting" class="gap-1.5" @click="finish">
            <Loader2Icon v-if="submitting" class="size-4 animate-spin" />
            <ShieldCheckIcon v-else class="size-4" />
            {{ remaining.length ? `Finish (${remaining.length} left)` : "Finish signing" }}
          </Button>
        </section>
      </template>
    </main>

    <SignaturePad
      v-model:open="padOpen"
      :aspect="padAspect"
      :kind="padField?.kind === 'initials' ? 'initials' : 'signature'"
      :name="session?.recipient.name"
      @adopt="adopt"
    />

    <Dialog v-model:open="declineOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Decline to sign?</DialogTitle>
          <DialogDescription>
            {{ envelope?.sender_name }} will be told. The request ends for everyone.
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-1.5">
          <Label for="decline-reason"
            >Reason <span class="text-muted-foreground">(optional)</span></Label
          >
          <Textarea
            id="decline-reason"
            v-model="declineReason"
            rows="3"
            placeholder="The amount in clause 4 is wrong."
          />
        </div>
        <DialogFooter>
          <Button variant="outline" @click="declineOpen = false">Keep reviewing</Button>
          <Button variant="destructive" :disabled="service.busy.value" @click="decline"
            >Decline</Button
          >
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
