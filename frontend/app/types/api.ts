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
}

export interface ListDocumentsParams {
  q?: string;
  status?: string;
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
