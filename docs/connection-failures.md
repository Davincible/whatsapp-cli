# When WhatsApp refuses the connection

Read this if any command fails at the handshake. The commonest case is an
outdated client, it recurs on WhatsApp's schedule rather than yours, and the
fix is one command.

## The short version

```bash
cd ~/Launchpad/Tools/whatsapp-cli && make upgrade
```

That bumps whatsmeow, prints the client version before and after, builds, runs
the tests, installs to `~/.local/bin/whatsapp`, and finishes with
`doctor --connect` so you can see the connection actually come back. Your
pairing survives; there is no QR to re-scan.

## Why it keeps happening

whatsmeow hardcodes the WhatsApp Web client version it reports at login, in
`store/clientpayload.go`. WhatsApp expires that version server-side after a few
months and then rejects the handshake with a 405. Nothing about your session,
your phone or this machine has gone wrong, and no amount of re-pairing helps.
The only fix is a newer whatsmeow carrying a newer version string.

It happened on 29 September 2026, on a whatsmeow pinned since 25 May. The
version went `2.3000.1040098269` → `2.3000.1048620361`.

`whatsapp doctor` prints the current value under **WhatsApp client version**, so
you can quote it without going to read whatsmeow's source.

## What you will see

```
error: WhatsApp refused the connection: client is out of date (405)
This build reports client version 2.3000.1040098269.
WhatsApp expires the client version whatsmeow hardcodes, so this recurs
every few months and is not caused by anything you did.
Fix it with:  cd ~/Launchpad/Tools/whatsapp-cli && make upgrade

to read the local store anyway, re-run with --no-auto-sync
```

Exit status is 1 and nothing is written to stdout. That is deliberate: see below.

## The failure this used to be

Before 29 September 2026 the same situation produced this instead:

```
Auto-syncing (last sync: 16 minutes ago)...
ERR Client outdated (405) connect failure (client version: 2.3000.1040098269)
ERR Error reading from websocket: ...
Sync timeout: the store may be behind. Re-run, or pass --no-auto-sync to read it as-is.
[ ...perfectly plausible JSON from the local database... ]
```

**Exit status 0.** Thirty seconds of wait, stale rows on stdout, and a closing
line that diagnoses a slow store when the truth is a rejected login. The real
cause was on screen, one line up, and then contradicted by the summary
underneath it. A person reading that reasonably concludes the store needs a
sync, and a script or agent reading the exit code concludes everything is fine.

This tool is used to decide what to reply to people. A read that quietly omits
today's messages while reporting success is a wrong answer, not a slow one.

Three things changed:

- **The refusal is caught and reported** with the real reason and the remedy.
- **Permanent refusals are fatal**: exit 1, no rows on stdout. A refusal that
  will never recover on its own must not degrade into a stale read.
- **It fails in about five seconds** instead of sitting out the 30-second
  auto-sync timeout.

Transient trouble — an unreachable network, a 503 — is still only a warning,
because the stored rows remain the best available answer and the warning says
so. `--no-auto-sync` reads the local store deliberately, and always exits 0.

## The trap if you are changing this code

whatsmeow does **not** send `*events.ConnectFailure` for the failures that
matter. That type is only the fallback for a reason it has no specific handling
for. The real dispatch, in whatsmeow's `connectionevents.go`:

| Refusal | Code | Event whatsmeow actually sends |
|---|---|---|
| Client outdated | 405 | `*events.ClientOutdated` |
| Temporarily banned | 402 | `*events.TemporaryBan` |
| Logged out / device gone / banned | 401, 403, 406 | `*events.LoggedOut{OnConnect: true}` |
| Session token expired or invalid | 413, 414 | `*events.CATRefreshError` (after a refresh attempt) |
| Anything else | — | `*events.ConnectFailure` |

The first attempt at this fix matched on `*events.ConnectFailure` alone. It
compiled, it passed lint, and it never fired once, because the one case anybody
cares about does not use that type. It was caught only by forcing the old
version into a test build and watching the 405 path do nothing.

`TestConnectFailureFromEventCoversRealDispatch` in
`internal/whatsapp/connectfail_test.go` pins the table above, and goes red
against exactly that mistake.

## Reproducing a 405 on purpose

You cannot wait for WhatsApp to expire a version, so force it. In a scratch
copy, set the rejected version before the CLI runs:

```go
import waStore "go.mau.fi/whatsmeow/store"

func main() {
	waStore.SetWAVersion(waStore.WAVersionContainer{2, 3000, 1040098269})
	// ... normal main
}
```

Build, run any read command, and check three things: exit status is 1, stdout is
empty, and the error carries the remedy. Never commit that call.
