export interface ApiError {
  statusCode?: number;
  error: string;
  message?: string;
}

export interface ApiMeta {
  page?: number;
  page_size?: number;
  total: number;
  has_more?: boolean;
}

export interface ApiResponse<T> {
  data: T;
  message?: string;
  meta?: ApiMeta;
}

// ── Auth ────────────────────────────────────────────────────────────────────

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  name: string;
  email: string;
  password: string;
}

export type UserStatus = "active" | "disabled";

export interface User {
  id: string;
  email: string;
  name: string;
  status: UserStatus;
  created_at: string;
  updated_at: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

// ── Documents ───────────────────────────────────────────────────────────────

export type DocumentStatus = "uploaded" | "processing" | "ready" | "failed" | "archived";

export interface DocumentOwner {
  id: string;
  name: string;
  email: string;
}

export interface OCRResponse {
  document_id: string;
  text: string;
  language: string;
  pages: number;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface Document {
  id: string;
  name: string;
  original_name: string;
  mime_type: string;
  size: number;
  status: DocumentStatus;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  owner?: DocumentOwner;
  shared_with?: DocumentOwner[];
  ocr?: OCRResponse;
  category?: Category;
  current_version_id?: string;
  version_count: number;
}

export type DocumentVersionSource =
  | "upload"
  | "storage_sync"
  | "onlyoffice_edit"
  | "stirling_pdf"
  | "workflow_revision"
  | "conversion"
  | "signature"
  | "restore";

export interface DocumentVersion {
  id: string;
  document_id: string;
  parent_version_id?: string;
  version: number;
  file_name: string;
  mime_type: string;
  size: number;
  checksum: string;
  created_by: string;
  creator?: User;
  source: DocumentVersionSource;
  change_summary: string;
  change_manifest?: Record<string, unknown>;
  created_at: string;
}

export interface DocumentContributor {
  user: User;
  versions_created: number;
  first_edited_at: string;
  last_edited_at: string;
}

export interface DocumentContributors {
  unique_editor_count: number;
  edit_count: number;
  contributors: DocumentContributor[];
}

export interface DocumentHistoryEvent {
  id: string;
  type: string;
  actor_id: string;
  action: string;
  resource: string;
  resource_id: string;
  document_version_id?: string;
  summary?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

export type WorkflowStatus =
  | "draft"
  | "pending"
  | "active"
  | "waiting_for_changes"
  | "waiting_for_signature"
  | "approved"
  | "rejected"
  | "completed"
  | "canceled";

export interface WorkflowStep {
  id: string;
  sequence: number;
  name: string;
  action_type: "review" | "approve" | "sign" | "edit" | "acknowledge" | "file";
  assignment_type: "user" | "department" | "external";
  assigned_user_id?: string;
  assigned_department_id?: string;
  external_email?: string;
  status: string;
  require_comment: boolean;
  require_new_version: boolean;
  activated_at?: string;
  completed_at?: string;
}

export interface DocumentRoute {
  id: string;
  document_id: string;
  started_version_id: string;
  current_version_id: string;
  title: string;
  status: WorkflowStatus;
  priority: "low" | "medium" | "high";
  current_step: number;
  returned_version_id?: string;
  due_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
  steps: WorkflowStep[];
}

export interface CreateDocumentRoute {
  title: string;
  priority: "low" | "medium" | "high";
  due_at?: string;
  steps: Array<{
    name: string;
    action_type: WorkflowStep["action_type"];
    assignment_type: WorkflowStep["assignment_type"];
    assigned_user_id?: string;
    assigned_department_id?: string;
    external_email?: string;
    require_comment: boolean;
    require_new_version: boolean;
  }>;
}

export interface RouteDecision {
  action:
    | "approve"
    | "reject"
    | "return_for_changes"
    | "resubmit"
    | "sign"
    | "acknowledge"
    | "file";
  comment?: string;
  return_to_step?: number;
}

export interface Category {
  id: string;
  name: string;
  description?: string;
  parent_id?: string;
  color?: string;
  document_count: number;
  created_at: string;
  updated_at: string;
}

export interface CategoryInput {
  name: string;
  description?: string;
  parent_id?: string;
  color?: string;
}

export interface StorageSyncResult {
  dry_run: boolean;
  scanned: number;
  imported: number;
  would_import?: number;
  skipped: number;
  failed: number;
  errors?: string[];
}

export interface ListDocumentsParams {
  q?: string;
  status?: string;
  category_id?: string;
  page?: number;
  page_size?: number;
}

// ── Health ──────────────────────────────────────────────────────────────────

export interface HealthResponse {
  status: string;
  env: string;
  features: Record<string, boolean>;
  created_at?: string;
  updated_at?: string;
}
