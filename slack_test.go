package slack_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	slackgo "github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"

	contractsnotification "github.com/goravel/framework/contracts/notification"
	// frameworkerrors.Is: based on earlier-session confirmation that
	// errors/errors.go wraps stdlib errors.Is, and that this error
	// type's own Is(target) compares by template text (so
	// EmptyRoute.Args(x) still matches bare EmptyRoute via Is). Not
	// re-verified against source this session — if this doesn't
	// compile or behave as expected, fall back to
	// assert.Contains(t, err.Error(), "...") the way the rest of this
	// file still does for PostMessageFailed's wrapped SDK errors.
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/goravel/slack"
)

// ---- Fakes ----

type slackNotifiable struct{ route any }

func (n *slackNotifiable) RouteNotificationFor(channel string) any {
	if channel == "slack" {
		return n.route
	}
	return nil
}

type plainNotification struct{}

func (p *plainNotification) Via(_ contractsnotification.Notifiable) []string {
	return []string{"slack"}
}

type richNotification struct{ msg slack.Message }

func (r *richNotification) Via(_ contractsnotification.Notifiable) []string { return []string{"slack"} }
func (r *richNotification) ToSlack(_ contractsnotification.Notifiable) slack.Message {
	return r.msg
}

// ---- Test server helpers ----
//
// slack-go/slack's PostMessage sends application/x-www-form-urlencoded
// data (confirmed against the real slack-go/slack test suite, e.g.
// chat_test.go — every handler there uses r.ParseForm(), not JSON
// decoding), and OptionAPIURL(u) redirects the client at a given base
// URL — also confirmed directly from slack-go/slack's own source
// (slack.go: "OptionAPIURL set the url for the client. only useful for
// testing.").

// newTestServer starts an httptest.Server whose /chat.postMessage
// handler responds however respond specifies, and returns a Channel
// pointed at it plus a func to read back the last request's form values.
//
// ParseForm failures now fail loudly (t.Errorf + 500 response) instead
// of continuing to respond as if the request had succeeded — the
// earlier version used assert.NoError inside the handler goroutine and
// kept going, so a malformed request would still get a 200/ok response,
// masking the real problem behind a confusing downstream assertion
// failure instead of a clear one at the point of failure.
func newTestServer(t *testing.T, respond http.HandlerFunc) (*slack.Channel, func() map[string][]string, func()) {
	t.Helper()

	var lastForm map[string][]string
	mux := http.NewServeMux()
	mux.HandleFunc("/chat.postMessage", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("r.ParseForm(): %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		lastForm = map[string][]string(r.Form)
		respond(w, r)
	})

	server := httptest.NewServer(mux)
	ch := slack.NewChannel("xoxb-test", slackgo.OptionAPIURL(server.URL+"/"))

	return ch, func() map[string][]string { return lastForm }, server.Close
}

func okResponse(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"ok": true, "channel": "C123", "ts": "1234567890.123456"}`)); err != nil {
		// Test server write failures are rare (client disconnected
		// mid-response) but silently ignoring them, as the previous
		// version did, hides a real signal if it ever happens.
		panic(err)
	}
}

func errorResponse(errCode string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"ok": false, "error": "` + errCode + `"}`)); err != nil {
			panic(err)
		}
	}
}

// ---- Tests ----

func TestChannel_Name(t *testing.T) {
	ch := slack.NewChannel("xoxb-test")
	assert.Equal(t, "slack", ch.Name())
}

func TestChannel_Send_PostsDefaultMessage_WhenNotSlackNotification(t *testing.T) {
	ch, lastForm, closeServer := newTestServer(t, okResponse)
	defer closeServer()

	err := ch.Send(&slackNotifiable{route: "#general"}, &plainNotification{})
	assert.NoError(t, err)

	form := lastForm()
	assert.Equal(t, "#general", form["channel"][0])
	assert.Contains(t, form["text"][0], "slack_test.plainNotification")
}

func TestChannel_Send_UsesToSlack_WhenSlackNotification(t *testing.T) {
	ch, lastForm, closeServer := newTestServer(t, okResponse)
	defer closeServer()

	n := &richNotification{msg: slack.Message{Text: "Deploy finished"}}
	err := ch.Send(&slackNotifiable{route: "#alerts"}, n)
	assert.NoError(t, err)

	form := lastForm()
	assert.Equal(t, "#alerts", form["channel"][0])
	assert.Equal(t, "Deploy finished", form["text"][0])
}

// TestChannel_Send_ReturnsError_WhenRouteIsInvalid replaces two
// near-identical tests (empty string route, nil route) with one
// table-driven test — both exercise the exact same code path
// (cast.ToString(...) == ""), just with a different zero-value input.
func TestChannel_Send_ReturnsError_WhenRouteIsInvalid(t *testing.T) {
	tests := []struct {
		name  string
		route any
	}{
		{"empty string", ""},
		{"nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := slack.NewChannel("xoxb-test") // no server needed — should never make a request

			err := ch.Send(&slackNotifiable{route: tt.route}, &plainNotification{})
			assert.Error(t, err)
			assert.True(t, frameworkerrors.Is(err, slack.EmptyRoute),
				"expected slack.EmptyRoute sentinel, got: %v", err)
		})
	}
}

