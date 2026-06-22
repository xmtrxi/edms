import type { Doc } from "@/data/mock-data";

export function downloadDoc(doc: Doc) {
  const ext =
    doc.type === "pdf"
      ? "pdf"
      : doc.type === "docx"
        ? "docx"
        : doc.type === "xlsx"
          ? "xlsx"
          : doc.type === "pptx"
            ? "pptx"
            : doc.type === "image"
              ? "png"
              : "txt";
  const content = doc.aiSummary
    ? `${doc.name}\n\n${doc.aiSummary}\n\nOwner: ${doc.owner}\nType: ${doc.type}\nSize: ${doc.size}\nTags: ${doc.tags.join(", ")}`
    : `${doc.name}\n\nOwner: ${doc.owner}\nType: ${doc.type}\nSize: ${doc.size}\nTags: ${doc.tags.join(", ")}`;
  const blob = new Blob([content], { type: "text/plain" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = doc.name.replace(/\.[^.]+$/, "") + "." + ext;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
