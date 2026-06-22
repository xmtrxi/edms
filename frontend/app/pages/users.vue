<script setup lang="ts">
import {
  CheckIcon,
  SearchIcon,
  ShieldCheckIcon,
  UserPlusIcon,
  UsersIcon,
  XIcon,
  MailIcon,
  FileTextIcon,
  ChevronRightIcon,
  ClipboardListIcon,
  ActivityIcon,
} from "@lucide/vue"
import { ref, computed } from "vue"
import { toast } from "vue-sonner"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { documents, workflows, activityFeed } from "@/data/mock-data"

definePageMeta({
  title: "Users & Permissions",
  description: "Manage team members, roles, and access permissions.",
  breadcrumbs: [{ label: "Workspace", to: "/" }, { label: "Users & Permissions" }],
})

useHead({
  title: "Users & Permissions — Lumen DMS",
  meta: [
    {
      name: "description",
      content: "Manage users, roles, and permissions across your organization.",
    },
  ],
})

interface AppUser {
  id: string
  name: string
  email: string
  initials: string
  role: "Admin" | "Editor" | "Commenter" | "Viewer"
  department: string
  documents: number
  joined: string
  status: "active" | "invited" | "disabled"
}

const users = ref<AppUser[]>([
  { id: "u1", name: "Aiko Tanaka", email: "aiko@contoso.com", initials: "AT", role: "Admin", department: "Executive", documents: 284, joined: "Jan 2024", status: "active" },
  { id: "u2", name: "Marcus Bell", email: "marcus@contoso.com", initials: "MB", role: "Editor", department: "Operations", documents: 156, joined: "Mar 2024", status: "active" },
  { id: "u3", name: "Sofia Reyes", email: "sofia@contoso.com", initials: "SR", role: "Editor", department: "Legal", documents: 92, joined: "Jun 2024", status: "active" },
  { id: "u4", name: "Idris Khan", email: "idris@contoso.com", initials: "IK", role: "Admin", department: "Engineering", documents: 210, joined: "Feb 2024", status: "active" },
  { id: "u5", name: "Petra Lindqvist", email: "petra@contoso.com", initials: "PL", role: "Editor", department: "Finance", documents: 178, joined: "Apr 2024", status: "active" },
  { id: "u6", name: "Jonas Weber", email: "jonas@contoso.com", initials: "JW", role: "Viewer", department: "Executive", documents: 45, joined: "Aug 2024", status: "active" },
  { id: "u7", name: "Naledi Okafor", email: "naledi@contoso.com", initials: "NO", role: "Commenter", department: "HR", documents: 67, joined: "Sep 2024", status: "invited" },
  { id: "u8", name: "David Chen", email: "david@contoso.com", initials: "DC", role: "Viewer", department: "Marketing", documents: 23, joined: "Nov 2024", status: "disabled" },
])

const roleDefs = ref([
  {
    role: "Admin",
    label: "Admin",
    desc: "Full system access — manage users, roles, and all documents",
    color: "bg-warning/10 text-warning border-warning/20",
    perms: ["view", "download", "comment", "edit", "share", "manage_users", "delete", "settings"],
  },
  {
    role: "Editor",
    label: "Editor",
    desc: "Create, edit, and share documents",
    color: "bg-info/10 text-info border-info/20",
    perms: ["view", "download", "comment", "edit", "share"],
  },
  {
    role: "Commenter",
    label: "Commenter",
    desc: "View and comment on documents",
    color: "bg-primary/10 text-primary border-primary/20",
    perms: ["view", "download", "comment"],
  },
  {
    role: "Viewer",
    label: "Viewer",
    desc: "Read-only access to documents",
    color: "bg-secondary/80 text-muted-foreground border-border/50",
    perms: ["view", "download"],
  },
])

const permLabels: Record<string, string> = {
  view: "View documents",
  download: "Download & print",
  comment: "Add comments",
  edit: "Edit documents",
  share: "Share documents",
  manage_users: "Manage users",
  delete: "Delete documents",
  settings: "System settings",
}

