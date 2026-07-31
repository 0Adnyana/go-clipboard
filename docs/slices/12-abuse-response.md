# Slice 12 — Abuse response

**Goal:** the operator can finally *respond*. Slice 9 gave them something to look at; this gives them
something to do about it — an unauthenticated report channel, an accounts-first panel, suspension and
deletion, and an append-only record of every action. It is the publishability gate: the first slice
after which handing out the address is defensible, because a flood is now somebody's afternoon rather
than an unanswerable event.

**User story:** *A report comes in about `/notes`. I look up who owns it, see the same account has
been reported four times this week, suspend them — and the clips they were holding expire on their
own without my touching them.*

> **Provenance.** With slice 9, this closes the one question the design notes explicitly park —
> *abuse reporting channel: undecided*. These notes are *ahead* of the record rather than behind it,
> as slices 4, 9, and 14 also are, and want folding back into the design notes before anything is
> built. Slice 9 is the read-only half of the operator story; this is the half that acts, which is
> why it — and not slice 9 — sits behind the second factor and carries the gate.

## Why it sits behind 2FA, and after slice 10

Slice 9 could skip the second factor because it neither mutates state nor exposes anyone. This slice
does both: it reads a reported account's private clip metadata, and it suspends, delimits, and
deletes accounts. So it takes the higher bar. An admin acting here must have 2FA enabled, which is
why this follows slice 10. Splitting the operator surface is what makes that gate precise — the
second factor guards the capability to act and to see individuals, not the capability to read a
dashboard. It is also why the deployment of slice 4 was never the same as an exposure: slice 4 built
the road, and this is the slice that makes opening it defensible.

## What I want out of it

- **What handling a report lets an admin see.** Slice 8 accepts "the server can read private
  content" as debt, tolerable precisely because nothing acts on it; this panel is the user interface
  for that capability, which is the other reason it sits behind 2FA. Public clip content was already
  visible in slice 9. Here a private clip under report shows name, owner, size, and timestamps and
  **never** its content — not to an admin, not on a report about it. Deleting a clip never requires
  reading it. Whoever reported it already had the password; the panel doesn't need what the reporter
  had. Every such privileged read is written to the audit log below.
- **The unit of action is the account, not the clip.** The same ceiling that makes abuse tolerable
  makes most reports unactionable — a report about a clip with twenty minutes left arrives after the
  clip is gone. What survives the ceiling is the account, so the panel is organised around accounts
  and a report is evidence about an owner rather than a work item about a clip. Suspension is
  therefore partial by construction: a suspended user can still paste anonymously for 2 hours at a
  time, like anyone else.
- **Anonymous clips have no owner, and this slice has to say what happens to them anyway.** It is
  the hole left open by making the anonymous tier permanent in slice 5: a report about an unowned
  clip has nothing to accumulate against, so the account-shaped panel has no answer for the tier
  most likely to be abused. Delete-the-clip still works and is worth almost nothing at a 2-hour
  ceiling. The candidates are an IP-shaped equivalent of the account view with all the accuracy
  problems that implies, a global kill switch for anonymous creation, or leaning on slice 13's
  challenge layer to raise the per-attempt cost instead. Undecided, and it is the one thing here that
  could argue back into slice 5.
- **Account actions, made cheap by the ceiling.** Suspend (cannot create; live clips are left alone
  and expire normally), lower the creation limit, delete. There is no mass-deletion path to write
  because nothing the account made outlives tomorrow. Suspension revokes sessions — an account that
  keeps a working session for another hour is not suspended. Every mutating action carries a required
  reason.
- **Reports are unauthenticated**, because an abuse channel behind a signup wall does not get used.
  A report snapshots the name, owner, size, timestamps, and a hash of the content. **It does not
  snapshot the content.** A queue holding copies of reported text is a permanent store of exactly
  what the ceiling exists not to keep, and it would quietly become the most sensitive thing in the
  system. The cost is accepted: act inside the clip's lifetime, or act on the pattern instead of the
  payload.
- **The report form is rate-limited, or the queue is a spam sink.** It registers into slice 3's
  mechanism keyed on client IP — an unauthenticated form offers no other handle — and keeps that
  slice's in-memory storage unchanged, since a flood of reports is adversary-chosen volume and
  losing the counters on a deploy costs nothing anyone would notice. It is the one limit whose
  default can be strict without ever inconveniencing a real user, because reporting is a rare and
  deliberate act rather than part of any flow. It is also not the whole answer: IPs are free here
  exactly as they are for the anonymous tier, so a report flood from many sources is the same shape
  of problem slice 13's challenge layer exists to meet, and the fallback until then is that the queue
  is pull-only — a spammed queue degrades into an operator ignoring it rather than into an outage.
- **An append-only audit log**, which is the first thing here with no ceiling over it — every other
  record of something *happening* is bounded by expiry. It cannot be, because it is the only
  surviving evidence of anything the panel touched; the clip in question was deleted the same day. It
  also holds the privileged reads slice 9 deliberately arranged never to need — reading a private
  clip's metadata here *is* a privileged act, unlike reading public content there. Append-only in the
  strongest sense that is cheap: no update or delete path exists at all, so an after-the-fact edit is
  not expressible rather than merely discouraged.
- **Admin auth is still not a second auth system.** Same session, same login, and the role check
  slice 9 introduced — plus, for the admin who *acts*, the 2FA requirement this slice adds, which is
  why it sits after slice 10. The panel stays invisible rather than forbidden to non-admins, since
  "forbidden" confirms it exists and tells a credential-stuffer what the prize is.
- Promoting the first admin is a documented step, not an undocumented `psql` session.
- The reserved-name list grows again if the report form turns out to need a path of its own.

## Not in this slice

- Paging or search on the accounts list. The ceiling bounds clips, not users.
- Recovering reported content once the clip has expired. Repeat reports against one account are the
  only durable signal — the alternative is storing exactly what the ceiling exists to discard.
- Notifying an admin that a report arrived; the queue is pull-only.
- An appeal path for a suspended account beyond whatever the abuse contact turns out to be.
- A challenge on the report form, or any adaptive humanity check. → slice 13, where the challenge
  layer is built as its own tier.

## Still undecided

- **Admin bootstrap.** A promote subcommand, a documented one-off SQL statement, or a first-run flag.
  Whichever it is, it needs writing down, because the first admin is the one privilege grant with no
  audit entry to point at.
- **Audit log retention.** The first thing in this project that outlives a day. A fixed window keeps
  the "nothing here persists" story nearly true; keeping it forever is defensible for a log of
  privileged reads but is a promise about a table nobody is watching.
- **What deleting an account does to its clips.** Cascade, or leave them to expire. Leaving them is
  cheaper and consistent with suspension, but then a deleted account's content briefly outlives the
  account and audit entries point at a user row that is gone.
- **Whether the abuse contact is an address or a form.** The in-app report form answers this for
  clips but not for the legal- or DMCA-shaped mail that arrives regardless of what the UI offers.
