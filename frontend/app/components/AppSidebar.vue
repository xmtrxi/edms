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
  } from "@lucide/vue";
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
  import { workflows, insights } from "@/data/mock-data";
  import { useOfficeState } from "@/lib/office-store";

  const route = useRoute();
  const { state } = useSidebar();
  const collapsed = computed(() => state.value === "collapsed");
  const isActive = (url: string) => (url === "/" ? route.path === "/" : route.path.startsWith(url));
  const office = useOfficeState();

  const pendingSlips = computed(
    () =>
      office.value.slips.filter((s) => s.status === "pending" || s.status === "in_review").length,
  );
  const activeWorkflows = computed(
    () => workflows.filter((w) => w.status === "pending" || w.status === "in_review").length,
  );
  const totalInsights = computed(() => insights.length);

  interface NavItem {
    title: string;
    url: string;
    icon: typeof LayoutDashboardIcon;
    badge?: string | number;
  }

  const main = ref<NavItem[]>([
    { title: "Dashboard", url: "/", icon: LayoutDashboardIcon },
    { title: "My Documents", url: "/documents", icon: FileTextIcon },
    { title: "Shared with me", url: "/shared", icon: Share2Icon },
    { title: "Favorites", url: "/favorites", icon: StarIcon },
    { title: "Recent", url: "/recent", icon: ClockIcon },
  ]);

  const admin = ref<NavItem[]>([
    { title: "Folders", url: "/admin/folders", icon: FolderOpenIcon },
    { title: "Access control", url: "/admin/access", icon: ShieldCheckIcon },
    { title: "Users & Permissions", url: "/users", icon: ShieldCheckIcon },
    { title: "Audit Log", url: "/audit", icon: HistoryIcon },
  ]);

  const workspaces = ref<NavItem[]>([
    { title: "Teams", url: "/teams", icon: UsersIcon },
    { title: "Office Desk", url: "/office", icon: Building2Icon },
    { title: "Categories", url: "/categories", icon: TagIcon },
    { title: "Workflows", url: "/workflows", icon: WorkflowIcon },
    { title: "Forms", url: "/forms", icon: ClipboardListIcon },
    { title: "AI Insights", url: "/ai-insights", icon: SparklesIcon },
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
      <div class="flex items-center gap-2.5 px-1.5 py-1">
        <div
          class="from-primary to-primary/60 text-primary-foreground shadow-glow grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-gradient-to-br"
        >
          <FolderOpenIcon class="h-4 w-4" />
        </div>
        <div v-if="!collapsed" class="flex flex-col leading-tight">
          <span class="text-sm font-semibold tracking-tight">Lumen DMS</span>
          <span class="text-muted-foreground text-[11px]">Contoso · Enterprise</span>
        </div>
      </div>
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
                    v-if="
                      !collapsed &&
                      (item.title === 'Office Desk'
                        ? pendingSlips
                        : item.title === 'Workflows'
                          ? activeWorkflows
                          : item.title === 'AI Insights'
                            ? totalInsights
                            : null)
                    "
                    variant="secondary"
                    class="bg-primary/10 text-primary ml-auto h-5 border-0 px-1.5 text-[10px] font-medium"
                  >
                    {{
                      item.title === "Office Desk"
                        ? pendingSlips
                        : item.title === "Workflows"
                          ? activeWorkflows
                          : totalInsights
                    }}
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
          <AvatarFallback class="bg-primary/10 text-primary text-xs font-medium">AT</AvatarFallback>
        </Avatar>
        <div v-if="!collapsed" class="flex flex-col overflow-hidden leading-tight">
          <span class="truncate text-sm font-medium">Aiko Tanaka</span>
          <span class="text-muted-foreground truncate text-[11px]">aiko@contoso.com</span>
        </div>
      </div>
    </SidebarFooter>
  </Sidebar>
</template>
