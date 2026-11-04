import {
  CalendarIcon,
  SquareCheckIcon,
  PenLineIcon,
  SignatureIcon,
  TypeIcon,
  UserIcon,
} from "@lucide/vue";
import type { Component } from "vue";

import type { EnvelopeStatus, RecipientStatus, SignatureFieldKind } from "@/types/api";

export interface FieldKindMeta {
  kind: SignatureFieldKind;
  label: string;
  icon: Component;
  /** Default size in PDF points. */
  width: number;
  height: number;
  hint: string;
}

export const fieldKinds: FieldKindMeta[] = [
  {
    kind: "signature",
    label: "Signature",
    icon: SignatureIcon,
    width: 170,
    height: 48,
    hint: "Drawn or typed signature",
  },
  {
    kind: "initials",
    label: "Initials",
    icon: PenLineIcon,
    width: 64,
    height: 36,
    hint: "Initials, often on each page",
  },
  {
    kind: "date",
    label: "Date signed",
    icon: CalendarIcon,
    width: 120,
    height: 20,
    hint: "Filled in when they sign",
  },
  {
    kind: "name",
    label: "Full name",
    icon: UserIcon,
    width: 160,
    height: 20,
    hint: "Their name, editable",
  },
  {
    kind: "text",
    label: "Text",
    icon: TypeIcon,
    width: 170,
    height: 20,
    hint: "Anything they type",
  },
  {
    kind: "checkbox",
    label: "Checkbox",
    icon: SquareCheckIcon,
    width: 16,
    height: 16,
    hint: "Optional tick box",
  },
];

export const fieldMeta = (kind: SignatureFieldKind) =>
  fieldKinds.find((item) => item.kind === kind) ?? fieldKinds[0]!;

/** Distinct colours so each recipient's fields are easy to tell apart. */
export const recipientColors = ["#2563eb", "#db2777", "#059669", "#d97706", "#7c3aed", "#0891b2"];
export const recipientColor = (index: number) =>
  recipientColors[
    ((index % recipientColors.length) + recipientColors.length) % recipientColors.length
  ]!;

export const envelopeStatusLabel: Record<EnvelopeStatus, string> = {
  sent: "Waiting",
  completed: "Completed",
  declined: "Declined",
  voided: "Cancelled",
};

export const recipientStatusLabel: Record<RecipientStatus, string> = {
  waiting: "Not sent yet",
  sent: "Sent",
  viewed: "Opened",
  signed: "Signed",
  declined: "Declined",
};
