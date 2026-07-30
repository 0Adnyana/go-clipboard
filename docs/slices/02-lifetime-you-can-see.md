# Slice 2 — Lifetime you can see

**Goal:** lifetime stops being a constant and becomes something the user picks and watches. The
create form grows up.

**User story:** *I'm moving a password, not notes, so I pick 10 minutes instead of a day — and I can
see it counting down.*

## What I want out of it

- **TTL presets, anchored absolutely to creation.** If the user picks 10 minutes, 10 minutes means
  10 minutes. No sliding TTL, no reset-on-write — either of those makes the ceiling fiction, since a
  clip polled by a bot would live forever.
- **Every preset here fits inside 2 hours**, because everything created in this slice is anonymous.
  The presets above that cap arrive with accounts in slice 5, so the choice to make now is how the
  cap is expressed: the form should read as *this is the anonymous limit* rather than *this is the
  limit*, or slice 5 has to rewrite it.
- **A countdown on the read page driven by the server's expiry**, not a timer started when the page
  loaded, plus a clear expired state.
- **An availability hint on the create form, advisory only.** It never reserves anything —
  correctness stays in the claim path, and two users told "available" must still race into the claim
  with exactly one winner. It exists purely so nobody types a paragraph and then learns the name is
  taken. It needs its own rate limit, because unthrottled it is a slug enumeration endpoint over the
  live namespace — and it ships here without one, which is half of why slice 3 exists.
- **A sweeper that deletes expired rows.** Pure housekeeping — the claim path already handles
  reclaim, so stopping the sweeper must not break correctness. What it buys is making "this table
  never holds more than 24 hours of writes" actually true.
- **Autovacuum tuned for this table.** A small table that is 100% insert-and-delete churn produces
  dead tuples faster than the defaults reclaim them. The operational risk here is bloat, not size.

## Not in this slice

- A rate limit on anything, including the enumeration endpoint this slice just added. → slice 3
- Anything clever about suggested alternative names; `notes2` is fine.

## Still undecided

- The exact preset set. Something like 10 minutes / 1 hour / 2 hours for anonymous clips, with the
  longer ones — up to 24 hours — unlocked by signing in from slice 5 onward.
- **A grace period on reclaim.** Expired slugs are instantly claimable, so someone racing to reclaim
  a name a second too late finds it gone. A short window where the slug is dead but not yet claimable
  would fix that.
