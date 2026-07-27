# Slice 7 — My clips

**Goal:** the authenticated surface stops being a single form. See what you are holding, fix a typo
without renaming, delete something early.

**User story:** *I see the three clips I currently have live with their countdowns, fix a typo in one
without changing its name, and delete the one I shouldn't have pasted.*

## What I want out of it

- A dashboard listing the signed-in user's live clips with name, countdown, and size. This is what
  supersedes the rejected `localStorage` recent-clips idea — with accounts, the list belongs on the
  server. Anonymous clips appear on nobody's dashboard, which is the most visible everyday reason to
  sign in and worth saying plainly on the create form.
- **Overwrite, as a genuinely separate operation from claiming.** This is the trap laid in slice 1
  finally sprung: claiming takes over a clip *because* it expired, overwriting changes a clip
  *because* it is still live. Opposite conditions answering opposite questions. Conflating them
  silently breaks one or the other, so they stay distinct.
- **Overwriting does not touch the lifetime.** Reset-on-write is explicitly rejected; extending stays
  its own action, still bounded by the clip's own ceiling from creation — 24 hours for an owned clip.
- **Acting on someone else's clip looks like the clip does not exist.** A "forbidden" response
  confirms it does.
- **An anonymous clip cannot be overwritten or deleted by anyone**, including the person who pasted
  it, because there is nobody to check against. It expires and that is the whole story. Both actions
  answer "not found" on an unowned clip, same as they would for a stranger's — no special case, no
  second-class error. Extend stays open to anyone holding the URL, since that is what slice 5 says
  unowned means.
- Edited content goes through the same fidelity validation as created content, not a second copy of
  those rules.
- Delete-now for the owner, freeing the name for reclaim (subject to whatever slice 2 decided about a
  grace period).
- The reserved-name list grows again.

## Not in this slice

- Edit history or undo. Deliberate: a 24-hour clipboard has no version story. → never.

## Still undecided

- Whether editing is a real in-place edit or a "replace contents" action. This is also the natural
  moment to settle slice 1's Tab-key question, since editing code in place is now a real flow.
