import { ref, readonly, type Ref } from "vue";

export type StampKind = "approved" | "reviewed" | "received" | "confidential" | "urgent" | "filed";
export type SignatureKind = "typed" | "drawn" | "initials";

export interface Comment {
  id: string;
  docId: string;
  author: string;
  initials: string;
  text: string;
  createdAt: number;
  resolved?: boolean;
  parentId?: string;
  mentions?: string[];
}

export interface Signature {
  id: string;
  docId: string;
  signer: string;
  initials: string;
  kind: SignatureKind;
  value: string;
  font?: string;
  role?: string;
  signedAt: number;
}

export interface Stamp {
  id: string;
  docId: string;
  kind: StampKind;
  by: string;
  at: number;
  note?: string;
}

export type RouteStepStatus =
  | "pending"
  | "active"
  | "approved"
  | "rejected"
  | "signed"
  | "forwarded";
export interface RouteStep {
  id: string;
  recipient: string;
  initials: string;
  role: string;
  action: "review" | "sign" | "approve" | "file" | "acknowledge";
  status: RouteStepStatus;
  note?: string;
  actedAt?: number;
}

export interface RoutingSlip {
  id: string;
  docId: string;
  docName: string;
  subject: string;
  origin: string;
  originInitials: string;
  priority: "low" | "normal" | "high" | "urgent";
  dueDate?: string;
  createdAt: number;
  steps: RouteStep[];
  status: "in_progress" | "completed" | "returned";
  notes?: string;
}

interface State {
  comments: Comment[];
  signatures: Signature[];
  stamps: Stamp[];
  slips: RoutingSlip[];
}

const KEY = "lumen.office.v1";
const CURRENT_USER = { name: "Aiko Tanaka", initials: "AT", role: "Executive Secretary" };

const SEED: State = {
  comments: [
    {
      id: "c1",
      docId: "doc_1",
      author: "Sofia Reyes",
      initials: "SR",
      text: "Please review section 4 — liability cap looks off.",
      createdAt: Date.parse("2026-05-28T09:00:00Z"),
    },
    {
      id: "c2",
      docId: "doc_1",
      author: "Marcus Bell",
      initials: "MB",
      text: "Agree, also clause 12.4 needs flagging to Legal.",
      createdAt: Date.parse("2026-05-28T10:14:00Z"),
      parentId: "c1",
    },
    {
      id: "c3",
      docId: "doc_3",
      author: "Naledi Okafor",
      initials: "NO",
      text: "Handbook reads well — ready for final sign-off.",
      createdAt: Date.parse("2026-05-29T08:30:00Z"),
      resolved: true,
    },
  ],
  signatures: [
    {
      id: "s1",
      docId: "doc_4",
      signer: "Idris Khan",
      initials: "IK",
      kind: "typed",
      value: "Idris Khan",
      font: "Great Vibes",
      role: "CTO",
      signedAt: Date.parse("2026-05-27T15:00:00Z"),
    },
  ],
  stamps: [
    {
      id: "st1",
      docId: "doc_4",
      kind: "reviewed",
      by: "Petra Lindqvist",
      at: Date.parse("2026-05-27T14:00:00Z"),
    },
  ],
  slips: [
    {
      id: "rs1",
      docId: "doc_1",
      docName: "MSA_Acme_Corp_v4.pdf",
      subject: "Acme MSA — for legal review and director sign-off",
      origin: "Secretary — Aiko Tanaka",
      originInitials: "AT",
      priority: "high",
      dueDate: "2026-06-02",
      createdAt: Date.parse("2026-05-29T08:00:00Z"),
      status: "in_progress",
      notes: "Please initial each page and return by Friday.",
      steps: [
        {
          id: "st1",
          recipient: "Sofia Reyes",
          initials: "SR",
          role: "Legal Counsel",
          action: "review",
          status: "approved",
          note: "Cleared with minor edits.",
          actedAt: Date.parse("2026-05-29T09:30:00Z"),
        },
        {
          id: "st2",
          recipient: "Marcus Bell",
          initials: "MB",
          role: "Director, Operations",
          action: "approve",
          status: "active",
        },
        {
          id: "st3",
          recipient: "Idris Khan",
          initials: "IK",
          role: "CTO",
          action: "sign",
          status: "pending",
        },
        {
          id: "st4",
          recipient: "Records Office",
          initials: "RO",
          role: "Filing",
          action: "file",
          status: "pending",
        },
      ],
    },
    {
      id: "rs2",
      docId: "doc_19",
      docName: "Acquisition_LOI.pdf",
      subject: "LOI — urgent counter-signature",
      origin: "Secretary — Aiko Tanaka",
      originInitials: "AT",
      priority: "urgent",
      dueDate: "2026-05-30",
      createdAt: Date.parse("2026-05-29T11:00:00Z"),
      status: "in_progress",
      steps: [
        {
          id: "st1",
          recipient: "Petra Lindqvist",
          initials: "PL",
          role: "CFO",
          action: "sign",
          status: "active",
        },
        {
          id: "st2",
          recipient: "Jonas Weber",
          initials: "JW",
          role: "CEO",
          action: "sign",
          status: "pending",
        },
      ],
    },
  ],
};

function load(): State {
  if (typeof window === "undefined") return SEED;
  try {
    const raw = window.localStorage.getItem(KEY);
    if (!raw) return SEED;
    return JSON.parse(raw) as State;
  } catch {
    return SEED;
  }
}

const state: Ref<State> = ref(load());

