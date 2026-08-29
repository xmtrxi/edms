<script setup lang="ts">
  import { PlusIcon } from "@lucide/vue";
  import { computed, onMounted, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import FormBuilderDialog from "@/components/forms/FormBuilderDialog.vue";
  import FormTemplateList from "@/components/forms/FormTemplateList.vue";
  import { Button } from "@/components/ui/button";
  import { foldersService } from "@/services/folders";
  import { formsService } from "@/services/forms";

  definePageMeta({
    title: "Forms",
    description: "Department forms, submissions, and approvals.",
    breadcrumbs: [{ label: "Collaboration", to: "/" }, { label: "Forms" }],
  });
  const { templates, loading, error, loadTemplates, createTemplate } = formsService();
  const { departments, users, fetchDepartments, fetchUsers } = foldersService();
  const builderOpen = shallowRef(false);
  const departmentNames = computed(
    () => new Map(departments.value.map((department) => [department.id, department.name])),
  );
  onMounted(() => {
    void Promise.all([loadTemplates(), fetchDepartments(), fetchUsers()]);
  });
  const saveTemplate = async (payload: Parameters<typeof createTemplate>[0]) => {
    const template = await createTemplate(payload);
    if (template) {
      toast.success("Form draft created");
      builderOpen.value = false;
    } else {
      toast.error(error.value || "Could not create form");
    }
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
    <div v-if="loading" class="text-muted-foreground text-sm">Loading forms…</div>
    <FormTemplateList
      v-else
      :templates="templates"
      :departments="departmentNames"
      @select="toast.info(`Open ${$event.name}`)"
    />
  </div>
  <FormBuilderDialog
    v-model:open="builderOpen"
    :departments="departments"
    :users="users"
    :saving="loading"
    @save="saveTemplate"
  />
</template>
