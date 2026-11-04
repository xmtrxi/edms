<script setup lang="ts">
  import {
    EyeIcon,
    EyeOffIcon,
    FileTextIcon,
    LockIcon,
    KeyRoundIcon,
    LogInIcon,
    MailIcon,
    ShieldCheckIcon,
  } from "@lucide/vue";
  import { computed, onMounted, ref } from "vue";
  import { toast } from "vue-sonner";

  import AuthShowcase from "@/components/auth/AuthShowcase.vue";
  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import { Separator } from "@/components/ui/separator";

  definePageMeta({
    layout: "auth",
  });

  useHead({
    title: "Sign in",
  });

  const auth = useAuthStore();
  const route = useRoute();
  const form = ref({ email: "", password: "" });
  const showPw = ref(false);
  // "password" first; "mfa" asks for the authenticator code; "unverified"
  // offers to resend the confirmation email.
  const step = ref<"password" | "mfa" | "unverified">("password");
  const code = ref("");
  const resending = ref(false);

  const passwordLogin = computed(() => auth.meta?.password_login !== false);
  const redirectTarget = () => {
    const redirect = String(route.query.redirect ?? "");
    // Only follow same-app paths, never an absolute URL from the query string.
    return redirect.startsWith("/") && !redirect.startsWith("//") ? redirect : "/";
  };

  onMounted(() => {
    void auth.fetchMeta();
    const ssoError = String(route.query.sso_error ?? "");
    if (ssoError) toast.error(ssoError);
  });

  const finish = () => {
    toast.success(`Welcome back, ${auth.user?.name ?? ""}`);
    navigateTo(redirectTarget());
  };

  const handleLogin = async () => {
    if (!form.value.email.trim() || !form.value.password.trim()) {
      toast.error("Please enter email and password");
      return;
    }
    const outcome = await auth.login({
      email: form.value.email.trim(),
      password: form.value.password,
    });
    if (outcome === "mfa") {
      step.value = "mfa";
      code.value = "";
      return;
    }
    if (outcome === "unverified") {
      step.value = "unverified";
      return;
    }
    if (outcome === "failed") {
      toast.error(auth.error || "Sign in failed");
      return;
    }
    finish();
  };

  const handleCode = async () => {
    if (!code.value.trim()) {
      toast.error("Enter the code from your authenticator app");
      return;
    }
    if (!(await auth.completeMfa(code.value.trim()))) {
      toast.error(auth.error || "That code is not valid");
      // An expired challenge needs the password again.
      if (auth.error.includes("expired")) step.value = "password";
      return;
    }
    finish();
  };

  const resend = async () => {
    resending.value = true;
    await auth.resendVerification(form.value.email.trim());
    resending.value = false;
    toast.success("If that address needs confirming, a new link is on its way.");
  };

  const startSso = () => {
    window.location.assign(auth.ssoStartUrl(redirectTarget()));
  };

  const brand = useBrand();
</script>

