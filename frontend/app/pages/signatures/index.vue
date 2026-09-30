<script setup lang="ts">
  import { InboxIcon, PenLineIcon, SendIcon, SignatureIcon } from "@lucide/vue";
  import { onMounted, ref, shallowRef, watch } from "vue";

  import EnvelopeCard from "@/components/signing/EnvelopeCard.vue";
  import { Button } from "@/components/ui/button";
  import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
  import type { SignatureEnvelope } from "@/types/api";
  import { formatDate } from "@/utils/format";

  definePageMeta({
    title: "Signatures",
    description: "Documents waiting for your signature, and requests you sent.",
    breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Signatures" }],
  });
  useHead({ title: "Signatures — Lumen DMS" });

  const route = useRoute();
  const router = useRouter();
  const service = signaturesService();
  const box = shallowRef<"inbox" | "sent">(route.query.box === "sent" ? "sent" : "inbox");
  const items = ref<SignatureEnvelope[]>([]);
  const loading = shallowRef(true);

  const load = async () => {
    loading.value = true;
    items.value = await service.list(box.value);
    loading.value = false;
  };
  onMounted(load);
  watch(box, (value) => {
    void router.replace({ query: value === "sent" ? { box: "sent" } : {} });
    void load();
  });

  // Signing links are absolute; inside the app they open as a route.
  const signPath = (link?: string) => (link ? new URL(link).pathname : "");
</script>

<template>
  <div class="mx-auto max-w-3xl space-y-4">
    <Tabs v-model="box">
      <TabsList>
        <TabsTrigger value="inbox" class="gap-1.5"
          ><InboxIcon class="size-4" />Waiting for me</TabsTrigger
        >
        <TabsTrigger value="sent" class="gap-1.5"><SendIcon class="size-4" />Sent</TabsTrigger>
      </TabsList>
    </Tabs>

    <div v-if="loading" class="space-y-2">
      <div v-for="i in 3" :key="i" class="bg-muted h-16 animate-pulse rounded-lg" />
    </div>

    <div v-else-if="!items.length" class="bg-card rounded-2xl border p-10 text-center">
      <SignatureIcon class="text-muted-foreground mx-auto size-8" />
      <p class="mt-3 font-semibold">
        {{ box === "inbox" ? "Nothing to sign" : "You haven’t sent anything for signature" }}
      </p>
      <p class="text-muted-foreground mx-auto mt-1 max-w-sm text-sm">
        {{
          box === "inbox"
            ? "When someone asks you to sign, it shows up here and in your email."
            : "Open a PDF or Office document and choose Request signatures."
        }}
      </p>
      <Button v-if="box === 'sent'" variant="outline" size="sm" class="mt-4" as-child>
        <NuxtLink to="/documents">Go to Drive</NuxtLink>
      </Button>
    </div>

    <ul v-else-if="box === 'inbox'" class="space-y-2">
      <li
        v-for="item in items"
        :key="item.id"
        class="bg-card flex items-center gap-3 rounded-lg border p-3"
      >
        <div class="bg-primary/10 text-primary grid size-9 shrink-0 place-items-center rounded-lg">
          <PenLineIcon class="size-4" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium">{{ item.title }}</p>
          <p class="text-muted-foreground truncate text-xs">
            From {{ item.sender_name }} · {{ formatDate(item.created_at) }}
          </p>
        </div>
        <Button size="sm" as-child>
          <NuxtLink :to="signPath(item.signing_link)">Review and sign</NuxtLink>
        </Button>
      </li>
    </ul>

    <div v-else class="bg-card space-y-2 rounded-xl border p-3">
      <EnvelopeCard
        v-for="item in items"
        :key="item.id"
        :envelope="item"
        show-document
        @changed="load"
      />
    </div>
  </div>
</template>
