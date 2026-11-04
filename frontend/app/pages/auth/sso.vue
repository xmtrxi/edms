<script setup lang="ts">
  import { onMounted } from "vue";

  import { Spinner } from "@/components/ui/spinner";

  definePageMeta({ layout: "auth" });
  useHead({ title: "Signing in" });

  const auth = useAuthStore();

  // The API puts the session token in the URL fragment, which is never sent
  // to a server. Read it, then drop it from the address bar and history.
  onMounted(async () => {
    const params = new URLSearchParams(window.location.hash.slice(1));
    const token = params.get("token") ?? "";
    const target = params.get("return") ?? "/";
    window.history.replaceState(null, "", window.location.pathname);
    if (!token || !(await auth.acceptSsoToken(token))) {
      navigateTo({
        path: "/auth/login",
        query: { sso_error: auth.error || "Single sign-on failed." },
      });
      return;
    }
    navigateTo(target.startsWith("/") && !target.startsWith("//") ? target : "/", {
      replace: true,
    });
  });
</script>

<template>
  <div
    class="text-muted-foreground flex min-h-dvh items-center justify-center gap-2 text-sm"
    role="status"
  >
    <Spinner class="h-4 w-4" /> Signing you in…
  </div>
</template>
