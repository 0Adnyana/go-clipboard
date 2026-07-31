# Slice 4 — Deployment

**MVP goal.** Completing this slice is the minimum that counts as a shipped product: slices 1–3 plus
somewhere that is not a laptop.

**Goal:** the project acquires somewhere to run that is not a laptop, and a path from commit to
running service with no human step in the middle. Like slice 3, nothing a user can see — and for the
last time.

**User story:** *I push to `main`, and a few minutes later the thing answering at a real hostname over
TLS is exactly that commit. I built nothing by hand, I copied no binary anywhere, and when a push is
wrong I put the previous image back faster than I could work out what I broke.*

> **Provenance, and a warning.** Like slices 9, 12, and 14, this slice is ahead of the record rather
> than behind it: the overview listed *how does this get deployed?* as an open question rather than a
> plan, and this is the answer. It also argues with two things the overview says out loud — that
> Docker was rejected, and that nothing before slice 12 is deployable. Both survive. The second only
> does so on a
> distinction the overview now spells out as *publishable* instead: a **deployment is not an
> exposure**. That is doing real work here, so it gets a heading rather than a parenthesis.

## Why it sits exactly here

**Not earlier, because slices 1–3 have nothing that is allowed to leave the laptop.** They are
explicitly a local milestone, so a pipeline built alongside them would deliver, on every push,
precisely the thing those slices say must not be reachable. Slice 3 is the first slice after which a
running instance is defensible at all, which is why this one follows it immediately.

**Not later, because every remaining slice arrives holding a secret or a public URL.** Slice 6 sends
the first real mail and needs both credentials and an absolute base URL to put inside a reset link.
Slice 11 needs a client secret and a redirect URI registered against a hostname that has to exist
before the registration does. Deferring this means each of those slices is built against an
environment that does not exist and then re-verified once it does, and it means retrofitting secret
handling onto a service already running — the same class of mistake as retrofitting a limiter onto a
live login route. So this is the last position where deployment is new infrastructure rather than a
migration of a running one, which is the identical argument slice 3 makes about itself. The two
belong next to each other.

**A deployment is not an exposure, and that is the whole reconciliation.** The overview's gate is
about *publishing* the service: an address handed to people, somewhere an anonymous flood is
somebody's afternoon. What this slice delivers is machinery — an artefact, a pipeline, a proxy that
terminates TLS, and somewhere for secrets to live — pointed at an environment with no users and
nothing announced. It ships no reason to tell anyone the address, and slice 12 stays the gate,
unweakened.

**Unadvertised is not private, though, which is why this cannot come before the limiter.** A fresh host
on a real hostname is scanned within the day, so "nobody knows about it" buys obscurity and very little
else. That is the same posture slice 1's warning already describes — open to the world, found by
whoever looks — with one difference that makes it survivable here rather than a slice earlier: creation
sits behind slice 3's limiter by the time anything is running.

## What I want out of it

- **The app is containerised; the developer's Postgres still is not.** The overview rejects Docker,
  and that rejection stands precisely where it was aimed: at provisioning a *database* on a machine
  already running the target version, where a container buys reproducibility nobody needed and an
  optional `compose.yaml` rots from never being exercised. A production image answers a different
  question — *what, exactly, is running* — and it is the cheapest honest answer available. So the
  container is a deployment artefact and never a development tool. `make dev`, `air`, and a Homebrew
  Postgres stay the way this project is worked on, or the dev loop starts paying for the deploy
  story.
- **One image per commit, built once, tagged by commit SHA.** Multi-stage: the Go build and
  `pnpm build` go in, a small runtime comes out carrying the server binary and the compiled frontend
  assets. `latest` is refused as a deploy target, because it makes *which commit is live*
  unanswerable, and that is the one question a deploy must always be able to answer. The image CI
  tested is the image that runs — nothing is rebuilt from source on the host.
- **A production Caddyfile, which is where the skeleton's central bet finally gets settled.** The
  skeleton claims Caddy makes the development topology equal to production minus TLS and a build
  step, and this is the first slice where that is testable rather than asserted. It is also not free:
  the dev config proxies everything that is not `/api/*` to Vite, so static delivery, SPA fallback,
  and the `/<slug>` catch-all are currently *Vite's behaviour* rather than configuration anybody
  wrote. Production is where they get written for the first time — a file server over the built
  assets, a fallback to `index.html`, `/api/*` still winning on matcher specificity rather than block
  order — and therefore where "those things never get written in Go" becomes true instead of merely
  intended. Automatic TLS is the one line development does not have.
- **Migrations are a pipeline step, and still never a startup step.** The skeleton already runs them
  only when invoked, through `migrate up` on the same binary that serves traffic — so the image
  carries its own migration runner and the deploy step is that image with a different argument, not a
  second tool needing a second copy of `DATABASE_URL`. Running them on container start would be a
  race the moment a second container exists, and there is no advisory lock; it would also tie every
  schema change to a restart. The health endpoint's pending-migration report stops being a curiosity
  on a status page and becomes the check that catches a deploy which half-landed.
