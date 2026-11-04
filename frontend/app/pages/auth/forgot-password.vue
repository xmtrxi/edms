<script setup lang="ts">
  import { ArrowLeftIcon, FileTextIcon, MailIcon } from "@lucide/vue";
  import { ref } from "vue";
  import { toast } from "vue-sonner";

  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";

  definePageMeta({ layout: "auth" });
  useHead({ title: "Reset password" });

  const auth = useAuthStore();
  const email = ref(String(useRoute().query.email ?? ""));
  const sending = ref(false);
  const sent = ref(false);

  const submit = async () => {
    if (!email.value.trim()) {
      toast.error("Enter the email address you sign in with");
      return;
    }
    sending.value = true;
    const ok = await auth.requestPasswordReset(email.value.trim());
    sending.value = false;
    if (!ok) {
      toast.error(auth.error || "Could not send the reset link");
      return;
    }
    sent.value = true;
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
        <h1 class="text-2xl font-bold tracking-tight">Reset your password</h1>
        <p class="text-muted-foreground mt-1 text-sm">
          We&apos;ll email you a link to choose a new one.
        </p>
      </div>

      <div v-if="sent" class="space-y-4" role="status">
        <div class="bg-muted/50 flex items-start gap-3 rounded-lg border p-3 text-sm">
          <MailIcon class="text-primary mt-0.5 h-4 w-4 shrink-0" />
          <p>
            If an account uses <span class="font-medium">{{ email }}</span
            >, a reset link is on its way. It works for one hour.
          </p>
        </div>
        <Button variant="outline" class="w-full" :disabled="sending" @click="submit">
          {{ sending ? "Sending…" : "Send it again" }}
        </Button>
      </div>

      <form v-else class="space-y-4" @submit.prevent="submit">
        <div class="space-y-1.5">
          <Label for="email">Email</Label>
          <Input
            id="email"
            v-model="email"
            type="email"
            autocomplete="email"
            placeholder="you@company.com"
            autofocus
          />
        </div>
        <Button type="submit" class="w-full" :disabled="sending">
          {{ sending ? "Sending…" : "Send reset link" }}
        </Button>
      </form>

      <NuxtLink
        to="/auth/login"
        class="text-muted-foreground hover:text-foreground mt-6 inline-flex items-center gap-1.5 text-sm"
      >
        <ArrowLeftIcon class="h-4 w-4" /> Back to sign in
      </NuxtLink>
    </div>
  </div>
</template>
