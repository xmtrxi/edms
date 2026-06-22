# EDMS Frontend

Electronic Document Management System — frontend built with [Nuxt 4](https://nuxt.com), [Vue 3](https://vuejs.org), [Tailwind CSS v4](https://tailwindcss.com), and [shadcn-vue](https://www.shadcn-vue.com).

## Tech Stack

| Layer | Tool |
|-------|------|
| Framework | Nuxt 4 (Vue 3 + TypeScript) |
| Styling | Tailwind CSS v4 + `tw-animate-css` |
| UI Components | shadcn-vue, Reka UI, Vaul Vue |
| Forms | VeeValidate + Zod |
| Icons | Lucide Vue (via `@nuxt/icon`) |
| Carousel | Embla Carousel Vue |
| Utilities | VueUse, Class Variance Authority, clsx, tailwind-merge |
| Linting | Oxlint |
| Formatting | Oxfmt |

## Prerequisites

- [Node.js](https://nodejs.org) >= 18
- [pnpm](https://pnpm.io) (recommended) — install via `npm install -g pnpm`
- [Oxlint / Oxfmt](https://oxc.rs) — installed as dev dependencies (no global install needed)

## Setup

```bash
pnpm install
```

This runs `nuxt prepare` automatically after install to generate TypeScript declarations.

## Development

```bash
pnpm dev
```

Starts the dev server on `http://localhost:3000` with hot module replacement.

## Build & Preview

```bash
pnpm build       # Production build
pnpm preview     # Preview production build locally
pnpm generate    # Static site generation
```

## Code Formatting

Formatting is handled by **Oxfmt**, a fast Rust-based formatter:

```bash
pnpm fmt          # Format all files in-place
pnpm fmt:check    # Check formatting (CI-friendly)
```

Oxfmt is configured via `.oxfmtrc.json` at the project root.

## Linting

Linting is handled by **Oxlint**, a fast Rust-based linter:

```bash
pnpm lint         # Check for lint errors
pnpm lint:fix     # Auto-fix lint errors
```

Oxlint is configured via `.oxlintrc.json` at the project root.

## Project Structure

```
├── app/                    # Application source
│   ├── components/         # Vue SFC components
│   ├── composables/        # Vue composables
│   ├── layouts/            # Nuxt layouts
│   ├── pages/              # File-based routing
│   ├── server/             # Nitro server routes / API
│   └── app.config.ts       # Runtime app config
├── components.json         # shadcn-vue configuration
├── nuxt.config.ts          # Nuxt configuration
├── oxfmtrc.json            # Formatter config
├── oxlintrc.json           # Linter config
├── package.json
├── pnpm-lock.yaml
├── pnpm-workspace.yaml
└── tsconfig.json
```

## Conventions

### Vue Components
- Single File Components (SFC) with `<script setup lang="ts">`
- Component filenames in **PascalCase** (e.g. `UserAvatar.vue`)
- Prefer Composition API over Options API

### TypeScript
- Use TypeScript in all `.vue` SFCs and `.ts` files
- Prefer arrow functions with explicit return types:

```ts
const funcName = async (param1: type): Promise<type> => {}
```

### Styling
- Use Tailwind utility classes
- Avoid scoped styles unless absolutely necessary
- Use `cva()` (class-variance-authority) for component variants

### shadcn-vue
- Components are managed via the shadcn-vue CLI
- Add new components: `npx shadcn-vue add <component>`
- Configuration lives in `components.json`

## Useful Commands

```bash
pnpm dev                  # Start dev server
pnpm build                # Production build
pnpm fmt                  # Format code with Oxfmt
pnpm fmt:check            # Check formatting
pnpm lint                 # Lint with Oxlint
pnpm lint:fix             # Lint and auto-fix
pnpm nuxt prepare         # Generate TypeScript types
```

## Learn More

- [Nuxt Documentation](https://nuxt.com/docs)
- [Vue 3 Documentation](https://vuejs.org/guide)
- [Tailwind CSS v4](https://tailwindcss.com/docs)
- [shadcn-vue](https://www.shadcn-vue.com)
- [Oxc Tools](https://oxc.rs)
