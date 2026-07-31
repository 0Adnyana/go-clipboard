# Slice 14 — Files

**Goal:** the first slice in which a clip is not text — last, and still provisional. It answers the
overview's open question — *are files ever in scope?* — with *yes, and last*, and it sits after slice
12 for a reason: files are the first feature in the product where the operator needs a way to respond
**before** it ships rather than after.

**User story:** *I drop a 4 MB PDF on the create form and name it `contract`. My colleague types
`host/contract` on their laptop and downloads the same bytes under the same filename — and in an
hour it is gone like everything else.*

> **Provenance.** Like slices 9 and 12, this is ahead of the record rather than behind it:
> `clipboard-design.md` has nothing about files, and the overview lists them as an open question
> rather than a plan. Unlike those, this slice also *contradicts* things earlier slices assume —
> slice 1's fidelity story is about text, slice 2's sweeper is about rows, and slice 9's panel is
> written on the assumption that public content is safe to show. Those want reconciling before
> anything is built.

## What I want out of it

- **A file is a clip, not a new kind of object.** Same flat pool, same lease, same absolute ceiling,
  same countdown, same `/<slug>`. A parallel `/f/<slug>` namespace would reintroduce exactly the
  prefix idea 3 rejects and would need its own reclaim rules alongside the first set. What changes is
  what the row points at, not what a name means.
- **A clip is text or a file, never both.** cl1p allows an attachment beside the text; that turns the
  read page into two things and makes the copy button ambiguous. One kind per name, chosen at
  creation, and the link stays the whole product.
- **Bytes come back out exactly as they went in — and so does the filename.** Idea 1 stops being
  about trailing newlines and starts being about byte counts, a `Content-Type` supplied by the
  uploader that cannot be trusted, and a filename that is user content in its own right with its own
  sanitisation, encoding, and capacity to lie. A truncated download is a fidelity failure and has to
  be detectable rather than silent, which is the same standard slice 1 sets for text.
- **A file read is a download, not a page.** The copy button has no meaning here; the read page shows
  the name, the filename, the size, the countdown, and a download action. This is the one place in
  the product where the core gesture changes, and it is worth saying out loud rather than discovering
  in the UI.
- **Uploading requires an account.** Same shape of argument as private clips in slice 8, different
  reason: this is a capacity decision, not a trust one. The anonymous tier is guarded by an IP-keyed
  limiter and IPs are free, which is survivable when a request costs kilobytes and is not when it
  costs megabytes of disk the service must hold for two hours. It also means the account now buys a
  third thing beyond lifetime and ownership.
- **The download never executes in this origin.** `Content-Disposition: attachment` and
  `X-Content-Type-Options: nosniff` on every file read, and a stored content type that is either
  ignored entirely or forced to `application/octet-stream`. An uploaded `.svg` or `.html` served
  inline at `host/notes` is stored XSS against the session cookie of every signed-in reader, and idea
  4 — one origin — is precisely what puts that cookie in reach. Nothing is rendered, previewed, or
  thumbnailed anywhere in the product, admin panel included.
- **Deleting the row has to delete the bytes.** Slice 2's sweeper deletes rows and nothing else,
  which is correct while the row *is* the content. Expiry, delete-now, burn, and overwrite all become
  space-reclaim operations, and the sweeper's failure mode changes from *the table grows*, which
  slice 2 calls harmless, to *the disk fills*, which is not. Slice 9's sweeper panel gains bytes
  reclaimed next to rows deleted.
- **Burn-after-read is where at-most-once delivery stops being theoretical.** Slice 8 accepts that an
  interrupted read loses the text; an interrupted 20 MB download on hotel wifi is that same failure
  made routine rather than rare. Two things follow: the burn happens on the click that starts the
  download, so a bare `GET` still destroys nothing, and the warning beside the checkbox has to be
  louder for files than it is for text.
- **Private clips compose unchanged, and are why the bytes cannot sit at a bare public path.**
  Passing the password buys reading that one clip. If the file itself is reachable at a URL that
  skips the password check, the feature is decorative. Whatever serves the bytes goes through the
  same authorisation as the read page, and if that ever means a signed URL, its life is bounded by
  the clip's.
- **Overwrite replaces the contents, not the kind.** A text clip never becomes a file clip or the
  reverse; slice 7's rule stays *same name, same lifetime, new contents*, within one kind.
- **Reports get better and the panel gets more careful, in opposite directions.** Slice 12 snapshots a
  hash rather than content, which for a file is close to the entirety of what an operator can act on
  and is durable evidence of a sort text reports never had. Against that, slice 9's *public content
  is visible* rule does not survive arbitrary bytes: an admin sees name, filename, size, declared
  type, and hash — never the file, and never a download.
- The reserved-name list grows again if the download path turns out to be a real path.

## Not in this slice

- Multiple files per clip, archives, or folder upload. The name is a handoff, not a share. → never.
- Anonymous uploads. Not sequencing — the account *is* the capacity control. → never, unless the
  storage answer changes what a megabyte costs.
- Preview, thumbnails, or in-browser rendering of anything, for anyone. → never; see the origin
  argument above.
- Resumable or chunked upload, and resumable download. A clipboard whose contents die within a day is
  the wrong place to grow a transfer protocol. → never.
- Virus scanning. There is no scanner in the stack, and adding one means a queue, which the overview
  closes off.
- Any protection from the server itself, still. Client-side encryption is the same open question it
  is in slice 8, and files make it larger rather than different.

## Still undecided

- **Where the bytes live.** This governs everything else here. `bytea` keeps the overview's
  "Postgres and nothing else" story intact and makes deleting the content atomic with deleting the
  row, but it points 100% insert-and-delete churn at a TOAST table at megabyte scale — the bloat
  caveat an order of magnitude worse. The filesystem or an object store is the ordinary answer, and
  costs the single-dependency story plus an orphan-reclaim path that can now fail independently of
  the database.
- **Whether Go serves the download at all.** Caddy owns the origin and static delivery is explicitly
  kept out of Go; if the bytes are on disk, an internal-redirect handoff would keep it that way. But
  private clips need an authorisation check first, and that is the one thing the proxy cannot do.
- **The size cap, which stops being a validation constant.** The text cap is still open in the
  overview; the file cap is a disk budget — cap times live clips times the 24-hour ceiling — and has
  to be picked as one.
- **Whether the read URL serves the file to `curl` in one step.** The overview's raw-path question is
  easier to answer for files than for text, since a file at a URL is what `curl` is for. Answering it
  here while leaving it open for text would be an odd place to land.
- **Whether a per-account byte quota is needed on top of the per-account creation limit.** A count
  and a volume stop being the same control once clips differ in size by four orders of magnitude.
- **What the takedown posture is.** The overview's last open question is a different magnitude for
  files than for text, and slice 12's account-shaped panel plus a content hash is the whole of the
  answer this slice currently has.

> **This is the slice that can argue back the furthest.** Every earlier one is a consequence of the
> five ideas; this one puts weight on two of them — the single Postgres dependency and the single
> origin — and the honest position is that if the storage answer needs infrastructure the project has
> refused everywhere else, *text only, forever* is still the better outcome.
