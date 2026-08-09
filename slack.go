package slack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/slack-go/slack"
	"github.com/spf13/cast"

	contractsnotification "github.com/goravel/framework/contracts/notification"
)

// Channel delivers notifications to Slack via chat.postMessage.
type Channel struct {
	client *slack.Client
	token  string // kept alongside client for the empty-token fast-fail check in Deliver
}

// NewChannel constructs the Slack channel. token is the bot token
// (xoxb-...) — see https://api.slack.com/authentication/token-types.
// httpClient is optional (nil uses slack-go/slack's own default
// net/http.Client) — mainly present so tests can point requests at an
// httptest.Server instead of the real Slack API.
func NewChannel(token string, httpClient *http.Client, opts ...slack.Option) *Channel {
	if httpClient != nil {
		opts = append(opts, slack.OptionHTTPClient(httpClient))
	}
	return &Channel{client: slack.New(token, opts...), token: token}
}

func (c *Channel) Name() string { return "slack" }

func (c *Channel) Send(
	notifiable contractsnotification.Notifiable,
	n contractsnotification.Notification,
) error {
	route, payload, err := c.Resolve(notifiable, n)
	if err != nil {
		return err
	}
	return c.Deliver(route, payload)
}

func (c *Channel) Resolve(
	notifiable contractsnotification.Notifiable,
	n contractsnotification.Notification,
) (string, []byte, error) {
	// RouteNotificationFor returns any (mail: string/[]string/
	// map[string]string, database: any-castable-to-string, custom: any
	// per-channel shape). A Slack channel/user ID is a single string,
	// same shape as database's ID, so this mirrors DatabaseChannel's
	// cast.ToString(...) pattern.
	route := cast.ToString(notifiable.RouteNotificationFor("slack"))
	if route == "" {
		return "", nil, EmptyRoute.Args(notifiable)
	}

	var msg Message
	if sn, ok := n.(Notification); ok {
		msg = sn.ToSlack(notifiable)
	} else {
		msg = c.defaultMessage(n)
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return "", nil, MarshalPayload.Args(n, err)
	}

	return route, payload, nil
}

func (c *Channel) Deliver(route string, payload []byte) error {
	if route == "" {
		return nil
	}
	if c.token == "" {
		return TokenNotConfigured
	}

	var msg Message
	if err := json.Unmarshal(payload, &msg); err != nil {
		return UnmarshalPayload.Args(err)
	}

	options := []slack.MsgOption{slack.MsgOptionText(msg.Text, false)}

	if len(msg.Attachments) > 0 {
		options = append(options, slack.MsgOptionAttachments(toSDKAttachments(msg.Attachments)...))
	}
	if msg.ThreadTS != "" {
		options = append(options, slack.MsgOptionTS(msg.ThreadTS))
	}

	if _, _, err := c.client.PostMessage(route, options...); err != nil {
		return PostMessageFailed.Args(err)
	}

	return nil
}

func (c *Channel) defaultMessage(n contractsnotification.Notification) Message {
	return Message{Text: fmt.Sprintf("New notification: %T", n)}
}

// toSDKAttachments maps our own Attachment type to slack-go/slack's.
// Timestamp maps to Ts json.Number — confirmed directly against the
// real slack-go/slack source (attachments.go: `Ts json.Number
// `json:"ts,omitempty"“), not guessed; json.Number is just a string
// underneath, hence the strconv.FormatInt conversion below.
func toSDKAttachments(attachments []Attachment) []slack.Attachment {
	out := make([]slack.Attachment, 0, len(attachments))
	for _, a := range attachments {
		sa := slack.Attachment{
			Title:  a.Title,
			Text:   a.Text,
			Color:  a.Color,
			Footer: a.Footer,
		}
		if a.Timestamp > 0 {
			sa.Ts = json.Number(strconv.FormatInt(a.Timestamp, 10))
		}
		for _, f := range a.Fields {
			sa.Fields = append(sa.Fields, slack.AttachmentField{
				Title: f.Title,
				Value: f.Value,
				Short: f.Short,
			})
		}
		out = append(out, sa)
	}
	return out
}

var (
	_ contractsnotification.Channel           = (*Channel)(nil)
	_ contractsnotification.ResolvableChannel = (*Channel)(nil)
)
