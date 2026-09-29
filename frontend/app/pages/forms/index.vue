<script setup lang="ts">
  import { CopyIcon, PlusIcon } from "@lucide/vue";
  import { computed, onMounted, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import FormBuilderDialog from "@/components/forms/FormBuilderDialog.vue";
  import FormReviewInbox from "@/components/forms/FormReviewInbox.vue";
  import FormTemplateList from "@/components/forms/FormTemplateList.vue";
  import { Button } from "@/components/ui/button";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
  } from "@/components/ui/dialog";
  import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
  import VisitorInviteDialog from "@/components/visitors/VisitorInviteDialog.vue";
  import VisitorRequestManager from "@/components/visitors/VisitorRequestManager.vue";
  import { foldersService } from "@/services/folders";
  import { formsService } from "@/services/forms";
  import { visitorsService } from "@/services/visitors";
  import type {
    FormSubmissionDetail,
    FormTemplate,
    VisitorRequest,
    VisitorRequestCreated,
  } from "@/types/api";

  definePageMeta({
    title: "Forms",
    description: "Department forms, submissions, and approvals.",
    breadcrumbs: [{ label: "Collaboration", to: "/" }, { label: "Forms" }],
  });
  const forms = formsService();
  const visitors = visitorsService();
  const { templates, pending, loading, reviewBusy, error } = forms;
  const { departments, users, fetchDepartments, fetchUsers } = foldersService();
  const builderOpen = shallowRef(false);
  const inviteOpen = shallowRef(false);
  const selectedTemplate = shallowRef<FormTemplate | null>(null);
  const selectedSubmission = shallowRef<FormSubmissionDetail | null>(null);
  const visitorRequests = shallowRef<VisitorRequest[]>([]);
  const visitorLoading = shallowRef(false);
  const visitorBusyId = shallowRef("");
  const renewedAccess = shallowRef<VisitorRequestCreated | null>(null);
  const departmentNames = computed(
    () => new Map(departments.value.map((department) => [department.id, department.name])),
  );
  const loadVisitorRequests = async () => {
    visitorLoading.value = true;
    visitorRequests.value = await visitors.requests();
    visitorLoading.value = false;
  };
  onMounted(() => {
    void Promise.all([
      forms.loadTemplates(),
      forms.loadPending(),
      loadVisitorRequests(),
      fetchDepartments(),
      fetchUsers(),
    ]);
  });
  const saveTemplate = async (payload: Parameters<(typeof forms)["createTemplate"]>[0]) => {
    const template = await forms.createTemplate(payload);
    if (template) {
      toast.success("Form draft created");
      builderOpen.value = false;
    } else {
      toast.error(error.value || "Could not create form");
    }
  };
  const selectTemplate = (template: FormTemplate) => {
    if (template.status !== "published") {
      toast.info("Publish this form before inviting a visitor");
      return;
    }
    selectedTemplate.value = template;
    inviteOpen.value = true;
  };
  const changeStatus = async (template: FormTemplate, status: "published" | "archived") => {
    const updated = await forms.setTemplateStatus(template.id, status);
    if (updated) toast.success(status === "published" ? "Form published" : "Form archived");
    else toast.error(error.value || "Could not update form");
  };
  const selectSubmission = async (id: string) => {
    selectedSubmission.value = await forms.submission(id);
  };
  const decideSubmission = async (payload: {
    id: string;
    action: "approve" | "reject" | "return";
    comment: string;
  }) => {
    const ok = await forms.decide(payload.id, payload.action, payload.comment);
    if (!ok) {
      toast.error(error.value || "Could not record decision");
      return;
    }
    selectedSubmission.value = null;
    toast.success(payload.action === "approve" ? "Submission approved" : "Decision recorded");
  };
  const addSubmissionComment = async (payload: { id: string; body: string }) => {
    const comment = await forms.addComment(payload.id, payload.body);
    if (!comment || !selectedSubmission.value) {
      toast.error(error.value || "Could not add note");
      return;
    }
    selectedSubmission.value = {
      ...selectedSubmission.value,
      comments: [...selectedSubmission.value.comments, comment],
    };
    toast.success("Note added");
  };
  const resendVisitor = async (request: VisitorRequest) => {
    visitorBusyId.value = request.id;
    renewedAccess.value = await visitors.resend(request.id);
    visitorBusyId.value = "";
    if (!renewedAccess.value) toast.error(visitors.error.value || "Could not renew access");
    else await loadVisitorRequests();
  };
  const revokeVisitor = async (request: VisitorRequest) => {
    visitorBusyId.value = request.id;
    const ok = await visitors.revoke(request.id);
    visitorBusyId.value = "";
    if (!ok) toast.error(visitors.error.value || "Could not revoke access");
    else {
      toast.success("Visitor access revoked");
      await loadVisitorRequests();
    }
  };
  const copyAccessCode = async () => {
    if (!renewedAccess.value) return;
    await navigator.clipboard.writeText(renewedAccess.value.access_code);
    toast.success("Access code copied");
  };
