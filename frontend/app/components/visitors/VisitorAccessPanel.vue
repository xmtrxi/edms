<script setup lang="ts">
  import { ArrowRightIcon, KeyRoundIcon, MailCheckIcon, ShieldCheckIcon } from "@lucide/vue";
  import { reactive, shallowRef } from "vue";

  import { Button } from "@/components/ui/button";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";

  const props = defineProps<{ loading: boolean; error: string }>();
  const emit = defineEmits<{ unlocked: [] }>();
  const service = visitorsService();
  const stage = shallowRef<"code" | "otp">("code");
  const developmentOTP = shallowRef("");
  const form = reactive({ email: "", accessCode: "", otp: "" });

  const requestCode = async () => {
    const otp = await service.requestOTP(form.email, form.accessCode.trim().toUpperCase());
    if (!service.error.value) {
      developmentOTP.value = otp;
      stage.value = "otp";
    }
  };

  const verify = async () => {
    if (await service.verify(form.email, form.accessCode.trim().toUpperCase(), form.otp)) {
      emit("unlocked");
    }
  };
</script>

<template>
  <section
    class="bg-card mx-auto grid max-w-5xl overflow-hidden rounded-2xl shadow-[0_24px_80px_-36px_oklch(0.18_0.03_250/0.45)] lg:grid-cols-[1.05fr_0.95fr]"
  >
    <div class="bg-primary text-primary-foreground relative min-h-72 overflow-hidden p-8 sm:p-12">
      <div class="bg-primary-foreground/20 absolute inset-x-0 bottom-0 h-px" />
      <div class="relative max-w-md">
        <ShieldCheckIcon class="mb-8 size-8 opacity-85" />
        <h1 class="text-4xl font-semibold tracking-[-0.03em] text-balance sm:text-5xl">
          Your paperwork, without the waiting room.
        </h1>
        <p class="text-primary-foreground/75 mt-5 max-w-[52ch] text-sm leading-6">
          Open the request your host prepared, provide the required information, and return here to
          follow each office hand-off.
        </p>
      </div>
      <div
        class="text-primary-foreground/65 relative mt-12 flex items-center gap-3 font-mono text-[11px] tracking-[0.14em] uppercase"
      >
        <span>Verify</span><span class="h-px w-8 bg-current" /><span>Complete</span
        ><span class="h-px w-8 bg-current" /><span>Track</span>
      </div>
    </div>

    <form
      class="flex flex-col justify-center p-8 sm:p-12"
      @submit.prevent="stage === 'code' ? requestCode() : verify()"
    >
      <div v-if="stage === 'code'">
        <KeyRoundIcon class="text-primary mb-5 size-6" />
        <h2 class="text-2xl font-semibold tracking-[-0.02em]">Open your request</h2>
        <p class="text-muted-foreground mt-2 text-sm leading-6">
          Use the email address and access code provided by your host.
        </p>
        <div class="mt-8 space-y-6">
          <div class="space-y-2">
            <Label for="visitor-email">Email address</Label>
            <Input
              id="visitor-email"
              v-model="form.email"
              type="email"
              autocomplete="email"
              required
            />
          </div>
          <div class="space-y-2">
            <Label for="visitor-code">Access code</Label>
            <Input
              id="visitor-code"
              v-model="form.accessCode"
              class="font-mono tracking-[0.2em] uppercase"
              autocomplete="one-time-code"
              maxlength="8"
              required
            />
          </div>
        </div>
      </div>
      <div v-else>
        <MailCheckIcon class="text-primary mb-5 size-6" />
        <h2 class="text-2xl font-semibold tracking-[-0.02em]">Check your email</h2>
        <p class="text-muted-foreground mt-2 text-sm leading-6">
          Enter the six-digit code sent to {{ form.email }}. It expires in 10 minutes.
        </p>
        <div
          v-if="developmentOTP"
          class="bg-accent text-accent-foreground mt-5 rounded-lg px-4 py-3 text-sm"
        >
          Development code:
          <strong class="font-mono tracking-[0.18em]">{{ developmentOTP }}</strong>
        </div>
        <div class="mt-8 space-y-2">
          <Label for="visitor-otp">Verification code</Label>
          <Input
            id="visitor-otp"
            v-model="form.otp"
            inputmode="numeric"
            autocomplete="one-time-code"
            class="font-mono text-lg tracking-[0.35em]"
            maxlength="6"
            required
          />
        </div>
      </div>
      <p
        v-if="props.error || service.error.value"
        role="alert"
        class="text-destructive mt-5 text-sm"
      >
        {{ props.error || service.error.value }}
      </p>
      <Button
        type="submit"
        size="lg"
        class="mt-8 w-full justify-between"
        :disabled="props.loading || service.loading.value"
      >
        {{ stage === "code" ? "Send verification code" : "Open request" }}
        <ArrowRightIcon class="size-4" />
      </Button>
      <button
        v-if="stage === 'otp'"
        type="button"
        class="text-muted-foreground hover:text-foreground mt-4 text-sm underline underline-offset-4"
        @click="stage = 'code'"
      >
        Use a different email or code
      </button>
    </form>
  </section>
</template>
