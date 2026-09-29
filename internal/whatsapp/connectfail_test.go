package whatsapp

import (
	"errors"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/types/events"
)

// TestConnectFailureFromEventCoversRealDispatch pins the mapping to what
// whatsmeow actually sends.
//
// This test exists because the first version of connectfail.go matched only
// *events.ConnectFailure. That compiled, passed lint and never fired, because
// whatsmeow sends *events.ConnectFailure solely as a fallback: a 405 arrives as
// *events.ClientOutdated, a ban as *events.TemporaryBan, an unlinked device as
// *events.LoggedOut. See handleConnectFailure in whatsmeow's connectionevents.go.
//
// If a future whatsmeow reshuffles those types, this test is what notices.
func TestConnectFailureFromEventCoversRealDispatch(t *testing.T) {
	tests := []struct {
		name       string
		event      any
		wantOK     bool
		wantReason events.ConnectFailureReason
		wantPerm   bool
	}{
		{
			name:       "client outdated is its own event type, not ConnectFailure",
			event:      &events.ClientOutdated{},
			wantOK:     true,
			wantReason: events.ConnectFailureClientOutdated,
			wantPerm:   true,
		},
		{
			name:       "temporary ban",
			event:      &events.TemporaryBan{},
			wantOK:     true,
			wantReason: events.ConnectFailureTempBanned,
			wantPerm:   true,
		},
		{
			name:       "logged out during connect",
			event:      &events.LoggedOut{OnConnect: true, Reason: events.ConnectFailureLoggedOut},
			wantOK:     true,
			wantReason: events.ConnectFailureLoggedOut,
			wantPerm:   true,
		},
		{
			name:   "logout outside connect is not a connect refusal",
			event:  &events.LoggedOut{OnConnect: false, Reason: events.ConnectFailureLoggedOut},
			wantOK: false,
		},
		{
			name:       "generic fallback still maps",
			event:      &events.ConnectFailure{Reason: events.ConnectFailureServiceUnavailable},
			wantOK:     true,
			wantReason: events.ConnectFailureServiceUnavailable,
			wantPerm:   false,
		},
		{
			name:   "unrelated event ignored",
			event:  &events.Connected{},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := connectFailureFromEvent(tt.event)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got.Reason != tt.wantReason {
				t.Errorf("Reason = %d, want %d", int(got.Reason), int(tt.wantReason))
			}
			if got.Permanent() != tt.wantPerm {
				t.Errorf("Permanent() = %v, want %v", got.Permanent(), tt.wantPerm)
			}
		})
	}
}

// TestOutdatedClientErrorCarriesTheFix checks that the message a person meets
// names the cause, the rejected version and the command that fixes it. The
// whole point of the type is that nobody has to re-derive the remedy.
func TestOutdatedClientErrorCarriesTheFix(t *testing.T) {
	failure, ok := connectFailureFromEvent(&events.ClientOutdated{})
	if !ok {
		t.Fatal("client outdated did not map to a failure")
	}

	msg := failure.Error()
	for _, want := range []string{"405", "client version", "make upgrade"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q:\n%s", want, msg)
		}
	}

	// It has to survive wrapping, because that is how it reaches the exit path.
	wrapped := errors.Join(errors.New("auto-sync failed"), error(failure))
	var out ConnectFailure
	if !errors.As(wrapped, &out) {
		t.Fatal("ConnectFailure does not survive errors.As after wrapping")
	}
	if !out.Permanent() {
		t.Error("unwrapped failure lost its Permanent verdict")
	}
}
