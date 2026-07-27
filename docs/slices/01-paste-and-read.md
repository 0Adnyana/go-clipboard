# Slice 1 — Paste and read

**Goal:** the whole product, minus everything that makes it safe to expose. Paste text on one
device, read it on another. No accounts, no choices, one fixed lifetime — two hours, which is the
permanent ceiling for anonymous clips rather than a placeholder that grows later.

**User story:** *I paste a block of text on my desktop, name it `notes`, and two minutes later I
type `host/notes` on my phone and get exactly what I pasted — every trailing newline, every tab.*

## What I want out of it

- A create form that takes a name and a body, and a read page at `/<slug>` that shows the text and
  copies it in one click. No session shared between the two devices.
- **Text comes back out exactly as it went in.** This is the reason the project exists, so it gets
  real attention here rather than being assumed: nothing is trimmed, wrapping is display-only, and
  the copy button copies the source string rather than what the DOM happens to hold. Payloads that
  cannot round-trip are rejected loudly instead of silently mangled.
- **Names are a lease, not a registry.** A slug held by an expired clip is freely claimable; a slug
  held by a live clip is refused with a bare *"that name is in use right now"* that says nothing
  about who holds it. The claim has to be atomic — no check-then-write window — and it must not
  double as an edit operation, because editing arrives later and relaxing the claim rule to
  accommodate it would quietly break reclaim.
- **Names are case-sensitive.** `/Notes` and `/notes` are different clips. The typed
  form is preserved end to end so the URL you create is the URL you open.
- **One flat pool, and it stays flat.** `host/notes`, never `host/alice/notes`. The link is the whole
  product and it has to survive being read off a screen and typed on a phone, which is an argument
  per-user namespaces cannot answer — a prefix nobody wants to type, on a URL whose entire job is to
  be short. Everyone draws from the same pool once accounts arrive; a registered user gets no
  reservation, no precedence, and no protection from an anonymous clip holding the name they wanted.
  First come, until it expires.
- **Reserved names are derived from one list.** The backend must refuse to create a clip that would
  be shadowed by a real page or asset, and that list has to have a single source, because later
  slices keep adding pages to it.
- **Nothing gets indexed.** `robots.txt` disallows everything and clip pages carry `noindex`,
  otherwise this becomes a searchable corpus of other people's pasted credentials.
- A plain-language note near the name field about what kind of link the user just made — `/notes` is
  effectively a bulletin board, `/k7mq2p9x` is effectively a secret link. Both are legitimate uses;
  the point is that the user should know which one they picked.
- A size cap enforced before the body is read into memory.

## Not in this slice

- Accounts — but note what they do *not* change. Anyone can create here and anyone can create
  forever; what signing in buys later is a longer lifetime, not the right to paste. → slice 5
- **Any limit on creation whatsoever.** The IP-keyed limiter is the permanent control on this tier
  rather than a placeholder, and it is a weak one because IPs are free — but it does not exist yet,
  so creation here is unbounded by anything except the 2-hour ceiling. → slice 3
- Choosing or seeing the lifetime, and any cleanup of expired rows. Fixed 2 hours, and expired rows
  just accumulate harmlessly. → slice 2
- Editing. A live clip cannot be changed by anyone, including whoever made it. → slice 7
- Checking whether a name is free before submitting. → slice 2
- Private clips; the exposure note is prose, not an affordance. → slice 8

## Still undecided

- The actual content size cap — a number is needed before the first create endpoint exists.
- Slug charset and length. Leaning `[a-z0-9_-]`, 3–64, ASCII only. Open whether 1–2 character slugs
  should be claimable at all.
- Whether Tab in the textarea is intercepted for indentation or left alone as focus movement.

> **Not deployable.** This slice and slice 2 are a local milestone only. Creation is open to the
> world with nothing behind it at all — not even an IP limiter, which arrives in slice 3 — and since
> the anonymous tier is permanent, closing creation was never going to be the gate. The gate is the
> abuse response in slice 11, and slice 3 is a precondition rather than an answer. Worth confirming
> that in the overview before treating any earlier slice as shippable.
