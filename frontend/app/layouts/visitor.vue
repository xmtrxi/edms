<template>
  <div class="visitor-shell min-h-dvh">
    <header class="mx-auto flex w-full max-w-6xl items-center justify-between px-5 py-6 sm:px-8">
      <NuxtLink to="/visit" class="flex items-center gap-3" aria-label="EDMS visitor desk">
        <span
          class="bg-primary text-primary-foreground grid size-9 place-items-center rounded-md font-mono text-xs font-semibold"
          >ED</span
        >
        <span>
          <span class="block text-sm leading-none font-semibold">Visitor desk</span>
          <span class="text-muted-foreground mt-1 block text-xs"
            >Office records &amp; requests</span
          >
        </span>
      </NuxtLink>
      <span class="text-muted-foreground hidden text-xs sm:block">Secure, temporary access</span>
    </header>
    <main class="mx-auto w-full max-w-6xl px-5 pb-12 sm:px-8">
      <slot />
    </main>
  </div>
</template>
