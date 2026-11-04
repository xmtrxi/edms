<script setup lang="ts">
  import { PlusIcon, Trash2Icon } from "@lucide/vue";
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
  import { Textarea } from "@/components/ui/textarea";
  import type { ApprovalStepDefinition, Department, FormField, User } from "@/types/api";

  const open = defineModel<boolean>({ required: true });
  const props = defineProps<{ departments: Department[]; users: User[]; saving: boolean }>();
  const emit = defineEmits<{
    save: [
      payload: {
        department_id: string;
        name: string;
        description: string;
        status: "draft" | "published";
        definition: {
          allowed_department_ids: string[];
          fields: FormField[];
          approval_steps: ApprovalStepDefinition[];
        };
      },
    ];
  }>();
  const form = reactive({
    departmentId: "",
    name: "",
    description: "",
    status: "draft" as "draft" | "published",
    allowed: [] as string[],
    fields: [] as FormField[],
    approvalSteps: [] as Array<ApprovalStepDefinition & { userText: string }>,
  });
  const canSave = computed(() =>
    Boolean(form.departmentId && form.name.trim() && form.allowed.length),
  );
  watch(open, (value) => {
    if (!value) return;
    form.departmentId = props.departments[0]?.id ?? "";
    form.allowed = form.departmentId ? [form.departmentId] : [];
    form.name = "";
    form.description = "";
    form.status = "draft";
    form.fields = [];
    form.approvalSteps = [];
  });
  const addField = () =>
    form.fields.push({
      id: `field_${Date.now()}`,
      label: "New field",
      type: "text",
      required: false,
    });
  const addApproval = () =>
    form.approvalSteps.push({
      name: `Approval ${form.approvalSteps.length + 1}`,
      completion_rule: "any_one",
      userText: "",
    });
  const save = () =>
    emit("save", {
      department_id: form.departmentId,
      name: form.name,
      description: form.description,
      status: form.status,
      definition: {
        allowed_department_ids: form.allowed,
        fields: form.fields,
        approval_steps: form.approvalSteps.map(({ userText, ...step }) => ({
          ...step,
          user_ids: userText.split(/[\s,]+/).filter(Boolean),
        })),
      },
    });
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
      <DialogHeader
        ><DialogTitle>Create form</DialogTitle
        ><DialogDescription
          >Build a draft for a department, then publish it when it is ready.</DialogDescription
        ></DialogHeader
      >
      <div class="grid gap-4 py-2">
        <div class="grid gap-2">
          <Label for="form-name">Name</Label
          ><Input id="form-name" v-model="form.name" placeholder="e.g. Annual leave request" />
        </div>
        <div class="grid gap-2">
          <Label for="form-description">Description</Label
          ><Textarea
            id="form-description"
            v-model="form.description"
            placeholder="Explain when this form should be used."
          />
        </div>
        <div class="grid gap-2">
          <Label>Owning department</Label
          ><select
            v-model="form.departmentId"
            class="border-input bg-background h-9 rounded-md border px-3 text-sm"
          >
            <option v-for="department in departments" :key="department.id" :value="department.id">
              {{ department.name }}
            </option>
          </select>
        </div>
        <div class="grid gap-2">
          <Label>Fields</Label>
          <div
            v-for="(field, index) in form.fields"
            :key="field.id"
            class="border-border grid grid-cols-[1fr_9rem_auto] gap-2 rounded-lg border p-3"
          >
            <Input v-model="field.label" aria-label="Field label" /><select
              v-model="field.type"
              class="border-input bg-background rounded-md border px-2 text-sm"
            >
              <option value="text">Text</option>
              <option value="textarea">Long text</option>
              <option value="number">Number</option>
              <option value="currency">Currency</option>
              <option value="date">Date</option>
              <option value="select">Select</option>
              <option value="checkbox">Checkbox</option>
              <option value="file">File</option></select
            ><Button
              size="icon"
              variant="ghost"
              :aria-label="`Remove field ${index + 1}`"
              @click="form.fields.splice(index, 1)"
              ><Trash2Icon class="h-4 w-4"
            /></Button>
          </div>
          <Button variant="outline" size="sm" class="w-fit" @click="addField"
            ><PlusIcon class="mr-1.5 h-4 w-4" />Add field</Button
          >
        </div>
        <div class="grid gap-2">
          <Label>Approval steps</Label>
          <div
            v-for="(step, index) in form.approvalSteps"
            :key="index"
            class="border-border grid gap-2 rounded-lg border p-3"
          >
            <Input v-model="step.name" aria-label="Approval step name" /><select
              v-model="step.completion_rule"
              class="border-input bg-background h-9 rounded-md border px-2 text-sm"
            >
              <option value="any_one">Any assigned person approves</option>
              <option value="everyone">Everyone approves</option></select
            ><Input v-model="step.userText" placeholder="Approver user IDs, comma-separated" />
            <p class="text-muted-foreground text-xs">
              {{ users.length }} active users are available for assignment.
            </p>
          </div>
          <Button variant="outline" size="sm" class="w-fit" @click="addApproval"
            ><PlusIcon class="mr-1.5 h-4 w-4" />Add approval step</Button
          >
        </div>
      </div>
      <DialogFooter
        ><Button variant="outline" @click="open = false">Cancel</Button
        ><Button :disabled="!canSave || saving" @click="save">{{
          saving ? "Creating…" : "Create draft"
        }}</Button></DialogFooter
      >
    </DialogContent>
  </Dialog>
</template>
