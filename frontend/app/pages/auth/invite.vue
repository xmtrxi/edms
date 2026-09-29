<script setup lang="ts">
  import { Building2Icon, LockIcon, UserIcon } from "@lucide/vue";
  import { computed, onMounted, shallowRef } from "vue";
  import { toast } from "vue-sonner";

  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import type { InvitationPreview } from "@/types/api";
  import { getToken } from "@/utils/token";

  definePageMeta({ layout: "auth" });
  useHead({ title: "Join organization — Lumen DMS" });

  const route = useRoute();
  const auth = useAuthStore();
  const token = computed(() => String(route.query.token ?? ""));

  const preview = shallowRef<InvitationPreview | null>(null);
  const loading = shallowRef(true);
  const signedIn = shallowRef(false);
  const name = shallowRef("");
  const password = shallowRef("");
  const confirm = shallowRef("");

  const roleLabel: Record<string, string> = {
    owner: "an owner",
    admin: "an admin",
    member: "a member",
    guest: "a guest",
  };

  onMounted(async () => {
    if (token.value) {
      preview.value = await auth.previewInvitation(token.value);
      name.value = preview.value?.name ?? "";
    }
    if (getToken()) signedIn.value = await auth.fetchSession();
    loading.value = false;
  });

  // A signed-in user whose email differs from the invitation must switch accounts.
  const wrongAccount = computed(
    () =>
      signedIn.value &&
      preview.value &&
      auth.user?.email.toLowerCase() !== preview.value.email.toLowerCase(),
  );

  const createAccount = async () => {
    if (password.value.length < 8) {
      toast.error("Use at least 8 characters for your password");
      return;
    }
    if (password.value !== confirm.value) {
      toast.error("Passwords do not match");
      return;
    }
    if (await auth.acceptInvitationAsNewUser(token.value, name.value.trim(), password.value)) {
      toast.success(`Welcome to ${preview.value?.organization_name}`);
      await navigateTo("/");
    } else toast.error(auth.error || "Could not accept the invitation");
  };

  const join = async () => {
    if (!(await auth.acceptInvitation(token.value))) {
      toast.error(auth.error || "Could not accept the invitation");
    }
  };

  const signInFirst = () => {
    auth.logout();
    navigateTo({ path: "/auth/login", query: { redirect: route.fullPath } });
  };
</script>

<template>
  <div class="bg-background flex min-h-dvh items-center justify-center p-6">
    <div class="bg-card w-full max-w-md rounded-2xl border p-8 shadow-sm">
      <div v-if="loading" class="space-y-3">
        <Skeleton class="h-10 w-10 rounded-xl" />
        <Skeleton class="h-6 w-2/3" />
        <Skeleton class="h-4 w-full" />
      </div>

      <div v-else-if="!preview" class="text-center">
        <h1 class="text-xl font-semibold">This invitation can't be used</h1>
        <p class="text-muted-foreground mt-2 text-sm">
          It may have expired, been revoked, or already been accepted. Ask the person who invited
          you to send a new one.
        </p>
        <Button class="mt-6" variant="outline" as-child>
          <NuxtLink to="/auth/login">Go to sign in</NuxtLink>
        </Button>
      </div>

      <template v-else>
        <div class="bg-primary/10 text-primary mb-5 grid size-12 place-items-center rounded-xl">
          <Building2Icon class="size-6" />
        </div>
        <h1 class="text-xl font-semibold tracking-tight">Join {{ preview.organization_name }}</h1>
        <p class="text-muted-foreground mt-1 text-sm">
          You've been invited as {{ roleLabel[preview.role] ?? "a member" }} with
          <span class="text-foreground font-medium">{{ preview.email }}</span
          >.
        </p>

        <div v-if="wrongAccount" class="mt-6 space-y-4">
          <p class="bg-muted rounded-lg p-3 text-sm">
            You're signed in as {{ auth.user?.email }}. Sign in with {{ preview.email }} to accept
            this invitation.
          </p>
          <Button class="w-full" @click="signInFirst">Switch account</Button>
        </div>

        <div v-else-if="signedIn" class="mt-6">
          <Button class="w-full" :disabled="auth.loading" @click="join">
            {{ auth.loading ? "Joining…" : `Join ${preview.organization_name}` }}
          </Button>
        </div>

        <div v-else-if="preview.account_exists" class="mt-6 space-y-3">
          <p class="text-muted-foreground text-sm">
            You already have an account. Sign in to accept.
          </p>
          <Button class="w-full" @click="signInFirst">Sign in to accept</Button>
        </div>

        <form v-else class="mt-6 space-y-4" @submit.prevent="createAccount">
          <div class="space-y-1.5">
            <Label for="name">Your name</Label>
            <div class="relative">
              <UserIcon
                class="text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2"
              />
              <Input id="name" v-model="name" class="pl-9" autocomplete="name" />
            </div>
          </div>
          <div class="space-y-1.5">
            <Label for="password">Choose a password</Label>
            <div class="relative">
              <LockIcon
                class="text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2"
              />
              <Input
                id="password"
                v-model="password"
                type="password"
                class="pl-9"
                autocomplete="new-password"
                placeholder="At least 8 characters"
              />
            </div>
          </div>
          <div class="space-y-1.5">
            <Label for="confirm">Confirm password</Label>
            <Input id="confirm" v-model="confirm" type="password" autocomplete="new-password" />
          </div>
          <Button type="submit" class="w-full" :disabled="auth.loading">
            {{ auth.loading ? "Creating your account…" : "Create account and join" }}
          </Button>
        </form>
      </template>
    </div>
  </div>
</template>
