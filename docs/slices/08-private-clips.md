# Slice 8 — Private clips

**Goal:** "public" stops being the only option, and slice 1's exposure note becomes a real
affordance. Two independent controls arrive together: a password on the clip, and burn-after-read.

**User story:** *I'm moving an API key, so I set a password on the clip. Anyone with the URL sees a
password prompt and nothing else — not the content, not its length, not who made it. I also tick
burn-after-read, so once it lands on the other machine it is gone rather than sitting there for the
rest of the hour.*

## What I want out of it

- **Private means a server-side password on the clip**, hashed, with attempts throttled per clip so
  "private" isn't decorative. Write access stays owner-only always: a read password buys reading, not
  writing.
- **The throttle is a counter on the clip row, not a registration into slice 3's limiter.** This is
  the one limit in the project that does not want that mechanism. Its key is a clip, so the counter
  is durable for free, deleted with the clip it protects, and has no cleanup problem to solve —
  where slice 3's in-process map would forget every attempt on a deploy and hold state for clips that
  no longer exist. The cost argument runs the same way as login's in slice 5: an attempt already pays
  for a password hash verification, so the write disappears next to it. Slice 3's shared interface is
  still the right shape to express *may this key act, and if not, for how long*; only the storage
  differs.
- **Private requires an account**, because write access is owner-only and an anonymous clip has no
  owner. The person who made it could not correct a typo in it, change the password, remove it, or
  delete it — they would be locked out of their own secret the moment they submitted it. That is not
  a thin version of the feature, it is a broken one.
- **Passing the password grants access to that one clip.** Not a session upgrade, not something that
  outlives the clip. Over-privileging this grant is the same class of mistake as over-privileging a
  half-finished 2FA login.
- **The locked screen leaks only existence.** Telling someone a password is required is unavoidable,
  so private clips are the deliberate exception to enumeration safety — but nothing else escapes: no
  content, no byte size, no timestamps, no owner.
- **No separate visibility flag.** "Public" means there is no password, full stop. Storing the same
  fact twice is how you end up with a clip marked public that still demands a password.
- **The private option is prominent at creation**, not buried in settings. Someone pasting an API key
  at `/test` has misunderstood what they are doing, and the create form is the only place to catch
  it. Signed-out, the option is visible and prompts a sign-in rather than being hidden — hiding it
  would remove the warning from exactly the person most likely to need it.
- **Collisions become unexplainable, and that is the accepted price of a flat namespace.** Bob types
  `notes`, Alice's private clip holds it, and the honest answer cannot be given. What keeps this from
  being a leak is that the refusal is *already* bare and identical in every case — slice 1's "that
  name is in use right now" says nothing about the holder whether the holder is public or private, so
  private clips add no new signal here. The cost is real but smaller: a user with no explanation and
  nothing to do but pick another name.
- Removing a password makes the clip public and actually clears the stored hash.
- Public clips are entirely unaffected — no extra field, no extra request, no behaviour change.

## Burn-after-read

A second, independent control: the clip is deleted once it has been read. Not a property of private
clips — **available on password and non-password clips alike**, and on anonymous ones, since burning
is driven by a reader's access rather than by anything only an owner can do.

- **Reading is a click, not a page load.** The read page for a burn clip loads inert — a warning and
  a *reveal* button, no content — and only the click destroys it. This is the decision the whole
  feature rests on: a page load is something a link unfurler, a prefetcher, or a curious owner can
  cause without meaning to, and any of those burning the clip means the recipient gets nothing.
  Making the destructive step an explicit act also makes it an *intentional* one, which is the right
  shape for something irreversible.
- **The two controls compose rather than nest.** Password, burn, both, or neither. On a clip that has
  both, submitting the correct password *is* the reveal — one deliberate act, not two, and the
  password form already cannot be triggered by a passing fetch. Reaching the locked screen never
  burns anything; if it did, anyone holding the URL could destroy a clip they cannot read, which is a
  one-request denial of service handed to every passer-by.
- **Time still applies.** A burn clip dies at first read *or* at expiry, whichever comes first. Burn
  shortens a lifetime; it never extends one, and it is not an alternative to picking a TTL.
- **Delivery is at-most-once, and the failure mode is losing the content.** If the read is
  interrupted after the clip is gone, it is gone — there is nothing to retry against. This is
  inherent to the feature rather than a bug to engineer around, but it is the thing to say plainly on
  the create form, because a user who expected a second chance has lost their text.
- **A burned clip is indistinguishable from an expired or never-existing one**, and the name returns
  to the pool on the same terms as any other dead clip. Nothing says "this was read" — that would
  tell an attacker their guess landed.
- **A public burn clip can still be destroyed by anyone who has the URL** — click-to-reveal means
  they have to mean it, not that they have to be authorised. That is the feature working as
  specified; it is also the strongest argument for pairing burn with a password, and worth saying
  next to the checkbox.
- **A bare `GET` never destroys anything, anywhere in the product.** That is worth holding as a rule
  rather than as a detail of this feature, because it is what makes the read URL safe to paste into
  a chat window, a bookmark, or a preview pane. Its cost is that `curl <url>` cannot fetch a burn
  clip in one step — see the overview's open question about whether the raw path is supported at all.

## Not in this slice

- Password recovery for a clip, by design — it expires within a day. → never.
- Any protection from the server itself: content is stored as plaintext and the server can read it.
  The password gates access to the bytes rather than protecting them. See below.

## Settled: a server-checked password over plaintext

The alternative was client-side encryption — deriving a key from the password in the browser so the
server stores only ciphertext and has nothing to surrender. Not taken. This is the last slice where
declining is cheap, so the reasons are recorded rather than left open.

- **It is not about the wire.** TLS already keeps the content from anyone on the network path. The
  only thing client-side encryption changes is whether the *server* can read what it is holding —
  a trust claim, not a fix for an attack this service is likely to meet. The 24-hour ceiling already
  caps a breach at whatever happens to be live in that instant; there is no archive to steal.
- **It would take the server out of the loop on the things this slice is built from.** With no way to
  tell a right password from a wrong one there is no "wrong password" message and no meaningful
  per-clip throttle: one fetch of the ciphertext and every further guess runs offline on the
  attacker's hardware, unobserved. The whole defence would rest on how good a passphrase the user
  chose, which is worse than a throttled online guess for exactly the careless user the prominent
  create-form option exists to catch.
- **Burn-after-read gets a failure mode it does not currently have.** The server would have to destroy
  the clip before anyone can know the passphrase was right, so a typo costs the content.
- **A forgotten passphrase would be terminal for the owner too**, not only for readers — it would take
  away the typo-fixing ability that is the stated reason private requires an account.
- **The raw/`curl` path is gone for private clips**, and reading one would require JavaScript forever.

Encryption at rest is a different and much smaller question, deferred to a later slice. It is
internal, invisible to users, and retrofittable with a migration — and it does not retire the caveat
above, because the key has to live wherever the running app can reach it.
