# Slice 13 — Challenge layer

**Goal:** a challenge gate — a CAPTCHA or equivalent proof-of-humanity — as a *third* defence tier on
top of rate limiting and abuse response. It fires on two surfaces where the actor is anonymous and
the cost of being wrong is real: anonymous account registration when the signals look suspicious, and
anonymous clip creation. It is deliberately late, because a challenge is the tier you reach for once
the cheaper two are in place and shown to be insufficient — not the one you start with.

**User story:** *A burst of sign-ups starts arriving from one corner of the internet faster than any
human could type. Instead of being refused outright — which a script just retries — they meet a
challenge, and the ones that are scripts stop there while a real person barely notices.*

## Why it is a third tier, and why here

Two defences already exist and each leaves the same gap. **Rate limiting** (slices 3 and 5) is the
first tier: it caps volume per key, but IPs are free, so a distributed source walks through it a thin
slice at a time. **Abuse response** (slice 12) is the second: it acts after the fact and on accounts,
so the anonymous tier — with nobody to suspend — slips past it too. The challenge layer is the third
and orthogonal one: it raises the *per-attempt* cost for a suspected non-human *before* the action
happens, which is precisely the gap the other two leave open on the anonymous tier.

**Not earlier, because a challenge is friction on the core flow**, and the whole product is a
ten-second handoff. Interposing it before the cheaper tiers are proven insufficient taxes every real
user to stop an abuser who a limit might have stopped for free.

**After slice 12, because deciding a flood is happening is a prerequisite for challenging it.** The
signals this slice keys on are the same ones slice 9's panel surfaces and slice 12's queue reacts to,
so the operator has to be able to see and respond to a flood before it makes sense to interpose a
challenge automatically.

## What I want out of it

- **A challenge on anonymous account registration — but only when activity looks suspicious.** Not
  every sign-up; a challenge on the happy path is a tax on growth. It fires on signal: registration
  velocity from an IP or subnet, a limiter already tripping, whatever slice 9 surfaces as anomalous.
  Adaptive, not blanket.
- **A challenge on anonymous clip creation.** This is the one surface with nobody to hold accountable
  and no account to suspend, so the tier that works after the fact never applies to it. A challenge
  is the only lever that raises cost on the anonymous tier without closing it — and the overview
  keeps that tier permanently open, so a lever that taxes rather than shuts is exactly the right
  shape.
- **Signed-in actions are exempt.** An account is already a cost paid and a handle to act on;
  challenging a signed-in user re-charges them for it. The challenge is for the tier that has no
  other accountability, which is the same reason abuse response is account-shaped and this is not.
- **It composes with the limiter rather than replacing it.** The limiter still runs. The challenge is
  what a suspicious-but-under-the-limit request meets, and what a refused request can be offered
  *instead of* a flat 429 — a way back for the real user caught by a coarse key, rather than only a
  wall.
- **The provider is a boundary, not a dependency baked through the app.** Whatever verifies a
  challenge — a third-party CAPTCHA, a hosted widget, a proof-of-work — sits behind one interface and
  is chosen by configuration, the same way slice 3 keeps its storage decision local. Verification is
  inline and nothing about it is durable, so it adds no table and keeps the "Postgres and nothing
  else" story intact even though the verifier itself may be an external call.
- **It fails toward refusal, not toward open.** If the challenge provider is unreachable, a
  *challenged* request is refused rather than waved through — the request was already suspicious, so
  failing open would hand the abuser exactly the bypass the tier exists to close. Unchallenged
  traffic is unaffected, so a provider outage degrades into stricter-than-usual rather than into a
  general outage.

## Not in this slice

- **Challenging signed-in users, or any authenticated flow.** The account is the handle; use it. →
  never, on the current model.
- **A challenge on login.** Credential stuffing is slice 5's IP-plus-email limiter's job. A challenge
  goes there only if that limiter proves insufficient — noted, not built.
- **Building a CAPTCHA.** Third-party or proof-of-work behind the interface; a homegrown humanity
  check is a research project, not a slice.

## Still undecided

- **What "looks suspicious" actually keys on**, and whether it can be expressed without the durable
  per-key state slice 3 deliberately avoided — the signal is the whole difficulty of the adaptive
  path.
- **Which provider**, and whether a self-hosted proof-of-work beats a third-party CAPTCHA given the
  project's standing preference for few dependencies and no external services it does not need.
- **Whether anonymous creation should be challenged always or only adaptively.** The same
  happy-path-tax question as registration, but harder: anonymous create *is* the product's core
  gesture, so the bar for taxing it is higher than for a sign-up.
- **Whether a challenge is even the right lever**, the same honest doubt slice 3 records about rate
  limiting. A cost attached to creation, slice 12's kill switch, or simply accepting the ceiling as
  the anonymous tier's only policing have not been weighed against a challenge here.