const query = ref("")
const inviteOpen = ref(false)
const inviteEmail = ref("")
const inviteRole = ref("Editor")
const selectedUser = ref<AppUser | null>(null)
const userDetailOpen = ref(false)
const roleEditOpen = ref(false)
const editingRole = ref<{ role: string; perms: string[] } | null>(null)

const filteredUsers = computed(() => {
  if (!query.value.trim()) return users.value
  const q = query.value.toLowerCase()
  return users.value.filter(
    (u) =>
      u.name.toLowerCase().includes(q) ||
      u.email.toLowerCase().includes(q) ||
      u.department.toLowerCase().includes(q),
  )
})

const updateUserRole = (id: string, role: AppUser["role"]) => {
  users.value = users.value.map((u) => (u.id === id ? { ...u, role } : u))
  toast.success("User role updated")
}

const inviteUser = () => {
  if (!inviteEmail.value.includes("@")) return toast.error("Enter a valid email")
  const initials = inviteEmail.value.slice(0, 2).toUpperCase()
  users.value = [
    {
      id: `u${Date.now()}`,
      name: inviteEmail.value,
      email: inviteEmail.value,
      initials,
      role: inviteRole.value as AppUser["role"],
      department: "—",
      documents: 0,
      joined: "Just now",
      status: "invited",
    },
    ...users.value,
  ]
  inviteEmail.value = ""
  inviteOpen.value = false
  toast.success(`Invited ${inviteEmail.value} as ${inviteRole.value}`)
}

function openUserDetail(u: AppUser) {
  selectedUser.value = u
  userDetailOpen.value = true
}

function openRoleEdit(r: (typeof roleDefs.value)[number]) {
  editingRole.value = { role: r.role, perms: [...r.perms] }
  roleEditOpen.value = true
}

function saveRolePerms() {
  if (!editingRole.value) return
  const idx = roleDefs.value.findIndex((r) => r.role === editingRole.value!.role)
  if (idx > -1) {
    roleDefs.value[idx] = { ...roleDefs.value[idx], perms: [...editingRole.value.perms] }
  }
  roleEditOpen.value = false
  editingRole.value = null
  toast.success("Permissions updated")
}

function togglePerm(perm: string) {
  if (!editingRole.value) return
  const idx = editingRole.value.perms.indexOf(perm)
  if (idx > -1) {
    editingRole.value.perms.splice(idx, 1)
  } else {
    editingRole.value.perms.push(perm)
  }
}

function userDocuments(name: string) {
  return documents.filter((d) => d.owner === name)
}

function userWorkflows(name: string) {
  return workflows.filter((w) => w.assignees.some((a) => w.assignees[0]?.name.startsWith(name.split(" ")[0])))
}

function userActivity(name: string) {
  return activityFeed.filter((a) => a.who === name)
}
</script>

