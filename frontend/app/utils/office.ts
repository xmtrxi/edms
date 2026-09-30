// Mirrors the backend's ONLYOFFICE format list.
const editable = new Set([
  "docx",
  "docm",
  "dotx",
  "odt",
  "ott",
  "rtf",
  "txt",
  "xlsx",
  "xlsm",
  "xltx",
  "ods",
  "ots",
  "csv",
  "pptx",
  "pptm",
  "potx",
  "odp",
  "otp",
]);
const viewable = new Set([
  "doc",
  "dot",
  "xls",
  "xlt",
  "ppt",
  "pps",
  "ppsx",
  "epub",
  "fb2",
  "html",
  "htm",
  "xps",
  "djvu",
]);

const extensionOf = (name: string) => name.split(".").pop()?.toLowerCase() ?? "";

/** Files ONLYOFFICE can edit in the browser. */
export const canEditInBrowser = (name: string) => editable.has(extensionOf(name));

/** Office files ONLYOFFICE can open, even if only to read. */
export const canOpenInEditor = (name: string) =>
  canEditInBrowser(name) || viewable.has(extensionOf(name));
