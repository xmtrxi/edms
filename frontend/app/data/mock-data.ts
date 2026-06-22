export type DocStatus = "ready" | "processing" | "ocr" | "synced" | "error" | "review";
export type DocSource = "sharepoint" | "local" | "upload";

export interface Doc {
  id: string;
  name: string;
  type: "pdf" | "docx" | "xlsx" | "pptx" | "image" | "txt";
  size: string;
  owner: string;
  ownerInitials: string;
  modified: string;
  modifiedISO: string;
  tags: string[];
  status: DocStatus;
  source: DocSource;
  library?: string;
  starred?: boolean;
  shared?: boolean;
  aiSummary?: string;
  entities?: string[];
  versions?: number;
  folder?: string;
  trashed?: boolean;
  archived?: boolean;
}

const owners = [
  ["Aiko Tanaka", "AT"],
  ["Marcus Bell", "MB"],
  ["Sofia Reyes", "SR"],
  ["Idris Khan", "IK"],
  ["Petra Lindqvist", "PL"],
  ["Jonas Weber", "JW"],
  ["Naledi Okafor", "NO"],
] as const;

const libraries = [
  "Contracts",
  "HR Policies",
  "Finance 2025",
  "Engineering",
  "Legal Vault",
  "Marketing",
];

const summaries = [
  "Master service agreement with Acme Corp covering Q3-Q4 deliverables, renewal clauses, and liability caps.",
  "Quarterly financial report including revenue breakdown, EBITDA, and forward-looking guidance for FY26.",
  "Employee handbook revision — updated remote work policy, expense reimbursement, and PTO accrual.",
  "Engineering RFC proposing migration from monolith to event-driven services with phased rollout plan.",
  "Vendor risk assessment with SOC2 mapping, residual risk score 2.3 / 5, and remediation timeline.",
  "Go-to-market plan for the Atlas product line — pricing tiers, ICP, and competitive positioning.",
];

const tagPool = [
  "Contract",
  "Confidential",
  "Q4",
  "Reviewed",
  "Draft",
  "Final",
  "Legal",
  "Finance",
  "HR",
  "Engineering",
  "GDPR",
  "Vendor",
  "Audit",
  "Renewal",
];

function pick<T>(arr: readonly T[], i: number): T {
  return arr[i % arr.length];
}

const names = [
  "MSA_Acme_Corp_v4.pdf",
  "Q3_Financial_Report.xlsx",
  "Employee_Handbook_2026.docx",
  "RFC-014_Event_Driven.pdf",
  "Vendor_Risk_Vantage.pdf",
  "Atlas_GTM_Plan.pptx",
  "NDA_Helix_Labs.pdf",
  "Board_Minutes_May.docx",
  "Salary_Bands_FY26.xlsx",
  "Brand_Guidelines_v7.pdf",
  "Architecture_Diagram.png",
  "SOC2_Audit_Report.pdf",
  "Lease_HQ_Berlin.pdf",
  "Roadmap_2026.pptx",
  "Marketing_Calendar.xlsx",
  "Privacy_Policy_EU.docx",
  "Incident_Postmortem.pdf",
  "Onboarding_Checklist.docx",
  "Acquisition_LOI.pdf",
  "Patent_Filing_US.pdf",
];

const types: Doc["type"][] = [
  "pdf",
  "xlsx",
  "docx",
  "pdf",
  "pdf",
  "pptx",
  "pdf",
  "docx",
  "xlsx",
  "pdf",
  "image",
  "pdf",
  "pdf",
  "pptx",
  "xlsx",
  "docx",
  "pdf",
  "docx",
  "pdf",
  "pdf",
];

const statuses: DocStatus[] = [
  "ready",
  "synced",
  "ocr",
  "ready",
  "review",
  "ready",
  "synced",
  "ready",
  "ready",
  "synced",
  "processing",
  "ready",
  "ready",
  "review",
  "ready",
  "synced",
  "ready",
  "ready",
  "review",
  "ready",
];

const sources: DocSource[] = [
  "sharepoint",
  "sharepoint",
  "local",
  "sharepoint",
  "upload",
  "local",
  "sharepoint",
  "sharepoint",
  "local",
  "sharepoint",
  "upload",
  "sharepoint",
  "sharepoint",
  "local",
  "sharepoint",
  "sharepoint",
  "local",
  "local",
  "upload",
  "sharepoint",
];

const SEED_NOW = new Date("2026-05-29T12:00:00Z").getTime();

function relativeTimeFrom(d: Date, nowMs: number): string {
  const sec = Math.floor((nowMs - d.getTime()) / 1000);
  if (sec < 60) return "just now";
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
  if (sec < 86400 * 30) return `${Math.floor(sec / 86400)}d ago`;
  return d.toISOString().slice(0, 10);
}

