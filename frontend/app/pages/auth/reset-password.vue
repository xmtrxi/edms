<script setup lang="ts">
  import { CheckCircle2Icon, FileTextIcon, TriangleAlertIcon } from "@lucide/vue";
  import { computed, ref } from "vue";
  import { toast } from "vue-sonner";

  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";

  definePageMeta({ layout: "auth" });
  useHead({ title: "Choose a new password" });

  const auth = useAuthStore();
  const token = computed(() => String(useRoute().query.token ?? ""));
  const password = ref("");
  const confirm = ref("");
  const saving = ref(false);
  const done = ref(false);

  const submit = async () => {
    if (password.value.length < 8 || password.value.length > 72) {
      toast.error("Use between 8 and 72 characters");
      return;
    }
    if (password.value !== confirm.value) {
      toast.error("Passwords do not match");
      return;
    }
    saving.value = true;
    const ok = await auth.resetPassword(token.value, password.value);
    saving.value = false;
    if (!ok) {
      toast.error(auth.error || "Could not reset the password");
      return;
    }
    // The reset signed out every session, including this browser's.
    await auth.logout();
    done.value = true;
  };

  const brand = useBrand();
</script>

<template>
  <div class="flex min-h-dvh items-center justify-center p-6">
    <div class="w-full max-w-sm">
      <div class="mb-8">
        <div class="mb-4 inline-flex items-center gap-2 text-lg font-semibold">
          <FileTextIcon class="text-primary h-5 w-5" /> {{ brand }}
        </div>
        <h1 class="text-2xl font-bold tracking-tight">Choose a new password</h1>
      </div>

      <div v-if="!token" class="space-y-4">
        <div class="flex items-start gap-3 rounded-lg border p-3 text-sm">
          <TriangleAlertIcon class="text-destructive mt-0.5 h-4 w-4 shrink-0" />
          <p>This link is incomplete. Open the link from your email again, or ask for a new one.</p>
        </div>
        <Button as-child class="w-full"
          ><NuxtLink to="/auth/forgot-password">Get a new link</NuxtLink></Button
        >
      </div>

      <div v-else-if="done" class="space-y-4" role="status">
        <div class="bg-muted/50 flex items-start gap-3 rounded-lg border p-3 text-sm">
          <CheckCircle2Icon class="text-primary mt-0.5 h-4 w-4 shrink-0" />
          <p>
            Your password was changed and you were signed out everywhere. Sign in with the new one.
          </p>
        </div>
        <Button as-child class="w-full"><NuxtLink to="/auth/login">Sign in</NuxtLink></Button>
      </div>

      <form v-else class="space-y-4" @submit.prevent="submit">
        <div class="space-y-1.5">
          <Label for="password">New password</Label>
          <Input
            id="password"
            v-model="password"
            type="password"
            autocomplete="new-password"
            autofocus
          />
          <p class="text-muted-foreground text-xs">At least 8 characters.</p>
        </div>
        <div class="space-y-1.5">
          <Label for="confirm">Confirm password</Label>
          <Input id="confirm" v-model="confirm" type="password" autocomplete="new-password" />
        </div>
        <Button type="submit" class="w-full" :disabled="saving">
          {{ saving ? "Saving…" : "Save password" }}
        </Button>
        <p v-if="auth.error.includes('expired')" class="text-muted-foreground text-center text-xs">
          <NuxtLink to="/auth/forgot-password" class="text-primary hover:underline"
            >Ask for a new link</NuxtLink
          >
        </p>
      </form>
    </div>
  </div>
</template>
