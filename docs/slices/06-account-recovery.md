# Slice 6 — Account recovery

**Goal:** account recovery, plus the verification mechanism lazy verification promised. Password
reset, verification email, and a verify endpoint — the first slice that needs mail.

**User story:** *I forgot my password, click a link in an email, set a new one, and I'm back — and if
someone else was using my session, they're not anymore.*

## What I want out of it

- Password reset: request, tokened link, completion. Tokens hashed at rest, single-use, expiring.
- **Completing a reset evicts every other session for that account.** A reset that leaves an
  attacker signed in is not a recovery mechanism.
- Verification email and a verify endpoint, so the flag slice 5 records finally gets set. A banner
  for unverified accounts that informs and blocks nothing.
- **Enumeration-safe throughout.** A reset request for an unknown address responds exactly like one
  for a known address.
- **Mail goes to stdout in development.** The project's convention is developer-managed local
  services rather than containers, so logging the link where `make dev` output already is beats
  adding a mail container. Needs config for a public base URL to build links from — which slice 4
  already had to introduce for the deployed environment.
- **And real delivery in the deployed environment, which is new.** The overview deferred real mail on
  the grounds that it needed somewhere deployed; slice 4 built that, so the reason has expired — and a
  reset link that only ever appears in a server log is not a recovery mechanism for anybody but the
  developer. Delivery is therefore a configured transport with credentials supplied as environment
  secrets, stdout stays the development transport, and which one is in use follows configuration
  rather than a build tag.
- The reserved-name list grows again.

## Not in this slice

- Any mail that is not a reset or a verification. No notifications, no digests, and nothing about a
  clip — the ceiling means there is never news worth sending.
- Retries, a queue, or a bounce-handling path. Send inline and let the user ask again, because the
  overview closes off queues and a failed reset is a repeatable request rather than lost data.
- Changing an email address — only verification of the original one.

## Still undecided

- Whether verified status gates anything yet. It does not need to until slice 11, where it becomes
  load-bearing for OAuth linking.
- **Which transport actually sends it**, and what the user is told when it fails. A provider API and
  SMTP differ in what they need from slice 4's secret handling, and a reset request that silently
  fails to send is indistinguishable — to the user and to the enumeration-safe response — from one
  that worked.