<template>
  <div class="bg-background flex min-h-dvh">
    <AuthShowcase />

    <div class="flex flex-1 items-center justify-center p-6">
      <div class="w-full max-w-sm">
        <div class="mb-8 text-center lg:text-left">
          <div class="mb-2 inline-flex items-center gap-2 text-lg font-semibold lg:hidden">
            <FileTextIcon class="text-primary h-5 w-5" /> {{ brand }}
          </div>
          <h1 class="text-2xl font-bold tracking-tight">Sign in</h1>
          <p class="text-muted-foreground mt-1 text-sm">Access your document workspace.</p>
        </div>

        <form v-if="step === 'mfa'" class="space-y-4" @submit.prevent="handleCode">
          <div class="bg-muted/50 flex items-start gap-3 rounded-lg border p-3 text-sm">
            <ShieldCheckIcon class="text-primary mt-0.5 h-4 w-4 shrink-0" />
            <p>
              Enter the six-digit code from your authenticator app. If you don&apos;t have your
              phone, use one of your recovery codes.
            </p>
          </div>
          <div class="space-y-1.5">
            <Label for="code">Code</Label>
            <Input
              id="code"
              v-model="code"
              inputmode="numeric"
              autocomplete="one-time-code"
              placeholder="123456"
              autofocus
            />
          </div>
          <Button type="submit" class="w-full gap-2" :disabled="auth.loading">
            <KeyRoundIcon class="h-4 w-4" />
            {{ auth.loading ? "Checking…" : "Verify and sign in" }}
          </Button>
          <button
            type="button"
            class="text-muted-foreground hover:text-foreground w-full text-center text-xs"
            @click="step = 'password'"
          >
            Use a different account
          </button>
        </form>

        <div v-else-if="step === 'unverified'" class="space-y-4">
          <div class="bg-muted/50 flex items-start gap-3 rounded-lg border p-3 text-sm">
            <MailIcon class="text-primary mt-0.5 h-4 w-4 shrink-0" />
            <p>
              Confirm your email address before signing in. We sent a link to
              <span class="font-medium">{{ form.email }}</span
              >.
            </p>
          </div>
          <Button class="w-full" variant="outline" :disabled="resending" @click="resend">
            {{ resending ? "Sending…" : "Send the link again" }}
          </Button>
          <button
            type="button"
            class="text-muted-foreground hover:text-foreground w-full text-center text-xs"
            @click="step = 'password'"
          >
            Back to sign in
          </button>
        </div>

        <template v-else>
          <Button
            v-if="auth.meta?.sso"
            type="button"
            variant="outline"
            class="mb-4 w-full gap-2"
            @click="startSso"
          >
            <KeyRoundIcon class="h-4 w-4" />
            {{ auth.meta.sso_label || "Single sign-on" }}
          </Button>
          <div v-if="auth.meta?.sso && passwordLogin" class="relative mb-4">
            <Separator />
            <span
              class="bg-background text-muted-foreground absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 px-2 text-xs"
              >or use your password</span
            >
          </div>
          <form v-if="passwordLogin" @submit.prevent="handleLogin" class="space-y-4">
            <div class="space-y-1.5">
              <Label for="email">Email</Label>
              <div class="relative">
                <MailIcon
                  class="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2"
                />
                <Input
                  id="email"
                  v-model="form.email"
                  type="email"
                  placeholder="you@company.com"
                  class="pl-9"
                  autocomplete="email"
                />
              </div>
            </div>
            <div class="space-y-1.5">
              <div class="flex items-center justify-between">
                <Label for="password">Password</Label>
                <NuxtLink
                  to="/auth/forgot-password"
                  class="text-muted-foreground hover:text-primary text-xs transition-colors"
                >
                  Forgot password?
                </NuxtLink>
              </div>
              <div class="relative">
                <LockIcon
                  class="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2"
                />
                <Input
                  id="password"
                  v-model="form.password"
                  :type="showPw ? 'text' : 'password'"
                  placeholder="• • • • • • • •"
                  class="pr-9 pl-9"
                  autocomplete="current-password"
                />
                <button
                  type="button"
                  @click="showPw = !showPw"
                  class="text-muted-foreground hover:text-foreground absolute top-1/2 right-3 -translate-y-1/2"
                >
                  <EyeOffIcon v-if="showPw" class="h-4 w-4" />
                  <EyeIcon v-else class="h-4 w-4" />
                </button>
              </div>
            </div>
            <Button type="submit" class="w-full gap-2" :disabled="auth.loading">
              <LogInIcon v-if="!auth.loading" class="h-4 w-4" />
              <span
                v-if="auth.loading"
                class="border-background h-4 w-4 animate-spin rounded-full border-2 border-t-transparent"
              />
              {{ auth.loading ? "Signing in…" : "Sign in" }}
            </Button>
          </form>

          <template v-if="passwordLogin">
            <div class="relative my-6">
              <Separator />
              <span
                class="bg-background text-muted-foreground absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 px-2 text-xs"
                >or</span
              >
            </div>

            <p class="text-muted-foreground text-center text-sm">
              Don&apos;t have an account?
              <NuxtLink to="/auth/register" class="text-primary font-medium hover:underline"
                >Create one</NuxtLink
              >
            </p>
          </template>
        </template>
      </div>
    </div>
  </div>
</template>
