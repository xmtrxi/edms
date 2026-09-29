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
  organization_name?: string;
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
  organization: Organization;
}

// ── Organizations ──────────────────────────────────────────────────────────

export type DeploymentMode = "saas" | "selfhosted";

/** Public description of the installation, from GET /meta. */
export interface DeploymentMeta {
  mode: DeploymentMode;
  open_signup: boolean;
  personal_workspaces: boolean;
  multi_org: boolean;
  organization_name?: string;
}

export type OrganizationKind = "personal" | "organization";
export type MembershipRole = "owner" | "admin" | "member" | "guest";
export type MembershipStatus = "active" | "suspended";

export interface Organization {
  id: string;
  slug: string;
  name: string;
  kind: OrganizationKind;
  role?: MembershipRole;
  created_at: string;
}

export interface SessionResponse {
  user: User;
  organization: Organization;
  organizations: Organization[];
}

export interface SwitchOrganizationResponse {
  token: string;
  organization: Organization;
}

export interface Member {
  id: string;
  user_id: string;
  email: string;
  name: string;
  role: MembershipRole;
  status: MembershipStatus;
  joined_at: string;
  invited_by?: string;
  /** "agent" for AI agent accounts that act through MCP. */
  kind?: string;
}

export interface Invitation {
  id: string;
  email: string;
  name: string;
  role: MembershipRole;
  status: string;
  expires_at: string;
  created_at: string;
}

export interface InvitationPreview {
  email: string;
  name: string;
  role: MembershipRole;
  organization_name: string;
  account_exists: boolean;
}

// ── Administration & forms ─────────────────────────────────────────────────

export interface Role {
  id: string;
  key: string;
  name: string;
  description?: string;
  protected: boolean;
  created_at: string;
  updated_at: string;
}

export interface FormField {
  id: string;
  label: string;
  type: "text" | "textarea" | "number" | "currency" | "date" | "select" | "checkbox" | "file";
  required?: boolean;
  options?: string[];
}

export interface ApprovalStepDefinition {
  name: string;
  completion_rule: "any_one" | "everyone";
  user_ids?: string[];
  department_ids?: string[];
}

export interface FormDefinition {
  allowed_department_ids: string[];
  fields: FormField[];
  approval_steps: ApprovalStepDefinition[];
}

