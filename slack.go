package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/spf13/cast"

	contractsnotification "github.com/goravel/framework/contracts/notification"

	"github.com/goravel/slack/contracts"
)

const defaultTimeout = 30 * time.Second

// Channel delivers notifications to Slack via chat.postMessage.
type Channel struct {
	client *slack.Client
	token  string
}

func NewChannel(token string, opts ...slack.Option) *Channel {
	return &Channel{client: slack.New(token, opts...), token: token}
}

func (c *Channel) Name() string { return contracts.ChannelName }

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

	if c.token == "" {
		return "", nil, ErrorTokenNotConfigured
	}

	var route string
	if r, ok := notifiable.(contracts.Routable); ok {
		route = r.RouteNotificationForSlack(n)
	}
	if route == "" {
		route = cast.ToString(notifiable.RouteNotificationFor(contracts.ChannelName))
	}
	if route == "" {
		return "", nil, ErrorEmptyRoute.Args(notifiable)
	}

	var msg contracts.Message
	if sn, ok := n.(contracts.Notification); ok {
		msg = sn.ToSlack(notifiable)
	} else {
		msg = c.defaultMessage(n)
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return "", nil, ErrorMarshalPayload.Args(n, err)
	}

	return route, payload, nil
}

func (c *Channel) Deliver(route string, payload []byte) error {
	if route == "" {
		return nil
	}

	var msg contracts.Message
	if err := json.Unmarshal(payload, &msg); err != nil {
		return ErrorUnmarshalPayload.Args(err)
	}

	options := make([]slack.MsgOption, 0, 3) // text + attachments + thread_ts, the common case
	options = append(options, slack.MsgOptionText(msg.Text, false))

	if len(msg.Attachments) > 0 {
		options = append(options, slack.MsgOptionAttachments(toSDKAttachments(msg.Attachments)...))
	}
	if msg.ThreadTS != "" {
		options = append(options, slack.MsgOptionTS(msg.ThreadTS))
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if _, _, err := c.client.PostMessageContext(ctx, route, options...); err != nil {
		return ErrorPostMessageFailed.Args(err)
	}

	return nil
}

func (c *Channel) defaultMessage(n contractsnotification.Notification) contracts.Message {
	typeName := strings.TrimPrefix(fmt.Sprintf("%T", n), "*")
	return contracts.Message{Text: fmt.Sprintf("You have a new %s notification.", typeName)}
}

func toSDKAttachments(attachments []contracts.Attachment) []slack.Attachment {
	out := make([]slack.Attachment, 0, len(attachments))
	for _, a := range attachments {
		sa := slack.Attachment{
			Title:  a.Title,
			Text:   a.Text,
			Color:  a.Color,
			Footer: a.Footer,
			Fields: make([]slack.AttachmentField, 0, len(a.Fields)),
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
