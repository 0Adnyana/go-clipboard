# go-clipboard frontend

React + TypeScript frontend for [go-clipboard](../README.md), built with Vite, Tailwind v4, shadcn/ui, and TanStack Router. It renders the stack status page today and owns the user-facing URL namespace (`/$slug`).

Run it through the root `make dev`, not on its own: the browser origin is the Caddy proxy on `:3000`, and requests to `:5173` bypass it — relative `/api/*` calls then hit Vite and come back as `index.html`.

## Layout

| Path | Purpose |
| --- | --- |
| `src/routes/` | File-based routes; `routeTree.gen.ts` is generated and committed |
| `src/components/ui/` | shadcn components, added through the CLI rather than by hand |
| `src/lib/api.ts` | The only place that calls `fetch` or constructs an API URL |
| `src/index.css` | Single `@import "tailwindcss"` plus the `@theme` tokens; there is no `tailwind.config.js` |

## Conventions

- **API access goes through `src/lib/api.ts`.** Components never call `fetch` and never name an upstream host — an absolute URL would bypass the proxy and reintroduce CORS.
- **`tanstackRouter()` is registered before `react()`** in `vite.config.ts`. The reverse order breaks route generation with no error.
- **The dev port is pinned** with `strictPort`, so an occupied `:5173` fails loudly instead of moving and leaving the proxy pointed at nothing.

## Scripts

```bash
pnpm dev      # dev server on :5173 (prefer `make dev` from the repository root)
pnpm build    # type check and production build
pnpm test     # type check only; there are no unit tests yet
pnpm lint     # oxlint
```
