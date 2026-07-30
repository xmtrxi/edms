export interface ApiError {
  error: string;
  message?: string;
  statusCode?: number;
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

export interface AuthResponse {
  user: User;
  token: string;
}

// ── Tenants ─────────────────────────────────────────────────────────────────

export type TenantStatus = "active" | "disabled";

export interface Tenant {
  id: string;
  name: string;
  slug: string;
  status: TenantStatus;
  created_at: string;
  updated_at: string;
}

// ── Users ───────────────────────────────────────────────────────────────────

export type UserStatus = "active" | "disabled";

export interface User {
  id: string;
  email: string;
  name: string;
  status: UserStatus;
  created_at?: string;
  updated_at?: string;
}

// ── Tenant Members ──────────────────────────────────────────────────────────

export type TenantRole = "owner" | "admin" | "member";

export interface TenantMember {
  id: string;
  tenant_id: string;
  user_id: string;
  role: TenantRole;
  created_at: string;
  updated_at: string;
}

export interface TenantMemberWithUser extends TenantMember {
  user?: User;
}

// ── Documents ───────────────────────────────────────────────────────────────

export type DocumentStatus = "uploaded" | "processing" | "ready" | "failed" | "archived";

export interface Document {
  id: string;
  tenant_id: string;
  owner_id: string;
  name: string;
  original_name: string;
  mime_type: string;
  size: number;
  storage_provider: string;
  storage_path: string;
  checksum: string;
  status: DocumentStatus;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface DocumentListResponse {
  items: DocumentResponse[];
}

export interface DocumentResponse {
  id: string;
  name: string;
  mime_type: string;
  size: number;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface DocumentUploadPayload {
  file: File;
  name?: string;
}

// ── Document Versions ───────────────────────────────────────────────────────

export interface DocumentVersion {
  id: string;
  document_id: string;
  version: number;
  storage_provider: string;
  storage_path: string;
  checksum: string;
  size: number;
  created_by: string;
  created_at: string;
}

// ── Tags ────────────────────────────────────────────────────────────────────

export interface Tag {
  id: string;
  tenant_id: string;
  name: string;
  color: string;
  created_at: string;
  updated_at: string;
}

// ── Document Tags ───────────────────────────────────────────────────────────

export interface DocumentTag {
  id: string;
  document_id: string;
  tag_id: string;
  created_by: string;
  created_at: string;
}

export interface DocumentTagWithName extends DocumentTag {
  tag?: Tag;
}

// ── Document Extractions ────────────────────────────────────────────────────

export interface DocumentExtraction {
  id: string;
  document_id: string;
  text: string;
  language: string;
  pages: number;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

// ── Comments ────────────────────────────────────────────────────────────────

export interface Comment {
  id: string;
  document_id: string;
  author_id: string;
  body: string;
  created_at: string;
  updated_at: string;
}

export interface CommentWithAuthor extends Comment {
  author?: User;
}

// ── Shares ──────────────────────────────────────────────────────────────────

export interface Share {
  id: string;
  document_id: string;
  created_by: string;
  token_hash: string;
  expires_at: string;
  revoked_at?: string;
  created_at: string;
}

export interface CreateShareRequest {
  document_id: string;
  expires_in_hours?: number;
}

// ── Signing Requests ────────────────────────────────────────────────────────

export type SigningStatus = "draft" | "sent" | "completed" | "cancelled" | "failed";

export interface SigningRequest {
  id: string;
  document_id: string;
  requested_by: string;
  provider: string;
  provider_document_id: string;
  status: SigningStatus;
  created_at: string;
  updated_at: string;
}

// ── Processing Jobs ─────────────────────────────────────────────────────────

export type ProcessingJobStatus = "queued" | "running" | "completed" | "failed";

export interface ProcessingJob {
  id: string;
  document_id: string;
  type: string;
  status: ProcessingJobStatus;
  attempts: number;
  last_error: string;
  external_id: string;
  result: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  completed_at?: string;
}

// ── Audit Events ───────────────────────────────────────────────────────────

export interface AuditEvent {
  id: string;
  tenant_id: string;
  actor_id: string;
  action: string;
  resource: string;
  resource_id: string;
  metadata: Record<string, unknown>;
  created_at: string;
}

export interface AuditEventWithActor extends AuditEvent {
  actor?: User;
}

// ── Workflow Requests ───────────────────────────────────────────────────────

export type WorkflowStatus = "pending" | "approved" | "rejected" | "canceled";

export interface WorkflowRequest {
  id: string;
  document_id: string;
  created_by: string;
  status: WorkflowStatus;
  created_at: string;
  updated_at: string;
}

export interface WorkflowRequestWithActions extends WorkflowRequest {
  actions?: WorkflowAction[];
  created_by_user?: User;
}

// ── Workflow Actions ────────────────────────────────────────────────────────

export type WorkflowActionKind = "approve" | "reject" | "comment" | "delegate";

export interface WorkflowAction {
  id: string;
  workflow_request_id: string;
  actor_id: string;
  action: WorkflowActionKind;
  note: string;
  created_at: string;
}

export interface WorkflowActionWithActor extends WorkflowAction {
  actor?: User;
}

// ── Health ──────────────────────────────────────────────────────────────────

export interface HealthResponse {
  status: string;
  env: string;
  features: Record<string, boolean>;
  created_at?: string;
  updated_at?: string;
}

// ── Pagination ──────────────────────────────────────────────────────────────

export interface PaginationParams {
  page?: number;
  per_page?: number;
}

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}