- **Configuration is environment, and secrets are in neither the repository nor the image.** The
  project consumes `DATABASE_URL` and a few ports today. This slice adds the public base URL that
  slice 6 needs for reset links, and the trusted forwarded hop that slice 3 wrote down as
  configuration and then could not fill, because the value depended on a deployment topology that did
  not exist. It exists after this slice, so that question becomes answerable here — see below, since
  the answer depends on where this runs. A secret baked into an image is a secret that needs a
  rebuild to rotate, and the overview has already deferred rotation without wanting to make it worse.
- **The pipeline's first job is to refuse.** `make test` and `make lint` already exist and are what
  CI runs: `go test`, `gofmt`, `go vet`, the frontend typecheck, and the frontend lint. Two additions
  earn their place. `sqlc generate` followed by a clean-tree check, so generated code cannot drift
  from the migrations directory it reads — the skeleton's guarantee is currently enforced by the
  developer remembering. And building the image itself on every commit, because a broken `Dockerfile`
  is a broken deploy and there is nowhere else to find that out.
- **Rollback is putting the previous tag back, and it is the first thing to reach for rather than the
  last.** The image is immutable and the database is the only state, so reverting is a pointer change
  measured in seconds against a fix measured in a push cycle. This holds *only* while each migration
  is compatible with the image before it, which is a constraint accepted rather than solved: one
  developer and one instance make "never ship a destructive migration alongside its consumer" a
  discipline, and nothing here enforces it.
- **Logs go to stdout, structured, and that is the entire observability story.** The same posture as
  no Redis and no queue — the container runtime already collects stdout, so an agent or an
  aggregation tier is a dependency bought before there is a question it answers. Mail-to-stdout
  becomes real mail in slice 6; logs-to-stdout stays exactly as it is.
- **Exactly one instance, stated as a constraint rather than discovered as one.** Every entry under
  the overview's single-instance debt — no migration advisory lock, one sweeper, slice 3's in-process
  limiter state — is correct only while the count is one, so the deployment says so out loud. It also
  turns one of those costs from theoretical into routine: a deploy restarts the process, so limiter
  state is now discarded on every push rather than on the rare restart. That is fine for the two
  IP-keyed limiters slice 3 built, and it is precisely the pressure under which slice 5's login
  lockout was told to decide whether it needs durability.

## Not in this slice

- **Telling anybody the address.** Building the road is not opening it. → slice 12 remains the
  publishability gate.
- A second instance, a load balancer, or a zero-downtime swap. The single-instance debt is carried
  forward untouched, so a deploy is a brief outage — which for a clipboard is a real answer rather
  than an embarrassed one.
- **Containerising Postgres for development, or containerising the dev loop at all.** → never; the
  overview's rejection is untouched. Production's database is a different question, below.
- A staging environment, per-PR preview deploys, or blue/green anything.
- Metrics, tracing, alerting, and uptime checks. Pull-only, like slice 12's report queue — nothing in
  this project pages anybody, and the sweeper's silent failure is answered by a panel in slice 9
  rather than by an alert here.
- Secret rotation, which the overview already defers.
- Migrations running automatically, anywhere, ever.
- Cron infrastructure, workers, or a queue. The sweeper is in-process and stays there.

## Still undecided

- **Where it runs, which governs nearly everything else here.** A single VPS with Docker Compose
  keeps the topology identical to development and hands the developer a host, its patching, and its
  disk. A managed container platform takes TLS, restarts, and the box away — but such platforms
  generally want to own the origin, and "Caddy owns the origin" is the one piece of infrastructure
  this project chose structurally. An edge terminating TLS in front of Caddy changes what idea 4 is
  asserting and changes which forwarded hop is trusted, so this decision and slice 3's open question
  are the same decision.
- **Whether production Postgres is managed or a container beside the app.** This is the one place the
  overview's Docker rejection genuinely reopens, because its premise — a machine already running the
  target version — is false on a fresh host. Managed costs money and brings backups with it; a
  container on the same box makes the database's disk the application's disk.
- **Registry, and whether the image is public.** GHCR is the obvious default given where the
  repository already lives. A public image publishes a dependency inventory: not a secret, but not
  nothing either.
- **Whether the deploy is pushed or pulled.** CI holding host credentials and performing the swap,
  versus the host watching the registry and pulling. The first puts production access into a CI
  secret; the second needs something running on the box to do the watching, which is one more thing
  that can quietly stop.
- **What is even worth backing up** — a much smaller question here than almost anywhere else, and
  worth answering rather than skipping for that reason. Everything in `clips` dies within 24 hours,
  so a backup is about accounts and, from slice 12, the audit log, which is the only table with no
  ceiling over it. A restore that loses every live clip costs close to nothing.
- **The hostname**, and whether a second one exists for anything at all.
- **Whether a few seconds of 502 on every push is acceptable.** Probably yes. Worth deciding rather
  than discovering, because a health-gated swap is the cheapest item on this list to add now and the
  most irritating to retrofit later.

> **What this slice adds that nothing tests.** A `Dockerfile`, a production proxy config, a compose
> or platform manifest, and a CI workflow: four files that are configuration rather than code, and
> the project's convention is unit tests only. Their failure mode is a deploy that does not happen,
> found out by pushing — which is tolerable at this size, and is the reason the pipeline is asked to
> build the image on every commit even when nothing is being deployed.
