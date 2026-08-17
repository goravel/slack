package slack_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	contractsnotification "github.com/goravel/framework/contracts/notification"
	frameworkerrors "github.com/goravel/framework/errors"
	"github.com/goravel/slack"
	"github.com/goravel/slack/contracts"
	slackgo "github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
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

type richNotification struct{ msg contracts.Message }

func (r *richNotification) Via(_ contractsnotification.Notifiable) []string { return []string{"slack"} }
func (r *richNotification) ToSlack(_ contractsnotification.Notifiable) contracts.Message {
	return r.msg
}

// ---- Test server helpers ----

// newTestServer starts an httptest.Server whose /chat.postMessage
// handler responds however respond specifies, and returns a Channel
// pointed at it plus a func to read back the last request's form
// values. lastForm is written on the server's own goroutine and read
// on the test goroutine — guarded by a mutex since httptest.Server
// serves requests concurrently with the test function continuing past
// the request that triggered the handler.
func newTestServer(t *testing.T, respond http.HandlerFunc) (*slack.Channel, func() map[string][]string, func()) {
	t.Helper()

	var mu sync.Mutex
	var lastForm map[string][]string

	mux := http.NewServeMux()
	mux.HandleFunc("/chat.postMessage", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("r.ParseForm(): %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		mu.Lock()
		lastForm = map[string][]string(r.Form)
		mu.Unlock()
		respond(w, r)
	})

	server := httptest.NewServer(mux)
	ch := slack.NewChannel("xoxb-test", slackgo.OptionAPIURL(server.URL+"/"))

	getLastForm := func() map[string][]string {
		mu.Lock()
		defer mu.Unlock()
		return lastForm
	}

	return ch, getLastForm, server.Close
}

func okResponse(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"ok": true, "channel": "C123", "ts": "1234567890.123456"}`)); err != nil {
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

// TestChannel_Send_AuthenticatesWithToken confirms the bot token
// actually reaches the wire — slack-go/slack sends it as a form field
// ("token"), not a header, so this asserts against form data rather
// than request headers.
func TestChannel_Send_AuthenticatesWithToken(t *testing.T) {
	ch, lastForm, closeServer := newTestServer(t, okResponse)
	defer closeServer()

	err := ch.Send(&slackNotifiable{route: "#general"}, &plainNotification{})
	assert.NoError(t, err)

	assert.Equal(t, "xoxb-test", lastForm()["token"][0])
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

	n := &richNotification{msg: contracts.Message{Text: "Deploy finished"}}
	err := ch.Send(&slackNotifiable{route: "#alerts"}, n)
	assert.NoError(t, err)

	form := lastForm()
	assert.Equal(t, "#alerts", form["channel"][0])
	assert.Equal(t, "Deploy finished", form["text"][0])
}

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
			assert.True(t, frameworkerrors.Is(err, slack.ErrorEmptyRoute),
				"expected slack.ErrorEmptyRoute sentinel, got: %v", err)
		})
	}
}

// TestChannel_Send_ReturnsError_WhenTokenNotConfigured exercises the
// full Send()->Resolve()->Deliver() path with no server involved — the
// token check now happens in Resolve, so this should fail before any
// network call would even be attempted.
func TestChannel_Send_ReturnsError_WhenTokenNotConfigured(t *testing.T) {
	ch := slack.NewChannel("")

	err := ch.Send(&slackNotifiable{route: "#general"}, &plainNotification{})
	assert.Error(t, err)
	assert.True(t, frameworkerrors.Is(err, slack.ErrorTokenNotConfigured),
		"expected slack.ErrorTokenNotConfigured sentinel, got: %v", err)
}

func TestChannel_Resolve_ReturnsError_WhenTokenNotConfigured(t *testing.T) {
	ch := slack.NewChannel("")

	_, _, err := ch.Resolve(&slackNotifiable{route: "#general"}, &plainNotification{})
	assert.Error(t, err)
	assert.True(t, frameworkerrors.Is(err, slack.ErrorTokenNotConfigured))
}

func TestChannel_Deliver_ReturnsError_WhenSlackAPIReturnsOkFalse(t *testing.T) {
	tests := []string{"channel_not_found", "not_in_channel"}

	for _, code := range tests {
		t.Run(code, func(t *testing.T) {
			ch, _, closeServer := newTestServer(t, errorResponse(code))
			defer closeServer()

			payload, err := json.Marshal(contracts.Message{Text: "hi"})
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

// TestChannel_Deliver_MapsAttachmentFields is the full field-mapping
// test — every Attachment/Field field asserted, not just Timestamp.
// Decodes the wire-format "attachments" form field back into a
// structured comparison rather than checking presence/individual
// values piecemeal, so a regression on any field (not just the one
// someone thought to check) fails this test.
func TestChannel_Deliver_MapsAttachmentFields(t *testing.T) {
	ch, lastForm, closeServer := newTestServer(t, okResponse)
	defer closeServer()

	payload, err := json.Marshal(contracts.Message{
		Text: "Invoice paid",
		Attachments: []contracts.Attachment{
			{
				Title:     "Details",
				Text:      "Invoice #123 has been paid in full.",
				Color:     "good",
				Footer:    "Billing system",
				Timestamp: 1700000000,
				Fields: []contracts.Field{
					{Title: "Amount", Value: "$99.00", Short: true},
					{Title: "Customer", Value: "Acme Corp", Short: true},
				},
			},
		},
	})
	assert.NoError(t, err)

	err = ch.Deliver("#billing", payload)
	assert.NoError(t, err)

	form := lastForm()
	assert.Contains(t, form, "attachments")

	var decoded []map[string]any
	assert.NoError(t, json.Unmarshal([]byte(form["attachments"][0]), &decoded))
	assert.Len(t, decoded, 1)

	att := decoded[0]
	assert.Equal(t, "Details", att["title"])
	assert.Equal(t, "Invoice #123 has been paid in full.", att["text"])
	assert.Equal(t, "good", att["color"])
	assert.Equal(t, "Billing system", att["footer"])
	// json.Number round-trips through JSON as a bare number, not a
	// quoted string — confirms Timestamp's mapping to Ts json.Number
	// used the right type, not just the right value.
	assert.EqualValues(t, 1700000000, att["ts"])

	fields, ok := att["fields"].([]any)
	assert.True(t, ok)
	assert.Len(t, fields, 2)

	field0 := fields[0].(map[string]any)
	assert.Equal(t, "Amount", field0["title"])
	assert.Equal(t, "$99.00", field0["value"])
	assert.Equal(t, true, field0["short"])

	field1 := fields[1].(map[string]any)
	assert.Equal(t, "Customer", field1["title"])
	assert.Equal(t, "Acme Corp", field1["value"])
	assert.Equal(t, true, field1["short"])
}

func TestChannel_Deliver_SendsThreadTS(t *testing.T) {
	ch, lastForm, closeServer := newTestServer(t, okResponse)
	defer closeServer()

	payload, err := json.Marshal(contracts.Message{Text: "reply", ThreadTS: "1234.5678"})
	assert.NoError(t, err)

	err = ch.Deliver("#general", payload)
	assert.NoError(t, err)

	form := lastForm()
	assert.Equal(t, "1234.5678", form["thread_ts"][0])
}
