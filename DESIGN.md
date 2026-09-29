---
name: EDMS
description: A calm processing desk for office records, visitor intake, and accountable hand-offs.
colors:
  ink-navy: "oklch(0.285 0.055 247)"
  paper-white: "oklch(0.972 0.008 82)"
  sheet-white: "oklch(0.995 0.004 82)"
  graphite: "oklch(0.205 0.025 246)"
  quiet-paper: "oklch(0.936 0.01 82)"
  quiet-ink: "oklch(0.47 0.027 245)"
  file-rule: "oklch(0.86 0.015 78)"
  stamp-red: "oklch(0.54 0.19 30)"
  approved-green: "oklch(0.52 0.12 158)"
  routing-blue: "oklch(0.53 0.11 225)"
typography:
  display:
    fontFamily: "IBM Plex Sans, Noto Sans Variable, sans-serif"
    fontSize: "clamp(2.25rem, 5vw, 4rem)"
    fontWeight: 600
    lineHeight: 1.05
    letterSpacing: "-0.035em"
  body:
    fontFamily: "IBM Plex Sans, Noto Sans Variable, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.6
  label:
    fontFamily: "IBM Plex Sans, Noto Sans Variable, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 600
    letterSpacing: "0.1em"
  tracking:
    fontFamily: "IBM Plex Mono, JetBrains Mono, ui-monospace, monospace"
    fontSize: "0.75rem"
    fontWeight: 500
rounded:
  control: "0px"
  field: "10px"
  panel: "16px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "16px"
  lg: "24px"
  xl: "32px"
components:
  button-primary:
    backgroundColor: "{colors.ink-navy}"
    textColor: "{colors.paper-white}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "0 24px"
    height: "40px"
  input-line:
    backgroundColor: "transparent"
    textColor: "{colors.graphite}"
    typography: "{typography.body}"
    rounded: "{rounded.control}"
    height: "40px"
  panel:
    backgroundColor: "{colors.sheet-white}"
    textColor: "{colors.graphite}"
    rounded: "{rounded.panel}"
    padding: "32px"
---

# Design System: EDMS

## Overview

**Creative North Star: "The Modern Processing Desk"**

EDMS should feel like arriving at a well-run records office: calm surfaces, legible routing, obvious ownership, and just enough ceremony to make sensitive work feel trustworthy. The system borrows the clarity of an intake counter, routing slip, and numbered case file without imitating old paper or adding decorative bureaucracy.

The interface is operational and moderately dense. Brand expression comes from deep ink fields, warm paper surfaces, precise tracking numerals, thin route lines, and status language that reads like a useful stamp. The visitor experience is quieter and more guided than the staff workspace, but both belong to the same desk.

**Key Characteristics:**

- Warm paper ground with deep ink anchors.
- Strong, compact typography and generous space between work groups.
- Monospace only for identifiers, measurements, dates, and tracking state.
- Flat controls; ambient depth is reserved for important work surfaces.
- Status is always stated in words and never communicated by color alone.

## Colors

The palette combines institutional ink and warm office paper with restrained operational status colors.

### Primary

- **Ink Navy:** anchors navigation, primary actions, visitor route panels, and the most important first-view surfaces.

### Secondary

- **Routing Blue:** indicates records moving through a process or information that is still active.
- **Approved Green:** marks completed work and affirmative outcomes.
- **Stamp Red:** is reserved for destructive actions, invalid access, and blocking errors.

### Neutral

- **Paper White:** is the application ground.
- **Sheet White:** is the working surface for forms, ledgers, and dialogs.
- **Graphite:** carries primary text.
- **Quiet Ink:** carries secondary copy while maintaining readable contrast.
- **File Rule:** separates records and form groups with a single fine line.

**The Ink Anchor Rule.** Use large ink fields only to orient the user, explain a workflow, or show its route. Do not turn every card or header navy.

## Typography

**Display Font:** IBM Plex Sans (with Noto Sans Variable and sans-serif fallback)  
**Body Font:** IBM Plex Sans (with Noto Sans Variable and sans-serif fallback)  
**Label/Mono Font:** IBM Plex Mono (with JetBrains Mono and ui-monospace fallback)

**Character:** The Plex pairing feels administrative without being cold. Sans carries action and explanation; Mono makes tracking identifiers and tabular state scannable.

