export const formatBytes = (bytes?: number): string => {
  if (!bytes || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
  const value = bytes / 1024 ** i;
  return `${value >= 10 || i === 0 ? Math.round(value) : value.toFixed(1)} ${units[i]}`;
};

export const formatDate = (iso?: string): string => {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
};

export const formatDateTime = (iso?: string): string => {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
};

export type FileKind = "pdf" | "docx" | "xlsx" | "pptx" | "image" | "txt";

export const fileTypeFromMime = (mime?: string): FileKind => {
  const m = (mime ?? "").toLowerCase();
  if (m.includes("pdf")) return "pdf";
  if (m.includes("word") || m.includes("wordprocessingml")) return "docx";
  if (m.includes("spreadsheet") || m.includes("excel") || m.includes("spreadsheetml"))
    return "xlsx";
  if (m.includes("presentation") || m.includes("powerpoint") || m.includes("presentationml"))
    return "pptx";
  if (m.includes("image")) return "image";
  return "txt";
};

export const initialsOf = (name?: string): string =>
  (name ?? "")
    .split(" ")
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("");
