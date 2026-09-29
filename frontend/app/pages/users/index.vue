<script setup lang="ts">
  import { MailIcon, UserPlusIcon, UsersIcon } from "@lucide/vue";
  import { computed, onMounted, ref } from "vue";
  import { toast } from "vue-sonner";

  import { initialsOf } from "@/utils/format";

  definePageMeta({
    title: "People & access",
    description: "Manage staff accounts and invitations.",
    breadcrumbs: [{ label: "Administration", to: "/" }, { label: "People" }],
  });
  const admin = administrationService();
  const query = ref("");
  const inviteOpen = ref(false);
  const name = ref("");
  const email = ref("");
  const filtered = computed(() =>
    admin.users.value.filter((item) =>
      `${item.name} ${item.email}`.toLowerCase().includes(query.value.toLowerCase()),
    ),
  );
  const invite = async () => {
    const link = await admin.invite(name.value, email.value);
    if (link) {
      toast.success("Invitation created");
      inviteOpen.value = false;
    }
  };
  onMounted(admin.load);
</script>
<template>
  <Teleport to="#page-actions-target"
    ><Button size="sm" class="gap-2" @click="inviteOpen = true"
      ><UserPlusIcon class="size-4" />Invite staff</Button
    ></Teleport
  >
  <div class="space-y-4">
    <div class="bg-card flex items-center gap-3 rounded-xl border p-3">
      <UsersIcon class="text-muted-foreground size-4" /><Input
        v-model="query"
        class="border-0 shadow-none"
        placeholder="Find staff by name or email…"
      /><Badge variant="secondary">{{ filtered.length }} people</Badge>
    </div>
    <div v-if="admin.error.value" class="bg-destructive/5 text-destructive rounded-xl p-3 text-sm">
      {{ admin.error.value }}
    </div>
    <div class="bg-card overflow-hidden rounded-2xl border shadow-sm">
      <div
        v-for="item in filtered"
        :key="item.id"
        class="flex items-center gap-4 border-b p-4 last:border-0"
      >
        <Avatar
          ><AvatarFallback>{{ initialsOf(item.name) }}</AvatarFallback></Avatar
        >
        <div class="min-w-0 flex-1">
          <p class="text-sm font-semibold">{{ item.name }}</p>
          <p class="text-muted-foreground truncate text-xs">{{ item.email }}</p>
        </div>
        <Badge
          :variant="item.status === 'active' ? 'secondary' : 'destructive'"
          class="capitalize"
          >{{ item.status }}</Badge
        >
      </div>
    </div>
  </div>
  <Dialog v-model:open="inviteOpen"
    ><DialogContent
      ><DialogHeader
        ><DialogTitle>Invite staff member</DialogTitle
        ><DialogDescription
          >They will receive a secure account setup link.</DialogDescription
        ></DialogHeader
      >
      <div class="space-y-3">
        <Input v-model="name" placeholder="Full name" />
        <div class="relative">
          <MailIcon class="text-muted-foreground absolute top-3 left-3 size-4" /><Input
            v-model="email"
            type="email"
            class="pl-9"
            placeholder="name@office.org"
          />
        </div>
      </div>
      <DialogFooter
        ><Button variant="outline" @click="inviteOpen = false">Cancel</Button
        ><Button :disabled="!name.trim() || !email.includes('@')" @click="invite"
          >Send invitation</Button
        ></DialogFooter
      ></DialogContent
    ></Dialog
  >
</template>
