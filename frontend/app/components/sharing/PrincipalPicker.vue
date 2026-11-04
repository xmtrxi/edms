<script setup lang="ts">
  import {
    AtSignIcon,
    Building2Icon,
    GlobeIcon,
    SearchIcon,
    ShieldIcon,
    UserIcon,
    UsersIcon,
  } from "@lucide/vue";
  import { useDebounceFn } from "@vueuse/core";
  import { computed, shallowRef, useId, watch } from "vue";

  import { Input } from "@/components/ui/input";
  import type { PrincipalOption, PrincipalType } from "@/types/api";

  const props = withDefaults(
    defineProps<{
      placeholder?: string;
      /** Offer "Invite <email>" when the query is an email address. */
      allowEmail?: boolean;
      /** Principal types to leave out, e.g. broad ones for restricted files. */
      exclude?: PrincipalType[];
      /** Already chosen principals, shown as disabled. */
      selectedKeys?: string[];
    }>(),
    { placeholder: "Add people, departments, groups or roles", allowEmail: false },
  );
  const emit = defineEmits<{
    select: [option: PrincipalOption];
    email: [email: string];
  }>();

  const sharing = sharingService();
  const listId = useId();
  const query = shallowRef("");
  const open = shallowRef(false);
  const loading = shallowRef(false);
  const options = shallowRef<PrincipalOption[]>([]);
  const active = shallowRef(0);

  const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  const typedEmail = computed(() =>
    props.allowEmail && emailPattern.test(query.value.trim()) ? query.value.trim() : "",
  );

  const visible = computed(() =>
    options.value.filter((option) => !props.exclude?.includes(option.type)),
  );

  // The email row goes first when no member matches the typed address.
  const rows = computed(() => {
    const items: Array<{ kind: "principal"; option: PrincipalOption } | { kind: "email" }> =
      visible.value.map((option) => ({ kind: "principal" as const, option }));
    const memberMatches = visible.value.some(
      (option) => option.description?.toLowerCase() === typedEmail.value.toLowerCase(),
    );
    if (typedEmail.value && !memberMatches) items.unshift({ kind: "email" });
    return items;
  });

  const load = useDebounceFn(async (value: string) => {
    loading.value = true;
    options.value = await sharing.searchPrincipals(value);
    loading.value = false;
    active.value = 0;
  }, 200);

  watch(query, (value) => {
    open.value = true;
    void load(value);
  });

  const onFocus = () => {
    open.value = true;
    if (!options.value.length) void load(query.value);
  };

  const keyOf = (option: PrincipalOption) => `${option.type}|${option.id}`;
  const isSelected = (option: PrincipalOption) => props.selectedKeys?.includes(keyOf(option));

  const choose = (index: number) => {
    const row = rows.value[index];
    if (!row) return;
    if (row.kind === "email") emit("email", typedEmail.value);
    else if (!isSelected(row.option)) emit("select", row.option);
    else return;
    query.value = "";
    open.value = false;
  };

  const onKeydown = (event: KeyboardEvent) => {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      open.value = true;
      active.value = Math.min(active.value + 1, rows.value.length - 1);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      active.value = Math.max(active.value - 1, 0);
    } else if (event.key === "Enter") {
      event.preventDefault();
      choose(active.value);
    } else if (event.key === "Escape") {
      open.value = false;
    }
  };

  const iconFor: Record<PrincipalType, typeof UserIcon> = {
    user: UserIcon,
    department: Building2Icon,
    group: UsersIcon,
    role: ShieldIcon,
    everyone: GlobeIcon,
  };
</script>

<template>
  <div class="relative">
    <SearchIcon
      class="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
    />
    <Input
      v-model="query"
      class="pl-9"
      role="combobox"
      autocomplete="off"
      :placeholder="placeholder"
      :aria-expanded="open"
      :aria-controls="listId"
      :aria-activedescendant="open && rows.length ? `${listId}-${active}` : undefined"
      @focus="onFocus"
      @blur="open = false"
      @keydown="onKeydown"
    />
    <ul
      v-show="open && (rows.length || loading || query)"
      :id="listId"
      role="listbox"
      class="bg-popover text-popover-foreground absolute z-50 mt-1 max-h-72 w-full overflow-y-auto rounded-lg border p-1 shadow-lg"
    >
      <li v-if="loading && !rows.length" class="text-muted-foreground px-3 py-2 text-sm">
        Searching…
      </li>
      <li
        v-else-if="!rows.length"
        class="text-muted-foreground px-3 py-2 text-sm"
        role="presentation"
      >
        {{ allowEmail ? "No match. Type a full email address to invite someone." : "No match." }}
      </li>
      <li
        v-for="(row, index) in rows"
        :id="`${listId}-${index}`"
        :key="row.kind === 'email' ? 'email' : keyOf(row.option)"
        role="option"
        :aria-selected="index === active"
        :aria-disabled="row.kind === 'principal' && isSelected(row.option)"
        class="flex cursor-pointer items-center gap-3 rounded-md px-2.5 py-2 text-sm aria-disabled:cursor-default aria-disabled:opacity-50"
        :class="index === active ? 'bg-accent text-accent-foreground' : ''"
        @mousedown.prevent="choose(index)"
        @mouseenter="active = index"
      >
        <template v-if="row.kind === 'email'">
          <span class="bg-muted grid size-8 shrink-0 place-items-center rounded-full">
            <AtSignIcon class="size-4" />
          </span>
          <span class="min-w-0 flex-1">
            <span class="block truncate font-medium">Invite {{ typedEmail }}</span>
            <span class="text-muted-foreground block truncate text-xs"
              >Shares with this email address</span
            >
          </span>
        </template>
        <template v-else>
          <span class="bg-muted grid size-8 shrink-0 place-items-center rounded-full">
            <component :is="iconFor[row.option.type]" class="size-4" />
          </span>
          <span class="min-w-0 flex-1">
            <span class="block truncate font-medium">{{ row.option.name }}</span>
            <span class="text-muted-foreground block truncate text-xs">{{
              row.option.description ?? (row.option.type === "everyone" ? "All members" : "")
            }}</span>
          </span>
          <span v-if="isSelected(row.option)" class="text-muted-foreground text-xs">Added</span>
        </template>
      </li>
    </ul>
  </div>
</template>