</script>

<template>
  <Teleport to="#page-actions-target"
    ><Button size="sm" @click="builderOpen = true"
      ><PlusIcon class="mr-1.5 h-4 w-4" />New form</Button
    ></Teleport
  >
  <div class="space-y-5">
    <div>
      <h1 class="text-xl font-semibold">Department forms</h1>
      <p class="text-muted-foreground mt-1 text-sm">
        Create structured requests, collect supporting documents, and route approvals.
      </p>
    </div>
    <Tabs default-value="forms" class="space-y-4">
      <TabsList>
        <TabsTrigger value="forms">Forms</TabsTrigger>
        <TabsTrigger value="inbox"
          >Approval inbox
          <span v-if="pending.length" class="ml-1">({{ pending.length }})</span></TabsTrigger
        >
        <TabsTrigger value="visitors">Visitor access</TabsTrigger>
      </TabsList>
      <TabsContent value="forms">
        <div v-if="loading" class="text-muted-foreground text-sm">Loading forms…</div>
        <FormTemplateList
          v-else
          :templates="templates"
          :departments="departmentNames"
          @select="selectTemplate"
          @publish="changeStatus($event, 'published')"
          @archive="changeStatus($event, 'archived')"
        />
      </TabsContent>
      <TabsContent value="inbox">
        <FormReviewInbox
          :submissions="pending"
          :selected="selectedSubmission"
          :loading="loading"
          :busy="reviewBusy"
          :error="error"
          @refresh="forms.loadPending"
          @select="selectSubmission"
          @decide="decideSubmission"
          @comment="addSubmissionComment"
        />
      </TabsContent>
      <TabsContent value="visitors">
        <VisitorRequestManager
          :requests="visitorRequests"
          :loading="visitorLoading"
          :busy-id="visitorBusyId"
          :error="visitors.error.value"
          @refresh="loadVisitorRequests"
          @resend="resendVisitor"
          @revoke="revokeVisitor"
        />
      </TabsContent>
    </Tabs>
  </div>
  <FormBuilderDialog
    v-model:open="builderOpen"
    :departments="departments"
    :users="users"
    :saving="loading"
    @save="saveTemplate"
  />
  <VisitorInviteDialog v-model="inviteOpen" :template="selectedTemplate" />
  <Dialog :open="Boolean(renewedAccess)" @update:open="(value) => !value && (renewedAccess = null)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>New visitor code</DialogTitle>
        <DialogDescription
          >The previous code has been replaced. Send this code through a trusted
          channel.</DialogDescription
        >
      </DialogHeader>
      <div class="bg-muted flex items-center justify-between gap-3 rounded-lg p-4">
        <strong class="font-mono text-xl tracking-[0.18em]">{{
          renewedAccess?.access_code
        }}</strong>
        <Button variant="ghost" size="icon" aria-label="Copy access code" @click="copyAccessCode"
          ><CopyIcon class="size-4"
        /></Button>
      </div>
      <DialogFooter><Button @click="renewedAccess = null">Done</Button></DialogFooter>
    </DialogContent>
  </Dialog>
</template>
