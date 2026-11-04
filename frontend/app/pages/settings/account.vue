<script setup lang="ts">
  import {
    CheckIcon,
    CopyIcon,
    KeyRoundIcon,
    LaptopIcon,
    LogOutIcon,
    MailWarningIcon,
    ShieldCheckIcon,
    UserIcon,
  } from "@lucide/vue";
  import { renderSVG } from "uqr";
  import { computed, onMounted, ref } from "vue";
  import { toast } from "vue-sonner";

  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
  } from "@/components/ui/dialog";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import type { AccountSession, MFASetup } from "@/types/api";
  import { formatDateTime } from "@/utils/format";

  definePageMeta({
    title: "Account",
    description: "Your profile, password and sign-in security.",
    breadcrumbs: [
      { label: "Workspace", to: "/" },
      { label: "Settings", to: "/settings" },
      { label: "Account" },
    ],
  });
  useHead({ title: "Account" });

  const auth = useAuthStore();
  const user = computed(() => auth.user);

  // Profile
  const name = ref(auth.user?.name ?? "");
  const savingName = ref(false);
  const saveName = async () => {
    savingName.value = true;
    const ok = await auth.updateProfile(name.value.trim());
    savingName.value = false;
    if (ok) toast.success("Name saved");
    else toast.error(auth.error || "Could not save your name");
  };

  const sendingVerification = ref(false);
  const sendVerification = async () => {
    sendingVerification.value = true;
    const ok = await auth.sendVerification();
    sendingVerification.value = false;
    if (ok) toast.success(`Confirmation link sent to ${user.value?.email}`);
    else toast.error(auth.error || "Could not send the link");
  };

  // Password
  const passwords = ref({ current: "", next: "", confirm: "" });
  const changingPassword = ref(false);
  const changePassword = async () => {
    const { current, next, confirm } = passwords.value;
    if (!current || !next) {
      toast.error("Enter your current and new password");
      return;
    }
    if (next.length < 8 || next.length > 72) {
      toast.error("Use between 8 and 72 characters");
      return;
    }
    if (next !== confirm) {
      toast.error("New passwords do not match");
      return;
    }
    changingPassword.value = true;
    const ok = await auth.changePassword(current, next);
    changingPassword.value = false;
    if (!ok) {
      toast.error(auth.error || "Could not change your password");
      return;
    }
    passwords.value = { current: "", next: "", confirm: "" };
    toast.success("Password changed. Your other sessions were signed out.");
    void loadSessions();
  };

  // Two-step sign-in
  const setup = ref<MFASetup | null>(null);
  const setupCode = ref("");
  const recoveryCodes = ref<string[]>([]);
  const mfaBusy = ref(false);
  const qr = computed(() => (setup.value ? renderSVG(setup.value.uri, { border: 1 }) : ""));
  const disableOpen = ref(false);
  const disableForm = ref({ password: "", code: "" });
  const regenerateCode = ref("");

  const beginMfa = async () => {
    mfaBusy.value = true;
    setup.value = await auth.beginMfa();
    mfaBusy.value = false;
    if (!setup.value) toast.error(auth.error || "Could not start setup");
  };

  const enableMfa = async () => {
    mfaBusy.value = true;
    const codes = await auth.enableMfa(setupCode.value.trim());
    mfaBusy.value = false;
    if (!codes) {
      toast.error(auth.error || "That code is not valid");
      return;
    }
    setup.value = null;
    setupCode.value = "";
    recoveryCodes.value = codes;
    toast.success("Two-step sign-in is on");
    void loadSessions();
  };

  const disableMfa = async () => {
    mfaBusy.value = true;
    const ok = await auth.disableMfa(disableForm.value.password, disableForm.value.code.trim());
    mfaBusy.value = false;
    if (!ok) {
      toast.error(auth.error || "Could not turn off two-step sign-in");
      return;
    }
    disableOpen.value = false;
    disableForm.value = { password: "", code: "" };
    toast.success("Two-step sign-in is off");
  };

  const regenerate = async () => {
    mfaBusy.value = true;
    const codes = await auth.regenerateRecoveryCodes(regenerateCode.value.trim());
    mfaBusy.value = false;
    if (!codes) {
      toast.error(auth.error || "That code is not valid");
      return;
    }
    regenerateCode.value = "";
    recoveryCodes.value = codes;
  };

  const copied = ref(false);
  const copyCodes = async () => {
    await navigator.clipboard.writeText(recoveryCodes.value.join("\n"));
    copied.value = true;
    setTimeout(() => (copied.value = false), 2000);
  };

  // Sessions
  const sessions = ref<AccountSession[]>([]);
  const loadingSessions = ref(false);
  const loadSessions = async () => {
    loadingSessions.value = true;
    sessions.value = await auth.listSessions();
    loadingSessions.value = false;
  };
  const revoke = async (session: AccountSession) => {
    if (!(await auth.revokeSession(session.id))) {
      toast.error(auth.error || "Could not sign out that session");
      return;
    }
    sessions.value = sessions.value.filter((s) => s.id !== session.id);
  };
  const revokeOthers = async () => {
    if (!(await auth.revokeOtherSessions())) {
      toast.error(auth.error || "Could not sign out other sessions");
      return;
    }
    sessions.value = sessions.value.filter((s) => s.current);
    toast.success("Signed out everywhere else");
  };

  const methodLabel: Record<string, string> = {
    password: "Password",
    sso: "Single sign-on",
    signup: "Account created",
    invite: "Invitation",
  };

  /** A readable device name from the user agent, without a parser library. */
  const deviceName = (ua = "") => {
    const browser = /Edg\//.test(ua)
      ? "Edge"
      : /Firefox\//.test(ua)
        ? "Firefox"
        : /Chrome\//.test(ua)
          ? "Chrome"
          : /Safari\//.test(ua)
            ? "Safari"
            : "";
    const os = /Windows/.test(ua)
      ? "Windows"
      : /Mac OS X/.test(ua)
        ? "macOS"
        : /Android/.test(ua)
          ? "Android"
          : /iPhone|iPad/.test(ua)
            ? "iOS"
            : /Linux/.test(ua)
              ? "Linux"
              : "";
    return [browser, os].filter(Boolean).join(" on ") || "Unknown device";
  };

  onMounted(loadSessions);
