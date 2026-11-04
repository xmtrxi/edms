import {
  ArchiveIcon,
  CheckCircle2Icon,
  CloudIcon,
  FileQuestionIcon,
  Loader2Icon,
  ScanLineIcon,
  ShieldAlertIcon,
} from "@lucide/vue";
import type { Component } from "vue";

export interface StatusMeta {
  label: string;
  tone: string;
  Icon: Component;
  spin?: boolean;
}

export const DOCUMENT_STATUSES = ["uploaded", "processing", "ready", "failed", "archived"] as const;

const statusMap: Record<string, StatusMeta> = {
  ready: { label: "Ready", tone: "bg-success/10 text-success", Icon: CheckCircle2Icon },
  synced: { label: "Synced", tone: "bg-info/10 text-info", Icon: CloudIcon },
  processing: {
    label: "Processing",
    tone: "bg-warning/10 text-warning",
    Icon: Loader2Icon,
    spin: true,
  },
  uploaded: { label: "Uploaded", tone: "bg-info/10 text-info", Icon: CloudIcon },
  ocr: { label: "OCR", tone: "bg-primary/10 text-primary", Icon: ScanLineIcon },
  review: { label: "Needs review", tone: "bg-warning/10 text-warning", Icon: ShieldAlertIcon },
  failed: { label: "Failed", tone: "bg-destructive/10 text-destructive", Icon: ShieldAlertIcon },
  error: { label: "Error", tone: "bg-destructive/10 text-destructive", Icon: ShieldAlertIcon },
  archived: { label: "Archived", tone: "bg-muted text-muted-foreground", Icon: ArchiveIcon },
};

export const getStatusMeta = (status?: string): StatusMeta =>
  (status && statusMap[status]) || {
    label: status || "Unknown",
    tone: "bg-muted text-muted-foreground",
    Icon: FileQuestionIcon,
  };
