<script setup lang="ts">
  import { EyeIcon, EyeOffIcon, FileTextIcon, LockIcon, MailIcon, UserIcon } from "@lucide/vue";
  import { ref } from "vue";
  import { toast } from "vue-sonner";

  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";

  definePageMeta({
    layout: "auth",
  });

  useHead({
    title: "Create account — Lumen DMS",
  });

  const auth = useAuthStore();
  const form = ref({ name: "", email: "", password: "", confirm: "" });
  const showPw = ref(false);

  const handleRegister = async () => {
    if (!form.value.name.trim() || !form.value.email.trim() || !form.value.password.trim()) {
      toast.error("Please fill in all fields");
      return;
    }
    if (form.value.password !== form.value.confirm) {
      toast.error("Passwords do not match");
      return;
    }
    const registered = await auth.register({
      name: form.value.name.trim(),
      email: form.value.email,
      password: form.value.password,
    });
    if (!registered) {
      toast.error(auth.error || "Could not create account");
      return;
    }
    const loggedIn = await auth.login({ email: form.value.email, password: form.value.password });
    if (!loggedIn) {
      toast.success("Account created — please sign in");
      navigateTo("/auth/login");
      return;
    }
    toast.success(`Welcome, ${auth.user?.name ?? ""}`);
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
          One workspace for capturing, classifying, routing, and retrieving your documents.
        </p>
      </div>
    </div>

    <div class="flex flex-1 items-center justify-center p-6">
      <div class="w-full max-w-sm">
        <div class="mb-8 text-center lg:text-left">
          <div class="mb-2 inline-flex items-center gap-2 text-lg font-semibold lg:hidden">
            <FileTextIcon class="text-primary h-5 w-5" /> Lumen
          </div>
          <h1 class="text-2xl font-bold tracking-tight">Create account</h1>
          <p class="text-muted-foreground mt-1 text-sm">Start your document workspace.</p>
        </div>

        <form @submit.prevent="handleRegister" class="space-y-4">
          <div class="space-y-1.5">
            <Label for="name">Full name</Label>
            <div class="relative">
              <UserIcon
                class="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2"
              />
              <Input
                id="name"
                v-model="form.name"
                type="text"
                placeholder="Ada Lovelace"
                class="pl-9"
                autocomplete="name"
              />
            </div>
          </div>
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
            <Label for="password">Password</Label>
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
                autocomplete="new-password"
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
          <div class="space-y-1.5">
            <Label for="confirm">Confirm password</Label>
            <Input
              id="confirm"
              v-model="form.confirm"
              type="password"
              placeholder="• • • • • • • •"
              autocomplete="new-password"
            />
          </div>
          <Button type="submit" class="w-full gap-2" :disabled="auth.loading">
            <span
              v-if="auth.loading"
              class="border-background h-4 w-4 animate-spin rounded-full border-2 border-t-transparent"
            />
            {{ auth.loading ? "Creating account…" : "Create account" }}
          </Button>
        </form>

        <p class="text-muted-foreground mt-6 text-center text-sm">
          Already have an account?
          <NuxtLink to="/auth/login" class="text-primary font-medium hover:underline"
            >Sign in</NuxtLink
          >
        </p>
      </div>
    </div>
  </div>
</template>