### Hierarchy

- **Display** (600, responsive up to 4rem, 1.05): one short thesis on an important first view.
- **Headline** (600, 1.5–2.25rem): page and feature titles with tight but never compressed tracking.
- **Title** (600, 1–1.125rem): ledger groups and panels.
- **Body** (400, 0.875–1rem, 1.6): explanations and form guidance, kept near 65ch.
- **Label** (600, 0.75rem, wide tracking, uppercase): compact control and field labels only.
- **Tracking** (500, 0.625–0.75rem): request IDs, timestamps, counts, and route metadata.

**The Data, Not Costume Rule.** Mono is for actual identifiers or measurement. Ordinary navigation and prose stay in Sans.

## Layout

Staff pages use the persistent sidebar and a fluid content column with 16px mobile, 24px tablet, and 32px desktop page insets. Work groups separate by 24–32px. Ledgers use full-width rows instead of card grids when records share a schema.

Visitor access uses a centered two-panel hand-off at desktop: explanation on ink, credential task on paper. Below the large breakpoint it becomes a single vertical case file. The route timeline follows the form on mobile and occupies a stable secondary column on wide screens.

## Elevation & Depth

Most interface surfaces separate with tone or one fine rule. Important work surfaces—visitor access, the active form, and primary ledgers—use one ambient shadow with a vertical offset and broad blur. Avoid combining a pronounced shadow and a visible border on the same surface.

### Shadow Vocabulary

- **Work surface** (`0 20px 70px -42px oklch(0.18 0.03 250 / 0.5)`): active forms and ledgers that need to lift from the paper ground.
- **Visitor desk** (`0 24px 80px -36px oklch(0.18 0.03 250 / 0.45)`): the single secure-access surface.

**The One Lift Rule.** In any viewport, one surface may carry the strongest ambient shadow. Everything around it remains quieter.

## Shapes

Primary and secondary buttons are square-cornered, like labels and office controls. Forms and content panels use gently curved 10–16px corners to keep dense administrative screens approachable. Pills are limited to small statuses. Dividers are one pixel; colored side stripes and decorative frames are not part of the system.

## Components

### Buttons

- **Shape:** square, label-like control (0px radius).
- **Primary:** Ink Navy with Paper White uppercase label; 40px default height.
- **Hover / Focus:** a restrained tonal shift on hover and a two-pixel semantic focus ring with three-pixel offset.
- **Secondary / Ghost:** paper or transparent surfaces with one quiet border only when separation is needed.

### Chips

- **Style:** small status text on a lightly tinted semantic background; compact rounded pill is allowed here.
- **State:** always pair color with an explicit word such as Ready, Processing, Returned, or Approved.

### Cards / Containers

- **Corner Style:** 16px for major work surfaces, 10–12px for smaller panels.
- **Background:** Sheet White above the Paper White ground.
- **Shadow Strategy:** ambient and sparse; ledger rows remain flat.
- **Border:** use either one fine rule or shadow, not both.
- **Internal Padding:** 24–32px for major surfaces and 16–20px for dense rows.

### Inputs / Fields

- **Style:** transparent field with a single bottom rule in the core system; enclosed 10px fields are acceptable inside compact dialogs.
- **Focus:** rule shifts to the ring color and the global focus outline remains visible.
- **Error / Disabled:** state is written next to the field; destructive color is supplemental.

### Navigation

Staff navigation stays compact and text-led. Active destinations use the sidebar accent surface and remain recognizable without icon color. Visitor navigation is intentionally minimal: product mark, security statement, and the current request only.

### Route Timeline

The timeline uses a single thin vertical rule with explicit status text. Completed nodes use Approved Green, the active node reverses to paper, and future nodes remain quiet but legible.

## Do's and Don'ts

### Do:

- **Do** make the current owner, state, or next action visible within the first scan.
- **Do** prefer one ledger or route over a field of same-size summary cards.
- **Do** use real API state and honest empty states.
- **Do** preserve keyboard focus, readable contrast, and reduced-motion behavior.

### Don't:

- **Don't** fabricate storage totals, AI counts, sync health, or productivity deltas.
- **Don't** use gradients, glass, grid overlays, or glow as generic decoration.
- **Don't** place monospace on headings or ordinary navigation.
- **Don't** rely on a colored dot without a visible status word.

