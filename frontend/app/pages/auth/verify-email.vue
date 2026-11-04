<script setup lang="ts">
  import { CheckCircle2Icon, FileTextIcon, MailIcon, TriangleAlertIcon } from "@lucide/vue";
  import { onMounted, ref } from "vue";
  import { toast } from "vue-sonner";

  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import { Spinner } from "@/components/ui/spinner";
  import { getToken } from "@/utils/token";

  definePageMeta({ layout: "auth" });
  useHead({ title: "Confirm your email" });

  const auth = useAuthStore();
  const route = useRoute();
  const token = String(route.query.token ?? "");
  // "sent" arrives from sign-up: the link is in the inbox, nothing to check yet.
  const email = ref(String(route.query.sent ?? ""));
  const state = ref<"sent" | "checking" | "verified" | "failed">(token ? "checking" : "sent");
  const resending = ref(false);
  const signedIn = Boolean(getToken());

  onMounted(async () => {
    if (!token) return;
    state.value = (await auth.verifyEmail(token)) ? "verified" : "failed";
  });

  const resend = async () => {
    if (!email.value.trim()) {
      toast.error("Enter your email address");
      return;
    }
    resending.value = true;
    await auth.resendVerification(email.value.trim());
    resending.value = false;
    state.value = "sent";
    toast.success("If that address needs confirming, a new link is on its way.");
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
        <h1 class="text-2xl font-bold tracking-tight">Confirm your email</h1>
      </div>

      <div
        v-if="state === 'checking'"
        class="text-muted-foreground flex items-center gap-2 text-sm"
        role="status"
      >
        <Spinner class="h-4 w-4" /> Confirming your address…
      </div>

      <div v-else-if="state === 'verified'" class="space-y-4" role="status">
        <div class="bg-muted/50 flex items-start gap-3 rounded-lg border p-3 text-sm">
          <CheckCircle2Icon class="text-primary mt-0.5 h-4 w-4 shrink-0" />
          <p>Your email address is confirmed.</p>
        </div>
        <Button as-child class="w-full">
          <NuxtLink :to="signedIn ? '/' : '/auth/login'">{{
            signedIn ? "Go to your workspace" : "Sign in"
          }}</NuxtLink>
        </Button>
      </div>

      <div v-else class="space-y-4">
        <div
          v-if="state === 'sent'"
          class="bg-muted/50 flex items-start gap-3 rounded-lg border p-3 text-sm"
          role="status"
        >
          <MailIcon class="text-primary mt-0.5 h-4 w-4 shrink-0" />
          <p>
            We sent a confirmation link<template v-if="email">
              to <span class="font-medium">{{ email }}</span></template
            >. Open it to finish setting up your account; it works for 48 hours.
          </p>
        </div>
        <div v-else class="flex items-start gap-3 rounded-lg border p-3 text-sm" role="alert">
          <TriangleAlertIcon class="text-destructive mt-0.5 h-4 w-4 shrink-0" />
          <p>{{ auth.error || "This link is invalid or has expired." }} Send yourself a new one.</p>
        </div>
        <form class="space-y-3" @submit.prevent="resend">
          <div class="space-y-1.5">
            <Label for="email">Email</Label>
            <Input id="email" v-model="email" type="email" autocomplete="email" />
          </div>
          <Button type="submit" variant="outline" class="w-full" :disabled="resending">
            {{ resending ? "Sending…" : "Send a new link" }}
          </Button>
        </form>
        <NuxtLink
          to="/auth/login"
          class="text-muted-foreground hover:text-foreground block text-center text-sm"
        >
          Back to sign in
        </NuxtLink>
      </div>
    </div>
  </div>
</template>
