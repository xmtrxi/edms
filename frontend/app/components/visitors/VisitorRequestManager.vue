<script setup lang="ts">
  import { BanIcon, KeyRoundIcon, RotateCwIcon, UserRoundCheckIcon } from "@lucide/vue";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import type { VisitorRequest } from "@/types/api";
  import { formatDateTime } from "@/utils/format";

  defineProps<{ requests: VisitorRequest[]; loading?: boolean; busyId?: string; error?: string }>();
  const emit = defineEmits<{
    refresh: [];
    resend: [request: VisitorRequest];
    revoke: [request: VisitorRequest];
  }>();
</script>

<template>
  <section class="border-border bg-card overflow-hidden rounded-xl border">
    <header
      class="border-border flex flex-wrap items-center justify-between gap-3 border-b px-5 py-4"
    >
      <div>
        <h2 class="text-sm font-semibold">Visitor access</h2>
        <p class="text-muted-foreground mt-0.5 text-xs">
          Temporary form links protected by an access code and email OTP.
        </p>
      </div>
      <Button variant="outline" size="sm" :disabled="loading" @click="emit('refresh')"
        ><RotateCwIcon class="mr-2 size-4" />Refresh</Button
      >
    </header>
    <p v-if="error" role="alert" class="text-destructive border-border border-b px-5 py-3 text-sm">
      {{ error }}
    </p>
    <div v-if="loading && !requests.length" class="text-muted-foreground p-8 text-center text-sm">
      Loading visitor access…
    </div>
    <div v-else-if="!requests.length" class="p-10 text-center">
      <UserRoundCheckIcon class="text-muted-foreground/50 mx-auto size-8" />
      <p class="mt-3 text-sm font-medium">No visitor access created</p>
      <p class="text-muted-foreground mt-1 text-xs">
        Open Forms and choose a published form to invite someone.
      </p>
    </div>
    <div v-else class="divide-border divide-y">
      <article
        v-for="request in requests"
        :key="request.id"
        class="flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-center"
      >
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <p class="truncate text-sm font-medium">{{ request.visitor_name }}</p>
            <Badge variant="outline">{{ request.status.replaceAll("_", " ") }}</Badge>
          </div>
          <p class="text-muted-foreground mt-1 truncate text-xs">{{ request.visitor_email }}</p>
          <p class="text-muted-foreground mt-1 text-[11px]">
            Expires {{ formatDateTime(request.expires_at) }}
          </p>
        </div>
        <div v-if="request.status === 'awaiting_visitor'" class="flex shrink-0 gap-2">
          <Button
            variant="outline"
            size="sm"
            :disabled="busyId === request.id"
            @click="emit('resend', request)"
            ><KeyRoundIcon class="mr-2 size-4" />New code</Button
          >
          <Button
            variant="ghost"
            size="sm"
            class="text-destructive"
            :disabled="busyId === request.id"
            @click="emit('revoke', request)"
            ><BanIcon class="mr-2 size-4" />Revoke</Button
          >
        </div>
      </article>
    </div>
  </section>
</template>
