import { ref, onMounted, onUnmounted } from "vue";

export interface Category {
  id: string;
  name: string;
  color: string;
  description?: string;
  createdAt: string;
  docIds: string[];
}

const KEY = "edms.categories.v1";

const seed: Category[] = [
  {
    id: "cat_legal",
    name: "Legal",
    color: "violet",
    description: "Contracts, NDAs, M&A, compliance.",
    createdAt: "2026-01-12",
    docIds: ["doc_1", "doc_7", "doc_19", "doc_20"],
  },
  {
    id: "cat_finance",
    name: "Finance",
    color: "emerald",
    description: "Reports, budgets, salary bands.",
    createdAt: "2026-01-12",
    docIds: ["doc_2", "doc_9", "doc_15"],
  },
  {
    id: "cat_hr",
    name: "HR & People",
    color: "rose",
    description: "Policies, handbooks, onboarding.",
    createdAt: "2026-02-04",
    docIds: ["doc_3", "doc_18"],
  },
  {
    id: "cat_engineering",
    name: "Engineering",
    color: "sky",
    description: "RFCs, architecture, postmortems.",
    createdAt: "2026-02-22",
    docIds: ["doc_4", "doc_11", "doc_17"],
  },
  {
    id: "cat_marketing",
    name: "Marketing",
    color: "amber",
    description: "GTM, brand, calendars.",
    createdAt: "2026-03-09",
    docIds: ["doc_6", "doc_10", "doc_15"],
  },
  {
    id: "cat_compliance",
    name: "Compliance",
    color: "cyan",
    description: "SOC2, GDPR, audits.",
    createdAt: "2026-04-01",
    docIds: ["doc_5", "doc_12", "doc_16"],
  },
];

function load(): Category[] {
  if (typeof window === "undefined") return seed;
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return seed;
    return JSON.parse(raw);
  } catch {
    return seed;
  }
}

function save(items: Category[]) {
  if (typeof window === "undefined") return;
  try {
    localStorage.setItem(KEY, JSON.stringify(items));
  } catch {
    /* ignore */
  }
}

type Listener = (items: Category[]) => void;
const listeners = new Set<Listener>();
let catState: Category[] | null = null;

function ensure(): Category[] {
  if (!catState) catState = load();
  return catState;
}

function emit() {
  if (catState) save(catState);
  listeners.forEach((l) => l(catState!));
}

export const categoriesStore = {
  list(): Category[] {
    return ensure();
  },
  subscribe(l: Listener) {
    listeners.add(l);
    return () => {
      listeners.delete(l);
    };
  },
  add(name: string, color: string, description?: string) {
    const items = ensure();
    const id = `cat_${name.toLowerCase().replace(/[^a-z0-9]+/g, "_")}_${Date.now().toString(36)}`;
    catState = [
      ...items,
      {
        id,
        name,
        color,
        description,
        createdAt: new Date().toISOString().slice(0, 10),
        docIds: [],
      },
    ];
    emit();
  },
  remove(id: string) {
    catState = ensure().filter((c) => c.id !== id);
    emit();
  },
  rename(id: string, name: string) {
    catState = ensure().map((c) => (c.id === id ? { ...c, name } : c));
    emit();
  },
  update(id: string, fields: Partial<Omit<Category, "id" | "docIds" | "createdAt">>) {
    catState = ensure().map((c) => (c.id === id ? { ...c, ...fields } : c));
    emit();
  },
  toggleDoc(id: string, docId: string) {
    catState = ensure().map((c) => {
      if (c.id !== id) return c;
      const has = c.docIds.includes(docId);
      return { ...c, docIds: has ? c.docIds.filter((d) => d !== docId) : [...c.docIds, docId] };
    });
    emit();
  },
};

export function useCategories() {
  const items = ref<Category[]>(categoriesStore.list());

  let unsub: (() => void) | null = null;
  onMounted(() => {
    unsub = categoriesStore.subscribe((newItems) => {
      items.value = newItems;
    });
  });
  onUnmounted(() => {
    unsub?.();
  });

  return items;
}

export const COLOR_OPTIONS = [
  { name: "violet", hex: "#8b5cf6" },
  { name: "emerald", hex: "#10b981" },
  { name: "rose", hex: "#f43f5e" },
  { name: "sky", hex: "#0ea5e9" },
  { name: "amber", hex: "#f59e0b" },
  { name: "cyan", hex: "#06b6d4" },
  { name: "fuchsia", hex: "#d946ef" },
  { name: "lime", hex: "#84cc16" },
  { name: "indigo", hex: "#6366f1" },
  { name: "orange", hex: "#f97316" },
];

export function colorStyle(color: string): Record<string, string> {
  const c = COLOR_OPTIONS.find((o) => o.name === color) ?? COLOR_OPTIONS[0];
  return {
    backgroundColor: `${c.hex}1a`,
    color: c.hex,
    borderColor: `${c.hex}40`,
  };
}
