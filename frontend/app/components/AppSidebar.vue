<script setup lang="ts">
  import {
    LayoutDashboardIcon,
    FileTextIcon,
    Share2Icon,
    StarIcon,
    ClockIcon,
    UsersIcon,
    WorkflowIcon,
    ClipboardListIcon,
    SparklesIcon,
    CloudIcon,
    ArchiveIcon,
    Trash2Icon,
    SettingsIcon,
    FolderOpenIcon,
    TagIcon,
    Building2Icon,
    ShieldCheckIcon,
    HistoryIcon,
    SignatureIcon,
    ZapIcon,
  } from "@lucide/vue";
  import { onMounted, shallowRef } from "vue";
  import { useRoute } from "vue-router";

  import { NuxtLink } from "#components";
  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import {
    Sidebar,
    SidebarContent,
    SidebarGroup,
    SidebarGroupContent,
    SidebarGroupLabel,
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
    SidebarHeader,
    SidebarFooter,
    useSidebar,
  } from "@/components/ui/sidebar";
  import type { DashboardOverview } from "@/types/api";

  const route = useRoute();
  const { state } = useSidebar();
  const collapsed = computed(() => state.value === "collapsed");
  const isActive = (url: string) => (url === "/" ? route.path === "/" : route.path.startsWith(url));
  const auth = useAuthStore();
  const workspace = workspaceService();
  const overview = shallowRef<DashboardOverview | null>(null);
  const activeWorkflows = computed(() => overview.value?.active_workflows ?? 0);
  onMounted(async () => {
    overview.value = await workspace.overview();
  });

  interface NavItem {
    title: string;
    url: string;
    icon: typeof LayoutDashboardIcon;
    badge?: string | number;
  }

  const main = ref<NavItem[]>([
    { title: "Dashboard", url: "/", icon: LayoutDashboardIcon },
    { title: "Drive", url: "/documents", icon: FileTextIcon },
    { title: "Shared with me", url: "/shared", icon: Share2Icon },
    { title: "Favorites", url: "/favorites", icon: StarIcon },
    { title: "Recent", url: "/recent", icon: ClockIcon },
    { title: "Signatures", url: "/signatures", icon: SignatureIcon },
  ]);

  const admin = ref<NavItem[]>([
    { title: "Folders", url: "/admin/folders", icon: FolderOpenIcon },
    { title: "Teams & access", url: "/admin/access", icon: ShieldCheckIcon },
    { title: "Automation", url: "/admin/automation", icon: ZapIcon },
    { title: "Members", url: "/users", icon: UsersIcon },
    { title: "Audit Log", url: "/audit", icon: HistoryIcon },
  ]);

  const workspaces = ref<NavItem[]>([
    { title: "Teams", url: "/teams", icon: UsersIcon },
    { title: "Office Desk", url: "/office", icon: Building2Icon },
    { title: "Categories", url: "/categories", icon: TagIcon },
    { title: "Workflows", url: "/workflows", icon: WorkflowIcon },
    { title: "Forms", url: "/forms", icon: ClipboardListIcon },
    { title: "Ask", url: "/ai-insights", icon: SparklesIcon },
    { title: "SharePoint", url: "/sharepoint", icon: CloudIcon },
  ]);

  const system = ref<NavItem[]>([
    { title: "Archive", url: "/archive", icon: ArchiveIcon },
    { title: "Trash", url: "/trash", icon: Trash2Icon },
    { title: "Settings", url: "/settings", icon: SettingsIcon },
  ]);
</script>