export const documents: Doc[] = names.map((name, i) => {
  const [owner, initials] = pick(owners, i);
  const days = (i * 3) % 60;
  const date = new Date(SEED_NOW - days * 86400000);
  return {
    id: `doc_${i + 1}`,
    name,
    type: types[i],
    size: `${(0.4 + (i % 9) * 1.3).toFixed(1)} MB`,
    owner,
    ownerInitials: initials,
    modifiedISO: date.toISOString(),
    modified: relativeTimeFrom(date, SEED_NOW),
    tags: [pick(tagPool, i), pick(tagPool, i + 3), pick(tagPool, i + 7)],
    status: statuses[i],
    source: sources[i],
    library: sources[i] === "sharepoint" ? pick(libraries, i) : undefined,
    starred: i % 5 === 0,
    shared: i % 3 === 0,
    aiSummary: pick(summaries, i),
    entities: ["Acme Corp", "EU GDPR", "$1.2M", "Q4 2025"].slice(0, 2 + (i % 3)),
    versions: 1 + (i % 6),
    folder: i % 4 === 0 ? "Legal" : i % 4 === 1 ? "Finance" : i % 4 === 2 ? "HR" : "Engineering",
    trashed: i === 6 || i === 11,
    archived: i === 9,
  };
});

export function relativeTime(d: Date): string {
  return relativeTimeFrom(d, Date.now());
}

export interface Workflow {
  id: string;
  title: string;
  doc: string;
  type: "approval" | "review" | "signature";
  status: "pending" | "approved" | "rejected" | "in_review";
  assignees: { name: string; initials: string }[];
  due: string;
  priority: "low" | "medium" | "high";
  progress: number;
}

export const workflows: Workflow[] = [
  {
    id: "wf1",
    title: "Approve MSA renewal",
    doc: "MSA_Acme_Corp_v4.pdf",
    type: "approval",
    status: "pending",
    assignees: [
      { name: "Aiko T.", initials: "AT" },
      { name: "Marcus B.", initials: "MB" },
    ],
    due: "Tomorrow",
    priority: "high",
    progress: 60,
  },
  {
    id: "wf2",
    title: "Legal review — NDA",
    doc: "NDA_Helix_Labs.pdf",
    type: "review",
    status: "in_review",
    assignees: [{ name: "Sofia R.", initials: "SR" }],
    due: "In 3 days",
    priority: "medium",
    progress: 35,
  },
  {
    id: "wf3",
    title: "Sign acquisition LOI",
    doc: "Acquisition_LOI.pdf",
    type: "signature",
    status: "pending",
    assignees: [
      { name: "Idris K.", initials: "IK" },
      { name: "Petra L.", initials: "PL" },
      { name: "Jonas W.", initials: "JW" },
    ],
    due: "Today",
    priority: "high",
    progress: 80,
  },
  {
    id: "wf4",
    title: "Handbook policy review",
    doc: "Employee_Handbook_2026.docx",
    type: "review",
    status: "approved",
    assignees: [{ name: "Naledi O.", initials: "NO" }],
    due: "Completed",
    priority: "low",
    progress: 100,
  },
  {
    id: "wf5",
    title: "Vendor risk sign-off",
    doc: "Vendor_Risk_Vantage.pdf",
    type: "approval",
    status: "rejected",
    assignees: [{ name: "Aiko T.", initials: "AT" }],
    due: "Returned",
    priority: "medium",
    progress: 100,
  },
  {
    id: "wf6",
    title: "Q3 financial approval",
    doc: "Q3_Financial_Report.xlsx",
    type: "approval",
    status: "in_review",
    assignees: [
      { name: "Marcus B.", initials: "MB" },
      { name: "Petra L.", initials: "PL" },
    ],
    due: "In 5 days",
    priority: "medium",
    progress: 50,
  },
];

export interface AIInsight {
  id: string;
  type: "summary" | "risk" | "classification" | "entity" | "related";
  title: string;
  doc: string;
  confidence: number;
  detail: string;
  severity?: "low" | "medium" | "high";
}

