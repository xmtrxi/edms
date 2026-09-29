<script setup lang="ts">
  import { CheckIcon, InboxIcon, XIcon } from "@lucide/vue";
  import { onMounted, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import type { AccessRequest, Document } from "@/types/api";
  import { formatDateTime, initialsOf } from "@/utils/format";

  definePageMeta({
    title: "Shared with me",
    description: "Documents others have shared with you, and requests for yours.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Shared" }],
  });

  useHead({ title: "Shared with me — Lumen DMS" });
  const workspace = workspaceService();
  const sharing = sharingService();
  const documents = shallowRef<Document[]>([]);
  const requests = shallowRef<AccessRequest[]>([]);
  const deciding = shallowRef("");

  const roleLabel: Record<string, string> = {
    viewer: "view",
    commenter: "comment on",
    editor: "edit",
  };

  const refresh = async () => {
    const [shared, pending] = await Promise.all([workspace.shared(), sharing.accessRequests()]);
    documents.value = shared;
    requests.value = pending;
  };

  const decide = async (request: AccessRequest, approve: boolean) => {
    deciding.value = request.id;
    const result = await sharing.decide(request.id, approve);
    deciding.value = "";
    if (!result) {
      toast.error(sharing.error.value || "Could not update the request");
      return;
    }
    requests.value = requests.value.filter((item) => item.id !== request.id);
    toast.success(
      approve
        ? `${request.requester_name || request.requester_email} can now ${roleLabel[request.role] ?? "view"} ${request.document_name}`
        : "Request declined",
    );
  };

  onMounted(refresh);
</script>

<template>
  <div class="space-y-6">
    <section
      v-if="requests.length"
      id="requests"
      class="bg-card rounded-2xl border shadow-sm"
      aria-labelledby="requests-title"
    >
      <header class="flex items-center gap-2 border-b px-4 py-3">
        <InboxIcon class="text-primary size-4" />
        <h2 id="requests-title" class="text-sm font-semibold">Access requests</h2>
        <Badge variant="secondary" class="h-5 px-1.5">{{ requests.length }}</Badge>
      </header>
      <ul class="divide-y">
        <li
          v-for="request in requests"
          :key="request.id"
          class="flex flex-wrap items-center gap-3 px-4 py-3"
        >
          <Avatar class="size-9">
            <AvatarFallback class="text-xs">{{
              initialsOf(request.requester_name || request.requester_email)
            }}</AvatarFallback>
          </Avatar>
          <div class="min-w-0 flex-1">
            <p class="text-sm">
              <span class="font-medium">{{
                request.requester_name || request.requester_email
              }}</span>
              wants to {{ roleLabel[request.role] ?? "view" }}
              <NuxtLink
                :to="`/documents/${request.document_id}`"
                class="font-medium underline-offset-2 hover:underline"
                >{{ request.document_name }}</NuxtLink
              >
            </p>
            <p class="text-muted-foreground truncate text-xs">
              {{ formatDateTime(request.created_at) }}
              <template v-if="request.message"> · “{{ request.message }}”</template>
            </p>
          </div>
          <div class="flex gap-2">
            <Button
              size="sm"
              variant="outline"
              :disabled="deciding === request.id"
              @click="decide(request, false)"
              ><XIcon class="size-3.5" />Decline</Button
            >
            <Button size="sm" :disabled="deciding === request.id" @click="decide(request, true)"
              ><CheckIcon class="size-3.5" />Approve</Button
            >
          </div>
        </li>
      </ul>
    </section>

    <DocumentsLiveDocumentCollection
      :documents="documents"
      :loading="workspace.loading.value"
      :error="workspace.error.value"
      empty-title="Nothing has been shared with you"
      @refresh="refresh"
    />
  </div>
</template>