<template>
  <Sidebar collapsible="icon" class="border-sidebar-border border-r">
    <SidebarHeader class="border-sidebar-border/60 border-b">
      <WorkspaceSwitcher />
    </SidebarHeader>

    <SidebarContent class="gap-0">
      <SidebarGroup>
        <SidebarGroupLabel
          v-if="!collapsed"
          class="text-muted-foreground/70 text-[11px] font-medium tracking-wider uppercase"
          >Workspace</SidebarGroupLabel
        >
        <SidebarGroupContent>
          <SidebarMenu>
            <SidebarMenuItem v-for="item in main" :key="item.title">
              <SidebarMenuButton
                :isActive="isActive(item.url)"
                :tooltip="item.title"
                class="data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-accent-foreground rounded-lg data-[active=true]:font-medium"
              >
                <NuxtLink :to="item.url" class="flex w-full items-center gap-2.5">
                  <component :is="item.icon" class="h-4 w-4 shrink-0" />
                  <span v-if="!collapsed" class="truncate">{{ item.title }}</span>
                  <Badge
                    v-if="!collapsed && item.badge"
                    variant="secondary"
                    class="bg-primary/10 text-primary ml-auto h-5 border-0 px-1.5 text-[10px] font-medium"
                  >
                    {{ item.badge }}
                  </Badge>
                </NuxtLink>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroupContent>
      </SidebarGroup>

      <SidebarGroup>
        <SidebarGroupLabel
          v-if="!collapsed"
          class="text-muted-foreground/70 text-[11px] font-medium tracking-wider uppercase"
          >Collaboration</SidebarGroupLabel
        >
        <SidebarGroupContent>
          <SidebarMenu>
            <SidebarMenuItem v-for="item in workspaces" :key="item.title">
              <SidebarMenuButton
                :isActive="isActive(item.url)"
                :tooltip="item.title"
                class="data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-accent-foreground rounded-lg data-[active=true]:font-medium"
              >
                <NuxtLink :to="item.url" class="flex w-full items-center gap-2.5">
                  <component :is="item.icon" class="h-4 w-4 shrink-0" />
                  <span v-if="!collapsed" class="truncate">{{ item.title }}</span>
                  <Badge
                    v-if="!collapsed && (item.title === 'Workflows' ? activeWorkflows : null)"
                    variant="secondary"
                    class="bg-primary/10 text-primary ml-auto h-5 border-0 px-1.5 text-[10px] font-medium"
                  >
                    {{ item.title === "Workflows" ? activeWorkflows : null }}
                  </Badge>
                </NuxtLink>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroupContent>
      </SidebarGroup>

      <SidebarGroup>
        <SidebarGroupLabel
          v-if="!collapsed"
          class="text-muted-foreground/70 text-[11px] font-medium tracking-wider uppercase"
          >Administration</SidebarGroupLabel
        >
        <SidebarGroupContent>
          <SidebarMenu>
            <SidebarMenuItem v-for="item in admin" :key="item.title">
              <SidebarMenuButton
                :isActive="isActive(item.url)"
                :tooltip="item.title"
                class="data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-accent-foreground rounded-lg data-[active=true]:font-medium"
              >
                <NuxtLink :to="item.url" class="flex w-full items-center gap-2.5">
                  <component :is="item.icon" class="h-4 w-4 shrink-0" />
                  <span v-if="!collapsed" class="truncate">{{ item.title }}</span>
                </NuxtLink>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroupContent>
      </SidebarGroup>

      <SidebarGroup>
        <SidebarGroupLabel
          v-if="!collapsed"
          class="text-muted-foreground/70 text-[11px] font-medium tracking-wider uppercase"
          >System</SidebarGroupLabel
        >
        <SidebarGroupContent>
          <SidebarMenu>
            <SidebarMenuItem v-for="item in system" :key="item.title">
              <SidebarMenuButton
                :isActive="isActive(item.url)"
                :tooltip="item.title"
                class="data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-accent-foreground rounded-lg data-[active=true]:font-medium"
              >
                <NuxtLink :to="item.url" class="flex w-full items-center gap-2.5">
                  <component :is="item.icon" class="h-4 w-4 shrink-0" />
                  <span v-if="!collapsed" class="truncate">{{ item.title }}</span>
                  <Badge
                    v-if="!collapsed && item.badge"
                    variant="secondary"
                    class="bg-primary/10 text-primary ml-auto h-5 border-0 px-1.5 text-[10px] font-medium"
                  >
                    {{ item.badge }}
                  </Badge>
                </NuxtLink>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroupContent>
      </SidebarGroup>
    </SidebarContent>

    <SidebarFooter class="border-sidebar-border/60 border-t">
      <div class="flex items-center gap-2.5 px-1.5 py-1.5">
        <Avatar class="h-8 w-8">
          <AvatarFallback class="bg-primary/10 text-primary text-xs font-medium">{{
            auth.user?.name?.slice(0, 2).toUpperCase() || "ED"
          }}</AvatarFallback>
        </Avatar>
        <div v-if="!collapsed" class="flex flex-col overflow-hidden leading-tight">
          <span class="truncate text-sm font-medium">{{ auth.user?.name || "Office user" }}</span>
          <span class="text-muted-foreground truncate text-[11px]">{{
            auth.user?.email || ""
          }}</span>
        </div>
      </div>
    </SidebarFooter>
  </Sidebar>
</template>
