import { onMounted, onUnmounted } from "vue";
import type { ComputedRef } from "vue";

interface Shortcut {
  key: string;
  ctrl?: boolean;
  shift?: boolean;
  alt?: boolean;
  handler: () => void;
  enabled?: boolean | ComputedRef<boolean>;
}

export function useKeyboardShortcuts(shortcuts: Shortcut[]) {
  function onKey(e: KeyboardEvent) {
    for (const s of shortcuts) {
      const enabled =
        typeof s.enabled === "function"
          ? (s.enabled as ComputedRef<boolean>).value
          : (s.enabled ?? true);
      if (!enabled) continue;
      const matchKey = e.key.toLowerCase() === s.key.toLowerCase();
      const matchCtrl = (e.metaKey || e.ctrlKey) === !!s.ctrl;
      const matchShift = e.shiftKey === !!s.shift;
      const matchAlt = e.altKey === !!s.alt;
      if (matchKey && matchCtrl && matchShift && matchAlt) {
        e.preventDefault();
        s.handler();
        return;
      }
    }
  }

  onMounted(() => window.addEventListener("keydown", onKey));
  onUnmounted(() => window.removeEventListener("keydown", onKey));
}
