import type { RetentionAction, RetentionPolicy } from "@/types/api";

export const retentionActions: { value: RetentionAction; label: string; hint: string }[] = [
  { value: "archive", label: "Archive", hint: "Hide from Drive; still searchable by admins" },
  { value: "trash", label: "Move to trash", hint: "Owners can restore them" },
  { value: "purge", label: "Delete from trash for good", hint: "Only files already in the trash" },
];

export const policySummary = (
  policy: Pick<RetentionPolicy, "action" | "keep_days" | "age_basis" | "folder_id" | "sensitivity">,
  folderName: (id?: string) => string,
) => {
  const age =
    policy.keep_days % 365 === 0
      ? `${policy.keep_days / 365} year${policy.keep_days === 365 ? "" : "s"}`
      : `${policy.keep_days} days`;
  const scope = `${policy.sensitivity ? `${policy.sensitivity} ` : ""}files${policy.folder_id ? ` in ${folderName(policy.folder_id)}` : ""}`;
  if (policy.action === "purge")
    return `Permanently delete ${scope} that have been in the trash for ${age}.`;
  const verb = policy.action === "archive" ? "Archive" : "Move to trash";
  return `${verb} ${scope} ${age} after they were ${policy.age_basis === "modified" ? "last changed" : "added"}.`;
};
