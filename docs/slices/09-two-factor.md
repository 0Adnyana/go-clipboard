# Slice 9 — Two-factor

**Goal:** TOTP on accounts. Strictly additive, and deliberately late — 2FA protects an account whose
entire asset base evaporates within a day. What the account really protects is the ability to create.

**User story:** *I scan a QR code into my authenticator, save my recovery codes, and from then on
signing in asks for a six-digit code.*

## What I want out of it

- Enrolment, recovery codes, and a second step at login.
- **The TOTP secret is encrypted at rest.** This introduces the first application-level secret beyond
  the database URL, so it is also a new required config value and a new startup failure mode — the
  server should refuse to start without it rather than dying later at enrolment.
- **Recovery codes are hashed and single-use**, or they are just a second password stored in the
  clear. Using one reveals nothing about the others.
- **The half-finished login state stays useless.** Between "password correct" and "fully
  authenticated" there is an intermediate token, and over-privileging it is one of the most common
  auth bugs there is. It must not create a clip, unlock a private clip, or reach account settings.
- Disabling 2FA requires re-authentication.
- Sane clock handling: a step of skew either way is accepted, and replaying the same code inside a
  step is not.

## Not in this slice

- Any rotation story for the app secret. Worth a note in the config docs.
- Trusted-device / remember-this-browser.
