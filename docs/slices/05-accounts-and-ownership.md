# Slice 5 — Accounts and ownership

**Goal:** accounts arrive, clips get an owner, and signing in unlocks the lifetimes above the
anonymous 2-hour cap. Creation does **not** close — the anonymous tier is permanent.

**User story:** *I sign up with an email and a password, and from then on my clips are mine — nobody
else can extend or take over a name I'm holding — and they can last a day instead of two hours.*

## What I want out of it

- Registration, login, logout, and sessions as cookies on a single origin. The proxy needs no
  changes — no CORS, no cookie-domain juggling — which is the payoff from the skeleton's one-origin
  decision.
- **Signing in unlocks lifetime, not creation.** This is the point of the slice. Anyone can still
  paste without an account, capped at 2 hours; what the account buys is the presets above that cap,
  up to the 24-hour ceiling, plus ownership. The account becomes something a user wants rather than a
  toll gate on the core flow — paid for by leaving one tier of the service with nobody to hold
  accountable. See *Still undecided*.
- **Clips have an owner, and ownership means something.** A second account cannot extend or delete a
  clip the first is holding. Taking over an *expired* clip is still allowed regardless of who owned
  it — that is what makes the namespace a lease pool.
- **An anonymous clip has no owner, permanently.** Nobody owns it, so extend stays available to
  anyone holding the URL exactly as in slice 2, and a signed-in user gets no special power over it.
  "Unowned" is a normal, lasting state rather than a transitional one, which is what the next section
  is about.
- **Lazy email verification.** Unverified accounts may create. The ceiling means an unverified
  spammer's output is gone by tomorrow, and putting an inbox round-trip inside the core flow defeats
  the point of a clipboard you open to move text in ten seconds. This slice only records that the
  email is unverified; it never checks it.
- **Enumeration-safe registration and login.** Neither reveals whether an address already has an
  account.
- **Two more limiters, registered into slice 3's mechanism rather than built here.** Login attempts
  keyed on email plus IP (credential stuffing), and clip creation keyed on the account. Slice 3's
  IP-keyed creation limiter keeps standing, because anonymous creation still exists — it is
  supplemented, not replaced, which also means the weakest limiter in the system is the permanent
  one.
- **Login is the first limiter with an argument for durable state, and slice 3 deliberately left the
  question here.** Two things separate it from the anonymous limiters. Its economics are different:
  a login attempt already pays for a password hash verification tuned to take tens or hundreds of
  milliseconds, next to which a row upsert disappears — the write objection that decided slice 3
  barely registers. And its stakes are different: a lockout that resets on every restart is a
  credential-stuffing window that a deploy hands out for free, and slice 4 makes deploys routine.
  If it does need to survive one, `UNLOGGED` Postgres is the first thing to reach for rather than a
  logged table — slice 3 recorded it as exactly this upgrade path. In-process memory stays the
  default until the case is actually made.
- The reserved-name list grows with the new auth pages, through the single derivation mechanism from
  slice 1 — the first real test of whether that mechanism works.

## Ownership is nullable, and stays that way

The ownership column looked like the expensive part of having gone anonymous first — a nullable
`owner_id` with nothing correct to backfill — and the plan was to add it nullable, wait out the
ceiling, then make it required. A permanent anonymous tier removes both the cost and the plan: the
column goes on nullable and never becomes required, because unowned clips keep arriving.

That trades a one-time migration for a standing obligation. `owner_id IS NULL` means *nobody owns
this*, not *we haven't backfilled yet*, so every ownership check has to answer it deliberately —
and the default answer differs per operation. Extend on an unowned clip is allowed to anyone;
overwrite and delete in slice 7 are allowed to nobody. Getting the NULL case wrong is now a live bug
rather than a migration artefact.

## Not in this slice

- Password reset, so a forgotten password is a lost account. Tolerable only because of what an
  account protects — *longer lifetimes and the names you are currently holding*, not any stored
  asset, and the fallback is the anonymous tier rather than nothing. → slice 6
- Sending any mail at all, and reading the verification flag. → slice 6
- Any way to see your own clips; the create form is the entire authenticated UI. → slice 7
- 2FA. Deliberate sequencing: 2FA protects an account whose whole asset base evaporates within a day.
  → slice 10

## Still undecided

- **Where abuse reports go.** This is the first slice where there is finally someone to hold
  accountable and still no way to act on it. → the gap slice 12 closes.
- **What abuse response looks like for the anonymous tier**, where there is no account to suspend
  and the only handles are an IP and a 2-hour ceiling. Deliberately left open here; slice 12 is where
  it has to be answered, and it is the main unresolved cost of keeping anonymous creation permanent.
- Whether sessions are database rows or signed tokens. Rows make slice 6's "invalidate every session"
  trivial, so decide with that in view.
- **Whether login lockout actually needs to survive a deploy**, and therefore whether this is the
  slice that gives the project its first durable limiter. It is the one limiter with a real argument
  for it; the argument has not been closed.
- **What the per-account creation limit is for.** Anonymous creation is already limited and stays
  open, so an account-keyed limit does not reduce what a determined abuser can paste — it exists so
  that slice 12 has a dial to turn on a specific account. Whether it needs a default worth enforcing,
  or only a floor and a lever, is a question for the slice that builds the lever.
- **Saving a clip's content as a permanent snapshot.** A registered user could save a snapshot of a
  clip's *content* — not the clip itself — so it outlives the lifetime ceiling, and later spin up a
  fresh clip from that saved content. This keeps the lease-pool model intact: the name and its
  2-to-24-hour lease still expire on schedule and return to the pool, and what persists is a stored
  asset the user owns, separate from the namespace. It is the first thing an account would protect
  that *is* a stored asset, which cuts against the "an account protects lifetimes and names, not any
  stored asset" framing that makes password reset skippable in slice 6 — so it can't land without
  reopening that. Optionally, a per-clip setting could let unregistered holders of the URL save the
  content too; that needs somewhere for an anonymous saver to keep it, which likely means it only
  makes sense once there is a signed-in place to put it. Attractive, but it introduces durable
  per-user storage the project has so far avoided, so it wants its own slice rather than a corner of
  this one.
