<script setup lang="ts">
  import { BellIcon, CheckCheckIcon } from "@lucide/vue";
  import { toast } from "vue-sonner";

  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Button } from "@/components/ui/button";
  import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
  } from "@/components/ui/dropdown-menu";
  import type { AppNotification, NotificationKind } from "@/types/api";
  import { formatRelative } from "@/utils/format";

  const auth = useAuthStore();
  const router = useRouter();
  const inbox = notificationsService();
  const { subscribe } = useRealtime();

  // Mentions already get their own toast from the layout; these ask for action.
  const toastKinds = new Set<NotificationKind>([
    "route_step",
    "form_approval",
    "access_request",
    "signature",
  ]);

  let stop: (() => void) | undefined;
  watch(
    () => auth.user?.id,
    (id) => {
      stop?.();
      stop = undefined;
      if (!id) return;
      void inbox.load();
      stop = subscribe(`user:${id}`, (event) => {
        if (event.type !== "notification") return;
        const notification = event.data as AppNotification;
        inbox.receive(notification);
        if (toastKinds.has(notification.kind)) {
          toast(notification.title, {
            description: notification.body?.slice(0, 140),
            action: notification.link
              ? { label: "Open", onClick: () => void open(notification) }
              : undefined,
          });
        }
      });
    },
    { immediate: true },
  );
  onBeforeUnmount(() => stop?.());

  const open = async (notification: AppNotification) => {
    void inbox.markRead(notification);
    if (notification.link) await router.push(notification.link);
  };

  const markAllRead = async () => {
    if (await inbox.markAllRead()) toast.success("All notifications marked as read");
  };

  const clearRead = async () => {
    if (await inbox.clearRead()) toast.success("Read notifications cleared");
  };

  const initials = (name = "") =>
    name
      .split(" ")
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase() ?? "")
      .join("") || "•";
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <Button
        variant="ghost"
        size="icon"
        class="text-muted-foreground hover:text-foreground relative"
        :aria-label="
          inbox.unread.value ? `Notifications, ${inbox.unread.value} unread` : 'Notifications'
        "
      >
        <BellIcon class="h-4 w-4" />
        <span
          v-if="inbox.unread.value > 0"
          class="bg-primary text-primary-foreground absolute -top-0.5 -right-0.5 grid h-4 min-w-4 place-items-center rounded-full px-1 text-[9px] font-semibold"
        >
          {{ inbox.unread.value > 99 ? "99+" : inbox.unread.value }}
        </span>
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" class="max-h-[420px] w-80 overflow-y-auto">
      <div class="flex items-center justify-between px-2 py-1.5">
        <DropdownMenuLabel class="text-xs font-semibold">Notifications</DropdownMenuLabel>
        <div class="flex items-center gap-1">
          <button
            v-if="inbox.unread.value > 0"
            class="text-muted-foreground hover:text-foreground hover:bg-secondary flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px]"
            @click="markAllRead"
          >
            <CheckCheckIcon class="h-3 w-3" /> Mark read
          </button>
          <button
            v-if="inbox.items.value.some((item) => item.read_at)"
            class="text-muted-foreground hover:text-destructive hover:bg-secondary rounded px-1.5 py-0.5 text-[10px]"
            @click="clearRead"
          >
            Clear read
          </button>
        </div>
      </div>
      <DropdownMenuSeparator />
      <div
        v-if="inbox.items.value.length === 0"
        class="text-muted-foreground p-4 text-center text-xs"
      >
        All caught up!
      </div>
      <button
        v-for="n in inbox.items.value"
        :key="n.id"
        type="button"
        class="hover:bg-secondary/60 focus-visible:bg-secondary/60 block w-full px-3 py-2 text-left transition-colors outline-none"
        @click="open(n)"
      >
        <div class="flex items-start gap-2.5">
          <Avatar class="mt-0.5 h-6 w-6">
            <AvatarFallback class="bg-secondary text-[9px]">{{
              initials(n.actor?.name)
            }}</AvatarFallback>
          </Avatar>
          <div class="min-w-0 flex-1 text-xs leading-snug">
            <p :class="n.read_at ? 'text-muted-foreground' : 'font-medium'">{{ n.title }}</p>
            <p v-if="n.body" class="text-muted-foreground mt-0.5 line-clamp-2">{{ n.body }}</p>
            <p class="text-muted-foreground mt-0.5 text-[10px]">
              {{ formatRelative(n.created_at) }}
            </p>
          </div>
          <span
            v-if="!n.read_at"
            class="bg-primary mt-1.5 h-2 w-2 shrink-0 rounded-full"
            aria-label="Unread"
          />
        </div>
      </button>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