export const insights: AIInsight[] = [
  {
    id: "ai1",
    type: "risk",
    title: "Auto-renewal clause detected",
    doc: "MSA_Acme_Corp_v4.pdf",
    confidence: 0.94,
    detail: "Section 12.4 auto-renews for 24 months unless terminated 90 days before expiry.",
    severity: "high",
  },
  {
    id: "ai2",
    type: "classification",
    title: "Reclassified as 'Confidential — Legal'",
    doc: "Acquisition_LOI.pdf",
    confidence: 0.98,
    detail: "Document contains M&A terms and named parties. Access scope narrowed automatically.",
    severity: "medium",
  },
  {
    id: "ai3",
    type: "summary",
    title: "Executive summary generated",
    doc: "Q3_Financial_Report.xlsx",
    confidence: 0.91,
    detail: "Revenue up 18% YoY, gross margin steady at 64%, three regions flagged for review.",
    severity: "low",
  },
  {
    id: "ai4",
    type: "entity",
    title: "12 entities extracted",
    doc: "Vendor_Risk_Vantage.pdf",
    confidence: 0.88,
    detail: "Parties, jurisdictions, monetary values, and 4 effective dates identified.",
    severity: "low",
  },
  {
    id: "ai5",
    type: "related",
    title: "3 related documents found",
    doc: "RFC-014_Event_Driven.pdf",
    confidence: 0.82,
    detail: "Linked to ADR-008, Roadmap_2026.pptx, and Architecture_Diagram.png.",
    severity: "low",
  },
  {
    id: "ai6",
    type: "risk",
    title: "Liability cap below policy",
    doc: "NDA_Helix_Labs.pdf",
    confidence: 0.79,
    detail: "Cap of $250k is below the $1M floor required by Legal Policy 4.2.",
    severity: "medium",
  },
];

export interface SharePointSite {
  id: string;
  name: string;
  url: string;
  libraries: number;
  documents: number;
  syncStatus: "healthy" | "syncing" | "conflict" | "paused";
  lastSync: string;
  conflicts?: number;
}

export const spSites: SharePointSite[] = [
  {
    id: "sp1",
    name: "Legal Vault",
    url: "contoso.sharepoint.com/sites/legal",
    libraries: 8,
    documents: 4221,
    syncStatus: "healthy",
    lastSync: "2m ago",
  },
  {
    id: "sp2",
    name: "Finance Hub",
    url: "contoso.sharepoint.com/sites/finance",
    libraries: 12,
    documents: 9834,
    syncStatus: "syncing",
    lastSync: "syncing now",
  },
  {
    id: "sp3",
    name: "People Ops",
    url: "contoso.sharepoint.com/sites/hr",
    libraries: 5,
    documents: 1820,
    syncStatus: "conflict",
    lastSync: "12m ago",
    conflicts: 3,
  },
  {
    id: "sp4",
    name: "Engineering Wiki",
    url: "contoso.sharepoint.com/sites/eng",
    libraries: 22,
    documents: 15470,
    syncStatus: "healthy",
    lastSync: "just now",
  },
  {
    id: "sp5",
    name: "Marketing Cloud",
    url: "contoso.sharepoint.com/sites/marketing",
    libraries: 6,
    documents: 3110,
    syncStatus: "paused",
    lastSync: "2h ago",
  },
];

export const activityFeed = [
  {
    id: "a1",
    who: "Aiko Tanaka",
    initials: "AT",
    action: "approved",
    target: "MSA_Acme_Corp_v4.pdf",
    when: "2m ago",
  },
  {
    id: "a2",
    who: "AI Assistant",
    initials: "AI",
    action: "classified",
    target: "Acquisition_LOI.pdf as Confidential",
    when: "8m ago",
  },
  {
    id: "a3",
    who: "Marcus Bell",
    initials: "MB",
    action: "uploaded",
    target: "Patent_Filing_US.pdf",
    when: "21m ago",
  },
  {
    id: "a4",
    who: "SharePoint Sync",
    initials: "SP",
    action: "synced",
    target: "Finance Hub — 142 files",
    when: "34m ago",
  },
  {
    id: "a5",
    who: "Sofia Reyes",
    initials: "SR",
    action: "commented on",
    target: "NDA_Helix_Labs.pdf",
    when: "1h ago",
  },
  {
    id: "a6",
    who: "Idris Khan",
    initials: "IK",
    action: "shared",
    target: "Q3_Financial_Report.xlsx with Finance team",
    when: "2h ago",
  },
];

export const dashboardMetrics = {
  totalDocs: 34_886,
  processingQueue: 27,
  ocrPending: 9,
  aiClassified: 12_440,
  recentUploads: 184,
  workflowsOpen: 14,
  storageUsedGB: 612,
  storageTotalGB: 1024,
  searchesToday: 1_247,
  spHealthPct: 98,
};

export const uploadSeries = [12, 19, 16, 22, 28, 14, 9, 18, 31, 27, 24, 33, 29, 41];
export const processingSeries = [4, 8, 6, 11, 14, 5, 3, 7, 12, 10, 9, 13, 11, 16];
