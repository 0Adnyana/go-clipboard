# Slice 9 — Admin observability / analytics

**Goal:** the operator gets a first thing to *look at* — a read-only panel over what the service is
doing in aggregate, plus the operational reads slice 4 left on stdout and slice 2 left silent. It
sees public and aggregate data only, changes nothing, and is deliberately the lighter half of the
operator story: the half that can exist without a second factor, because it can neither act nor
expose anyone.

**User story:** *I open the panel and see how many clips are live, how creation splits across the
anonymous and signed-in tiers, when the sweeper last ran and how many rows it took — and nothing
about any particular person.*

> **Provenance.** Most slices inherit their decisions from `clipboard-design.md`. This one and slice
> 12 do not — that document has no operator surface, only a parked *abuse reporting channel:
> undecided*. So these notes are *ahead* of the record rather than behind it, as slices 4 and 14 also
> are, and want folding back into the design notes before anything is built. This is the read-only
> half of the operator story; slice 12 is the half that acts.

## Why it splits from abuse response

The operator story has two halves with different costs, and folding them together — as an earlier
plan did — over-charged the cheap one. *Watching* the service (live counts, tier mix, sweeper
health) exposes nobody and changes nothing, so it can ride on an ordinary admin session with a role
check and no second factor. *Responding* to abuse (reading a reported account's private clip
metadata, suspending, deleting) both exposes individuals and mutates state, so it earns the higher
bar — 2FA, an audit log, and the publishability gate, all of which slice 12 takes. Keeping the two
apart lets the observability panel land first and stay lightweight, and it names precisely which
capability the second factor is protecting rather than gating the whole surface behind it.

## Why it sits exactly here

**Not earlier, because there is nothing to observe until there is something running with accounts
in it.** Slice 4 puts the service somewhere real and sends its logs to stdout; slice 2's sweeper
runs but reports only through the health endpoint. This slice is the first that turns those into a
surface, and there is no reason to build a panel before the things it watches exist.

**Not later, because slice 12 needs the role and the panel this slice introduces.** Abuse response
is a set of actions bolted onto an operator surface; building that surface — the admin role, the
panel, the invisible-not-forbidden posture — once and read-only first means slice 12 adds buttons
and a second factor rather than inventing the whole thing behind the gate.

## What I want out of it

- **The role is a column, not a system.** One privilege level above "user", introduced here because
  this is the first slice with an operator surface. A roles table or a policy engine for a single
  bit is speculative work that makes every later query harder to read. Config-driven admin —
  privileged emails in an env var — is rejected: it puts the answer to "who may see inside the
  service" somewhere a typo grants it and nothing records that it changed.
- **Read-only, with no exceptions.** Nothing on this panel mutates state — no suspend, no delete, no
  limit dial. The moment a control changes something it belongs in slice 12 behind the second
  factor. This invariant is what lets the whole slice skip the 2FA gate honestly.
- **Public and aggregate only.** Aggregate figures — live clip count, creation rate, the
  anonymous/signed-in split, how much of the ceiling is in use, refusals from slice 3's limiter over
  time — expose no individual. Public clip content is already reachable by anyone holding the URL, so
  surfacing it here reveals nothing new. A private clip contributes to the counts and never shows its
  content, name, owner, or size to this panel; that per-clip view is a report-handling capability and
  lives in slice 12.
- **No 2FA gate, and that is the point of splitting.** Requiring a second factor to read a dashboard
  that exposes nobody would be a bar with nothing behind it — it would gate the cheap half on the
  cost of the expensive one, which is the exact mistake the split exists to avoid. The panel is still
  *invisible* rather than forbidden to non-admins, since "forbidden" confirms it exists.
- **The sweeper becomes visible.** Slice 2 makes it housekeeping that correctness does not depend on,
  which is exactly what makes its failure silent — nothing breaks, the table just grows. The panel
  shows last sweep, rows deleted, and current expired-but-unswept count, reporting what it actually
  observed rather than what it assumes. This is also the operational read that slice 2's autovacuum
  tuning is otherwise guessing at, and the signal slice 4's deferred alerting decided to answer with
  a panel rather than a page.
- **Admin auth is not a second auth system.** Same session, same login, plus a role check — a
  separate admin login is a second attack surface guarding the same database. Slice 12 adds the 2FA
  requirement on top for the admin who *acts*; reading here needs only the role.
- The reserved-name list grows again — the fourth exercise of slice 1's derivation mechanism.

## Not in this slice

- **The publishability gate.** Watching a flood is not responding to one, so this slice does not make
  the service publishable — it makes it *legible*. → slice 12 remains the gate.
- **Any mutating action.** Suspend, delete, lower a limit — all → slice 12, behind 2FA.
- **Per-account or per-private-clip drill-down.** Reading an individual's private clip metadata is a
  report-handling capability, exposes someone, and belongs behind the second factor. → slice 12.
- **Paging or search on a list of accounts or clips.** The aggregate view needs neither. → slice 12
  if it needs it at all.
- Metrics, tracing, alerting, and uptime checks. Pull-only, the same posture slice 4 set: nothing in
  this project pages anybody, and this panel is where the operator goes to look rather than being
  told.

## Still undecided

- **What the aggregate view is actually for**, beyond curiosity — which numbers change a decision
  versus which are just a dashboard. The load-bearing one is the sweeper's expired-but-unswept count;
  the rest earn their place only if they inform a config value the overview still leaves open.
- **Whether "public content is safe to show" survives slice 14.** It holds for text; arbitrary
  uploaded bytes are a different question, and files revisit this rule rather than inherit it.