</script>

<template>
  <div class="mx-auto max-w-3xl space-y-5">
    <section class="bg-card rounded-2xl border shadow-sm">
      <div class="flex items-center gap-3 border-b p-6">
        <UserIcon class="text-primary size-5" />
        <div>
          <h1 class="text-lg font-semibold">Profile</h1>
          <p class="text-muted-foreground mt-0.5 text-sm">How you appear to colleagues.</p>
        </div>
      </div>
      <form class="space-y-4 p-6" @submit.prevent="saveName">
        <div class="space-y-1.5">
          <Label for="name">Name</Label>
          <div class="flex gap-2">
            <Input id="name" v-model="name" autocomplete="name" class="max-w-sm" />
            <Button
              type="submit"
              variant="outline"
              :disabled="savingName || !name.trim() || name === user?.name"
            >
              {{ savingName ? "Saving…" : "Save" }}
            </Button>
          </div>
        </div>
        <div class="space-y-1.5">
          <Label>Email</Label>
          <div class="flex flex-wrap items-center gap-2 text-sm">
            <span>{{ user?.email }}</span>
            <Badge v-if="user?.email_verified" variant="secondary" class="gap-1"
              ><CheckIcon class="size-3" />Confirmed</Badge
            >
            <template v-else>
              <Badge variant="outline" class="gap-1"
                ><MailWarningIcon class="size-3" />Not confirmed</Badge
              >
              <Button
                type="button"
                size="sm"
                variant="link"
                class="h-auto p-0"
                :disabled="sendingVerification"
                @click="sendVerification"
              >
                {{ sendingVerification ? "Sending…" : "Send confirmation link" }}
              </Button>
            </template>
          </div>
        </div>
      </form>
    </section>

    <section class="bg-card rounded-2xl border shadow-sm">
      <div class="flex items-center gap-3 border-b p-6">
        <KeyRoundIcon class="text-primary size-5" />
        <div>
          <h2 class="text-lg font-semibold">Password</h2>
          <p class="text-muted-foreground mt-0.5 text-sm">
            Changing it signs you out of every other session.
          </p>
        </div>
      </div>
      <form class="grid gap-4 p-6 sm:max-w-sm" @submit.prevent="changePassword">
        <div class="space-y-1.5">
          <Label for="current-password">Current password</Label>
          <Input
            id="current-password"
            v-model="passwords.current"
            type="password"
            autocomplete="current-password"
          />
        </div>
        <div class="space-y-1.5">
          <Label for="new-password">New password</Label>
          <Input
            id="new-password"
            v-model="passwords.next"
            type="password"
            autocomplete="new-password"
          />
        </div>
        <div class="space-y-1.5">
          <Label for="confirm-password">Confirm new password</Label>
          <Input
            id="confirm-password"
            v-model="passwords.confirm"
            type="password"
            autocomplete="new-password"
          />
        </div>
        <div>
          <Button type="submit" :disabled="changingPassword">
            {{ changingPassword ? "Changing…" : "Change password" }}
          </Button>
        </div>
      </form>
    </section>

    <section class="bg-card rounded-2xl border shadow-sm">
      <div class="flex items-center gap-3 border-b p-6">
        <ShieldCheckIcon class="text-primary size-5" />
        <div class="flex-1">
          <h2 class="text-lg font-semibold">Two-step sign-in</h2>
          <p class="text-muted-foreground mt-0.5 text-sm">
            Ask for a code from an authenticator app after your password.
          </p>
        </div>
        <Badge :variant="user?.mfa_enabled ? 'secondary' : 'outline'">{{
          user?.mfa_enabled ? "On" : "Off"
        }}</Badge>
      </div>

      <div class="space-y-5 p-6">
        <div v-if="recoveryCodes.length" class="space-y-3 rounded-xl border p-4" role="status">
          <p class="text-sm font-medium">Save your recovery codes</p>
          <p class="text-muted-foreground text-xs">
            Each code signs you in once if you lose your phone. They won&apos;t be shown again.
          </p>
          <ul class="grid grid-cols-2 gap-1 font-mono text-sm">
            <li v-for="code in recoveryCodes" :key="code">{{ code }}</li>
          </ul>
          <div class="flex gap-2">
            <Button size="sm" variant="outline" class="gap-1.5" @click="copyCodes">
              <CheckIcon v-if="copied" class="size-4" /><CopyIcon v-else class="size-4" />
              {{ copied ? "Copied" : "Copy" }}
            </Button>
            <Button size="sm" variant="ghost" @click="recoveryCodes = []"
              >I&apos;ve saved them</Button
            >
          </div>
        </div>

        <template v-if="!user?.mfa_enabled">
          <Button v-if="!setup" :disabled="mfaBusy" @click="beginMfa"
            >Set up two-step sign-in</Button
          >
          <form v-else class="grid gap-5 sm:grid-cols-[auto_1fr]" @submit.prevent="enableMfa">
            <!-- eslint-disable-next-line vue/no-v-html -- SVG generated locally from the setup URI -->
            <div
              class="size-44 rounded-lg border bg-white p-2"
              aria-label="QR code for your authenticator app"
              v-html="qr"
            />
            <div class="space-y-3 text-sm">
              <ol class="text-muted-foreground list-decimal space-y-1 pl-4">
                <li>
                  Scan the code with an authenticator app such as Microsoft Authenticator or Google
                  Authenticator.
                </li>
                <li>Enter the six-digit code it shows.</li>
              </ol>
              <p class="text-muted-foreground text-xs">
                Can&apos;t scan? Enter this key:
                <code class="bg-muted rounded px-1 py-0.5 break-all">{{ setup.secret }}</code>
              </p>
              <div class="flex gap-2">
                <Input
                  v-model="setupCode"
                  inputmode="numeric"
                  autocomplete="one-time-code"
                  placeholder="123456"
                  class="max-w-36"
                />
                <Button type="submit" :disabled="mfaBusy || setupCode.trim().length < 6"
                  >Turn on</Button
                >
                <Button type="button" variant="ghost" @click="setup = null">Cancel</Button>
              </div>
            </div>
          </form>
        </template>

        <template v-else>
          <form class="flex flex-wrap items-end gap-2" @submit.prevent="regenerate">
            <div class="space-y-1.5">
              <Label for="regen-code">New recovery codes</Label>
              <Input
                id="regen-code"
                v-model="regenerateCode"
                inputmode="numeric"
                placeholder="Code from your app"
                class="w-48"
              />
            </div>
            <Button type="submit" variant="outline" :disabled="mfaBusy || !regenerateCode.trim()"
              >Replace codes</Button
            >
          </form>
          <Button variant="outline" class="text-destructive" @click="disableOpen = true"
            >Turn off two-step sign-in</Button
          >
        </template>
      </div>
    </section>

    <section class="bg-card rounded-2xl border shadow-sm">
      <div class="flex flex-wrap items-center gap-3 border-b p-6">
        <LaptopIcon class="text-primary size-5" />
        <div class="flex-1">
          <h2 class="text-lg font-semibold">Where you&apos;re signed in</h2>
          <p class="text-muted-foreground mt-0.5 text-sm">
            Sign out of devices you don&apos;t recognise.
          </p>
        </div>
        <Button variant="outline" size="sm" :disabled="sessions.length < 2" @click="revokeOthers">
          Sign out everywhere else
        </Button>
      </div>
      <p v-if="loadingSessions && !sessions.length" class="text-muted-foreground p-6 text-sm">
        Loading…
      </p>
      <ul v-else class="divide-y">
        <li v-for="session in sessions" :key="session.id" class="flex items-center gap-4 p-5">
          <div class="min-w-0 flex-1">
            <p class="flex items-center gap-2 text-sm font-medium">
              {{ deviceName(session.user_agent) }}
              <Badge v-if="session.current" variant="secondary">This device</Badge>
            </p>
            <p class="text-muted-foreground mt-0.5 text-xs">
              {{ methodLabel[session.method] ?? session.method }}
              <template v-if="session.ip"> · {{ session.ip }}</template>
              · active {{ formatDateTime(session.last_seen_at) }}
            </p>
          </div>
          <Button
            v-if="!session.current"
            variant="ghost"
            size="sm"
            class="gap-1.5"
            @click="revoke(session)"
          >
            <LogOutIcon class="size-4" /> Sign out
          </Button>
        </li>
      </ul>
    </section>

    <Dialog v-model:open="disableOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Turn off two-step sign-in?</DialogTitle>
          <DialogDescription>Your password alone will be enough to sign in.</DialogDescription>
        </DialogHeader>
        <form id="disable-mfa" class="space-y-3" @submit.prevent="disableMfa">
          <div class="space-y-1.5">
            <Label for="disable-password">Password</Label>
            <Input
              id="disable-password"
              v-model="disableForm.password"
              type="password"
              autocomplete="current-password"
            />
          </div>
          <div class="space-y-1.5">
            <Label for="disable-code">Code from your app or a recovery code</Label>
            <Input id="disable-code" v-model="disableForm.code" autocomplete="one-time-code" />
          </div>
        </form>
        <DialogFooter>
          <Button variant="ghost" @click="disableOpen = false">Cancel</Button>
          <Button type="submit" form="disable-mfa" variant="destructive" :disabled="mfaBusy"
            >Turn off</Button
          >
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
