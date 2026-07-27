# go-clipboard — overview

**What this is for.** The slices in `docs/slices/` say *what gets built and in what order*. This says
*what the thing is and why*, briefly. If the two disagree, this is the intent and the slice gets
corrected. Detail lives in the slices; don't duplicate it here.

## What it is

An online clipboard in the shape of [cl1p](https://cl1p.net): paste text under a name on one device,
type `host/<name>` on another, get it back. Neither end needs an account — the URL *is* the handoff.
Anonymous clips last up to 2 hours; signing in buys up to 24.

Two motives, pulling different ways: a ten-second cross-device handoff that then disappears, and a
deliberate exercise in hand-building session auth, TOTP, and OAuth. The learning motive is a valid
reason for a decision here (it is why passkeys are set aside) as long as it is named as the reason.

## The five ideas everything follows from

1. **Text round-trips exactly.** Not trimmed, normalised, or re-wrapped. Payloads that can't
   round-trip are refused loudly rather than mangled.
2. **Nothing outlives 24 hours from creation; nothing anonymous outlives 2.** Absolute, not sliding,
   not extendable past the ceiling. This is the big simplifier: it makes open creation survivable and
   version history, mass deletion, and early 2FA unnecessary.
3. **Names are a lease over one flat pool, not a registry.** Live names are held, expired ones are
   freely claimable, claiming is atomic and is *not* the same operation as editing. `host/notes`,
   never `host/alice/notes` — the link is the product.
4. **One origin.** Caddy splits `/api/*` from everything else. Cookies just work, CORS never exists,
   and the top-level path namespace belongs to the frontend, not Go.
5. **Say as little as possible.** Auth flows never reveal whether an address is known; acting on
   someone else's clip looks like the clip doesn't exist; nothing is indexed. Private clips are the
   one exception — the password prompt admits existence, and nothing more.

## Features

| Feature | Slice | What it is |
| --- | --- | --- |
| Paste and read | [1](slices/01-paste-and-read.md) | Create form, read page at `/<slug>`, copy button, size cap, case-insensitive names |
| Lifetime | [2](slices/02-lifetime-you-can-see.md) | User-chosen TTL, server-driven countdown, explicit extend bounded by the ceiling |
| Anonymous rate limiting | [3](slices/03-rate-limiting.md) | IP-keyed limiter over anonymous creation and the availability hint, behind one interface later slices register into |
| Deployment **(MVP)** | [4](slices/04-deployment.md) | Commit-tagged image, production Caddyfile, CI that ships what it tested, migrations as a step |
| Accounts and ownership | [5](slices/05-accounts-and-ownership.md) | Email/password accounts, cookie sessions, `owner_id` on clips, longer lifetimes, per-account limits |
| Getting back in | [6](slices/06-getting-back-in.md) | Email verification (lazy) and password reset; completing a reset evicts every session |
| My clips | [7](slices/07-my-clips.md) | Dashboard of live clips with countdowns, in-place overwrite that doesn't touch the TTL, delete-now — all owner-only |
| Private clips | [8](slices/08-private-clips.md) | Server-checked password on a clip, throttled per clip; read-only access, owner must exist, so it requires an account |
| Burn-after-read | [8](slices/08-private-clips.md) | Deleted on read, click-to-reveal so a bare `GET` never destroys anything; works on anonymous clips too |
| Two-factor | [9](slices/09-two-factor.md) | TOTP with hashed single-use recovery codes |
| OAuth | [10](slices/10-oauth.md) | Provider sign-in that auto-links only when both sides assert a verified email |
| Admin and abuse response | [11](slices/11-admin-and-abuse-response.md) | Unauthenticated reports, accounts-first panel, suspension, append-only audit log; never captures private content |
| Files | [12](slices/12-files.md) | A file is a clip: same pool, lease, ceiling, and `/<slug>`. Text or file, never both |

Anonymous pasting is a permanent tier, not a stage to grow out of. An account buys four things:
lifetimes past 2 hours, a name only you can change or take back, a list of what you're holding, and
private clips.

## The stack, and why

- **Go, stdlib `net/http`.** Few routes; `chi` is the upgrade path if that changes.
- **Postgres, developer-provisioned.** Consumes `DATABASE_URL` and nothing else. No Redis, no queue —
  anything that persists lives in Postgres.
- **`pgx/v5`, `sqlc`, `goose`.** `sqlc` reads the migrations dir so generated code can't drift.
  Migrations run only when invoked, never on startup; health reports pending ones.
- **Caddy owns the origin.** Dev topology equals prod minus TLS and a build step, and it keeps static
  delivery, SPA fallback, and the `/<slug>` catch-all permanently out of Go.
- **Docker for the deployed app only** (slice 4). The dev loop stays Homebrew, `make`, `air`.
- **React, TypeScript, Vite, Tailwind v4, shadcn/ui, TanStack Router.** File-based routes; `$slug`
  encodes the user-owned namespace. Thin `fetch` client, relative paths only.
- **`make` + `air` + `pnpm`, unit tests only, mail to stdout in dev.**

## Scope

**MVP is slice 4.** Slices 1–4 are the minimum that counts as done: anonymous paste and read, a
lifetime you can see, a limiter that makes an unadvertised host survivable, and a path from commit to
a real hostname. Everything after that is accounts and the learning motive.

**In.** Text (files last, slice 12). Two browsers, or a browser and a terminal. One instance, one
region, one database. Both ends completable signed-out.

**Never** — closed, not parked: edit history or undo; password recovery for a *clip*; recovering
reported content after expiry; mass deletion of an account's clips; protection from the server
operator (client-side encryption is declined in slice 8); static delivery or SPA fallback inside Go.

**Deferred with intent.** Passkeys, trusted-device 2FA, app-secret rotation, admin paging and search,
TanStack Query, encryption at rest (internal-only, so it's cheap to add later).

**Nothing before slice 11 is publishable** — which since slice 4 is no longer the same as deployable.
Slice 4 is the MVP and the deployable gate; slice 3 slows an anonymous flood; slice 11 is the first
slice that lets the operator *respond* to one.

## Caveats and accepted debt

- **The server can read private clip content.** Plaintext plus a server-checked password (slice 8);
  slice 11 then builds a UI for that capability. Mitigated structurally, but permanent in practice.
- **A flat namespace plus private clips makes some collisions unexplainable.** The refusal message is
  identical in every case, so nothing leaks — the user just has nothing to do but pick another name.
- **The read path is a slug oracle**, and the availability hint is a purpose-built enumeration
  endpoint. Slice 3 slows it without closing it.
- **Extending is racy against reclaim.** A second late and the name may be gone.
- **Bloat, not size, is the operational risk.** 100% insert-and-delete churn outruns default
  autovacuum; the sweeper fails silently, so slice 11 surfaces it.
- **The audit log is the only table with no ceiling over it.**
- **The anonymous tier has nobody to hold accountable, permanently.** IPs are free, suspension is
  partial by construction, and slice 11's panel is account-shaped.
- **`owner_id` is nullable forever**, so every ownership check decides what NULL means: extend yes,
  overwrite and delete no.
- **Single-instance by construction.** No advisory lock, one sweeper, in-process limiter state. A
  second instance would double every limit; a deploy is a brief outage and drops limiter state.

## Open questions

Each is elaborated where it lands.

**Numbers.** Content size cap? Slug charset and length bounds — are 1–2 character slugs claimable?
The two TTL preset sets? Does the size cap differ by tier? → slices 1, 2, 3

**Anonymous abuse response.** What does it look like with no account to suspend? → slice 11 (the one
question that could argue back into slice 5)

**Mechanisms.** Sessions as rows or signed tokens? → slice 5. In-place edit or replace-contents, and
Tab in the textarea? → slice 7. Grace period on reclaim? → slice 2. Does any limiter need durable
state? Slice 3 says no for the anonymous ones and leaves each later limiter to answer for itself →
slice 5.

**Operations.** Where does it run, and is production Postgres managed or a container? Which forwarded
hop is trusted? → slices 3, 4. Admin bootstrap? Audit log retention? What does deleting an account do
to its live clips? → slice 11.

**Still unowned.** Is `curl host/notes` a first-class interface — does the read URL serve raw text to
a non-browser client? (Half-answered: a bare `GET` never destroys anything.) → slices 8, 12. Is there
a legal or takedown posture beyond the abuse queue? → slices 11, 12.