// TestChannel_Send_ReturnsError_WhenTokenNotConfigured exercises the
// full production path — Resolve succeeds, Deliver fails on the empty
// token — not just Deliver() directly. Nothing else in this file
// covered Send() reaching TokenNotConfigured before this.
func TestChannel_Send_ReturnsError_WhenTokenNotConfigured(t *testing.T) {
	ch := slack.NewChannel("") // no server needed — token check short-circuits before any request

	err := ch.Send(&slackNotifiable{route: "#general"}, &plainNotification{})
	assert.Error(t, err)
	assert.True(t, frameworkerrors.Is(err, slack.TokenNotConfigured),
		"expected slack.TokenNotConfigured sentinel, got: %v", err)
}

func TestChannel_Deliver_ReturnsError_WhenTokenNotConfigured(t *testing.T) {
	ch := slack.NewChannel("") // no server needed — token check short-circuits first

	payload, err := json.Marshal(slack.Message{Text: "hi"})
	assert.NoError(t, err)

	err = ch.Deliver("#general", payload)
	assert.Error(t, err)
	assert.True(t, frameworkerrors.Is(err, slack.TokenNotConfigured))
}

// TestChannel_Deliver_ReturnsError_WhenSlackAPIReturnsOkFalse confirms
// slack-go/slack surfaces Slack's "ok": false failures as a proper Go
// error automatically — no manual status-code/body-field parsing needed
// on our side, unlike the raw-HTTP version this replaced. Table-driven
// over two distinct Slack error codes (not just one), since a single
// case can't distinguish "we always return whatever error code Slack
// sends" from "we happen to handle this one specific code."
func TestChannel_Deliver_ReturnsError_WhenSlackAPIReturnsOkFalse(t *testing.T) {
	tests := []string{"channel_not_found", "not_in_channel"}

	for _, code := range tests {
		t.Run(code, func(t *testing.T) {
			ch, _, closeServer := newTestServer(t, errorResponse(code))
			defer closeServer()

			payload, err := json.Marshal(slack.Message{Text: "hi"})
			assert.NoError(t, err)

			err = ch.Deliver("#nonexistent", payload)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), code)
		})
	}
}

func TestChannel_Deliver_NoOp_WhenEmptyRoute(t *testing.T) {
	ch := slack.NewChannel("xoxb-test") // no server needed
	err := ch.Deliver("", []byte(`{}`))
	assert.NoError(t, err)
}

func TestChannel_Deliver_ReturnsError_WhenMalformedPayload(t *testing.T) {
	ch := slack.NewChannel("xoxb-test") // no server needed
	err := ch.Deliver("#general", []byte(`{not valid json`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestChannel_Deliver_SendsAttachments(t *testing.T) {
	ch, lastForm, closeServer := newTestServer(t, okResponse)
	defer closeServer()

	payload, err := json.Marshal(slack.Message{
		Text: "Invoice paid",
		Attachments: []slack.Attachment{
			{Title: "Details", Color: "good", Fields: []slack.Field{
				{Title: "Amount", Value: "$99.00", Short: true},
			}},
		},
	})
	assert.NoError(t, err)

	err = ch.Deliver("#billing", payload)
	assert.NoError(t, err)

	form := lastForm()
	assert.Contains(t, form, "attachments")
}

// TestChannel_Deliver_MapsAttachmentTimestamp confirms the
// Attachment.Timestamp → slack.Attachment.Ts (json.Number) conversion —
// confirmed directly against the real slack-go/slack source
// (attachments.go: `Ts json.Number`), not guessed. Attachments are sent
// as a single JSON-encoded form field ("attachments"), so this decodes
// that field back out to check the ts value round-tripped correctly as
// a number, not a string that happens to look like one.
func TestChannel_Deliver_MapsAttachmentTimestamp(t *testing.T) {
	ch, lastForm, closeServer := newTestServer(t, okResponse)
	defer closeServer()

	payload, err := json.Marshal(slack.Message{
		Text: "Deploy finished",
		Attachments: []slack.Attachment{
			{Title: "Details", Timestamp: 1700000000},
		},
	})
	assert.NoError(t, err)

	err = ch.Deliver("#deploys", payload)
	assert.NoError(t, err)

	form := lastForm()
	assert.Contains(t, form, "attachments")

	var decoded []map[string]any
	assert.NoError(t, json.Unmarshal([]byte(form["attachments"][0]), &decoded))
	assert.Len(t, decoded, 1)
	// json.Number round-trips through JSON as a bare number, not a
	// quoted string — confirms the mapping used the right type.
	assert.EqualValues(t, 1700000000, decoded[0]["ts"])
}

func TestChannel_Deliver_SendsThreadTS(t *testing.T) {
	// "thread_ts" confirmed directly against slack-go/slack's real
	// source (chat.go: MsgOptionTS does config.values.Set("thread_ts", ts)).
	ch, lastForm, closeServer := newTestServer(t, okResponse)
	defer closeServer()

	payload, err := json.Marshal(slack.Message{Text: "reply", ThreadTS: "1234.5678"})
	assert.NoError(t, err)

	err = ch.Deliver("#general", payload)
	assert.NoError(t, err)

	form := lastForm()
	assert.Equal(t, "1234.5678", form["thread_ts"][0])
}
