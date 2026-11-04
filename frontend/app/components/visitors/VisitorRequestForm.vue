<script setup lang="ts">
  import { ArrowRightIcon, CheckCircle2Icon, ClipboardListIcon, PaperclipIcon } from "@lucide/vue";
  import { computed, reactive, ref } from "vue";

  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import { Textarea } from "@/components/ui/textarea";
  import type { FormField, VisitorPortal } from "@/types/api";

  const props = defineProps<{
    portal: VisitorPortal;
    loading: boolean;
    error: string;
    attachments: Array<{ id: string; name: string; size: number }>;
  }>();
  const emit = defineEmits<{ submit: [values: Record<string, unknown>]; upload: [file: File] }>();
  const values = reactive<Record<string, unknown>>({});
  const isSubmitted = computed(() => Boolean(props.portal.submission));
  const fileInput = ref<HTMLInputElement | null>(null);
  const chooseFile = () => fileInput.value?.click();
  const fileSelected = (event: Event) => {
    const file = (event.target as HTMLInputElement).files?.[0];
    if (file) emit("upload", file);
  };

  const inputType = (field: FormField) => {
    if (field.type === "number" || field.type === "currency") return "number";
    if (field.type === "date") return "date";
    return "text";
  };
</script>

<template>
  <section
    class="bg-card rounded-2xl p-6 shadow-[0_20px_70px_-42px_oklch(0.18_0.03_250/0.5)] sm:p-9"
  >
    <div class="flex items-start gap-4">
      <span
        class="bg-primary text-primary-foreground grid size-10 shrink-0 place-items-center rounded-lg"
        ><ClipboardListIcon class="size-5"
      /></span>
      <div>
        <h1 class="text-2xl font-semibold tracking-[-0.025em]">{{ portal.template.name }}</h1>
        <p class="text-muted-foreground mt-1 max-w-[65ch] text-sm leading-6">
          {{ portal.template.description || "Complete the information requested by the office." }}
        </p>
      </div>
    </div>

    <div v-if="isSubmitted" class="mt-9 flex items-start gap-4 border-t pt-8">
      <CheckCircle2Icon class="text-success mt-0.5 size-6 shrink-0" />
      <div>
        <h2 class="font-semibold">Your information is with the office</h2>
        <p class="text-muted-foreground mt-1 text-sm leading-6">
          No further action is required now. The route on this page updates as staff review the
          request.
        </p>
      </div>
    </div>

    <form
      v-else
      class="mt-9 space-y-7 border-t pt-8"
      @submit.prevent="emit('submit', { ...values })"
    >
      <div v-for="field in portal.template.definition.fields" :key="field.id" class="space-y-2">
        <Label :for="field.id"
          >{{ field.label }}
          <span v-if="field.required" class="text-destructive" aria-hidden="true">*</span></Label
        >
        <Textarea
          v-if="field.type === 'textarea'"
          :id="field.id"
          v-model="values[field.id] as string"
          :required="field.required"
        />
        <select
          v-else-if="field.type === 'select'"
          :id="field.id"
          v-model="values[field.id]"
          :required="field.required"
          class="border-input bg-background focus-visible:ring-ring h-10 w-full rounded-md border px-3 text-sm focus-visible:ring-2 focus-visible:outline-none"
        >
          <option value="" disabled>Select an option</option>
          <option v-for="option in field.options" :key="option" :value="option">
            {{ option }}
          </option>
        </select>
        <label
          v-else-if="field.type === 'checkbox'"
          class="border-input flex min-h-11 items-center gap-3 rounded-md border px-3 text-sm"
        >
          <input
            v-model="values[field.id] as boolean"
            type="checkbox"
            class="accent-primary size-4"
          />
          <span>Yes</span>
        </label>
        <div
          v-else-if="field.type === 'file'"
          class="bg-muted/25 rounded-xl border border-dashed p-4"
        >
          <input ref="fileInput" type="file" class="sr-only" @change="fileSelected" /><Button
            type="button"
            variant="outline"
            size="sm"
            class="gap-2"
            :disabled="loading"
            @click="chooseFile"
            ><PaperclipIcon class="size-4" />Attach supporting file</Button
          >
          <ul v-if="attachments.length" class="text-muted-foreground mt-3 space-y-1 text-xs">
            <li v-for="item in attachments" :key="item.id">
              {{ item.name }} · {{ Math.ceil(item.size / 1024) }} KB
            </li>
          </ul>
          <p class="text-muted-foreground mt-2 text-xs">PDF, image, or office file up to 20 MB.</p>
        </div>
        <Input
          v-else
          :id="field.id"
          v-model="values[field.id] as string"
          :type="inputType(field)"
          :required="field.required"
        />
      </div>
      <p v-if="error" role="alert" class="text-destructive text-sm">{{ error }}</p>
      <Button
        type="submit"
        size="lg"
        class="w-full justify-between sm:w-auto sm:min-w-56"
        :disabled="loading"
      >
        {{ loading ? "Sending…" : "Send to the office" }}<ArrowRightIcon class="size-4" />
      </Button>
    </form>
  </section>
</template>
