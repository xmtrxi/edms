<script setup lang="ts">
  import { CheckIcon, CopyIcon, HardDriveIcon, KeyRoundIcon, Trash2Icon } from "@lucide/vue";
  import { computed, onMounted, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Checkbox } from "@/components/ui/checkbox";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import type { ApiResponse, ApiToken } from "@/types/api";
  import { apiErrorMessage } from "@/utils/api";
  import { formatDate } from "@/utils/format";

  const auth = useAuthStore();
  const tokens = shallowRef<ApiToken[]>([]);
  const name = shallowRef("");
  const scopes = shallowRef<string[]>(["webdav", "read", "write"]);
  const secret = shallowRef("");
  const copied = shallowRef("");
  const busy = shallowRef(false);

  const scopeOptions = [
    { value: "read", label: "Read files" },
    { value: "write", label: "Change files" },
    { value: "webdav", label: "WebDAV drive" },
    { value: "mcp", label: "AI agents (MCP)" },
  ];

  // WebDAV lives next to the API, outside /api.
  const webdavUrl = computed(() => {
    const api = useRuntimeConfig().public.apiBase as string;
    try {
      return `${new URL(api, window.location.origin).origin}/webdav/`;
    } catch {
      return "/webdav/";
    }
  });

  const load = async () => {
    const { $api } = useNuxtApp();
    try {
      tokens.value = (await $api<ApiResponse<ApiToken[]>>("/tokens")).data;
    } catch (err) {
      toast.error(apiErrorMessage(err));
    }
  };
  onMounted(load);

  const toggleScope = (scope: string, on: boolean) => {
    scopes.value = on
      ? [...new Set([...scopes.value, scope])]
      : scopes.value.filter((item) => item !== scope);
  };

  const create = async () => {
    const { $api } = useNuxtApp();
    busy.value = true;
    try {
      const res = await $api<ApiResponse<ApiToken>>("/tokens", {
        method: "POST",
        body: { name: name.value.trim(), scopes: scopes.value },
      });
      secret.value = res.data.secret ?? "";
      name.value = "";
      await load();
    } catch (err) {
      toast.error(apiErrorMessage(err));
    } finally {
      busy.value = false;
    }
  };

  const revoke = async (token: ApiToken) => {
    const { $api } = useNuxtApp();
    try {
      await $api(`/tokens/${encodeURIComponent(token.id)}`, { method: "DELETE" });
      tokens.value = tokens.value.filter((item) => item.id !== token.id);
      toast.success(`${token.name} revoked`);
    } catch (err) {
      toast.error(apiErrorMessage(err));
    }
  };

  const copy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      copied.value = value;
    } catch {
      toast.error("Copy failed. Select the text and copy it manually.");
    }
  };
</script>

<template>
  <section class="bg-card rounded-2xl border shadow-sm">
    <div class="border-b p-6">
      <h2 class="flex items-center gap-2 text-lg font-semibold">
        <KeyRoundIcon class="size-5" /> Access tokens and desktop drive
      </h2>
      <p class="text-muted-foreground mt-1 text-sm">
        Tokens let apps and AI agents act as you in {{ auth.organization?.name }}, limited to what
        you choose. They also connect the drive to your computer.
      </p>
    </div>

    <div class="space-y-4 p-6">
      <div class="bg-muted/60 space-y-2 rounded-xl p-4 text-sm">
        <p class="flex items-center gap-2 font-medium">
          <HardDriveIcon class="size-4" /> Open the drive on your computer
        </p>
        <p class="text-muted-foreground">
          In Windows Explorer choose “Map network drive”, in macOS Finder “Connect to Server”, and
          use:
        </p>
        <div class="flex gap-2">
          <Input :model-value="webdavUrl" readonly class="h-8 font-mono text-xs" />
          <Button
            size="icon"
            variant="outline"
            class="size-8"
            aria-label="Copy drive address"
            @click="copy(webdavUrl)"
          >
            <CheckIcon v-if="copied === webdavUrl" class="size-4" /><CopyIcon
              v-else
              class="size-4"
            />
          </Button>
        </div>
        <p class="text-muted-foreground">
          Sign in with <span class="text-foreground font-medium">{{ auth.user?.email }}</span> and a
          token that has the WebDAV scope as the password.
        </p>
      </div>

      <div v-if="secret" class="border-primary/40 bg-primary/5 space-y-2 rounded-xl border p-4">
        <p class="text-sm font-medium">Copy your new token now. It won’t be shown again.</p>
        <div class="flex gap-2">
          <Input :model-value="secret" readonly class="h-8 font-mono text-xs" />
          <Button
            size="icon"
            variant="outline"
            class="size-8"
            aria-label="Copy token"
            @click="copy(secret)"
          >
            <CheckIcon v-if="copied === secret" class="size-4" /><CopyIcon v-else class="size-4" />
          </Button>
        </div>
      </div>

      <form class="space-y-3" @submit.prevent="create">
        <div class="space-y-1.5">
          <Label for="token-name">New token</Label>
          <Input id="token-name" v-model="name" placeholder="Work laptop, Claude, Zapier…" />
        </div>
        <fieldset class="flex flex-wrap gap-4">
          <legend class="sr-only">Scopes</legend>
          <label
            v-for="option in scopeOptions"
            :key="option.value"
            class="flex items-center gap-2 text-sm"
          >
            <Checkbox
              :model-value="scopes.includes(option.value)"
              @update:model-value="(value) => toggleScope(option.value, Boolean(value))"
            />
            {{ option.label }}
          </label>
        </fieldset>
        <Button type="submit" :disabled="!name.trim() || !scopes.length || busy"
          >Create token</Button
        >
      </form>

      <ul v-if="tokens.length" class="divide-y rounded-xl border">
        <li
          v-for="token in tokens"
          :key="token.id"
          class="flex flex-wrap items-center gap-3 px-4 py-3"
        >
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium">{{ token.name }}</p>
            <p class="text-muted-foreground text-xs">
              <span class="font-mono">{{ token.prefix }}…</span> · created
              {{ formatDate(token.created_at) }}
              <template v-if="token.last_used_at">
                · last used {{ formatDate(token.last_used_at) }}</template
              >
            </p>
          </div>
          <div class="flex flex-wrap gap-1">
            <Badge v-for="scope in token.scopes" :key="scope" variant="secondary">{{
              scope
            }}</Badge>
          </div>
          <Button
            variant="ghost"
            size="icon"
            class="size-8"
            :aria-label="`Revoke ${token.name}`"
            @click="revoke(token)"
          >
            <Trash2Icon class="size-4" />
          </Button>
        </li>
      </ul>
    </div>
  </section>
</template>
