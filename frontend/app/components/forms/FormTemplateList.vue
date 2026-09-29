<script setup lang="ts">
  import { ClipboardListIcon, FileTextIcon } from "@lucide/vue";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Card, CardContent } from "@/components/ui/card";
  import type { FormTemplate } from "@/types/api";

  const props = defineProps<{ templates: FormTemplate[]; departments: Map<string, string> }>();
  const emit = defineEmits<{
    select: [template: FormTemplate];
    publish: [template: FormTemplate];
    archive: [template: FormTemplate];
  }>();
  const departmentName = (id: string) => props.departments.get(id) ?? "Department";
</script>

<template>
  <div v-if="templates.length" class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
    <Card
      v-for="template in templates"
      :key="template.id"
      class="border-border hover:shadow-elegant transition-shadow"
    >
      <CardContent class="p-5">
        <div class="flex items-start justify-between gap-3">
          <div class="bg-primary/10 text-primary grid h-9 w-9 place-items-center rounded-lg">
            <ClipboardListIcon class="h-4 w-4" />
          </div>
          <Badge
            :variant="template.status === 'published' ? 'default' : 'secondary'"
            class="capitalize"
            >{{ template.status }}</Badge
          >
        </div>
        <h2 class="mt-4 text-base font-semibold">{{ template.name }}</h2>
        <p class="text-muted-foreground mt-1 line-clamp-2 text-sm">
          {{ template.description || "No description provided." }}
        </p>
        <div class="text-muted-foreground mt-4 flex items-center gap-2 text-xs">
          <FileTextIcon class="h-3.5 w-3.5" />{{ template.definition.fields.length }} fields ·
          {{ departmentName(template.department_id) }}
        </div>
        <div class="mt-4 flex gap-2">
          <Button
            v-if="template.status === 'draft'"
            class="flex-1"
            size="sm"
            @click="emit('publish', template)"
            >Publish form</Button
          ><Button
            v-else
            class="flex-1"
            variant="secondary"
            size="sm"
            @click="emit('select', template)"
            >Invite visitor</Button
          ><Button
            v-if="template.status === 'published'"
            variant="ghost"
            size="sm"
            @click="emit('archive', template)"
            >Archive</Button
          >
        </div>
      </CardContent>
    </Card>
  </div>
  <div
    v-else
    class="border-border text-muted-foreground rounded-lg border border-dashed px-6 py-12 text-center text-sm"
  >
    No forms have been published yet.
  </div>
</template>
