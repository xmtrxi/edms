<script setup lang="ts">
  import { ChevronRightIcon } from "@lucide/vue";

  import { SidebarProvider } from "@/components/ui/sidebar";

  const route = useRoute();
  const pageMeta = computed(() => route.meta as Record<string, any>);
</script>

<template>
  <SidebarProvider>
    <div class="bg-background text-foreground flex min-h-dvh w-full">
      <AppSidebar />
      <div class="flex min-w-0 flex-1 flex-col">
        <TopBar />
        <main class="min-w-0 flex-1">
          <div v-if="pageMeta.title" class="border-border/70 bg-background/60 border-b">
            <div class="px-4 pt-5 pb-4 sm:px-6 lg:px-8">
              <nav
                v-if="pageMeta.breadcrumbs?.length"
                class="text-muted-foreground mb-2 flex items-center gap-1 text-xs"
                aria-label="Breadcrumb"
              >
                <template v-for="(c, i) in pageMeta.breadcrumbs" :key="i">
                  <span class="flex items-center gap-1">
                    <NuxtLink
                      v-if="c.to"
                      :to="c.to"
                      class="hover:text-foreground transition-colors"
                      >{{ c.label }}</NuxtLink
                    >
                    <span v-else>{{ c.label }}</span>
                    <ChevronRightIcon v-if="i < pageMeta.breadcrumbs.length - 1" class="h-3 w-3" />
                  </span>
                </template>
              </nav>
              <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
                <div class="min-w-0">
                  <h1 class="text-foreground text-2xl font-semibold tracking-tight">
                    {{ pageMeta.title }}
                  </h1>
                  <p v-if="pageMeta.description" class="text-muted-foreground mt-1 text-sm">
                    {{ pageMeta.description }}
                  </p>
                </div>
                <div id="page-actions-target" class="flex shrink-0 items-center gap-2" />
              </div>
            </div>
          </div>
          <div class="px-4 py-6 sm:px-6 lg:px-8">
            <slot />
          </div>
        </main>
      </div>
    </div>
    <UploadTray />
  </SidebarProvider>
</template>
