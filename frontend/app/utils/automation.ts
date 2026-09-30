import type { AutomationAction, AutomationRule, AutomationTrigger, Sensitivity } from "@/types/api";

export const triggerOptions: { value: AutomationTrigger; label: string; hint: string }[] = [
  {
    value: "document.created",
    label: "A file is added",
    hint: "Uploads, new documents and file requests",
  },
  {
    value: "document.labelled",
    label: "A file has been read",
    hint: "After text extraction, sensitive-data checks and AI classification",
  },
  { value: "version.created", label: "A new version is saved", hint: "Uploads and browser edits" },
  { value: "signature.completed", label: "Everyone has signed", hint: "The sealed copy was saved" },
];

export const sensitivityOptions: { value: Sensitivity; label: string }[] = [
  { value: "public", label: "Public" },
  { value: "internal", label: "Internal" },
  { value: "confidential", label: "Confidential" },
  { value: "restricted", label: "Restricted" },
];

const triggerPhrase: Record<AutomationTrigger, string> = {
  "document.created": "a file is added",
  "document.labelled": "a file has been read",
  "version.created": "a new version is saved",
  "signature.completed": "everyone has signed",
};

export const actionSummary = (action: AutomationAction, folderName: (id?: string) => string) => {
  switch (action.type) {
    case "move_to_folder":
      return `move it to ${folderName(action.folder_id)}`;
    case "set_sensitivity":
      return `mark it ${action.sensitivity}`;
    case "grant_access":
      return `let ${action.principal_name || "someone"} ${action.role === "editor" ? "edit" : action.role === "commenter" ? "comment" : "view"}`;
    case "notify":
      return `email ${action.emails?.join(", ")}`;
  }
};

/** A readable sentence describing a rule. */
export const ruleSummary = (
  rule: Pick<AutomationRule, "trigger" | "folder_id" | "conditions" | "actions">,
  folderName: (id?: string) => string,
) => {
  const parts = [`When ${triggerPhrase[rule.trigger]}`];
  if (rule.folder_id) parts.push(`in ${folderName(rule.folder_id)}`);
  const c = rule.conditions ?? {};
  const filters: string[] = [];
  if (c.name_contains) filters.push(`its name contains “${c.name_contains}”`);
  if (c.extensions?.length)
    filters.push(`it is ${c.extensions.map((ext) => `.${ext.replace(/^\./, "")}`).join(" or ")}`);
  if (c.sensitivity) filters.push(`it is ${c.sensitivity}`);
  if (c.classification) filters.push(`AI classed it as “${c.classification}”`);
  if (c.sensitive_data) filters.push("it holds ID or card numbers");
  if (filters.length) parts.push(`and ${filters.join(" and ")}`);
  return `${parts.join(" ")}, ${rule.actions.map((action) => actionSummary(action, folderName)).join(", then ")}.`;
};
