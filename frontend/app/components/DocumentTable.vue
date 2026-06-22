<script setup lang="ts">
  import { EyeIcon, StarIcon, SparklesIcon } from "@lucide/vue";

  import { Avatar, AvatarFallback } from "@/components/ui/avatar";
  import { Badge } from "@/components/ui/badge";
  import { Button } from "@/components/ui/button";
  import { Checkbox } from "@/components/ui/checkbox";
  import type { Doc } from "@/data/mock-data";
  import { cn } from "@/lib/utils";

  const props = defineProps<{
    docs: Doc[];
    checked?: Set<string>;
  }>();

  const emit = defineEmits<{
    navigate: [doc: Doc];
    "quick-view": [doc: Doc];
    "update:checked": [value: Set<string>];
  }>();

  function toggle(id: string) {
    const n = new Set(props.checked || []);
    if (n.has(id)) n.delete(id);
    else n.add(id);
    emit("update:checked", n);
  }

  function isAI(d: Doc) {
    return !!d.aiSummary;
  }
</script>

<template>
  <div class="border-border bg-card shadow-elegant overflow-hidden rounded-2xl border">
    <div class="overflow-x-auto">
      <table class="w-full text-sm">
        <thead class="bg-secondary/50 text-muted-foreground text-xs">
          <tr>
            <th class="w-10 px-3 py-2.5"></th>
            <th class="px-2 py-2.5 text-left font-medium">Name</th>
            <th class="hidden px-3 py-2.5 text-left font-medium md:table-cell">Owner</th>
            <th class="hidden px-3 py-2.5 text-left font-medium lg:table-cell">Tags</th>
            <th class="hidden px-3 py-2.5 text-left font-medium md:table-cell">Status</th>
            <th class="hidden px-3 py-2.5 text-right font-medium sm:table-cell">Size</th>
            <th class="px-3 py-2.5 text-right font-medium">Modified</th>
            <th class="w-12 px-2 py-2.5"></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="d in docs"
            :key="d.id"
            :class="cn('border-border/70 border-t transition-colors', 'hover:bg-secondary/40')"
          >
            <td class="px-3 py-2" @click.stop>
              <Checkbox
                v-if="checked"
                :checked="checked.has(d.id)"
                @update:checked="toggle(d.id)"
                :aria-label="`Select ${d.name}`"
              />
            </td>
            <td class="px-2 py-2">
              <button
                @click="emit('navigate', d)"
                class="flex w-full min-w-0 items-center gap-2.5 text-left"
              >
                <FileTypeIcon :type="d.type" size="sm" />
                <div class="min-w-0">
                  <div class="flex items-center gap-1.5">
                    <span class="text-foreground truncate text-sm font-medium">{{ d.name }}</span>
                    <StarIcon v-if="d.starred" class="fill-warning text-warning h-3 w-3 shrink-0" />
                    <Badge
                      v-if="isAI(d)"
                      variant="secondary"
                      class="bg-primary/10 text-primary h-5 gap-1 border-0 px-1.5 text-[10px] font-medium"
                    >
                      <SparklesIcon class="h-3 w-3" /> AI
                    </Badge>
                  </div>
                  <div class="mt-0.5 flex items-center gap-1.5 lg:hidden">
                    <StatusBadges :source="d.source" :library="d.library" />
                  </div>
                </div>
              </button>
            </td>
            <td class="hidden px-3 py-2 md:table-cell">
              <div class="flex items-center gap-2">
                <Avatar class="h-6 w-6">
                  <AvatarFallback class="bg-secondary text-[10px]">{{
                    d.ownerInitials
                  }}</AvatarFallback>
                </Avatar>
                <span class="text-muted-foreground truncate text-xs">{{ d.owner }}</span>
              </div>
            </td>
            <td class="hidden px-3 py-2 lg:table-cell">
              <div class="flex flex-wrap gap-1">
                <Badge
                  v-for="t in d.tags.slice(0, 2)"
                  :key="t"
                  variant="secondary"
                  class="bg-secondary text-muted-foreground h-5 border-0 px-1.5 text-[10px] font-medium"
                  >{{ t }}</Badge
                >
              </div>
            </td>
            <td class="hidden px-3 py-2 md:table-cell">
              <StatusBadges :status="d.status" />
            </td>
            <td
              class="text-muted-foreground hidden px-3 py-2 text-right text-xs tabular-nums sm:table-cell"
            >
              {{ d.size }}
            </td>
            <td class="text-muted-foreground px-3 py-2 text-right text-xs">{{ d.modified }}</td>
            <td class="px-2 py-2" @click.stop>
              <Button
                variant="ghost"
                size="icon"
                class="h-7 w-7"
                @click="emit('quick-view', d)"
                aria-label="Quick view"
              >
                <EyeIcon class="h-3.5 w-3.5" />
              </Button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