export interface FormTemplate {
  id: string;
  department_id: string;
  name: string;
  description?: string;
  status: "draft" | "published" | "archived";
  version: number;
  definition: FormDefinition;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface FormSubmission {
  id: string;
  template_id: string;
  template_version: number;
  department_id: string;
  submitted_by: string;
  status: "awaiting_approval" | "returned" | "approved" | "rejected";
  values: Record<string, unknown>;
  definition: FormDefinition;
  created_at: string;
  updated_at: string;
}

export interface FormComment {
  id: string;
  submission_id: string;
  author_id: string;
  body: string;
  created_at: string;
}

export interface FormApprovalStep {
  id: string;
  submission_id: string;
  sequence: number;
  name: string;
  completion_rule: "any_one" | "everyone";
  status: "pending" | "active" | "approved";
  created_at: string;
  completed_at?: string;
}

export interface FormApprovalAssignee {
  id: string;
  step_id: string;
  user_id: string;
  department_id?: string;
  status: "pending" | "approve" | "reject" | "return";
  decided_at?: string;
}

export interface FormSubmissionDocument {
  id: string;
  submission_id: string;
  document_id: string;
  created_at: string;
}

export interface FormSubmissionDetail {
  submission: FormSubmission;
  steps: FormApprovalStep[];
  assignees: FormApprovalAssignee[];
  comments: FormComment[];
  documents: FormSubmissionDocument[];
}

export interface VisitorRequest {
  id: string;
  template_id: string;
  department_id: string;
  visitor_name: string;
  visitor_email: string;
  submission_id?: string;
  status: "awaiting_visitor" | "submitted" | "revoked";
  expires_at: string;
  created_at: string;
}

export interface VisitorRequestCreated {
  request: VisitorRequest;
  access_code: string;
  public_url: string;
}

export interface VisitorApprovalStep {
  id: string;
  submission_id: string;
  sequence: number;
  name: string;
  completion_rule: "any_one" | "everyone";
  status: "pending" | "active" | "approved";
  created_at: string;
  completed_at?: string;
}

export interface VisitorPortal {
  request: VisitorRequest;
  template: FormTemplate;
  submission?: FormSubmission;
  steps: VisitorApprovalStep[];
}

// ── Documents ───────────────────────────────────────────────────────────────

export type DocumentStatus =
  | "uploaded"
  | "processing"
  | "ready"
  | "failed"
  | "archived"
  | "trashed";

export interface DocumentOwner {
  id: string;
  name: string;
  email: string;
}

export interface OCRResponse {
  document_id: string;
  text: string;
  markdown: string;
  language: string;
  pages: number;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface ProcessingJob {
  id: string;
  document_id: string;
  type: string;
  status: "queued" | "running" | "completed" | "failed";
  attempts: number;
  last_error?: string;
  external_id?: string;
  result?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  completed_at?: string;
}

export interface Document {
  id: string;
  name: string;
  original_name: string;
  folder_id?: string;
  folder_path?: string;
  folder_color?: string;
  folder_category_ids?: string[];
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
  sensitivity?: Sensitivity;
  inherit_permissions?: boolean;
  /** What the signed-in user may do: view, download, comment, edit, share, delete… */
  permissions?: string[];
}

export interface DashboardOverview {
  total_documents: number;
  ready_documents: number;
  pending_forms: number;
  active_workflows: number;
  visitor_requests: number;
  recent_documents: Document[];
  recent_submissions: FormSubmission[];
}

export interface DocumentShare {
  id: string;
  document_id: string;
  shared_with_user_id?: string;
  shared_with_email: string;
  permission: "view" | "download";
  expires_at: string;
  revoked_at?: string;
  created_at: string;
}

export interface AIJob {
  id: string;
  kind: "classify" | "ask" | "summarize" | "extract";
  status: "running" | "completed" | "failed";
  document_ids: string[];
  result?: Record<string, unknown>;
  error?: string;
  created_at: string;
  completed_at?: string;
}

export interface DocumentFolder {
  path: string;
  name: string;
  parent_path?: string;
  color?: string;
  category_ids?: string[];
  categories?: Category[];
  count: number;
  size: number;
  updated_at: string;
}

export interface SearchResultDocument extends Document {
  snippet?: string;
  version_id?: string;
  chunk_index: number;
  headings?: string[];
  page_numbers?: number[];
  page_start?: number;
  page_end?: number;
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
  folder_path?: string;
  folder_id?: string;
  page?: number;
  page_size?: number;
  mode?: "documents" | "passages";
  document_id?: string;
}

export interface ManagedFolder {
  id: string;
  parent_id?: string;
  ancestor_ids?: string[];
  depth: number;
  name: string;
  description?: string;
  color?: string;
  inherit_permissions: boolean;
  document_count: number;
  can_manage?: boolean;
  can_upload?: boolean;
  created_at: string;
  updated_at: string;
}

export interface FolderInput {
  name: string;
  parent_id?: string;
  description?: string;
  color?: string;
  inherit_permissions?: boolean;
}

export type FolderRole = "manager" | "editor" | "contributor" | "commenter" | "viewer" | "uploader";

export type PrincipalType = "user" | "department" | "group" | "role" | "everyone";

/** A grant as shown in access lists, with a readable principal name. */
export interface FolderGrant {
  id: string;
  principal_type: PrincipalType;
  principal_id: string;
  name: string;
  description?: string;
  role: FolderRole;
  allow?: string[];
  deny?: string[];
  /** "folder" for this folder, "inherited" from a parent, "document" for file grants. */
  source: "folder" | "inherited" | "document";
  source_name?: string;
}

export interface FolderGrantInput {
  principal_type: PrincipalType;
  principal_id: string;
  role: FolderRole;
  allow?: string[];
  deny?: string[];
}

export interface PrincipalOption {
  type: PrincipalType;
  id: string;
  name: string;
  description?: string;
}

// ── Sharing ────────────────────────────────────────────────────────────────

export type Sensitivity = "public" | "internal" | "confidential" | "restricted";
export type ShareRole = "viewer" | "commenter" | "editor";

export interface ShareView {
  id: string;
  kind: "user" | "email" | "link";
  user_id?: string;
  email?: string;
  name?: string;
  role: ShareRole;
  link_scope?: "organization" | "anyone";
  has_password: boolean;
  allow_download: boolean;
  expires_at: string;
  created_at: string;
}

export interface SharePolicy {
  allow_external: boolean;
  allow_public_links: boolean;
  allowed_domains: string[];
  max_link_days: number;
}

export interface DocumentAccess {
  document_id: string;
  owner_id: string;
  owner_name: string;
  folder_id?: string;
  folder_name?: string;
  inherit_permissions: boolean;
  sensitivity: Sensitivity;
  inherited: FolderGrant[];
  grants: FolderGrant[];
  shares: ShareView[];
  links: ShareView[];
  policy: SharePolicy;
  can_share: boolean;
  pending_requests: number;
}

export interface ShareResult {
  kind: "grant" | "user" | "email";
  link?: string;
}

export interface AccessRequest {
  id: string;
  document_id: string;
  document_name: string;
  requester_id: string;
  requester_name: string;
  requester_email: string;
  role: ShareRole;
  message?: string;
  status: "pending" | "approved" | "denied";
  created_at: string;
}

/** Who can see a new upload. */
export interface UploadAccess {
  mode: "inherit" | "private" | "custom";
  sensitivity?: Sensitivity;
  grants?: FolderGrantInput[];
}

export interface LinkDocument {
  document: Document;
  role: ShareRole;
  link_scope?: "organization" | "anyone";
  can_download: boolean;
  expires_at: string;
}

export interface Group {
  id: string;
  name: string;
  description?: string;
  member_count: number;
  created_at: string;
}

export interface GroupMember {
  id: string;
  user_id: string;
  name: string;
  email: string;
  role: string;
}

export interface Department {
  id: string;
  parent_id?: string;
  name: string;
  description?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

// ── Health ──────────────────────────────────────────────────────────────────

export interface HealthResponse {
  status: string;
  env: string;
  features: Record<string, boolean>;
  created_at?: string;
  updated_at?: string;
}

// ── Viewer ─────────────────────────────────────────────────────────────────

/** Size of one PDF page in points (1/72 inch), used to place overlays. */
export interface PdfPageSize {
  width: number;
  height: number;
}

export type PreviewKind = "pdf" | "image" | "text" | "video" | "audio";

// ── Drive ──────────────────────────────────────────────────────────────────

export interface FileRequest {
  id: string;
  folder_id: string;
  title: string;
  instructions?: string;
  max_files: number;
  received_files: number;
  expires_at: string;
  closed: boolean;
  created_at: string;
  /** Only returned when the request is created. */
  url?: string;
}

export interface PublicFileRequest {
  title: string;
  instructions?: string;
  organization: string;
  requested_by: string;
  remaining: number;
  expires_at: string;
}

export interface ApiToken {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  last_used_at?: string;
  expires_at?: string;
  created_at: string;
  /** Only returned when the token is created. */
  secret?: string;
}

// ── Assistant ──────────────────────────────────────────────────────────────

export interface AICitation {
  index: number;
  document_id: string;
  document_name: string;
  version_id?: string;
  page_start?: number;
  page_end?: number;
  snippet: string;
}

export interface AIAnswer {
  answer: string;
  citations: AICitation[];
  sources: AICitation[];
}
