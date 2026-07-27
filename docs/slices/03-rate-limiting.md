# Slice 3 — Anonymous rate limiting

**Goal:** the anonymous tier stops being defenceless. Slices 1 and 2 assert nothing about abuse; this
slice builds a limiter over the two anonymous surfaces that exist by now, and shapes it behind one
interface so that every later limiter is a registration rather than a second invention.

**User story:** *I leave the local milestone running on a public address for an afternoon. Someone
finds the create form and hammers it, gets refused, and the table does not grow. I never hear about
it, which is the point.*

> **This is the first of the two slices with no user-visible feature**, and slice 4 is the other —
> they are adjacent for a reason, since a limiter with nowhere to run and a deployment with no
> limiter are each half of the same milestone. Its success condition is that nobody notices it, which
> is more than can be said even of slice 11, which is at least operator-*facing*. The reason it is a
> slice rather than a footnote inside slice 1 is sequencing, below, and the reason it is not inside
> slice 5 is that slice 5 ships a login form.

## Why it sits exactly here

**Not earlier, because slices 1 and 2 have nothing worth protecting.** They are explicitly a local
milestone — nothing is exposed, so a limiter written alongside them would be speculative work with no
call site. The two slices are corrected rather than refactored: they currently *claim* an IP-keyed
limiter that would not have existed, and saying plainly that they have none makes their
not-deployable warning honest.

**Not later, because slice 5 introduces login.** The anonymous limits this slice builds are a
capacity problem — *someone fills the namespace*, which the 2-hour ceiling already bounds. A
credential endpoint is not: its limiter's absence is a vulnerability. So this is the last position
where a limiter is new infrastructure rather than a retrofit onto a live login route, which makes it
the only correct position rather than a convenient one. What that limiter should *be* is slice 5's
question, not this slice's.

**What it does not do is make anything deployable.** A limiter is prevention, not abuse *response*:
there is no kill switch, no view of who is being refused, and IPs are free, so a distributed flood
still lands. Slice 4 goes on to build a deployment regardless, on the narrower ground that a service
nobody has been told about is not an exposure — but slice 11 remains the deployability gate, and its
open question survives both slices untouched.

## What I want out of it

- **Two registrations, and both of them anonymous and keyed on client IP.** Anonymous creation, and
  slice 2's availability hint. Sharing a key is the whole reason this is tractable here: identity,
  storage, and refusal get built once, against the only tier that has nobody to hold accountable.
- **One interface, roughly *may this key act, and if not, for how long*.** It exists so that the
  limiters slices 5, 8, and 11 need are a constructor call rather than a rewrite of five call sites —
  and so that each of them keeps its own storage decision, which is the point of the section below.
  Shaping the interface is in scope here; deciding what the other limiters key on is not.
- **State lives in process memory, and that is a decision rather than a default.** See below — the
  short version is that the two limiters landing here are the two that face volume chosen by an
  adversary and the two that lose nothing when the process restarts. Durable storage is the
  identified upgrade path if a later limiter needs it, in the same sense that `chi` is the upgrade
  path for routing.
- **Bounded memory from the first version, not as later hardening.** A map keyed on client IP grows
  with the number of distinct attackers, so a distributed source — or one host walking an IPv6
  allocation — turns the limiter into an unbounded allocation and hands over the failure it was
  installed to prevent. A hard cap with eviction is part of the feature, not a refinement of it.
- **Which client is being limited is a configured question, not a constant.** Behind Caddy,
  `RemoteAddr` is always the proxy, so the limiter reads a forwarded header — and it must trust that
  header only from the hop it knows about. Trusting it blindly means the limiter is bypassed by
  setting one header, which is worse than having none, because it looks like protection. A future
  deployment behind a CDN or a second proxy changes which hop is trustworthy, so this is
  configuration.
- **IPv6 is bucketed to a prefix, not the full address.** A single subscriber is routinely handed a
  /64 or larger. Keyed on /128, an attacker gets as many keys as they care to use while IPv4 users
  stay constrained normally — an asymmetry that quietly makes the limiter decorative on half the
  internet.
- **A refusal is a 429 with `Retry-After`, in the existing JSON error shape**, and it says how long
  rather than only that something was refused. It reveals nothing about the limit's key or size.
- **The numbers are configuration.** Every limit and window is a config value with a default, because
  none of them can be chosen correctly before the service has seen traffic.

