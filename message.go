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
//
// No Blocks (Block Kit) field — slack-go/slack's MsgOptionBlocks needs
// typed Block objects, not raw JSON, and building a typed Block Kit
// API is out of scope for this rewrite. Attachments cover the common
// "title + fields + color" notification case; add Block Kit support
// later if a real need shows up.
type Message struct {
	// Text is the fallback/plain-text message body.
	Text string
	// Attachments are legacy secondary-message attachments — still
	// supported by chat.postMessage, simpler than Block Kit for basic
	// "title + fields + color" use cases.
	Attachments []Attachment
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
