<script setup lang="ts">
  import { Building2Icon, PlusIcon, UsersIcon } from "@lucide/vue";
  import { onMounted, ref } from "vue";
  import { toast } from "vue-sonner";

  definePageMeta({
    title: "Departments",
    description: "Office teams and their record responsibilities.",
    breadcrumbs: [{ label: "Administration", to: "/" }, { label: "Departments" }],
  });
  const admin = administrationService();
  const open = ref(false);
  const name = ref("");
  const description = ref("");
  const create = async () => {
    if (!name.value.trim()) return;
    const item = await admin.createDepartment({ name: name.value, description: description.value });
    if (item) {
      toast.success("Department created");
      open.value = false;
      name.value = "";
      description.value = "";
    }
  };
  onMounted(admin.load);
</script>
<template>
  <Teleport to="#page-actions-target"
    ><Button size="sm" class="gap-2" @click="open = true"
      ><PlusIcon class="size-4" />New department</Button
    ></Teleport
  >
  <div
    v-if="admin.error.value"
    class="border-destructive/25 bg-destructive/5 text-destructive mb-4 rounded-xl border p-3 text-sm"
  >
    {{ admin.error.value }}
  </div>
  <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
    <article
      v-for="item in admin.departments.value"
      :key="item.id"
      class="bg-card rounded-2xl border p-5 shadow-sm"
    >
      <div class="flex items-center gap-3">
        <div class="bg-primary/10 text-primary grid size-11 place-items-center rounded-xl">
          <Building2Icon class="size-5" />
        </div>
        <div>
          <h2 class="font-semibold">{{ item.name }}</h2>
          <p class="text-muted-foreground text-xs">Office department</p>
        </div>
      </div>
      <p class="text-muted-foreground mt-4 min-h-10 text-sm">
        {{ item.description || "No description added yet." }}
      </p>
      <div class="text-muted-foreground mt-4 flex items-center gap-2 border-t pt-3 text-xs">
        <UsersIcon class="size-3.5" />Manage members from Access control
      </div>
    </article>
  </div>
  <Dialog v-model:open="open"
    ><DialogContent
      ><DialogHeader
        ><DialogTitle>Create department</DialogTitle
        ><DialogDescription
          >Group staff and assign form approval routes.</DialogDescription
        ></DialogHeader
      >
      <div class="space-y-3">
        <Input v-model="name" placeholder="Department name" /><Textarea
          v-model="description"
          placeholder="What records does this office own?"
        />
      </div>
      <DialogFooter
        ><Button variant="outline" @click="open = false">Cancel</Button
        ><Button :disabled="!name.trim()" @click="create">Create</Button></DialogFooter
      ></DialogContent
    ></Dialog
  >
</template>