<template>
  <Teleport to="#page-actions-target">
    <Button size="sm" class="gap-1.5" @click="inviteOpen = true">
      <UserPlusIcon class="h-4 w-4" /> Invite user
    </Button>
  </Teleport>

  <Tabs default-value="users" class="w-full">
    <TabsList class="bg-secondary/60">
      <TabsTrigger value="users" class="gap-1.5"><UsersIcon class="h-3.5 w-3.5" /> Users</TabsTrigger>
      <TabsTrigger value="roles" class="gap-1.5"><ShieldCheckIcon class="h-3.5 w-3.5" /> Roles &amp; permissions</TabsTrigger>
    </TabsList>

    <TabsContent value="users" class="mt-4 space-y-4">
      <div class="relative max-w-md">
        <SearchIcon class="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2" />
        <Input v-model="query" placeholder="Search users…" class="bg-secondary/60 focus-visible:bg-card focus-visible:border-border h-9 rounded-lg border-transparent pl-9" />
        <button v-if="query" @click="query = ''" class="hover:bg-secondary text-muted-foreground hover:text-foreground absolute top-1/2 right-2 grid h-5 w-5 -translate-y-1/2 place-items-center rounded"><XIcon class="h-3 w-3" /></button>
      </div>

      <div class="border-border bg-card overflow-hidden rounded-xl border">
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="bg-secondary/50 text-muted-foreground text-xs">
              <tr>
                <th class="px-4 py-3 text-left font-medium">User</th>
                <th class="hidden px-4 py-3 text-left font-medium sm:table-cell">Department</th>
                <th class="hidden px-4 py-3 text-left font-medium md:table-cell">Documents</th>
                <th class="hidden px-4 py-3 text-left font-medium md:table-cell">Joined</th>
                <th class="px-4 py-3 text-left font-medium">Role</th>
                <th class="px-4 py-3 text-left font-medium">Status</th>
                <th class="px-4 py-3 text-left font-medium" />
              </tr>
            </thead>
            <tbody>
              <tr v-for="u in filteredUsers" :key="u.id" class="border-border/70 hover:bg-secondary/40 border-t transition-colors cursor-pointer" @click="openUserDetail(u)">
                <td class="px-4 py-3">
                  <div class="flex items-center gap-2.5">
                    <Avatar class="h-8 w-8"><AvatarFallback class="bg-secondary text-[10px]">{{ u.initials }}</AvatarFallback></Avatar>
                    <div class="min-w-0">
                      <p class="truncate text-sm font-medium">{{ u.name }}</p>
                      <p class="text-muted-foreground truncate text-[11px]">{{ u.email }}</p>
                    </div>
                  </div>
                </td>
                <td class="text-muted-foreground hidden px-4 py-3 text-xs sm:table-cell">{{ u.department }}</td>
                <td class="text-muted-foreground hidden px-4 py-3 text-xs md:table-cell"><span class="tabular-nums">{{ u.documents }}</span></td>
                <td class="text-muted-foreground hidden px-4 py-3 text-xs md:table-cell">{{ u.joined }}</td>
                <td class="px-4 py-3" @click.stop>
                  <Select :model-value="u.role" @update:model-value="(v: string) => updateUserRole(u.id, v as AppUser['role'])">
                    <SelectTrigger class="h-7 w-28 text-xs"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="Admin">Admin</SelectItem>
                      <SelectItem value="Editor">Editor</SelectItem>
                      <SelectItem value="Commenter">Commenter</SelectItem>
                      <SelectItem value="Viewer">Viewer</SelectItem>
                    </SelectContent>
                  </Select>
                </td>
                <td class="px-4 py-3">
                  <Badge variant="secondary" :class="`border-0 text-[10px] ${u.status === 'active' ? 'bg-success/10 text-success' : u.status === 'invited' ? 'bg-info/10 text-info' : 'bg-secondary/60 text-muted-foreground'}`">{{ u.status }}</Badge>
                </td>
                <td class="px-4 py-3" @click.stop>
                  <ChevronRightIcon class="h-4 w-4 text-muted-foreground/50" />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="filteredUsers.length === 0" class="text-muted-foreground p-8 text-center text-sm">No users match your search.</div>
      </div>
    </TabsContent>

    <TabsContent value="roles" class="mt-4 space-y-4">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Card v-for="r in roleDefs" :key="r.role" class="border-border shadow-sm hover:shadow-md transition-all">
          <CardContent class="p-4">
            <div class="flex items-center justify-between">
              <Badge variant="secondary" :class="`border text-[10px] ${r.color}`">{{ r.role }}</Badge>
              <Button variant="ghost" size="sm" class="h-6 px-2 text-[11px] gap-1" @click="openRoleEdit(r)">
                Edit
              </Button>
            </div>
            <p class="text-muted-foreground mt-2 text-xs">{{ r.desc }}</p>
            <Separator class="my-3" />
            <div class="space-y-1.5 text-xs">
              <div v-for="perm in ['view', 'download', 'comment', 'edit', 'share', 'manage_users', 'delete', 'settings']" :key="perm" class="flex items-center gap-1.5">
                <div v-if="r.perms.includes(perm)" class="text-success"><CheckIcon class="h-3 w-3" /></div>
                <div v-else class="text-muted-foreground/30"><XIcon class="h-3 w-3" /></div>
                {{ permLabels[perm] }}
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card class="border-border shadow-sm">
        <CardContent class="p-5">
          <div class="flex items-center justify-between mb-3">
            <h3 class="flex items-center gap-1.5 text-sm font-semibold"><ShieldCheckIcon class="text-primary h-4 w-4" /> Permission matrix</h3>
            <Button variant="outline" size="sm" class="h-7 text-xs gap-1" @click="() => { const r = roleDefs[0]; editingRole = { role: r.role, perms: [...r.perms] }; roleEditOpen = true }">
              Edit all
            </Button>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full text-xs">
              <thead>
                <tr class="border-border border-b">
                  <th class="text-muted-foreground py-1.5 pr-3 text-left font-medium">Capability</th>
                  <th v-for="r in roleDefs" :key="r.role" class="px-2 py-1.5 text-center font-medium">
                    <Badge variant="secondary" :class="`border text-[10px] ${r.color}`">{{ r.role }}</Badge>
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(label, key) in permLabels" :key="key" class="border-border/50 border-b last:border-0">
                  <td class="text-muted-foreground py-2 pr-3">{{ label }}</td>
                  <td v-for="r in roleDefs" :key="r.role" class="px-2 py-2 text-center">
                    <button
                      @click="openRoleEdit(r)"
                      :class="r.perms.includes(key) ? 'bg-success/15 text-success hover:bg-success/25' : 'bg-secondary/40 text-muted-foreground/40 hover:bg-secondary/70'"
                      class="inline-flex h-5 w-5 items-center justify-center rounded transition-colors"
                    >
                      <CheckIcon v-if="r.perms.includes(key)" class="h-3 w-3" />
                      <XIcon v-else class="h-3 w-3" />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </TabsContent>
  </Tabs>

  <!-- Invite dialog -->
  <Dialog :open="inviteOpen" @update:open="inviteOpen = $event">
    <DialogContent class="max-w-md">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2"><UserPlusIcon class="text-primary h-4 w-4" /> Invite user</DialogTitle>
        <DialogDescription>Send an invitation to join your organization.</DialogDescription>
      </DialogHeader>
      <div class="space-y-3">
        <div class="space-y-1.5">
          <label class="text-xs font-medium">Email address</label>
          <div class="relative">
            <MailIcon class="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2" />
            <Input v-model="inviteEmail" placeholder="colleague@company.com" class="pl-9" @keydown.enter="inviteUser" />
          </div>
        </div>
        <div class="space-y-1.5">
          <label class="text-xs font-medium">Role</label>
          <Select v-model="inviteRole">
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="Admin">Admin</SelectItem>
              <SelectItem value="Editor">Editor</SelectItem>
              <SelectItem value="Commenter">Commenter</SelectItem>
              <SelectItem value="Viewer">Viewer</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      <DialogFooter>
        <Button variant="ghost" @click="inviteOpen = false">Cancel</Button>
        <Button @click="inviteUser" class="gap-1.5"><MailIcon class="h-3.5 w-3.5" /> Send invitation</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <!-- User detail panel -->
  <Dialog :open="userDetailOpen" @update:open="userDetailOpen = $event">
    <DialogContent class="max-w-lg">
      <DialogHeader v-if="selectedUser">
        <div class="flex items-center gap-3">
          <Avatar class="h-10 w-10"><AvatarFallback class="bg-secondary text-xs">{{ selectedUser.initials }}</AvatarFallback></Avatar>
          <div>
            <DialogTitle class="text-base">{{ selectedUser.name }}</DialogTitle>
            <p class="text-xs text-muted-foreground">{{ selectedUser.email }} · {{ selectedUser.department }}</p>
          </div>
        </div>
      </DialogHeader>
      <div v-if="selectedUser" class="space-y-4">
        <div class="flex items-center gap-3">
          <Badge variant="secondary" class="text-[10px] border-0" :class="selectedUser.status === 'active' ? 'bg-success/10 text-success' : selectedUser.status === 'invited' ? 'bg-info/10 text-info' : 'bg-secondary/60 text-muted-foreground'">{{ selectedUser.status }}</Badge>
          <span class="text-xs text-muted-foreground">Joined {{ selectedUser.joined }}</span>
          <span class="text-xs text-muted-foreground">{{ selectedUser.documents }} documents</span>
        </div>

        <div>
          <h4 class="text-xs font-semibold flex items-center gap-1.5 mb-2"><ClipboardListIcon class="h-3.5 w-3.5 text-primary" /> Active tasks</h4>
          <div v-if="userWorkflows(selectedUser.name).length === 0" class="text-xs text-muted-foreground/60 py-2">No active tasks</div>
          <div v-for="w in userWorkflows(selectedUser.name).filter(w => w.status !== 'approved' && w.status !== 'rejected')" :key="w.id" class="flex items-center gap-2 py-1.5 border-b border-border/30 last:border-0">
            <div class="h-1.5 w-1.5 rounded-full" :class="w.priority === 'high' ? 'bg-destructive' : w.priority === 'medium' ? 'bg-warning' : 'bg-muted-foreground/40'" />
            <div class="flex-1 min-w-0">
              <p class="text-xs font-medium truncate">{{ w.title }}</p>
              <p class="text-[10px] text-muted-foreground">{{ w.doc }} · {{ w.due }}</p>
            </div>
            <Badge variant="secondary" class="text-[10px] border-0 capitalize">{{ w.status.replace('_', ' ') }}</Badge>
          </div>
        </div>

        <div>
          <h4 class="text-xs font-semibold flex items-center gap-1.5 mb-2"><FileTextIcon class="h-3.5 w-3.5 text-primary" /> Recent documents</h4>
          <div v-for="d in userDocuments(selectedUser.name).slice(0, 4)" :key="d.id" class="flex items-center gap-2 py-1.5 border-b border-border/30 last:border-0">
            <FileTextIcon class="h-3 w-3 text-muted-foreground shrink-0" />
            <span class="text-xs truncate flex-1">{{ d.name }}</span>
            <span class="text-[10px] text-muted-foreground">{{ d.modified }}</span>
          </div>
          <p v-if="userDocuments(selectedUser.name).length > 4" class="text-[10px] text-muted-foreground mt-1">+{{ userDocuments(selectedUser.name).length - 4 }} more</p>
          <p v-else-if="userDocuments(selectedUser.name).length === 0" class="text-xs text-muted-foreground/60 py-2">No documents</p>
        </div>

        <div>
          <h4 class="text-xs font-semibold flex items-center gap-1.5 mb-2"><ActivityIcon class="h-3.5 w-3.5 text-primary" /> Recent activity</h4>
          <div v-for="a in userActivity(selectedUser.name).slice(0, 3)" :key="a.id" class="flex items-center gap-2 py-1.5 text-xs border-b border-border/30 last:border-0">
            <div class="h-1.5 w-1.5 rounded-full bg-primary/40" />
            <span class="text-muted-foreground"><span class="font-medium text-foreground/80">{{ a.action }}</span> {{ a.target }}</span>
            <span class="text-[10px] text-muted-foreground ml-auto shrink-0">{{ a.when }}</span>
          </div>
          <p v-if="userActivity(selectedUser.name).length === 0" class="text-xs text-muted-foreground/60 py-2">No recent activity</p>
        </div>
      </div>
    </DialogContent>
  </Dialog>

  <!-- Role edit dialog -->
  <Dialog :open="roleEditOpen" @update:open="roleEditOpen = $event">
    <DialogContent class="max-w-sm">
      <DialogHeader v-if="editingRole">
        <DialogTitle class="flex items-center gap-2 text-base"><ShieldCheckIcon class="h-4 w-4 text-primary" /> Edit {{ editingRole.role }} permissions</DialogTitle>
        <DialogDescription>Toggle permissions for this role.</DialogDescription>
      </DialogHeader>
      <div v-if="editingRole" class="space-y-1 pt-2">
        <label v-for="(label, key) in permLabels" :key="key" class="flex items-center gap-3 rounded-lg px-3 py-2.5 hover:bg-secondary/40 transition-colors cursor-pointer">
          <input type="checkbox" :checked="editingRole.perms.includes(key)" @change="togglePerm(key)" class="accent-primary h-4 w-4 rounded" />
          <span class="text-xs">{{ label }}</span>
        </label>
      </div>
      <DialogFooter>
        <Button variant="ghost" @click="roleEditOpen = false">Cancel</Button>
        <Button @click="saveRolePerms"><CheckIcon class="h-3.5 w-3.5 mr-1.5" /> Save</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
