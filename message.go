package slack

import (
	contractsnotification "github.com/goravel/framework/contracts/notification"
)

// Notification is implemented by notifications that want full control
// over the outgoing Slack message. If a notification going through the
// "slack" channel doesn't implement this, a minimal default message
// (just the notification's type name) is sent instead.
type Notification interface {
	contractsnotification.Notification
	// ToSlack returns the Message to post via the Slack Web API.
	ToSlack(notifiable contractsnotification.Notifiable) Message
}

// Message is a chat.postMessage payload. See
// https://api.slack.com/methods/chat.postMessage for field semantics.
// Only the fields commonly needed for notifications are exposed —
// Blocks (Block Kit) is deliberately left as raw json.RawMessage rather
// than a typed builder, since Block Kit's schema is large and mostly
// orthogonal to what a "send a notification" use case needs; pass
// pre-built JSON if you need it.
type Message struct {
	// Text is the fallback/plain-text message body. Required by Slack
	// even when Blocks is set (used for notifications, screen readers).
	Text string
	// Attachments are legacy secondary-message attachments — still
	// supported by chat.postMessage, simpler than Block Kit for basic
	// "title + fields + color" use cases.
	Attachments []Attachment
	// Blocks is raw Block Kit JSON (a JSON array), for callers that need
	// richer layouts than Attachments supports. Leave nil to omit.
	Blocks []byte
	// ThreadTS, if set, posts this message as a reply in the given
	// thread (Slack's "ts" of the parent message).
	ThreadTS string
}

// Attachment is a single legacy Slack message attachment.
type Attachment struct {
	// Title is the bold attachment title.
	Title string
	// Text is the attachment body text.
	Text string
	// Color is "good", "warning", "danger", or a hex string like "#36a64f".
	Color string
	// Fields are key-value pairs displayed in a table inside the attachment.
	Fields []Field
	// Footer is small text shown at the bottom of the attachment.
	Footer string
	// Timestamp is a Unix timestamp shown in the attachment footer.
	Timestamp int64
}

// Field is a single key-value pair inside an Attachment.
type Field struct {
	// Title is the field label.
	Title string
	// Value is the field content (supports Slack mrkdwn).
	Value string
	// Short controls whether the field appears side-by-side with other short fields.
	Short bool
}
