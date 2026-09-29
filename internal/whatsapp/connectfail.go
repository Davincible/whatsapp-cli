package whatsapp

import (
	"fmt"

	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types/events"
)

// repoHint is where `make upgrade` lives. A pointer for the reader, not a path
// this process resolves.
const repoHint = "~/Launchpad/Tools/whatsapp-cli"

// ConnectFailure is a refusal from WhatsApp's servers during the login
// handshake, captured so a command can report why it has no connection.
//
// Without this the refusal was invisible to this process. whatsmeow logs it and
// gives up, but nothing here noticed, so every read fell through to the
// 30-second auto-sync timeout, printed "the store may be behind" — a stale-store
// diagnosis for what is actually a rejected login — and exited 0 with whatever
// rows the database already held.
//
// Note on the event types below. whatsmeow does NOT send *events.ConnectFailure
// for the failures that matter; that is only the fallback for a reason it has no
// specific handling for. An outdated client arrives as *events.ClientOutdated, a
// ban as *events.TemporaryBan, an unlinked device as *events.LoggedOut. Matching
// on ConnectFailure alone compiles, passes lint and silently never fires, which
// is exactly what the first version of this file did.
type ConnectFailure struct {
	Reason  events.ConnectFailureReason
	Message string
}

// connectFailureFromEvent maps a whatsmeow event to a failure, reporting false
// for events that are not a login refusal.
func connectFailureFromEvent(evt any) (ConnectFailure, bool) {
	switch v := evt.(type) {
	case *events.ClientOutdated:
		return ConnectFailure{
			Reason:  events.ConnectFailureClientOutdated,
			Message: "client is out of date",
		}, true
	case *events.TemporaryBan:
		return ConnectFailure{
			Reason:  events.ConnectFailureTempBanned,
			Message: fmt.Sprintf("temporarily banned: %s", v.String()),
		}, true
	case *events.LoggedOut:
		// Only a logout that arrives during connect is a refusal. A logout
		// raised mid-session is the phone unlinking us and is reported
		// elsewhere; either way the remedy is the same.
		if !v.OnConnect {
			return ConnectFailure{}, false
		}
		return ConnectFailure{Reason: v.Reason, Message: v.Reason.String()}, true
	case *events.CATRefreshError:
		return ConnectFailure{
			Reason:  events.ConnectFailureCATExpired,
			Message: fmt.Sprintf("session token refresh failed: %v", v.Error),
		}, true
	case *events.ConnectFailure:
		return ConnectFailure{Reason: v.Reason, Message: v.Reason.String()}, true
	default:
		return ConnectFailure{}, false
	}
}

// Permanent reports whether retrying could ever succeed. An outdated client, an
// unlinked device or a ban stay broken until something changes on this machine,
// so a command should stop and say so rather than serve stale rows under a
// warning nobody reads.
func (f ConnectFailure) Permanent() bool {
	switch f.Reason {
	case events.ConnectFailureClientOutdated,
		events.ConnectFailureBadUserAgent,
		events.ConnectFailureLoggedOut,
		events.ConnectFailureUnknownLogout,
		events.ConnectFailureMainDeviceGone,
		events.ConnectFailureTempBanned,
		events.ConnectFailureCATExpired,
		events.ConnectFailureCATInvalid:
		return true
	default:
		return false
	}
}

// Remedy returns the specific thing to do about this failure, or "" when there
// is nothing better to say than the reason itself. The point is that whoever
// meets the error does not have to work the fix out a second time.
func (f ConnectFailure) Remedy() string {
	switch f.Reason {
	case events.ConnectFailureClientOutdated, events.ConnectFailureBadUserAgent:
		return "WhatsApp expires the client version whatsmeow hardcodes, so this recurs\n" +
			"every few months and is not caused by anything you did.\n" +
			"Fix it with:  cd " + repoHint + " && make upgrade"
	case events.ConnectFailureLoggedOut, events.ConnectFailureUnknownLogout, events.ConnectFailureMainDeviceGone:
		return "The phone has unlinked this device. Re-pair with:  whatsapp auth login"
	case events.ConnectFailureTempBanned:
		return "This account is temporarily banned. Nothing to fix here; wait it out."
	case events.ConnectFailureCATExpired, events.ConnectFailureCATInvalid:
		return "The stored session token is no longer valid. Re-pair with:  whatsapp auth login"
	default:
		return ""
	}
}

// Error renders the failure the way a person meets it: what happened, the client
// version that was rejected, and what to do next.
func (f ConnectFailure) Error() string {
	msg := fmt.Sprintf("WhatsApp refused the connection: %s (%d)", f.Message, int(f.Reason))
	if f.Reason == events.ConnectFailureClientOutdated || f.Reason == events.ConnectFailureBadUserAgent {
		msg += fmt.Sprintf("\nThis build reports client version %s.", store.GetWAVersion().String())
	}
	if r := f.Remedy(); r != "" {
		msg += "\n" + r
	}
	return msg
}
