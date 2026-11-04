/** What this installation has switched on, read once from /health. */
export interface Features {
  ai: boolean;
  editor: boolean;
  esign: boolean;
  realtime: boolean;
  signing: boolean;
  malware_scan?: boolean;
  pdf_tools?: boolean;
  /** "Open in Microsoft 365" for files stored in SharePoint. */
  microsoft365?: boolean;
  search?: boolean;
  /** Text recognition with Docling. */
  ocr?: boolean;
}

let pending: Promise<void> | null = null;

export const useFeatures = () => {
  const features = useState<Features | null>("features", () => null);
  if (import.meta.client && !features.value && !pending) {
    const { $api } = useNuxtApp();
    pending = $api<{ features: Features }>("/health")
      .then((response) => {
        features.value = response.features;
      })
      .catch(() => {
        pending = null;
      });
  }
  return features;
};
