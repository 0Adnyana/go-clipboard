# Slice 11 — OAuth

**Goal:** sign in with a provider, and land on the account you already had. The last additive slice.

**User story:** *I sign in with Google instead of a password, and it lands on the account I already
had rather than making a second one.*

## What I want out of it

The linking rule is the whole slice:

> **Rule:** auto-link only when the local account's email is verified **and** the provider asserts
> the email is verified. Otherwise require proof of ownership before merging.

Auto-linking on a bare email match is an account-takeover vector when either side is unverified — an
attacker who pre-registers the victim's address inherits their account, or the reverse. Because
verification is *lazy* by choice back in slice 5, unverified local accounts exist in normal
operation, so this rule is load-bearing rather than theoretical. This is where the verification flag
finally gates something.

The rest:

- Provider sign-in and sign-up, with state and PKCE actually validated so a replayed callback fails.
- Unlinking a provider only while the account retains another way to sign in.
- **2FA is not bypassed by a provider sign-in.** An account with 2FA enabled still gets asked.

## Not in this slice

- Passkeys, which would have nearly erased the signup wall. Set aside in order to learn conventional
  auth — a legitimate reason for this project, but not a security argument.