## Where the state lives

The overview framed this as one question — *in-process memory is free and dies with the process;
Postgres is durable and adds a write to every attempt* — and applied it to all five limiters at once.
The five do not have the same economics, so this slice answers it for its own two and leaves the
other three to the slices that build them.

**Measured against what the guarded operation already costs, the write objection nearly vanishes for
creation.** It is already an `INSERT` in a transaction, so a limiter write could simply join it. The
availability hint is the opposite case: a cheap indexed `SELECT` fired as the user types, so a
durable counter would cost more than the thing it limits. And both already need the database, so a
Postgres-backed limiter would add no new dependency and no new failure mode — when the database is
down, everything it guards is down anyway.

**Two arguments push back, and they are why this slice chooses memory.** Bloat is already the named
operational risk, and repeated `UPDATE`s against a small hot table generate dead tuples faster than
the insert-and-delete churn slice 2 tunes autovacuum for — so a durable limiter means a second table
with the same silent failure mode. More decisively, a durable limiter makes every *rejected* request
a write, so its cost scales with the attack and the limiter amplifies the load it exists to absorb.
Both objections land hardest on a limiter facing adversary-chosen volume, which is exactly what an
anonymous IP-keyed limiter is — and neither of these two loses anything worth keeping when the
process restarts, since the attacker has to start their window over too.

**Options considered and not taken:**

- **`UNLOGGED` Postgres.** The real middle ground: no WAL, survives clean restarts and deploys,
  truncated only on crash recovery — close to the durability actually wanted, since losing counters
  after a crash is fine and losing them on every `air` reload is not. Held in reserve as the upgrade
  path for whichever limiter first needs to survive a deploy, which is not either of these.
- **Deriving it from the clips table.** Tempting and nearly free: the ceiling already makes `clips` a
  self-expiring log of recent activity, so a creation limit is a `count(*)` over a window with no new
  table, no writes, and no cleanup. It fails on TTL choice — a user picking the shortest preset has
  their rows swept before the rate window closes, so the cheapest evasion is short-lived clips.
  Closing that needs tombstones, which touch the atomic-claim path, and idea 3 is not worth
  disturbing to save a table.
- **A single row per key, GCRA-style**, holding one theoretical-arrival-time timestamp. Exact, no
  fixed-window seam, one row per key regardless of traffic. The right shape *if* any of this becomes
  durable, which is why it is recorded here rather than decided.
- **Rate limiting in Caddy.** Right instinct — stop a flood before it reaches Go — but it needs a
  third-party module and therefore a custom build, breaking the `brew install caddy` prerequisite,
  and it cannot see accounts, cannot key on email, and cannot answer in the product's error shape.
  If the anonymous tier is ever genuinely flooded the answer is upstream, and this is where it goes.

Keeping the storage decision behind the interface is what makes it local to each limiter and
reversible per limiter. That is the cheap insurance worth buying regardless of how the rest lands.

## Not in this slice

- **Every limiter that is not anonymous and IP-keyed**, including the storage question for each of
  them. Login attempts and per-account creation → slice 5. Per-clip password attempts → slice 8,
  which takes the interface and declines the storage. The unauthenticated report form → slice 11.
- Durable state of any kind, and therefore anything that survives a deploy — which slice 4 turns into
  a routine event rather than a rare one, since every push restarts the process.
- A kill switch, an IP-shaped admin view, or any way to *respond* to a flood in progress. → slice 11
- Multi-instance correctness. In-process state is per-process, which is a new entry under the
  overview's standing single-instance debt rather than a new problem.

## Still undecided

- **Every number.** Requests per window, window length, the memory cap, and how long a refusal asks
  the caller to wait. They belong with the overview's other unfilled numbers, and none of them can be
  picked honestly before the create endpoint has seen traffic.
- **Which forwarded hop is trusted**, expressed as configuration here, but the value depends on a
  deployment topology that does not exist yet. → slice 4, which creates one and inherits the question
  along with it.
- **Whether the availability hint needs a limit distinct from creation's**, given that it is the
  cheaper request and the better enumeration oracle of the two.
- **Whether rate limiting is even the right lever for the anonymous tier.** The overview is already
  honest that IPs are free and that this tier has nobody to hold accountable. A limiter is the
  assumed answer rather than a chosen one, and the alternatives — a cost attached to creation, or
  slice 11's kill switch — have never been weighed against it.