function persist() {
  if (typeof window !== "undefined") {
    try {
      window.localStorage.setItem(KEY, JSON.stringify(state.value));
    } catch {
      /* ignore */
    }
  }
}

export function currentUser() {
  return CURRENT_USER;
}

export function useOfficeState() {
  return readonly(state);
}

const id = () => Math.random().toString(36).slice(2, 10);

export function addComment(docId: string, text: string, parentId?: string) {
  if (!text.trim()) return;
  state.value = {
    ...state.value,
    comments: [
      ...state.value.comments,
      {
        id: id(),
        docId,
        author: CURRENT_USER.name,
        initials: CURRENT_USER.initials,
        text: text.trim(),
        createdAt: Date.now(),
        parentId,
        mentions: Array.from(text.matchAll(/@([A-Za-z]+)/g)).map((m) => m[1]),
      },
    ],
  };
  persist();
}

export function resolveComment(commentId: string) {
  state.value = {
    ...state.value,
    comments: state.value.comments.map((c) =>
      c.id === commentId ? { ...c, resolved: !c.resolved } : c,
    ),
  };
  persist();
}

export function deleteComment(commentId: string) {
  state.value = {
    ...state.value,
    comments: state.value.comments.filter((c) => c.id !== commentId && c.parentId !== commentId),
  };
  persist();
}

export function addSignature(s: Omit<Signature, "id" | "signedAt">) {
  state.value = {
    ...state.value,
    signatures: [...state.value.signatures, { ...s, id: id(), signedAt: Date.now() }],
  };
  persist();
}

export function addStamp(docId: string, kind: StampKind, note?: string) {
  state.value = {
    ...state.value,
    stamps: [
      ...state.value.stamps,
      { id: id(), docId, kind, by: CURRENT_USER.name, at: Date.now(), note },
    ],
  };
  persist();
}

export function createRoutingSlip(
  slip: Omit<RoutingSlip, "id" | "createdAt" | "status" | "origin" | "originInitials"> & {
    origin?: string;
  },
) {
  const newSlip: RoutingSlip = {
    ...slip,
    id: id(),
    createdAt: Date.now(),
    status: "in_progress",
    origin: slip.origin ?? `Secretary — ${CURRENT_USER.name}`,
    originInitials: CURRENT_USER.initials,
    steps: slip.steps.map((s, i) => ({ ...s, status: i === 0 ? "active" : "pending" })),
  };
  state.value = { ...state.value, slips: [newSlip, ...state.value.slips] };
  persist();
}

export function advanceStep(
  slipId: string,
  stepId: string,
  status: RouteStepStatus,
  note?: string,
) {
  state.value = {
    ...state.value,
    slips: state.value.slips.map((slip) => {
      if (slip.id !== slipId) return slip;
      let activated = false;
      const steps = slip.steps.map((s, i, arr) => {
        if (s.id === stepId) return { ...s, status, note: note ?? s.note, actedAt: Date.now() };
        if (
          !activated &&
          i > 0 &&
          arr[i - 1].id === stepId &&
          (status === "approved" || status === "signed" || status === "forwarded") &&
          s.status === "pending"
        ) {
          activated = true;
          return { ...s, status: "active" as RouteStepStatus };
        }
        return s;
      });
      const newStatus: RoutingSlip["status"] =
        status === "rejected"
          ? "returned"
          : steps.every(
                (s) => s.status === "approved" || s.status === "signed" || s.status === "forwarded",
              )
            ? "completed"
            : "in_progress";
      return { ...slip, steps, status: newStatus };
    }),
  };
  persist();
}

export function deleteRoutingSlip(slipId: string) {
  state.value = { ...state.value, slips: state.value.slips.filter((s) => s.id !== slipId) };
  persist();
}

export function commentsForDoc(docId: string, s: State) {
  return s.comments.filter((c) => c.docId === docId).sort((a, b) => a.createdAt - b.createdAt);
}
export function signaturesForDoc(docId: string, s: State) {
  return s.signatures.filter((x) => x.docId === docId);
}
export function stampsForDoc(docId: string, s: State) {
  return s.stamps.filter((x) => x.docId === docId);
}
export function slipsForDoc(docId: string, s: State) {
  return s.slips.filter((x) => x.docId === docId);
}

export const STAMP_META: Record<StampKind, { label: string; ring: string; text: string }> = {
  approved: { label: "APPROVED", ring: "ring-success/60 text-success", text: "text-success" },
  reviewed: { label: "REVIEWED", ring: "ring-info/60 text-info", text: "text-info" },
  received: { label: "RECEIVED", ring: "ring-primary/60 text-primary", text: "text-primary" },
  confidential: {
    label: "CONFIDENTIAL",
    ring: "ring-destructive/60 text-destructive",
    text: "text-destructive",
  },
  urgent: { label: "URGENT", ring: "ring-warning/60 text-warning", text: "text-warning" },
  filed: {
    label: "FILED",
    ring: "ring-muted-foreground/40 text-muted-foreground",
    text: "text-muted-foreground",
  },
};

export const DIRECTORY = [
  { name: "Sofia Reyes", initials: "SR", role: "Legal Counsel" },
  { name: "Marcus Bell", initials: "MB", role: "Director, Operations" },
  { name: "Idris Khan", initials: "IK", role: "CTO" },
  { name: "Petra Lindqvist", initials: "PL", role: "CFO" },
  { name: "Jonas Weber", initials: "JW", role: "CEO" },
  { name: "Naledi Okafor", initials: "NO", role: "Head of HR" },
  { name: "Records Office", initials: "RO", role: "Filing" },
];
