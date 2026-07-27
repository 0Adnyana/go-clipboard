# Slice 11 — Admin and abuse response

**Goal:** the first slice that gives the operator something to *look at*. Slice 4 gave them somewhere
to run the service and no way to see inside it; this closes the one question the design notes
explicitly park — *abuse reporting channel: undecided* — and it is where the service stops relying
entirely on the 24-hour ceiling to police itself.

**User story:** *A report comes in about `/notes`. I look up who owns it, see the same account has
been reported four times this week, suspend them — and the clips they were holding expire on their
own without my touching them.*

> **Provenance.** Most slices inherit their decisions from `clipboard-design.md`. This one does not —
> that document has no operator surface, only the parked question. So these notes are *ahead* of the
> record rather than behind it, as slices 4 and 12 also are, and want folding back into the design
> notes before anything is built.

## What I want out of it

- **The role is a column, not a system.** One privilege level above "user". A roles table or a policy
  engine for a single bit is speculative work that makes every later query harder to read.
  Config-driven admin — privileged emails in an env var — is rejected: it puts the answer to "who may
  read other people's text" somewhere a typo grants it and nothing records that it changed.
- **What an admin can see is the centre of this slice.** Slice 8 accepts "the server can read private
  content" as debt, tolerable precisely because nothing acts on it. A panel is a user interface for
  that capability. So: public clip content is visible and every view is logged; a private clip shows
  name, owner, size, and timestamps and **never** its content — not to an admin, not on a report
  about it. Deleting a clip never requires reading it. Whoever reported it already had the password;
  the panel doesn't need what the reporter had.
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
  problems that implies, a global kill switch for anonymous creation, or accepting that the tier is
  policed by its ceiling alone. Undecided, and it is the one thing here that could argue back into
  slice 5.
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
  exactly as they are for the anonymous tier, and the fallback is that the queue is pull-only, so a
  spammed queue degrades into an operator ignoring it rather than into an outage.
- **An append-only audit log**, which is the first thing here with no ceiling over it — every other
  record of something *happening* is bounded by expiry. It cannot be, because it is the only
  surviving evidence of anything the panel touched; the clip in question was deleted the same day.
  Append-only in the strongest sense that is cheap: no update or delete path exists at all, so an
  after-the-fact edit is not expressible rather than merely discouraged.
- **The sweeper becomes visible.** Slice 2 makes it housekeeping that correctness does not depend on,
  which is exactly what makes its failure silent — nothing breaks, the table just grows. The panel
  shows last sweep, rows deleted, and current expired-but-unswept count, reporting what it actually
  observed rather than what it assumes. This is also the operational read that slice 2's autovacuum
  tuning is otherwise guessing at.
- **Admin auth is not a second auth system.** Same session, same login, plus a role check — a
  separate admin login is a second attack surface guarding the same database. Two additions: an admin
  account must have 2FA enabled (which is why this sits after slice 9), and the panel is invisible
  rather than forbidden to non-admins, since "forbidden" confirms it exists and tells a
  credential-stuffer what the prize is.
- Promoting the first admin is a documented step, not an undocumented `psql` session.
- The reserved-name list grows again — the fourth exercise of slice 1's derivation mechanism.

## Not in this slice

- Paging or search on the accounts list. The ceiling bounds clips, not users.
- Recovering reported content once the clip has expired. Repeat reports against one account are the
  only durable signal — the alternative is storing exactly what the ceiling exists to discard.
- Notifying an admin that a report arrived; the queue is pull-only.
- An appeal path for a suspended account beyond whatever the abuse contact turns out to be.

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
