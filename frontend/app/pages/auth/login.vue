<script setup lang="ts">
  import {
    EyeIcon,
    EyeOffIcon,
    FileTextIcon,
    LockIcon,
    LogInIcon,
    MailIcon,
    SparklesIcon,
  } from "@lucide/vue";
  import { ref } from "vue";
  import { toast } from "vue-sonner";

  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import { Separator } from "@/components/ui/separator";

  definePageMeta({
    layout: "auth",
  });

  useHead({
    title: "Sign in — Lumen DMS",
  });

  const auth = useAuthStore();
  const form = ref({ email: "", password: "" });
  const showPw = ref(false);

  const handleLogin = async () => {
    if (!form.value.email.trim() || !form.value.password.trim()) {
      toast.error("Please enter email and password");
      return;
    }
    const ok = await auth.login({ email: form.value.email, password: form.value.password });
    if (!ok) {
      toast.error(auth.error || "Sign in failed");
      return;
    }
    toast.success(`Welcome back, ${auth.user?.name ?? ""}`);
    navigateTo("/");
  };
</script>

<template>
  <div class="bg-background flex min-h-dvh">
    <div
      class="from-primary/5 via-primary/[0.02] to-background relative hidden flex-1 items-center justify-center overflow-hidden bg-gradient-to-br lg:flex"
    >
      <div
        class="from-primary/10 absolute inset-0 bg-[radial-gradient(ellipse_at_top_right,_var(--tw-gradient-stops))] via-transparent to-transparent"
      />
      <div class="relative max-w-md px-8 text-center">
        <div
          class="bg-primary/10 text-primary ring-primary/20 mx-auto mb-6 grid h-20 w-20 place-items-center rounded-2xl ring-1"
        >
          <FileTextIcon class="h-10 w-10" />
        </div>
        <h2 class="text-3xl font-bold tracking-tight">Lumen DMS</h2>
        <p class="text-muted-foreground mt-3 leading-relaxed">
          Intelligent document management for modern enterprises. AI-powered classification, smart
          routing, and seamless collaboration.
        </p>
        <div class="mt-8 grid grid-cols-3 gap-4 text-left">
          <div class="border-border bg-card/60 rounded-xl border p-3">
            <SparklesIcon class="text-primary mb-1 h-5 w-5" />
            <p class="text-xs font-medium">AI Classification</p>
            <p class="text-muted-foreground mt-0.5 text-[10px]">Auto-tag &amp; summarize</p>
          </div>
          <div class="border-border bg-card/60 rounded-xl border p-3">
            <LockIcon class="text-primary mb-1 h-5 w-5" />
            <p class="text-xs font-medium">Secure Vault</p>
            <p class="text-muted-foreground mt-0.5 text-[10px]">Encrypted at rest</p>
          </div>
          <div class="border-border bg-card/60 rounded-xl border p-3">
            <svg
              class="text-primary mb-1 h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
              />
            </svg>
            <p class="text-xs font-medium">Smart Routing</p>
            <p class="text-muted-foreground mt-0.5 text-[10px]">Approval workflows</p>
          </div>
        </div>
      </div>
    </div>

    <div class="flex flex-1 items-center justify-center p-6">
      <div class="w-full max-w-sm">
        <div class="mb-8 text-center lg:text-left">
          <div class="mb-2 inline-flex items-center gap-2 text-lg font-semibold lg:hidden">
            <FileTextIcon class="text-primary h-5 w-5" /> Lumen
          </div>
          <h1 class="text-2xl font-bold tracking-tight">Sign in</h1>
          <p class="text-muted-foreground mt-1 text-sm">Access your document workspace.</p>
        </div>

        <form @submit.prevent="handleLogin" class="space-y-4">
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
              <button
                type="button"
                class="text-muted-foreground hover:text-primary text-xs transition-colors"
              >
                Forgot password?
              </button>
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
      </div>
    </div>
  </div>
</template>
