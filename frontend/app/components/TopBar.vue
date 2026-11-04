<script setup lang="ts">
  import {
    CommandIcon,
    LogOutIcon,
    MoonIcon,
    PlusIcon,
    SearchIcon,
    SettingsIcon,
    SparklesIcon,
    SunIcon,
    UserIcon,
  } from "@lucide/vue";
  import { ref, onMounted } from "vue";
  import { useRouter } from "vue-router";

  import UploadDialog from "@/components/documents/UploadDialog.vue";
  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
  } from "@/components/ui/dropdown-menu";
  import { Separator } from "@/components/ui/separator";
  import { SidebarTrigger } from "@/components/ui/sidebar";
  import { useKeyboardShortcuts } from "@/composables/useKeyboardShortcuts";

  const router = useRouter();

  const auth = useAuthStore();
  const me = computed(() => {
    const name = auth.user?.name ?? "";
    const initials = name
      .split(" ")
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase() ?? "")
      .join("");
    return { name, initials, email: auth.user?.email ?? "" };
  });

  const uploadOpen = ref(false);
  const paletteOpen = ref(false);
  const THEME_KEY = "lumen.theme";
  const isDark = ref(false);

  onMounted(() => {
    const stored = localStorage.getItem(THEME_KEY);
    if (stored === "dark") {
      isDark.value = true;
      document.documentElement.classList.add("dark");
    } else {
      isDark.value = document.documentElement.classList.contains("dark");
    }
  });

  const toggleTheme = () => {
    isDark.value = !isDark.value;
    document.documentElement.classList.toggle("dark", isDark.value);
    localStorage.setItem(THEME_KEY, isDark.value ? "dark" : "light");
  };

  const logout = async () => {
    await auth.logout();
    router.push("/auth/login");
  };

  useKeyboardShortcuts([
    {
      key: "/",
      handler: () => {
        paletteOpen.value = true;
      },
    },
    {
      key: "n",
      ctrl: true,
      handler: () => {
        uploadOpen.value = true;
      },
    },
  ]);
</script>

<template>
  <header
    class="border-border/70 glass sticky top-0 z-30 flex h-14 items-center gap-2 border-b px-3 sm:px-4"
  >
    <SidebarTrigger class="text-muted-foreground hover:text-foreground" />
    <Separator orientation="vertical" class="mx-1 hidden h-5 sm:block" />

    <button
      @click="paletteOpen = true"
      class="group bg-secondary/60 hover:bg-secondary hover:border-border relative flex h-9 max-w-2xl flex-1 items-center rounded-lg border border-transparent px-3 text-left transition-colors"
    >
      <SearchIcon class="text-muted-foreground h-4 w-4" />
      <span class="text-muted-foreground ml-2 truncate text-sm"
        >Search documents, people, libraries…</span
      >
      <span class="text-muted-foreground ml-auto hidden items-center gap-1 text-[11px] sm:flex">
        <kbd
          class="border-border bg-background inline-flex h-5 items-center gap-0.5 rounded border px-1.5 font-mono"
        >
          <CommandIcon class="h-3 w-3" />K
        </kbd>
        <span class="text-muted-foreground/50">/</span>
      </span>
    </button>

    <div class="ml-auto flex items-center gap-1">
      <Button
        variant="ghost"
        size="sm"
        class="text-muted-foreground hover:text-foreground hidden gap-1.5 md:inline-flex"
        @click="router.push('/ai-insights')"
      >
        <SparklesIcon class="text-primary h-4 w-4" />
        Ask AI
      </Button>
      <Button
        variant="ghost"
        size="icon"
        class="text-muted-foreground hover:text-foreground"
        aria-label="Toggle theme"
        @click="toggleTheme"
      >
        <SunIcon v-if="isDark" class="h-4 w-4" />
        <MoonIcon v-else class="h-4 w-4" />
      </Button>

      <NotificationBell />

      <Separator orientation="vertical" class="mx-1 h-5" />

      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button
            variant="ghost"
            size="icon"
            class="h-8 w-8 overflow-hidden rounded-full p-0"
            aria-label="User menu"
          >
            <Avatar class="h-8 w-8">
              <AvatarFallback class="bg-primary/10 text-primary text-xs font-medium">{{
                me.initials
              }}</AvatarFallback>
            </Avatar>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-48">
          <DropdownMenuLabel class="font-normal">
            <div class="font-medium">{{ me.name }}</div>
            <div class="text-muted-foreground text-xs font-normal">{{ me.email }}</div>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem @click="router.push('/settings/account')">
            <UserIcon class="mr-2 h-4 w-4" /> Account
          </DropdownMenuItem>
          <DropdownMenuItem @click="router.push('/settings')">
            <SettingsIcon class="mr-2 h-4 w-4" /> Settings
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem @click="logout">
            <LogOutIcon class="mr-2 h-4 w-4" /> Log out
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Button size="sm" class="shadow-elegant gap-1.5" @click="uploadOpen = true">
        <PlusIcon class="h-4 w-4" />
        <span class="hidden sm:inline">Upload</span>
      </Button>
    </div>
    <UploadDialog :open="uploadOpen" @update:open="uploadOpen = $event" />
    <CommandPalette :open="paletteOpen" @update:open="paletteOpen = $event" />
  </header>
</template>
